package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	CookieName     = "ga_session"
	CSRFCookieName = "ga_csrf"
	CSRFHeaderName = "X-CSRF-Token"
)

var (
	ErrInvalidSession = errors.New("invalid session")
	ErrExpiredSession = errors.New("session expired")
)

// SessionNeverExpires — Expires=-1 в токене: сессия не протухает по TTL (только revoke).
const SessionNeverExpires int64 = -1

// PersistentCookieTTL — Max-Age cookie для роли dashboard (~10 лет).
const PersistentCookieTTL = 10 * 365 * 24 * time.Hour

// Session — содержимое cookie после успешного логина.
type Session struct {
	Username       string `json:"u"`
	Role           string `json:"r"`
	Expires        int64  `json:"e"`             // unix seconds
	SessionVersion int64  `json:"sv"`            // must match User.SessionVersion (revoke stamp)
	SID            string `json:"sid,omitempty"` // registry id for per-device revoke
}

// SessionManager подписывает и проверяет cookie сессии.
type SessionManager struct {
	secret []byte
	ttl    time.Duration
}

func NewSessionManager(secret string, ttl time.Duration) (*SessionManager, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("session secret is empty")
	}
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &SessionManager{secret: []byte(secret), ttl: ttl}, nil
}

func (m *SessionManager) TTL() time.Duration { return m.ttl }

// CookieTTLForRole — TTL для Set-Cookie: dashboard получает длительную сессию.
func CookieTTLForRole(role string, defaultTTL time.Duration) time.Duration {
	if HasPersistentSession(role) {
		return PersistentCookieTTL
	}
	return defaultTTL
}

func (m *SessionManager) Issue(username, role string, sessionVersion int64) (string, Session, error) {
	if m == nil {
		return "", Session{}, ErrInvalidSession
	}
	expires := time.Now().Add(m.ttl).Unix()
	if HasPersistentSession(role) {
		expires = SessionNeverExpires
	}
	sess := Session{
		Username:       username,
		Role:           role,
		Expires:        expires,
		SessionVersion: sessionVersion,
		SID:            newSessionID(),
	}
	raw, err := json.Marshal(sess)
	if err != nil {
		return "", Session{}, err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	sig := m.sign(payload)
	return payload + "." + sig, sess, nil
}

func (m *SessionManager) Parse(token string) (Session, error) {
	if m == nil {
		return Session{}, ErrInvalidSession
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Session{}, ErrInvalidSession
	}
	if !hmac.Equal([]byte(parts[1]), []byte(m.sign(parts[0]))) {
		return Session{}, ErrInvalidSession
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Session{}, ErrInvalidSession
	}
	var sess Session
	if err := json.Unmarshal(raw, &sess); err != nil {
		return Session{}, ErrInvalidSession
	}
	if sess.Username == "" || !ValidRole(sess.Role) {
		return Session{}, ErrInvalidSession
	}
	if sess.Expires > 0 && time.Now().Unix() > sess.Expires {
		return Session{}, ErrExpiredSession
	}
	return sess, nil
}

// UserLookup supplies live session checks against the user store.
type UserLookup interface {
	Get(username string) (UserPublic, bool)
	// SessionVersion — stamp for revoke; missing user → ok=false.
	SessionVersion(username string) (version int64, ok bool)
}

// SessionPresence — реестр активных cookie-сессий (per-device revoke).
type SessionPresence interface {
	Has(sid string) bool
	Touch(sid string)
}

// LiveSession подставляет актуальную роль и проверяет session_version.
// Пользователь удалён / stamp не совпал → ok=false (stolen cookie после revoke).
// users == nil — оставляет cookie как есть (тесты / AUTH_DISABLED path).
func LiveSession(users UserLookup, sess Session) (Session, bool) {
	if users == nil {
		return sess, true
	}
	pub, ok := users.Get(sess.Username)
	if !ok {
		return Session{}, false
	}
	if !pub.Enabled {
		return Session{}, false
	}
	sv, ok := users.SessionVersion(sess.Username)
	if !ok || sess.SessionVersion != sv {
		return Session{}, false
	}
	sess.Role = pub.Role
	sess.SessionVersion = sv
	if sess.Expires == SessionNeverExpires && !HasPersistentSession(sess.Role) {
		return Session{}, false
	}
	return sess, true
}

// LiveSessionChecked = LiveSession + активный sid в реестре.
// registry == nil (typed) — проверка sid пропускается (unit-тесты / smoke без store).
func LiveSessionChecked(users UserLookup, registry *SessionRegistry, sess Session) (Session, bool) {
	live, ok := LiveSession(users, sess)
	if !ok {
		return Session{}, false
	}
	if registry == nil {
		return live, true
	}
	if strings.TrimSpace(sess.SID) == "" || !registry.Has(sess.SID) {
		return Session{}, false
	}
	registry.Touch(sess.SID)
	return live, true
}

func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	}
	return hex.EncodeToString(b[:])
}

func (m *SessionManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
