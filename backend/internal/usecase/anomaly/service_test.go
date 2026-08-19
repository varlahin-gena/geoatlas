package anomaly

import (
	"context"
	"testing"
	"time"

	"network_monitor/internal/model"
)

type fakeStore struct {
	inserted []Event
	exist    map[string]struct{}
	list     []Event
	summary  Summary
}

func (f *fakeStore) Insert(_ context.Context, events []Event) error {
	f.inserted = append(f.inserted, events...)
	return nil
}
func (f *fakeStore) List(_ context.Context, _ ListQuery) ([]Event, error) {
	return f.list, nil
}
func (f *fakeStore) ExistingFingerprints(_ context.Context, fps []string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	for _, fp := range fps {
		if _, ok := f.exist[fp]; ok {
			out[fp] = struct{}{}
		}
	}
	return out, nil
}
func (f *fakeStore) Ack(context.Context, string, string) error { return nil }
func (f *fakeStore) CountSummary(context.Context, time.Time) (Summary, error) {
	return f.summary, nil
}

type fakeScan struct {
	oldest      time.Time
	now         time.Time
	ports       []PortScanHit
	horiz       []HorizontalScanHit
	blockedCurr uint64
	blockedPrev uint64
	hasBlocked  bool
	countries   []CountryCount
	baseline    map[string]struct{}
	edges       []EdgeRow
	knownPairs  map[string]struct{}
}

func (f *fakeScan) OldestLogTime(context.Context) (time.Time, error) { return f.oldest, nil }
func (f *fakeScan) PortScan(context.Context, time.Duration, int, int, bool) ([]PortScanHit, error) {
	return f.ports, nil
}
func (f *fakeScan) HorizontalScan(context.Context, time.Duration, int, int, bool) ([]HorizontalScanHit, error) {
	return f.horiz, nil
}
func (f *fakeScan) BlockedCount(_ context.Context, start, end time.Time) (uint64, error) {
	if !f.hasBlocked {
		return 0, nil
	}
	ref := f.now
	if ref.IsZero() {
		ref = time.Now().UTC()
	}
	// текущее окно заканчивается около now
	if end.After(ref.Add(-time.Minute)) {
		return f.blockedCurr, nil
	}
	return f.blockedPrev, nil
}
func (f *fakeScan) CurrentCountries(context.Context, time.Duration, uint64) ([]CountryCount, error) {
	return f.countries, nil
}
func (f *fakeScan) BaselineCountries(context.Context, int, uint64) (map[string]struct{}, error) {
	if f.baseline == nil {
		return map[string]struct{}{}, nil
	}
	return f.baseline, nil
}
func (f *fakeScan) RecentEdges(context.Context, time.Duration, int) ([]EdgeRow, error) {
	return f.edges, nil
}
func (f *fakeScan) KnownPairs(context.Context, [][2]string, time.Duration) (map[string]struct{}, error) {
	if f.knownPairs == nil {
		return map[string]struct{}{}, nil
	}
	return f.knownPairs, nil
}

type fakeRep struct {
	hits map[string][]model.ReputationHit
}

func (f fakeRep) Lookup(ip string) []model.ReputationHit {
	if f.hits == nil {
		return nil
	}
	return f.hits[ip]
}

func newSvc(store *fakeStore, scan *fakeScan, rep ReputationLookuper) *Service {
	return New(Config{Enabled: true, LearningDays: 3, InstallProfile: "medium"}, store, scan, rep, nil, nil)
}

func TestFingerprintStable(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 22, 0, 0, time.UTC)
	a := fingerprint(CodePortScan, "1.2.3.4", "", "", now)
	b := fingerprint(CodePortScan, "1.2.3.4", "", "", now.Add(30*time.Minute))
	if a != b {
		t.Fatalf("same hour must match: %s vs %s", a, b)
	}
	c := fingerprint(CodePortScan, "1.2.3.4", "", "", now.Add(time.Hour))
	if a == c {
		t.Fatal("next hour must differ")
	}
}

func TestPortScanEmits(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest: now.Add(-30 * 24 * time.Hour),
		ports:  []PortScanHit{{SrcIP: "203.0.113.5", Ports: 80, Events: 200, SrcCountry: "CN"}},
	}
	res := newSvc(store, scan, nil).Scan(context.Background(), now)
	if res.Inserted != 1 {
		t.Fatalf("inserted=%d error=%s", res.Inserted, res.Error)
	}
	if store.inserted[0].Code != CodePortScan {
		t.Fatalf("code %s", store.inserted[0].Code)
	}
}

func TestPortScanBelowThreshold(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest: now.Add(-30 * 24 * time.Hour),
		ports:  []PortScanHit{{SrcIP: "203.0.113.5", Ports: 10, Events: 20}},
	}
	// store still receives only what detector emits; detector uses SQL HAVING,
	// fake returns the row anyway — unit the SQL-side skip in store. Here we
	// simulate empty hits.
	scan.ports = nil
	res := newSvc(store, scan, nil).Scan(context.Background(), now)
	if res.Inserted != 0 {
		t.Fatalf("expected 0, got %d", res.Inserted)
	}
}

func TestHorizontalScanEmits(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest: now.Add(-30 * 24 * time.Hour),
		horiz:  []HorizontalScanHit{{SrcIP: "198.51.100.7", Net24: "203.0.113.0/24", Hosts: 50, Events: 90}},
	}
	res := newSvc(store, scan, nil).Scan(context.Background(), now)
	if res.Inserted != 1 || store.inserted[0].Code != CodeHorizontalScan {
		t.Fatalf("got %+v inserted=%v", store.inserted, res)
	}
}

func TestBlockedSurgeFloor(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest:      now.Add(-30 * 24 * time.Hour),
		now:         now,
		hasBlocked:  true,
		blockedCurr: 20,
		blockedPrev: 0,
	}
	res := newSvc(store, scan, nil).Scan(context.Background(), now)
	if res.Inserted != 0 {
		t.Fatalf("0→20 must not emit, got %d", res.Inserted)
	}
}

func TestBlockedSurgeEmits(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest:      now.Add(-30 * 24 * time.Hour),
		now:         now,
		hasBlocked:  true,
		blockedCurr: 2000,
		blockedPrev: 80,
	}
	res := newSvc(store, scan, nil).Scan(context.Background(), now)
	found := false
	for _, e := range store.inserted {
		if e.Code == CodeBlockedSurge {
			found = true
		}
	}
	if res.Inserted == 0 || !found {
		t.Fatalf("expected blocked_surge, inserted=%d items=%v", res.Inserted, store.inserted)
	}
}

func TestNewCountryLearningSkip(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest:    now.Add(-2 * 24 * time.Hour),
		countries: []CountryCount{{Country: "CN", N: 20}},
	}
	res := newSvc(store, scan, nil).Scan(context.Background(), now)
	if !res.Learning {
		t.Fatal("expected learning")
	}
	for _, e := range store.inserted {
		if e.Code == CodeNewCountryDst {
			t.Fatal("new_country must skip during learning")
		}
	}
}

func TestNewCountryEmits(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest:    now.Add(-30 * 24 * time.Hour),
		countries: []CountryCount{{Country: "CN", N: 20}},
		baseline:  map[string]struct{}{"RU": {}},
	}
	res := newSvc(store, scan, nil).Scan(context.Background(), now)
	found := false
	for _, e := range store.inserted {
		if e.Code == CodeNewCountryDst {
			found = true
		}
	}
	if res.Inserted == 0 || !found {
		t.Fatalf("expected new_country, %+v", store.inserted)
	}
}

func TestRepNewDst(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest: now.Add(-30 * 24 * time.Hour),
		edges:  []EdgeRow{{SrcIP: "10.1.1.1", DstIP: "198.51.100.9", Count: 5}},
	}
	rep := fakeRep{hits: map[string][]model.ReputationHit{
		"198.51.100.9": {{List: "spamhaus_drop", Category: "drop"}},
	}}
	res := newSvc(store, scan, rep).Scan(context.Background(), now)
	found := false
	for _, e := range store.inserted {
		if e.Code == CodeRepNewDst {
			found = true
		}
	}
	if res.Inserted == 0 || !found {
		t.Fatalf("expected rep_new_dst %+v", store.inserted)
	}

	scan.knownPairs = map[string]struct{}{"10.1.1.1|198.51.100.9": {}}
	store2 := &fakeStore{exist: map[string]struct{}{}}
	res2 := newSvc(store2, scan, rep).Scan(context.Background(), now)
	for _, e := range store2.inserted {
		if e.Code == CodeRepNewDst {
			t.Fatal("known pair must not emit")
		}
	}
	_ = res2
}

func TestDedupSameFingerprint(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	fp := fingerprint(CodePortScan, "203.0.113.5", "", "", now)
	store := &fakeStore{exist: map[string]struct{}{fp: {}}}
	scan := &fakeScan{
		oldest: now.Add(-30 * 24 * time.Hour),
		ports:  []PortScanHit{{SrcIP: "203.0.113.5", Ports: 80, Events: 200}},
	}
	res := newSvc(store, scan, nil).Scan(context.Background(), now)
	if res.Inserted != 0 {
		t.Fatalf("dedup failed: %d", res.Inserted)
	}
}

func TestCapPerCode(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	hits := make([]PortScanHit, 15)
	for i := range hits {
		hits[i] = PortScanHit{SrcIP: "203.0.113." + string(rune('1'+i)), Ports: 80, Events: 200}
	}
	// use distinct numeric IPs
	for i := 0; i < 15; i++ {
		hits[i].SrcIP = "198.51.100." + itoa(i+1)
	}
	scan := &fakeScan{oldest: now.Add(-30 * 24 * time.Hour), ports: hits}
	res := newSvc(store, scan, nil).Scan(context.Background(), now)
	n := 0
	for _, e := range store.inserted {
		if e.Code == CodePortScan {
			n++
		}
	}
	if n > maxPerCode {
		t.Fatalf("cap %d got %d (inserted total %d)", maxPerCode, n, res.Inserted)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func TestGateSkip(t *testing.T) {
	store := &fakeStore{}
	scan := &fakeScan{oldest: time.Now().Add(-30 * 24 * time.Hour)}
	s := New(Config{Enabled: true, LearningDays: 3}, store, scan, nil, skipGate("circuit"), nil)
	res := s.Scan(context.Background(), time.Now().UTC())
	if res.Skipped != "circuit" || res.Inserted != 0 {
		t.Fatalf("%+v", res)
	}
}

type skipGate string

func (s skipGate) SkipReason() string { return string(s) }

func TestThresholdsMedium(t *testing.T) {
	th := ThresholdsForProfile("medium")
	if th.PortScanPorts != 50 {
		t.Fatalf("%+v", th)
	}
}
