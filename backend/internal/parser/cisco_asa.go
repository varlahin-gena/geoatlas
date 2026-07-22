package parser

import (
	"regexp"
	"strings"
	"time"

	"network_monitor/internal/model"
)

type CiscoASA struct{}

func (p *CiscoASA) Vendor() string { return "cisco-asa" }

func (p *CiscoASA) CanParse(line string) bool {
	return strings.Contains(line, "%ASA-")
}

func (p *CiscoASA) Parse(line string) (model.TrafficLog, bool) {
	return parseLina(line, "cisco-asa")
}

// ShouldSkip: строка распознана как ASA (CanParse), но не разобрана.
// Если в ней нет пары IPv4 — это админ/системное событие, не относящееся
// к графу. Помечаем как осознанный пропуск (не ошибка парсинга).
// Если IP-пара есть, а разобрать не смогли — вернётся false, и строка
// попадёт в parse_errors, сигнализируя о пробеле в покрытии.
func (p *CiscoASA) ShouldSkip(line string) bool {
	return len(ipv4RE.FindAllString(line, 2)) < 2
}

// ============================================================
// Общий разбор Lina-сообщений (ASA и FTD используют одни и те же
// коды/форматы, отличается лишь префикс %ASA-/%FTD-).
// ============================================================

type linaPattern struct {
	id     string         // код(ы) — для справки
	re     *regexp.Regexp // именованные группы: src,dst,sport,dport,szone,dzone,proto,rule,act,dir
	action string         // фиксированное действие ("allow"/"deny") или "" — вывести из группы act
	proto  string         // фиксированный протокол, если не захвачен группой proto
}

// Префикс %(?:ASA|FTD)-<sev>-<id>: — общий для обоих вендоров.
//
// Важно про 302013/15 Built: ASA всегда пишет интерфейс с меньшим security level
// первым (после for), а с большим — вторым (после to). Направление задаёт
// inbound|outbound: при outbound инициатор — вторая сторона (см. Cisco doc 116149).
var linaPatterns = []linaPattern{
	// --- Разрешённый/учётный трафик (Built/Teardown) ---

	// 302013/14/15/16 — Built/Teardown TCP|UDP connection
	// Пример (Cisco FAQ): Built outbound TCP connection 9 for outside:10.1.2.1/22 (...) to inside:10.1.1.2/53496 (...)
	{
		id: "302013-302016",
		re: regexp.MustCompile(
			`%(?:ASA|FTD)-\d+-30201[3-6]:\s+(?:Built|Teardown)\s+(?:(?P<dir>inbound|outbound)\s+)?(?P<proto>TCP|UDP)\s+connection\s+\d+\s+for\s+(?P<szone>[^:]+):(?P<src>\d+\.\d+\.\d+\.\d+)/(?P<sport>\d+).*?\bto\s+(?P<dzone>[^:]+):(?P<dst>\d+\.\d+\.\d+\.\d+)/(?P<dport>\d+)`),
		action: "allow",
	},
	// 302017/18 — Built/Teardown GRE connection
	{
		id: "302017-302018",
		re: regexp.MustCompile(
			`%(?:ASA|FTD)-\d+-30201[78]:\s+(?:Built|Teardown)\s+(?:(?P<dir>inbound|outbound)\s+)?.*?\bfor\s+(?P<szone>[^:]+):(?P<src>\d+\.\d+\.\d+\.\d+)/(?P<sport>\d+).*?\bto\s+(?P<dzone>[^:]+):(?P<dst>\d+\.\d+\.\d+\.\d+)/(?P<dport>\d+)`),
		action: "allow",
		proto:  "GRE",
	},
	// 302020/21 — Built/Teardown ICMP connection (faddr=dst, laddr=src)
	{
		id: "302020-302021",
		re: regexp.MustCompile(
			`%(?:ASA|FTD)-\d+-30202[01]:\s+(?:Built|Teardown).*?ICMP\s+connection\s+for\s+faddr\s+(?P<dst>\d+\.\d+\.\d+\.\d+)/\d+\s+gaddr\s+[\d.]+/\d+\s+laddr\s+(?P<src>\d+\.\d+\.\d+\.\d+)`),
		action: "allow",
		proto:  "ICMP",
	},

	// --- Deny / ACL ---

	// 106023 — Deny|Permit PROTO src ZONE:IP/port dst ZONE:IP/port by access-group NAME
	// Пример: %ASA-4-106023: Deny tcp src outside:192.168.0.50/21979 dst inside:192.168.0.49/23 by access-group "acl_outside"
	{
		id: "106023",
		re: regexp.MustCompile(
			`(?i)%(?:ASA|FTD)-\d+-106023:\s+(?P<act>deny|permit)\s+(?P<proto>\S+)\s+(?:src\s+)?(?P<szone>[^:]+):(?P<src>\d+\.\d+\.\d+\.\d+)(?:/(?P<sport>\d+))?.*?\bdst\s+(?P<dzone>[^:]+):(?P<dst>\d+\.\d+\.\d+\.\d+)(?:/(?P<dport>\d+))?.*?access-group\s+"?(?P<rule>[^"\s]+)`),
	},
	// 106100 — access-list NAME permitted|denied PROTO ZONE/IP(port) -> ZONE/IP(port)
	{
		id: "106100",
		re: regexp.MustCompile(
			`(?i)%(?:ASA|FTD)-\d+-106100:\s+access-list\s+(?P<rule>\S+)\s+(?P<act>permitted|denied)\s+(?P<proto>\S+)\s+(?P<szone>[^/]+)/(?P<src>\d+\.\d+\.\d+\.\d+)\((?P<sport>\d+)\)\s*->\s*(?P<dzone>[^/]+)/(?P<dst>\d+\.\d+\.\d+\.\d+)\((?P<dport>\d+)\)`),
	},
	// 106001 — Inbound|Outbound TCP connection denied from IP/port to IP/port
	{
		id: "106001",
		re: regexp.MustCompile(
			`(?i)%(?:ASA|FTD)-\d+-106001:\s+(?:Inbound|Outbound)\s+(?P<proto>TCP)\s+connection\s+denied\s+from\s+(?P<src>\d+\.\d+\.\d+\.\d+)/(?P<sport>\d+)\s+to\s+(?P<dst>\d+\.\d+\.\d+\.\d+)/(?P<dport>\d+)`),
		action: "deny",
	},
	// 106006/07 — Deny inbound UDP from IP/port to IP/port
	{
		id: "106006-106007",
		re: regexp.MustCompile(
			`(?i)%(?:ASA|FTD)-\d+-10600[67]:\s+Deny\s+inbound\s+(?P<proto>UDP).*?from\s+(?P<src>\d+\.\d+\.\d+\.\d+)/(?P<sport>\d+)\s+to\s+(?P<dst>\d+\.\d+\.\d+\.\d+)/(?P<dport>\d+)`),
		action: "deny",
	},
	// 106014 — Deny inbound ICMP from IP to IP
	{
		id: "106014",
		re: regexp.MustCompile(
			`(?i)%(?:ASA|FTD)-\d+-106014:\s+Deny\s+inbound\s+ICMP.*?from\s+(?P<src>\d+\.\d+\.\d+\.\d+)\s+to\s+(?P<dst>\d+\.\d+\.\d+\.\d+)`),
		action: "deny",
		proto:  "ICMP",
	},
	// 106015 — Deny TCP|UDP (no connection) from IP/port to IP/port
	{
		id: "106015",
		re: regexp.MustCompile(
			`(?i)%(?:ASA|FTD)-\d+-106015:\s+Deny\s+(?P<proto>TCP|UDP).*?from\s+(?P<src>\d+\.\d+\.\d+\.\d+)/(?P<sport>\d+)\s+to\s+(?P<dst>\d+\.\d+\.\d+\.\d+)/(?P<dport>\d+)`),
		action: "deny",
	},
	// 106021 — Deny PROTO reverse path check from IP to IP
	{
		id: "106021",
		re: regexp.MustCompile(
			`(?i)%(?:ASA|FTD)-\d+-106021:\s+Deny\s+(?P<proto>\S+)\s+reverse\s+path\s+check\s+from\s+(?P<src>\d+\.\d+\.\d+\.\d+)\s+to\s+(?P<dst>\d+\.\d+\.\d+\.\d+)`),
		action: "deny",
	},
	// 710003 — PROTO access denied by ACL from IP/port to ZONE:IP/port
	{
		id: "710003",
		re: regexp.MustCompile(
			`(?i)%(?:ASA|FTD)-\d+-710003:\s+(?P<proto>\S+)\s+access\s+denied\s+by\s+ACL\s+from\s+(?P<src>\d+\.\d+\.\d+\.\d+)/(?P<sport>\d+)\s+to\s+(?P<dzone>[^:]+):(?P<dst>\d+\.\d+\.\d+\.\d+)/(?P<dport>\d+)`),
		action: "deny",
	},
}

var (
	asaBytesRE = regexp.MustCompile(`bytes\s+(\d+)`)
)

// parseLina прогоняет строку по таблице Lina-паттернов (первое совпадение).
func parseLina(line, vendor string) (model.TrafficLog, bool) {
	for i := range linaPatterns {
		pat := &linaPatterns[i]
		m := pat.re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		return buildLina(pat, m, newCiscoBase(line, vendor)), true
	}
	return model.TrafficLog{}, false
}

func newCiscoBase(line, vendor string) model.TrafficLog {
	marker := "%ASA"
	if strings.Contains(line, "%FTD") {
		marker = "%FTD"
	}
	prefix := line
	if idx := strings.Index(line, marker); idx > 0 {
		prefix = line[:idx]
	}
	ts := time.Now()
	if t, ok := parseSyslogTime(prefix); ok {
		ts = t
	}
	return model.TrafficLog{
		Timestamp: ts,
		Vendor:    vendor,
		Device:    extractCiscoDevice(line, marker),
		Raw:       line,
	}
}

func buildLina(pat *linaPattern, m []string, base model.TrafficLog) model.TrafficLog {
	tl := base
	var actWord, dir string

	for i, name := range pat.re.SubexpNames() {
		if i == 0 || name == "" || m[i] == "" {
			continue
		}
		switch name {
		case "src":
			tl.SrcIP = m[i]
		case "dst":
			tl.DstIP = m[i]
		case "sport":
			tl.SrcPort = parseUint32(m[i])
		case "dport":
			tl.DstPort = parseUint32(m[i])
		case "szone":
			tl.SrcZone = strings.TrimSpace(m[i])
		case "dzone":
			tl.DstZone = strings.TrimSpace(m[i])
		case "proto":
			tl.Proto = mapProtoNumber(m[i]) // умеет и число, и имя
		case "rule":
			tl.Rule = strings.Trim(m[i], `"`)
		case "act":
			actWord = m[i]
		case "dir":
			dir = strings.ToLower(m[i])
		}
	}

	// outbound: инициатор на стороне с большим security level (после "to").
	if dir == "outbound" {
		tl.SrcIP, tl.DstIP = tl.DstIP, tl.SrcIP
		tl.SrcPort, tl.DstPort = tl.DstPort, tl.SrcPort
		tl.SrcZone, tl.DstZone = tl.DstZone, tl.SrcZone
	}

	if tl.Proto == "" && pat.proto != "" {
		tl.Proto = pat.proto
	}

	switch {
	case pat.action != "":
		tl.Action = pat.action
	case actWord != "":
		a := strings.ToLower(actWord)
		if strings.HasPrefix(a, "permit") || a == "allow" || a == "built" {
			tl.Action = "allow"
		} else {
			tl.Action = "deny"
		}
	}

	// Teardown-события несут объём: "... bytes 12345"
	if strings.Contains(tl.Raw, "Teardown") {
		if mb := asaBytesRE.FindStringSubmatch(tl.Raw); mb != nil {
			tl.BytesSent = parseUint64(mb[1])
		}
	}

	return tl
}

func extractCiscoDevice(line, marker string) string {
	idx := strings.Index(line, marker)
	if idx <= 0 {
		return ""
	}
	before := strings.TrimSpace(line[:idx])
	fields := strings.Fields(before)
	for i := len(fields) - 1; i >= 0; i-- {
		f := fields[i]
		if f == ":" || f == "" || strings.HasPrefix(f, "<") || strings.Contains(f, ":") {
			continue
		}
		return f
	}
	return ""
}

// ---- Разбор времени syslog-префикса (используется парсерами Cisco) ----

var (
	isoTimeRE = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?`)
	bsdTimeRE = regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+(?:\d{4}\s+)?\d{2}:\d{2}:\d{2}`)
)

func parseSyslogTime(prefix string) (time.Time, bool) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return time.Time{}, false
	}

	if m := isoTimeRE.FindString(prefix); m != "" {
		for _, layout := range []string{
			time.RFC3339Nano, time.RFC3339,
			"2006-01-02T15:04:05", "2006-01-02 15:04:05",
		} {
			if t, err := time.Parse(layout, m); err == nil {
				return t, true
			}
		}
	}

	if m := bsdTimeRE.FindString(prefix); m != "" {
		m = strings.Join(strings.Fields(m), " ")
		for _, layout := range []string{"Jan 2 2006 15:04:05", "Jan 2 15:04:05"} {
			t, err := time.Parse(layout, m)
			if err != nil {
				continue
			}
			if !strings.Contains(layout, "2006") {
				now := time.Now()
				t = time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
				if t.After(now.Add(24 * time.Hour)) {
					t = t.AddDate(-1, 0, 0)
				}
			}
			return t, true
		}
	}

	return time.Time{}, false
}
