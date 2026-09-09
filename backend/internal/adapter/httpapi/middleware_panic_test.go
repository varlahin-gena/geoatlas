package httpapi

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type recordingMetrics struct {
	inFlight atomic.Int64
	peak     atomic.Int64
	observed atomic.Int64
	status   atomic.Int64
}

func (m *recordingMetrics) Handler() http.Handler { return http.NotFoundHandler() }

func (m *recordingMetrics) ObserveHTTP(_, _ string, status int, _ time.Duration) {
	m.observed.Add(1)
	m.status.Store(int64(status))
}

func (m *recordingMetrics) IncInFlight() {
	if n := m.inFlight.Add(1); n > m.peak.Load() {
		m.peak.Store(n)
	}
}

func (m *recordingMetrics) DecInFlight() { m.inFlight.Add(-1) }

// Паника в хендлере не должна навсегда завышать in-flight gauge,
// а сам запрос обязан попасть в гистограмму как 500.
func TestMetricsMiddlewareReleasesInFlightOnPanic(t *testing.T) {
	m := &recordingMetrics{}
	h := recoverMW(metricsMW(m)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/events", nil))

	if got := m.inFlight.Load(); got != 0 {
		t.Fatalf("in-flight gauge leaked: got %d want 0", got)
	}
	if m.peak.Load() != 1 {
		t.Fatalf("gauge never incremented: peak %d", m.peak.Load())
	}
	if m.observed.Load() != 1 {
		t.Fatalf("panic not observed: %d observations", m.observed.Load())
	}
	if got := m.status.Load(); got != http.StatusInternalServerError {
		t.Fatalf("observed status %d want 500", got)
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("response code %d want 500", rr.Code)
	}
}

func TestMetricsMiddlewareObservesNormalResponse(t *testing.T) {
	m := &recordingMetrics{}
	h := metricsMW(m)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/events", nil))

	if got := m.inFlight.Load(); got != 0 {
		t.Fatalf("in-flight gauge not released: %d", got)
	}
	if got := m.status.Load(); got != http.StatusTeapot {
		t.Fatalf("observed status %d want 418", got)
	}
}

// loggingMW не должен глотать панику — recoverMW обязан её увидеть.
func TestLoggingMiddlewarePropagatesPanic(t *testing.T) {
	h := recoverMW(loggingMW(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/events", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("response code %d want 500", rr.Code)
	}
}
