package main

import (
	"path/filepath"
	"time"

	"network_monitor/internal/adapter/backupfs"
	"network_monitor/internal/adapter/backupjob"
	"network_monitor/internal/adapter/backupschedulefile"
	"network_monitor/internal/adapter/clickhouse/backupstore"
	"network_monitor/internal/adapter/clickhouse/geostore"
	"network_monitor/internal/adapter/clickhouse/perrorstore"
	"network_monitor/internal/adapter/clickhouse/sysstore"
	"network_monitor/internal/adapter/clickhouse/trafficstore"
	"network_monitor/internal/adapter/geoipcodec"
	httpapi "network_monitor/internal/adapter/httpapi"
	"network_monitor/internal/adapter/parseradapter"
	"network_monitor/internal/adapter/systemlive"
	"network_monitor/internal/config"
	"network_monitor/internal/parser"
	usecaseauth "network_monitor/internal/usecase/auth"
	usecasebackup "network_monitor/internal/usecase/backup"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
	"network_monitor/internal/usecase/parseerrors"
	"network_monitor/internal/usecase/parsetest"
	usecasesystem "network_monitor/internal/usecase/system"
)

func buildHTTP(cfg config.Config, a *app, auth authParts, bg backgroundParts, parsers *parser.Registry) *httpapi.Server {
	trafficRepo := trafficstore.NewTrafficRepository(a.pools.API)
	geoRepo := geostore.NewGeoRepository(a.pools.API, a.pools.Ingest)
	// Не передавать typed-nil *ReloadableReputationIndex в ReputationLookuper:
	// interface!=nil при dyn=nil → enrichMapReputation падает на Lookup.
	var repLookuper usecaseevents.ReputationLookuper
	if bg.repIdx != nil {
		repLookuper = bg.repIdx
	}
	eventsUC := usecaseevents.New(trafficRepo, bg.geo, repLookuper)
	geoUC := usecasegeo.New(geoRepo, trafficRepo, bg.geo, a.geoJobs, geoipcodec.New(), cfg.MaxGeoUploadRanges)
	parseErrorsUC := parseerrors.New(perrorstore.NewParseErrorRepository(a.pools.API, a.pools.Ingest))
	parseTestAdapter := parseradapter.NewParseTest(parsers)
	parseTestUC := parsetest.New(parseTestAdapter, bg.geo, parseTestAdapter)
	systemRepo := sysstore.NewSystemRepository(a.pools.API)
	systemUC := usecasesystem.New(usecasesystem.Dependencies{
		Metrics:            systemRepo,
		Edges:              systemRepo,
		Ingest:             &systemlive.IngestAdapter{Src: a.ingestSvc},
		GeoIndex:           bg.geo,
		Profiles:           systemlive.ProfileAdapter{},
		InstallProfilePath: cfg.InstallProfilePath,
		InstallMetaPath:    cfg.InstallMetaPath,
		Maintenance:        a.geoJobs,
	})
	authUC := usecaseauth.New(auth.users, auth.sessions)
	dataDir := filepath.Dir(cfg.AuthUsersFile)
	if dataDir == "." || dataDir == "" {
		dataDir = "/app/data"
	}
	opts := usecasebackup.Options{
		Enabled:      cfg.BackupEnabled,
		Dir:          cfg.BackupDir,
		DataDir:      dataDir,
		Keep:         cfg.BackupKeep,
		IncludeEdges: cfg.BackupIncludeEdges,
		IncludeAuth:  cfg.BackupIncludeAuth,
	}
	schedStore := backupschedulefile.New(cfg.BackupScheduleFile, usecasebackup.DefaultsSchedule(opts))
	backupUC := usecasebackup.New(opts, backupstore.NewBackupRunner(a.pools.Background), backupfs.New(cfg.BackupDir), schedStore)
	a.backupJobs = backupjob.NewFromService(backupUC, time.Minute)
	return httpapi.NewServer(
		cfg, a.ingestSvc, eventsUC, geoUC, bg.repUC, parseErrorsUC, systemUC, systemRepo,
		parseTestUC, bg.retentionUC, backupUC, authUC, auth.users, auth.sessions, auth.apiTokens,
		httpapi.WithMetrics(a.prom),
	)
}
