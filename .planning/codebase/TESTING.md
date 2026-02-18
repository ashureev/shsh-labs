# Testing Patterns

**Analysis Date:** 2026-02-18

## Test Framework

**Go (Backend):**
- Runner: Go's built-in `testing` package (standard library)
- No external test framework - uses standard `testing.T`
- Assertion style: Manual comparisons with `t.Errorf()`, `t.Fatalf()`
- Parallel execution: `t.Parallel()` used in appropriate tests

**Frontend (React):**
- Runner: Vitest (configured in `vite.config.js`)
- Environment: jsdom
- Globals: Enabled
- Command: `npm run test` or `npm run test -- --run` (CI)

**Python (Agent):**
- Runner: pytest
- Async support: pytest-asyncio
- Command: `pytest python-agent/tests -q`

## Test File Organization

**Location:**
- Go: Co-located with implementation files (e.g., `handler.go` + `handler_test.go`)
- Frontend: Co-located in `src/` directory (e.g., `basic.test.jsx`)
- Python: Separate `tests/` directory within `python-agent/`

**Naming:**
- Go: `*_test.go` for unit tests, `*_benchmark_test.go` for benchmarks
- Frontend: `*.test.jsx` or `*.bench.test.jsx`
- Python: `test_*.py`

**Structure:**
```
internal/
├── agent/
│   ├── conversation_logger.go
│   └── conversation_logger_test.go
├── api/
│   ├── handler.go
│   └── handler_test.go
├── terminal/
│   ├── manager.go
│   ├── manager_test.go
│   ├── monitor_benchmark_test.go
│   ├── monitor_exit_test.go
│   ├── osc133_parser_test.go
│   └── ptycontroller_test.go
```

## Test Structure

**Go Test Pattern:**
```go
func TestFunctionName(t *testing.T) {
    // Setup
    sm := NewSessionManager()
    conn := &websocket.Conn{}
    userID := "user123"
    sessionID := "tab-1"

    // Execute
    sm.Register(userID, sessionID, conn)
    active := sm.GetActive(userID, sessionID)

    // Assert
    if active != conn {
        t.Errorf("Expected connection %v, got %v", conn, active)
    }
}
```

**Table-Driven Tests:**
```go
func TestIsDevelopment(t *testing.T) {
    tests := []struct {
        name        string
        env         string
        frontendURL string
        want        bool
    }{
        {"EnvDev", "development", "", true},
        {"EnvProd", "production", "", false},
        {"URLMatchesLocalhost", "", "http://localhost:3000", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            h := &Handler{frontendRedirectURL: tt.frontendURL}
            if got := h.isDevelopment(); got != tt.want {
                t.Errorf("Handler.isDevelopment() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

**Parallel Tests:**
```go
func TestConversationLoggerWritesPerSessionNDJSON(t *testing.T) {
    t.Parallel()
    // Test implementation
}
```

**HTTP Testing:**
```go
func TestJSON(t *testing.T) {
    w := httptest.NewRecorder()
    data := map[string]string{"foo": "bar"}

    JSON(w, http.StatusOK, data)

    resp := w.Result()
    if resp.StatusCode != http.StatusOK {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
    // ... decode and verify response body
}
```

## Mocking

**Go Mocking Approach:**
- No mocking framework detected
- Uses real implementations with test doubles (e.g., `&websocket.Conn{}`)
- Interface-based design allows for easy mocking

**Example:**
```go
// Using nil for dependencies that aren't used in test
controller := NewPTYController(nil, config, nil)

// Using empty structs for simple cases
sm := NewSessionManager()
conn := &websocket.Conn{}  // Real type, empty instance
```

**What to Mock:**
- External connections (WebSocket, gRPC) - use empty instances
- File system operations - use temporary directories (`t.TempDir()`)
- Time-dependent code - use sleeps or time injection

## Fixtures and Factories

**Test Data:**
- Inline test data defined in test functions
- No separate fixture files for Go
- Helper functions for complex setup (e.g., `waitForLogLine`)

**Temporary Resources:**
```go
dir := t.TempDir()  // Automatic cleanup
logger, err := NewConversationLogger(ConversationLogConfig{
    Enabled:   true,
    Dir:       dir,
    QueueSize: 16,
}, slog.Default())
defer func() { _ = logger.Close() }()
```

**Helper Functions:**
```go
func waitForLogLine(t *testing.T, path string) string {
    t.Helper()
    deadline := time.Now().Add(2 * time.Second)
    for time.Now().Before(deadline) {
        data, err := os.ReadFile(path)
        if err == nil && len(data) > 0 {
            lines := strings.Split(strings.TrimSpace(string(data)), "\n")
            if len(lines) > 0 {
                return lines[len(lines)-1]
            }
        }
        time.Sleep(20 * time.Millisecond)
    }
    t.Fatalf("timed out waiting for log file %s", path)
    return ""
}
```

## Coverage

**Requirements:**
- No explicit coverage target enforced
- Coverage collection available via Makefile: `make test-coverage`
- Generates HTML report: `coverage.html`

**View Coverage:**
```bash
make test-coverage
# Or manually:
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out -o coverage.html
```

## Test Types

**Unit Tests:**
- Fast, isolated tests
- No external dependencies (use nil or empty structs)
- Example: `TestDefaultPTYConfig`, `TestJSON`

**Integration Tests:**
- Marked with build tag: `//go:build integration`
- Run via: `make test-integration` or `go test -tags=integration ./internal/...`
- Tests with real dependencies

**Benchmark Tests:**
```go
func BenchmarkExtractPWDFromOutput(b *testing.B) {
    tm := NewTerminalMonitor(nil, nil, nil)
    userID := "user1"
    sessionID := "session1"
    tm.RegisterSession(userID, sessionID, "container1", "/home/user")
    data := []byte("some output\ncd /tmp\nmore output")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        tm.extractPWDFromOutput(userID, sessionID, data)
    }
}
```

**Concurrent Tests:**
```go
func TestSessionManager_ConcurrentAccess(t *testing.T) {
    sm := NewSessionManager()
    userID := "concurrentUser"

    go func() {
        for i := 0; i < 1000; i++ {
            sm.Register(userID, "tab-"+strconv.Itoa(i), &websocket.Conn{})
        }
    }()

    go func() {
        for i := 0; i < 1000; i++ {
            sm.GetActive(userID, "tab-"+strconv.Itoa(i))
        }
    }()

    time.Sleep(100 * time.Millisecond)
}
```

## Common Patterns

**Async Testing:**
```go
// Use channels or polling with timeouts
deadline := time.Now().Add(2 * time.Second)
for time.Now().Before(deadline) {
    // Check condition
    time.Sleep(20 * time.Millisecond)
}
t.Fatalf("timed out waiting for condition")
```

**Error Testing:**
```go
if err != nil {
    t.Fatalf("NewConversationLogger failed: %v", err)
}

// Or for expected errors:
if err == nil {
    t.Fatal("expected error, got nil")
}
```

**Environment Variable Tests:**
```go
func TestIsDevelopment(t *testing.T) {
    origEnv := os.Getenv("APP_ENV")
    defer os.Setenv("APP_ENV", origEnv)

    os.Setenv("APP_ENV", "development")
    // ... test code
}
```

**Cleanup Pattern:**
```go
defer func() { _ = logger.Close() }()
// Or for temp dirs (automatic):
dir := t.TempDir()
```

## Frontend Testing

**Basic Test:**
```jsx
import { describe, it, expect } from 'vitest';

describe('Frontend Test Infrastructure', () => {
    it('should successfully run a basic test', () => {
        expect(true).toBe(true);
    });
});
```

**Configuration:**
- Vitest configured in `vite.config.js`
- Globals enabled (no need to import `describe`, `it`, `expect`)
- jsdom environment for DOM testing

## CI/CD Integration

**GitHub Actions:**
- Go tests run on every PR and push to main
- Frontend tests optional (manual trigger)
- Python tests optional (manual trigger)

**Workflows:**
- `ci-required.yml`: Runs Go tests, linting, frontend lint, Python ruff
- `full-stack-manual.yml`: Optional test runners for all components

**Quality Gates:**
- All tests must pass
- Linting must pass (golangci-lint, eslint, ruff)
- Build must succeed

---

*Testing analysis: 2026-02-18*
