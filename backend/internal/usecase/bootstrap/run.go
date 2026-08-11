package bootstrap

import (
	"context"
	"time"
)

// RunStartup выполняет Ensure*, затем backfill или refresh ready, и при необходимости
// планирует geo enrich. Логика вынесена из cmd composition root.
func RunStartup(ctx context.Context, deps Dependencies, opts Options, warn WarnFunc, info InfoFunc) {
	if warn == nil {
		warn = func(string, error) {}
	}
	if info == nil {
		info = func(string, ...any) {}
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 6 * time.Hour
	}
	bctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	info("edges agg: background worker scheduled",
		"skip_startup_backfill", opts.SkipStartupBackfill)

	if deps.Schema != nil {
		if err := deps.Schema.EnsureTTLOnlyDropParts(bctx); err != nil {
			warn("ttl_only_drop_parts ensure failed", err)
		}
		if err := deps.Schema.EnsureTrafficLogsIPv4(bctx); err != nil {
			warn("traffic_logs ipv4 ensure failed", err)
		}
		if err := deps.Schema.EnsureTrafficLogsSuccess(bctx); err != nil {
			warn("traffic_logs success column ensure failed", err)
		}
		if err := deps.Schema.EnsureEdgesAggSchema(bctx); err != nil {
			warn("edges agg schema ensure failed", err)
		}
		if err := deps.Schema.EnsureGeoEdgesAggSchema(bctx); err != nil {
			warn("geo edges agg schema ensure failed", err)
		}
		if opts.ReputationEnabled {
			if err := deps.Schema.EnsureReputationRanges(bctx); err != nil {
				warn("reputation_ranges ensure failed", err)
			}
		}
	}

	if deps.Retention != nil {
		if err := deps.Retention.ApplyFromStore(bctx); err != nil {
			warn("retention TTL apply failed", err)
		} else {
			info("retention: TTL applied from store")
		}
	}

	if bctx.Err() != nil {
		return
	}

	if opts.SkipStartupBackfill {
		if deps.Ready != nil {
			if err := deps.Ready.RefreshEdgesAggReady(bctx); err != nil {
				warn("edges agg ready check failed", err)
			}
			if err := deps.Ready.RefreshGeoEdgesAggReady(bctx); err != nil {
				warn("geo edges ready check failed", err)
			}
		}
		info("startup backfill skipped — use POST /api/system/maintenance/backfill")
		return
	}

	if deps.Backfill != nil {
		if err := deps.Backfill.BackfillEdgesAgg(bctx); err != nil {
			warn("edges agg backfill failed", err)
		}
		if err := deps.Backfill.BackfillGeoEdgesAgg(bctx); err != nil {
			warn("geo edges agg backfill failed", err)
		}
	}

	// Geo backfill только после Ensure*: не конкурирует с agg INSERT SELECT.
	if bctx.Err() != nil {
		return
	}
	if deps.Geo != nil && deps.Geo.RangeCount() > 0 && deps.Enrich != nil {
		info("geo backfill: scheduled after Ensure*",
			"lookback_days", opts.GeoBackfillLookbackDays)
		deps.Enrich.ScheduleEnrichOnly(bctx, 30*time.Minute)
	}
}
