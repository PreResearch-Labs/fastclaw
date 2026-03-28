package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Session struct {
	ID        string
	UserID    string
	Username  string
	Role      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	secret   string
	maxAge   time.Duration
}

func NewSessionStore(secret string, maxAgeSeconds int) *SessionStore {
	if secret == "" {
		b := make([]byte, 32)
		rand.Read(b)
		secret = hex.EncodeToString(b)
	}
	if maxAgeSeconds <= 0 {
		maxAgeSeconds = 86400
	}
	return &SessionStore{
		sessions: make(map[string]*Session),
		secret:   secret,
		maxAge:   time.Duration(maxAgeSeconds) * time.Second,
	}
}

func (s *SessionStore) Create(userID, username, role string) *Session {
	id := generateSessionID()
	now := time.Now()
	sess := &Session{
		ID:        id,
		UserID:    userID,
		Username:  username,
		Role:      role,
		CreatedAt: now,
		ExpiresAt: now.Add(s.maxAge),
	}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	return sess
}

func (s *SessionStore) Get(id string) *Session {
	s.mu.RLock()
	sess, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	if time.Now().After(sess.ExpiresAt) {
		s.Delete(id)
		return nil
	}
	return sess
}

func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

func (s *SessionStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
}

func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *SessionStore) SetCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "fastclaw_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(s.maxAge.Seconds()),
	})
}

func (s *SessionStore) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "fastclaw_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func (s *SessionStore) GetFromRequest(r *http.Request) *Session {
	cookie, err := r.Cookie("fastclaw_session")
	if err != nil {
		return nil
	}
	return s.Get(cookie.Value)
}

func IsPublicPath(path string) bool {
	public := []string{
		"/api/auth/login",
		"/api/status",
		"/login",
		"/_next/",
		"/favicon.ico",
	}
	for _, p := range public {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".woff") || strings.HasSuffix(path, ".woff2") ||
		strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".ico") ||
		strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".ttf") {
		return true
	}
	return false
}
