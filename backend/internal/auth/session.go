package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

// Session — содержимое cookie после успешного логина.
type Session struct {
	Username       string `json:"u"`
	Role           string `json:"r"`
	Expires        int64  `json:"e"` // unix seconds
	SessionVersion int64  `json:"sv"` // must match User.SessionVersion (revoke stamp)
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

func (m *SessionManager) Issue(username, role string, sessionVersion int64) (string, Session, error) {
	if m == nil {
		return "", Session{}, ErrInvalidSession
	}
	sess := Session{
		Username:       username,
		Role:           role,
		Expires:        time.Now().Add(m.ttl).Unix(),
		SessionVersion: sessionVersion,
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
	if time.Now().Unix() > sess.Expires {
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
	sv, ok := users.SessionVersion(sess.Username)
	if !ok || sess.SessionVersion != sv {
		return Session{}, false
	}
	sess.Role = pub.Role
	sess.SessionVersion = sv
	return sess, true
}

func (m *SessionManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
