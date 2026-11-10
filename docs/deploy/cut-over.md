# Cut-over runbook: from option-trader to optdesk

The design is `docs/design/cut-over.md` (Decisions 1 to 10). This runbook lists the 15 steps of
Decision 10 in order. Every value in angle brackets is a placeholder the owner fills in; no path,
account id or token is written into this repository. Commands print counts and statuses only.

Rules that hold for every step:

- The old DuckDB file is never read live: steps 2 and 12 make a copy, and the exporter reads the
  copy, read-only.
- The old serve is not restarted after step 11. After that step the old system's collection gap is
  the one the alignment index accepts.
- The old DuckDB copy is kept until the record `docs/deploy/cut-over-record.md` is merged.
- No step imports an owner-entered table (owner inputs, plans, settings, follow list, spread
  kinds): owner decision 2026-11-09. The owner enters them again by hand in optdesk.
- Live-only figures (class `live_only` in `docs/deploy/cut-over-figures.toml`: they need quotes,
  chains, IV or Greeks of the day, which nothing stored holds) are checked by the parallel run
  only; no import or replay checks them.
- A rehearsal is an import and a verify on a fresh scratch database each time: a new database
  from `ot setup`, named by the environment file of the profile used.

## Steps

1. Back up the new database and restore the backup into a scratch database; compare the row counts
   of the two. This is the restore drill the alignment index defers to this design. Stop here if a
   count differs.
2. Copy the old DuckDB file (a copy, never the live file) to `<copy-1>.duckdb`.
3. Export the account history from the copy; it writes `account.jsonl` and prints counts only:

   ```
   python -m ot_history_export account <copy-1>.duckdb <export-1-dir> --through <day>
   ```

4. Rehearsal 1, on a fresh scratch database, with the export of step 3:

   ```
   ot account-import <export-1-dir> --map <alias>=<alias> --through <day> --report <import-1.json>
   ot account-import <export-1-dir> --map <alias>=<alias> --through <day> --verify --report <verify-1.json>
   ```

   The verify must exit 0 (`Missing` and `Different` both 0). Keep `<verify-1.json>`.
5. Rehearsal 2, on another fresh scratch database and on a later export (repeat steps 2 and 3 into
   `<export-2-dir>`), with the same two commands. Keep `<verify-2.json>`.
6. Export `account.jsonl` again, with `--through` the last complete session day before the day the
   new scheduler will start; the old collectors and the old serve are still running:

   ```
   python -m ot_history_export account <copy-1>.duckdb <export-final-dir> --through <last-complete-day>
   ```

7. The final import, the same command on the live database:

   ```
   ot account-import <export-final-dir> --map <alias>=<alias> --through <last-complete-day> --report <import-final.json>
   ```

   `Conflict` above 0 makes it exit non-zero: stop and read the counts.
8. Verify the final import (it writes nothing); it must exit 0:

   ```
   ot account-import <export-final-dir> --map <alias>=<alias> --through <last-complete-day> --verify --report <verify-final.json>
   ```

   The three reports `<verify-1.json>`, `<verify-2.json>` and `<verify-final.json>` are the
   evidence; their SHA-256 hashes go into the record.
9. Start the new scheduler (owner; `docs/deploy/scheduler.md`). It starts on the session day after
   `--through`, so an import capture and a live capture never share a day.
10. The parallel run: N session days (10 unless the owner changes it), both systems collecting. Each
    session day the owner saves the page JSON of the old serve into the pages directory
    `<pages-dir>`. Live-only figures are checked by this run only.
11. Stop the old collectors (owner). The old serve is not restarted after this step.
12. Copy the old DuckDB file again, to `<copy-2>.duckdb`. Keep both copies until the record is
    merged.
13. Export `figure.jsonl` from that copy and the saved pages, bounded by the first and the last
    session day of the parallel run (this is the second export of figures, after the parallel run):

    ```
    python -m ot_history_export figures <copy-2>.duckdb <pages-dir> <figure-dir> --figures-from <first-day> --figures-to <last-day>
    ```

14. Compare every figure of the register with optdesk's readings:

    ```
    ot compare-figures --old <figure-dir>/figure.jsonl --register docs/conventions/register.toml --classes docs/deploy/cut-over-figures.toml --explained <explained.toml> --report <compare.json>
    ```

    It exits 0 only when no figure is `unexplained`. An id the classes file does not name is an
    error and no report is written. The owner writes the `[[explained]]` entries (`id`, `reason`,
    `date`) of `<explained.toml>`; the classes file holds none. `unchecked` figures are printed with
    their reasons (`path_not_found`, `no_old_value`, `not_computed_yet`); read each one.
15. Close: the owner fills `docs/deploy/cut-over-record.md` (the three verify report hashes, the
    dates, the exit code and the `unchecked` count of step 14 with each reason, and the check that
    `web/no-compute-allow.txt` has no line) and merges it. The cut-over is closed when that pull
    request is merged; only then may the old DuckDB copies be deleted.
