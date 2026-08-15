package ingeststore

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
)

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

// IsRetryableInsertError — классификатор для usecase/ingest retry (ClickHouse + сеть).
func IsRetryableInsertError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	// DeadlineExceeded родителя — не ретраим; короткий attempt timeout отсекается в usecase.
	if errors.Is(err, context.DeadlineExceeded) {
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
