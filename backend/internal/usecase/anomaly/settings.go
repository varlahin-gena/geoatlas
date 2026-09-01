package anomaly

import (
	"context"
	"errors"
	"fmt"
	"time"

	"geoatlas/internal/apperr"
	"geoatlas/internal/config"
)

const (
	minScanIntervalMin = 1
	maxScanIntervalMin = 1440
	minLearningDays    = 1
	maxLearningDays    = 30
	minSuppressHours   = 1
	maxSuppressHours   = 168
	minNewCountryShare = 0.01
	maxNewCountryShare = 1.0
)

var (
	ErrInvalidAnomalySettings = apperr.InvalidInput("invalid anomaly settings")
)

// Settings — редактируемые параметры движка (JSON на диске).
type Settings struct {
	Enabled            bool    `json:"enabled"`
	ScanIntervalMin    int     `json:"scan_interval_min"`
	LearningDays       int     `json:"learning_days"`
	SuppressHours      int     `json:"suppress_hours"`
	IncludePrivate     bool    `json:"include_private"`
	NewCountryMinShare float64 `json:"new_country_min_share"`
	UpdatedAt          string  `json:"updated_at,omitempty"`
}

// SettingsStore — персистентность настроек.
type SettingsStore interface {
	Load() (Settings, error)
	Save(s Settings) error
}

// SettingsView — ответ GET /api/anomalies/settings.
type SettingsView struct {
	Settings       Settings   `json:"settings"`
	InstallProfile string     `json:"install_profile"`
	Thresholds     Thresholds `json:"thresholds"`
	Status         ScanStatus `json:"status"`
}

// IntervalUpdater — hot-reload интервала планировщика.
type IntervalUpdater func(time.Duration)

// SettingsService — GET/PUT настроек + Apply к Service.
type SettingsService struct {
	store      SettingsStore
	anomaly    *Service
	seed       Settings
	onInterval IntervalUpdater
}

func NewSettingsService(store SettingsStore, anomaly *Service, seed Settings, onInterval IntervalUpdater) *SettingsService {
	return &SettingsService{
		store:      store,
		anomaly:    anomaly,
		seed:       normalizeSettings(seed),
		onInterval: onInterval,
	}
}

func DefaultSettingsFromConfig(cfg config.Config) Settings {
	min := int(cfg.AnomalyScanInterval / time.Minute)
	if min < minScanIntervalMin {
		min = 5
	}
	share := cfg.AnomalyNewCountryMinShare
	if share <= 0 {
		share = 0.05
	}
	return Settings{
		Enabled:            true,
		ScanIntervalMin:    min,
		LearningDays:       cfg.AnomalyLearningDays,
		SuppressHours:      cfg.AnomalySuppressHours,
		IncludePrivate:     cfg.AnomalyIncludePrivate,
		NewCountryMinShare: share,
	}
}

func (s *SettingsService) LoadAndApply() (Settings, error) {
	if s == nil || s.anomaly == nil {
		return Settings{}, errors.New("anomaly settings service not configured")
	}
	st, err := s.loadNormalized()
	if err != nil {
		return Settings{}, err
	}
	s.anomaly.ApplySettings(st)
	if s.onInterval != nil {
		s.onInterval(time.Duration(st.ScanIntervalMin) * time.Minute)
	}
	return st, nil
}

func (s *SettingsService) GetView() (SettingsView, error) {
	if s == nil || s.anomaly == nil {
		return SettingsView{}, errors.New("anomaly settings service not configured")
	}
	st, err := s.loadNormalized()
	if err != nil {
		return SettingsView{}, err
	}
	cfg := s.anomaly.cfgSnapshot()
	th := ThresholdsForProfile(cfg.InstallProfile)
	if cfg.NewCountryMinShare > 0 {
		th.NewCountryMinShare = cfg.NewCountryMinShare
	}
	return SettingsView{
		Settings:       st,
		InstallProfile: cfg.InstallProfile,
		Thresholds:     th,
		Status:         s.anomaly.Status(),
	}, nil
}

func (s *SettingsService) Update(ctx context.Context, in Settings) (SettingsView, error) {
	if s == nil || s.store == nil || s.anomaly == nil {
		return SettingsView{}, errors.New("anomaly settings service not configured")
	}
	_ = ctx
	out, err := validateSettings(in)
	if err != nil {
		return SettingsView{}, err
	}
	out.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.store.Save(out); err != nil {
		return SettingsView{}, err
	}
	s.anomaly.ApplySettings(out)
	if s.onInterval != nil {
		s.onInterval(time.Duration(out.ScanIntervalMin) * time.Minute)
	}
	return s.GetView()
}

func (s *SettingsService) loadNormalized() (Settings, error) {
	if s == nil || s.store == nil {
		return normalizeSettings(s.seed), nil
	}
	raw, err := s.store.Load()
	if err != nil {
		return Settings{}, err
	}
	if isEmptySettings(raw) {
		return normalizeSettings(s.seed), nil
	}
	return normalizeSettings(raw), nil
}

func isEmptySettings(st Settings) bool {
	return !st.Enabled &&
		st.ScanIntervalMin == 0 &&
		st.LearningDays == 0 &&
		st.SuppressHours == 0 &&
		!st.IncludePrivate &&
		st.NewCountryMinShare == 0 &&
		st.UpdatedAt == ""
}

func normalizeSettings(st Settings) Settings {
	if st.ScanIntervalMin < minScanIntervalMin {
		st.ScanIntervalMin = 5
	}
	if st.ScanIntervalMin > maxScanIntervalMin {
		st.ScanIntervalMin = maxScanIntervalMin
	}
	if st.LearningDays < minLearningDays {
		st.LearningDays = 3
	}
	if st.LearningDays > maxLearningDays {
		st.LearningDays = maxLearningDays
	}
	if st.SuppressHours < minSuppressHours {
		st.SuppressHours = 24
	}
	if st.SuppressHours > maxSuppressHours {
		st.SuppressHours = maxSuppressHours
	}
	if st.NewCountryMinShare <= 0 {
		st.NewCountryMinShare = 0.05
	}
	if st.NewCountryMinShare < minNewCountryShare {
		st.NewCountryMinShare = minNewCountryShare
	}
	if st.NewCountryMinShare > maxNewCountryShare {
		st.NewCountryMinShare = maxNewCountryShare
	}
	return st
}

func validateSettings(in Settings) (Settings, error) {
	if in.ScanIntervalMin < minScanIntervalMin || in.ScanIntervalMin > maxScanIntervalMin {
		return Settings{}, fmt.Errorf("%w: scan_interval_min out of range", ErrInvalidAnomalySettings)
	}
	if in.LearningDays < minLearningDays || in.LearningDays > maxLearningDays {
		return Settings{}, fmt.Errorf("%w: learning_days out of range", ErrInvalidAnomalySettings)
	}
	if in.SuppressHours < minSuppressHours || in.SuppressHours > maxSuppressHours {
		return Settings{}, fmt.Errorf("%w: suppress_hours out of range", ErrInvalidAnomalySettings)
	}
	if in.NewCountryMinShare < minNewCountryShare || in.NewCountryMinShare > maxNewCountryShare {
		return Settings{}, fmt.Errorf("%w: new_country_min_share out of range", ErrInvalidAnomalySettings)
	}
	out := normalizeSettings(in)
	out.Enabled = in.Enabled
	out.IncludePrivate = in.IncludePrivate
	return out, nil
}
