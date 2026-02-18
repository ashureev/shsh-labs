# Tech Debt Cleanup: Easy Wins

## What This Is

Preventive maintenance to refactor the messiest areas of the SHSH codebase and remove redundant code. Focus on "easy wins" that centralize configuration and reduce complexity without touching critical path logic for terminal sessions.

## Core Value

Terminal sessions must not break — all changes maintain backward compatibility and avoid modifying core terminal monitor state management.

## Requirements

### Validated

(Existing codebase capabilities — inferred from codebase map)

- ✓ Container provisioning via Docker API — existing
- ✓ WebSocket terminal sessions with PTY — existing
- ✓ AI agent integration via gRPC — existing
- ✓ SQLite-based session storage — existing
- ✓ Anonymous user identity management — existing

### Active

- [ ] Centralize magic numbers and timeouts in config package
- [ ] Remove redundant error handling patterns
- [ ] Fix inconsistent mutex unlock paths (defer vs manual)
- [ ] Standardize on structured logging patterns
- [ ] Consolidate duplicate utility functions

### Out of Scope

- Terminal monitor complexity refactor (782 lines) — too risky for terminal sessions
- Nested mutex hierarchy redesign — affects critical path
- gRPC TLS implementation — requires infrastructure changes
- Container name race condition fix — complex locking changes
- WebSocket connection leak fix — affects terminal session lifecycle
- Database layer refactoring — SQLite busy timeout issues

## Context

SHSH is a Linux playground platform with AI tutoring. The codebase has accumulated tech debt:

- Hardcoded timeouts scattered across 10+ files
- Inconsistent error wrapping patterns
- Some manual mutex unlocks alongside deferred ones
- Duplicate helper functions for string parsing
- Mixed logging styles (some with slog, some fmt.Printf remnants)

The codebase uses Clean Architecture with Go backend, Python AI agent, and React frontend. Key files to touch:
- `internal/config/config.go` — extend with timeout configs
- `internal/api/container.go` — redundant error handling
- `internal/agent/handler.go` — magic numbers
- `internal/container/manager.go` — inconsistent patterns

## Constraints

- **Backward Compatibility**: All public APIs unchanged; config additions only
- **Terminal Sessions**: No modifications to `internal/terminal/monitor.go` or `internal/terminal/osc133_parser.go`
- **Testing**: Must pass existing tests; no test behavior changes
- **Scope**: "Easy wins" only — low risk, high clarity improvements
- **Timeline**: Single milestone; discrete phases per area

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Exclude terminal monitor from cleanup | 782 lines of complex state; high risk of breaking sessions | — Pending validation after other cleanup |
| Centralize timeouts in config package | Eliminates magic numbers; enables tuning without recompile | — Pending |
| Maintain all existing interfaces | Backward compatibility requirement | — Pending |

---
*Last updated: 2026-02-18 after initialization*
