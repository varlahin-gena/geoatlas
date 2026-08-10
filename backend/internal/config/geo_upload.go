package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"network_monitor/internal/installprofile"
)

// GeoUploadDefaultsForBackendMemoryGB — безопасные лимиты upload/replace GeoIP
// относительно cgroup backend (пиковый RAM ≈ старый индекс + новый CSV в памяти).
func GeoUploadDefaultsForBackendMemoryGB(gb int) (maxBytes int64, maxRanges int) {
	if gb <= 0 {
		gb = 2
	}
	// Байт-лимиты ориентированы на реальные GeoIP CSV (~400–500+ МиБ),
	// которые на practice проходят без OOM при достаточной RAM backend.
	// Пик при повторном replace по-прежнему режет checkUploadLimits (409).
	switch {
	case gb <= 1:
		return 256 << 20, 800_000 // tiny — тесно; лучше не 500+ МиБ
	case gb <= 2:
		return 512 << 20, 2_000_000 // small — типичный ~463 МиБ CSV
	case gb <= 4:
		return 1 << 30, 3_000_000 // medium
	case gb <= 8:
		return 1536 << 20, 5_000_000 // large (~1.5 GiB)
	default:
		return 2 << 30, 8_000_000 // xlarge
	}
}

// ResolveGeoUploadLimits подставляет лимиты из install-profile, если env не задан.
// Вызывать после FromEnv (нужен InstallProfilePath).
func (c *Config) ResolveGeoUploadLimits() {
	if c == nil {
		return
	}
	bytesFromEnv := envInt64Set("GEOIP_UPLOAD_MAX_BYTES") || envInt64Set("MAX_GEO_UPLOAD_SIZE")
	rangesFromEnv := envIntSet("GEOIP_UPLOAD_MAX_RANGES")

	gb := 2
	src := "default"
	if p, err := installprofile.Load(c.InstallProfilePath); err == nil && p != nil {
		if p.Limits.Backend.MemoryGB > 0 {
			gb = p.Limits.Backend.MemoryGB
			src = "install-profile"
		}
	}
	defBytes, defRanges := GeoUploadDefaultsForBackendMemoryGB(gb)

	if !bytesFromEnv {
		c.MaxGeoUploadSize = defBytes
	}
	if !rangesFromEnv {
		c.MaxGeoUploadRanges = defRanges
	}
	if c.MaxGeoUploadSize <= 0 {
		c.MaxGeoUploadSize = defBytes
	}
	if c.MaxGeoUploadRanges <= 0 {
		c.MaxGeoUploadRanges = defRanges
	}

	slog.Info("geo upload limits",
		"max_bytes", c.MaxGeoUploadSize,
		"max_ranges", c.MaxGeoUploadRanges,
		"backend_memory_gb", gb,
		"source", src,
		"bytes_from_env", bytesFromEnv,
		"ranges_from_env", rangesFromEnv,
	)
}

func envInt64Set(key string) bool {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return false
	}
	_, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return err == nil
}

func envIntSet(key string) bool {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return false
	}
	_, err := strconv.Atoi(strings.TrimSpace(raw))
	return err == nil
}

// firstEnvInt64 returns the first set env among keys, else def.
func firstEnvInt64(p *envParser, def int64, keys ...string) int64 {
	for _, key := range keys {
		raw, ok := os.LookupEnv(key)
		if !ok {
			continue
		}
		if n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil {
			return n
		}
		p.invalid(key)
		return def
	}
	return def
}

func firstEnvInt(p *envParser, def int, keys ...string) int {
	for _, key := range keys {
		raw, ok := os.LookupEnv(key)
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			return n
		}
		p.invalid(key)
		return def
	}
	return def
}
