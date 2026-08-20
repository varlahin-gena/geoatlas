package searchtemplates

import "strings"

// Service — CRUD личных шаблонов поиска.
type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(username string) ([]Template, error) {
	if s == nil || s.store == nil {
		return nil, ErrUnavailable
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, ErrInvalidName
	}
	return s.store.List(username)
}

func (s *Service) ListAll() ([]TemplateWithAuthor, error) {
	if s == nil || s.store == nil {
		return nil, ErrUnavailable
	}
	return s.store.ListAll()
}

func (s *Service) Create(username, name, query string) (Template, error) {
	if s == nil || s.store == nil {
		return Template{}, ErrUnavailable
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return Template{}, ErrInvalidName
	}
	name, query = strings.TrimSpace(name), strings.TrimSpace(query)
	if err := ValidateNameQuery(name, query); err != nil {
		return Template{}, err
	}
	return s.store.Create(username, name, query)
}

func (s *Service) Update(username, id, name, query string) (Template, error) {
	if s == nil || s.store == nil {
		return Template{}, ErrUnavailable
	}
	username = strings.TrimSpace(username)
	id = strings.TrimSpace(id)
	if username == "" || id == "" {
		return Template{}, ErrNotFound
	}
	name, query = strings.TrimSpace(name), strings.TrimSpace(query)
	if err := ValidateNameQuery(name, query); err != nil {
		return Template{}, err
	}
	return s.store.Update(username, id, name, query)
}

func (s *Service) Delete(username, id string) error {
	if s == nil || s.store == nil {
		return ErrUnavailable
	}
	username = strings.TrimSpace(username)
	id = strings.TrimSpace(id)
	if username == "" || id == "" {
		return ErrNotFound
	}
	return s.store.Delete(username, id)
}
