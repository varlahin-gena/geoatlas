package syslogngstats

import "testing"

func TestParseCSVDestinationsAndSources(t *testing.T) {
	body := []byte(`src.network;s_udp#0;;a;processed;100
src.network;s_tcp#0;;a;processed;50
dst.network;d_backend_udp;;a;dropped;3
dst.network;d_backend_tcp;;a;dropped;1
dst.network;d_backend_udp;;a;queued;10
dst.network;d_backend_tcp;;a;queued;4
center;;received;a;processed;150
`)
	snap, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if snap.UDPProcessed != 100 || snap.TCPProcessed != 50 || snap.ProcessedTotal != 150 {
		t.Fatalf("processed udp=%v tcp=%v total=%v", snap.UDPProcessed, snap.TCPProcessed, snap.ProcessedTotal)
	}
	if snap.DroppedTotal != 4 {
		t.Fatalf("dropped=%v", snap.DroppedTotal)
	}
	if snap.Queued != 14 {
		t.Fatalf("queued=%v", snap.Queued)
	}
}

func TestParsePrometheus(t *testing.T) {
	body := []byte(`# TYPE syslogng_src_processed counter
syslogng_src_processed{id="s_udp#0"} 20
syslogng_src_processed{id="s_tcp#0"} 5
syslogng_dst_dropped{id="d_backend_udp#0"} 2
syslogng_dst_queued{id="d_backend_tcp"} 7
`)
	snap, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if snap.UDPProcessed != 20 || snap.TCPProcessed != 5 || snap.ProcessedTotal != 25 {
		t.Fatalf("processed %+v", snap)
	}
	if snap.DroppedTotal != 2 || snap.Queued != 7 {
		t.Fatalf("dst %+v", snap)
	}
}

func TestParseEmpty(t *testing.T) {
	if _, err := Parse(nil); err == nil {
		t.Fatal("expected error")
	}
}
