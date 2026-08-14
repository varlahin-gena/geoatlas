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

	prevSyslogDrop int64
	prevSyslogProc int64
	prevSyslogTS   time.Time

	healthPrevDrop    int64
	healthPrevBufDrop int64
	healthPrevTS      time.Time
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

// ObserveSyslogNG returns syslog-ng drop and processed rates between scrapes.
func (r *RateSampler) ObserveSyslogNG(droppedTotal, processedTotal int64) (dropsPerSec, eventsPerSec float64) {
	if r == nil {
		return 0, 0
	}
	now := time.Now()
	r.mu.Lock()
	prevTS, prevDrop, prevProc := r.prevSyslogTS, r.prevSyslogDrop, r.prevSyslogProc
	r.prevSyslogDrop, r.prevSyslogProc, r.prevSyslogTS = droppedTotal, processedTotal, now
	r.mu.Unlock()
	if prevTS.IsZero() {
		return 0, 0
	}
	dt := now.Sub(prevTS).Seconds()
	if dt <= 0 {
		return 0, 0
	}
	delta := func(current, previous int64) float64 {
		value := float64(current - previous)
		if value < 0 {
			return 0
		}
		return value / dt
	}
	return delta(droppedTotal, prevDrop), delta(processedTotal, prevProc)
}

// HealthDropRates — admission и buffer drop rates между вызовами Health (один dt).
func (r *RateSampler) HealthDropRates(droppedTotal, bufferDropsTotal int64) (dropsPerSec, bufferDropsPerSec float64) {
	if r == nil {
		return 0, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if !r.healthPrevTS.IsZero() {
		if dt := now.Sub(r.healthPrevTS).Seconds(); dt > 0 {
			dropsPerSec = float64(droppedTotal-r.healthPrevDrop) / dt
			if dropsPerSec < 0 {
				dropsPerSec = 0
			}
			bufferDropsPerSec = float64(bufferDropsTotal-r.healthPrevBufDrop) / dt
			if bufferDropsPerSec < 0 {
				bufferDropsPerSec = 0
			}
		}
	}
	r.healthPrevDrop, r.healthPrevBufDrop, r.healthPrevTS = droppedTotal, bufferDropsTotal, now
	return dropsPerSec, bufferDropsPerSec
}

// DropsPerSec — совместимость (только admission). Для Health используйте HealthDropRates.
func (r *RateSampler) DropsPerSec(droppedTotal int64) float64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	bufPrev := r.healthPrevBufDrop
	r.mu.Unlock()
	d, _ := r.HealthDropRates(droppedTotal, bufPrev)
	return d
}
