package anomalystore

import (
	"context"
	"strings"
	"testing"

	usecaseanomaly "network_monitor/internal/usecase/anomaly"
)

func TestDisplayIP(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"0.0.0.0", ""},
		{"  203.0.113.5  ", "203.0.113.5"},
		{"not-an-ip", "not-an-ip"},
	}
	for _, tc := range tests {
		if got := displayIP(tc.in); got != tc.want {
			t.Fatalf("displayIP(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRepositoryInsertNilGuard(t *testing.T) {
	var r *Repository
	if err := r.Insert(context.Background(), []usecaseanomaly.Event{{Code: "port_scan"}}); err != nil {
		t.Fatalf("nil repo insert: %v", err)
	}
	r = &Repository{}
	if err := r.Insert(context.Background(), nil); err != nil {
		t.Fatalf("empty events: %v", err)
	}
}

func TestRepositoryListNilCH(t *testing.T) {
	r := &Repository{}
	_, err := r.List(context.Background(), usecaseanomaly.ListQuery{Limit: 10})
	if err == nil {
		t.Fatal("want error for nil clickhouse")
	}
}

func TestPrivateSrcSQL(t *testing.T) {
	if privateSrcSQL(true) != "" {
		t.Fatal("includePrivate=true should be empty clause")
	}
	got := privateSrcSQL(false)
	if got == "" || !strings.Contains(got, "10.0.0.0") || !strings.Contains(got, "192.168.0.0") {
		t.Fatalf("private filter: %q", got)
	}
}
