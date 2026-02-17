// Package config provides application configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Port             string
	FrontendURL      string
	DBPath           string
	SessionTTL       time.Duration
	ContainerRuntime string // Docker runtime: "" = default (runc), "runsc" = gVisor
	ConversationLog  ConversationLogConfig
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
