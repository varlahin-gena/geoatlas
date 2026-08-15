package ingest

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"
)

// Параметры retry (переопределяются в тестах).
var (
	insertRetryAttempts  = 5
	insertRetryBase      = 100 * time.Millisecond
	insertRetryMax       = 4 * time.Second
	circuitFailThreshold = 5
	circuitCooldown      = 15 * time.Second
)

var errInsertCircuitOpen = errors.New("ingest: clickhouse insert circuit open")

func insertBackoff(attempt int) time.Duration {
	// attempt 1 → base, 2 → 2*base, … с полным jitter.
	if attempt < 1 {
		attempt = 1
	}
	exp := insertRetryBase * time.Duration(1<<(attempt-1))
	if exp > insertRetryMax {
		exp = insertRetryMax
	}
	if exp <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(exp)+1))
	if err != nil {
		return exp / 2
	}
	return time.Duration(n.Int64())
}

func shouldRetryInsert(err error, classify InsertErrorClassifier) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, errInsertCircuitOpen) {
		return false
	}
	if classify == nil {
		// Без классификатора (тесты) — ретраим всё, кроме отмены/circuit.
		return true
	}
	return classify.IsRetryableInsertError(err)
}

// insertWithRetry вызывает fn с таймаутом на попытку; при retryable-ошибках —
// exponential backoff + jitter, пока жив parent ctx.
func insertWithRetry(ctx context.Context, attemptTimeout time.Duration, table string, classify InsertErrorClassifier, fn func(context.Context) error) error {
	if attemptTimeout <= 0 {
		attemptTimeout = 30 * time.Second
	}
	attempts := insertRetryAttempts
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		actx, cancel := context.WithTimeout(ctx, attemptTimeout)
		err := fn(actx)
		cancel()
		if err == nil {
			if attempt > 1 {
				slog.Info("ingest: insert succeeded after retry",
					"table", table, "attempts", attempt)
			}
			return nil
		}
		lastErr = err

		retry := attempt < attempts && shouldRetryInsert(err, classify) && ctx.Err() == nil
		if !retry {
			break
		}
		delay := insertBackoff(attempt)
		slog.Warn("ingest: insert failed, retrying",
			"table", table, "attempt", attempt, "max", attempts, "backoff", delay.String(), "err", err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w (during retry backoff): %v", ctx.Err(), lastErr)
		case <-time.After(delay):
		}
	}
	return lastErr
}
