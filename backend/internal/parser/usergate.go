package parser

import (
	"strconv"
	"strings"
	"time"

	"geoatlas/internal/model"
)

type UserGateCEF struct{}

func (p *UserGateCEF) Vendor() string { return "usergate" }

func (p *UserGateCEF) CanParse(line string) bool {
	v, ok := cefVendor(line)
	return ok && strings.EqualFold(v, "Usergate")
}

type usergateCEFExt struct {
	src, dst, spt, dpt, act, proto string
	rt, cs1, cs2, cs3, cs4, cs5    string
	in, out, cn1, cn2              string
	deviceExternalId, dvchost, dvc string
}

func (e *usergateCEFExt) set(k, v string) {
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
	case "rt":
		e.rt = v
	case "cs1":
		e.cs1 = v
	case "cs2":
		e.cs2 = v
	case "cs3":
		e.cs3 = v
	case "cs4":
		e.cs4 = v
	case "cs5":
		e.cs5 = v
	case "in":
		e.in = v
	case "out":
		e.out = v
	case "cn1":
		e.cn1 = v
	case "cn2":
		e.cn2 = v
	case "deviceExternalId":
		e.deviceExternalId = v
	case "dvchost":
		e.dvchost = v
	case "dvc":
		e.dvc = v
	}
}

func (p *UserGateCEF) Parse(line string) (model.TrafficLog, bool) {
	_, ext, prefix, ok := parseCEF(line)
	if !ok {
		return model.TrafficLog{}, false
	}

	var u usergateCEFExt
	walkCEFExt(ext, u.set)

	if u.src == "" || u.dst == "" {
		return model.TrafficLog{}, false
	}

	ts := time.Now()
	if u.rt != "" {
		if v, err := strconv.ParseInt(u.rt, 10, 64); err == nil {
			if len(u.rt) <= 10 {
				ts = time.Unix(v, 0)
			} else {
				ts = time.UnixMilli(v)
			}
		}
	}

	proto := u.proto
	if proto != "" {
		proto = strings.ToUpper(proto)
	}

	// По docs.usergate.com (журнал трафика CEF):
	//   in/cn1  — байты/пакеты источник → назначение (sent)
	//   out/cn2 — байты/пакеты назначение → источник (recv)
	// Это обратно типичному CEF ArcSight (out=sent), но так документирует UserGate.
	return model.TrafficLog{
		Timestamp:   ts,
		Vendor:      "usergate",
		Device:      extractDeviceFromCEF(prefix, u.deviceExternalId, u.dvchost, u.dvc),
		SrcIP:       u.src,
		DstIP:       u.dst,
		SrcPort:     parseUint32(u.spt),
		DstPort:     parseUint32(u.dpt),
		Action:      string(NormalizeAction(u.act)),
		Rule:        u.cs1,
		Proto:       proto,
		SrcZone:     u.cs2,
		DstZone:     u.cs4,
		SrcCountry:  u.cs3,
		DstCountry:  u.cs5,
		BytesSent:   parseUint64(u.in),
		BytesRecv:   parseUint64(u.out),
		PacketsSent: parseUint64(u.cn1),
		PacketsRecv: parseUint64(u.cn2),
		Raw:         line,
	}, true
}
