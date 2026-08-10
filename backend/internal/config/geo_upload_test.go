package config

import "testing"

func TestGeoUploadDefaultsForBackendMemoryGB(t *testing.T) {
	cases := []struct {
		gb        int
		minBytes  int64
		maxRanges int
	}{
		{1, 256 << 20, 2_000_000},
		{2, 512 << 20, 4_000_000},
		{4, 1 << 30, 5_000_000},
		{8, 1536 << 20, 8_000_000},
		{16, 2 << 30, 12_000_000},
	}
	for _, tc := range cases {
		b, r := GeoUploadDefaultsForBackendMemoryGB(tc.gb)
		if b != tc.minBytes || r != tc.maxRanges {
			t.Fatalf("gb=%d got bytes=%d ranges=%d want %d/%d", tc.gb, b, r, tc.minBytes, tc.maxRanges)
		}
	}
}
