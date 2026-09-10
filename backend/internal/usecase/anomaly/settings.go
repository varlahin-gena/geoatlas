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

	minThresholdInt     = 1
	maxThresholdInt     = 100_000
	maxThresholdBytes   = uint64(10_000_000_000)
	maxThresholdCount   = uint64(1_000_000)
	minSurgeRatio       = 1.0
	maxSurgeRatio       = 100.0
	minBeaconRegularity = 0.0
	maxBeaconRegularity = 1.0
	minBeaconHours      = 1
	maxBeaconHours      = 168
)

var (
	ErrInvalidAnomalySettings = apperr.InvalidInput("invalid anomaly settings")
)

// Settings — редактируемые параметры движка (JSON на диске).
type Settings struct {
	Enabled            bool        `json:"enabled"`
	ScanIntervalMin    int         `json:"scan_interval_min"`
	LearningDays       int         `json:"learning_days"`
	SuppressHours      int         `json:"suppress_hours"`
	IncludePrivate     bool        `json:"include_private"`
	NewCountryMinShare float64     `json:"new_country_min_share"`
	Thresholds         *Thresholds `json:"thresholds,omitempty"`
	UpdatedAt          string      `json:"updated_at,omitempty"`
}

// SettingsStore — персистентность настроек.
type SettingsStore interface {
	Load() (Settings, error)
	Save(s Settings) error
}

// SettingsView — ответ GET /api/anomalies/settings.
type SettingsView struct {
	Settings          Settings   `json:"settings"`
	InstallProfile    string     `json:"install_profile"`
	Thresholds        Thresholds `json:"thresholds"`
	ThresholdDefaults Thresholds `json:"threshold_defaults"`
	Status            ScanStatus `json:"status"`
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

func DefaultSettingsFromConfig(cfg config.AnomalyConfig) Settings {
	min := int(cfg.ScanInterval / time.Minute)
	if min < minScanIntervalMin {
		min = 5
	}
	share := cfg.NewCountryMinShare
	if share <= 0 {
		share = 0.05
	}
	return Settings{
		Enabled:            true,
		ScanIntervalMin:    min,
		LearningDays:       cfg.LearningDays,
		SuppressHours:      cfg.SuppressHours,
		IncludePrivate:     cfg.IncludePrivate,
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

func (s *SettingsService) GetView(ctx context.Context) (SettingsView, error) {
	if s == nil || s.anomaly == nil {
		return SettingsView{}, errors.New("anomaly settings service not configured")
	}
	st, err := s.loadNormalized()
	if err != nil {
		return SettingsView{}, err
	}
	cfg := s.anomaly.cfgSnapshot()
	profile := cfg.InstallProfile
	if profile == "" {
		profile = "medium"
	}
	return SettingsView{
		Settings:          st,
		InstallProfile:    profile,
		Thresholds:        EffectiveThresholds(profile, st.Thresholds, st.NewCountryMinShare),
		ThresholdDefaults: ThresholdsForProfile(profile),
		Status:            s.anomaly.LiveStatus(ctx),
	}, nil
}

func (s *SettingsService) Update(ctx context.Context, in Settings) (SettingsView, error) {
	if s == nil || s.store == nil || s.anomaly == nil {
		return SettingsView{}, errors.New("anomaly settings service not configured")
	}
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
	return s.GetView(ctx)
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
		st.Thresholds == nil &&
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
	if in.Thresholds != nil {
		if err := validateThresholds(*in.Thresholds); err != nil {
			return Settings{}, err
		}
	}
	out := normalizeSettings(in)
	out.Enabled = in.Enabled
	out.IncludePrivate = in.IncludePrivate
	out.Thresholds = in.Thresholds
	return out, nil
}

func validateThresholds(th Thresholds) error {
	checkInt := func(name string, v int) error {
		if v < minThresholdInt || v > maxThresholdInt {
			return fmt.Errorf("%w: %s out of range", ErrInvalidAnomalySettings, name)
		}
		return nil
	}
	checkUint := func(name string, v uint64) error {
		if v < uint64(minThresholdInt) || v > maxThresholdCount {
			return fmt.Errorf("%w: %s out of range", ErrInvalidAnomalySettings, name)
		}
		return nil
	}
	checkBytes := func(name string, v uint64) error {
		if v < uint64(minThresholdInt) || v > maxThresholdBytes {
			return fmt.Errorf("%w: %s out of range", ErrInvalidAnomalySettings, name)
		}
		return nil
	}
	checkRatio := func(name string, v float64) error {
		if v < minSurgeRatio || v > maxSurgeRatio {
			return fmt.Errorf("%w: %s out of range", ErrInvalidAnomalySettings, name)
		}
		return nil
	}
	checkShare := func(name string, v float64) error {
		if v < minNewCountryShare || v > maxNewCountryShare {
			return fmt.Errorf("%w: %s out of range", ErrInvalidAnomalySettings, name)
		}
		return nil
	}
	checkRegularity := func(name string, v float64) error {
		if v < minBeaconRegularity || v > maxBeaconRegularity {
			return fmt.Errorf("%w: %s out of range", ErrInvalidAnomalySettings, name)
		}
		return nil
	}

	checks := []error{
		checkInt("port_scan_ports", th.PortScanPorts),
		checkInt("port_scan_events", th.PortScanEvents),
		checkInt("horizontal_hosts", th.HorizontalHosts),
		checkInt("horizontal_events", th.HorizontalEvents),
		checkRatio("surge_ratio", th.SurgeRatio),
		checkUint("surge_abs_min", th.SurgeAbsMin),
		checkUint("surge_floor", th.SurgeFloor),
		checkUint("new_country_min", th.NewCountryMin),
		checkUint("new_country_baseline", th.NewCountryBaseline),
		checkShare("new_country_min_share", th.NewCountryMinShare),
		checkUint("rep_min_events", th.RepMinEvents),
		checkRatio("byte_surge_ratio", th.ByteSurgeRatio),
		checkBytes("byte_surge_abs_min", th.ByteSurgeAbsMin),
		checkBytes("byte_surge_floor", th.ByteSurgeFloor),
		checkBytes("beacon_max_avg_bytes", th.BeaconMaxAvgBytes),
		checkRegularity("beacon_min_regularity", th.BeaconMinRegularity),
		checkInt("lateral_hosts", th.LateralHosts),
		checkInt("lateral_events", th.LateralEvents),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	if th.BeaconMinHours < minBeaconHours || th.BeaconMinHours > maxBeaconHours {
		return fmt.Errorf("%w: beacon_min_hours out of range", ErrInvalidAnomalySettings)
	}
	return nil
}
