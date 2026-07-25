package systemlive

import (
	"network_monitor/internal/ingest"
	"network_monitor/internal/usecase/system"
)

type IngestStatsSource interface {
	Stats() ingest.StatsSnapshot
}

// IngestAdapter translates ingest package snapshots into the system use-case port.
type IngestAdapter struct {
	Src IngestStatsSource
}

var _ system.IngestLive = (*IngestAdapter)(nil)

func (a *IngestAdapter) Snapshot() (system.IngestSnapshot, bool) {
	if a == nil || a.Src == nil {
		return system.IngestSnapshot{}, false
	}
	snapshot := a.Src.Stats()
	return system.IngestSnapshot{
		State:         snapshot.State,
		ReceivedTotal: snapshot.ReceivedTotal, ParsedTotal: snapshot.ParsedTotal,
		InsertedTotal: snapshot.InsertedTotal, SkippedTotal: snapshot.SkippedTotal,
		ParseErrorsTotal: snapshot.ParseErrorsTotal, BufferedLines: snapshot.BufferedLines,
		QueueDepth: snapshot.QueueDepth, QueueCapacity: snapshot.QueueCapacity,
		QueueBytes: snapshot.QueueBytes, QueueBytesCapacity: snapshot.QueueBytesCapacity,
		DroppedTotal: snapshot.DroppedTotal, Connections: snapshot.Connections,
		UDPReceived: snapshot.UDP.ReceivedTotal, UDPConnections: snapshot.UDP.Connections,
		TCPReceived: snapshot.TCP.ReceivedTotal, TCPConnections: snapshot.TCP.Connections,
		LastError: snapshot.LastError, LastDropAt: snapshot.LastDropAt,
	}, true
}
