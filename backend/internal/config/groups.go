package config

import "time"

// BackupConfig — UI + native BACKUP TO Disk('backups').
type BackupConfig struct {
	Enabled      bool
	IncludeEdges bool
	IncludeAuth  bool
	Dir          string
	ScheduleFile string
	Keep         int
}

// TLSConfig — host cert paths and HTTPS UI settings (layout matches usecase/tls.Config).
type TLSConfig struct {
	CertDir      string
	CertFile     string
	KeyFile      string
	HTTPSEnabled string
	HTTPSPort    string
	HTTPRedirect string
	ReloadCmd    string
}

// AuthConfig — UI sessions, seed users, and API Bearer tokens.
type AuthConfig struct {
	Disabled            bool // AUTH_DISABLED
	SessionSecret       string
	SessionTTLHours     int
	AdminUser           string
	AdminPassword       string
	AdminMustReset      bool
	OperatorUser        string
	OperatorPassword    string
	UsersFile           string
	APITokensFile       string
	APIAuthToken        string
	APIAuthPreviousToken string
	APIAuthDisabled     bool // API_AUTH_DISABLED
	APIOpsToken         string
	APIOpsPreviousToken string
}

// IngestConfig — syslog ingest listeners, queues, and peer auth.
type IngestConfig struct {
	ListenAddr     string
	UDPListenAddr  string
	TCPListenAddr  string
	BatchSize      int
	QueueSize      int
	QueueMaxBytes  int
	Workers        int
	FlushSec       int
	MaxConnections int
	ConnIdleSec    int
	SharedSecret   string
	AllowFrom      string
}

// HTTPThreatConfig — proxy gate and API rate limits.
type HTTPThreatConfig struct {
	TrustedProxies   string
	RequireProxy     bool
	APIRateLimitRPS  float64
	APIRateLimitBurst int
}

// GeoConfig — GeoIP snapshot, upload limits, and enrich/backfill.
type GeoConfig struct {
	SnapshotFile           string
	MaxUploadSize          int64
	MaxUploadRanges        int
	EnrichOnIngest         bool
	BackfillLookbackDays   int
	SkipStartupBackfill    bool
}

// ReputationConfig — offline lists and feed refresh.
type ReputationConfig struct {
	MaxUploadSize int64
	FetchEnabled  bool
	FetchInterval time.Duration
	Feeds         []ReputationFeed
	FeedsFile     string
}

// AnomalyConfig — map anomaly engine + settings file.
type AnomalyConfig struct {
	Enabled                       bool
	ScanInterval                  time.Duration
	IncludePrivate                bool
	LearningDays                  int
	SuppressHours                 int
	NewCountryMinShare            float64
	NewCountryRepeatCooldownHours int
	SettingsFile                  string
}

// LogConfig — process log level/format.
type LogConfig struct {
	Level  string
	Format string
}
