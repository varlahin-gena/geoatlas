package geoip

import (
	"strconv"
	"testing"

	"geoatlas/internal/model"
)

func benchIndex(ranges int) *Index {
	idx := New()
	rs := make([]model.GeoRange, ranges)
	span := uint32(uint64(1<<32) / uint64(ranges))
	for i := range rs {
		start := uint32(i) * span
		end := start + span - 1
		if i == ranges-1 {
			end = ^uint32(0)
		}
		rs[i] = model.GeoRange{
			StartIP: start,
			EndIP:   end,
			Country: "RU",
			City:    "c" + strconv.Itoa(i%64),
			Lat:     55,
			Lon:     37,
		}
	}
	idx.ReplaceRanges(rs)
	return idx
}

func BenchmarkLookupHit(b *testing.B) {
	idx := benchIndex(32768)
	b.ReportAllocs()
	for b.Loop() {
		_ = idx.Lookup("10.1.100.11")
	}
}

func BenchmarkLookupMissInvalid(b *testing.B) {
	idx := benchIndex(32768)
	b.ReportAllocs()
	for b.Loop() {
		_ = idx.Lookup("not-an-ip")
	}
}

func BenchmarkIPToUint32(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = IPToUint32("10.1.100.11")
	}
}
