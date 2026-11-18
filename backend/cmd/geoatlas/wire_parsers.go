package main

import (
	"context"
	"log/slog"

	"geoatlas/internal/adapter/clickhouse/parserstore"
	"geoatlas/internal/adapter/parseradapter"
	"geoatlas/internal/parser"
	usecaseparsers "geoatlas/internal/usecase/parsers"
)

type parserParts struct {
	reg     *parser.ReloadableRegistry
	uc      *usecaseparsers.Service
	samples *parseradapter.SamplesHolder
}

func wireParsers(ctx context.Context, a *app) parserParts {
	store := parserstore.New(a.pools.API, a.pools.Ingest)
	reg := parser.NewReloadable(nil)
	samples := parseradapter.NewSamplesHolder(parser.SamplesFromRules(nil))
	reloader := parseradapter.NewReloader(store, reg, samples)
	uc := usecaseparsers.New(store, reloader)

	if err := uc.EnsureSeed(ctx); err != nil {
		slog.Warn("parser rules seed failed", "err", err)
	}
	if err := reloader.Reload(ctx); err != nil {
		slog.Warn("parser registry reload failed; using in-memory seeds", "err", err)
		if built, err2 := parser.BuildRegistry(parser.DefaultSeedRules()); err2 == nil {
			reg.Swap(built)
			samples.Store(parser.SamplesFromRules(parser.DefaultSeedRules()))
		}
	} else {
		slog.Info("parser registry loaded from clickhouse")
	}
	return parserParts{reg: reg, uc: uc, samples: samples}
}
