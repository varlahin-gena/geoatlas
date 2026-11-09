package connectorsconf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"geoatlas/internal/usecase/connectors"
)

func TestWriteDefaultAndExtra(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zz_connectors.conf")
	w := New(path)
	doc := connectors.Document{Connectors: []connectors.Connector{
		connectors.DefaultConnector(),
		{ID: "fw-a", Name: "A", Port: 5514, Protocols: "udp", Enabled: true},
		{ID: "off", Name: "Off", Port: 5515, Protocols: "both", Enabled: false},
	}}
	got, err := w.Write(doc)
	if err != nil || got != path {
		t.Fatalf("write: %v %q", err, got)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		"source s_default_udp",
		"source s_default_tcp",
		"source s_fw_a_udp",
		"@@ga/udp/`ga_ingest_token`/fw-a/@@",
		"port(5514)",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "s_off_") {
		t.Fatal("disabled connector must not render sources")
	}
}
