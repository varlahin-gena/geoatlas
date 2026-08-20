package searchtemplatesfile

import (
	"errors"
	"path/filepath"
	"testing"

	"network_monitor/internal/usecase/searchtemplates"
)

func TestStoreCRUDAndIsolation(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "search_templates.json"))

	a, err := s.Create("admin", "RU blocks", "country:Россия AND rule:block")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if a.ID == "" || a.Name != "RU blocks" {
		t.Fatalf("unexpected template: %+v", a)
	}

	_, err = s.Create("operator", "Op query", "device:fw1")
	if err != nil {
		t.Fatalf("create operator: %v", err)
	}

	adminList, err := s.List("admin")
	if err != nil {
		t.Fatalf("list admin: %v", err)
	}
	if len(adminList) != 1 || adminList[0].Name != "RU blocks" {
		t.Fatalf("admin list = %+v", adminList)
	}

	opList, err := s.List("operator")
	if err != nil {
		t.Fatalf("list operator: %v", err)
	}
	if len(opList) != 1 || opList[0].Name != "Op query" {
		t.Fatalf("operator list = %+v", opList)
	}

	updated, err := s.Update("admin", a.ID, "RU blocks v2", "country:Россия")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "RU blocks v2" || updated.Query != "country:Россия" {
		t.Fatalf("updated = %+v", updated)
	}

	all, err := s.ListAll()
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("list all len=%d want 2", len(all))
	}
	for _, item := range all {
		if item.Username == "" || item.Name == "" {
			t.Fatalf("missing author/name: %+v", item)
		}
	}

	if err := s.Delete("admin", a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	adminList, err = s.List("admin")
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(adminList) != 0 {
		t.Fatalf("admin should be empty, got %+v", adminList)
	}
}

func TestStoreEmptyPathUnavailable(t *testing.T) {
	s := New("")
	if _, err := s.List("u"); !errors.Is(err, searchtemplates.ErrUnavailable) {
		t.Fatalf("empty path list: %v", err)
	}
}

func TestStorePerUserLimit(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "t.json"))
	for i := 0; i < searchtemplates.MaxPerUser; i++ {
		if _, err := s.Create("u", "n", "q"); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if _, err := s.Create("u", "n", "q"); !errors.Is(err, searchtemplates.ErrLimitExceeded) {
		t.Fatalf("over limit: %v", err)
	}
}
