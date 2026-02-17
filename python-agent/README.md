# Python Agent Service

The Python Agent Service is a gRPC-based AI/ML processing service that replaces the embedded Go MicroAgent. It provides the same functionality (safety checking, pattern matching, silence policy, and LLM integration) using Python's rich ML ecosystem.

## Architecture

```
Frontend (React) → Go Backend (Chi) → gRPC Client → Python Agent Service
                                                          ↓
                                                   Safety/Pattern/Silence/LLM
                                                          ↓
                                                    LLM (Gemini/Claude)
                                                           ↓
                                                    gRPC Response
                                                           ↓
                                         Go Backend → SSE → Frontend
```

## Features

- **Safety Checker**: Tiered safety system (Tier 1 Hard Block, Tier 2 Confirm, Tier 3 Log)
- **Pattern Engine**: Regex-based pattern matching with confidence scoring
- **Silence Policy**: Intelligent silence rules to avoid spam
- **LLM Integration**: LangChain-based LLM client with circuit breaker and rate limiting
- **LangGraph Runtime**: Graph-based chat/terminal flow with checkpoint persistence
- **Redis Support**: Session state storage via Redis

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Start all services
docker compose up -d

# View logs
docker compose logs -f python-agent

# Stop all services
docker compose down
```

### Local Development

1. Install dependencies:
```bash
cd python-agent
uv sync
```

2. Set environment variables:
```bash
export GOOGLE_API_KEY=your_key
export REDIS_URL=localhost:6379
export GRPC_PORT=50051
```

3. Generate protobuf code:
```bash
make proto-generate-python
```

4. Run the service:
```bash
python -m app.main
```

### Docker Image Build (Optimized)

The production image is lockfile-driven (`uv.lock`) and uses a distroless runtime.

```bash
# Standard build
docker build -f python-agent/Dockerfile -t shsh-python-agent:latest ./python-agent

# Optimized buildx variant (zstd compression + optional cache flags)
make docker-build-python-agent-optimized \
  PY_AGENT_BUILDX_ARGS="--cache-from=type=registry,ref=your-registry/shsh-python-agent:cache --cache-to=type=registry,ref=your-registry/shsh-python-agent:cache,mode=max"
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `GRPC_PORT` | 50051 | gRPC server port |
| `LOG_LEVEL` | INFO | Logging level |
| `REDIS_URL` | localhost:6379 | Redis connection URL |
| `LLM_PROVIDER` | google | LLM provider (google/anthropic/openrouter) |
| `LLM_MODEL` | gemini-2.0-flash-exp | LLM model name |
| `GOOGLE_API_KEY` | - | Google API key |
| `PATTERN_CONFIDENCE_THRESHOLD` | 0.7 | Minimum pattern confidence |
| `PROACTIVE_COOLDOWN_SECONDS` | 120 | Cooldown between proactive messages |
| `GEMINI_COUNT_TOKENS_TIMEOUT_SECONDS` | 3 | Timeout for Gemini token preflight before estimate fallback |

## gRPC API

### Chat (Unary)
```protobuf
rpc Chat(ChatRequest) returns (ChatResponse);
```

### Process Terminal
```protobuf
rpc ProcessTerminal(TerminalInput) returns (AgentResponse);
```

### Health Check
```protobuf
rpc Health(HealthRequest) returns (HealthResponse);
```

## Project Structure

```
python-agent/
├── app/
│   ├── __init__.py
│   ├── main.py              # Entry point
│   ├── config.py            # Pydantic settings
│   ├── server.py            # gRPC servicer
│   ├── generated/           # Generated protobuf code
│   └── pipeline/
│       ├── __init__.py
│       ├── safety.py        # Safety checker
│       ├── patterns.py      # Pattern engine
│       ├── silence.py       # Silence checker
│       ├── llm.py           # LLM client
│       └── types.py         # Shared pipeline response model
├── proto/
│   └── agent.proto          # Protobuf definition
├── Dockerfile
├── uv.lock
├── requirements.txt
└── pyproject.toml
```

## Testing

```bash
# Run Python tests
pytest -q

# Run with coverage
pytest --cov=app tests/
```

## Integration with Go Backend

Set `PYTHON_AGENT_ADDR=localhost:50051` in the Go backend to enable the Python gRPC agent.
Current runtime is LangGraph-only on the Python side.
