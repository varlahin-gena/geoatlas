package reputation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"network_monitor/internal/apperr"
	"network_monitor/internal/model"
	reppkg "network_monitor/internal/reputation"
)

var feedNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// Service — application use cases для репутационных списков.
type Service struct {
	store     RangeStore
	index     Index
	codec     Codec
	refresher FeedRefresher
	feedStore FeedStore

	mu      sync.Mutex
	lastErr map[string]string // list_name → last fetch error
}

func New(store RangeStore, index Index, codec Codec, refresher FeedRefresher, feedStore FeedStore) *Service {
	return &Service{
		store:     store,
		index:     index,
		codec:     codec,
		refresher: refresher,
		feedStore: feedStore,
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
		return UploadResult{}, apperr.InvalidCSV(err)
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
		return nil, apperr.InvalidInput("ip is required")
	}
	if parsed := net.ParseIP(ipStr); parsed == nil || parsed.To4() == nil {
		return nil, apperr.InvalidInput("invalid IPv4 address")
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
		return apperr.InvalidInput("list name is required")
	}
	// Если список был URL-фидом — снимаем его из расписания, иначе вернётся при refresh.
	if err := s.removeFeedConfig(name); err != nil {
		return err
	}
	return s.DeleteListData(ctx, name)
}

// DeleteListData удаляет диапазоны из CH и индекса (без правки файла фидов).
func (s *Service) DeleteListData(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return apperr.InvalidInput("list name is required")
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

func (s *Service) ListFeeds() ([]Feed, error) {
	if s.feedStore == nil {
		return nil, nil
	}
	feeds, ok, err := s.feedStore.Load()
	if err != nil {
		return nil, err
	}
	if !ok || feeds == nil {
		return []Feed{}, nil
	}
	return feeds, nil
}

// ListCatalog — кураторские пресеты для UI.
func (s *Service) ListCatalog() []Feed {
	return CatalogFeeds()
}

func (s *Service) AddFeed(ctx context.Context, feed Feed) error {
	_ = ctx
	feed, err := normalizeFeedInput(feed)
	if err != nil {
		return err
	}
	if s.feedStore == nil {
		return fmt.Errorf("feed store not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	feeds, _, err := s.feedStore.Load()
	if err != nil {
		return err
	}
	for _, f := range feeds {
		if f.Name == feed.Name {
			return apperr.Conflict(fmt.Sprintf("feed %q already exists", feed.Name))
		}
	}
	feeds = append(feeds, feed)
	if err := s.feedStore.Save(feeds); err != nil {
		return err
	}
	if s.refresher != nil {
		s.refresher.SetFeeds(feeds)
	}
	return nil
}

func (s *Service) RemoveFeed(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return apperr.InvalidInput("feed name is required")
	}
	if err := s.removeFeedConfig(name); err != nil {
		return err
	}
	return s.DeleteListData(ctx, name)
}

// removeFeedConfig убирает фид из JSON и scheduler (no-op если нет / не сконфигурирован).
func (s *Service) removeFeedConfig(name string) error {
	if s.feedStore == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	feeds, ok, err := s.feedStore.Load()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	next := make([]Feed, 0, len(feeds))
	found := false
	for _, f := range feeds {
		if f.Name == name {
			found = true
			continue
		}
		next = append(next, f)
	}
	if !found {
		return nil
	}
	if err := s.feedStore.Save(next); err != nil {
		return err
	}
	if s.refresher != nil {
		s.refresher.SetFeeds(next)
	}
	return nil
}

func normalizeFeedInput(feed Feed) (Feed, error) {
	feed.Name = strings.TrimSpace(feed.Name)
	feed.URL = strings.TrimSpace(feed.URL)
	feed.Category = strings.TrimSpace(feed.Category)
	feed.Format = strings.ToLower(strings.TrimSpace(feed.Format))
	if feed.Name == "" || !feedNameRe.MatchString(feed.Name) {
		return feed, apperr.InvalidInput("invalid feed name (use letters, digits, _-; max 64)")
	}
	if feed.URL == "" {
		return feed, apperr.InvalidInput("url is required")
	}
	u, err := url.Parse(feed.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return feed, apperr.InvalidInput("url must be http(s)")
	}
	if feed.Category == "" {
		feed.Category = "unknown"
	}
	if len(feed.Category) > 64 {
		return feed, apperr.InvalidInput("category too long")
	}
	if feed.Format == "" {
		feed.Format = "netset"
	}
	feed.Format = reppkg.NormalizeFeedFormat(feed.Format)
	if !reppkg.IsSupportedFeedFormat(feed.Format) {
		return feed, apperr.InvalidInput(fmt.Sprintf("unsupported format %q (netset|plain|spamhaus_json|csv_ip)", feed.Format))
	}
	return feed, nil
}

func (s *Service) Refresh(ctx context.Context, force bool) (RefreshResult, error) {
	if s.refresher == nil {
		return RefreshResult{}, fmt.Errorf("feed refresher not configured")
	}
	return s.refresher.RefreshAll(ctx, force)
}

// ApplyListRanges пишет один список в CH+индекс (для fetch job).
// Возвращает число диапазонов этого списка после normalize — не размер всей таблицы.
func (s *Service) ApplyListRanges(ctx context.Context, listName string, ranges []model.ReputationRange) (int, error) {
	if _, err := s.store.ReplaceList(ctx, listName, ranges); err != nil {
		return 0, err
	}
	if s.index != nil {
		s.index.ReplaceList(listName, ranges)
	}
	s.SetFeedError(listName, "")
	return len(reppkg.NormalizeRanges(ranges)), nil
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

func IsClientError(err error) bool {
	return apperr.IsClient(err)
}

func IsConflict(err error) bool {
	return errors.Is(err, apperr.ErrConflict)
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
