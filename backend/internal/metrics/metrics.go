package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	IngestReceivedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_received_total",
		Help: "Total syslog/log lines received by ingest",
	}, []string{"transport"})

	IngestParsedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ingest_parsed_total",
		Help: "Successfully parsed lines",
	})

	IngestSkippedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ingest_skipped_total",
		Help: "Intentionally skipped lines (SkipParser)",
	})

	IngestParseErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ingest_parse_errors_total",
		Help: "Lines that failed to parse",
	})

	IngestInsertedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ingest_inserted_total",
		Help: "Rows inserted into traffic_logs",
	})

	IngestQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ingest_queue_depth",
		Help: "Current depth of the ingest line queue",
	})

	IngestQueueCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ingest_queue_capacity",
		Help: "Capacity of the ingest line queue",
	})

	IngestFlushDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ingest_flush_duration_seconds",
		Help:    "Duration of ClickHouse batch flush",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 15, 30},
	})

	ClickHouseInsertErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "clickhouse_insert_errors_total",
		Help: "Failed ClickHouse insert attempts (including those later retried)",
	}, []string{"table"})

	ClickHouseInsertRetries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "clickhouse_insert_retries_total",
		Help: "Extra ClickHouse insert attempts that eventually succeeded (attempt-1 counted)",
	}, []string{"table"})

	ClickHouseInsertCircuitOpen = promauto.NewCounter(prometheus.CounterOpts{
		Name: "clickhouse_insert_circuit_open_total",
		Help: "Times the ingest insert circuit breaker rejected or opened",
	})

	IngestParseErrorBufDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ingest_parse_error_buf_dropped_total",
		Help: "Parse-error buffer entries dropped because the buffer was at capacity (ClickHouse outage)",
	})

	IngestTrafficBufDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ingest_traffic_buf_dropped_total",
		Help: "Traffic-log buffer entries dropped because the buffer was at capacity (ClickHouse outage)",
	})

	IngestQueueDroppedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_queue_dropped_total",
		Help: "Lines dropped because the ingest line queue was full (TCP backpressure)",
	}, []string{"transport"})

	IngestFrameTooLargeTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ingest_frame_too_large_total",
		Help: "TCP syslog frames rejected because they exceeded the maximum frame size",
	})

	APIRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "api_request_duration_seconds",
		Help:    "HTTP API request latency",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 15, 30, 60},
	}, []string{"method", "path", "status"})
)

// Handler exposes Prometheus /metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}
