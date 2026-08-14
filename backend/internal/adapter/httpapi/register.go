package httpapi

import "net/http"

// RouteInfo describes a registered HTTP route (method + path template).
type RouteInfo struct {
	Method string // GET, POST, ...
	Path   string // /api/users/{username}
}

type routeRegistrar struct {
	mux    *http.ServeMux
	routes []RouteInfo
}

func newRouteRegistrar() *routeRegistrar {
	return &routeRegistrar{mux: http.NewServeMux()}
}

func (rr *routeRegistrar) Handle(method, path string, h http.Handler) {
	rr.routes = append(rr.routes, RouteInfo{Method: method, Path: path})
	rr.mux.Handle(method+" "+path, withRoutePattern(path, h))
}

func (rr *routeRegistrar) Handler() http.Handler {
	return rr.mux
}

func (rr *routeRegistrar) Routes() []RouteInfo {
	out := make([]RouteInfo, len(rr.routes))
	copy(out, rr.routes)
	return out
}
