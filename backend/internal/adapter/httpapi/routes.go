package httpapi

import "net/http"

// routeWiring — handlers + MW для регистрации маршрутов.
// Порядок register*() должен совпадать с прежней таблицей в NewServer.
type routeWiring struct {
	rr *routeRegistrar

	health *HealthHandler
	events *EventsHandler
	ingest *IngestHandler
	geo    *GeoHandler
	rep    *ReputationHandler
	system *SystemHandler
	parse  *ParseHandler
	auth   *AuthHandler
	users  *UsersHandler
	tokens *APITokensHandler
	tpl    *SearchTemplatesHandler
	anom   *AnomalyHandler
	hunts  *HuntsHandler

	loginMW middleware
	adminMW middleware
	opsMW   middleware
	csrf    middleware

	prom MetricsRecorder

	maxLogUpload        int64
	maxGeoUpload        int64
	maxReputationUpload int64
}

func (w *routeWiring) registerAll() {
	w.registerAuthRoutes()
	w.registerUserRoutes()
	w.registerSearchTemplateRoutes()
	w.registerHuntRoutes()
	w.registerTokenRoutes()
	w.registerProbeRoutes()
	w.registerMapRoutes()
	w.registerAdminSystemRoutes()
	w.registerUploadRoutes()
}

func (w *routeWiring) handle(method, path string, h http.Handler) {
	w.rr.Handle(method, path, h)
}
