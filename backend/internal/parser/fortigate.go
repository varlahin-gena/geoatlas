package parser

import (
	"strconv"
	"strings"
	"time"

	"geoatlas/internal/model"
)

type FortigateCEF struct{}

func (p *FortigateCEF) Vendor() string { return "fortigate" }

func (p *FortigateCEF) CanParse(line string) bool {
	if !strings.Contains(line, "CEF:") {
		return false
	}
	// В официальных примерах: CEF: 0|Fortinet|Fortigate|...
	lower := strings.ToLower(line)
	return strings.Contains(lower, "|fortinet|")
}

func (p *FortigateCEF) Parse(line string) (model.TrafficLog, bool) {
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
	if et := strings.TrimSpace(ext["FTNTFGTeventtime"]); et != "" {
		if v, err := strconv.ParseInt(et, 10, 64); err == nil {
			if len(et) <= 10 {
				ts = time.Unix(v, 0)
			} else {
				ts = time.UnixMilli(v)
			}
		}
	}

	rule := strings.TrimSpace(ext["app"])
	if pid := strings.TrimSpace(ext["FTNTFGTpolicyid"]); pid != "" && pid != "0" {
		if rule == "" {
			rule = "policy " + pid
		} else {
			rule = rule + " (policy " + pid + ")"
		}
	}

	// В реальных CEF FortiGate FTNTFGTsrcintfrole часто "undefined";
	// тогда берём deviceInboundInterface / deviceOutboundInterface (docs.fortinet.com).
	srcZone := firstNonEmpty(
		skipUndefined(ext["FTNTFGTsrcintfrole"]),
		ext["deviceInboundInterface"],
		ext["FTNTFGTsrcintf"],
	)
	dstZone := firstNonEmpty(
		skipUndefined(ext["FTNTFGTdstintfrole"]),
		ext["deviceOutboundInterface"],
		ext["FTNTFGTdstintf"],
	)

	return model.TrafficLog{
		Timestamp:   ts,
		Vendor:      "fortigate",
		Device:      extractDeviceFromCEF(prefix, ext),
		SrcIP:       src,
		DstIP:       dst,
		SrcPort:     parseUint32(ext["spt"]),
		DstPort:     parseUint32(ext["dpt"]),
		Action:      string(NormalizeAction(ext["act"])),
		Rule:        rule,
		Proto:       mapProtoNumber(ext["proto"]),
		SrcZone:     srcZone,
		DstZone:     dstZone,
		SrcCountry:  strings.TrimSpace(ext["FTNTFGTsrccountry"]),
		DstCountry:  strings.TrimSpace(ext["FTNTFGTdstcountry"]),
		BytesSent:   parseUint64(ext["out"]),
		BytesRecv:   parseUint64(ext["in"]),
		PacketsSent: parseUint64(ext["FTNTFGTsentpkt"]),
		PacketsRecv: parseUint64(ext["FTNTFGTrcvdpkt"]),
		Raw:         line,
	}, true
}

func skipUndefined(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "undefined") {
		return ""
	}
	return s
}
