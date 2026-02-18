# External Integrations

**Analysis Date:** 2026-02-18

## APIs & External Services

**LLM Providers (AI Features):**
- **Google Gemini** - Primary LLM provider
  - SDK: `google-genai` (Python)
  - LangChain integration: `langchain-google-genai`
  - Model: `gemini-2.5-flash-lite-preview-06-2025` (configurable)
  - Auth: `GOOGLE_API_KEY` environment variable
  - Token counting endpoint with timeout fallback

- **Anthropic Claude** - Secondary LLM provider
  - SDK: `langchain-anthropic`
  - Via LangChain interface

- **OpenRouter** - Alternative provider aggregation
  - SDK: Via LangChain OpenRouter integration
  - Provides access to multiple model providers

**Container Runtime:**
- **Docker Engine API** - Core infrastructure dependency
  - Go SDK: `github.com/docker/docker`
  - Unix socket access: `/var/run/docker.sock`
  - Used for: Container lifecycle, execution, volume management
  - Supports alternative runtimes: gVisor (runsc) via `CONTAINER_RUNTIME` env var

## Data Storage

**Databases:**
- **SQLite** (Embedded)
  - Location: Configurable via `DB_PATH` (default: `./data/playground.db`)
  - Driver: `modernc.org/sqlite` (CGO-free)
  - Purpose: User sessions, agent state, challenge progress
  - File: `/home/ashu/Downloads/mycodes/shsh-labs/internal/store/sqlite.go`

**Caching & Session Store:**
- **Redis** (`redis/redis-stack-server:7.4.0-v6`)
  - Connection: `REDIS_URL` environment variable (default: `localhost:6379`)
  - Purpose: Session state, conversation checkpointing, TTL management
  - Used by: Python agent (LangGraph checkpointing), Go backend
  - Persistence: Configured with volume `redis-data`

**File Storage:**
- **Local filesystem** - Conversation logs and user data
  - Path: Configurable via `CONVERSATION_LOG_DIR`
  - Format: NDJSON (Newline Delimited JSON)
  - Rotation: Automatic with configurable queue size

## Authentication & Identity

**Auth Provider:**
- **Custom/None** - No external auth provider detected
- Identity: Anonymous user sessions via UUID
- Session management: Redis-backed with TTL (60 minutes default)
- Container isolation: Per-user Docker containers provide security boundary

## Monitoring & Observability

**Error Tracking:**
- None detected (no Sentry, Rollbar, etc.)

**Logging:**
- **Go Backend:** Structured logging via `log/slog`
  - Format: JSON in production
  - Destination: stdout/stderr
  - Rotation: Docker log driver (json-file, max-size: 10m, max-file: 3)

- **Python Agent:** Structured logging via `structlog`
  - Configurable level via `LOG_LEVEL` (default: INFO)
  - Includes OpenInference instrumentation for LangChain

**Metrics:**
- None detected (no Prometheus, DataDog, etc.)

**Health Checks:**
- Go backend: HTTP `/health` endpoint
- Python agent: gRPC `Health` service method
- Docker Compose: `backend-healthcheck` service with curl

## CI/CD & Deployment

**CI Pipeline:**
- **GitHub Actions** (`.github/workflows/ci-required.yml`)
  - Go quality: tests, build, golangci-lint
  - Frontend linting
  - Path-filtered jobs (only run on relevant changes)
  - Tools: `actions/setup-go@v5`, `golangci/golangci-lint-action@v9`

**Hosting:**
- **Docker Compose** (primary deployment method)
  - Services: backend, python-agent, redis
  - Optional: backend-healthcheck
  - Build profiles: `build` (for playground image), `ai` (for AI features)

**Container Registry:**
- Local image builds (no external registry configured)
- Images: `shsh-backend:latest`, `shsh-python-agent:latest`, `playground:latest`

## Inter-Service Communication

**Go Backend ↔ Python Agent:**
- Protocol: gRPC over TCP
- Port: 50051 (configurable via `PYTHON_AGENT_ADDR`)
- Service definition: `/home/ashu/Downloads/mycodes/shsh-labs/python-agent/proto/agent.proto`
- Methods:
  - `Chat` - Streaming chat between user and AI
  - `ProcessTerminal` - Terminal command processing
  - `UpdateSessionSignals` - Typing/editor state sync
  - `Health` - Service health checks
- Keepalive: 2 minutes with 10 second timeout

**Frontend ↔ Go Backend:**
- Protocol: HTTP/1.1 and WebSocket
- WebSocket endpoint: `/ws/terminal`
- API endpoints: `/api/*`
- Real-time: Server-Sent Events for AI responses

**Python Agent ↔ Redis:**
- Purpose: LangGraph checkpoint persistence
- Library: `langgraph-checkpoint-redis`

## Environment Configuration

**Required Environment Variables:**

**Core Application:**
- `PORT` - HTTP server port (default: 8080)
- `DB_PATH` - SQLite database file path
- `REDIS_URL` - Redis connection string
- `PYTHON_AGENT_ADDR` - gRPC address for Python agent

**AI Features (Optional):**
- `GOOGLE_API_KEY` - Required for AI mentor feature
- `LLM_PROVIDER` - Provider selection (google/anthropic/openrouter)
- `LLM_MODEL` - Model name override

**Container Runtime:**
- `CONTAINER_RUNTIME` - Empty for Docker, "runsc" for gVisor

**Conversation Logging:**
- `CONVERSATION_LOG_ENABLED` - Enable per-session logs (default: true)
- `CONVERSATION_LOG_DIR` - Log directory path
- `CONVERSATION_LOG_GLOBAL_ENABLED` - Enable global aggregate log
- `CONVERSATION_LOG_QUEUE_SIZE` - Memory queue size (default: 1000)

**Secrets Location:**
- `.env` file (gitignored, never commit)
- Docker secrets via environment variables
- No secret management service (Vault, AWS Secrets Manager, etc.)

## Webhooks & Callbacks

**Incoming:**
- None detected

**Outgoing:**
- None detected

## Network Architecture

**Docker Network:**
- Name: `shsh-network` (bridge driver)
- Services:
  - `backend` (Go): Port 8080
  - `python-agent` (Python gRPC): Port 50051 (internal)
  - `redis`: Port 6379 (internal and external)

**Port Exposure:**
- 8080: Go backend API and WebSocket
- 6379: Redis (optional external access)
- 50051: Python gRPC (optional external access in dev)

---

*Integration audit: 2026-02-18*
