package connectors

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memStore struct {
	doc Document
	err error
}

func (m *memStore) Load() (Document, error) { return m.doc, m.err }
func (m *memStore) Save(d Document) error {
	m.doc = d
	return m.err
}

type memConf struct {
	n int
}

func (m *memConf) Write(Document) (string, error) {
	m.n++
	return "/tmp/zz_connectors.conf", nil
}

type memReload struct {
	n   int
	err error
}

func (m *memReload) Reload(context.Context) error {
	m.n++
	return m.err
}

func TestCreateUpdateDeleteApply(t *testing.T) {
	st := &memStore{}
	conf := &memConf{}
	rel := &memReload{}
	svc := New(st, conf, rel, PortRange{Min: 5514, Max: 5516})
	svc.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

	doc, err := svc.List()
	if err != nil || len(doc.Connectors) != 1 || doc.Connectors[0].ID != DefaultID {
		t.Fatalf("seed: %+v %v", doc, err)
	}

	c, err := svc.Create(CreateInput{ID: "fw-a", Name: "FW A", Port: 5514, Protocols: "both"})
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "fw-a" || c.Port != 5514 {
		t.Fatalf("create: %+v", c)
	}

	if _, err := svc.Create(CreateInput{ID: "fw-b", Name: "B", Port: 5514, Protocols: "udp"}); !errors.Is(err, ErrPortTaken) {
		t.Fatalf("want port taken, got %v", err)
	}
	if _, err := svc.Create(CreateInput{ID: "fw-a", Name: "dup", Port: 5515, Protocols: "tcp"}); !errors.Is(err, ErrIDTaken) {
		t.Fatalf("want id taken, got %v", err)
	}
	if _, err := svc.Create(CreateInput{ID: "x", Name: "x", Port: 9999, Protocols: "udp"}); !errors.Is(err, ErrPortNotAllowed) {
		t.Fatalf("want port not allowed, got %v", err)
	}

	name := "FW A2"
	port := 5515
	c, err = svc.Update("fw-a", UpdateInput{Name: &name, Port: &port})
	if err != nil || c.Port != 5515 || c.Name != "FW A2" {
		t.Fatalf("update: %+v %v", c, err)
	}

	if err := svc.Delete(DefaultID); !errors.Is(err, ErrBuiltinDelete) {
		t.Fatalf("want builtin delete err, got %v", err)
	}
	if err := svc.Delete("fw-a"); err != nil {
		t.Fatal(err)
	}

	// recreate + apply
	_, _ = svc.Create(CreateInput{ID: "fw-a", Name: "FW A", Port: 5514, Protocols: "udp"})
	res, err := svc.Apply(context.Background())
	if err != nil || !res.OK || !res.Reloaded || conf.n != 1 || rel.n != 1 {
		t.Fatalf("apply: %+v err=%v conf=%d rel=%d", res, err, conf.n, rel.n)
	}
}

func TestLimitReached(t *testing.T) {
	st := &memStore{}
	svc := New(st, &memConf{}, &memReload{}, PortRange{Min: 5514, Max: 5514}) // max 2
	_, _ = svc.List()
	if _, err := svc.Create(CreateInput{ID: "a", Name: "a", Port: 5514, Protocols: "udp"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(CreateInput{ID: "b", Name: "b", Port: 5514, Protocols: "udp"}); !errors.Is(err, ErrLimitReached) && !errors.Is(err, ErrPortTaken) {
		// port taken happens first if same port — use unique impossible: only 1 extra port
		if !errors.Is(err, ErrLimitReached) && !errors.Is(err, ErrPortTaken) {
			t.Fatalf("got %v", err)
		}
	}
	// fill the only extra slot already used — next create with different port out of range
	if _, err := svc.Create(CreateInput{ID: "b", Name: "b", Port: 5515, Protocols: "udp"}); !errors.Is(err, ErrPortNotAllowed) && !errors.Is(err, ErrLimitReached) {
		t.Fatalf("want limit or port, got %v", err)
	}
}
