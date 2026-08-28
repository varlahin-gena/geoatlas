package sqlclause

import (
	"strings"

	"geoatlas/internal/mapsearch"
)

// MapSearchColumns — выражения ClickHouse (не user input) для bind-предикатов mapsearch.
type MapSearchColumns struct {
	AllConcat   string
	IP          []string
	Port        []string
	Country     []string
	City        []string
	Action      []string
	Device      []string
	SrcIP       []string
	DstIP       []string
	SrcPort     []string
	DstPort     []string
	SrcCountry  []string
	DstCountry  []string
	SrcCity     []string
	DstCity     []string
	Src         []string
	Dst         []string
	Proto       []string
	Zone        []string
}

func (c MapSearchColumns) cols(f mapsearch.Field) []string {
	switch f {
	case mapsearch.FieldIP:
		return c.IP
	case mapsearch.FieldPort:
		return c.Port
	case mapsearch.FieldCountry:
		return c.Country
	case mapsearch.FieldCity:
		return c.City
	case mapsearch.FieldAction:
		return c.Action
	case mapsearch.FieldDevice:
		return c.Device
	case mapsearch.FieldSrcIP:
		return c.SrcIP
	case mapsearch.FieldDstIP:
		return c.DstIP
	case mapsearch.FieldSrcPort:
		return c.SrcPort
	case mapsearch.FieldDstPort:
		return c.DstPort
	case mapsearch.FieldSrcCountry:
		return c.SrcCountry
	case mapsearch.FieldDstCountry:
		return c.DstCountry
	case mapsearch.FieldSrcCity:
		return c.SrcCity
	case mapsearch.FieldDstCity:
		return c.DstCity
	case mapsearch.FieldSrc:
		return c.Src
	case mapsearch.FieldDst:
		return c.Dst
	case mapsearch.FieldProto:
		return c.Proto
	case mapsearch.FieldZone:
		return c.Zone
	default:
		return nil
	}
}

var LogsMapSearchColumns = MapSearchColumns{
	AllConcat: "concat_ws(' ', toString(src_ip), toString(dst_ip), toString(src_port), toString(dst_port), src_country, dst_country, src_city, dst_city, action, device, proto, src_zone, dst_zone)",
	IP:        []string{"toString(src_ip)", "toString(dst_ip)"},
	Port:      []string{"toString(src_port)", "toString(dst_port)"},
	Country:   []string{"src_country", "dst_country"},
	City:      []string{"src_city", "dst_city"},
	Action:    []string{"action"},
	Device:    []string{"device"},
	SrcIP:     []string{"toString(src_ip)"},
	DstIP:     []string{"toString(dst_ip)"},
	SrcPort:   []string{"toString(src_port)"},
	DstPort:   []string{"toString(dst_port)"},
	SrcCountry: []string{"src_country"},
	DstCountry: []string{"dst_country"},
	SrcCity:   []string{"src_city"},
	DstCity:   []string{"dst_city"},
	Src:       []string{"toString(src_ip)", "src_country"},
	Dst:       []string{"toString(dst_ip)", "dst_country"},
	Proto:     []string{"proto"},
	Zone:      []string{"src_zone", "dst_zone"},
}

var GeoMapSearchColumns = MapSearchColumns{
	AllConcat: "concat_ws(' ', src_key, dst_key, src_label, dst_label, toString(src_port), toString(dst_port), src_country, dst_country, src_city, dst_city, last_action, device, proto)",
	IP:        []string{"src_key", "dst_key", "src_label", "dst_label"},
	Port:      []string{"toString(src_port)", "toString(dst_port)"},
	Country:   []string{"src_country", "dst_country", "src_key", "dst_key"},
	City:      []string{"src_city", "dst_city", "src_label", "dst_label"},
	Action:    []string{"last_action"},
	Device:    []string{"device"},
	SrcIP:     []string{"src_key", "src_label"},
	DstIP:     []string{"dst_key", "dst_label"},
	SrcPort:   []string{"toString(src_port)"},
	DstPort:   []string{"toString(dst_port)"},
	SrcCountry: []string{"src_country"},
	DstCountry: []string{"dst_country"},
	SrcCity:   []string{"src_city", "src_label"},
	DstCity:   []string{"dst_city", "dst_label"},
	Src:       []string{"src_key", "src_label", "src_country"},
	Dst:       []string{"dst_key", "dst_label", "dst_country"},
	Proto:     []string{"proto"},
	Zone:      []string{"src_key", "dst_key"},
}

var IPAggMapSearchColumns = MapSearchColumns{
	AllConcat: "concat_ws(' ', toString(src_ip), toString(dst_ip), toString(src_port), toString(dst_port), src_country, dst_country, src_city, dst_city, last_action, device, proto)",
	IP:        []string{"toString(src_ip)", "toString(dst_ip)"},
	Port:      []string{"toString(src_port)", "toString(dst_port)"},
	Country:   []string{"src_country", "dst_country"},
	City:      []string{"src_city", "dst_city"},
	Action:    []string{"last_action"},
	Device:    []string{"device"},
	SrcIP:     []string{"toString(src_ip)"},
	DstIP:     []string{"toString(dst_ip)"},
	SrcPort:   []string{"toString(src_port)"},
	DstPort:   []string{"toString(dst_port)"},
	SrcCountry: []string{"src_country"},
	DstCountry: []string{"dst_country"},
	SrcCity:   []string{"src_city"},
	DstCity:   []string{"dst_city"},
	Src:       []string{"toString(src_ip)", "src_country"},
	Dst:       []string{"toString(dst_ip)", "dst_country"},
	Proto:     []string{"proto"},
	Zone:      []string{"toString(src_ip)", "toString(dst_ip)"},
}

var countryNeedles = map[string][]string{
	"россия":             {"Россия", "Russia", "Russian Federation", "RU"},
	"russia":             {"Россия", "Russia", "Russian Federation", "RU"},
	"russian federation": {"Россия", "Russia", "Russian Federation", "RU"},
	"ru":                 {"Россия", "Russia", "RU"},
	"сша":                {"США", "United States", "USA", "US"},
	"united states":      {"США", "United States", "USA", "US"},
	"usa":                {"США", "United States", "USA", "US"},
	"us":                 {"США", "United States", "USA", "US"},
	"казахстан":          {"Казахстан", "Kazakhstan", "KZ"},
	"kazakhstan":         {"Казахстан", "Kazakhstan", "KZ"},
	"китай":              {"Китай", "China", "CN"},
	"china":              {"Китай", "China", "CN"},
	"германия":           {"Германия", "Germany", "DE"},
	"germany":            {"Германия", "Germany", "DE"},
	"de":                 {"Германия", "Germany", "DE"},
}

// MapSearchSQL returns a parenthesized predicate and bind args, or empty if nothing to filter.
func MapSearchSQL(c mapsearch.Compiled, cols MapSearchColumns) (clause string, args []any) {
	if c.Empty {
		return "", nil
	}
	if c.Simple != "" {
		return "positionCaseInsensitiveUTF8(" + cols.AllConcat + ", ?) > 0", []any{c.Simple}
	}
	if c.Root == nil {
		return "", nil
	}
	return nodeSQL(c.Root, cols)
}

func nodeSQL(n *mapsearch.Node, cols MapSearchColumns) (string, []any) {
	if n == nil {
		return "", nil
	}
	switch n.Kind {
	case mapsearch.KindNot:
		inner, args := nodeSQL(n.Expr, cols)
		if inner == "" {
			return "", nil
		}
		return "NOT (" + inner + ")", args
	case mapsearch.KindAnd:
		l, la := nodeSQL(n.Left, cols)
		r, ra := nodeSQL(n.Right, cols)
		return joinBool("AND", l, r, la, ra)
	case mapsearch.KindOr:
		l, la := nodeSQL(n.Left, cols)
		r, ra := nodeSQL(n.Right, cols)
		return joinBool("OR", l, r, la, ra)
	default:
		return termSQL(n, cols)
	}
}

func joinBool(op, l, r string, la, ra []any) (string, []any) {
	switch {
	case l == "" && r == "":
		return "", nil
	case l == "":
		return r, ra
	case r == "":
		return l, la
	default:
		return "(" + l + ") " + op + " (" + r + ")", append(append([]any{}, la...), ra...)
	}
}

func termSQL(n *mapsearch.Node, cols MapSearchColumns) (string, []any) {
	needles := []string{n.Value}
	if n.Field == mapsearch.FieldCountry || n.Field == mapsearch.FieldSrcCountry || n.Field == mapsearch.FieldDstCountry || n.Field == mapsearch.FieldAll {
		if extra := countryNeedles[strings.ToLower(strings.TrimSpace(n.Value))]; len(extra) > 0 {
			needles = extra
		}
	}
	if n.Field == mapsearch.FieldAll || n.Field == "" {
		parts := make([]string, 0, len(needles))
		args := make([]any, 0, len(needles))
		for _, needle := range needles {
			if strings.TrimSpace(needle) == "" {
				continue
			}
			part, arg := colMatchSQL(cols.AllConcat, needle, n.Op)
			parts = append(parts, part)
			args = append(args, arg)
		}
		return joinByOp(n.Op, parts), args
	}
	fieldCols := cols.cols(n.Field)
	if len(fieldCols) == 0 {
		return "", nil
	}
	var parts []string
	var args []any
	for _, needle := range needles {
		if strings.TrimSpace(needle) == "" {
			continue
		}
		for _, col := range fieldCols {
			part, arg := colMatchSQL(col, needle, n.Op)
			parts = append(parts, part)
			args = append(args, arg)
		}
	}
	return joinByOp(n.Op, parts), args
}

func colMatchSQL(col, needle string, op mapsearch.Op) (string, any) {
	switch op {
	case mapsearch.OpEq:
		return "lowerUTF8(" + col + ") = lowerUTF8(?)", needle
	case mapsearch.OpNe:
		return "lowerUTF8(" + col + ") != lowerUTF8(?)", needle
	default:
		return "positionCaseInsensitiveUTF8(" + col + ", ?) > 0", needle
	}
}

func joinByOp(op mapsearch.Op, parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	joiner := " OR "
	if op == mapsearch.OpNe {
		joiner = " AND "
	}
	return "(" + strings.Join(parts, joiner) + ")"
}
