"""Layering guard: the import-linter contracts of pyproject.toml hold."""

from __future__ import annotations

import logging
import sys
from collections.abc import Iterator
from pathlib import Path

import pytest
from importlinter.cli import lint_imports

PYPROJECT = Path(__file__).resolve().parents[2] / "pyproject.toml"


@pytest.fixture(autouse=True)
def _keep_logging_config() -> Iterator[None]:
    """lint_imports calls logging.config.dictConfig, which disables every
    existing logger and replaces the root handlers; undo that for other tests."""
    root = logging.getLogger()
    handlers, level = list(root.handlers), root.level
    loggers = [
        lg
        for lg in logging.Logger.manager.loggerDict.values()
        if isinstance(lg, logging.Logger)
    ]
    state = [
        (lg, lg.disabled, lg.level, list(lg.handlers), lg.propagate) for lg in loggers
    ]
    yield
    root.handlers[:] = handlers
    root.setLevel(level)
    for lg, disabled, lvl, hs, prop in state:
        lg.disabled, lg.propagate = disabled, prop
        lg.setLevel(lvl)
        lg.handlers[:] = hs


def test_import_contracts_hold() -> None:
    assert lint_imports(config_filename=str(PYPROJECT), no_cache=True) == 0


def test_a_violation_is_reported(tmp_path: Path) -> None:
    pkg = tmp_path / "tinyroot"
    (pkg / "low").mkdir(parents=True)
    (pkg / "high").mkdir()
    for init in (pkg / "__init__.py", pkg / "low" / "__init__.py"):
        init.write_text("", encoding="utf-8")
    (pkg / "high" / "__init__.py").write_text("X = 1\n", encoding="utf-8")
    (pkg / "low" / "mod.py").write_text("from tinyroot import high\n", encoding="utf-8")
    config = tmp_path / "linter.toml"
    config.write_text(
        '[tool.importlinter]\nroot_package = "tinyroot"\n\n'
        "[[tool.importlinter.contracts]]\n"
        'name = "low must not import high"\ntype = "forbidden"\n'
        'source_modules = ["tinyroot.low"]\nforbidden_modules = ["tinyroot.high"]\n',
        encoding="utf-8",
    )
    sys.path.insert(0, str(tmp_path))
    try:
        assert lint_imports(config_filename=str(config), no_cache=True) != 0
    finally:
        sys.path.remove(str(tmp_path))


def test_lint_writes_nothing_to_cwd(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.chdir(tmp_path)
    lint_imports(config_filename=str(PYPROJECT), no_cache=True)
    assert list(tmp_path.iterdir()) == []
