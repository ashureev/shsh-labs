"""Tests for PatternEngine."""

import pytest

from app.pipeline.patterns import PatternEngine


@pytest.fixture
def engine():
    return PatternEngine()


class TestPatternEngine:
    def test_match_man_command(self, engine):
        match = engine.match("man ls")
        assert match is not None
        assert match.definition.name == "man_command"
        assert match.confidence >= 0.7

    def test_match_help_flag(self, engine):
        match = engine.match("ls --help")
        assert match is not None
        assert match.definition.name == "help_flag"

    def test_match_cd_home(self, engine):
        assert engine.match("cd").definition.name == "cd_home"
        assert engine.match("cd ~").definition.name == "cd_home"
        assert engine.match("cd   ").definition.name == "cd_home"

    def test_match_cd_up(self, engine):
        assert engine.match("cd ..").definition.name == "cd_up"
        assert engine.match("cd .. ").definition.name == "cd_up"

    def test_match_pwd(self, engine):
        assert engine.match("pwd").definition.name == "pwd"
        assert engine.match("pwd ").definition.name == "pwd"

    def test_match_ls_simple(self, engine):
        assert engine.match("ls").definition.name == "ls_simple"
        assert engine.match("ls  ").definition.name == "ls_simple"

    def test_match_ls_detailed(self, engine):
        assert engine.match("ls -la").definition.name == "ls_detailed"
        assert engine.match("ls -l").definition.name == "ls_detailed"

    def test_match_cat_file(self, engine):
        assert engine.match("cat file.txt").definition.name == "cat_file"

    def test_match_mkdir(self, engine):
        assert engine.match("mkdir newdir").definition.name == "mkdir"

    def test_match_touch(self, engine):
        assert engine.match("touch file").definition.name == "touch"

    def test_match_chmod(self, engine):
        assert engine.match("chmod 777 file").definition.name == "chmod"

    def test_no_match(self, engine):
        assert engine.match("echo hello") is None
        assert engine.match("random command") is None
        assert engine.match("") is None

    def test_match_confidence_calculation(self, engine):
        # Long command vs short pattern name
        match = engine.match("man a_very_long_command_name_that_makes_ratio_small")
        assert match.confidence <= 1.0
        assert match.confidence >= 0.7
