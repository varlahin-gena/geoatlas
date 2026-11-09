package connectors

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Service — CRUD + Apply for syslog connectors.
type Service struct {
	store  Store
	conf   ConfWriter
	reload Reloader
	ports  PortRange
	now    func() time.Time
}

func New(store Store, conf ConfWriter, reload Reloader, ports PortRange) *Service {
	return &Service{
		store:  store,
		conf:   conf,
		reload: reload,
		ports:  ports,
		now:    time.Now,
	}
}

// PortRange exposes published extra-port window for UI.
func (s *Service) PortRange() PortRange {
	if s == nil {
		return PortRange{}
	}
	return s.ports
}

func (s *Service) MaxConnectors() int {
	if s == nil {
		return 1
	}
	return s.ports.MaxConnectors()
}

// EnsureSeed loads document or writes default connector.
func (s *Service) EnsureSeed() (Document, error) {
	doc, err := s.store.Load()
	if err != nil {
		return Document{}, err
	}
	if len(doc.Connectors) == 0 {
		doc = Document{Connectors: []Connector{DefaultConnector()}, UpdatedAt: s.now().UTC().Format(time.RFC3339)}
		if err := s.store.Save(doc); err != nil {
			return doc, err
		}
		return doc, nil
	}
	doc = s.normalizeDoc(doc)
	return doc, nil
}

func (s *Service) List() (Document, error) {
	doc, err := s.EnsureSeed()
	if err != nil {
		return Document{}, err
	}
	return doc, nil
}

func (s *Service) Get(id string) (Connector, error) {
	doc, err := s.EnsureSeed()
	if err != nil {
		return Connector{}, err
	}
	id = strings.ToLower(strings.TrimSpace(id))
	for _, c := range doc.Connectors {
		if c.ID == id {
			return c, nil
		}
	}
	return Connector{}, ErrNotFound
}

type CreateInput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Port      int    `json:"port"`
	Protocols string `json:"protocols"`
	Enabled   *bool  `json:"enabled"`
}

func (s *Service) Create(in CreateInput) (Connector, error) {
	doc, err := s.EnsureSeed()
	if err != nil {
		return Connector{}, err
	}
	if len(doc.Connectors) >= s.MaxConnectors() {
		return Connector{}, ErrLimitReached
	}
	id, err := normalizeID(in.ID)
	if err != nil {
		return Connector{}, err
	}
	if id == DefaultID {
		return Connector{}, fmt.Errorf("%w: id %q is reserved", ErrInvalid, DefaultID)
	}
	name, err := normalizeName(in.Name)
	if err != nil {
		return Connector{}, err
	}
	proto, err := normalizeProtocols(in.Protocols)
	if err != nil {
		return Connector{}, err
	}
	if err := validatePort(in.Port, s.ports, id); err != nil {
		return Connector{}, err
	}
	for _, c := range doc.Connectors {
		if c.ID == id {
			return Connector{}, ErrIDTaken
		}
		if c.Port == in.Port {
			return Connector{}, fmt.Errorf("%w: %d used by %s", ErrPortTaken, in.Port, c.ID)
		}
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	c := Connector{ID: id, Name: name, Port: in.Port, Protocols: proto, Enabled: enabled}
	doc.Connectors = append(doc.Connectors, c)
	doc = s.normalizeDoc(doc)
	doc.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	if err := s.store.Save(doc); err != nil {
		return Connector{}, err
	}
	return c, nil
}

type UpdateInput struct {
	Name      *string `json:"name"`
	Port      *int    `json:"port"`
	Protocols *string `json:"protocols"`
	Enabled   *bool   `json:"enabled"`
}

func (s *Service) Update(id string, in UpdateInput) (Connector, error) {
	doc, err := s.EnsureSeed()
	if err != nil {
		return Connector{}, err
	}
	id = strings.ToLower(strings.TrimSpace(id))
	idx := -1
	for i, c := range doc.Connectors {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Connector{}, ErrNotFound
	}
	c := doc.Connectors[idx]
	if in.Name != nil {
		name, err := normalizeName(*in.Name)
		if err != nil {
			return Connector{}, err
		}
		c.Name = name
	}
	if in.Protocols != nil {
		proto, err := normalizeProtocols(*in.Protocols)
		if err != nil {
			return Connector{}, err
		}
		c.Protocols = proto
	}
	if in.Enabled != nil {
		c.Enabled = *in.Enabled
	}
	if in.Port != nil {
		if c.Builtin && *in.Port != DefaultPort {
			return Connector{}, fmt.Errorf("%w: builtin connector must use port 514", ErrInvalid)
		}
		if err := validatePort(*in.Port, s.ports, c.ID); err != nil {
			return Connector{}, err
		}
		for _, o := range doc.Connectors {
			if o.ID != c.ID && o.Port == *in.Port {
				return Connector{}, fmt.Errorf("%w: %d used by %s", ErrPortTaken, *in.Port, o.ID)
			}
		}
		c.Port = *in.Port
	}
	doc.Connectors[idx] = c
	doc = s.normalizeDoc(doc)
	doc.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	if err := s.store.Save(doc); err != nil {
		return Connector{}, err
	}
	return c, nil
}

func (s *Service) Delete(id string) error {
	doc, err := s.EnsureSeed()
	if err != nil {
		return err
	}
	id = strings.ToLower(strings.TrimSpace(id))
	out := make([]Connector, 0, len(doc.Connectors))
	found := false
	for _, c := range doc.Connectors {
		if c.ID != id {
			out = append(out, c)
			continue
		}
		found = true
		if c.Builtin || c.ID == DefaultID {
			return ErrBuiltinDelete
		}
	}
	if !found {
		return ErrNotFound
	}
	doc.Connectors = out
	doc = s.normalizeDoc(doc)
	doc.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	return s.store.Save(doc)
}

// Apply writes syslog-ng conf and reloads the agent.
func (s *Service) Apply(ctx context.Context) (ApplyResult, error) {
	doc, err := s.EnsureSeed()
	if err != nil {
		return ApplyResult{}, err
	}
	res := ApplyResult{ConnectorN: len(doc.Connectors)}
	for _, c := range doc.Connectors {
		if c.Enabled {
			res.EnabledCount++
		}
	}
	if s.conf == nil {
		return res, fmt.Errorf("conf writer not configured")
	}
	path, err := s.conf.Write(doc)
	if err != nil {
		return res, err
	}
	res.ConfPath = path
	if s.reload == nil {
		res.OK = true
		res.ReloadError = "reloader not configured"
		return res, nil
	}
	if err := s.reload.Reload(ctx); err != nil {
		res.ReloadError = err.Error()
		// Conf already written; surface reload failure in ReloadError (HTTP 200 + payload).
		return res, nil //nolint:nilerr // intentional: soft-fail reload
	}
	res.Reloaded = true
	res.OK = true
	return res, nil
}

func (s *Service) normalizeDoc(doc Document) Document {
	hasDefault := false
	out := make([]Connector, 0, len(doc.Connectors)+1)
	seen := map[string]struct{}{}
	for _, c := range doc.Connectors {
		id, err := normalizeID(c.ID)
		if err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		name := strings.TrimSpace(c.Name)
		if name == "" {
			name = id
		}
		proto, err := normalizeProtocols(c.Protocols)
		if err != nil {
			proto = ProtoBoth
		}
		if id == DefaultID {
			c.Builtin = true
			c.Port = DefaultPort
			hasDefault = true
		}
		c.ID = id
		c.Name = name
		c.Protocols = proto
		out = append(out, c)
	}
	if !hasDefault {
		out = append([]Connector{DefaultConnector()}, out...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Builtin != out[j].Builtin {
			return out[i].Builtin
		}
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].ID < out[j].ID
	})
	doc.Connectors = out
	return doc
}
