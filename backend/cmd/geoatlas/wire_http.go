package main

import (
	"path/filepath"
	"time"

	"geoatlas/internal/adapter/anomalyjob"
	"geoatlas/internal/adapter/anomalysettingsfile"
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
	"geoatlas/internal/adapter/huntadapter"
	"geoatlas/internal/adapter/huntjob"
	"geoatlas/internal/adapter/huntsfile"
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
	usecasehunts "geoatlas/internal/usecase/hunts"
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
	geoUC := usecasegeo.New(geoRepo, trafficRepo, bg.geo, a.geoJobs, geoipcodec.New(), cfg.Geo.MaxUploadRanges)
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
	dataDir := filepath.Dir(cfg.Auth.UsersFile)
	if dataDir == "." || dataDir == "" {
		dataDir = "/app/data"
	}
	opts := usecasebackup.Options{
		Enabled:      cfg.Backup.Enabled,
		Dir:          cfg.Backup.Dir,
		DataDir:      dataDir,
		Keep:         cfg.Backup.Keep,
		IncludeEdges: cfg.Backup.IncludeEdges,
		IncludeAuth:  cfg.Backup.IncludeAuth,
	}
	schedStore := backupschedulefile.New(cfg.Backup.ScheduleFile, usecasebackup.DefaultsSchedule(opts))
	backupUC := usecasebackup.New(opts, backupstore.NewBackupRunner(a.pools.Background), backupfs.New(cfg.Backup.Dir), schedStore)
	backupUC.SetLogService(logsUC)
	backupUC.SetHeavySlot(a.heavy)
	a.backupJobs = backupjob.NewFromService(backupUC, time.Minute)

	var anomalyUC *usecaseanomaly.Service
	var anomalySettingsUC *usecaseanomaly.SettingsService
	if cfg.Anomaly.Enabled {
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
			IncludePrivate:                cfg.Anomaly.IncludePrivate,
			LearningDays:                  cfg.Anomaly.LearningDays,
			InstallProfile:                profileName,
			SuppressHours:                 cfg.Anomaly.SuppressHours,
			NewCountryMinShare:            cfg.Anomaly.NewCountryMinShare,
			NewCountryRepeatCooldownHours: cfg.Anomaly.NewCountryRepeatCooldownHours,
		}, apiRepo, bgRepo, anomRep, anomalyjob.Gate{Ingest: a.ingestSvc}, a.prom)
		anomalyUC.SetEnterpriseNets(geoUC)
		a.anomalyJobs = anomalyjob.New(anomalyUC, cfg.Anomaly.ScanInterval, time.Minute)
		a.anomalyJobs.SetLimiter(a.heavy)
		seed := usecaseanomaly.DefaultSettingsFromConfig(cfg.Anomaly)
		anomalySettingsUC = usecaseanomaly.NewSettingsService(
			anomalysettingsfile.New(cfg.Anomaly.SettingsFile),
			anomalyUC,
			seed,
			a.anomalyJobs.SetInterval,
		)
		if st, err := anomalySettingsUC.LoadAndApply(); err == nil {
			a.anomalyJobs.SetInterval(time.Duration(st.ScanIntervalMin) * time.Minute)
		}
	}

	searchTemplatesUC := searchtemplates.New(searchtemplatesfile.New(cfg.SearchTemplatesFile))

	var huntsUC *usecasehunts.Service
	var huntReporter usecasehunts.AnomalyReporter
	if anomalyUC != nil {
		huntReporter = huntadapter.HuntAnomalyReporter{Anomaly: anomalyUC}
	}
	timeout := cfg.QueryTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	huntsUC = usecasehunts.New(
		huntsfile.New(cfg.HuntsFile),
		huntadapter.MapRunner{Events: eventsUC},
		huntReporter,
		huntadapter.HeavyGate{Try: a.heavy.TryAcquire, Rel: a.heavy.Release},
		timeout,
	)
	a.huntJobs = huntjob.New(huntsUC, time.Minute)

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
		AnomalySettingsUC: anomalySettingsUC,
		HuntsUC:           huntsUC,
		Logs:              logsUC,
	}, httpapi.WithMetrics(a.prom))
}
