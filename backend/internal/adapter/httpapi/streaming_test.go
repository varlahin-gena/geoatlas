package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type noopMetrics struct{}

func (noopMetrics) Handler() http.Handler                          { return http.NotFoundHandler() }
func (noopMetrics) ObserveHTTP(string, string, int, time.Duration) {}
func (noopMetrics) IncInFlight()                                   {}
func (noopMetrics) DecInFlight()                                   {}

// deadlineWriter — ResponseWriter, умеющий то, что нужно стримингу.
type deadlineWriter struct {
	http.ResponseWriter
	flushed  int
	deadline time.Time
}

func (w *deadlineWriter) Flush() { w.flushed++ }

func (w *deadlineWriter) SetWriteDeadline(t time.Time) error {
	w.deadline = t
	return nil
}

// http.ResponseController идёт вглубь только через Unwrap. Без него дедлайн и
// Flush упираются в statusRecorder из logging/metrics, и стриминг ломается.
func TestResponseControllerReachesConnThroughMiddleware(t *testing.T) {
	base := &deadlineWriter{ResponseWriter: httptest.NewRecorder()}
	want := time.Now().Add(9 * time.Minute)

	var ctrlErr, flushErr error
	h := loggingMW(metricsMW(noopMetrics{})(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			rc := http.NewResponseController(w)
			ctrlErr = rc.SetWriteDeadline(want)
			flushErr = rc.Flush()
		},
	)))

	h.ServeHTTP(base, httptest.NewRequest(http.MethodGet, "/api/geo-ranges/export", nil))

	if ctrlErr != nil {
		t.Fatalf("SetWriteDeadline through middleware: %v", ctrlErr)
	}
	if !base.deadline.Equal(want) {
		t.Fatalf("deadline %v did not reach the connection (want %v)", base.deadline, want)
	}
	if flushErr != nil {
		t.Fatalf("Flush through middleware: %v", flushErr)
	}
	if base.flushed != 1 {
		t.Fatalf("Flush reached the connection %d times, want 1", base.flushed)
	}
}

func TestStatusRecorderUnwrapsToOriginal(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: inner, status: http.StatusOK}
	if rec.Unwrap() != http.ResponseWriter(inner) {
		t.Fatal("Unwrap must return the wrapped writer")
	}
}

// Экспорт обязан оставаться вне withTimeout: http.TimeoutHandler буферизует
// ответ целиком, что для полной выгрузки geo_ranges означает OOM.
func TestExportRouteRegisteredWithoutTimeoutHandler(t *testing.T) {
	rr := newRouteRegistrar()
	streamed := make(chan struct{})
	rr.Handle("GET", "/api/geo-ranges/export", http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("net,country\n"))
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Errorf("flush: %v", err)
			}
			close(streamed)
		},
	))

	srv := httptest.NewServer(rr.Handler())
	t.Cleanup(srv.Close)

	resp, err := srv.Client().Get(srv.URL + "/api/geo-ranges/export")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	select {
	case <-streamed:
	case <-time.After(5 * time.Second):
		t.Fatal("handler never streamed")
	}

	buf := make([]byte, 12)
	if _, err := io.ReadFull(resp.Body, buf); err != nil {
		t.Fatalf("read streamed chunk: %v", err)
	}
	if string(buf) != "net,country\n" {
		t.Fatalf("streamed chunk %q", buf)
	}
}
