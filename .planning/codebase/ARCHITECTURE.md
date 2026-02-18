# Architecture

**Analysis Date:** 2026-02-18

## Pattern Overview

**Overall:** Clean Architecture / Layered Architecture with Hexagonal influences

SHSH is a full-stack Linux playground platform with AI tutoring capabilities. The architecture follows a modular design with clear separation of concerns between the Go backend API, Python AI agent service, and React frontend.

**Key Characteristics:**
- Dependency injection via constructor injection
- Interface-driven design for testability
- Repository pattern for data access
- gRPC for inter-service communication
- WebSocket for real-time terminal sessions
- Event-driven AI interactions via SSE (Server-Sent Events)

## Layers

**Transport Layer (API):**
- Purpose: HTTP handlers, routing, middleware
- Location: `internal/api/`, `internal/terminal/`, `internal/agent/`
- Contains: HTTP handlers, WebSocket handlers, SSE connections, request/response types
- Depends on: Domain services, store, container management
- Used by: External clients (frontend, CLI)

**Service Layer (Domain Logic):**
- Purpose: Business logic and orchestration
- Location: `internal/container/`, `internal/terminal/`, `internal/agent/service.go`
- Contains: Container lifecycle management, terminal session coordination, AI agent coordination
- Depends on: Store interfaces, Docker client, gRPC clients
- Used by: Transport layer handlers

**Data Access Layer (Repository):**
- Purpose: Data persistence abstraction
- Location: `internal/store/`
- Contains: Repository interfaces and SQLite implementation
- Depends on: Domain models
- Used by: Service layer, transport layer

**Domain Layer:**
- Purpose: Core business entities
- Location: `internal/domain/`
- Contains: User, Session, AgentSession, Challenge structs
- Depends on: Nothing (pure data)
- Used by: All other layers

**Infrastructure Layer:**
- Purpose: External system integrations
- Location: `internal/container/manager.go`, `internal/agent/grpc_client.go`
- Contains: Docker API client, gRPC client, Redis connection
- Depends on: Domain models, configuration
- Used by: Service layer

## Data Flow

**Container Provisioning Flow:**

1. Client POST `/api/provision` → `ContainerHandler.Provision()` (`internal/api/container.go`)
2. Handler extracts userID from context (via identity middleware)
3. Handler calls `repo.GetUser()` to fetch/create user record (`internal/store/sqlite.go`)
4. Handler calls `mgr.EnsureContainer()` (`internal/container/manager.go`)
5. DockerManager creates/starts container via Docker API
6. Handler updates container ID in database via `repo.UpdateContainerID()`
7. Response returns container status to client

**Terminal Session Flow:**

1. Client connects WebSocket to `/ws/terminal`
2. `WebSocketHandler.ServeHTTP()` upgrades connection (`internal/terminal/websocket.go`)
3. Handler registers connection in `SessionManager` (`internal/terminal/manager.go`)
4. Handler creates Docker exec session via `mgr.CreateExecSession()`
5. Bidirectional streaming: WebSocket <-> PTY <-> Docker exec
6. TerminalMonitor (if AI enabled) captures output via OSC 133 parsing
7. Monitor sends relevant events to Python agent via gRPC
8. Agent responses streamed back via SSE to frontend

**AI Agent Interaction Flow:**

1. User types command in terminal
2. TerminalMonitor parses output using OSC 133 sequences (`internal/terminal/osc133_parser.go`)
3. Monitor detects command completion, sends to AgentService (`internal/agent/service.go`)
4. AgentService calls Python agent via gRPC (`internal/agent/grpc_client.go`)
5. Python agent processes through LangGraph pipeline (`python-agent/app/graph_builder.py`)
6. Agent response returned via gRPC
7. Agent handler broadcasts to all SSE connections for that user (`internal/agent/handler.go`)
8. Frontend receives AI message via EventSource, displays in sidebar

**Session Cleanup Flow:**

1. TTL worker runs periodically (`internal/container/ttl.go`)
2. Queries database for expired sessions via `repo.GetExpiredSessions()`
3. For each expired session: calls `mgr.StopContainer()`
4. Closes terminal sessions via `sm.CloseSession()`
5. Cleans up database records

## Key Abstractions

**Repository Interface:**
- Purpose: Abstract data persistence
- Location: `internal/store/store.go`
- Pattern: Repository pattern with interface segregation
- Implementation: SQLite with modernc.org/sqlite (CGO-free)

**Container Manager Interface:**
- Purpose: Abstract container runtime
- Location: `internal/container/manager.go` (lines 48-70)
- Pattern: Strategy pattern
- Implementation: DockerManager using Docker API

**Agent Interface:**
- Purpose: Abstract AI agent operations
- Location: `internal/agent/agent_interface.go`
- Pattern: Adapter pattern
- Implementation: gRPC client to Python service

**Session Manager:**
- Purpose: Manage WebSocket connections
- Location: `internal/terminal/manager.go`
- Pattern: Registry pattern
- Thread-safe map of userID -> sessionID -> websocket.Conn

**Identity Middleware:**
- Purpose: Anonymous user identification
- Location: `internal/identity/identity.go`
- Pattern: Middleware/Decorator
- Sets userID in context from cookie or generates new one

## Entry Points

**Main Server:**
- Location: `cmd/server/main.go`
- Triggers: Direct execution (`go run cmd/server/main.go`)
- Responsibilities:
  - Load configuration from environment
  - Initialize dependencies (database, Docker, session manager)
  - Conditionally initialize Python agent gRPC client
  - Setup HTTP router with middleware chain
  - Start TTL worker goroutine
  - Start HTTP server with graceful shutdown

**Python Agent Service:**
- Location: `python-agent/app/main.py`
- Triggers: Direct execution (`python -m app.main`)
- Responsibilities:
  - Initialize gRPC server
  - Setup Redis connection for session state
  - Start LangGraph agent pipeline
  - Handle agent requests from Go backend

**Frontend SPA:**
- Location: `src/main.jsx` (dev), `web/dist/` (production)
- Triggers: Browser navigation
- Responsibilities:
  - React application rendering
  - Client-side routing via react-router-dom
  - WebSocket terminal connections
  - SSE connection for AI messages

## Error Handling

**Strategy:** Structured logging with slog (Go) and structlog (Python)

**Patterns:**
- HTTP handlers return JSON error responses with consistent format: `{"error": "message"}`
- Repository errors wrapped with context using `fmt.Errorf("...: %w", err)`
- WebSocket errors logged at Debug level (expected disconnections)
- Retry logic for database operations with exponential backoff (see `updateContainerIDWithRetry`)
- Graceful degradation: AI features disabled if Python agent unavailable

**Critical Error Handling:**
- Database connection failure: Server exits immediately (line 48-50, `cmd/server/main.go`)
- Docker initialization failure: Server exits immediately
- Container health check failure: Logged but not fatal

## Cross-Cutting Concerns

**Logging:**
- Go: Structured JSON logging via `log/slog`
- Python: Structured logging via `structlog`
- All components include user_id, session_id, container_id where applicable

**Validation:**
- Configuration validation in `internal/config/config.go` (lines 61-77)
- User ID sanitization in Python agent (`_validate_user_id`)
- Request body size limits (1MB max)

**Authentication:**
- Anonymous identity via secure cookies
- Identity middleware sets userID in request context
- No traditional auth - designed for anonymous playground access

**CORS:**
- Configurable allowed origins via `FRONTEND_URL` environment variable
- Development mode allows all origins

**Concurrency:**
- sync.Map for provisionLocks and destroyLocks (per-user mutexes)
- sync.RWMutex for SessionManager connection registry
- Channel-based communication for SSE message queuing

---

*Architecture analysis: 2026-02-18*
