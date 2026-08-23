package parser

import (
	"strings"
	"time"

	"geoatlas/internal/model"
)

type GenericKV struct{}

func (p *GenericKV) Vendor() string { return "generic" }

func (p *GenericKV) CanParse(line string) bool {
	return strings.Contains(line, "src=") && strings.Contains(line, "dst=")
}

func (p *GenericKV) Parse(line string) (model.TrafficLog, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return model.TrafficLog{}, false
	}
	get := func(prefix string) string {
		for _, f := range fields {
			if strings.HasPrefix(f, prefix) {
				return strings.TrimPrefix(f, prefix)
			}
		}
		return ""
	}
	src := get("src=")
	dst := get("dst=")
	if src == "" || dst == "" {
		return model.TrafficLog{}, false
	}
	return model.TrafficLog{
		Timestamp:   time.Now(),
		Vendor:      "generic",
		SrcIP:       src,
		DstIP:       dst,
		SrcPort:     parseUint32(get("spt=")),
		DstPort:     parseUint32(get("dpt=")),
		Action:      string(NormalizeAction(get("act="))),
		Rule:        get("rule="),
		Proto:       strings.ToUpper(strings.TrimSpace(get("proto="))),
		BytesSent:   parseUint64(get("out=")),
		BytesRecv:   parseUint64(get("in=")),
		PacketsSent: parseUint64(get("cn1=")),
		PacketsRecv: parseUint64(get("cn2=")),
		Raw:         line,
	}, true
}
