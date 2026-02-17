"""gRPC server for SHSH Python agent."""

from __future__ import annotations

import asyncio
import logging
import time
from typing import AsyncIterator, Optional
from uuid import uuid4

import grpc
import redis
from grpc import ServicerContext
from langchain_core.messages import HumanMessage

from app.pipeline.llm import create_llm_client
from app.pipeline.silence import SessionState
from app.pipeline.types import PipelineResponse
from app.session_store import SessionStore

try:
    from app.generated import agent_pb2, agent_pb2_grpc
except ImportError:
    import sys

    sys.path.insert(0, "/app/app/generated")
    import agent_pb2  # type: ignore[import-not-found]
    import agent_pb2_grpc  # type: ignore[import-not-found]

from app.checkpointer import CheckpointerHandle, close_checkpointer, create_checkpointer
from app.config import Settings
from app.graph_builder import AgentState, GraphBuilder


def _ensure_message_id(message):
    """Ensure message has an ID for checkpoint removal."""
    if not getattr(message, "id", None):
        message.id = str(uuid4())
    return message


def _validate_user_id(user_id: str) -> str:
    """Validate and sanitize user_id.

    Args:
        user_id: The user ID to validate.

    Returns:
        The sanitized user_id.

    Raises:
        ValueError: If user_id is invalid.
    """
    if not user_id:
        raise ValueError("user_id is required")
    if not isinstance(user_id, str):
        raise ValueError("user_id must be a string")
    # GitHub user IDs are alphanumeric with hyphens/underscores
    # Allow additional chars for safety but validate length
    if len(user_id) > 128:
        raise ValueError("user_id exceeds maximum length of 128 characters")
    return user_id


def _validate_session_id(session_id: Optional[str], user_id: str) -> str:
    """Validate and return session_id (defaults to user_id if empty).

    Args:
        session_id: The session ID to validate, or None.
        user_id: The user ID to use as fallback.

    Returns:
        The validated session_id or user_id as fallback.

    Raises:
        ValueError: If session_id is provided but invalid.
    """
    if not session_id:
        return user_id
    if not isinstance(session_id, str):
        raise ValueError("session_id must be a string")
    if len(session_id) > 256:
        raise ValueError("session_id exceeds maximum length of 256 characters")
    return session_id


logger = logging.getLogger(__name__)


class AgentServicer(agent_pb2_grpc.AgentServiceServicer):
    """Agent gRPC implementation using LangGraph only."""

    def __init__(self, settings: Settings):
        self.settings = settings
        self.session_store = SessionStore(settings)
        self.llm_client = create_llm_client(settings)
        self._checkpointer_handle: Optional[CheckpointerHandle] = None
        self.terminal_app = None
        self.chat_app = None

    def _build_agent_state(
        self,
        user_id: str,
        session_id: str,
        session: SessionState,
        *,
        command: str = "",
        pwd: str = "",
        exit_code: int = 0,
        output: str = "",
        messages: Optional[list] = None,
    ) -> AgentState:
        """Build an AgentState dictionary with common defaults.

        Args:
            user_id: The validated user ID.
            session_id: The validated session ID.
            session: The session state object.
            command: The terminal command (for terminal mode).
            pwd: Present working directory (for terminal mode).
            exit_code: Command exit code (for terminal mode).
            output: Command output (for terminal mode).
            messages: List of messages (for chat mode).

        Returns:
            AgentState dictionary ready for LangGraph processing.
        """
        return {
            "user_id": user_id,
            "session_id": session_id,
            "command": command,
            "pwd": pwd,
            "exit_code": exit_code,
            "output": output,
            "messages": messages if messages is not None else [],
            "summary": "",
            "session": session,
            "routing_outcome": None,
            "_silence_result": None,
            "response": None,
        }

    async def initialize(self) -> None:
        graph_builder = GraphBuilder(self.settings, self.llm_client)
        self._checkpointer_handle = await create_checkpointer(self.settings)

        self.terminal_app = graph_builder.build_terminal_graph().compile(
            checkpointer=self._checkpointer_handle.saver
        )
        self.chat_app = graph_builder.build_chat_graph().compile(
            checkpointer=self._checkpointer_handle.saver
        )

        logger.info("AgentServicer initialized", extra={"langgraph": True})

    async def close(self) -> None:
        await close_checkpointer(self._checkpointer_handle)
        self.session_store.close()

    async def Chat(  # noqa: N802
        self,
        request: agent_pb2.ChatRequest,
        context: ServicerContext,
    ) -> AsyncIterator[agent_pb2.ChatResponse]:
        try:
            user_id = _validate_user_id(request.user_id)
            session_id = _validate_session_id(request.session_id, user_id)
            streamed_content = False
            emitted_error = False

            session = self.session_store.load(user_id) or SessionState(user_id=user_id)
            config = {"configurable": {"thread_id": session_id}}
            state = self._build_agent_state(
                user_id=user_id,
                session_id=session_id,
                session=session,
                messages=[_ensure_message_id(HumanMessage(content=request.message))],
            )

            async for event in self.chat_app.astream_events(state, config=config, version="v1"):
                kind = event["event"]
                if kind == "on_chat_model_stream":
                    content = event["data"]["chunk"].content
                    if content:
                        streamed_content = True
                        yield agent_pb2.ChatResponse(
                            content=content,
                            is_complete=False,
                            response_type="chat",
                        )

                if kind == "on_chain_end":
                    output = event["data"].get("output")
                    if isinstance(output, dict):
                        if "session" in output:
                            self.session_store.save(output["session"])

                        response = output.get("response")
                        if response and response.type == "error" and not emitted_error:
                            emitted_error = True
                            yield agent_pb2.ChatResponse(
                                content=response.content,
                                is_complete=True,
                                response_type="error",
                                error_message=response.content,
                            )
                            return
                        if response and response.content and not streamed_content:
                            yield agent_pb2.ChatResponse(
                                content=response.content,
                                is_complete=False,
                                response_type="chat",
                            )
                            streamed_content = True

            yield agent_pb2.ChatResponse(is_complete=True, response_type="chat")

        except (ValueError, grpc.RpcError, asyncio.CancelledError) as exc:
            logger.exception("chat request failed", error_type=type(exc).__name__)
            yield agent_pb2.ChatResponse(
                content="",
                is_complete=True,
                response_type="error",
                error_message=str(exc),
            )
        except Exception as exc:  # noqa: BLE001
            logger.exception("chat request failed: unexpected error", error_type=type(exc).__name__)
            yield agent_pb2.ChatResponse(
                content="",
                is_complete=True,
                response_type="error",
                error_message="An unexpected error occurred",
            )

    async def ProcessTerminal(  # noqa: N802
        self,
        request: agent_pb2.TerminalInput,
        context: ServicerContext,
    ) -> AsyncIterator[agent_pb2.AgentResponse]:
        try:
            user_id = _validate_user_id(request.user_id)
            session_id = _validate_session_id(request.session_id, user_id)
            emitted_response_key: Optional[tuple] = None
            session = self.session_store.load(user_id) or SessionState(user_id=user_id)

            config = {"configurable": {"thread_id": session_id}}
            state = self._build_agent_state(
                user_id=user_id,
                session_id=session_id,
                session=session,
                command=request.command,
                pwd=request.pwd,
                exit_code=request.exit_code,
                output=request.output,
            )

            async for event in self.terminal_app.astream_events(state, config=config, version="v1"):
                kind = event["event"]
                if kind != "on_chain_end":
                    continue

                output = event["data"].get("output")
                if not isinstance(output, dict):
                    continue

                if "session" in output:
                    self.session_store.save(output["session"])

                response = output.get("response")
                if response:
                    response_key = self._response_key(response)
                    if emitted_response_key != response_key:
                        yield self._to_proto_response(response, user_id)
                        emitted_response_key = response_key

        except (ValueError, grpc.RpcError, asyncio.CancelledError) as exc:
            logger.exception("terminal processing failed", error_type=type(exc).__name__)
            yield agent_pb2.AgentResponse(
                type="error",
                content=f"Error processing command: {exc}",
                user_id=user_id,
            )
        except Exception as exc:  # noqa: BLE001
            logger.exception(
                "terminal processing failed: unexpected error",
                error_type=type(exc).__name__,
            )
            yield agent_pb2.AgentResponse(
                type="error",
                content="An unexpected error occurred while processing the command",
                user_id=user_id,
            )

    async def Health(  # noqa: N802
        self,
        request: agent_pb2.HealthRequest,
        context: ServicerContext,
    ) -> agent_pb2.HealthResponse:
        return agent_pb2.HealthResponse(
            healthy=True,
            version=self.settings.service_version,
            timestamp=int(time.time()),
            status="ready",
        )

    async def UpdateSessionSignals(  # noqa: N802
        self,
        request: agent_pb2.SessionSignalRequest,
        context: ServicerContext,
    ) -> agent_pb2.SessionSignalResponse:
        try:
            user_id = _validate_user_id(request.user_id)
            session = self.session_store.load(user_id) or SessionState(user_id=user_id)
            session.in_editor_mode = request.in_editor_mode
            session.is_typing = request.is_typing
            session.just_self_corrected = request.just_self_corrected
            self.session_store.save(session)
            return agent_pb2.SessionSignalResponse(ok=True, status="updated")
        except (ValueError, redis.RedisError) as exc:
            logger.exception("UpdateSessionSignals failed", error_type=type(exc).__name__)
            return agent_pb2.SessionSignalResponse(ok=False, status=str(exc))
        except Exception as exc:  # noqa: BLE001
            logger.exception(
                "UpdateSessionSignals failed: unexpected error",
                error_type=type(exc).__name__,
            )
            return agent_pb2.SessionSignalResponse(ok=False, status="An unexpected error occurred")

    def _to_proto_response(
        self, response: PipelineResponse, user_id: str
    ) -> agent_pb2.AgentResponse:
        return agent_pb2.AgentResponse(
            type=response.type,
            content=response.content,
            sidebar=response.sidebar,
            silent=response.silent,
            alert=response.alert,
            require_confirm=response.require_confirm,
            pattern=response.pattern,
            tools_used=response.tools_used,
            block=response.block,
            user_id=user_id,
        )

    @staticmethod
    def _response_key(response: PipelineResponse) -> tuple:
        return (
            response.type,
            response.content,
            response.sidebar,
            response.silent,
            response.alert,
            response.require_confirm,
            response.pattern,
            tuple(response.tools_used),
            response.block,
        )


class AgentServer:
    """gRPC server host."""

    def __init__(self, settings: Settings):
        self.settings = settings
        self.server: Optional[grpc.aio.Server] = None
        self.servicer: Optional[AgentServicer] = None

    async def start(self) -> None:
        self.server = grpc.aio.server(
            options=[
                ("grpc.max_send_message_length", 50 * 1024 * 1024),
                ("grpc.max_receive_message_length", 50 * 1024 * 1024),
            ]
        )
        self.servicer = AgentServicer(self.settings)
        await self.servicer.initialize()
        agent_pb2_grpc.add_AgentServiceServicer_to_server(self.servicer, self.server)
        address = f"[::]:{self.settings.grpc_port}"
        self.server.add_insecure_port(address)
        await self.server.start()
        logger.info("gRPC server started", extra={"address": address})

    async def stop(self) -> None:
        if self.server:
            await self.server.stop(5)
            if self.servicer:
                await self.servicer.close()
            logger.info("gRPC server stopped")

    async def wait_for_termination(self) -> None:
        if self.server:
            await self.server.wait_for_termination()
