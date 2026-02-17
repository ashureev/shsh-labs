"""Tests for SilenceChecker."""

from datetime import datetime, timedelta, timezone

import pytest

from app.pipeline.silence import SessionState, SilenceChecker, SilenceReason, TerminalInput


@pytest.fixture
def checker():
    return SilenceChecker(cooldown_seconds=60)


@pytest.fixture
def session():
    return SessionState(user_id="u1")


@pytest.fixture
def terminal_input():
    return TerminalInput(
        command="ls",
        pwd="/home/user",
        exit_code=0,
        output="file.txt",
        timestamp=datetime.now(timezone.utc),
        user_id="u1",
    )


class TestSilenceChecker:
    def test_silence_in_editor_mode(self, checker, session, terminal_input):
        session.in_editor_mode = True
        decision = checker.check(session, terminal_input)
        assert decision.silent is True
        assert decision.reason == SilenceReason.IN_EDITOR_MODE.value

    def test_silence_just_self_corrected(self, checker, session, terminal_input):
        session.just_self_corrected = True
        decision = checker.check(session, terminal_input)
        assert decision.silent is True
        assert decision.reason == SilenceReason.SELF_CORRECTED.value

    def test_silence_safe_exploration(self, checker, session, terminal_input):
        # ls is a safe command
        terminal_input.command = "ls -la"
        terminal_input.exit_code = 0
        decision = checker.check(session, terminal_input)
        assert decision.silent is True
        assert decision.reason == SilenceReason.SAFE_EXPLORATION.value

    def test_speak_safe_exploration_with_error(self, checker, session, terminal_input):
        # ls with error should not be silent
        terminal_input.command = "ls -la"
        terminal_input.exit_code = 1
        decision = checker.check(session, terminal_input)
        assert decision.silent is False
        assert decision.reason == SilenceReason.MAY_SPEAK.value

    def test_silence_cooldown_active(self, checker, session, terminal_input):
        terminal_input.command = "unknown_command"  # Not a safe command
        terminal_input.exit_code = 0  # Success? weird but let's assume

        session.last_proactive_msg = datetime.now(timezone.utc) - timedelta(seconds=10)
        # Cooldown is 60s
        decision = checker.check(session, terminal_input)
        assert decision.silent is True
        assert decision.reason == SilenceReason.COOLDOWN.value

    def test_speak_cooldown_expired(self, checker, session, terminal_input):
        terminal_input.command = "unknown_command"
        terminal_input.exit_code = 0

        session.last_proactive_msg = datetime.now(timezone.utc) - timedelta(seconds=61)
        decision = checker.check(session, terminal_input)
        assert decision.silent is False
        assert decision.reason == SilenceReason.MAY_SPEAK.value

    def test_speak_error_bypasses_cooldown(self, checker, session, terminal_input):
        terminal_input.command = "bad_command"
        terminal_input.exit_code = 127

        session.last_proactive_msg = datetime.now(timezone.utc) - timedelta(seconds=10)
        decision = checker.check(session, terminal_input)
        assert decision.silent is False
        assert decision.reason == SilenceReason.MAY_SPEAK.value

    def test_silence_user_typing(self, checker, session, terminal_input):
        terminal_input.command = "unknown"
        terminal_input.exit_code = 0
        # Ensure cooldown doesn't trigger
        session.last_proactive_msg = None

        session.is_typing = True
        decision = checker.check(session, terminal_input)
        assert decision.silent is True
        assert decision.reason == SilenceReason.USER_TYPING.value

    def test_speak_default(self, checker, session, terminal_input):
        terminal_input.command = "unknown"
        terminal_input.exit_code = 0
        session.last_proactive_msg = None
        session.is_typing = False
        session.in_editor_mode = False
        session.just_self_corrected = False

        decision = checker.check(session, terminal_input)
        assert decision.silent is False
        assert decision.reason == SilenceReason.MAY_SPEAK.value

    def test_record_proactive_message(self, checker, session):
        assert session.last_proactive_msg is None
        checker.record_proactive_message(session)
        assert session.last_proactive_msg is not None

    def test_reset_self_corrected(self, checker, session):
        session.just_self_corrected = True
        checker.reset_self_corrected(session)
        assert session.just_self_corrected is False
