# Option field reasons: why a field of a stored option quote is missing is reported by pyside, recorded per run and shown on the option data page

Author: the optdesk window (draft)
Owner approved: (pending)
Date: 2026-11-12
Source: owner 2026-11-12: "未平仓量全空 ，这种错误理由不能每次都你去找，需要有点明确的记录，可以方便查询？？" The owner works on the web, so the record is read on a page, not in code or in a terminal.

## Goal

A stored `obs.option_eod` row whose `open_interest` is null says nothing about why. The gateway side knows: pyside (`pyside/src/optdesk/ibkr/ibgateway.py`, `_quote`) knows which fields it asked for (`wanted_fields` of `spec.py`), which wait ended (the release wait or the grace wait for `open_interest`), which value it dropped as a sentinel (`_number`: NaN, `UNSET_DOUBLE`, a negative) and which IBKR error ended the quote. It tells the Go side only the status, and `statusOf` in `internal/jobs/optioneod/map.go` folds OK and PARTIAL into row status 1. After this design pyside reports, per quote, each wanted field it did not deliver and the reason; each `option_eod` run records per underlying how many usable quotes lack each field and the reported reason; `/options/data` shows that record among its notes and says plainly when no record exists for the day.

## Modules

- contracts (`contracts/**`, `internal/contracts/**`, `pyside/src/optdesk/contracts/**`): the proto message and the generated Go and Python code (interface entry).
- broker (`internal/broker/**`): `broker.Quote` carries the reported reasons; the grpc client converts them (interface entry: `internal/broker/api.go`).
- pyside (`pyside/**`): reports the reasons.
- store (`internal/store/**`, `migrations/**`): one migration adds two reason codes.
- jobs (`internal/jobs/**`): `optioneod` tallies the reported reasons and writes the rows.
- pages (`internal/pages/**`): `option_data_page` shows them; `ops_today_page` is kept from taking them as a run's reason.
- ot (`cmd/**`): the composition root hands the writer to the job and the reader to the page.

## Interface

`contracts/proto/ibkr/v1` (additive, `ibkr.v1` stays; the pull request line is `ibkr.v1 additive`):

    enum MissingReason { MISSING_REASON_UNSPECIFIED = 0; MISSING_REASON_NOT_RECEIVED_RELEASE = 1; MISSING_REASON_NOT_RECEIVED_GRACE = 2; MISSING_REASON_INVALID = 3; MISSING_REASON_ERROR = 4; MISSING_REASON_RECEIVED_ZERO = 5; MISSING_REASON_MARKET_DATA_TYPE = 6; }   // quote_types.proto
    message MissingField { string field = 1; MissingReason reason = 2; int32 error_code = 3; }                                                                      // quote.proto
    // Quote gets: repeated MissingField missing = 28;

Meaning, fixed here so the three sides agree: NOT_RECEIVED_RELEASE: no tick for the field by the time the quote ended or was released (the item timeout of the kind, or the release of the line for a field no wait holds back); NOT_RECEIVED_GRACE: the field is a grace field of the kind (`open_interest` of OPTION_EOD) and its grace wait ended without it; INVALID: the ticker held a value pyside drops (a negative or other sentinel, not NaN, which is "not yet set"); ERROR: the quote ended with an IBKR error, `error_code` is its code; RECEIVED_ZERO: ticks arrived for the field but every one is 0.0, i.e. both call and put open interest are 0.0 (`_open_interest` treats 0.0 as absent; a single 0.0 on the wrong side is the gateway's placeholder and is not a reason); MARKET_DATA_TYPE: the quote's market data type at the end is not 1 (live), i.e. delayed, frozen or delayed frozen (2, 3, 4), and the field is missing. The field name of the proto, of `broker.MissingField.Field` and of the writer's lookup is the Quote field name (`implied_vol`); the job maps it to the column name `iv` for scope and text only. There is no NOT_REQUESTED: every field is streamed with generic ticks 100 and 101 (`reqMktData(..., False, False)`), so a field is never left unrequested; `last` is simply a field no wait holds back. `field` is the Quote field name (`bid`, `ask`, `last`, `volume`, `open_interest`, `implied_vol`, `delta`, `gamma`, `vega`, `theta`). pyside lists them for the kinds OPTION_EOD and OPTION_OPEN only; the other kinds send an empty list.

`internal/broker/api.go` (broker, the second interface entry):

    type MissingReason int // MissingNotReceivedRelease, MissingNotReceivedGrace, MissingInvalid, MissingError, MissingReceivedZero, MissingMarketDataType; String() gives not_received_release, not_received_grace, invalid, error, received_zero, market_data_type
    type MissingField struct { Field string; Reason MissingReason; ErrorCode int }
    // Quote gets: Missing []MissingField (nil when pyside reported none)

`grpcbroker.fromQuote` fills it; an UNSPECIFIED reason is a Permanent error, as an unspecified status is. No other interface file changes: `runview` (`internal/store/runview/api.go`) is used as declared. New, outside the interface files:

    // internal/jobs/optioneod: hands the job a writer; nil keeps no reason rows.
    func WithReasons(j contracts.Job, w runview.ReasonWriter) (contracts.Job, error) // error when j is not an option_eod job of this package
    // internal/pages/option_data_page: new optional field of Config
    Reasons runview.Reader // nil: no reason notes

## Decisions

1. **The reason comes from the source.** pyside computes `missing` for each Quote of OPTION_EOD and OPTION_OPEN from facts it holds (the Interface section), in a new file `pyside/src/optdesk/ibkr/missing.py` (a pure function over the kind spec, the ticker, the fields got, the error and which wait ended). Go infers nothing from PARTIAL. A quote of an older pyside has no `missing`; the job then counts its nil field as `unreported`.
2. **Codes.** `ops.option_run_reason` (0036) has the primary key (session_day, dataset_id, subject_key, scope) and a CHECK on `code`. Migration `0038_option_run_reason_fields.sql` replaces the check with the eleven codes and two more: `field_missing` (scope `field:<name>`, or `quotes` for quotes without content) and `fields_complete` (scope `fields`). The number is 0038 because the open pull request 1855 takes 0037; the issue is ordered after it. No new table, view or grant.
3. **Which quotes count.** A quote whose row status (`statusOf`) is 1 is usable; each of the fields bid, ask, last, volume, open_interest, iv, delta, gamma, theta, vega that is nil counts as missing, with the cause of its entry in `Quote.Missing` (`not_received_release`, `not_received_grace`, `invalid`, `received_zero`, `market_data_type`, `error_<code>`), else `unreported`. A quote of row status 2 or 5 counts once in `quotes`, never per field, with `no_data`, `no_subscription`, `timeout`, `disconnected`, `error_<code>` (`error` for code 0) or `empty` (OK or PARTIAL with no bid, ask, last and iv).
4. **Detail text.** `<field> missing <n>/<total> usable quotes: <cause> <k>, <cause> <k>` (causes by count descending, then name), e.g. `open_interest missing 200/200 usable quotes: not_received_grace 200`; `quotes`: `no quote 12/212 quotes: no_data 10, timeout 2`; `fields_complete`: `no missing field in 200 usable quotes`. The field name in the text is the column name (`iv` for `implied_vol`).
5. **Nothing missing is recorded.** A key that stored at least one usable quote and has no field row writes one `fields_complete` row. So every key that stored something has at least one row of the day, and the page can tell "no record" from "nothing missing".
6. **When it is written.** `runKey` tallies the quotes across the chunks it stored and after its last chunk calls `ReplaceReasons(ctx, day, k.DatasetID, k.SubjectKey, rows)` once, `At` from the job clock in UTC. A key whose chunks stored nothing (a Retryable budget stop before the first insert, a Permanent error, no chain or price) writes nothing, so the earlier run's reasons stay. A writer error does NOT fail the run: it is logged and the run keeps its quotes and its result, so a missing migration can never stop collection; a nil writer writes nothing. `ReplaceReasons` replaces the whole set of (day, dataset, subject): `option_eod` is the only writer of reason rows for its dataset and key (the one other caller, `optionopen/plan.go`, writes its own dataset and subject), so nothing else is erased; a second writer would need a merge first. Stored rows (`mapRow`, `statusOf`) do not change.
7. **Where the owner reads it.** `option_data_page.Build` reads `Reasons.ReasonsOn(ctx, chainDay)` where `chainDay` is the session day of the chain it shows, keeps the rows with code `field_missing` or `fields_complete` and `Subject` equal to `Config.Symbol`, and puts each `Detail` first in `notes`, ahead of `Config.Notes`, within the 20 notes of `maxNotes`. When the chain exists, `Reasons` is set, `Symbol` is not empty and no such row exists, the first note is `no field reasons recorded for <chainDay>`. A reader error is an error of `Build`. The schema does not change. The page shows the stored text and computes nothing. The record is the last run of the key; the chain may hold rows of several runs of that day, so a count can differ from the rows shown (see Left out).
8. **ops_today_page.** `runRow` takes the first reason code of a run as the run's `reason_code`; it skips `field_missing` and `fields_complete` for that choice and keeps their details. The page is not wired in `cmd/ot` today, so this is a guard.
9. **Wiring.** A new file `cmd/ot/option_field_reasons.go` holds all new code: opening `ops.OpenRuns` with the runner DSN (writer) and with the pages DSN (reader), the wrap with `WithReasons` (declared by the jobs seam entry) and the page field (declared by the pages seam entry), so the ot entry needs only the seams and the migration, never an unmerged tally or page logic. Edits of `runonce.go` (586 lines), `wire.go` (790 of 800) and `site.go` (603) are at most one added line each for a field or a call (three lines for `site.go`); `wire_optioneod.go` (139 lines) takes the wrap. A file that would pass 800 lines is reported, not split.
10. **pyside file size: an edit whose net line count fits, no split.** `pyside/src/optdesk/ibkr/ibgateway.py` has 799 lines, no ceiling is listed in `docs/design/ratchet.json` (owner-only) and a worker cannot add one. The edit of `_quote`: `_open_interest` (lines 102 to 113, 12 lines) moves to the new `missing.py` and `ibgateway.py` imports it (+1); a variable `grace_ended` is set before the waits (+1) and set where the grace timeout is today a bare `pass` (that line is replaced, net 0); one line computes the missing list (+1); three `missing=` arguments go to the three rows built after the ticker is read, the complete, the error and the timed-out quote (+3). Net 799 - 12 + 1 + 1 + 1 + 3 = 793 lines, an acceptance result of the pyside issue (at most 795). No file is split. The decision for a field is made in `missing.py` from the facts passed: the error code, the market data type, `grace_ended`, the raw ticker values and the fields got. The RECEIVED_ZERO decision is made in the moved `_open_interest`, where the tick is seen. `transport.py` (789) takes one loop of three lines.
11. **Found cause on 2026-11-12:** a read-only probe on port 4001 at about 14:40Z (US market open; `reqMarketDataType(2)` asked but market data type 1 (live) seen on all; generic ticks `100,101`) found: AAPL 20261204 C355 had `callOpenInterest` 2073 after 1.0 s and `putOpenInterest` 0.0 (the placeholder of the other side); P250 had `putOpenInterest` 856 after 1.0 s and `callOpenInterest` 0.0; P355 had both NaN for all 30 s although bid, ask and greeks arrived. So during US hours open interest arrives in about 1 s for most contracts, the 0.0 is only the wrong-side placeholder (already handled), and some contracts never get it. The cause of the all-NULL day (the stored 11-12 rows were collected about 10:08Z, US market closed; whether open interest arrives outside US hours is untested) is not yet found; this design records it per field and reason at the next collection.

## Issues

1. contracts (interface): the proto change and the generated Go and Python. Owner-only path, the owner merges. Order: first.
2. broker (interface): `broker.Quote.Missing`, `fromQuote`. Order: after 1.
3. pyside: `missing.py`, the row and the conversion (Decision 10). `transport.py` is an owner-only path, the owner merges. Order: after 1.
4. store: migration 0038; pull request 1855 (migration 0037) merges before it is attached. Order: first.
5. jobs seam: `optioneod.WithReasons` and the writer field of the job, declared and stored, not yet used. Order: independent.
6. pages seam: the field `Config.Reasons` of `option_data_page`, declared, not yet used. Order: independent.
7. jobs/optioneod: Decisions 3 to 6 (the tally and the writes). Order: after 2 and 5.
8. pages: `option_data_page` notes (Decision 7) and `ops_today_page` (Decision 8). Order: after 6.
9. ot: Decision 9. Order: after 4, 5, 6. The migration 0038 holds auto-deploy: the window runs migrate after the merge of entry 4, and entry 9 merges after it. Pull request 1855 (0037) lands first, else entry 4 is renumbered. Rows appear once 7 has landed and notes once 8 has.

Entries 3 to 8 run in parallel once 1 and 2 have landed (5 and 6 need nothing); entry 9 needs only the seams 5 and 6 and the migration, no sibling's unmerged logic.

## Left out

- `option_open` and `obs.option_open`: its row status already separates `partial`; its quotes carry `missing` after this design, so a later issue can record them with the same tally.
- A column `reason` on `obs.option_eod`: the per-run table is the record; a row reason would change a dataset contract.
- A history of reasons: `ReplaceReasons` keeps the last run of a key and day. The page chain can hold rows of several runs of the day, so counts may differ from rows shown.
- The reasons of kinds other than OPTION_EOD and OPTION_OPEN (underlying IV, snapshot, dividends).
- A day-wide list of all underlyings and the reasons on `ops_today_page` beyond the guard of Decision 8.
- The value of the grace (`OI_GRACE_S`, a separate fix): this design only reports that the grace ended.
- Migration number: 0038 assumes 1855 lands 0037 first; if it does not, the issue is re-cut with the next free number.

## Adversarial

Read of the first draft: `scratchpad/design/option-field-reasons-adversarial.md` (the dispatcher's copy of the review; next to this file in the scratchpad).

1. The text did not say why (partial restated the symptom), and no record looked like nothing missing. Adopted: Decision 1 (reason from pyside, proto change) and Decision 5 (`fields_complete`, "no field reasons recorded" note). Not adopted: a per-day history; the chain/last-run mismatch is named in Decision 7 and Left out.
2. Wiring not already there, file sizes, `ops_today_page` taking the code. Adopted: Decision 9 (new file, one-line edits, report if over 800), Decision 8 (skip in `runRow`), Decision 10 for the pyside files.
3. Simpler: count in the page from stored rows. Not adopted: it cannot say why (a nil `open_interest` of status 1 has no cause) and the owner's question is the cause; the table and migration stay because the reason exists only at collection time.
4. Lock-in of a CHECK code and literals in three modules. Adopted in part: one migration with two codes; the strings are literals in the migration, the job and the page, each side pinned by a test. Not adopted: a structured detail or a table of own; the sentence is what the owner reads.
5. `partial` for any field and `last` never waited for. Adopted: the cause is the reported reason, `last` is a field no wait holds back (no NOT_REQUESTED reason); a nil field without a report is `unreported`. The `Subject` equal to `Symbol` check (translated JP symbols) stays untested at the page: the job writes `SubjectKey` which is the watch symbol the page also gets from the same names; named here, not solved.
6. Migration 0037 collides with 1855, and a Retryable stop clears earlier reasons. Adopted: 0038 with the order after 1855 (Decision 2, Issues), and Decision 6 (nothing stored writes nothing).

Advisor review r1 of the pull request (B1 to B3):

- B1 (ibgateway.py at 799 lines, no offset): adopted, Decision 10 names the offset and the result of 792 lines.
- B2 (order of the ot entry, a writer error failing collection): adopted, Issues entry 9 (after 4, 5, 6; migrate by the window) and Decision 6 (a writer error is logged, never fails the run).
- B3 (vocabulary: NOT_REQUESTED cannot happen, a received 0.0 is dropped, market data type): adopted, Interface and Decision 11 (the cause found by the live probe is filled in before judgement).

Design session of PR 1911 (head 49e699e9):

- Point 3 (ibgateway.py one line under its cap): adopted, Decision 10 states the edit with net 793 lines and says no file is split.
- Point 2 (the ot entry needs unmerged output of two siblings): adopted, Issues 5 and 6 declare `WithReasons` and `Config.Reasons` first; entry 9 is ordered after 4, 5, 6 only.
- Notes: the owner-merge lines are in the drafts of entries 1 and 3 and Issues; `ReplaceReasons` single writer is stated in Decision 6; the `implied_vol` to `iv` mapping is stated in the Interface; each issue draft carries acceptance items (golden tests of `missing.py` per reason, the detail text, the "no field reasons recorded" note, the migration codes).
