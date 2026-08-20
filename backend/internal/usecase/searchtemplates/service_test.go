package searchtemplates

import (
	"errors"
	"strings"
	"testing"
)

type memStore struct {
	users map[string][]Template
}

func newMemStore() *memStore {
	return &memStore{users: map[string][]Template{}}
}

func (m *memStore) List(username string) ([]Template, error) {
	return append([]Template(nil), m.users[username]...), nil
}

func (m *memStore) ListAll() ([]TemplateWithAuthor, error) {
	out := make([]TemplateWithAuthor, 0)
	for user, items := range m.users {
		for _, t := range items {
			out = append(out, TemplateWithAuthor{Template: t, Username: user})
		}
	}
	return out, nil
}

func (m *memStore) Create(username, name, query string) (Template, error) {
	t := Template{ID: "id-1", Name: name, Query: query, UpdatedAt: "now"}
	m.users[username] = append(m.users[username], t)
	return t, nil
}

func (m *memStore) Update(username, id, name, query string) (Template, error) {
	list := m.users[username]
	for i := range list {
		if list[i].ID != id {
			continue
		}
		list[i].Name = name
		list[i].Query = query
		m.users[username] = list
		return list[i], nil
	}
	return Template{}, ErrNotFound
}

func (m *memStore) Delete(username, id string) error {
	list := m.users[username]
	next := make([]Template, 0, len(list))
	found := false
	for _, t := range list {
		if t.ID == id {
			found = true
			continue
		}
		next = append(next, t)
	}
	if !found {
		return ErrNotFound
	}
	m.users[username] = next
	return nil
}

func TestValidateNameQuery(t *testing.T) {
	if err := ValidateNameQuery("", "q"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("empty name: %v", err)
	}
	if err := ValidateNameQuery("n", ""); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("empty query: %v", err)
	}
	if err := ValidateNameQuery(strings.Repeat("я", MaxNameLen+1), "q"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("long name: %v", err)
	}
	if err := ValidateNameQuery("n", strings.Repeat("q", MaxQueryLen+1)); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("long query: %v", err)
	}
	if err := ValidateNameQuery("ok", "country:RU"); err != nil {
		t.Fatalf("valid: %v", err)
	}
}

func TestServiceNilUnavailable(t *testing.T) {
	var s *Service
	if _, err := s.List("u"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil service list: %v", err)
	}
	s = New(nil)
	if _, err := s.Create("u", "n", "q"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil store create: %v", err)
	}
}

func TestServiceValidation(t *testing.T) {
	s := New(newMemStore())
	if _, err := s.Create("u", "", "q"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("empty name: %v", err)
	}
	if _, err := s.Create("u", "n", ""); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("empty query: %v", err)
	}
	if _, err := s.Create("  ", "n", "q"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("empty username: %v", err)
	}
	if _, err := s.Update("u", "id", "", "q"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("update empty name: %v", err)
	}
	if err := s.Delete("u", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete empty id: %v", err)
	}
}

func TestServiceCRUD(t *testing.T) {
	s := New(newMemStore())
	created, err := s.Create("admin", "RU", "country:RU")
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.List("admin")
	if err != nil || len(list) != 1 || list[0].Name != "RU" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	updated, err := s.Update("admin", created.ID, "RU2", "country:US")
	if err != nil || updated.Name != "RU2" {
		t.Fatalf("update=%+v err=%v", updated, err)
	}
	if err := s.Delete("admin", created.ID); err != nil {
		t.Fatal(err)
	}
	list, err = s.List("admin")
	if err != nil || len(list) != 0 {
		t.Fatalf("after delete list=%+v err=%v", list, err)
	}
}
