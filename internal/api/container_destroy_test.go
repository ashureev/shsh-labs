//nolint:revive // "api" package name is intentionally concise for this layer.
package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ashureev/shsh-labs/internal/domain"
	"github.com/ashureev/shsh-labs/internal/identity"
	"github.com/ashureev/shsh-labs/internal/terminal"
	"github.com/docker/docker/client"
)

type fakeRepo struct {
	mu    sync.Mutex
	users map[string]*domain.User
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: make(map[string]*domain.User)}
}

func (f *fakeRepo) GetUser(_ context.Context, userID string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	user := f.users[userID]
	if user == nil {
		return nil, nil
	}
	copy := *user
	return &copy, nil
}

func (f *fakeRepo) UpsertUser(_ context.Context, user *domain.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	copy := *user
	f.users[user.UserID] = &copy
	return nil
}

func (f *fakeRepo) UpdateLastSeen(_ context.Context, _ string, _ time.Time) error { return nil }

func (f *fakeRepo) UpdateContainerID(_ context.Context, userID string, containerID string, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	user := f.users[userID]
	if user == nil {
		return nil
	}
	user.ContainerID = containerID
	user.UpdatedAt = time.Now()
	return nil
}

func (f *fakeRepo) GetExpiredSessions(_ context.Context, _ time.Duration) ([]*domain.User, error) {
	return nil, nil
}

func (f *fakeRepo) Ping(_ context.Context) error { return nil }
func (f *fakeRepo) Close() error                 { return nil }

func (f *fakeRepo) GetAgentSession(_ context.Context, _ string) (*domain.AgentSession, error) {
	return nil, nil
}
func (f *fakeRepo) UpsertAgentSession(_ context.Context, _ *domain.AgentSession) error { return nil }
func (f *fakeRepo) DeleteAgentSession(_ context.Context, _ string) error               { return nil }
func (f *fakeRepo) CleanupExpiredSessions(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}
func (f *fakeRepo) DeleteLegacyLocalState(_ context.Context) (int64, int64, error) { return 0, 0, nil }

type fakeManager struct{}

func (f *fakeManager) EnsureContainer(context.Context, string, string, time.Time, map[string]string) (string, error) {
	return "", nil
}
func (f *fakeManager) StopContainer(context.Context, string) error     { return nil }
func (f *fakeManager) IsRunning(context.Context, string) (bool, error) { return false, nil }
func (f *fakeManager) CreateExecSession(context.Context, string) (string, io.ReadWriteCloser, error) {
	return "", nil, nil
}
func (f *fakeManager) ResizeExecSession(context.Context, string, uint, uint) error { return nil }
func (f *fakeManager) Client() *client.Client                                      { return nil }
func (f *fakeManager) EnsureNetwork(context.Context) (string, error)               { return "", nil }

type fakeSessionResetter struct {
	mu          sync.Mutex
	calls       int
	lastUserID  string
	lastSession string
	err         error
}

func (f *fakeSessionResetter) ResetSession(_ context.Context, userID, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastUserID = userID
	f.lastSession = sessionID
	return f.err
}

func (f *fakeSessionResetter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeSessionResetter) lastSessionID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastSession
}

func TestDestroyResetsCurrentSession(t *testing.T) {
	repo := newFakeRepo()
	base := NewHandler(repo, &fakeManager{}, terminal.NewSessionManager(), "")
	resetter := &fakeSessionResetter{}
	handler := NewContainerHandlerWithAIConfigAndSessionReset(base, true, nil, resetter)

	req := httptest.NewRequest(http.MethodPost, "/api/destroy", nil)
	req.Header.Set(identity.SessionHeaderName, "tab-ephemeral")
	rr := httptest.NewRecorder()

	mw := identity.Middleware(repo, true)
	mw(http.HandlerFunc(handler.Destroy)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if resetter.callCount() != 1 {
		t.Fatalf("expected exactly one immediate reset call, got %d", resetter.callCount())
	}
	if resetter.lastSessionID() != "tab-ephemeral" {
		t.Fatalf("expected session id tab-ephemeral, got %q", resetter.lastSessionID())
	}
}

func TestDestroyReturnsSuccessWhenResetFails(t *testing.T) {
	repo := newFakeRepo()
	base := NewHandler(repo, &fakeManager{}, terminal.NewSessionManager(), "")
	resetter := &fakeSessionResetter{err: errors.New("agent unavailable")}
	handler := NewContainerHandlerWithAIConfigAndSessionReset(base, true, nil, resetter)

	req := httptest.NewRequest(http.MethodPost, "/api/destroy", nil)
	req.Header.Set(identity.SessionHeaderName, "tab-ephemeral")
	rr := httptest.NewRecorder()

	mw := identity.Middleware(repo, true)
	mw(http.HandlerFunc(handler.Destroy)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if resetter.callCount() < 1 {
		t.Fatalf("expected at least one reset attempt, got %d", resetter.callCount())
	}
}
