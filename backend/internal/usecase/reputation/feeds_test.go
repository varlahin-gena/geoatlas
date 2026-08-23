package reputation_test

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"geoatlas/internal/adapter/reputationcodec"
	"geoatlas/internal/adapter/reputationfeedsfile"
	"geoatlas/internal/model"
	"geoatlas/internal/safeurl"
	usecasereputation "geoatlas/internal/usecase/reputation"
)

type memStore struct {
	lists map[string][]model.ReputationRange
}

func (m *memStore) Load(ctx context.Context) ([]model.ReputationRange, error) {
	var out []model.ReputationRange
	for _, rs := range m.lists {
		out = append(out, rs...)
	}
	return out, nil
}
func (m *memStore) ReplaceAll(ctx context.Context, ranges []model.ReputationRange) (int, error) {
	m.lists = map[string][]model.ReputationRange{}
	return m.ReplaceList(ctx, "", ranges)
}
func (m *memStore) ReplaceList(ctx context.Context, listName string, ranges []model.ReputationRange) (int, error) {
	if m.lists == nil {
		m.lists = map[string][]model.ReputationRange{}
	}
	if listName == "" && len(ranges) > 0 {
		listName = ranges[0].ListName
	}
	m.lists[listName] = ranges
	return len(ranges), nil
}
func (m *memStore) DeleteList(ctx context.Context, listName string) error {
	delete(m.lists, listName)
	return nil
}
func (m *memStore) ListMeta(ctx context.Context) ([]model.ReputationListMeta, error) {
	return nil, nil
}

type memIndex struct{}

func (memIndex) Lookup(string) []model.ReputationHit             { return nil }
func (memIndex) RangeCount() int                                 { return 0 }
func (memIndex) ReplaceAll([]model.ReputationRange)              {}
func (memIndex) ReplaceList(string, []model.ReputationRange)     {}
func (memIndex) DeleteList(string)                               {}
func (memIndex) ListMeta() []model.ReputationListMeta            { return nil }
func (memIndex) Snapshot() []model.ReputationRange               { return nil }

type stubRefresher struct {
	feeds []usecasereputation.Feed
}

func (s *stubRefresher) RefreshAll(ctx context.Context, force bool) (usecasereputation.RefreshResult, error) {
	return usecasereputation.RefreshResult{}, nil
}
func (s *stubRefresher) SetFeeds(feeds []usecasereputation.Feed) { s.feeds = feeds }

func TestAddRemoveFeed(t *testing.T) {
	orig := safeurl.LookupIPv4
	t.Cleanup(func() { safeurl.LookupIPv4 = orig })
	safeurl.LookupIPv4 = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}

	path := filepath.Join(t.TempDir(), "feeds.json")
	store := reputationfeedsfile.New(path)
	if err := store.Save(nil); err != nil {
		t.Fatal(err)
	}
	ref := &stubRefresher{}
	svc := usecasereputation.New(&memStore{lists: map[string][]model.ReputationRange{}}, memIndex{}, reputationcodec.New(), ref, store)

	err := svc.AddFeed(context.Background(), usecasereputation.Feed{
		Name: "dshield", URL: "https://example.com/dshield.netset", Category: "attacks", Format: "netset",
	})
	if err != nil {
		t.Fatal(err)
	}
	feeds, err := svc.ListFeeds()
	if err != nil || len(feeds) != 1 || feeds[0].Name != "dshield" {
		t.Fatalf("list: %+v err=%v", feeds, err)
	}
	if len(ref.feeds) != 1 {
		t.Fatalf("refresher not updated: %+v", ref.feeds)
	}

	if err := svc.AddFeed(context.Background(), usecasereputation.Feed{
		Name: "dshield", URL: "https://example.com/other", Category: "attacks",
	}); err == nil || !usecasereputation.IsConflict(err) {
		t.Fatalf("expected conflict duplicate err, got %v", err)
	}

	if err := svc.AddFeed(context.Background(), usecasereputation.Feed{
		Name: "evil", URL: "http://127.0.0.1/list", Category: "attacks", Format: "netset",
	}); err == nil {
		t.Fatal("expected SSRF reject for loopback URL")
	}

	if err := svc.RemoveFeed(context.Background(), "dshield"); err != nil {
		t.Fatal(err)
	}
	feeds, err = svc.ListFeeds()
	if err != nil || len(feeds) != 0 {
		t.Fatalf("after remove: %+v err=%v", feeds, err)
	}
	if len(ref.feeds) != 0 {
		t.Fatalf("refresher should be empty: %+v", ref.feeds)
	}
}
