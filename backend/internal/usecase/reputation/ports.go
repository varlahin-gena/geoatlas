package reputation

import (
	"context"
	"io"

	"network_monitor/internal/model"
)

// RangeStore — персистентность reputation_ranges.
type RangeStore interface {
	Load(ctx context.Context) ([]model.ReputationRange, error)
	ReplaceAll(ctx context.Context, ranges []model.ReputationRange) (int, error)
	// ReplaceList заменяет один список; int — число диапазонов этого списка после записи.
	ReplaceList(ctx context.Context, listName string, ranges []model.ReputationRange) (int, error)
	DeleteList(ctx context.Context, listName string) error
	ListMeta(ctx context.Context) ([]model.ReputationListMeta, error)
}

// Index — in-memory multi-hit индекс.
type Index interface {
	Lookup(ipStr string) []model.ReputationHit
	RangeCount() int
	ReplaceAll(ranges []model.ReputationRange)
	ReplaceList(listName string, ranges []model.ReputationRange)
	DeleteList(listName string)
	ListMeta() []model.ReputationListMeta
	Snapshot() []model.ReputationRange
}

// FeedRefresher — фоновый/ручной fetch URL-фидов.
type FeedRefresher interface {
	RefreshAll(ctx context.Context, force bool) (RefreshResult, error)
	SetFeeds(feeds []Feed)
}

// FeedStore — персистентный список URL-фидов (JSON-файл).
type FeedStore interface {
	Load() (feeds []Feed, ok bool, err error)
	Save(feeds []Feed) error
}

// Codec — CSV / netset парсинг.
type Codec interface {
	ReadCSV(r io.Reader) ([]model.ReputationRange, error)
	ParseNetset(r io.Reader, listName, category, source string) ([]model.ReputationRange, error)
}

// RefreshResult — итог обновления фидов.
type RefreshResult struct {
	Updated []string          `json:"updated"`
	Skipped []string          `json:"skipped"`
	Failed  []string          `json:"failed"`
	Errors  map[string]string `json:"errors,omitempty"`
	Counts  map[string]int    `json:"counts,omitempty"` // число диапазонов по успешно обновлённым фидам
}
