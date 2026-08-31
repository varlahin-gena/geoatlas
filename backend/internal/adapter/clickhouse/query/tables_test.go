package query

import "testing"

func TestMapShadowPairsMatchLiveAndBackupTables(t *testing.T) {
	live := LiveTables()
	bak := BackupTables()
	pairs := MapShadowPairs()
	if len(pairs) != 6 {
		t.Fatalf("pairs=%d want 6", len(pairs))
	}
	want := [][2]string{
		{live.Logs, bak.Logs},
		{live.EdgesDaily, bak.EdgesDaily},
		{live.EdgesHourly, bak.EdgesHourly},
		{live.EdgesCity, bak.EdgesCity},
		{live.EdgesCountry, bak.EdgesCountry},
		{live.EdgesContinent, bak.EdgesContinent},
	}
	for i, p := range pairs {
		if p != want[i] {
			t.Fatalf("pair[%d]=%v want %v", i, p, want[i])
		}
	}
	names := MapShadowNames()
	if len(names) != 6 || names[0] != bak.Logs {
		t.Fatalf("names=%v", names)
	}
}
