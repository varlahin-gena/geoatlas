package clickhouse

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/model"
)

// FetchLatestMetrics возвращает последние значения всех метрик за 5 минут.
func FetchLatestMetrics(ctx context.Context, ch clickhouse.Conn) ([]model.MetricRecord, error) {
	rows, err := ch.Query(ctx, `
		SELECT
			argMax(timestamp, timestamp) AS ts,
			metric_type, target, metric_name,
			if(isFinite(argMax(value, timestamp)), argMax(value, timestamp), 0) AS value,
			argMax(labels, timestamp) AS labels
		FROM system_metrics
		WHERE timestamp >= now() - INTERVAL 5 MINUTE
		GROUP BY metric_type, target, metric_name
		ORDER BY metric_type, target, metric_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.MetricRecord
	for rows.Next() {
		var r model.MetricRecord
		if err := rows.Scan(&r.Timestamp, &r.MetricType, &r.Target, &r.MetricName, &r.Value, &r.Labels); err != nil {
			return nil, err
		}
		if math.IsNaN(r.Value) || math.IsInf(r.Value, 0) {
			r.Value = 0
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FetchMetricHistory возвращает таймсерии для указанных метрик с downsampling.
func FetchMetricHistory(
	ctx context.Context,
	ch clickhouse.Conn,
	keys []model.MetricKey,
	period time.Duration,
	step time.Duration,
) (map[string][]model.HistoryPoint, error) {
	if len(keys) == 0 {
		return map[string][]model.HistoryPoint{}, nil
	}

	conds := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys)*3)
	for _, k := range keys {
		conds = append(conds, "(metric_type = ? AND target = ? AND metric_name = ?)")
		args = append(args, k.Type, k.Target, k.Name)
	}
	whereKeys := strings.Join(conds, " OR ")

	periodSec := int64(period.Seconds())
	stepSec := int64(step.Seconds())
	if stepSec < 1 {
		stepSec = 1
	}

	query := fmt.Sprintf(`
		SELECT
			toStartOfInterval(timestamp, INTERVAL %d SECOND) AS bucket,
			metric_type, target, metric_name,
			if(isFinite(argMax(value, timestamp)), argMax(value, timestamp), 0) AS v
		FROM system_metrics
		WHERE timestamp >= now() - INTERVAL %d SECOND
		  AND (%s)
		GROUP BY bucket, metric_type, target, metric_name
		ORDER BY bucket
	`, stepSec, periodSec, whereKeys)

	rows, err := ch.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]model.HistoryPoint, len(keys))
	for _, k := range keys {
		result[k.String()] = []model.HistoryPoint{}
	}

	for rows.Next() {
		var (
			bucket                time.Time
			mType, mTarget, mName string
			value                 float64
		)
		if err := rows.Scan(&bucket, &mType, &mTarget, &mName, &value); err != nil {
			return nil, err
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			value = 0
		}
		key := model.MetricKey{Type: mType, Target: mTarget, Name: mName}.String()
		result[key] = append(result[key], model.HistoryPoint{Timestamp: bucket, Value: value})
	}

	return result, rows.Err()
}
