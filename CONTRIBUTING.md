# Contributing to SHSH Playground

Thanks for your interest in contributing! Here's how to get started.

## Development Setup

```bash
git clone https://github.com/ashureev/shsh-labs
cd shsh-labs
cp .env.example .env
# Set your GOOGLE_API_KEY in .env
```

### Backend (Go 1.24+)

```bash
go build ./cmd/server
go test ./...
```

### Frontend (Node 22+)

```bash
npm install
npm run dev
npm run lint
```

### Full Stack (Docker)

```bash
docker compose build
docker compose --profile build build
docker compose up -d
```

## Making Changes

1. Fork the repo and create a feature branch
2. Make your changes
3. Run tests: `go test ./...` and `npm run lint`
4. Submit a pull request

## Code Style

- **Go**: Follow standard Go conventions. Run `go vet ./...`
- **Frontend**: ESLint + Prettier configured via `npm run lint`
- **Commits**: Clear, descriptive messages

## Project Structure

```
cmd/server/          # Go entrypoint
internal/            # Backend packages (api, agent, container, store, etc.)
python-agent/        # Python LangGraph agent service
src/                 # React frontend (Vite)
```

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
