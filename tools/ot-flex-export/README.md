# ot-flex-export

Dev-only exporter of option-trader's Flex trade rows (`flex_trade_row`, every revision) to JSON Lines,
for `ot ledger-import` (docs/design/option-ledger.md, Decision 7). Not run in CI; nothing imports it at run time.

1. Use a copy of option-trader's DuckDB file, never the live one.
2. Run it once:

       uv run --project tools/ot-flex-export python -m ot_flex_export COPY.duckdb OUT.jsonl

   The file is opened read-only. It prints counts only. It refuses a table whose columns differ from
   those of option-trader's `flex/store.py`.
3. Then run `ot ledger-import OUT.jsonl`.

Test: `uv run --project tools/ot-flex-export pytest`.
