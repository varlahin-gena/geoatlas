package events

import (
	"context"
	"strings"
	"time"

	"network_monitor/internal/model"
)

// GetSeriesInput — параметры временного ряда для страны.
type GetSeriesInput struct {
	TimeRange  model.TimeRange
	Country    string
	DataSource string // live|backup — table scope выбирает репозиторий
	Timeout    time.Duration
}

// GetSeriesResult — ответ для sparkline.
type GetSeriesResult struct {
	Country    string
	BucketSec  int
	Points     []SeriesPoint
	Period     string
	Amount     int
	From       time.Time
	To         time.Time
}

// GetSeries возвращает allowed/blocked по bucket для страны (src или dst).
func (s *Service) GetSeries(ctx context.Context, in GetSeriesInput) (GetSeriesResult, error) {
	country := strings.TrimSpace(in.Country)
	out := GetSeriesResult{
		Country: country,
		Period:  in.TimeRange.Mode,
		Amount:  in.TimeRange.Amount,
		From:    in.TimeRange.From,
		To:      in.TimeRange.To,
		Points:  []SeriesPoint{},
	}
	if country == "" {
		return out, nil
	}
	points, bucket, err := s.traffic.ScanCountrySeries(ctx, in.TimeRange, country, in.DataSource, in.Timeout)
	if err != nil {
		return GetSeriesResult{}, err
	}
	out.BucketSec = bucket
	if points != nil {
		out.Points = points
	}
	return out, nil
}
