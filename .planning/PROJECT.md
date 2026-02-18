# Tech Debt Cleanup: Easy Wins

## What This Is

Preventive maintenance to refactor the messiest areas of the SHSH codebase and remove redundant code. Focus on "easy wins" that centralize configuration and reduce complexity without touching critical path logic for terminal sessions.

## Core Value

Terminal sessions must not break — all changes maintain backward compatibility and avoid modifying core terminal monitor state management.

## Requirements

### Validated

**Shipped in v1.0 (2026-02-18):**

- ✓ Centralize magic numbers and timeouts in config package — 17 env vars added
- ✓ Remove redundant error handling patterns — fixed ERR-01, verified ERR-02/03/04
- ✓ Fix inconsistent mutex unlock paths — verified defer patterns, created CONVENTIONS.md
- ✓ Standardize on structured logging patterns — verified slog usage across codebase
- ✓ Consolidate duplicate utility functions — created internal/shared package

**Existing codebase capabilities:**

- ✓ Container provisioning via Docker API
- ✓ WebSocket terminal sessions with PTY
- ✓ AI agent integration via gRPC
- ✓ SQLite-based session storage
- ✓ Anonymous user identity management

### Active

(None — all v1.0 requirements completed)

### Out of Scope

- Terminal monitor complexity refactor (782 lines) — too risky for terminal sessions
- Nested mutex hierarchy redesign — affects critical path
- gRPC TLS implementation — requires infrastructure changes
- Container name race condition fix — complex locking changes
- WebSocket connection leak fix — affects terminal session lifecycle
- Database layer refactoring — SQLite busy timeout issues

## Context

SHSH is a Linux playground platform with AI tutoring. The v1.0 cleanup milestone successfully addressed accumulated tech debt:

**Before v1.0:**
- Hardcoded timeouts scattered across 10+ files
- Inconsistent error wrapping patterns
- Some manual mutex unlocks alongside deferred ones
- Duplicate helper functions for string parsing
- Mixed logging styles (some with slog, some fmt.Printf remnants)

**After v1.0:**
- 17 configurable environment variables in 5 categories
- Consistent fmt.Errorf with %w pattern across codebase
- Verified defer mutex patterns with documented conventions
- Shared SQLite error utilities in internal/shared package
- Complete structured logging with slog

The codebase uses Clean Architecture with Go backend, Python AI agent, and React frontend.

## Constraints

- **Backward Compatibility**: All public APIs unchanged; config additions only ✓
- **Terminal Sessions**: No modifications to `internal/terminal/monitor.go` or `internal/terminal/osc133_parser.go` ✓
- **Testing**: Must pass existing tests; no test behavior changes ✓
- **Scope**: "Easy wins" only — low risk, high clarity improvements ✓

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Exclude terminal monitor from cleanup | 782 lines of complex state; high risk of breaking sessions | ✓ Validated — sessions stable, no incidents |
| Centralize timeouts in config package | Eliminates magic numbers; enables tuning without recompile | ✓ Validated — 17 env vars, backward compatible |
| Maintain all existing interfaces | Backward compatibility requirement | ✓ Validated — zero breaking changes |
| Create internal/shared package | DRY principle for SQLite error checking | ✓ Validated — 3 files refactored |
| Document mutex conventions | Ensure future code follows defer pattern | ✓ Validated — CONVENTIONS.md created |

---

## Current State (v1.0)

**Milestone:** v1.0 Tech Debt Cleanup — Easy Wins (shipped 2026-02-18)
**Next Milestone:** None planned — maintenance complete

---

*Last updated: 2026-02-18 after v1.0 milestone completion*
