package reputationjob

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"network_monitor/internal/config"
	"network_monitor/internal/model"
	reppkg "network_monitor/internal/reputation"
	usecasereputation "network_monitor/internal/usecase/reputation"
)

// Applier — запись списка после успешного download.
type Applier interface {
	ApplyListRanges(ctx context.Context, listName string, ranges []model.ReputationRange) (int, error)
	SetFeedError(listName, errMsg string)
}

// Scheduler периодически качает REPUTATION_FEEDS.
type Scheduler struct {
	feeds    []config.ReputationFeed
	interval time.Duration
	enabled  bool
	applier  Applier
	client   *http.Client

	mu      sync.Mutex
	etag    map[string]string
	lastMod map[string]string
	cancel  context.CancelFunc
	done    chan struct{}
}

func New(feeds []config.ReputationFeed, interval time.Duration, enabled bool, applier Applier) *Scheduler {
	if interval < time.Minute {
		interval = time.Minute
	}
	return &Scheduler{
		feeds:    feeds,
		interval: interval,
		enabled:  enabled,
		applier:  applier,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
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

func (s *Scheduler) runOnce(ctx context.Context, force bool) usecasereputation.RefreshResult {
	res := usecasereputation.RefreshResult{
		Errors: map[string]string{},
	}
	for _, feed := range s.feeds {
		if ctx.Err() != nil {
			break
		}
		status, err := s.fetchOne(ctx, feed, force)
		switch status {
		case "updated":
			res.Updated = append(res.Updated, feed.Name)
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
	return res
}

func (s *Scheduler) fetchOne(ctx context.Context, feed config.ReputationFeed, force bool) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed.URL, nil)
	if err != nil {
		return "failed", err
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
		return "failed", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		slog.Info("reputation feed not modified", "list", feed.Name)
		return "skipped", nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "failed", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20)) // 64 MiB cap
	if err != nil {
		return "failed", err
	}

	format := strings.ToLower(feed.Format)
	if format == "" {
		format = "netset"
	}
	var ranges []model.ReputationRange
	switch format {
	case "netset":
		ranges, err = reppkg.ParseNetset(strings.NewReader(string(body)), feed.Name, feed.Category, "url", time.Now().UTC())
	default:
		return "failed", fmt.Errorf("unsupported feed format %q", format)
	}
	if err != nil {
		return "failed", err
	}

	if _, err := s.applier.ApplyListRanges(ctx, feed.Name, ranges); err != nil {
		return "failed", err
	}

	s.mu.Lock()
	if et := resp.Header.Get("ETag"); et != "" {
		s.etag[feed.Name] = et
	}
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		s.lastMod[feed.Name] = lm
	}
	s.mu.Unlock()

	slog.Info("reputation feed updated", "list", feed.Name, "ranges", len(ranges))
	return "updated", nil
}
