package ingestnet

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"network_monitor/internal/model"
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
	// QueueMaxBytes — лимит суммы len(line) в очереди (default 256 MiB).
	// Drop при переполнении depth ИЛИ bytes.
	QueueMaxBytes int
	Workers       int
	QueryTimeout  time.Duration

	MaxConnections  int
	ConnIdleTimeout time.Duration

	// SharedSecret — токен в @@nm/{udp|tcp}/<token>/@@; пусто = legacy (только insecure/dev).
	SharedSecret string
	// AllowFrom — CSV peer allowlist; пусто = любой peer (не рекомендуется).
	AllowFrom string
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
	InsertObs     usecaseingest.InsertObserver
	Retryable     usecaseingest.InsertErrorClassifier
}

type Service struct {
	cfg        Config
	procDeps   ProcessorDeps
	stats      *stats
	processors []*usecaseingest.Processor
	circuit    *usecaseingest.CircuitBreaker
	peers      *PeerAllowlist

	lineCh      chan ingestedLine
	queueBytes  atomic.Int64
	connSem     chan struct{}
	activeConn  atomic.Int64
	lastDropLog atomic.Int64 // unix nano of last drop warn log
	wg          sync.WaitGroup

	// drainRoot отменяет in-flight drain при AbortDrain (до pools.Close).
	drainMu     sync.Mutex
	drainRoot   context.Context
	drainCancel context.CancelFunc
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
	if cfg.QueueMaxBytes <= 0 {
		cfg.QueueMaxBytes = 256 << 20 // 256 MiB
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
	circuit := usecaseingest.NewCircuitBreaker()
	s := &Service{
		cfg:      cfg,
		procDeps: deps,
		stats:    st,
		circuit:  circuit,
		peers:    NewPeerAllowlist(cfg.AllowFrom),
		lineCh:   make(chan ingestedLine, cfg.QueueSize),
		connSem:  make(chan struct{}, cfg.MaxConnections),
	}
	ucDeps := usecaseingest.Deps{
		Logs: deps.Logs, Errors: deps.Errors, Parser: deps.Parser,
		Geo: deps.Geo, EnrichCountry: deps.EnrichCountry,
		BatchSize: cfg.BatchSize, QueryTimeout: cfg.QueryTimeout,
		Circuit: circuit, InsertObs: deps.InsertObs, Retryable: deps.Retryable,
	}
	s.processors = make([]*usecaseingest.Processor, cfg.Workers)
	for i := range s.processors {
		s.processors[i] = usecaseingest.NewProcessor(ucDeps, st)
	}
	return s
}

func (s *Service) Stats() model.IngestLiveStats {
	snap := s.stats.snapshot()
	if s.lineCh != nil {
		snap.QueueDepth = int64(len(s.lineCh))
		snap.QueueCapacity = int64(cap(s.lineCh))
	}
	snap.QueueBytes = s.queueBytes.Load()
	snap.QueueBytesCapacity = int64(s.cfg.QueueMaxBytes)
	if s.circuit != nil {
		snap.CircuitOpen = s.circuit.Open()
	}
	return snap
}

func (s *Service) Run(ctx context.Context) error {
	// WithoutCancel: Run ctx отменяется на shutdown, но AbortDrain должен жить отдельно.
	drainRoot, drainCancel := context.WithCancel(context.WithoutCancel(ctx))
	s.drainMu.Lock()
	s.drainRoot = drainRoot
	s.drainCancel = drainCancel
	s.drainMu.Unlock()
	defer func() {
		drainCancel()
		s.drainMu.Lock()
		s.drainRoot = nil
		s.drainCancel = nil
		s.drainMu.Unlock()
	}()

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
			"queue_max_bytes", s.cfg.QueueMaxBytes,
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
				if s.peers != nil && !s.peers.Empty() && !s.peers.Allows(conn.RemoteAddr()) {
					remote := conn.RemoteAddr().String()
					_ = conn.Close()
					s.stats.authRejected.Add(1)
					slog.Warn("ingest: peer not allowed", "remote", remote)
					continue
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
	snap := s.Stats()
	slog.Info("ingest: shutting down",
		"queue_depth", snap.QueueDepth,
		"queue_capacity", snap.QueueCapacity,
		"queue_bytes", snap.QueueBytes,
		"queue_bytes_capacity", snap.QueueBytesCapacity,
		"drain_timeout", s.drainTimeout().String(),
		"dropped_total", snap.DroppedTotal,
	)
	for _, ln := range listeners {
		_ = ln.Close()
	}
	acceptWg.Wait()
	s.wg.Wait()
	left := int64(0)
	if s.lineCh != nil {
		left = int64(len(s.lineCh))
	}
	if left > 0 {
		slog.Warn("ingest: stopped with undrained queue", "queue_depth", left)
	} else {
		slog.Info("ingest: stopped")
	}
	return nil
}

func (s *Service) drainTimeout() time.Duration {
	// Не берём полный QueryTimeout (часто 3m) как бюджет drain — SIGTERM должен
	// укладываться в разумный maxDrain; insertBudget — оценка одного flush.
	insertBudget := 30 * time.Second
	if s.cfg.QueryTimeout > 0 && s.cfg.QueryTimeout < insertBudget {
		insertBudget = s.cfg.QueryTimeout
	}
	const maxDrain = 2 * time.Minute
	if s == nil || s.lineCh == nil {
		return insertBudget
	}
	depth := len(s.lineCh)
	if depth == 0 {
		return insertBudget
	}
	workers := s.cfg.Workers
	if workers < 1 {
		workers = 1
	}
	batch := s.cfg.BatchSize
	if batch < 1 {
		batch = 1
	}
	flush := s.cfg.FlushInterval
	if flush < time.Second {
		flush = time.Second
	}
	perCycle := workers * batch
	cycles := (depth + perCycle - 1) / perCycle
	estimated := time.Duration(cycles) * (flush + insertBudget)
	if estimated < insertBudget {
		return insertBudget
	}
	if estimated > maxDrain {
		return maxDrain
	}
	return estimated
}

// ShutdownWaitTimeout — бюджет ожидания для main после отмены ctx
// (не короче drainTimeout worker'ов + небольшой запас).
func (s *Service) ShutdownWaitTimeout() time.Duration {
	d := s.drainTimeout()
	const margin = 5 * time.Second
	const maxWait = 2*time.Minute + margin
	wait := d + margin
	if wait > maxWait {
		return maxWait
	}
	return wait
}

// AbortDrain отменяет in-flight drain contexts (после таймаута ожидания в main),
// чтобы workers не держали CH-соединения к моменту pools.Close.
func (s *Service) AbortDrain() {
	if s == nil {
		return
	}
	s.drainMu.Lock()
	cancel := s.drainCancel
	s.drainMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) beginDrain(ctx context.Context) (context.Context, context.CancelFunc) {
	// WithoutCancel: worker ctx уже Done на shutdown; drain дописывает очередь
	// до timeout или AbortDrain (через drainRoot).
	base := context.WithoutCancel(ctx)
	drainCtx, cancel := context.WithTimeout(base, s.drainTimeout())

	s.drainMu.Lock()
	root := s.drainRoot
	s.drainMu.Unlock()
	if root == nil {
		return drainCtx, cancel
	}
	stop := context.AfterFunc(root, cancel)
	return drainCtx, func() {
		stop()
		cancel()
	}
}

func (s *Service) noteQueueDrop(remote, transport string) {
	total := s.stats.droppedTotal.Load()
	now := time.Now().UnixNano()
	last := s.lastDropLog.Load()
	const quiet = int64(10 * time.Second)
	shouldLog := total == 1 || total%1000 == 1 || last == 0 || now-last >= quiet
	if !shouldLog {
		return
	}
	if !s.lastDropLog.CompareAndSwap(last, now) {
		// Другой writer уже залогировал — не спамим.
		return
	}
	slog.Warn("ingest: queue full, dropping line",
		"remote", remote,
		"transport", transportOrUnknown(transport),
		"queue_depth", len(s.lineCh),
		"queue_capacity", cap(s.lineCh),
		"queue_bytes", s.queueBytes.Load(),
		"queue_bytes_capacity", s.cfg.QueueMaxBytes,
		"dropped_total", total,
	)
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
			s.noteQueueDrop(remote, transport)
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
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, net.ErrClosed) {
			return true
		}
		err = opErr.Err
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.ECONNRESET || errno == syscall.EPIPE
	}
	// Fallback: WSAECONNRESET / обёртки без typed Errno.
	msg := err.Error()
	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "use of closed network connection")
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func (s *Service) worker(ctx context.Context, proc *usecaseingest.Processor) {
	defer s.wg.Done()

	t := time.NewTicker(s.cfg.FlushInterval)
	defer t.Stop()

	inErr := false
	noteErr := func(err error) {
		if !inErr {
			inErr = true
			s.stats.noteWorkerError(err.Error())
		} else {
			s.stats.setLastError(err.Error())
		}
	}
	noteOK := func() {
		if inErr {
			inErr = false
			s.stats.noteWorkerOK()
		}
	}

	for {
		// Пока insert circuit open — не забираем из очереди (иначе processor
		// буфер дропает oldest без учёта в dropped_total). Очередь растёт →
		// admission drops видны в DroppedTotal; flush по тикеру пробует half-open.
		if s.circuit != nil && s.circuit.Open() {
			wait := s.circuit.RemainingOpen()
			if wait <= 0 || wait > 200*time.Millisecond {
				wait = 200 * time.Millisecond
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				drainCtx, cancel := s.beginDrain(ctx)
				s.drainWorker(drainCtx, proc)
				cancel()
				return
			case <-t.C:
				timer.Stop()
				if _, err := proc.Flush(ctx); err != nil {
					noteErr(err)
					slog.Error("ingest: flush error", "err", err)
				} else {
					noteOK()
				}
			case <-timer.C:
			}
			continue
		}

		select {
		case <-ctx.Done():
			drainCtx, cancel := s.beginDrain(ctx)
			s.drainWorker(drainCtx, proc)
			cancel()
			return
		case item := <-s.lineCh:
			s.releaseQueueBytes(item.line)
			transport, line, ok := ResolveTransportAuth(item.line, item.transport, s.cfg.SharedSecret)
			if !ok {
				s.stats.authRejected.Add(1)
				continue
			}
			if _, _, err := proc.ProcessLine(ctx, line, transport); err != nil {
				noteErr(err)
				slog.Error("ingest: process error", "err", err)
				continue
			}
			noteOK()
		case <-t.C:
			if _, err := proc.Flush(ctx); err != nil {
				noteErr(err)
				slog.Error("ingest: flush error", "err", err)
				continue
			}
			noteOK()
		}
	}
}

func (s *Service) drainWorker(ctx context.Context, proc *usecaseingest.Processor) {
	for {
		if ctx.Err() != nil {
			return
		}
		if s.circuit != nil && s.circuit.Open() {
			wait := s.circuit.RemainingOpen()
			if wait <= 0 || wait > 200*time.Millisecond {
				wait = 200 * time.Millisecond
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			// После cooldown пробуем flush (half-open) прежде чем снова dequeue.
			if _, err := proc.Flush(ctx); err != nil {
				slog.Error("ingest: drain flush error", "err", err)
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case item := <-s.lineCh:
			s.releaseQueueBytes(item.line)
			transport, line, ok := ResolveTransportAuth(item.line, item.transport, s.cfg.SharedSecret)
			if !ok {
				s.stats.authRejected.Add(1)
				continue
			}
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

func (s *Service) releaseQueueBytes(line string) {
	if n := int64(len(line)); n > 0 {
		s.queueBytes.Add(-n)
	}
}
