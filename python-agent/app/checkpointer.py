"""LangGraph checkpointer factory."""

from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import Any, Optional

from langgraph.checkpoint.memory import MemorySaver

from app.config import Settings

logger = logging.getLogger(__name__)


@dataclass(slots=True)
class CheckpointerHandle:
    """Container for checkpointer and optional async cleanup owner."""

    saver: Any
    owner: Optional[Any] = None


async def create_checkpointer(settings: Settings) -> CheckpointerHandle:
    """Create async-compatible checkpointer for LangGraph."""
    try:
        from langgraph.checkpoint.redis import AsyncRedisSaver

        redis_url = f"redis://{settings.redis_url}"
        saver = AsyncRedisSaver(redis_url=redis_url)
        await saver.__aenter__()
        await saver.asetup()
        logger.info("Using Redis checkpointer", extra={"redis_url": settings.redis_url})
        return CheckpointerHandle(saver=saver, owner=saver)
    except Exception as exc:  # noqa: BLE001
        if settings.allow_memory_checkpointer:
            logger.warning("Redis checkpointer unavailable, falling back to MemorySaver: %s", exc)
            return CheckpointerHandle(saver=MemorySaver(), owner=None)
        raise RuntimeError(f"redis checkpointer initialization failed: {exc}") from exc


async def close_checkpointer(handle: Optional[CheckpointerHandle]) -> None:
    """Close async checkpointer resources when applicable."""
    if handle is None or handle.owner is None:
        return
    await handle.owner.__aexit__(None, None, None)
