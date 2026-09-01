package mapsearch

import (
	"strings"
	"time"
)

// QueryCostTier — оценка нагрузки map/events запроса на ClickHouse.
type QueryCostTier string

const (
	QueryCostLight  QueryCostTier = "light"
	QueryCostMedium QueryCostTier = "medium"
	QueryCostHeavy  QueryCostTier = "heavy"

	mapLimitCapHeavy      = 3000
	mapLimitCapHeavyFilt  = 8000
	mapLimitCapMedium     = 10000
)

// MapQueryCostInput — параметры для оценки стоимости запроса карты.
type MapQueryCostInput struct {
	GroupBy   string
	Mode      string // minutes|hours|days|absolute
	Amount    int
	From      time.Time
	To        time.Time
	Country   string
	Query     string
	RepActive bool
}

// MapQueryCost — результат оценки + рекомендуемый потолок LIMIT.
type MapQueryCost struct {
	Tier     QueryCostTier `json:"tier"`
	Reasons  []string      `json:"reasons,omitempty"`
	LimitCap int           `json:"limit_cap"`
}

func isIPGroup(groupBy string) bool {
	switch strings.ToLower(strings.TrimSpace(groupBy)) {
	case "ip", "subnet":
		return true
	default:
		return false
	}
}

func hasMapFilter(country, query string) bool {
	return strings.TrimSpace(country) != "" || strings.TrimSpace(query) != ""
}

// AssessMapQueryCost оценивает, насколько дорогим будет GROUP BY / scan для карты.
func AssessMapQueryCost(in MapQueryCostInput) MapQueryCost {
	groupBy := strings.ToLower(strings.TrimSpace(in.GroupBy))
	if groupBy == "" {
		groupBy = "city"
	}
	filtered := hasMapFilter(in.Country, in.Query)
	ipGroup := isIPGroup(groupBy)

	var reasons []string
	heavy := false
	medium := false

	add := func(r string, h, m bool) {
		reasons = append(reasons, r)
		if h {
			heavy = true
		}
		if m {
			medium = true
		}
	}

	switch strings.ToLower(strings.TrimSpace(in.Mode)) {
	case "minutes":
		if ipGroup {
			add("группировка по IP на коротком периоде (минуты)", true, false)
		} else if in.Amount <= 60 {
			add("короткий период (минуты)", false, true)
		}
	case "hours":
		if ipGroup && in.Amount <= 12 {
			add("группировка по IP на периоде до 12 часов", true, false)
		} else if ipGroup {
			add("группировка по IP на часовом периоде", false, true)
		} else if in.Amount >= 6 {
			add("период от 6 часов без daily geo-agg", false, true)
		}
	case "days":
		if ipGroup && in.Amount >= 7 {
			add("группировка по IP на периоде 7+ дней", true, false)
		} else if ipGroup && in.Amount >= 3 && !filtered {
			add("группировка по IP без фильтра на периоде 3+ дней", true, false)
		} else if ipGroup {
			add("группировка по IP", false, true)
		} else if in.Amount >= 14 && !filtered {
			add("длинный период без фильтра", false, true)
		} else if in.Amount >= 7 && !filtered {
			add("период 7+ дней без фильтра", false, true)
		}
	case "absolute":
		if in.To.After(in.From) {
			span := in.To.Sub(in.From)
			if ipGroup && span > 7*24*time.Hour {
				add("группировка по IP на диапазоне >7 суток", true, false)
			} else if ipGroup && span > 48*time.Hour && !filtered {
				add("группировка по IP на широком диапазоне без фильтра", true, false)
			} else if span > 14*24*time.Hour && !filtered {
				add("широкий диапазон без фильтра", false, true)
			}
		}
	}

	if in.RepActive && ipGroup {
		add("фильтр репутации при группировке по IP", true, false)
	}

	tier := QueryCostLight
	cap := 50000
	switch {
	case heavy:
		tier = QueryCostHeavy
		cap = mapLimitCapHeavy
		if filtered {
			cap = mapLimitCapHeavyFilt
		}
	case medium:
		tier = QueryCostMedium
		cap = mapLimitCapMedium
	}

	return MapQueryCost{Tier: tier, Reasons: reasons, LimitCap: cap}
}

// EffectiveMapLimit применяет tier-cap к запрошенному limit (минимум 1).
func EffectiveMapLimit(requested int, cost MapQueryCost) (applied int, capped bool) {
	if requested <= 0 {
		requested = 10000
	}
	applied = requested
	if cost.LimitCap > 0 && applied > cost.LimitCap {
		applied = cost.LimitCap
		capped = true
	}
	if applied < 1 {
		applied = 1
	}
	return applied, capped
}
