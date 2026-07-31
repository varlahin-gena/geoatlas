package sqlclause

import "fmt"

// GeoEdgesTable возвращает имя daily-агрегата по city|country.
// Неизвестный groupBy → "" (не интерполируем произвольные строки в SQL).
func GeoEdgesTable(groupBy string) string {
	switch groupBy {
	case "city", "country":
		return "traffic_edges_" + groupBy + "_daily"
	default:
		return ""
	}
}

// GeoEdgesMV возвращает имя materialized view для geo-агрегата.
func GeoEdgesMV(groupBy string) string {
	switch groupBy {
	case "city", "country":
		return "traffic_edges_" + groupBy + "_daily_mv"
	default:
		return ""
	}
}

func colRef(table, col string) string {
	if table == "" {
		return col
	}
	return table + "." + col
}

// CityKeyExpr / CountryKeyExpr — ключи по сохранённым колонкам traffic_logs
// (live GeoIP path в service.IPGroupMeta может отличаться при miss).
func CityKeyExpr(side string) string {
	return cityKeyExpr(side, "")
}

func cityKeyExpr(side, table string) string {
	city := colRef(table, side+"_city")
	country := colRef(table, side+"_country")
	ip := colRef(table, side+"_ip")
	// Без города/страны ключ = IP (не city:unknown): иначе разные адреса
	// схлопываются в один узел и дуга на карте становится self-loop.
	badCountry := fmt.Sprintf(`%[1]s IN ('', 'Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved')`, country)
	return fmt.Sprintf(`multiIf(
		trimBoth(%[1]s) != '' AND trimBoth(%[2]s) != '' AND NOT (%[4]s),
			concat(%[1]s, ', ', %[2]s),
		trimBoth(%[1]s) != '',
			%[1]s,
		trimBoth(%[2]s) != '' AND NOT (%[4]s),
			concat('city:country:', %[2]s),
		%[3]s
	)`, city, country, ip, badCountry)
}

func CityLabelExpr(side string) string {
	return cityLabelExpr(side, "")
}

func cityLabelExpr(side, table string) string {
	city := colRef(table, side+"_city")
	country := colRef(table, side+"_country")
	ip := colRef(table, side+"_ip")
	badCountry := fmt.Sprintf(`%[1]s IN ('', 'Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved')`, country)
	return fmt.Sprintf(`multiIf(
		trimBoth(%[1]s) != '', %[1]s,
		trimBoth(%[2]s) != '' AND NOT (%[4]s), %[2]s,
		%[3]s
	)`, city, country, ip, badCountry)
}

func CountryKeyExpr(side string) string {
	return countryKeyExpr(side, "")
}

func countryKeyExpr(side, table string) string {
	country := colRef(table, side+"_country")
	return fmt.Sprintf(`if(trimBoth(%[1]s) = '' OR %[1]s IN ('Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved'), 'Неизвестно', %[1]s)`, country)
}

// IPKeyExpr — ключ/лейбл узла при группировке по IP (колонка String).
func IPKeyExpr(side string) string {
	return side + "_ip"
}

// SubnetKeyExpr — IPv4 /24; не-IPv4 → unknown (как service.IPGroupMeta).
func SubnetKeyExpr(side string) string {
	ip := side + "_ip"
	// 4294967040 = 0xFFFFFF00
	return fmt.Sprintf(
		`if(isIPv4String(%[1]s), concat(IPv4NumToString(bitAnd(IPv4StringToNum(%[1]s), toUInt32(4294967040))), '/24'), 'unknown')`,
		ip,
	)
}

// GeoGroupExprs возвращает SQL-выражения ключей/лейблов для GROUP BY карты.
func GeoGroupExprs(groupBy string) (srcKey, dstKey, srcLabel, dstLabel string) {
	return geoGroupExprs(groupBy, "")
}

// GeoGroupExprsPrefixed — как GeoGroupExprs, но колонки квалифицированы table.
// Нужно для MV/INSERT SELECT, где anyState(src_city) AS src_city иначе
// подставляет AggregateFunction в trimBoth(src_city) (CH code 43).
func GeoGroupExprsPrefixed(table, groupBy string) (srcKey, dstKey, srcLabel, dstLabel string) {
	return geoGroupExprs(groupBy, table)
}

func geoGroupExprs(groupBy, table string) (srcKey, dstKey, srcLabel, dstLabel string) {
	switch groupBy {
	case "city":
		return cityKeyExpr("src", table), cityKeyExpr("dst", table),
			cityLabelExpr("src", table), cityLabelExpr("dst", table)
	case "ip":
		k := colRef(table, IPKeyExpr("src"))
		d := colRef(table, IPKeyExpr("dst"))
		return k, d, k, d
	case "subnet":
		if table == "" {
			k := SubnetKeyExpr("src")
			d := SubnetKeyExpr("dst")
			return k, d, k, d
		}
		k := fmt.Sprintf(
			`if(isIPv4String(%[1]s), concat(IPv4NumToString(bitAnd(IPv4StringToNum(%[1]s), toUInt32(4294967040))), '/24'), 'unknown')`,
			colRef(table, "src_ip"),
		)
		d := fmt.Sprintf(
			`if(isIPv4String(%[1]s), concat(IPv4NumToString(bitAnd(IPv4StringToNum(%[1]s), toUInt32(4294967040))), '/24'), 'unknown')`,
			colRef(table, "dst_ip"),
		)
		return k, d, k, d
	default:
		return countryKeyExpr("src", table), countryKeyExpr("dst", table),
			countryKeyExpr("src", table), countryKeyExpr("dst", table)
	}
}

// GeoCoordOK — условие валидных координат для sumIf / countIf в geo-агрегатах.
const GeoCoordOK = `(src_lat != 0 OR src_lon != 0) AND (dst_lat != 0 OR dst_lon != 0)`

// CoordWeightSQL — вес пар с координатами (совместимо с Bool в CH 24+/25).
func CoordWeightSQL() string {
	return "sum(if(" + GeoCoordOK + ", toUInt64(1), toUInt64(0)))"
}
