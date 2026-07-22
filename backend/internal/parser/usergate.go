package parser

import (
	"strconv"
	"strings"
	"time"

	"network_monitor/internal/model"
)

type UserGateCEF struct{}

func (p *UserGateCEF) Vendor() string { return "usergate" }

func (p *UserGateCEF) CanParse(line string) bool {
	if !strings.Contains(line, "CEF:") {
		return false
	}
	// В документации UserGate vendor пишется как "Usergate" (docs.usergate.com).
	lower := strings.ToLower(line)
	return strings.Contains(lower, "|usergate|")
}

func (p *UserGateCEF) Parse(line string) (model.TrafficLog, bool) {
	_, ext, prefix, ok := ParseCEF(line)
	if !ok {
		return model.TrafficLog{}, false
	}

	src := strings.TrimSpace(ext["src"])
	dst := strings.TrimSpace(ext["dst"])
	if src == "" || dst == "" {
		return model.TrafficLog{}, false
	}

	ts := time.Now()
	if rt := strings.TrimSpace(ext["rt"]); rt != "" {
		if v, err := strconv.ParseInt(rt, 10, 64); err == nil {
			if len(rt) <= 10 {
				ts = time.Unix(v, 0)
			} else {
				ts = time.UnixMilli(v)
			}
		}
	}

	// По docs.usergate.com (журнал трафика CEF):
	//   in/cn1  — байты/пакеты источник → назначение (sent)
	//   out/cn2 — байты/пакеты назначение → источник (recv)
	// Это обратно типичному CEF ArcSight (out=sent), но так документирует UserGate.
	return model.TrafficLog{
		Timestamp:   ts,
		Vendor:      "usergate",
		Device:      extractDeviceFromCEF(prefix, ext),
		SrcIP:       src,
		DstIP:       dst,
		SrcPort:     parseUint32(ext["spt"]),
		DstPort:     parseUint32(ext["dpt"]),
		Action:      string(NormalizeAction(ext["act"])),
		Rule:        strings.TrimSpace(ext["cs1"]),
		Proto:       strings.ToUpper(strings.TrimSpace(ext["proto"])),
		SrcZone:     strings.TrimSpace(ext["cs2"]),
		DstZone:     strings.TrimSpace(ext["cs4"]),
		SrcCountry:  strings.TrimSpace(ext["cs3"]),
		DstCountry:  strings.TrimSpace(ext["cs5"]),
		BytesSent:   parseUint64(ext["in"]),
		BytesRecv:   parseUint64(ext["out"]),
		PacketsSent: parseUint64(ext["cn1"]),
		PacketsRecv: parseUint64(ext["cn2"]),
		Raw:         line,
	}, true
}
