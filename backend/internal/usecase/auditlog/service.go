package auditlog

import (
	"context"
	"time"
)

type DREvent struct {
	Timestamp time.Time      `json:"timestamp"`
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Target    string         `json:"target"`
	Status    string         `json:"status"`
	Message   string         `json:"message"`
	Meta      map[string]any `json:"meta,omitempty"`
}

type AuditEvent struct {
	Timestamp    time.Time      `json:"timestamp"`
	Actor        string         `json:"actor"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Result       string         `json:"result"`
	IP           string         `json:"ip,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
}

type DRQuery struct {
	Since  time.Time
	Limit  int
	Action string
	Status string
	Actor  string
}

type AuditQuery struct {
	Since  time.Time
	Limit  int
	Action string
	Result string
	Actor  string
}

type Store interface {
	WriteDR(context.Context, DREvent) error
	WriteAudit(context.Context, AuditEvent) error
	ListDR(context.Context, DRQuery) ([]DREvent, error)
	ListAudit(context.Context, AuditQuery) ([]AuditEvent, error)
}

type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Enabled() bool { return s != nil && s.store != nil }

func (s *Service) WriteDR(ctx context.Context, e DREvent) error {
	if !s.Enabled() {
		return nil
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	return s.store.WriteDR(ctx, e)
}

func (s *Service) WriteAudit(ctx context.Context, e AuditEvent) error {
	if !s.Enabled() {
		return nil
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	return s.store.WriteAudit(ctx, e)
}

func (s *Service) ListDR(ctx context.Context, q DRQuery) ([]DREvent, error) {
	if !s.Enabled() {
		return []DREvent{}, nil
	}
	if q.Limit < 1 || q.Limit > 200 {
		q.Limit = 100
	}
	if q.Since.IsZero() {
		q.Since = time.Now().UTC().Add(-30 * 24 * time.Hour)
	}
	return s.store.ListDR(ctx, q)
}

func (s *Service) ListAudit(ctx context.Context, q AuditQuery) ([]AuditEvent, error) {
	if !s.Enabled() {
		return []AuditEvent{}, nil
	}
	if q.Limit < 1 || q.Limit > 200 {
		q.Limit = 100
	}
	if q.Since.IsZero() {
		q.Since = time.Now().UTC().Add(-30 * 24 * time.Hour)
	}
	return s.store.ListAudit(ctx, q)
}
