package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateConfigRejectsMalformedCriticalInteger(t *testing.T) {
	t.Setenv("INGEST_WORKERS", "abc")

	err := FromEnv().ValidateConfig()
	if err == nil || !strings.Contains(err.Error(), "INGEST_WORKERS") {
		t.Fatalf("ValidateConfig() error = %v, want INGEST_WORKERS parse error", err)
	}
}

func TestValidateConfigAllowsUnsetCriticalValues(t *testing.T) {
	unsetEnv(t, "INGEST_WORKERS")
	unsetEnv(t, "AUTH_DISABLED")

	if err := FromEnv().ValidateConfig(); err != nil {
		t.Fatalf("ValidateConfig() error = %v, want nil for unset values", err)
	}
}

func TestValidateConfigRejectsMalformedSecurityBoolean(t *testing.T) {
	t.Setenv("AUTH_DISABLED", "maybe")

	err := FromEnv().ValidateConfig()
	if err == nil || !strings.Contains(err.Error(), "AUTH_DISABLED") {
		t.Fatalf("ValidateConfig() error = %v, want AUTH_DISABLED parse error", err)
	}
}

func TestGeoSnapshotFileDefaultBesideUsers(t *testing.T) {
	unsetEnv(t, "GEOIP_SNAPSHOT_FILE")
	got := geoSnapshotFile("/app/data/users.json")
	want := filepath.Join("/app/data", "geo_index.snap")
	if got != want {
		t.Fatalf("geoSnapshotFile() = %q, want %q", got, want)
	}
}

func TestGeoSnapshotFileOffAndOverride(t *testing.T) {
	t.Setenv("GEOIP_SNAPSHOT_FILE", "off")
	if geoSnapshotFile("/app/data/users.json") != "" {
		t.Fatal("GEOIP_SNAPSHOT_FILE=off must disable snapshot")
	}
	t.Setenv("GEOIP_SNAPSHOT_FILE", "/var/lib/geo_index.snap")
	if geoSnapshotFile("/app/data/users.json") != "/var/lib/geo_index.snap" {
		t.Fatalf("override: %q", geoSnapshotFile("/app/data/users.json"))
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}
