"""Layering guard: the import-linter contracts of pyproject.toml hold."""

from __future__ import annotations

import sys
from pathlib import Path

from importlinter.cli import lint_imports

PYPROJECT = Path(__file__).resolve().parents[2] / "pyproject.toml"


def test_import_contracts_hold() -> None:
    assert lint_imports(config_filename=str(PYPROJECT)) == 0


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
        assert lint_imports(config_filename=str(config)) != 0
    finally:
        sys.path.remove(str(tmp_path))
