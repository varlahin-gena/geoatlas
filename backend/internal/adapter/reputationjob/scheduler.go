package reputationjob

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"network_monitor/internal/adapter/heavytask"
	"network_monitor/internal/model"
	reppkg "network_monitor/internal/reputation"
	"network_monitor/internal/safeurl"
	usecasereputation "network_monitor/internal/usecase/reputation"
)

// Applier — запись списка после успешного download.
type Applier interface {
	ApplyListRanges(ctx context.Context, listName string, ranges []model.ReputationRange) (int, error)
	SetFeedError(listName, errMsg string)
	ListLists(ctx context.Context) ([]model.ReputationListMeta, error)
	// DeleteListData — только CH+индекс (без правки reputation_feeds.json).
	DeleteListData(ctx context.Context, name string) error
}

// Scheduler периодически качает REPUTATION_FEEDS.
type Scheduler struct {
	feeds    []usecasereputation.Feed
	interval time.Duration
	enabled  bool
	applier  Applier
	client   *http.Client
	heavy    *heavytask.Limiter

	mu      sync.Mutex
	etag    map[string]string
	lastMod map[string]string
	cancel  context.CancelFunc
	done    chan struct{}
}

// SetLimiter — общий слот тяжёлых задач.
func (s *Scheduler) SetLimiter(l *heavytask.Limiter) {
	if s == nil {
		return
	}
	s.heavy = l
}

func New(feeds []usecasereputation.Feed, interval time.Duration, enabled bool, applier Applier) *Scheduler {
	if interval < time.Minute {
		interval = time.Minute
	}
	return &Scheduler{
		feeds:    feeds,
		interval: interval,
		enabled:  enabled,
		applier:  applier,
		client: safeurl.SecureHTTPClient(&http.Client{
			Timeout: 120 * time.Second,
		}),
		etag:    map[string]string{},
		lastMod: map[string]string{},
		done:    make(chan struct{}),
	}
}

// Start запускает фоновый цикл (неблокирующий).
func (s *Scheduler) Start(parent context.Context) {
	if s == nil {
		return
	}
	if !s.enabled || s.applier == nil {
		select {
		case <-s.done:
		default:
			close(s.done)
		}
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	go func() {
		defer close(s.done)
		// Первый fetch сразу после старта.
		s.runOnce(ctx, false)
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.runOnce(ctx, false)
			}
		}
	}()
}

func (s *Scheduler) Shutdown(ctx context.Context) {
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	select {
	case <-s.done:
	case <-ctx.Done():
	}
}

func (s *Scheduler) RefreshAll(ctx context.Context, force bool) (usecasereputation.RefreshResult, error) {
	return s.runOnce(ctx, force), nil
}

// SetFeeds подменяет набор URL-фидов (из UI / JSON-файла).
func (s *Scheduler) SetFeeds(feeds []usecasereputation.Feed) {
	if s == nil {
		return
	}
	cp := make([]usecasereputation.Feed, len(feeds))
	copy(cp, feeds)
	s.mu.Lock()
	s.feeds = cp
	s.mu.Unlock()
}

func (s *Scheduler) snapshotFeeds() []usecasereputation.Feed {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]usecasereputation.Feed, len(s.feeds))
	copy(out, s.feeds)
	return out
}

func (s *Scheduler) runOnce(ctx context.Context, force bool) usecasereputation.RefreshResult {
	res := usecasereputation.RefreshResult{
		Errors: map[string]string{},
		Counts: map[string]int{},
	}
	if err := s.heavy.Acquire(ctx); err != nil {
		return res
	}
	defer s.heavy.Release()

	feeds := s.snapshotFeeds()
	for _, feed := range feeds {
		if ctx.Err() != nil {
			break
		}
		status, n, err := s.fetchOne(ctx, feed, force)
		switch status {
		case "updated":
			res.Updated = append(res.Updated, feed.Name)
			res.Counts[feed.Name] = n
		case "skipped":
			res.Skipped = append(res.Skipped, feed.Name)
		default:
			res.Failed = append(res.Failed, feed.Name)
			if err != nil {
				res.Errors[feed.Name] = err.Error()
				if s.applier != nil {
					s.applier.SetFeedError(feed.Name, err.Error())
				}
			}
		}
	}
	s.pruneObsoleteURLLists(ctx, feeds, &res)
	return res
}

// pruneObsoleteURLLists удаляет URL-списки, которых больше нет в конфиге
// (например устаревший агрегат firehol_level1). CSV upload (source=upload) не трогает.
func (s *Scheduler) pruneObsoleteURLLists(ctx context.Context, feeds []usecasereputation.Feed, res *usecasereputation.RefreshResult) {
	if s == nil || s.applier == nil || res == nil {
		return
	}
	keep := make(map[string]struct{}, len(feeds))
	for _, f := range feeds {
		keep[f.Name] = struct{}{}
	}
	lists, err := s.applier.ListLists(ctx)
	if err != nil {
		slog.Warn("reputation prune: list meta failed", "err", err)
		return
	}
	for _, m := range lists {
		if m.Source != "url" {
			continue
		}
		if _, ok := keep[m.Name]; ok {
			continue
		}
		if err := s.applier.DeleteListData(ctx, m.Name); err != nil {
			slog.Warn("reputation prune: delete failed", "list", m.Name, "err", err)
			continue
		}
		slog.Info("reputation obsolete URL list removed", "list", m.Name)
		res.Updated = append(res.Updated, "removed:"+m.Name)
	}
}

func (s *Scheduler) fetchOne(ctx context.Context, feed usecasereputation.Feed, force bool) (string, int, error) {
	if err := safeurl.ValidateHTTPURL(feed.URL); err != nil {
		return "failed", 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed.URL, nil)
	if err != nil {
		return "failed", 0, err
	}
	req.Header.Set("User-Agent", "network-monitor-reputation/1.0")
	s.mu.Lock()
	if !force {
		if et := s.etag[feed.Name]; et != "" {
			req.Header.Set("If-None-Match", et)
		}
		if lm := s.lastMod[feed.Name]; lm != "" {
			req.Header.Set("If-Modified-Since", lm)
		}
	}
	s.mu.Unlock()

	resp, err := s.client.Do(req)
	if err != nil {
		return "failed", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		slog.Info("reputation feed not modified", "list", feed.Name)
		return "skipped", 0, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "failed", 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20)) // 64 MiB cap
	if err != nil {
		return "failed", 0, err
	}
	if looksDeprecatedEmpty(body) {
		return "failed", 0, fmt.Errorf("feed empty or deprecated (no IPv4 entries)")
	}

	format := strings.ToLower(feed.Format)
	if format == "" {
		format = "netset"
	}
	ranges, err := reppkg.ParseFeedBody(format, bytes.NewReader(body), feed.Name, feed.Category, "url", time.Now().UTC())
	if err != nil {
		return "failed", 0, err
	}

	n, err := s.applier.ApplyListRanges(ctx, feed.Name, ranges)
	if err != nil {
		return "failed", 0, err
	}

	s.mu.Lock()
	if et := resp.Header.Get("ETag"); et != "" {
		s.etag[feed.Name] = et
	}
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		s.lastMod[feed.Name] = lm
	}
	s.mu.Unlock()

	slog.Info("reputation feed updated", "list", feed.Name, "ranges", n)
	return "updated", n, nil
}

func looksDeprecatedEmpty(body []byte) bool {
	if !containsASCIIFold(body, "deprecated") {
		return false
	}
	// Есть ли хоть один IPv4-токен вне комментариев — грубая проверка.
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if bytes.ContainsAny(line, "0123456789") && bytes.IndexByte(line, '.') >= 0 {
			return false
		}
	}
	return true
}

// containsASCIIFold ищет ASCII-подстроку без аллокации ToLower всего body.
func containsASCIIFold(haystack []byte, needle string) bool {
	n := len(needle)
	if n == 0 || len(haystack) < n {
		return false
	}
	for i := 0; i+n <= len(haystack); i++ {
		ok := true
		for j := 0; j < n; j++ {
			a := haystack[i+j]
			b := needle[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
