package sqlclause

import (
	"net"
	"strconv"
	"strings"
	"unicode"

	"geoatlas/internal/mapsearch"
)

// MapSearchColumns — выражения ClickHouse (не user input) для bind-предикатов mapsearch.
type MapSearchColumns struct {
	// Simple — колонки для OR positionCaseInsensitive (без concat_ws).
	Simple []string

	IP         []string
	Port       []string
	Country    []string
	City       []string
	Action     []string
	Device     []string
	SrcIP      []string
	DstIP      []string
	SrcPort    []string
	DstPort    []string
	SrcCountry []string
	DstCountry []string
	SrcCity    []string
	DstCity    []string
	Src        []string
	Dst        []string
	Proto      []string
	Zone       []string

	// IPv4Src/IPv4Dst — нативные IPv4-колонки (bloom). Пусто → только строковые IP[].
	IPv4Src string
	IPv4Dst string
	// PortSrc/PortDst — нативные UInt port-колонки. Пусто → toString position/eq.
	PortSrc string
	PortDst string
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
	Simple: []string{
		"toString(src_ip)", "toString(dst_ip)",
		"toString(src_port)", "toString(dst_port)",
		"src_country", "dst_country", "src_city", "dst_city",
		"action", "device", "proto", "src_zone", "dst_zone",
	},
	IP:         []string{"toString(src_ip)", "toString(dst_ip)"},
	Port:       []string{"toString(src_port)", "toString(dst_port)"},
	Country:    []string{"src_country", "dst_country"},
	City:       []string{"src_city", "dst_city"},
	Action:     []string{"action"},
	Device:     []string{"device"},
	SrcIP:      []string{"toString(src_ip)"},
	DstIP:      []string{"toString(dst_ip)"},
	SrcPort:    []string{"toString(src_port)"},
	DstPort:    []string{"toString(dst_port)"},
	SrcCountry: []string{"src_country"},
	DstCountry: []string{"dst_country"},
	SrcCity:    []string{"src_city"},
	DstCity:    []string{"dst_city"},
	Src:        []string{"toString(src_ip)", "src_country"},
	Dst:        []string{"toString(dst_ip)", "dst_country"},
	Proto:      []string{"proto"},
	Zone:       []string{"src_zone", "dst_zone"},
	IPv4Src:    "src_ip",
	IPv4Dst:    "dst_ip",
	PortSrc:    "src_port",
	PortDst:    "dst_port",
}

var GeoMapSearchColumns = MapSearchColumns{
	Simple: []string{
		"src_key", "dst_key", "src_label", "dst_label",
		"toString(src_port)", "toString(dst_port)",
		"src_country", "dst_country", "src_city", "dst_city",
		"last_action", "device", "proto",
	},
	IP:         []string{"src_key", "dst_key", "src_label", "dst_label"},
	Port:       []string{"toString(src_port)", "toString(dst_port)"},
	Country:    []string{"src_country", "dst_country", "src_key", "dst_key"},
	City:       []string{"src_city", "dst_city", "src_label", "dst_label"},
	Action:     []string{"last_action"},
	Device:     []string{"device"},
	SrcIP:      []string{"src_key", "src_label"},
	DstIP:      []string{"dst_key", "dst_label"},
	SrcPort:    []string{"toString(src_port)"},
	DstPort:    []string{"toString(dst_port)"},
	SrcCountry: []string{"src_country"},
	DstCountry: []string{"dst_country"},
	SrcCity:    []string{"src_city", "src_label"},
	DstCity:    []string{"dst_city", "dst_label"},
	Src:        []string{"src_key", "src_label", "src_country"},
	Dst:        []string{"dst_key", "dst_label", "dst_country"},
	Proto:      []string{"proto"},
	Zone:       []string{"src_key", "dst_key"},
	PortSrc:    "src_port",
	PortDst:    "dst_port",
}

var IPAggMapSearchColumns = MapSearchColumns{
	Simple: []string{
		"toString(src_ip)", "toString(dst_ip)",
		"toString(src_port)", "toString(dst_port)",
		"src_country", "dst_country", "src_city", "dst_city",
		"last_action", "device", "proto",
	},
	IP:         []string{"toString(src_ip)", "toString(dst_ip)"},
	Port:       []string{"toString(src_port)", "toString(dst_port)"},
	Country:    []string{"src_country", "dst_country"},
	City:       []string{"src_city", "dst_city"},
	Action:     []string{"last_action"},
	Device:     []string{"device"},
	SrcIP:      []string{"toString(src_ip)"},
	DstIP:      []string{"toString(dst_ip)"},
	SrcPort:    []string{"toString(src_port)"},
	DstPort:    []string{"toString(dst_port)"},
	SrcCountry: []string{"src_country"},
	DstCountry: []string{"dst_country"},
	SrcCity:    []string{"src_city"},
	DstCity:    []string{"dst_city"},
	Src:        []string{"toString(src_ip)", "src_country"},
	Dst:        []string{"toString(dst_ip)", "dst_country"},
	Proto:      []string{"proto"},
	Zone:       []string{"toString(src_ip)", "toString(dst_ip)"},
	IPv4Src:    "src_ip",
	IPv4Dst:    "dst_ip",
	PortSrc:    "src_port",
	PortDst:    "dst_port",
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
		return simpleSQL(c.Simple, cols)
	}
	if c.Root == nil {
		return "", nil
	}
	return nodeSQL(c.Root, cols)
}

func simpleSQL(needle string, cols MapSearchColumns) (string, []any) {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return "", nil
	}
	if looksFullIPv4(needle) {
		return ipTermSQL(mapsearch.FieldIP, needle, mapsearch.OpContains, cols)
	}
	if port, ok := parseExactPort(needle); ok {
		return portEqBothSQL(cols, port)
	}
	return simpleORSQL(needle, cols)
}

func simpleORSQL(needle string, cols MapSearchColumns) (string, []any) {
	if len(cols.Simple) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(cols.Simple))
	args := make([]any, 0, len(cols.Simple))
	for _, col := range cols.Simple {
		parts = append(parts, "positionCaseInsensitiveUTF8("+col+", ?) > 0")
		args = append(args, needle)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
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
		args := make([]any, 0, len(needles)*len(cols.Simple))
		for _, needle := range needles {
			if strings.TrimSpace(needle) == "" {
				continue
			}
			part, pargs := simpleORSQL(needle, cols)
			if part == "" {
				continue
			}
			parts = append(parts, part)
			args = append(args, pargs...)
		}
		return joinByOp(n.Op, parts), args
	}

	switch n.Field {
	case mapsearch.FieldIP, mapsearch.FieldSrcIP, mapsearch.FieldDstIP, mapsearch.FieldSrc, mapsearch.FieldDst:
		var parts []string
		var args []any
		for _, needle := range needles {
			if strings.TrimSpace(needle) == "" {
				continue
			}
			part, pargs := ipTermSQL(n.Field, needle, n.Op, cols)
			if part == "" {
				continue
			}
			parts = append(parts, part)
			args = append(args, pargs...)
		}
		return joinByOp(n.Op, parts), args
	case mapsearch.FieldPort, mapsearch.FieldSrcPort, mapsearch.FieldDstPort:
		var parts []string
		var args []any
		for _, needle := range needles {
			if strings.TrimSpace(needle) == "" {
				continue
			}
			part, pargs := portTermSQL(n.Field, needle, n.Op, cols)
			if part == "" {
				continue
			}
			parts = append(parts, part)
			args = append(args, pargs...)
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

func ipTermSQL(field mapsearch.Field, needle string, op mapsearch.Op, cols MapSearchColumns) (string, []any) {
	needle = strings.TrimSpace(needle)
	src, dst := ipv4Sides(field, cols)
	full := looksFullIPv4(needle)
	prefix := looksIPv4Prefix(needle)

	if (src != "" || dst != "") && (full || prefix) {
		var parts []string
		var args []any
		addNative := func(col string) {
			if col == "" {
				return
			}
			switch {
			case full && op == mapsearch.OpNe:
				parts = append(parts, col+" != toIPv4OrNull(?)")
				args = append(args, needle)
			case full && (op == mapsearch.OpEq || op == mapsearch.OpContains):
				parts = append(parts, col+" = toIPv4OrNull(?)")
				args = append(args, needle)
			case prefix && op != mapsearch.OpNe:
				parts = append(parts, "startsWith(toString("+col+"), ?)")
				args = append(args, needle)
			case prefix && op == mapsearch.OpNe:
				parts = append(parts, "NOT startsWith(toString("+col+"), ?)")
				args = append(args, needle)
			}
		}
		addNative(src)
		addNative(dst)
		if len(parts) > 0 {
			return joinByOp(op, parts), args
		}
	}

	// String / geo-key fallback (and non-IP needles on src/dst which also match country).
	fieldCols := cols.cols(field)
	if len(fieldCols) == 0 {
		return "", nil
	}
	var parts []string
	var args []any
	for _, col := range fieldCols {
		part, arg := colMatchSQL(col, needle, op)
		parts = append(parts, part)
		args = append(args, arg)
	}
	return joinByOp(op, parts), args
}

func ipv4Sides(field mapsearch.Field, cols MapSearchColumns) (src, dst string) {
	switch field {
	case mapsearch.FieldSrcIP, mapsearch.FieldSrc:
		return cols.IPv4Src, ""
	case mapsearch.FieldDstIP, mapsearch.FieldDst:
		return "", cols.IPv4Dst
	default:
		return cols.IPv4Src, cols.IPv4Dst
	}
}

func portTermSQL(field mapsearch.Field, needle string, op mapsearch.Op, cols MapSearchColumns) (string, []any) {
	if port, ok := parseExactPort(needle); ok && (op == mapsearch.OpEq || op == mapsearch.OpContains || op == mapsearch.OpNe) {
		src, dst := portSides(field, cols)
		var parts []string
		var args []any
		add := func(col string) {
			if col == "" {
				return
			}
			if op == mapsearch.OpNe {
				parts = append(parts, col+" != ?")
			} else {
				parts = append(parts, col+" = ?")
			}
			args = append(args, port)
		}
		add(src)
		add(dst)
		if len(parts) > 0 {
			return joinByOp(op, parts), args
		}
	}
	fieldCols := cols.cols(field)
	if len(fieldCols) == 0 {
		return "", nil
	}
	var parts []string
	var args []any
	for _, col := range fieldCols {
		part, arg := colMatchSQL(col, needle, op)
		parts = append(parts, part)
		args = append(args, arg)
	}
	return joinByOp(op, parts), args
}

func portSides(field mapsearch.Field, cols MapSearchColumns) (src, dst string) {
	switch field {
	case mapsearch.FieldSrcPort:
		return cols.PortSrc, ""
	case mapsearch.FieldDstPort:
		return "", cols.PortDst
	default:
		return cols.PortSrc, cols.PortDst
	}
}

func portEqBothSQL(cols MapSearchColumns, port uint32) (string, []any) {
	var parts []string
	var args []any
	if cols.PortSrc != "" {
		parts = append(parts, cols.PortSrc+" = ?")
		args = append(args, port)
	}
	if cols.PortDst != "" {
		parts = append(parts, cols.PortDst+" = ?")
		args = append(args, port)
	}
	if len(parts) == 0 {
		return simpleORSQL(strconv.FormatUint(uint64(port), 10), cols)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
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

func looksFullIPv4(s string) bool {
	ip := net.ParseIP(strings.TrimSpace(s))
	return ip != nil && ip.To4() != nil && strings.Count(s, ".") == 3
}

func looksIPv4Prefix(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || looksFullIPv4(s) {
		return false
	}
	// "10." / "10.0." / "10.0.0"
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 4 {
		return false
	}
	for i, p := range parts {
		if p == "" {
			// trailing dot → empty last segment OK if not first
			if i == 0 || i != len(parts)-1 {
				return false
			}
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	return true
}

func parseExactPort(s string) (uint32, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 65535 {
		return 0, false
	}
	return uint32(n), true
}
