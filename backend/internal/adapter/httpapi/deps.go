package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"network_monitor/internal/config"
	usecasesystem "network_monitor/internal/usecase/system"
)

// Deps — общие зависимости HTTP-слоя (без clickhouse.Conn — только usecase/ports).
type Deps struct {
	cfg             config.Config
	ingest          Ingester
	parseErrorsUC   ParseErrorsAPI
	parseTestUC     ParseTestAPI
	eventsUC        EventsAPI
	geoUC           GeoAPI
	reputationUC    ReputationAPI
	systemUC        SystemAPI
	systemPinger    usecasesystem.ClickHousePinger
	retentionUC     RetentionAPI
	backupUC        BackupAPI
	authUC          AuthAPI
	users           UserDirectory // middleware / LiveSession
	sessions        SessionParser
	apiTokens       APITokenStore
	searchTemplates SearchTemplatesStore
}

func NewDeps(
	cfg config.Config,
	ingestSvc Ingester,
	eventsUC EventsAPI,
	geoUC GeoAPI,
	reputationUC ReputationAPI,
	parseErrorsUC ParseErrorsAPI,
	systemUC SystemAPI,
	systemPinger usecasesystem.ClickHousePinger,
	parseTestUC ParseTestAPI,
	retentionUC RetentionAPI,
	backupUC BackupAPI,
) *Deps {
	d := &Deps{
		cfg:           cfg,
		eventsUC:      eventsUC,
		geoUC:         geoUC,
		reputationUC:  reputationUC,
		parseErrorsUC: parseErrorsUC,
		parseTestUC:   parseTestUC,
		systemUC:      systemUC,
		systemPinger:  systemPinger,
		retentionUC:   retentionUC,
		backupUC:      backupUC,
	}
	if ingestSvc != nil {
		d.ingest = ingestSvc
	}
	return d
}

func (d *Deps) WithAuth(authUC AuthAPI, users UserDirectory, sessions SessionParser, apiTokens APITokenStore) *Deps {
	if d == nil {
		return nil
	}
	d.authUC = authUC
	d.users = users
	d.sessions = sessions
	d.apiTokens = apiTokens
	return d
}

func (d *Deps) WithSearchTemplates(store SearchTemplatesStore) *Deps {
	if d == nil {
		return nil
	}
	d.searchTemplates = store
	return d
}

type HealthHandler struct{ *Deps }
type EventsHandler struct{ *Deps }
type IngestHandler struct{ *Deps }
type GeoHandler struct{ *Deps }
type SystemHandler struct{ *Deps }
type ParseHandler struct{ *Deps }

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("writeJSON: marshal failed", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"json marshal failed"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeInternalError(w http.ResponseWriter, logMsg string, err error) {
	slog.Error(logMsg, "err", err)
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal server error"})
}
