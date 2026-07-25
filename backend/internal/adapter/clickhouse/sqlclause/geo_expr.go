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

// CityKeyExpr / CountryKeyExpr — ключи по сохранённым колонкам traffic_logs
// (live GeoIP path в service.IPGroupMeta может отличаться при miss).
func CityKeyExpr(side string) string {
	city := side + "_city"
	country := side + "_country"
	ip := side + "_ip"
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
	city := side + "_city"
	country := side + "_country"
	ip := side + "_ip"
	badCountry := fmt.Sprintf(`%[1]s IN ('', 'Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved')`, country)
	return fmt.Sprintf(`multiIf(
		trimBoth(%[1]s) != '', %[1]s,
		trimBoth(%[2]s) != '' AND NOT (%[4]s), %[2]s,
		%[3]s
	)`, city, country, ip, badCountry)
}

func CountryKeyExpr(side string) string {
	country := side + "_country"
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
	switch groupBy {
	case "city":
		return CityKeyExpr("src"), CityKeyExpr("dst"), CityLabelExpr("src"), CityLabelExpr("dst")
	case "ip":
		k := IPKeyExpr("src")
		d := IPKeyExpr("dst")
		return k, d, k, d
	case "subnet":
		k := SubnetKeyExpr("src")
		d := SubnetKeyExpr("dst")
		return k, d, k, d
	default:
		return CountryKeyExpr("src"), CountryKeyExpr("dst"), CountryKeyExpr("src"), CountryKeyExpr("dst")
	}
}

// GeoCoordOK — условие валидных координат для sumIf / countIf в geo-агрегатах.
const GeoCoordOK = `(src_lat != 0 OR src_lon != 0) AND (dst_lat != 0 OR dst_lon != 0)`

// CoordWeightSQL — вес пар с координатами (совместимо с Bool в CH 24+/25).
func CoordWeightSQL() string {
	return "sum(if(" + GeoCoordOK + ", toUInt64(1), toUInt64(0)))"
}
