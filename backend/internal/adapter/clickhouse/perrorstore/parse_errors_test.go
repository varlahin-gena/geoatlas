package perrorstore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestListParseErrorsValidation(t *testing.T) {
	ch := &stubConn{}
	if _, err := ListParseErrors(context.Background(), ch, 0, ""); err == nil {
		t.Fatal("limit 0")
	}
	if _, err := ListParseErrors(context.Background(), ch, 5001, ""); err == nil {
		t.Fatal("limit too high")
	}
}

func TestListParseErrorsOK(t *testing.T) {
	ch := &stubConn{}
	ts := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	ch.push([]any{"id-1", ts, "cisco", "bad", "raw line"})
	rows, err := ListParseErrors(context.Background(), ch, 10, "")
	if err != nil || len(rows) != 1 || rows[0].ID != "id-1" {
		t.Fatalf("%v %v", rows, err)
	}
}

func TestListParseErrorsWithSearch(t *testing.T) {
	ch := &stubConn{}
	ts := time.Now().UTC()
	ch.push([]any{"a", ts, "v", "r", "raw"})
	rows, err := ListParseErrors(context.Background(), ch, 5, "needle")
	if err != nil || len(rows) != 1 {
		t.Fatalf("%v %v", rows, err)
	}
}

func TestListParseErrorsQueryAndScanError(t *testing.T) {
	ch := &stubConn{}
	ch.pushErr(errors.New("down"))
	if _, err := ListParseErrors(context.Background(), ch, 10, ""); err == nil {
		t.Fatal("want query err")
	}
	ch.push([]any{"only"})
	if _, err := ListParseErrors(context.Background(), ch, 10, ""); err == nil {
		t.Fatal("want scan err")
	}
}

func TestDeleteParseErrors(t *testing.T) {
	ch := &stubConn{}
	if err := DeleteParseErrors(context.Background(), ch, nil); err != nil {
		t.Fatal(err)
	}
	tooMany := make([]string, 1001)
	for i := range tooMany {
		tooMany[i] = "x"
	}
	if err := DeleteParseErrors(context.Background(), ch, tooMany); err == nil {
		t.Fatal("want too many")
	}
	if err := DeleteParseErrors(context.Background(), ch, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if len(ch.execCalls) != 1 || !strings.Contains(ch.execCalls[0], "DELETE") {
		t.Fatalf("%v", ch.execCalls)
	}
}

func TestDeleteAllParseErrors(t *testing.T) {
	ch := &stubConn{}
	if err := DeleteAllParseErrors(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	if len(ch.execCalls) != 1 || !strings.Contains(ch.execCalls[0], "TRUNCATE") {
		t.Fatalf("%v", ch.execCalls)
	}
}

func TestParseErrorRepository(t *testing.T) {
	api := &stubConn{}
	write := &stubConn{}
	ts := time.Now().UTC()
	api.push([]any{"1", ts, "v", "r", "raw"})
	r := NewParseErrorRepository(api, write)
	rows, err := r.List(context.Background(), 10, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("%v %v", rows, err)
	}
	if err := r.Delete(context.Background(), []string{"1"}); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(write.execCalls) != 2 {
		t.Fatalf("write exec=%d", len(write.execCalls))
	}
}
