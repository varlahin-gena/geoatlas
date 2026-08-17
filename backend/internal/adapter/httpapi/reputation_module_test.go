package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"network_monitor/internal/adapter/httpapi"
	"network_monitor/internal/auth"
	"network_monitor/internal/config"
	usecaseauth "network_monitor/internal/usecase/auth"
	usecasesystem "network_monitor/internal/usecase/system"
)

func TestReputationEnabledOnMe(t *testing.T) {
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
	sessions, err := auth.NewSessionManager("rep-flag-session-secret-key-32b!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	authUC := usecaseauth.New(users, sessions)
	systemUC := usecasesystem.New(usecasesystem.Dependencies{})

	cfg := config.Config{
		ListenAddr:             ":0",
		APIAuthToken:           "rep-flag-token",
		QueryTimeout:           time.Minute,
		ReputationFetchEnabled: true,
	}
	srv := httpapi.NewServer(httpapi.Params{
		Cfg:          cfg,
		SystemUC:     systemUC,
		SystemPinger: stubPinger{},
		AuthUC:       authUC,
		Users:        users,
		Sessions:     sessions,
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}

	resp, err := client.Post(ts.URL+"/api/auth/login", "application/json",
		strings.NewReader(`{"username":"admin","password":"adminpass1"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login %d", resp.StatusCode)
	}

	me, err := client.Get(ts.URL + "/api/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	defer me.Body.Close()
	if me.StatusCode != http.StatusOK {
		t.Fatalf("me %d", me.StatusCode)
	}
	var user map[string]any
	if err := json.NewDecoder(me.Body).Decode(&user); err != nil {
		t.Fatal(err)
	}
	if user["reputationEnabled"] != true {
		t.Fatalf("reputationEnabled: want true, got %#v", user["reputationEnabled"])
	}
}
