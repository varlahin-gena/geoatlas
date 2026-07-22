package ingest

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"network_monitor/internal/metrics"
	usecaseingest "network_monitor/internal/usecase/ingest"
)

type Binding struct {
	Addr      string
	Transport string // "udp" or "tcp" — источник на syslog-ng (514/udp vs 514/tcp)
}

type Config struct {
	Bindings      []Binding
	ListenAddr    string // legacy: один listener без разделения транспорта
	BatchSize     int
	FlushInterval time.Duration
	QueueSize     int
	Workers       int
	QueryTimeout  time.Duration

	MaxConnections  int
	ConnIdleTimeout time.Duration
}

type ingestedLine struct {
	line      string
	transport string
}

// ProcessorDeps — порты для usecase Processor (без clickhouse.Conn).
type ProcessorDeps struct {
	Logs          usecaseingest.TrafficLogInserter
	Errors        usecaseingest.ParseErrorInserter
	Parser        usecaseingest.LineParser
	Geo           usecaseingest.GeoLookup
	EnrichCountry bool
}

type Service struct {
	cfg        Config
	procDeps   ProcessorDeps
	stats      *stats
	processors []*usecaseingest.Processor

	lineCh     chan ingestedLine
	connSem    chan struct{}
	activeConn atomic.Int64
	wg         sync.WaitGroup
}

func NewService(cfg Config, deps ProcessorDeps) *Service {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 3 * time.Second
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 300000
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 256
	}
	if cfg.ConnIdleTimeout <= 0 {
		cfg.ConnIdleTimeout = 5 * time.Minute
	}
	if len(cfg.Bindings) == 0 && cfg.ListenAddr != "" {
		cfg.Bindings = []Binding{{Addr: cfg.ListenAddr, Transport: ""}}
	}

	st := newStats()
	s := &Service{
		cfg:      cfg,
		procDeps: deps,
		stats:    st,
		lineCh:   make(chan ingestedLine, cfg.QueueSize),
		connSem:  make(chan struct{}, cfg.MaxConnections),
	}
	ucDeps := usecaseingest.Deps{
		Logs: deps.Logs, Errors: deps.Errors, Parser: deps.Parser,
		Geo: deps.Geo, EnrichCountry: deps.EnrichCountry,
		BatchSize: cfg.BatchSize, QueryTimeout: cfg.QueryTimeout,
	}
	s.processors = make([]*usecaseingest.Processor, cfg.Workers)
	for i := range s.processors {
		s.processors[i] = usecaseingest.NewProcessor(ucDeps, st)
	}
	metrics.IngestQueueCapacity.Set(float64(cfg.QueueSize))
	return s
}

func (s *Service) Stats() StatsSnapshot {
	snap := s.stats.snapshot()
	if s.lineCh != nil {
		snap.QueueDepth = int64(len(s.lineCh))
		snap.QueueCapacity = int64(cap(s.lineCh))
		metrics.IngestQueueDepth.Set(float64(snap.QueueDepth))
		metrics.IngestQueueCapacity.Set(float64(snap.QueueCapacity))
	}
	return snap
}

func (s *Service) Run(ctx context.Context) error {
	for i := range s.processors {
		s.wg.Add(1)
		go s.worker(ctx, s.processors[i])
	}

	if len(s.cfg.Bindings) == 0 {
		slog.Info("ingest: HTTP-only mode (no listen bindings configured)")
		s.stats.setState("running")
		<-ctx.Done()
		s.wg.Wait()
		slog.Info("ingest: stopped")
		return nil
	}

	var listeners []net.Listener
	defer func() {
		for _, ln := range listeners {
			_ = ln.Close()
		}
	}()

	var acceptWg sync.WaitGroup
	for _, binding := range s.cfg.Bindings {
		binding := binding
		ln, err := net.Listen("tcp", binding.Addr)
		if err != nil {
			return err
		}
		listeners = append(listeners, ln)

		label := binding.Addr
		if binding.Transport != "" {
			label = binding.Transport + "@" + binding.Addr
		}
		slog.Info("ingest: listening",
			"addr", label,
			"workers", s.cfg.Workers,
			"batch", s.cfg.BatchSize,
			"queue", s.cfg.QueueSize,
			"max_connections", s.cfg.MaxConnections,
			"conn_idle", s.cfg.ConnIdleTimeout.String(),
		)

		acceptWg.Add(1)
		go func(ln net.Listener, transport string) {
			defer acceptWg.Done()

			for {
				conn, err := ln.Accept()
				if err != nil {
					select {
					case <-ctx.Done():
						return
					default:
						slog.Warn("ingest: accept error", "addr", ln.Addr().String(), "err", err)
						continue
					}
				}
				select {
				case s.connSem <- struct{}{}:
					s.wg.Add(1)
					go s.handleConn(ctx, conn, transport)
				default:
					remote := conn.RemoteAddr().String()
					_ = conn.Close()
					slog.Warn("ingest: max connections reached, rejecting",
						"remote", remote,
						"limit", s.cfg.MaxConnections,
						"active", s.activeConn.Load(),
					)
				}
			}
		}(ln, binding.Transport)
	}

	s.stats.setState("running")
	<-ctx.Done()
	for _, ln := range listeners {
		_ = ln.Close()
	}
	acceptWg.Wait()
	s.wg.Wait()
	slog.Info("ingest: stopped")
	return nil
}

func (s *Service) drainTimeout() time.Duration {
	if s.cfg.QueryTimeout > 0 {
		return s.cfg.QueryTimeout
	}
	return 30 * time.Second
}

func detachTimeout(_ context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func (s *Service) handleConn(ctx context.Context, conn net.Conn, transport string) {
	defer s.wg.Done()
	defer func() { <-s.connSem }()
	defer conn.Close()

	s.activeConn.Add(1)
	defer s.activeConn.Add(-1)

	s.stats.addConnection(transport, 1)
	defer s.stats.addConnection(transport, -1)

	remote := conn.RemoteAddr().String()
	slog.Info("ingest: connection opened", "remote", remote, "transport", transportOrUnknown(transport))

	reader := newFrameReader(conn)
	var linesOnConn int64
	idle := s.cfg.ConnIdleTimeout

	for {
		if idle > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(idle))
		}
		line, err := reader.ReadLine()
		if err != nil {
			if errors.Is(err, errFrameTooLarge) {
				metrics.IngestFrameTooLargeTotal.Inc()
				slog.Warn("ingest: frame too large, closing connection",
					"remote", remote, "max_bytes", maxFrameBytes, "lines", linesOnConn)
			} else if err != io.EOF && !isClosedConn(err) && !isTimeout(err) {
				slog.Warn("ingest: read error", "remote", remote, "err", err, "lines", linesOnConn)
			} else if isTimeout(err) {
				slog.Info("ingest: idle timeout", "remote", remote, "idle", idle.String(), "lines", linesOnConn)
			}
			break
		}
		linesOnConn++
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !s.TryEnqueue(line, transport) {
			if s.stats.droppedTotal.Load()%1000 == 1 {
				slog.Warn("ingest: queue full, dropping line",
					"remote", remote,
					"transport", transportOrUnknown(transport),
					"queue", cap(s.lineCh),
					"dropped_total", s.stats.droppedTotal.Load(),
				)
			}
		}
	}
	slog.Info("ingest: connection closed", "remote", remote, "transport", transportOrUnknown(transport), "lines", linesOnConn)
}

func transportOrUnknown(transport string) string {
	if transport == "" {
		return "unknown"
	}
	return transport
}

func isClosedConn(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "connection reset")
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "i/o timeout")
}

func (s *Service) worker(ctx context.Context, proc *usecaseingest.Processor) {
	defer s.wg.Done()

	t := time.NewTicker(s.cfg.FlushInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			drainCtx, cancel := detachTimeout(ctx, s.drainTimeout())
			s.drainWorker(drainCtx, proc)
			cancel()
			return
		case item := <-s.lineCh:
			metrics.IngestQueueDepth.Set(float64(len(s.lineCh)))
			transport, line := ResolveTransport(item.line, item.transport)
			if _, _, err := proc.ProcessLine(ctx, line, transport); err != nil {
				s.stats.setState("error")
				s.stats.setLastError(err.Error())
				slog.Error("ingest: process error", "err", err)
				continue
			}
			s.stats.setState("running")
			s.stats.setLastError("")
		case <-t.C:
			if _, err := proc.Flush(ctx); err != nil {
				s.stats.setState("error")
				s.stats.setLastError(err.Error())
				slog.Error("ingest: flush error", "err", err)
				continue
			}
			s.stats.setState("running")
			s.stats.setLastError("")
		}
	}
}

func (s *Service) drainWorker(ctx context.Context, proc *usecaseingest.Processor) {
	for {
		select {
		case item := <-s.lineCh:
			transport, line := ResolveTransport(item.line, item.transport)
			if _, _, err := proc.ProcessLine(ctx, line, transport); err != nil {
				slog.Error("ingest: drain process error", "err", err)
			}
		default:
			if _, err := proc.Flush(ctx); err != nil {
				slog.Error("ingest: drain flush error", "err", err)
			}
			return
		}
	}
}
