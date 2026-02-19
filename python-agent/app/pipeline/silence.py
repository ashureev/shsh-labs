"""Silence policy enforcement."""

from dataclasses import dataclass
from datetime import datetime, timezone
from enum import Enum
from typing import Optional


class SilenceReason(str, Enum):
    SELF_CORRECTED = "self_corrected"
    SAFE_EXPLORATION = "safe_exploration"
    COOLDOWN = "cooldown"
    USER_TYPING = "user_typing"
    IN_EDITOR_MODE = "in_editor_mode"
    MAY_SPEAK = "may_speak"


@dataclass(slots=True)
class SessionState:
    user_id: str
    in_editor_mode: bool = False
    just_self_corrected: bool = False
    is_typing: bool = False
    last_proactive_msg: Optional[datetime] = None


@dataclass(slots=True)
class TerminalInput:
    command: str
    pwd: str
    exit_code: int
    output: str


@dataclass(slots=True)
class SilenceDecision:
    silent: bool
    reason: str


class SilenceChecker:
    """Apply PRD silence rules in deterministic order."""

    SAFE_COMMANDS = {
        "ls",
        "cd",
        "pwd",
        "cat",
        "less",
        "head",
        "tail",
        "echo",
        "man",
        "clear",
        "exit",
    }

    def __init__(self, cooldown_seconds: int = 120) -> None:
        self.cooldown_seconds = cooldown_seconds

    def check(self, session: Optional[SessionState], input_data: TerminalInput) -> SilenceDecision:
        """Check if the AI should remain silent based on session state and input.

        Args:
            session: Current session state, or None if no session exists.
            input_data: Terminal input data including command and exit code.

        Returns:
            SilenceDecision indicating whether to be silent and why.
        """
        has_error = input_data.exit_code != 0

        if session and session.in_editor_mode:
            return SilenceDecision(True, SilenceReason.IN_EDITOR_MODE.value)

        if session and session.just_self_corrected:
            return SilenceDecision(
                True, SilenceReason.SELF_CORRECTED.value
            )

        parts = input_data.command.strip().split()
        # Safe exploration silence only applies to successful commands.
        if not has_error and parts and parts[0] in self.SAFE_COMMANDS:
            return SilenceDecision(
                True, SilenceReason.SAFE_EXPLORATION.value
            )

        # Cooldown should not block error help.
        if not has_error and session and session.last_proactive_msg is not None:
            elapsed = (datetime.now(timezone.utc) - session.last_proactive_msg).total_seconds()
            if elapsed < self.cooldown_seconds:
                return SilenceDecision(
                    True, SilenceReason.COOLDOWN.value
                )

        if session and session.is_typing:
            return SilenceDecision(
                True, SilenceReason.USER_TYPING.value
            )

        return SilenceDecision(False, SilenceReason.MAY_SPEAK.value)

    def record_proactive_message(self, session: Optional[SessionState]) -> None:
        if session:
            session.last_proactive_msg = datetime.now(timezone.utc)

    def reset_self_corrected(self, session: Optional[SessionState]) -> None:
        if session:
            session.just_self_corrected = False
