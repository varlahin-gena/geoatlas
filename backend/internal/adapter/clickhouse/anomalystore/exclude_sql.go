package anomalystore

import (
	"context"

	"geoatlas/internal/adapter/clickhouse/sqlclause"
	"geoatlas/internal/mapsearch"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

// notMatch — «AND NOT (запрос карты)» для строк traffic_logs.
// Часовые агрегаты хранят страну и действие как состояние агрегата, поэтому
// исключение считается по журналу, где те же поля обычные колонки.
func notMatch(ctx context.Context, code string) (string, []any) {
	q := usecaseanomaly.ExclusionQuery(ctx, code)
	if q == "" {
		return "", nil
	}
	pred, args := sqlclause.MapSearchSQL(mapsearch.Compile(q), sqlclause.LogsMapSearchColumns)
	if pred == "" {
		return "", nil
	}
	return " AND NOT (" + pred + ")", args
}
