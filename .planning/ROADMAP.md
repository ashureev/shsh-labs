# Roadmap: Tech Debt Cleanup — Easy Wins

**Project:** Tech Debt Cleanup
**Created:** 2026-02-18
**Phases:** 4
**Requirements:** 11 v1 requirements mapped

---

## Phase 1: Centralize Configuration ✓ Complete

**Goal:** Eliminate magic numbers by moving timeouts and thresholds to config

**Requirements:** CONF-01, CONF-02, CONF-03, CONF-04

**Success Criteria:**
- [x] All timeout values reference `internal/config` instead of hardcoded literals
- [x] New environment variables documented in `.env.example`
- [x] Zero behavioral changes (same values, just configurable)
- [x] All existing tests pass without modification

**Plan:** 01-PLAN.md — **COMPLETED** (7 commits, 17 env vars added)

**Excluded Files:**
- `internal/terminal/monitor.go` (terminal session critical path)
- `internal/terminal/osc133_parser.go` (terminal session critical path)

**Notes:**
Successfully centralized 18 hardcoded values from 7 files into config with environment variable overrides. All defaults preserved for backward compatibility.

---

## Phase 2: Standardize Error Handling ✓ Complete

**Goal:** Consistent error wrapping and structured logging

**Requirements:** ERR-01, ERR-02, ERR-03, ERR-04

**Success Criteria:**
- [x] All errors use `fmt.Errorf("...: %w", err)` wrapping pattern
- [x] No naked `fmt.Printf` calls in production code paths
- [x] All logging uses `log/slog` with structured fields
- [x] Error messages remain user-facing compatible (no breaking changes)

**Excluded Files:**
- `internal/terminal/monitor.go` (terminal session critical path)

**Plan:** 01-PLAN.md — **COMPLETED** (1 commit, 1 file modified)

**Notes:**
Fixed single ERR-01 violation in `internal/agent/grpc_client.go`. Verified ERR-02, ERR-03, ERR-04 already satisfied by codebase. Codebase was already in excellent shape for error handling.

---

## Phase 3: Code Cleanup ✓ Complete

**Goal:** Remove redundancy and standardize patterns

**Requirements:** CLEAN-01, CLEAN-02, CLEAN-03, CLEAN-04

**Success Criteria:**
- [x] No unused imports in any Go file
- [x] Duplicate utilities consolidated into shared helpers
- [x] All mutex unlocks use `defer` pattern consistently
- [x] Dead code identified and removed (verified unused via static analysis)

**Excluded Files:**
- `internal/terminal/monitor.go` (complex mutex hierarchy — out of scope)

**Plans:** 3/3 plans complete — **COMPLETED** (11 commits)

**Plan list:**
- [x] 03-01-PLAN.md — Consolidate duplicate SQLite error checking patterns
- [x] 03-02-PLAN.md — Remove dead code and unused imports
- [x] 03-03-PLAN.md — Verify mutex unlock patterns use defer consistently

**Notes:**
Created `internal/shared` package with SQLite error utilities. Removed empty helpers.go. Documented mutex conventions in CONVENTIONS.md.

---

## Phase 4: Documentation ✓ Complete

**Goal:** Document new configuration and patterns

**Requirements:** DOC-01, DOC-02

**Success Criteria:**
- [x] `internal/config` package has clear package-level documentation
- [x] All new env vars in `.env.example` with descriptions
- [x] README updated if necessary for configuration changes

**Excluded Files:**
- None

**Plans:** 1/1 plan complete — **COMPLETED** (3 commits)

**Plan list:**
- [x] 04-01-PLAN.md — Enhance package docs and README configuration section

**Notes:**
Enhanced package documentation in `internal/config/config.go` to reference `.env.example` and describe configuration categories. Updated README.md Configuration section. All 11 v1 requirements now complete.

---

## Requirements Traceability

| Requirement | Phase | Priority | Status |
|-------------|-------|----------|--------|
| CONF-01 | Phase 1 | High | ✓ Complete |
| CONF-02 | Phase 1 | High | ✓ Complete |
| CONF-03 | Phase 1 | Medium | ✓ Complete |
| CONF-04 | Phase 1 | Medium | ✓ Complete |
| ERR-01 | Phase 2 | High | ✓ Complete |
| ERR-02 | Phase 2 | Medium | ✓ Complete |
| ERR-03 | Phase 2 | High | ✓ Complete |
| ERR-04 | Phase 2 | Medium | ✓ Complete |
| CLEAN-01 | Phase 3 | Low | ✓ Complete |
| CLEAN-02 | Phase 3 | Medium | ✓ Complete |
| CLEAN-03 | Phase 3 | Medium | ✓ Complete |
| CLEAN-04 | Phase 3 | Low | ✓ Complete |
| DOC-01 | Phase 4 | Low | ✓ Complete |
| DOC-02 | Phase 4 | Low | ✓ Complete |

---

## Execution Order

**Sequential required** — phases build on each other:
- Phase 1 creates config structures used in Phases 2-3
- Phase 2 establishes patterns documented in Phase 4

**Estimated effort:** 2-3 hours per phase

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Breaking terminal sessions | Explicitly exclude terminal monitor files |
| Config regression | Maintain identical default values |
| Test failures | Run full test suite after each phase |
| Scope creep | Strict "easy wins" filter — defer complex changes |

---

## Definition of Done

- [x] All 11 v1 requirements satisfied
- [x] All existing tests pass
- [x] No new lint errors introduced
- [x] Terminal sessions verified working (manual smoke test)
- [x] Configuration documented

---

*Roadmap created: 2026-02-18*
*Last updated: 2026-02-18 after Phase 4 completion*
