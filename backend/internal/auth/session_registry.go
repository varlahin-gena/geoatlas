package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"geoatlas/internal/fileatomic"
)

// SessionRecord — одна активная cookie-сессия в реестре.
type SessionRecord struct {
	ID         string    `json:"id"`
	Username   string    `json:"username"`
	Role       string    `json:"role"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type sessionFile struct {
	Sessions []SessionRecord `json:"sessions"`
}

// SessionRegistry — file-backed реестр sid → SessionRecord.
type SessionRegistry struct {
	mu        sync.Mutex
	path      string
	byID      map[string]SessionRecord
	lastTouch map[string]time.Time
	touchMin  time.Duration
	now       func() time.Time
}

// NewSessionRegistry loads sessions.json (missing file → empty).
func NewSessionRegistry(path string) (*SessionRegistry, error) {
	r := &SessionRegistry{
		path:      strings.TrimSpace(path),
		byID:      make(map[string]SessionRecord),
		lastTouch: make(map[string]time.Time),
		touchMin:  time.Minute,
		now:       func() time.Time { return time.Now().UTC() },
	}
	if r.path == "" {
		return r, nil
	}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *SessionRegistry) load() error {
	data, err := fileatomic.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var f sessionFile
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	now := r.now()
	for _, rec := range f.Sessions {
		if strings.TrimSpace(rec.ID) == "" {
			continue
		}
		if !rec.ExpiresAt.IsZero() && rec.ExpiresAt.Before(now) {
			continue
		}
		r.byID[rec.ID] = rec
	}
	return nil
}

func (r *SessionRegistry) persistLocked() error {
	if r.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	items := make([]SessionRecord, 0, len(r.byID))
	for _, rec := range r.byID {
		items = append(items, rec)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	raw, err := json.MarshalIndent(sessionFile{Sessions: items}, "", "  ")
	if err != nil {
		return err
	}
	return fileatomic.WriteFile(r.path, raw, 0o600)
}

// Register adds or replaces a session record.
func (r *SessionRegistry) Register(rec SessionRecord) error {
	if r == nil || strings.TrimSpace(rec.ID) == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.LastSeenAt.IsZero() {
		rec.LastSeenAt = now
	}
	r.pruneExpiredLocked(now)
	r.byID[rec.ID] = rec
	return r.persistLocked()
}

// Has reports whether sid is still active (not revoked / not expired).
func (r *SessionRegistry) Has(sid string) bool {
	if r == nil {
		return false
	}
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneExpiredLocked(r.now())
	_, ok := r.byID[sid]
	return ok
}

// Touch updates last_seen_at at most once per touchMin.
func (r *SessionRegistry) Touch(sid string) {
	if r == nil {
		return
	}
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.byID[sid]
	if !ok {
		return
	}
	now := r.now()
	if last, ok := r.lastTouch[sid]; ok && now.Sub(last) < r.touchMin {
		return
	}
	rec.LastSeenAt = now
	r.byID[sid] = rec
	r.lastTouch[sid] = now
	_ = r.persistLocked()
}

// Revoke removes one session by id. Returns true if it existed.
func (r *SessionRegistry) Revoke(sid string) (bool, error) {
	if r == nil {
		return false, nil
	}
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return false, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[sid]; !ok {
		return false, nil
	}
	delete(r.byID, sid)
	delete(r.lastTouch, sid)
	return true, r.persistLocked()
}

// RevokeUser removes all sessions for username.
func (r *SessionRegistry) RevokeUser(username string) (int, error) {
	if r == nil {
		return 0, nil
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return 0, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for id, rec := range r.byID {
		if !strings.EqualFold(rec.Username, username) {
			continue
		}
		delete(r.byID, id)
		delete(r.lastTouch, id)
		n++
	}
	if n == 0 {
		return 0, nil
	}
	return n, r.persistLocked()
}

// List returns active sessions (expired pruned), newest first.
func (r *SessionRegistry) List() []SessionRecord {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneExpiredLocked(r.now())
	out := make([]SessionRecord, 0, len(r.byID))
	for _, rec := range r.byID {
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastSeenAt.After(out[j].LastSeenAt)
	})
	return out
}

func (r *SessionRegistry) pruneExpiredLocked(now time.Time) {
	for id, rec := range r.byID {
		if !rec.ExpiresAt.IsZero() && rec.ExpiresAt.Before(now) {
			delete(r.byID, id)
			delete(r.lastTouch, id)
		}
	}
}

// ExpiresFromSession maps cookie Expires unix to time.Time (zero = never).
func ExpiresFromSession(sess Session) time.Time {
	if sess.Expires <= 0 || sess.Expires == SessionNeverExpires {
		return time.Time{}
	}
	return time.Unix(sess.Expires, 0).UTC()
}
