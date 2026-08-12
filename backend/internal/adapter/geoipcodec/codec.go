package geoipcodec

import (
	"io"

	"network_monitor/internal/geoip"
	"network_monitor/internal/model"
	usecasegeo "network_monitor/internal/usecase/geo"
)

// Codec wraps geoip helpers for usecase/geo.RangeCodec.
type Codec struct{}

func New() *Codec { return &Codec{} }

var _ usecasegeo.RangeCodec = (*Codec)(nil)

func (Codec) ReadCSV(r io.Reader) ([]model.GeoRange, error) { return geoip.ReadCSV(r) }
func (Codec) ReadCSVSnapshot(r io.Reader) ([]model.GeoRange, *geoip.BuiltSnapshot, error) {
	return geoip.ReadCSVSnapshot(r)
}
func (Codec) WriteCSV(w io.Writer, ranges []model.GeoRange) error {
	return geoip.WriteCSV(w, ranges)
}
func (Codec) Normalize(ranges []model.GeoRange) ([]model.GeoRange, int) {
	return geoip.NormalizeRanges(ranges)
}
func (Codec) CheckNonOverlapping(ranges []model.GeoRange) error {
	return geoip.CheckNonOverlapping(ranges)
}
func (Codec) ParseEntry(network, country, region, city string, lat, lon float64) (model.GeoRange, error) {
	return geoip.ParseRangeEntry(network, country, region, city, lat, lon)
}
func (Codec) ParseNetwork(network string) (uint32, uint32, bool) {
	return geoip.ParseNetworkField(network)
}
func (Codec) FormatNetwork(start, end uint32) string {
	return geoip.FormatNetwork(start, end)
}
