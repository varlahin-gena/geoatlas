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
	def := snap.Connectors["default"]
	if def == nil || def.UDPProcessed != 100 || def.TCPProcessed != 50 || def.Dropped != 4 || def.Queued != 14 {
		t.Fatalf("default connector %+v", def)
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
	def := snap.Connectors["default"]
	if def == nil || def.UDPProcessed != 20 || def.TCPProcessed != 5 || def.Dropped != 2 || def.Queued != 7 {
		t.Fatalf("default connector %+v", def)
	}
}

func TestParsePrometheusPerConnector(t *testing.T) {
	body := []byte(`# TYPE syslogng_src_processed counter
syslogng_src_processed{id="s_default_udp#0"} 1000
syslogng_src_processed{id="s_default_tcp#0"} 500
syslogng_src_processed{id="s_id_1515_udp#0"} 200
syslogng_src_processed{id="s_id_5516_tcp#0"} 300
syslogng_dst_dropped{id="d_id_1515_udp#0"} 4
syslogng_dst_queued{id="d_id_5516_tcp#0"} 9
`)
	snap, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if snap.UDPProcessed != 1200 || snap.TCPProcessed != 800 || snap.ProcessedTotal != 2000 {
		t.Fatalf("totals %+v", snap)
	}
	def := snap.Connectors["default"]
	if def == nil || def.UDPProcessed != 1000 || def.TCPProcessed != 500 {
		t.Fatalf("default %+v", def)
	}
	c1515 := snap.Connectors["id_1515"]
	if c1515 == nil || c1515.UDPProcessed != 200 || c1515.Dropped != 4 {
		t.Fatalf("id_1515 %+v", c1515)
	}
	c5516 := snap.Connectors["id_5516"]
	if c5516 == nil || c5516.TCPProcessed != 300 || c5516.Queued != 9 {
		t.Fatalf("id_5516 %+v", c5516)
	}
}

func TestParseConnectorSourceIDStripsInstance(t *testing.T) {
	cases := []struct {
		in, id, proto string
	}{
		{"s_udp#0", "default", "udp"},
		{"s_default_udp#0", "default", "udp"},
		{"s_id_1515_tcp#0", "id_1515", "tcp"},
		{"d_id_5516_udp#12", "id_5516", "udp"},
		{"s_default_udp", "default", "udp"},
	}
	for _, tc := range cases {
		id, proto := parseConnectorSourceID(tc.in)
		if id != tc.id || proto != tc.proto {
			t.Fatalf("%q => %q/%q, want %q/%q", tc.in, id, proto, tc.id, tc.proto)
		}
	}
}

func TestParseEmpty(t *testing.T) {
	if _, err := Parse(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePrometheusInputOutputEvents(t *testing.T) {
	// Default stats-exporter format in syslog-ng 4.9+ (without --with-legacy-metrics).
	body := []byte(`# TYPE syslogng_input_events_total counter
syslogng_input_events_total{id="s_default_udp#0",result="processed"} 1000
syslogng_input_events_total{id="s_default_tcp#0",result="processed"} 500
syslogng_input_events_total{id="s_id_1515_udp#0",result="processed"} 200
syslogng_input_events_total{id="s_id_5516_tcp#0",result="processed"} 300
syslogng_output_events_total{id="d_id_1515_udp#0",driver_instance="tcp,backend:1514",result="dropped"} 4
syslogng_output_events_total{id="d_id_5516_tcp#0",driver_instance="tcp,backend:1514",result="queued"} 9
syslogng_output_events_total{id="d_id_5516_tcp#0",driver_instance="tcp,backend:1514",result="delivered"} 300
`)
	snap, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if snap.UDPProcessed != 1200 || snap.TCPProcessed != 800 || snap.ProcessedTotal != 2000 {
		t.Fatalf("totals %+v", snap)
	}
	if snap.DroppedTotal != 4 || snap.Queued != 9 {
		t.Fatalf("dst dropped=%v queued=%v", snap.DroppedTotal, snap.Queued)
	}
	def := snap.Connectors["default"]
	if def == nil || def.UDPProcessed != 1000 || def.TCPProcessed != 500 {
		t.Fatalf("default %+v", def)
	}
	c1515 := snap.Connectors["id_1515"]
	if c1515 == nil || c1515.UDPProcessed != 200 || c1515.Dropped != 4 {
		t.Fatalf("id_1515 %+v", c1515)
	}
	c5516 := snap.Connectors["id_5516"]
	if c5516 == nil || c5516.TCPProcessed != 300 || c5516.Queued != 9 {
		t.Fatalf("id_5516 %+v", c5516)
	}
}
