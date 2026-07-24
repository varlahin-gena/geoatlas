package reputation

import (
	"strings"
	"testing"
	"time"
)

func TestParseSpamhausJSON_NDJSON(t *testing.T) {
	raw := `{"cidr":"203.0.113.0/24","sblid":"SBL1"}
{"type":"metadata","timestamp":1}
{"cidr":"198.51.100.10","sblid":"SBL2"}
{"cidr":"2001:db8::/32","sblid":"SBL6"}
`
	ranges, err := ParseSpamhausJSON(strings.NewReader(raw), "spamhaus_drop_official", "drop", "url", time.Unix(100, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 2 {
		t.Fatalf("want 2 ipv4 ranges, got %d: %+v", len(ranges), ranges)
	}
	if ranges[0].ListName != "spamhaus_drop_official" || ranges[0].Category != "drop" {
		t.Fatalf("%+v", ranges[0])
	}
}

func TestParseSpamhausJSON_Array(t *testing.T) {
	raw := `[{"cidr":"10.0.0.0/8"},{"type":"metadata"},{"cidr":"192.0.2.1"}]`
	ranges, err := ParseSpamhausJSON(strings.NewReader(raw), "x", "drop", "url", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	// 10/8 is public-routable parse — ParseNetworkField accepts it; we don't filter RFC1918 at parse time
	if len(ranges) < 1 {
		t.Fatalf("%+v", ranges)
	}
}

func TestParseCSVIP_DstIP(t *testing.T) {
	raw := `DstIP,DstPort,Firstseen
198.51.100.1,443,2024-01-01
203.0.113.5,80,2024-01-02
`
	ranges, err := ParseCSVIP(strings.NewReader(raw), "sslbl", "c2", "url", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 2 {
		t.Fatalf("got %d", len(ranges))
	}
}

func TestParseCSVIP_IPColumn(t *testing.T) {
	raw := `# comment
ip,malware
1.2.3.4,emotet
5.6.7.8,qakbot
`
	ranges, err := ParseCSVIP(strings.NewReader(raw), "feodo", "c2", "url", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 2 {
		t.Fatalf("got %d %+v", len(ranges), ranges)
	}
}

func TestParseFeedBody_PlainAlias(t *testing.T) {
	raw := "8.8.8.8\n# c\n1.1.1.1/32\n"
	ranges, err := ParseFeedBody("plain", strings.NewReader(raw), "t", "attacks", "url", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 2 {
		t.Fatalf("%d", len(ranges))
	}
}

func TestNormalizeFeedFormat(t *testing.T) {
	if NormalizeFeedFormat("plain") != "netset" {
		t.Fatal(NormalizeFeedFormat("plain"))
	}
	if !IsSupportedFeedFormat("spamhaus_json") || !IsSupportedFeedFormat("csv_ip") {
		t.Fatal("expected supported")
	}
	if IsSupportedFeedFormat("stix") {
		t.Fatal("stix should be unsupported")
	}
}
