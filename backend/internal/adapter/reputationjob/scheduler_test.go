package reputationjob

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"geoatlas/internal/adapter/heavytask"
	"geoatlas/internal/model"
	usecasereputation "geoatlas/internal/usecase/reputation"
)

type fakeApplier struct {
	applied   map[string]int
	feedErr   map[string]string
	lists     []model.ReputationListMeta
	listErr   error
	deleteErr error
	deleted   []string
	applyErr  error
}

func (f *fakeApplier) ApplyListRanges(_ context.Context, listName string, ranges []model.ReputationRange) (int, error) {
	if f.applyErr != nil {
		return 0, f.applyErr
	}
	if f.applied == nil {
		f.applied = map[string]int{}
	}
	f.applied[listName] = len(ranges)
	return len(ranges), nil
}
func (f *fakeApplier) SetFeedError(listName, errMsg string) {
	if f.feedErr == nil {
		f.feedErr = map[string]string{}
	}
	f.feedErr[listName] = errMsg
}
func (f *fakeApplier) ListLists(context.Context) ([]model.ReputationListMeta, error) {
	return f.lists, f.listErr
}
func (f *fakeApplier) DeleteListData(_ context.Context, name string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, name)
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLooksDeprecatedEmpty(t *testing.T) {
	if looksDeprecatedEmpty([]byte("# spamhaus drop\n1.2.3.0/24\n")) {
		t.Fatal("active feed with IPs must not look deprecated")
	}
	if !looksDeprecatedEmpty([]byte("# DEPRECATED — use new URL\n# no ips left\n")) {
		t.Fatal("deprecated empty feed")
	}
	if looksDeprecatedEmpty([]byte("# DEPRECATED\n8.8.8.8\n")) {
		t.Fatal("deprecated but still has IP")
	}
	if looksDeprecatedEmpty([]byte("just a normal list\n")) {
		t.Fatal("no deprecated marker")
	}
}

func TestContainsASCIIFold(t *testing.T) {
	if !containsASCIIFold([]byte("Foo DEPRECATED bar"), "deprecated") {
		t.Fatal("case-insensitive hit")
	}
	if containsASCIIFold([]byte("deprecat"), "deprecated") {
		t.Fatal("partial must miss")
	}
	if containsASCIIFold([]byte("ab"), "deprecated") {
		t.Fatal("short haystack")
	}
	if containsASCIIFold([]byte("x"), "") {
		t.Fatal("empty needle")
	}
}

func TestNewClampsIntervalAndNilGuards(t *testing.T) {
	s := New(nil, time.Second, false, nil)
	if s.interval < time.Minute {
		t.Fatalf("interval=%v", s.interval)
	}
	var nilS *Scheduler
	nilS.SetLimiter(heavytask.New(1))
	nilS.Start(context.Background())
	nilS.Shutdown(context.Background())
	nilS.SetFeeds(nil)
	nilS.pruneObsoleteURLLists(context.Background(), nil, nil)
}

func TestStartDisabledAndSetFeeds(t *testing.T) {
	app := &fakeApplier{}
	s := New(nil, time.Minute, false, app)
	s.SetLimiter(heavytask.New(1))
	s.Start(context.Background())
	s.Shutdown(context.Background())

	s.SetFeeds([]usecasereputation.Feed{{Name: "a", URL: "http://8.8.8.8/a"}})
	got := s.snapshotFeeds()
	if len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("%+v", got)
	}
}

func TestRefreshAllFetchPaths(t *testing.T) {
	app := &fakeApplier{
		lists: []model.ReputationListMeta{
			{Name: "keep", Source: "url"},
			{Name: "gone", Source: "url"},
			{Name: "upload", Source: "upload"},
		},
	}
	feeds := []usecasereputation.Feed{
		{Name: "keep", URL: "http://8.8.8.8/keep", Format: "netset", Category: "malware"},
		{Name: "badurl", URL: "http://127.0.0.1/x", Format: "netset"},
		{Name: "httpfail", URL: "http://8.8.8.8/fail", Format: "netset"},
		{Name: "notmod", URL: "http://8.8.8.8/nm", Format: "netset"},
		{Name: "depr", URL: "http://8.8.8.8/depr", Format: "netset"},
		{Name: "applyfail", URL: "http://8.8.8.8/af", Format: ""},
	}
	s := New(feeds, time.Minute, true, app)
	s.etag["notmod"] = `"abc"`
	s.lastMod["notmod"] = "Wed, 01 Jan 2020 00:00:00 GMT"
	s.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.Path, "keep"):
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("1.2.3.0/24\n")),
				Header:     http.Header{"ETag": []string{`"e1"`}, "Last-Modified": []string{"Thu, 01 Jan 2021 00:00:00 GMT"}},
			}, nil
		case strings.Contains(req.URL.Path, "fail"):
			return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("nope")), Header: make(http.Header)}, nil
		case strings.Contains(req.URL.Path, "nm"):
			return &http.Response{StatusCode: http.StatusNotModified, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		case strings.Contains(req.URL.Path, "depr"):
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("# DEPRECATED\n# empty\n")),
				Header:     make(http.Header),
			}, nil
		case strings.Contains(req.URL.Path, "af"):
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("9.9.9.0/24\n")),
				Header:     make(http.Header),
			}, nil
		default:
			return nil, errors.New("unexpected")
		}
	})}
	app.applyErr = errors.New("apply boom")
	// first run applyfail with applyErr — then clear for keep... actually keep runs first.
	// Order: keep, badurl, httpfail, notmod, depr, applyfail
	app.applyErr = nil
	res, err := s.RefreshAll(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Updated) < 1 {
		t.Fatalf("updated=%v", res.Updated)
	}
	foundRemoved := false
	for _, u := range res.Updated {
		if u == "removed:gone" {
			foundRemoved = true
		}
	}
	if !foundRemoved {
		t.Fatalf("prune missing: %+v", res)
	}
	if len(res.Failed) == 0 || len(res.Skipped) == 0 {
		t.Fatalf("failed=%v skipped=%v errors=%v", res.Failed, res.Skipped, res.Errors)
	}
	if app.feedErr["badurl"] == "" && app.feedErr["httpfail"] == "" && app.feedErr["depr"] == "" {
		t.Fatalf("expected SetFeedError: %+v", app.feedErr)
	}

	// force + apply error
	app.applyErr = errors.New("apply boom")
	s.SetFeeds([]usecasereputation.Feed{{Name: "af2", URL: "http://8.8.8.8/af", Format: "netset"}})
	res2, _ := s.RefreshAll(context.Background(), true)
	if len(res2.Failed) != 1 {
		t.Fatalf("%+v", res2)
	}
}

func TestPruneListErrors(t *testing.T) {
	app := &fakeApplier{listErr: errors.New("meta fail")}
	s := New(nil, time.Minute, true, app)
	res := &usecasereputation.RefreshResult{}
	s.pruneObsoleteURLLists(context.Background(), nil, res)

	app = &fakeApplier{
		lists:     []model.ReputationListMeta{{Name: "old", Source: "url"}},
		deleteErr: errors.New("del fail"),
	}
	s.applier = app
	s.pruneObsoleteURLLists(context.Background(), nil, res)
	if len(res.Updated) != 0 {
		t.Fatalf("%+v", res)
	}
}

func TestRunOnceHeavyAcquireCancel(t *testing.T) {
	lim := heavytask.New(1)
	if err := lim.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := New(nil, time.Minute, true, &fakeApplier{})
	s.heavy = lim
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := s.runOnce(ctx, false)
	if len(res.Updated)+len(res.Failed)+len(res.Skipped) != 0 {
		t.Fatalf("%+v", res)
	}
	lim.Release()
}

func TestRunOnceStopsOnCtx(t *testing.T) {
	app := &fakeApplier{}
	s := New([]usecasereputation.Feed{
		{Name: "a", URL: "http://8.8.8.8/a", Format: "netset"},
		{Name: "b", URL: "http://8.8.8.8/b", Format: "netset"},
	}, time.Minute, true, app)
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	s.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		n++
		cancel()
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("1.1.1.0/24\n")),
			Header:     make(http.Header),
		}, nil
	})}
	_ = s.runOnce(ctx, true)
	if n < 1 {
		t.Fatal("expected at least one fetch")
	}
}
