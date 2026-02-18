# Requirements: Tech Debt Cleanup — Easy Wins

**Defined:** 2026-02-18
**Core Value:** Terminal sessions must not break — all changes maintain backward compatibility

## v1 Requirements

### Config Centralization

- [x] **CONF-01**: All hardcoded timeouts centralized in `internal/config/config.go`
- [x] **CONF-02**: Container operation timeouts configurable via environment
- [x] **CONF-03**: Rate limiting thresholds configurable via environment
- [x] **CONF-04**: WebSocket retry delays centralized (not hardcoded)

### Error Handling

- [ ] **ERR-01**: Consistent error wrapping with `fmt.Errorf("...: %w", err)` pattern
- [ ] **ERR-02**: Remove redundant error type checks (already handled by wrapping)
- [ ] **ERR-03**: Standardize on structured logging (slog) throughout codebase
- [ ] **ERR-04**: Eliminate fmt.Printf/fmt.Println usage in production code

### Code Cleanup

- [x] **CLEAN-01**: Remove unused imports across all Go files
- [ ] **CLEAN-02**: Consolidate duplicate string parsing utilities
- [x] **CLEAN-03**: Standardize mutex unlock patterns (use defer consistently)
- [x] **CLEAN-04**: Remove dead code (unused functions, commented blocks)

### Documentation

- [ ] **DOC-01**: Add package-level documentation for `internal/config`
- [ ] **DOC-02**: Document timeout configuration in `.env.example`

## v2 Requirements

Deferred to future — not in current scope.

### Architecture Improvements

- **ARCH-01**: Refactor terminal monitor complexity (782 lines)
- **ARCH-02**: Redesign nested mutex hierarchies
- **ARCH-03**: Implement proper gRPC connection backoff

## Out of Scope

| Feature | Reason |
|---------|--------|
| Terminal monitor refactor | Too risky for terminal sessions — explicitly excluded per constraints |
| gRPC TLS implementation | Infrastructure change, not cleanup |
| Container race condition fix | Complex locking changes, not an "easy win" |
| WebSocket connection leak fix | Affects terminal session lifecycle |
| SQLite layer refactoring | Busy timeout issues require architectural changes |
| New features | This is cleanup only, no new functionality |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| CONF-01 | Phase 1 | Complete |
| CONF-02 | Phase 1 | Complete |
| CONF-03 | Phase 1 | Complete |
| CONF-04 | Phase 1 | Complete |
| ERR-01 | Phase 2 | Pending |
| ERR-02 | Phase 2 | Pending |
| ERR-03 | Phase 2 | Pending |
| ERR-04 | Phase 2 | Pending |
| CLEAN-01 | Phase 3 | Complete |
| CLEAN-02 | Phase 3 | Pending |
| CLEAN-03 | Phase 3 | Complete |
| CLEAN-04 | Phase 3 | Complete |
| DOC-01 | Phase 4 | Pending |
| DOC-02 | Phase 4 | Pending |

**Coverage:**
- v1 requirements: 11 total
- Mapped to phases: 11
- Unmapped: 0 ✓

---
*Requirements defined: 2026-02-18*
*Last updated: 2026-02-18 after Phase 1 completion*
