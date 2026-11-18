package parsers

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"geoatlas/internal/apperr"
	"geoatlas/internal/parser"
	"geoatlas/internal/usecase/parsetest"
)

// ListItem — правило или built-in stub для GUI.
type ListItem struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Vendor   string           `json:"vendor"`
	Enabled  bool             `json:"enabled"`
	Priority int              `json:"priority"`
	Seed     bool             `json:"seed,omitempty"`
	Source   string           `json:"source"` // db | builtin
	Editable bool             `json:"editable"`
	Kind     parser.RuleKind  `json:"kind,omitempty"`
	Spec     *parser.RuleSpec `json:"spec,omitempty"`
}

// Service — CRUD + seed + dry-run для parser rules.
type Service struct {
	store  Store
	reload RegistryReloader
}

func New(store Store, reload RegistryReloader) *Service {
	return &Service{store: store, reload: reload}
}

// EnsureSeed inserts missing default CEF/KV rules; does not overwrite existing.
func (s *Service) EnsureSeed(ctx context.Context) error {
	if s == nil || s.store == nil {
		return nil
	}
	existing, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	have := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		have[r.ID] = struct{}{}
	}
	changed := false
	for _, seed := range parser.DefaultSeedRules() {
		if _, ok := have[seed.ID]; ok {
			continue
		}
		if err := s.store.Upsert(ctx, seed); err != nil {
			return fmt.Errorf("seed %q: %w", seed.ID, err)
		}
		changed = true
	}
	if changed && s.reload != nil {
		return s.reload.Reload(ctx)
	}
	return nil
}

// List returns DB rules + built-in Cisco stubs, sorted by priority.
func (s *Service) List(ctx context.Context) ([]ListItem, error) {
	if s == nil || s.store == nil {
		return nil, apperr.InvalidInput("parsers store unavailable")
	}
	rules, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ListItem, 0, len(rules)+2)
	for _, r := range rules {
		spec := r.Spec
		out = append(out, ListItem{
			ID: r.ID, Name: r.Name, Vendor: r.Vendor,
			Enabled: r.Enabled, Priority: r.Priority, Seed: r.Seed,
			Source: "db", Editable: true, Kind: r.Spec.Kind, Spec: &spec,
		})
	}
	pri := parser.BuiltinPriorities()
	out = append(out,
		ListItem{
			ID: "cisco-ftd", Name: "Cisco FTD", Vendor: "cisco-ftd",
			Enabled: true, Priority: pri["cisco-ftd"],
			Source: "builtin", Editable: false,
		},
		ListItem{
			ID: "cisco-asa", Name: "Cisco ASA", Vendor: "cisco-asa",
			Enabled: true, Priority: pri["cisco-asa"],
			Source: "builtin", Editable: false,
		},
	)
	// Sort by priority then id (stable for UI).
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Priority < out[i].Priority ||
				(out[j].Priority == out[i].Priority && out[j].ID < out[i].ID) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id string) (ListItem, error) {
	id = strings.TrimSpace(id)
	items, err := s.List(ctx)
	if err != nil {
		return ListItem{}, err
	}
	for _, it := range items {
		if it.ID == id {
			return it, nil
		}
	}
	return ListItem{}, apperr.NotFound("parser " + id + " not found")
}

type CreateInput struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Vendor   string          `json:"vendor"`
	Enabled  *bool           `json:"enabled"`
	Priority int             `json:"priority"`
	Spec     parser.RuleSpec `json:"spec"`
}

func (s *Service) Create(ctx context.Context, in CreateInput) (parser.RuleRecord, error) {
	id := normalizeID(in.ID)
	if id == "" {
		return parser.RuleRecord{}, apperr.InvalidInput("id is required")
	}
	if isBuiltinID(id) {
		return parser.RuleRecord{}, apperr.Conflict("id reserved for built-in parser")
	}
	exists, err := s.store.Exists(ctx, id)
	if err != nil {
		return parser.RuleRecord{}, err
	}
	if exists {
		return parser.RuleRecord{}, apperr.Conflict("parser " + id + " already exists")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	priority := in.Priority
	if priority == 0 {
		priority = 250
	}
	rec := parser.RuleRecord{
		ID: id, Name: strings.TrimSpace(in.Name), Vendor: strings.TrimSpace(in.Vendor),
		Enabled: enabled, Priority: priority, Seed: false, Spec: in.Spec,
	}
	if rec.Name == "" {
		rec.Name = rec.Vendor
	}
	if rec.Vendor == "" {
		rec.Vendor = id
	}
	if err := rec.Validate(); err != nil {
		return parser.RuleRecord{}, apperr.InvalidInput(err.Error())
	}
	if err := s.store.Upsert(ctx, rec); err != nil {
		return parser.RuleRecord{}, err
	}
	if err := s.reloadNow(ctx); err != nil {
		return parser.RuleRecord{}, err
	}
	return rec, nil
}

type UpdateInput struct {
	Name     *string          `json:"name"`
	Vendor   *string          `json:"vendor"`
	Enabled  *bool            `json:"enabled"`
	Priority *int             `json:"priority"`
	Spec     *parser.RuleSpec `json:"spec"`
}

func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (parser.RuleRecord, error) {
	id = normalizeID(id)
	if isBuiltinID(id) {
		return parser.RuleRecord{}, apperr.InvalidInput("built-in parsers cannot be edited")
	}
	cur, err := s.findDB(ctx, id)
	if err != nil {
		return parser.RuleRecord{}, err
	}
	if in.Name != nil {
		cur.Name = strings.TrimSpace(*in.Name)
	}
	if in.Vendor != nil {
		cur.Vendor = strings.TrimSpace(*in.Vendor)
	}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
	}
	if in.Priority != nil {
		cur.Priority = *in.Priority
	}
	if in.Spec != nil {
		cur.Spec = *in.Spec
	}
	if err := cur.Validate(); err != nil {
		return parser.RuleRecord{}, apperr.InvalidInput(err.Error())
	}
	if err := s.store.Upsert(ctx, cur); err != nil {
		return parser.RuleRecord{}, err
	}
	if err := s.reloadNow(ctx); err != nil {
		return parser.RuleRecord{}, err
	}
	return cur, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	id = normalizeID(id)
	if isBuiltinID(id) {
		return apperr.InvalidInput("built-in parsers cannot be deleted")
	}
	if _, err := s.findDB(ctx, id); err != nil {
		return err
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	return s.reloadNow(ctx)
}

type OrderEntry struct {
	ID       string `json:"id"`
	Priority int    `json:"priority"`
}

func (s *Service) Reorder(ctx context.Context, entries []OrderEntry) error {
	if len(entries) == 0 {
		return apperr.InvalidInput("order is empty")
	}
	for _, e := range entries {
		id := normalizeID(e.ID)
		if isBuiltinID(id) {
			continue // built-in priorities fixed
		}
		cur, err := s.findDB(ctx, id)
		if err != nil {
			return err
		}
		cur.Priority = e.Priority
		if err := s.store.Upsert(ctx, cur); err != nil {
			return err
		}
	}
	return s.reloadNow(ctx)
}

// DryRunInput — прогон строк через текущий registry или черновик spec.
type DryRunInput struct {
	Text   string           `json:"text"`
	RuleID string           `json:"rule_id,omitempty"`
	Spec   *parser.RuleSpec `json:"spec,omitempty"`
	Vendor string           `json:"vendor,omitempty"`
	Name   string           `json:"name,omitempty"`
}

func (s *Service) DryRun(ctx context.Context, in DryRunInput, live parser.ParseVerboseFunc) (parsetest.Result, error) {
	_ = ctx
	text := in.Text
	if strings.TrimSpace(text) == "" {
		return parsetest.Result{}, apperr.InvalidInput("text is required")
	}

	runner := live
	if in.Spec != nil {
		rec := parser.RuleRecord{
			ID: "dry-run", Name: firstNonEmptyStr(in.Name, "dry-run"),
			Vendor:  firstNonEmptyStr(in.Vendor, "dry-run"),
			Enabled: true, Priority: 0, Spec: *in.Spec,
		}
		rp, err := parser.NewRuleParser(rec)
		if err != nil {
			return parsetest.Result{}, apperr.InvalidInput(err.Error())
		}
		reg := parser.NewRegistry(rp)
		runner = reg.ParseVerbose
	} else if id := normalizeID(in.RuleID); id != "" && !isBuiltinID(id) {
		cur, err := s.findDB(ctx, id)
		if err != nil {
			return parsetest.Result{}, err
		}
		rp, err := parser.NewRuleParser(cur)
		if err != nil {
			return parsetest.Result{}, apperr.InvalidInput(err.Error())
		}
		reg := parser.NewRegistry(rp)
		runner = reg.ParseVerbose
	}
	if runner == nil {
		return parsetest.Result{}, apperr.InvalidInput("no parser available for dry-run")
	}

	svc := parsetest.New(parseVerboseAdapter{fn: runner}, nil, nil)
	return svc.Run(strings.NewReader(text))
}

type parseVerboseAdapter struct {
	fn parser.ParseVerboseFunc
}

func (a parseVerboseAdapter) ParseVerbose(line string) parser.ParseResult {
	return a.fn(line)
}

func (s *Service) findDB(ctx context.Context, id string) (parser.RuleRecord, error) {
	rules, err := s.store.List(ctx)
	if err != nil {
		return parser.RuleRecord{}, err
	}
	for _, r := range rules {
		if r.ID == id {
			return r, nil
		}
	}
	return parser.RuleRecord{}, apperr.NotFound("parser " + id + " not found")
}

func (s *Service) reloadNow(ctx context.Context) error {
	if s.reload == nil {
		return nil
	}
	return s.reload.Reload(ctx)
}

func isBuiltinID(id string) bool {
	return id == "cisco-asa" || id == "cisco-ftd"
}

func normalizeID(id string) string {
	id = strings.TrimSpace(strings.ToLower(id))
	var b strings.Builder
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
