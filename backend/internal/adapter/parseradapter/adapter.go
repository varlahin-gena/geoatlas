package parseradapter

import (
	"network_monitor/internal/parser"
	usecaseingest "network_monitor/internal/usecase/ingest"
	"network_monitor/internal/usecase/parsetest"
)

// Adapter wraps *parser.Registry for usecase/ingest.LineParser.
type Adapter struct {
	reg *parser.Registry
}

func New(reg *parser.Registry) *Adapter {
	return &Adapter{reg: reg}
}

var _ usecaseingest.LineParser = (*Adapter)(nil)

func (a *Adapter) ParseVerbose(line string) usecaseingest.ParseResult {
	if a == nil || a.reg == nil {
		return usecaseingest.ParseResult{Reason: "parser unavailable"}
	}
	res := a.reg.ParseVerbose(line)
	return usecaseingest.ParseResult{
		OK: res.OK, Skipped: res.Skipped, Log: res.Log,
		Vendor: res.Vendor, Reason: res.Reason,
	}
}

func (a *Adapter) ContainsIPv4(line string) bool {
	return parser.ContainsIPv4(line)
}

// ParseTestAdapter — VerboseParser + SamplesProvider для usecase/parsetest.
type ParseTestAdapter struct {
	reg *parser.Registry
}

func NewParseTest(reg *parser.Registry) *ParseTestAdapter {
	return &ParseTestAdapter{reg: reg}
}

var (
	_ parsetest.VerboseParser   = (*ParseTestAdapter)(nil)
	_ parsetest.SamplesProvider = (*ParseTestAdapter)(nil)
)

func (a *ParseTestAdapter) ParseVerbose(line string) parsetest.ParseResult {
	if a == nil || a.reg == nil {
		return parsetest.ParseResult{Reason: "parser unavailable"}
	}
	res := a.reg.ParseVerbose(line)
	return parsetest.ParseResult{
		OK: res.OK, Skipped: res.Skipped, Log: res.Log,
		Vendor: res.Vendor, Reason: res.Reason,
	}
}

func (a *ParseTestAdapter) SamplesByVendor() map[string][]string {
	return parser.SamplesByVendor()
}
