# Option open, plan B: contracts computed, confirmed by listing, cached valid and invalid for seven days, listed on the spot when the cache lacks them; US and Japan

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 3 (revision 1 was a JP gap stage with a Postcondition; revision 2 the owner's shape with a second job family; this revision builds the one list job of `option-contract-lists.md` with the cache and the band, adds the JP manifest of the same job, the silence at the empty plan, the pull re-check and the runs page; it amends `option-contract-lists.md` in the same pull request)

Owner decision "b", 2026-11-12 00:33Z: invalid contracts are cached for 7 days and re-checked early when the spot leaves the band they were computed for (Decisions 5 and 13).
Source: owner ask optdesk-2940, 2026-11-12: "when the stored data is bad, collect on the spot; no one-wrong-step-then-all-wrong". Owner shape, same day: compute the candidate contracts; before the open confirm with IBKR which are valid and which are not; store both with a validity of 7 days; after 7 days check again, and a contract still invalid stays cached as invalid; collect only the valid ones; a contract the cache lacks or whose validity ended is checked on the spot; IBKR's real limit is the concurrent market-data lines, not a daily cap. The owner works on the web page, so what must be seen is on a page. Advisor reviews r1 and r2 (PR 1750), option-trader advisor (prior art, Doubt 14).
Read on origin/main at 488e8aac: `AGENTS.md`, `docs/design/modules.md`, `docs/design/option-contract-lists.md` (Decisions 4, 6, 9, 10; entries 1 to 6), `migrations/0034_option_list.sql`, `internal/jobs/optionopen/{listed,plan,pull,jp,core}.go` and the tomls, `internal/store/openquote/api.go`, `internal/runner/reconcile/{reconcile,runkey,window}.go`, `internal/runner/sched/gaps.go`, `pyside/src/optdesk/ibkr/pacing.py`, `cmd/ot/wire_optionopen.go`. "[unverified]" marks a claim not checked in code, docs or a probe. No absolute path of the author's machine is in this file.

Goal: for a watched name of either market, on a session day, the plan and the pull read contracts that a gateway answer has confirmed (valid) or refused (invalid, remembered for 7 days); a group with no fresh answer is listed before the plan; a run that cannot do its work is never a silent no_value and is on a page the same morning; the pull re-lists an expiry when the open spot has moved the target zone out of the confirmed range. One list job serves both markets.

## What main holds today, against the owner's model

| Owner step | On main |
|---|---|
| compute the candidates | done: `candidates` and `ChooseExpiries` (`listed.go`, `plan.go`, `band.go`), US and JP (`jp.go`) |
| confirm with IBKR before the open | US: the evening `option_open_resolve` (per contract) and the 08:00 ET `option_open_resolve_gap`. JP: the evening `option_open_resolve_jp` only. Listing by expiry is designed in `option-contract-lists.md`: its rpc is merged (entry 1, #1751), the store interface and the tables (entries 3 and 4, migration 0034) are on main; the broker method (entry 2), the job (entry 5) and its wiring (entry 6) are not built |
| store valid and invalid, 7 days | valid: `master.option_contract`, no end date. Invalid: `master.option_not_listed` marks, 7 plus or minus 2 days (`retry_days`, `jitter_days`), written for an answered "not listed" only |
| re-check after 7 days, still invalid stays invalid | the mark's `retry_at` per contract; no band, no early re-check |
| collect only the valid ones | done: the plan reads `ListOptions` (`plan.go:205`) |
| check what is missing on the spot | none before the open for a name with no stored contract; the plan is a no_value or a Retryable, the pull has no key when the plan is empty (Decision 10) |

So about four of the six steps exist for the US. The delta of this design: the cache (7 days, band, early re-check) in the one list job, JP as a second manifest of that job, the silence at the empty plan, the pull re-check, and a page of today's failures.

## Decisions

### The model, stated once

1. Each stage reads its stored input; the list job is the only stage that asks the broker for what the stored input lacks, and it runs before the plan; valid and invalid are both stored and collection uses the valid ones only. There is no second job family: the job `option_list_preopen` of `option-contract-lists.md` (Decision 4) is built with the freshness and the band of this design (this pull request amends its Decision 4, its "real list" rule and its entry 5), and the JP side is a second manifest of the same job code. The jobs `option_list_gap`, `option_list_gap_jp` and the keys `gap_only` and `unserved_retryable` of revision 2 are gone: a list key that is not complete is `Retryable` before its window close, so it is run again at every turn and every group that is not fresh is listed then (Decision 6).

### Confirm by listing, not per contract

2. One `ListContracts` per (underlying, expiry, class) returns every listed strike (`option-contract-lists.md` Decision 1). Validity is membership in that answer: a strike in it is valid (stored in `master.option_contract`); a computed candidate of a listed expiry that is not in it is invalid. The count of requests is the count of groups, never of contracts.
3. The negative cache is the group, not a row per strike: the plan reads valid contracts only, so an absent strike is never planned and never asked. Only a gateway answer makes a group fresh: a timeout, no answer, a disconnect, `ErrNotServed` and a `short` list leave it unfresh, so nothing is cached invalid from them (the test the per-contract path has, `classify`, `listed.go:153-176`). An answer that is real but empty for an expiry that the chain lists (the gateway answered, nothing is listed) is stored as outcome `listed` with `Count` 0 and is fresh for the seven days like any other group: it is not a daily gap and not a daily re-ask. This amends the "real list" rule of `option-contract-lists.md` Decision 6, which made an empty answer `short`; the risk, a transient empty answer cached for seven days, is Doubt 6, and the early re-check of Decision 5 is its only guard. The per-contract marks of `master.option_not_listed` stay for the per-contract path and are not written by the list job.
4. Freshness, seven days, no jitter (the owner said 7; the plus or minus 2 days of option-trader are prior art, Doubt 14). A group is fresh when `master.option_list` holds a row with outcome `listed` whose `listed_session` is on or after the day `D - (list_ttl_days - 1)` (calendar days): `list_ttl_days = 1` is the daily behaviour of `option-contract-lists.md` (its default) and 7 means D-6 to D. The `listed` recount and the end reasons of that job count a fresh group as listed, so a group listed on D-3 is not `not_finished` on D. The valid cache has no seven-day end (a listed contract cannot stop being valid before its expiry); the invalid cache ends with the freshness of its group. All groups listed on one day fall stale on the same morning seven days later, so the load of every eighth day is the first-day load (Decision 19), bounded by the budget and the stop time; a long weekend adds nothing, the days are calendar days.
5. Early re-check when the spot leaves the band (owner decision "b"): a fresh group is listed again when the target zone computed from the newest spot is no longer inside the range the group was confirmed for (Decisions 12 and 13). The check runs in the list job at each pass (the newest stored close) and in the pull (the open spot, Decision 13, entry 5). A group with no stored range (listed before the columns) or a name with no spot is treated as stale.
6. On the spot, from the same job: the list key is `Retryable` while any group of any name is neither fresh nor answered, and the runner runs a `Retryable` key before its window close at every 15-minute turn and every slot instant (`reconcile.go:349`, `ticker.go`), so a group that failed at 06:15 is asked again at the next turn, in the order of need (held names, priority names, then the rest, nearest expiry first: the order of `option-contract-lists.md` Decision 4), and a group already fresh costs no broker call. The dataset window of the list job closes early (Decision 11), so a key still unfinished at the close ends `expired` and is a gap the same morning. A name added to the watch list after the key is present is listed by nothing before the plan (Doubt 5).
7. JP: the same job code, a JP manifest (`list_jp.go`, `job_list_preopen_jp.toml`): the JP rule set `jpRules` (`jp.go`: exchange OSE.JPN, multiplier 100, class = stock code, `N225` as the index of class 225, names without options excluded), the JP expiry window (DTE 14 to 90), calendar XTKS, zone Asia/Tokyo, dataset `master.option_list_jp`, subject `preopen`, start 06:30 JST, `stop_time` 07:00 JST. JP names are never SMART and never guessed: class, exchange and multiplier come from the chain definition. Whether the pyside `ListContracts` serves OSE names is unverified (Doubt 1).

### Silence: a missing input is never a silent no_value

8. The rule, for list, resolve, plan and pull, US and JP: a key ends `NoValue` only when a gateway answer proves the market has nothing; a missing stored input is never such an answer. The cases on main: "no stored close" (`listed.go:328`, `plan.go:202`), "no resolved chain" (`plan.go:209`) and "not on the stock list" (`plan.go:155`) end `NoValue` today and become `Retryable`; "wrong day" (`pull.go:218`, `jp.go:257`) is not a data fault and stays; "no con_id" is already `Retryable` (`listed.go:316`, #1746; the JP con_id fix has landed, #1745 and #1749, and the design holds with or without it). A `Retryable` run before the window close is not counted (`reconcile.go:349`), is run again at every turn and writes one line `fault stage=<id> day=<day> key=<key> reason=<text>` through the job's `Log`.
9. A clean result with `Usable` 0 is already `empty_before_close` and retried (`runkey.go:189-193`), so no `Postcondition` method is added to the existing jobs: a false `Postcondition` is a counted run that is never re-run (`runkey.go:182-188`, `reconcile.go:349-357`). The list job of `option-contract-lists.md` has its own and keeps it.
10. The silence where it is born. The plan job (`plan.go:120-175`) ends a run in which watched names exist and no item was stored with `NoValue` or no error today, and the pull `Plan` (`pull.go:176-186`) returns no keys and no error when the day's plan is empty: no expected key, no run, no gap, nothing on any page (the pull of 2026-11-11). A `Plan` error would not help: `planLogged` (`reconcile.go:149-165`) skips the job for the pass with one Warn line that holds the job id, the day and the kind, and nothing else sees it. So: (a) the plan `Run` ends `Retryable` with the text `plan stored no item: <first reason>` when its keys exist and `Returned` is 0; (b) the pull `Plan` returns one key (`obs.option_open`, subject `plan-empty`) when the plan of the day is empty; the key is held until the pull's own slot like any key (`notDue`), so it is due after the plan's slot; its `Run` reads the plan again and, if it is still empty, ends `Retryable` with `no plan stored for the day`; when a plan exists the next pass returns the batches and the key is dropped from the expected set. The `Retryable` run is stored with its error code in `ops.capture_run` (`runkey.go` verdict), is seen on the page of Decision 11 the same morning, and ends `expired` at the close of the pull dataset, a gap on Telegram and `ot check scheduler`.

### Same-morning visibility

11. What is seen today: `sched/gaps.go` `keyKind` gives a missing key a gap only after the window close of its dataset (`!closed || running` gives none, except the crash brake) and a `no_value` key none unless the runner wrote `expired`; the readers of open gaps are `ot check scheduler`, the Telegram notifier, the morning notice and the freshness read; no page lists the gaps of the day (`pages/gaps.Lines` has only test callers), and `ops.capture_run` has no `rd` view and no reader (the views over `ops` on main are `rd.job_gap_r1` and `rd.expectation_r1`). The owner works on the web page, so this design adds one page, `ops_today_page`, with one section "today": the open gaps of the day (`GapReader.OpenGaps` through `pages/gaps.Lines`) and the runs of the day that erred or stored nothing (a new view `rd.capture_run_r1` over `ops.capture_run`, in the migration of entry 2; the view computes nothing and has no filter, the builder filters). Entries 6 to 9 are the page: schema, builder, web view, wiring. For the list datasets the window closes early so that a key still `Retryable` is an `expired` gap the same morning: the job's `Window()` third value is a negative close offset, `-8h30m` of the session close (US 16:00 gives 07:30 ET, the `stop_time`; JP 15:30 gives 07:00 JST, the JP `stop_time`); the readers of the window use `CloseAt` only (`reconcile.go:302`, `gaps.go:139`, `runkey.go` deadline `max(CloseAt, start) + RunBound`), `OpenAt` is read by no runner path and `rd.expectation_r1` keeps its `closed` test on null times, so a close before the open misleads no reader on main [unverified for readers outside `internal` and `cmd`].

### The band, the windows and the strikes

12. The band, per expiry, no DTE buckets: strikes K from `spot*exp(-band_k*s)` to `spot*exp(+band_k*s)`, `s = sigma*sqrt(DTE/365)`, `band_k = 1.5` (manifest). Delta (US 0.15 to 0.30, JP 0.10 to 0.35) is the pull's filter after the quotes and never builds the band. Floors, in the manifest: the band reaches at least `band_floor_pct = 0.10` of spot on each side (lo at most `0.90*spot`, hi at least `1.10*spot`) and holds at least `min_strikes_side = 3` listed strikes on each side. `ListBand` is a new function of the list job beside `Band` and `StoredBand` (`planfunc.go:161-171`); the per-contract resolve keeps `StoredBand` unchanged. There is no upper cap of the 25 percent kind: a listing is one request whatever the band, and the pull's delta filter bounds what is collected (Doubt 11).
13. The zone and the early re-check. The target zone of an expiry from a spot S' is, per side, `S'*exp(-1.05*s)` to `S'*exp(-0.5*s)` below and `S'*exp(+0.5*s)` to `S'*exp(+1.05*s)` above (`zone_near = 0.5`, `zone_far = 1.05`). When the zone is not inside the confirmed range `[BandLo, BandHi]` of that expiry (stored with the `Sigma` of the band), that expiry is listed again, one listing. The room is `band_k - zone_far = 0.45` of `s`: at sigma 0.30 and 30 DTE about 4 percent [computed: 1.5 s = 0.129, 1.05 s = 0.090]. In the list job it runs at each pass on the newest stored close. In the pull it runs in `Core.stock` (`core.go:174-185`) on the open spot, the first live spot after an overnight move: before the quotes of an expiry whose zone left the range, the core lists that expiry once (`ListContracts`, one in flight, counted against the pull's own `budget_minutes`), and the contracts listed inside the zone that the stored plan does not hold are written as plan items (the `PlanWriter` port that the plan job has, `plan.go:41`, now also given to the core) and added to the targets of that expiry, so a strike that enters the band after a gap is collected the same open. `core.go` is a file of the jobs module (`internal/jobs/**`).
14. Sigma, one rule for both markets: the larger of the stored ATM implied vol (`Stored.sigmas`, `inputs.go:150`: the implied vol of the stored contract nearest the spot on the newest `option_eod` day) and the 20-day realised vol of the stored daily closes. On a first listing a name has no stored contract, so `Stored.sigmas` returns 0 and 0 (`inputs.go:153-155`): the IV leg is absent exactly when the band is first needed, so a first band uses the realised vol of the stored closes (21 closes of `price_bar`, one small function behind the same reader; main stores `market.rv20` for SPX, NDX and RUT only, #1677 and #1679; JP close coverage is unverified), and without it `DefaultSigma` 0.30 (`planfunc.go:33`) with the source `default` stored (`SigmaSource`: `iv`, `rv20`, `default`). Never a live IV before the open. Then the 10 percent and 3-strike floors set a first band that does not depend on the sigma being right (Doubt 13).
15. Spot before the open: the prior close from the stored bar, US and JP alike. Order: freshness first, then the bar only for a name that has a stale group or whose range is to be tested; a missing bar is asked of the gateway once (`HistoryClient`, `internal/broker/api.go:250`, the last daily bar, one request) and, if that fails too, the key is `Retryable`, never `NoValue`.
16. No strike is generated from an assumed grid: one `ListContracts` per (underlying, expiry, class) returns the real grid, whatever its step (2.5 or 0.5 near the money on a short-dated expiry, so 497.5 is simply in the answer; coarse on LEAPS; 16.67 alternating 17 and 16 on one JP name, `option-capture-jp-stocks.md`). The candidates of an expiry are the listed strikes inside the band plus, when fewer than `min_strikes_side` lie on a side, the nearest 3 listed strikes of that side. The group row stores `BandLo`, `BandHi`, `Sigma`, `SigmaSource` and `InBand`; `InBand` 0 is the negative entry "expiry E had no strike in the band on day D". One listing is in flight at a time (`ResolveInFlight: 1`) because option-trader overloaded the gateway with open-strike listings at 148 expiries a day (#776).
17. The windows are a list because the owner will want others: the list job's DTE windows are a manifest list `[[window_dte]]` of `min_dte` and `max_dte`, today one entry per market (US 21 to 45, the window of `option-contract-lists.md` Decision 8; JP 14 to 90); a two-week window (7 to 14) or LEAPS (180 to 400) is a manifest edit, and Decisions 12 to 16 hold for each (`sqrt(DTE/365)` narrows a short band and widens a LEAPS band, the floors keep a short band from collapsing). The plan, the pull and the resolve read the stored contracts by their own `min_dte` and `max_dte`; widening them is the entry that follows the owner's choice of windows (Doubt 10).

### Load: IBKR's limits

18. There is no daily cap and no per-name count of our own. The limits, from the code that enforces them and from IBKR's documented numbers:
   - Message rate: the TWS API allows 50 messages per second by default [unverified: IBKR documentation, from memory]; `MessagePacer` (`pyside/src/optdesk/ibkr/pacing.py`) holds 40 starts per second for every message, a cancel counts. It binds everything on the one connection and is far above a list run.
   - Contract-detail style requests (`ListChains`, `ListContracts`, `ResolveContract`, bars) go through `DetailsLane`: in flight 2 at the start, at most 5 when ten answers in a row average under 1 s, halved when ten answers average over 3 s; a timeout sets the limit to 1 and pauses 30 s; a 20 s timeout; more than 2 unresolved block new starts; no new start while the oldest request in flight is older than 5 s (`_OLDEST_MAX_S`). The Go side dials these jobs with `ResolveInFlight: 1` (`wire_optionopen.go`) and `CallTimeout` 25 s. A probe gave 5.0 s per list request [unverified]; `option-contract-lists.md` Decision 9 uses 7.3 s.
   - Concurrent market-data lines: 100 by default per account [unverified]; `LineBudget` holds 70 lines and 50 snapshots in flight and shrinks on error 101. This bounds the pull and the refresh, not the lookups.
19. The counts and the worst-case times, one listing in flight. US, today's window, 45 names (the figure of `option-contract-lists.md` Decision 9: 12 held, 5 daily-expiry names, 36.5 minutes at 7.3 s, so 25.0 minutes at 5.0 s [computed: 36.5 x 5 / 7.3]): the first day, and every eighth day when all groups fall stale together, runs from 06:15 ET and ends by about 06:40 at 5 s or 06:52 at 7.3 s; its budget is 45 minutes and its `stop_time` and window close 07:30 ET, so the run is inside 06:15 to 07:30 with the 07:30 `fred_series` and the 08:00 chain untouched (the slack of Decision 10 of that design). A day with ordinary traffic lists the groups of the new expiries and the groups whose zone left the range. JP: seven names with options (`option-capture-jp-stocks.md`, four expiries each on the day it probed, two near months then quarters), the window 14 to 90 DTE holds about 2: 14 groups, 28 at most, 70 to 140 s; it starts 06:30 JST with `stop_time` 07:00 JST; in summer 06:30 JST is 17:30 ET, inside the US evening resolve (17:00 to about 17:41 ET), so it starts when the worker frees (about 06:41 JST) and 19 minutes remain, enough for 228 listings at 5 s. The windows not built (SPX, NDX and RUT with daily expiries and a second class each, about 50 groups per index in 21 to 45 DTE; 16 in 7 to 14; about 16 in 180 to 400 [assumption: expiry calendars from memory, class names SPXW, NDXP, RUTW unverified]) add about 82 listings per index, 410 s each, so three indices add 20 minutes to a first day and a first day of all three windows would be near 45 minutes: the owner's later choice, Doubt 10. The pull re-check adds 5 s per relisted expiry inside the pull's `budget_minutes` of 8: on a gap day that moves every name, 45 names by 2 expiries is 90 listings, 450 s, nearly the whole budget, and the quotes of the names after it are deferred with `BUDGET` as they are today when the budget ends; JP 7 names by 2 is 14 listings, 70 s. Typical days relist a few.

### What does not change

20. This pull request changes two files under `docs/design/`: this one and `option-contract-lists.md` (Decision 4, the "real list" rule, entry 5 and its Order; the pinned blob ids of `listed.go`, which entry 3 here edits, become a check on the diff). `docs/design/modules.md` is not edited: the new files sit in the rows jobs, store, contracts, pages, web and ot. `Contracts: unchanged` for this pull request; the issue of entry 6 adds a file under `contracts/schemas` and carries `Contracts: changed`. `option_list_run` is not written by this design (its CHECK of reasons holds no new one and `PutListRuns` overwrites on conflict, `lists.go:33-40`); no reason is added. Not edited: `internal/jobs/api.go`.

## Interface

Two interface changes, both store: `internal/store/openquote/api.go` (`ListRecord` gains `BandLo`, `BandHi`, `Sigma`, `SigmaSource`, `InBand`) and `internal/store/api.go` (`RunRow` and `RunReader`), entry 1; and the page schema of entry 6 (contracts). Every other entry works through them or through none: the JP constructor has the type of `NewResolveJP`, the manifest keys are values of the package `optionopen`. The entries that depend on `option-contract-lists.md` use its interfaces as they land.

## Issues

Order in one line: entries 1, 3 and 6 start together; 2 after 1; 4 after 3, 1, 2 and entry 5 of `option-contract-lists.md`; 5 after 3 and 4; 7 after 1 and 6; 8 after 6; 9 after 2 and 7; 10 after 4 and after `ot setup pg` and migrate.

### 1 store-list-api (interface)

Module: store. Size: S.
Files: `internal/store/openquote/api.go`, `internal/store/openquote/api_test.go`, `internal/store/api.go`, `internal/store/api_test.go`, `docs/contracts/store.md`
Design: docs/design/option-open-gap-jp.md#interface
Order: first; before 2, 7 and 9. Interface entry: it carries the Design line.
Goal: `ListRecord` gains `BandLo` and `BandHi` (decimal text: the confirmed range, Decision 12; empty means none stored), `Sigma` (decimal text: the sigma the band was built with, so the zone test needs no second source), `SigmaSource` (`iv`, `rv20`, `default`) and `InBand` (int: listed strikes inside the band, 0 is the negative entry of Decision 16); `store.RunRow` (session day, dataset, key, status, finished at, returned and usable counts, error code, reason code) and `store.RunReader` with `RunsOn(ctx, day)`, rows ascending by finished at and never nil; `docs/contracts/store.md` states them. Tests of the contract from both sides with fakes.
Acceptance:
1. `awk '/^type ListRecord struct/,/^}/' internal/store/openquote/api.go | grep -c "BandLo\|BandHi\|Sigma\b\|SigmaSource\|InBand"` prints 5.
2. `grep -c "RunsOn\|type RunRow" internal/store/api.go` prints 2, and `grep -c "BandLo\|RunRow" docs/contracts/store.md` prints at least 2.
3. `git diff $(git merge-base HEAD origin/main) -- internal/store/openquote/api.go internal/store/api.go | grep -c "^-[^-]"` prints 0 (only additions).
4. `grep -rhE "^func (TestListRecordBandRoundTrip|TestListRecordBandEmptyIsNone|TestRunReaderContractBothSides|TestRunsOnNeverNil|TestRunsOnAscending)\(" internal/store | wc -l` prints 5
5. `git diff --name-only origin/main` lists no path outside the Files line.
6. The result matches docs/design/option-open-gap-jp.md#interface.
Checked by CI: ci-go; `go test ./internal/store/... -race -count=1`.

### 2 store-list-impl

Module: store. Size: S.
Files: `migrations/<next>_option_list_band_run_view.sql` (new), `internal/store/optionopenpg/lists.go`, `internal/store/optionopenpg/lists_test.go`, `internal/store/optionopenpg/lists_pg_test.go`, `internal/store/ops/runs.go` (new), `internal/store/ops/runs_test.go` (new), `internal/store/ops/runs_pg_test.go` (new)
Design: docs/design/option-open-gap-jp.md#interface
Order: after 1. The migration number is the next free number on main when the entry is built, not when this design merges; whichever of two merging migrations lands second renumbers, so no check here names a number. The window runs `setup pg`, migrate and `setup pg` after the merge; entry 10 merges after that.
Goal: one migration: nullable `band_lo numeric`, `band_hi numeric`, `sigma numeric`, `in_band integer` and `sigma_source text` on `master.option_list` (check: `iv`, `rv20` or `default`; `band_lo <= band_hi` when both are set; no grant change) and the view `rd.capture_run_r1` over the columns session_day, dataset_id, subject_key, status, finished_at, returned_n, usable_n, error_code and reason_code of `ops.capture_run` (no filter, no arithmetic), granted `SELECT` to `ot_ro_pages` and `ot_runner` with `USAGE` on `rd` as `rd.option_eod_r1` is; `PutLists` writes the columns and `Lists` reads them; a `short` or `error` outcome keeps the stored band as it keeps `listed_at`; `ops/runs.go` implements `RunReader` over the view.
Acceptance:
1. `f=$(git diff --name-only origin/main -- migrations); echo "$f" | wc -l` prints 1, and `grep -c "ADD COLUMN band_lo\|ADD COLUMN band_hi\|ADD COLUMN sigma \|ADD COLUMN in_band\|ADD COLUMN sigma_source" $f` prints 5 and `grep -c "DROP COLUMN" $f` prints 5 (the Down).
2. `f=$(git diff --name-only origin/main -- migrations); grep -c "CREATE VIEW rd.capture_run_r1" $f` prints 1, `grep -c "GRANT SELECT ON rd.capture_run_r1 TO ot_ro_pages" $f` prints 1 and the lines of the view hold no `WHERE`, `JOIN` or aggregate: `awk '/CREATE VIEW rd.capture_run_r1/,/;/' $f | grep -c "WHERE\|JOIN\|count(\|sum("` prints 0.
3. `grep -c "band_lo\|band_hi\|in_band\|sigma_source" internal/store/optionopenpg/lists.go` prints at least 8.
4. `grep -rhE "^func (TestPutListsWritesBand|TestListsReadsBand|TestShortOutcomeKeepsBand|TestRunsOnReadsView)\(" internal/store | wc -l` prints 4, and `head -1 internal/store/optionopenpg/lists_pg_test.go` is `//go:build pg`.
5. `git diff --name-only origin/main` lists no path outside the Files line.
6. The result matches docs/design/option-open-gap-jp.md#interface.
Checked by CI: ci-go; `go test ./internal/store/... -race -count=1`; the `pg` tests run only in GitHub CI.

### 3 optionopen-no-silent-no-value

Module: jobs. Size: M.
Files: `internal/jobs/optionopen/listed.go`, `internal/jobs/optionopen/listed_test.go`, `internal/jobs/optionopen/plan.go`, `internal/jobs/optionopen/plan_test.go`, `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/pull_test.go`, `internal/jobs/optionopen/jp.go`, `internal/jobs/optionopen/jp_test.go`, `internal/jobs/optionopen/fakes_test.go`
Design: docs/design/option-open-gap-jp.md#silence-a-missing-input-is-never-a-silent-no_value
Order: independent of entries 1 and 2; entry 4 and entry 5 land after it (they edit `fakes_test.go`, `pull.go` and `jp.go` too). It edits `listed.go`, which entry 5 of `option-contract-lists.md` pins by blob id; that design is amended in this pull request so the pin is a check on the diff.
Goal: Decisions 8, 9 and 10: "no stored close" (`listed.go:328`, `plan.go:202`), "no resolved chain" (`plan.go:209`) and "not on the stock list" (`plan.go:155`) are `Retryable` with the fault line; the plan `Run` ends `Retryable` with `plan stored no item: <first reason>` when its keys exist and nothing was stored; the pull `Plan` returns the key `plan-empty` when the day's plan is empty and its `Run` ends `Retryable` while the plan is still empty; `errNoPlan` as a `NoValue` is gone (`pull.go:160`); if `jp.go` has its own `Plan` for the JP pull it takes the same rule, else it is untouched.
Acceptance:
1. `grep -c 'NoValue, "no stored close' internal/jobs/optionopen/listed.go internal/jobs/optionopen/plan.go` prints 0 for both, `grep -c 'Retryable, "no stored close' internal/jobs/optionopen/listed.go internal/jobs/optionopen/plan.go` prints 1 for both, and `grep -c 'NoValue, "no resolved chain\|NoValue, "%s is not on the stock list' internal/jobs/optionopen/plan.go` prints 0.
2. `grep -c 'NoValue.*errNoPlan' internal/jobs/optionopen/pull.go` prints 0, `grep -c 'plan-empty' internal/jobs/optionopen/pull.go` prints at least 1 and `grep -c 'plan stored no item' internal/jobs/optionopen/plan.go` prints 1.
3. `grep -c "fault stage=" internal/jobs/optionopen/listed.go internal/jobs/optionopen/plan.go internal/jobs/optionopen/pull.go` prints at least 1 for each of the three files.
4. `grep -rhE "^func (TestNoStoredCloseIsRetryable|TestNoChainIsRetryable|TestNotOnStockListIsRetryable|TestPlanStoringNothingIsRetryable|TestEmptyPlanGivesPlanEmptyKey|TestPlanEmptyKeyRunIsRetryable|TestPlanEmptyKeyGoneWhenPlanExists|TestRetryableRerunsBeforeClose|TestFaultLineOnRetryable)\(" internal/jobs/optionopen | wc -l` prints 9
5. `git diff --name-only origin/main` lists no path outside the Files line.
6. The result matches docs/design/option-open-gap-jp.md#silence-a-missing-input-is-never-a-silent-no_value.
Checked by CI: ci-go; `go test ./internal/jobs/optionopen -race -count=1` with fake stores and a fake clock (a key with no bar is `Retryable` and is run again on the next fake turn; the plan-empty key is held before the pull slot).

### 4 optionopen-list-jp

Module: jobs. Size: S.
Files: `internal/jobs/optionopen/list_jp.go` (new), `internal/jobs/optionopen/list_jp_test.go` (new), `internal/jobs/optionopen/job_list_preopen_jp.toml` (new)
Design: docs/design/option-open-gap-jp.md#confirm-by-listing-not-per-contract
Order: after 3, after 1 and 2, and after entry 5 of `option-contract-lists.md` as this pull request amends it (the list job with the freshness, the band and the window list is built there, not here).
Goal: Decision 7: the JP manifest of the one list job: `NewListPreopenJP` over `jpRules` and `parseJPRules(planManifestJP)` with the JP window (`[[window_dte]]` 14 to 90), calendar XTKS, zone Asia/Tokyo, dataset `master.option_list_jp`, subject `preopen`, start 06:30 JST (`session_open` `-150m`), `stop_time` 07:00, `close_offset = "-8h30m"`, `list_ttl_days = 7`, the band keys of Decision 12 and the zone keys of Decision 13; no new job code.
Acceptance:
1. `grep -c 'close_offset = "-8h30m"\|list_ttl_days = 7\|band_k = 1.5\|band_floor_pct = 0.10\|min_strikes_side = 3\|zone_near = 0.5\|zone_far = 1.05\|dataset = "master.option_list_jp"\|zone = "Asia/Tokyo"\|calendar = "XTKS"' internal/jobs/optionopen/job_list_preopen_jp.toml` prints 10.
2. `grep -c "SMART" internal/jobs/optionopen/list_jp.go internal/jobs/optionopen/job_list_preopen_jp.toml` prints 0 for both and `grep -c "parseJPRules(planManifestJP)" internal/jobs/optionopen/list_jp.go` prints 1.
3. `git diff origin/main -- internal/jobs/optionopen/listed.go internal/jobs/optionopen/manifest.go internal/jobs/optionopen/job_resolve_jp.toml internal/jobs/optionopen/job_plan_jp.toml internal/jobs/optionopen/job_pull_jp.toml | wc -l` prints 0.
4. `grep -rhE "^func (TestJPRequestsAreFullySpecified|TestJPListUsesOseClassAndMultiplier|TestJPWindowIs14To90|TestJPUnservedEndsExpiredAtClose)\(" internal/jobs/optionopen | wc -l` prints 4
5. `git diff --name-only origin/main` lists no path outside the Files line.
6. The result matches docs/design/option-open-gap-jp.md#confirm-by-listing-not-per-contract.
Checked by CI: ci-go; `go test ./internal/jobs/optionopen -race -count=1` (a fake broker recording every request: fully specified OSE.JPN requests of class = code and multiplier 100, a fake clock, no sleep).

### 5 optionopen-pull-band-relist

Module: jobs. Size: M.
Files: `internal/jobs/optionopen/core.go`, `internal/jobs/optionopen/core_test.go`, `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/pull_test.go`, `internal/jobs/optionopen/jp.go`, `internal/jobs/optionopen/jp_test.go`, `internal/jobs/optionopen/fakes_test.go`
Design: docs/design/option-open-gap-jp.md#the-band-the-windows-and-the-strikes
Order: after 3 and 4 (3 edits `pull.go` and `jp.go`; 4 and the amended list job give the zone test and the stored range).
Goal: Decision 13 in the pull, US and JP: in `Core.stock`, after the open spot is read and before the quotes of an expiry, the zone test runs against the confirmed range read from the list store; an expiry outside it (a missing range counts as outside) is listed once through a `ListContracts` port of the core, one in flight, counted against `budgetSpent`, once per expiry per run; the contracts listed inside the zone that the stored plan does not hold are written as plan items through the plan store's `PlanWriter` and added to the targets of that expiry; a failed listing leaves the stored plan in use and writes one fault line; no `NoValue`. If the batch grows past `max_per_key` the next pass cuts the batches again (the keys keep their subjects).
Acceptance:
1. `grep -c "ListContracts\|PutPlan" internal/jobs/optionopen/core.go` prints at least 2.
2. `grep -rhE "^func (TestPullRelistsExpiryWhenZoneLeavesRange|TestPullAddsPlanItemsForNewStrikes|TestPullKeepsPlanWhenRelistFails|TestPullRelistsOneInFlight|TestPullRelistCountsAgainstBudget|TestPullRelistsOncePerExpiry|TestPullMissingRangeRelists)\(" internal/jobs/optionopen | wc -l` prints 7
3. `git diff --name-only origin/main` lists no path outside the Files line.
4. The result matches docs/design/option-open-gap-jp.md#the-band-the-windows-and-the-strikes.
Checked by CI: ci-go; `go test ./internal/jobs/optionopen -race -count=1` with a fake broker, a fake list store, a fake plan store and a fake clock (no sleep).

### 6 contracts-ops-today-page-schema (interface)

Module: contracts. Size: S.
Files: `contracts/schemas/page.ops_today_page.v1.json` (new), `web/src/generated/ops_today_page.v1.ts` (new, generated), `internal/contracts/ops_today_page_v1_test.go` (new), `docs/contracts/page-schemas.md`
Design: docs/design/option-open-gap-jp.md#same-morning-visibility
Order: independent of the other entries (it builds in parallel with 1 and 3); before 7 and 8. Interface entry: it carries the Design line and its pull request carries `Contracts: changed` (the schema is a file under `contracts/schemas`; the generated type travels with it, never edited by hand).
Goal: the JSON schema of `ops_today_page`, version 1: `day`, one section `today` with `gaps` (rows of `dataset`, `day`, `reason`, `text`) and `runs` (rows of `dataset`, `key`, `status`, `error_code`, `reason_code`, `returned_n`, `usable_n`, `text`) and `notes`; every text is the producer's and the schema holds no number the view must compute; the contract test holds one valid and two invalid payloads (a missing `today`, an unknown field).
Acceptance:
1. `grep -c "page.ops_today_page.v1" docs/contracts/page-schemas.md` prints at least 1 and `grep -c '"additionalProperties": false' contracts/schemas/page.ops_today_page.v1.json` prints at least 3.
2. `grep -c "OpsTodayPage\|Run\b\|Gap\b" web/src/generated/ops_today_page.v1.ts` prints at least 3, and the generated file equals the output of the project's generator for the schema (CI regenerates and runs `git diff --exit-code`).
3. `grep -rhE "^func (TestOpsTodayPageValid|TestOpsTodayPageMissingTodayRefused|TestOpsTodayPageUnknownFieldRefused)\(" internal/contracts | wc -l` prints 3
4. `git diff --name-only origin/main` lists no path outside the Files line.
5. The result matches docs/design/option-open-gap-jp.md#same-morning-visibility.
Checked by CI: ci-go; `go test ./internal/contracts -race -count=1`; the contracts generation check.

### 7 pages-ops-today-builder

Module: pages. Size: S.
Files: `internal/pages/ops_today_page/builder.go` (new), `internal/pages/ops_today_page/builder_test.go` (new), `internal/pages/ops_today_page/purity_test.go` (new), `docs/contracts/pages.md`
Design: docs/design/option-open-gap-jp.md#same-morning-visibility
Order: after 1 and 6 (it reads `store.RunRow` and `store.Gap` and builds the schema's payload); in parallel with 8.
Goal: `Build(runs []store.RunRow, gaps []store.Gap, now time.Time)` returns the payload: the open gaps as the lines of `pages/gaps.Lines` and today's runs that erred (`error_code` set) or stored nothing (finished with `usable_n` 0), each with a `text` the builder writes; the filter is the only logic; no database import.
Acceptance:
1. `grep -c "pgx\|database/sql" internal/pages/ops_today_page/builder.go` prints 0, `grep -c "gaps.Lines" internal/pages/ops_today_page/builder.go` prints 1 and `grep -c "UsableN" internal/pages/ops_today_page/builder.go` prints at least 1.
2. `grep -rhE "^func (TestErroredRunIsListed|TestEmptyRunIsListed|TestCleanRunIsNotListed|TestOpenGapIsListed|TestNoRowsGivesEmptyPayload|TestPayloadValidatesAgainstSchema)\(" internal/pages/ops_today_page | wc -l` prints 6
3. `git diff --name-only origin/main` lists no path outside the Files line.
4. The result matches docs/design/option-open-gap-jp.md#same-morning-visibility.
Checked by CI: ci-go; `go test ./internal/pages/ops_today_page -race -count=1`.

### 8 web-ops-today-view

Module: web. Size: S.
Files: `web/src/pages/ops_today/OpsTodayPage.tsx` (new), `web/src/pages/ops_today/OpsTodayPage.test.tsx` (new), `web/src/main.tsx`, `web/src/shell/TopNav.tsx`
Design: docs/design/option-open-gap-jp.md#same-morning-visibility
Order: after 6; in parallel with 7 (the test uses a fixture payload of the schema).
Goal: a new page, because `web/` has no ops or status page to extend (the nearest are the Notices page, about option positions, and the Diagnosis page, about companies); it follows `DiagnosisPage.tsx`: the view shows the producer's texts and computes, ranks and formats nothing; reachable from the existing nav through a "Runs" link in the menu of `TopNav.tsx` (path `/ops/today`) and a route in `main.tsx`; the eleven items of `docs/design/unified-site.md` Decision 3 are unchanged.
Frontend lines (`docs/adr/0034-frontend-stack.md`):
- 按 frontend-stack：用 shadcn Data Table（TanStack Table 8）做 runs 表，列头取自 schema，不排序不求和。
- 按 frontend-stack：用 shadcn Card 和 Badge 做 gaps 区块和状态标记，文字取自 producer。
- 按 frontend-stack：用 Ajv standalone（`validate` of `@/types/ops_today_page`）做 payload 校验，校验失败显示 StaleAlert。
- 按 frontend-stack：用 Lucide 做导航图标，用 Tailwind 做布局，不加新依赖。
- 按 frontend-stack：用 Vitest 和 jsdom 做渲染测试，用 Playwright 做版面测试。
Acceptance:
1. `grep -c "ops_today_page" web/src/pages/ops_today/OpsTodayPage.tsx` prints at least 1, and `grep -c "/ops/today" web/src/shell/TopNav.tsx web/src/main.tsx` prints at least 1 for each.
2. `grep -c "sort(\|reduce(\|toFixed(\|Intl\." web/src/pages/ops_today/OpsTodayPage.tsx` prints 0 (the view computes nothing).
3. `git diff origin/main -- package.json pnpm-lock.yaml | wc -l` prints 0 (no new dependency).
4. `grep -cE "^\s*(it|test)\(" web/src/pages/ops_today/OpsTodayPage.test.tsx` prints at least 3.
5. `git diff --name-only origin/main` lists no path outside the Files line.
6. The result matches docs/design/option-open-gap-jp.md#same-morning-visibility.
Checked by CI: web: `tsc`, `eslint`, `vitest` and the build.

### 9 ot-ops-today-page-wiring

Module: ot. Size: S.
Files: `cmd/ot/ops_today_site.go` (new), `cmd/ot/ops_today_site_test.go` (new), `cmd/ot/wire.go`, `docs/contracts/ot/pages/ops_today_page.md` (new)
Design: docs/design/option-open-gap-jp.md#same-morning-visibility
Order: after 2 and 7 (it needs the view and the builder); in parallel with 8. The entry reads `cmd/ot/pagereg_test.go`, `cmd/ot/pagelist_test.go` and `cmd/ot/check_blocks.json`; a file there that names every page is reported in its pull request description as a needed change and not edited (the Files rule).
Goal: `ops_today_page` is served by `ot serve` from the run reader and the gap reader, read-only, like `notices_site.go`; the contract doc states the sources and the day.
Acceptance:
1. `grep -c "ops_today_page" cmd/ot/wire.go cmd/ot/ops_today_site.go` prints at least 1 for each.
2. `grep -c "^Order: \|^Sources: " docs/contracts/ot/pages/ops_today_page.md` prints 2.
3. `grep -rhE "^func (TestOpsTodaySiteServesPayload|TestOpsTodaySiteIsReadOnly)\(" cmd/ot | wc -l` prints 2.
4. `git diff --name-only origin/main` lists no path outside the Files line.
5. The result matches docs/design/option-open-gap-jp.md#same-morning-visibility.
Checked by CI: ci-go; `go test ./cmd/ot -race -count=1`.

### 10 ot-list-jp-wiring

Module: ot. Size: S.
Files: `cmd/ot/wire_optionlist.go`, `cmd/ot/wire_optionlist_test.go`, `docs/contracts/ot/jobs/option_list_preopen_jp.md` (new)
Design: docs/design/option-open-gap-jp.md#confirm-by-listing-not-per-contract
Order: after 4, and after `ot setup pg` and migrate have run for entry 2 (a deploy of this entry before the migration makes every list write fail on a missing column); after entry 6 of `option-contract-lists.md`, whose wiring file it edits.
Goal: register `option_list_preopen_jp` (06:30 JST) through the same family as the US list job, one resolve in flight, the list store of the opened option open store; a store that lacks the list port leaves the job out with one startup line, not silently; the US list job gets its `list_ttl_days = 7` from its toml (entry 5 of `option-contract-lists.md`), so this entry registers one job only.
Acceptance:
1. `grep -c "option_list_preopen_jp" cmd/ot/wire_optionlist.go cmd/ot/wire_optionlist_test.go` prints at least 1 for each.
2. `grep -c "^Order: \|^Slot: " docs/contracts/ot/jobs/option_list_preopen_jp.md` prints 2.
3. `grep -rhE "^func (TestListJPRegistered|TestListJPLeftOutWithoutListStore)\(" cmd/ot | wc -l` prints 2.
4. `git diff --name-only origin/main` lists no path outside the Files line.
5. The result matches docs/design/option-open-gap-jp.md#confirm-by-listing-not-per-contract.
Checked by CI: ci-go; `go test ./cmd/ot -race -count=1`.

## Checked by CI

ci-go runs `go test ./internal/store/... ./internal/jobs/optionopen ./internal/contracts ./internal/pages/ops_today_page ./cmd/ot -race -count=1` for entries 1 to 10 (entry 8: web `tsc`, `eslint`, `vitest` and the build). Nothing here needs PostgreSQL (the `pg` tests run in GitHub CI only), a live broker or a sleep. The first live check is the dispatcher's: one read-only probe of `ListContracts` on an OSE name on port 4001 once the rpc entry and the pyside restart are in (Doubt 1).

## Left out

- A new gap kind `retrying` in `sched/gaps.go`: the notifier prints an unknown reason as "unknown", `notify` is another module, the rank would need renumbering and every routine `Retryable` of every job would become a gap. The early window close and the runs page give the same-morning view with no runner change.
- A second list job family (`option_list_gap`, `option_list_gap_jp`), `gap_only`, `unserved_retryable`, `window.close_offset` as a parser key of its own (the offset is the third value of `Window()`): gone, Decision 1.
- A per-contract resolve for JP, a daily or per-name cap of our own, and any write of `master.option_list_run` by this design.
- A live snapshot of the underlying when the stored bar is missing: Decision 15 asks the gateway for the last daily bar instead.
- Stopping the next stage when a key is retrying: the stages are ordered by the clock; the plan reads what is stored and its own `Retryable` shows a name that has nothing.
- Widening the plan, the pull and the resolve to windows other than the list job's first one (Decision 17, Doubt 10).

## Doubts

1. Whether the pyside `ListContracts` serves the OSE names (class = stock code, OSE.JPN, multiplier 100) is unverified. The dispatcher probes port 4001 once entry 1 of `option-contract-lists.md` (#1751) and the pyside restart are in (read-only; never the account port 4002). Consequence if it does not serve them: every JP name ends `unserved`, which `option-contract-lists.md` Decision 5 ends with `Err` nil and the key unmarked; with the early window close of Decision 11 that key expires at 07:00 JST and is a gap the same morning, the JP plan finds no stored chain and is `Retryable` (Decision 8), and JP stays on the evening per-contract resolve of today until the rpc serves OSE.
2. The page of entries 6 to 9 is a new page: `web/` has none to extend. Its nav slot is a "Runs" link in the `TopNav` menu; whether the owner wants a main item is the owner's.
3. The seven days are a group freshness here, not a per-contract date, because one listing answers a whole expiry. The valid side ends at the contract's expiry, not at seven days; the valid ones are refreshed when the group goes stale. Owner call if valid contracts must also be re-listed on a fixed seven days.
4. IBKR's numbers (50 messages per second, 100 market-data lines) are from memory [unverified]; the 5.0 s per list request is a probe, `option-contract-lists.md` Decision 9 uses 7.3 s; the code limits (pyside `pacing.py`) are verified.
5. The list key is one key per market and day: a name added to the watch list after the key is present is listed by nothing before the plan, the plan is `Retryable` for it and the next day lists it. Owner call if a later pass is wanted.
6. A transient empty answer for an expiry the chain lists is cached seven days (Decision 3); the early re-check is its only guard. Owner call if an empty answer should be confirmed by a second ask before it is cached.
7. `store()` in `listed.go` gives every option instrument `CalendarID` XNYS, also for JP; whether that is right for OSE instruments is unverified and not changed here.
8. The Go client of these jobs dials with `ResolveInFlight: 1`, which keeps the lane at one request and is itself a bound the owner did not set; whether the list job dials with the default of 5 is left to entry 10 and the probe.
9. The 7 plus or minus 2 days of the per-contract marks (`jitter_days`) stay for the per-contract path; the list job has no jitter (Decision 4).
10. Which DTE windows are wanted (two-week 7 to 14, LEAPS 180 to 400, others) and for which names is the owner's later choice; this design makes them configuration of the list job only (Decision 17); the index names multiply the listings (Decision 19).
11. A LEAPS band is wide (sigma 0.30 and 365 DTE: 0.64 to 1.57 of spot) with no 25 percent cap, so the contract rows stored per expiry grow; the number of rows per expiry on a coarse LEAPS grid is unverified.
12. `rv20` of a stock is not stored on main; the list job computes it from stored closes (Decision 14); JP close coverage is unverified.
13. The first band, per 100 of spot, when a name has no stored contract and so no IV (Decision 14); sigma is the realised vol of the stored closes if there are 21, else the default 0.30. A US name at the default 0.30: 21 DTE (the window's first day) gives s = 0.072 and the band 89.8 to 111.4 (1.5 s = 0.108); 30 DTE gives 87.9 to 113.8; 45 DTE gives 84.9 to 117.8; at an rv20 of 0.15 and 30 DTE the formula gives 93.8 to 106.7 and the 10 percent floor sets 90 to 110. A JP name at the default 0.30: 14 DTE gives 91.6 to 109.2, widened by the floor to 90 to 110; 30 DTE gives 87.9 to 113.8; 90 DTE gives 80.0 to 125.0. The real figures for the owner's names (7 JP names with options, `option-capture-jp-stocks.md`) need their stored closes and are not read here; the JP grid steps (250, 100, 25 and 16.67 on one name) are the listing's to give, and whether 3 listed strikes lie on each side of a band at those steps is unverified.
14. Prior art, option-trader (via its advisor), and where this design differs: k = 1.5 for every expiry and both markets (`plan.py:128-144`), the first strike on each side of spot always kept (the intent of the 3-strike floor); its fallback without a sigma was a plain band of spot x (1 plus or minus 0.30), not a sigma of 0.30 in the formula (here 0.30 enters the formula with its reason stored: a choice, narrower on short expiries); its sigma was the underlying's 30-day IV from IBKR (tick 106) of the newest stored quote (here the larger of the stored ATM IV and rv20: a choice); "at least 3 strikes per JP side" was never built there; its listing refresh widened the band by 0.10 x spot per side and 3 days of DTE (`QUALIFY_MARGIN`), which matches the 10 percent floor (the 3 days are not taken); it kept a not-listed cache of 7 days with plus or minus 2 days of jitter and a re-ask on a 5 percent move or a new strike in the chain (here the zone test, no jitter, no chain call); cost on record: about 5 s per (underlying, expiry), 33 names by 3 expiries in about 8 minutes, and 148 expiries a day overloaded the gateway (#776). Nothing is on record of a wanted strike being missed.
15. The pull re-check writes plan items for new strikes through the plan store's writer (Decision 13); the key's batches are cut again at the next pass if a batch grows past `max_per_key`, which changes the plan hash of the day and replaces the expected set; whether any reader keys on the old hash is unverified.

## Adversarial

(filled after the review)
