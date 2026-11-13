# option_eod becomes the in-session daily chain snapshot, and the store read picks the newest in-session capture

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 2

Source: live probe 2026-11-13 01:06Z (after-close snapshots frozen, open interest 0 of 10, IV 4 of 10); owner decisions 2026-11-13 in the dispatcher window.

Goal: `option_eod` and `option_eod_jp` take one chain snapshot a day inside the session, 90 minutes after the open; a run for a day other than the clock's current day refuses; every row says whether its quote was live; every reader of `obs.option_eod` gets, per day and instrument, the newest in-session row.

## Today

| Piece | On main |
|---|---|
| window | `[window] rule = "after_close"`, `lateness_hours` (5 US, 2 JP); `optioneod/window.go` rejects any other rule and returns `open 0, close = lateness`; the runner carries the offsets as plain values (`runner/reconcile/window.go` `ruleOf`, `windowFor`), so a new rule needs no runner change |
| slot | `job.toml`: `anchor = "session_close"`, `offset = "0s"`; `job_jp.toml`: `anchor = "time"`, `15h50m` |
| dead keys | `freeze_after` and `stop_after` (JP "1700") are parsed and hashed (`manifest.go`), stored in `job.stop` (`run.go:100`) and read nowhere, although `docs/contracts/jobs/option_eod.md:13` says a run after `stop_after` is `NoValue` |
| bug | `Run` ignores its `day` and uses the clock date (`run.go:119-123`); `TestPastDayAndEarlyRunStore` (`run_test.go:355`) pins it; `bareRun` (`runner/reconcile/backfill_force.go:246-256`) calls `Run` with a past day |
| capture | `map.go:83` writes `SnapshotKind: "eod"`; the column is a free string; each row has its own `MarketDataType` (1 live, 2 frozen, 3 delayed, 4 delayed frozen) ; `captureID` (`map.go:57-60`, called at `run.go:204`) is `uuid.NewSHA1(OID, day|dataset|subject)`, the key is (session_day, capture_id, instrument_id) (`migrations/0006_option_eod.sql:42`) and the insert is `ON CONFLICT DO NOTHING` (`store/snapshot/optioneod.go:28`), so a second run of the same day and subject is dropped silently |
| store read | `OptionEod` (`store/snapshot/optioneod.go:92`, SQL `selectOptionEodSQL`) returns every capture's rows, ordered by day, instrument, capture_id |
| readers (non-test) | all through `OptionEodReader.OptionEod`: `computefigures/ivinput.go:58` and `subjects.go:47`, `optioncandidates/calls.go:245`, `input.go:135,151`, `optionopen/inputs.go:195`, `pages/option_data_page/builder.go:188`, `option_boards/load.go:71`, `option_tracking_page/context.go:256`, `structures_page/load.go:145`, `risk_page/load.go:151`, `cmd/ot/optionledger.go:458`, `cmd/ot/companies_inputs.go:149`. `OptionEodLatest` (catalog; `subjects.go:41`, `input.go:78`, `settings_page`, `site.go`, `settings.go`, `companies_inputs.go:61`, `optionledger.go:433`, `context.go:186`) answers newest day per underlying and does not depend on the capture. Each reader keeps its own newest-capture pick (greatest capture_id, generation, or received_at); none reads `snapshot_kind` |
| text | `obs.option_eod.r1.json` and its generated `docs/contracts/datasets/obs.option_eod.md` and `internal/contracts/gen/records.go` comments: "frozen after the close"; `docs/contracts/store.md:7`; migration `0006` comment (not edited) |
| ordering | `computefigures` `After()` lists `option_eod`, `option_eod_jp`: an order, not a time; it holds for a morning slot |

## Decisions

1. `option_open` is unchanged. No dedicated close snapshot (backtest data will be bought). A missed day stays missing: an option bar is refused by IBKR (error 162).
2. Meaning: `option_eod` is the in-session daily chain snapshot (open interest, volume, IV, greeks, bid and ask). The job and dataset names stay (Left out).
3. Slot: manifest `schedule` anchor `session_open`, offset `90m`, `run_bound = "16m"` for both files. This is scheduler timing only: no run is refused for its time of day (development mode stays; retries use the same slot rule and the window below).
4. Window: new rule `in_session`, key `open_after_minutes = 90` in both files, replaces `lateness_hours`; the window is session open + 90 min to session close, close offset 0. `after_close` is no longer accepted. Only `optioneod/window.go` changes; the runner is unchanged.
5. `stop_after` is dropped entirely: the key leaves `job_jp.toml`, the manifest field, the `job.stop` field and the contract line. `freeze_after` stays (Left out).
6. Day (bug): `Run` reads its `day` argument. If `day` is not the clock's local date in the manifest zone, `Run` returns `Permanent` with the reason "option_eod: day D is not the current day; a past option chain cannot be collected" and calls no port and writes nothing. The refusal applies to every caller of `Run`, backfill and force (`bareRun`) included: a past option chain cannot be collected live. No other job gets a time refusal. A non-session date is not refused here (no calendar is read; the broker answers with frozen quotes and Decision 8 marks them).
7. Several runs on one day: `captureID` also takes the run's start instant from the job clock (`day|dataset|subject|start`), so each run has its own `capture_id` and its rows are appended, never dropped or overwritten. The key, the insert and the migrations are unchanged.
8. In-session flag from the quote, not a calendar: a row is written with `snapshot_kind = "session"` when its `MarketDataType` is 1 and `"off_session"` when it is 2, 3 or 4. No Calendar parameter, no constructor or `cmd/ot` change. Rows written before this change hold `"eod"` and rank as `"session"`; the after-close legacy rows being ranked as session is an accepted imprecision (Left out). The 2026-11-12 rows (taken mid-session by a console run) are kept as that day's in-session sample.
9. Pick inside the store read: `OptionEod` returns, per `session_day` and `instrument_id`, the rows of status ok of one capture only: among rows ranked `session` (or `eod`) if any, else among `off_session` rows; the one with the greatest `received_at`, then `capture_id`. A (session_day, instrument_id) with no ok row returns one row too: the newest by `received_at`, then `capture_id`, among its non-ok rows (the scope is the key, so an instrument with ok rows and another without are each decided alone). A later off-session run never hides an earlier in-session row. Ordering stays day, instrument, capture_id. The pick is in the reader (`optioneod.go` read path, in Go after the scan or in `selectOptionEodSQL`), not an exported function, so no job or page imports a new symbol (`scripts/arch/main_test.go:183`).
10. Every reader in the Today row is covered by Decision 9 through the one read; no reader file changes for the pick, and their own newest-pick code stays (it sees one row per key, so it is idempotent; removing it is Left out). `OptionEodLatest` is unchanged. No reader needs a second capture of the same day.
11. Pages show the flag: `option_data_page` emits boolean `off_session` per underlying when the picked rows of its newest day are `off_session`; the field is optional in the schema (absent means false), so there is no schema version bump and existing fixtures stay valid. `option_boards` and the other pages are unchanged.
12. Own implementation of the pick (about 25 lines in the store read), because no library picks rows by a domain flag.

## Modules

No row is added or changed in `docs/design/modules.md`. Paths belong to store (`internal/store/**`, `docs/contracts/store.md`), contracts (`contracts/**`, `internal/contracts/**`, `docs/contracts/datasets/**`, `docs/contracts/page-schemas.md`), jobs (`internal/jobs/**`, `docs/contracts/jobs/**`), pages (`internal/pages/**`).

## Interface

1. Module store: the contract text of `OptionEodReader` in `docs/contracts/store.md` changes (the returned rows are the picked rows of Decision 9); `internal/store/api.go` and every signature are unchanged.
2. Module contracts: dataset text of `obs.option_eod` (shape unchanged; `snapshot_kind` values `session`, `off_session`, legacy `eod`); page schema `option_data_page` v1 gains optional `off_session`; generated output goes with its source. `Contracts: changed`.

## Issues

Build order: every entry compiles and its own tests pass against main. BASE is `git merge-base origin/main HEAD`. A grep that "prints nothing" prints nothing and exits 1.

### 1 option-eod-read-pick (interface)

Module: store. Size: S.
Order: first.
Files: `internal/store/snapshot/optioneod.go`, `internal/store/snapshot/optioneod_test.go`, `internal/store/snapshot/optioneod_pg_test.go`, `docs/contracts/store.md`
Design: docs/design/option-chain-daily.md#interface (Decisions 8, 9)
Acceptance:
1. `docs/contracts/store.md` states (`internal/store/api.go` is at its 800 lines and is not edited) the rule of Decision 9; `git grep -n "frozen after the close\|after the close" -- docs/contracts/store.md` prints nothing.
2. `optioneod.go` holds the pick in the read path and exports nothing new: `git diff BASE -- internal/store/snapshot/optioneod.go | grep -c "^+func [A-Z]"` prints 0.
3. `optioneod_test.go` `TestOptionEodPick` covers: a session row beats a later off_session row of the same instrument and day; a day with only off_session rows returns the newest of them; a legacy `eod` row ranks as session; ties by `received_at` then `capture_id`; a non-ok row is not picked while an ok row exists, and a key with only non-ok rows over several captures returns the newest of them (one row); two days and two instruments stay separate; the order stays day, instrument, capture_id. `optioneod_pg_test.go` (tag `pg`) asserts the same through the database.
4. `git diff --name-only BASE` lists only paths of the Files line.

### 2 option-eod-contract (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/datasets/obs.option_eod.r1.json`, `docs/contracts/datasets/obs.option_eod.md`, `contracts/schemas/page.option_data_page.v1.json`, `internal/contracts/gen/records.go`, `web/src/generated/option_data_page.v1.ts`, `docs/contracts/page-schemas.md`
Design: docs/design/option-chain-daily.md#interface (Decisions 8, 11)
Acceptance:
1. The dataset description and the `snapshot_kind` description (json, generated md and the generated comments in `internal/contracts/gen/records.go`) say "in-session daily chain snapshot" and name `session`, `off_session` and `eod` (legacy); `git grep -n "frozen after the close" -- contracts/datasets docs/contracts/datasets` prints nothing (`docs/contracts/store.md` is entry 1's).
2. The page schema holds `off_session` as a boolean property of `$defs/Underlying` that is not in `required`; no fixture changes in this entry; the generated TypeScript carries the optional field (source and output in this one change).
3. `git diff BASE -- contracts/fixtures` prints nothing: the `obs.option_eod.r1.json` line of `MANIFEST.synthetic-v1.json` is the row fixture, not the dataset schema, and stays unchanged.
4. `git diff --name-only BASE` lists only paths of the Files line.

### 3 option-eod-in-session

Module: jobs. Size: M.
Order: first.
Files: `internal/jobs/optioneod/job.toml`, `internal/jobs/optioneod/job_jp.toml`, `internal/jobs/optioneod/manifest.go`, `internal/jobs/optioneod/window.go`, `internal/jobs/optioneod/run.go`, `internal/jobs/optioneod/map.go`, `internal/jobs/optioneod/plan_test.go`, `internal/jobs/optioneod/jp_test.go`, `internal/jobs/optioneod/jp_stock_test.go`, `internal/jobs/optioneod/run_test.go`, `internal/jobs/optioneod/map_test.go`, `internal/jobs/optioneod/schedule_test.go`, `internal/jobs/optioneod/us_hash_test.go`, `internal/jobs/optioneod/held_test.go`, `internal/jobs/optioneod/held_aliases_test.go`, `internal/jobs/optioneod/watch_test.go`, `internal/jobs/optioneod/select_test.go`, `internal/jobs/optioneod/types_test.go`, `docs/contracts/jobs/option_eod.md`, `docs/contracts/jobs/option_eod_jp.md`
Design: docs/design/option-chain-daily.md (Decisions 2 to 8)
Acceptance:
1. Both toml files read `rule = "in_session"`, `open_after_minutes = 90` and a schedule `anchor = "session_open"`, `offset = "90m"`; `git grep -n "after_close\|lateness_hours\|stop_after\|StopAfter\|j.stop" -- internal/jobs/optioneod docs/contracts/jobs/option_eod.md docs/contracts/jobs/option_eod_jp.md` prints nothing.
2. `windowOf` returns open offset 90 min and close offset 0 for `in_session` and an error for any other rule.
3. `Run` uses its `day` argument and never formats the clock into a day; a day other than the clock's local date in the zone returns `Permanent` with the text of Decision 6, with no broker call and no write (read in `Run`; the test `TestPastDayRefuses` has a rig with a clock on another date and asserts zero calls and zero writes, for US and JP).
4. `map.go` writes `session` for `MarketDataType` 1 and `off_session` for 2, 3, 4 (`TestSnapshotKindFromQuote`); two runs on one day write two distinct capture_ids and neither updates the other.
4a. `captureID` takes the run's start instant from the job clock: `TestCaptureIDStable` becomes stable for one (day, key, start) and different for two starts; two runs of one day and subject (`TestTwoRunsTwoCaptures`, through the fake writer and the rig clock) hand the writer two row sets under two `capture_id`s; no migration.
5. `TestPastDayAndEarlyRunStore` is replaced: `git grep -n "func TestPastDayAndEarlyRunStore" -- internal/jobs/optioneod` prints nothing. The tests that read the old window, the dropped key or the clock date are updated: `plan_test.go` (lateness), `jp_test.go` (Window values), `jp_stock_test.go` (the clock day of the calls, `TestJPPlanHashUnchanged`: the new JP hash is pinned and the line says the hash changed with the window keys and `stop_after`), `us_hash_test.go` (the US hash), `held_test.go`, `held_aliases_test.go`, `watch_test.go`, `select_test.go`, `types_test.go` (runs on the clock's date). `TestStopAfterReturnsNoValue`, `TestGuards` and `TestJPWrongDayNoRows` exist in no test file (`git grep` finds them only in the two contract docs, which name tests that are gone): the docs are rewritten here. `cmd/ot` (`runonce_option_test.go` uses stub jobs, `jp_wire_test.go` and `held_underlying_rows_test.go` use fake slots, `wire_optioneod_conid_test.go` runs on a fake clock of its own day) and the runner backfill/force tests (fake jobs) pin neither `session_close`, the lateness, nor a past-day `Run` of the real job: none is in Files.
6. `docs/contracts/jobs/option_eod.md` and `option_eod_jp.md` state the meaning (in session, not close), the refusal and the flag.
7. `git diff --name-only BASE` lists only paths of the Files line.

### 4 option-data-page-off-session

Module: pages and contracts (the fixture paths belong to contracts; they are the builder's output and sit here so that the entry stays one change). Size: S.
Order: after 2.
Files: `internal/pages/option_data_page/builder.go`, `internal/pages/option_data_page/builder_test.go`, `internal/pages/option_data_page/page.go`, `contracts/fixtures/page.option_data_page.v1.off-session.json` (new), `contracts/fixtures/page.option_data_page.v1.off-session-input.json` (new), `contracts/fixtures/MANIFEST.synthetic-v1.json`
Design: docs/design/option-chain-daily.md (Decision 11)
Acceptance:
1. The builder sets `off_session` true on an underlying whose picked rows of the newest day are all `snapshot_kind = "off_session"`, and leaves it unset otherwise (read in `builder.go`).
2. This entry writes the two new fixtures from its own builder output; `MANIFEST.synthetic-v1.json` gains their two lines with sha256 and every existing line is unchanged. `builder_test.go` builds the `off-session` fixture input and equals the `off-session` page fixture, validates it against the schema, and the `ok` golden test passes unchanged.
3. `git diff --name-only BASE` lists only paths of the Files line.

## Checked by CI

`go test ./internal/store/... ./internal/jobs/... ./internal/pages/... ./internal/contracts/... ./cmd/ot/... -race -count=1`, the `pg` tests in GitHub CI, `go run ./scripts/arch --check`, `go build ./...`, the contracts generation check, and web `tsc`, `eslint`, `vitest`.

## Left out

- Renaming the job or the dataset (`option_eod`, `obs.option_eod`, `OptionEod`, migration, ledger ids). Not now: a rename touches the store, generated records, the ledger and every reader for no behaviour change.
- A dedicated close snapshot (backtest data will be bought), bar recovery of a missed day (IBKR refuses option bars, error 162), live collection of a past day (the broker answers for today only).
- `freeze_after` (parsed, unused): removing it changes every manifest hash for nothing.
- Legacy `eod` rows that were taken after the close rank as session: an accepted imprecision, no migration.
- Removing the readers' own newest-pick code, the web rendering of `off_session`, and any `off_session` mark on pages other than `option_data_page`.
- A calendar check of the session day or of the XTKS lunch break: the flag comes from the quote (Decision 8) and the calendar would need a constructor change in `cmd/ot` for no result the quote does not give.

## Doubts

1. `OptionEod` now returns one capture per day and instrument; a future reader that needs every capture (a history of captures) needs a new read. None in the Today row does.
2. `Permanent` for a past-day refusal makes a reconcile of past gaps log one failure per day; `NoValue` would hide it. Chosen `Permanent` because the owner asked for a named refusal.
3. Live quotes (type 1) after the close, if the broker ever gives them, would rank as session; the 01:06Z probe saw frozen ones.
4. Delayed data (`MarketDataType` 3) taken in session is stored as `off_session`; if the broker serves only delayed quotes in session, every row is off_session and still readable.
5. Manual runs rely on the same-job guard `JobLocks.TryLock` (`internal/runner/api.go:264`, type at 261): two runs of one job never overlap, so one capture set at a time.

## Adversarial

Read: the adversarial answer (`ocd-adversarial.md`) stays in the dispatcher's scratch directory on the machine.

1. Gap against the goal (option_data_page missing, eight readers uncovered, backfill/force): adopted. `option_data_page` is issue 4; the readers are listed and covered through the store read (Decisions 9, 10); backfill and force are named in Decision 6.
2. Biggest risk (PickCapture in a declarations-only `api.go`, arch rule; flag as a clock claim): adopted. The pick is inside the reader, nothing exported; the flag comes from the quote's `MarketDataType`.
3. Simpler way (flag from `MarketDataType`, pick in the reader): adopted as the design; `internal/compute` as a home is not needed.
4. Lock-in (free-string kinds, legacy `eod`, required page field): partly adopted. The page field is optional with no version bump; legacy `eod` ranking as session is kept and named in Left out; the three-value `snapshot_kind` stays because the owner wants the column used.
5. Claim most likely false (calendar held by the job; `TestPastDayAndEarlyRunStore` the only pin; wrong TS path and unmarked new files): adopted. No calendar is used; the other tests that call `Run` are in issue 3; the TS path is `web/src/generated/option_data_page.v1.ts`; new files are marked.
6. Likeliest slip (Files misses, Order): adopted. The tests and fixtures named are in Files lines (`gap` and `bad-version` fixtures need no change because the field is optional; entry 4 writes the new fixtures and their manifest lines); issue 3 no longer waits for issue 2; `stop_after` is dropped, which also answers the manual-trigger contradiction.
