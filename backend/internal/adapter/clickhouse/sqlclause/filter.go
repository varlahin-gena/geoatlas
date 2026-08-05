package sqlclause

import (
	"fmt"

	"network_monitor/internal/model"
)

// ActionWhereSQL — WHERE-фрагмент фильтрации по action (пустая строка = all).
func ActionWhereSQL(filter string) string {
	blockedIn := model.BlockedInClause()
	switch filter {
	case "blocked":
		return fmt.Sprintf(" AND lower(action) IN (%s)", blockedIn)
	case "allowed":
		return fmt.Sprintf(" AND lower(action) NOT IN (%s) AND lower(action) NOT IN ('','unknown')", blockedIn)
	default:
		return ""
	}
}

// CountIfBlockedSQL / CountIfAllowedSQL — выражения для raw GROUP BY по traffic_logs.
func CountIfBlockedSQL() string {
	return fmt.Sprintf("countIf(lower(action) IN (%s))", model.BlockedInClause())
}

func CountIfAllowedSQL() string {
	return fmt.Sprintf(
		"countIf(lower(action) NOT IN (%s) AND lower(action) NOT IN ('','unknown'))",
		model.BlockedInClause(),
	)
}

// SumBlockedSQL / SumAllowedSQL — выражения для MV/backfill.
// if(...,1,0) вместо toUInt64(bool): в CH 24+/25 Bool нельзя надёжно кастовать в UInt64.
func SumBlockedSQL() string {
	return fmt.Sprintf("sum(if(lower(action) IN (%s), toUInt64(1), toUInt64(0)))", model.BlockedInClause())
}

func SumAllowedSQL() string {
	return fmt.Sprintf(
		"sum(if(lower(action) NOT IN (%s) AND lower(action) NOT IN ('','unknown'), toUInt64(1), toUInt64(0)))",
		model.BlockedInClause(),
	)
}

// HavingAggFilterSQL — HAVING для pre-agg таблиц с blocked_cnt/allowed_cnt.
func HavingAggFilterSQL(filter string) string {
	switch filter {
	case "blocked":
		return " HAVING sum(blocked_cnt) > 0"
	case "allowed":
		return " HAVING sum(allowed_cnt) > 0"
	default:
		return ""
	}
}

// OrderByAggFilterSQL — ORDER BY для top-N в ClickHouse (совпадает с rawAggSortKey).
// Для traffic_edges_daily (колонки src_ip/dst_ip).
func OrderByAggFilterSQL(filter string) string {
	switch filter {
	case "blocked":
		return "ORDER BY blocked_cnt DESC, cnt DESC, src_ip, dst_ip"
	case "allowed":
		return "ORDER BY allowed_cnt DESC, cnt DESC, src_ip, dst_ip"
	default:
		return "ORDER BY cnt DESC, src_ip, dst_ip"
	}
}

// OrderByGeoAggFilterSQL — ORDER BY для traffic_edges_{city|country}_daily
// (src_key/dst_key; src_ip там нет — иначе CH code 47 и fallback на traffic_logs).
// coord_weight первым: иначе пары без координат забивают LIMIT.
func OrderByGeoAggFilterSQL(filter string) string {
	switch filter {
	case "blocked":
		return "ORDER BY coord_weight DESC, blocked_cnt DESC, cnt DESC, src_key, dst_key"
	case "allowed":
		return "ORDER BY coord_weight DESC, allowed_cnt DESC, cnt DESC, src_key, dst_key"
	default:
		return "ORDER BY coord_weight DESC, cnt DESC, src_key, dst_key"
	}
}

// OrderByMapAggFilterSQL — top-N для карты: сначала пары с координатами,
// иначе LAN без geo забивает LIMIT и карта IP/subnet остаётся пустой.
func OrderByMapAggFilterSQL(filter string) string {
	geoFirst := "countIf(" + GeoCoordOK + ") DESC"
	switch filter {
	case "blocked":
		return "ORDER BY " + geoFirst + ", blocked_cnt DESC, cnt DESC, src_ip, dst_ip"
	case "allowed":
		return "ORDER BY " + geoFirst + ", allowed_cnt DESC, cnt DESC, src_ip, dst_ip"
	default:
		return "ORDER BY " + geoFirst + ", cnt DESC, src_ip, dst_ip"
	}
}
