# data repair, steps 2 and 3 (what bars can serve): daily bars are stored by key and a forced console run of a past day overwrites them

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 1

Source: owner 2026-11-13 S1 to S3 ("overwrite by key, a re-run replaces", "key = instrument + the data's own date, capture date only a column", "a repair is a page action"); storage plan v2 of the advisor (P5, P6 items 1 and 3, cut order steps 2 and 3). The decisions of those documents are not changed here. The precedent is `data-repair-fx.md` (landed: entries #2151, #2152, #2156); its table shape (`status`, `updated_at`, `written_by`), column grants, upsert statement, optional-upserter interface and any-day console row are copied, not re-argued.

Goal: `obs.price_bar` becomes repairable. Its key is `(instrument_id, basis, session_day)` (no epoch); a run stores each pulled bar by that key: a differing value replaces the stored one, an equal one touches nothing, no old value is kept. The console row `bars_refresh` takes a chosen day (not after today) and Force; a forced past day re-pulls 400 days ending there and overwrites them.

Why one design for steps 2 and 3 and not the whole of step 2: step 2 is "Force and day for every by-key console row", but a row offers Force only when its table is by-key, and today only `bars_refresh` can become so in one step. The other force-no rows wait on their own dataset step: `fred_series` (series key, step 4), `compute_figures` (readings by key, step 6), `earnings_refresh` and `dividend_capture` (step 7), `fundamentals` (step 7 for the table; its reason is also the Alpha Vantage quota), `option_eod`, `option_eod_jp`, `option_eod_index`, `quote_snapshot` (step 8, captures), `account_capture` and `option_candidates` (captures, append-only by owner rule), `position_marks` (attempt rows), `underlying_resolve`, `iv_history`, `option_list_preopen`, `option_list_zone`, `option_list_preopen_jp` (reason `not_proved`: nothing proves a second run equals the first; `iv_history` writes readings, step 6). The eight listed-only rows have no form at all. Each of those steps flips its own row in the same design.

## Today

All paths at origin/main 29c24a9a.

- `migrations/0021_bars.sql:10-31` creates `obs.price_bar (instrument_id, session_day, basis, adjust_epoch, open, high, low, close, volume, currency, source, received_at)`, key `(instrument_id, basis, adjust_epoch, session_day)`, prices and volume as text with pattern checks, `price_bar_source_ck` (`ibkr`), `price_bar_basis_ck` (`trades`, `adjusted_last`), `price_bar_epoch_ck` (>= 1); `:48-53` the view `rd.price_bar_r1` keeps the rows of the highest epoch of each instrument and basis through a correlated `max(adjust_epoch)` per row; `:64-66` `ot_w_bar` has INSERT on the table and SELECT on the view.
- `internal/store/barpg/store.go:23-29` `insertPriceBarSQL` is a plain INSERT with `ON CONFLICT DO NOTHING`; `:108` `InsertPriceBars`; `read.go:12-16` reads the view. `internal/store/bar/api.go:18` `PriceBarWriter`, `:26` `PriceBarReader`; `FxRateUpserter` (`:52`) is the precedent.
- `internal/jobs/barsrefresh/run.go:75-139` `runKey`: reads the whole stored pair, `epoch = max(maxEpoch(stored), 1)`; a name with no row inside `backfill_days` (400) is a first pull of 400 days (capped at `backfill_names_per_run`, in memory, `claimFirstPull` `:57`), else `daily_days` (5); the bar query has no `End`; `restated` (`restate.go`) compares pulled closes with stored ones at tolerance 0.00001 and, when one differs, `epoch++` and a 400-day re-pull; `rows` (`:161`) leaves out days already stored. `deepen.go` pulls pieces of `deepen_piece_days` (730) older than the oldest stored day up to `deepen_days` (1825); `adjusted.go` writes the whole 400-day `adjusted_last` window as `epoch + 1` once a week (cadence read from the newest `ReceivedAt`). `job.go:139` says a re-run inserts only what is missing.
- `cmd/ot/jobconsole_jobs.go:98` declares `todayOnly("bars_refresh", "epoch_and_caps", textEpochAndCaps)`; `:45` the text; `jobconsole_jobs_test.go:78` lists the reason code. The table has 28 rows: 5 any-day, 15 today-only, 8 listed-only; force-yes are `calendar_refresh` and `fx_refresh`; 18 are force-no (`TestEveryForceNoJobHasReason`).
- `cmd/ot/wire.go:490` `noBarStore.InsertPriceBars` returns the no-DSN error and has no upsert method; `wire.go` is 795 lines. `internal/jobs/computefigures` reads bars only through `PriceBarReader.PriceBars` (`hv.go:27`); its tests build `contracts.PriceBar` with `AdjustEpoch` (`hv_test.go`, `rv_test.go`, `tech_test.go`).
- `contracts.PriceBar` (generated from `contracts/datasets/obs.price_bar.r1.json`) has the field `AdjustEpoch`. It is not changed here (Decision 2).

## Decisions

1. Key and shape (owner S2; plan P6). `obs.price_bar` is keyed `(instrument_id, basis, session_day)`. `basis` stays in the key: `trades` and `adjusted_last` are two series of one instrument. It gains `status` (`ok`, `no_value`, `void`; default `ok`), `updated_at` and `written_by`, appended after `received_at` so that a positional INSERT of twelve values still works; `open`, `high`, `low`, `close` and `volume` become `numeric` (`USING col::numeric`, which keeps the scale a text carried), `open`..`close` lose NOT NULL, and a check keeps all four positive unless `status = 'no_value'`; the text pattern checks go. `source` stays `ibkr` (a second source and its rank are step 9). `received_at` stays the time of the first insert and is never updated.
2. `adjust_epoch` stays as a column, out of the key, `DEFAULT 1`, and no code writes it: the old binary keeps working on the migrated table during the deploy window (its INSERT still names the column and, for an existing key, adds nothing). The record `contracts.PriceBar` keeps `AdjustEpoch`; the view returns the column, which is 1 in every row after the migration. Dropping the column and the record field is a contracts change (Left out).
3. The migration (owner-only DDL, run by the window) first deletes the rows the view hides today (those of an epoch below the maximum of their instrument and basis) and sets `adjust_epoch` to 1 on the rest, so the table reads as the view does now; then changes the key. This deletes only rows no reader sees. Old history that a restatement hid is pulled again by the deepening step (Decision 7).
4. The view `rd.price_bar_r1` is dropped and created again with the same twelve columns and types (numbers as `col::text`), `WHERE status = 'ok'` and no `max(adjust_epoch)` subquery: the correlated read of plan P6 item 1 ends here. No reader, page or record changes: `barpg/read.go` selects the same columns.
5. Roles (as `data-repair-fx.md` Decision 3). `ot_w_bar` keeps table-level INSERT and gets `UPDATE` on `open, high, low, close, volume, currency, source, status, updated_at, written_by` and `SELECT` on `instrument_id, basis, session_day, open, high, low, close, volume, currency, source, status` (the arbiter columns and the columns of `DO UPDATE ... WHERE`). No DELETE, no TRUNCATE, no UPDATE of the key, `received_at` or `adjust_epoch`. `ot_ro_pages` is unchanged.
6. The write (as FX Decision 4). `INSERT ... ON CONFLICT (instrument_id, basis, session_day) DO UPDATE SET open, high, low, close, volume, currency, source, status = 'ok', updated_at = now(), written_by WHERE (stored open, high, low, close, volume, currency, source, status) IS DISTINCT FROM (new ones) RETURNING instrument_id`, one statement per row, one transaction per call; numeric equality decides; the insert names no `adjust_epoch`. The comparison includes `source`: with the only source `ibkr` this changes nothing today; the rule that a backup source only fills gaps (never replaces an `ibkr` row) is added to this statement by step 9, and until then no second source may be written. Two counts: written and unchanged. An empty list makes no call; a key twice in one call is applied in order.
7. The job. `New` and `NewBarsRefreshFunc` are unchanged. `Run` asserts the writer to `bar.PriceBarUpserter` and returns a Retryable error naming the missing bar store otherwise (as `fxrefresh`; `noBarStore` and `wire.go` are not edited). Per key:
   - The pull window ends at the run day: `End` = day when the day is before the UTC date of the clock, empty otherwise (a scheduled run of today is unchanged); bars after the day are dropped. A past-day run (a forced repair, or the scheduler catching up) pulls `backfill_days` (400) at once, because a repair wants a range and one request covers it; it does not use the first-pull cap unless the name has no row inside the window (then as today). Today's run keeps `daily_days` (5) or the first-pull 400.
   - The weekly `adjusted_last` step and the deepening step run only for a run of today (day not before the UTC date); a past-day run never runs them, so a repair never writes today's adjusted window.
   - Every pulled bar is upserted in one call with `writtenBy = "bars_refresh:" + day`; the "leave out stored days" step goes. The deepening pieces and the weekly `adjusted_last` window are upserted the same way.
   - A restatement (a pulled close differs from the stored one beyond the tolerance, the rule of `restate.go`) no longer rotates an epoch. The 400-day window is upserted, and then the older stored days, which the epoch used to hide, are pulled again in pieces of `deepen_piece_days` ending the day before the oldest pulled day, down to the oldest stored day, each piece upserted; a piece with no older day stops the loop. Without this a split would leave unadjusted history next to adjusted history.
   - `maxEpoch` and every use of the epoch go; the weekly cadence still reads the newest `ReceivedAt`, which now means the newest first-insert (a week with no new trading day asks again each day until one arrives).
   - A bar that is not a positive decimal is Permanent, a store error Retryable unless typed (as today). `InsertPriceBars` stays in the writer interface, unused (Left out).
8. The console. `bars_refresh` becomes `{ID: "bars_refresh", Kind: kindAnyDay, Force: true}`; the post, the queue, `BackfillWith` and its hold of a key until its slot on the day of today are used as they are. A forced run of a day runs every key of the job (no key picker, Left out). `textEpochAndCaps` and the reason code `epoch_and_caps` go; the table keeps 28 rows: 6 any-day, 14 today-only, 8 listed-only, force-yes `calendar_refresh`, `fx_refresh`, `bars_refresh`, 17 force-no.
9. Deploy. Entry 2 holds the auto-deploy (migration). The window runs, after the merge of entry 2: rebuild `ot`, `ot setup pg`, migrate, `ot setup pg` again. Entries 3 and 4 merge after the migration. If entry 3 lands first, `UpsertPriceBars` fails Retryable (no column `status`) and no data changes. Between the migration and entry 3 the old binary's insert works and writes nothing for a key that exists (a pg test shows it); its restatement test would then loop on two pulls a day and store nothing, until entry 3 merges.
10. Pacing: a past-day run asks 400 days for every key and, on a restatement, up to three more pieces; it skips `claimFirstPull` for a name that has rows, so the cap of 10 first pulls does not bound it. It is bounded by the number of keys and by the console's one-run-at-a-time hold of the job.
11. No old value is kept (owner S1). The row keeps `source`, `updated_at` and `written_by` (job and day). No `void` is written by code here.

## Modules

No row of `docs/design/modules.md` changes. The entries touch: store (`internal/store/**`, `migrations/**`, `docs/contracts/store.md`; entries 1 and 2), jobs (`internal/jobs/**`, `docs/contracts/jobs/**`; entry 3) and ot (`cmd/**`; entry 4). `internal/jobs/barsrefresh` already imports `internal/store/bar`; no new import edge.

## Interface

One interface change, entry 1, in `internal/store/bar/api.go` (additive):

    // BarWriteCount is what one UpsertPriceBars call did to the rows it was given: Written rows were inserted or
    // replaced, Unchanged rows were equal to the stored one.
    type BarWriteCount struct {
        Written, Unchanged int
    }

    // PriceBarUpserter writes daily bars by the key (InstrumentID, Basis, SessionDay): a differing row replaces the
    // stored one, an equal row changes nothing, no old value is kept. writtenBy names the run (job and day). It is an
    // optional method of the value OpenBarFunc returns; the job asserts it at Run.
    type PriceBarUpserter interface {
        UpsertPriceBars(ctx context.Context, rows []contracts.PriceBar, writtenBy string) (BarWriteCount, error)
    }

`PriceBarWriter`, `PriceBarReader`, `BarStore`, `OpenBarFunc` and `contracts.PriceBar` are unchanged: no fake breaks. The row's `AdjustEpoch` is ignored by the upsert.

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. Entry 2 holds the auto-deploy; entries 3 and 4 merge after the window has run the migration (Decision 9). `Contracts: unchanged` (no file under `contracts/schemas`, `contracts/datasets` or `contracts/proto` changes). Test runs are in `## Checked by CI`.

### 1 data-repair-bars-api (interface)

Module: store. Size: S.
Order: first.
Files: `internal/store/bar/api.go`, `internal/store/bar/api_test.go`
Design: docs/design/data-repair-bars.md#Issues (Decisions 6, 7; Interface)
Acceptance:
1. `api.go` declares `BarWriteCount` and `PriceBarUpserter` as in the Interface section, with docs and no body; `PriceBarWriter`, `PriceBarReader`, `FxRateWriter`, `FxRateUpserter`, `BarStore` and `OpenBarFunc` are unchanged (their declarations show no diff line).
2. `api_test.go` has a new contract test `TestPriceBarUpserterContract` (`t.Parallel()`) calling `UpsertPriceBars` through the interface on a fake: it counts a written and an unchanged row and the caller reads both back, an empty list returns zero counts, and the fake satisfies `PriceBarWriter` and `PriceBarUpserter`.
3. `git diff --name-only BASE` lists only paths of the Files line; `api.go` stays under 100 lines.

### 2 data-repair-bars-store

Module: store. Size: M.
Order: after 1. Holds the auto-deploy: it carries a migration (Decision 9).
Files: `migrations/0041_price_bar_repair.sql`, `internal/store/barpg/store.go`, `internal/store/barpg/read.go`, `internal/store/barpg/store_test.go`, `internal/store/barpg/store_pg_test.go`, `internal/store/barpg/upsert_pg_test.go`, `internal/store/setup/setup.go`, `docs/contracts/store.md`
Design: docs/design/data-repair-bars.md#Issues (Decisions 1 to 6, 9, 10)
`read.go` changes only the comment on `selectPriceBarSQL` ("The view holds the winner epoch"); its SQL is unchanged (`git diff BASE -- internal/store/barpg/read.go` shows comment lines only). `setup.go` changes only the comment of `roleBar`; `setup_test.go` is not edited. `git grep -l "0021_bars" -- '*.go'` prints only `store_pg_test.go` and `setup.go` (a comment), so no other test builds the table. `internal/store/barpg/store_pg_test.go` is 447 lines: its new tests go into the new file.
Acceptance:
1. `0041_price_bar_repair.sql` has an Up and a Down part, is owned by `ot_owner`, and in Up, in this order: drops the view; deletes the rows whose `adjust_epoch` is below the maximum of their `(instrument_id, basis)`; sets `adjust_epoch` to 1; drops the primary key and the pattern and positive checks on the four prices; turns the five columns to `numeric` (`USING col::numeric`); drops NOT NULL on `open`, `high`, `low`, `close`; adds the primary key `(instrument_id, basis, session_day)`; sets `DEFAULT 1` on `adjust_epoch`; adds the positive check unless `status = 'no_value'`; adds `status` (check `ok`, `no_value`, `void`; default `ok`), `updated_at` (filled from `received_at`, then NOT NULL, default `now()`) and `written_by` (default empty) after `received_at`; creates `rd.price_bar_r1` again with the twelve columns, `col::text` for the five numbers, `WHERE status = 'ok'` and no subquery, owned by `ot_owner`, with SELECT to `ot_w_bar` and `ot_ro_pages`.
2. The grants of the migration are those of Decision 5 (`git grep -n "GRANT" -- migrations/0041_price_bar_repair.sql` shows the UPDATE and SELECT column lists of Decision 5 and nothing on `adjust_epoch`, `received_at`, `updated_at` or `written_by` for SELECT); no DELETE, no TRUNCATE. Down first raises an exception, changing nothing, when any row has a status other than `ok`, a NULL price or a number the old pattern refuses; then it revokes, restores text columns, the old key `(instrument_id, basis, adjust_epoch, session_day)`, the checks and the old view, and drops the three columns. Down does not bring back the rows Up deleted (the file says so).
3. `store.go` has `UpsertPriceBars(ctx, rows, writtenBy) (bar.BarWriteCount, error)` and `var _ bar.PriceBarUpserter = (*Store)(nil)`; its SQL is the statement of Decision 6 (`ON CONFLICT (instrument_id, basis, session_day) DO UPDATE`, `IS DISTINCT FROM`, `RETURNING instrument_id`, numeric casts, no `adjust_epoch` in the column list), one transaction per call through `insertAll`, written = rows returned and unchanged = given minus returned, an empty list makes no call, class 23 is Permanent and every other error Retryable. `InsertPriceBars`, `insertPriceBarSQL` and the FX writes are not changed. The package comment says `obs.price_bar` and `obs.fx_rate` are both upserted.
4. `store_test.go` (fake database seam, `t.Parallel()`) has new tests `TestUpsertBarSQLReplacesOnlyWhenDifferent` (text holds `DO UPDATE`, `IS DISTINCT FROM`, no `DELETE`, never sets `received_at`, the key or `adjust_epoch`), `TestUpsertBarEmptyMakesNoCall`, `TestUpsertBarCountsWrittenUnchanged` (three given, two returned: written 2, unchanged 1) and `TestUpsertBarWrapsErrors`; the existing insert-text test is unchanged.
5. `store_pg_test.go` (tag `pg`) applies `0041_price_bar_repair.sql` after `0039_fx_rate_repair.sql` in `newDatabase`; `TestBarRoleGrants` keeps its denials but its `UPDATE obs.price_bar SET close = '1'` denial becomes denials of `SET session_day`, `SET received_at` and `SET adjust_epoch`, plus `SELECT received_at FROM obs.price_bar`; `TestNewEpochWins` is replaced by the new tests of item 6; the other tests still pass unchanged.
6. `upsert_pg_test.go` (tag `pg`, `t.Parallel()`, one database each) has new tests: `TestPriceBarUpsertAsWriterRole` (as `ot_w_bar`: insert; the same value again is unchanged and `updated_at` does not move; `100.50` after `100.5` is unchanged; a different close is replaced, `received_at` stays, `written_by` and `updated_at` change; a `void` row answered with a value is replaced; `trades` and `adjusted_last` of one day are two rows), `TestPriceBarViewHasOneRowPerKey` (an owner-made `void` row is not in `rd.price_bar_r1`; `PriceBars` returns one row per day with `AdjustEpoch` 1), `TestPriceBarMigrationKeepsWinners` (a database at 0021 with rows of epochs 1 and 2 gets 0041: the epoch-2 rows read back with the same text, the epoch-1 rows of days the epoch-2 set lacks are gone, status `ok`, `updated_at` equal `received_at`) and `TestOldPriceBarInsertSQLWorksOnMigratedTable` (as `ot_w_bar`, the unchanged `insertPriceBarSQL` with epoch 2 inserts a new key and adds nothing for an existing key, no error).
7. `docs/contracts/store.md` says `obs.price_bar` is upserted by `UpsertPriceBars` (Decisions 5 and 6, no old value kept, the key without epoch) and no longer says "append-only"/"insert-only" for it; the `roleBar` comment names the new grants.
8. `git diff --name-only BASE` lists only paths of the Files line; `store.go` stays under 300 lines; the pull request body carries `Contracts: unchanged`.

### 3 data-repair-bars-job

Module: jobs. Size: L.
Order: after 1; merged after entry 2 and the migration (deploy meaning only; not a build dependency).
Files: `internal/jobs/barsrefresh/run.go`, `internal/jobs/barsrefresh/adjusted.go`, `internal/jobs/barsrefresh/deepen.go`, `internal/jobs/barsrefresh/restate.go`, `internal/jobs/barsrefresh/job.go`, `internal/jobs/barsrefresh/run_test.go`, `internal/jobs/barsrefresh/deepen_test.go`, `internal/jobs/barsrefresh/adjusted_test.go`, `internal/jobs/barsrefresh/repair_test.go`, `docs/contracts/jobs/bars_refresh.md`
Design: docs/design/data-repair-bars.md#Issues (Decisions 6, 7)
`types.go`, `types_test.go`, `manifest.go`, `manifest_test.go`, `plan.go`, `plan_test.go`, `restate_test.go` and `job.toml` are not edited: the constructor type is unchanged, `restate_test.go` tests `restated` and `differs` only (`git grep -n "maxEpoch" -- internal/jobs/barsrefresh` after the change prints nothing). `cmd/ot/wire.go:258` and `cmd/ot/wire_computefigures_test.go:114` keep their calls (`git grep -ln "barsrefresh\." -- cmd` is unchanged). `internal/jobs/computefigures` tests build `PriceBar{AdjustEpoch: 1}` and compile unchanged.
Acceptance:
1. `Run` asserts the writer to `bar.PriceBarUpserter` before any broker call and, when it is not, returns a Retryable error naming the missing bar store, never falling back to `InsertPriceBars`; `git grep -n "InsertPriceBars\|maxEpoch\|AdjustEpoch: *epoch\|epoch++" -- internal/jobs/barsrefresh --exclude=*_test.go` prints nothing; the stored-days skip in `rows` is gone.
2. A run of a day before the clock's UTC date sends the bar query with `End` = the day and `DurationDays` = `backfill_days`; a run of today sends no `End` and `daily_days` (or `backfill_days` for a first pull, under the cap as today); bars after the day are dropped; every kept bar goes into one `UpsertPriceBars` call (per pull piece) with `writtenBy` = `bars_refresh:` plus the day; deepening pieces and the weekly `adjusted_last` window go through the same call.
3. A past-day run never calls the deepening or weekly adjusted step (new test in item 5). A restatement upserts the 400-day window and then pulls the older stored days again in pieces of `deepen_piece_days`, ending the day before the oldest pulled day, down to the oldest stored day, upserting each piece; a piece with no older day stops the loop; a close that is not a positive decimal is Permanent and nothing of that key's pull is written.
4. `run_test.go` extends `memBars` with `UpsertPriceBars` (the rules of Decision 6 on a map by key) and drops `TestRestatedDayRepullsUnderNextEpoch`, `TestReaddedNameWritesAtOrAbovePairEpoch` and `TestDailyPullInsertsOnlyNewDays`; `deepen_test.go` and `adjusted_test.go` lose their epoch assertions (`TestAdjustedWritesWholeWindowAsNextEpoch` becomes new `TestAdjustedOverwritesWholeWindowByKey`); the other existing tests keep their assertions; every test calls `t.Parallel()`.
5. `repair_test.go` has the new tests `TestDailyPullReplacesDifferingDay`, `TestEqualDayIsNotRewritten` (`100.50` against `100.5`), `TestPastDayAsksEndDayAnd400Days`, `TestTodayAsksNoEnd`, `TestBarsAfterDayAreDropped`, `TestPastDayRunsNeitherAdjustedNorDeepen`, `TestEmptyAnswerNeverReplaces` (stored rows stay, no write call), `TestRestatementRepullsOlderStoredDays` (a split: the window and two older pieces are overwritten, no stale old close stays), `TestRestatementStopsAtEmptyPiece` and `TestRunWithoutUpserterIsRetryable` (a writer that is only a `PriceBarWriter`: `New` succeeds, `Run` returns a Retryable error, no broker call).
6. `docs/contracts/jobs/bars_refresh.md` states the new `Run` (End rule, 400-day past-day window, upsert and `writtenBy`, restatement without epoch) and lists the tests of items 4 and 5; the comment of `RestartSensitive` in `job.go` says a re-run is idempotent.
7. `git diff --name-only BASE` lists only paths of the Files line; `run.go` stays under 300 lines and `repair_test.go` under 400; the pull request body carries `Contracts: unchanged`.

### 4 data-repair-bars-console

Module: ot. Size: S.
Order: after 3 (deploy meaning, not a build order: the change builds alone; it merges after entry 3 and the migration so that a forced run does what the page promises).
Files: `cmd/ot/jobconsole_jobs.go`, `cmd/ot/jobconsole_jobs_test.go`
Design: docs/design/data-repair-bars.md#Issues (Decision 8)
`jobconsole_post.go`, `jobconsole_site_test.go`, `jobconsole_turn_test.go` and `internal/pages/job_console` are not edited (`git grep -n "bars_refresh" -- 'cmd/ot/jobconsole_*.go'` prints only the two Files).
Acceptance:
1. `jobTable` holds `{ID: "bars_refresh", Kind: kindAnyDay, Force: true}` in the same place; `textEpochAndCaps` is gone (`git grep -n "epoch_and_caps\|textEpochAndCaps" -- cmd` prints nothing); the table still has 28 rows and the other rows are unchanged.
2. `jobconsole_jobs_test.go`: the any-day set (`compute_figures`, `fred_series`, `fundamentals`, `fx_refresh`, `option_candidates`) gains `bars_refresh` (six rows); `wantTwelve` (15 entries) loses `bars_refresh` and holds 14; the force-yes set is exactly `calendar_refresh`, `fx_refresh`, `bars_refresh`; the force-no count goes from 18 to 17 ("want 17"); the code list loses `epoch_and_caps`; the renamed tests are `TestAnyDayIsExactlyTheSix`, `TestForceYesIsCalendarFxAndBars` (new names, same bodies edited), each with `t.Parallel()`; the variable `wantTwelve` keeps its name; `TestTableIsTheWiredJobSet` and `TestListedOnlyIsExactlyTheEight` are unchanged.
3. In `TestFxRefreshRunValidatesPastDayWithForce` the last assertion (`post("bars_refresh", "2026-11-13", "on")` refused) now uses the force-no job `fred_series`. A new test `TestBarsRefreshRunValidatesPastDayWithForce` calls `readJobRun` with job `bars_refresh`, a day before today and `force=on` and gets a run with force true; a day after today is refused.
4. `git diff --name-only BASE` lists only paths of the Files line; the pull request body carries `Contracts: unchanged`.

## Checked by CI

`go test ./internal/store/... ./internal/jobs/barsrefresh/... ./internal/jobs/computefigures/... ./cmd/ot/... -race -count=1` (the `pg` tests of entry 2, with `0041` applied to a fresh database, run only in GitHub CI), `go run ./scripts/arch --check`, `go build ./...`, `golangci-lint`, and the contracts generation check (no change expected). Results only a run shows: the upsert counts as `ot_w_bar` with the column grants alone (`TestPriceBarUpsertAsWriterRole`); the unchanged old insert works on the migrated table (`TestOldPriceBarInsertSQLWorksOnMigratedTable`); winners survive the migration (`TestPriceBarMigrationKeepsWinners`); the role still cannot delete, truncate or update a key (`TestBarRoleGrants`); a forced past day asks the broker for 400 days ending on it and overwrites (entry 3). The review runs no test.

## Left out

- The other force-no rows (the list under "Why one design"): each flips with its own dataset step. The console range field (a `from` day in the request) is not built: a forced past day already covers 400 days of bars; a range for the other datasets is decided in their steps.
- A key picker on the page (one instrument): a forced run covers every key of the job.
- `adjusted_last` repair by a forced run: it is rewritten weekly by the cadence and nothing reads it; a forced run of a past day does not run the weekly step (its cadence reads newer rows).
- Dropping `adjust_epoch`, the `AdjustEpoch` field of `contracts.PriceBar` and the "append-only" text of `contracts/datasets/obs.price_bar.r1.json` (a contracts change with generated output), and `InsertPriceBars` with `insertPriceBarSQL` (unused after entry 3; a later cleanup of `PriceBarWriter` and its fakes).
- The `source` check, a second bar source, source rank and the bought history (step 9; P5).
- The text of `docs/design/job-console.md` (it still says Force is yes for one job): owner-only design file.

## Doubts

1. The grants follow the PostgreSQL rules for `ON CONFLICT ... DO UPDATE ... WHERE` as in the FX design; no database was available here, so the pg tests of entry 2 are the check and run only in GitHub CI.
2. The migration number 0041 is the next free one on origin/main 29c24a9a; a design that lands first moves it.
3. The 400-day window of a past-day run is a choice, not a probe: one request per key returns 400 days, the same request a first pull sends today. Whether the service honours `End` for TRADES bars with the form `pyside/src/optdesk/ibkr/history.py` sends was seen working for `bars deepen`; not probed again here.
4. A repair overwrites with what IBKR answers now; a restated close cannot be undone from the database (owner accepted, S1). A split makes the restatement path pull up to 1825 days (3 pieces) for that key.
5. `received_at` now means the first insert; the weekly `adjusted_last` cadence reads it. A week with no new trading day for a name asks again each day until one arrives.
6. The UTC rule (as FX) treats a scheduled run that executes after 00:00 UTC for an ET or JP key as a past day: it pulls 400 days instead of 5 and upserts them; the same result, more rows compared.
7. `USING volume::numeric` has no pattern check behind it today (`0021_bars.sql` has none on `volume`): one non-numeric volume aborts the migration, which changes nothing; the owner reads the error and the row.
8. A split after the chosen day D leaves stored days after D on the old basis; a repair of D alone does not detect it. The restatement check of the next scheduled run does.
9. During the deploy window the old binary pulls twice a day on a restated name and stores nothing (Decision 9); short and harmless, but it is a request cost.

## Adversarial

Full answer: the scratchpad file adversarial-repair.md.

1. Gap against the goal: adopted in part. Source comparison explained in Decision 6 (backup-fills-gaps rule added by step 9; no second source until then); past-day runs skip the weekly adjusted and deepening steps (Decision 7, new test); a bar older than D-400 is repaired by forcing an older day (not changed).
2. Biggest risk, pacing and the UTC rule: adopted as named (Decision 10, Doubts 6 and 8); not bounded by a cap, by the owner's no-guard rule.
3. Simpler way, drop the restatement rewrite and keep text: not adopted. Without the rewrite a split leaves old and new basis side by side; text keeps the scale problem that numeric ends (same reason as the FX design).
4. Lock-in: not changed; the dead column and record field are named in Left out, the deletion in Decision 3.
5. Claim most likely false, `jobconsole_jobs_test.go:191` refusing force on `bars_refresh`: adopted (entry 4 item 3 moves it to `fred_series`).
6. Likeliest slip: adopted by the same fix, the past-day test, and Doubt 7 for the volume cast; `TestBarRoundTrip` is listed in entry 2 item 5 as unchanged and checked by CI.
