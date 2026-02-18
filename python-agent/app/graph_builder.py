"""Pure LangGraph orchestration for terminal and chat flows (2026 Architecture)."""

from __future__ import annotations

import asyncio
import logging
from datetime import datetime, timezone
from math import floor
from typing import Annotated, Optional, TypedDict

from langchain_core.messages import AIMessage, BaseMessage, HumanMessage, RemoveMessage
from langchain_core.runnables import RunnableConfig
from langgraph.graph import END, START, StateGraph
from langgraph.graph.message import add_messages

from app.config import Settings
from app.pipeline.llm import LLMClient
from app.pipeline.patterns import PatternEngine
from app.pipeline.safety import SafetyChecker, SafetyTier
from app.pipeline.silence import SessionState, SilenceChecker, TerminalInput
from app.pipeline.types import PipelineResponse
from app.utils import ensure_message_id

logger = logging.getLogger(__name__)

# Constants
TOKEN_ESTIMATION_RATIO = 4
MAX_COMPACT_BATCH = 30
SNIPPET_MAX_LENGTH = 180
SNIPPET_TRUNCATE_LENGTH = SNIPPET_MAX_LENGTH - len("...")  # 177


class AgentState(TypedDict):
    """
    Minimally defined state for LangGraph.
    Separates inputs, memory, and outputs.
    """
    # --- Context/Inputs ---
    user_id: str
    session_id: str
    command: str
    pwd: str
    exit_code: int
    output: str

    # --- Memory ---
    messages: Annotated[list, add_messages]
    summary: str
    session: SessionState  # Redis-backed session state

    # --- Routing/Internal ---
    # outcome: Used for deterministic routing edges (e.g. "unsafe", "silent", "pattern", "continue")
    routing_outcome: Optional[str]

    # --- Output ---
    response: Optional[PipelineResponse]


class GraphBuilder:
    """Builds terminal and chat graphs with deterministic routing (2026 Architecture)."""

    def __init__(self, settings: Settings, llm_client: LLMClient):
        self.settings = settings
        self.llm_client = llm_client
        self.safety_checker = SafetyChecker()
        self.pattern_engine = PatternEngine()
        self.silence_checker = SilenceChecker(cooldown_seconds=settings.proactive_cooldown_seconds)

    # --- Nodes ---

    async def guardian_node(self, state: AgentState) -> dict:
        """
        Guardian Node: Parallel execution of Safety, Silence, and Pattern checks.
        Acts as the first line of defense and fast-path response.
        """
        command = state["command"]

        async def check_safety():
            if not self.settings.enable_safety:
                return None
            return self.safety_checker.check(command)

        async def check_silence():
            if not self.settings.enable_silence:
                return None
            return self.silence_checker.check(
                state.get("session"),
                TerminalInput(
                    command=command,
                    pwd=state["pwd"],
                    exit_code=state["exit_code"],
                    output=state["output"],
                    timestamp=datetime.now(timezone.utc),
                    user_id=state["user_id"],
                ),
            )

        async def check_pattern():
            if not self.settings.enable_patterns:
                return None
            # Pattern matching is fast regex, safe to run in async wrapper
            return self.pattern_engine.match(command)

        # Parallel execution (FastChecks)
        safety_block, silence_decision, pattern_match = await asyncio.gather(
            check_safety(), check_silence(), check_pattern()
        )

        # 1. Safety Block (Highest Priority)
        if safety_block:
            if safety_block.tier == SafetyTier.TIER_1_HARD_BLOCK:
                return {
                    "routing_outcome": "unsafe",
                    "response": PipelineResponse(
                        type="safety",
                        block=True,
                        alert=safety_block.message,
                        content=f"Command blocked: {safety_block.message}",
                        sidebar=f"Command blocked: {safety_block.message}",
                    )
                }
            if safety_block.tier == SafetyTier.TIER_2_CONFIRM_INTENT:
                return {
                    "routing_outcome": "unsafe",
                    "response": PipelineResponse(
                        type="safety",
                        require_confirm=True,
                        content=f"Confirm intent: {safety_block.message}",
                        sidebar=f"Confirm intent: {safety_block.message}",
                    )
                }

        # 2. Pattern Check (Priority over Silence)
        if pattern_match and pattern_match.confidence >= self.settings.pattern_confidence_threshold:
            # Pattern matched - we speak.
            self.silence_checker.record_proactive_message(state.get("session"))
            return {
                "routing_outcome": "pattern",
                "response": PipelineResponse(
                    type="pattern",
                    content=pattern_match.definition.response,
                    sidebar=pattern_match.definition.response,
                    pattern=pattern_match.definition.name,
                )
            }

        # 3. Silence Check
        if silence_decision and silence_decision.silent:
            self.silence_checker.reset_self_corrected(state.get("session"))
            return {
                "routing_outcome": "silent",
                "response": PipelineResponse(type="silent", silent=True)
            }

        # 4. Continue to Planner
        return {
            "routing_outcome": "continue",
        }

    async def planner_node(self, state: AgentState, config: RunnableConfig) -> dict:
        """
        Planner Node (LLM): Prepares context and generates response.
        Formerly 'llm_node'.
        """
        # Double check routing outcome to be safe, though edges should handle it
        if state.get("routing_outcome") in ("unsafe", "pattern", "silent"):
            return {}

        # 4. LLM Generation
        if not self.settings.enable_llm or state["exit_code"] == 0:
            # Default to silent if success and no LLM or pattern
            return {
                "routing_outcome": "silent",
                "response": PipelineResponse(type="silent", silent=True)
            }

        cmd_output = self._truncate_output(state["output"])

        system_prompt = self.llm_client.build_system_prompt(mode="terminal")
        user_prompt = self.llm_client.build_terminal_prompt(
            command=state["command"],
            pwd=state["pwd"],
            exit_code=state["exit_code"],
            output=cmd_output,
        )

        (
            system_prompt,
            history,
            remove_ops,
            preflight_tokens,
            compaction_applied,
            token_count_mode,
        ) = await self._prepare_compacted_context(
            state=state,
            system_prompt=system_prompt,
            user_prompt=user_prompt,
            history_messages=state.get("messages", []),
        )

        # Execute LLM
        result = await self.llm_client.generate(
            system_prompt=system_prompt,
            user_prompt=user_prompt,
            conversation_history=history,
            user_id=state["user_id"],
            session_id=state["session_id"],
            metadata={
                "node": "planner_llm",
                "token_count_mode": token_count_mode,
                "preflight_tokens": preflight_tokens,
                "compaction_applied": compaction_applied,
            },
            preflight_tokens=preflight_tokens,
            compaction_applied=compaction_applied,
            config=config,
        )

        if result.error:
             return {
                "routing_outcome": "error",
                "response": PipelineResponse(type="error", content="AI assistant unavailable")
             }

        self.silence_checker.record_proactive_message(state.get("session"))

        # Message construction
        human_msg = HumanMessage(content=f"Command: {state['command']}")
        ai_msg = AIMessage(content=result.response)

        # Updates
        updates = {
            "messages": remove_ops
            + [ensure_message_id(human_msg), ensure_message_id(ai_msg)],
            "response": PipelineResponse(
                type="llm", content=result.response, sidebar=result.response
            ),
            "summary": state.get("summary", ""),
        }

        return updates

    async def chat_node(self, state: AgentState, config: RunnableConfig) -> dict:
        """Node for pure chat interactions."""
        current_content = ""
        if state.get("messages"):
             current_content = state["messages"][-1].content

        history_messages = state.get("messages", [])[:-1] if state.get("messages") else []

        system_prompt = self.llm_client.build_system_prompt(mode="chat")
        (
            system_prompt,
            history,
            remove_ops,
            preflight_tokens,
            compaction_applied,
            token_count_mode,
        ) = await self._prepare_compacted_context(
            state=state,
            system_prompt=system_prompt,
            user_prompt=current_content,
            history_messages=history_messages,
        )

        result = await self.llm_client.generate(
            system_prompt=system_prompt,
            user_prompt=current_content,
            conversation_history=history,
            user_id=state["user_id"],
            session_id=state["session_id"],
            metadata={"node": "chat_llm"},
            preflight_tokens=preflight_tokens,
            config=config,
        )

        if result.error:
             return {
                 "response": PipelineResponse(type="error", content="AI unavailable")
             }

        ai_msg = AIMessage(content=result.response)
        return {
            "messages": remove_ops + [ensure_message_id(ai_msg)],
            "response": PipelineResponse(
                type="llm", content=result.response, sidebar=result.response
            ),
            "summary": state.get("summary", ""),
        }


    # --- Edges & Routing ---

    def route_guardian(self, state: AgentState) -> str:
        outcome = state.get("routing_outcome")
        if outcome in ("unsafe", "pattern", "silent"):
            return END
        return "planner"

    # --- Graph Construction ---

    def build_terminal_graph(self) -> StateGraph:
        graph = StateGraph(AgentState)
        graph.add_node("guardian", self.guardian_node)
        # Pattern node merged into guardian
        graph.add_node("planner", self.planner_node)

        graph.add_edge(START, "guardian")
        graph.add_conditional_edges("guardian", self.route_guardian)
        graph.add_edge("planner", END)
        return graph

    def build_chat_graph(self) -> StateGraph:
        graph = StateGraph(AgentState)
        graph.add_node("chat", self.chat_node)
        graph.add_edge(START, "chat")
        graph.add_edge("chat", END)
        return graph

    # --- Helpers (Compaction/Utils) ---

    def _to_history(self, messages: list) -> list[dict[str, str]]:
        history: list[dict[str, str]] = []
        for message in messages:
            if not hasattr(message, "type") or not hasattr(message, "content"):
                continue
            if message.type not in ("ai", "human"):
                continue
            role = "assistant" if message.type == "ai" else "user"
            history.append({"role": role, "content": str(message.content)})
        return history

    def _compose_system_prompt(self, system_prompt: str, summary: str) -> str:
        if not summary:
            return system_prompt
        return f"{system_prompt}\n\nConversation summary:\n{summary}"

    def _merge_summary(self, previous: str, old_messages: list[BaseMessage]) -> str:
        lines: list[str] = []
        for message in old_messages:
            role = "assistant" if getattr(message, "type", "") == "ai" else "user"
            content = str(getattr(message, "content", "")).strip()
            if not content:
                continue
            snippet = content.replace("\n", " ")
            if len(snippet) > SNIPPET_MAX_LENGTH:
                snippet = f"{snippet[:SNIPPET_TRUNCATE_LENGTH]}..."
            lines.append(f"- {role}: {snippet}")

        merged = previous.strip()
        if lines:
            additions = "\n".join(lines)
            merged = f"{merged}\n{additions}" if merged else additions

        max_chars = self.settings.conversation_summary_max_chars
        if len(merged) <= max_chars:
            return merged
        return merged[len(merged) - max_chars :]

    def _messages_to_transcript(self, messages: list[BaseMessage]) -> str:
        lines: list[str] = []
        for message in messages:
            message_type = getattr(message, "type", "")
            if message_type not in ("human", "ai"):
                continue
            role = "assistant" if message_type == "ai" else "user"
            content = str(getattr(message, "content", "")).strip()
            if not content:
                continue
            lines.append(f"{role}: {content}")
        return "\n".join(lines)

    def _truncate_output(self, output: str) -> str:
        max_size = self.settings.max_output_size_kb * 1024
        if len(output) <= max_size:
            return output
        lines = output.splitlines()
        keep = self.settings.head_tail_lines
        if len(lines) <= keep * 2:
            return output[:max_size]
        return "\n".join(lines[:keep]) + "\n...\n" + "\n".join(lines[-keep:])

    async def _summarize_messages(
        self,
        previous_summary: str,
        old_messages: list[BaseMessage],
        user_id: str,
        session_id: str,
    ) -> str:
        if not old_messages:
            return previous_summary

        transcript = self._messages_to_transcript(old_messages)
        if not transcript:
            return previous_summary

        fallback_summary = self._merge_summary(previous_summary, old_messages)
        if not self.settings.enable_llm:
            return fallback_summary

        system_prompt = (
            "You maintain compact memory for an AI Linux tutor conversation. "
            "Return only the updated summary. Keep it factual and concise. "
            "Maximum 6 bullet lines and 120 words total."
        )
        if previous_summary.strip():
            user_prompt = (
                "Existing summary:\n"
                f"{previous_summary}\n\n"
                "New conversation lines:\n"
                f"{transcript}\n\n"
                "Extend the summary with new durable details only. "
                "Keep goals, key facts, decisions, and unresolved questions. "
                "Drop repetition, greetings, and temporary details."
            )
        else:
            user_prompt = (
                "Conversation lines:\n"
                f"{transcript}\n\n"
                "Create an initial concise summary with goals, key facts, "
                "decisions, and unresolved questions. "
                "Drop repetition, greetings, and temporary details."
            )

        result = await self.llm_client.generate(
            system_prompt=system_prompt,
            user_prompt=user_prompt,
            conversation_history=[],
            user_id=user_id,
            session_id=session_id,
            metadata={"node": "summary_llm"},
            enforce_rate_limit=False,
        )
        if result.error or not result.response.strip():
            return fallback_summary

        summary = result.response.strip()
        max_chars = self.settings.conversation_summary_max_chars
        if len(summary) <= max_chars:
            return summary
        return summary[:max_chars]

    async def _prepare_compacted_context(
        self,
        state: AgentState,
        system_prompt: str,
        user_prompt: str,
        history_messages: list[BaseMessage],
    ) -> tuple[str, list[dict[str, str]], list[RemoveMessage], int, bool, str]:
        summary = state.get("summary", "")
        compacted = False
        token_mode = "estimated"
        remove_ops: list[RemoveMessage] = []

        # Basic config checks
        keep_messages = max(
            self.settings.conversation_min_recent_messages,
            self.settings.conversation_recent_turns_keep * 2,
        )
        soft_limit = floor(
            self.settings.gemini_context_window_tokens * self.settings.conversation_soft_token_ratio
        )

        min_messages = self.settings.conversation_compaction_trigger_min_messages
        if len(history_messages) < min_messages:
            total_chars = sum(len(getattr(m, "content", "")) for m in history_messages)
            estimated = total_chars // TOKEN_ESTIMATION_RATIO
            return (
                system_prompt,
                self._to_history(history_messages),
                remove_ops,
                estimated,
                compacted,
                "estimated_skipped",
            )

        # Preflight check
        working_messages = [ensure_message_id(m) for m in history_messages]
        composed_system = self._compose_system_prompt(system_prompt, summary)
        history = self._to_history(working_messages)

        preflight, token_mode = await self.llm_client.count_tokens_for_messages(
            self.llm_client.build_messages(
                system_prompt=composed_system,
                user_prompt=user_prompt,
                conversation_history=history,
            )
        )

        if not self.settings.conversation_compaction_enabled or preflight <= soft_limit:
            return composed_system, history, remove_ops, preflight, compacted, token_mode

        # Compaction needed
        if len(working_messages) <= keep_messages:
            return composed_system, history, remove_ops, preflight, compacted, token_mode

        compact_count = min(len(working_messages) - keep_messages, MAX_COMPACT_BATCH)
        to_compact = working_messages[:compact_count]
        working_messages = working_messages[compact_count:]

        summary = await self._summarize_messages(
            previous_summary=summary,
            old_messages=to_compact,
            user_id=state["user_id"],
            session_id=state["session_id"],
        )
        compacted = True

        # Side effect to persist summary immediately if needed.
        # Ideally we rely on graph state update returned by PlannerNode.
        state["summary"] = summary

        for message in to_compact:
            if getattr(message, "id", None):
                remove_ops.append(RemoveMessage(id=message.id))

        # Re-calc
        composed_system = self._compose_system_prompt(system_prompt, summary)
        history = self._to_history(working_messages)
        preflight, token_mode = await self.llm_client.count_tokens_for_messages(
             self.llm_client.build_messages(
                system_prompt=composed_system,
                user_prompt=user_prompt,
                conversation_history=history,
            )
        )

        return composed_system, history, remove_ops, preflight, compacted, token_mode
