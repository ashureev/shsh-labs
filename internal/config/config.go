// Package config provides application configuration.
//
// Configuration is loaded from environment variables with sensible defaults.
// All timeouts and operational parameters are configurable.
//
// Configuration categories:
//   - Timeouts: Container stop/create, health checks, cleanup, TTL worker
//   - Resources: Memory limits, CPU quotas, PIDs limits
//   - Rate Limiting: Request limits per time window
//   - SSE: Server-Sent Events retry and keepalive settings
//   - Retry: Database retry attempts and delays
//
// For a complete list of all environment variables, see .env.example
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// TimeoutConfig holds timeout-related configuration.
type TimeoutConfig struct {
	ContainerStop     time.Duration // Container stop timeout
	ContainerCreate   time.Duration // Container create timeout
	HealthCheck       time.Duration // Health check DB timeout
	DestroyCleanup    time.Duration // Background destroy timeout
	TTLWorkerInterval time.Duration // TTL cleanup worker interval
}

// ContainerConfig holds container resource and retry configuration.
type ContainerConfig struct {
	MemoryLimitBytes    int64         // Memory limit in bytes (default: 512MB)
	CPUQuota            int64         // CPU quota (default: 50000 = 0.5 CPU)
	PidsLimit           int64         // PIDs limit (default: 256)
	CreateRetryAttempts int           // Container create retry attempts (default: 20)
	CreateRetryDelay    time.Duration // Delay between create retries (default: 250ms)
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	RequestsPerWindow int           // Max requests per window (default: 10)
	WindowDuration    time.Duration // Rate limit window (default: 1m)
}

// SSEConfig holds Server-Sent Events configuration.
type SSEConfig struct {
	MaxRequestBodySize int64         // Max request body size in bytes (default: 1MB)
	RetryDelay         time.Duration // SSE retry delay (default: 5s)
	KeepaliveInterval  time.Duration // SSE keepalive interval (default: 10s)
}

// RetryConfig holds retry-related configuration.
type RetryConfig struct {
	DatabaseMaxRetries     int           // Max database retry attempts (default: 3)
	DatabaseRetryBaseDelay time.Duration // Base delay for DB retries (default: 50ms)
}

// Config holds all application configuration.
type Config struct {
	Port             string
	FrontendURL      string
	DBPath           string
	SessionTTL       time.Duration
	ContainerRuntime string // Docker runtime: "" = default (runc), "runsc" = gVisor
	ConversationLog  ConversationLogConfig
	Timeout          TimeoutConfig
	Container        ContainerConfig
	RateLimit        RateLimitConfig
	SSE              SSEConfig
	Retry            RetryConfig
}

// ConversationLogConfig controls JSON conversation logging.
type ConversationLogConfig struct {
	Enabled       bool
	Dir           string
	GlobalEnabled bool
	GlobalPath    string
	QueueSize     int
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	queueSize := getEnvInt("CONVERSATION_LOG_QUEUE_SIZE", 1000)
	if queueSize <= 0 {
		queueSize = 1000
	}

	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		FrontendURL:      getEnv("FRONTEND_URL", ""),
		DBPath:           getEnv("DB_PATH", "./data/playground.db"),
		SessionTTL:       60 * time.Minute,
		ContainerRuntime: getEnv("CONTAINER_RUNTIME", ""),
		ConversationLog: ConversationLogConfig{
			Enabled:       getEnvBool("CONVERSATION_LOG_ENABLED", true),
			Dir:           getEnv("CONVERSATION_LOG_DIR", "./data/logs/conversations"),
			GlobalEnabled: getEnvBool("CONVERSATION_LOG_GLOBAL_ENABLED", false),
			GlobalPath:    getEnv("CONVERSATION_LOG_GLOBAL_PATH", "./data/logs/conversations/all.ndjson"),
			QueueSize:     queueSize,
		},
		Timeout: TimeoutConfig{
			ContainerStop:     getEnvDuration("SHSH_CONTAINER_STOP_TIMEOUT", 10*time.Second),
			ContainerCreate:   getEnvDuration("SHSH_CONTAINER_CREATE_TIMEOUT", 2*time.Minute),
			HealthCheck:       getEnvDuration("SHSH_HEALTH_CHECK_TIMEOUT", 5*time.Second),
			DestroyCleanup:    getEnvDuration("SHSH_DESTROY_CLEANUP_TIMEOUT", 30*time.Second),
			TTLWorkerInterval: getEnvDuration("SHSH_TTL_WORKER_INTERVAL", 5*time.Minute),
		},
		Container: ContainerConfig{
			MemoryLimitBytes:    getEnvInt64("SHSH_CONTAINER_MEMORY_LIMIT", 512*1024*1024),
			CPUQuota:            getEnvInt64("SHSH_CONTAINER_CPU_QUOTA", 50000),
			PidsLimit:           getEnvInt64("SHSH_CONTAINER_PIDS_LIMIT", 256),
			CreateRetryAttempts: getEnvInt("SHSH_CONTAINER_CREATE_RETRY_ATTEMPTS", 20),
			CreateRetryDelay:    getEnvDuration("SHSH_CONTAINER_CREATE_RETRY_DELAY", 250*time.Millisecond),
		},
		RateLimit: RateLimitConfig{
			RequestsPerWindow: getEnvInt("SHSH_RATE_LIMIT_REQUESTS", 10),
			WindowDuration:    getEnvDuration("SHSH_RATE_LIMIT_WINDOW", time.Minute),
		},
		SSE: SSEConfig{
			MaxRequestBodySize: getEnvInt64("SHSH_SSE_MAX_BODY_SIZE", 1<<20), // 1MB
			RetryDelay:         getEnvDuration("SHSH_SSE_RETRY_DELAY", 5*time.Second),
			KeepaliveInterval:  getEnvDuration("SHSH_SSE_KEEPALIVE_INTERVAL", 10*time.Second),
		},
		Retry: RetryConfig{
			DatabaseMaxRetries:     getEnvInt("SHSH_DB_MAX_RETRIES", 3),
			DatabaseRetryBaseDelay: getEnvDuration("SHSH_DB_RETRY_BASE_DELAY", 50*time.Millisecond),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate checks that all required configuration fields are set.
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("PORT cannot be empty")
	}
	if c.DBPath == "" {
		return fmt.Errorf("DB_PATH cannot be empty")
	}
	if c.ConversationLog.Dir == "" {
		return fmt.Errorf("CONVERSATION_LOG_DIR cannot be empty")
	}
	if c.ConversationLog.GlobalPath == "" {
		return fmt.Errorf("CONVERSATION_LOG_GLOBAL_PATH cannot be empty")
	}
	if c.ConversationLog.QueueSize <= 0 {
		return fmt.Errorf("CONVERSATION_LOG_QUEUE_SIZE must be > 0")
	}
	return nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.FrontendURL == "" ||
		strings.Contains(c.FrontendURL, "localhost") ||
		strings.Contains(c.FrontendURL, "127.0.0.1")
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return n
}

func getEnvInt64(key string, fallback int64) int64 {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	d, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return d
}

// IsContainer returns true if running inside a Docker container.
func IsContainer() bool {
	if os.Getenv("CONTAINER") == "true" {
		return true
	}
	// Check for .dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}
