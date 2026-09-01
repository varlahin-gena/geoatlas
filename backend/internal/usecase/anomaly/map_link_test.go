package anomaly

import "testing"

func TestMapLinkForBlockedSurgeUsesIPScope(t *testing.T) {
	got := MapLinkFor(Event{
		Code:   CodeBlockedSurge,
		Device: "10.10.0.0/16",
	})
	if got.Group != "ip" {
		t.Fatalf("group = %q, want ip", got.Group)
	}
	if got.Filter != "blocked" {
		t.Fatalf("filter = %q, want blocked", got.Filter)
	}
	if got.Query != "(src:10.10. OR dst:10.10.)" {
		t.Fatalf("query = %q", got.Query)
	}
}

func TestMapLinkForBlockedSurgeWideNetWithLabelUsesCity(t *testing.T) {
	got := MapLinkFor(Event{
		Code:   CodeBlockedSurge,
		Device: "10.72.0.0-10.72.255.254",
		Detail: map[string]any{
			"label":   "Belozernii, СТГ, Россия",
			"network": "10.72.0.0-10.72.255.254",
		},
	})
	if got.Group != "city" {
		t.Fatalf("group = %q, want city", got.Group)
	}
	if got.Query != "city:Belozernii" {
		t.Fatalf("query = %q", got.Query)
	}
}

func TestMapLinkForUsesCountryScope(t *testing.T) {
	got := MapLinkFor(Event{
		Code:       CodeNewCountryDst,
		DstCountry: "Israel",
	})
	if got.Group != "country" {
		t.Fatalf("group = %q, want country", got.Group)
	}
	if got.Country != "Israel" {
		t.Fatalf("country = %q", got.Country)
	}
	if got.Query != "dst:Israel" {
		t.Fatalf("query = %q", got.Query)
	}
}

func TestMapLinkForFallsBackToCityScope(t *testing.T) {
	got := MapLinkFor(Event{
		Code:    "custom_city_alert",
		SrcCity: "Moscow",
	})
	if got.Group != "city" {
		t.Fatalf("group = %q, want city", got.Group)
	}
	if got.Query != "city:Moscow" {
		t.Fatalf("query = %q", got.Query)
	}
}

func TestMapLinkForByteSurge(t *testing.T) {
	got := MapLinkFor(Event{Code: CodeByteSurge, SrcIP: "10.1.1.1"})
	if got.Period != "2h" || got.Query != "src:10.1.1.1" {
		t.Fatalf("%+v", got)
	}
}

func TestMapLinkForBeaconing(t *testing.T) {
	got := MapLinkFor(Event{Code: CodeBeaconing, SrcIP: "10.0.0.1", DstIP: "1.2.3.4"})
	if got.Period != "1d" || got.Query != "src:10.0.0.1 dst:1.2.3.4" {
		t.Fatalf("%+v", got)
	}
}

func TestMapLinkForLateralFanout(t *testing.T) {
	got := MapLinkFor(Event{Code: CodeLateralFanout, SrcIP: "10.0.0.2"})
	if got.Period != "1h" || got.Query != "src:10.0.0.2" {
		t.Fatalf("%+v", got)
	}
}
