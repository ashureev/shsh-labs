"""Tests for SafetyChecker."""

import pytest

from app.pipeline.safety import SafetyChecker, SafetyTier


@pytest.fixture
def checker():
    return SafetyChecker()


class TestSafetyChecker:
    def test_tier_1_hard_block_rm_rf_root(self, checker):
        block = checker.check("rm -rf /")
        assert block is not None
        assert block.tier == SafetyTier.TIER_1_HARD_BLOCK
        assert "delete the entire filesystem" in block.message

    def test_tier_1_hard_block_fork_bomb(self, checker):
        block = checker.check(":(){ :|:& };:")
        assert block is not None
        assert block.tier == SafetyTier.TIER_1_HARD_BLOCK
        assert "Fork bomb" in block.message

    def test_tier_1_hard_block_mkfs(self, checker):
        block = checker.check("mkfs.ext4 /dev/sda1")
        assert block is not None
        assert block.tier == SafetyTier.TIER_1_HARD_BLOCK
        assert "Filesystem format" in block.message

    def test_tier_1_hard_block_dd_to_disk(self, checker):
        block = checker.check("dd if=/dev/zero of=/dev/sda")
        assert block is not None
        assert block.tier == SafetyTier.TIER_1_HARD_BLOCK
        assert "Direct disk writes" in block.message

    def test_tier_2_confirm_dd_generic(self, checker):
        block = checker.check("dd if=file of=file2")
        assert block is not None
        assert block.tier == SafetyTier.TIER_2_CONFIRM_INTENT
        assert "dd" in block.message

    def test_tier_2_confirm_chmod_recursive_world_writable(self, checker):
        block = checker.check("chmod -R 777 .")
        assert block is not None
        assert block.tier == SafetyTier.TIER_2_CONFIRM_INTENT
        assert "Recursive world-writable" in block.message

    def test_tier_2_confirm_chown_recursive(self, checker):
        block = checker.check("chown -R user:group .")
        assert block is not None
        assert block.tier == SafetyTier.TIER_2_CONFIRM_INTENT
        assert "Recursive ownership" in block.message

    def test_tier_2_confirm_rm_recursive(self, checker):
        block = checker.check("rm -rf folder")
        assert block is not None
        assert block.tier == SafetyTier.TIER_2_CONFIRM_INTENT
        assert "Recursive/force delete" in block.message

    def test_tier_2_confirm_kill_9(self, checker):
        block = checker.check("kill -9 1234")
        assert block is not None
        assert block.tier == SafetyTier.TIER_2_CONFIRM_INTENT
        assert "Force-killing" in block.message

    def test_tier_3_log_sudo(self, checker):
        block = checker.check("sudo ls")
        assert block is not None
        assert block.tier == SafetyTier.TIER_3_LOG_ONLY
        assert block.log_category == "privilege_escalation"

    def test_tier_3_log_package_install(self, checker):
        block = checker.check("apt install git")
        assert block is not None
        assert block.tier == SafetyTier.TIER_3_LOG_ONLY
        assert block.log_category == "package_install"

    def test_safe_command(self, checker):
        assert checker.check("ls -la") is None
        assert checker.check("echo hello") is None
        assert checker.check("cd /tmp") is None
