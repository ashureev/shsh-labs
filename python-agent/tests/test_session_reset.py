"""Tests for session reset and session-scoped store keys."""

import sys
import types
from unittest.mock import AsyncMock, MagicMock

import pytest


def _install_proto_stubs() -> None:
    class _ProtoMessage:
        def __init__(self, **kwargs):
            for key, value in kwargs.items():
                setattr(self, key, value)

    pb2 = types.ModuleType("agent_pb2")
    for name in (
        "ChatRequest",
        "ChatResponse",
        "TerminalInput",
        "AgentResponse",
        "SessionSignalRequest",
        "SessionSignalResponse",
        "ResetSessionRequest",
        "ResetSessionResponse",
        "HealthRequest",
        "HealthResponse",
    ):
        setattr(pb2, name, type(name, (_ProtoMessage,), {}))

    pb2_grpc = types.ModuleType("agent_pb2_grpc")
    pb2_grpc.AgentServiceServicer = type("AgentServiceServicer", (), {})

    sys.modules["app.generated.agent_pb2"] = pb2
    sys.modules["app.generated.agent_pb2_grpc"] = pb2_grpc
    sys.modules["agent_pb2"] = pb2
    sys.modules["agent_pb2_grpc"] = pb2_grpc


_install_proto_stubs()

from app.config import Settings  # noqa: E402
from app.pipeline.silence import SessionState  # noqa: E402
from app.server import AgentServicer, agent_pb2  # noqa: E402
from app.session_store import SessionStore  # noqa: E402


@pytest.mark.asyncio
async def test_reset_session_success() -> None:
    servicer = AgentServicer(Settings())
    servicer.session_store = MagicMock()
    servicer.session_store.delete.return_value = True
    saver = MagicMock()
    saver.adelete_thread = AsyncMock(return_value=None)
    servicer._checkpointer_handle = MagicMock(saver=saver)

    resp = await servicer.ResetSession(
        agent_pb2.ResetSessionRequest(user_id="u1", session_id="sess-1"),
        None,
    )

    assert resp.ok is True
    servicer.session_store.delete.assert_called_once_with("u1", "sess-1")
    saver.adelete_thread.assert_awaited_once_with("sess-1")


@pytest.mark.asyncio
async def test_reset_session_returns_error_on_checkpoint_failure() -> None:
    servicer = AgentServicer(Settings())
    servicer.session_store = MagicMock()
    servicer.session_store.delete.return_value = True
    saver = MagicMock()
    saver.adelete_thread = AsyncMock(side_effect=RuntimeError("redis down"))
    servicer._checkpointer_handle = MagicMock(saver=saver)

    resp = await servicer.ResetSession(
        agent_pb2.ResetSessionRequest(user_id="u1", session_id="sess-1"),
        None,
    )

    assert resp.ok is False
    assert "checkpoint_delete_failed" in resp.status


def test_session_store_is_scoped_by_session_id() -> None:
    settings = Settings()
    store = SessionStore(settings)

    class _FakeRedis:
        def __init__(self) -> None:
            self.data = {}

        def get(self, key):
            return self.data.get(key)

        def setex(self, key, ttl, value):  # noqa: ARG002
            self.data[key] = value

        def delete(self, key):
            self.data.pop(key, None)

        def close(self):
            return None

    store.redis = _FakeRedis()
    session = SessionState(user_id="u1")
    session.in_editor_mode = True

    store.save(session, "sess-a")
    store.save(SessionState(user_id="u1"), "sess-b")

    sess_a = store.load("u1", "sess-a")
    sess_b = store.load("u1", "sess-b")

    assert sess_a is not None and sess_a.in_editor_mode is True
    assert sess_b is not None and sess_b.in_editor_mode is False
