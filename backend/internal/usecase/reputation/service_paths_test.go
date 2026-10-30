package reputation_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"geoatlas/internal/adapter/reputationcodec"
	"geoatlas/internal/model"
	"geoatlas/internal/safeurl"
	usecasereputation "geoatlas/internal/usecase/reputation"
)

type trackingIndex struct {
	ranges []model.ReputationRange
	hits   map[string][]model.ReputationHit
}

func (i *trackingIndex) Lookup(ip string) []model.ReputationHit {
	if i.hits == nil {
		return nil
	}
	return i.hits[ip]
}
func (i *trackingIndex) RangeCount() int { return len(i.ranges) }
func (i *trackingIndex) ReplaceAll(ranges []model.ReputationRange) {
	i.ranges = append([]model.ReputationRange{}, ranges...)
}
func (i *trackingIndex) ReplaceList(listName string, ranges []model.ReputationRange) {
	kept := i.ranges[:0]
	for _, r := range i.ranges {
		if r.ListName != listName {
			kept = append(kept, r)
		}
	}
	i.ranges = append(kept, ranges...)
}
func (i *trackingIndex) DeleteList(listName string) {
	kept := i.ranges[:0]
	for _, r := range i.ranges {
		if r.ListName != listName {
			kept = append(kept, r)
		}
	}
	i.ranges = kept
}
func (i *trackingIndex) ListMeta() []model.ReputationListMeta {
	by := map[string]model.ReputationListMeta{}
	for _, r := range i.ranges {
		m := by[r.ListName]
		m.Name = r.ListName
		m.Category = r.Category
		m.Count++
		m.Source = r.Source
		m.UpdatedAt = r.UpdatedAt
		by[r.ListName] = m
	}
	out := make([]model.ReputationListMeta, 0, len(by))
	for _, m := range by {
		out = append(out, m)
	}
	return out
}
func (i *trackingIndex) Snapshot() []model.ReputationRange {
	return append([]model.ReputationRange{}, i.ranges...)
}

type errStore struct {
	memStore
	loadErr, replaceErr, deleteErr, metaErr error
}

func (e *errStore) Load(ctx context.Context) ([]model.ReputationRange, error) {
	if e.loadErr != nil {
		return nil, e.loadErr
	}
	return e.memStore.Load(ctx)
}
func (e *errStore) ReplaceList(ctx context.Context, listName string, ranges []model.ReputationRange) (int, error) {
	if e.replaceErr != nil {
		return 0, e.replaceErr
	}
	return e.memStore.ReplaceList(ctx, listName, ranges)
}
func (e *errStore) DeleteList(ctx context.Context, listName string) error {
	if e.deleteErr != nil {
		return e.deleteErr
	}
	return e.memStore.DeleteList(ctx, listName)
}
func (e *errStore) ListMeta(ctx context.Context) ([]model.ReputationListMeta, error) {
	if e.metaErr != nil {
		return nil, e.metaErr
	}
	return []model.ReputationListMeta{{Name: "from-store", Count: 1}}, nil
}

type errFeedStore struct {
	loadErr, saveErr error
	feeds            []usecasereputation.Feed
	ok               bool
}

func (e *errFeedStore) Load() ([]usecasereputation.Feed, bool, error) {
	if e.loadErr != nil {
		return nil, false, e.loadErr
	}
	return e.feeds, e.ok, nil
}
func (e *errFeedStore) Save(feeds []usecasereputation.Feed) error {
	if e.saveErr != nil {
		return e.saveErr
	}
	e.feeds = feeds
	e.ok = true
	return nil
}

func sampleCSV(n int) string {
	var b strings.Builder
	b.WriteString("Network,List,Category\n")
	for i := 0; i < n; i++ {
		b.WriteString("1.2.3.")
		b.WriteByte(byte('0' + (i % 10)))
		if i%2 == 0 {
			b.WriteString(",alpha,malware\n")
		} else {
			b.WriteString(",beta,c2\n")
		}
	}
	return b.String()
}

func TestSetRefresherAndNilService(t *testing.T) {
	var nilSvc *usecasereputation.Service
	nilSvc.SetRefresher(&stubRefresher{})

	svc := usecasereputation.New(&memStore{}, &trackingIndex{}, reputationcodec.New(), nil, nil)
	ref := &stubRefresher{}
	svc.SetRefresher(ref)
	got, err := svc.Refresh(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	_ = got
}

func TestRefreshWithoutRefresher(t *testing.T) {
	svc := usecasereputation.New(&memStore{}, &trackingIndex{}, reputationcodec.New(), nil, nil)
	if _, err := svc.Refresh(context.Background(), false); err == nil {
		t.Fatal("expected refresher not configured")
	}
}

func TestUploadCSVDryRunAndApply(t *testing.T) {
	store := &memStore{lists: map[string][]model.ReputationRange{}}
	idx := &trackingIndex{}
	svc := usecasereputation.New(store, idx, reputationcodec.New(), nil, nil)

	dry, err := svc.UploadCSV(context.Background(), strings.NewReader(sampleCSV(6)), true)
	if err != nil {
		t.Fatal(err)
	}
	if !dry.DryRun || dry.Count != 6 || len(dry.Sample) != 5 || len(dry.Lists) != 2 {
		t.Fatalf("dry: %+v", dry)
	}
	if len(store.lists) != 0 {
		t.Fatal("dry-run must not write store")
	}

	applied, err := svc.UploadCSV(context.Background(), strings.NewReader(sampleCSV(3)), false)
	if err != nil {
		t.Fatal(err)
	}
	if applied.DryRun || applied.Count != 3 || len(applied.Lists) != 2 {
		t.Fatalf("apply: %+v", applied)
	}
	if len(idx.ranges) != 3 {
		t.Fatalf("index ranges=%d", len(idx.ranges))
	}

	if _, err := svc.UploadCSV(context.Background(), strings.NewReader("bad"), false); err == nil {
		t.Fatal("expected invalid csv")
	}

	badStore := &errStore{replaceErr: errors.New("ch down")}
	svc2 := usecasereputation.New(badStore, idx, reputationcodec.New(), nil, nil)
	if _, err := svc2.UploadCSV(context.Background(), strings.NewReader(sampleCSV(1)), false); err == nil {
		t.Fatal("expected store error")
	}
}

func TestLookupPaths(t *testing.T) {
	idx := &trackingIndex{hits: map[string][]model.ReputationHit{
		"8.8.8.8": {{List: "dns", Category: "dns"}},
	}}
	svc := usecasereputation.New(&memStore{}, idx, reputationcodec.New(), nil, nil)

	if _, err := svc.Lookup(""); err == nil {
		t.Fatal("empty ip")
	}
	if _, err := svc.Lookup("not-an-ip"); err == nil {
		t.Fatal("invalid ip")
	}
	if _, err := svc.Lookup("2001:db8::1"); err == nil {
		t.Fatal("ipv6 rejected")
	}
	hits, err := svc.Lookup(" 8.8.8.8 ")
	if err != nil || len(hits) != 1 || hits[0].List != "dns" {
		t.Fatalf("hits=%+v err=%v", hits, err)
	}

	svcNilIdx := usecasereputation.New(&memStore{}, nil, reputationcodec.New(), nil, nil)
	out, err := svcNilIdx.Lookup("1.1.1.1")
	if err != nil || out != nil {
		t.Fatalf("nil index: %+v %v", out, err)
	}
}

func TestListListsPaths(t *testing.T) {
	now := time.Now().UTC()
	idx := &trackingIndex{ranges: []model.ReputationRange{{
		ListName: "alpha", Category: "malware", StartIP: 1, EndIP: 2, Source: "upload", UpdatedAt: now,
	}}}
	svc := usecasereputation.New(&memStore{}, idx, reputationcodec.New(), nil, nil)
	svc.SetFeedError("alpha", "fetch failed")
	meta, err := svc.ListLists(context.Background())
	if err != nil || len(meta) != 1 || meta[0].LastError != "fetch failed" {
		t.Fatalf("index meta: %+v err=%v", meta, err)
	}
	svc.SetFeedError("alpha", "")

	emptyIdx := &trackingIndex{}
	store := &errStore{}
	svc2 := usecasereputation.New(store, emptyIdx, reputationcodec.New(), nil, nil)
	meta, err = svc2.ListLists(context.Background())
	if err != nil || len(meta) != 1 || meta[0].Name != "from-store" {
		t.Fatalf("store meta: %+v err=%v", meta, err)
	}

	store.metaErr = errors.New("meta boom")
	if _, err := svc2.ListLists(context.Background()); err == nil {
		t.Fatal("expected meta error")
	}
}

func TestDeleteListAndData(t *testing.T) {
	store := &memStore{lists: map[string][]model.ReputationRange{
		"dshield": {{ListName: "dshield", StartIP: 1, EndIP: 2}},
	}}
	idx := &trackingIndex{ranges: []model.ReputationRange{{ListName: "dshield", StartIP: 1, EndIP: 2}}}
	ref := &stubRefresher{}
	feedStore := &errFeedStore{
		ok:    true,
		feeds: []usecasereputation.Feed{{Name: "dshield", URL: "https://example.com/dshield.netset", Category: "attacks", Format: "netset"}},
	}
	svc := usecasereputation.New(store, idx, reputationcodec.New(), ref, feedStore)

	if err := svc.DeleteList(context.Background(), "dshield"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.lists["dshield"]; ok {
		t.Fatal("list should be gone")
	}
	if len(idx.ranges) != 0 {
		t.Fatalf("index still has %d", len(idx.ranges))
	}
	if len(feedStore.feeds) != 0 || len(ref.feeds) != 0 {
		t.Fatalf("feeds after delete: store=%+v ref=%+v", feedStore.feeds, ref.feeds)
	}

	if err := svc.DeleteList(context.Background(), "  "); err == nil {
		t.Fatal("empty name")
	}
	if err := svc.DeleteListData(context.Background(), ""); err == nil {
		t.Fatal("empty DeleteListData")
	}

	bad := &errStore{deleteErr: errors.New("del fail")}
	svcBad := usecasereputation.New(bad, idx, reputationcodec.New(), nil, nil)
	if err := svcBad.DeleteListData(context.Background(), "x"); err == nil {
		t.Fatal("expected delete error")
	}
}

func TestListFeedsAndCatalog(t *testing.T) {
	svcNil := usecasereputation.New(&memStore{}, nil, reputationcodec.New(), nil, nil)
	feeds, err := svcNil.ListFeeds()
	if err != nil || feeds != nil {
		t.Fatalf("nil feedStore: %+v %v", feeds, err)
	}
	cat := svcNil.ListCatalog()
	if len(cat) < 3 {
		t.Fatalf("catalog=%d", len(cat))
	}

	fs := &errFeedStore{ok: false}
	svc := usecasereputation.New(&memStore{}, nil, reputationcodec.New(), nil, fs)
	feeds, err = svc.ListFeeds()
	if err != nil || len(feeds) != 0 {
		t.Fatalf("empty ok=false: %+v %v", feeds, err)
	}
	fs.loadErr = errors.New("load fail")
	if _, err := svc.ListFeeds(); err == nil {
		t.Fatal("expected load error")
	}
}

func TestAddFeedValidationAndErrors(t *testing.T) {
	orig := safeurl.LookupIPv4
	t.Cleanup(func() { safeurl.LookupIPv4 = orig })
	safeurl.LookupIPv4 = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}

	svcNoStore := usecasereputation.New(&memStore{}, nil, reputationcodec.New(), nil, nil)
	if err := svcNoStore.AddFeed(context.Background(), usecasereputation.Feed{
		Name: "ok", URL: "https://example.com/a", Format: "netset",
	}); err == nil {
		t.Fatal("feed store not configured")
	}

	fs := &errFeedStore{ok: true}
	svc := usecasereputation.New(&memStore{}, nil, reputationcodec.New(), &stubRefresher{}, fs)

	cases := []usecasereputation.Feed{
		{Name: "", URL: "https://example.com/a"},
		{Name: "bad name!", URL: "https://example.com/a"},
		{Name: "ok", URL: ""},
		{Name: "ok", URL: "https://example.com/a", Category: strings.Repeat("x", 65)},
		{Name: "ok", URL: "https://example.com/a", Format: "weird"},
	}
	for _, c := range cases {
		if err := svc.AddFeed(context.Background(), c); err == nil {
			t.Fatalf("expected validation error for %+v", c)
		}
	}

	// Default category + format when empty.
	if err := svc.AddFeed(context.Background(), usecasereputation.Feed{
		Name: "plain1", URL: "https://example.com/plain",
	}); err != nil {
		t.Fatal(err)
	}
	if fs.feeds[0].Category != "unknown" || fs.feeds[0].Format == "" {
		t.Fatalf("defaults: %+v", fs.feeds[0])
	}

	fs.loadErr = errors.New("boom")
	if err := svc.AddFeed(context.Background(), usecasereputation.Feed{
		Name: "plain2", URL: "https://example.com/plain2",
	}); err == nil {
		t.Fatal("expected load error")
	}
	fs.loadErr = nil
	fs.saveErr = errors.New("save boom")
	if err := svc.AddFeed(context.Background(), usecasereputation.Feed{
		Name: "plain2", URL: "https://example.com/plain2",
	}); err == nil {
		t.Fatal("expected save error")
	}
}

func TestRemoveFeedPaths(t *testing.T) {
	svc := usecasereputation.New(&memStore{lists: map[string][]model.ReputationRange{}}, &trackingIndex{}, reputationcodec.New(), nil, nil)
	if err := svc.RemoveFeed(context.Background(), "  "); err == nil {
		t.Fatal("empty name")
	}
	// No feed store: removeFeedConfig no-op, then DeleteListData.
	if err := svc.RemoveFeed(context.Background(), "missing"); err != nil {
		t.Fatal(err)
	}

	fs := &errFeedStore{
		ok:    true,
		feeds: []usecasereputation.Feed{{Name: "keep", URL: "https://example.com/k"}},
	}
	fs.loadErr = errors.New("load")
	svc2 := usecasereputation.New(&memStore{}, &trackingIndex{}, reputationcodec.New(), nil, fs)
	if err := svc2.RemoveFeed(context.Background(), "keep"); err == nil {
		t.Fatal("expected removeFeedConfig load error")
	}
}

func TestApplyListRangesAndReload(t *testing.T) {
	store := &memStore{lists: map[string][]model.ReputationRange{}}
	idx := &trackingIndex{}
	svc := usecasereputation.New(store, idx, reputationcodec.New(), nil, nil)

	n, err := svc.ApplyListRanges(context.Background(), "joblist", []model.ReputationRange{
		{ListName: "joblist", Category: "c2", StartIP: 10, EndIP: 20, Source: "url"},
		{ListName: "joblist", Category: "c2", StartIP: 11, EndIP: 15, Source: "url"}, // overlap normalize
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("normalized count=%d", n)
	}
	if len(store.lists["joblist"]) != 2 {
		t.Fatalf("store wrote %d", len(store.lists["joblist"]))
	}

	bad := &errStore{replaceErr: errors.New("replace")}
	svcBad := usecasereputation.New(bad, idx, reputationcodec.New(), nil, nil)
	if _, err := svcBad.ApplyListRanges(context.Background(), "x", nil); err == nil {
		t.Fatal("expected replace error")
	}

	if err := svc.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(idx.ranges) != 2 {
		t.Fatalf("reload index=%d", len(idx.ranges))
	}

	loadFail := &errStore{loadErr: errors.New("load")}
	svcFail := usecasereputation.New(loadFail, idx, reputationcodec.New(), nil, nil)
	if err := svcFail.Reload(context.Background()); err == nil {
		t.Fatal("expected reload error")
	}
}

func TestWithoutRetiredEmptyAndAllRetired(t *testing.T) {
	out, changed := usecasereputation.WithoutRetired(nil)
	if out != nil || changed {
		t.Fatalf("%+v %v", out, changed)
	}
	out, changed = usecasereputation.WithoutRetired([]usecasereputation.Feed{
		{Name: "sslbl"}, {Name: "cruzit_web_attacks"},
	})
	if out != nil || !changed {
		t.Fatalf("%+v %v", out, changed)
	}
}
