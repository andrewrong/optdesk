# option_eod becomes the in-session daily chain snapshot, and readers pick the newest in-session capture

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 1

Source: live probe 2026-11-13 01:06Z (after-close snapshots frozen, open interest 0 of 10, IV 4 of 10); owner decisions 2026-11-13 in the dispatcher window.

Goal: `option_eod` and `option_eod_jp` take one chain snapshot a day inside the session, 90 minutes after the open; a run for any day other than the current session day refuses; every capture says whether it was taken in session; readers use, per day, the newest in-session capture.

## Today

| Piece | On main |
|---|---|
| window | `[window] rule = "after_close"`, `lateness_hours` (5 US, 2 JP); `optioneod/window.go` rejects any other rule and returns `open 0, close = lateness`; the runner carries the offsets as plain values (`reconcile/window.go:42 ruleOf`, `windowFor` = session open + OpenOffset, session close + CloseOffset), so a new rule needs no runner change |
| slot | `job.toml`: `anchor = "session_close"`, `offset = "0s"`; `job_jp.toml`: `anchor = "time"`, `15h50m` |
| unused keys | `freeze_after` is parsed and hashed (`manifest.go:146-165`) and read nowhere else; `stop_after` (JP "1700") is parsed and stored in `job.stop` (`run.go:100`), read nowhere, although `docs/contracts/jobs/option_eod.md:13` says a run after it is `NoValue` |
| bug | `Run` ignores its `day` and uses the clock date (`run.go:119-123`); `TestPastDayAndEarlyRunStore` (`run_test.go:355`) pins it |
| capture | `map.go:83` writes `SnapshotKind: "eod"`; the column is a free string (`contracts/datasets/obs.option_eod.r1.json:24`), so no schema change is needed to carry the flag |
| readers | `optioncandidates/calls.go:249` keeps per instrument the greatest generation then capture_id; `input.go:78` takes the newest chain day from `Catalog.OptionEodLatest`; `computefigures/ivinput.go:55-70` keeps per instrument the greatest `received_at`; `pages/option_boards/load.go:54` greatest capture_id; `pages/option_tracking_page/context.go:269` newest per contract. None reads `snapshot_kind`, none depends on "close" except the text |
| text | `obs.option_eod.r1.json` and the generated `docs/contracts/datasets/obs.option_eod.md`: "frozen after the close"; `docs/contracts/store.md:7`; migration `0006` comment (not edited: applied migrations stay) |
| ordering | `computefigures` `After()` lists `option_eod`, `option_eod_jp` (`schedule.go:52`): an order, not a time; it holds for a morning slot |

## Decisions

1. `option_open` is unchanged. No dedicated close snapshot (backtest data will be bought). Nothing here needs bars: an option bar is refused by IBKR (error 162), so a missed day stays missing.
2. Meaning: `option_eod` is the in-session daily chain snapshot (open interest, volume, IV, greeks, bid and ask). The job and dataset keep their names (see Left out).
3. Slot: manifest `schedule` anchor `session_open`, offset `90m`, `run_bound = "16m"` for both markets; the session calendar gives the open (XNYS and XTKS; an early close moves nothing, the open is the anchor). XTKS opens 09:00, so 10:30 is before the lunch break; its manifest sets `stop_after = "1130"` so a retry never lands in the break or after it.
4. Window: new rule `in_session` in the manifest, key `open_after_minutes` (90 in both files) replaces `lateness_hours`; the window is session open + 90 min to session close, close offset 0. `after_close` is no longer accepted. This is a change of the optioneod manifest reader only (`windowOf`), not of the runner. Retries stay inside the window.
5. `stop_after` is enforced in `Run`: at or after it the run is `NoValue` with "after session" and no port is called (what the contract already says). `freeze_after` stays parsed and unused (Left out).
6. Day (bug): `Run` reads its `day` argument and never defaults it. If `day` is not the clock's local date in the manifest zone, `Run` returns `Permanent` "option_eod: day D is not the current session day; a past chain cannot be collected live" and calls no port and writes nothing. A day that is the clock's date but not a session of the calendar is refused the same way, text "not a session". Dev mode stays for the other jobs: no time-of-day refusal is added to any of them.
7. Several runs on one day: each run is a new `capture_id`; rows are appended, never overwritten (the key already holds capture_id; nothing changes).
8. In-session flag: `snapshot_kind` carries it, no column and no migration. A capture whose clock instant lies in [session open, session close] of its day by the session calendar writes `"session"`; any other `"off_session"`. XTKS lunch is not modelled by the calendar and counts as in session. Rows written before this change hold `"eod"`; readers treat `"eod"` as in session (the 2026-11-12 rows, taken mid-session by a console run, are kept as that day's in-session sample; older after-close rows are the day's only capture and stay readable).
9. Reader rule, one function: per instrument and day, among status-ok rows take those in session (`"session"`, `"eod"`); if there are none take the off-session rows; within the chosen set take the greatest `received_at` (then capture_id). A later off-session run never hides an earlier in-session one. The function lives in the store module, `snapshot.PickCapture(rows []contracts.OptionEod) (picked []contracts.OptionEod, offSession bool)`, so that no reader writes its own rule.
10. Readers: `optioncandidates` (`calls.go` chain rows), `computefigures` (`ivinput.go`), `option_boards` and `option_tracking_page` call `PickCapture`. The pages mark a day served from off-session captures: the option data page gets a boolean `off_session` per underlying (the page reads `PickCapture`'s second result; the front end only renders it). Existing page fixtures gain the field `false`.
11. Own implementation, because no library picks rows by a domain flag; about 25 lines, behind the store interface.

## Modules

No row is added or changed in `docs/design/modules.md`. Paths belong to: store (`internal/store/**`, `docs/contracts/store.md`), contracts (`contracts/**`, `docs/contracts/datasets/**`, `docs/contracts/page-schemas.md`), jobs (`internal/jobs/**`, `docs/contracts/jobs/**`), pages (`internal/pages/**`, `docs/contracts/pages.md`). The `web/` render of `off_session` is a separate web issue the owner may ask for after the page schema lands (Left out).

## Interface

1. Module store, `internal/store/snapshot` (declaration in `internal/store/api.go`): `PickCapture` as in Decision 9; `OptionEodReader` is unchanged.
2. Module contracts: dataset text only for `obs.option_eod` (shape unchanged, snapshot_kind documented as `session`, `off_session`, legacy `eod`); page schema `option_data` gets `off_session` (bool, required) per underlying; generated output goes with its source. `Contracts: changed`.

## Issues

Build order: every entry compiles and its own tests pass against main. BASE is `git merge-base origin/main HEAD`. A grep that "prints nothing" prints nothing and exits 1.

### 1 pick-capture (interface)

Module: store. Size: S.
Order: first.
Files: `internal/store/api.go`, `internal/store/api_test.go`, `internal/store/snapshot/pick.go`, `internal/store/snapshot/pick_test.go`, `docs/contracts/store.md`
Design: docs/design/option-chain-daily.md#interface (Decisions 8, 9)
Acceptance:
1. `git grep -n "func PickCapture" -- internal/store/snapshot/pick.go` prints one line; the doc in `api.go` states the rule of Decision 9.
2. `TestPickCapture` covers: in-session beats a later off-session row; only off-session rows give `offSession` true; a legacy `eod` row counts as in session; ties by `received_at` then capture_id; a non-ok row is ignored.
3. `docs/contracts/store.md` line 7 no longer says "frozen after the close": `git grep -n "after the close" -- docs/contracts/store.md` prints nothing.
4. `git diff --name-only BASE` lists only paths of the Files line.

### 2 option-eod-contract (interface)

Module: contracts. Size: S.
Order: after 1.
Files: `contracts/datasets/obs.option_eod.r1.json`, `docs/contracts/datasets/obs.option_eod.md`, `contracts/schemas/page.option_data_page.v1.json`, `contracts/fixtures/page.option_data_page.v1.ok.json`, `contracts/fixtures/page.option_data_page.v1.jpy.json`, `contracts/fixtures/page.option_data_page.v1.stale.json`, `contracts/fixtures/page.option_data_page.v1.terms.json`, `contracts/fixtures/page.option_data_page.v1.choices.json`, `internal/contracts/gen/records.go`, `web/src/generated/page.option_data_page.v1.ts`, `docs/contracts/page-schemas.md`
Design: docs/design/option-chain-daily.md#interface (Decisions 8, 10)
Acceptance:
1. The dataset description and `snapshot_kind` description in the json and the generated md say "in-session daily chain snapshot" and name `session`, `off_session`, `eod` (legacy); `git grep -n "frozen after the close" -- contracts docs/contracts internal/contracts` prints nothing.
2. The page schema requires boolean `off_session` per underlying; every fixture of the Files line holds `"off_session": false`; the generated Go and TypeScript carry it (source and output in this one change).
3. `git diff --name-only BASE` lists only paths of the Files line.

### 3 option-eod-in-session

Module: jobs. Size: M.
Order: after 1 and 2.
Files: `internal/jobs/optioneod/job.toml`, `internal/jobs/optioneod/job_jp.toml`, `internal/jobs/optioneod/manifest.go`, `internal/jobs/optioneod/window.go`, `internal/jobs/optioneod/run.go`, `internal/jobs/optioneod/map.go`, `internal/jobs/optioneod/schedule.go`, `internal/jobs/optioneod/run_test.go`, `internal/jobs/optioneod/schedule_test.go`, `internal/jobs/optioneod/us_hash_test.go`, `internal/jobs/optioneod/map_test.go`, `docs/contracts/jobs/option_eod.md`, `docs/contracts/jobs/option_eod_jp.md`
Design: docs/design/option-chain-daily.md (Decisions 2 to 8)
Acceptance:
1. Both toml files read `rule = "in_session"` and `open_after_minutes = 90`, a schedule with `anchor = "session_open"` and `offset = "90m"`; `git grep -n "after_close\|lateness_hours" -- internal/jobs/optioneod docs/contracts/jobs` prints nothing; `job_jp.toml` has `stop_after = "1130"`.
2. `windowOf` returns open offset 90 min and close offset 0 for `in_session` and an error for any other rule.
3. `Run` uses its `day` argument (no `time.DateOnly` formatting of the clock in `Run`); a day other than the clock's local date returns `Permanent` with the text of Decision 6, with no broker call and no write; a non-session date likewise.
4. `Run` at or after `stop_after` returns `NoValue` "after session" with no port call (`j.stop` is read).
5. `map.go` writes `session` when the clock instant is inside the session of the day (read from the calendar the job already holds) and `off_session` otherwise; two runs on one day write two distinct capture_ids and neither updates the other.
6. `TestPastDayAndEarlyRunStore` is replaced: `git grep -n "func TestPastDayAndEarlyRunStore" -- internal/jobs/optioneod` prints nothing; the new tests are `TestPastDayRefuses`, `TestNotSessionRefuses`, `TestStopAfterNoValue`, `TestSnapshotKindInSession`; the hash test pin of the manifest is updated (the plan hash of the manifest changes; stored rows keep their hash).
7. `docs/contracts/jobs/option_eod.md` states the meaning (in session, not close), the refusal and the flag.
8. `git diff --name-only BASE` lists only paths of the Files line.

### 4 option-eod-readers-jobs

Module: jobs. Size: S.
Order: after 1.
Files: `internal/jobs/optioncandidates/calls.go`, `internal/jobs/optioncandidates/calls_test.go`, `internal/jobs/computefigures/ivinput.go`, `internal/jobs/computefigures/ivinput_test.go`
Design: docs/design/option-chain-daily.md (Decisions 9, 10)
Acceptance:
1. `chainQuotes` and `ivInputCounted` call `snapshot.PickCapture`; `git grep -n "ReceivedAt.After\|r.CaptureID >= old.CaptureID" -- internal/jobs/optioncandidates/calls.go internal/jobs/computefigures/ivinput.go` prints nothing.
2. A test per file: a day with an in-session capture and a later off-session one uses the in-session rows; a day with only off-session rows uses them.
3. `git diff --name-only BASE` lists only paths of the Files line.

### 5 option-eod-readers-pages

Module: pages. Size: S.
Order: after 1 and 2.
Files: `internal/pages/option_boards/load.go`, `internal/pages/option_boards/load_test.go`, `internal/pages/option_tracking_page/context.go`, `internal/pages/option_tracking_page/context_test.go`, `docs/contracts/pages.md`
Design: docs/design/option-chain-daily.md (Decisions 9, 10)
Acceptance:
1. Both loaders call `snapshot.PickCapture`; the option data page sets `off_session` from its second result.
2. A test per loader: in-session capture wins over a later off-session one; a day of only off-session rows sets `off_session` true.
3. `docs/contracts/pages.md` names the field. `git diff --name-only BASE` lists only paths of the Files line.

## Checked by CI

`go test ./internal/store/... ./internal/jobs/... ./internal/pages/... ./internal/contracts/... -race -count=1`, `go run ./scripts/arch --check`, `go build ./...`, the contracts generation check, and the web `tsc`, `eslint`, `vitest` for the generated type.

## Left out

- Renaming the job or the dataset (`option_eod`, `obs.option_eod`, `OptionEod`, the migration, the ledger ids): the name is a label; a rename touches the store, the generated records, the ledger and every reader for no change in behaviour. Not now; revisit if a new dataset ever needs the name.
- A dedicated close snapshot, bar-based recovery of a missed day (error 162), live collection of a past day.
- `freeze_after` (parsed, unused): removing it changes every manifest hash; left until a manifest change needs it.
- A migration of old rows or a rewrite of `0006` text; `obs.option_eod_jp` ids and the JP dataset text.
- XTKS lunch break in the calendar (`Session` has no break); `stop_after = "1130"` is the guard.
- The web rendering of `off_session` (labelled "after close" or similar) and `option_open` readers (`optionopen/inputs.go`), unchanged.

## Doubts

1. Legacy `eod` rows counted as in session: old after-close rows are then ranked with new in-session ones on a day that has both only if a day has both; owner may prefer to rank them off session.
2. `Permanent` for a past-day refusal makes a reconcile of past gaps log one failure per day; `NoValue` would hide it. Chosen `Permanent` because the owner asked for a named refusal.
