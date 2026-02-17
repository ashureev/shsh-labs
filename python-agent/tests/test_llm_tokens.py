import asyncio
from types import SimpleNamespace
from unittest.mock import AsyncMock, MagicMock

import pytest
from langchain_core.messages import HumanMessage

from app.config import Settings
from app.pipeline.llm import LLMClient


@pytest.mark.asyncio
async def test_count_tokens_uses_estimate_for_non_gemini() -> None:
    settings = Settings(llm_provider="openrouter")
    client = LLMClient(model=MagicMock(), settings=settings)
    tokens, mode = await client.count_tokens_for_messages([HumanMessage(content="hello world")])
    assert tokens > 0
    assert mode == "estimated"


@pytest.mark.asyncio
async def test_count_tokens_falls_back_to_estimate_when_gemini_count_fails() -> None:
    settings = Settings(llm_provider="gemini", google_api_key="test-key")
    client = LLMClient(model=MagicMock(), settings=settings)
    client._gemini_count_tokens_sync = MagicMock(side_effect=RuntimeError("down"))
    tokens, mode = await client.count_tokens_for_messages([HumanMessage(content="hello world")])
    assert tokens > 0
    assert mode == "estimated"


@pytest.mark.asyncio
async def test_count_tokens_falls_back_to_estimate_when_gemini_count_times_out() -> None:
    settings = Settings(
        llm_provider="gemini",
        google_api_key="test-key",
        gemini_count_tokens_timeout_seconds=1,
    )
    client = LLMClient(model=MagicMock(), settings=settings)

    async def slow_count(messages):  # noqa: ARG001
        await asyncio.sleep(2)
        return 999

    client._count_tokens_gemini = slow_count
    tokens, mode = await client.count_tokens_for_messages([HumanMessage(content="hello world")])
    assert tokens > 0
    assert mode == "estimated"


@pytest.mark.asyncio
async def test_generate_accepts_callback_manager_in_run_config() -> None:
    settings = Settings(llm_provider="openrouter")

    model = MagicMock()
    model.ainvoke = AsyncMock(return_value=SimpleNamespace(content="ok", usage_metadata=None))

    client = LLMClient(model=model, settings=settings)

    class FakeCallbackManager:
        def __init__(self) -> None:
            self.handlers = []

        def add_handler(self, callback, inherit=True) -> None:  # noqa: ARG002
            self.handlers.append(callback)

    result = await client.generate(
        system_prompt="sys",
        user_prompt="hello",
        user_id="u1",
        session_id="s1",
        config={"callbacks": FakeCallbackManager()},
    )

    assert result.error is None
    assert result.response == "ok"
