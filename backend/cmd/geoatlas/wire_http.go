package main

import (
	"path/filepath"
	"time"

	"geoatlas/internal/adapter/anomalyjob"
	"geoatlas/internal/adapter/backupfs"
	"geoatlas/internal/adapter/backupjob"
	"geoatlas/internal/adapter/backupschedulefile"
	"geoatlas/internal/adapter/clickhouse/anomalystore"
	"geoatlas/internal/adapter/clickhouse/auditstore"
	"geoatlas/internal/adapter/clickhouse/backupstore"
	"geoatlas/internal/adapter/clickhouse/geostore"
	"geoatlas/internal/adapter/clickhouse/perrorstore"
	"geoatlas/internal/adapter/clickhouse/sysstore"
	"geoatlas/internal/adapter/clickhouse/trafficstore"
	"geoatlas/internal/adapter/geoipcodec"
	httpapi "geoatlas/internal/adapter/httpapi"
	"geoatlas/internal/adapter/parseradapter"
	"geoatlas/internal/adapter/searchtemplatesfile"
	"geoatlas/internal/adapter/systemlive"
	"geoatlas/internal/config"
	"geoatlas/internal/installprofile"
	"geoatlas/internal/parser"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
	usecaseauth "geoatlas/internal/usecase/auth"
	usecasebackup "geoatlas/internal/usecase/backup"
	usecaseevents "geoatlas/internal/usecase/events"
	usecasegeo "geoatlas/internal/usecase/geo"
	"geoatlas/internal/usecase/parseerrors"
	"geoatlas/internal/usecase/parsetest"
	"geoatlas/internal/usecase/searchtemplates"
	usecasesystem "geoatlas/internal/usecase/system"
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
	geoUC.SetEnterpriseStore(geoRepo)
	geoUC.SetHeavySlot(a.heavy)
	if p, err := installprofile.Load(cfg.InstallProfilePath); err == nil && p != nil && p.Limits.Backend.MemoryGB > 0 {
		geoUC.SetSoftMemLimitBytes(config.BackendSoftMemLimitBytes(p.Limits.Backend.MemoryGB))
	}
	parseErrorsUC := parseerrors.New(perrorstore.NewParseErrorRepository(a.pools.API, a.pools.Ingest))
	parseTestAdapter := parseradapter.NewParseTest(parsers)
	parseTestUC := parsetest.New(parseTestAdapter, bg.geo, parseTestAdapter)
	systemRepo := sysstore.NewSystemRepository(a.pools.API)
	logRepo := auditstore.New(a.pools.API)
	logsUC := usecaseaudit.New(logRepo)
	systemUC := usecasesystem.New(usecasesystem.Dependencies{
		Metrics:            systemRepo,
		Edges:              systemRepo,
		Ingest:             &systemlive.IngestAdapter{Src: a.ingestSvc},
		SyslogNG:           &systemlive.SyslogNGAdapter{URL: cfg.SyslogStatsURL},
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
	backupUC.SetLogService(logsUC)
	backupUC.SetHeavySlot(a.heavy)
	a.backupJobs = backupjob.NewFromService(backupUC, time.Minute)

	var anomalyUC *usecaseanomaly.Service
	if cfg.AnomalyEnabled {
		apiRepo := anomalystore.New(a.pools.API)
		bgRepo := anomalystore.New(a.pools.Background)
		var anomRep usecaseanomaly.ReputationLookuper
		if bg.repIdx != nil {
			anomRep = bg.repIdx
		}
		profileName := "medium"
		if p, err := installprofile.Load(cfg.InstallProfilePath); err == nil && p != nil && p.Profile != "" {
			profileName = p.Profile
		}
		anomalyUC = usecaseanomaly.New(usecaseanomaly.Config{
			Enabled:                       true,
			IncludePrivate:                cfg.AnomalyIncludePrivate,
			LearningDays:                  cfg.AnomalyLearningDays,
			InstallProfile:                profileName,
			SuppressHours:                 cfg.AnomalySuppressHours,
			NewCountryMinShare:            cfg.AnomalyNewCountryMinShare,
			NewCountryRepeatCooldownHours: cfg.AnomalyNewCountryRepeatCooldownHours,
		}, apiRepo, bgRepo, anomRep, anomalyjob.Gate{Ingest: a.ingestSvc}, a.prom)
		anomalyUC.SetEnterpriseNets(geoUC)
		a.anomalyJobs = anomalyjob.New(anomalyUC, cfg.AnomalyScanInterval, time.Minute)
		a.anomalyJobs.SetLimiter(a.heavy)
	}

	searchTemplatesUC := searchtemplates.New(searchtemplatesfile.New(cfg.SearchTemplatesFile))

	return httpapi.NewServer(httpapi.Params{
		Cfg:               cfg,
		Ingest:            a.ingestSvc,
		EventsUC:          eventsUC,
		GeoUC:             geoUC,
		ReputationUC:      bg.repUC,
		ParseErrorsUC:     parseErrorsUC,
		SystemUC:          systemUC,
		SystemPinger:      systemRepo,
		ParseTestUC:       parseTestUC,
		RetentionUC:       bg.retentionUC,
		BackupUC:          backupUC,
		AuthUC:            authUC,
		Users:             auth.users,
		Sessions:          auth.sessions,
		APITokens:         auth.apiTokens,
		SearchTemplatesUC: searchTemplatesUC,
		AnomalyUC:         anomalyUC,
		Logs:              logsUC,
	}, httpapi.WithMetrics(a.prom))
}
