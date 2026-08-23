package systemlive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyslogNGAdapterEmptyURL(t *testing.T) {
	var a *SyslogNGAdapter
	if _, ok := a.Snapshot(context.Background()); ok {
		t.Fatal("nil adapter")
	}
	a = &SyslogNGAdapter{URL: "  "}
	if _, ok := a.Snapshot(context.Background()); ok {
		t.Fatal("blank url")
	}
}

func TestSyslogNGAdapterFetchError(t *testing.T) {
	a := &SyslogNGAdapter{URL: "http://127.0.0.1:1", Client: &http.Client{}}
	snap, ok := a.Snapshot(context.Background())
	if !ok {
		t.Fatal("want ok with down exporter")
	}
	if snap.Up {
		t.Fatalf("snap: %+v", snap)
	}
}

func TestSyslogNGAdapterFetchOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`syslogng_dst_dropped{id="d_backend"} 1
syslogng_dst_queued{id="d_backend"} 2
syslogng_src_processed{id="s_udp"} 60
syslogng_src_processed{id="s_tcp"} 40
`))
	}))
	defer srv.Close()

	a := &SyslogNGAdapter{URL: srv.URL}
	snap, ok := a.Snapshot(context.Background())
	if !ok {
		t.Fatal("want ok")
	}
	if !snap.Up || snap.DroppedTotal != 1 || snap.Queued != 2 {
		t.Fatalf("snap: %+v", snap)
	}
	if snap.ProcessedTotal != 100 || snap.UDPProcessed != 60 || snap.TCPProcessed != 40 {
		t.Fatalf("counts: processed=%d udp=%d tcp=%d", snap.ProcessedTotal, snap.UDPProcessed, snap.TCPProcessed)
	}
}
