package anomaly

import "testing"

func TestEffectiveThresholdsProfileOnly(t *testing.T) {
	got := EffectiveThresholds("small", nil, 0)
	if got.PortScanPorts != 40 || got.PortScanEvents != 80 {
		t.Fatalf("profile defaults: %+v", got)
	}
	if got.NewCountryMinShare != 0.05 {
		t.Fatalf("share: %v", got.NewCountryMinShare)
	}
}

func TestEffectiveThresholdsOverrides(t *testing.T) {
	ov := ThresholdsForProfile("small")
	ov.PortScanPorts = 55
	got := EffectiveThresholds("small", &ov, 0.08)
	if got.PortScanPorts != 55 {
		t.Fatalf("override ports: %d", got.PortScanPorts)
	}
	if got.NewCountryMinShare != 0.08 {
		t.Fatalf("settings share wins: %v", got.NewCountryMinShare)
	}
}

func TestValidateThresholds(t *testing.T) {
	th := ThresholdsForProfile("medium")
	if err := validateThresholds(th); err != nil {
		t.Fatal(err)
	}
	bad := th
	bad.PortScanPorts = 0
	if err := validateThresholds(bad); err == nil {
		t.Fatal("expected error for port_scan_ports")
	}
	bad = th
	bad.SurgeRatio = 0.5
	if err := validateThresholds(bad); err == nil {
		t.Fatal("expected error for surge_ratio")
	}
}
