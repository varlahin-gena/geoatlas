package query

import "context"

type tablesCtxKey struct{}

// Tables — набор имён таблиц для map/events SQL (live или shadow бэкапа).
type Tables struct {
	Logs         string
	EdgesDaily   string
	EdgesHourly  string
	EdgesCity       string
	EdgesCountry    string
	EdgesContinent  string
}

func LiveTables() Tables {
	return Tables{
		Logs:         "traffic_logs",
		EdgesDaily:   "traffic_edges_daily",
		EdgesHourly:  "traffic_edges_hourly",
		EdgesCity:      "traffic_edges_city_daily",
		EdgesCountry:   "traffic_edges_country_daily",
		EdgesContinent: "traffic_edges_continent_daily",
	}
}

// BackupTables — shadow после «Подключить» (RESTORE … AS ga_bak_*).
func BackupTables() Tables {
	return Tables{
		Logs:         "ga_bak_traffic_logs",
		EdgesDaily:   "ga_bak_traffic_edges_daily",
		EdgesHourly:  "ga_bak_traffic_edges_hourly",
		EdgesCity:      "ga_bak_traffic_edges_city_daily",
		EdgesCountry:   "ga_bak_traffic_edges_country_daily",
		EdgesContinent: "ga_bak_traffic_edges_continent_daily",
	}
}

func (t Tables) IsBackup() bool {
	return t.Logs == BackupTables().Logs
}

func (t Tables) GeoEdges(groupBy string) string {
	switch groupBy {
	case "city":
		return t.EdgesCity
	case "country":
		return t.EdgesCountry
	case "continent":
		return t.EdgesContinent
	default:
		return ""
	}
}

func WithTables(ctx context.Context, t Tables) context.Context {
	return context.WithValue(ctx, tablesCtxKey{}, t)
}

func TablesOf(ctx context.Context) Tables {
	if ctx == nil {
		return LiveTables()
	}
	if t, ok := ctx.Value(tablesCtxKey{}).(Tables); ok && t.Logs != "" {
		return t
	}
	return LiveTables()
}

// MapShadowPairs — live → ga_bak_* для attach (только то, что нужно карте).
func MapShadowPairs() [][2]string {
	live := LiveTables()
	bak := BackupTables()
	return [][2]string{
		{live.Logs, bak.Logs},
		{live.EdgesDaily, bak.EdgesDaily},
		{live.EdgesHourly, bak.EdgesHourly},
		{live.EdgesCity, bak.EdgesCity},
		{live.EdgesCountry, bak.EdgesCountry},
		{live.EdgesContinent, bak.EdgesContinent},
	}
}

func MapShadowNames() []string {
	pairs := MapShadowPairs()
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, p[1])
	}
	return out
}
