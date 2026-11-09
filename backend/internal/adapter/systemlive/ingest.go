package systemlive

import (
	"geoatlas/internal/model"
	"geoatlas/internal/usecase/system"
)

type IngestStatsSource interface {
	Stats() model.IngestLiveStats
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
	out := system.IngestSnapshot{
		State:         snapshot.State,
		ReceivedTotal: snapshot.ReceivedTotal, ParsedTotal: snapshot.ParsedTotal,
		InsertedTotal: snapshot.InsertedTotal, SkippedTotal: snapshot.SkippedTotal,
		ParseErrorsTotal: snapshot.ParseErrorsTotal, BufferedLines: snapshot.BufferedLines,
		QueueDepth: snapshot.QueueDepth, QueueCapacity: snapshot.QueueCapacity,
		QueueBytes: snapshot.QueueBytes, QueueBytesCapacity: snapshot.QueueBytesCapacity,
		DroppedTotal: snapshot.DroppedTotal, BufferDropsTotal: snapshot.BufferDropsTotal,
		Connections: snapshot.Connections, CircuitOpen: snapshot.CircuitOpen,
		UDPReceived: snapshot.UDP.ReceivedTotal, UDPConnections: snapshot.UDP.Connections,
		TCPReceived: snapshot.TCP.ReceivedTotal, TCPConnections: snapshot.TCP.Connections,
		LastError: snapshot.LastError, LastDropAt: snapshot.LastDropAt,
	}
	if len(snapshot.Connectors) > 0 {
		out.Connectors = make([]system.IngestConnectorSnapshot, 0, len(snapshot.Connectors))
		for _, c := range snapshot.Connectors {
			out.Connectors = append(out.Connectors, system.IngestConnectorSnapshot{
				ID: c.ID, ReceivedTotal: c.ReceivedTotal, DroppedTotal: c.DroppedTotal,
				AuthRejectedTotal: c.AuthRejectedTotal, LastEventAt: c.LastEventAt,
			})
		}
	}
	return out, true
}
