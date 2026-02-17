"""Configuration for the Python agent service."""

from functools import lru_cache
from typing import Literal

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings loaded from environment."""

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    service_name: str = Field(default="shsh-python-agent")
    service_version: str = Field(default="0.2.0")
    grpc_port: int = Field(default=50051)
    log_level: str = Field(default="INFO")

    llm_provider: Literal["google", "gemini", "anthropic", "openrouter"] = Field(default="google")
    llm_model: str = Field(default="gemini-2.0-flash-exp")
    google_api_key: str = Field(default="")
    anthropic_api_key: str = Field(default="")
    openrouter_api_key: str = Field(default="")

    redis_url: str = Field(default="localhost:6379")
    redis_password: str = Field(default="")
    redis_db: int = Field(default=0)
    session_ttl_hours: int = Field(default=24)

    enable_safety: bool = Field(default=True)
    enable_patterns: bool = Field(default=True)
    enable_silence: bool = Field(default=True)
    enable_llm: bool = Field(default=True)

    pattern_confidence_threshold: float = Field(default=0.7)
    proactive_cooldown_seconds: int = Field(default=120)
    max_output_size_kb: int = Field(default=50)
    head_tail_lines: int = Field(default=20)
    conversation_compaction_enabled: bool = Field(default=True)
    conversation_soft_token_ratio: float = Field(default=0.70)
    conversation_hard_token_ratio: float = Field(default=0.85)
    conversation_recent_turns_keep: int = Field(default=3)
    conversation_min_recent_messages: int = Field(default=2)
    conversation_summary_max_chars: int = Field(default=2000)
    conversation_compaction_trigger_min_messages: int = Field(default=6)
    gemini_context_window_tokens: int = Field(default=1_000_000)
    gemini_count_tokens_timeout_seconds: int = Field(default=3)

    allow_memory_checkpointer: bool = Field(default=True)

    rate_limit_rps: int = Field(default=1)
    rate_limit_window_seconds: int = Field(default=1)
    circuit_breaker_threshold: int = Field(default=5)
    circuit_breaker_timeout_seconds: int = Field(default=30)

    @property
    def session_ttl_seconds(self) -> int:
        return self.session_ttl_hours * 3600


@lru_cache
def get_settings() -> Settings:
    return Settings()
