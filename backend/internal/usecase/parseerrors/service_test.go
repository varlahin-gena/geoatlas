package parseerrors

import (
	"context"
	"errors"
	"testing"

	"network_monitor/internal/model"
)

type stubRepo struct {
	items     []model.ParseErrorRow
	listErr   error
	lastLimit int
	deleted   []string
	all       bool
}

func (s *stubRepo) List(ctx context.Context, limit int, search string) ([]model.ParseErrorRow, error) {
	s.lastLimit = limit
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.items, nil
}

func (s *stubRepo) Delete(ctx context.Context, ids []string) error {
	s.deleted = append([]string(nil), ids...)
	return nil
}

func (s *stubRepo) DeleteAll(ctx context.Context) error {
	s.all = true
	return nil
}

func TestListClampsLimit(t *testing.T) {
	repo := &stubRepo{items: []model.ParseErrorRow{{ID: "1"}}}
	uc := New(repo)

	if _, err := uc.List(context.Background(), ListInput{Limit: 0}); err != nil {
		t.Fatal(err)
	}
	if repo.lastLimit != 1 {
		t.Fatalf("limit=0 → %d, want 1", repo.lastLimit)
	}

	if _, err := uc.List(context.Background(), ListInput{Limit: 10000}); err != nil {
		t.Fatal(err)
	}
	if repo.lastLimit != 5000 {
		t.Fatalf("limit=10000 → %d, want 5000", repo.lastLimit)
	}
}

func TestDeleteRequiresIDs(t *testing.T) {
	uc := New(&stubRepo{})
	err := uc.Delete(context.Background(), DeleteInput{})
	if !IsClientError(err) || !errors.Is(err, ErrNoIDs) {
		t.Fatalf("err=%v", err)
	}
}

func TestDeleteRejectsTooManyIDs(t *testing.T) {
	repo := &stubRepo{}
	uc := New(repo)
	ids := make([]string, maxDeleteIDs+1)
	for i := range ids {
		ids[i] = "id"
	}
	err := uc.Delete(context.Background(), DeleteInput{IDs: ids})
	if !IsClientError(err) || !errors.Is(err, ErrTooManyIDs) {
		t.Fatalf("err=%v", err)
	}
	if len(repo.deleted) != 0 {
		t.Fatalf("unexpected delete call: %d ids", len(repo.deleted))
	}
}

func TestDeleteAll(t *testing.T) {
	repo := &stubRepo{}
	uc := New(repo)
	if err := uc.Delete(context.Background(), DeleteInput{All: true}); err != nil {
		t.Fatal(err)
	}
	if !repo.all {
		t.Fatal("expected DeleteAll")
	}
}
