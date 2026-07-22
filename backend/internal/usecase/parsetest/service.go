package parsetest

import (
	"bufio"
	"io"
	"strings"

	"network_monitor/internal/mapagg"
)

const MaxLines = 200 // защита от вставки огромного файла в тест

// Row — одна строка результата parse-test.
type Row struct {
	N          int    `json:"n"`
	Line       string `json:"line"`
	Parsed     bool   `json:"parsed"`
	Skipped    bool   `json:"skipped"`
	Vendor     string `json:"vendor,omitempty"`
	Reason     string `json:"reason,omitempty"`
	SrcIP      string `json:"src_ip,omitempty"`
	DstIP      string `json:"dst_ip,omitempty"`
	SrcPort    uint32 `json:"src_port,omitempty"`
	DstPort    uint32 `json:"dst_port,omitempty"`
	Action     string `json:"action,omitempty"`
	Proto      string `json:"proto,omitempty"`
	SrcCountry string `json:"src_country,omitempty"`
	DstCountry string `json:"dst_country,omitempty"`
}

// Result — сводка parse-test.
type Result struct {
	Received  int   `json:"received"`
	Parsed    int   `json:"parsed"`
	Skipped   int   `json:"skipped"`
	Errors    int   `json:"errors"`
	Truncated bool  `json:"truncated"`
	MaxLines  int   `json:"max_lines"`
	Results   []Row `json:"results"`
}

// Service — application use cases для /api/parse-test и /api/parse-samples.
type Service struct {
	parser  VerboseParser
	geo     GeoLookuper
	samples SamplesProvider
}

func New(parser VerboseParser, geo GeoLookuper, samples SamplesProvider) *Service {
	return &Service{parser: parser, geo: geo, samples: samples}
}

// Samples отдаёт пресеты строк по вендорам.
func (s *Service) Samples() map[string][]string {
	if s == nil || s.samples == nil {
		return map[string][]string{}
	}
	out := s.samples.SamplesByVendor()
	if out == nil {
		return map[string][]string{}
	}
	return out
}

// Run разбирает до MaxLines непустых строк из r.
func (s *Service) Run(r io.Reader) (Result, error) {
	if s == nil || s.parser == nil {
		return Result{MaxLines: MaxLines, Results: []Row{}}, nil
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	rows := make([]Row, 0, 32)
	var received, parsed, skipped int
	truncated := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if received >= MaxLines {
			truncated = true
			break
		}
		received++

		res := s.parser.ParseVerbose(line)
		row := Row{
			N: received, Line: line,
			Parsed: res.OK, Skipped: res.Skipped, Vendor: res.Vendor,
		}

		switch {
		case res.OK:
			parsed++
			tl := res.Log
			row.SrcIP = tl.SrcIP
			row.DstIP = tl.DstIP
			row.SrcPort = tl.SrcPort
			row.DstPort = tl.DstPort
			row.Action = string(tl.Action)
			row.Proto = tl.Proto

			if s.geo != nil {
				if m := mapagg.IPGroupMeta(s.geo, tl.SrcIP, "country"); m.Valid {
					row.SrcCountry = m.Key
				}
				if m := mapagg.IPGroupMeta(s.geo, tl.DstIP, "country"); m.Valid {
					row.DstCountry = m.Key
				}
			}
		case res.Skipped:
			skipped++
			if res.Reason != "" {
				row.Reason = res.Reason
			} else {
				row.Reason = "распознано парсером «" + res.Vendor + "», событие осознанно пропущено"
			}
		default:
			row.Reason = res.Reason
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return Result{}, err
	}

	return Result{
		Received:  received,
		Parsed:    parsed,
		Skipped:   skipped,
		Errors:    received - parsed - skipped,
		Truncated: truncated,
		MaxLines:  MaxLines,
		Results:   rows,
	}, nil
}
