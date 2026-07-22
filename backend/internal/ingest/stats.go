package ingest

import (
	"sync/atomic"
	"time"
)

// TransportStats — счётчики приёма по транспорту (udp/tcp на входе syslog-ng).
type TransportStats struct {
	ReceivedTotal int64 `json:"received_total"`
	Connections   int64 `json:"connections"`
}

// StatsSnapshot — снимок метрик live-ingest для API и мониторинга.
type StatsSnapshot struct {
	State            string         `json:"state"`
	ReceivedTotal    int64          `json:"received_total"`
	ParsedTotal      int64          `json:"parsed_total"`
	InsertedTotal    int64          `json:"inserted_total"`
	SkippedTotal     int64          `json:"skipped_total"`
	ParseErrorsTotal int64          `json:"parse_errors_total"`
	BufferedLines    int64          `json:"buffered_lines"`
	QueueDepth       int64          `json:"queue_depth"`
	QueueCapacity    int64          `json:"queue_capacity"`
	DroppedTotal     int64          `json:"dropped_total"`
	Connections      int64          `json:"connections"`
	UDP              TransportStats `json:"udp"`
	TCP              TransportStats `json:"tcp"`
	LastFlushAt      string         `json:"last_flush_at,omitempty"`
	LastError        string         `json:"last_error,omitempty"`
}

type transportStats struct {
	receivedTotal atomic.Int64
	connections   atomic.Int64
}

func (t *transportStats) snapshot() TransportStats {
	return TransportStats{
		ReceivedTotal: t.receivedTotal.Load(),
		Connections:   t.connections.Load(),
	}
}

type stats struct {
	state            atomic.Value // string
	receivedTotal    atomic.Int64
	parsedTotal      atomic.Int64
	insertedTotal    atomic.Int64
	skippedTotal     atomic.Int64
	parseErrorsTotal atomic.Int64
	bufferedLines    atomic.Int64
	droppedTotal     atomic.Int64
	connections      atomic.Int64
	udp              transportStats
	tcp              transportStats
	lastFlushAt      atomic.Value // string
	lastError        atomic.Value // string
}

func newStats() *stats {
	s := &stats{}
	s.state.Store("idle")
	return s
}

func (s *stats) setState(v string) { s.state.Store(v) }

func (s *stats) setLastError(msg string) {
	if msg == "" {
		s.lastError.Store("")
		return
	}
	s.lastError.Store(msg)
}

func (s *stats) setLastFlushAt(t time.Time) {
	if t.IsZero() {
		s.lastFlushAt.Store("")
		return
	}
	s.lastFlushAt.Store(t.UTC().Format(time.RFC3339))
}

func (s *stats) addReceived(transport string) {
	s.receivedTotal.Add(1)
	switch transport {
	case "udp":
		s.udp.receivedTotal.Add(1)
	case "tcp":
		s.tcp.receivedTotal.Add(1)
	}
}

// LineStats (usecase/ingest) adapters.
func (s *stats) AddReceived(transport string) { s.addReceived(transport) }
func (s *stats) AddSkipped(n int64)           { s.skippedTotal.Add(n) }
func (s *stats) AddParsed(n int64)            { s.parsedTotal.Add(n) }
func (s *stats) AddParseErrors(n int64)       { s.parseErrorsTotal.Add(n) }
func (s *stats) AddInserted(n int64)          { s.insertedTotal.Add(n) }
func (s *stats) AddBuffered(delta int64)      { s.addBuffered(delta) }
func (s *stats) SetLastFlushAt(t time.Time)   { s.setLastFlushAt(t) }

func (s *stats) addConnection(transport string, delta int64) {
	s.connections.Add(delta)
	switch transport {
	case "udp":
		s.udp.connections.Add(delta)
	case "tcp":
		s.tcp.connections.Add(delta)
	}
}

func (s *stats) addBuffered(delta int64) {
	s.bufferedLines.Add(delta)
}

func (s *stats) addDropped(n int64) {
	s.droppedTotal.Add(n)
}

func (s *stats) snapshot() StatsSnapshot {
	state, _ := s.state.Load().(string)
	lastFlush, _ := s.lastFlushAt.Load().(string)
	lastErr, _ := s.lastError.Load().(string)
	return StatsSnapshot{
		State:            state,
		ReceivedTotal:    s.receivedTotal.Load(),
		ParsedTotal:      s.parsedTotal.Load(),
		InsertedTotal:    s.insertedTotal.Load(),
		SkippedTotal:     s.skippedTotal.Load(),
		ParseErrorsTotal: s.parseErrorsTotal.Load(),
		BufferedLines:    s.bufferedLines.Load(),
		DroppedTotal:     s.droppedTotal.Load(),
		Connections:      s.connections.Load(),
		UDP:              s.udp.snapshot(),
		TCP:              s.tcp.snapshot(),
		LastFlushAt:      lastFlush,
		LastError:        lastErr,
	}
}
