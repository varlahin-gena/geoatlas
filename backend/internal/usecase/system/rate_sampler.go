package system

import (
	"sync"
	"time"
)

// RateSampler считает events/drops per second между последовательными
// наблюдениями. Состояние привязано к экземпляру Service, а не к пакету.
type RateSampler struct {
	mu sync.Mutex

	prevRecv    int64
	prevUDP     int64
	prevTCP     int64
	prevDrop    int64
	prevBufDrop int64
	prevTS      time.Time

	healthPrevDrop int64
	healthPrevTS   time.Time
}

// ObserveRates обновляет counters и возвращает rate-метрики (events/drops per sec).
// Ключи совпадают с pipeline.rate в CollectStats.
func (r *RateSampler) ObserveRates(snap IngestSnapshot) map[string]float64 {
	rate := map[string]float64{
		"udp_events_per_sec":   0,
		"tcp_events_per_sec":   0,
		"drops_per_sec":        0,
		"buffer_drops_per_sec": 0,
	}
	if r == nil {
		return rate
	}
	now := time.Now()
	r.mu.Lock()
	prevTS, prevRecv := r.prevTS, r.prevRecv
	prevUDP, prevTCP, prevDrop := r.prevUDP, r.prevTCP, r.prevDrop
	prevBufDrop := r.prevBufDrop
	r.prevRecv, r.prevUDP, r.prevTCP = snap.ReceivedTotal, snap.UDPReceived, snap.TCPReceived
	r.prevDrop, r.prevBufDrop, r.prevTS = snap.DroppedTotal, snap.BufferDropsTotal, now
	r.mu.Unlock()

	if prevTS.IsZero() {
		return rate
	}
	dt := now.Sub(prevTS).Seconds()
	if dt <= 0 {
		return rate
	}
	delta := func(current, previous int64) float64 {
		value := float64(current - previous)
		if value < 0 {
			return 0
		}
		return value / dt
	}
	rate["events_per_sec"] = delta(snap.ReceivedTotal, prevRecv)
	rate["input_events_per_sec"] = rate["events_per_sec"]
	rate["udp_events_per_sec"] = delta(snap.UDPReceived, prevUDP)
	rate["tcp_events_per_sec"] = delta(snap.TCPReceived, prevTCP)
	rate["drops_per_sec"] = delta(snap.DroppedTotal, prevDrop)
	rate["buffer_drops_per_sec"] = delta(snap.BufferDropsTotal, prevBufDrop)
	return rate
}

// DropsPerSec — rate дропов между вызовами Health (отдельный ряд от ObserveRates).
func (r *RateSampler) DropsPerSec(droppedTotal int64) float64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	var rate float64
	if !r.healthPrevTS.IsZero() {
		if dt := now.Sub(r.healthPrevTS).Seconds(); dt > 0 {
			rate = float64(droppedTotal-r.healthPrevDrop) / dt
			if rate < 0 {
				rate = 0
			}
		}
	}
	r.healthPrevDrop, r.healthPrevTS = droppedTotal, now
	return rate
}
