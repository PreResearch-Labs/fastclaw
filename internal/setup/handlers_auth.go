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
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
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

	var result []auth.User
	result = append(result, users...)
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

	jsonResponse(w, http.StatusOK, user)
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
