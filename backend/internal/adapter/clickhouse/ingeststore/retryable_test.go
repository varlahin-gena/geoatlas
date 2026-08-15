package ingeststore

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func TestIsRetryableInsertError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{context.Canceled, false},
		{context.DeadlineExceeded, false},
		{errors.New("syntax error"), false},
		{errors.New("code: 60, message: Table x doesn't exist"), false},
		{errors.New("code: 516, message: Authentication failed"), false},
		{&clickhouse.Exception{Code: 60, Message: "UNKNOWN_TABLE"}, false},
		{&clickhouse.Exception{Code: 241, Message: "Memory limit exceeded"}, true},
		{errors.New("code: 241, message: Memory limit exceeded"), true},
		{errors.New("connection refused"), true},
		{errors.New("i/o timeout"), true},
		{errors.New("weird unknown"), true},
		{&net.OpError{Op: "dial", Err: errors.New("connection refused")}, true},
	}
	for _, tc := range cases {
		if got := IsRetryableInsertError(tc.err); got != tc.want {
			t.Errorf("%v: got %v want %v", tc.err, got, tc.want)
		}
	}
}
