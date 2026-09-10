package httpapi

import "net/http"

func (w *routeWiring) registerProbeRoutes() {
	// --- Probes открыты (docker/k8s). /live и /health — процесс; /ready — CH+ingest. ---
	w.handle("GET", "/live", withTimeout(http.HandlerFunc(w.health.Live), healthTimeout))
	w.handle("GET", "/api/live", withTimeout(http.HandlerFunc(w.health.Live), healthTimeout))
	w.handle("GET", "/health", withTimeout(http.HandlerFunc(w.health.Live), healthTimeout))
	w.handle("GET", "/api/health", withTimeout(http.HandlerFunc(w.health.Live), healthTimeout))
	w.handle("GET", "/ready", withTimeout(http.HandlerFunc(w.health.Ready), healthTimeout))
	w.handle("GET", "/api/ready", withTimeout(http.HandlerFunc(w.health.Ready), healthTimeout))
	w.handle("GET", "/api/ingest/stats",
		withTimeout(chain(http.HandlerFunc(w.ingest.GetIngestStats), w.opsMW), healthTimeout),
	)
	// Prometheus scrape: Bearer≥ops / administrator (как ingest/stats).
	w.handle("GET", "/metrics", chain(metricsHandler(w.prom), w.opsMW))
}

func (w *routeWiring) registerAdminSystemRoutes() {
	// --- Только администратор ---
	w.handle("GET", "/api/system/stats",
		withTimeout(chain(http.HandlerFunc(w.system.GetSystemStats), w.adminMW), readTimeout),
	)
	w.handle("GET", "/api/system/history",
		withTimeout(chain(http.HandlerFunc(w.system.GetSystemHistory), w.adminMW), readTimeout),
	)
	w.handle("GET", "/api/system/edges-agg",
		withTimeout(chain(http.HandlerFunc(w.system.GetEdgesAggStatus), w.adminMW), healthTimeout),
	)
	w.handle("POST", "/api/system/maintenance/backfill",
		chain(http.HandlerFunc(w.system.PostMaintenanceBackfill), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("GET", "/api/system/install-profile",
		withTimeout(chain(http.HandlerFunc(w.system.GetInstallProfile), w.adminMW), healthTimeout),
	)
	w.handle("GET", "/api/system/retention",
		withTimeout(chain(http.HandlerFunc(w.system.GetRetention), w.adminMW), healthTimeout),
	)
	w.handle("PUT", "/api/system/retention",
		chain(http.HandlerFunc(w.system.PutRetention), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("GET", "/api/system/tls",
		withTimeout(chain(http.HandlerFunc(w.system.GetTLS), w.adminMW), healthTimeout),
	)
	w.handle("PUT", "/api/system/tls",
		chain(http.HandlerFunc(w.system.PutTLS), w.adminMW, w.csrf, maxBytesMW(1<<20)),
	)
	w.handle("POST", "/api/system/tls/reload",
		chain(http.HandlerFunc(w.system.PostTLSReload), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("GET", "/api/system/backups",
		withTimeout(chain(http.HandlerFunc(w.system.GetBackups), w.adminMW), healthTimeout),
	)
	w.handle("GET", "/api/dr/history",
		withTimeout(chain(http.HandlerFunc(w.system.GetDRHistory), w.adminMW), readTimeout),
	)
	w.handle("GET", "/api/audit",
		withTimeout(chain(http.HandlerFunc(w.system.GetAuditLog), w.adminMW), readTimeout),
	)
	w.handle("POST", "/api/system/backups",
		chain(http.HandlerFunc(w.system.PostBackup), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("POST", "/api/system/backups/{name}/attach",
		chain(http.HandlerFunc(w.system.PostBackupAttach), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("POST", "/api/system/backups/{name}/detach",
		chain(http.HandlerFunc(w.system.PostBackupDetach), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("DELETE", "/api/system/backups/{name}",
		chain(http.HandlerFunc(w.system.DeleteBackup), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("GET", "/api/system/backup-schedule",
		withTimeout(chain(http.HandlerFunc(w.system.GetBackupSchedule), w.adminMW), healthTimeout),
	)
	w.handle("PUT", "/api/system/backup-schedule",
		chain(http.HandlerFunc(w.system.PutBackupSchedule), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("GET", "/api/parse-errors",
		withTimeout(chain(http.HandlerFunc(w.parse.ListParseErrors), w.adminMW), readTimeout),
	)
	w.handle("GET", "/api/parse-samples",
		withTimeout(chain(http.HandlerFunc(w.parse.ParseSamples), w.adminMW), readTimeout),
	)
	w.handle("POST", "/api/parse-test",
		chain(http.HandlerFunc(w.parse.ParseTest), w.adminMW, w.csrf, maxBytesMW(maxParseTestSize)),
	)
	w.handle("POST", "/api/parse-errors/delete",
		chain(http.HandlerFunc(w.parse.DeleteParseErrors), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
}

func (w *routeWiring) registerUploadRoutes() {
	// --- Мутирующие: Bearer / administrator (не operator); open если *AUTH_DISABLED ---
	w.handle("POST", "/api/ingest",
		chain(http.HandlerFunc(w.ingest.IngestLogs), w.opsMW, w.csrf, maxBytesMW(w.maxLogUpload)),
	)
	w.handle("POST", "/upload-logs",
		chain(http.HandlerFunc(w.ingest.UploadLogs), w.opsMW, w.csrf, maxBytesMW(w.maxLogUpload)),
	)
	w.handle("POST", "/upload-geo",
		chain(http.HandlerFunc(w.geo.UploadGeo), w.opsMW, w.csrf, maxBytesMW(w.maxGeoUpload)),
	)
	w.handle("POST", "/upload-reputation",
		chain(http.HandlerFunc(w.rep.UploadReputation), w.opsMW, w.csrf, maxBytesMW(w.maxReputationUpload)),
	)
}
