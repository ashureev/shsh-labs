"""Tiered command safety policies."""

import re
from dataclasses import dataclass
from enum import IntEnum
from typing import Optional


class SafetyTier(IntEnum):
    TIER_1_HARD_BLOCK = 1
    TIER_2_CONFIRM_INTENT = 2
    TIER_3_LOG_ONLY = 3


@dataclass(slots=True)
class SafetyBlock:
    tier: SafetyTier
    message: str
    log_category: str = ""


@dataclass(slots=True)
class SafetyRule:
    pattern: re.Pattern[str]
    tier: SafetyTier
    message: str
    log_category: str = ""


class SafetyChecker:
    """Evaluates shell commands against tiered rules."""

    def __init__(self) -> None:
        self.rules: list[SafetyRule] = []
        self._init_default_rules()

    def _init_default_rules(self) -> None:
        tier1 = [
            (r"^rm\s+-[rf]+\s+/\s*$", "This would delete the entire filesystem."),
            (r"^rm\s+-[rf]+\s+/\s+", "Recursive delete from root is blocked."),
            (r":\(\)\s*\{\s*:\|:\s*&\s*\}\s*;?\s*:", "Fork bomb detected."),
            (r"^mkfs\.", "Filesystem format command is blocked."),
            (r"^dd\s+.*\bof=/dev/(sd[a-z]|nvme\d+n\d+)", "Direct disk writes are blocked."),
            (r">\s*/dev/(sd[a-z]|nvme\d+n\d+)", "Redirecting output to block devices is blocked."),
        ]
        tier2 = [
            (r"^dd\s+", "`dd` can destroy data. Confirm intent."),
            (r"^chmod\s+-R\s+777", "Recursive world-writable permissions are risky. Confirm."),
            (r"^chown\s+-R", "Recursive ownership change. Confirm intent."),
            (r"^rm\s+-[rf]+", "Recursive/force delete detected. Confirm intent."),
            (r"^kill\s+-9", "Force-killing processes can be disruptive. Confirm."),
        ]
        tier3 = [
            (r"^sudo\s+", "privilege_escalation"),
            (r"^(apt|apt-get|yum|dnf)\s+install", "package_install"),
            (r"^systemctl\s+", "service_management"),
            (r"^(ufw|iptables)\s+", "firewall_config"),
        ]

        for pattern, message in tier1:
            self.rules.append(
                SafetyRule(re.compile(pattern), SafetyTier.TIER_1_HARD_BLOCK, message)
            )
        for pattern, message in tier2:
            self.rules.append(
                SafetyRule(re.compile(pattern), SafetyTier.TIER_2_CONFIRM_INTENT, message)
            )
        for pattern, category in tier3:
            self.rules.append(
                SafetyRule(re.compile(pattern), SafetyTier.TIER_3_LOG_ONLY, "", category)
            )

    def check(self, command: str) -> Optional[SafetyBlock]:
        """Check if a command matches any safety rules.

        Args:
            command: The shell command to check.

        Returns:
            SafetyBlock if the command violates a rule, None if safe.
        """
        for rule in self.rules:
            if rule.pattern.search(command):
                return SafetyBlock(rule.tier, rule.message, rule.log_category)
        return None
