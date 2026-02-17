"""Regression tests for gRPC streaming behavior (Refactored)."""

import sys
import types
from types import SimpleNamespace
from unittest.mock import MagicMock

import pytest

from app.config import Settings
from app.pipeline.silence import SessionState
from app.pipeline.types import PipelineResponse


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

from app.server import AgentServicer, agent_pb2  # noqa: E402


class _StubApp:
    def __init__(self, events):
        self._events = events

    async def astream_events(self, state, config, version):  # noqa: ARG002
        for event in self._events:
            yield event


def _servicer_with_store(events, is_chat=False):
    settings = Settings()
    servicer = AgentServicer(settings)
    servicer.session_store = MagicMock()
    servicer.session_store.load.return_value = SessionState(user_id="u1")
    if is_chat:
        servicer.chat_app = _StubApp(events)
    else:
        servicer.terminal_app = _StubApp(events)
    return servicer


@pytest.mark.asyncio
async def test_process_terminal_emits_guardian_node_response() -> None:
    events = [
        {
            "event": "on_chain_end",
            "name": "guardian",
            "data": {
                "output": {
                    "response": PipelineResponse(type="pattern", content="tip", sidebar="tip")
                }
            },
        }
    ]
    servicer = _servicer_with_store(events)
    request = agent_pb2.TerminalInput(
        user_id="u1",
        session_id="",
        command="cd ..",
        pwd="",
        exit_code=0,
        output="",
    )

    messages = [msg async for msg in servicer.ProcessTerminal(request, None)]

    assert len(messages) == 1
    assert messages[0].type == "pattern"
    assert messages[0].content == "tip"


@pytest.mark.asyncio
async def test_process_terminal_uses_final_llm_response_not_chunk_stream() -> None:
    llm_response = PipelineResponse(type="llm", content="final answer", sidebar="final answer")
    events = [
        {
            "event": "on_chat_model_stream",
            "name": "planner_llm",  # updated name from Refactor
            "data": {"chunk": SimpleNamespace(content="token1")},
        },
        {
            "event": "on_chain_end",
            "name": "LangGraph",
            "data": {"output": {"response": llm_response, "session": SessionState(user_id="u1")}},
        },
    ]
    servicer = _servicer_with_store(events)
    request = agent_pb2.TerminalInput(
        user_id="u1",
        session_id="",
        command="badcmd",
        pwd="",
        exit_code=127,
        output="",
    )

    messages = [msg async for msg in servicer.ProcessTerminal(request, None)]

    assert len(messages) == 1
    assert messages[0].type == "llm"
    assert messages[0].content == "final answer"
    servicer.session_store.save.assert_called_once()


@pytest.mark.asyncio
async def test_process_terminal_emits_llm_response_from_node_end_event() -> None:
    llm_response = PipelineResponse(
        type="llm", content="node-end answer", sidebar="node-end answer"
    )
    events = [
        {
            "event": "on_chain_end",
            "name": "planner",
            "data": {"output": {"response": llm_response}},
        },
    ]
    servicer = _servicer_with_store(events)
    request = agent_pb2.TerminalInput(
        user_id="u1",
        session_id="",
        command="badcmd",
        pwd="",
        exit_code=127,
        output="",
    )

    messages = [msg async for msg in servicer.ProcessTerminal(request, None)]

    assert len(messages) == 1
    assert messages[0].type == "llm"
    assert messages[0].content == "node-end answer"


@pytest.mark.asyncio
async def test_process_terminal_dedupes_guardian_and_final_output() -> None:
    response = PipelineResponse(type="pattern", content="tip", sidebar="tip")
    events = [
        {
            "event": "on_chain_end",
            "name": "guardian",
            "data": {"output": {"response": response}},
        },
        {
            "event": "on_chain_end",
            "name": "LangGraph",
            "data": {"output": {"response": response, "session": SessionState(user_id="u1")}},
        },
    ]
    servicer = _servicer_with_store(events)
    request = agent_pb2.TerminalInput(
        user_id="u1",
        session_id="",
        command="cd ..",
        pwd="",
        exit_code=0,
        output="",
    )

    messages = [msg async for msg in servicer.ProcessTerminal(request, None)]

    assert len(messages) == 1
    assert messages[0].type == "pattern"


@pytest.mark.asyncio
async def test_chat_persists_session_and_falls_back_to_final_response() -> None:
    events = [
        {
            "event": "on_chain_end",
            "name": "LangGraph",
            "data": {
                "output": {
                    "response": PipelineResponse(type="llm", content="fallback reply"),
                    "session": SessionState(user_id="u1"),
                }
            },
        }
    ]
    servicer = _servicer_with_store(events, is_chat=True)
    request = agent_pb2.ChatRequest(user_id="u1", session_id="", message="hello")

    messages = [msg async for msg in servicer.Chat(request, None)]

    assert len(messages) == 2
    assert messages[0].content == "fallback reply"
    assert messages[0].is_complete is False
    assert messages[1].is_complete is True
    servicer.session_store.save.assert_called_once()
