# Docker Operations Runbook

## Daily Commands

- Build all images:
```bash
make docker-build-all
```

- Start stack with existing images (safe default):
```bash
make docker-up
```

- Rebuild and start stack:
```bash
make docker-up-build
```

- Check service status:
```bash
make docker-status
```

- Stop stack:
```bash
make docker-down
```

- Follow backend container logs:
```bash
make docker-logs
```

- Safe manual cleanup (dangling images only):
```bash
make docker-clean
```

## Image Names Used

- Backend: `shsh-backend:latest`
- Python agent: `shsh-python-agent:latest`
- Playground: `playground:latest`
- Backend health probe sidecar: `curlimages/curl:8.12.1`

## Notes

- The backend provisions learner containers from `playground:latest`.
- `make docker-up` uses `docker compose up -d --no-build` so it does not overwrite optimized local images.
- If you intentionally want fresh builds for services, use `make docker-up-build`.
- Backend now runs on a distroless runtime (`gcr.io/distroless/static-debian12`) for smaller attack surface.
- Distroless images do not include shell/curl/package manager, so backend health checks are handled by the `backend-healthcheck` compose sidecar.

## Troubleshooting

- Python agent unhealthy due to protobuf import errors:
  Rebuild Python image with `make docker-build-python-agent` then `make docker-up-build`.

- Docker daemon/socket permission issues:
  Confirm Docker is running and your user can access `/var/run/docker.sock`.

- Stale image tags:
  Rebuild with `make docker-build-all`, then restart using `make docker-up-build`.

- Backend health status:
  Run `docker inspect --format='{{json .State.Health.Status}}' shsh-backend-healthcheck` to verify the backend HTTP `/health` endpoint is reachable.
