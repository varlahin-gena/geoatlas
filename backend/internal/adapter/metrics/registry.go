// Package metrics вЂ” Prometheus registry РґР»СЏ backend (HTTP + ingest).
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"network_monitor/internal/model"
)

const namespace = "nm"

// IngestStats — снимок live-ingest (реализация: *ingestnet.Service).
type IngestStats interface {
	Stats() model.IngestLiveStats
}

// Registry вЂ” РјРµС‚СЂРёРєРё РїСЂРѕС†РµСЃСЃР° + handler /metrics.
type Registry struct {
	reg *prometheus.Registry

	httpRequests *prometheus.HistogramVec
	httpInflight prometheus.Gauge

	insertDuration *prometheus.HistogramVec
	insertRows     *prometheus.CounterVec

	anomalyScanDuration prometheus.Histogram
	anomalyDetected     *prometheus.CounterVec
	anomalyScanErrors   *prometheus.CounterVec
	anomalySkippedTick  *prometheus.CounterVec
	anomalyInsert       prometheus.Counter

	ingest *ingestCollector
}

// New СЃРѕР·РґР°С‘С‚ registry. ingestSrc РјРѕР¶РµС‚ Р±С‹С‚СЊ nil (С‚РѕРіРґР° ingest gauges = 0).
func New(ingestSrc IngestStats) *Registry {
	reg := prometheus.NewRegistry()
	_ = reg.Register(collectors.NewGoCollector())
	_ = reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	r := &Registry{
		reg: reg,
		httpRequests: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency by method, route template, and status class.",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		}, []string{"method", "route", "code"}),
		httpInflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_requests_in_flight",
			Help:      "In-flight HTTP requests.",
		}),
		insertDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "ingest_insert_duration_seconds",
			Help:      "ClickHouse traffic_logs insert batch latency.",
			Buckets:   []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 15, 30},
		}, []string{"result"}),
		insertRows: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "ingest_insert_rows_total",
			Help:      "Rows inserted into traffic_logs (by result).",
		}, []string{"result"}),
		anomalyScanDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "anomaly_scan_duration_seconds",
			Help:      "Anomaly scanner tick duration.",
			Buckets:   []float64{.05, .1, .25, .5, 1, 2.5, 5, 10, 25},
		}),
		anomalyDetected: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "anomaly_detected_total",
			Help:      "Anomaly events inserted (by code and severity).",
		}, []string{"code", "severity"}),
		anomalyScanErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "anomaly_scan_errors_total",
			Help:      "Anomaly detector/insert errors (by code).",
		}, []string{"code"}),
		anomalySkippedTick: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "anomaly_skipped_tick_total",
			Help:      "Anomaly scanner ticks skipped (circuit/rebuild/disabled).",
		}, []string{"reason"}),
		anomalyInsert: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "anomaly_insert_total",
			Help:      "Rows inserted into anomaly_events.",
		}),
		ingest: newIngestCollector(ingestSrc),
	}
	reg.MustRegister(r.httpRequests, r.httpInflight, r.insertDuration, r.insertRows,
		r.anomalyScanDuration, r.anomalyDetected, r.anomalyScanErrors, r.anomalySkippedTick, r.anomalyInsert, r.ingest)
	return r
}

// SetIngest binds live ingest stats source (call after NewService).
func (r *Registry) SetIngest(src IngestStats) {
	if r != nil && r.ingest != nil {
		r.ingest.setSource(src)
	}
}

// Handler вЂ” promhttp scrape endpoint (Р±РµР· РІСЃС‚СЂРѕРµРЅРЅРѕРіРѕ auth).
func (r *Registry) Handler() http.Handler {
	if r == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "metrics unavailable", http.StatusServiceUnavailable)
		})
	}
	return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{Registry: r.reg})
}

// ObserveHTTP records one completed request.
func (r *Registry) ObserveHTTP(method, route string, status int, d time.Duration) {
	if r == nil {
		return
	}
	code := strconv.Itoa(status/100) + "xx"
	r.httpRequests.WithLabelValues(method, route, code).Observe(d.Seconds())
}

func (r *Registry) IncInFlight() {
	if r != nil {
		r.httpInflight.Inc()
	}
}
func (r *Registry) DecInFlight() {
	if r != nil {
		r.httpInflight.Dec()
	}
}

// ObserveInsert implements usecase/ingest.InsertObserver.
func (r *Registry) ObserveInsert(d time.Duration, rows int, success bool) {
	if r == nil {
		return
	}
	result := "ok"
	if !success {
		result = "error"
	}
	r.insertDuration.WithLabelValues(result).Observe(d.Seconds())
	if rows > 0 {
		r.insertRows.WithLabelValues(result).Add(float64(rows))
	}
}

// ObserveScan implements usecase/anomaly.Metrics.
func (r *Registry) ObserveScan(d time.Duration, inserted int, skipReason string) {
	if r == nil {
		return
	}
	if skipReason != "" {
		r.anomalySkippedTick.WithLabelValues(skipReason).Inc()
		return
	}
	r.anomalyScanDuration.Observe(d.Seconds())
	if inserted > 0 {
		r.anomalyInsert.Add(float64(inserted))
	}
}

func (r *Registry) IncDetected(code, severity string) {
	if r == nil {
		return
	}
	r.anomalyDetected.WithLabelValues(code, severity).Inc()
}

func (r *Registry) IncScanError(code string) {
	if r == nil {
		return
	}
	r.anomalyScanErrors.WithLabelValues(code).Inc()
}
