package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"network_monitor/internal/auth"
	"network_monitor/internal/config"
	usecaseauth "network_monitor/internal/usecase/auth"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
	"network_monitor/internal/usecase/parseerrors"
	"network_monitor/internal/usecase/parsetest"
	usecaseretention "network_monitor/internal/usecase/retention"
	usecasesystem "network_monitor/internal/usecase/system"
)

// Deps — общие зависимости HTTP-слоя (без clickhouse.Conn — только usecase/ports).
type Deps struct {
	cfg           config.Config
	ingest        Ingester
	parseErrorsUC *parseerrors.Service
	parseTestUC   *parsetest.Service
	eventsUC      *usecaseevents.Service
	geoUC         *usecasegeo.Service
	systemUC      *usecasesystem.Service
	systemPinger  usecasesystem.ClickHousePinger
	retentionUC   *usecaseretention.Service
	authUC        *usecaseauth.Service
	users         *auth.UserStore // middleware / LiveSession
	sessions      *auth.SessionManager
	apiTokens     *auth.TokenStore
}

func NewDeps(
	cfg config.Config,
	ingestSvc Ingester,
	eventsUC *usecaseevents.Service,
	geoUC *usecasegeo.Service,
	parseErrorsUC *parseerrors.Service,
	systemUC *usecasesystem.Service,
	systemPinger usecasesystem.ClickHousePinger,
	parseTestUC *parsetest.Service,
	retentionUC *usecaseretention.Service,
) *Deps {
	d := &Deps{
		cfg:           cfg,
		eventsUC:      eventsUC,
		geoUC:         geoUC,
		parseErrorsUC: parseErrorsUC,
		parseTestUC:   parseTestUC,
		systemUC:      systemUC,
		systemPinger:  systemPinger,
		retentionUC:   retentionUC,
	}
	if ingestSvc != nil {
		d.ingest = ingestSvc
	}
	return d
}

func (d *Deps) WithAuth(authUC *usecaseauth.Service, users *auth.UserStore, sessions *auth.SessionManager, apiTokens *auth.TokenStore) *Deps {
	if d == nil {
		return nil
	}
	d.authUC = authUC
	d.users = users
	d.sessions = sessions
	d.apiTokens = apiTokens
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
