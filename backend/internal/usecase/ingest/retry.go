package ingest

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
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

// permanentCHCodes — серверные коды ClickHouse, при которых ретрай бесполезен.
// См. ErrorCodes.cpp в ClickHouse.
var permanentCHCodes = map[int32]struct{}{
	60:  {}, // UNKNOWN_TABLE
	81:  {}, // UNKNOWN_DATABASE
	62:  {}, // SYNTAX_ERROR
	47:  {}, // UNKNOWN_IDENTIFIER
	48:  {}, // NOT_FOUND_COLUMN_IN_BLOCK
	16:  {}, // NO_SUCH_COLUMN_IN_TABLE (legacy)
	36:  {}, // BAD_ARGUMENTS
	53:  {}, // TYPE_MISMATCH
	80:  {}, // INCORRECT_QUERY
	117: {}, // NUMBER_OF_COLUMNS_DOESNT_MATCH
	121: {}, // UNKNOWN_SETTING
	194: {}, // IP_ADDRESS_NOT_ALLOWED
	192: {}, // UNKNOWN_USER
	193: {}, // WRONG_PASSWORD
	195: {}, // REQUIRED_PASSWORD
	516: {}, // AUTHENTICATION_FAILED
	497: {}, // ACCESS_DENIED
}

// reCHCode — fallback, если Exception обёрнут в строку без errors.As.
var reCHCode = regexp.MustCompile(`(?i)\bcode:\s*(\d+)\b`)

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

func isRetryableInsertError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	// DeadlineExceeded родителя — не ретраим; короткий attempt timeout отсекается ниже.
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, errInsertCircuitOpen) {
		return false
	}

	if code, ok := extractCHCode(err); ok {
		if _, permanent := permanentCHCodes[code]; permanent {
			return false
		}
		// Прочие CH-коды (MEMORY_LIMIT, TIMEOUT_EXCEEDED, TOO_MANY_SIMULTANEOUS_QUERIES…) — ретраим.
		return true
	}

	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	msg := strings.ToLower(err.Error())
	permanent := []string{
		"syntax error",
		"unknown table",
		"unknown database",
		"authentication failed",
		"access denied",
		"wrong password",
		"unknown user",
	}
	for _, p := range permanent {
		if strings.Contains(msg, p) {
			return false
		}
	}

	retryable := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"i/o timeout",
		"timeout",
		"eof",
		"network is unreachable",
		"no connection",
		"temporarily unavailable",
		"too many connections",
		"dial tcp",
		"connect: ",
		"server closed",
		"unexpected packet",
		"read: connection",
		"write: connection",
		"acquire conn timeout",
	}
	for _, r := range retryable {
		if strings.Contains(msg, r) {
			return true
		}
	}
	// Неизвестные ошибки — ретраим осторожно (сеть/CH часто так сигналят).
	return true
}

func extractCHCode(err error) (int32, bool) {
	var ex *clickhouse.Exception
	if errors.As(err, &ex) && ex != nil {
		return ex.Code, true
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		if m := reCHCode.FindStringSubmatch(e.Error()); len(m) == 2 {
			n, convErr := strconv.ParseInt(m[1], 10, 32)
			if convErr != nil {
				continue
			}
			return int32(n), true
		}
	}
	return 0, false
}

// insertWithRetry вызывает fn с таймаутом на попытку; при retryable-ошибках —
// exponential backoff + jitter, пока жив parent ctx.
func insertWithRetry(ctx context.Context, attemptTimeout time.Duration, table string, fn func(context.Context) error) error {
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

		retry := attempt < attempts && isRetryableInsertError(err) && ctx.Err() == nil
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
