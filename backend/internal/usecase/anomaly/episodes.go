package anomaly

import (
	"sort"
	"strings"
	"time"
)

const episodeHourWindow = time.Hour

// EpisodeID группирует алерты по якорному IP и часовому окну detected_at.
func EpisodeID(e Event, now time.Time) string {
	anchor := episodeAnchorIP(e)
	if anchor == "" {
		if e.Fingerprint != "" {
			return e.Fingerprint
		}
		return fingerprintAt("episode", "", "", "solo", now.UTC().Truncate(episodeHourWindow))
	}
	t := e.DetectedAt
	if t.IsZero() {
		t = now
	}
	bucket := t.UTC().Truncate(episodeHourWindow)
	return fingerprintAt("episode", anchor, "", "", bucket)
}

func episodeAnchorIP(e Event) string {
	src := displayIP(e.SrcIP)
	if src != "" {
		return src
	}
	return displayIP(e.DstIP)
}

func assignEpisodeIDs(events []Event, now time.Time) {
	for i := range events {
		events[i].EpisodeID = EpisodeID(events[i], now)
	}
}

// EpisodeSummary — агрегат для GET /api/anomalies/episodes.
type EpisodeSummary struct {
	EpisodeID   string    `json:"episode_id"`
	AnchorIP    string    `json:"anchor_ip,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	AlertCount  int       `json:"alert_count"`
	HighCount   int       `json:"high_count"`
	WarnCount   int       `json:"warn_count"`
	MaxSeverity string    `json:"max_severity"`
	Codes       []string  `json:"codes,omitempty"`
	Fingerprints []string `json:"fingerprints,omitempty"`
}

func buildEpisodeSummaries(items []Event) []EpisodeSummary {
	if len(items) == 0 {
		return []EpisodeSummary{}
	}
	type acc struct {
		sum EpisodeSummary
		codes map[string]struct{}
	}
	byID := map[string]*acc{}
	order := make([]string, 0)
	for _, e := range items {
		id := strings.TrimSpace(e.EpisodeID)
		if id == "" {
			id = EpisodeID(e, e.DetectedAt)
		}
		a, ok := byID[id]
		if !ok {
			a = &acc{
				sum: EpisodeSummary{
					EpisodeID: id,
					AnchorIP:  episodeAnchorIP(e),
					StartedAt: e.DetectedAt,
					UpdatedAt: e.DetectedAt,
				},
				codes: map[string]struct{}{},
			}
			byID[id] = a
			order = append(order, id)
		}
		a.sum.AlertCount++
		a.sum.Fingerprints = append(a.sum.Fingerprints, e.Fingerprint)
		if e.Code != "" {
			a.codes[e.Code] = struct{}{}
		}
		switch e.Severity {
		case SeverityHigh:
			a.sum.HighCount++
		case SeverityWarn:
			a.sum.WarnCount++
		}
		if e.DetectedAt.Before(a.sum.StartedAt) || a.sum.StartedAt.IsZero() {
			a.sum.StartedAt = e.DetectedAt
		}
		if e.DetectedAt.After(a.sum.UpdatedAt) {
			a.sum.UpdatedAt = e.DetectedAt
		}
	}
	out := make([]EpisodeSummary, 0, len(order))
	for _, id := range order {
		a := byID[id]
		a.sum.MaxSeverity = episodeMaxSeverity(a.sum.HighCount, a.sum.WarnCount)
		if len(a.codes) > 0 {
			a.sum.Codes = make([]string, 0, len(a.codes))
			for c := range a.codes {
				a.sum.Codes = append(a.sum.Codes, c)
			}
			sort.Strings(a.sum.Codes)
		}
		out = append(out, a.sum)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MaxSeverity != out[j].MaxSeverity {
			return severityRank(out[i].MaxSeverity) > severityRank(out[j].MaxSeverity)
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func episodeMaxSeverity(high, warn int) string {
	if high > 0 {
		return SeverityHigh
	}
	if warn > 0 {
		return SeverityWarn
	}
	return SeverityInfo
}

func severityRank(sev string) int {
	switch sev {
	case SeverityHigh:
		return 3
	case SeverityWarn:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}
