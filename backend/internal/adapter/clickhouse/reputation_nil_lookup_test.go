package clickhouse_test

import (
	"testing"

	chadapter "network_monitor/internal/adapter/clickhouse"
	"network_monitor/internal/model"
)

func TestReloadableReputationIndexLookupNilReceiver(t *testing.T) {
	var idx *chadapter.ReloadableReputationIndex
	var lookuper interface {
		Lookup(string) []model.ReputationHit
	} = idx
	if lookuper == nil {
		t.Fatal("typed nil should not equal nil interface")
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Lookup on typed-nil index panicked: %v", r)
		}
	}()
	got := lookuper.Lookup("1.1.1.1")
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}
