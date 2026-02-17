# syntax=docker/dockerfile:1.7
FROM ghcr.io/astral-sh/uv:python3.11-bookworm-slim AS deps

WORKDIR /app

# Prepare locked dependency exports first to maximize cache reuse.
COPY pyproject.toml uv.lock ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv export --frozen --format requirements.txt --no-dev --no-emit-project -o /tmp/requirements.runtime.txt && \
    uv pip install --system --prefix /opt/python -r /tmp/requirements.runtime.txt && \
    uv export --frozen --format requirements.txt --only-group build --no-emit-project -o /tmp/requirements.build.txt && \
    uv pip install --system -r /tmp/requirements.build.txt

FROM deps AS builder

WORKDIR /app

# Copy application code after dependency layers.
COPY proto/ ./proto/
COPY app/ ./app/

# Generate protobuf code.
RUN python -m grpc_tools.protoc \
    -I./proto \
    --python_out=./app/generated \
    --grpc_python_out=./app/generated \
    ./proto/agent.proto

# grpc_tools generates absolute imports by default; make them package-relative
# so imports work when loaded via app.generated.
RUN sed -i 's/^import agent_pb2 as/from . import agent_pb2 as/' ./app/generated/agent_pb2_grpc.py && \
    touch ./app/generated/__init__.py

# hadolint ignore=DL3006
FROM gcr.io/distroless/python3-debian12

WORKDIR /app

COPY --link --from=deps /opt/python /opt/python
COPY --link --from=builder /app/app ./app

ENV PYTHONPATH=/opt/python/lib/python3.11/site-packages:/app \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONOPTIMIZE=2

EXPOSE 50051

CMD ["-m", "app.main"]
