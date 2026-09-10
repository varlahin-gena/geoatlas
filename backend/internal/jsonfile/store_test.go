package jsonfile

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type sample struct {
	N int    `json:"n"`
	S string `json:"s"`
}

func TestStoreRoundtrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sample.json")
	miss := sample{N: 7, S: "miss"}
	s := New(path, "sample path empty", miss)

	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != miss {
		t.Fatalf("missing file: %+v", got)
	}

	want := sample{N: 3, S: "ok"}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestStoreEmptyPath(t *testing.T) {
	t.Parallel()
	miss := sample{N: 1}
	s := New("  ", "path is empty", miss)
	got, err := s.Load()
	if err != nil || got != miss {
		t.Fatalf("Load empty path: %+v %v", got, err)
	}
	if err := s.Save(sample{}); err == nil || err.Error() != "path is empty" {
		t.Fatalf("Save empty path: %v", err)
	}
}

func TestStoreHooks(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "hook.json")
	s := New(path, "empty", sample{},
		WithSaveHook(func(v sample) (sample, error) {
			v.S = "saved"
			return v, nil
		}),
		WithLoadHook(func(raw sample) (sample, error) {
			raw.S = raw.S + "-loaded"
			return raw, nil
		}),
	)
	if err := s.Save(sample{N: 2, S: "x"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.N != 2 || got.S != "saved-loaded" {
		t.Fatalf("hooks: %+v", got)
	}
}

func TestStoreConcurrent(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "conc.json")
	s := New(path, "empty", sample{N: -1})
	if err := s.Save(sample{N: 0}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			_ = s.Save(sample{N: n})
		}(i)
		go func() {
			defer wg.Done()
			_, _ = s.Load()
		}()
	}
	wg.Wait()
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.N < 0 || got.N > 31 {
		t.Fatalf("unexpected final value: %+v", got)
	}
}
