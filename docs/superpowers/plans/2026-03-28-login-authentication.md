# FastClaw 登录认证系统实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 FastClaw Web UI 添加登录认证系统，支持多用户、管理员/普通用户两种角色。

**Architecture:** Session-based 认证 + SQLite 用户存储 + bcrypt 密码哈希。后端新增 `internal/auth` 包处理用户管理和 Session，前端新增登录页面和路由保护。

**Tech Stack:** Go + SQLite + bcrypt + Next.js (React)

---

## 文件结构

### 后端新增文件
```
internal/auth/
  user.go          # 用户模型和 CRUD 操作
  session.go       # Session 管理
  auth.go          # 认证中间件
  store.go         # SQLite 用户存储

internal/setup/
  handlers_auth.go # 认证 API 处理器 (新增)
```

### 后端修改文件
```
internal/config/config.go   # 添加 AuthConfig 结构
internal/setup/server.go    # 添加认证路由和中间件
go.mod                      # 添加依赖
```

### 前端新增文件
```
web/src/app/login/
  page.tsx         # 登录页面
  layout.tsx       # 简洁布局 (无侧边栏)

web/src/lib/
  auth.ts          # 认证 API 和状态管理
```

### 前端修改文件
```
web/src/app/page.tsx           # 添加认证检查
web/src/app/overview/layout.tsx # 添加认证保护
web/src/app/settings/page.tsx  # 添加用户管理标签
web/src/lib/api.ts             # 添加认证 API
```

---

## Task 1: 添加 Go 依赖

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: 添加 bcrypt 和 SQLite 驱动依赖**

运行命令:
```bash
go get golang.org/x/crypto/bcrypt
go get github.com/mattn/go-sqlite3
```

预期: go.mod 和 go.sum 更新

- [ ] **Step 2: 验证依赖**

运行: `go mod tidy`
预期: 无错误

- [ ] **Step 3: 提交**

```bash
git add go.mod go.sum
git commit -s -m "chore: add bcrypt and sqlite3 dependencies for auth system"
```

---

## Task 2: 扩展配置结构

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: 在 config.go 末尾添加 AuthConfig 结构**

在 `internal/config/config.go` 文件末尾添加:

```go

// AuthConfig holds authentication settings for Web UI.
type AuthConfig struct {
	Enabled       bool   `json:"enabled,omitempty"`
	SessionSecret string `json:"sessionSecret,omitempty"`
	SessionMaxAge int    `json:"sessionMaxAge,omitempty"` // seconds, default 86400
}

// UserConfig holds a user entry in config (for bootstrapping).
type UserConfig struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"` // plaintext, converted to hash on first run
	Role     string `json:"role,omitempty"`     // "admin" or "user"
}
```

- [ ] **Step 2: 在 Config 结构中添加 Auth 字段**

找到 `Config` 结构体 (约第 117 行), 在 `Skills` 字段后添加:

```go
	Auth        AuthConfig                 `json:"auth,omitempty"`
```

- [ ] **Step 3: 验证编译**

运行: `go build ./...`
预期: 无错误

- [ ] **Step 4: 提交**

```bash
git add internal/config/config.go
git commit -s -m "feat(config): add AuthConfig for Web UI authentication"
```

---

## Task 3: 创建用户模型和存储

**Files:**
- Create: `internal/auth/user.go`
- Create: `internal/auth/store.go`

- [ ] **Step 1: 创建用户模型文件 `internal/auth/user.go`**

```go
package auth

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
	LastLoginAt  time.Time `json:"lastLoginAt,omitempty"`
}

type UserJSON struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	CreatedAt   string `json:"createdAt"`
	LastLoginAt string `json:"lastLoginAt,omitempty"`
}

func (u *User) ToJSON() UserJSON {
	return UserJSON{
		ID:          u.ID,
		Username:    u.Username,
		Role:        u.Role,
		CreatedAt:   u.CreatedAt.Format(time.RFC3339),
		LastLoginAt: u.LastLoginAt.Format(time.RFC3339),
	}
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func ValidatePassword(password string) bool {
	return len(password) >= 8
}
```

- [ ] **Step 2: 创建用户存储文件 `internal/auth/store.go`**

```go
package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type UserStore struct {
	db *sql.DB
}

func NewUserStore(dataDir string) (*UserStore, error) {
	dbPath := filepath.Join(dataDir, "users.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open users db: %w", err)
	}

	store := &UserStore{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return store, nil
}

func (s *UserStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			created_at DATETIME NOT NULL,
			last_login_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	`)
	return err
}

func (s *UserStore) Close() error {
	return s.db.Close()
}

func (s *UserStore) Create(username, password, role string) (*User, error) {
	if !ValidatePassword(password) {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	id := generateID()
	now := time.Now()

	_, err = s.db.Exec(
		`INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, username, hash, role, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return &User{
		ID:           id,
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		CreatedAt:    now,
	}, nil
}

func (s *UserStore) GetByUsername(username string) (*User, error) {
	u := &User{}
	var lastLogin sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, role, created_at, last_login_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &lastLogin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		u.LastLoginAt = lastLogin.Time
	}
	return u, nil
}

func (s *UserStore) GetByID(id string) (*User, error) {
	u := &User{}
	var lastLogin sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, role, created_at, last_login_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &lastLogin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		u.LastLoginAt = lastLogin.Time
	}
	return u, nil
}

func (s *UserStore) List() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, password_hash, role, created_at, last_login_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var lastLogin sql.NullTime
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &lastLogin); err != nil {
			return nil, err
		}
		if lastLogin.Valid {
			u.LastLoginAt = lastLogin.Time
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *UserStore) UpdatePassword(id, password string) error {
	if !ValidatePassword(password) {
		return fmt.Errorf("password must be at least 8 characters")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

func (s *UserStore) UpdateLastLogin(id string) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

func (s *UserStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *UserStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *UserStore) EnsureDefaultAdmin() (string, error) {
	count, err := s.Count()
	if err != nil {
		return "", err
	}
	if count > 0 {
		return "", nil
	}

	password := os.Getenv("FASTCLAW_ADMIN_PASSWORD")
	if password == "" {
		b := make([]byte, 16)
		rand.Read(b)
		password = hex.EncodeToString(b)
	}

	_, err = s.Create("admin", password, "admin")
	if err != nil {
		return "", err
	}
	return password, nil
}
```

- [ ] **Step 3: 验证编译**

运行: `go build ./internal/auth/...`
预期: 无错误

- [ ] **Step 4: 提交**

```bash
git add internal/auth/user.go internal/auth/store.go
git commit -s -m "feat(auth): add User model and SQLite user store"
```

---

## Task 4: 创建 Session 管理

**Files:**
- Create: `internal/auth/session.go`

- [ ] **Step 1: 创建 Session 管理文件 `internal/auth/session.go`**

```go
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
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
	secret   []byte
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
		secret:   []byte(secret),
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
	now := time.Now()
	for id, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
	s.mu.Unlock()
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

func (s *SessionStore) Sign(data string) string {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *SessionStore) Verify(data, sig string) bool {
	expected := s.Sign(data)
	return hmac.Equal([]byte(sig), []byte(expected))
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
		strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".ico") {
		return true
	}
	return false
}
```

- [ ] **Step 2: 验证编译**

运行: `go build ./internal/auth/...`
预期: 无错误

- [ ] **Step 3: 提交**

```bash
git add internal/auth/session.go
git commit -s -m "feat(auth): add session management with cookie support"
```

---

## Task 5: 创建认证中间件

**Files:**
- Create: `internal/auth/auth.go`

- [ ] **Step 1: 创建认证中间件文件 `internal/auth/auth.go`**

```go
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

	rateLimit    map[string]*rateLimitEntry
	rateLimitMu  sync.Mutex
}

type rateLimitEntry struct {
	count     int
	resetAt   time.Time
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
```

- [ ] **Step 2: 验证编译**

运行: `go build ./internal/auth/...`
预期: 无错误

- [ ] **Step 3: 提交**

```bash
git add internal/auth/auth.go
git commit -s -m "feat(auth): add authentication middleware with rate limiting"
```

---

## Task 6: 创建认证 API 处理器

**Files:**
- Create: `internal/setup/handlers_auth.go`

- [ ] **Step 1: 创建认证处理器文件 `internal/setup/handlers_auth.go`**

```go
package setup

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fastclaw-ai/fastclaw/internal/auth"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, loginResponse{Error: "invalid request"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		jsonResponse(w, http.StatusBadRequest, loginResponse{Error: "username and password required"})
		return
	}

	ip := getRemoteIP(r)
	sess, err := s.auth.Login(req.Username, req.Password, ip)
	if err != nil {
		if err == auth.ErrRateLimited {
			jsonResponse(w, http.StatusTooManyRequests, loginResponse{Error: err.Error()})
			return
		}
		jsonResponse(w, http.StatusUnauthorized, loginResponse{Error: err.Error()})
		return
	}

	s.auth.Sessions.SetCookie(w, sess.ID)
	jsonResponse(w, http.StatusOK, loginResponse{OK: true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.Logout(r)
	s.auth.Sessions.ClearCookie(w)
	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	sess := s.auth.GetSession(r)
	if sess == nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":       sess.UserID,
		"username": sess.Username,
		"role":     sess.Role,
	})
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.auth.Store.List()
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var result []auth.UserJSON
	for _, u := range users {
		result = append(result, u.ToJSON())
	}
	jsonResponse(w, http.StatusOK, result)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "username and password required"})
		return
	}

	if req.Role != "admin" && req.Role != "user" {
		req.Role = "user"
	}

	user, err := s.auth.Store.Create(req.Username, req.Password, req.Role)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, user.ToJSON())
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	sess := s.auth.GetSession(r)
	if sess != nil && sess.UserID == id {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "cannot delete yourself"})
		return
	}

	if err := s.auth.Store.Delete(id); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

func (s *Server) handleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if err := s.auth.Store.UpdatePassword(id, req.Password); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	sess := s.auth.GetSession(r)
	if sess == nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if err := s.auth.Store.UpdatePassword(sess.UserID, req.Password); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

func getRemoteIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
```

- [ ] **Step 2: 验证编译**

运行: `go build ./internal/setup/...`
预期: 编译错误 (Server 缺少 auth 字段)

- [ ] **Step 3: 提交**

```bash
git add internal/setup/handlers_auth.go
git commit -s -m "feat(setup): add authentication API handlers"
```

---

## Task 7: 修改 Server 集成认证

**Files:**
- Modify: `internal/setup/server.go`

- [ ] **Step 1: 在 Server 结构体添加 auth 字段**

在 `Server` 结构体 (约第 30 行) 添加:

```go
	auth        *auth.Auth
```

- [ ] **Step 2: 添加 import**

在 import 块中添加:

```go
	"github.com/fastclaw-ai/fastclaw/internal/auth"
```

- [ ] **Step 3: 添加 SetAuth 方法**

在 `SetAPIServer` 方法后添加:

```go

func (s *Server) SetAuth(a *auth.Auth) {
	s.auth = a
}
```

- [ ] **Step 4: 在 Run 方法中添加认证路由**

找到 `// API routes` 注释 (约第 78 行), 在其后添加:

```go
	// Auth routes (public)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleGetCurrentUser)
	mux.HandleFunc("PUT /api/auth/password", s.handleChangePassword)

	// User management (admin only)
	if s.auth != nil && s.auth.Enabled {
		mux.HandleFunc("GET /api/users", s.auth.AdminMiddleware(s.handleListUsers))
		mux.HandleFunc("POST /api/users", s.auth.AdminMiddleware(s.handleCreateUser))
		mux.HandleFunc("DELETE /api/users/{id}", s.auth.AdminMiddleware(s.handleDeleteUser))
		mux.HandleFunc("PUT /api/users/{id}/password", s.auth.AdminMiddleware(s.handleResetUserPassword))
	}
```

- [ ] **Step 5: 为现有 API 添加认证中间件**

将现有的 API 路由用中间件包裹。修改 `handleStatus` 之后的所有路由:

```go
	// Protected API routes
	if s.auth != nil && s.auth.Enabled {
		mux.HandleFunc("GET /api/config", s.auth.Middleware(s.handleGetConfig))
		mux.HandleFunc("POST /api/config", s.auth.Middleware(s.handleUpdateConfig))
		mux.HandleFunc("POST /api/test-provider", s.auth.Middleware(s.handleTestProvider))
		mux.HandleFunc("POST /api/save-config", s.auth.Middleware(s.handleSaveConfig))
		mux.HandleFunc("POST /api/chat", s.auth.Middleware(s.handleChat))
		mux.HandleFunc("GET /api/agents", s.auth.Middleware(s.handleListAgents))
		mux.HandleFunc("POST /api/agents", s.auth.Middleware(s.handleCreateAgent))
		mux.HandleFunc("PUT /api/agents/{id}", s.auth.Middleware(s.handleUpdateAgent))
		mux.HandleFunc("DELETE /api/agents/{id}", s.auth.Middleware(s.handleDeleteAgent))
		mux.HandleFunc("GET /api/skills", s.auth.Middleware(s.handleListSkills))
		mux.HandleFunc("DELETE /api/skills/{name}", s.auth.Middleware(s.handleDeleteSkill))
		mux.HandleFunc("GET /api/plugins", s.auth.Middleware(s.handleListPlugins))
		mux.HandleFunc("PUT /api/plugins/{id}", s.auth.Middleware(s.handleUpdatePlugin))
		mux.HandleFunc("GET /api/tasks", s.auth.Middleware(s.handleListTasks))
		mux.HandleFunc("GET /api/channels", s.auth.Middleware(s.handleListChannels))
		mux.HandleFunc("GET /api/cron", s.auth.Middleware(s.handleListCronJobs))
		mux.HandleFunc("POST /api/cron", s.auth.Middleware(s.handleCreateCronJob))
		mux.HandleFunc("PUT /api/cron/{id}", s.auth.Middleware(s.handleUpdateCronJob))
		mux.HandleFunc("DELETE /api/cron/{id}", s.auth.Middleware(s.handleDeleteCronJob))
	} else {
		mux.HandleFunc("GET /api/config", s.handleGetConfig)
		mux.HandleFunc("POST /api/config", s.handleUpdateConfig)
		mux.HandleFunc("POST /api/test-provider", s.handleTestProvider)
		mux.HandleFunc("POST /api/save-config", s.handleSaveConfig)
		mux.HandleFunc("POST /api/chat", s.handleChat)
		mux.HandleFunc("GET /api/agents", s.handleListAgents)
		mux.HandleFunc("POST /api/agents", s.handleCreateAgent)
		mux.HandleFunc("PUT /api/agents/{id}", s.handleUpdateAgent)
		mux.HandleFunc("DELETE /api/agents/{id}", s.handleDeleteAgent)
		mux.HandleFunc("GET /api/skills", s.handleListSkills)
		mux.HandleFunc("DELETE /api/skills/{name}", s.handleDeleteSkill)
		mux.HandleFunc("GET /api/plugins", s.handleListPlugins)
		mux.HandleFunc("PUT /api/plugins/{id}", s.handleUpdatePlugin)
		mux.HandleFunc("GET /api/tasks", s.handleListTasks)
		mux.HandleFunc("GET /api/channels", s.handleListChannels)
		mux.HandleFunc("GET /api/cron", s.handleListCronJobs)
		mux.HandleFunc("POST /api/cron", s.handleCreateCronJob)
		mux.HandleFunc("PUT /api/cron/{id}", s.handleUpdateCronJob)
		mux.HandleFunc("DELETE /api/cron/{id}", s.handleDeleteCronJob)
	}
```

删除原有的重复路由定义。

- [ ] **Step 6: 验证编译**

运行: `go build ./internal/setup/...`
预期: 无错误

- [ ] **Step 7: 提交**

```bash
git add internal/setup/server.go
git commit -s -m "feat(setup): integrate authentication middleware into server"
```

---

## Task 8: 在 main.go 初始化认证

**Files:**
- Modify: `cmd/fastclaw/main.go`

- [ ] **Step 1: 添加 auth import**

在 import 块添加:

```go
	"github.com/fastclaw-ai/fastclaw/internal/auth"
```

- [ ] **Step 2: 添加认证初始化函数**

在 `runGateway` 函数中, `slog.Info("starting gateway")` 后添加:

```go
	// Initialize auth
	homeDir, _ := config.HomeDir()
	userStore, err := auth.NewUserStore(homeDir)
	if err != nil {
		return fmt.Errorf("init user store: %w", err)
	}

	// Ensure default admin exists
	initialPassword, err := userStore.EnsureDefaultAdmin()
	if err != nil {
		return fmt.Errorf("ensure default admin: %w", err)
	}
	if initialPassword != "" {
		slog.Info("created default admin", "username", "admin", "password", initialPassword)
	}

	sessionSecret := cfg.Auth.SessionSecret
	if sessionSecret == "" {
		sessionSecret = cfg.Gateway.Auth.Token
	}
	sessions := auth.NewSessionStore(sessionSecret, cfg.Auth.SessionMaxAge)
	authObj := auth.NewAuth(userStore, sessions, cfg.Auth.Enabled || cfg.Gateway.Mode == "public")
```

- [ ] **Step 3: 将 auth 传递给 webSrv**

在 `webSrv := setup.NewServer(...)` 后添加:

```go
	webSrv.SetAuth(authObj)
```

- [ ] **Step 4: 验证编译**

运行: `go build ./cmd/fastclaw/...`
预期: 无错误

- [ ] **Step 5: 提交**

```bash
git add cmd/fastclaw/main.go
git commit -s -m "feat(main): initialize auth system on gateway startup"
```

---

## Task 9: 创建前端登录页面

**Files:**
- Create: `web/src/app/login/layout.tsx`
- Create: `web/src/app/login/page.tsx`

- [ ] **Step 1: 创建登录布局文件 `web/src/app/login/layout.tsx`**

```tsx
export default function LoginLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen bg-background">
      {children}
    </div>
  );
}
```

- [ ] **Step 2: 创建登录页面 `web/src/app/login/page.tsx`**

```tsx
"use client";

import { useState, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { login, getMe } from "@/lib/auth";

export default function LoginPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    getMe()
      .then((user) => {
        if (user) {
          const redirect = searchParams.get("redirect") || "/overview";
          router.replace(redirect);
        } else {
          setChecking(false);
        }
      })
      .catch(() => setChecking(false));
  }, [router, searchParams]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const result = await login(username, password);
      if (result.ok) {
        const redirect = searchParams.get("redirect") || "/overview";
        router.replace(redirect);
      } else {
        setError(result.error || "Login failed");
      }
    } catch {
      setError("Could not connect to server");
    } finally {
      setLoading(false);
    }
  };

  if (checking) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-zinc-950">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-zinc-700 border-t-violet-500" />
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-950 px-4">
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className="absolute -top-[40%] left-1/2 h-[800px] w-[800px] -translate-x-1/2 rounded-full bg-primary/5 blur-3xl" />
      </div>

      <Card className="w-full max-w-sm backdrop-blur-sm">
        <CardHeader className="space-y-4 text-center">
          <div className="mx-auto flex h-14 w-14 items-center justify-center">
            <img src="/logo.png" alt="FastClaw" className="h-14 w-14 rounded-xl" />
          </div>
          <div>
            <CardTitle className="text-2xl font-bold">
              <span className="bg-gradient-to-r from-violet-400 via-cyan-400 to-violet-400 bg-clip-text text-transparent">
                FastClaw
              </span>
            </CardTitle>
            <CardDescription className="mt-2">
              Sign in to your account
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="admin"
                required
                autoComplete="username"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                required
                autoComplete="current-password"
              />
            </div>
            {error && (
              <p className="text-sm text-destructive">{error}</p>
            )}
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "Signing in..." : "Sign In"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
```

- [ ] **Step 3: 提交**

```bash
git add web/src/app/login/
git commit -s -m "feat(web): add login page with dark theme"
```

---

## Task 10: 创建前端认证 API

**Files:**
- Create: `web/src/lib/auth.ts`

- [ ] **Step 1: 创建认证 API 文件 `web/src/lib/auth.ts`**

```typescript
export interface User {
  id: string;
  username: string;
  role: string;
}

export interface LoginResult {
  ok: boolean;
  error?: string;
}

export async function login(username: string, password: string): Promise<LoginResult> {
  const res = await fetch("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  return res.json();
}

export async function logout(): Promise<void> {
  await fetch("/api/auth/logout", { method: "POST" });
}

export async function getMe(): Promise<User | null> {
  try {
    const res = await fetch("/api/auth/me");
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}

export async function changePassword(password: string): Promise<{ ok: boolean; error?: string }> {
  const res = await fetch("/api/auth/password", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password }),
  });
  return res.json();
}

export async function getUsers(): Promise<User[]> {
  const res = await fetch("/api/users");
  return res.json();
}

export async function createUser(username: string, password: string, role: string): Promise<User> {
  const res = await fetch("/api/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password, role }),
  });
  return res.json();
}

export async function deleteUser(id: string): Promise<{ ok: boolean }> {
  const res = await fetch(`/api/users/${id}`, { method: "DELETE" });
  return res.json();
}

export async function resetUserPassword(id: string, password: string): Promise<{ ok: boolean }> {
  const res = await fetch(`/api/users/${id}/password`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password }),
  });
  return res.json();
}
```

- [ ] **Step 2: 提交**

```bash
git add web/src/lib/auth.ts
git commit -s -m "feat(web): add authentication API functions"
```

---

## Task 11: 修改根页面添加认证检查

**Files:**
- Modify: `web/src/app/page.tsx`

- [ ] **Step 1: 修改根页面添加认证检查**

替换 `web/src/app/page.tsx` 内容:

```tsx
"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getStatus, getMe } from "@/lib/api";

export default function RootRedirect() {
  const router = useRouter();
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    Promise.all([getStatus(), getMe()])
      .then(([status, user]) => {
        if (!user) {
          router.replace("/login");
          return;
        }
        if (status.configured) {
          router.replace("/overview/");
        } else {
          router.replace("/onboard/");
        }
      })
      .catch(() => {
        router.replace("/login");
      })
      .finally(() => setChecking(false));
  }, [router]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-950">
      <div className="h-8 w-8 animate-spin rounded-full border-2 border-zinc-700 border-t-violet-500" />
    </div>
  );
}
```

- [ ] **Step 2: 更新 api.ts 导出**

在 `web/src/lib/api.ts` 末尾确保导出 `getMe`:

```typescript
export { getMe } from "./auth";
```

实际上我们已经在 auth.ts 中定义了 getMe，所以需要修改 page.tsx 的 import:

```tsx
import { getStatus } from "@/lib/api";
import { getMe } from "@/lib/auth";
```

- [ ] **Step 3: 提交**

```bash
git add web/src/app/page.tsx
git commit -s -m "feat(web): add auth check to root redirect"
```

---

## Task 12: 添加路由保护

**Files:**
- Modify: `web/src/app/overview/layout.tsx`
- Create: `web/src/components/auth-guard.tsx`

- [ ] **Step 1: 创建认证守卫组件 `web/src/components/auth-guard.tsx`**

```tsx
"use client";

import { useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { getMe, type User } from "@/lib/auth";

export function useAuth() {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    getMe()
      .then((u) => {
        setUser(u);
        if (!u) {
          router.replace(`/login?redirect=${encodeURIComponent(pathname)}`);
        }
      })
      .catch(() => {
        router.replace(`/login?redirect=${encodeURIComponent(pathname)}`);
      })
      .finally(() => setLoading(false));
  }, [router, pathname]);

  return { user, loading };
}

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { loading } = useAuth();

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-zinc-950">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-zinc-700 border-t-violet-500" />
      </div>
    );
  }

  return <>{children}</>;
}
```

- [ ] **Step 2: 修改 overview layout 使用 AuthGuard**

修改 `web/src/app/overview/layout.tsx`, 包裹 AuthGuard:

```tsx
import { AuthGuard } from "@/components/auth-guard";

export default function OverviewLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <AuthGuard>{children}</AuthGuard>;
}
```

- [ ] **Step 3: 提交**

```bash
git add web/src/components/auth-guard.tsx web/src/app/overview/layout.tsx
git commit -s -m "feat(web): add auth guard for protected routes"
```

---

## Task 13: 在 Settings 添加用户管理

**Files:**
- Modify: `web/src/app/settings/page.tsx`

- [ ] **Step 1: 完整替换 settings 页面内容**

替换 `web/src/app/settings/page.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Database, Webhook, Save, Check, Users, Trash2, KeyRound, UserPlus } from "lucide-react";
import { getConfig, updateConfig, type ConfigResponse } from "@/lib/api";
import { useAuth, getUsers, createUser, deleteUser, resetUserPassword, type User } from "@/lib/auth";

export default function SettingsPage() {
  const [config, setConfig] = useState<ConfigResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  const [storageType, setStorageType] = useState("file");
  const [dsn, setDsn] = useState("");
  const [webhookEnabled, setWebhookEnabled] = useState(false);
  const [webhookToken, setWebhookToken] = useState("");
  const [webhookPath, setWebhookPath] = useState("/hooks");

  const { user } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [usersLoading, setUsersLoading] = useState(true);
  const [newUsername, setNewUsername] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [newRole, setNewRole] = useState("user");
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    setLoading(true);
    getConfig()
      .then((cfg) => {
        setConfig(cfg);
        setStorageType(cfg.storage?.type || "file");
        setDsn(cfg.storage?.dsn || "");
        setWebhookEnabled(cfg.hooks?.enabled || false);
        setWebhookToken(cfg.hooks?.token || "");
        setWebhookPath(cfg.hooks?.path || "/hooks");
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (user?.role === "admin") {
      setUsersLoading(true);
      getUsers()
        .then(setUsers)
        .catch(() => {})
        .finally(() => setUsersLoading(false));
    }
  }, [user]);

  const handleSave = async () => {
    setSaving(true);
    await updateConfig({
      storage: { type: storageType, dsn },
      hooks: {
        enabled: webhookEnabled,
        token: webhookToken,
        path: webhookPath,
      },
    });
    setSaving(false);
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  };

  const handleCreateUser = async () => {
    if (!newUsername || !newPassword) return;
    setCreating(true);
    try {
      const u = await createUser(newUsername, newPassword, newRole);
      setUsers([...users, u]);
      setNewUsername("");
      setNewPassword("");
      setNewRole("user");
    } catch (e) {
      console.error(e);
    }
    setCreating(false);
  };

  const handleDeleteUser = async (id: string) => {
    if (!confirm("Delete this user?")) return;
    await deleteUser(id);
    setUsers(users.filter((u) => u.id !== id));
  };

  const handleResetPassword = async (id: string) => {
    const password = prompt("Enter new password (min 8 characters):");
    if (!password || password.length < 8) return;
    await resetUserPassword(id, password);
    alert("Password reset successfully");
  };

  if (loading) {
    return (
      <div className="p-6 space-y-6 max-w-3xl mx-auto">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-64 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6 max-w-3xl mx-auto">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">Settings</h2>
          <p className="text-sm text-muted-foreground mt-1">
            Gateway configuration
          </p>
        </div>
        <Button
          onClick={handleSave}
          disabled={saving}
          variant={saved ? "outline" : "default"}
          className={saved ? "border-emerald-500/30 text-emerald-600 dark:text-emerald-400" : ""}
        >
          {saved ? (
            <>
              <Check className="h-4 w-4 mr-2" />
              Saved
            </>
          ) : (
            <>
              <Save className="h-4 w-4 mr-2" />
              {saving ? "Saving..." : "Save Settings"}
            </>
          )}
        </Button>
      </div>

      <Tabs defaultValue="storage" className="w-full">
        <TabsList className="grid w-full grid-cols-2">
          <TabsTrigger value="storage">General</TabsTrigger>
          {user?.role === "admin" && <TabsTrigger value="users">Users</TabsTrigger>}
        </TabsList>

        <TabsContent value="storage" className="space-y-6 mt-6">
          <div className="rounded-lg border border-border bg-card">
            <div className="p-5 pb-3">
              <div className="flex items-center gap-2 mb-1">
                <Database className="h-4 w-4 text-blue-500" />
                <h3 className="font-medium">Storage</h3>
              </div>
              <p className="text-sm text-muted-foreground">
                Configure data persistence backend
              </p>
            </div>
            <div className="px-5 pb-5 space-y-4">
              <div className="space-y-2">
                <Label>Storage Type</Label>
                <Select value={storageType} onValueChange={(v) => v && setStorageType(v)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="file">File System</SelectItem>
                    <SelectItem value="sqlite">SQLite</SelectItem>
                    <SelectItem value="postgres">PostgreSQL</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {storageType !== "file" && (
                <div className="space-y-2">
                  <Label>Connection String (DSN)</Label>
                  <Input
                    value={dsn}
                    onChange={(e) => setDsn(e.target.value)}
                    placeholder={storageType === "sqlite" ? "./data.db" : "postgres://user:pass@host:5432/fastclaw"}
                    className="font-mono text-sm"
                  />
                </div>
              )}
            </div>
          </div>

          <div className="rounded-lg border border-border bg-card">
            <div className="p-5">
              <div className="flex items-center justify-between">
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <Webhook className="h-4 w-4 text-cyan-500" />
                    <h3 className="font-medium">Webhooks</h3>
                  </div>
                  <p className="text-sm text-muted-foreground">
                    HTTP webhook ingress for external integrations
                  </p>
                </div>
                <Switch
                  checked={webhookEnabled}
                  onCheckedChange={setWebhookEnabled}
                />
              </div>
            </div>
            {webhookEnabled && (
              <div className="px-5 pb-5 space-y-4">
                <Separator />
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>Webhook Path</Label>
                    <Input
                      value={webhookPath}
                      onChange={(e) => setWebhookPath(e.target.value)}
                      placeholder="/hooks"
                      className="font-mono text-sm"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>Bearer Token</Label>
                    <Input
                      type="password"
                      value={webhookToken}
                      onChange={(e) => setWebhookToken(e.target.value)}
                      placeholder="secret-token"
                      className="font-mono text-sm"
                    />
                  </div>
                </div>
              </div>
            )}
          </div>
        </TabsContent>

        {user?.role === "admin" && (
          <TabsContent value="users" className="space-y-6 mt-6">
            <div className="rounded-lg border border-border bg-card">
              <div className="p-5 pb-3">
                <div className="flex items-center gap-2 mb-1">
                  <Users className="h-4 w-4 text-violet-500" />
                  <h3 className="font-medium">User Management</h3>
                </div>
                <p className="text-sm text-muted-foreground">
                  Create and manage user accounts
                </p>
              </div>

              <div className="px-5 pb-5 space-y-4">
                <div className="grid grid-cols-1 sm:grid-cols-4 gap-2">
                  <Input
                    value={newUsername}
                    onChange={(e) => setNewUsername(e.target.value)}
                    placeholder="Username"
                  />
                  <Input
                    type="password"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    placeholder="Password"
                  />
                  <Select value={newRole} onValueChange={setNewRole}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="user">User</SelectItem>
                      <SelectItem value="admin">Admin</SelectItem>
                    </SelectContent>
                  </Select>
                  <Button onClick={handleCreateUser} disabled={creating || !newUsername || !newPassword}>
                    <UserPlus className="h-4 w-4 mr-2" />
                    Create
                  </Button>
                </div>

                <Separator />

                {usersLoading ? (
                  <Skeleton className="h-32 w-full" />
                ) : (
                  <div className="space-y-2">
                    {users.map((u) => (
                      <div
                        key={u.id}
                        className="flex items-center justify-between rounded-lg border p-3"
                      >
                        <div className="flex items-center gap-3">
                          <span className="font-medium">{u.username}</span>
                          <Badge variant={u.role === "admin" ? "default" : "outline"}>
                            {u.role}
                          </Badge>
                        </div>
                        <div className="flex items-center gap-2">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleResetPassword(u.id)}
                            disabled={u.id === user?.id}
                          >
                            <KeyRound className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleDeleteUser(u.id)}
                            disabled={u.id === user?.id}
                          >
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </TabsContent>
        )}
      </Tabs>
    </div>
  );
}
```

---

## Task 14: 集成测试

**Files:**
- 无新增

- [ ] **Step 1: 重新构建 Docker 镜像**

```bash
docker compose down
docker compose build --no-cache
docker compose up -d
```

- [ ] **Step 2: 测试首次启动**

```bash
docker compose logs | grep "created default admin"
```
预期: 看到默认密码输出

- [ ] **Step 3: 测试登录流程**

```bash
curl -X POST http://localhost:18953/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<password>"}' \
  -c cookies.txt
```
预期: 返回 `{"ok":true}`

- [ ] **Step 4: 测试认证 API**

```bash
curl http://localhost:18953/api/auth/me -b cookies.txt
```
预期: 返回用户信息

- [ ] **Step 5: 测试 Web UI**

打开浏览器访问 http://localhost:18953
预期: 重定向到登录页

- [ ] **Step 6: 提交测试通过**

```bash
git add -A
git commit -s -m "test: verify authentication system integration"
```

---

## 完成清单

- [ ] 所有后端文件创建并编译通过
- [ ] 所有前端文件创建并构建通过
- [ ] Docker 构建成功
- [ ] 首次启动创建默认管理员
- [ ] 登录流程正常
- [ ] 认证中间件保护 API
- [ ] Web UI 路由保护生效
- [ ] 用户管理功能正常