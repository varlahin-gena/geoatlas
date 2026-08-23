package main

import (
	"path/filepath"
	"time"

	"network_monitor/internal/adapter/anomalyjob"
	"network_monitor/internal/adapter/backupfs"
	"network_monitor/internal/adapter/backupjob"
	"network_monitor/internal/adapter/backupschedulefile"
	"network_monitor/internal/adapter/clickhouse/anomalystore"
	"network_monitor/internal/adapter/clickhouse/auditstore"
	"network_monitor/internal/adapter/clickhouse/backupstore"
	"network_monitor/internal/adapter/clickhouse/geostore"
	"network_monitor/internal/adapter/clickhouse/perrorstore"
	"network_monitor/internal/adapter/clickhouse/sysstore"
	"network_monitor/internal/adapter/clickhouse/trafficstore"
	"network_monitor/internal/adapter/geoipcodec"
	httpapi "network_monitor/internal/adapter/httpapi"
	"network_monitor/internal/adapter/parseradapter"
	"network_monitor/internal/adapter/searchtemplatesfile"
	"network_monitor/internal/adapter/systemlive"
	"network_monitor/internal/config"
	"network_monitor/internal/installprofile"
	"network_monitor/internal/parser"
	usecaseanomaly "network_monitor/internal/usecase/anomaly"
	usecaseaudit "network_monitor/internal/usecase/auditlog"
	usecaseauth "network_monitor/internal/usecase/auth"
	usecasebackup "network_monitor/internal/usecase/backup"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
	"network_monitor/internal/usecase/parseerrors"
	"network_monitor/internal/usecase/parsetest"
	"network_monitor/internal/usecase/searchtemplates"
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
	geoUC.SetEnterpriseStore(geoRepo)
	geoUC.SetHeavySlot(a.heavy)
	if p, err := installprofile.Load(cfg.InstallProfilePath); err == nil && p != nil && p.Limits.Backend.MemoryGB > 0 {
		// 75% cgroup backend → soft ceiling для HeapAlloc+upload snapshot.
		geoUC.SetSoftMemLimitBytes(uint64(p.Limits.Backend.MemoryGB) * 3 / 4 * (1 << 30))
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
