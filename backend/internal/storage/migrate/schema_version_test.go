package migrate

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
)

func TestNeedsSchemaDDLLogic(t *testing.T) {
	// Pure branching used by Ensure*: v < desired → need DDL.
	cases := []struct {
		have, want uint32
		need       bool
	}{
		{0, 1, true},
		{1, 1, false},
		{2, 1, false},
		{1, 2, true},
	}
	for _, tc := range cases {
		need := tc.have < tc.want
		if need != tc.need {
			t.Fatalf("have=%d want=%d: need=%v", tc.have, tc.want, need)
		}
	}
	if schemaVersionEdgesAgg < 1 || schemaVersionGeoEdges < 1 {
		t.Fatal("schema versions must be >= 1")
	}
}

func TestIsNoSchemaVersionRow(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{sql.ErrNoRows, true},
		{fmt.Errorf("wrap: %w", sql.ErrNoRows), true},
		{errors.New("clickhouse: no rows in result set"), true},
		{errors.New("empty result"), true},
		{errors.New("connection refused"), false},
		{errors.New("read: i/o timeout"), false},
		{contextCanceledErr{}, false},
	}
	for _, tc := range cases {
		if got := isNoSchemaVersionRow(tc.err); got != tc.want {
			t.Fatalf("isNoSchemaVersionRow(%v)=%v want %v", tc.err, got, tc.want)
		}
	}
}

type contextCanceledErr struct{}

func (contextCanceledErr) Error() string { return "context canceled" }
