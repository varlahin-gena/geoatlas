package hunts

import (
	"errors"
	"strings"
	"unicode/utf8"

	"geoatlas/internal/apperr"
)

const (
	MaxNameLen     = 80
	MaxNotesLen    = 500
	MaxPerUser     = 30
	MaxHuntsAll    = 500
	MaxRunHistory  = 20
	MinIntervalMin = 15
	MaxIntervalMin = 1440
	MaxRunsKeep    = 20
)

var (
	ErrInvalidName      = apperr.InvalidInput("invalid hunt name")
	ErrInvalidMapState  = apperr.InvalidInput("invalid hunt map state")
	ErrInvalidSchedule  = apperr.InvalidInput("invalid hunt schedule")
	ErrNotFound         = apperr.NotFound("hunt not found")
	ErrLimitExceeded    = apperr.Conflict("hunt limit exceeded")
	ErrUnavailable      = errors.New("hunts not configured")
)

// MapState — полное состояние карты для saved hunt.
type MapState struct {
	Period     string `json:"period"`
	PeriodFrom string `json:"period_from,omitempty"`
	PeriodTo   string `json:"period_to,omitempty"`
	GroupBy    string `json:"group_by"`
	Filter     string `json:"filter"`
	Query      string `json:"query,omitempty"`
	Country    string `json:"country,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	DataSource string `json:"data_source,omitempty"`
}

// Schedule — периодический запуск hunt.
type Schedule struct {
	Enabled       bool    `json:"enabled"`
	IntervalMin   int     `json:"interval_min"`
	EdgeThreshold int     `json:"edge_threshold"`
	EdgeRatio     float64 `json:"edge_ratio"`
}

// RunResult — снимок одного прогона.
type RunResult struct {
	At        string  `json:"at"`
	EdgeCount int     `json:"edge_count"`
	RawPairs  int     `json:"raw_pairs"`
	QueryCost string  `json:"query_cost,omitempty"`
	Skipped   string  `json:"skipped,omitempty"`
	Breach    bool    `json:"breach"`
	PrevEdges int     `json:"prev_edges,omitempty"`
	Ratio     float64 `json:"ratio,omitempty"`
}

// Hunt — saved hunt пользователя.
type Hunt struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Notes     string     `json:"notes,omitempty"`
	Map       MapState   `json:"map"`
	Schedule  Schedule   `json:"schedule"`
	Runs      []RunResult `json:"runs,omitempty"`
	UpdatedAt string     `json:"updated_at,omitempty"`
	LastRunAt string     `json:"last_run_at,omitempty"`
}

// HuntWithAuthor — admin overview.
type HuntWithAuthor struct {
	Hunt
	Username string `json:"username"`
}

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLen {
		return ErrInvalidName
	}
	return nil
}

func ValidateMapState(st MapState) error {
	group := strings.ToLower(strings.TrimSpace(st.GroupBy))
	if group == "" {
		group = "city"
	}
	switch group {
	case "ip", "subnet", "city", "country", "continent":
	default:
		return ErrInvalidMapState
	}
	filter := normalizeFilter(st.Filter)
	switch filter {
	case "all", "allowed", "blocked":
	default:
		return ErrInvalidMapState
	}
	period := strings.TrimSpace(st.Period)
	if period == "" {
		period = "1d"
	}
	if period == "custom" {
		if strings.TrimSpace(st.PeriodFrom) == "" || strings.TrimSpace(st.PeriodTo) == "" {
			return ErrInvalidMapState
		}
	}
	if st.Limit < 0 || st.Limit > 50000 {
		return ErrInvalidMapState
	}
	return nil
}

func NormalizeSchedule(in Schedule) Schedule {
	out := in
	if out.IntervalMin < MinIntervalMin {
		out.IntervalMin = 60
	}
	if out.IntervalMin > MaxIntervalMin {
		out.IntervalMin = MaxIntervalMin
	}
	if out.EdgeThreshold < 0 {
		out.EdgeThreshold = 0
	}
	if out.EdgeRatio < 1 {
		out.EdgeRatio = 3
	}
	return out
}

func ValidateSchedule(in Schedule) (Schedule, error) {
	out := NormalizeSchedule(in)
	if out.IntervalMin < MinIntervalMin || out.IntervalMin > MaxIntervalMin {
		return Schedule{}, ErrInvalidSchedule
	}
	if out.EdgeThreshold < 0 {
		return Schedule{}, ErrInvalidSchedule
	}
	if out.EdgeRatio > 0 && out.EdgeRatio < 1 {
		return Schedule{}, ErrInvalidSchedule
	}
	return out, nil
}

func trimRuns(runs []RunResult) []RunResult {
	if len(runs) <= MaxRunHistory {
		return runs
	}
	return append([]RunResult(nil), runs[len(runs)-MaxRunHistory:]...)
}
