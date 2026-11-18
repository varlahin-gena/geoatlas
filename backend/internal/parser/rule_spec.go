package parser

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RuleKind — тип data-driven парсера.
type RuleKind string

const (
	RuleKindCEF RuleKind = "cef"
	RuleKindKV  RuleKind = "kv"
)

// FieldMap — ключи CEF extension / KV → поля TrafficLog.
// Значение — имя ключа в сырой строке (например "src", "FTNTFGTeventtime").
type FieldMap struct {
	Src         string `json:"src,omitempty"`
	Dst         string `json:"dst,omitempty"`
	Spt         string `json:"spt,omitempty"`
	Dpt         string `json:"dpt,omitempty"`
	Act         string `json:"act,omitempty"`
	Proto       string `json:"proto,omitempty"`
	Rule        string `json:"rule,omitempty"`
	SrcZone     string `json:"src_zone,omitempty"`
	DstZone     string `json:"dst_zone,omitempty"`
	SrcCountry  string `json:"src_country,omitempty"`
	DstCountry  string `json:"dst_country,omitempty"`
	BytesSent   string `json:"bytes_sent,omitempty"`
	BytesRecv   string `json:"bytes_recv,omitempty"`
	PacketsSent string `json:"packets_sent,omitempty"`
	PacketsRecv string `json:"packets_recv,omitempty"`
	Time        string `json:"time,omitempty"`
}

// RuleJoin — склейка rule = primary + " (policy N)" при ненулевом policy.
type RuleJoin struct {
	Primary string `json:"primary,omitempty"`
	Policy  string `json:"policy,omitempty"`
}

// Transforms — пост-обработка после извлечения полей.
type Transforms struct {
	ProtoMapNumber     bool      `json:"proto_map_number,omitempty"`
	TimeUnit           string    `json:"time_unit,omitempty"` // auto|s|ms
	SkipUndefinedZones bool      `json:"skip_undefined_zones,omitempty"`
	SrcZoneFallback    []string  `json:"src_zone_fallback,omitempty"`
	DstZoneFallback    []string  `json:"dst_zone_fallback,omitempty"`
	RuleJoin           *RuleJoin `json:"rule_join,omitempty"`
}

// RuleMatch — условие CanParse.
type RuleMatch struct {
	// CEF Vendor (kind=cef), сравнение без учёта регистра.
	CEFVendor string `json:"cef_vendor,omitempty"`
	// Для kind=kv: требуемые ключи без '=' (например ["src","dst"]).
	RequiredKeys []string `json:"required_keys,omitempty"`
}

// RuleSpec — JSON-спецификация data-driven парсера.
type RuleSpec struct {
	Kind       RuleKind   `json:"kind"`
	Match      RuleMatch  `json:"match"`
	Fields     FieldMap   `json:"fields"`
	Transforms Transforms `json:"transforms"`
	Samples    []string   `json:"samples,omitempty"`
}

// RuleRecord — метаданные правила (хранилище / API).
type RuleRecord struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Vendor   string   `json:"vendor"`
	Enabled  bool     `json:"enabled"`
	Priority int      `json:"priority"`
	Seed     bool     `json:"seed,omitempty"`
	Spec     RuleSpec `json:"spec"`
}

// Validate проверяет минимальную корректность спецификации.
func (s RuleSpec) Validate() error {
	switch s.Kind {
	case RuleKindCEF:
		if strings.TrimSpace(s.Match.CEFVendor) == "" {
			return fmt.Errorf("cef: match.cef_vendor is required")
		}
	case RuleKindKV:
		if len(s.Match.RequiredKeys) == 0 {
			return fmt.Errorf("kv: match.required_keys is required")
		}
	default:
		return fmt.Errorf("unknown kind %q (want cef|kv)", s.Kind)
	}
	if strings.TrimSpace(s.Fields.Src) == "" || strings.TrimSpace(s.Fields.Dst) == "" {
		return fmt.Errorf("fields.src and fields.dst are required")
	}
	tu := strings.ToLower(strings.TrimSpace(s.Transforms.TimeUnit))
	if tu != "" && tu != "auto" && tu != "s" && tu != "ms" {
		return fmt.Errorf("transforms.time_unit must be auto|s|ms")
	}
	return nil
}

// ValidateRecord проверяет id/vendor/spec.
func (r RuleRecord) Validate() error {
	id := strings.TrimSpace(r.ID)
	if id == "" {
		return fmt.Errorf("id is required")
	}
	if strings.ContainsAny(id, " \t\r\n/") {
		return fmt.Errorf("id must not contain whitespace or '/'")
	}
	if strings.TrimSpace(r.Vendor) == "" {
		return fmt.Errorf("vendor is required")
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return r.Spec.Validate()
}

// ParseRuleSpec декодирует JSON spec.
func ParseRuleSpec(raw []byte) (RuleSpec, error) {
	var s RuleSpec
	if err := json.Unmarshal(raw, &s); err != nil {
		return RuleSpec{}, err
	}
	if err := s.Validate(); err != nil {
		return RuleSpec{}, err
	}
	return s, nil
}

// MarshalSpec сериализует spec в JSON.
func MarshalSpec(s RuleSpec) ([]byte, error) {
	return json.Marshal(s)
}
