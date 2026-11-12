# Late collection: every pull stores its own rows, one rule reads them, a missing key is tried again until the next open

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 4 (revision 1 changed the key and added views; 2 added a retry tuple, a run count and a reason; 3 widened the window; revision 4 leaves the window and its close exactly as on main and adds a second time, collect-until, after which nothing is collected; late runs are one key per pass; option_open is collected late too; the observed time reaches the page before a late quote can be shown)

Source: owner asks 3015, 3021, 3023, 3027, 3029, 3032 and 3034, in meaning. Losing the open is fine while the market is still open: collect late, and repair what was missed until the next session opens; do not be rigid. A stop time only judges "normal or late", it never gives up: the owner trades options and needs the data, and repeated option pulls are fine. Storage must be extensible with no hard-coded logic; the owner does not care which capture a row came from, what matters is that collected data is stored and can be repaired; a capture id may be kept and used by logic but must never stop data from being added.
Read on origin/main at edd3bce7: `AGENTS.md`, `docs/design/modules.md`, `docs/design/scheduler.md`, `docs/design/option-open-gap-jp.md`, `docs/design/job-console.md`, the reconcile, sched, shared and store code the entries name, the manifests of the three jobs, `cmd/ot/listedjob.go`, `cmd/ot/wire_underlying.go` and `cmd/ot/schedstatus.go`, and the advisor's reviews r1 and r2. File:line cites are of that commit; "[unverified]" marks what no read could settle.

Goal: every pull of a snapshot dataset (option_open, option_eod, quote_snapshot; US and Japan) stores its rows whenever it is made, however late and however often; every page that shows a slot's value shows when it was observed; the pages and computations read one row per contract by one rule; the window, its close, its gap and its alert stay exactly as today and are the health mark; after the close a key that is missing, partial, empty or no_value is still collected, one key per pass, until the market's next open.

## What main holds today, against the owner's words

1. A retry is a no-op by key. The three tables key on a capture id (`migrations/0006_option_eod.sql:42`, `0007_quote_snapshot.sql:28`, `0029_option_open.sql:47`) and the capture id is a SHA1 of known text: `day|dataset|subject` (`internal/jobs/optioneod/map.go:57-59`, used at `optioneod/run.go:210`), `day|slot|instrument` (`optionopen/core.go:477-480`), `day|dataset|subject` (`quotesnapshot/run.go:77-79`, used at `run.go:122`). The inserts are `ON CONFLICT DO NOTHING` (`snapshot/optioneod.go:28`, `snapshot/quotesnapshot.go:24`, `optionopenpg/queries.go:19`). A second pull of a key adds nothing; the first answer stays, even an empty one. Three texts say so: `internal/jobs/api.go:10-12`, `contracts/datasets/obs.option_open.r1.json:14` and `docs/contracts/datasets/obs.option_open.md:10` ("deterministic ... meets the primary key").
2. Guards turn a late run into a refusal: "wrong day" `NoValue` when the key's day is not the local date (`optioneod/run.go:118-120`, `optionopen/pull.go:222-224`, `optionopen/jp.go:261-263`, `quotesnapshot/run.go:99-101`); "missed slot" `NoValue` ten minutes after a slot (`quotesnapshot/run.go:108-109`); `stop_after` as `NoValue` "after session" (`optioneod/run.go:123-124`, `option_eod_jp` only, 1700) or `Permanent` "not started" (`optionopen/pull.go:246-247`, `jp.go:284-286`; `job_pull.toml` 1000 ET, `job_pull_jp.toml` 0930 JST); `freeze_after` returns an empty result when the local time of day is before it (`optioneod/run.go:126-128`, 1615 US, 1550 JP), by the clock of now and not the key's day, so a late pull of an earlier day's key before that time of day does nothing. `quotesnapshot/run.go:150` ends `NoValue` "no quote" at the first empty slot. The slots of `quote_snapshot` are 0945, 1230 and 1530 but its one scheduled instant is the close minus 30 minutes (`quotesnapshot/job.toml:5` and its `[schedule]`), so today only the 1530 key produces; the other two end `NoValue` "missed slot".
3. The runner marks and skips. A `NoValue` result is marked `no_value` (`runkey.go:176-177`) and a marked key is skipped by `Reconcile` (`reconcile.go:255-257`). A counted run ends a key after two (`reconcile.go:333-364`). A key missing at its window close gets one more run after the close (reason `late`, `reconcile.go:336`), and then `closeRow`/`expireIfClosed` (`reconcile.go:370-421`) rewrite the latest run to `expired` or `partial` and mark `no_value`; `gaps.go:139` (`closed`) opens one `ops.job_gap` row per job, dataset and day (`migrations/0012_scheduler.sql:15`), which the notifier reports once. The window closes at the session close plus `lateness_hours` (`window.go:27-38`). `Backfill` and the console run a closed key that is missing or `no_value` (`backfill.go:60-100`, `backfillKey` at 282-300).
4. Readers take all rows. `OptionEod` and `QuoteSnapshots` return every row of a range ordered by day, instrument and capture id (`snapshot/optioneod.go:31-39`, `snapshot/quotesnapshot.go:27-31`); `optionopenpg` takes the newest row per instrument (`optionopenpg/queries.go:32-37`; contract text `openquote/api.go:35-37`). Consumers pick again by their own "latest" (`optioncandidates/calls.go:252`, `input.go:159`, `option_boards/load.go:85`, `option_data_page/builder.go:219-297`, `cmd/ot/live_quotes.go:230,486`, `companies_inputs.go`).
5. What a page shows of time: `option_boards/quote.go:98` shows the open row's `ReceivedAt`; `option_data_page` prints per point `Slot`, bid, ask, last and no time (`builder.go:110-116,236-239`), and its `data_time` is the maximum over all points (`builder.go:243-244`).

## Decisions

1. **A pull's capture id is its own, and the key does not change.** The capture id becomes `uuid.NewSHA1(OID, day|dataset|subject|start)` (option_open: `day|slot|instrument|start`) on one line each, where `start` is the instant the run began, `time.Time.UTC().Format(time.RFC3339Nano)`. Same-key pulls then have different capture ids, so a re-pull never meets the primary key and always appends; the primary keys, tables and base views stay, and `x-primary-key` stays. The three texts of item 1 of the section above say "one per pull" instead (`Contracts: changed`: `contracts/datasets/obs.option_open.r1.json` and `docs/contracts/datasets/obs.option_open.md` are contracts files). `capture_id` stays a column and no query decides by it except as the last tie key of a rule. Rows are only appended. What bounds the copies: a late run is one key per pass and at most one an hour per key (Decision 5) and stops at the collect-until time; inside the window a run is started as today. A late pull that returns only `no_data` rows appends them (about a chain of rows per run, `optioneod/run.go:377-380`): at most 7 runs a key from a Friday close to Monday's open for a list of 30 names (the one-key-per-pass cap spaces the keys), about 21 thousand unusable rows a weekend; one lone key can have about 65, one an hour; the reader hides them and the table keeps them (Doubt 4).
2. **One rule, in one place, with a final tie key.** One SQL fragment in `internal/store/shared` (allowed by `docs/design/modules.md:37`; `scripts/arch/main.go:102-104` has `sharedSub`; the package is new) is used by the three readers; each reader keeps its column list and signature. The rule picks one row per subject (contract; for `quote_snapshot` contract and slot) and ends every ordering with `received_at DESC, capture_id DESC` (never an unordered tie).
   - `option_eod` and `quote_snapshot`: priced rows before unpriced (`bid`, `ask` or `last` not NULL), among priced rows live data (`market_data_type = 1`) before frozen or delayed, for the option tables complete (`delta` not NULL) before incomplete, then `generation DESC`, then the newest. A frozen or last-only row of a later pull never displaces a live, complete one, and a live empty row never displaces a frozen priced one.
   - `option_open`: the open is the earliest live priced row at or after the slot within its session, full stop (`market_data_type = 1`, price present, `received_at >= requested slot instant` which the job guarantees by not quoting before its slot, and `in_session`). The greeks of the open row are its own; a page may show greeks from another row only when it says so. "Newest" applies to the day's best row only: when a contract has no such row the reader returns its newest row, and that row, stored after the session or without a live price, is never presented as the open: its `observed_at` travels with it (Decision 4) and the page shows the time. `in_session` is a new column of `rd.option_open_r1` (one migration: the view gains `EXISTS (SELECT 1 FROM ops.dataset_window w WHERE w.dataset_id LIKE 'obs.option_open%' AND w.session_day = session_day AND received_at BETWEEN w.open_at AND w.close_at)`; the US and Japan sessions do not overlap in time, so no currency or dataset mapping is needed; the new column is read by the reader and is not in `contracts.OptionOpen`).
   A later row for one contract appends and is read with no change of any consumer.
3. **The guards stop refusing; the window and its marks stay.** For the snapshot jobs the "wrong day" guard goes (the reconciler passes the key's day), "missed slot" goes, `stop_after` goes (a manifest with it fails to load; "normal or late" is carried by the observed time on the page and by the window-close gap), `freeze_after` applies only to a key whose day is the local date of now (so a late pull of an earlier day is never a no-op), and the before-slot guard builds its slot instant from the key's day, not from now (`quotesnapshot/run.go:105`); `option_open` and `option_open_jp` get that lower bound too, which they lack (`docs/design/job-console.md` lists them listed-only for it). This amends Decision 8 of `option-open-gap-jp.md` ("wrong day ... stays") for the two option_open pulls only. The `option_open` share shortfall (`optionopen/pull.go:259-262`) becomes `Retryable` (today `Permanent` ends the key at once), so the missing share is pulled again in the window and in the late phase. `quote_snapshot`: with "missed slot" gone, the 15:30 run pulls all three keys and stores 15:30 quotes under the slots 0945 and 1230; that is correct data with a true time, and it must never reach a page without the time, so the entry that removes the guard is built after the entries that put the observed time on the page (Issues, order).
4. **Every page that shows a slot's value shows its observed time.** `option_data_page` adds `observed_at` to each quote point (schema `contracts/schemas/page.option_data_page.v1.json`, generated `web/src/generated/option_data_page.v1.ts`, builder `option_data_page/builder.go:236-239`, view `web/src/pages/optiondata/QuoteChart.tsx`, fixtures under `contracts/fixtures/page.option_data_page.v1.*`); `option_boards` already shows the time of the open row (`quote.go:98`). The page shows the time and so the lateness; no "late" flag is computed by a page. Pages that show only closes (`option_eod` values) are outside this decision: the eod row's `received_at` is the close pull and no other row is shown.
5. **Late collection after the close, one plain method.** `runner.Late` (new in `internal/runner/api.go` next to `Windowed`, `Bounded` and `Postcondition`, lines 87-121): `LateCollect() bool`, standard-library type only, so a job package does not import `runner`. A job with it is collected after its window closed, until the market's next open; a job without it is unchanged to the line. The window, its close, `closeRow`, the `expired` or `partial` mark, the gap and the alert are not changed; they are the health mark, at today's time, once. Inside the window the one change is that a `NoValue` result of such a job before its close is not marked: `verdict` (`runkey.go:172-198`) gives it the reason `empty_before_close` and no mark, as an empty answer gets today, so the key keeps being tried each pass until the close; the run after the close ends it as today (`reconcile.go:336`). `deadlineFor` (`runkey.go:253-262`) and the per-run cut are unchanged.
   - **collect-until**: `shared.CollectUntil(ctx, cal clock.Calendar, calendarID string, day contracts.Day) (time.Time, error)` in `internal/runner/shared` (the open of `cal.NextSession(ctx, calendarID, day)`, `clock/api.go:31`), the one function the reconciler uses. The ticker's `windowEnd` (`ticker.go:418-432`) and the restart blocker (`cmd/ot/schedstatus.go:104-115`) keep reading the window close: the restart blocker is not affected, a late run is idempotent and may be cut by a restart.
   - **the late phase**: after the normal walk of a `Reconcile` pass (`reconcile.go:106-121`) `late.go` finds, for each calendar of the jobs that have `Late`, the session days of the last 14 whose window has closed and whose collect-until is after now (on a Monday before the open that is Friday; after the open Friday is out), and takes the keys of that day's stored plan whose dataset belongs to a job with `Late` and whose mark is not `present` (missing or `no_value`, which covers keys closed `expired` or `partial`). It runs at most one such key per pass, oldest first (a pass returns after it), with the reason of the key's close row (`expired` or `partial`) through the same `r.run` that `backfillKey` uses; a usable answer marks the key `present` and the existing gap syncer clears its gap, and any other answer leaves `no_value`. The run keeps the close row's reason on purpose: `keyKind` (`gaps.go:169-182`) lets a verdict run reset `expired` to its own reason, so a fruitless run with a new reason would clear the open gap while the key is still `no_value`; `gaps.go` is not changed. A key is skipped while a run of it is running, when its latest run started less than one hour ago, and when its latest run ended `Permanent` (a fault of the configuration is not time); a crashed run does not count. A late key is started only when now plus its run bound is before the earliest `NextStart` of `store.SchedulerLedger.Status(ctx).Jobs` (the ticker writes them, `ticker.go:409-410`), else the pass logs `late_yield` and ends, so a late key does not hold the worker past a due slot (a nil scheduler ledger, as in the commands, has no yield). A calendar error ends the late phase of that calendar for the pass with a logged reason `late_calendar` and does not fail `Reconcile`. Spacing: the pass cadence is 15 minutes (`ticker.go:31-32`), a key is retried at most hourly, one key per pass, so N late keys take N passes a round. The console and `ot backfill` run any closed key at any time and are not limited by this phase.
   Load: at most one late run per pass, bounded by `budget_minutes` 8 and `chunk` 50 of its manifest, so the gateway sees at most 50 lines at once; IBKR's own limits are from memory [unverified].
6. **Only the option datasets are late.** `LateCollect` is offered by a conditional wrapper like `listedPost` (`cmd/ot/wire_underlying.go:136-165`): a new `listedLate` type is returned by `listedOf`'s builder only when the newest inner job has `LateCollect`, and `listedOf` (172-181) handles it, so bars, dividends, earnings, fundamentals, `ivhistory`, `computefigures` and candidates, which are built by the same listed wrapper, are not `runner.Late`. A test asserts it.
7. **Partial keys.** A present key is final: `option_eod`, `option_eod_jp` and `quote_snapshot` have no completeness check, and `option_open` is present from `min_ok_share` 0.5 of its items. Their missing share is not pulled again by the scheduler; the owner's press pulls it, and nothing shows a partial key except the counts of the run and the observed times (Left out, Doubt 3).
8. **Order against the others.** Entry 5 (runner) edits `internal/runner/api.go`, `reconcile/reconcile.go` and `reconcile/runkey.go` with entries 0 and 3 of `docs/design/job-console.md` (PR #1732): it is built after both. Entry 4 of that design edits `ticker.go`, which this design does not touch: no order. Entry 6 (jobs) edits `optionopen/pull.go`, `jp.go` and their tests with entry 3 of `option-open-gap-jp.md`, and entry 4 (store) edits `openquote/api.go` with entry 1 of it: both after those entries. The console's force rows for `option_eod`, `option_eod_jp` and `quote_snapshot` can flip to yes once Decisions 1 and 2 are on main, a follow-up of that design; `BackfillWith` already runs a closed key (`backfill.go:60-100`), so the owner's press of a late key works.
9. **Extension test, as acceptance.** A new pull time (another entry of `slots` in `quotesnapshot/job.toml`, another `slot` in a `job_pull*.toml`) changes no key, table or job id: entry 6 has the test. A second source that can observe the same instant is not covered: it would need a source column in the key, and it is not built until a second source exists.

## Interface

Interface entries, each changing only its own module's api file: the page schema `contracts/schemas/page.option_data_page.v1.json` (entry 1), `runner.Late` (entry 5), the doc contracts of `openquote.OptionOpenReader` and the store readers (entry 4: `internal/store/openquote/api.go` and `internal/store/api.go`, comments only, `internal/store/api.go` is exactly 800 lines and must not grow), the comment of `jobs.CaptureID` (entry 6, comment only). Contracts: changed (the schema, its fixtures and generated type, and the capture id text of `obs.option_open`); one migration, the view column `in_session`, the next free number on main when the entry is built (`option-open-gap-jp.md` entry 2 takes 0035).

## Issues

Order in one line (own entries): 1 and 4 and 5 first; 2 and 3 after 1; the cross-design lines name the other designs' entries in words: 6 after 4, 5 and entry 3 of `option-open-gap-jp.md`; 7 after 2, 3, 5 and 6; 8 after 5 and 6.

### 1 contracts-observed-at (interface)

Module: contracts. Size: S.
Files: `contracts/schemas/page.option_data_page.v1.json`, `contracts/fixtures/page.option_data_page.v1.ok.json`, `contracts/fixtures/page.option_data_page.v1.choices.json`, `contracts/fixtures/page.option_data_page.v1.gap.json`, `contracts/fixtures/page.option_data_page.v1.jpy.json`, `contracts/fixtures/page.option_data_page.v1.stale.json`, `contracts/fixtures/page.option_data_page.v1.terms.json`, `web/src/generated/option_data_page.v1.ts`, `contracts/datasets/obs.option_open.r1.json`, `docs/contracts/datasets/obs.option_open.md`, `internal/contracts/gen/records.go`
Design: docs/design/late-collection.md#interface
Order: first. Interface entry: it carries the Design line and the Contracts line `Contracts: changed`.
Goal: each point of `quote_series` carries `observed_at` (RFC 3339, UTC); the fixtures carry it; the generated type carries it (generated output with its source); the capture id text of `obs.option_open` says one per pull. Any generated file under `internal/contracts/gen` that carries the description changes with it.
Acceptance:
1. `grep -c "observed_at" contracts/schemas/page.option_data_page.v1.json` prints at least 2 (the property and the `required` list), and `grep -c "observed_at" web/src/generated/option_data_page.v1.ts` prints at least 1.
2. `grep -c "observed_at" contracts/fixtures/page.option_data_page.v1.ok.json` prints at least 1.
3. `grep -c "Deterministic per" contracts/datasets/obs.option_open.r1.json docs/contracts/datasets/obs.option_open.md internal/contracts/gen/records.go` prints `:0` for each of the three files (the generated copy is at `records.go:2424` on main).
Checked by CI: ci-go and ci-web; `go test ./internal/contracts/... -race -count=1` (the fixtures validate against the schema), `tsc`, `eslint`, `vitest`, the build.

### 2 pages-observed-time

Module: pages. Size: S.
Files: `internal/pages/option_data_page/builder.go`, `internal/pages/option_data_page/builder_test.go`
Design: docs/design/late-collection.md#decisions
Order: after 1.
Goal: `quotePoint` gains `ObservedAt` from the row's `ReceivedAt` (UTC); the page shows the time of every point.
Acceptance:
1. `grep -n "observed_at" internal/pages/option_data_page/builder.go` prints the line of the struct tag, and `grep -rhE "^func (TestQuotePointCarriesObservedAt|TestQuotePointObservedAtIsUTC)\(" internal/pages/option_data_page | wc -l` prints 2.
Checked by CI: ci-go; `go test ./internal/pages/... -race -count=1`.

### 3 web-observed-time

Module: web. Size: S.
Files: `web/src/pages/optiondata/QuoteChart.tsx`, `web/test/pages/optiondata/QuoteChart.test.tsx` (new)
Design: docs/design/late-collection.md#decisions
Order: after 1.
Goal: the chart or its table shows `observed_at` for each point as given, with no computation.
Acceptance:
1. `grep -c "observed_at" web/src/pages/optiondata/QuoteChart.tsx` prints at least 1.
Checked by CI: ci-web; `tsc`, `eslint`, `vitest` and the build.

### 4 store-observed-rule (interface)

Module: store. Size: M.
Files: `internal/store/shared/observed.go` (new), `internal/store/shared/observed_test.go` (new), `internal/store/snapshot/optioneod.go`, `internal/store/snapshot/quotesnapshot.go`, `internal/store/snapshot/optioneod_test.go`, `internal/store/snapshot/quotesnapshot_test.go`, `internal/store/snapshot/optioneod_pg_test.go`, `internal/store/snapshot/quotesnapshot_pg_test.go`, `internal/store/optionopenpg/queries.go`, `internal/store/optionopenpg/store_test.go`, `internal/store/optionopenpg/store_pg_test.go`, `internal/store/openquote/api.go`, `internal/store/openquote/api_test.go`, `internal/store/api.go`, `migrations/0036_option_open_in_session.sql` (new; the number is the next free number on main when the entry is built), `docs/contracts/store.md`
Design: docs/design/late-collection.md#decisions
Order: first.
Cross-design order: after entry one of `option-open-gap-jp.md`. Interface entry (`openquote/api.go` and `store/api.go` change their documented behaviour; the interface lands before the entries that use it).
Goal: Decision 2: the fragment in `shared`; the three readers use it with columns and signatures unchanged; the migration adds `in_session` to `rd.option_open_r1` (with its Down); `openquote.OptionOpenReader` and the two store readers say what the rule says (`store/api.go` by editing comment lines without adding any: the file stays 800 lines); `TestOptionOpenRoundTripAndGrants` (`store_pg_test.go:153-181`) expects the earliest live priced row and not "newest of instrument 2".
Acceptance:
1. `grep -rn "DISTINCT ON" internal/store/snapshot internal/store/optionopenpg --include=*.go --exclude=*_test.go` prints nothing and exits 1, and `grep -c "DISTINCT ON" internal/store/shared/observed.go` prints at least 1.
2. `grep -c "in_session" migrations/0036_option_open_in_session.sql` prints at least 2, and `grep -E "^(UPDATE|DELETE|TRUNCATE)" migrations/0036_option_open_in_session.sql` prints nothing and exits 1.
3. `grep -rhE "^func (TestRuleLiveBeatsFrozenNewer|TestRulePricedBeatsEmptyNewer|TestRuleCompleteBeatsLastOnly|TestRuleTieEndsWithCaptureId|TestRuleOpenIsEarliestLivePricedInSession|TestRuleOpenIgnoresRowAfterSession|TestRuleOpenFallsBackToNewestRow|TestRuleGreeksStayWithTheirRow|TestRuleKeepsSlotsApart|TestOptionEodReaderOneRowPerContract|TestQuoteSnapshotReaderOneRowPerSlot|TestOptionOpenReaderEarliestInSession|TestSecondPullAppendsRowSameKeyOtherCapture)\(" internal/store | wc -l` prints 13 (the append and the session tests are `pg` tests, run in GitHub CI only).
Checked by CI: ci-go; `go test ./internal/store/... -race -count=1`; the `pg` tests (the migration up and down, the view, the append) in GitHub CI.

### 5 runner-late-collect (interface)

Module: runner. Size: M.
Files: `internal/runner/api.go`, `internal/runner/api_test.go`, `docs/contracts/runner.md`, `internal/runner/shared/collectuntil.go` (new), `internal/runner/shared/collectuntil_test.go` (new), `internal/runner/reconcile/late.go` (new), `internal/runner/reconcile/late_test.go` (new), `internal/runner/reconcile/reconcile.go`, `internal/runner/reconcile/runkey.go`
Design: docs/design/late-collection.md#interface
Order: first.
Cross-design order: after the entries named zero and three of `job-console.md`. Interface entry: it carries the Design line.
Goal: Decision 5, for a job with `Late`; a job without it behaves as before and its tests pass unchanged; `window.go`, `ticker.go`, `gaps.go`, `backfill.go` and `cmd/ot/schedstatus.go` are not changed.
Acceptance:
1. `grep -cE "^type Late interface" internal/runner/api.go` prints 1, and `grep -cE "^\s+LateCollect\(\) bool" internal/runner/api.go` prints 1.
2. `grep -c "func CollectUntil" internal/runner/shared/collectuntil.go` prints 1, and `grep -c "CollectUntil" internal/runner/reconcile/late.go` prints at least 1.
3. `grep -rhE "^func (TestCollectUntilIsNextSessionOpen|TestCollectUntilAcrossFourDayHoliday|TestLateKeyAfterCloseKeepsCloseReason|TestLateRunsOneKeyPerPass|TestLateSpacingOneHourPerKey|TestLatePermanentIsNotRepeated|TestLateYieldsToDueSlot|TestLateCalendarErrorDoesNotFailReconcile|TestLateStopsAtCollectUntil|TestLateSuccessMarksPresentAndClearsGap|TestLateNoValueBeforeCloseIsNotMarked|TestLateKeyAtCloseGetsExpiredAndOneGap|TestLateFruitlessRunKeepsExpiredAndGap|TestLatePicksFridayOnMondayBeforeOpen|TestJobWithoutLateIsUnchanged)\(" internal/runner | wc -l` prints 15.
Checked by CI: ci-go; `go test ./internal/runner/... -race -count=1` with the fake clock (no test sleeps).

### 6 jobs-late-option-datasets (interface)

Module: jobs. Size: M. Interface entry: it edits `internal/jobs/api.go` (the `CaptureID` comment, lines 41 and 116), its own module's api file, and carries the Design line.
Files: `internal/jobs/api.go`, `internal/jobs/optioneod/map.go`, `internal/jobs/optioneod/run.go`, `internal/jobs/optioneod/manifest.go`, `internal/jobs/optioneod/job.toml`, `internal/jobs/optioneod/job_jp.toml`, `internal/jobs/optioneod/map_test.go`, `internal/jobs/optioneod/types_test.go`, `internal/jobs/optioneod/run_test.go`, `internal/jobs/optioneod/jp_test.go`, `internal/jobs/optionopen/core.go`, `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/jp.go`, `internal/jobs/optionopen/job_pull.toml`, `internal/jobs/optionopen/job_pull_jp.toml`, `internal/jobs/optionopen/core_test.go`, `internal/jobs/optionopen/pull_test.go`, `internal/jobs/optionopen/jp_test.go`, `docs/contracts/jobs/option_eod.md`, `docs/contracts/jobs/option_eod_jp.md`
Design: docs/design/late-collection.md#decisions
Order: after 4, 5.
Cross-design order: after entry three of `option-open-gap-jp.md`.
Goal: Decisions 1 and 3 for `option_eod`, `option_eod_jp`, `option_open` and `option_open_jp`: the capture id carries the start (one line each); the three guards and `stop_after` are gone; `freeze_after` applies to the key's own day only; `option_open` and its JP pull do not quote before their slot and a share shortfall is `Retryable`; each job has `LateCollect() bool` returning true; the comment of `jobs.CaptureID` says one per pull; `optioneod/types_test.go:57` (`captureOf`) and the tests that asserted the stable id change to the new rule.
Acceptance:
1. `git grep -n "stop_after" -- internal/jobs/optioneod/job.toml internal/jobs/optioneod/job_jp.toml internal/jobs/optionopen/job_pull.toml internal/jobs/optionopen/job_pull_jp.toml` prints nothing and exits 1.
2. `git grep -n "after session\|wrong day\|not started" -- internal/jobs/optioneod/run.go internal/jobs/optionopen/pull.go internal/jobs/optionopen/jp.go ':!*_test.go'` prints nothing and exits 1.
3. `git grep -nE "^[^/]*uuid\.NewSHA1\(" -- internal/jobs/optioneod/map.go internal/jobs/optionopen/core.go ':!*_test.go'` prints two lines, each containing `RFC3339Nano` (the comment at `map.go:57` is not a code line). `plan.go` is not in the Files: the plan id hash at `plan.go:252` does not change.
4. `git grep -n "LateCollect() bool" -- internal/jobs/optioneod internal/jobs/optionopen ':!*_test.go'` prints at least two lines (one in each package).
5. `grep -rhE "^func (TestCaptureIDPerPull|TestOptionOpenCaptureIDPerPull|TestSecondPullOfKeyAppendsRows|TestRunOnKeyDayAfterMidnightLocal|TestFreezeAfterAppliesOnlyToTodaysKey|TestOptionOpenBeforeSlotStoresNothing|TestOptionOpenShortfallIsRetryable|TestStopAfterInManifestIsRejected|TestNewSlotChangesNoKeyTableOrJobID)\(" internal/jobs/optioneod internal/jobs/optionopen | wc -l` prints 9 (entry 7 adds its own `TestCaptureIDPerPull` in another package).
Checked by CI: ci-go; `go test ./internal/jobs/... -race -count=1`.

### 7 jobs-quotesnapshot-late

Module: jobs. Size: S.
Files: `internal/jobs/quotesnapshot/run.go`, `internal/jobs/quotesnapshot/manifest.go`, `internal/jobs/quotesnapshot/job.toml`, `internal/jobs/quotesnapshot/run_test.go`, `internal/jobs/quotesnapshot/types_test.go`, `internal/jobs/quotesnapshot/schedule_test.go`
Design: docs/design/late-collection.md#decisions
Order: after 2, 3, 5, 6 (the observed time is on the page). A late quote under an earlier slot label with no time must not be deployed, not even for one release.
Goal: Decision 3 for `quote_snapshot`: the capture id carries the start, "wrong day" and "missed slot" are gone, `grace_minutes` is removed (a manifest with it fails to load), the before-slot guard builds the slot from the key's day, `LateCollect() bool` returns true.
Acceptance:
1. `git grep -n "missed slot\|wrong day\|grace" -- internal/jobs/quotesnapshot/run.go internal/jobs/quotesnapshot/job.toml` prints nothing and exits 1.
2. `git grep -nE "^[^/]*uuid\.NewSHA1\(" -- internal/jobs/quotesnapshot/run.go` prints one line containing `RFC3339Nano`, and `git grep -c "LateCollect() bool" -- internal/jobs/quotesnapshot/run.go` prints `internal/jobs/quotesnapshot/run.go:1`.
3. `grep -rhE "^func (TestCaptureIDPerPull|TestQuoteSnapshotLateSlotRuns|TestQuoteSnapshotBeforeSlotUsesKeyDay|TestGraceInManifestIsRejected)\(" internal/jobs/quotesnapshot | wc -l` prints 4.
Checked by CI: ci-go; `go test ./internal/jobs/quotesnapshot -race -count=1`.

### 8 ot-late-wrapper

Module: ot. Size: S.
Files: `cmd/ot/wire_underlying.go`, `cmd/ot/listedjob.go`, `cmd/ot/listedjob_test.go`, `cmd/ot/dryrun.go`, `cmd/ot/dryrun_test.go`, `docs/contracts/ot.md`
Design: docs/design/late-collection.md#decisions
Order: after 5 and 6.
Goal: Decision 6: `listedLate` offered only for an inner job with `LateCollect`, `listedOf` handles it; the dry run's words follow the removal of `stop_after` (`dryrun.go:29`, `dryrun_test.go:140`, `docs/contracts/ot.md:10`).
Acceptance:
1. `grep -c "listedLate" cmd/ot/wire_underlying.go` prints at least 2, and `grep -rhE "^func (TestListedLateOnlyForLateJobs|TestListedBarsJobIsNotLate|TestListedLateKeepsWindowAndPostcondition)\(" cmd/ot | wc -l` prints 3.
2. `grep -n "stop_after" cmd/ot/dryrun.go cmd/ot/dryrun_test.go docs/contracts/ot.md` prints nothing and exits 1.
Checked by CI: ci-go; `go test ./cmd/ot -race -count=1`.

## Checked by CI

ci-go runs `go test ./internal/runner/... ./internal/store/... ./internal/jobs/... ./internal/pages/... ./internal/contracts/... ./cmd/ot -race -count=1`; ci-web runs `tsc`, `eslint`, `vitest` and the build for entries 1 and 3; the `pg` tests (the migration, the append, the session rule) run in GitHub CI only. No test sleeps (fake clock) and none connects to a broker.

## Left out

- A primary-key change, views other than the `in_session` column, a `source` column, a calendar-based day guard, a retry count, new reasons, a new gap kind and a notify change: earlier revisions had them.
- A widened window: the window and its close are unchanged; the collect-until time lives only in the late phase.
- A "late" flag computed by a page: the pages show the observed time (Decision 4).
- Repair of a present partial key (Decision 7) and a page that lists partial `option_eod` or `quote_snapshot` keys: the today section of `option-open-gap-jp.md` lists only the datasets of that design.
- Jobs other than the option datasets; a global cap of pulls per day for IBKR's lines.
- The console's force rows (Decision 8): a follow-up of `job-console.md`.

## Doubts

1. Whether the gateway answers `market_data_type` 1 for option ticks after the close is unverified (`pyside/src/optdesk/ibkr/spec.py` not read in full); if it does, rank 1 of the reader separates less, and the earliest-in-session rule for the open does not depend on it.
2. Whether the auto-deploy consults `ot sched status` for every merge is unverified; the restart blocker reads the window close, which does not move.
3. A present partial key is final and nothing lists it but the observed times; the owner may want a page for it (a follow-up).
4. The growth of unusable rows by late pulls (Decision 1) is estimated, not measured.
5. A calendar with no session within 14 days back logs `late_calendar` and no late run happens for it; the arch check of `internal/store/shared` was not run [unverified].

## Adversarial

Round 3 answers the advisor's review r2 of revision 3 and the dispatcher's two additions. The advisor's r2 file is `late-collection-review-r2.md` and the drafting round's own check is `designs/late-collection-adversarial-r2.md`. Adopted unless stated:

- B1 (key unmarked at the end): solved by not widening: the window, the run after the close and `closeRow` are as today (Decision 5); test `TestLateKeyAtCloseGetsExpiredAndOneGap`, and a fruitless late run keeps the close reason so the gap stays (`TestLateFruitlessRunKeepsExpiredAndGap`).
- B2 (restart blocker): unaffected, said in Decision 5, `windowEnd` and `schedstatus.go` untouched, checked by entry 5 item 3.
- B3 (every listed job forwarded): adopted, conditional `listedLate` (Decision 6, entry 8, test `TestListedBarsJobIsNotLate`).
- B4 (a pass holds the worker): adopted, one late key per pass, hourly per key, yield to the earliest `NextStart` (Decision 5, tests `TestLateRunsOneKeyPerPass`, `TestLateYieldsToDueSlot`).
- B5 (false slot label of `quote_snapshot`): adopted, the observed time reaches the page first (Decision 4, entries 1 to 3) and entry 7 is built after them.
- B6 (capture id contract text): adopted, entry 1 carries `contracts/datasets/obs.option_open.r1.json` and its doc, `Contracts: changed`.
- B7 (reader contracts and tests): adopted, entry 4 is an interface entry with `openquote/api.go`, `store/api.go` (not growing), their tests and `store_pg_test.go`.
- B8 (rank defects): adopted, the open is the earliest live priced row in session, the rank is split by dataset, greeks stay with their row, every rule ends `received_at DESC, capture_id DESC` (Decision 2).
- B9 (`windowFor` signature, `NextSession`): adopted by removal: `windowFor` is unchanged, `CollectUntil` uses `NextSession` and is in `internal/runner/shared`, errors skip the late phase (Decision 5).
- Dispatcher decision 5 to 9 points: `option_open` is collected late and the reader's session rule decides the open (Decisions 2 and 3, with `in_session`); `freeze_after` fixed (Decision 3, `TestFreezeAfterAppliesOnlyToTodaysKey`); no_data growth stated (Decision 1, Doubt 4); extension test is a named test (entry 6 item 5); the stale claims about the gap-jp runs page and `job-console.md:82` are gone (Decision 7, Decision 8); order against job-console entry 4 and gap-jp entries 1 and 3 is in Decision 8; vacuous items dropped; the header sha is 51654033.
- Dispatcher addition (1): entry 7 (the removal of the missed-slot guard) is after the entries that put the observed time on the page. Addition (2): "newest" applies to the day's best row only and a row after the session is stored and shown with its time, never as the open (Decision 2).
- Not adopted: a "late" mark computed by a page (Decision 4): the time is shown and the owner reads it.
