package ingest

import (
	"log/slog"
	"sync/atomic"
	"time"
)

// CircuitBreaker — общий circuit для insert'ов (все workers делят один экземпляр).
// Без shared breaker при outage CH часть workers открывает circuit, остальные
// продолжают долбить пул.
type CircuitBreaker struct {
	fails     atomic.Int64
	openUntil atomic.Int64 // unix nano; 0 = closed
}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{}
}

func (c *CircuitBreaker) Check() error {
	if c == nil {
		return nil
	}
	until := c.openUntil.Load()
	if until == 0 {
		return nil
	}
	if time.Now().UnixNano() < until {
		return errInsertCircuitOpen
	}
	return nil
}

// Open reports whether inserts are currently blocked by the circuit.
func (c *CircuitBreaker) Open() bool {
	return c.Check() != nil
}

// RemainingOpen — сколько ждать до закрытия окна; 0 если circuit закрыт.
func (c *CircuitBreaker) RemainingOpen() time.Duration {
	if c == nil {
		return 0
	}
	until := c.openUntil.Load()
	if until == 0 {
		return 0
	}
	left := until - time.Now().UnixNano()
	if left <= 0 {
		return 0
	}
	return time.Duration(left)
}

func (c *CircuitBreaker) NoteSuccess() {
	if c == nil {
		return
	}
	c.fails.Store(0)
	c.openUntil.Store(0)
}

func (c *CircuitBreaker) NoteFailure() {
	if c == nil {
		return
	}
	n := c.fails.Add(1)
	if n < int64(circuitFailThreshold) {
		return
	}
	until := time.Now().Add(circuitCooldown).UnixNano()
	prev := c.openUntil.Load()
	now := time.Now().UnixNano()
	// Уже открыт — не продлеваем окно на каждый fail (иначе cooldown никогда не кончится).
	if prev > now {
		return
	}
	if c.openUntil.CompareAndSwap(prev, until) {
		slog.Warn("ingest: insert circuit open",
			"failures", n, "cooldown", circuitCooldown.String())
	}
}

// OpenForTest форсирует открытый circuit (только тесты).
func (c *CircuitBreaker) OpenForTest(d time.Duration) {
	if c == nil {
		return
	}
	c.fails.Store(int64(circuitFailThreshold))
	c.openUntil.Store(time.Now().Add(d).UnixNano())
}
