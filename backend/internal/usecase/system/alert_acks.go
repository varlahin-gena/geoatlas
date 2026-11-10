package system

import (
	"errors"
	"strings"
	"sync"
)

var (
	ErrAlertNotFound   = errors.New("alert not found")
	ErrAlertNotAckable = errors.New("alert is not ackable")
)

type alertAckKey struct {
	Code   string
	Target string
}

type alertAck struct {
	Baseline    float64
	HasBaseline bool
}

// AlertAckStore holds in-memory dismissals for warn alerts.
type AlertAckStore struct {
	mu   sync.Mutex
	acks map[alertAckKey]alertAck
}

func NewAlertAckStore() *AlertAckStore {
	return &AlertAckStore{acks: make(map[alertAckKey]alertAck)}
}

func baselineMetric(code string, stats SystemStatsResponse) (float64, bool) {
	switch code {
	case "ingest_dropped_total":
		if p, ok := stats.Pipeline["ingest"]; ok {
			return p["dropped_total"], true
		}
	case "ingest_buffer_dropped_total":
		if p, ok := stats.Pipeline["ingest"]; ok {
			return p["buffer_drops_total"], true
		}
	case "syslogng_dropped_total":
		if p, ok := stats.Pipeline["syslogng"]; ok {
			return p["dropped_total"], true
		}
	}
	return 0, false
}

func (s *AlertAckStore) Ack(code, target string, stats SystemStatsResponse, alerts []Alert) error {
	if s == nil {
		return ErrAlertNotFound
	}
	code = normalizeAlertField(code)
	target = normalizeAlertField(target)
	if code == "" {
		return ErrAlertNotFound
	}

	var found *Alert
	for i := range alerts {
		a := &alerts[i]
		if a.Code == code && a.Target == target {
			found = a
			break
		}
	}
	if found == nil {
		return ErrAlertNotFound
	}
	if found.Level != "warn" {
		return ErrAlertNotAckable
	}

	entry := alertAck{}
	if v, ok := baselineMetric(code, stats); ok {
		entry.Baseline = v
		entry.HasBaseline = true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.acks == nil {
		s.acks = make(map[alertAckKey]alertAck)
	}
	s.acks[alertAckKey{Code: code, Target: target}] = entry
	return nil
}

func (s *AlertAckStore) Unack(code, target string) {
	if s == nil {
		return
	}
	code = normalizeAlertField(code)
	target = normalizeAlertField(target)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.acks, alertAckKey{Code: code, Target: target})
}

// Filter drops acked warns, GCs stale acks, and reopens baseline alerts when the counter grows.
func (s *AlertAckStore) Filter(alerts []Alert, stats SystemStatsResponse) []Alert {
	if s == nil {
		return alerts
	}

	present := make(map[alertAckKey]struct{}, len(alerts))
	for _, a := range alerts {
		present[alertAckKey{Code: a.Code, Target: a.Target}] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.acks) == 0 {
		return alerts
	}

	for key, ack := range s.acks {
		if _, ok := present[key]; !ok {
			delete(s.acks, key)
			continue
		}
		if ack.HasBaseline {
			if cur, ok := baselineMetric(key.Code, stats); ok && cur > ack.Baseline {
				delete(s.acks, key)
			}
		}
	}

	if len(alerts) == 0 {
		return alerts
	}

	out := make([]Alert, 0, len(alerts))
	for _, a := range alerts {
		key := alertAckKey{Code: a.Code, Target: a.Target}
		ack, ok := s.acks[key]
		if !ok {
			out = append(out, a)
			continue
		}
		if ack.HasBaseline {
			if cur, ok := baselineMetric(a.Code, stats); ok && cur > ack.Baseline {
				out = append(out, a)
				continue
			}
		}
		// suppressed (episode or baseline still holds)
	}
	return out
}

func normalizeAlertField(v string) string {
	return strings.TrimSpace(v)
}
