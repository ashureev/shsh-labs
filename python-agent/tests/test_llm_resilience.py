"""Tests for LLM resilience components (RateLimiter, CircuitBreaker)."""

import time

import pytest

from app.pipeline.llm import CircuitBreaker, RateLimiter


@pytest.mark.asyncio
class TestRateLimiter:
    async def test_allow_within_limit(self):
        # 10 RPS
        limiter = RateLimiter(rps=10, window_seconds=1)
        for _ in range(10):
            allowed, retry_after = await limiter.allow()
            assert allowed is True
            assert retry_after == 0.0

    async def test_block_exceeding_limit(self):
        # 1 RPS
        limiter = RateLimiter(rps=1, window_seconds=1)

        # First call allowed
        allowed, _ = await limiter.allow()
        assert allowed is True

        # Second call blocked
        allowed, retry_after = await limiter.allow()
        assert allowed is False
        assert retry_after > 0.0

    async def test_reset_after_window(self):
        limiter = RateLimiter(rps=1, window_seconds=0.1)

        await limiter.allow()
        allowed, _ = await limiter.allow()
        assert allowed is False

        time.sleep(0.15)

        allowed, _ = await limiter.allow()
        assert allowed is True


@pytest.mark.asyncio
class TestCircuitBreaker:
    async def test_initially_closed(self):
        cb = CircuitBreaker(threshold=3, timeout_seconds=10)
        assert await cb.can_execute() is True

    async def test_opens_after_failures(self):
        cb = CircuitBreaker(threshold=2, timeout_seconds=10)

        await cb.record_failure()
        # 1st failure
        assert await cb.can_execute() is True

        await cb.record_failure()
        # 2nd failure -> open
        assert await cb.can_execute() is False

    async def test_resets_after_success(self):
        cb = CircuitBreaker(threshold=2, timeout_seconds=10)
        await cb.record_failure()
        await cb.record_success()
        assert cb.failure_count == 0
        assert await cb.can_execute() is True

    async def test_recovers_after_timeout(self):
        cb = CircuitBreaker(threshold=1, timeout_seconds=0.1)

        await cb.record_failure()
        assert await cb.can_execute() is False

        time.sleep(0.15)
        assert await cb.can_execute() is True
        assert cb.is_open is False
