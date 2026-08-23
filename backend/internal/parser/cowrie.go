package parser

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"geoatlas/internal/model"
)

// CowrieJSON — парсер событий honeypot Cowrie.
//
// Поддерживает два формата доставки:
//   1) jsonlog → файл cowrie.json: строгий JSON (двойные кавычки), одно событие в строке;
//   2) remotesyslog → syslog-ng: Python-dict repr (одинарные кавычки) с syslog-префиксом,
//      напр.: "Jun 30 18:50:43 172.18.0.1 {'eventid': 'cowrie.session.connect', ...}".
//
// В рёбра графа превращаются только:
//   - cowrie.session.connect (атакующий → honeypot),
//   - cowrie.login.* при наличии src_ip+dst_ip (типично remotesyslog).
// События вроде cowrie.direct-tcpip.* тоже содержат src/dst, но dst там —
// цель проксирования (SMTP/DNS и т.п.), не honeypot; их пропускаем.
// Остальные cowrie-события — ShouldSkip, не ошибка парсинга.
type CowrieJSON struct{}

func (p *CowrieJSON) Vendor() string { return "cowrie" }

func (p *CowrieJSON) CanParse(line string) bool {
	// Лояльно к обоим форматам кавычек: важно лишь наличие eventid=cowrie.*
	return strings.Contains(line, "eventid") && strings.Contains(line, "cowrie.")
}

func (p *CowrieJSON) Parse(line string) (model.TrafficLog, bool) {
	ev, recognized := parseCowrie(line)
	if !recognized || !cowrieToGraph(ev.EventID) {
		return model.TrafficLog{}, false
	}
	src := strings.TrimSpace(ev.SrcIP)
	dst := strings.TrimSpace(ev.DstIP)
	if src == "" || dst == "" {
		// распознано, но без сетевой пары → не TrafficLog (решит ShouldSkip)
		return model.TrafficLog{}, false
	}

	ts := time.Now()
	if ev.Timestamp != "" {
		if v, err := time.Parse(time.RFC3339Nano, ev.Timestamp); err == nil {
			ts = v
		}
	}

	return model.TrafficLog{
		Timestamp: ts,
		Vendor:    "cowrie",
		Device:    strings.TrimSpace(ev.Sensor),
		SrcIP:     src,
		DstIP:     dst,
		SrcPort:   ev.SrcPort,
		DstPort:   ev.DstPort,
		Action:    string(NormalizeAction(cowrieAction(ev.EventID))),
		Rule:      strings.ToLower(strings.TrimSpace(ev.Protocol)), // ssh | telnet
		Proto:     "TCP",
		Raw:       line,
	}, true
}

// ShouldSkip реализует опциональный интерфейс SkipParser: распознанное cowrie-событие,
// которое не нужно в графе (нет пары, или не session.connect/login.*) — не ошибка.
func (p *CowrieJSON) ShouldSkip(line string) bool {
	ev, recognized := parseCowrie(line)
	if !recognized {
		return false
	}
	if !cowrieToGraph(ev.EventID) {
		return true
	}
	return strings.TrimSpace(ev.SrcIP) == "" || strings.TrimSpace(ev.DstIP) == ""
}

// cowrieToGraph — события, из которых имеет смысл строить ребро attacker→honeypot.
func cowrieToGraph(eventid string) bool {
	return strings.Contains(eventid, "session.connect") ||
		strings.Contains(eventid, "login.")
}

func cowrieAction(eventid string) string {
	switch {
	case strings.Contains(eventid, "session.closed"),
		strings.Contains(eventid, "connection.lost"):
		return "close"
	case strings.Contains(eventid, "login.failed"):
		return "reject"
	default: // cowrie.session.connect и пр.
		return "accept"
	}
}

// ---- извлечение полей (JSON + фоллбэк на Python-repr) ----

type cowrieEvent struct {
	EventID   string `json:"eventid"`
	SrcIP     string `json:"src_ip"`
	DstIP     string `json:"dst_ip"`
	SrcPort   uint32 `json:"src_port"`
	DstPort   uint32 `json:"dst_port"`
	Protocol  string `json:"protocol"`
	Sensor    string `json:"sensor"`
	Timestamp string `json:"timestamp"`
}

// parseCowrie возвращает извлечённое событие и признак того, что это валидное cowrie-событие.
func parseCowrie(line string) (cowrieEvent, bool) {
	var ev cowrieEvent

	start := strings.IndexByte(line, '{')
	if start < 0 {
		return ev, false
	}
	payload := line[start:] // срезаем возможный syslog-префикс

	// 1) Строгий JSON (jsonlog). Корректно игнорирует вложенные массивы/объекты.
	if err := json.Unmarshal([]byte(payload), &ev); err == nil {
		return ev, strings.HasPrefix(ev.EventID, "cowrie.")
	}

	// 2) Python-repr (remotesyslog): точечные регулярки по нужным ключам.
	ev = cowrieEvent{
		EventID:   cowField(reCowEventID, payload),
		SrcIP:     cowField(reCowSrcIP, payload),
		DstIP:     cowField(reCowDstIP, payload),
		SrcPort:   parseUint32(cowField(reCowSrcPort, payload)),
		DstPort:   parseUint32(cowField(reCowDstPort, payload)),
		Protocol:  cowField(reCowProto, payload),
		Sensor:    cowField(reCowSensor, payload),
		Timestamp: cowField(reCowTime, payload),
	}
	return ev, strings.HasPrefix(ev.EventID, "cowrie.")
}

func cowKeyStr(key string) *regexp.Regexp {
	return regexp.MustCompile(`['"]` + regexp.QuoteMeta(key) + `['"]\s*:\s*['"]([^'"]*)['"]`)
}

func cowKeyInt(key string) *regexp.Regexp {
	return regexp.MustCompile(`['"]` + regexp.QuoteMeta(key) + `['"]\s*:\s*(\d+)`)
}

var (
	reCowEventID = cowKeyStr("eventid")
	reCowSrcIP   = cowKeyStr("src_ip")
	reCowDstIP   = cowKeyStr("dst_ip")
	reCowProto   = cowKeyStr("protocol")
	reCowSensor  = cowKeyStr("sensor")
	reCowTime    = cowKeyStr("timestamp")
	reCowSrcPort = cowKeyInt("src_port")
	reCowDstPort = cowKeyInt("dst_port")
)

func cowField(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
