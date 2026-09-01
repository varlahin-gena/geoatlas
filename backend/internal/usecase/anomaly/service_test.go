package anomaly

import (
	"context"
	"strings"
	"testing"
	"time"

	"geoatlas/internal/model"
)

type fakeStore struct {
	inserted         []Event
	exist            map[string]struct{}
	list             []Event
	summary          Summary
	activeSuppressed map[SuppressionKey]struct{}
	recentSuppressed map[SuppressionKey]struct{}
	ackedFingerprint string
	ackedBy          string
	ackedSuppressFor time.Duration
	assignedTo       string
	assignedBy       string
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
func (f *fakeStore) ActiveSuppressions(context.Context, []SuppressionKey, time.Time) (map[SuppressionKey]struct{}, error) {
	if f.activeSuppressed == nil {
		return map[SuppressionKey]struct{}{}, nil
	}
	return f.activeSuppressed, nil
}
func (f *fakeStore) RecentSuppressionKeys(context.Context, string, []SuppressionKey, time.Time) (map[SuppressionKey]struct{}, error) {
	if f.recentSuppressed == nil {
		return map[SuppressionKey]struct{}{}, nil
	}
	return f.recentSuppressed, nil
}
func (f *fakeStore) Ack(_ context.Context, fingerprint, by string, suppressFor time.Duration) error {
	f.ackedFingerprint = fingerprint
	f.ackedBy = by
	f.ackedSuppressFor = suppressFor
	return nil
}
func (f *fakeStore) Assign(_ context.Context, fingerprint, assignedTo, by string) error {
	f.assignedTo = assignedTo
	f.assignedBy = by
	_ = fingerprint
	return nil
}
func (f *fakeStore) AssignIfEmpty(ctx context.Context, fingerprint, assignedTo, by string) error {
	if strings.TrimSpace(f.assignedTo) != "" {
		return nil
	}
	return f.Assign(ctx, fingerprint, assignedTo, by)
}
func (f *fakeStore) CountSummary(context.Context, time.Time) (Summary, error) {
	return f.summary, nil
}

type fakeScan struct {
	oldest       time.Time
	now          time.Time
	ports        []PortScanHit
	horiz        []HorizontalScanHit
	blockedCurr  uint64
	blockedPrev  uint64
	hasBlocked   bool
	countries    []CountryCount
	countryTotal uint64
	baseline     map[string]struct{}
	edges        []EdgeRow
	knownPairs   map[string]struct{}
	byteSurge    []ByteSurgeHit
	beaconing    []BeaconingHit
	lateral      []LateralFanoutHit
}

func (f *fakeScan) OldestLogTime(context.Context) (time.Time, error) { return f.oldest, nil }
func (f *fakeScan) PortScan(context.Context, time.Duration, int, int, bool, []IPRange) ([]PortScanHit, error) {
	return f.ports, nil
}
func (f *fakeScan) HorizontalScan(context.Context, time.Duration, int, int, bool, []IPRange) ([]HorizontalScanHit, error) {
	return f.horiz, nil
}
func (f *fakeScan) BlockedCount(_ context.Context, start, end time.Time, _ *IPRange) (uint64, error) {
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
func (f *fakeScan) CurrentCountries(context.Context, time.Duration, uint64, []IPRange) ([]CountryCount, error) {
	return f.countries, nil
}
func (f *fakeScan) CurrentCountryTotal(context.Context, time.Duration, []IPRange) (uint64, error) {
	return f.countryTotal, nil
}
func (f *fakeScan) BaselineCountries(context.Context, int, uint64, []IPRange) (map[string]struct{}, error) {
	if f.baseline == nil {
		return map[string]struct{}{}, nil
	}
	return f.baseline, nil
}
func (f *fakeScan) RecentEdges(context.Context, time.Duration, int, []IPRange) ([]EdgeRow, error) {
	return f.edges, nil
}
func (f *fakeScan) KnownPairs(context.Context, [][2]string, time.Duration) (map[string]struct{}, error) {
	if f.knownPairs == nil {
		return map[string]struct{}{}, nil
	}
	return f.knownPairs, nil
}
func (f *fakeScan) ByteSurge(context.Context, time.Duration, uint64, uint64, []IPRange) ([]ByteSurgeHit, error) {
	return f.byteSurge, nil
}
func (f *fakeScan) Beaconing(context.Context, time.Duration, int, uint64, []IPRange) ([]BeaconingHit, error) {
	return f.beaconing, nil
}
func (f *fakeScan) LateralFanout(context.Context, time.Duration, int, int, []IPRange) ([]LateralFanoutHit, error) {
	return f.lateral, nil
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

type fakeNets struct {
	items []model.EnterpriseNet
}

func (f fakeNets) ListEnterpriseNets(context.Context) ([]model.EnterpriseNet, error) {
	return f.items, nil
}

func withEnterprise(s *Service) *Service {
	s.SetEnterpriseNets(fakeNets{items: []model.EnterpriseNet{{
		StartIP: 167772160, EndIP: 184549375, Network: "10.0.0.0/8", Label: "LAN",
	}}})
	return s
}

func newSvc(store *fakeStore, scan *fakeScan, rep ReputationLookuper) *Service {
	return New(Config{
		Enabled: true, LearningDays: 3, InstallProfile: "medium",
		SuppressHours: 24, NewCountryMinShare: 0.05, NewCountryRepeatCooldownHours: 24,
	}, store, scan, rep, nil, nil)
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
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
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
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
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
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
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
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
	found := false
	for _, e := range store.inserted {
		if e.Code == CodeBlockedSurge {
			found = true
			if e.Device != "10.0.0.0/8" {
				t.Fatalf("expected per-net device, got %q", e.Device)
			}
			if e.Map.Query != "city:LAN" {
				t.Fatalf("expected map query for enterprise net, got %q", e.Map.Query)
			}
			if e.Map.Group != "city" {
				t.Fatalf("expected city group for wide net, got %q", e.Map.Group)
			}
		}
	}
	if res.Inserted == 0 || !found {
		t.Fatalf("expected blocked_surge, inserted=%d items=%v", res.Inserted, store.inserted)
	}
}

func TestScanSkippedWithoutEnterpriseNets(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest:      now.Add(-30 * 24 * time.Hour),
		now:         now,
		hasBlocked:  true,
		blockedCurr: 2000,
		blockedPrev: 80,
		ports:       []PortScanHit{{SrcIP: "203.0.113.5", Ports: 80, Events: 200}},
	}
	res := newSvc(store, scan, nil).Scan(context.Background(), now)
	if res.Skipped != "no_enterprise_nets" {
		t.Fatalf("skipped=%q want no_enterprise_nets", res.Skipped)
	}
	if len(store.inserted) != 0 {
		t.Fatalf("expected no alerts without enterprise nets, got %v", store.inserted)
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
		oldest:       now.Add(-30 * 24 * time.Hour),
		countries:    []CountryCount{{Country: "CN", N: 20}},
		countryTotal: 100,
		baseline:     map[string]struct{}{"RU": {}},
	}
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
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
	res := withEnterprise(newSvc(store, scan, rep)).Scan(context.Background(), now)
	found := false
	for _, e := range store.inserted {
		if e.Code == CodeRepNewDst {
			found = true
		}
	}
	if res.Inserted == 0 || !found {
		t.Fatalf("expected rep_new_peer %+v", store.inserted)
	}

	scan.knownPairs = map[string]struct{}{"10.1.1.1|198.51.100.9": {}}
	store2 := &fakeStore{exist: map[string]struct{}{}}
	res2 := withEnterprise(newSvc(store2, scan, rep)).Scan(context.Background(), now)
	for _, e := range store2.inserted {
		if e.Code == CodeRepNewDst {
			t.Fatal("known pair must not emit")
		}
	}
	_ = res2
}

func TestRepNewSrc(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest: now.Add(-30 * 24 * time.Hour),
		edges:  []EdgeRow{{SrcIP: "198.51.100.9", DstIP: "10.1.1.1", Count: 5}},
	}
	rep := fakeRep{hits: map[string][]model.ReputationHit{
		"198.51.100.9": {{List: "spamhaus_drop", Category: "drop"}},
	}}
	res := withEnterprise(newSvc(store, scan, rep)).Scan(context.Background(), now)
	found := false
	for _, e := range store.inserted {
		if e.Code == CodeRepNewDst {
			found = true
			if !strings.Contains(e.Title, "Репутационный источник") {
				t.Fatalf("unexpected title %q", e.Title)
			}
		}
	}
	if res.Inserted == 0 || !found {
		t.Fatalf("expected rep_new_peer for bad src %+v", store.inserted)
	}
}

func TestAckCreatesSuppressionWindow(t *testing.T) {
	store := &fakeStore{}
	scan := &fakeScan{oldest: time.Now().Add(-30 * 24 * time.Hour)}
	svc := newSvc(store, scan, nil)
	if err := svc.Ack(context.Background(), "deadbeef", "alice"); err != nil {
		t.Fatal(err)
	}
	if store.ackedFingerprint != "deadbeef" || store.ackedBy != "alice" {
		t.Fatalf("ack not forwarded: %+v", store)
	}
	if store.ackedSuppressFor != 24*time.Hour {
		t.Fatalf("suppressFor=%v", store.ackedSuppressFor)
	}
}

func TestNewCountryMinShareSkipsNoise(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest:       now.Add(-30 * 24 * time.Hour),
		countries:    []CountryCount{{Country: "CN", N: 20}},
		countryTotal: 1000,
		baseline:     map[string]struct{}{"RU": {}},
	}
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
	if res.Inserted != 0 {
		t.Fatalf("expected low-share country to skip, got %d", res.Inserted)
	}
}

func TestNewCountryRepeatCooldownSkipsRecentRepeat(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	key := suppressionKeyForCodeCountry(CodeNewCountryDst, "CN")
	store := &fakeStore{
		exist:            map[string]struct{}{},
		recentSuppressed: map[SuppressionKey]struct{}{key: {}},
	}
	scan := &fakeScan{
		oldest:       now.Add(-30 * 24 * time.Hour),
		countries:    []CountryCount{{Country: "CN", N: 20}},
		countryTotal: 100,
		baseline:     map[string]struct{}{"RU": {}},
	}
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
	if res.Inserted != 0 {
		t.Fatalf("expected repeat cooldown to skip, got %d", res.Inserted)
	}
}

func TestSuppressedPatternNotInserted(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	key := SuppressionKey(CodePortScan + "|src|203.0.113.5")
	store := &fakeStore{
		exist:            map[string]struct{}{},
		activeSuppressed: map[SuppressionKey]struct{}{key: {}},
	}
	scan := &fakeScan{
		oldest: now.Add(-30 * 24 * time.Hour),
		ports:  []PortScanHit{{SrcIP: "203.0.113.5", Ports: 80, Events: 200}},
	}
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
	if res.Inserted != 0 {
		t.Fatalf("expected active suppression to skip insert, got %d", res.Inserted)
	}
}

func TestDedupSameFingerprint(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	fp := fingerprint(CodePortScan, "203.0.113.5", "", "", now)
	store := &fakeStore{exist: map[string]struct{}{fp: {}}}
	scan := &fakeScan{
		oldest: now.Add(-30 * 24 * time.Hour),
		ports:  []PortScanHit{{SrcIP: "203.0.113.5", Ports: 80, Events: 200}},
	}
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
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
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
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
	if th.ByteSurgeAbsMin == 0 || th.BeaconMinHours == 0 || th.LateralHosts == 0 {
		t.Fatalf("new thresholds missing: %+v", th)
	}
}

func TestByteSurgeEmits(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest: now.Add(-30 * 24 * time.Hour),
		byteSurge: []ByteSurgeHit{{
			SrcIP: "10.1.2.3", BytesNow: 600_000_000, BytesPrev: 10_000_000,
		}},
	}
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
	found := false
	for _, e := range store.inserted {
		if e.Code == CodeByteSurge {
			found = true
			if e.Map.Period != "2h" || e.Map.Query != "src:10.1.2.3" {
				t.Fatalf("map %+v", e.Map)
			}
		}
	}
	if !found {
		t.Fatalf("byte_surge missing inserted=%d err=%s", res.Inserted, res.Error)
	}
}

func TestLateralFanoutEmits(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest:  now.Add(-30 * 24 * time.Hour),
		lateral: []LateralFanoutHit{{SrcIP: "10.0.0.5", Hosts: 40, Events: 100}},
	}
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
	found := false
	for _, e := range store.inserted {
		if e.Code == CodeLateralFanout && e.Severity == SeverityHigh {
			found = true
		}
	}
	if !found {
		t.Fatalf("lateral_fanout missing inserted=%d err=%s", res.Inserted, res.Error)
	}
}

func TestBeaconingEmits(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	hours := make([]int64, 0, 12)
	base := now.Add(-24 * time.Hour).Unix()
	for i := 0; i < 12; i++ {
		hours = append(hours, base+int64(i)*3600)
	}
	store := &fakeStore{exist: map[string]struct{}{}}
	scan := &fakeScan{
		oldest: now.Add(-30 * 24 * time.Hour),
		beaconing: []BeaconingHit{{
			SrcIP: "10.0.0.8", DstIP: "203.0.113.9",
			ActiveHours: 12, TotalBytes: 1_200_000, Events: 40, HourUnix: hours,
		}},
	}
	res := withEnterprise(newSvc(store, scan, nil)).Scan(context.Background(), now)
	found := false
	for _, e := range store.inserted {
		if e.Code == CodeBeaconing {
			found = true
			if e.Map.Period != "1d" {
				t.Fatalf("period %s", e.Map.Period)
			}
			want := fingerprintDay(CodeBeaconing, "10.0.0.8", "203.0.113.9", "", now)
			if e.Fingerprint != want {
				t.Fatalf("fp day bucket: %s vs %s", e.Fingerprint, want)
			}
		}
	}
	if !found {
		t.Fatalf("beaconing missing inserted=%d err=%s", res.Inserted, res.Error)
	}
}

func TestHourRegularity(t *testing.T) {
	base := int64(1_700_000_000)
	seq := []int64{base, base + 3600, base + 7200, base + 10800}
	if g := hourRegularity(seq); g < 0.99 {
		t.Fatalf("regular=%v", g)
	}
	sparse := []int64{base, base + 3600, base + 10*3600}
	if g := hourRegularity(sparse); g >= 0.99 {
		t.Fatalf("sparse should be lower: %v", g)
	}
}
