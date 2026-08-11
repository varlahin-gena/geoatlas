package backup

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	MinKeep = 1
	MaxKeep = 90
)

var ErrInvalidSchedule = errors.New("invalid backup schedule")

// Schedule — ежедневное автосоздание + политика keep/edges/auth.
type Schedule struct {
	Enabled      bool   `json:"enabled"`
	Hour         int    `json:"hour"`
	Minute       int    `json:"minute"`
	Timezone     string `json:"timezone"`
	Keep         int    `json:"keep"`
	IncludeEdges bool   `json:"include_edges"`
	IncludeAuth  bool   `json:"include_auth"`
	UpdatedAt    string `json:"updated_at,omitempty"`
	LastRunAt    string `json:"last_run_at,omitempty"`
	LastRunDate  string `json:"last_run_date,omitempty"` // YYYY-MM-DD в timezone расписания
}

// ScheduleStore — JSON-файл расписания.
type ScheduleStore interface {
	Load() (Schedule, error)
	Save(Schedule) error
}

// DefaultsSchedule — seed из env Options (авто выключено).
func DefaultsSchedule(opts Options) Schedule {
	tz := "Europe/Moscow"
	keep := opts.Keep
	if keep < MinKeep {
		keep = 7
	}
	return Schedule{
		Enabled:      false,
		Hour:         2,
		Minute:       30,
		Timezone:     tz,
		Keep:         keep,
		IncludeEdges: opts.IncludeEdges,
		IncludeAuth:  opts.IncludeAuth,
	}
}

func ValidateSchedule(in Schedule) (Schedule, error) {
	tz := strings.TrimSpace(in.Timezone)
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return Schedule{}, fmt.Errorf("%w: timezone %q: %v", ErrInvalidSchedule, tz, err)
	}
	if in.Hour < 0 || in.Hour > 23 {
		return Schedule{}, fmt.Errorf("%w: hour must be 0..23 (got %d)", ErrInvalidSchedule, in.Hour)
	}
	if in.Minute < 0 || in.Minute > 59 {
		return Schedule{}, fmt.Errorf("%w: minute must be 0..59 (got %d)", ErrInvalidSchedule, in.Minute)
	}
	keep := in.Keep
	if keep < MinKeep || keep > MaxKeep {
		return Schedule{}, fmt.Errorf("%w: keep must be %d..%d (got %d)", ErrInvalidSchedule, MinKeep, MaxKeep, keep)
	}
	return Schedule{
		Enabled:      in.Enabled,
		Hour:         in.Hour,
		Minute:       in.Minute,
		Timezone:     tz,
		Keep:         keep,
		IncludeEdges: in.IncludeEdges,
		IncludeAuth:  in.IncludeAuth,
		UpdatedAt:    in.UpdatedAt,
		LastRunAt:    in.LastRunAt,
		LastRunDate:  strings.TrimSpace(in.LastRunDate),
	}, nil
}

// NextRunAt — ближайший запуск: сегодняшняя HH:MM, либо «сейчас» если слот уже прошёл
// и сегодня ещё не было успешного автобэкапа (догон), иначе завтра.
func NextRunAt(sch Schedule, now time.Time) time.Time {
	loc, err := time.LoadLocation(sch.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	candidate := time.Date(local.Year(), local.Month(), local.Day(), sch.Hour, sch.Minute, 0, 0, loc)
	dateKey := local.Format("2006-01-02")
	if sch.LastRunDate == dateKey {
		return candidate.Add(24 * time.Hour).UTC()
	}
	if local.Before(candidate) {
		return candidate.UTC()
	}
	// Слот сегодня уже прошёл, успешного прогона ещё нет — тикер догонит.
	return now.UTC()
}

// ShouldFire — true, если сегодня ещё не было успешного автобэкапа и локальное время
// уже достигло (или прошло) слот HH:MM (догон после простоя / ErrBusy / сбоя job).
func ShouldFire(sch Schedule, now time.Time) (fire bool, dateKey string) {
	if !sch.Enabled {
		return false, ""
	}
	loc, err := time.LoadLocation(sch.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	dateKey = local.Format("2006-01-02")
	if sch.LastRunDate == dateKey {
		return false, dateKey
	}
	candidate := time.Date(local.Year(), local.Month(), local.Day(), sch.Hour, sch.Minute, 0, 0, loc)
	if local.Before(candidate) {
		return false, dateKey
	}
	return true, dateKey
}
