package parsetest

import (
	"io"
	"strings"
	"testing"

	"geoatlas/internal/model"
	"geoatlas/internal/parser"
)

type fakeParser struct {
	byLine map[string]parser.ParseResult
}

func (f fakeParser) ParseVerbose(line string) parser.ParseResult {
	if f.byLine != nil {
		if r, ok := f.byLine[line]; ok {
			return r
		}
	}
	return parser.ParseResult{Reason: "nope"}
}

type fakeGeo map[string]model.GeoLookup

func (f fakeGeo) Lookup(ip string) model.GeoLookup {
	if lk, ok := f[ip]; ok {
		return lk
	}
	return model.GeoLookup{}
}

type fakeSamples map[string][]string

func (f fakeSamples) SamplesByVendor() map[string][]string { return f }

type nilSamples struct{}

func (nilSamples) SamplesByVendor() map[string][]string { return nil }

func TestSamplesGuards(t *testing.T) {
	var s *Service
	if out := s.Samples(); len(out) != 0 {
		t.Fatalf("%v", out)
	}
	s = New(nil, nil, nil)
	if out := s.Samples(); len(out) != 0 {
		t.Fatalf("%v", out)
	}
	s = New(nil, nil, nilSamples{})
	if out := s.Samples(); len(out) != 0 {
		t.Fatalf("%v", out)
	}
	s = New(nil, nil, fakeSamples{"cisco": {"line"}})
	out := s.Samples()
	if len(out["cisco"]) != 1 {
		t.Fatalf("%v", out)
	}
}

func TestRunNilParser(t *testing.T) {
	var s *Service
	res, err := s.Run(strings.NewReader("x"))
	if err != nil || res.MaxLines != MaxLines || len(res.Results) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	s = New(nil, nil, nil)
	res, err = s.Run(strings.NewReader("x"))
	if err != nil || len(res.Results) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestRunParsedSkippedErrorAndGeo(t *testing.T) {
	p := fakeParser{byLine: map[string]parser.ParseResult{
		"ok": {
			OK: true, Vendor: "v",
			Log: model.TrafficLog{
				SrcIP: "1.1.1.1", DstIP: "8.8.8.8",
				SrcPort: 1, DstPort: 443, Action: "allow", Proto: "tcp",
			},
		},
		"skip": {Skipped: true, Vendor: "v", Reason: "drop"},
		"skip2": {Skipped: true, Vendor: "fw"},
		"bad":  {Reason: "parse fail"},
	}}
	geo := fakeGeo{
		"1.1.1.1": {Found: true, Country: "AU", Lat: 1, Lon: 2},
		"8.8.8.8": {Found: true, Country: "US", Lat: 3, Lon: 4},
	}
	s := New(p, geo, nil)
	input := "\n  \nok\nskip\nskip2\nbad\n"
	res, err := s.Run(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if res.Received != 4 || res.Parsed != 1 || res.Skipped != 2 || res.Errors != 1 {
		t.Fatalf("%+v", res)
	}
	if res.Results[0].SrcCountry != "AU" || res.Results[0].DstCountry != "US" {
		t.Fatalf("%+v", res.Results[0])
	}
	if res.Results[1].Reason != "drop" {
		t.Fatalf("%+v", res.Results[1])
	}
	if !strings.Contains(res.Results[2].Reason, "пропущено") {
		t.Fatalf("%+v", res.Results[2])
	}
	if res.Results[3].Reason != "parse fail" {
		t.Fatalf("%+v", res.Results[3])
	}
}

func TestRunWithoutGeo(t *testing.T) {
	p := fakeParser{byLine: map[string]parser.ParseResult{
		"ok": {OK: true, Vendor: "v", Log: model.TrafficLog{SrcIP: "1.1.1.1", DstIP: "2.2.2.2"}},
	}}
	s := New(p, nil, nil)
	res, err := s.Run(strings.NewReader("ok"))
	if err != nil || res.Parsed != 1 || res.Results[0].SrcCountry != "" {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestRunTruncatesAtMaxLines(t *testing.T) {
	s := New(countingParser{}, nil, nil)
	var sb strings.Builder
	for i := 0; i < MaxLines+3; i++ {
		sb.WriteString("row ")
		sb.WriteByte(byte('a' + i%26))
		sb.WriteByte(byte('0' + i%10))
		sb.WriteByte('\n')
	}
	res, err := s.Run(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || res.Received != MaxLines {
		t.Fatalf("%+v", res)
	}
}

type countingParser struct{}

func (countingParser) ParseVerbose(line string) parser.ParseResult {
	return parser.ParseResult{OK: true, Vendor: "v", Log: model.TrafficLog{SrcIP: "1.1.1.1", DstIP: "2.2.2.2", Action: "allow"}}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestRunScannerError(t *testing.T) {
	s := New(countingParser{}, nil, nil)
	_, err := s.Run(errReader{})
	if err == nil {
		t.Fatal("want scanner err")
	}
}
