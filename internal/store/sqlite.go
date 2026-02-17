package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ashureev/shsh-labs/internal/domain"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements Repository using SQLite.
type SQLiteStore struct {
	db             *sql.DB
	agentSessionMu sync.Mutex // Mutex for agent session operations to prevent SQLITE_BUSY
}

// NewSQLite creates a new SQLite-backed repository.
func NewSQLite(dbPath string) (Repository, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	// Open database with WAL mode for better concurrency.
	dsn := dbPath + "?_journal=WAL&_sync=NORMAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("initialize schema: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) initSchema() error {
	query := `
	PRAGMA busy_timeout = 5000;
	CREATE TABLE IF NOT EXISTS users (
		user_id TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		container_id TEXT,
		last_seen_at INTEGER NOT NULL,
		volume_path TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_last_seen ON users(last_seen_at) WHERE container_id IS NOT NULL;

	CREATE TABLE IF NOT EXISTS agent_sessions (
		user_id TEXT PRIMARY KEY,
		last_proactive_msg INTEGER,
		attempt_count INTEGER DEFAULT 0,
		just_self_corrected INTEGER DEFAULT 0,
		is_typing INTEGER DEFAULT 0,
		challenge_json TEXT,
		messages_json TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_agent_sessions_updated ON agent_sessions(updated_at);
	`
	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}

// Ping verifies database connectivity.
func (s *SQLiteStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// GetUser retrieves a user by their user ID.
func (s *SQLiteStore) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	query := `
		SELECT user_id, username, container_id,
		       last_seen_at, volume_path, created_at, updated_at 
		FROM users WHERE user_id = ?`

	row := s.db.QueryRowContext(ctx, query, userID)

	var user domain.User
	var containerID sql.NullString
	var lastSeen, createdAt, updatedAt int64

	err := row.Scan(
		&user.UserID, &user.Username, &containerID,
		&lastSeen, &user.VolumePath, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan user row: %w", err)
	}

	user.ContainerID = containerID.String
	user.LastSeenAt = time.Unix(lastSeen, 0)
	user.CreatedAt = time.Unix(createdAt, 0)
	user.UpdatedAt = time.Unix(updatedAt, 0)

	return &user, nil
}

// UpsertUser creates or updates a user record.
func (s *SQLiteStore) UpsertUser(ctx context.Context, user *domain.User) error {
	query := `
	INSERT INTO users (user_id, username, container_id, last_seen_at, volume_path, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id) DO UPDATE SET
		username = excluded.username,
		last_seen_at = excluded.last_seen_at,
		updated_at = excluded.updated_at`

	var containerID interface{}
	if user.ContainerID != "" {
		containerID = user.ContainerID
	}

	_, err := s.db.ExecContext(ctx, query,
		user.UserID, user.Username, containerID,
		user.LastSeenAt.Unix(), user.VolumePath,
		user.CreatedAt.Unix(), user.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

// UpdateLastSeen updates the last_seen_at timestamp for a user.
func (s *SQLiteStore) UpdateLastSeen(ctx context.Context, userID string, lastSeen time.Time) error {
	query := `UPDATE users SET last_seen_at = ?, updated_at = ? WHERE user_id = ?`
	result, err := s.db.ExecContext(ctx, query, lastSeen.Unix(), time.Now().Unix(), userID)
	if err != nil {
		return fmt.Errorf("update last_seen: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		slog.Warn("UpdateLastSeen affected 0 rows", "user_id", userID)
	}

	return nil
}

// UpdateContainerID updates the container_id for a user.
func (s *SQLiteStore) UpdateContainerID(ctx context.Context, userID string, containerID string, expectedID string) error {
	query := `UPDATE users SET container_id = ?, updated_at = ? WHERE user_id = ?`
	args := []interface{}{nil, time.Now().Unix(), userID}

	if containerID != "" {
		args[0] = containerID
	}

	if expectedID != "" {
		query += ` AND container_id = ?`
		args = append(args, expectedID)
	}

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update container_id: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		slog.Warn("UpdateContainerID affected 0 rows", "user_id", userID, "expected_id", expectedID)
		if expectedID != "" {
			return fmt.Errorf("optimistic lock failed: container_id does not match expected_id")
		}
		return fmt.Errorf("user not found")
	}

	return nil
}

// GetExpiredSessions retrieves users whose containers have exceeded the inactivity TTL.
func (s *SQLiteStore) GetExpiredSessions(ctx context.Context, ttl time.Duration) ([]*domain.User, error) {
	threshold := time.Now().Add(-ttl).Unix()
	query := `
		SELECT user_id, username, container_id,
		       last_seen_at, volume_path, created_at, updated_at 
		FROM users WHERE container_id IS NOT NULL AND last_seen_at < ?`

	rows, err := s.db.QueryContext(ctx, query, threshold)
	if err != nil {
		return nil, fmt.Errorf("query expired sessions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("failed to close expired sessions rows", "error", closeErr)
		}
	}()

	var users []*domain.User
	for rows.Next() {
		var user domain.User
		var containerID sql.NullString
		var lastSeen, createdAt, updatedAt int64

		if err := rows.Scan(
			&user.UserID, &user.Username, &containerID,
			&lastSeen, &user.VolumePath, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan expired session row: %w", err)
		}

		user.ContainerID = containerID.String
		user.LastSeenAt = time.Unix(lastSeen, 0)
		user.CreatedAt = time.Unix(createdAt, 0)
		user.UpdatedAt = time.Unix(updatedAt, 0)
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired sessions: %w", err)
	}

	return users, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}

// GetAgentSession retrieves agent session state for a user.
func (s *SQLiteStore) GetAgentSession(ctx context.Context, userID string) (*domain.AgentSession, error) {
	s.agentSessionMu.Lock()
	defer s.agentSessionMu.Unlock()

	query := `
		SELECT user_id, last_proactive_msg, attempt_count, just_self_corrected,
		       is_typing, challenge_json, messages_json, created_at, updated_at
		FROM agent_sessions WHERE user_id = ?`

	row := s.db.QueryRowContext(ctx, query, userID)

	var session domain.AgentSession
	var lastProactiveMsg sql.NullInt64
	var challengeJSON sql.NullString
	var messagesJSON string
	var createdAt, updatedAt int64

	err := row.Scan(
		&session.UserID, &lastProactiveMsg, &session.AttemptCount,
		&session.JustSelfCorrected, &session.IsTyping,
		&challengeJSON, &messagesJSON,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan agent session: %w", err)
	}

	session.CreatedAt = time.Unix(createdAt, 0)
	session.UpdatedAt = time.Unix(updatedAt, 0)

	if lastProactiveMsg.Valid {
		ts := time.Unix(lastProactiveMsg.Int64, 0)
		session.LastProactiveMsg = &ts
	}
	if challengeJSON.Valid {
		session.ChallengeJSON = &challengeJSON.String
	}
	session.MessagesJSON = messagesJSON

	return &session, nil
}

// UpsertAgentSession creates or updates agent session state.
func (s *SQLiteStore) UpsertAgentSession(ctx context.Context, session *domain.AgentSession) error {
	s.agentSessionMu.Lock()
	defer s.agentSessionMu.Unlock()

	query := `
		INSERT INTO agent_sessions (
			user_id, last_proactive_msg, attempt_count, just_self_corrected,
			is_typing, challenge_json, messages_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			last_proactive_msg = COALESCE(excluded.last_proactive_msg, agent_sessions.last_proactive_msg),
			attempt_count = excluded.attempt_count,
			just_self_corrected = excluded.just_self_corrected,
			is_typing = excluded.is_typing,
			challenge_json = COALESCE(excluded.challenge_json, agent_sessions.challenge_json),
			messages_json = excluded.messages_json,
			updated_at = excluded.updated_at`

	var lastProactiveMsg interface{}
	if session.LastProactiveMsg != nil {
		lastProactiveMsg = session.LastProactiveMsg.Unix()
	}

	var challengeJSON interface{}
	if session.ChallengeJSON != nil {
		challengeJSON = *session.ChallengeJSON
	}

	_, err := s.db.ExecContext(ctx, query,
		session.UserID, lastProactiveMsg, session.AttemptCount,
		session.JustSelfCorrected, session.IsTyping,
		challengeJSON, session.MessagesJSON,
		session.CreatedAt.Unix(), time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("upsert agent session: %w", err)
	}
	return nil
}

// DeleteAgentSession removes agent session state.
// Implements retry logic with exponential backoff to handle SQLITE_BUSY errors.
func (s *SQLiteStore) DeleteAgentSession(ctx context.Context, userID string) error {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := s.deleteAgentSessionOnce(ctx, userID)
		if err == nil {
			return nil
		}

		// Check if it's a SQLITE_BUSY error
		if strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY") {
			if i < maxRetries-1 {
				delay := baseDelay * time.Duration(1<<i) // exponential backoff: 100ms, 200ms, 400ms
				slog.Debug("DeleteAgentSession failed with SQLITE_BUSY, retrying",
					"user_id", userID,
					"attempt", i+1,
					"delay", delay)
				time.Sleep(delay)
				continue
			}
		}

		// Non-retryable error or max retries exceeded
		return fmt.Errorf("failed to delete agent session for %s after %d attempts: %w", userID, maxRetries, err)
	}

	return nil
}

// deleteAgentSessionOnce performs a single delete attempt.
func (s *SQLiteStore) deleteAgentSessionOnce(ctx context.Context, userID string) error {
	s.agentSessionMu.Lock()
	defer s.agentSessionMu.Unlock()

	query := `DELETE FROM agent_sessions WHERE user_id = ?`
	_, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("delete agent session: %w", err)
	}
	return nil
}

// CleanupExpiredSessions removes sessions older than TTL.
func (s *SQLiteStore) CleanupExpiredSessions(ctx context.Context, ttl time.Duration) (int64, error) {
	threshold := time.Now().Add(-ttl).Unix()
	query := `DELETE FROM agent_sessions WHERE updated_at < ?`
	result, err := s.db.ExecContext(ctx, query, threshold)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired sessions: %w", err)
	}
	return result.RowsAffected()
}

// DeleteLegacyLocalState removes pre-migration single-user local records.
func (s *SQLiteStore) DeleteLegacyLocalState(ctx context.Context) (int64, int64, error) {
	s.agentSessionMu.Lock()
	defer s.agentSessionMu.Unlock()

	userRes, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE user_id = ?`, "local")
	if err != nil {
		return 0, 0, fmt.Errorf("delete legacy local user: %w", err)
	}
	userRows, err := userRes.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("legacy local user rows affected: %w", err)
	}

	agentRes, err := s.db.ExecContext(ctx, `DELETE FROM agent_sessions WHERE user_id = ?`, "local")
	if err != nil {
		return 0, 0, fmt.Errorf("delete legacy local agent session: %w", err)
	}
	agentRows, err := agentRes.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("legacy local agent session rows affected: %w", err)
	}

	return userRows, agentRows, nil
}
