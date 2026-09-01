package anomaly

import (
	"testing"
	"time"
)

func TestEpisodeIDSameAnchorSameHour(t *testing.T) {
	now := time.Date(2026, 3, 1, 10, 15, 0, 0, time.UTC)
	a := Event{DetectedAt: now, SrcIP: "10.0.0.1", Fingerprint: "aaa"}
	b := Event{DetectedAt: now.Add(20 * time.Minute), SrcIP: "10.0.0.1", Code: CodePortScan, Fingerprint: "bbb"}
	if EpisodeID(a, now) != EpisodeID(b, now) {
		t.Fatal("expected same episode within hour")
	}
}

func TestEpisodeIDDifferentHour(t *testing.T) {
	now := time.Date(2026, 3, 1, 10, 15, 0, 0, time.UTC)
	a := Event{DetectedAt: now, SrcIP: "10.0.0.1"}
	b := Event{DetectedAt: now.Add(time.Hour), SrcIP: "10.0.0.1"}
	if EpisodeID(a, now) == EpisodeID(b, now) {
		t.Fatal("expected different episodes across hours")
	}
}

func TestBuildEpisodeSummaries(t *testing.T) {
	now := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	items := []Event{
		{DetectedAt: now, SrcIP: "10.0.0.1", Severity: SeverityHigh, Code: CodePortScan, Fingerprint: "a"},
		{DetectedAt: now.Add(5 * time.Minute), SrcIP: "10.0.0.1", Severity: SeverityWarn, Code: CodeBlockedSurge, Fingerprint: "b"},
	}
	assignEpisodeIDs(items, now)
	sums := buildEpisodeSummaries(items)
	if len(sums) != 1 || sums[0].AlertCount != 2 || sums[0].HighCount != 1 {
		t.Fatalf("got %+v", sums)
	}
}
