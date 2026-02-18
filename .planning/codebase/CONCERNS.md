# Codebase Concerns

**Analysis Date:** 2026-02-18

## Tech Debt

### Terminal Monitor Complexity
- **Issue:** `internal/terminal/monitor.go` is 782 lines with complex state management
- **Files:** `internal/terminal/monitor.go`, `internal/terminal/osc133_parser.go` (654 lines)
- **Impact:** Race conditions between OSC 133 parser state and monitor state; dual mutex hierarchy (tm.mu + session.mu) creates deadlock risk
- **Fix approach:** Extract state machine into dedicated type; use single source of truth for session state

### Nested Mutex Patterns
- **Issue:** Multiple components hold nested locks (tm.mu -> session.mu, parser -> monitor)
- **Files:** `internal/terminal/monitor.go:276-285`, `internal/terminal/monitor.go:380-402`
- **Impact:** Lock ordering violations cause deadlocks under load; inconsistent unlock paths (some deferred, some manual)
- **Fix approach:** Define strict lock hierarchy; use defer consistently; extract critical sections into methods

### gRPC Client Connection Management
- **Issue:** gRPC client uses insecure credentials and has no reconnection backoff
- **Files:** `internal/agent/grpc_client.go:67-69`
- **Impact:** No TLS encryption to Python agent; thundering herd on reconnect; no circuit breaker
- **Fix approach:** Add mTLS support; implement exponential backoff; add health check circuit breaker

### Magic Numbers and Timeouts
- **Issue:** Hardcoded timeouts scattered throughout codebase
- **Files:**
  - `internal/terminal/monitor.go:546-547` (500ms/2s timeouts)
  - `internal/container/manager.go:44-45` (20 retries, 250ms delay)
  - `internal/agent/handler.go:184` (10 req/min rate limit)
- **Impact:** Unpredictable behavior under load; cannot tune without recompilation
- **Fix approach:** Centralize timeout configuration in `internal/config`

## Known Issues

### Container Name Race Condition
- **Symptoms:** "container name already in use" errors under concurrent provisioning
- **Files:** `internal/container/manager.go:167-201`, `internal/api/container.go:82-83`
- **Trigger:** Multiple requests for same user before first container creation completes
- **Workaround:** Retry loop with force-stop of conflicting container (inefficient)
- **Root cause:** Load-store race in provisionLocks map

### SQLite Busy Timeouts Insufficient
- **Symptoms:** SQLITE_BUSY errors under concurrent agent session operations
- **Files:** `internal/store/sqlite.go:21`, `internal/store/sqlite.go:362`
- **Trigger:** Rapid session updates + agent session writes
- **Workaround:** 5-second busy timeout + application-level mutex
- **Fix approach:** Connection pool tuning; separate read replicas; batch updates

### WebSocket Connection Leaks
- **Symptoms:** File descriptor exhaustion under load
- **Files:** `internal/terminal/websocket.go:99-100`
- **Trigger:** Abrupt client disconnections without proper close handshake
- **Workaround:** None detected
- **Fix approach:** Add read timeout; ensure Close() is called in all paths

## Security Considerations

### No TLS for Internal gRPC
- **Risk:** Man-in-the-middle attacks between Go server and Python agent
- **Files:** `internal/agent/grpc_client.go:68`
- **Current mitigation:** None (insecure credentials)
- **Recommendations:** Add mutual TLS with client certificates

### Container Escape Risk
- **Risk:** Privileged container operations may allow host escape
- **Files:** `internal/container/manager.go:224-258` (DNS fix runs as root)
- **Current mitigation:** gVisor runtime option available
- **Recommendations:** Use gVisor by default; drop all capabilities; read-only rootfs

### WebSocket Origin Check Bypass
- **Risk:** `OriginPatterns: []string{"*"}` allows any origin
- **Files:** `internal/terminal/websocket.go:93`
- **Current mitigation:** Separate origin check function exists but patterns are permissive
- **Recommendations:** Enforce strict origin matching in production

### Information Disclosure in Error Messages
- **Risk:** Internal error details exposed to clients
- **Files:** `internal/api/container.go:113`, `internal/agent/handler.go:203-224`
- **Current mitigation:** Generic error messages for some paths
- **Recommendations:** Consistent error sanitization; log details internally only

## Performance Bottlenecks

### Synchronous Container Operations
- **Problem:** Container start/stop blocks HTTP handlers
- **Files:** `internal/api/container.go:95-170`
- **Cause:** Docker API calls made synchronously in request path
- **Improvement path:** Async provisioning with state machine; queue-based approach

### Terminal Monitor Job Queue
- **Problem:** Fixed 100-slot job queue with drop-on-full behavior
- **Files:** `internal/terminal/monitor.go:526-537`
- **Cause:** AI analysis jobs can overflow during command bursts
- **Improvement path:** Backpressure to client; priority queue; worker pool scaling

### SQLite Write Serialization
- **Problem:** All writes serialized through single mutex
- **Files:** `internal/store/sqlite.go:21`
- **Cause:** SQLite busy timeout + application mutex double-protection
- **Improvement path:** WAL mode tuning; batch writes; separate connection for reads

### Memory Buffer Growth
- **Problem:** Unbounded output buffer in terminal sessions
- **Files:** `internal/terminal/monitor.go:127` (64KB limit exists but not enforced everywhere)
- **Cause:** `maxBufferSize` checked inconsistently
- **Improvement path:** Enforce limit in all buffer writes; circular buffer for large outputs

## Fragile Areas

### OSC 133 Parser State Machine
- **Files:** `internal/terminal/osc133_parser.go`
- **Why fragile:** Complex regex-based parsing with multiple state transitions; no fuzz testing
- **Safe modification:** Add comprehensive test cases before changes; use property-based testing
- **Test coverage:** Unit tests exist but no integration tests with real shell output

### Agent Session Signal Handling
- **Files:** `internal/agent/service.go:54-96`
- **Why fragile:** Silent failures on signal updates; no validation of signal delivery
- **Safe modification:** Add error propagation; implement retry with backoff
- **Test coverage:** No tests for concurrent signal updates

### Conversation Logger Queue
- **Files:** `internal/agent/conversation_logger.go`
- **Why fragile:** Async queue with no durability guarantees; drop-on-full behavior
- **Safe modification:** Add persistence to disk; implement backpressure
- **Test coverage:** Basic tests exist but no stress testing

### Container TTL Cleanup
- **Files:** `internal/container/ttl.go`
- **Why fragile:** Time-based cleanup with no distributed coordination
- **Safe modification:** Add leader election; use database for coordination
- **Test coverage:** No tests for concurrent cleanup scenarios

## Scaling Limits

### WebSocket Broadcast Fan-Out
- **Current capacity:** Single-node broadcast loop
- **Limit:** `internal/agent/handler.go:373-449` - all connections in single goroutine
- **Scaling path:** Shard by userID; distributed pub/sub (Redis)

### SQLite Database
- **Current capacity:** Single-node SQLite with WAL mode
- **Limit:** Write throughput limited by single-file WAL
- **Scaling path:** PostgreSQL migration; read replicas

### In-Memory Session State
- **Current capacity:** Unbounded maps with TTL
- **Limit:** `internal/terminal/manager.go` - no memory limits on session storage
- **Scaling path:** Redis-backed sessions; memory quotas per user

### Docker Daemon Dependency
- **Current capacity:** Single Docker daemon
- **Limit:** All containers on one host; no multi-host scheduling
- **Scaling path:** Kubernetes integration; containerd direct integration

## Dependencies at Risk

### `coder/websocket` (WebSocket Library)
- **Risk:** Relatively new library (v1.0 in 2023); API changes possible
- **Impact:** WebSocket handling is core to product
- **Migration plan:** Monitor for v2 announcements; abstract behind interface

### `modernc.org/sqlite` (SQLite Driver)
- **Risk:** CGO-free SQLite has subtle compatibility differences
- **Impact:** Database corruption or locking issues
- **Migration plan:** Keep `mattn/go-sqlite3` as alternative build tag

### Docker API Client
- **Risk:** Docker API version deprecation
- **Impact:** Container operations fail
- **Migration plan:** Use containerd client directly; abstract runtime interface

## Missing Critical Features

### Graceful Shutdown
- **Problem:** No graceful shutdown of active WebSocket connections
- **Files:** `cmd/server/main.go` - signal handling not shown in read portion
- **Blocks:** Zero-downtime deployments
- **Priority:** High

### Distributed Tracing
- **Problem:** No request correlation IDs across terminal-AI-chat flow
- **Blocks:** Debugging production issues
- **Priority:** Medium

### Rate Limiting per User
- **Problem:** Only chat endpoint has rate limiting; container ops unprotected
- **Files:** `internal/agent/handler.go:184` (only rate limit)
- **Blocks:** DoS protection
- **Priority:** High

### Container Resource Metrics
- **Problem:** No CPU/memory monitoring per container
- **Blocks:** Resource exhaustion detection
- **Priority:** Medium

## Test Coverage Gaps

### Concurrency Testing
- **What's not tested:** Concurrent container provisioning for same user
- **Files:** `internal/api/container.go:82-83`
- **Risk:** Race conditions in production only
- **Priority:** High

### WebSocket Reconnection
- **What's not tested:** LastEventID replay functionality
- **Files:** `internal/agent/handler.go:518-602`
- **Risk:** Message loss on reconnect
- **Priority:** High

### Terminal Monitor Stress
- **What's not tested:** High-frequency command execution
- **Files:** `internal/terminal/monitor.go`
- **Risk:** Job queue overflow; memory growth
- **Priority:** Medium

### Container Lifecycle Edge Cases
- **What's not tested:** Docker daemon restart mid-operation
- **Files:** `internal/container/manager.go`
- **Risk:** Orphaned containers; state inconsistency
- **Priority:** High

---

*Concerns audit: 2026-02-18*
