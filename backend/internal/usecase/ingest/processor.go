package ingest

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"network_monitor/internal/model"
)

const (
	maxParseErrorRawBytes = 16 * 1024
	maxParseErrorBufBytes = 32 * 1024 * 1024
)

// Processor разбирает строки логов и пакетно пишет через порты вставки.
type Processor struct {
	logs          TrafficLogInserter
	errors        ParseErrorInserter
	parser        LineParser
	batchSize     int
	queryTimeout  time.Duration
	stats         LineStats
	geo           GeoLookup
	enrichCountry bool
	insertObs     InsertObserver
	retryable     InsertErrorClassifier

	buf         []model.TrafficLog
	errBuf      []model.ParseError
	errBufBytes int

	circuit *CircuitBreaker
}

func NewProcessor(d Deps, stats LineStats) *Processor {
	batch := d.BatchSize
	if batch <= 0 {
		batch = 10000
	}
	circuit := d.Circuit
	if circuit == nil {
		circuit = NewCircuitBreaker()
	}
	return &Processor{
		logs:          d.Logs,
		errors:        d.Errors,
		parser:        d.Parser,
		batchSize:     batch,
		queryTimeout:  d.QueryTimeout,
		stats:         stats,
		geo:           d.Geo,
		enrichCountry: d.EnrichCountry,
		insertObs:     d.InsertObs,
		retryable:     d.Retryable,
		buf:           make([]model.TrafficLog, 0, batch),
		errBuf:        make([]model.ParseError, 0, 256),
		circuit:       circuit,
	}
}

// LineOutcome — результат обработки одной строки.
type LineOutcome int

const (
	OutcomeEmpty LineOutcome = iota
	OutcomeParsed
	OutcomeSkipped
	OutcomeParseError
)

func (p *Processor) ProcessLine(ctx context.Context, line string, transport string) (LineOutcome, int, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return OutcomeEmpty, 0, nil
	}

	if p.stats != nil {
		p.stats.AddReceived(transport)
	}

	res := p.parser.ParseVerbose(line)
	if !res.OK {
		if res.Skipped {
			if p.stats != nil {
				p.stats.AddSkipped(1)
			}
			return OutcomeSkipped, 0, nil
		}
		if p.parser == nil || !p.parser.ContainsIPv4(line) {
			if p.stats != nil {
				p.stats.AddSkipped(1)
			}
			return OutcomeSkipped, 0, nil
		}
		if p.stats != nil {
			p.stats.AddParseErrors(1)
		}
		p.appendParseError(model.ParseError{
			Timestamp: time.Now(),
			Vendor:    res.Vendor,
			Reason:    res.Reason,
			Raw:       line,
		})
		if len(p.errBuf) >= p.batchSize {
			if err := p.flushErrors(ctx); err != nil {
				return OutcomeParseError, 0, err
			}
		}
		return OutcomeParseError, 0, nil
	}

	if p.stats != nil {
		p.stats.AddParsed(1)
	}
	logEntry := res.Log
	if logEntry.ParsedAt.IsZero() {
		logEntry.ParsedAt = time.Now()
	}
	p.enrichGeo(&logEntry)
	p.appendTrafficLog(logEntry)

	if len(p.buf) >= p.batchSize {
		n, err := p.Flush(ctx)
		return OutcomeParsed, n, err
	}
	return OutcomeParsed, 0, nil
}

func (p *Processor) enrichGeo(logEntry *model.TrafficLog) {
	if p.geo == nil {
		return
	}
	enrichSide := func(ip string, country, city, region *string, lat, lon *float64) {
		lk := p.geo.Lookup(ip)
		if !lk.Found {
			return
		}
		if city != nil && strings.TrimSpace(*city) == "" && strings.TrimSpace(lk.City) != "" {
			*city = lk.City
		}
		if region != nil && strings.TrimSpace(*region) == "" && strings.TrimSpace(lk.Region) != "" {
			*region = lk.Region
		}
		if lat != nil && lon != nil && *lat == 0 && *lon == 0 && (lk.Lat != 0 || lk.Lon != 0) {
			*lat, *lon = lk.Lat, lk.Lon
		}
		if p.enrichCountry && country != nil && model.NeedsCountry(*country) && model.UsableCountry(lk.Country) {
			*country = lk.Country
		}
	}
	enrichSide(logEntry.SrcIP, &logEntry.SrcCountry, &logEntry.SrcCity, &logEntry.SrcRegion, &logEntry.SrcLat, &logEntry.SrcLon)
	enrichSide(logEntry.DstIP, &logEntry.DstCountry, &logEntry.DstCity, &logEntry.DstRegion, &logEntry.DstLat, &logEntry.DstLon)
}

func (p *Processor) Flush(ctx context.Context) (int, error) {
	n := len(p.buf)
	if err := p.flushLogs(ctx); err != nil {
		return 0, err
	}
	if err := p.flushErrors(ctx); err != nil {
		return n, err
	}
	return n, nil
}

func (p *Processor) flushLogs(ctx context.Context) error {
	if len(p.buf) == 0 {
		return nil
	}
	if err := p.checkInsertCircuit(); err != nil {
		return err
	}

	n := len(p.buf)
	start := time.Now()
	err := insertWithRetry(ctx, p.queryTimeout, "traffic_logs", p.retryable, func(actx context.Context) error {
		if p.logs == nil {
			return nil
		}
		return p.logs.InsertTrafficLogs(actx, p.buf)
	})
	if p.insertObs != nil {
		p.insertObs.ObserveInsert(time.Since(start), n, err == nil)
	}
	if err != nil {
		p.noteInsertFailure()
		return err
	}
	p.noteInsertSuccess()

	if p.stats != nil {
		p.stats.AddInserted(int64(n))
		p.stats.AddBuffered(-int64(n))
		p.stats.SetLastFlushAt(time.Now())
	}
	p.buf = p.buf[:0]
	return nil
}

func (p *Processor) flushErrors(ctx context.Context) error {
	if len(p.errBuf) == 0 {
		return nil
	}
	if err := p.checkInsertCircuit(); err != nil {
		return err
	}
	err := insertWithRetry(ctx, p.queryTimeout, "parse_errors", p.retryable, func(actx context.Context) error {
		if p.errors == nil {
			return nil
		}
		return p.errors.InsertParseErrors(actx, p.errBuf)
	})
	if err != nil {
		p.noteInsertFailure()
		slog.Error("ingest: insert parse_errors failed", "err", err)
		return err
	}
	p.noteInsertSuccess()
	p.errBuf = p.errBuf[:0]
	p.errBufBytes = 0
	return nil
}

func (p *Processor) checkInsertCircuit() error {
	if p == nil {
		return nil
	}
	return p.circuit.Check()
}

func (p *Processor) noteInsertSuccess() {
	if p == nil {
		return
	}
	p.circuit.NoteSuccess()
}

func (p *Processor) noteInsertFailure() {
	if p == nil {
		return
	}
	p.circuit.NoteFailure()
}

func (p *Processor) maxIngestBuf() int {
	n := p.batchSize * 5
	if n < 1000 {
		n = 1000
	}
	const hardCap = 50000
	if n > hardCap {
		n = hardCap
	}
	return n
}

func (p *Processor) maxParseErrorBuf() int { return p.maxIngestBuf() }
func (p *Processor) maxTrafficBuf() int    { return p.maxIngestBuf() }

func (p *Processor) appendTrafficLog(entry model.TrafficLog) {
	entry.Raw = ""
	max := p.maxTrafficBuf()
	if len(p.buf) >= max {
		drop := len(p.buf) - max + 1
		if drop < 1 {
			drop = 1
		}
		p.buf = append(p.buf[:0], p.buf[drop:]...)
		if p.stats != nil {
			p.stats.AddBuffered(-int64(drop))
			p.stats.AddBufferDropped(int64(drop))
		}
		slog.Warn("ingest: traffic buffer at capacity, dropping oldest",
			"dropped", drop, "cap", max)
	}
	p.buf = append(p.buf, entry)
	if p.stats != nil {
		p.stats.AddBuffered(1)
	}
}

func truncateParseErrorRaw(s string) string {
	if len(s) <= maxParseErrorRawBytes {
		return s
	}
	return s[:maxParseErrorRawBytes]
}

func (p *Processor) dropOldestParseErrors(n int) {
	if n < 1 || len(p.errBuf) == 0 {
		return
	}
	if n > len(p.errBuf) {
		n = len(p.errBuf)
	}
	for i := 0; i < n; i++ {
		p.errBufBytes -= len(p.errBuf[i].Raw)
	}
	if p.errBufBytes < 0 {
		p.errBufBytes = 0
	}
	p.errBuf = append(p.errBuf[:0], p.errBuf[n:]...)
}

func (p *Processor) appendParseError(pe model.ParseError) {
	pe.Raw = truncateParseErrorRaw(pe.Raw)
	max := p.maxParseErrorBuf()
	dropped := 0
	for len(p.errBuf) >= max {
		n := len(p.errBuf) - max + 1
		p.dropOldestParseErrors(n)
		dropped += n
	}
	for p.errBufBytes+len(pe.Raw) > maxParseErrorBufBytes && len(p.errBuf) > 0 {
		p.dropOldestParseErrors(1)
		dropped++
	}
	if dropped > 0 {
		if p.stats != nil {
			p.stats.AddBufferDropped(int64(dropped))
		}
		slog.Warn("ingest: parse error buffer at capacity, dropping oldest",
			"dropped", dropped, "cap_entries", max, "cap_bytes", maxParseErrorBufBytes)
	}
	p.errBuf = append(p.errBuf, pe)
	p.errBufBytes += len(pe.Raw)
}

// SeedParseError / ForceFlush* — тестовые швы.
func (p *Processor) SeedParseError(pe model.ParseError) { p.appendParseError(pe) }
func (p *Processor) ForceFlushErrors(ctx context.Context) error {
	return p.flushErrors(ctx)
}
func (p *Processor) ForceFlushLogs(ctx context.Context) error { return p.flushLogs(ctx) }
func (p *Processor) MaxParseErrorBuf() int                    { return p.maxParseErrorBuf() }
func (p *Processor) MaxTrafficBuf() int                       { return p.maxTrafficBuf() }
func (p *Processor) EnrichGeo(logEntry *model.TrafficLog)     { p.enrichGeo(logEntry) }

func (p *Processor) ErrBufLen() int { return len(p.errBuf) }
func (p *Processor) BufLen() int    { return len(p.buf) }
func (p *Processor) ErrBufRawLen(i int) int {
	if i < 0 || i >= len(p.errBuf) {
		return -1
	}
	return len(p.errBuf[i].Raw)
}
