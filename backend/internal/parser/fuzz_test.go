package parser

import "testing"

func FuzzParseVerbose(f *testing.F) {
	for _, s := range Samples() {
		f.Add(s.Line)
	}
	f.Add("")
	f.Add("not a log line at all")
	f.Add("CEF:0|x|y|1|2|3|4|src=1.2.3.4")
	f.Add("%ASA-6-302013: Built outbound TCP connection")
	f.Add(`{"eventid":"cowrie.session.connect"}`)

	reg := NewRegistry(
		&UserGateCEF{},
		&FortigateCEF{},
		&CiscoFTD{},
		&CiscoASA{},
		&CowrieJSON{},
		&GenericKV{},
	)

	f.Fuzz(func(t *testing.T, line string) {
		// Не должно паниковать на произвольном вводе.
		_ = reg.ParseVerbose(line)
	})
}
