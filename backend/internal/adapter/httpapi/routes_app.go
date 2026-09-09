package httpapi

import "net/http"

func (w *routeWiring) registerSearchTemplateRoutes() {
	// --- Личные шаблоны поиска (карта) ---
	w.handle("GET", "/api/me/search-templates",
		withTimeout(chain(http.HandlerFunc(w.tpl.ListMine), w.loginMW), healthTimeout),
	)
	w.handle("POST", "/api/me/search-templates",
		chain(http.HandlerFunc(w.tpl.CreateMine), w.loginMW, w.csrf, maxBytesMW(64<<10)),
	)
	w.handle("PUT", "/api/me/search-templates/{id}",
		chain(http.HandlerFunc(w.tpl.UpdateMine), w.loginMW, w.csrf, maxBytesMW(64<<10)),
	)
	w.handle("DELETE", "/api/me/search-templates/{id}",
		chain(http.HandlerFunc(w.tpl.DeleteMine), w.loginMW, w.csrf),
	)
	w.handle("GET", "/api/search-templates",
		withTimeout(chain(http.HandlerFunc(w.tpl.ListAll), w.adminMW), healthTimeout),
	)
}

func (w *routeWiring) registerHuntRoutes() {
	// --- Saved hunts (полное состояние карты + расписание) ---
	w.handle("GET", "/api/me/hunts",
		withTimeout(chain(http.HandlerFunc(w.hunts.ListMine), w.loginMW), healthTimeout),
	)
	w.handle("POST", "/api/me/hunts",
		chain(http.HandlerFunc(w.hunts.CreateMine), w.loginMW, w.csrf, maxBytesMW(64<<10)),
	)
	w.handle("PUT", "/api/me/hunts/{id}",
		chain(http.HandlerFunc(w.hunts.UpdateMine), w.loginMW, w.csrf, maxBytesMW(64<<10)),
	)
	w.handle("DELETE", "/api/me/hunts/{id}",
		chain(http.HandlerFunc(w.hunts.DeleteMine), w.loginMW, w.csrf),
	)
	w.handle("POST", "/api/me/hunts/{id}/run",
		withTimeout(chain(http.HandlerFunc(w.hunts.RunMine), w.loginMW, w.csrf), readTimeout),
	)
	w.handle("GET", "/api/hunts",
		withTimeout(chain(http.HandlerFunc(w.hunts.ListAll), w.adminMW), healthTimeout),
	)
}

func (w *routeWiring) registerMapRoutes() {
	// --- Карта / статус: любой залогиненный ---
	w.handle("GET", "/api/events",
		withTimeout(chain(http.HandlerFunc(w.events.GetEvents), w.loginMW), readTimeout),
	)
	w.handle("GET", "/api/events/series",
		withTimeout(chain(http.HandlerFunc(w.events.GetEventsSeries), w.loginMW), readTimeout),
	)
	w.handle("GET", "/api/anomalies/summary",
		withTimeout(chain(http.HandlerFunc(w.anom.Summary), w.loginMW), healthTimeout),
	)
	w.handle("GET", "/api/anomalies/status",
		withTimeout(chain(http.HandlerFunc(w.anom.Status), w.opsMW), healthTimeout),
	)
	w.handle("GET", "/api/anomalies",
		withTimeout(chain(http.HandlerFunc(w.anom.List), w.loginMW), readTimeout),
	)
	w.handle("GET", "/api/anomalies/episodes",
		withTimeout(chain(http.HandlerFunc(w.anom.Episodes), w.loginMW), readTimeout),
	)
	w.handle("POST", "/api/anomalies/{fingerprint}/ack",
		chain(http.HandlerFunc(w.anom.Ack), w.loginMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("POST", "/api/anomalies/{fingerprint}/assign",
		chain(http.HandlerFunc(w.anom.Assign), w.loginMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("GET", "/api/anomalies/settings",
		withTimeout(chain(http.HandlerFunc(w.anom.GetSettings), w.adminMW), healthTimeout),
	)
	w.handle("PUT", "/api/anomalies/settings",
		chain(http.HandlerFunc(w.anom.PutSettings), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("GET", "/api/system/status",
		withTimeout(chain(http.HandlerFunc(w.system.GetSystemStatus), w.loginMW), readTimeout),
	)
	w.handle("GET", "/api/system/version",
		withTimeout(chain(http.HandlerFunc(w.system.GetSystemVersion), w.loginMW), healthTimeout),
	)
	w.handle("GET", "/api/geo-missing",
		withTimeout(chain(http.HandlerFunc(w.geo.GetGeoMissing), w.adminMW), readTimeout),
	)
	// Без withTimeout: TimeoutHandler буферизует ответ целиком, экспорт стримит.
	w.handle("GET", "/api/geo-ranges/export",
		chain(http.HandlerFunc(w.geo.ExportGeoRangesCSV), w.opsMW),
	)
	w.handle("POST", "/api/geo-ranges/clear",
		chain(http.HandlerFunc(w.geo.ClearGeoRanges), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("GET", "/api/geo-ranges",
		withTimeout(chain(http.HandlerFunc(w.geo.ListGeoRanges), w.adminMW), readTimeout),
	)
	w.handle("POST", "/api/geo-ranges",
		chain(http.HandlerFunc(w.geo.AppendGeoRange), w.opsMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("PUT", "/api/geo-ranges",
		chain(http.HandlerFunc(w.geo.UpdateGeoRange), w.opsMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("GET", "/api/enterprise-nets",
		withTimeout(chain(http.HandlerFunc(w.geo.ListEnterpriseNets), w.adminMW), readTimeout),
	)
	w.handle("POST", "/api/enterprise-nets",
		chain(http.HandlerFunc(w.geo.AddEnterpriseNet), w.opsMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("DELETE", "/api/enterprise-nets/{start_ip}/{end_ip}",
		chain(http.HandlerFunc(w.geo.DeleteEnterpriseNet), w.opsMW, w.csrf),
	)

	w.handle("GET", "/api/reputation/lists",
		withTimeout(chain(http.HandlerFunc(w.rep.ListLists), w.adminMW), readTimeout),
	)
	w.handle("DELETE", "/api/reputation/lists/{name}",
		chain(http.HandlerFunc(w.rep.DeleteList), w.opsMW, w.csrf),
	)
	w.handle("GET", "/api/reputation/feeds",
		withTimeout(chain(http.HandlerFunc(w.rep.ListFeeds), w.adminMW), readTimeout),
	)
	w.handle("POST", "/api/reputation/feeds",
		chain(http.HandlerFunc(w.rep.AddFeed), w.opsMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("DELETE", "/api/reputation/feeds/{name}",
		chain(http.HandlerFunc(w.rep.RemoveFeed), w.opsMW, w.csrf),
	)
	w.handle("GET", "/api/reputation/catalog",
		withTimeout(chain(http.HandlerFunc(w.rep.ListCatalog), w.adminMW), readTimeout),
	)
	w.handle("POST", "/api/reputation/refresh",
		chain(http.HandlerFunc(w.rep.Refresh), w.opsMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("GET", "/api/reputation/lookup",
		withTimeout(chain(http.HandlerFunc(w.rep.Lookup), w.loginMW), healthTimeout),
	)
}
