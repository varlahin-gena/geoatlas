package reputation

import (
	"errors"
	"fmt"
	"time"

	"geoatlas/internal/apperr"
	"geoatlas/internal/config"
)

const (
	minFetchIntervalHours = 1
	maxFetchIntervalHours = 168 // 7 дней
)

var ErrInvalidReputationSettings = apperr.InvalidInput("invalid reputation settings")

// Settings — редактируемые параметры модуля репутации (JSON на диске).
type Settings struct {
	FetchIntervalHours int    `json:"fetch_interval_hours"`
	UpdatedAt          string `json:"updated_at,omitempty"`
}

// SettingsStore — персистентность настроек.
type SettingsStore interface {
	Load() (Settings, error)
	Save(s Settings) error
}

// IntervalUpdater — hot-reload интервала планировщика фидов.
type IntervalUpdater func(time.Duration)

// SettingsService — GET/PUT настроек + Apply к job.
type SettingsService struct {
	store      SettingsStore
	seed       Settings
	onInterval IntervalUpdater
}

func NewSettingsService(store SettingsStore, seed Settings, onInterval IntervalUpdater) *SettingsService {
	return &SettingsService{
		store:      store,
		seed:       normalizeSettings(seed),
		onInterval: onInterval,
	}
}

func DefaultSettingsFromConfig(cfg config.ReputationConfig) Settings {
	hours := int(cfg.FetchInterval / time.Hour)
	if hours < minFetchIntervalHours {
		if cfg.FetchInterval > 0 {
			hours = minFetchIntervalHours
		} else {
			hours = 6
		}
	}
	if hours > maxFetchIntervalHours {
		hours = maxFetchIntervalHours
	}
	return Settings{FetchIntervalHours: hours}
}

func (s *SettingsService) LoadAndApply() (Settings, error) {
	if s == nil {
		return Settings{}, errors.New("reputation settings service not configured")
	}
	st, err := s.loadNormalized()
	if err != nil {
		return Settings{}, err
	}
	s.applyInterval(st)
	return st, nil
}

func (s *SettingsService) Get() (Settings, error) {
	if s == nil {
		return Settings{}, errors.New("reputation settings service not configured")
	}
	return s.loadNormalized()
}

func (s *SettingsService) Update(in Settings) (Settings, error) {
	if s == nil || s.store == nil {
		return Settings{}, errors.New("reputation settings service not configured")
	}
	out, err := validateSettings(in)
	if err != nil {
		return Settings{}, err
	}
	out.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.store.Save(out); err != nil {
		return Settings{}, err
	}
	s.applyInterval(out)
	return out, nil
}

func (s *SettingsService) applyInterval(st Settings) {
	if s == nil || s.onInterval == nil {
		return
	}
	s.onInterval(time.Duration(st.FetchIntervalHours) * time.Hour)
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
	return st.FetchIntervalHours == 0 && st.UpdatedAt == ""
}

func normalizeSettings(st Settings) Settings {
	if st.FetchIntervalHours < minFetchIntervalHours {
		st.FetchIntervalHours = 6
	}
	if st.FetchIntervalHours > maxFetchIntervalHours {
		st.FetchIntervalHours = maxFetchIntervalHours
	}
	return st
}

func validateSettings(in Settings) (Settings, error) {
	if in.FetchIntervalHours < minFetchIntervalHours || in.FetchIntervalHours > maxFetchIntervalHours {
		return Settings{}, fmt.Errorf("%w: fetch_interval_hours out of range [%d, %d]",
			ErrInvalidReputationSettings, minFetchIntervalHours, maxFetchIntervalHours)
	}
	return Settings{FetchIntervalHours: in.FetchIntervalHours}, nil
}
