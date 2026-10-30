package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAddrAndTokenHelpers(t *testing.T) {
	c := Config{ClickHouse: ClickHouseConfig{Host: "h", Port: "9000"}, Auth: AuthConfig{
		APIOpsToken: "ops", APIOpsPreviousToken: "prev",
	}}
	if c.ClickHouse.Addr() != "h:9000" || c.ClickHouseAddr() != "h:9000" {
		t.Fatal(c.ClickHouseAddr())
	}
	toks := c.APIOpsTokens()
	if len(toks) != 2 {
		t.Fatalf("%v", toks)
	}
}

func TestEnvParserErrorPaths(t *testing.T) {
	t.Setenv("GA_TEST_FLOAT", "nope")
	t.Setenv("GA_TEST_PORT", "99999")
	t.Setenv("GA_TEST_I64", "x")
	t.Setenv("GA_TEST_DUR", "0")
	t.Setenv("GA_TEST_FLEX", "bad")
	p := &envParser{}
	if p.float("GA_TEST_FLOAT", 1.5) != 1.5 {
		t.Fatal("float")
	}
	if p.port("GA_TEST_PORT", "8123") != "8123" {
		t.Fatal("port")
	}
	if p.int64("GA_TEST_I64", 7) != 7 {
		t.Fatal("i64")
	}
	if p.durationSeconds("GA_TEST_DUR", time.Minute) != time.Minute {
		t.Fatal("dur")
	}
	if p.durationFlexible("GA_TEST_FLEX", time.Hour) != time.Hour {
		t.Fatal("flex")
	}
	t.Setenv("GA_TEST_FLEX2", "6h")
	if p.durationFlexible("GA_TEST_FLEX2", time.Hour) != 6*time.Hour {
		t.Fatal("flex ok")
	}
	if envOr("GA_TEST_MISSING_XYZ", "def") != "def" {
		t.Fatal("envOr")
	}
}

func TestResolveGeoUploadLimits(t *testing.T) {
	var nilC *Config
	nilC.ResolveGeoUploadLimits()

	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	body := `{"profile":"large","limits":{"backend":{"memory_gb":8}},"host":{},"capacity":{}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{InstallProfilePath: path}
	cfg.ResolveGeoUploadLimits()
	if cfg.Geo.MaxUploadSize <= 0 || cfg.Geo.MaxUploadRanges <= 0 {
		t.Fatalf("%+v", cfg.Geo)
	}

	t.Setenv("GEOIP_UPLOAD_MAX_BYTES", "12345")
	t.Setenv("GEOIP_UPLOAD_MAX_RANGES", "99")
	// env set → keep existing Geo values (do not overwrite from profile)
	cfg2 := &Config{InstallProfilePath: path, Geo: GeoConfig{MaxUploadSize: 111, MaxUploadRanges: 222}}
	cfg2.ResolveGeoUploadLimits()
	if cfg2.Geo.MaxUploadSize != 111 || cfg2.Geo.MaxUploadRanges != 222 {
		t.Fatalf("%+v", cfg2.Geo)
	}
	if !envInt64Set("GEOIP_UPLOAD_MAX_BYTES") || envInt64Set("GA_NEVER_SET_XYZ") {
		t.Fatal("envInt64Set")
	}
	if !envIntSet("GEOIP_UPLOAD_MAX_RANGES") || envIntSet("GA_NEVER_SET_XYZ") {
		t.Fatal("envIntSet")
	}
}
