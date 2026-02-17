"""Pattern engine for high-confidence command hints."""

import re
from dataclasses import dataclass
from typing import Optional


@dataclass(slots=True)
class PatternDefinition:
    name: str
    regex: re.Pattern[str]
    response: str
    priority: int


@dataclass(slots=True)
class PatternMatch:
    definition: PatternDefinition
    confidence: float


class PatternEngine:
    """Simple deterministic regex matcher."""

    def __init__(self) -> None:
        self.patterns: list[PatternDefinition] = []
        self._register_defaults()

    def _register_defaults(self) -> None:
        raw = [
            (
                "man_command",
                r"^man\s+\S+",
                "Manual pages are great for command details. Use `/` to search and `q` to quit.",
                100,
            ),
            (
                "help_flag",
                r"\s+--help\s*$",
                "`--help` is the fastest way to inspect command usage.",
                100,
            ),
            (
                "cd_home",
                r"^cd\s*(~)?\s*$",
                "`cd` with no argument sends you to your home directory.",
                95,
            ),
            ("cd_up", r"^cd\s+\.\.\s*$", "`cd ..` moves to the parent directory.", 95),
            ("pwd", r"^pwd\s*$", "`pwd` prints your current working directory.", 95),
            ("ls_simple", r"^ls\s*$", "Try `ls -la` to include hidden files and metadata.", 90),
            (
                "ls_detailed",
                r"^ls\s+-la?\s*$",
                "`ls -la` shows permission bits, ownership, and timestamps.",
                90,
            ),
            (
                "cat_file",
                r"^cat\s+\S+",
                "`cat` prints a file in full; use `less` for long outputs.",
                85,
            ),
            ("mkdir", r"^mkdir\s+\S+", "Use `mkdir -p` when parent directories may not exist.", 85),
            ("touch", r"^touch\s+\S+", "`touch` creates empty files or updates timestamps.", 85),
            (
                "chmod",
                r"^chmod\s+",
                "Permissions: read=4, write=2, execute=1; combine per user/group/other.",
                80,
            ),
        ]

        for name, regex, response, priority in raw:
            self.patterns.append(
                PatternDefinition(
                    name=name, regex=re.compile(regex), response=response, priority=priority
                )
            )
        self.patterns.sort(key=lambda item: item.priority, reverse=True)

    def match(self, command: str) -> Optional[PatternMatch]:
        """Match a command against registered patterns.

        Args:
            command: The shell command to match.

        Returns:
            PatternMatch with confidence if a pattern matches, None otherwise.
        """
        text = command.strip()
        if not text:
            return None

        for pattern in self.patterns:
            if pattern.regex.search(text):
                literal_size = max(len(pattern.name), 1)
                confidence = min(1.0, 0.7 + min(0.3, literal_size / max(len(text), 1) * 0.3))
                return PatternMatch(definition=pattern, confidence=confidence)
        return None
