package model

import "time"

// MetricRecord — одна точка метрики (system_metrics).
type MetricRecord struct {
	Timestamp  time.Time
	MetricType string
	Target     string
	MetricName string
	Value      float64
	Labels     string
}

// HistoryPoint — точка таймсерии.
type HistoryPoint struct {
	Timestamp time.Time `json:"t"`
	Value     float64   `json:"v"`
}

// MetricKey — ключ метрики type.target.name.
type MetricKey struct {
	Type   string
	Target string
	Name   string
}

func (k MetricKey) String() string {
	return k.Type + "." + k.Target + "." + k.Name
}
