package anomaly

import (
	"context"
	"testing"
	"time"

	"geoatlas/internal/config"
)

type memSettingsStore struct {
	data Settings
}

func (m *memSettingsStore) Load() (Settings, error) {
	return m.data, nil
}

func (m *memSettingsStore) Save(s Settings) error {
	m.data = s
	return nil
}

func TestValidateSettings(t *testing.T) {
	ok := Settings{
		Enabled:            true,
		ScanIntervalMin:    10,
		LearningDays:       5,
		SuppressHours:      48,
		IncludePrivate:     true,
		NewCountryMinShare: 0.1,
	}
	got, err := validateSettings(ok)
	if err != nil {
		t.Fatal(err)
	}
	if got.ScanIntervalMin != 10 || !got.IncludePrivate {
		t.Fatalf("got %+v", got)
	}

	if _, err := validateSettings(Settings{ScanIntervalMin: 0}); err == nil {
		t.Fatal("expected error for scan interval")
	}
	if _, err := validateSettings(Settings{
		ScanIntervalMin: 5, LearningDays: 5, SuppressHours: 24, NewCountryMinShare: 2,
	}); err == nil {
		t.Fatal("expected error for share")
	}
}

func TestSettingsServiceUpdateApplies(t *testing.T) {
	store := &memSettingsStore{}
	svc := New(Config{Enabled: true, InstallProfile: "medium"}, nil, nil, nil, nil, nil)
	settings := NewSettingsService(store, svc, DefaultSettingsFromConfig(config.Config{
		AnomalyScanInterval: 5 * time.Minute,
	}), nil)

	in := Settings{
		Enabled:            false,
		ScanIntervalMin:    15,
		LearningDays:       7,
		SuppressHours:      12,
		IncludePrivate:     true,
		NewCountryMinShare: 0.08,
	}
	view, err := settings.Update(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if view.Settings.Enabled {
		t.Fatal("expected scan disabled")
	}
	if svc.cfgSnapshot().LearningDays != 7 {
		t.Fatalf("cfg not applied: %+v", svc.cfgSnapshot())
	}
	if store.data.LearningDays != 7 {
		t.Fatalf("not saved: %+v", store.data)
	}
}

func TestDefaultSettingsFromConfig(t *testing.T) {
	st := DefaultSettingsFromConfig(config.Config{
		AnomalyScanInterval:       10 * time.Minute,
		AnomalyLearningDays:       4,
		AnomalySuppressHours:      36,
		AnomalyIncludePrivate:     true,
		AnomalyNewCountryMinShare: 0.07,
	})
	if st.ScanIntervalMin != 10 || st.LearningDays != 4 || !st.IncludePrivate {
		t.Fatalf("got %+v", st)
	}
}
