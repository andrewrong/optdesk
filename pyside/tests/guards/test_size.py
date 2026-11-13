"""Size guard: no source file passes its line limit (size.toml)."""

from __future__ import annotations

import tomllib
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SRC = ROOT / "src" / "optdesk"
GENERATED = SRC / "contracts" / "genpb"


def _config() -> tuple[int, dict[str, int]]:
    data = tomllib.loads((ROOT / "size.toml").read_text(encoding="utf-8"))
    return int(data["limit"]), {k: int(v) for k, v in data.get("ceilings", {}).items()}


def _line_count(path: Path) -> int:
    return len(path.read_text(encoding="utf-8").splitlines())


def _source_files() -> list[Path]:
    return sorted(p for p in SRC.rglob("*.py") if GENERATED not in p.parents)


def test_no_source_file_passes_its_limit() -> None:
    limit, ceilings = _config()
    over = []
    for path in _source_files():
        rel = path.relative_to(ROOT).as_posix()
        allowed = ceilings.get(rel, limit)
        lines = _line_count(path)
        if lines > allowed:
            over.append(f"{rel}: {lines} lines, allowed {allowed}")
    assert not over, "files over their limit:\n" + "\n".join(over)


def test_every_ceiling_names_a_file_over_the_limit() -> None:
    limit, ceilings = _config()
    stale = []
    for rel in ceilings:
        path = ROOT / rel
        if not path.is_file():
            stale.append(f"{rel}: no such file")
        elif _line_count(path) <= limit:
            stale.append(f"{rel}: {_line_count(path)} lines, not over {limit}")
    assert not stale, "ceilings to remove:\n" + "\n".join(stale)
