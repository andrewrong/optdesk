# candidates spot: the sell-put candidates card shows the newest stored stock quote and the day it was taken

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-24
Plan: P2

Source: owner ask optdesk-5750. Existing designs: `docs/design/option-candidates.md` (the job), `docs/design/option-candidates-page.md` and `docs/design/candidates-page-v2.md` (the page). This design edits none of them.

Goal (what the owner sees): the card of a US sell-put candidate shows a current price of the stock, taken from the newest stored stock quote, with the day it was taken ("现价 $x 存档 <day>", or "<day> 报价" when the quote is from an earlier session than the rows). It never labels a previous session's price with today's day. Where no stored quote exists the card says the price is the option chain's, with the chain's day.

## Today

All paths at origin/main 150962cb.

- `internal/jobs/optioncandidates/source.go` `chain` returns `chainResult.spot`, the underlying price of the chain: a row of `option_open`, or on the fallback the price of the previous session's `option_eod` rows (`eodQuotes`). `convert.go` `toStoreRow` stores it as `store.Candidate.Spot`. `CandidateStores` (`types.go`) has no quote reader.
- `internal/pages/option_candidates_page/tiers.go` `companyView` prints `spotNoteOf(string(rowsDay))`: the day of the rows, not the day the price was taken. After an open where underlyings fell back to `option_eod`, a previous session's price carries today's day.
- `obs.quote_snapshot` (job `quote_snapshot`) holds a stock price taken on the day, read by `store.QuoteSnapshotReader.QuoteSnapshots(ctx, instrumentID, from, to)`. `cmd/ot/candidates.go` `optionCandidates` builds the stores and does not pass it; the factory already holds it as `f.snap` (`cmd/ot/runonce.go`).
- `Candidate.Marks` is a stored JSON array of text; the job writes `eod_fallback` into it and the page reads it (`parseMarks`, `marksCell`).

## Modules

jobs (optioncandidates), ot (cmd/ot), pages (option_candidates_page). Three entries: the job first, the wiring after it, the page independent (a row without the new marks keeps today's label, so the page can land before or after the job).

## Interface

No shared interface file changes. `store.QuoteSnapshotReader` and `store.Candidate` are used as they are; no migration, no contract file. The three facts travel as strings inside the existing `Candidate.Marks`: `spot_quote:<day>`, `spot_chain` and `chain_day:<day>` (day as `YYYY-MM-DD`). `CandidateStores.Quotes` is a field of a type in the job's own package (`types.go`), not of `internal/jobs/api.go`.

## Decisions

1. No new column and no new table: marks carry the day of the spot and the day of the chain, because a field of `store.Candidate` would edit `internal/store/api.go`, the candidate table and the migrations. Own implementation of the mark texts, because no library applies; the mark names are constants in the job (`spot.go`) and in the page (`strings.go`).
2. The spot is the `Last` of the newest stored stock quote: session days from seven days before the run day up to the run day, rows with status ok and a `Last`, not received after the run's clock time; greatest `SessionDay`, then greatest `ReceivedAt`. A read error or no row falls back to the chain price with the mark `spot_chain`; it is never an error of the run.
3. Instrument id: `QuoteSnapshots` is keyed by the same id as `ul.id` of `source.go`. `ul.id` is the `master.instrument` id of the stock (`input.go` takes it from `InstrumentNames` with type stock, `underlyingRef.id`), and `quote_snapshot` writes `QuoteSnapshot.InstrumentID` from `ResolveInstrument` of the `ibkr_conid` identifier in `internal/jobs/quotesnapshot/run.go` `collect`, the same table. So `Quotes.QuoteSnapshots(ctx, ul.id, from, to)` is right with no mapping; a stock that `quote_snapshot` does not list has no rows and gets `spot_chain`.
4. `chain_day:<day>` is written only when the chain is from an earlier session than the run day (the `eod_fallback` path, `ul.chainDay`); an open chain has no such mark. The existing `eod_fallback` mark and counts are unchanged. Order of the marks in the stored text: those of the rule, `eod_fallback`, `chain_day`, then `spot_quote` or `spot_chain`; `runIDOf` covers them.
5. `Quotes` is optional and nil-safe: nil gives today's behaviour with no new mark, so every existing test of the package and every other wiring (the JP job) is unchanged. `cmd/ot/wire_optioncandidates_jp.go` is not touched.
6. The page labels from the marks and does no date arithmetic: a day earlier than `rowsDay` (text comparison of two `YYYY-MM-DD` texts) reads "<month-day> 报价", otherwise "存档 <month-day>"; `spot_chain` reads "期权链价 <month-day>"; no spot mark (rows stored before this change) reads as today. `chain_day:<day>` is a badge "期权数据 <month-day>" on the row so that the premium says which session's options it uses; `spot_quote` and `spot_chain` are not badges. Chinese text is only in `zh_texts.go`, mark names only in `strings.go`.
7. Rows already stored keep today's label until the next run; no backfill (the card of past days is history).

## Issues

All entries: `Plan: P2`, `Contracts: unchanged`. Size S each.

### 1 cspot-1-jobs

Module: jobs. Order: none.
Files: `internal/jobs/optioncandidates/source.go`, `internal/jobs/optioncandidates/spot.go`, `internal/jobs/optioncandidates/spot_test.go`, `internal/jobs/optioncandidates/types.go`, `internal/jobs/optioncandidates/job_test.go`
Acceptance: 1. `CandidateStores.Quotes` (new, `store.QuoteSnapshotReader`) is optional; nil keeps today's spot and writes no new mark. 2. `spot.go` (new) has one function that gives the Decision 2 price and its session day. 3. `chain` sets the spot from it and the marks of Decisions 2 and 4 in the order of Decision 4. 4. `TestSpotFromQuote` and `TestCandidatesSpotMarks` (new) show the cases of the draft: newest capture of the newest session, a row after the clock, no rows; open chain, eod fallback with a quote of the previous session, no quote; `Quotes` nil stores the rows it stored before.

### 2 cspot-2-ot

Module: ot. Order: after 1.
Files: `cmd/ot/candidates.go`, `cmd/ot/candidates_test.go`
Acceptance: 1. `optionCandidates` sets `Quotes` to `f.snap`. 2. `TestOptionCandidatesQuotes` (new) shows the stores carry a non-nil `Quotes` when the factory has a quote store.

### 3 cspot-3-page

Module: pages. Order: none.
Files: `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/strings.go`, `internal/pages/option_candidates_page/cells.go`, `internal/pages/option_candidates_page/tiers_test.go`, `internal/pages/option_candidates_page/markhints_test.go`
Acceptance: 1. The card note and the badge of Decision 6. 2. `TestCompanyCardSpotNote` and `TestMarksCellChainDay` (new) show the four notes and the badge; the golden files are unchanged.

## Checked by CI

- `ci-go`: `go build ./...`, `go vet ./...`, `go test ./internal/jobs/optioncandidates/... ./internal/pages/option_candidates_page/... ./cmd/ot/...`.
- After the next US open run (dispatcher check, not review): the stored rows carry `spot_quote:<day>` or `spot_chain`, the names with `spot_chain` are counted, and a name on the fallback shows "报价" with the chain-day badge.

## Adversarial

Full answer: the dispatcher's scratch file `candidates-spot.adversarial.md` (not committed).

1. Gap vs goal: adopted. The newest quote at the open is usually the previous session's 15:30 capture, so the label says "<month-day> 报价" (quote of that day), never "收盘" (Decision 6, Goal). A newer open-time price is left out (Left out).
2. Coverage risk: adopted. `quote_snapshot` lists instruments by manifest; a stock it does not list gets `spot_chain` and the card says so; the dispatcher check counts `spot_chain` names after the first run (Checked by CI).
3. Simpler way (label from `eod_fallback` and `chain_day` only): not adopted. It would still show a chain price; the owner asked for the stock's own quote.
4. Lock-in (string marks): accepted, not changed. Mark names are constants in both packages, each tested; a field of `store.Candidate` is the alternative (Left out).
5. False claim: adopted ("收盘" removed). The instrument-id claim was checked and holds (Decision 3); status is the int 1.
6. Slip: noted. The clock filter is tested with the fake clock (entry 1); the golden files stay unchanged (entry 3).

## Left out

- A price newer than the last stored quote (today's `option_open` underlying price); the card shows the day of what it prints.
- A numeric spot-day field on `store.Candidate` (would edit the store interface and the table).
- The JP job and page: their spot comes from the JP open quote; not part of ask optdesk-5750.
- Backfill of rows stored before this change (Decision 7).
