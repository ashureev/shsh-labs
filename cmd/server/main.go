// SHSH - Agentic Linux Tutor Server
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ashureev/shsh-labs/internal/agent"
	"github.com/ashureev/shsh-labs/internal/api"
	"github.com/ashureev/shsh-labs/internal/config"
	"github.com/ashureev/shsh-labs/internal/container"
	"github.com/ashureev/shsh-labs/internal/identity"
	"github.com/ashureev/shsh-labs/internal/middleware"
	"github.com/ashureev/shsh-labs/internal/store"
	"github.com/ashureev/shsh-labs/internal/terminal"
	"github.com/ashureev/shsh-labs/web"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting server", "port", cfg.Port, "dev", cfg.IsDevelopment())

	// Initialize dependencies.
	repo, err := store.NewSQLite(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := repo.Close(); closeErr != nil {
			slog.Error("Failed to close repository", "error", closeErr)
		}
	}()

	if err := repo.Ping(context.Background()); err != nil {
		slog.Error("Database health check failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Database connected")

	usersDeleted, sessionsDeleted, err := repo.DeleteLegacyLocalState(context.Background())
	if err != nil {
		slog.Error("Failed to cleanup legacy local state", "error", err)
		os.Exit(1)
	}
	slog.Info("Legacy local state cleanup complete", "users_deleted", usersDeleted, "agent_sessions_deleted", sessionsDeleted)

	mgr, err := container.NewDockerManagerWithConfig(cfg)
	if err != nil {
		slog.Error("Failed to initialize container manager", "error", err)
		os.Exit(1)
	}
	slog.Info("Container manager initialized")

	// Ensure custom bridge network exists for playground containers.
	networkID, err := mgr.EnsureNetwork(context.Background())
	if err != nil {
		slog.Error("Failed to ensure playground network", "error", err)
		os.Exit(1)
	}
	slog.Info("Playground network ready", "network_id", networkID)

	// Initialize services.
	sm := terminal.NewSessionManager()

	// Initialize handlers.
	baseHandler := api.NewHandler(repo, mgr, sm, cfg.FrontendURL)
	healthHandler := api.NewHealthHandlerWithConfig(repo, cfg)
	wsHandler := terminal.NewWebSocketHandler(repo, mgr, sm, cfg.FrontendURL, cfg.IsDevelopment())

	// Initialize Python Agent gRPC client (optional)
	pythonAgentAddr := os.Getenv("PYTHON_AGENT_ADDR")
	var agentHandler *agent.Handler
	var sidebarChan chan *agent.Response
	var conversationLogger agent.ConversationLogger
	aiEnabled := false
	//nolint:nestif // Startup wiring is intentionally sequential to keep dependency setup explicit.
	if pythonAgentAddr != "" {
		slog.Info("Attempting to connect to Python Agent Service via gRPC", "address", pythonAgentAddr)

		grpcClient, err := agent.NewGrpcClient(pythonAgentAddr, logger)
		if err != nil {
			slog.Warn("Failed to connect to Python agent, AI features will be disabled", "error", err)
			aiEnabled = false
		} else {
			defer grpcClient.Close()
			aiEnabled = true

			// Create channel for Agent responses to sidebar
			sidebarChan = make(chan *agent.Response, 100)

			conversationLogger, err = agent.NewConversationLogger(agent.ConversationLogConfig{
				Enabled:       cfg.ConversationLog.Enabled,
				Dir:           cfg.ConversationLog.Dir,
				GlobalEnabled: cfg.ConversationLog.GlobalEnabled,
				GlobalPath:    cfg.ConversationLog.GlobalPath,
				QueueSize:     cfg.ConversationLog.QueueSize,
			}, logger)
			if err != nil {
				slog.Error("Failed to initialize conversation logger", "error", err)
				os.Exit(1)
			}

			// Initialize agent handler with gRPC client
			agentHandler, err = agent.NewHandlerWithGrpcClientAndConfig(mgr.Client(), repo, sidebarChan, grpcClient, conversationLogger, cfg)
			if err != nil {
				slog.Error("Failed to initialize agent handler with gRPC", "error", err)
				os.Exit(1)
			}
			defer agentHandler.Close()

			// Initialize terminal monitor with OSC 133 support and fallback detection
			terminalMonitor := terminal.NewMonitor(agentHandler.GetService(), sidebarChan, logger)
			wsHandler.SetMonitor(terminalMonitor)
			slog.Info("Terminal monitor initialized with OSC 133 support")
		}
	}
	if !aiEnabled {
		slog.Info("AI features disabled (PYTHON_AGENT_ADDR not set or connection failed)")
	}

	// Create container handler with AI enabled flag, config, and optional agent session reset support.
	var sessionResetter interface {
		ResetSession(ctx context.Context, userID, sessionID string) error
	}
	if agentHandler != nil {
		sessionResetter = agentHandler.GetService()
	}
	containerHandler := api.NewContainerHandlerWithAIConfigAndSessionReset(baseHandler, aiEnabled, cfg, sessionResetter)

	// Setup router.
	r := chi.NewRouter()

	// Global middleware.
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Heartbeat("/health"))
	r.Use(middleware.CORS([]string{"*"}))
	r.Use(identity.Middleware(repo, cfg.IsDevelopment()))

	// Public routes.
	healthHandler.RegisterHealth(r)

	// All routes use identity middleware (no auth needed).
	containerHandler.RegisterRoutes(r)

	// Agent routes (only if AI is enabled)
	if agentHandler != nil {
		agentHandler.RegisterRoutes(r)
	}

	// WebSocket endpoint.
	r.Get("/ws/terminal", wsHandler.ServeHTTP)

	// Serve embedded frontend (SPA catch-all).
	r.Handle("/*", web.SPAHandler())

	// Create server.
	// Note: SSE connections require long timeouts (no WriteTimeout)
	// Keepalive runs every 10s to maintain connection
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0,                 // 0 = no timeout for SSE support
		IdleTimeout:  120 * time.Second, // 2 minutes for idle connections
	}

	// Start TTL worker.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	container.StartTTLWorkerWithConfig(ctx, repo, mgr, cfg.SessionTTL, sm.CloseSession, cfg)
	slog.Info("TTL worker started", "session_ttl", cfg.SessionTTL)

	// Start server.
	go func() {
		slog.Info("Server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal.
	<-ctx.Done()
	stop()

	slog.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped successfully")
}
