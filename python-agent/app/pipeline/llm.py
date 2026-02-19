"""LLM client with rate limiting and circuit breaker."""

import asyncio
import logging
import time
from dataclasses import dataclass
from typing import Any, Optional

from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, HumanMessage, SystemMessage
from langchain_core.runnables import RunnableConfig

from app.config import Settings

logger = logging.getLogger(__name__)

# Token estimation constants
TOKEN_ESTIMATION_CHARS_PER_TOKEN = 4  # Approximate characters per token
TOKEN_ESTIMATION_BASE_OVERHEAD = 128  # Base token overhead for estimation
TOKEN_ESTIMATION_MIN_TOKENS = 1  # Minimum token count to return


@dataclass(slots=True)
class LLMResult:
    response: str
    duration_ms: int
    error: Optional[Exception] = None
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0
    cached_tokens: int = 0
    preflight_tokens: int = 0
    compaction_applied: bool = False


class CircuitBreaker:
    def __init__(self, threshold: int, timeout_seconds: int) -> None:
        self.threshold = threshold
        self.timeout_seconds = timeout_seconds
        self.failure_count = 0
        self.last_failure_time: Optional[float] = None
        self.is_open = False
        self._lock = asyncio.Lock()

    async def can_execute(self) -> bool:
        async with self._lock:
            if not self.is_open:
                return True
            if (time.time() - self.last_failure_time) >= self.timeout_seconds:
                self.is_open = False
                self.failure_count = 0
                return True
            return False

    async def record_success(self) -> None:
        async with self._lock:
            self.failure_count = 0
            self.is_open = False
            self.last_failure_time = None

    async def record_failure(self) -> None:
        async with self._lock:
            self.failure_count += 1
            self.last_failure_time = time.time()
            if self.failure_count >= self.threshold:
                self.is_open = True


class RateLimiter:
    def __init__(self, rps: int, window_seconds: int) -> None:
        self.rps = rps
        self.window_seconds = window_seconds
        self.requests: list[float] = []
        self._lock = asyncio.Lock()

    async def allow(self) -> tuple[bool, float]:
        async with self._lock:
            now = time.time()
            window_start = now - self.window_seconds
            self.requests = [ts for ts in self.requests if ts > window_start]

            if len(self.requests) < self.rps:
                self.requests.append(now)
                return True, 0.0

            retry_after = self.requests[0] + self.window_seconds - now
            return False, max(retry_after, 0.0)


class LLMClient:
    def __init__(self, model: Optional[BaseChatModel], settings: Settings) -> None:
        self.model = model
        self.settings = settings
        self._gemini_client: Optional[Any] = None
        self._gemini_init_lock = asyncio.Lock()
        self.circuit_breaker = CircuitBreaker(
            threshold=settings.circuit_breaker_threshold,
            timeout_seconds=settings.circuit_breaker_timeout_seconds,
        )
        self.rate_limiter = RateLimiter(
            rps=settings.rate_limit_rps,
            window_seconds=settings.rate_limit_window_seconds,
        )

    def _blocked_result(self, start: float, error: Exception) -> LLMResult:
        return LLMResult("", int((time.time() - start) * 1000), error)

    async def generate(
        self,
        system_prompt: str,
        user_prompt: str,
        conversation_history: Optional[list[dict[str, str]]] = None,
        user_id: Optional[str] = None,
        session_id: Optional[str] = None,
        metadata: Optional[dict[str, Any]] = None,
        preflight_tokens: int = 0,
        compaction_applied: bool = False,
        enforce_rate_limit: bool = True,
        config: Optional[RunnableConfig] = None,
    ) -> LLMResult:
        start = time.time()

        if enforce_rate_limit:
            allowed, retry_after = await self.rate_limiter.allow()
            if not allowed:
                logger.warning(
                    "LLM request rate limited",
                    extra={
                        "user_id": user_id,
                        "session_id": session_id,
                        "retry_after": round(retry_after, 3),
                        "node": (metadata or {}).get("node", ""),
                    },
                )
                return self._blocked_result(start, Exception(f"rate limited: {retry_after:.2f}s"))

        if self.model is None:
            logger.error(
                "LLM request blocked: model not configured",
                extra={
                    "user_id": user_id,
                    "session_id": session_id,
                    "node": (metadata or {}).get("node", ""),
                },
            )
            return self._blocked_result(start, Exception("model not configured"))

        if not await self.circuit_breaker.can_execute():
            logger.warning(
                "LLM request blocked: circuit breaker open",
                extra={
                    "user_id": user_id,
                    "session_id": session_id,
                    "node": (metadata or {}).get("node", ""),
                },
            )
            return self._blocked_result(start, Exception("circuit breaker open"))

        messages = self.build_messages(
            system_prompt=system_prompt,
            user_prompt=user_prompt,
            conversation_history=conversation_history,
        )

        run_config = (config or {}).copy()

        # Merge metadata
        if not isinstance(run_config.get("metadata"), dict):
            run_config["metadata"] = {}
        run_config["metadata"].update(
            {
                "user_id": user_id,
                "session_id": session_id,
                **(metadata or {}),
            }
        )

        try:
            response = await self.model.ainvoke(messages, config=run_config)
            text = str(response.content)
            usage = self._extract_usage_metadata(response)
            prompt_tokens, completion_tokens, total_tokens, cached_tokens = usage
            await self.circuit_breaker.record_success()
            logger.info(
                "LLM request completed",
                extra={
                    "user_id": user_id,
                    "session_id": session_id,
                    "node": (metadata or {}).get("node", ""),
                    "preflight_tokens": preflight_tokens,
                    "prompt_tokens": prompt_tokens,
                    "completion_tokens": completion_tokens,
                    "total_tokens": total_tokens,
                    "cached_tokens": cached_tokens,
                    "compaction_applied": compaction_applied,
                },
            )
            return LLMResult(
                response=text,
                duration_ms=int((time.time() - start) * 1000),
                prompt_tokens=prompt_tokens,
                completion_tokens=completion_tokens,
                total_tokens=total_tokens,
                cached_tokens=cached_tokens,
                preflight_tokens=preflight_tokens,
                compaction_applied=compaction_applied,
            )
        except Exception as exc:  # noqa: BLE001
            await self.circuit_breaker.record_failure()
            logger.exception(
                "LLM request failed",
                extra={
                    "user_id": user_id,
                    "session_id": session_id,
                    "node": (metadata or {}).get("node", ""),
                    "preflight_tokens": preflight_tokens,
                    "compaction_applied": compaction_applied,
                },
            )
            return self._blocked_result(start, exc)

    def build_messages(
        self,
        system_prompt: str,
        user_prompt: str,
        conversation_history: Optional[list[dict[str, str]]] = None,
    ) -> list[Any]:
        messages: list[Any] = [SystemMessage(content=system_prompt)]
        for item in conversation_history or []:
            role = item.get("role", "")
            content = item.get("content", "")
            if role == "assistant":
                messages.append(AIMessage(content=content))
            else:
                messages.append(HumanMessage(content=content))
        messages.append(HumanMessage(content=user_prompt))
        return messages

    async def count_tokens_for_messages(self, messages: list[Any]) -> tuple[int, str]:
        is_gemini = self.settings.llm_provider in ("google", "gemini")
        if not is_gemini or not self.settings.google_api_key:
            estimated = self.estimate_tokens(messages)
            return estimated, "estimated"

        try:
            count = await asyncio.wait_for(
                self._count_tokens_gemini(messages),
                timeout=self.settings.gemini_count_tokens_timeout_seconds,
            )
            return count, "gemini_count_tokens"
        except asyncio.TimeoutError:
            logger.warning(
                "Gemini count_tokens timed out; falling back to estimate",
                extra={"timeout_seconds": self.settings.gemini_count_tokens_timeout_seconds},
            )
            estimated = self.estimate_tokens(messages)
            return estimated, "estimated"
        except Exception:  # noqa: BLE001
            logger.warning("Gemini count_tokens failed; falling back to estimate")
            estimated = self.estimate_tokens(messages)
            return estimated, "estimated"

    def estimate_tokens(self, messages: list[Any]) -> int:
        """Estimate token count from message content.

        Uses a simple heuristic: characters divided by ratio plus overhead.

        Args:
            messages: List of messages with content attribute.

        Returns:
            Estimated token count (minimum 1).
        """
        total_chars = 0
        for message in messages:
            content = getattr(message, "content", "")
            total_chars += len(str(content))
        return max(
            TOKEN_ESTIMATION_MIN_TOKENS,
            (total_chars // TOKEN_ESTIMATION_CHARS_PER_TOKEN) + TOKEN_ESTIMATION_BASE_OVERHEAD,
        )

    async def _count_tokens_gemini(self, messages: list[Any]) -> int:
        """Thread-safe Gemini token counting with lazy initialization."""
        from google import genai

        async with self._gemini_init_lock:
            if self._gemini_client is None:
                self._gemini_client = genai.Client(api_key=self.settings.google_api_key)

        # Run the actual API call in thread pool
        return await asyncio.to_thread(self._gemini_count_tokens_sync, messages)

    def _gemini_count_tokens_sync(self, messages: list[Any]) -> int:

        contents = []
        for message in messages:
            message_type = getattr(message, "type", "")
            content = str(getattr(message, "content", ""))
            role = "model" if message_type == "ai" else "user"
            contents.append(
                {
                    "role": role,
                    "parts": [{"text": content}],
                }
            )

        response = self._gemini_client.models.count_tokens(
            model=self.settings.llm_model,
            contents=contents,
        )
        total_tokens = getattr(response, "total_tokens", 0) or getattr(response, "totalTokens", 0)
        return int(total_tokens or 0)

    @staticmethod
    def _get_token_count(metadata: Any, *keys: str) -> int:
        for key in keys:
            val = getattr(metadata, key, None)
            if val is not None and val != 0:
                return int(val)
        return 0

    def _extract_usage_metadata(self, response: Any) -> tuple[int, int, int, int]:
        metadata = getattr(response, "usage_metadata", None)
        if metadata is None:
            return 0, 0, 0, 0

        return (
            self._get_token_count(metadata, "input_tokens", "prompt_token_count"),
            self._get_token_count(metadata, "output_tokens", "candidates_token_count"),
            self._get_token_count(metadata, "total_tokens", "total_token_count"),
            self._get_token_count(metadata, "cached_tokens", "cached_content_token_count"),
        )

    def build_system_prompt(self) -> str:
        # Keep the previous concise unified system style for both chat and terminal.
        return (
            "Concise Linux assistant. Be brief.\n\n"
            "- Greetings: 1 sentence\n"
            "- Errors: 1-2 sentences explaining issue + fix\n"
            "- Questions: 1-2 sentences + example\n"
            "- Max 3 sentences total\n"
            "- Use backticks for commands\n"
            "- Reference history when relevant\n\n"
            "Keep it SHORT."
        )

    def build_terminal_prompt(self, command: str, pwd: str, exit_code: int, output: str) -> str:
        return (
            f"Current directory: {pwd}\n"
            f"Command: `{command}`\n"
            f"Exit code: {exit_code}\n"
            f"Output:\n```\n{output}\n```\n"
            "Explain what happened and the next best command."
        )


def create_llm_client(settings: Settings) -> LLMClient:
    model: Optional[BaseChatModel] = None

    if not settings.enable_llm:
        return LLMClient(model=None, settings=settings)

    try:
        if settings.llm_provider in ("google", "gemini") and settings.google_api_key:
            from langchain_google_genai import ChatGoogleGenerativeAI

            model = ChatGoogleGenerativeAI(
                model=settings.llm_model,
                google_api_key=settings.google_api_key,
                temperature=0.2,
            )
        elif settings.llm_provider == "anthropic" and settings.anthropic_api_key:
            from langchain_anthropic import ChatAnthropic

            model = ChatAnthropic(
                model=settings.llm_model,
                anthropic_api_key=settings.anthropic_api_key,
                temperature=0.2,
            )
        elif settings.llm_provider == "openrouter" and settings.openrouter_api_key:
            from langchain_openai import ChatOpenAI

            model = ChatOpenAI(
                model=settings.llm_model,
                api_key=settings.openrouter_api_key,
                base_url="https://openrouter.ai/api/v1",
                temperature=0.2,
            )
    except Exception:  # noqa: BLE001
        logger.exception("Failed to initialize LLM provider: %s", settings.llm_provider)
        model = None

    return LLMClient(model=model, settings=settings)
