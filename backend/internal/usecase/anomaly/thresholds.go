package anomaly

import "strings"

// Thresholds — пороги детекторов для install profile.
type Thresholds struct {
	PortScanPorts      int
	PortScanEvents     int
	HorizontalHosts    int
	HorizontalEvents   int
	SurgeRatio         float64
	SurgeAbsMin        uint64
	SurgeFloor         uint64
	NewCountryMin      uint64
	NewCountryBaseline uint64
	NewCountryMinShare float64
	RepMinEvents       uint64
}

func ThresholdsForProfile(name string) Thresholds {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "tiny":
		return Thresholds{
			PortScanPorts: 30, PortScanEvents: 50,
			HorizontalHosts: 20, HorizontalEvents: 40,
			SurgeRatio: 5, SurgeAbsMin: 80, SurgeFloor: 10,
			NewCountryMin: 5, NewCountryBaseline: 10, NewCountryMinShare: 0.05, RepMinEvents: 3,
		}
	case "small":
		return Thresholds{
			PortScanPorts: 40, PortScanEvents: 80,
			HorizontalHosts: 30, HorizontalEvents: 60,
			SurgeRatio: 5, SurgeAbsMin: 120, SurgeFloor: 15,
			NewCountryMin: 5, NewCountryBaseline: 10, NewCountryMinShare: 0.05, RepMinEvents: 3,
		}
	case "large":
		return Thresholds{
			PortScanPorts: 75, PortScanEvents: 150,
			HorizontalHosts: 60, HorizontalEvents: 120,
			SurgeRatio: 4, SurgeAbsMin: 400, SurgeFloor: 40,
			NewCountryMin: 5, NewCountryBaseline: 10, NewCountryMinShare: 0.05, RepMinEvents: 3,
		}
	case "xlarge":
		return Thresholds{
			PortScanPorts: 100, PortScanEvents: 250,
			HorizontalHosts: 80, HorizontalEvents: 160,
			SurgeRatio: 4, SurgeAbsMin: 800, SurgeFloor: 50,
			NewCountryMin: 8, NewCountryBaseline: 15, NewCountryMinShare: 0.05, RepMinEvents: 3,
		}
	default: // medium and unknown
		return Thresholds{
			PortScanPorts: 50, PortScanEvents: 100,
			HorizontalHosts: 40, HorizontalEvents: 80,
			SurgeRatio: 5, SurgeAbsMin: 200, SurgeFloor: 20,
			NewCountryMin: 5, NewCountryBaseline: 10, NewCountryMinShare: 0.05, RepMinEvents: 3,
		}
	}
}

func scoreAgainst(observed, threshold float64, severity string) float32 {
	w := float32(0.6)
	switch severity {
	case SeverityHigh:
		w = 1
	case SeverityInfo:
		w = 0.3
	}
	if threshold <= 0 {
		return w
	}
	ratio := observed / threshold
	if ratio > 1 {
		ratio = 1
	}
	if ratio < 0 {
		ratio = 0
	}
	return w * float32(ratio)
}
