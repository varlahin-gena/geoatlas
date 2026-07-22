package model

// GeoMissingIPRow — кандидат без координат в traffic_logs (до live GeoIP-фильтра).
type GeoMissingIPRow struct {
	IP         string
	Count      uint64
	AsSrc      uint64
	AsDst      uint64
	SamplePeer string
	LogCountry string
	LogCity    string
	LogLat     float64
	LogLon     float64
}
