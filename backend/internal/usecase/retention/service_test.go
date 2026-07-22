package retention

import (
	"context"
	"errors"
	"testing"
)

type memStore struct {
	s   Settings
	err error
}

func (m *memStore) Load() (Settings, error) {
	if m.err != nil {
		return Settings{}, m.err
	}
	if m.s.TrafficLogsDays == 0 {
		return Defaults(), nil
	}
	return m.s, nil
}
func (m *memStore) Save(s Settings) error {
	if m.err != nil {
		return m.err
	}
	m.s = s
	return nil
}

type memApplier struct {
	calls []string
	fail  string
}

func (m *memApplier) ApplyTrafficLogs(_ context.Context, days int) error {
	m.calls = append(m.calls, "traffic")
	if m.fail == "traffic" {
		return errors.New("boom")
	}
	_ = days
	return nil
}
func (m *memApplier) ApplyEdges(_ context.Context, days int) error {
	m.calls = append(m.calls, "edges")
	if m.fail == "edges" {
		return errors.New("boom")
	}
	_ = days
	return nil
}
func (m *memApplier) ApplyParseErrors(_ context.Context, days int) error {
	m.calls = append(m.calls, "parse")
	_ = days
	return nil
}
func (m *memApplier) ApplySystemMetrics(_ context.Context, days int) error {
	m.calls = append(m.calls, "metrics")
	_ = days
	return nil
}

func TestUpdateValidates(t *testing.T) {
	svc := New(&memStore{}, &memApplier{})
	_, err := svc.Update(context.Background(), Settings{
		TrafficLogsDays: 0, EdgesDays: 30, ParseErrorsDays: 7, SystemMetricsDays: 7,
	})
	if !IsClientError(err) {
		t.Fatalf("want ErrInvalidDays, got %v", err)
	}
}

func TestUpdateApplies(t *testing.T) {
	store := &memStore{}
	app := &memApplier{}
	svc := New(store, app)
	out, err := svc.Update(context.Background(), Settings{
		TrafficLogsDays: 14, EdgesDays: 21, ParseErrorsDays: 3, SystemMetricsDays: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.TrafficLogsDays != 14 || out.UpdatedAt == "" {
		t.Fatalf("unexpected out: %+v", out)
	}
	if len(app.calls) != 4 {
		t.Fatalf("calls=%v", app.calls)
	}
	got, err := svc.Get()
	if err != nil || got.TrafficLogsDays != 14 {
		t.Fatalf("Get=%+v err=%v", got, err)
	}
}

func TestApplyFromStoreDefaults(t *testing.T) {
	app := &memApplier{}
	svc := New(&memStore{}, app)
	if err := svc.ApplyFromStore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(app.calls) != 4 {
		t.Fatalf("calls=%v", app.calls)
	}
}
