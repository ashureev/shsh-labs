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

	"github.com/ashureev/shsh-labs/internal/api"
	"github.com/ashureev/shsh-labs/internal/config"
	"github.com/ashureev/shsh-labs/internal/container"
	"github.com/ashureev/shsh-labs/internal/identity"
	"github.com/ashureev/shsh-labs/internal/llm"
	"github.com/ashureev/shsh-labs/internal/middleware"
	"github.com/ashureev/shsh-labs/internal/store"
	"github.com/ashureev/shsh-labs/internal/terminal"
	"github.com/ashureev/shsh-labs/internal/tutor"
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

	slog.Info("Starting SHSH local server", "port", cfg.Port, "dev", cfg.IsDevelopment())

	// Initialize SQLite Database (WAL Mode)
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
	slog.Info("Database connected (WAL mode active)")

	// Initialize Docker Container Manager
	mgr, err := container.NewDockerManagerWithConfig(cfg)
	if err != nil {
		slog.Error("Failed to initialize container manager", "error", err)
		os.Exit(1)
	}
	slog.Info("Container manager initialized")

	// Ensure bridge network exists
	networkID, err := mgr.EnsureNetwork(context.Background())
	if err != nil {
		slog.Error("Failed to ensure playground network", "error", err)
		os.Exit(1)
	}
	slog.Info("Playground network ready", "network_id", networkID)

	// Determine Initial LLM Provider
	llmCfg := initLLMConfig(repo)
	llmProvider, err := llm.NewProvider(llmCfg)
	if err != nil {
		slog.Warn("Failed to initialize LLM provider", "error", err)
	} else {
		slog.Info("LLM provider initialized", "provider", llmCfg.Provider, "model", llmCfg.Model)
	}

	// Initialize AI Tutor Engine with Sandbox Tool Runner
	tutorEngine := tutor.NewEngine(llmProvider, mgr, tutor.DefaultConfig())
	slog.Info("AI Tutor Engine initialized with sandbox tools")

	// Initialize session manager
	sm := terminal.NewSessionManager()

	// Initialize HTTP and WebSocket Handlers
	baseHandler := api.NewHandler(repo, mgr, sm, cfg.FrontendURL)
	healthHandler := api.NewHealthHandlerWithConfig(repo, cfg)
	containerHandler := api.NewContainerHandlerWithAIConfigAndSessionReset(baseHandler, true, cfg, nil)
	tutorHandler := api.NewTutorHandler(tutorEngine, repo, sm)
	settingsHandler := api.NewSettingsHandler(repo, tutorEngine)
	wsHandler := terminal.NewWebSocketHandler(repo, mgr, sm, tutorEngine, cfg.FrontendURL, cfg.IsDevelopment())

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Heartbeat("/health"))
	r.Use(middleware.CORS([]string{"*"}))
	r.Use(identity.Middleware(repo, cfg.IsDevelopment()))

	// Public routes
	healthHandler.RegisterHealth(r)

	// API routes
	r.Route("/api", func(apiRouter chi.Router) {
		containerHandler.RegisterRoutes(apiRouter)
		tutorHandler.RegisterRoutes(apiRouter)
		settingsHandler.RegisterRoutes(apiRouter)
	})

	// WebSocket endpoint
	r.Get("/ws/terminal", wsHandler.ServeHTTP)

	// Serve embedded frontend (SPA catch-all)
	r.Handle("/*", web.SPAHandler())

	// Create server with unbounded SSE write timeout
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	// Start TTL worker
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	container.StartTTLWorkerWithConfig(ctx, repo, mgr, cfg.SessionTTL, sm.CloseSession, cfg)
	slog.Info("TTL worker started", "session_ttl", cfg.SessionTTL)

	// Start server
	go func() {
		slog.Info("Server listening on http://localhost:"+cfg.Port, "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
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

func initLLMConfig(repo store.Repository) llm.ProviderConfig {
	ctx := context.Background()
	dbProvider, _ := repo.GetSetting(ctx, "llm_provider")
	dbModel, _ := repo.GetSetting(ctx, "llm_model")
	dbAPIKey, _ := repo.GetSetting(ctx, "llm_api_key")
	dbBaseURL, _ := repo.GetSetting(ctx, "llm_base_url")

	if dbProvider != "" {
		return llm.ProviderConfig{
			Provider: dbProvider,
			Model:    dbModel,
			APIKey:   dbAPIKey,
			BaseURL:  dbBaseURL,
		}
	}

	// Fallback to environment variables
	if geminiKey := os.Getenv("GOOGLE_API_KEY"); geminiKey != "" {
		model := os.Getenv("LLM_MODEL")
		if model == "" {
			model = "gemini-2.5-flash"
		}
		return llm.DefaultGeminiConfig(geminiKey, model)
	}

	if openAIKey := os.Getenv("OPENAI_API_KEY"); openAIKey != "" {
		model := os.Getenv("LLM_MODEL")
		if model == "" {
			model = "gpt-4o-mini"
		}
		return llm.ProviderConfig{
			Provider: "openai",
			APIKey:   openAIKey,
			Model:    model,
		}
	}

	// Default to Ollama local
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "llama3.2"
	}
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434/v1"
	}

	return llm.ProviderConfig{
		Provider: "ollama",
		BaseURL:  baseURL,
		APIKey:   "ollama",
		Model:    model,
	}
}
