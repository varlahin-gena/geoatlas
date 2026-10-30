package reputationcodec

import (
	"strings"
	"testing"
)

func TestCodecReadCSVAndNetset(t *testing.T) {
	c := New()
	csv := "Network,List,Category\n1.2.3.0/24,drop,malware\n"
	ranges, err := c.ReadCSV(strings.NewReader(csv))
	if err != nil || len(ranges) != 1 {
		t.Fatalf("%v %v", ranges, err)
	}
	ns, err := c.ParseNetset(strings.NewReader("9.9.9.0/24\n# comment\n"), "spamhaus", "botnet", "url")
	if err != nil || len(ns) != 1 {
		t.Fatalf("%v %v", ns, err)
	}
}
