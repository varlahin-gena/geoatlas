package mapsearch

import (
	"testing"
	"time"
)

func TestAssessMapQueryCostHeavyIPShortPeriod(t *testing.T) {
	cost := AssessMapQueryCost(MapQueryCostInput{
		GroupBy: "ip",
		Mode:    "hours",
		Amount:  6,
	})
	if cost.Tier != QueryCostHeavy {
		t.Fatalf("tier=%q want heavy", cost.Tier)
	}
	if cost.LimitCap != mapLimitCapHeavy {
		t.Fatalf("cap=%d", cost.LimitCap)
	}
}

func TestAssessMapQueryCostHeavyIPWithFilterHigherCap(t *testing.T) {
	cost := AssessMapQueryCost(MapQueryCostInput{
		GroupBy: "ip",
		Mode:    "days",
		Amount:  7,
		Query:   "src_ip:10.0.0.1",
	})
	if cost.Tier != QueryCostHeavy {
		t.Fatalf("tier=%q", cost.Tier)
	}
	if cost.LimitCap != mapLimitCapHeavyFilt {
		t.Fatalf("cap=%d want filtered heavy cap", cost.LimitCap)
	}
}

func TestAssessMapQueryCostLightCityDay(t *testing.T) {
	cost := AssessMapQueryCost(MapQueryCostInput{
		GroupBy: "city",
		Mode:    "days",
		Amount:  1,
	})
	if cost.Tier != QueryCostLight {
		t.Fatalf("tier=%q want light", cost.Tier)
	}
}

func TestEffectiveMapLimitCapsHeavy(t *testing.T) {
	cost := AssessMapQueryCost(MapQueryCostInput{GroupBy: "ip", Mode: "hours", Amount: 1})
	applied, capped := EffectiveMapLimit(20000, cost)
	if !capped || applied != mapLimitCapHeavy {
		t.Fatalf("applied=%d capped=%v", applied, capped)
	}
}

func TestEffectiveMapLimitAbsoluteWideIP(t *testing.T) {
	now := time.Now().UTC()
	cost := AssessMapQueryCost(MapQueryCostInput{
		GroupBy: "ip",
		Mode:    "absolute",
		From:    now.Add(-10 * 24 * time.Hour),
		To:      now,
	})
	if cost.Tier != QueryCostHeavy {
		t.Fatalf("tier=%q", cost.Tier)
	}
}
