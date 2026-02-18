# Coding Conventions

**Analysis Date:** 2026-02-18

## Naming Patterns

**Files:**
- Go files use `snake_case.go` for implementation files
- Test files use `snake_case_test.go` pattern (co-located with implementation)
- Benchmark files use `benchmark_test.go` suffix (e.g., `monitor_benchmark_test.go`)
- No separate `__tests__` directories - tests live alongside code

**Functions:**
- Public functions use `PascalCase` (e.g., `NewHandler`, `EnsureContainer`)
- Private functions use `camelCase` (e.g., `detectExitCode`, `extractPWDFromOutput`)
- Constructor pattern: `New{Type}` (e.g., `NewDockerManager`, `NewSessionManager`)
- Interface naming: `{Action}er` or `{Noun}` (e.g., `Manager`, `AgentProcessor`)

**Variables:**
- Local variables use `camelCase` (e.g., `containerName`, `volumeName`)
- Constants use `PascalCase` or `SCREAMING_SNAKE_CASE` for private/public
- Package-level constants defined at top of files
- Short variable names acceptable for small scopes (e.g., `w` for `http.ResponseWriter`)

**Types:**
- Structs use `PascalCase` (e.g., `DockerManager`, `Handler`)
- Interfaces typically end in `-er` (e.g., `Manager`, `Processor`)
- Type aliases use `PascalCase`

## Code Style

**Formatting:**
- Standard Go formatting enforced via `gofmt`
- Import organization via `goimports`
- Line length: no strict limit, but be reasonable
- Use tabs for indentation

**Linting:**
- Tool: `golangci-lint` v2.6 with extensive configuration (`.golangci.yml`)
- Enabled linters: govet, errcheck, staticcheck, ineffassign, unused, unconvert
- Style linters: misspell, goconst, nakedret, dogsled, godot, revive
- Security: gosec
- Complexity: gocyclo (max 20), nestif (max 5), gocognit (max 30)
- Error handling: errorlint, err113

**Key Linting Rules:**
- Cyclomatic complexity max: 20
- Cognitive complexity max: 30
- Nesting complexity max: 5
- No naked returns
- No unused parameters
- Error strings should not be capitalized (except when starting with proper nouns)

## Import Organization

**Order:**
1. Standard library imports
2. Third-party imports
3. Internal project imports (grouped by path)

**Example:**
```go
import (
    "context"
    "encoding/json"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/coder/websocket"
    "github.com/docker/docker/client"

    "github.com/ashureev/shsh-labs/internal/container"
    "github.com/ashureev/shsh-labs/internal/store"
    "github.com/ashureev/shsh-labs/internal/terminal"
)
```

**Path Aliases:**
- No path aliases used in this codebase
- Full import paths used throughout

## Error Handling

**Patterns:**
- Errors wrapped with context using `fmt.Errorf` with `%w` verb
- Error messages: lowercase, no punctuation at end
- Example: `fmt.Errorf("create docker client: %w", err)`

**Error Return Style:**
```go
if err != nil {
    return nil, fmt.Errorf("contextual description: %w", err)
}
```

**Sentinel Errors:**
- Not explicitly defined in current codebase
- Uses error wrapping throughout

**Logging Errors:**
- Use structured logging with `log/slog`
- Log at appropriate level (Warn, Error)
- Include structured fields: `slog.Warn("failed to update signal", "user_id", userID, "error", err)`

## Logging

**Framework:** `log/slog` (standard library)

**Patterns:**
- Structured logging with key-value pairs
- Use constants for log keys when repeated
- Log at startup with configuration details
- Example: `slog.Info("Docker client initialized", "runtime", runtime)`

**Log Levels:**
- `Info`: Startup messages, major state changes
- `Warn`: Recoverable errors, signal update failures
- `Error`: (Not explicitly seen in samples, but implied)

## Comments

**When to Comment:**
- Package comments at top of file explain purpose
- Public APIs have descriptive comments
- Complex logic explained inline
- Example: `// Package api provides HTTP handlers for the SHSH API.`

**Style:**
- Complete sentences with proper punctuation
- Start with capital letter
- Explain "why" not "what"

**Code Section Comments:**
```go
// Container configuration.
const (
    imageName       = "playground:latest"
    containerUser   = "1000"
)
```

## Function Design

**Size:**
- Functions should be focused and under 50 lines when possible
- Complexity limits enforced by linter (gocyclo: 20)
- Some functions naturally longer (e.g., `EnsureContainer` in `manager.go`)

**Parameters:**
- Context as first parameter: `ctx context.Context`
- Structs for related parameters
- Options pattern for optional configuration

**Return Values:**
- Return concrete types, accept interfaces
- Error always last return value
- Named returns used sparingly

## Module Design

**Package Structure:**
- Clear package purposes (e.g., `container`, `terminal`, `api`)
- Interface definitions in consuming packages or separate files
- Implementation details unexported

**Exports:**
- Minimal public API surface
- Constructor functions return interfaces when appropriate
- Types capitalized to export, lowercase to hide

**Barrel Files:**
- Not used in Go codebase
- Each file explicitly imports what it needs

## Frontend Conventions (React/JavaScript)

**Files:**
- Components use `PascalCase.jsx` (e.g., `TerminalSession.bench.test.jsx`)
- Tests co-located with `.test.jsx` suffix

**Linting:**
- ESLint with React Hooks and React Refresh plugins
- Ignores: `dist`, `web/dist`
- Rule: `no-unused-vars` with pattern `^[A-Z_]` for ignored vars

**Build:**
- Vite for development and production builds
- Vitest for testing
- Tailwind CSS for styling

---

*Convention analysis: 2026-02-18*
