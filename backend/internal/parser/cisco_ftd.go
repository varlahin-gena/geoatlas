package parser

import (
	"regexp"
	"strings"
	"time"

	"network_monitor/internal/model"
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
	kv := parseFTDKV(line[idx:])

	src := strings.TrimSpace(kv["SrcIP"])
	dst := strings.TrimSpace(kv["DstIP"])
	if src == "" || dst == "" {
		return model.TrafficLog{}, false
	}

	base := newCiscoBase(line, "cisco-ftd")

	// Приоритет времени: FirstPacketSecond > syslog-префикс > время приёма.
	if fps := strings.TrimSpace(kv["FirstPacketSecond"]); fps != "" {
		if t, err := time.Parse(time.RFC3339, fps); err == nil {
			base.Timestamp = t
		}
	}

	base.SrcIP = src
	base.DstIP = dst
	base.SrcPort = parseUint32(kv["SrcPort"])
	base.DstPort = parseUint32(kv["DstPort"])
	base.Action = normalizeFTDAction(kv["AccessControlRuleAction"])
	base.Rule = strings.TrimSpace(kv["AccessControlRuleName"])
	base.Proto = strings.ToUpper(strings.TrimSpace(kv["Protocol"]))
	// В логах встречаются и Zone, и Interface (docs Rapid7 / Cisco SFIMS).
	base.SrcZone = firstNonEmpty(kv["IngressZone"], kv["IngressInterface"])
	base.DstZone = firstNonEmpty(kv["EgressZone"], kv["EgressInterface"])
	base.BytesSent = parseUint64(kv["InitiatorBytes"])
	base.BytesRecv = parseUint64(kv["ResponderBytes"])
	base.PacketsSent = parseUint64(kv["InitiatorPackets"])
	base.PacketsRecv = parseUint64(kv["ResponderPackets"])
	return base, true
}

// parseFTDKV парсит формат "Key1: value1, Key2: value2, ...".
func parseFTDKV(s string) map[string]string {
	out := make(map[string]string, 32)
	for _, p := range strings.Split(s, ", ") {
		if kv := strings.SplitN(p, ":", 2); len(kv) == 2 {
			out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return out
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
