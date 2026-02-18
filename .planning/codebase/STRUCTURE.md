# Codebase Structure

**Analysis Date:** 2026-02-18

## Directory Layout

```
/home/ashu/Downloads/mycodes/shsh-labs/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── container/
│   └── logging/                 # Container logging configuration
├── data/
│   └── logs/
│       └── conversations/       # Conversation log storage
├── dist/                        # Production build output (frontend)
├── docs/                        # Documentation
├── internal/
│   ├── agent/                   # AI agent integration
│   ├── api/                     # HTTP handlers
│   ├── config/                  # Configuration management
│   ├── container/               # Docker container management
│   ├── domain/                  # Domain models
│   ├── identity/                # User identity middleware
│   ├── middleware/              # HTTP middleware
│   ├── proto/                   # Protocol buffer definitions
│   ├── store/                   # Data persistence
│   └── terminal/                # WebSocket terminal handling
├── python-agent/                # Python AI service
│   ├── app/
│   │   ├── generated/           # Protobuf generated code
│   │   ├── pipeline/            # LangGraph pipeline components
│   │   ├── checkpointer.py      # State checkpointing
│   │   ├── config.py            # Agent configuration
│   │   ├── graph_builder.py     # LangGraph workflow builder
│   │   ├── main.py              # Agent entry point
│   │   ├── server.py            # gRPC server implementation
│   │   └── session_store.py     # Redis session storage
│   └── tests/                   # Python tests
├── src/                         # Frontend React application
│   ├── components/              # React components
│   ├── context/                 # React context providers
│   ├── store/                   # Zustand state management
│   ├── App.jsx                  # Main app component
│   ├── index.css                # Global styles
│   └── main.jsx                 # Frontend entry point
└── web/
    └── embed.go                 # Frontend embedding for Go binary
```

## Directory Purposes

**cmd/server/:**
- Purpose: Application entry points
- Contains: Main server initialization, dependency wiring, graceful shutdown
- Key files: `cmd/server/main.go`
- Pattern: Standard Go project layout (cmd/ for executables)

**internal/:**
- Purpose: Private application code (not importable by external packages)
- Contains: All business logic, following Clean Architecture layers
- Pattern: Standard Go internal/ convention

**internal/agent/:**
- Purpose: AI agent integration via gRPC
- Contains: gRPC client, agent handlers, SSE message broadcasting, conversation logging
- Key files: `grpc_client.go`, `handler.go`, `service.go`, `conversation_logger.go`
- Dependencies: Python agent service via gRPC

**internal/api/:**
- Purpose: HTTP transport layer
- Contains: REST API handlers, request/response types
- Key files: `container.go` (provisioning), `handler.go` (base handler utilities)
- Patterns: Handler structs with dependency injection, method-per-endpoint

**internal/config/:**
- Purpose: Configuration management
- Contains: Config struct, environment variable loading, validation
- Key files: `config.go`
- Pattern: Centralized config with environment-based defaults

**internal/container/:**
- Purpose: Docker container lifecycle management
- Contains: Container manager interface and Docker implementation, TTL worker
- Key files: `manager.go`, `ttl.go`
- Pattern: Interface abstraction for container runtime

**internal/domain/:**
- Purpose: Core domain entities
- Contains: User, Session, AgentSession, Challenge structs
- Key files: `user.go`, `session.go`, `agent_session.go`, `challenge.go`
- Pattern: Pure data structures, no dependencies

**internal/identity/:**
- Purpose: Anonymous user identification
- Contains: Identity middleware, cookie management
- Key files: `identity.go`
- Pattern: Middleware setting context values

**internal/store/:**
- Purpose: Data persistence
- Contains: Repository interface and SQLite implementation
- Key files: `store.go` (interface), `sqlite.go` (implementation)
- Pattern: Repository pattern with interface segregation

**internal/terminal/:**
- Purpose: WebSocket terminal sessions
- Contains: WebSocket handler, session manager, PTY controller, OSC 133 parser, terminal monitor
- Key files: `websocket.go`, `manager.go`, `monitor.go`, `osc133_parser.go`
- Pattern: Bidirectional streaming with connection registry

**python-agent/:**
- Purpose: Python-based AI agent service
- Contains: LangGraph pipeline, gRPC server, LLM integration
- Key files: `app/main.py`, `app/server.py`, `app/graph_builder.py`
- Pattern: Separate service communicating via gRPC

**src/:**
- Purpose: Frontend React application
- Contains: React components, hooks, state management, styles
- Key files: `App.jsx`, `main.jsx`, `index.css`
- Pattern: Component-based SPA with Zustand for state

## Key File Locations

**Entry Points:**
- Go server: `cmd/server/main.go`
- Python agent: `python-agent/app/main.py`
- Frontend dev: `src/main.jsx`
- Frontend production: `web/dist/` (embedded via `web/embed.go`)

**Configuration:**
- Go config: `internal/config/config.go`
- Python config: `python-agent/app/config.py`
- Environment: `.env` (not committed), `.env.example` (template)
- Docker: `docker-compose.yml`, `Dockerfile`, `Dockerfile.backend`

**Core Logic:**
- Container management: `internal/container/manager.go`
- Terminal handling: `internal/terminal/websocket.go`
- Agent integration: `internal/agent/handler.go`
- Data persistence: `internal/store/sqlite.go`

**API Handlers:**
- Container endpoints: `internal/api/container.go`
- Base utilities: `internal/api/handler.go`
- Health checks: `internal/api/container.go` (HealthHandler)

**Domain Models:**
- User: `internal/domain/user.go`
- Session: `internal/domain/session.go`
- AgentSession: `internal/domain/agent_session.go`

**Frontend Components:**
- Terminal: `src/components/TerminalSession.jsx`
- Dashboard: `src/components/Dashboard.jsx`
- AI Sidebar: `src/components/AIChatSidebar.jsx`
- Provisioning: `src/components/ProvisioningState.jsx`

**Testing:**
- Go tests: Co-located with source files (`*_test.go`)
- Python tests: `python-agent/tests/`
- Frontend tests: Co-located (`*.test.jsx`)

## Naming Conventions

**Files:**
- Go: `snake_case.go` for implementation, `snake_case_test.go` for tests
- Python: `snake_case.py` for modules
- Frontend: `PascalCase.jsx` for components, `camelCase.js` for utilities

**Directories:**
- Go: Single word or `snake_case` for multi-word (e.g., `python-agent`)
- All lowercase, no spaces

**Go Specific:**
- Interfaces: `-er` suffix (e.g., `Manager`, `Repository`)
- Implementations: Struct name matches interface minus suffix (e.g., `DockerManager` implements `Manager`)
- Constructors: `New` + Type name (e.g., `NewDockerManager`)
- Handlers: Handler suffix (e.g., `ContainerHandler`)

**Frontend Specific:**
- Components: PascalCase (e.g., `TerminalSession.jsx`)
- Hooks: `use` prefix (e.g., `useAuth`)
- Stores: `Store` suffix (e.g., `chatStore`)

## Where to Add New Code

**New API Endpoint:**
1. Add handler method to appropriate handler in `internal/api/`
2. Register route in `RegisterRoutes()` method
3. Add tests in `internal/api/handler_test.go`

**New Database Entity:**
1. Define struct in `internal/domain/` (e.g., `new_entity.go`)
2. Add methods to `Repository` interface in `internal/store/store.go`
3. Implement in `internal/store/sqlite.go`
4. Add database migration logic in sqlite initialization

**New Container Runtime:**
1. Implement `Manager` interface from `internal/container/manager.go`
2. Add configuration option in `internal/config/config.go`
3. Wire up in `cmd/server/main.go` based on config

**New Frontend Component:**
1. Create component file in `src/components/` (PascalCase.jsx)
2. Export from component file
3. Import in parent component or add route in `App.jsx`
4. Add styles using Tailwind classes or `src/index.css`

**New State Management:**
1. Create store in `src/store/` (e.g., `newStore.js`)
2. Use Zustand pattern: `create((set, get) => ({...}))`
3. Import and use in components via hook

**New AI Pipeline Component:**
1. Add node implementation in `python-agent/app/pipeline/`
2. Import and wire in `python-agent/app/graph_builder.py`
3. Update state types if needed in `python-agent/app/pipeline/types.py`

## Special Directories

**web/dist/:**
- Purpose: Production frontend build
- Generated: Yes (via `npm run build`)
- Committed: No (in .gitignore)
- Embedded: Yes (via `web/embed.go` using `//go:embed`)

**data/:**
- Purpose: Local data storage
- Contains: SQLite database, conversation logs
- Gitignored: Yes (runtime data)
- Volume-mounted: Yes (in Docker)

**python-agent/app/generated/:**
- Purpose: Protobuf generated code
- Generated: Yes (via protoc)
- Committed: Yes (for simplicity)
- Regenerate: `make proto` or manual protoc command

**internal/proto/:**
- Purpose: Protobuf definition files (.proto)
- Used by: Both Go and Python services
- Build: Generates code for both languages

**dist/ (root):**
- Purpose: Final compiled binary output
- Generated: Yes (via `go build`)
- Contains: Single binary with embedded frontend
- Gitignored: Yes

---

*Structure analysis: 2026-02-18*
