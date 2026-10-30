package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"geoatlas/internal/adapter/geoipcodec"
	"geoatlas/internal/adapter/httpapi/authmw"
	"geoatlas/internal/adapter/httpapi/loginthrottle"
	"geoatlas/internal/adapter/reputationcodec"
	"geoatlas/internal/apperr"
	"geoatlas/internal/auth"
	"geoatlas/internal/config"
	"geoatlas/internal/model"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
	usecaseauth "geoatlas/internal/usecase/auth"
	usecasebackup "geoatlas/internal/usecase/backup"
	usecasegeo "geoatlas/internal/usecase/geo"
	usecasehunts "geoatlas/internal/usecase/hunts"
	"geoatlas/internal/usecase/parseerrors"
	usecasereputation "geoatlas/internal/usecase/reputation"
	usecaseretention "geoatlas/internal/usecase/retention"
	"geoatlas/internal/usecase/searchtemplates"
	usecasesystem "geoatlas/internal/usecase/system"
	usecasetls "geoatlas/internal/usecase/tls"
)

func call(h http.HandlerFunc, method, path string, body string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestSystemHandlerNilAndBareUC(t *testing.T) {
	nilH := &SystemHandler{SystemDeps: &SystemDeps{queryTimeout: time.Second}}
	for _, fn := range []http.HandlerFunc{
		nilH.GetSystemStats, nilH.GetSystemStatus, nilH.GetSystemVersion, nilH.GetSystemHistory,
		nilH.GetEdgesAggStatus, nilH.GetInstallProfile, nilH.GetRetention, nilH.PutRetention,
		nilH.GetBackups, nilH.PostBackup, nilH.PostBackupAttach, nilH.PostBackupDetach,
		nilH.DeleteBackup, nilH.GetBackupSchedule, nilH.PutBackupSchedule,
		nilH.GetDRHistory, nilH.GetAuditLog, nilH.GetTLS, nilH.PutTLS, nilH.PostTLSReload,
	} {
		rec := call(fn, http.MethodGet, "/x", "")
		if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusBadRequest {
			// PutTLS without body → bad request via decode; others 503
			if rec.Code < 400 {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		}
	}
	rec := call(nilH.PostMaintenanceBackfill, http.MethodPost, "/x", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("backfill nil: %d", rec.Code)
	}
	_ = nilH.failedLoginsSnapshot()

	sys := usecasesystem.New(usecasesystem.Dependencies{})
	h := &SystemHandler{SystemDeps: &SystemDeps{queryTimeout: time.Second, systemUC: sys}}
	// error paths (metrics not configured) still exercise handlers
	for _, fn := range []http.HandlerFunc{h.GetSystemStats, h.GetSystemStatus, h.GetSystemHistory} {
		rec := call(fn, http.MethodGet, "/api/system/stats?period=1h&metrics=cpu", "")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("want 500, got %d %s", rec.Code, rec.Body.String())
		}
	}
	rec = call(h.GetSystemVersion, http.MethodGet, "/api/system/version", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("version: %d", rec.Code)
	}
	rec = call(h.GetEdgesAggStatus, http.MethodGet, "/api/system/edges-agg", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("edges: %d", rec.Code)
	}
	rec = call(h.PostMaintenanceBackfill, http.MethodPost, "/api/system/maintenance/backfill", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("backfill: %d", rec.Code)
	}
	rec = call(h.GetInstallProfile, http.MethodGet, "/api/system/install-profile", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("profile: %d", rec.Code)
	}
}

func TestReputationHandlerNilAndUC(t *testing.T) {
	nilH := &ReputationHandler{ReputationDeps: &ReputationDeps{}}
	for _, fn := range []http.HandlerFunc{
		nilH.UploadReputation, nilH.ListLists, nilH.DeleteList, nilH.Refresh,
		nilH.Lookup, nilH.ListFeeds, nilH.ListCatalog, nilH.AddFeed, nilH.RemoveFeed,
	} {
		rec := call(fn, http.MethodGet, "/api/reputation", `{"name":"x","url":"https://example.com"}`)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("nil: %d %s", rec.Code, rec.Body.String())
		}
	}

	store := &repMemStore{lists: map[string][]model.ReputationRange{}}
	idx := &repMemIndex{}
	uc := usecasereputation.New(store, idx, reputationcodec.New(), &repStubRef{}, nil)
	h := &ReputationHandler{ReputationDeps: &ReputationDeps{reputationUC: uc}}

	csv := "Network,List,Category\n1.2.3.0/24,custom,malware\n"
	req := httptest.NewRequest(http.MethodPost, "/api/reputation/upload?dry_run=1", strings.NewReader(csv))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	h.UploadReputation(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload dry: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/reputation/upload", strings.NewReader(csv))
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	h.UploadReputation(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h.ListLists, http.MethodGet, "/api/reputation/lists", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("lists: %d", rec.Code)
	}
	rec = call(h.Lookup, http.MethodGet, "/api/reputation/lookup?ip=1.2.3.4", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("lookup: %d", rec.Code)
	}
	rec = call(h.ListFeeds, http.MethodGet, "/api/reputation/feeds", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("feeds: %d", rec.Code)
	}
	rec = call(h.ListCatalog, http.MethodGet, "/api/reputation/catalog", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog: %d", rec.Code)
	}
	rec = call(h.Refresh, http.MethodPost, "/api/reputation/refresh?force=1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/reputation/lists/custom", nil)
	req.SetPathValue("name", "custom")
	rec = httptest.NewRecorder()
	h.DeleteList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}

	// multipart missing file
	req = httptest.NewRequest(http.MethodPost, "/api/reputation/upload", strings.NewReader("--b\r\n--b--\r\n"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=b")
	rec = httptest.NewRecorder()
	h.UploadReputation(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("multipart missing: %d", rec.Code)
	}
}

type repMemStore struct {
	lists map[string][]model.ReputationRange
}

func (m *repMemStore) Load(ctx context.Context) ([]model.ReputationRange, error) {
	var out []model.ReputationRange
	for _, rs := range m.lists {
		out = append(out, rs...)
	}
	return out, nil
}
func (m *repMemStore) ReplaceAll(ctx context.Context, ranges []model.ReputationRange) (int, error) {
	m.lists = map[string][]model.ReputationRange{}
	return m.ReplaceList(ctx, "", ranges)
}
func (m *repMemStore) ReplaceList(ctx context.Context, listName string, ranges []model.ReputationRange) (int, error) {
	if m.lists == nil {
		m.lists = map[string][]model.ReputationRange{}
	}
	if listName == "" && len(ranges) > 0 {
		listName = ranges[0].ListName
	}
	m.lists[listName] = ranges
	return len(ranges), nil
}
func (m *repMemStore) DeleteList(ctx context.Context, listName string) error {
	delete(m.lists, listName)
	return nil
}
func (m *repMemStore) ListMeta(ctx context.Context) ([]model.ReputationListMeta, error) {
	return nil, nil
}

type repMemIndex struct{ n int }

func (repMemIndex) Lookup(string) []model.ReputationHit     { return nil }
func (i *repMemIndex) RangeCount() int                      { return i.n }
func (i *repMemIndex) ReplaceAll(r []model.ReputationRange) { i.n = len(r) }
func (i *repMemIndex) ReplaceList(_ string, r []model.ReputationRange) {
	i.n = len(r)
}
func (i *repMemIndex) DeleteList(string)                 { i.n = 0 }
func (repMemIndex) ListMeta() []model.ReputationListMeta { return nil }
func (repMemIndex) Snapshot() []model.ReputationRange    { return nil }

type repStubRef struct{}

func (repStubRef) RefreshAll(ctx context.Context, force bool) (usecasereputation.RefreshResult, error) {
	return usecasereputation.RefreshResult{Updated: []string{"x"}}, nil
}
func (repStubRef) SetFeeds([]usecasereputation.Feed) {}

type tplMemStore struct {
	users map[string][]searchtemplates.Template
}

func (m *tplMemStore) List(username string) ([]searchtemplates.Template, error) {
	return append([]searchtemplates.Template(nil), m.users[username]...), nil
}
func (m *tplMemStore) ListAll() ([]searchtemplates.TemplateWithAuthor, error) {
	return nil, nil
}
func (m *tplMemStore) Create(username, name, query string) (searchtemplates.Template, error) {
	t := searchtemplates.Template{ID: "id-1", Name: name, Query: query, UpdatedAt: "now"}
	if m.users == nil {
		m.users = map[string][]searchtemplates.Template{}
	}
	m.users[username] = append(m.users[username], t)
	return t, nil
}
func (m *tplMemStore) Update(username, id, name, query string) (searchtemplates.Template, error) {
	list := m.users[username]
	for i := range list {
		if list[i].ID == id {
			list[i].Name, list[i].Query = name, query
			m.users[username] = list
			return list[i], nil
		}
	}
	return searchtemplates.Template{}, searchtemplates.ErrNotFound
}
func (m *tplMemStore) Delete(username, id string) error {
	list := m.users[username]
	next := list[:0]
	found := false
	for _, t := range list {
		if t.ID == id {
			found = true
			continue
		}
		next = append(next, t)
	}
	if !found {
		return searchtemplates.ErrNotFound
	}
	m.users[username] = next
	return nil
}

func TestSearchTemplatesHandlerPaths(t *testing.T) {
	nilH := &SearchTemplatesHandler{SearchTemplatesDeps: &SearchTemplatesDeps{}}
	for _, fn := range []http.HandlerFunc{nilH.ListMine, nilH.CreateMine, nilH.UpdateMine, nilH.DeleteMine, nilH.ListAll} {
		rec := call(fn, http.MethodGet, "/x", `{"name":"n","query":"q"}`)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("nil tpl: %d", rec.Code)
		}
	}

	svc := searchtemplates.New(&tplMemStore{})
	h := &SearchTemplatesHandler{SearchTemplatesDeps: &SearchTemplatesDeps{
		authDisabled: true, searchTemplates: svc,
	}}
	rec := call(h.ListMine, http.MethodGet, "/api/search-templates", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.CreateMine, http.MethodPost, "/api/search-templates", `{"name":"mine","query":"country:RU"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodPut, "/api/search-templates/id-1", strings.NewReader(`{"name":"mine2","query":"country:US"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "id-1")
	rec = httptest.NewRecorder()
	h.UpdateMine(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/search-templates/id-1", nil)
	req.SetPathValue("id", "id-1")
	rec = httptest.NewRecorder()
	h.DeleteMine(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d", rec.Code)
	}
	rec = call(h.ListAll, http.MethodGet, "/api/search-templates/all", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("listall: %d", rec.Code)
	}

	// unauthorized when auth on and no session
	h2 := &SearchTemplatesHandler{SearchTemplatesDeps: &SearchTemplatesDeps{
		searchTemplates: svc,
	}}
	rec = call(h2.ListMine, http.MethodGet, "/x", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: %d", rec.Code)
	}
	// missing id
	req = httptest.NewRequest(http.MethodPut, "/x", strings.NewReader(`{"name":"n","query":"q"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "")
	rec = httptest.NewRecorder()
	h.UpdateMine(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing id: %d", rec.Code)
	}
}

func TestAnomalyHandlerDisabledPaths(t *testing.T) {
	h := &AnomalyHandler{AnomalyDeps: &AnomalyDeps{authDisabled: true, queryTimeout: time.Second}}
	for _, fn := range []http.HandlerFunc{h.List, h.Summary, h.Status, h.Episodes} {
		rec := call(fn, http.MethodGet, "/api/anomalies?include_acked=1&limit=10&severity=high&code=X&since=2020-01-01T00:00:00Z", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("anomaly ok path: %d", rec.Code)
		}
	}
	for _, fn := range []http.HandlerFunc{h.Ack, h.Assign, h.GetSettings, h.PutSettings} {
		rec := call(fn, http.MethodPost, "/api/anomalies/x", `{"assigned_to":"a"}`)
		if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusBadRequest {
			t.Fatalf("disabled write: %d", rec.Code)
		}
	}
	if h.actorName(httptest.NewRequest(http.MethodGet, "/", nil)) != "anonymous" {
		t.Fatal("actor")
	}
}

func TestParseHandlerUnavailable(t *testing.T) {
	h := &ParseHandler{ParseDeps: &ParseDeps{queryTimeout: time.Second}}
	rec := call(h.ListParseErrors, http.MethodGet, "/api/parse-errors?limit=10&search=x", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("list: %d", rec.Code)
	}
	rec = call(h.DeleteParseErrors, http.MethodPost, "/api/parse-errors", `{"all":true}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("delete: %d", rec.Code)
	}
	rec = call(h.ParseTest, http.MethodPost, "/api/parse-test", `{}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("parsetest: %d", rec.Code)
	}
	rec = call(h.ParseSamples, http.MethodGet, "/api/parse-samples", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("samples nil: %d", rec.Code)
	}
}

func TestGeoHandlerClearExportNil(t *testing.T) {
	h := &GeoHandler{GeoDeps: &GeoDeps{}}
	rec := call(h.ClearGeoRanges, http.MethodPost, "/api/geo/ranges/clear", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("clear: %d", rec.Code)
	}
	rec = call(h.ExportGeoRangesCSV, http.MethodGet, "/api/geo/ranges.csv", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("export: %d", rec.Code)
	}
}

func TestAPITokensUnavailableAndErrors(t *testing.T) {
	h := &APITokensHandler{AuthDeps: &AuthDeps{}}
	for _, fn := range []http.HandlerFunc{h.List, h.Create, h.Rotate, h.Revoke} {
		rec := call(fn, http.MethodGet, "/x", `{"current_password":"x"}`)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("tokens nil: %d", rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	writeTokenStoreError(rec, auth.ErrInvalidTokenName)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid name: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	writeTokenStoreError(rec, auth.ErrTokenNotFound)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("not found: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	writeTokenStoreError(rec, auth.ErrTokenLimit)
	if rec.Code != http.StatusConflict {
		t.Fatalf("limit: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	writeTokenStoreError(rec, errors.New("boom"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("internal: %d", rec.Code)
	}
	if _, err := parseOptionalExpiry("not-rfc"); err == nil {
		t.Fatal("bad expiry")
	}
	if _, err := parseOptionalExpiry(time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if _, err := parseOptionalExpiry(""); err != nil {
		t.Fatal(err)
	}
}

func TestServerWithMetricsOption(t *testing.T) {
	s := NewServer(Params{}, WithMetrics(nil), nil)
	if s == nil || s.Handler() == nil {
		t.Fatal("server")
	}
}

func TestIngestUploadLogsNil(t *testing.T) {
	h := &IngestHandler{IngestDeps: &IngestDeps{}}
	rec := call(h.UploadLogs, http.MethodPost, "/api/ingest/upload", "line\n")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("upload nil: %d %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/upload", strings.NewReader("--b\r\n--b--"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=b")
	rec = httptest.NewRecorder()
	h.UploadLogs(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("multipart: %d", rec.Code)
	}
}

func TestReputationAddRemoveFeedWithoutStore(t *testing.T) {
	store := &repMemStore{lists: map[string][]model.ReputationRange{}}
	uc := usecasereputation.New(store, &repMemIndex{}, reputationcodec.New(), &repStubRef{}, nil)
	h := &ReputationHandler{ReputationDeps: &ReputationDeps{reputationUC: uc}}
	rec := call(h.AddFeed, http.MethodPost, "/api/reputation/feeds", `{"name":"f1","url":"https://example.com/a","format":"netset"}`)
	if rec.Code < 400 {
		t.Fatalf("add without feed store should fail: %d %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/reputation/feeds/f1", nil)
	req.SetPathValue("name", "f1")
	rec = httptest.NewRecorder()
	h.RemoveFeed(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body.String())
	}
}

func TestClientIPAndAuditHelpers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	if ip := clientIPFromRequest(req); ip == "" {
		t.Fatal("expected ip")
	}
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	_ = clientIPFromRequest(req)
}

type pathRetStore struct{ s usecaseretention.Settings }

func (m *pathRetStore) Load() (usecaseretention.Settings, error) {
	if m.s.TrafficLogsDays == 0 {
		return usecaseretention.Defaults(), nil
	}
	return m.s, nil
}
func (m *pathRetStore) Save(s usecaseretention.Settings) error { m.s = s; return nil }

type pathRetApply struct{}

func (pathRetApply) ApplyTrafficLogs(context.Context, int) error   { return nil }
func (pathRetApply) ApplyEdges(context.Context, int) error         { return nil }
func (pathRetApply) ApplyParseErrors(context.Context, int) error   { return nil }
func (pathRetApply) ApplySystemMetrics(context.Context, int) error { return nil }

type pathBakStore struct {
	mu       sync.Mutex
	ready    bool
	entries  []usecasebackup.Entry
	exists   map[string]bool
	attached string
}

func newPathBakStore() *pathBakStore {
	return &pathBakStore{ready: true, exists: map[string]bool{}}
}
func (m *pathBakStore) DirReady() bool { return m.ready }
func (m *pathBakStore) List() ([]usecasebackup.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]usecasebackup.Entry(nil), m.entries...), nil
}
func (m *pathBakStore) Exists(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.exists[name]
}
func (m *pathBakStore) WriteAuthTarball(string, string) error { return nil }
func (m *pathBakStore) WriteSource(name, source string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exists[name] = true
	m.entries = append(m.entries, usecasebackup.Entry{Name: name, Source: source, CreatedAt: time.Now().UTC()})
	return nil
}
func (m *pathBakStore) Prune(int) error { return nil }
func (m *pathBakStore) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.exists, name)
	next := m.entries[:0]
	for _, e := range m.entries {
		if e.Name != name {
			next = append(next, e)
		}
	}
	m.entries = next
	return nil
}
func (m *pathBakStore) Attached() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attached, nil
}
func (m *pathBakStore) SetAttached(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attached = name
	return nil
}

type pathBakSched struct {
	mu  sync.Mutex
	sch usecasebackup.Schedule
}

func (m *pathBakSched) Load() (usecasebackup.Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sch, nil
}
func (m *pathBakSched) Save(s usecasebackup.Schedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sch = s
	return nil
}

type pathBakRunner struct{}

func (pathBakRunner) BackupTables(context.Context, string, []string) error { return nil }
func (pathBakRunner) RestoreMapShadow(context.Context, string) error       { return nil }
func (pathBakRunner) DropMapShadow(context.Context) error                  { return nil }
func (pathBakRunner) TableExists(context.Context, string) (bool, error)    { return true, nil }

func waitBakIdle(t *testing.T, s *usecasebackup.Service) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.Status().State != "running" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("backup still running: %+v", s.Status())
}

func pathSelfSignedPEM(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "geoatlas.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"geoatlas.test"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return string(certPEM), string(keyPEM)
}

func TestSystemRetentionBackupTLSAuditHappy(t *testing.T) {
	retUC := usecaseretention.New(&pathRetStore{}, pathRetApply{})
	bakStore := newPathBakStore()
	bakStore.exists["ga-1"] = true
	bakStore.entries = []usecasebackup.Entry{{Name: "ga-1", Source: usecasebackup.SourceManual, CreatedAt: time.Now().UTC()}}
	sch := &pathBakSched{sch: usecasebackup.DefaultsSchedule(usecasebackup.Options{Keep: 7})}
	bakUC := usecasebackup.New(usecasebackup.Options{Enabled: true, Keep: 7}, pathBakRunner{}, bakStore, sch)
	logs := usecaseaudit.New(&fakeAuditStore{
		audit: []usecaseaudit.AuditEvent{{Actor: "admin", Action: "auth.login", Result: "succeeded"}},
	})
	tlsUC := usecasetls.New(usecasetls.Config{
		CertDir: t.TempDir(), CertFile: "fullchain.pem", KeyFile: "privkey.pem",
		HTTPSEnabled: "auto", HTTPSPort: "443", HTTPRedirect: "1",
	})
	reauth := NewReauthChecker(config.Config{Auth: config.AuthConfig{Disabled: true}}, nil, nil, nil)
	lim := loginthrottle.New(5, time.Minute, time.Minute)
	h := &SystemHandler{SystemDeps: &SystemDeps{
		queryTimeout: time.Second, retentionUC: retUC, backupUC: bakUC, tlsUC: tlsUC,
		reauth: reauth, logs: logs, loginLimiter: lim,
	}}

	rec := call(h.GetRetention, http.MethodGet, "/api/system/retention", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get ret: %d", rec.Code)
	}
	rec = call(h.PutRetention, http.MethodPut, "/api/system/retention",
		`{"traffic_logs_days":14,"edges_days":21,"parse_errors_days":3,"system_metrics_days":5}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put ret: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h.GetBackups, http.MethodGet, "/api/system/backups", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog: %d", rec.Code)
	}
	rec = call(h.PostBackup, http.MethodPost, "/api/system/backups", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	waitBakIdle(t, bakUC)

	req := httptest.NewRequest(http.MethodPost, "/api/system/backups/ga-1/attach", nil)
	req.SetPathValue("name", "ga-1")
	rec = httptest.NewRecorder()
	h.PostBackupAttach(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("attach: %d %s", rec.Code, rec.Body.String())
	}
	waitBakIdle(t, bakUC)

	req = httptest.NewRequest(http.MethodPost, "/api/system/backups/ga-1/detach", nil)
	req.SetPathValue("name", "ga-1")
	rec = httptest.NewRecorder()
	h.PostBackupDetach(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("detach: %d %s", rec.Code, rec.Body.String())
	}
	waitBakIdle(t, bakUC)

	req = httptest.NewRequest(http.MethodDelete, "/api/system/backups/ga-1", nil)
	req.SetPathValue("name", "ga-1")
	rec = httptest.NewRecorder()
	h.DeleteBackup(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete bak: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h.GetBackupSchedule, http.MethodGet, "/api/system/backup-schedule", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get sch: %d", rec.Code)
	}
	rec = call(h.PutBackupSchedule, http.MethodPut, "/api/system/backup-schedule",
		`{"enabled":true,"hour":3,"minute":15,"timezone":"UTC","keep":5,"include_edges":true,"include_auth":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put sch: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h.GetAuditLog, http.MethodGet, "/api/system/audit?limit=10&since=2020-01-01T00:00:00Z&action=auth.login&result=succeeded&actor=admin", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("audit: %d %s", rec.Code, rec.Body.String())
	}
	_ = h.failedLoginsSnapshot()

	rec = call(h.GetTLS, http.MethodGet, "/api/system/tls", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get tls: %d", rec.Code)
	}
	certPEM, keyPEM := pathSelfSignedPEM(t)
	body := `{"cert_pem":` + jsonQuote(certPEM) + `,"key_pem":` + jsonQuote(keyPEM) + `,"current_password":"x"}`
	rec = call(h.PutTLS, http.MethodPut, "/api/system/tls", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("put tls: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.PostTLSReload, http.MethodPost, "/api/system/tls/reload", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("reload: %d", rec.Code)
	}
}

func jsonQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

type pathAnomStore struct {
	list []usecaseanomaly.Event
	sum  usecaseanomaly.Summary
}

func (f *pathAnomStore) Insert(context.Context, []usecaseanomaly.Event) error { return nil }
func (f *pathAnomStore) List(context.Context, usecaseanomaly.ListQuery) ([]usecaseanomaly.Event, error) {
	return f.list, nil
}
func (f *pathAnomStore) ExistingFingerprints(context.Context, []string) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}
func (f *pathAnomStore) ActiveSuppressions(context.Context, []usecaseanomaly.SuppressionKey, time.Time) (map[usecaseanomaly.SuppressionKey]struct{}, error) {
	return map[usecaseanomaly.SuppressionKey]struct{}{}, nil
}
func (f *pathAnomStore) RecentSuppressionKeys(context.Context, string, []usecaseanomaly.SuppressionKey, time.Time) (map[usecaseanomaly.SuppressionKey]struct{}, error) {
	return map[usecaseanomaly.SuppressionKey]struct{}{}, nil
}
func (f *pathAnomStore) Ack(context.Context, string, string, time.Duration) error { return nil }
func (f *pathAnomStore) Assign(context.Context, string, string, string) error     { return nil }
func (f *pathAnomStore) AssignIfEmpty(context.Context, string, string, string) error {
	return nil
}
func (f *pathAnomStore) CountSummary(context.Context, time.Time) (usecaseanomaly.Summary, error) {
	return f.sum, nil
}

type pathAnomScan struct{}

func (pathAnomScan) OldestLogTime(context.Context) (time.Time, error) { return time.Time{}, nil }
func (pathAnomScan) PortScan(context.Context, time.Duration, int, int, bool, []usecaseanomaly.IPRange) ([]usecaseanomaly.PortScanHit, error) {
	return nil, nil
}
func (pathAnomScan) HorizontalScan(context.Context, time.Duration, int, int, bool, []usecaseanomaly.IPRange) ([]usecaseanomaly.HorizontalScanHit, error) {
	return nil, nil
}
func (pathAnomScan) BlockedCount(context.Context, time.Time, time.Time, *usecaseanomaly.IPRange) (uint64, error) {
	return 0, nil
}
func (pathAnomScan) CurrentCountries(context.Context, time.Duration, uint64, []usecaseanomaly.IPRange) ([]usecaseanomaly.CountryCount, error) {
	return nil, nil
}
func (pathAnomScan) CurrentCountryTotal(context.Context, time.Duration, []usecaseanomaly.IPRange) (uint64, error) {
	return 0, nil
}
func (pathAnomScan) BaselineCountries(context.Context, int, uint64, []usecaseanomaly.IPRange) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}
func (pathAnomScan) RecentEdges(context.Context, time.Duration, int, []usecaseanomaly.IPRange) ([]usecaseanomaly.EdgeRow, error) {
	return nil, nil
}
func (pathAnomScan) KnownPairs(context.Context, [][2]string, time.Duration) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}
func (pathAnomScan) ByteSurge(context.Context, time.Duration, uint64, uint64, []usecaseanomaly.IPRange) ([]usecaseanomaly.ByteSurgeHit, error) {
	return nil, nil
}
func (pathAnomScan) Beaconing(context.Context, time.Duration, int, uint64, []usecaseanomaly.IPRange) ([]usecaseanomaly.BeaconingHit, error) {
	return nil, nil
}
func (pathAnomScan) LateralFanout(context.Context, time.Duration, int, int, []usecaseanomaly.IPRange) ([]usecaseanomaly.LateralFanoutHit, error) {
	return nil, nil
}

type pathAnomSettings struct {
	data usecaseanomaly.Settings
}

func (m *pathAnomSettings) Load() (usecaseanomaly.Settings, error) { return m.data, nil }
func (m *pathAnomSettings) Save(s usecaseanomaly.Settings) error   { m.data = s; return nil }

func TestAnomalyAvailableAndSettings(t *testing.T) {
	store := &pathAnomStore{list: []usecaseanomaly.Event{{
		Fingerprint: "fp1", Code: usecaseanomaly.CodePortScan, Severity: "high",
	}}}
	uc := usecaseanomaly.New(usecaseanomaly.Config{
		Enabled: true, LearningDays: 3, SuppressHours: 24, InstallProfile: "medium",
	}, store, pathAnomScan{}, nil, nil, nil)
	seed := usecaseanomaly.DefaultSettingsFromConfig(config.AnomalyConfig{ScanInterval: 5 * time.Minute, LearningDays: 3, SuppressHours: 24})
	settings := usecaseanomaly.NewSettingsService(&pathAnomSettings{data: seed}, uc, seed, nil)
	h := &AnomalyHandler{AnomalyDeps: &AnomalyDeps{
		authDisabled: true, queryTimeout: time.Second, anomalyUC: uc, anomalySettings: settings,
	}}

	rec := call(h.List, http.MethodGet, "/api/anomalies?include_acked=1&limit=10&severity=high&code=port_scan&since=2020-01-01T00:00:00Z", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.Summary, http.MethodGet, "/api/anomalies/summary", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("summary: %d", rec.Code)
	}
	rec = call(h.Status, http.MethodGet, "/api/anomalies/status", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	rec = call(h.Episodes, http.MethodGet, "/api/anomalies/episodes?include_acked=1&since=2020-01-01T00:00:00Z", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("episodes: %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/anomalies/fp1/ack", nil)
	req.SetPathValue("fingerprint", "fp1")
	rec = httptest.NewRecorder()
	h.Ack(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ack: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/anomalies/fp1/assign", strings.NewReader(`{"assigned_to":"alice"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("fingerprint", "fp1")
	rec = httptest.NewRecorder()
	h.Assign(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("assign: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.GetSettings, http.MethodGet, "/api/anomalies/settings", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get settings: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.PutSettings, http.MethodPut, "/api/anomalies/settings",
		`{"enabled":false,"scan_interval_min":15,"learning_days":7,"suppress_hours":12,"include_private":true,"new_country_min_share":0.08}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put settings: %d %s", rec.Code, rec.Body.String())
	}
	// actorName via session context (auth enabled)
	h2 := &AnomalyHandler{AnomalyDeps: &AnomalyDeps{anomalyUC: uc}}
	req = httptest.NewRequest(http.MethodPost, "/api/anomalies/fp1/ack", nil)
	req.SetPathValue("fingerprint", "fp1")
	req = req.WithContext(authmw.ContextWithSession(req.Context(), auth.Session{Username: "bob", Role: auth.RoleOperator}))
	rec = httptest.NewRecorder()
	h2.Ack(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ack sess: %d %s", rec.Code, rec.Body.String())
	}
	if h2.actorName(req) != "bob" {
		t.Fatalf("actor=%s", h2.actorName(req))
	}
	if h2.actorName(httptest.NewRequest(http.MethodGet, "/", nil)) != "bearer" {
		t.Fatal("bearer actor")
	}
}

func TestPutTLSInvalidPEM(t *testing.T) {
	tlsUC := usecasetls.New(usecasetls.Config{
		CertDir: t.TempDir(), CertFile: "fullchain.pem", KeyFile: "privkey.pem",
	})
	reauth := NewReauthChecker(config.Config{Auth: config.AuthConfig{Disabled: true}}, nil, nil, nil)
	h := &SystemHandler{SystemDeps: &SystemDeps{tlsUC: tlsUC, reauth: reauth}}
	rec := call(h.PutTLS, http.MethodPut, "/api/system/tls",
		`{"cert_pem":"not-a-cert","key_pem":"not-a-key","current_password":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid pem: %d %s", rec.Code, rec.Body.String())
	}
}

type pathPingerOK struct{}

func (pathPingerOK) Ping(context.Context) error { return nil }

func TestLoginReadyDomainErrorsHuntsBoost(t *testing.T) {
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
	sessions, err := auth.NewSessionManager("login-boost-session-secret-key32", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	authUC := usecaseauth.New(users, sessions)
	lim := loginthrottle.New(20, time.Minute, time.Minute)
	authH := &AuthHandler{AuthDeps: &AuthDeps{
		authUC: authUC, users: users, sessions: sessions, loginLimiter: lim,
	}}
	rec := call(authH.Login, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"adminpass1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(authH.Login, http.MethodPost, "/api/auth/login", `{"username":"","password":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("login empty: %d", rec.Code)
	}
	rec = call(authH.Logout, http.MethodPost, "/api/auth/logout", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("logout: %d", rec.Code)
	}
	rec = call((&AuthHandler{AuthDeps: &AuthDeps{authDisabled: true}}).Me, http.MethodGet, "/api/auth/me", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("me disabled: %d", rec.Code)
	}

	sysUC := usecasesystem.New(usecasesystem.Dependencies{})
	ready := &HealthHandler{HealthDeps: &HealthDeps{systemUC: sysUC, systemPinger: pathPingerOK{}}}
	rec = call(ready.Ready, http.MethodGet, "/api/ready", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("ready: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(ready.Live, http.MethodGet, "/api/live", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("live: %d", rec.Code)
	}

	for _, e := range []error{
		nil,
		&http.MaxBytesError{Limit: 10},
		apperr.TooLarge("big"),
		apperr.NotFound("gone"),
		apperr.Conflict("dup"),
		usecasebackup.ErrDisabled,
		usecasebackup.ErrUnavailable,
		searchtemplates.ErrUnavailable,
		apperr.InvalidInput("bad"),
		errors.New("other"),
	} {
		rec = httptest.NewRecorder()
		writeDomainError(rec, "x", e)
	}
	_ = parseLimit("nope")
	_ = parseLimit("")
	_ = parseLimit("5")
	tr, err := parseEventTimeRange(map[string][]string{"minutes": {"15"}})
	if err != nil || tr.Mode != "minutes" {
		t.Fatalf("minutes: %+v %v", tr, err)
	}

	svc := usecasehunts.New(newHuntMem(), huntRunnerOK{}, nil, nil, time.Minute)
	hh := NewHuntsHandler(svc)
	req := withUser(httptest.NewRequest(http.MethodDelete, "/api/hunts/missing", nil), "alice")
	req.SetPathValue("id", "missing")
	rec = httptest.NewRecorder()
	hh.DeleteMine(rec, req)
	if rec.Code < 400 {
		t.Fatalf("delete missing: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	hh.ListAll(rec, httptest.NewRequest(http.MethodGet, "/api/hunts/all", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("listall: %d", rec.Code)
	}

	tpl := searchtemplates.New(&tplMemStore{})
	th := &SearchTemplatesHandler{SearchTemplatesDeps: &SearchTemplatesDeps{
		searchTemplates: tpl, sessions: sessions,
	}}
	sv, _ := users.SessionVersion("admin")
	tok, _, err := sessions.Issue("admin", auth.RoleAdministrator, sv)
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/search-templates", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
	rec = httptest.NewRecorder()
	th.ListMine(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tpl cookie: %d %s", rec.Code, rec.Body.String())
	}

	rec = call((&IngestHandler{IngestDeps: &IngestDeps{}}).GetIngestStats, http.MethodGet, "/x", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("ingest nil stats: %d", rec.Code)
	}
	rec = call((&IngestHandler{IngestDeps: &IngestDeps{}}).IngestLogs, http.MethodPost, "/x", "line\n")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ingest nil: %d", rec.Code)
	}
	_ = metricsHandler(nil)
	_ = cookieSecure(nil)
	reqHTTPS := httptest.NewRequest(http.MethodGet, "/", nil)
	reqHTTPS.Header.Set("X-Forwarded-Proto", "https")
	if !cookieSecure(reqHTTPS) {
		t.Fatal("https forwarded")
	}
	reqSess := httptest.NewRequest(http.MethodGet, "/", nil)
	reqSess = reqSess.WithContext(authmw.ContextWithSession(reqSess.Context(), auth.Session{Username: "carol"}))
	if actorFromRequest(reqSess) != "carol" {
		t.Fatal("actor sess")
	}
	if actorFromRequest(nil) != "system" {
		t.Fatal("actor nil")
	}
	reqBad := httptest.NewRequest(http.MethodDelete, "/x", nil)
	reqBad.SetPathValue("start_ip", "bad")
	reqBad.SetPathValue("end_ip", "1")
	rec = httptest.NewRecorder()
	(&GeoHandler{GeoDeps: &GeoDeps{
		geoUC: usecasegeo.New(nil, nil, nil, nil, geoipcodec.New(), 0),
	}}).DeleteEnterpriseNet(rec, reqBad)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad ent ids: %d", rec.Code)
	}
	rec = call(hh.CreateMine, http.MethodPost, "/api/hunts", `{`)
	if rec.Code < 400 {
		t.Fatalf("hunts bad json with no user: %d", rec.Code)
	}

	svcNil := usecasehunts.New(&pathNilHuntStore{}, nil, nil, nil, time.Minute)
	hhNil := NewHuntsHandler(svcNil)
	rec = httptest.NewRecorder()
	hhNil.ListAll(rec, httptest.NewRequest(http.MethodGet, "/api/hunts/all", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("listall nil: %d", rec.Code)
	}

	if (&Server{}).Handler() != nil {
		t.Fatal("nil server handler")
	}
	failReady := &HealthHandler{HealthDeps: &HealthDeps{
		systemUC: sysUC, systemPinger: pathPingerFail{},
	}}
	rec = call(failReady.Ready, http.MethodGet, "/api/ready", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready fail: %d", rec.Code)
	}

	sv, _ = users.SessionVersion("admin")
	tok2, _, _ := sessions.Issue("admin", auth.RoleAdministrator, sv)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/change-password",
		strings.NewReader(`{"old_password":"wrong","new_password":"adminpass9"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok2})
	rec = httptest.NewRecorder()
	authH.ChangePassword(rec, req)
	if rec.Code < 400 {
		t.Fatalf("bad old pw: %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/auth/geo-wizard-dismiss", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok2})
	rec = httptest.NewRecorder()
	authH.DismissGeoWizard(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("dismiss missing: %d", rec.Code)
	}
	authBearer := &AuthHandler{AuthDeps: &AuthDeps{
		apiAuthTokens: []string{"boost-admin-token"},
		users:         users, sessions: sessions,
	}}
	req = httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	req.Header.Set("Authorization", "Bearer boost-admin-token")
	rec = httptest.NewRecorder()
	authBearer.Check(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("check bearer: %d", rec.Code)
	}
}

type pathNilHuntStore struct{}

func (pathNilHuntStore) List(string) ([]usecasehunts.Hunt, error) { return nil, nil }
func (pathNilHuntStore) ListAll() ([]usecasehunts.HuntWithAuthor, error) {
	return nil, nil
}
func (pathNilHuntStore) Create(string, usecasehunts.Hunt) (usecasehunts.Hunt, error) {
	return usecasehunts.Hunt{}, nil
}
func (pathNilHuntStore) Update(string, string, usecasehunts.Hunt) (usecasehunts.Hunt, error) {
	return usecasehunts.Hunt{}, usecasehunts.ErrNotFound
}
func (pathNilHuntStore) Delete(string, string) error { return usecasehunts.ErrNotFound }
func (pathNilHuntStore) SaveRun(string, string, usecasehunts.Hunt) error {
	return usecasehunts.ErrNotFound
}

type pathPingerFail struct{}

func (pathPingerFail) Ping(context.Context) error { return errors.New("ch down") }

type pathTokStore struct {
	mu   sync.Mutex
	byID map[string]auth.APITokenPublic
	n    int
}

func (m *pathTokStore) List() []auth.APITokenPublic {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]auth.APITokenPublic, 0, len(m.byID))
	for _, t := range m.byID {
		out = append(out, t)
	}
	return out
}
func (m *pathTokStore) Create(name, scope string, expiresAt time.Time) (auth.APITokenPublic, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byID == nil {
		m.byID = map[string]auth.APITokenPublic{}
	}
	m.n++
	id := "tok-" + strconv.Itoa(m.n)
	pub := auth.APITokenPublic{ID: id, Name: name, Scope: scope}
	if !expiresAt.IsZero() {
		pub.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
	}
	m.byID[id] = pub
	return pub, "secret-" + id, nil
}
func (m *pathTokStore) Rotate(id string) (auth.APITokenPublic, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pub, ok := m.byID[id]
	if !ok {
		return auth.APITokenPublic{}, "", auth.ErrTokenNotFound
	}
	return pub, "rotated-" + id, nil
}
func (m *pathTokStore) Revoke(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[id]; !ok {
		return auth.ErrTokenNotFound
	}
	delete(m.byID, id)
	return nil
}
func (m *pathTokStore) Verify(string) (string, bool) { return "", false }

func TestAPITokensCreateRotateRevoke(t *testing.T) {
	toks := &pathTokStore{}
	reauth := NewReauthChecker(config.Config{Auth: config.AuthConfig{Disabled: true}}, nil, nil, nil)
	h := &APITokensHandler{AuthDeps: &AuthDeps{apiTokens: toks, reauth: reauth}}

	rec := call(h.List, http.MethodGet, "/api/api-tokens", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	exp := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	rec = call(h.Create, http.MethodPost, "/api/api-tokens",
		`{"name":"bot","scope":"ops","expires_at":"`+exp+`","current_password":"x"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	id := toks.List()[0].ID
	req := httptest.NewRequest(http.MethodPost, "/api/api-tokens/"+id+"/rotate", strings.NewReader(`{"current_password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", id)
	rec = httptest.NewRecorder()
	h.Rotate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rotate: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/api-tokens/"+id+"/revoke", strings.NewReader(`{"current_password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", id)
	rec = httptest.NewRecorder()
	h.Revoke(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.Create, http.MethodPost, "/api/api-tokens", `{"name":"x","scope":"ops","expires_at":"bad","current_password":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad expiry: %d", rec.Code)
	}
}

type pathParseRepo struct {
	items []model.ParseErrorRow
}

func (s *pathParseRepo) List(context.Context, int, string) ([]model.ParseErrorRow, error) {
	return s.items, nil
}
func (s *pathParseRepo) Delete(context.Context, []string) error { return nil }
func (s *pathParseRepo) DeleteAll(context.Context) error        { return nil }

func TestParseErrorsServiceHappy(t *testing.T) {
	uc := parseerrors.New(&pathParseRepo{items: []model.ParseErrorRow{{ID: "1", Reason: "boom"}}})
	h := &ParseHandler{ParseDeps: &ParseDeps{queryTimeout: time.Second, parseErrorsUC: uc}}
	rec := call(h.ListParseErrors, http.MethodGet, "/api/parse-errors?limit=10&search=x", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.DeleteParseErrors, http.MethodPost, "/api/parse-errors", `{"all":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete all: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.DeleteParseErrors, http.MethodPost, "/api/parse-errors", `{"ids":["1"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete ids: %d %s", rec.Code, rec.Body.String())
	}
}
