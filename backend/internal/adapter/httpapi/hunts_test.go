package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"geoatlas/internal/adapter/httpapi/authmw"
	"geoatlas/internal/auth"
	usecasehunts "geoatlas/internal/usecase/hunts"
)

type huntMemStore struct {
	items map[string][]usecasehunts.Hunt
}

func newHuntMem() *huntMemStore {
	return &huntMemStore{items: map[string][]usecasehunts.Hunt{}}
}

func (m *huntMemStore) List(username string) ([]usecasehunts.Hunt, error) {
	return append([]usecasehunts.Hunt(nil), m.items[username]...), nil
}

func (m *huntMemStore) ListAll() ([]usecasehunts.HuntWithAuthor, error) {
	out := make([]usecasehunts.HuntWithAuthor, 0)
	for u, list := range m.items {
		for _, h := range list {
			out = append(out, usecasehunts.HuntWithAuthor{Hunt: h, Username: u})
		}
	}
	return out, nil
}

func (m *huntMemStore) Create(username string, hunt usecasehunts.Hunt) (usecasehunts.Hunt, error) {
	hunt.ID = "id-1"
	m.items[username] = append(m.items[username], hunt)
	return hunt, nil
}

func (m *huntMemStore) Update(username, id string, hunt usecasehunts.Hunt) (usecasehunts.Hunt, error) {
	list := m.items[username]
	for i := range list {
		if list[i].ID != id {
			continue
		}
		hunt.ID = id
		list[i] = hunt
		m.items[username] = list
		return hunt, nil
	}
	return usecasehunts.Hunt{}, usecasehunts.ErrNotFound
}

func (m *huntMemStore) Delete(username, id string) error {
	list := m.items[username]
	next := make([]usecasehunts.Hunt, 0, len(list))
	found := false
	for _, h := range list {
		if h.ID == id {
			found = true
			continue
		}
		next = append(next, h)
	}
	if !found {
		return usecasehunts.ErrNotFound
	}
	m.items[username] = next
	return nil
}

func (m *huntMemStore) SaveRun(username, id string, hunt usecasehunts.Hunt) error {
	_, err := m.Update(username, id, hunt)
	return err
}

type huntRunnerOK struct{}

func (huntRunnerOK) RunMap(context.Context, usecasehunts.MapState, time.Duration) (usecasehunts.MapSnapshot, error) {
	return usecasehunts.MapSnapshot{EdgeCount: 2, RawPairs: 2}, nil
}

func withUser(r *http.Request, user string) *http.Request {
	ctx := authmw.ContextWithSession(r.Context(), auth.Session{Username: user, Role: auth.RoleOperator})
	return r.WithContext(ctx)
}

func TestHuntsHandlerUnavailableAndUnauthorized(t *testing.T) {
	h := NewHuntsHandler(nil)
	rec := httptest.NewRecorder()
	h.ListMine(rec, httptest.NewRequest(http.MethodGet, "/api/hunts", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil svc: %d", rec.Code)
	}
	svc := usecasehunts.New(newHuntMem(), nil, nil, nil, time.Minute)
	h = NewHuntsHandler(svc)
	rec = httptest.NewRecorder()
	h.ListMine(rec, httptest.NewRequest(http.MethodGet, "/api/hunts", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no session: %d", rec.Code)
	}
}

func TestHuntsHandlerCRUDAndRun(t *testing.T) {
	svc := usecasehunts.New(newHuntMem(), huntRunnerOK{}, nil, nil, time.Minute)
	h := NewHuntsHandler(svc)

	body := []byte(`{"name":"h1","map":{"period":"1d","group_by":"city","filter":"all"},"schedule":{"enabled":false,"interval_min":60,"edge_threshold":0,"edge_ratio":3}}`)
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/hunts", bytes.NewReader(body)), "alice")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateMine(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ListMine(rec, withUser(httptest.NewRequest(http.MethodGet, "/api/hunts", nil), "alice"))
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var listed struct {
		Hunts []usecasehunts.Hunt `json:"hunts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || len(listed.Hunts) != 1 {
		t.Fatalf("list body: %s err=%v", rec.Body.String(), err)
	}
	id := listed.Hunts[0].ID

	upd := []byte(`{"name":"h2","map":{"period":"1d","group_by":"city","filter":"all"},"schedule":{"enabled":false,"interval_min":60,"edge_threshold":0,"edge_ratio":3}}`)
	req = withUser(httptest.NewRequest(http.MethodPut, "/api/hunts/"+id, bytes.NewReader(upd)), "alice")
	req.SetPathValue("id", id)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.UpdateMine(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}

	req = withUser(httptest.NewRequest(http.MethodPost, "/api/hunts/"+id+"/run", nil), "alice")
	req.SetPathValue("id", id)
	rec = httptest.NewRecorder()
	h.RunMine(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ListAll(rec, httptest.NewRequest(http.MethodGet, "/api/hunts/all", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("listall: %d", rec.Code)
	}

	req = withUser(httptest.NewRequest(http.MethodDelete, "/api/hunts/"+id, nil), "alice")
	req.SetPathValue("id", id)
	rec = httptest.NewRecorder()
	h.DeleteMine(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d", rec.Code)
	}
}
