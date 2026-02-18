# Technology Stack

**Analysis Date:** 2026-02-18

## Languages

**Primary:**
- **Go 1.24.4** - Backend API, container orchestration, WebSocket handling
- **TypeScript/JavaScript (React)** - Frontend UI with Vite build system
- **Python 3.11+** - AI/ML agent service with LangChain

**Secondary:**
- **Protocol Buffers (proto3)** - gRPC service definitions between Go and Python
- **SQL** - SQLite database queries
- **Dockerfile** - Multi-stage container builds

## Runtime

**Go Backend:**
- Go version: 1.24.4
- Module: `github.com/ashureev/shsh-labs`
- CGO enabled for SQLite (modernc.org/sqlite)

**Frontend:**
- Node.js 22 (LTS)
- ES Modules (type: "module")
- Build target: ES2020+

**Python Agent:**
- Python 3.11+ (Docker uses 3.11)
- Package manager: uv (via uv.lock)
- Build system: setuptools

**Package Managers:**
- Go: Built-in go modules (`go.mod`, `go.sum`)
- Node.js: npm (`package.json`, `package-lock.json`)
- Python: uv (`pyproject.toml`, `uv.lock`)

## Frameworks

**Core:**
- **Chi Router v5** (`github.com/go-chi/chi/v5`) - HTTP routing and middleware
- **React 19.2** - Frontend UI framework
- **Vite 7.2** - Frontend build tool and dev server
- **FastAPI** (Python) - HTTP server for Python agent (secondary to gRPC)
- **LangGraph** (Python) - AI agent state machine and conversation flow

**Real-time Communication:**
- **WebSocket** (`github.com/coder/websocket`) - Terminal streaming
- **gRPC** (`google.golang.org/grpc`) - Go-to-Python agent communication
- **Server-Sent Events (SSE)** - AI response streaming to frontend

**State Management:**
- **Zustand 5.0** - React state management

**Styling:**
- **Tailwind CSS 4.1** - Utility-first CSS
- **Framer Motion** - React animations

**Testing:**
- **Vitest** - Frontend unit testing
- **Testing Library** - React component testing
- **pytest** - Python testing

**Build/Dev:**
- **Vite** - Frontend bundling with manual chunking
- **Make** - Build orchestration
- **Docker + Buildx** - Container builds
- **golangci-lint** - Go linting
- **Ruff** - Python linting

## Key Dependencies

**Critical Infrastructure:**
- **Docker Engine API** (`github.com/docker/docker`) - Container lifecycle management
- **SQLite** (`modernc.org/sqlite`) - Embedded database (CGO-free)
- **Redis** (`redis/redis-stack-server`) - Session storage and state management

**AI/ML (Python):**
- **LangChain** + **LangGraph** - LLM orchestration and stateful agents
- **Google GenAI** - Gemini model integration
- **Anthropic** - Claude model support
- **OpenRouter** - Alternative model provider

**Frontend Terminal:**
- **XTerm.js** (`@xterm/xterm` + addons) - In-browser terminal emulator
  - `@xterm/addon-fit` - Terminal resizing
  - `@xterm/addon-web-links` - URL detection
  - `@xterm/addon-webgl` - WebGL rendering
  - `@xterm/addon-unicode11` - Unicode support

**Content Rendering:**
- **React Markdown** - Markdown rendering with GitHub Flavored Markdown

**Utilities:**
- **godotenv** - Environment variable loading
- **structlog** (Python) - Structured logging
- **pydantic** (Python) - Settings validation

## Configuration

**Environment:**
- Configuration via environment variables (12-factor app style)
- `.env` file support for local development
- `.env.example` documents all required variables

**Key Configuration Files:**
- `/home/ashu/Downloads/mycodes/shsh-labs/go.mod` - Go dependencies
- `/home/ashu/Downloads/mycodes/shsh-labs/package.json` - Node.js dependencies
- `/home/ashu/Downloads/mycodes/shsh-labs/python-agent/pyproject.toml` - Python dependencies
- `/home/ashu/Downloads/mycodes/shsh-labs/vite.config.js` - Frontend build config
- `/home/ashu/Downloads/mycodes/shsh-labs/.golangci.yml` - Go linting rules
- `/home/ashu/Downloads/mycodes/shsh-labs/docker-compose.yml` - Service orchestration

**Build:**
- Multi-stage Docker builds for optimization
- Frontend embedded in Go binary via `go:embed`
- Distroless final images for security
- Lockfile-driven Python builds (uv.lock)

## Platform Requirements

**Development:**
- Go 1.24+
- Node.js 22+
- Python 3.11+ (with uv)
- Docker Engine 24+
- Make
- Protocol Buffers compiler (for regenerating protos)

**Production:**
- Docker Engine with container runtime (runc or gVisor/runsc)
- Linux x86_64 (AMD64)
- Redis server
- Optional: Google API key for AI features (terminal-only mode works without)

**Deployment Targets:**
- Docker Compose (primary)
- Individual containers via Makefile
- Distroless runtime images

---

*Stack analysis: 2026-02-18*
