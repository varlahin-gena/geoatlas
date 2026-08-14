package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"network_monitor/internal/model"
	"network_monitor/internal/parser"
)

type registryParser struct{ reg *parser.Registry }

func (r registryParser) ParseVerbose(line string) parser.ParseResult {
	return r.reg.ParseVerbose(line)
}

func (r registryParser) ContainsIPv4(line string) bool {
	return parser.ContainsIPv4(line)
}

func testParsers() LineParser {
	return registryParser{reg: parser.NewRegistry(
		&parser.UserGateCEF{},
		&parser.FortigateCEF{},
		&parser.CiscoFTD{},
		&parser.CiscoASA{},
		&parser.CowrieJSON{},
		&parser.GenericKV{},
	)}
}

type stubInserter struct {
	trafficErr error
	parseErr   error
	trafficN   int
	parseN     int
}

func (s *stubInserter) InsertTrafficLogs(ctx context.Context, logs []model.TrafficLog) error {
	s.trafficN += len(logs)
	return s.trafficErr
}

func (s *stubInserter) InsertParseErrors(ctx context.Context, items []model.ParseError) error {
	s.parseN += len(items)
	return s.parseErr
}

type memStats struct {
	received, skipped, parsed, parseErrs, inserted, buffered, bufferDrops int64
}

func (m *memStats) AddReceived(string)         { m.received++ }
func (m *memStats) AddSkipped(n int64)         { m.skipped += n }
func (m *memStats) AddParsed(n int64)          { m.parsed += n }
func (m *memStats) AddParseErrors(n int64)     { m.parseErrs += n }
func (m *memStats) AddInserted(n int64)        { m.inserted += n }
func (m *memStats) AddBuffered(delta int64)    { m.buffered += delta }
func (m *memStats) AddBufferDropped(n int64)   { m.bufferDrops += n }
func (m *memStats) SetLastFlushAt(t time.Time) {}

func testDeps(ins *stubInserter) Deps {
	return Deps{
		Logs: ins, Errors: ins, Parser: testParsers(),
		BatchSize: 10000, QueryTimeout: time.Minute,
	}
}

func TestProcessorOutcomeStats(t *testing.T) {
	st := &memStats{}
	ins := &stubInserter{}
	proc := NewProcessor(testDeps(ins), st)
	outcome, _, err := proc.ProcessLine(context.Background(), "mystery event from 203.0.113.10 to nowhere", "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if outcome != OutcomeParseError {
		t.Fatalf("outcome = %v", outcome)
	}
	if st.received != 1 || st.parseErrs != 1 || st.skipped != 0 {
		t.Fatalf("stats=%+v", st)
	}
	if proc.ErrBufLen() != 1 {
		t.Fatalf("errBuf=%d", proc.ErrBufLen())
	}
}

func TestProcessorSkipsParseErrorWithoutIPv4(t *testing.T) {
	st := &memStats{}
	proc := NewProcessor(testDeps(&stubInserter{}), st)
	outcome, _, err := proc.ProcessLine(context.Background(), "not a log line at all", "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if outcome != OutcomeSkipped {
		t.Fatalf("outcome=%v", outcome)
	}
	if st.skipped != 1 || st.parseErrs != 0 || proc.ErrBufLen() != 0 {
		t.Fatalf("stats=%+v errBuf=%d", st, proc.ErrBufLen())
	}
}

type stubGeo map[string]model.GeoLookup

func (s stubGeo) Lookup(ip string) model.GeoLookup {
	if lk, ok := s[ip]; ok {
		return lk
	}
	return model.GeoLookup{}
}

func TestEnrichGeoFillsCountry(t *testing.T) {
	geo := stubGeo{
		"8.8.8.8": {Country: "United States", City: "Mountain View", Lat: 37.4, Lon: -122.1, Found: true},
	}
	d := testDeps(&stubInserter{})
	d.Geo = geo
	d.EnrichCountry = true
	proc := NewProcessor(d, nil)
	entry := model.TrafficLog{SrcIP: "10.0.0.1", DstIP: "8.8.8.8", DstCountry: ""}
	proc.EnrichGeo(&entry)
	if entry.DstCountry != "United States" {
		t.Fatalf("dst_country=%q", entry.DstCountry)
	}
}

func TestEnrichGeoCountryOptOut(t *testing.T) {
	geo := stubGeo{
		"8.8.8.8": {Country: "United States", City: "Mountain View", Lat: 37.4, Lon: -122.1, Found: true},
	}
	d := testDeps(&stubInserter{})
	d.Geo = geo
	d.EnrichCountry = false
	proc := NewProcessor(d, nil)
	entry := model.TrafficLog{SrcIP: "10.0.0.1", DstIP: "8.8.8.8", DstCountry: ""}
	proc.EnrichGeo(&entry)
	if entry.DstCountry != "" {
		t.Fatalf("want empty country, got %q", entry.DstCountry)
	}
	if entry.DstCity != "Mountain View" {
		t.Fatalf("city=%q", entry.DstCity)
	}
}

func TestFlushErrorsKeepsBufferOnInsertFailure(t *testing.T) {
	oldA := insertRetryAttempts
	insertRetryAttempts = 1
	t.Cleanup(func() { insertRetryAttempts = oldA })

	ins := &stubInserter{parseErr: errors.New("clickhouse down")}
	proc := NewProcessor(testDeps(ins), nil)
	proc.SeedParseError(model.ParseError{Timestamp: time.Now(), Vendor: "test", Reason: "bad", Raw: "raw"})
	if err := proc.ForceFlushErrors(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if proc.ErrBufLen() != 1 {
		t.Fatalf("errBuf=%d", proc.ErrBufLen())
	}
	ins.parseErr = nil
	if err := proc.ForceFlushErrors(context.Background()); err != nil {
		t.Fatal(err)
	}
	if proc.ErrBufLen() != 0 {
		t.Fatalf("errBuf=%d after success", proc.ErrBufLen())
	}
}

func TestProcessorErrBufCapDropsOldest(t *testing.T) {
	d := testDeps(&stubInserter{})
	d.BatchSize = 2
	proc := NewProcessor(d, nil)
	for i := 0; i < proc.MaxParseErrorBuf()+5; i++ {
		proc.SeedParseError(model.ParseError{Reason: "x", Raw: "line"})
	}
	if got := proc.ErrBufLen(); got != proc.MaxParseErrorBuf() {
		t.Fatalf("errBuf=%d want %d", got, proc.MaxParseErrorBuf())
	}
}

func TestProcessorParseErrorRawTruncated(t *testing.T) {
	proc := NewProcessor(testDeps(&stubInserter{}), nil)
	raw := strings.Repeat("z", maxParseErrorRawBytes+4096)
	proc.SeedParseError(model.ParseError{Reason: "x", Raw: raw})
	if got := proc.ErrBufRawLen(0); got != maxParseErrorRawBytes {
		t.Fatalf("raw len=%d", got)
	}
}

func TestFlushLogsKeepsBufferOnInsertFailure(t *testing.T) {
	oldA := insertRetryAttempts
	insertRetryAttempts = 1
	t.Cleanup(func() { insertRetryAttempts = oldA })

	ins := &stubInserter{trafficErr: errors.New("clickhouse down")}
	st := &memStats{}
	proc := NewProcessor(testDeps(ins), st)
	line := `src=10.0.0.1 dst=8.8.8.8 action=allow proto=tcp sport=12345 dport=443`
	if _, _, err := proc.ProcessLine(context.Background(), line, ""); err != nil {
		t.Fatal(err)
	}
	if err := proc.ForceFlushLogs(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if proc.BufLen() != 1 || st.buffered != 1 {
		t.Fatalf("buf=%d buffered=%d", proc.BufLen(), st.buffered)
	}
}

func TestProcessorTrafficBufCapDropsOldest(t *testing.T) {
	oldA := insertRetryAttempts
	insertRetryAttempts = 1
	t.Cleanup(func() { insertRetryAttempts = oldA })

	ins := &stubInserter{trafficErr: errors.New("clickhouse down")}
	st := &memStats{}
	d := testDeps(ins)
	d.BatchSize = 2
	proc := NewProcessor(d, st)
	max := proc.MaxTrafficBuf()
	line := `src=10.0.0.1 dst=8.8.8.8 action=allow proto=tcp sport=12345 dport=443`
	for i := 0; i < max+5; i++ {
		_, _, _ = proc.ProcessLine(context.Background(), line, "")
	}
	if got := proc.BufLen(); got != max {
		t.Fatalf("buf=%d want %d", got, max)
	}
	if st.buffered != int64(max) {
		t.Fatalf("buffered=%d want %d", st.buffered, max)
	}
	if st.bufferDrops < 5 {
		t.Fatalf("bufferDrops=%d want >=5", st.bufferDrops)
	}
}

func TestProcessorErrBufCapCountsBufferDrops(t *testing.T) {
	d := testDeps(&stubInserter{})
	d.BatchSize = 2
	st := &memStats{}
	proc := NewProcessor(d, st)
	extra := 5
	for i := 0; i < proc.MaxParseErrorBuf()+extra; i++ {
		proc.SeedParseError(model.ParseError{Reason: "x", Raw: "line"})
	}
	if st.bufferDrops < int64(extra) {
		t.Fatalf("bufferDrops=%d want >=%d", st.bufferDrops, extra)
	}
}
