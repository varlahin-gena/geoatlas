package ingestnet

import (
	"sync/atomic"
	"time"

	"network_monitor/internal/model"
)

type transportStats struct {
	receivedTotal atomic.Int64
	connections   atomic.Int64
}

func (t *transportStats) snapshot() model.IngestTransportStats {
	return model.IngestTransportStats{
		ReceivedTotal: t.receivedTotal.Load(),
		Connections:   t.connections.Load(),
	}
}

type stats struct {
	state            atomic.Value // string: idle|running (error выводится из activeErrors)
	activeErrors     atomic.Int64 // workers, у которых последний цикл завершился ошибкой
	receivedTotal    atomic.Int64
	parsedTotal      atomic.Int64
	insertedTotal    atomic.Int64
	skippedTotal     atomic.Int64
	parseErrorsTotal atomic.Int64
	bufferedLines    atomic.Int64
	droppedTotal     atomic.Int64
	bufferDropsTotal atomic.Int64
	authRejected     atomic.Int64
	lastDropAtUnix   atomic.Int64 // unix nano; 0 = never
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

// noteWorkerError / noteWorkerOK — агрегированный health без last-writer-wins flicker:
// state=error, пока хотя бы один worker в ошибке.
func (s *stats) noteWorkerError(msg string) {
	s.activeErrors.Add(1)
	s.setLastError(msg)
}

func (s *stats) noteWorkerOK() {
	for {
		cur := s.activeErrors.Load()
		if cur <= 0 {
			s.setLastError("")
			return
		}
		if s.activeErrors.CompareAndSwap(cur, cur-1) {
			if cur-1 == 0 {
				s.setLastError("")
			}
			return
		}
	}
}

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
func (s *stats) AddBufferDropped(n int64)     { s.addBufferDropped(n) }
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
	s.lastDropAtUnix.Store(time.Now().UnixNano())
}

func (s *stats) addBufferDropped(n int64) {
	if n <= 0 {
		return
	}
	s.bufferDropsTotal.Add(n)
	s.lastDropAtUnix.Store(time.Now().UnixNano())
}

func (s *stats) snapshot() model.IngestLiveStats {
	state, _ := s.state.Load().(string)
	if s.activeErrors.Load() > 0 {
		state = "error"
	}
	lastFlush, _ := s.lastFlushAt.Load().(string)
	lastErr, _ := s.lastError.Load().(string)
	var lastDrop string
	if ns := s.lastDropAtUnix.Load(); ns > 0 {
		lastDrop = time.Unix(0, ns).UTC().Format(time.RFC3339)
	}
	return model.IngestLiveStats{
		State:             state,
		ReceivedTotal:     s.receivedTotal.Load(),
		ParsedTotal:       s.parsedTotal.Load(),
		InsertedTotal:     s.insertedTotal.Load(),
		SkippedTotal:      s.skippedTotal.Load(),
		ParseErrorsTotal:  s.parseErrorsTotal.Load(),
		BufferedLines:     s.bufferedLines.Load(),
		DroppedTotal:      s.droppedTotal.Load(),
		BufferDropsTotal:  s.bufferDropsTotal.Load(),
		AuthRejectedTotal: s.authRejected.Load(),
		LastDropAt:        lastDrop,
		Connections:       s.connections.Load(),
		UDP:               s.udp.snapshot(),
		TCP:               s.tcp.snapshot(),
		LastFlushAt:       lastFlush,
		LastError:         lastErr,
	}
}
