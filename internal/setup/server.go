package setup

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/fastclaw-ai/fastclaw/internal/api"
	"github.com/fastclaw-ai/fastclaw/internal/auth"
	"github.com/fastclaw-ai/fastclaw/internal/config"
	"github.com/fastclaw-ai/fastclaw/internal/taskqueue"
)

// AgentHandle is a minimal interface for interacting with an agent from the web UI.
type AgentHandle interface {
	Name() string
	HandleWebChat(ctx context.Context, text string) string
}

// AgentProvider gives the server access to the running agents.
type AgentProvider interface {
	AllAgents() []AgentHandle
	AgentByID(id string) AgentHandle
}

// Server serves the setup wizard UI and handles config API endpoints.
type Server struct {
	port          int
	bind          string // "loopback" or "all"
	gatewayCfg    *config.GatewayCfg
	onConfig      func(*config.Config) // called after config is saved
	agentProvider AgentProvider
	taskQueue     *taskqueue.Queue
	apiServer     *api.Server
	auth          *auth.Auth
	startedAt     time.Time
}

// NewServer creates a setup wizard server on the given port.
func NewServer(port int, onConfig func(*config.Config)) *Server {
	return &Server{port: port, bind: "all", onConfig: onConfig, startedAt: time.Now()}
}

// SetGatewayConfig sets the gateway configuration for bind address and HTTP endpoints.
func (s *Server) SetGatewayConfig(cfg *config.GatewayCfg) {
	s.gatewayCfg = cfg
	if cfg.Bind != "" {
		s.bind = cfg.Bind
	}
	if cfg.Port > 0 {
		s.port = cfg.Port
	}
}

// SetAgentProvider sets the agent provider for chat and status endpoints.
func (s *Server) SetAgentProvider(ap AgentProvider) {
	s.agentProvider = ap
}

// SetTaskQueue sets the task queue for the tasks API endpoint.
func (s *Server) SetTaskQueue(tq *taskqueue.Queue) {
	s.taskQueue = tq
}

// SetAPIServer sets the OpenAI-compatible API server for /v1/* and /ws routes.
func (s *Server) SetAPIServer(apiSrv *api.Server) {
	s.apiServer = apiSrv
}

// SetAuth sets the authentication module for protected routes.
func (s *Server) SetAuth(a *auth.Auth) {
	s.auth = a
}

// Run starts the HTTP server and blocks until the context is canceled
// or the setup is completed.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	// API routes
	// Health check is public (minimal info for load balancers)
	mux.HandleFunc("GET /api/health", s.handleHealth)

	// Auth routes (public, no auth required)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleGetCurrentUser)
	mux.HandleFunc("PUT /api/auth/password", s.handleChangePassword)

	// User management routes (admin only, wrapped with middleware)
	if s.auth != nil && s.auth.Enabled {
		mux.HandleFunc("GET /api/users", s.auth.AdminMiddleware(s.handleListUsers))
		mux.HandleFunc("POST /api/users", s.auth.AdminMiddleware(s.handleCreateUser))
		mux.HandleFunc("DELETE /api/users/{id}", s.auth.AdminMiddleware(s.handleDeleteUser))
		mux.HandleFunc("PUT /api/users/{id}/password", s.auth.AdminMiddleware(s.handleResetUserPassword))
	}

	// Protected routes
	if s.auth != nil && s.auth.Enabled {
		mux.HandleFunc("GET /api/status", s.auth.Middleware(s.handleStatus))
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
		mux.HandleFunc("GET /api/status", s.handleStatus)
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

	// OpenAI-compatible API and WebSocket gateway
	if s.apiServer != nil {
		s.apiServer.RegisterRoutes(mux)
	}

	// Static files from embedded web/out
	webRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		return fmt.Errorf("setup: embed sub: %w", err)
	}

	// Serve static files with SPA fallback
	mux.Handle("/", spaHandler{fs: webRoot})

	// Determine bind address
	var addr string
	if s.bind == "all" {
		addr = fmt.Sprintf("0.0.0.0:%d", s.port)
	} else {
		addr = fmt.Sprintf("127.0.0.1:%d", s.port)
	}
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown on context cancel
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("setup: listen %s: %w", addr, err)
	}

	slog.Info("web UI running", "url", fmt.Sprintf("http://localhost:%d", s.port))
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// spaHandler serves static files, falling back to the directory's index.html
// for paths that don't match a file (to support client-side routing).
type spaHandler struct {
	fs fs.FS
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Strip trailing slash (except root) to normalize
	if path != "/" && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}

	// fs.FS paths don't have leading slash
	fsPath := strings.TrimPrefix(path, "/")
	if fsPath == "" {
		fsPath = "."
	}

	// Try to open the file directly (static assets like .js, .css, .ico)
	f, err := h.fs.Open(fsPath)
	if err == nil {
		stat, statErr := f.Stat()
		f.Close()
		if statErr == nil && !stat.IsDir() {
			http.ServeFileFS(w, r, h.fs, fsPath)
			return
		}
	}

	// For route paths (/onboard, /overview, /chat), look for index.html in that dir
	var indexPath string
	if fsPath == "." {
		indexPath = "index.html"
	} else {
		indexPath = fsPath + "/index.html"
	}
	f, err = h.fs.Open(indexPath)
	if err == nil {
		f.Close()
		http.ServeFileFS(w, r, h.fs, indexPath)
		return
	}

	// Try path.html
	htmlPath := fsPath + ".html"
	f, err = h.fs.Open(htmlPath)
	if err == nil {
		f.Close()
		http.ServeFileFS(w, r, h.fs, htmlPath)
		return
	}

	// Fall back to index.html for client-side routing
	http.ServeFileFS(w, r, h.fs, "index.html")
}
