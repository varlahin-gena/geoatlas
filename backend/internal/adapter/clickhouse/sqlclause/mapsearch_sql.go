package sqlclause

import (
	"strings"

	"geoatlas/internal/mapsearch"
)

// MapSearchColumns — выражения ClickHouse (не user input) для bind-предикатов mapsearch.
type MapSearchColumns struct {
	AllConcat string
	IP        []string
	Country   []string
	City      []string
	Rule      []string
	Device    []string
	Src       []string
	Dst       []string
	Proto     []string
	Zone      []string
}

func (c MapSearchColumns) cols(f mapsearch.Field) []string {
	switch f {
	case mapsearch.FieldIP:
		return c.IP
	case mapsearch.FieldCountry:
		return c.Country
	case mapsearch.FieldCity:
		return c.City
	case mapsearch.FieldRule:
		return c.Rule
	case mapsearch.FieldDevice:
		return c.Device
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
	AllConcat: "concat_ws(' ', toString(src_ip), toString(dst_ip), src_country, dst_country, src_city, dst_city, rule, device, proto, src_zone, dst_zone)",
	IP:        []string{"toString(src_ip)", "toString(dst_ip)"},
	Country:   []string{"src_country", "dst_country"},
	City:      []string{"src_city", "dst_city"},
	Rule:      []string{"rule"},
	Device:    []string{"device"},
	Src:       []string{"toString(src_ip)", "src_country"},
	Dst:       []string{"toString(dst_ip)", "dst_country"},
	Proto:     []string{"proto"},
	Zone:      []string{"src_zone", "dst_zone"},
}

var GeoMapSearchColumns = MapSearchColumns{
	AllConcat: "concat_ws(' ', src_key, dst_key, src_label, dst_label, src_country, dst_country, src_city, dst_city, rule, device, proto)",
	IP:        []string{"src_key", "dst_key", "src_label", "dst_label"},
	Country:   []string{"src_country", "dst_country", "src_key", "dst_key"},
	City:      []string{"src_city", "dst_city", "src_label", "dst_label"},
	Rule:      []string{"rule"},
	Device:    []string{"device"},
	Src:       []string{"src_key", "src_label", "src_country"},
	Dst:       []string{"dst_key", "dst_label", "dst_country"},
	Proto:     []string{"proto"},
	Zone:      []string{"src_key", "dst_key"},
}

var IPAggMapSearchColumns = MapSearchColumns{
	AllConcat: "concat_ws(' ', toString(src_ip), toString(dst_ip), src_country, dst_country, src_city, dst_city, rule, device, proto)",
	IP:        []string{"toString(src_ip)", "toString(dst_ip)"},
	Country:   []string{"src_country", "dst_country"},
	City:      []string{"src_city", "dst_city"},
	Rule:      []string{"rule"},
	Device:    []string{"device"},
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
	if n.Field == mapsearch.FieldCountry || n.Field == mapsearch.FieldAll {
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
			parts = append(parts, "positionCaseInsensitiveUTF8("+cols.AllConcat+", ?) > 0")
			args = append(args, needle)
		}
		return orJoin(parts), args
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
			parts = append(parts, "positionCaseInsensitiveUTF8("+col+", ?) > 0")
			args = append(args, needle)
		}
	}
	return orJoin(parts), args
}

func orJoin(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}
