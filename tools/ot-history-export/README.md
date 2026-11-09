# ot-history-export

Dev-only exporter of option-trader's old account snapshots and old figures to JSON Lines, for
`ot account-import` and `ot compare-figures` (docs/design/cut-over.md, Decisions 1, 2 and 4).
Not run in CI; nothing imports it at run time.

1. Use a copy of option-trader's DuckDB file, never the live one. The file is opened read-only.
2. Account snapshots (the last snapshot of each alias and session day, with positions and summary
   values), up to the last complete session day:

       uv run --project tools/ot-history-export python -m ot_history_export account COPY.duckdb OUTDIR --through DAY

   `--through` is required. A later day is not written and is counted `later_day_refused`.
3. Old figures, after the parallel run: save the old serve's page JSON of each session day as
   `PAGES_DIR/DAY/<page>.json`, then:

       uv run --project tools/ot-history-export python -m ot_history_export figures COPY.duckdb PAGES_DIR OUTDIR --figures-from DAY --figures-to DAY

   The `pages` paths of `docs/conventions/register.toml` are read in the saved pages (`--register FILE`
   names another register); the first segment of a path is the page file. The rows of the stored
   `reading` table are added. No line outside the two days is written. A path it cannot find is
   counted `path_not_found`, a null or non-numeric value `no_old_value`; neither is written as zero.

It prints counts only. It reads only the tables `account_snapshot`, `account_position`,
`account_summary` and `reading`; owner-entered tables are not exported. It refuses a table whose
columns differ from those of `src/ot_history_export/accounts.py` and `figures.py`, and a file with
an alias that looks like an IBKR account id (letters then digits): only aliases leave the old system.

Test: `uv run --project tools/ot-history-export pytest`.
