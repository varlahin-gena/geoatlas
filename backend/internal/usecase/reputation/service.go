package reputation

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"network_monitor/internal/model"
	reppkg "network_monitor/internal/reputation"
)

// Service — application use cases для репутационных списков.
type Service struct {
	store     RangeStore
	index     Index
	codec     Codec
	refresher FeedRefresher

	mu      sync.Mutex
	lastErr map[string]string // list_name → last fetch error
}

func New(store RangeStore, index Index, codec Codec, refresher FeedRefresher) *Service {
	return &Service{
		store:     store,
		index:     index,
		codec:     codec,
		refresher: refresher,
		lastErr:   map[string]string{},
	}
}

// SetRefresher подключает fetch job после создания Service (избегает цикличной инициализации).
func (s *Service) SetRefresher(r FeedRefresher) {
	if s != nil {
		s.refresher = r
	}
}

// SetFeedError сохраняет last_error для UI (вызывается из job).
func (s *Service) SetFeedError(listName, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if errMsg == "" {
		delete(s.lastErr, listName)
	} else {
		s.lastErr[listName] = errMsg
	}
}

type UploadResult struct {
	DryRun bool                    `json:"dry_run"`
	Count  int                     `json:"count"`
	Lists  []string                `json:"lists,omitempty"`
	Sample []model.ReputationRange `json:"sample,omitempty"`
}

func (s *Service) UploadCSV(ctx context.Context, r io.Reader, dryRun bool) (UploadResult, error) {
	ranges, err := s.codec.ReadCSV(r)
	if err != nil {
		return UploadResult{}, err
	}
	lists := uniqueLists(ranges)
	if dryRun {
		sample := ranges
		if len(sample) > 5 {
			sample = sample[:5]
		}
		return UploadResult{DryRun: true, Count: len(ranges), Lists: lists, Sample: sample}, nil
	}
	// Группируем по list_name и заменяем каждый список.
	byList := map[string][]model.ReputationRange{}
	for _, rr := range ranges {
		byList[rr.ListName] = append(byList[rr.ListName], rr)
	}
	total := 0
	for name, group := range byList {
		n, err := s.store.ReplaceList(ctx, name, group)
		if err != nil {
			return UploadResult{}, err
		}
		total += len(group)
		_ = n
		if s.index != nil {
			s.index.ReplaceList(name, group)
		}
	}
	return UploadResult{Count: total, Lists: lists}, nil
}

func (s *Service) Lookup(ipStr string) ([]model.ReputationHit, error) {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" {
		return nil, clientErr{fmt.Errorf("ip is required")}
	}
	if parsed := net.ParseIP(ipStr); parsed == nil || parsed.To4() == nil {
		return nil, clientErr{fmt.Errorf("invalid IPv4 address")}
	}
	if s.index == nil {
		return nil, nil
	}
	return s.index.Lookup(ipStr), nil
}

func (s *Service) ListLists(ctx context.Context) ([]model.ReputationListMeta, error) {
	var meta []model.ReputationListMeta
	if s.index != nil && s.index.RangeCount() > 0 {
		meta = s.index.ListMeta()
	} else if s.store != nil {
		var err error
		meta, err = s.store.ListMeta(ctx)
		if err != nil {
			return nil, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range meta {
		if e, ok := s.lastErr[meta[i].Name]; ok {
			meta[i].LastError = e
		}
	}
	return meta, nil
}

func (s *Service) DeleteList(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return clientErr{fmt.Errorf("list name is required")}
	}
	if err := s.store.DeleteList(ctx, name); err != nil {
		return err
	}
	if s.index != nil {
		s.index.DeleteList(name)
	}
	s.SetFeedError(name, "")
	return nil
}

func (s *Service) Refresh(ctx context.Context, force bool) (RefreshResult, error) {
	if s.refresher == nil {
		return RefreshResult{}, fmt.Errorf("feed refresher not configured")
	}
	return s.refresher.RefreshAll(ctx, force)
}

// ApplyListRanges пишет один список в CH+индекс (для fetch job).
func (s *Service) ApplyListRanges(ctx context.Context, listName string, ranges []model.ReputationRange) (int, error) {
	n, err := s.store.ReplaceList(ctx, listName, ranges)
	if err != nil {
		return 0, err
	}
	if s.index != nil {
		s.index.ReplaceList(listName, ranges)
	}
	s.SetFeedError(listName, "")
	return n, nil
}

func (s *Service) Reload(ctx context.Context) error {
	ranges, err := s.store.Load(ctx)
	if err != nil {
		return err
	}
	if s.index != nil {
		s.index.ReplaceAll(ranges)
	}
	return nil
}

type clientErr struct{ error }

func IsClientError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(clientErr); ok {
		return true
	}
	return reppkg.IsClientCSVError(err)
}

func uniqueLists(ranges []model.ReputationRange) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, r := range ranges {
		if _, ok := seen[r.ListName]; ok {
			continue
		}
		seen[r.ListName] = struct{}{}
		out = append(out, r.ListName)
	}
	return out
}

// DefaultCodec — обёртка над package reputation.
type DefaultCodec struct{}

func (DefaultCodec) ReadCSV(r io.Reader) ([]model.ReputationRange, error) {
	return reppkg.ReadCSV(r)
}

func (DefaultCodec) ParseNetset(r io.Reader, listName, category, source string) ([]model.ReputationRange, error) {
	return reppkg.ParseNetset(r, listName, category, source, time.Now().UTC())
}
