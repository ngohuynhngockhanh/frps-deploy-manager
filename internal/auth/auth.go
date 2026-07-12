// Package auth provides local UI authentication: argon2id password checks,
// in-memory sessions, CSRF tokens and simple login rate limiting. The tool is
// single-operator and loopback-only, so an in-memory session store is fine.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/crypto"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

const (
	// CookieName is the session cookie.
	CookieName = "fdm_session"
	// sessionTTL is the sliding session lifetime (persisted, survives restart).
	sessionTTL = 7 * 24 * time.Hour
	// lockout parameters for login attempts (per username).
	maxAttempts   = 8
	attemptWindow = 10 * time.Minute
)

// ErrInvalidCredentials is returned on a bad username/password.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrRateLimited is returned when too many failed logins occur.
var ErrRateLimited = errors.New("too many attempts, try again later")

type session struct {
	userID   int64
	username string
	role     string
	csrf     string
	expires  time.Time
}

type attempt struct {
	count int
	first time.Time
}

// Manager holds sessions and login state.
type Manager struct {
	store *store.Store

	mu       sync.Mutex
	sessions map[string]*session
	attempts map[string]*attempt
}

// NewManager creates an auth manager backed by the store.
func NewManager(st *store.Store) *Manager {
	return &Manager{
		store:    st,
		sessions: make(map[string]*session),
		attempts: make(map[string]*attempt),
	}
}

// SeedAdmin creates the default admin account if no operators exist.
func (m *Manager) SeedAdmin(username, password string) error {
	n, err := m.store.CountAppUsers()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = m.store.CreateAppUser(username, hash, store.RoleAdmin)
	return err
}

// Login verifies credentials and creates a session, returning the session token
// and CSRF token. It enforces a per-username attempt limit.
func (m *Manager) Login(username, password string) (token, csrf string, err error) {
	if m.rateLimited(username) {
		return "", "", ErrRateLimited
	}
	u, err := m.store.GetAppUserByUsername(username)
	if err != nil {
		m.recordFailure(username)
		return "", "", ErrInvalidCredentials
	}
	ok, err := crypto.VerifyPassword(password, u.PwHash)
	if err != nil || !ok {
		m.recordFailure(username)
		return "", "", ErrInvalidCredentials
	}
	m.clearAttempts(username)

	token = randomToken()
	csrf = randomToken()
	exp := time.Now().Add(sessionTTL)
	m.mu.Lock()
	m.sessions[token] = &session{userID: u.ID, username: username, role: u.Role, csrf: csrf, expires: exp}
	m.mu.Unlock()
	// Persist so the login survives a restart.
	_ = m.store.SaveSession(store.Session{Token: token, Username: username, CSRF: csrf, ExpiresAt: exp.Unix()})
	return token, csrf, nil
}

// Logout invalidates a session token.
func (m *Manager) Logout(token string) {
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
	_ = m.store.DeleteSession(token)
}

// lookup returns the session for a token, sliding its expiry, or nil. On a
// cold cache (e.g. after a restart) it falls back to the persisted session.
func (m *Manager) lookup(token string) *session {
	if token == "" {
		return nil
	}
	m.mu.Lock()
	s := m.sessions[token]
	m.mu.Unlock()

	if s == nil {
		// Cache miss — try the persisted store (survives restart).
		ps, err := m.store.GetSession(token)
		if err != nil {
			return nil
		}
		s = &session{username: ps.Username, csrf: ps.CSRF, expires: time.Unix(ps.ExpiresAt, 0)}
		// Re-hydrate role/id (not stored in the sessions table).
		if au, err := m.store.GetAppUserByUsername(ps.Username); err == nil {
			s.userID = au.ID
			s.role = au.Role
		}
		m.mu.Lock()
		m.sessions[token] = s
		m.mu.Unlock()
	}

	if time.Now().After(s.expires) {
		m.mu.Lock()
		delete(m.sessions, token)
		m.mu.Unlock()
		_ = m.store.DeleteSession(token)
		return nil
	}
	s.expires = time.Now().Add(sessionTTL)
	return s
}

// --- request helpers ---

type ctxKey int

const userKey ctxKey = 0

// Middleware requires a valid session; otherwise redirects browsers to /login
// and returns 401 for /api requests.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie(CookieName)
		var tok string
		if c != nil {
			tok = c.Value
		}
		s := m.lookup(tok)
		if s == nil {
			if isAPI(r) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			return
		}
		// CSRF for mutating requests.
		if isMutating(r.Method) && !m.checkCSRF(r, s.csrf) {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(withUser(r, s)))
	})
}

// SetSessionCookie writes the session cookie (loopback HTTP, so no Secure flag).
func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

// ClearSessionCookie expires the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: -1,
	})
}

// SessionUser returns the authenticated username from the request's session
// cookie (validating + sliding it), or "". Used by the dashboard proxy, which
// shares the session cookie (cookies are not port-scoped).
func (m *Manager) SessionUser(r *http.Request) string {
	s := m.lookup(CookieValue(r))
	if s == nil {
		return ""
	}
	return s.username
}

// CurrentUser returns the authenticated username, or "".
func CurrentUser(r *http.Request) string {
	v, _ := r.Context().Value(userKey).(userInfo)
	return v.name
}

// CSRFToken returns the CSRF token for the current session, or "".
func CSRFToken(r *http.Request) string {
	v, _ := r.Context().Value(userKey).(userInfo)
	return v.csrf
}

// CurrentUserID returns the authenticated operator's id, or 0.
func CurrentUserID(r *http.Request) int64 {
	v, _ := r.Context().Value(userKey).(userInfo)
	return v.id
}

// CurrentRole returns the authenticated operator's role, or "".
func CurrentRole(r *http.Request) string {
	v, _ := r.Context().Value(userKey).(userInfo)
	return v.role
}

// IsAdmin reports whether the current operator is an admin.
func IsAdmin(r *http.Request) bool {
	return CurrentRole(r) == "admin"
}

// CookieValue returns the raw session token from the request cookie.
func CookieValue(r *http.Request) string {
	c, _ := r.Cookie(CookieName)
	if c == nil {
		return ""
	}
	return c.Value
}

func (m *Manager) checkCSRF(r *http.Request, want string) bool {
	got := r.Header.Get("X-CSRF-Token")
	if got == "" {
		got = r.FormValue("_csrf")
	}
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// --- rate limiting ---

func (m *Manager) rateLimited(username string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.attempts[username]
	if a == nil {
		return false
	}
	if time.Since(a.first) > attemptWindow {
		delete(m.attempts, username)
		return false
	}
	return a.count >= maxAttempts
}

func (m *Manager) recordFailure(username string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.attempts[username]
	if a == nil || time.Since(a.first) > attemptWindow {
		m.attempts[username] = &attempt{count: 1, first: time.Now()}
		return
	}
	a.count++
}

func (m *Manager) clearAttempts(username string) {
	m.mu.Lock()
	delete(m.attempts, username)
	m.mu.Unlock()
}

// --- misc ---

type userInfo struct {
	id   int64
	name string
	role string
	csrf string
}

func withUser(r *http.Request, s *session) context.Context {
	return context.WithValue(r.Context(), userKey, userInfo{id: s.userID, name: s.username, role: s.role, csrf: s.csrf})
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func isAPI(r *http.Request) bool {
	return len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/"
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}
