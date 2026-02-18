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

## Phase 3: Code Cleanup

**Goal:** Remove redundancy and standardize patterns

**Requirements:** CLEAN-01, CLEAN-02, CLEAN-03, CLEAN-04

**Success Criteria:**
1. No unused imports in any Go file
2. Duplicate utilities consolidated into shared helpers
3. All mutex unlocks use `defer` pattern consistently
4. Dead code identified and removed (verified unused via static analysis)

**Excluded Files:**
- `internal/terminal/monitor.go` (complex mutex hierarchy — out of scope)

**Notes:**
Use `golangci-lint` to identify dead code and unused imports

---

## Phase 4: Documentation

**Goal:** Document new configuration and patterns

**Requirements:** DOC-01, DOC-02

**Success Criteria:**
1. `internal/config` package has clear package-level documentation
2. All new env vars in `.env.example` with descriptions
3. README updated if necessary for configuration changes

**Excluded Files:**
- None

**Notes:**
Lightweight phase — focus on discoverability of new config options

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
| CLEAN-01 | Phase 3 | Low |
| CLEAN-02 | Phase 3 | Medium |
| CLEAN-03 | Phase 3 | Medium |
| CLEAN-04 | Phase 3 | Low |
| DOC-01 | Phase 4 | Low |
| DOC-02 | Phase 4 | Low |

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

- [ ] All 11 v1 requirements satisfied
- [ ] All existing tests pass
- [ ] No new lint errors introduced
- [ ] Terminal sessions verified working (manual smoke test)
- [ ] Configuration documented

---

*Roadmap created: 2026-02-18*
*Last updated: 2026-02-18 after Phase 2 completion*
