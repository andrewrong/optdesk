"""Entry point of `python -m ot_history_export account|figures ...`."""

import argparse
import sys
from datetime import date
from pathlib import Path

import duckdb

from ot_history_export.accounts import SchemaError, export_account
from ot_history_export.figures import DEFAULT_REGISTER, export_figures


def _day(text: str) -> date:
    try:
        return date.fromisoformat(text)
    except ValueError as e:
        raise argparse.ArgumentTypeError(f"not a day YYYY-MM-DD: {text!r}") from e


def _parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="python -m ot_history_export")
    sub = p.add_subparsers(dest="command", required=True)
    a = sub.add_parser("account", help="write account.jsonl")
    a.add_argument("db", type=Path)
    a.add_argument("outdir", type=Path)
    a.add_argument("--through", type=_day, required=True)
    f = sub.add_parser("figures", help="write figure.jsonl")
    f.add_argument("db", type=Path)
    f.add_argument("pages_dir", type=Path)
    f.add_argument("outdir", type=Path)
    f.add_argument("--figures-from", type=_day, required=True)
    f.add_argument("--figures-to", type=_day, required=True)
    f.add_argument("--register", type=Path, default=DEFAULT_REGISTER)
    return p


def main(argv: list[str]) -> int:
    """Run a command; print counts only, never a value, an alias or an account number."""
    args = _parser().parse_args(argv)
    try:
        if args.command == "account":
            counts = export_account(args.db, args.outdir, args.through)
        else:
            counts = export_figures(
                args.db, args.pages_dir, args.outdir,
                args.figures_from, args.figures_to, args.register,
            )
    except SchemaError as e:
        print(f"refused: {e}")
        return 1
    except duckdb.Error as e:
        print(f"cannot read the database: {type(e).__name__}")
        return 1
    except (OSError, ValueError) as e:
        print(f"failed: {type(e).__name__}")
        return 1
    for key, n in counts.items():
        print(f"{key}: {n}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
