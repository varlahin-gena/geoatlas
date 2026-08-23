package parseerrors

import (
	"context"
	"errors"
	"strings"

	"geoatlas/internal/apperr"
	"geoatlas/internal/model"
)

// Repository — журнал ошибок парсинга.
type Repository interface {
	List(ctx context.Context, limit int, search string) ([]model.ParseErrorRow, error)
	Delete(ctx context.Context, ids []string) error
	DeleteAll(ctx context.Context) error
}

// Service — application use cases для /api/parse-errors.
type Service struct {
	repo Repository
}

func New(repo Repository) *Service {
	return &Service{repo: repo}
}

type ListInput struct {
	Limit  int
	Search string
}

type ListResult struct {
	Items []model.ParseErrorRow
}

func (s *Service) List(ctx context.Context, in ListInput) (ListResult, error) {
	limit := in.Limit
	if limit < 1 {
		limit = 1
	}
	if limit > 5000 {
		limit = 5000
	}
	items, err := s.repo.List(ctx, limit, strings.TrimSpace(in.Search))
	if err != nil {
		return ListResult{}, err
	}
	if items == nil {
		items = []model.ParseErrorRow{}
	}
	return ListResult{Items: items}, nil
}

type DeleteInput struct {
	IDs []string
	All bool
}

const maxDeleteIDs = 1000

var (
	ErrNoIDs      = apperr.InvalidInput("no ids provided")
	ErrTooManyIDs = apperr.InvalidInput("too many ids provided")
)

func (s *Service) Delete(ctx context.Context, in DeleteInput) error {
	if in.All {
		return s.repo.DeleteAll(ctx)
	}
	if len(in.IDs) == 0 {
		return ErrNoIDs
	}
	if len(in.IDs) > maxDeleteIDs {
		return ErrTooManyIDs
	}
	return s.repo.Delete(ctx, in.IDs)
}

func IsClientError(err error) bool {
	return errors.Is(err, ErrNoIDs) || apperr.IsClient(err)
}
