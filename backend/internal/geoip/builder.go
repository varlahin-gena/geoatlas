package geoip

import "network_monitor/internal/model"

// BuiltSnapshot — готовый compact snapshot для атомарной публикации в Index.
type BuiltSnapshot struct {
	snap    *snapshot
	skipped int
}

func (b *BuiltSnapshot) RangeCount() int {
	if b == nil || b.snap == nil {
		return 0
	}
	return len(b.snap.rows)
}

func (b *BuiltSnapshot) ApproxBytes() uint64 {
	if b == nil || b.snap == nil {
		return 0
	}
	return b.snap.approxBytes
}

func (b *BuiltSnapshot) Skipped() int {
	if b == nil {
		return 0
	}
	return b.skipped
}

// Ranges — материализация compact snapshot в []GeoRange (для stamp/тестов).
func (b *BuiltSnapshot) Ranges() []model.GeoRange {
	if b == nil || b.snap == nil || len(b.snap.rows) == 0 {
		return nil
	}
	out := make([]model.GeoRange, len(b.snap.rows))
	for i, row := range b.snap.rows {
		out[i] = b.snap.toGeoRange(row)
	}
	return out
}

// CompactBuilder строит compact snapshot потоково по уже отсортированным диапазонам.
// Невалидные и пересекающиеся диапазоны пропускаются так же, как NormalizeRanges.
type CompactBuilder struct {
	countries dictionaryBuilder
	regions   dictionaryBuilder
	cities    dictionaryBuilder
	rows      []rangeRow
	prev      *rangeRow
	skipped   int
}

func NewCompactBuilder(capHint int) *CompactBuilder {
	return &CompactBuilder{
		countries: newDictionaryBuilder(min(capHint, 256)),
		regions:   newDictionaryBuilder(min(capHint, 512)),
		cities:    newDictionaryBuilder(min(capHint, 2048)),
		rows:      make([]rangeRow, 0, max(capHint, 0)),
	}
}

func (b *CompactBuilder) AddRange(g model.GeoRange) bool {
	if b == nil {
		return false
	}
	if g.EndIP < g.StartIP {
		b.skipped++
		return false
	}
	if b.prev != nil && g.StartIP <= b.prev.EndIP {
		b.skipped++
		return false
	}
	row := rangeRow{
		StartIP:   g.StartIP,
		EndIP:     g.EndIP,
		CountryID: b.countries.ID(g.Country),
		RegionID:  b.regions.ID(g.Region),
		CityID:    b.cities.ID(g.City),
		Lat:       g.Lat,
		Lon:       g.Lon,
	}
	b.rows = append(b.rows, row)
	b.prev = &b.rows[len(b.rows)-1]
	return true
}

func (b *CompactBuilder) Build() *BuiltSnapshot {
	if b == nil {
		return &BuiltSnapshot{snap: &snapshot{}}
	}
	snap := &snapshot{
		rows:      b.rows,
		countries: b.countries.values,
		regions:   b.regions.values,
		cities:    b.cities.values,
		approxBytes: uint64(cap(b.rows))*rangeRowSize +
			uint64(cap(b.countries.values))*stringHeaderSize + b.countries.bytes +
			uint64(cap(b.regions.values))*stringHeaderSize + b.regions.bytes +
			uint64(cap(b.cities.values))*stringHeaderSize + b.cities.bytes,
	}
	return &BuiltSnapshot{snap: snap, skipped: b.skipped}
}
