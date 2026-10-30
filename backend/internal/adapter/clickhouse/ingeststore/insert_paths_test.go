package ingeststore

import (
	"context"
	"errors"
	"testing"
	"time"

	"geoatlas/internal/model"
)

func TestInsertTrafficAndParseErrors(t *testing.T) {
	ch := &stubConn{}
	repo := NewIngestRepository(ch)

	if err := repo.InsertTrafficLogs(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertParseErrors(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	logs := []model.TrafficLog{{
		Timestamp: time.Now().UTC(),
		Vendor:    "fortigate",
		SrcIP:     "10.0.0.1",
		DstIP:     "8.8.8.8",
	}}
	if err := repo.InsertTrafficLogs(context.Background(), logs); err != nil {
		t.Fatal(err)
	}
	if ch.colAppends != trafficColumnCount {
		t.Fatalf("colAppends=%d want %d", ch.colAppends, trafficColumnCount)
	}

	ch.colAppends = 0
	ch.prepareErr = errors.New("prep fail")
	if err := InsertTrafficLogs(context.Background(), ch, logs); err == nil {
		t.Fatal("want prep err")
	}
	ch.prepareErr = nil
	ch.colAppendEr = errors.New("col fail")
	if err := InsertTrafficLogs(context.Background(), ch, logs); err == nil {
		t.Fatal("want col err")
	}
	ch.colAppendEr = nil
	ch.sendErr = errors.New("send fail")
	if err := InsertTrafficLogs(context.Background(), ch, logs); err == nil {
		t.Fatal("want send err")
	}
	ch.sendErr = nil

	items := []model.ParseError{{
		Timestamp: time.Now().UTC(),
		Vendor:    "x",
		Reason:    "bad",
		Raw:       "line",
	}}
	if err := repo.InsertParseErrors(context.Background(), items); err != nil {
		t.Fatal(err)
	}
	ch.prepareErr = errors.New("pe prep")
	if err := InsertParseErrors(context.Background(), ch, items); err == nil {
		t.Fatal("want pe prep")
	}
	ch.prepareErr = nil
	ch.appendErr = errors.New("pe append")
	if err := InsertParseErrors(context.Background(), ch, items); err == nil {
		t.Fatal("want pe append")
	}
	ch.appendErr = nil
	ch.sendErr = errors.New("pe send")
	if err := InsertParseErrors(context.Background(), ch, items); err == nil {
		t.Fatal("want pe send")
	}
}

func TestReleaseNil(t *testing.T) {
	releaseTrafficColumns(nil)
}
