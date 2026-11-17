package reputation

import (
	"errors"
	"testing"
	"time"

	"geoatlas/internal/config"
)

type memSettingsStore struct {
	st  Settings
	err error
}

func (m *memSettingsStore) Load() (Settings, error) {
	if m.err != nil {
		return Settings{}, m.err
	}
	return m.st, nil
}

func (m *memSettingsStore) Save(s Settings) error {
	if m.err != nil {
		return m.err
	}
	m.st = s
	return nil
}

func TestDefaultSettingsFromConfig(t *testing.T) {
	st := DefaultSettingsFromConfig(config.ReputationConfig{FetchInterval: 6 * time.Hour})
	if st.FetchIntervalHours != 6 {
		t.Fatalf("got %d", st.FetchIntervalHours)
	}
	st = DefaultSettingsFromConfig(config.ReputationConfig{FetchInterval: 30 * time.Minute})
	if st.FetchIntervalHours != 1 {
		t.Fatalf("sub-hour → 1h, got %d", st.FetchIntervalHours)
	}
	st = DefaultSettingsFromConfig(config.ReputationConfig{})
	if st.FetchIntervalHours != 6 {
		t.Fatalf("zero → 6h, got %d", st.FetchIntervalHours)
	}
}

func TestSettingsServiceUpdateAndApply(t *testing.T) {
	var applied time.Duration
	store := &memSettingsStore{}
	svc := NewSettingsService(store, Settings{FetchIntervalHours: 6}, func(d time.Duration) {
		applied = d
	})
	got, err := svc.LoadAndApply()
	if err != nil || got.FetchIntervalHours != 6 || applied != 6*time.Hour {
		t.Fatalf("load: %+v err=%v applied=%v", got, err, applied)
	}

	out, err := svc.Update(Settings{FetchIntervalHours: 12})
	if err != nil {
		t.Fatal(err)
	}
	if out.FetchIntervalHours != 12 || out.UpdatedAt == "" || applied != 12*time.Hour {
		t.Fatalf("%+v applied=%v", out, applied)
	}
	got, err = svc.Get()
	if err != nil || got.FetchIntervalHours != 12 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestSettingsServiceValidate(t *testing.T) {
	svc := NewSettingsService(&memSettingsStore{}, Settings{FetchIntervalHours: 6}, nil)
	if _, err := svc.Update(Settings{FetchIntervalHours: 0}); err == nil {
		t.Fatal("expected range error")
	}
	if _, err := svc.Update(Settings{FetchIntervalHours: 200}); err == nil {
		t.Fatal("expected range error")
	}
}

func TestSettingsServiceLoadErrors(t *testing.T) {
	store := &memSettingsStore{err: errors.New("boom")}
	svc := NewSettingsService(store, Settings{FetchIntervalHours: 6}, nil)
	if _, err := svc.Get(); err == nil {
		t.Fatal("expected load error")
	}
	var nilSvc *SettingsService
	if _, err := nilSvc.Get(); err == nil {
		t.Fatal("nil service")
	}
}
