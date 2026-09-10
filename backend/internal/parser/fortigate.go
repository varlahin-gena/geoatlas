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
	v, ok := cefVendor(line)
	return ok && strings.EqualFold(v, "Fortinet")
}

type fortigateCEFExt struct {
	src, dst, spt, dpt, act, proto string
	eventtime, app, policyid       string
	srcintfrole, dstintfrole       string
	inIf, outIf, srcintf, dstintf  string
	srccountry, dstcountry         string
	out, in, sentpkt, rcvdpkt      string
	deviceExternalId, dvchost, dvc string
}

func (e *fortigateCEFExt) set(k, v string) {
	switch k {
	case "src":
		e.src = v
	case "dst":
		e.dst = v
	case "spt":
		e.spt = v
	case "dpt":
		e.dpt = v
	case "act":
		e.act = v
	case "proto":
		e.proto = v
	case "FTNTFGTeventtime":
		e.eventtime = v
	case "app":
		e.app = v
	case "FTNTFGTpolicyid":
		e.policyid = v
	case "FTNTFGTsrcintfrole":
		e.srcintfrole = v
	case "FTNTFGTdstintfrole":
		e.dstintfrole = v
	case "deviceInboundInterface":
		e.inIf = v
	case "deviceOutboundInterface":
		e.outIf = v
	case "FTNTFGTsrcintf":
		e.srcintf = v
	case "FTNTFGTdstintf":
		e.dstintf = v
	case "FTNTFGTsrccountry":
		e.srccountry = v
	case "FTNTFGTdstcountry":
		e.dstcountry = v
	case "out":
		e.out = v
	case "in":
		e.in = v
	case "FTNTFGTsentpkt":
		e.sentpkt = v
	case "FTNTFGTrcvdpkt":
		e.rcvdpkt = v
	case "deviceExternalId":
		e.deviceExternalId = v
	case "dvchost":
		e.dvchost = v
	case "dvc":
		e.dvc = v
	}
}

func (p *FortigateCEF) Parse(line string) (model.TrafficLog, bool) {
	_, ext, prefix, ok := parseCEF(line)
	if !ok {
		return model.TrafficLog{}, false
	}

	var f fortigateCEFExt
	walkCEFExt(ext, f.set)

	if f.src == "" || f.dst == "" {
		return model.TrafficLog{}, false
	}

	ts := time.Now()
	if f.eventtime != "" {
		if v, err := strconv.ParseInt(f.eventtime, 10, 64); err == nil {
			if len(f.eventtime) <= 10 {
				ts = time.Unix(v, 0)
			} else {
				ts = time.UnixMilli(v)
			}
		}
	}

	rule := f.app
	if f.policyid != "" && f.policyid != "0" {
		if rule == "" {
			rule = "policy " + f.policyid
		} else {
			rule = rule + " (policy " + f.policyid + ")"
		}
	}

	// В реальных CEF FortiGate FTNTFGTsrcintfrole часто "undefined";
	// тогда берём deviceInboundInterface / deviceOutboundInterface (docs.fortinet.com).
	srcZone := skipUndefined(f.srcintfrole)
	if srcZone == "" {
		srcZone = f.inIf
	}
	if srcZone == "" {
		srcZone = f.srcintf
	}
	dstZone := skipUndefined(f.dstintfrole)
	if dstZone == "" {
		dstZone = f.outIf
	}
	if dstZone == "" {
		dstZone = f.dstintf
	}

	return model.TrafficLog{
		Timestamp:   ts,
		Vendor:      "fortigate",
		Device:      extractDeviceFromCEF(prefix, f.deviceExternalId, f.dvchost, f.dvc),
		SrcIP:       f.src,
		DstIP:       f.dst,
		SrcPort:     parseUint32(f.spt),
		DstPort:     parseUint32(f.dpt),
		Action:      string(NormalizeAction(f.act)),
		Rule:        rule,
		Proto:       mapProtoNumber(f.proto),
		SrcZone:     srcZone,
		DstZone:     dstZone,
		SrcCountry:  f.srccountry,
		DstCountry:  f.dstcountry,
		BytesSent:   parseUint64(f.out),
		BytesRecv:   parseUint64(f.in),
		PacketsSent: parseUint64(f.sentpkt),
		PacketsRecv: parseUint64(f.rcvdpkt),
		Raw:         line,
	}, true
}

func skipUndefined(s string) string {
	if s == "" || strings.EqualFold(s, "undefined") {
		return ""
	}
	return s
}
