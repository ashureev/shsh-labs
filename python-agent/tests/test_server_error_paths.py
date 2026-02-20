"""Tests for server.py error paths and input validation.

Covers:
- Fix 3: ProcessTerminal UnboundLocalError — user_id must be defined before try block
- Fix 9: _validate_user_id character-set allowlist
"""

import sys
import types
from unittest.mock import MagicMock, patch

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

from app.server import _USER_ID_RE, AgentServicer, _validate_id, agent_pb2  # noqa: E402

# ---------------------------------------------------------------------------
# Fix 9: _validate_user_id character-set validation
# ---------------------------------------------------------------------------


class TestValidateUserId:
    def test_valid_alphanumeric(self):
        assert _validate_id("alice123", "user_id", 128, pattern=_USER_ID_RE) == "alice123"

    def test_valid_with_hyphen(self):
        assert _validate_id("alice-bob", "user_id", 128, pattern=_USER_ID_RE) == "alice-bob"

    def test_valid_with_underscore(self):
        assert _validate_id("alice_bob", "user_id", 128, pattern=_USER_ID_RE) == "alice_bob"

    def test_valid_mixed(self):
        assert _validate_id("Alice_Bob-123", "user_id", 128, pattern=_USER_ID_RE) == "Alice_Bob-123"

    def test_rejects_empty(self):
        with pytest.raises(ValueError, match="required"):
            _validate_id("", "user_id", 128, pattern=_USER_ID_RE)

    def test_rejects_path_separator_forward_slash(self):
        """Path separators could affect Redis key construction."""
        with pytest.raises(ValueError, match="invalid characters"):
            _validate_id("alice/bob", "user_id", 128, pattern=_USER_ID_RE)

    def test_rejects_path_separator_backslash(self):
        with pytest.raises(ValueError, match="invalid characters"):
            _validate_id("alice\\bob", "user_id", 128, pattern=_USER_ID_RE)

    def test_rejects_dot_dot(self):
        """Directory traversal attempt."""
        with pytest.raises(ValueError, match="invalid characters"):
            _validate_id("../etc/passwd", "user_id", 128, pattern=_USER_ID_RE)

    def test_rejects_space(self):
        with pytest.raises(ValueError, match="invalid characters"):
            _validate_id("alice bob", "user_id", 128, pattern=_USER_ID_RE)

    def test_rejects_at_sign(self):
        with pytest.raises(ValueError, match="invalid characters"):
            _validate_id("alice@example.com", "user_id", 128, pattern=_USER_ID_RE)

    def test_rejects_too_long(self):
        with pytest.raises(ValueError, match="maximum length"):
            _validate_id("a" * 129, "user_id", 128, pattern=_USER_ID_RE)


# ---------------------------------------------------------------------------
# Fix 3: ProcessTerminal — user_id must not be UnboundLocalError in except path
# ---------------------------------------------------------------------------


def _servicer_with_failing_store():
    """Return a servicer whose session_store.load raises to trigger the except path."""
    from app.config import Settings

    servicer = AgentServicer(Settings())
    servicer.session_store = MagicMock()
    # Raise after user_id is validated so we reach the except block.
    servicer.session_store.load.side_effect = RuntimeError("store unavailable")
    return servicer


@pytest.mark.asyncio
async def test_process_terminal_no_unbound_local_error_on_store_failure() -> None:
    """ProcessTerminal must not raise UnboundLocalError when session_store.load fails.

    Before Fix 3, user_id was only assigned inside the try block. If _validate_user_id
    succeeded but session_store.load raised, the except handler referenced user_id
    before it was bound, causing UnboundLocalError instead of a clean error response.
    """
    servicer = _servicer_with_failing_store()
    request = agent_pb2.TerminalInput(
        user_id="valid-user",
        session_id="",
        command="ls",
        pwd="",
        exit_code=0,
        output="",
    )

    # Patch the structlog logger so its extra kwargs don't fail under stdlib logging.
    mock_logger = MagicMock()
    with patch("app.server.logger", mock_logger):
        messages = [msg async for msg in servicer.ProcessTerminal(request, None)]

    assert len(messages) >= 1
    assert messages[0].type == "error"
    # user_id must be echoed back correctly (proves it was bound before except)
    assert messages[0].user_id == "valid-user"


@pytest.mark.asyncio
async def test_process_terminal_rejects_invalid_user_id() -> None:
    """ProcessTerminal must return an error response for invalid user_id chars."""
    from app.config import Settings

    servicer = AgentServicer(Settings())
    servicer.session_store = MagicMock()

    request = agent_pb2.TerminalInput(
        user_id="bad/user",  # path separator — rejected by _validate_user_id
        session_id="",
        command="ls",
        pwd="",
        exit_code=0,
        output="",
    )

    mock_logger = MagicMock()
    with patch("app.server.logger", mock_logger):
        messages = [msg async for msg in servicer.ProcessTerminal(request, None)]

    assert len(messages) >= 1
    assert messages[0].type == "error"
