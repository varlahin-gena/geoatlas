package sqlclause

import "fmt"

// GeoEnrichIPTable — ephemeral IP→geo lookup для rebuild geo-edges без ALTER UPDATE
// на traffic_logs. Заполняется EnrichLogsMissingGeo, читается INSERT SELECT.
const GeoEnrichIPTable = "ga_geo_enrich_ip"

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

// HourTimestampRangeSQL — фильтр одного часа. Bind: hourStart, hourStart (DateTime).
func HourTimestampRangeSQL(tsCol string) string {
	return fmt.Sprintf("%[1]s >= toDateTime(?) AND %[1]s < toDateTime(?) + INTERVAL 1 HOUR", tsCol)
}

// CountryNeedsSQL — условие «страна нужна из GeoIP» (зеркало model.NeedsCountry).
func CountryNeedsSQL(col string) string {
	return fmt.Sprintf(`(%[1]s = '' OR lower(%[1]s) IN ('unknown', 'reserved') OR %[1]s = 'Неизвестно' OR lengthUTF8(trimBoth(%[1]s)) = 2)`, col)
}

// CityNeedsSQL — пустой/placeholder city, можно заменить из lookup.
func CityNeedsSQL(col string) string {
	return fmt.Sprintf(`(%[1]s = '' OR lower(%[1]s) IN ('unknown', 'неизвестно'))`, col)
}

// TrafficLogsEnrichedFromSQL — FROM-подзапрос: traffic_logs + LEFT JOIN ga_geo_enrich_ip.
// Исторические дыры закрываются на чтении без ALTER UPDATE.
// Пустая lookup-таблица → поведение как FROM traffic_logs.
func TrafficLogsEnrichedFromSQL(wherePred string) string {
	sgCountryNeed := CountryNeedsSQL("traffic_logs.src_country")
	dgCountryNeed := CountryNeedsSQL("traffic_logs.dst_country")
	return fmt.Sprintf(`
		FROM (
			SELECT
				traffic_logs.timestamp AS timestamp,
				traffic_logs.action AS action,
				traffic_logs.rule AS rule,
				traffic_logs.proto AS proto,
				traffic_logs.src_port AS src_port,
				traffic_logs.dst_port AS dst_port,
				traffic_logs.device AS device,
				traffic_logs.src_zone AS src_zone,
				traffic_logs.dst_zone AS dst_zone,
				traffic_logs.bytes_sent AS bytes_sent,
				traffic_logs.bytes_recv AS bytes_recv,
				traffic_logs.packets_sent AS packets_sent,
				traffic_logs.packets_recv AS packets_recv,
				traffic_logs.src_ip AS src_ip,
				traffic_logs.dst_ip AS dst_ip,
				if(traffic_logs.src_lat = 0 AND traffic_logs.src_lon = 0 AND (sg.lat != 0 OR sg.lon != 0),
					sg.lat, traffic_logs.src_lat) AS src_lat,
				if(traffic_logs.src_lat = 0 AND traffic_logs.src_lon = 0 AND (sg.lat != 0 OR sg.lon != 0),
					sg.lon, traffic_logs.src_lon) AS src_lon,
				if(traffic_logs.dst_lat = 0 AND traffic_logs.dst_lon = 0 AND (dg.lat != 0 OR dg.lon != 0),
					dg.lat, traffic_logs.dst_lat) AS dst_lat,
				if(traffic_logs.dst_lat = 0 AND traffic_logs.dst_lon = 0 AND (dg.lat != 0 OR dg.lon != 0),
					dg.lon, traffic_logs.dst_lon) AS dst_lon,
				if(%[5]s AND sg.city != '', sg.city, traffic_logs.src_city) AS src_city,
				if(%[6]s AND dg.city != '', dg.city, traffic_logs.dst_city) AS dst_city,
				if(traffic_logs.src_region = '' AND sg.region != '', sg.region, traffic_logs.src_region) AS src_region,
				if(traffic_logs.dst_region = '' AND dg.region != '', dg.region, traffic_logs.dst_region) AS dst_region,
				if(%[2]s AND sg.country != '', sg.country, traffic_logs.src_country) AS src_country,
				if(%[3]s AND dg.country != '', dg.country, traffic_logs.dst_country) AS dst_country
			FROM traffic_logs
			LEFT JOIN %[1]s AS sg ON traffic_logs.src_ip = sg.ip
			LEFT JOIN %[1]s AS dg ON traffic_logs.dst_ip = dg.ip
			WHERE %[4]s
		) AS traffic_logs
	`, GeoEnrichIPTable, sgCountryNeed, dgCountryNeed, wherePred,
		CityNeedsSQL("traffic_logs.src_city"), CityNeedsSQL("traffic_logs.dst_city"))
}

// MapLogsFromSQL — FROM для скана карты. Live traffic_logs — с overlay ga_geo_enrich_ip.
func MapLogsFromSQL(logsTable, wherePred string) string {
	if logsTable == "traffic_logs" {
		return TrafficLogsEnrichedFromSQL(wherePred)
	}
	return fmt.Sprintf("FROM %s\n\t\tWHERE %s", logsTable, wherePred)
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

// cityKeyExpr / countryKeyExpr — ключи по сохранённым колонкам traffic_logs
// (live GeoIP path в service.IPGroupMeta может отличаться при miss).
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

func countryKeyExpr(side, table string) string {
	country := colRef(table, side+"_country")
	return fmt.Sprintf(`if(trimBoth(%[1]s) = '' OR %[1]s IN ('Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved'), 'Неизвестно', %[1]s)`, country)
}

func ipKeyExpr(side, table string) string {
	return fmt.Sprintf("toString(%s)", colRef(table, side+"_ip"))
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
