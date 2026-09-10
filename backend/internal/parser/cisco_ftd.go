package parser

import (
	"regexp"
	"strings"
	"time"

	"geoatlas/internal/model"
)

type CiscoFTD struct{}

func (p *CiscoFTD) Vendor() string { return "cisco-ftd" }

func (p *CiscoFTD) CanParse(line string) bool {
	if !strings.Contains(line, "%FTD-") {
		return false
	}
	// KV connection-события (430002/430003), 313009 или Lina-коды.
	return strings.Contains(line, "AccessControlRuleAction:") ||
		strings.Contains(line, "SrcIP:") ||
		strings.Contains(line, "%FTD-4-313009:") ||
		ftdConnHintRE.MatchString(line) ||
		ftdLinaHintRE.MatchString(line)
}

// ftdConnHintRE — connection start/end (430002 / 430003).
var ftdConnHintRE = regexp.MustCompile(`%FTD-\d+-43000[23]:`)

// ftdLinaHintRE — быстрый предикат «это Lina-код, который мы умеем».
// Список id должен совпадать с linaPatterns (без дублирования логики матчинга).
var ftdLinaHintRE = regexp.MustCompile(
	`%FTD-\d+-(?:30201[3-8]|30202[01]|106023|106100|106001|10600[67]|106014|106015|106021|710003):`,
)

var ftdDeny313009RE = regexp.MustCompile(
	`%FTD-\d+-313009:\s+Denied[^,]*,\s+for\s+([\w_-]+):([\d.]+)/(\d+)\s+\([^)]*\)\s+to\s+([\w_-]+):([\d.]+)/(\d+)`,
)

func (p *CiscoFTD) Parse(line string) (model.TrafficLog, bool) {
	// 1) 313009 — Denied ICMP (отдельный текстовый формат)
	if m := ftdDeny313009RE.FindStringSubmatch(line); m != nil {
		base := newCiscoBase(line, "cisco-ftd")
		base.Proto = "ICMP"
		base.Action = "deny"
		base.SrcZone, base.SrcIP, base.SrcPort = m[1], m[2], parseUint32(m[3])
		base.DstZone, base.DstIP, base.DstPort = m[4], m[5], parseUint32(m[6])
		return base, true
	}

	// 2) KV connection-события Firepower (AccessControlRuleAction / SrcIP)
	if tl, ok := parseFTDConnection(line); ok {
		return tl, true
	}

	// 3) Lina-коды (общие с ASA)
	return parseLina(line, "cisco-ftd")
}

// ShouldSkip: см. CiscoASA.ShouldSkip — та же эвристика «нет пары IPv4 → не трафик».
func (p *CiscoFTD) ShouldSkip(line string) bool {
	return len(ipv4RE.FindAllString(line, 2)) < 2
}

type ftdConnExt struct {
	srcIP, dstIP, srcPort, dstPort               string
	action, rule, proto                          string
	ingressZone, ingressIf, egressZone, egressIf string
	initiatorBytes, responderBytes               string
	initiatorPackets, responderPackets           string
	firstPacketSecond                            string
}

func (e *ftdConnExt) set(k, v string) {
	switch k {
	case "SrcIP":
		e.srcIP = v
	case "DstIP":
		e.dstIP = v
	case "SrcPort":
		e.srcPort = v
	case "DstPort":
		e.dstPort = v
	case "AccessControlRuleAction":
		e.action = v
	case "AccessControlRuleName":
		e.rule = v
	case "Protocol":
		e.proto = v
	case "IngressZone":
		e.ingressZone = v
	case "IngressInterface":
		e.ingressIf = v
	case "EgressZone":
		e.egressZone = v
	case "EgressInterface":
		e.egressIf = v
	case "InitiatorBytes":
		e.initiatorBytes = v
	case "ResponderBytes":
		e.responderBytes = v
	case "InitiatorPackets":
		e.initiatorPackets = v
	case "ResponderPackets":
		e.responderPackets = v
	case "FirstPacketSecond":
		e.firstPacketSecond = v
	}
}

func parseFTDConnection(line string) (model.TrafficLog, bool) {
	// Реальные логи (Cisco / Rapid7) могут начинать KV с любого из этих ключей,
	// без EventPriority/DeviceUUID.
	idx := -1
	for _, marker := range []string{
		"EventPriority:", "DeviceUUID:", "AccessControlRuleAction:", "SrcIP:",
	} {
		if i := strings.Index(line, marker); i != -1 && (idx == -1 || i < idx) {
			idx = i
		}
	}
	if idx == -1 {
		return model.TrafficLog{}, false
	}

	var f ftdConnExt
	walkFTDKV(line[idx:], f.set)

	if f.srcIP == "" || f.dstIP == "" {
		return model.TrafficLog{}, false
	}

	base := newCiscoBase(line, "cisco-ftd")

	// Приоритет времени: FirstPacketSecond > syslog-префикс > время приёма.
	if f.firstPacketSecond != "" {
		if t, err := time.Parse(time.RFC3339, f.firstPacketSecond); err == nil {
			base.Timestamp = t
		}
	}

	srcZone := f.ingressZone
	if srcZone == "" {
		srcZone = f.ingressIf
	}
	dstZone := f.egressZone
	if dstZone == "" {
		dstZone = f.egressIf
	}

	proto := f.proto
	if proto != "" {
		proto = strings.ToUpper(proto)
	}

	base.SrcIP = f.srcIP
	base.DstIP = f.dstIP
	base.SrcPort = parseUint32(f.srcPort)
	base.DstPort = parseUint32(f.dstPort)
	base.Action = normalizeFTDAction(f.action)
	base.Rule = f.rule
	base.Proto = proto
	// В логах встречаются и Zone, и Interface (docs Rapid7 / Cisco SFIMS).
	base.SrcZone = srcZone
	base.DstZone = dstZone
	base.BytesSent = parseUint64(f.initiatorBytes)
	base.BytesRecv = parseUint64(f.responderBytes)
	base.PacketsSent = parseUint64(f.initiatorPackets)
	base.PacketsRecv = parseUint64(f.responderPackets)
	return base, true
}

// walkFTDKV вызывает fn для каждой пары "Key: value" в формате
// "Key1: value1, Key2: value2, ..." — то же разбиение, что у прежнего
// strings.Split(s, ", ") + SplitN(":", 2).
func walkFTDKV(s string, fn func(key, val string)) {
	start := 0
	n := len(s)
	for i := 0; i < n; i++ {
		if s[i] != ',' || i+1 >= n || s[i+1] != ' ' {
			continue
		}
		emitFTDKVPart(s[start:i], fn)
		start = i + 2
		i++ // skip the space; loop ++ moves past it
	}
	emitFTDKVPart(s[start:], fn)
}

func emitFTDKVPart(p string, fn func(key, val string)) {
	colon := strings.IndexByte(p, ':')
	if colon < 0 {
		return
	}
	key := strings.TrimSpace(p[:colon])
	if key == "" {
		return
	}
	fn(key, strings.TrimSpace(p[colon+1:]))
}

func normalizeFTDAction(action string) string {
	s := strings.ToLower(strings.TrimSpace(action))
	switch s {
	case "allow", "trust", "monitor":
		return "allow"
	case "block", "block with reset",
		"interactive block", "interactive block with reset":
		return "block"
	default:
		if s == "" {
			return "unknown"
		}
		return s
	}
}
