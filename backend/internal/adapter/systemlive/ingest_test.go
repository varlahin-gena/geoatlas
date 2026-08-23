package systemlive

import (
	"testing"

	"geoatlas/internal/model"
)

type stubIngestStats struct {
	stats model.IngestLiveStats
}

func (s stubIngestStats) Stats() model.IngestLiveStats { return s.stats }

func TestIngestAdapterNil(t *testing.T) {
	var a *IngestAdapter
	if _, ok := a.Snapshot(); ok {
		t.Fatal("nil adapter should not be ok")
	}
	a = &IngestAdapter{}
	if _, ok := a.Snapshot(); ok {
		t.Fatal("nil source should not be ok")
	}
}

func TestIngestAdapterMapsFields(t *testing.T) {
	src := stubIngestStats{stats: model.IngestLiveStats{
		State: "running", ReceivedTotal: 10, ParsedTotal: 9, InsertedTotal: 8,
		SkippedTotal: 1, ParseErrorsTotal: 2, BufferedLines: 3,
		QueueDepth: 4, QueueCapacity: 100, QueueBytes: 50, QueueBytesCapacity: 200,
		DroppedTotal: 5, BufferDropsTotal: 6, Connections: 2, CircuitOpen: true,
		LastError: "boom", UDP: model.IngestTransportStats{ReceivedTotal: 7, Connections: 1},
		TCP: model.IngestTransportStats{ReceivedTotal: 3, Connections: 1},
	}}
	a := &IngestAdapter{Src: src}
	snap, ok := a.Snapshot()
	if !ok {
		t.Fatal("want ok")
	}
	if snap.State != "running" || snap.ReceivedTotal != 10 || snap.CircuitOpen != true {
		t.Fatalf("snapshot: %+v", snap)
	}
	if snap.UDPReceived != 7 || snap.TCPReceived != 3 {
		t.Fatalf("transport: udp=%d tcp=%d", snap.UDPReceived, snap.TCPReceived)
	}
	if snap.LastError != "boom" {
		t.Fatalf("last error = %q", snap.LastError)
	}
}
