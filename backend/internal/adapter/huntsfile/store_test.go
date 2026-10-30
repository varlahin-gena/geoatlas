package huntsfile

import (
	"errors"
	"path/filepath"
	"testing"

	"geoatlas/internal/usecase/hunts"
)

func TestStoreCRUD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hunts.json")
	s := New(path)

	h := hunts.Hunt{
		Name: "one",
		Map:  hunts.MapState{Period: "1d", GroupBy: "city", Filter: "all"},
	}
	created, err := s.Create("alice", h)
	if err != nil || created.ID == "" {
		t.Fatalf("create: %+v %v", created, err)
	}
	list, err := s.List("alice")
	if err != nil || len(list) != 1 || list[0].Name != "one" {
		t.Fatalf("list: %+v %v", list, err)
	}
	created.Name = "two"
	upd, err := s.Update("alice", created.ID, created)
	if err != nil || upd.Name != "two" {
		t.Fatalf("update: %+v %v", upd, err)
	}
	upd.Runs = []hunts.RunResult{{EdgeCount: 3, At: "t"}}
	upd.LastRunAt = "t"
	if err := s.SaveRun("alice", created.ID, upd); err != nil {
		t.Fatalf("saverun: %v", err)
	}
	all, err := s.ListAll()
	if err != nil || len(all) != 1 || all[0].Username != "alice" {
		t.Fatalf("listall: %+v %v", all, err)
	}
	if err := s.Delete("alice", created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.Delete("alice", created.ID); !errors.Is(err, hunts.ErrNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestStoreValidationAndLimits(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "h.json"))
	if _, err := s.Create("", hunts.Hunt{Name: "x"}); !errors.Is(err, hunts.ErrInvalidName) {
		t.Fatalf("empty user: %v", err)
	}
	if _, err := s.List(""); !errors.Is(err, hunts.ErrInvalidName) {
		t.Fatalf("list empty user: %v", err)
	}
	empty := New("")
	if _, err := empty.Create("u", hunts.Hunt{Name: "x"}); err == nil {
		t.Fatal("empty path create should fail on save")
	}
}

func TestStorePerUserLimit(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "h.json"))
	for i := 0; i < hunts.MaxPerUser; i++ {
		if _, err := s.Create("u", hunts.Hunt{Name: "n", Map: hunts.MapState{Period: "1d"}}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	if _, err := s.Create("u", hunts.Hunt{Name: "n", Map: hunts.MapState{Period: "1d"}}); !errors.Is(err, hunts.ErrLimitExceeded) {
		t.Fatalf("limit: %v", err)
	}
}
