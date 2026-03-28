package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Auth struct {
	Store    *UserStore
	Sessions *SessionStore
	Enabled  bool

	rateLimit   map[string]*rateLimitEntry
	rateLimitMu sync.Mutex
}

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

func NewAuth(store *UserStore, sessions *SessionStore, enabled bool) *Auth {
	return &Auth{
		Store:     store,
		Sessions:  sessions,
		Enabled:   enabled,
		rateLimit: make(map[string]*rateLimitEntry),
	}
}

func (a *Auth) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.Enabled {
			next(w, r)
			return
		}

		if IsPublicPath(r.URL.Path) {
			next(w, r)
			return
		}

		sess := a.Sessions.GetFromRequest(r)
		if sess == nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		next(w, r)
	}
}

func (a *Auth) AdminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.Enabled {
			next(w, r)
			return
		}

		sess := a.Sessions.GetFromRequest(r)
		if sess == nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		if sess.Role != "admin" {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}

		next(w, r)
	}
}

func (a *Auth) GetSession(r *http.Request) *Session {
	if !a.Enabled {
		return nil
	}
	return a.Sessions.GetFromRequest(r)
}

func (a *Auth) CheckRateLimit(ip string) bool {
	a.rateLimitMu.Lock()
	defer a.rateLimitMu.Unlock()

	now := time.Now()
	entry, ok := a.rateLimit[ip]
	if !ok || now.After(entry.resetAt) {
		a.rateLimit[ip] = &rateLimitEntry{
			count:   1,
			resetAt: now.Add(time.Minute),
		}
		return true
	}

	if entry.count >= 5 {
		return false
	}

	entry.count++
	return true
}

func (a *Auth) Login(username, password, ip string) (*Session, error) {
	if !a.CheckRateLimit(ip) {
		return nil, ErrRateLimited
	}

	user, err := a.Store.GetByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if !CheckPassword(password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	if err := a.Store.UpdateLastLogin(user.ID); err != nil {
		slog.Warn("update last login", "error", err)
	}

	sess := a.Sessions.Create(user.ID, user.Username, user.Role)
	return sess, nil
}

func (a *Auth) Logout(r *http.Request) {
	sess := a.Sessions.GetFromRequest(r)
	if sess != nil {
		a.Sessions.Delete(sess.ID)
	}
}

var (
	ErrInvalidCredentials = &AuthError{Message: "invalid username or password"}
	ErrRateLimited        = &AuthError{Message: "too many login attempts, please wait"}
)

type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
