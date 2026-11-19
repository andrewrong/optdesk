# ot family `option_candidates_jp`

Order: 183

Family file: `cmd/ot/wire_optioncandidates_jp.go`, registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`), after `option_open_jp` (180). Design: `docs/design/candidates-jp.md#7` (Decisions 1, 4, 13, 26, 27 and 30).

## Order

- Order 183: unique and strictly increasing in the registry.
- The hold is removed (issue #3187): the family has no `startOff`, so `ot scheduler` lists `option_candidates_jp` over the same stores as the run-once build and runs it. The job console lists it as a today-only job in `Asia/Tokyo` (force allowed), and the data-health table names `candidates.option_candidate` for `XTKS`.

## Run-once

`ot run-once --job option_candidates_jp` builds the job (`optioncandidates.NewJP`) over the stores `option_candidates` opened: the option open store line (`OT_PG_DSN_OPTION_OPEN`), the candidate store as a `candidatecur.Writer`, the JP watch entries, the account store, and the rule that `deps.newCurrencyRule` returns (default `optioncand.NewCurrency`).

## Left out

With the option open store missing (no line, or it could not be opened) the job is left out with one startup line naming the job and the key; the US job `option_candidates` still runs. The same holds when the candidate store cannot write the currency of a row.

## Page and command

The candidates page gets the Japan group from the candidate store (`siteReaders.candCur`, `candidateReaders.CandCur`); the demo profile and `ot page` leave it nil. `ot candidates` prints for a JP name a line that points to the page; its read of `Candidates` stays USD-only.

## Version

v1: registered, held, run by `ot run-once` (issue #3180).
v2: the hold is removed; scheduled (issue #3187).
