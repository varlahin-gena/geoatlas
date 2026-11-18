package parser

import (
	"strconv"
	"strings"
	"time"

	"geoatlas/internal/model"
)

// RuleParser — data-driven Parser по RuleRecord.
type RuleParser struct {
	rec RuleRecord
}

func NewRuleParser(rec RuleRecord) (*RuleParser, error) {
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	rec.ID = strings.TrimSpace(rec.ID)
	rec.Vendor = strings.TrimSpace(rec.Vendor)
	rec.Name = strings.TrimSpace(rec.Name)
	return &RuleParser{rec: rec}, nil
}

func (p *RuleParser) Vendor() string {
	if p == nil {
		return ""
	}
	return p.rec.Vendor
}

func (p *RuleParser) Record() RuleRecord {
	if p == nil {
		return RuleRecord{}
	}
	return p.rec
}

func (p *RuleParser) CanParse(line string) bool {
	if p == nil {
		return false
	}
	switch p.rec.Spec.Kind {
	case RuleKindCEF:
		v, ok := cefVendor(line)
		return ok && strings.EqualFold(v, p.rec.Spec.Match.CEFVendor)
	case RuleKindKV:
		for _, k := range p.rec.Spec.Match.RequiredKeys {
			key := strings.TrimSpace(k)
			if key == "" {
				continue
			}
			if !strings.Contains(line, key+"=") {
				return false
			}
		}
		return len(p.rec.Spec.Match.RequiredKeys) > 0
	default:
		return false
	}
}

func (p *RuleParser) Parse(line string) (model.TrafficLog, bool) {
	if p == nil {
		return model.TrafficLog{}, false
	}
	switch p.rec.Spec.Kind {
	case RuleKindCEF:
		return p.parseCEF(line)
	case RuleKindKV:
		return p.parseKV(line)
	default:
		return model.TrafficLog{}, false
	}
}

func (p *RuleParser) parseCEF(line string) (model.TrafficLog, bool) {
	_, ext, prefix, ok := parseCEF(line)
	if !ok {
		return model.TrafficLog{}, false
	}
	kv := map[string]string{}
	walkCEFExt(ext, func(k, v string) { kv[k] = v })
	return p.buildFromKV(line, kv, prefix)
}

func (p *RuleParser) parseKV(line string) (model.TrafficLog, bool) {
	kv := map[string]string{}
	for _, f := range strings.Fields(line) {
		i := strings.IndexByte(f, '=')
		if i <= 0 {
			continue
		}
		kv[f[:i]] = f[i+1:]
	}
	return p.buildFromKV(line, kv, "")
}

func (p *RuleParser) buildFromKV(line string, kv map[string]string, cefPrefix string) (model.TrafficLog, bool) {
	f := p.rec.Spec.Fields
	src := lookup(kv, f.Src)
	dst := lookup(kv, f.Dst)
	if src == "" || dst == "" {
		return model.TrafficLog{}, false
	}

	tr := p.rec.Spec.Transforms
	srcZone := pickZone(lookup(kv, f.SrcZone), fallbackVals(kv, tr.SrcZoneFallback), tr.SkipUndefinedZones)
	dstZone := pickZone(lookup(kv, f.DstZone), fallbackVals(kv, tr.DstZoneFallback), tr.SkipUndefinedZones)

	rule := lookup(kv, f.Rule)
	if tr.RuleJoin != nil {
		primary := lookup(kv, tr.RuleJoin.Primary)
		policy := lookup(kv, tr.RuleJoin.Policy)
		rule = joinRule(primary, policy)
	}

	proto := lookup(kv, f.Proto)
	if tr.ProtoMapNumber {
		proto = mapProtoNumber(proto)
	} else if proto != "" {
		proto = strings.ToUpper(proto)
	}

	device := ""
	if p.rec.Spec.Kind == RuleKindCEF {
		device = extractDeviceFromCEF(cefPrefix,
			lookup(kv, "deviceExternalId"),
			lookup(kv, "dvchost"),
			lookup(kv, "dvc"),
		)
	}

	return model.TrafficLog{
		Timestamp:   parseRuleTime(lookup(kv, f.Time), tr.TimeUnit),
		Vendor:      p.rec.Vendor,
		Device:      device,
		SrcIP:       src,
		DstIP:       dst,
		SrcPort:     parseUint32(lookup(kv, f.Spt)),
		DstPort:     parseUint32(lookup(kv, f.Dpt)),
		Action:      string(NormalizeAction(lookup(kv, f.Act))),
		Rule:        rule,
		Proto:       proto,
		SrcZone:     srcZone,
		DstZone:     dstZone,
		SrcCountry:  lookup(kv, f.SrcCountry),
		DstCountry:  lookup(kv, f.DstCountry),
		BytesSent:   parseUint64(lookup(kv, f.BytesSent)),
		BytesRecv:   parseUint64(lookup(kv, f.BytesRecv)),
		PacketsSent: parseUint64(lookup(kv, f.PacketsSent)),
		PacketsRecv: parseUint64(lookup(kv, f.PacketsRecv)),
		Raw:         line,
	}, true
}

func lookup(kv map[string]string, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	return kv[key]
}

func fallbackVals(kv map[string]string, keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if v := lookup(kv, k); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func pickZone(primary string, fallbacks []string, skipUndef bool) string {
	cands := append([]string{primary}, fallbacks...)
	for _, v := range cands {
		if skipUndef {
			v = skipUndefined(v)
		}
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func joinRule(primary, policy string) string {
	primary = strings.TrimSpace(primary)
	policy = strings.TrimSpace(policy)
	if policy != "" && policy != "0" {
		if primary == "" {
			return "policy " + policy
		}
		return primary + " (policy " + policy + ")"
	}
	return primary
}

func parseRuleTime(raw, unit string) time.Time {
	now := time.Now()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return now
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return now
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "s":
		return time.Unix(v, 0)
	case "ms":
		return time.UnixMilli(v)
	default: // auto
		if len(raw) <= 10 {
			return time.Unix(v, 0)
		}
		return time.UnixMilli(v)
	}
}
