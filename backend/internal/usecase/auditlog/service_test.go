package auditlog

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	dr     []DREvent
	audit  []AuditEvent
	drQ    DRQuery
	auditQ AuditQuery
	err    error
}

func (f *fakeStore) WriteDR(_ context.Context, e DREvent) error {
	if f.err != nil {
		return f.err
	}
	f.dr = append(f.dr, e)
	return nil
}
func (f *fakeStore) WriteAudit(_ context.Context, e AuditEvent) error {
	if f.err != nil {
		return f.err
	}
	f.audit = append(f.audit, e)
	return nil
}
func (f *fakeStore) ListDR(_ context.Context, q DRQuery) ([]DREvent, error) {
	f.drQ = q
	if f.err != nil {
		return nil, f.err
	}
	return f.dr, nil
}
func (f *fakeStore) ListAudit(_ context.Context, q AuditQuery) ([]AuditEvent, error) {
	f.auditQ = q
	if f.err != nil {
		return nil, f.err
	}
	return f.audit, nil
}

func TestEnabledAndNilGuards(t *testing.T) {
	var s *Service
	if s.Enabled() {
		t.Fatal("nil service")
	}
	s = New(nil)
	if s.Enabled() {
		t.Fatal("nil store")
	}
	ctx := context.Background()
	if err := s.WriteDR(ctx, DREvent{}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteAudit(ctx, AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	if out, err := s.ListDR(ctx, DRQuery{}); err != nil || len(out) != 0 {
		t.Fatalf("%v %v", out, err)
	}
	if out, err := s.ListAudit(ctx, AuditQuery{}); err != nil || len(out) != 0 {
		t.Fatalf("%v %v", out, err)
	}
}

func TestWriteSetsTimestamp(t *testing.T) {
	st := &fakeStore{}
	s := New(st)
	ctx := context.Background()
	if err := s.WriteDR(ctx, DREvent{Actor: "a", Action: "x"}); err != nil {
		t.Fatal(err)
	}
	if st.dr[0].Timestamp.IsZero() {
		t.Fatal("DR timestamp")
	}
	fixed := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := s.WriteAudit(ctx, AuditEvent{Timestamp: fixed, Actor: "a"}); err != nil {
		t.Fatal(err)
	}
	if !st.audit[0].Timestamp.Equal(fixed) {
		t.Fatalf("%v", st.audit[0].Timestamp)
	}
}

func TestListDefaults(t *testing.T) {
	st := &fakeStore{dr: []DREvent{{Action: "a"}}, audit: []AuditEvent{{Action: "b"}}}
	s := New(st)
	ctx := context.Background()
	out, err := s.ListDR(ctx, DRQuery{Limit: 0})
	if err != nil || len(out) != 1 || st.drQ.Limit != 100 || st.drQ.Since.IsZero() {
		t.Fatalf("%+v q=%+v err=%v", out, st.drQ, err)
	}
	outA, err := s.ListAudit(ctx, AuditQuery{Limit: 500})
	if err != nil || len(outA) != 1 || st.auditQ.Limit != 100 || st.auditQ.Since.IsZero() {
		t.Fatalf("%+v q=%+v err=%v", outA, st.auditQ, err)
	}
	since := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	_, _ = s.ListDR(ctx, DRQuery{Limit: 50, Since: since})
	if st.drQ.Limit != 50 || !st.drQ.Since.Equal(since) {
		t.Fatalf("%+v", st.drQ)
	}
}

func TestStoreErrorsPropagate(t *testing.T) {
	st := &fakeStore{err: errors.New("boom")}
	s := New(st)
	ctx := context.Background()
	if err := s.WriteDR(ctx, DREvent{}); err == nil {
		t.Fatal("want err")
	}
	if err := s.WriteAudit(ctx, AuditEvent{}); err == nil {
		t.Fatal("want err")
	}
	if _, err := s.ListDR(ctx, DRQuery{Limit: 10, Since: time.Now()}); err == nil {
		t.Fatal("want err")
	}
	if _, err := s.ListAudit(ctx, AuditQuery{Limit: 10, Since: time.Now()}); err == nil {
		t.Fatal("want err")
	}
}
