package parser

import "testing"

func parseableSamples() []Sample {
	out := make([]Sample, 0, len(sampleCorpus))
	for _, s := range sampleCorpus {
		if !s.Skip {
			out = append(out, s)
		}
	}
	return out
}

func BenchmarkParseVerboseMixed(b *testing.B) {
	r := testRegistry()
	samples := parseableSamples()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_ = r.ParseVerbose(samples[i%len(samples)].Line)
		i++
	}
}

func BenchmarkParseVerboseByVendor(b *testing.B) {
	r := testRegistry()
	for _, s := range parseableSamples() {
		b.Run(s.Vendor+"/"+s.Desc, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(s.Line)))
			for b.Loop() {
				_ = r.ParseVerbose(s.Line)
			}
		})
	}
}
