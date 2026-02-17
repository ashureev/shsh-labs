# SHSH Agentic Linux Tutor - Makefile
# Coder-inspired build system

.PHONY: all build test lint clean dev install-tools migrate proto-generate proto-generate-go proto-generate-python proto-clean \
	docker-build docker-build-backend docker-build-python-agent docker-build-python-agent-optimized docker-build-playground docker-build-all docker-run docker-stop docker-logs \
	docker-up docker-up-build docker-down docker-status docker-clean

# Variables
BINARY_NAME := shsh
PLAYGROUND_BINARY := playground_server
MAIN_PKG := ./cmd/server
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags "-s -w"

# Proto variables
PROTO_DIR := python-agent/proto
PROTO_GO_OUT := internal/proto/agent
PROTO_PY_OUT := python-agent/app/generated

# Default target
all: build

# Build the main server binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PKG)

# Build the playground server
build-playground:
	@echo "Building $(PLAYGROUND_BINARY)..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(PLAYGROUND_BINARY) ./cmd/playground

# Build all binaries
build-all: build build-playground

# Run tests
test:
	@echo "Running tests..."
	$(GO) test $(GOFLAGS) ./internal/... -short

# Run all tests including integration
test-integration:
	@echo "Running integration tests..."
	$(GO) test $(GOFLAGS) ./internal/... -tags=integration

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test $(GOFLAGS) -coverprofile=coverage.out ./internal/...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Run terminal tests specifically
test-terminal:
	@echo "Running terminal tests..."
	$(GO) test $(GOFLAGS) ./internal/terminal/... -v

# Lint the code
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Vet code
vet:
	@echo "Running go vet..."
	$(GO) vet ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME) $(PLAYGROUND_BINARY)
	rm -f coverage.out coverage.html
	$(GO) clean

# Install development tools
install-tools:
	@echo "Installing development tools..."
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/cosmtrek/air@latest

# Development server with hot reload
dev:
	@echo "Starting development server..."
	air

# Run the server
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Database operations
migrate:
	@echo "Running database migrations..."
	$(GO) run ./cmd/migrate

# Docker image naming
BACKEND_IMAGE := shsh-backend
PY_AGENT_IMAGE := shsh-python-agent
PLAYGROUND_IMAGE := playground
DOCKER_TAG := latest
PY_AGENT_BUILDX_ARGS ?=

# Docker commands (canonical low-risk workflow)
docker-build-backend:
	@echo "Building backend Docker image..."
	docker build -f Dockerfile.backend -t $(BACKEND_IMAGE):$(DOCKER_TAG) .

docker-build:
	@$(MAKE) docker-build-backend

docker-build-python-agent:
	@echo "Building Python Agent Docker image..."
	docker build -f python-agent/Dockerfile -t $(PY_AGENT_IMAGE):$(DOCKER_TAG) ./python-agent

docker-build-python-agent-optimized:
	@echo "Building Python Agent Docker image with buildx cache + zstd compression..."
	docker buildx build --load \
	  -f python-agent/Dockerfile \
	  $(PY_AGENT_BUILDX_ARGS) \
	  --output type=docker,compression=zstd,compression-level=3 \
	  -t $(PY_AGENT_IMAGE):$(DOCKER_TAG) ./python-agent

docker-build-playground:
	@echo "Building playground Docker image..."
	docker build -f Dockerfile -t $(PLAYGROUND_IMAGE):$(DOCKER_TAG) .

docker-build-all: docker-build-backend docker-build-python-agent docker-build-playground
	@echo "All Docker images built."

docker-run:
	@echo "Running backend Docker container..."
	docker run -d \
	  --name $(BACKEND_IMAGE) \
	  --network host \
	  --restart unless-stopped \
	  -v /var/run/docker.sock:/var/run/docker.sock \
	  -v $(PWD)/.env:/.env:ro \
	  -v $(PWD)/data:/data \
	  $(BACKEND_IMAGE):$(DOCKER_TAG)

docker-stop:
	@echo "Stopping backend container..."
	docker stop $(BACKEND_IMAGE) && docker rm $(BACKEND_IMAGE)

docker-logs:
	@echo "Showing backend logs..."
	docker logs -f $(BACKEND_IMAGE)

# Frontend operations
frontend-install:
	@echo "Installing frontend dependencies..."
	npm install

frontend-dev:
	@echo "Starting frontend development server..."
	npm run dev

frontend-build:
	@echo "Building frontend..."
	npm run build

frontend-lint:
	@echo "Linting frontend..."
	npm run lint

# Full development setup
setup: install-tools frontend-install
	@echo "Development setup complete!"

# CI pipeline
ci: fmt vet lint test
	@echo "CI checks passed!"

# Protobuf generation
proto-generate: proto-generate-go proto-generate-python

proto-generate-go:
	@echo "Generating Go protobuf code..."
	@mkdir -p $(PROTO_GO_OUT)
	protoc -I$(PROTO_DIR) \
		--go_out=$(PROTO_GO_OUT) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_GO_OUT) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/agent.proto
	@echo "Go protobuf code generated in $(PROTO_GO_OUT)"

proto-generate-python:
	@echo "Generating Python protobuf code..."
	@mkdir -p $(PROTO_PY_OUT)
	python3 -m grpc_tools.protoc \
		-I$(PROTO_DIR) \
		--python_out=$(PROTO_PY_OUT) \
		--grpc_python_out=$(PROTO_PY_OUT) \
		$(PROTO_DIR)/agent.proto
	@touch $(PROTO_PY_OUT)/__init__.py
	@echo "Python protobuf code generated in $(PROTO_PY_OUT)"

proto-clean:
	@echo "Cleaning generated protobuf code..."
	rm -f $(PROTO_GO_OUT)/*.pb.go
	rm -f $(PROTO_PY_OUT)/*_pb2*.py

# Compose lifecycle
docker-up:
	@echo "Starting all services with existing images..."
	docker compose up -d --no-build

docker-up-build:
	@echo "Building and starting all services..."
	docker compose up -d --build

docker-down:
	@echo "Stopping all services..."
	docker compose down

docker-status:
	@echo "Service status..."
	docker compose ps

docker-clean:
	@echo "Pruning dangling Docker images (safe manual cleanup)..."
	docker image prune -f

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the main server binary"
	@echo "  build-playground - Build the playground server binary"
	@echo "  build-all      - Build all binaries"
	@echo "  test           - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-terminal  - Run terminal tests"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  clean          - Clean build artifacts"
	@echo "  install-tools  - Install development tools"
	@echo "  dev            - Run development server with hot reload"
	@echo "  run            - Build and run the server"
	@echo "  migrate        - Run database migrations"
	@echo "  docker-build-backend - Build backend Docker image"
	@echo "  docker-build-python-agent - Build Python Agent Docker image"
	@echo "  docker-build-python-agent-optimized - Build Python Agent with buildx + zstd (use PY_AGENT_BUILDX_ARGS for cache flags)"
	@echo "  docker-build-playground - Build playground Docker image"
	@echo "  docker-build-all - Build backend, python-agent, and playground images"
	@echo "  docker-build   - Alias for docker-build-backend"
	@echo "  docker-run     - Run backend Docker container"
	@echo "  docker-stop    - Stop and remove backend container"
	@echo "  docker-logs    - Show backend container logs"
	@echo "  docker-up      - Start compose services without rebuilding"
	@echo "  docker-up-build - Build and start compose services"
	@echo "  docker-down    - Stop compose services"
	@echo "  docker-status  - Show compose service status"
	@echo "  docker-clean   - Prune dangling Docker images"
	@echo "  frontend-install - Install frontend dependencies"
	@echo "  frontend-dev   - Start frontend dev server"
	@echo "  frontend-build - Build frontend for production"
	@echo "  frontend-lint  - Lint frontend code"
	@echo "  setup          - Full development setup"
	@echo "  ci             - Run CI pipeline"
	@echo "  proto-generate - Generate protobuf code for Go and Python"
	@echo "  proto-generate-go - Generate Go protobuf code"
	@echo "  proto-generate-python - Generate Python protobuf code"
	@echo "  proto-clean    - Clean generated protobuf code"
	@echo "  help           - Show this help message"
