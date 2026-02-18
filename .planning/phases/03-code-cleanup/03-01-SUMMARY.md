---
phase: 03-code-cleanup
plan: 01
type: execute
subsystem: shared-utilities
tags: [refactoring, sqlite, error-handling, dry]
dependency_graph:
  requires: []
  provides: [shared-sqlite-errors]
  affects: [internal/store/sqlite.go, internal/container/ttl.go, internal/api/container.go]
tech_stack:
  added: []
  patterns: [shared helper functions, error string matching consolidation]
key_files:
  created:
    - internal/shared/sqlite_errors.go
  modified:
    - internal/store/sqlite.go
    - internal/container/ttl.go
    - internal/api/container.go
decisions: []
metrics:
  duration_seconds: 92
  completed_date: 2026-02-18
---

# Phase 03 Plan 01: Consolidate SQLite Error Checking Patterns Summary

**One-liner:** Created shared SQLite error utilities and refactored 3 files to eliminate duplicate error string matching patterns.

---

## What Was Built

### Shared SQLite Error Utilities

Created `internal/shared/sqlite_errors.go` with three exported functions:

- `IsSQLiteBusyError(err error) bool` - checks for "SQLITE_BUSY" in error string
- `IsSQLiteLockedError(err error) bool` - checks for "database is locked" in error string
- `IsSQLiteConflictError(err error) bool` - checks for either busy or locked errors

### Refactored Files

Updated three files to use the shared helpers:

1. **internal/store/sqlite.go** - `DeleteAgentSession` function
2. **internal/container/ttl.go** - `deleteAgentSessionWithRetry` and `updateContainerIDWithRetry` functions
3. **internal/api/container.go** - `updateContainerIDWithRetry` function

All files now use `shared.IsSQLiteConflictError(err)` instead of inline `strings.Contains` checks.

---

## Commits

| Commit | Type | Description |
|--------|------|-------------|
| e09f8a7 | feat | Create shared SQLite error utilities |
| 23b1b8c | refactor | Use shared helpers in sqlite.go |
| 196c10c | refactor | Use shared helpers in ttl.go |
| 97809de | refactor | Use shared helpers in container.go |

---

## Verification Results

- [x] `go build ./...` - All packages build successfully
- [x] `go test ./internal/store/... ./internal/container/... ./internal/api/...` - Tests pass
- [x] `go build ./internal/shared/...` - Shared package builds successfully
- [x] No duplicate `strings.Contains` patterns for SQLite errors remain (only in shared utility)
- [x] All three target files import and use `shared.IsSQLiteConflictError`

---

## Deviations from Plan

None - plan executed exactly as written.

---

## Self-Check: PASSED

- [x] `internal/shared/sqlite_errors.go` exists with all three exported functions
- [x] `internal/store/sqlite.go` uses `shared.IsSQLiteConflictError`
- [x] `internal/container/ttl.go` uses `shared.IsSQLiteConflictError`
- [x] `internal/api/container.go` uses `shared.IsSQLiteConflictError`
- [x] All commits exist in git history
- [x] All builds pass

---

*Summary generated: 2026-02-18*
