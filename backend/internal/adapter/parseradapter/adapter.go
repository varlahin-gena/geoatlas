package parseradapter

import (
	"sync/atomic"

	"geoatlas/internal/parser"
	usecaseingest "geoatlas/internal/usecase/ingest"
	"geoatlas/internal/usecase/parsetest"
)

// RegistrySource — live registry snapshot.
type RegistrySource interface {
	ParseVerbose(line string) parser.ParseResult
}

// Adapter wraps a live registry for usecase/ingest.LineParser.
type Adapter struct {
	src RegistrySource
}

func New(src RegistrySource) *Adapter {
	return &Adapter{src: src}
}

// NewFromRegistry keeps backward-compatible wiring for tests.
func NewFromRegistry(reg *parser.Registry) *Adapter {
	return New(reg)
}

var _ usecaseingest.LineParser = (*Adapter)(nil)

func (a *Adapter) ParseVerbose(line string) parser.ParseResult {
	if a == nil || a.src == nil {
		return parser.ParseResult{Reason: "parser unavailable"}
	}
	return a.src.ParseVerbose(line)
}

func (a *Adapter) ContainsIPv4(line string) bool {
	return parser.ContainsIPv4(line)
}

// SamplesHolder — atomic map for /api/parse-samples.
type SamplesHolder struct {
	cur atomic.Pointer[map[string][]string]
}

func NewSamplesHolder(initial map[string][]string) *SamplesHolder {
	h := &SamplesHolder{}
	h.Store(initial)
	return h
}

func (h *SamplesHolder) Store(m map[string][]string) {
	if h == nil {
		return
	}
	if m == nil {
		m = map[string][]string{}
	}
	cp := make(map[string][]string, len(m))
	for k, v := range m {
		cp[k] = append([]string(nil), v...)
	}
	h.cur.Store(&cp)
}

func (h *SamplesHolder) SamplesByVendor() map[string][]string {
	if h == nil {
		return parser.SamplesByVendor()
	}
	p := h.cur.Load()
	if p == nil || *p == nil {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(*p))
	for k, v := range *p {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// ParseTestAdapter — VerboseParser + SamplesProvider для usecase/parsetest.
type ParseTestAdapter struct {
	src     RegistrySource
	samples *SamplesHolder
}

func NewParseTest(src RegistrySource, samples *SamplesHolder) *ParseTestAdapter {
	return &ParseTestAdapter{src: src, samples: samples}
}

// NewParseTestFromRegistry — совместимость со старыми тестами (hardcoded samples).
func NewParseTestFromRegistry(reg *parser.Registry) *ParseTestAdapter {
	return NewParseTest(reg, NewSamplesHolder(parser.SamplesByVendor()))
}

var (
	_ parsetest.VerboseParser   = (*ParseTestAdapter)(nil)
	_ parsetest.SamplesProvider = (*ParseTestAdapter)(nil)
)

func (a *ParseTestAdapter) ParseVerbose(line string) parser.ParseResult {
	if a == nil || a.src == nil {
		return parser.ParseResult{Reason: "parser unavailable"}
	}
	return a.src.ParseVerbose(line)
}

func (a *ParseTestAdapter) SamplesByVendor() map[string][]string {
	if a == nil || a.samples == nil {
		return parser.SamplesByVendor()
	}
	return a.samples.SamplesByVendor()
}
