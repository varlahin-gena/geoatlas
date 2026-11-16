package config

import "testing"

func TestGeoUploadDefaultsForBackendMemoryGB(t *testing.T) {
	cases := []struct {
		gb        float64
		minBytes  int64
		maxRanges int
	}{
		{0.75, 256 << 20, 2_000_000},
		{1, 256 << 20, 2_000_000},
		{2, 512 << 20, 4_000_000},
		{4, 1 << 30, 5_000_000},
		{8, 1536 << 20, 8_000_000},
		{16, 2 << 30, 12_000_000},
	}
	for _, tc := range cases {
		b, r := GeoUploadDefaultsForBackendMemoryGB(tc.gb)
		if b != tc.minBytes || r != tc.maxRanges {
			t.Fatalf("gb=%g got bytes=%d ranges=%d want %d/%d", tc.gb, b, r, tc.minBytes, tc.maxRanges)
		}
	}
}

func TestBackendSoftMemLimitBytes(t *testing.T) {
	if got := BackendSoftMemLimitBytes(0); got != 0 {
		t.Fatalf("zero gb: got=%d", got)
	}
	// 2 GiB profile → 1.5 GiB soft (не 1 GiB из-за uint64*3/4 до умножения на GiB).
	want := uint64(2) * 3 * (1 << 30) / 4
	if got := BackendSoftMemLimitBytes(2); got != want {
		t.Fatalf("gb=2 got=%d want=%d", got, want)
	}
	// 0.75 GiB cgroup → 75% = 576 MiB.
	wantTiny := uint64(576 << 20)
	if got := BackendSoftMemLimitBytes(0.75); got != wantTiny {
		t.Fatalf("gb=0.75 got=%d want=%d", got, wantTiny)
	}
}
