package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"geoatlas/internal/adapter/geoipcodec"
	"geoatlas/internal/auth"
	"geoatlas/internal/config"
	"geoatlas/internal/geoip"
	"geoatlas/internal/model"
	usecaseauth "geoatlas/internal/usecase/auth"
	usecaseevents "geoatlas/internal/usecase/events"
	usecasegeo "geoatlas/internal/usecase/geo"
)

func TestAuthDisabledShortcuts(t *testing.T) {
	h := &AuthHandler{AuthDeps: &AuthDeps{authDisabled: true}}
	for _, fn := range []http.HandlerFunc{h.ChangePassword, h.LogoutAll, h.DismissGeoWizard} {
		rec := call(fn, http.MethodPost, "/x", `{"current_password":"x"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("disabled auth: %d %s", rec.Code, rec.Body.String())
		}
	}
}

func TestUsersHandlerCRUD(t *testing.T) {
	hash, err := auth.HashPassword("adminpass1")
	if err != nil {
		t.Fatal(err)
	}
	users, err := auth.NewUserStore(auth.User{
		Username: "admin", PasswordHash: string(hash), Role: auth.RoleAdministrator, FullName: "Admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := auth.NewSessionManager("users-crud-session-secret-key-32b!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	authUC := usecaseauth.New(users, sessions)
	reauth := NewReauthChecker(config.Config{Auth: config.AuthConfig{Disabled: true}}, authUC, sessions, nil)

	h := &UsersHandler{AuthDeps: &AuthDeps{
		authUC: authUC, users: users, sessions: sessions, reauth: reauth,
	}}

	rec := call(h.List, http.MethodGet, "/api/users", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.Directory, http.MethodGet, "/api/users/directory", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("directory: %d", rec.Code)
	}

	mustReset := true
	body := `{"username":"alice","password":"alicepass1","role":"operator","full_name":"Alice","must_reset_password":true}`
	rec = call(h.Create, http.MethodPost, "/api/users", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	_ = mustReset

	req := httptest.NewRequest(http.MethodPut, "/api/users/alice/role", strings.NewReader(`{"role":"dashboard"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("username", "alice")
	rec = httptest.NewRecorder()
	h.SetRole(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set role: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/users/alice/full-name", strings.NewReader(`{"full_name":"Alice A"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("username", "alice")
	rec = httptest.NewRecorder()
	h.SetFullName(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set name: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/users/alice/reset-password", strings.NewReader(
		`{"password":"alicepass2","must_reset_password":false,"current_password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("username", "alice")
	rec = httptest.NewRecorder()
	h.ResetPassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/users/alice", strings.NewReader(`{"current_password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("username", "alice")
	rec = httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}

	// error branches
	rec = call(h.Create, http.MethodPost, "/api/users", `{"username":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing must_reset: %d", rec.Code)
	}
	rec = call(h.Create, http.MethodPost, "/api/users", `{"username":"admin","password":"duppass1234","role":"operator","must_reset_password":false}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("dup: %d %s", rec.Code, rec.Body.String())
	}

	nilH := &UsersHandler{AuthDeps: &AuthDeps{}}
	for _, fn := range []http.HandlerFunc{nilH.List, nilH.Directory, nilH.Create} {
		rec := call(fn, http.MethodGet, "/x", `{"username":"u","password":"p","role":"operator","must_reset_password":true}`)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("nil users: %d", rec.Code)
		}
	}
	dirOff := &UsersHandler{AuthDeps: &AuthDeps{authDisabled: true}}
	rec = call(dirOff.Directory, http.MethodGet, "/x", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("dir disabled: %d", rec.Code)
	}
}

func TestWriteUserStoreErrorBranches(t *testing.T) {
	cases := []error{
		auth.ErrUserExists, auth.ErrUserNotFound, auth.ErrInvalidUsername, auth.ErrInvalidPassword,
		auth.ErrInvalidRole, auth.ErrInvalidFullName, auth.ErrLastAdmin, auth.ErrSelfDelete,
		auth.ErrBadOldPassword, usecaseauth.ErrNotConfigured, errIngestUnavailable,
	}
	for _, e := range cases {
		rec := httptest.NewRecorder()
		writeUserStoreError(rec, e)
		if rec.Code < 400 {
			t.Fatalf("%v -> %d", e, rec.Code)
		}
	}
}

func TestGeoAndEventsNilHelpers(t *testing.T) {
	geo := &GeoHandler{GeoDeps: &GeoDeps{}}
	for _, fn := range []http.HandlerFunc{
		geo.ListGeoRanges, geo.AppendGeoRange, geo.UpdateGeoRange,
		geo.ListEnterpriseNets, geo.AddEnterpriseNet, geo.DeleteEnterpriseNet, geo.GetGeoMissing,
	} {
		rec := call(fn, http.MethodGet, "/x?limit=10&q=ru&ip=1.1.1.1", `{"network":"10.0.0.0/8","country":"XX"}`)
		if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusBadRequest {
			t.Fatalf("geo nil: %d %s", rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodDelete, "/x", nil)
	req.SetPathValue("start_ip", "bad")
	req.SetPathValue("end_ip", "1")
	// still nil geo → 503 before parse
	rec := httptest.NewRecorder()
	geo.DeleteEnterpriseNet(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("delete ent: %d", rec.Code)
	}

	ev := &EventsHandler{EventsDeps: &EventsDeps{}}
	rec = call(ev.GetEventsSeries, http.MethodGet, "/api/events/series", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("series nil: %d", rec.Code)
	}

	if parseGeoRangesLimit("0") != 2000 || parseGeoRangesLimit("50") != 50 || parseGeoRangesLimit("99999") != 10000 {
		t.Fatal("parseGeoRangesLimit")
	}
	if parseGeoMissingLimit("0") != 500 || parseGeoMissingLimit("20") != 20 || parseGeoMissingLimit("99999") != 5000 {
		t.Fatal("parseGeoMissingLimit")
	}
	if parseDays("0") != 1 || parseDays("5") != 5 || parseDays("99") != 30 {
		t.Fatal("parseDays")
	}
	if _, err := parseTimeParam(""); err == nil {
		t.Fatal("empty time")
	}
	if _, err := parseTimeParam("2024-01-02"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseTimeParam("2024-01-02T15:04:05Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseTimeParam("nope"); err == nil {
		t.Fatal("bad time")
	}
	tr, err := parseEventTimeRange(map[string][]string{
		"from": {"2024-01-01T00:00:00Z"}, "to": {"2024-01-02T00:00:00Z"},
	})
	if err != nil || tr.Mode == "" {
		t.Fatalf("tr=%+v err=%v", tr, err)
	}
	tr, err = parseEventTimeRange(map[string][]string{"hours": {"6"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = tr
	_ = toEnterpriseNetDTO(model.EnterpriseNet{Network: "10.0.0.0/8", Label: "lab"})
}

func TestAuthSessionChangeLogoutDismiss(t *testing.T) {
	hash, err := auth.HashPassword("adminpass1")
	if err != nil {
		t.Fatal(err)
	}
	users, err := auth.NewUserStore(auth.User{
		Username: "admin", PasswordHash: string(hash), Role: auth.RoleAdministrator, FullName: "Admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := auth.NewSessionManager("auth-sess-secret-key-32bytes!!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	authUC := usecaseauth.New(users, sessions)
	reauth := NewReauthChecker(config.Config{Auth: config.AuthConfig{Disabled: true}}, authUC, sessions, nil)
	h := &AuthHandler{AuthDeps: &AuthDeps{
		authUC: authUC, users: users, sessions: sessions, reauth: reauth,
	}}

	sv, ok := users.SessionVersion("admin")
	if !ok {
		t.Fatal("sv")
	}
	tok, _, err := sessions.Issue("admin", auth.RoleAdministrator, sv)
	if err != nil {
		t.Fatal(err)
	}
	cookieReq := func(method, path, body string) *http.Request {
		var r *http.Request
		if body != "" {
			r = httptest.NewRequest(method, path, strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
		} else {
			r = httptest.NewRequest(method, path, nil)
		}
		r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
		r.RemoteAddr = "127.0.0.1:9"
		return r
	}

	rec := httptest.NewRecorder()
	h.ChangePassword(rec, cookieReq(http.MethodPost, "/api/auth/change-password",
		`{"old_password":"adminpass1","new_password":"adminpass2"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("change: %d %s", rec.Code, rec.Body.String())
	}
	// bump invalidated old cookie — re-issue
	sv, _ = users.SessionVersion("admin")
	tok, _, err = sessions.Issue("admin", auth.RoleAdministrator, sv)
	if err != nil {
		t.Fatal(err)
	}

	dismissed := true
	_ = dismissed
	rec = httptest.NewRecorder()
	h.DismissGeoWizard(rec, cookieReq(http.MethodPost, "/api/auth/geo-wizard-dismiss", `{"dismissed":true}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("dismiss: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.LogoutAll(rec, cookieReq(http.MethodPost, "/api/auth/logout-all", `{"current_password":"x"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("logout-all: %d %s", rec.Code, rec.Body.String())
	}

	// unauthorized / bad body branches
	rec = call(h.ChangePassword, http.MethodPost, "/x", `{"old_password":"a","new_password":"b"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie: %d", rec.Code)
	}
	rec = call(h.DismissGeoWizard, http.MethodPost, "/x", `{}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("dismiss no cookie: %d", rec.Code)
	}
}

type pathGeoStore struct {
	idx    *geoip.Index
	ranges []model.GeoRange
}

func (m *pathGeoStore) Replace(_ context.Context, ranges []model.GeoRange) (int, error) {
	m.ranges = append([]model.GeoRange(nil), ranges...)
	if m.idx != nil {
		m.idx.ReplaceRanges(ranges)
	}
	return len(ranges), nil
}
func (m *pathGeoStore) Truncate(context.Context) error {
	m.ranges = nil
	if m.idx != nil {
		m.idx.ReplaceRanges(nil)
	}
	return nil
}
func (m *pathGeoStore) Load(context.Context) ([]model.GeoRange, error) {
	return append([]model.GeoRange(nil), m.ranges...), nil
}
func (m *pathGeoStore) Count(context.Context) (int, error) { return len(m.ranges), nil }
func (m *pathGeoStore) FindByIP(_ context.Context, ipStr string) (model.GeoRange, bool, error) {
	ip := geoip.IPToUint32(ipStr)
	for _, g := range m.ranges {
		if ip >= g.StartIP && ip <= g.EndIP {
			return g, true, nil
		}
	}
	return model.GeoRange{}, false, nil
}
func (m *pathGeoStore) ListPage(_ context.Context, limit int, _ string) ([]model.GeoRange, int, int, bool, error) {
	n := len(m.ranges)
	if limit <= 0 || limit > n {
		limit = n
	}
	return append([]model.GeoRange(nil), m.ranges[:limit]...), n, n, false, nil
}

type pathGeoJobs struct{}

func (pathGeoJobs) ScheduleReloadAndEnrich(context.Context, time.Duration) {}

type pathMissing struct {
	rows []model.GeoMissingIPRow
}

func (m pathMissing) ScanGeoMissingIPsForTimeRange(context.Context, model.TimeRange, int, time.Duration) ([]model.GeoMissingIPRow, error) {
	return m.rows, nil
}

type pathEntStore struct {
	nets map[[2]uint32]model.EnterpriseNet
}

func (m *pathEntStore) ListEnterpriseNets(context.Context) ([]model.EnterpriseNet, error) {
	out := make([]model.EnterpriseNet, 0, len(m.nets))
	for _, n := range m.nets {
		out = append(out, n)
	}
	return out, nil
}
func (m *pathEntStore) UpsertEnterpriseNet(_ context.Context, net model.EnterpriseNet) error {
	if m.nets == nil {
		m.nets = map[[2]uint32]model.EnterpriseNet{}
	}
	m.nets[[2]uint32{net.StartIP, net.EndIP}] = net
	return nil
}
func (m *pathEntStore) DeleteEnterpriseNet(_ context.Context, startIP, endIP uint32) error {
	delete(m.nets, [2]uint32{startIP, endIP})
	return nil
}
func (m *pathEntStore) CountEnterpriseNets(context.Context) (int, error) { return len(m.nets), nil }

type pathTraffic struct{}

func (pathTraffic) ScanMapAggs(context.Context, model.TimeRange, usecaseevents.MapScanQuery, time.Duration) (usecaseevents.MapAggScanResult, error) {
	return usecaseevents.MapAggScanResult{}, nil
}
func (pathTraffic) ScanCountrySeries(context.Context, model.TimeRange, string, string, time.Duration) ([]usecaseevents.SeriesPoint, int, error) {
	return []usecaseevents.SeriesPoint{{T: time.Unix(1, 0).UTC(), Allowed: 1, Blocked: 2, Total: 3}}, 3600, nil
}

func TestGeoRangesEnterpriseMissingHappy(t *testing.T) {
	idx := geoip.New()
	store := &pathGeoStore{idx: idx}
	geoUC := usecasegeo.New(store, pathMissing{rows: []model.GeoMissingIPRow{{
		IP: "9.9.9.9", Count: 2, AsSrc: 1, AsDst: 1, SamplePeer: "1.1.1.1",
		LogCountry: "XX", LogCity: "Nowhere",
	}}}, idx, pathGeoJobs{}, geoipcodec.New(), 100_000)
	ent := &pathEntStore{}
	geoUC.SetEnterpriseStore(ent)
	h := &GeoHandler{GeoDeps: &GeoDeps{
		geoUC: geoUC, maxGeoUploadSize: 1 << 20, maxGeoUploadRanges: 1000, queryTimeout: time.Minute,
	}}

	rec := call(h.AppendGeoRange, http.MethodPost, "/api/geo/ranges",
		`{"network":"203.0.113.0/24","country":"US","region":"CA","city":"Lab","lat":37.4,"lon":-122.1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("append: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.ListGeoRanges, http.MethodGet, "/api/geo/ranges?limit=50&q=US", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.ListGeoRanges, http.MethodGet, "/api/geo/ranges?ip=203.0.113.10", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list ip: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.UpdateGeoRange, http.MethodPut, "/api/geo/ranges",
		`{"original_network":"203.0.113.0/24","network":"203.0.113.0/24","country":"US","region":"CA","city":"Lab2","lat":37.4,"lon":-122.1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.ExportGeoRangesCSV, http.MethodGet, "/api/geo/ranges.csv", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Network") {
		t.Fatalf("export: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h.AddEnterpriseNet, http.MethodPost, "/api/geo/enterprise-nets",
		`{"network":"10.50.0.0/16","label":"corp","country":"RU","region":"MOW","city":"Moscow"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("add ent: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.ListEnterpriseNets, http.MethodGet, "/api/geo/enterprise-nets", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list ent: %d", rec.Code)
	}
	var start, end uint32
	for k := range ent.nets {
		start, end = k[0], k[1]
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/geo/enterprise-nets/"+strconv.FormatUint(uint64(start), 10), nil)
	req.SetPathValue("start_ip", strconv.FormatUint(uint64(start), 10))
	req.SetPathValue("end_ip", strconv.FormatUint(uint64(end), 10))
	rec = httptest.NewRecorder()
	h.DeleteEnterpriseNet(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("del ent: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h.GetGeoMissing, http.MethodGet, "/api/geo/missing?hours=6&limit=20", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("missing hours: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.GetGeoMissing, http.MethodGet,
		"/api/geo/missing?from=2024-01-01T00:00:00Z&to=2024-01-02T00:00:00Z&limit=10", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("missing abs: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h.ClearGeoRanges, http.MethodPost, "/api/geo/ranges/clear", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body.String())
	}
}

func TestEventsSeriesHappy(t *testing.T) {
	evUC := usecaseevents.New(pathTraffic{}, geoip.New(), nil)
	h := &EventsHandler{EventsDeps: &EventsDeps{eventsUC: evUC, queryTimeout: time.Minute}}
	rec := call(h.GetEventsSeries, http.MethodGet, "/api/events/series?country=RU&hours=6", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("series: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.GetEventsSeries, http.MethodGet,
		"/api/events/series?country=US&from=2024-01-01T00:00:00Z&to=2024-01-02T00:00:00Z", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("series abs: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.GetEventsSeries, http.MethodGet, "/api/events/series?hours=6", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("no country: %d", rec.Code)
	}
	rec = call(h.GetEvents, http.MethodGet, "/api/events?period=1h&group_by=country&hours=6&filter=all", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("events map: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.GetEvents, http.MethodGet,
		"/api/events?from=2024-01-01T00:00:00Z&to=2024-01-02T00:00:00Z&group_by=city&source=backup", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("backup not attached: %d", rec.Code)
	}
	rec = call(h.GetEventsSeries, http.MethodGet, "/api/events/series?country=RU&source=backup&hours=1", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("series backup: %d", rec.Code)
	}
}

func TestAuthCheckMeWithSession(t *testing.T) {
	hash, err := auth.HashPassword("adminpass1")
	if err != nil {
		t.Fatal(err)
	}
	users, err := auth.NewUserStore(auth.User{
		Username: "admin", PasswordHash: string(hash), Role: auth.RoleAdministrator,
	})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := auth.NewSessionManager("check-me-session-secret-key-32b!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	authUC := usecaseauth.New(users, sessions)
	h := &AuthHandler{AuthDeps: &AuthDeps{authUC: authUC, users: users, sessions: sessions}}
	sv, _ := users.SessionVersion("admin")
	tok, _, err := sessions.Issue("admin", auth.RoleAdministrator, sv)
	if err != nil {
		t.Fatal(err)
	}
	withCookie := func(method, path string) *http.Request {
		req := httptest.NewRequest(method, path, nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
		return req
	}
	rec := httptest.NewRecorder()
	h.Me(rec, withCookie(http.MethodGet, "/api/auth/me"))
	if rec.Code != http.StatusOK {
		t.Fatalf("me: %d %s", rec.Code, rec.Body.String())
	}
	for _, fn := range []http.HandlerFunc{h.Check, h.CheckOps, h.CheckAdmin} {
		rec = httptest.NewRecorder()
		fn(rec, withCookie(http.MethodGet, "/x"))
		if rec.Code != http.StatusOK {
			t.Fatalf("check: %d", rec.Code)
		}
	}
	if _, err := users.Create("op", "oppass12345", auth.RoleOperator, "Op", false); err != nil {
		t.Fatal(err)
	}
	svOp, _ := users.SessionVersion("op")
	tokOp, _, err := sessions.Issue("op", auth.RoleOperator, svOp)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tokOp})
	rec = httptest.NewRecorder()
	h.CheckOps(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("ops for op: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.CheckAdmin(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin for op: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Check(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("check for op: %d", rec.Code)
	}
}

func TestGeoUploadDryAndApply(t *testing.T) {
	idx := geoip.New()
	store := &pathGeoStore{idx: idx}
	geoUC := usecasegeo.New(store, pathMissing{}, idx, pathGeoJobs{}, geoipcodec.New(), 100_000)
	h := &GeoHandler{GeoDeps: &GeoDeps{
		geoUC: geoUC, maxGeoUploadSize: 1 << 20, maxGeoUploadRanges: 100_000,
	}}
	csv := "Network,Country,Region,City,Latitude,Longitude\n198.51.100.0/24,US,TX,Lab,30.0,-97.0\n"
	req := httptest.NewRequest(http.MethodPost, "/upload-geo?dry_run=1", strings.NewReader(csv))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	h.UploadGeo(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/upload-geo", strings.NewReader(csv))
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	h.UploadGeo(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body.String())
	}

	rec = call((&GeoHandler{GeoDeps: &GeoDeps{}}).UploadGeo, http.MethodPost, "/upload-geo", csv)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil geo: %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/upload-geo", strings.NewReader(csv))
	req.ContentLength = 1 << 30
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	h.UploadGeo(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("too large: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/upload-geo", strings.NewReader("--b\r\n--b--\r\n"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=b")
	rec = httptest.NewRecorder()
	h.UploadGeo(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("multipart missing: %d", rec.Code)
	}
}

type pathIngest struct{}

func (pathIngest) Stats() model.IngestLiveStats { return model.IngestLiveStats{State: "running"} }
func (pathIngest) FeedReader(context.Context, io.Reader, string) (model.IngestStats, error) {
	return model.IngestStats{Received: 1, Queued: 1}, nil
}

func TestIngestHappyPaths(t *testing.T) {
	h := &IngestHandler{IngestDeps: &IngestDeps{ingest: pathIngest{}, ingestFlushSec: 1}}
	rec := call(h.GetIngestStats, http.MethodGet, "/api/ingest/stats", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: %d", rec.Code)
	}
	rec = call(h.IngestLogs, http.MethodPost, "/api/ingest", "src=1.1.1.1 dst=8.8.8.8\n")
	if rec.Code != http.StatusAccepted && rec.Code != http.StatusOK {
		t.Fatalf("ingest: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.UploadLogs, http.MethodPost, "/api/ingest/upload", "line\n")
	if rec.Code != http.StatusAccepted && rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
}
