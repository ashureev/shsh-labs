"""Tests for LangGraph runtime (Refactored)."""

import unittest
from unittest.mock import AsyncMock, MagicMock

import pytest
from langchain_core.messages import AIMessage, HumanMessage

from app.config import Settings
from app.graph_builder import AgentState, GraphBuilder
from app.pipeline.llm import LLMClient, LLMResult
from app.pipeline.safety import SafetyBlock, SafetyTier
from app.pipeline.silence import SessionState, SilenceDecision


@pytest.fixture
def settings() -> Settings:
    return Settings(
        enable_safety=True,
        enable_patterns=True,
        enable_silence=True,
        enable_llm=True,
        proactive_cooldown_seconds=120,
    )


@pytest.fixture
def mock_llm_client() -> MagicMock:
    client = MagicMock(spec=LLMClient)
    client.build_system_prompt = MagicMock(return_value="system")
    client.build_terminal_prompt = MagicMock(return_value="terminal prompt")
    client.build_messages = MagicMock(return_value=[HumanMessage(content="ctx")])
    client.count_tokens_for_messages = AsyncMock(return_value=(64, "estimated"))
    client.generate = AsyncMock(
        return_value=LLMResult(response="Test response", duration_ms=50)
    )
    return client


@pytest.fixture
def graph_builder(settings: Settings, mock_llm_client: MagicMock) -> GraphBuilder:
    gb = GraphBuilder(settings, mock_llm_client)
    # Mock internal checkers to avoid side effects/heuristic complexity in unit tests
    gb.safety_checker.check = MagicMock(return_value=None)
    gb.silence_checker.check = MagicMock(
        return_value=SilenceDecision(silent=False, reason="test")
    )
    gb.pattern_engine.match = MagicMock(return_value=None)
    return gb


def _base_state(**kwargs) -> AgentState:
    state: AgentState = {
        "user_id": "user-1",
        "session_id": "sess-1",
        "command": "",
        "pwd": "/home/user",
        "exit_code": 0,
        "output": "",
        "messages": [],
        "summary": "",
        "session": SessionState(user_id="user-1"),
        "routing_outcome": None,
        "response": None,
    }
    state.update(kwargs)
    return state


class TestGraphBuilder:
    def test_build_terminal_graph(self, graph_builder: GraphBuilder) -> None:
        graph = graph_builder.build_terminal_graph()
        assert graph.compile() is not None

    def test_build_chat_graph(self, graph_builder: GraphBuilder) -> None:
        graph = graph_builder.build_chat_graph()
        assert graph.compile() is not None

    @pytest.mark.asyncio
    async def test_guardian_blocks_unsafe(self, graph_builder: GraphBuilder) -> None:
        # Arrange
        graph_builder.safety_checker.check.return_value = SafetyBlock(
            tier=SafetyTier.TIER_1_HARD_BLOCK, message="unsafe"
        )

        # Act
        result = await graph_builder.guardian_node(_base_state(command="rm -rf /"))

        # Assert
        assert result["routing_outcome"] == "unsafe"
        assert result["response"] is not None
        assert result["response"].block is True

    @pytest.mark.asyncio
    async def test_guardian_returns_pattern_match(self, graph_builder: GraphBuilder) -> None:
        # Arrange
        match_mock = MagicMock()
        match_mock.confidence = 1.0
        match_mock.definition.response = "Pattern response"
        match_mock.definition.name = "test_pattern"
        graph_builder.pattern_engine.match.return_value = match_mock

        # Act
        # Guardian now checks pattern internally
        state = _base_state(command="help")
        result = await graph_builder.guardian_node(state)

        # Assert
        assert result["routing_outcome"] == "pattern"
        assert result["response"] is not None
        assert result["response"].type == "pattern"
        assert result["response"].content == "Pattern response"


class TestTerminalFlow:
    @pytest.mark.asyncio
    async def test_planner_node_generates_on_error(
        self, graph_builder: GraphBuilder, mock_llm_client: MagicMock
    ) -> None:
        state = _base_state(
            command="ls",
            exit_code=1, # Error code
            output="command not found",
            routing_outcome="continue"
        )
        # Force LLM enable (default is True in fixture, but explicit here)
        graph_builder.settings.enable_llm = True

        result = await graph_builder.planner_node(state, config={})

        assert result["response"] is not None
        assert result["response"].type == "llm"
        assert mock_llm_client.generate.called

    @pytest.mark.asyncio
    async def test_planner_node_silent_on_success(
        self, graph_builder: GraphBuilder, mock_llm_client: MagicMock
    ) -> None:
        state = _base_state(
            command="ls",
            exit_code=0, # Success
            output="file.txt",
            routing_outcome="continue"
        )
        graph_builder.settings.enable_llm = True

        result = await graph_builder.planner_node(state, config={})

        assert result["response"] is not None
        assert result["response"].type == "silent"
        assert not mock_llm_client.generate.called


class TestChatFlow:
    @pytest.mark.asyncio
    async def test_chat_uses_history(
        self, graph_builder: GraphBuilder, mock_llm_client: MagicMock
    ) -> None:
        # Mock count_tokens to avoid needing real API or complex mock
        mock_llm_client.count_tokens_for_messages.return_value = (10, "estimated")

        app = graph_builder.build_chat_graph().compile()
        state = _base_state(
            messages=[
                HumanMessage(content="What is pwd?"),
                AIMessage(content="pwd prints current directory"),
                HumanMessage(content="Give me an example"),
            ]
        )

        result = await app.ainvoke(state)
        # The result of ainvoke includes the final state
        # The chat_node appends messages
        assert "response" in result
        assert result["response"].type == "llm"
        # We start with 3 messages, add 1 response -> 4 messages
        assert len(result["messages"]) == 4
        assert mock_llm_client.generate.called

    @pytest.mark.asyncio
    async def test_chat_compacts_history_when_token_budget_exceeded(
        self, graph_builder: GraphBuilder, mock_llm_client: MagicMock
    ) -> None:
        graph_builder.settings.gemini_context_window_tokens = 1000
        graph_builder.settings.conversation_soft_token_ratio = 0.7
        graph_builder.settings.conversation_hard_token_ratio = 0.85
        graph_builder.settings.conversation_compaction_trigger_min_messages = 2
        graph_builder.settings.conversation_recent_turns_keep = 1
        graph_builder.settings.conversation_min_recent_messages = 2

        # We need to simulate the summarization LLM call too if it happens

        call_count = 0

        async def fake_count_tokens(messages):
            nonlocal call_count
            call_count += 1
            if call_count == 1:
                return 800, "gemini_count_tokens"
            return 600, "gemini_count_tokens"

        mock_llm_client.count_tokens_for_messages = AsyncMock(side_effect=fake_count_tokens)

        app = graph_builder.build_chat_graph().compile()
        state = _base_state(
            messages=[
                HumanMessage(content="u1"),
                AIMessage(content="a1"),
                HumanMessage(content="u2"),
                AIMessage(content="a2"),
                HumanMessage(content="u3"),
            ]
        )

        result = await app.ainvoke(state)

        assert result["response"] is not None
        # compaction_applied is no longer in state, but summary should be updated
        assert result["summary"] != ""

        # Verify compaction
        contents = [getattr(m, "content", "") for m in result["messages"]]
        # We keep 1 turn (2 messages) + current message?
        # keep_messages = max(2, 1*2) = 2.
        # So we keep last 2 messages from history + new one?
        # working_messages has 5 messages. keep 2. compact 3.
        # But u3 is new? No, all are history in ainvoke state input?
        # Actually in chat_node:
        # current_content = messages[-1]
        # history = messages[:-1]
        # so u3 is current. history is u1, a1, u2, a2.
        # keep 2: u2, a2.
        # compact: u1, a1.

        # So expected: u2, a2, u3 (re-added as Human), response (AI)
        # But wait, logic:
        # remove_ops removes compacted messages.

        # Verify that u1 and a1 are NOT in the final messages list or are removed?
        # LangGraph add_messages reducer handles RemoveMessage.
        # So checking contents should show they are gone.

        assert "u1" not in contents
        assert "a1" not in contents
        assert "u2" in contents

    @pytest.mark.asyncio
    async def test_small_history_skips_token_counting(
        self, graph_builder: GraphBuilder, mock_llm_client: MagicMock
    ) -> None:
        graph_builder.settings.conversation_compaction_trigger_min_messages = 10
        mock_llm_client.count_tokens_for_messages = AsyncMock()

        app = graph_builder.build_chat_graph().compile()
        state = _base_state(
            messages=[
                HumanMessage(content="u1"),
                AIMessage(content="a1"),
                HumanMessage(content="u2"),
            ]
        )

        await app.ainvoke(state)
        mock_llm_client.count_tokens_for_messages.assert_not_called()

    @pytest.mark.asyncio
    async def test_guardian_logs_tier_3(self, graph_builder: GraphBuilder) -> None:
        # Arrange
        graph_builder.safety_checker.check.return_value = SafetyBlock(
            tier=SafetyTier.TIER_3_LOG_ONLY, message="log only", log_category="test_cat"
        )
        # Mock logger info to verify logging
        with unittest.mock.patch("app.graph_builder.logger") as mock_logger:
            # Act
            state = _base_state(command="sudo rm -rf /")
            result = await graph_builder.guardian_node(state)

            # Assert
            # Tier 3 is log-only; it should not set routing_outcome to unsafe.
            # But might pass to pattern check.
            # However, if no pattern match, it might default to silence or LLM.
            # We just want to check that it didn't return "unsafe" AND logged.
            assert result.get("routing_outcome") != "unsafe"
            mock_logger.info.assert_called()
            # Verify "tier-3 safety match (log-only)" is in the call args
            assert "tier-3 safety match (log-only)" in str(mock_logger.info.call_args)

    def test_iter_messages_helper(self, graph_builder: GraphBuilder) -> None:
        messages = [
            HumanMessage(content="u1"),
            AIMessage(content="a1"),
            HumanMessage(content=""),  # Empty content should be skipped
            "not a message",  # Invalid type should be skipped
            AIMessage(content="  "),  # Whitespace only should be skipped
        ]
        iterator = graph_builder._iter_messages(messages)
        results = list(iterator)
        assert len(results) == 2
        assert results[0] == ("user", "u1")
        assert results[1] == ("assistant", "a1")
