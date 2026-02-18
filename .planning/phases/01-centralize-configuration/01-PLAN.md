# Plan 01: Centralize Configuration

**Phase:** 1 - Centralize Configuration
**Requirement:** CONF-01, CONF-02, CONF-03, CONF-04
**Wave:** 1
**Autonomous:** Yes

---

## Objective

Eliminate magic numbers by moving timeouts and thresholds to config. All existing behavior preserved — same values, just configurable via environment variables.

---

## Must-Haves (Success Criteria)

1. All timeout values reference `internal/config` instead of hardcoded literals
2. New environment variables documented in `.env.example`
3. Zero behavioral changes (same values, just configurable)
4. All existing tests pass without modification
5. Config loading uses consistent patterns (getEnvDuration, getEnvInt64)

---

## Tasks

### Task 1: Extend Config Structure

**Files:** `internal/config/config.go`

Add new configuration structs and helper functions:

1. Add `TimeoutConfig` struct with:
   - `ContainerStop` (default: 10s) — container stop timeout
   - `ContainerCreate` (default: 2m) — container create timeout
   - `HealthCheck` (default: 5s) — health check DB timeout
   - `DestroyCleanup` (default: 30s) — background destroy timeout
   - `TTLWorkerInterval` (default: 5m) — TTL cleanup worker interval

2. Add `ContainerConfig` struct with:
   - `MemoryLimitBytes` (default: 536870912 = 512MB)
   - `CPUQuota` (default: 50000 = 0.5 CPU)
   - `PidsLimit` (default: 256)
   - `CreateRetryAttempts` (default: 20)
   - `CreateRetryDelay` (default: 250ms)

3. Add `RateLimitConfig` struct with:
   - `RequestsPerWindow` (default: 10)
   - `WindowDuration` (default: 1m)

4. Add `SSEConfig` struct with:
   - `MaxRequestBodySize` (default: 1048576 = 1MB)
   - `RetryDelay` (default: 5s)
   - `KeepaliveInterval` (default: 10s)

5. Add `RetryConfig` struct with:
   - `DatabaseMaxRetries` (default: 3)
   - `DatabaseRetryBaseDelay` (default: 50ms)

6. Add helper functions:
   - `getEnvDuration(key string, fallback time.Duration) time.Duration`
   - `getEnvInt64(key string, fallback int64) int64` (needed for values like MemoryLimitBytes that exceed int32)

7. Update `Config` struct to embed new configs

8. Update `Load()` to populate new config values from environment:
   - `SHSH_CONTAINER_STOP_TIMEOUT`
   - `SHSH_CONTAINER_CREATE_TIMEOUT`
   - `SHSH_HEALTH_CHECK_TIMEOUT`
   - `SHSH_DESTROY_CLEANUP_TIMEOUT`
   - `SHSH_TTL_WORKER_INTERVAL`
   - `SHSH_CONTAINER_MEMORY_LIMIT`
   - `SHSH_CONTAINER_CPU_QUOTA`
   - `SHSH_CONTAINER_PIDS_LIMIT`
   - `SHSH_CONTAINER_CREATE_RETRY_ATTEMPTS`
   - `SHSH_CONTAINER_CREATE_RETRY_DELAY`
   - `SHSH_RATE_LIMIT_REQUESTS`
   - `SHSH_RATE_LIMIT_WINDOW`
   - `SHSH_SSE_MAX_BODY_SIZE`
   - `SHSH_SSE_RETRY_DELAY`
   - `SHSH_SSE_KEEPALIVE_INTERVAL`
   - `SHSH_DB_MAX_RETRIES`
   - `SHSH_DB_RETRY_BASE_DELAY`

---

### Task 2: Update Container Manager

**Files:** `internal/container/manager.go`

1. Change `DockerManager` to accept `*config.Config` instead of just `runtime string`
2. Replace hardcoded constants with config references:
   - `stopTimeoutSecs` → `cfg.Container.StopTimeout`
   - `memoryLimitBytes` → `cfg.Container.MemoryLimitBytes`
   - `cpuQuota` → `cfg.Container.CPUQuota`
   - `pidsLimit` → `cfg.Container.PidsLimit`
   - `createRetryAttempts` → `cfg.Container.CreateRetryAttempts`
   - `createRetryDelay` → `cfg.Container.CreateRetryDelay`

3. Update `NewDockerManager` signature to accept config

4. Update `StopContainer` to use config timeout
5. Update `EnsureContainer` to use config retry settings

---

### Task 3: Update Agent Handler

**Files:** `internal/agent/handler.go`

1. Modify `Handler` struct to include `*config.Config`
2. Update constructor functions to accept and store config
3. Replace hardcoded values:
   - `maxRequestBodySize` const → `cfg.SSE.MaxRequestBodySize`
   - `rateLimiter := NewRateLimiter(10, time.Minute)` → `NewRateLimiter(cfg.RateLimit.RequestsPerWindow, cfg.RateLimit.WindowDuration)`
   - SSE retry `5000` → `cfg.SSE.RetryDelay.Milliseconds()`
   - Keepalive `10 * time.Second` → `cfg.SSE.KeepaliveInterval`

---

### Task 4: Update Container TTL Worker

**Files:** `internal/container/ttl.go`

1. Add config parameter to `StartTTLWorker` and retry functions
2. Replace hardcoded retry values:
   - `maxRetries := 3` → `cfg.Retry.DatabaseMaxRetries`
   - `baseDelay := 50ms` → `cfg.Retry.DatabaseRetryBaseDelay`
   - `baseDelay := 100ms` → use cfg value with multiplier
3. Replace `ttlWorkerInterval = 5 * time.Minute` with `cfg.Timeout.TTLWorkerInterval`

---

### Task 5: Update API Container Handler

**Files:** `internal/api/container.go`

1. Pass config through to retry logic and handlers
2. Replace hardcoded values in `updateContainerIDWithRetry`:
   - `maxRetries := 3` → `cfg.Retry.DatabaseMaxRetries`
   - `baseDelay := 50ms` → `cfg.Retry.DatabaseRetryBaseDelay`
3. Replace hardcoded values in `Destroy`:
   - `30 * time.Second` context timeout → `cfg.Timeout.DestroyCleanup`
4. Replace hardcoded values in `HealthHandler.Health`:
   - `5 * time.Second` context timeout → `cfg.Timeout.HealthCheck`

---

### Task 6: Update Main Application

**Files:** `cmd/server/main.go` (or equivalent)

1. Pass config to all components that now require it:
   - `container.NewDockerManager(cfg)`
   - `agent.NewHandlerWithGrpcClient(..., cfg)`
   - `container.StartTTLWorker(..., cfg)`

---

### Task 7: Document Environment Variables

**Files:** `.env.example`

Add new configuration options with descriptions:

```bash
# Container Timeouts
SHSH_CONTAINER_STOP_TIMEOUT=10s
SHSH_CONTAINER_CREATE_TIMEOUT=2m
SHSH_DESTROY_CLEANUP_TIMEOUT=30s
SHSH_TTL_WORKER_INTERVAL=5m

# Container Resource Limits
SHSH_CONTAINER_MEMORY_LIMIT=536870912
SHSH_CONTAINER_CPU_QUOTA=50000
SHSH_CONTAINER_PIDS_LIMIT=256

# Container Retry Settings
SHSH_CONTAINER_CREATE_RETRY_ATTEMPTS=20
SHSH_CONTAINER_CREATE_RETRY_DELAY=250ms

# Rate Limiting
SHSH_RATE_LIMIT_REQUESTS=10
SHSH_RATE_LIMIT_WINDOW=1m

# SSE Settings
SHSH_SSE_MAX_BODY_SIZE=1048576
SHSH_SSE_RETRY_DELAY=5s
SHSH_SSE_KEEPALIVE_INTERVAL=10s

# Database Retry Settings
SHSH_DB_MAX_RETRIES=3
SHSH_DB_RETRY_BASE_DELAY=50ms
```

---

## Verification

### Automated Verification

```bash
# Build should succeed
go build ./...

# All tests should pass
go test ./...

# No new lint errors
golangci-lint run

# Verify no hardcoded timeouts remain (excluding terminal files)
! grep -r "time\.Second \* [0-9]\+" --include="*.go" internal/ | grep -v "_test.go" | grep -v "terminal/"
! grep -r "[0-9] \* time\.Second" --include="*.go" internal/ | grep -v "_test.go" | grep -v "terminal/"
```

### Manual Verification

1. Start server with default config — verify normal operation
2. Start server with custom timeouts — verify overrides applied
3. Run container lifecycle (create, use, destroy) — verify no regressions
4. Check agent chat endpoint — verify rate limiting still works

---

## Rollback

If issues found:
1. Revert to previous commit
2. Values return to hardcoded defaults
3. No state changes to database or filesystem

---

## Files Modified

- `internal/config/config.go` — extended config structures
- `internal/container/manager.go` — use config values
- `internal/agent/handler.go` — use config values
- `internal/container/ttl.go` — use config values
- `internal/api/container.go` — use config values
- `cmd/server/main.go` — pass config to components
- `.env.example` — document new variables

---

## Exclusions (Per Requirements)

- `internal/terminal/monitor.go` — terminal session critical path
- `internal/terminal/osc133_parser.go` — terminal session critical path

---

*Plan created: 2026-02-18*
*Phase: 01-centralize-configuration*
