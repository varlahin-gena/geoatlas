package main

import (
	chadapter "network_monitor/internal/adapter/clickhouse"
	"network_monitor/internal/adapter/geoipcodec"
	httpapi "network_monitor/internal/adapter/httpapi"
	"network_monitor/internal/adapter/parseradapter"
	"network_monitor/internal/adapter/systemlive"
	"network_monitor/internal/config"
	"network_monitor/internal/parser"
	usecaseauth "network_monitor/internal/usecase/auth"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
	"network_monitor/internal/usecase/parseerrors"
	"network_monitor/internal/usecase/parsetest"
	usecasesystem "network_monitor/internal/usecase/system"
)

func buildHTTP(cfg config.Config, a *app, auth authParts, bg backgroundParts, parsers *parser.Registry) *httpapi.Server {
	trafficRepo := chadapter.NewTrafficRepository(a.pools.API)
	geoRepo := chadapter.NewGeoRepository(a.pools.API, a.pools.Ingest)
	// Не передавать typed-nil *ReloadableReputationIndex в ReputationLookuper:
	// interface!=nil при dyn=nil → enrichMapReputation падает на Lookup.
	var repLookuper usecaseevents.ReputationLookuper
	if bg.repIdx != nil {
		repLookuper = bg.repIdx
	}
	eventsUC := usecaseevents.New(trafficRepo, bg.geo, repLookuper)
	geoUC := usecasegeo.New(geoRepo, trafficRepo, bg.geo, a.geoJobs, geoipcodec.New(), cfg.MaxGeoUploadRanges)
	parseErrorsUC := parseerrors.New(chadapter.NewParseErrorRepository(a.pools.API, a.pools.Ingest))
	parseTestAdapter := parseradapter.NewParseTest(parsers)
	parseTestUC := parsetest.New(parseTestAdapter, bg.geo, parseTestAdapter)
	systemRepo := chadapter.NewSystemRepository(a.pools.API)
	systemUC := usecasesystem.New(usecasesystem.Dependencies{
		Metrics:            systemRepo,
		Edges:              systemRepo,
		Ingest:             &systemlive.IngestAdapter{Src: a.ingestSvc},
		Profiles:           systemlive.ProfileAdapter{},
		InstallProfilePath: cfg.InstallProfilePath,
		InstallMetaPath:    cfg.InstallMetaPath,
		Maintenance:        a.geoJobs,
	})
	authUC := usecaseauth.New(auth.users, auth.sessions)
	return httpapi.NewServer(
		cfg, a.ingestSvc, eventsUC, geoUC, bg.repUC, parseErrorsUC, systemUC, systemRepo,
		parseTestUC, bg.retentionUC, authUC, auth.users, auth.sessions, auth.apiTokens,
	)
}
