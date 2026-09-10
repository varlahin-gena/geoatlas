package ingest

import (
	"context"
	"testing"

	"geoatlas/internal/geoip"
	"geoatlas/internal/model"
	"geoatlas/internal/parser"
)

func BenchmarkProcessLineMixed(b *testing.B) {
	var lines []string
	for _, s := range parser.Samples() {
		if !s.Skip {
			lines = append(lines, s.Line)
		}
	}
	ins := &stubInserter{}
	deps := testDeps(ins)
	proc := NewProcessor(deps, nil)
	ctx := context.Background()

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_, _, err := proc.ProcessLine(ctx, lines[i%len(lines)], "tcp")
		if err != nil {
			b.Fatal(err)
		}
		i++
	}
}

func BenchmarkProcessLineWithGeo(b *testing.B) {
	var lines []string
	for _, s := range parser.Samples() {
		if !s.Skip {
			lines = append(lines, s.Line)
		}
	}
	idx := geoip.New()
	idx.ReplaceRanges([]model.GeoRange{
		{StartIP: 0, EndIP: ^uint32(0), Country: "RU", City: "MSK", Lat: 55, Lon: 37},
	})
	ins := &stubInserter{}
	deps := testDeps(ins)
	deps.Geo = idx
	deps.EnrichCountry = true
	proc := NewProcessor(deps, nil)
	ctx := context.Background()

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_, _, err := proc.ProcessLine(ctx, lines[i%len(lines)], "tcp")
		if err != nil {
			b.Fatal(err)
		}
		i++
	}
}
