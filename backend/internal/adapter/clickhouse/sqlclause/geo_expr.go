package sqlclause

import "fmt"

// GeoEnrichIPTable — ephemeral IP→geo lookup для rebuild geo-edges без ALTER UPDATE
// на traffic_logs. Заполняется EnrichLogsMissingGeo, читается INSERT SELECT.
const GeoEnrichIPTable = "nm_geo_enrich_ip"

const (
	IPEdgesDailyTable  = "traffic_edges_daily"
	IPEdgesDailyMV     = "traffic_edges_daily_mv"
	IPEdgesHourlyTable = "traffic_edges_hourly"
	IPEdgesHourlyMV    = "traffic_edges_hourly_mv"
)

// DayTimestampRangeSQL — prune-friendly фильтр дня. Bind: day, day (Date).
func DayTimestampRangeSQL(tsCol string) string {
	return fmt.Sprintf("%[1]s >= toDateTime(?) AND %[1]s < toDateTime(?) + INTERVAL 1 DAY", tsCol)
}

// CountryNeedsSQL — условие «страна нужна из GeoIP» (зеркало model.NeedsCountry).
func CountryNeedsSQL(col string) string {
	return fmt.Sprintf(`(%[1]s = '' OR lower(%[1]s) IN ('unknown', 'reserved') OR %[1]s = 'Неизвестно' OR lengthUTF8(trimBoth(%[1]s)) = 2)`, col)
}

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
	ip := fmt.Sprintf("toString(%s)", colRef(table, side+"_ip"))
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
	ip := fmt.Sprintf("toString(%s)", colRef(table, side+"_ip"))
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

// IPKeyExpr — ключ/лейбл узла при группировке по IP (dotted-quad String).
func IPKeyExpr(side string) string {
	return ipKeyExpr(side, "")
}

func ipKeyExpr(side, table string) string {
	return fmt.Sprintf("toString(%s)", colRef(table, side+"_ip"))
}

// SubnetKeyExpr — IPv4 /24 (колонка типа IPv4).
func SubnetKeyExpr(side string) string {
	return subnetKeyExpr(side, "")
}

func subnetKeyExpr(side, table string) string {
	ip := colRef(table, side+"_ip")
	// 4294967040 = 0xFFFFFF00
	return fmt.Sprintf(
		`concat(IPv4NumToString(bitAnd(toUInt32(%[1]s), toUInt32(4294967040))), '/24')`,
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
		k := ipKeyExpr("src", table)
		d := ipKeyExpr("dst", table)
		return k, d, k, d
	case "subnet":
		k := subnetKeyExpr("src", table)
		d := subnetKeyExpr("dst", table)
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
