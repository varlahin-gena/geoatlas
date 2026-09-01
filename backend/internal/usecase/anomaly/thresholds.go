package anomaly

import "strings"

// Thresholds — пороги детекторов для install profile.
type Thresholds struct {
	PortScanPorts      int     `json:"port_scan_ports"`
	PortScanEvents     int     `json:"port_scan_events"`
	HorizontalHosts    int     `json:"horizontal_hosts"`
	HorizontalEvents   int     `json:"horizontal_events"`
	SurgeRatio         float64 `json:"surge_ratio"`
	SurgeAbsMin        uint64  `json:"surge_abs_min"`
	SurgeFloor         uint64  `json:"surge_floor"`
	NewCountryMin      uint64  `json:"new_country_min"`
	NewCountryBaseline uint64  `json:"new_country_baseline"`
	NewCountryMinShare float64 `json:"new_country_min_share"`
	RepMinEvents       uint64  `json:"rep_min_events"`

	ByteSurgeRatio  float64 `json:"byte_surge_ratio"`
	ByteSurgeAbsMin uint64  `json:"byte_surge_abs_min"`
	ByteSurgeFloor  uint64  `json:"byte_surge_floor"`

	BeaconMinHours      int     `json:"beacon_min_hours"`
	BeaconMaxAvgBytes   uint64  `json:"beacon_max_avg_bytes"`
	BeaconMinRegularity float64 `json:"beacon_min_regularity"`

	LateralHosts  int `json:"lateral_hosts"`
	LateralEvents int `json:"lateral_events"`
}

func ThresholdsForProfile(name string) Thresholds {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "tiny":
		return Thresholds{
			PortScanPorts: 30, PortScanEvents: 50,
			HorizontalHosts: 20, HorizontalEvents: 40,
			SurgeRatio: 5, SurgeAbsMin: 80, SurgeFloor: 10,
			NewCountryMin: 5, NewCountryBaseline: 10, NewCountryMinShare: 0.05, RepMinEvents: 3,
			ByteSurgeRatio: 5, ByteSurgeAbsMin: 50_000_000, ByteSurgeFloor: 1_000_000,
			BeaconMinHours: 8, BeaconMaxAvgBytes: 200_000, BeaconMinRegularity: 0.55,
			LateralHosts: 15, LateralEvents: 30,
		}
	case "small":
		return Thresholds{
			PortScanPorts: 40, PortScanEvents: 80,
			HorizontalHosts: 30, HorizontalEvents: 60,
			SurgeRatio: 5, SurgeAbsMin: 120, SurgeFloor: 15,
			NewCountryMin: 5, NewCountryBaseline: 10, NewCountryMinShare: 0.05, RepMinEvents: 3,
			ByteSurgeRatio: 5, ByteSurgeAbsMin: 80_000_000, ByteSurgeFloor: 2_000_000,
			BeaconMinHours: 10, BeaconMaxAvgBytes: 250_000, BeaconMinRegularity: 0.55,
			LateralHosts: 20, LateralEvents: 40,
		}
	case "large":
		return Thresholds{
			PortScanPorts: 75, PortScanEvents: 150,
			HorizontalHosts: 60, HorizontalEvents: 120,
			SurgeRatio: 4, SurgeAbsMin: 400, SurgeFloor: 40,
			NewCountryMin: 5, NewCountryBaseline: 10, NewCountryMinShare: 0.05, RepMinEvents: 3,
			ByteSurgeRatio: 4, ByteSurgeAbsMin: 200_000_000, ByteSurgeFloor: 10_000_000,
			BeaconMinHours: 12, BeaconMaxAvgBytes: 400_000, BeaconMinRegularity: 0.6,
			LateralHosts: 40, LateralEvents: 80,
		}
	case "xlarge":
		return Thresholds{
			PortScanPorts: 100, PortScanEvents: 250,
			HorizontalHosts: 80, HorizontalEvents: 160,
			SurgeRatio: 4, SurgeAbsMin: 800, SurgeFloor: 50,
			NewCountryMin: 8, NewCountryBaseline: 15, NewCountryMinShare: 0.05, RepMinEvents: 3,
			ByteSurgeRatio: 4, ByteSurgeAbsMin: 400_000_000, ByteSurgeFloor: 20_000_000,
			BeaconMinHours: 14, BeaconMaxAvgBytes: 500_000, BeaconMinRegularity: 0.6,
			LateralHosts: 50, LateralEvents: 100,
		}
	default: // medium and unknown
		return Thresholds{
			PortScanPorts: 50, PortScanEvents: 100,
			HorizontalHosts: 40, HorizontalEvents: 80,
			SurgeRatio: 5, SurgeAbsMin: 200, SurgeFloor: 20,
			NewCountryMin: 5, NewCountryBaseline: 10, NewCountryMinShare: 0.05, RepMinEvents: 3,
			ByteSurgeRatio: 5, ByteSurgeAbsMin: 100_000_000, ByteSurgeFloor: 5_000_000,
			BeaconMinHours: 10, BeaconMaxAvgBytes: 300_000, BeaconMinRegularity: 0.55,
			LateralHosts: 25, LateralEvents: 50,
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
