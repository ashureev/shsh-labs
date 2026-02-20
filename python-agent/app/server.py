"""gRPC server for SHSH Python agent."""

from __future__ import annotations

import asyncio
import logging
import re
import time
from typing import AsyncIterator, Optional

import grpc
import redis
from grpc import ServicerContext
from langchain_core.messages import HumanMessage

from app.checkpointer import CheckpointerHandle, close_checkpointer, create_checkpointer
from app.config import Settings
from app.generated import agent_pb2, agent_pb2_grpc
from app.graph_builder import AgentState, GraphBuilder
from app.pipeline.llm import create_llm_client
from app.pipeline.silence import SessionState
from app.pipeline.types import PipelineResponse
from app.session_store import SessionStore
from app.utils import ensure_message_id

# Allowlist for user_id: alphanumeric, hyphens, underscores only.
_USER_ID_RE = re.compile(r"^[a-zA-Z0-9_\-]+$")


def _validate_id(
    value: str | None,
    name: str,
    max_len: int,
    *,
    required: bool = True,
    pattern: re.Pattern[str] | None = None,
    default: str = "",
) -> str:
    """Validate an identifier string. Raises ValueError if invalid."""
    if not value:
        if required:
            raise ValueError(f"{name} is required")
        return default
    if not isinstance(value, str):
        raise ValueError(f"{name} must be a string")
    if len(value) > max_len:
        raise ValueError(f"{name} exceeds maximum length of {max_len} characters")
    if pattern and not pattern.match(value):
        raise ValueError(
            f"{name} contains invalid characters (allowed: a-z, A-Z, 0-9, _, -)"
        )
    return value


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
        """Build an AgentState dictionary with common defaults."""
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
            user_id = _validate_id(request.user_id, "user_id", 128, pattern=_USER_ID_RE)
            session_id = _validate_id(
                request.session_id,
                "session_id",
                256,
                required=False,
                default=user_id,
            )
            streamed_content = False
            emitted_error = False

            session = self.session_store.load(user_id, session_id) or SessionState(user_id=user_id)
            config = {"configurable": {"thread_id": session_id}}
            state = self._build_agent_state(
                user_id=user_id,
                session_id=session_id,
                session=session,
                messages=[ensure_message_id(HumanMessage(content=request.message))],
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
                            self.session_store.save(output["session"], session_id)

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
            logger.exception("chat request failed", extra={"error_type": type(exc).__name__})
            yield agent_pb2.ChatResponse(
                content="",
                is_complete=True,
                response_type="error",
                error_message=str(exc),
            )
        except Exception as exc:  # noqa: BLE001
            logger.exception(
                "chat request failed: unexpected error",
                extra={"error_type": type(exc).__name__},
            )
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
        # Initialize user_id before try so the except handler can always reference
        # it, even if _validate_user_id raises before the inner assignment.
        user_id = request.user_id
        try:
            user_id = _validate_id(request.user_id, "user_id", 128, pattern=_USER_ID_RE)
            session_id = _validate_id(
                request.session_id,
                "session_id",
                256,
                required=False,
                default=user_id,
            )
            emitted_response_key: Optional[tuple] = None
            session = self.session_store.load(user_id, session_id) or SessionState(user_id=user_id)

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
                    self.session_store.save(output["session"], session_id)

                response = output.get("response")
                if response:
                    response_key = self._response_key(response)
                    if emitted_response_key != response_key:
                        yield self._to_proto_response(response, user_id)
                        emitted_response_key = response_key

        except (ValueError, grpc.RpcError, asyncio.CancelledError) as exc:
            logger.exception("terminal processing failed", extra={"error_type": type(exc).__name__})
            yield agent_pb2.AgentResponse(
                type="error",
                content=f"Error processing command: {exc}",
                user_id=user_id,
            )
        except Exception as exc:  # noqa: BLE001
            logger.exception(
                "terminal processing failed: unexpected error",
                extra={"error_type": type(exc).__name__},
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
            user_id = _validate_id(request.user_id, "user_id", 64, pattern=_USER_ID_RE)
            session_id = _validate_id(
                request.session_id,
                "session_id",
                256,
                required=False,
                default=user_id,
            )
            session = self.session_store.load(user_id, session_id) or SessionState(user_id=user_id)
            session.in_editor_mode = request.in_editor_mode
            session.is_typing = request.is_typing
            session.just_self_corrected = request.just_self_corrected
            self.session_store.save(session, session_id)
            return agent_pb2.SessionSignalResponse(ok=True, status="updated")
        except (ValueError, redis.RedisError) as exc:
            logger.exception(
                "UpdateSessionSignals failed",
                extra={"error_type": type(exc).__name__},
            )
            return agent_pb2.SessionSignalResponse(ok=False, status=str(exc))
        except Exception as exc:  # noqa: BLE001
            logger.exception(
                "UpdateSessionSignals failed: unexpected error",
                extra={"error_type": type(exc).__name__},
            )
            return agent_pb2.SessionSignalResponse(ok=False, status="An unexpected error occurred")

    async def ResetSession(  # noqa: N802
        self,
        request: agent_pb2.ResetSessionRequest,
        context: ServicerContext,
    ) -> agent_pb2.ResetSessionResponse:
        try:
            user_id = _validate_id(request.user_id, "user_id", 128, pattern=_USER_ID_RE)
            session_id = _validate_id(
                request.session_id,
                "session_id",
                256,
                required=False,
                default=user_id,
            )

            session_deleted = self.session_store.delete(user_id, session_id)
            checkpoint_deleted = await self._delete_checkpoint_thread(session_id)

            if session_deleted and checkpoint_deleted:
                return agent_pb2.ResetSessionResponse(ok=True, status="reset")

            status_parts = []
            if not session_deleted:
                status_parts.append("session_store_delete_failed")
            if not checkpoint_deleted:
                status_parts.append("checkpoint_delete_failed")
            status = ",".join(status_parts) if status_parts else "reset_partial"
            return agent_pb2.ResetSessionResponse(ok=False, status=status)
        except ValueError as exc:
            logger.exception("ResetSession failed", extra={"error_type": type(exc).__name__})
            return agent_pb2.ResetSessionResponse(ok=False, status=str(exc))
        except Exception as exc:  # noqa: BLE001
            logger.exception(
                "ResetSession failed: unexpected error",
                extra={"error_type": type(exc).__name__},
            )
            return agent_pb2.ResetSessionResponse(ok=False, status="An unexpected error occurred")

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

    async def _delete_checkpoint_thread(self, session_id: str) -> bool:
        if self._checkpointer_handle is None:
            return True
        saver = self._checkpointer_handle.saver
        try:
            delete_async = getattr(saver, "adelete_thread", None)
            if callable(delete_async):
                await delete_async(session_id)
                return True
            delete_sync = getattr(saver, "delete_thread", None)
            if callable(delete_sync):
                delete_sync(session_id)
                return True
            logger.warning("Checkpointer does not support thread deletion")
            return False
        except Exception:  # noqa: BLE001
            logger.exception("Failed to delete checkpoint thread", extra={"session_id": session_id})
            return False


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
