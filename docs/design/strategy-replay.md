# strategy replay, step 2: hold to expiry against his actual result on all his short-put lots, and the early-close rules on the lots that have stored quotes, each with its own count

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-14
Revision: 3

Source: owner 2026-11-13 (record `optdesk-owner-2026-11-13.md`, P2 to P4): every rule cites a source, every number is marked [V] sourced, [S] snippet only, [O] owner, [T] trial, [NF] none found; hold to expiry is the default exit, sometimes he closes early, replay both and compare. Owner 2026-11-13, the lot set: short puts only (answer "1. a"). The advisor's plan `option-strategy-plan.md` (sections 5, 6, 9) and the valuation notes section 8 (a thin own daily replayer). The alternative rule: close at 50% of the credit kept or at 21 days to expiry (tastylive) [S]; an independent study on SPX 2001 to 2020 found no management better than none [V]; Cboe PUT holds to expiry [V]. Existing design: `strategy-framework.md` (merged: `strategy.Strategy.Manage`, `shortput` with `close_mode`, `early_keep`, `early_dte`), `option-ledger.md`, `flex-xml-import.md`, `option-captures.md` (storage step 8, PR 2292, not merged when this was written). Revision 2 follows `replay-adversarial.md` (the dispatcher's scratchpad, outside the repository); `## Adversarial` answers it.

Goal: two questions, two sets of data, never mixed. (a) Does holding to expiry beat what he actually did, on every short-put lot he has: needs the ledger and daily underlying closes, no option quote. (b) Would an early-close rule have done better: needs stored quotes of the option after entry, so it is answered only on the lots that have them, and says how many. One pure replayer, one loader and one command `ot strategy-replay`. No migration, table, role, secret or library; nothing is written.

## Today

- The ledger: his executions are `optionledger.flex_trade_row` (imported Flex XML and the daily fetch), read by `store.OptionLedgerReader.FlexTradeRows(alias)`; `compute.OptionLedger.Replay` gives `compute.LedgerPosition` with `Contract`, `Right`, `Strike`, `Expiry`, `OpenedOn`, `ClosedOn`, `OpenPrice`, `Status` (open, closed, expired, assigned, exercised), `Fills` (day, action, quantity, price, commission), `Quantity`, `Multiplier`, `Currency`, `RealizedPnL`, `Early` (bool, declared in `internal/compute/api.go`: an early assignment). `cmd/ot/optionledger.go` maps a position to a stored option by underlying name, expiry, strike and right (`markKeyOf`, `markReaders.marks`).
- Stored quotes: `store.OptionEodReader.OptionEod(underlyingID, from, to)` returns `obs.option_eod` rows with `status`, `bid`, `ask`, `received_at`, `underlying_price`; the capture id is a uuid and says nothing about order. Chains exist from 2026-11-07. Rows before storage step 8 may be several a day and only for the contracts the capture planned; after step 8 (not merged) one row per contract and day; the loader handles both and relies on neither.
- Daily underlying closes: `bar.PriceBarReader.PriceBars(instrumentID, basis, from, to)` (`internal/store/bar`), opened by `a.deps.openBar` (line `OT_PG_DSN_BAR`).
- The rule: `shortput.Manage` is `hold` in `close_mode=expiry`; in `early` it closes with `early_keep` when `(OpenPrice - ask) / OpenPrice >= early_keep` at the ask of the quote whose `InstrumentID` equals `Position.Contract`, else with `early_dte` when the days to expiry are at most `early_dte`. It does not read the right.
- His own habit in the data (`vol-edge.md`): he often buys a put back before expiry (median about 23 days kept, 86% of the credit) while saying he mostly holds. The ledger's own close fills are therefore a priced early-close record of his, and are used as `actual`.

## Decisions

1. **The lot set is a parameter `lots`: default `short_puts` (owner 2026-11-13, "1. a"); the other value `short_lots` (every short lot, calls included) runs the same code.** The filter is the only difference: a lot is short when `Quantity` is negative; for `short_puts` its `Right` is `P`. Settlement is by right.
2. **(a) The hold record needs no quotes, and hold and actual are priced on one basis.** For every lot of the set the hold outcome is credit minus intrinsic value at the underlying close (`PriceBarReader`, basis `trades`) of the settle day; credit is his actual entry fill price. The settle day is the expiry day, and for a lot assigned before expiry the assignment day. His actual is on the same basis, gross per share, commission treated the same on both sides (left out of both, printed in net beside): for `held` and `closed_early` and `rolled` it is credit minus his buy-back price from `Fills` (a lot that expired worthless: the credit); for an assigned or exercised lot it is credit minus intrinsic value at the assignment-day close (the stock leg lives in the cycle, not in the position, so `RealizedPnL` is never the basis). Each lot carries one fate: `held` (expired worthless), `assigned_early` (`LedgerPosition.Early`), `assigned` (assigned or exercised on the expiry day, counted apart from `held` because it is a loss on the stock, not a kept credit), `closed_early` (a buy-back before expiry), `rolled` (a buy-back and a new short put on the same underlying the same day or the next session day with a later expiry), `open`. Totals per fate are over the lots hold can price (a settle close exists and the settle day is not after the as-of day); the others are printed as counts by reason (`not_expired`, `no_settle_price`, `split_in_life`). Lots of fate `rolled` are on their own row outside every total, and the output says "a rolled lot's actual does not include the lot it was rolled into". For a `held` lot the difference of hold and actual is printed as a check and must be zero. Early assignment is not otherwise modelled.
3. **(b) The early rules need quotes, so each variant is reported over its own priced set.** A variant is the `shortput` strategy with `early` and its two numbers (default `early_keep` 0.50 and `early_dte` 21 [S], more pairs by flags [T]). A lot is priced under a variant only when its hold is settled (Decision 2) and every session day from the entry day plus one up to the close day (or to the expiry when nothing closes) has an ok stored quote of the contract; a lot with a day missing is excluded with the reason `gap_before_close`, because a rule that could not look on that day cannot be said to have held. The variant's figures cover the lots priced under it; the count is printed beside every figure, and beside it the hold figure and his actual over the same lots, so one rule is always read on identical lots. Printed beside the lines: the number of lots still open, the number excluded by each reason, and as the first output lines the first and last day that quotes exist. Today this table is expected to be empty or nearly: that is the honest result of the stored data, not an error. No common set across variants; nothing of (a) is dropped because (b) cannot price it.
4. **A minimum count, [T] 20 lots.** A line over fewer lots is printed with the count and the words `under 20 [T]: no conclusion`; the number is a trial value of the command (`--min-lots`), not a source's.
5. **The walk and a missing quote.** For each lot and variant the walk visits the session days after the entry day up to the expiry: the snapshot carries the day's stored ok quote of the contract (`Position.Contract` is the stored instrument id text) or no quote; `Manage` answers; the first `close` closes the lot at that day's ask. A day without an ok quote is a gap, counted; it is never filled by a mid, a last, a neighbouring day or a model. Reasons: `gap_before_close` (Decision 3: a day without a quote anywhere from the entry day plus one to the close day, the close day included); `no_instrument` (no stored option matched the lot); `no_settle_price`; `not_expired`; `split_in_life`. Only after the settle day does a held lot settle.
6. **`early_dte` does not fire for a lot opened inside the window.** The replayer reads `Strategy.Params().Values["early_dte"]` and, for a lot whose days to expiry on the entry day were already at most that number, ignores a `close` with reason `early_dte` (the lot is held under that clause; `early_keep` still applies). Otherwise the rule would close it the day after entry. A strategy without the key is not affected.
7. **The stored quote of a day, and which slot it is.** Per contract and day the loader takes the ok row with the latest `received_at`; two rows with the same `received_at` are taken by the larger capture id (a fixed, written tie-break), never by id order as time. Rows before and after storage step 8 are handled by the same rule. The stored `option_eod` quote is the daily snapshot taken 90 minutes after the open (`option-chain-daily.md`, anchor `session_open`, offset `90m`), a morning quote, not a close; the walk's rule therefore acts on a morning ask and the settle uses the close, and the report header says so.
8. **Which days may be used.** Entry is his fill, not a stored quote. Quotes exist from 2026-11-07, so most lots opened earlier have none after entry and the early variants price few or none of them; the first output lines name the first and last stored quote day and the lots with at least one stored quote after entry against lots in the set. A lot is excluded with `split_in_life` when the ratio of the `adjusted_last` close to the `trades` close of its underlying differs between its entry day and its settle day (a split or other adjustment inside its life makes strike and close unequal bases); the count is printed. A model for the missing marks is Left out. A past day is replayed with the settings as stored today unless as-of settings exist; they do not (plan v2.5): `## Left out`.
9. **Gross and net.** The comparison uses gross per share (Decision 2): his credit minus the buy-back price or minus intrinsic value, so the rule and his actual are priced alike and commission is on neither side; his net per share (`RealizedPnL` over quantity and multiplier, commissions inside, assigned lots without the stock leg) is printed beside it as information only. Money is per currency, never summed across currencies.
10. **Where the code sits.** Own glue behind an interface so a library can replace it: the pure walk is `internal/strategy/replay` behind `internal/strategy/replayapi` (kept because AGENTS.md wants an interface file and a contract test from both sides for what other modules use, and `cmd/ot` is the only user; the root `api.go` of `strategy` is not edited). The loader and the command are in `cmd/ot` (ot module), in new files; the loader reads through the stores' own interfaces. It uses `markKeyOf` of `cmd/ot/optionledger.go` in place, one copy, only as the key that names a contract; `markKeyOf` and `markReaders.marks` find the newest quote of open lots only, so resolving every lot's option over its date range (the instrument ids by underlying name, expiry, strike and right through the catalog, terms and names readers, then `OptionEod` per underlying from the first entry to the last expiry) is new code of issue 3, `strategy_replay_market.go`. The command reads with the same lines and roles as `ot ledger` (`cmd/ot/optionledger.go:225-265`): `OT_PG_DSN_OPTION_LEDGER` for the trade rows, `OT_PG_DSN_PAGES` (role `ot_ro_pages`) for the option rows and instruments, `OT_PG_DSN_RUNNER` for the session days (as `ledgerSessions` reads them), `OT_PG_DSN_BAR` for the bars, all read-only. Own implementation because no mainstream library replays a user-supplied rule over stored quotes; about 250 lines.
11. **Runnable alone, idempotent, products as page actions.** The command reads and writes nothing; the same stores give the same text (map order is never printed; lots sort by entry day, then contract). A page is a later entry (Left out); it would render the `Report` this produces, the front end computing nothing.
12. **Not a headline.** No win rate and no Sharpe (negatively skewed returns, plan section 5). Printed: lots, kept share of credit (sum over sum), mean and worst per share, closed early, held, in the money, distinct expiry cycles (overlap makes lots not independent [NF]).

## Modules

One entry cell changes in `docs/design/modules.md` (this design carries the edit): row `strategy`, entry cell gains `internal/strategy/replayapi` (declarations only). `internal/strategy/replay` is an implementation package under the existing `internal/strategy/**` glob. No row is added; `jobs` is not touched; `ot` already depends on `strategy`, `store`, `compute`. `cmd/ot/wire.go` (796 of 800) is not edited: the command registers in its own file by `init`, and opens stores through the existing `deps` openers (`openOptionLedgerStore`, `openOptionEod`, `openOptionCatalog`, `openOptionTerms`, `openInstrumentName`, `openBar`, `openSessionDay`, `newOptionLedger`).

## Interface

Module strategy, `internal/strategy/replayapi/api.go` (declarations only; imports `contracts` and `strategy`):

```go
// Replayer walks each lot day by day under each variant. Pure: no I/O, no clock.
type Replayer interface {
	Replay(lots []Lot, mkt Market, variants []Variant, sessionDays []contracts.Day, asOf contracts.Day, minLots int) Report
}

type Variant struct {
	Name     string // "early_50_21"
	Strategy strategy.Strategy
}

// Lot is one short lot. Money is decimal text per share; days YYYY-MM-DD.
type Lot struct {
	ID, Underlying, Right, Strike, Currency, Multiplier string
	InstrumentID string        // stored option instrument id text, empty when none matched
	Expiry, EntryDay contracts.Day
	Credit       string        // his entry fill price
	Fate         string        // held, assigned, assigned_early, closed_early, rolled, open
	SettleDay    contracts.Day // expiry, or the assignment day of an assigned_early lot
	ActualGross  string        // credit minus his buy-back price; for an assigned lot credit minus intrinsic at the assignment-day close; empty while open
	ActualNet    string        // realized over quantity and multiplier; empty while open
}

// Market holds the day's ok quote by instrument id and the daily close by underlying name; an absent day is absent.
type Market struct {
	Quotes map[string]map[contracts.Day]Quote
	Closes map[string]map[contracts.Day]string
	AdjustedCloses map[string]map[contracts.Day]string // basis adjusted_last, for the split check
}
type Quote struct{ Bid, Ask string }

// Outcome is one lot under one variant ("hold" is the variant that never closes).
type Outcome struct {
	LotID, Variant string
	Priced         bool
	Reason         string // not priced: no_instrument, gap_before_close, no_settle_price, not_expired, split_in_life
	ClosedOn       contracts.Day
	CloseReason    string // an Action.Reason word or "expiry"
	Gross          string
	InTheMoney     bool
}

// Line is one printed row: one variant (or "actual", or "hold") over one set of lots in one currency.
type Line struct {
	Set, Variant, ParamHash, Currency string // Set: "all" for the hold record, "priced:<variant>" for an early variant, "rolled" for the rolled lots outside any total
	Lots, Cycles, ClosedEarly, Held, Assigned, Rolled, InTheMoney int
	KeptShare, MeanGross, WorstGross string
	HoldKeptShare, ActualKeptShare string // the same lots, so a rule is read against hold and against him
	UnderMinimum bool
}

type Report struct {
	Lots, LotsWithQuote, StillOpen, LotDays, QuoteDays int
	FirstQuoteDay, LastQuoteDay contracts.Day // the stored quote days the walk could use; empty when none
	Excluded map[string]int // reason -> lots, per variant name prefixed
	Outcomes []Outcome
	Lines    []Line
}
```

## Issues

Order: 1 first; 2 after 1; 3 after 2 (it wires the engine in and its tests may use a fake `Replayer` only for the printing).

| # | entry | module | size | order |
|---|---|---|---|---|
| 1 | replay-api (interface) | strategy | S | first |
| 2 | replay-engine | strategy | M | after 1 |
| 3 | ot-strategy-replay | ot | L | after 2 |

### 1 replay-api (interface)

Module: strategy. Size: S.
Order: first.
Files: `internal/strategy/replayapi/api.go`, `internal/strategy/replayapi/api_test.go`, `docs/contracts/strategy.md`
Design: docs/design/strategy-replay.md#Interface (Decisions 2, 3, 5, 10)
New package; `docs/design/modules.md` carries the entry cell and the issue does not edit it.
Acceptance:
1. `api.go` declares `Replayer`, `Variant`, `Lot`, `Market`, `Quote`, `Outcome`, `Line` and `Report` with the fields of the Interface section and nothing else: no function body, variable or constant; it imports only `contracts` and `strategy`.
2. `api_test.go` has the new tests `TestReplayerFromBothSides` (a fake `Replayer` on one side, a call with a `Market` that lacks a day on the other, every field of `Lot`, `Outcome`, `Line` and `Report` written and read back) and `TestReplayapiImportsNoStoreJobsPages` (new; fails on an import of `internal/store`, `internal/jobs` or `internal/pages` in a non-test file of the package).
3. `docs/contracts/strategy.md` states that an absent day of `Market.Quotes` is no quote and is never filled, the five `Outcome.Reason` words (`no_instrument`, `gap_before_close`, `no_settle_price`, `not_expired`, `split_in_life`), the six `Lot.Fate` words, that a `Line` covers the lots priced under its variant and carries the hold and actual figures of the same lots, and that the replayer writes nothing.
4. The change edits only the paths of this entry's Files; the pull request body carries `Contracts: unchanged` and a `Design:` line.
5. The result matches docs/design/strategy-replay.md, Interface.

### 2 replay-engine

Module: strategy. Size: M.
Order: after 1.
Files: `internal/strategy/replay/replay.go`, `internal/strategy/replay/walk.go`, `internal/strategy/replay/report.go`, `internal/strategy/replay/replay_test.go`, `internal/strategy/replay/report_test.go`, `internal/strategy/replay/testdata/walk.json`
Design: docs/design/strategy-replay.md#Interface (Decisions 2, 3, 4, 5, 6, 8, 9, 11, 12)
Own implementation behind `replayapi.Replayer`, standard library only (`math/big`), because no mainstream library replays a user-supplied rule over stored quotes.
Acceptance:
1. `replay.New()` returns a `replayapi.Replayer`; the non-test files import only the standard library, `contracts`, `compute`, `strategy` and `strategy/replayapi`, and call no clock, file or network function.
2. `walk.go` visits for each lot and variant the session days after `EntryDay` up to `Expiry`, builds a `strategy.Snapshot` whose `Quotes` holds one `compute.OptionQuote` (`InstrumentID`, `Bid`, `Ask`, `QuoteOK` true) when `Market.Quotes` has the day and none otherwise, calls `Manage` with a `strategy.Position` whose `Contract` is `Lot.InstrumentID`, and stops at the first `close`.
3. A close is priced at that day's ask; no close settles at intrinsic value from `Market.Closes` on `Lot.SettleDay`, by right, floor zero; no function in the package reads a mid, a last, a neighbouring day or a model value.
4. Reasons: a lot priced under a quote-needing variant requires an ok quote in `Market.Quotes` on every session day from `EntryDay` plus one to the close day (to `Expiry` when nothing closes), else `gap_before_close`; an empty `InstrumentID` gives `no_instrument`; a missing close on `SettleDay` gives `no_settle_price`; `SettleDay` after `asOf` gives `not_expired`; a lot whose `AdjustedCloses` over `Closes` ratio differs between `EntryDay` and `SettleDay` gives `split_in_life`; the variant `hold` is not made unpriced by a missing quote.
5. A `close` with reason `early_dte` is ignored when the lot's days to expiry on `EntryDay` were at most `Strategy.Params().Values["early_dte"]` (absent key: no effect).
6. `Lines` has the `all` set rows `hold` and, per `Fate` (`held`, `assigned`, `assigned_early`, `closed_early`), `actual`, each total over the lots hold can price, and for `held` a check row whose difference of hold and actual is computed; lots of `Fate` `rolled` are one `rolled` row outside every total; each early variant has one row over the lots priced under it with `HoldKeptShare` and `ActualKeptShare` over those same lots; a row with `Lots` below `minLots` has `UnderMinimum` true; `Report.StillOpen`, `FirstQuoteDay`, `LastQuoteDay` and `Excluded` by reason are filled; no figure sums two currencies; `Cycles` counts distinct `(Underlying, Expiry)`.
7. The report is a function of the arguments: outcomes sort by lot id then variant, and no map is iterated into an output order.
8. The new tests `TestHoldNeedsNoQuote`, `TestEarlyKeepClosesAtAsk`, `TestEarlyDTEIgnoredForLotOpenedInsideWindow`, `TestGapBeforeCloseIsUnpriced`, `TestNotExpiredIsNotMissingPrice`, `TestAssignedItmLotActualEqualsHoldBasis`, `TestRolledLotsOutsideTotals`, `TestSplitInLifeExcluded`, `TestEarlyLineCoversOwnPricedLotsWithHoldBeside`, `TestUnderMinimumFlag`, `TestReportIsDeterministic` (all new) exist in `replay_test.go` and `report_test.go`, each with `t.Parallel()`, over the new `testdata/walk.json` (no real account data, one assigned in-the-money lot among them) and a `shortput` variant of fixed parameters.
9. The change edits only the paths of this entry's Files; the pull request body carries `Contracts: unchanged`.
10. The result matches docs/design/strategy-replay.md, Decisions 2 to 6, 8, 9 and 12.

### 3 ot-strategy-replay

Module: ot. Size: L.
Order: after 2
Files: `cmd/ot/strategy_replay.go`, `cmd/ot/strategy_replay_lots.go`, `cmd/ot/strategy_replay_market.go`, `cmd/ot/strategy_replay_print.go`, `cmd/ot/strategy_replay_test.go`, `cmd/ot/strategy_replay_lots_test.go`, `cmd/ot/strategy_replay_market_test.go`, `cmd/ot/env_test.go`, `docs/contracts/ot/strategy-replay.md`
Design: docs/design/strategy-replay.md#Issues (Decisions 1, 2, 7, 8, 10, 11)
The command registers by `init` in its own file; `cmd/ot/wire.go` is not edited and `cmd/ot/optionledger.go` is not edited (its `markKeyOf` is called in place, one copy). `env_test.go` gains a `leakExcluded["strategy-replay"]` entry (not a `leakRows` row) with the reason "the command opens no database in the leak deps; its open errors are fixed texts and TestStrategyReplayOpenErrorWithheld covers them", as `strategy-compare` has. No same-package test file uses an identifier this issue changes.
Acceptance:
1. `strategy_replay.go` registers `strategy-replay` with `register` in an `init`, parses `--lots` (`short_puts` default, `short_lots`; another value is an error naming both), `--alias`, `--as-of`, `--early-keep`, `--early-dte`, `--min-lots` (default 20), and returns an error for the demo profile as `strategy_compare.go` does.
2. `strategy_replay_lots.go` builds trades from `FlexTradeRows` as `positions_page/builder.go` does, replays them with `a.deps.newOptionLedger()`, keeps the lots of Decision 1, sets `Fate` and `SettleDay` by Decision 2 (`assigned_early` from `LedgerPosition.Early`; `rolled` when a buy-back and a new short put of the same underlying with a later expiry open on the same day or the next session day), and sets `ActualGross` (assigned lots: credit minus intrinsic at the settle-day close) and `ActualNet` from `RealizedPnL`, both empty while open.
3. `strategy_replay_market.go` resolves every lot's stored option over its date range (new code; `markKeyOf` of `optionledger.go` is called in place for the key, not copied, and its `marks` function is not reused), reads `OptionEod` per underlying from the first entry to the last expiry, keeps per contract and day the row with status ok and the latest `received_at` (equal times: the larger `capture_id`), reads the `trades` and `adjusted_last` closes of entry and settle days through `bar.PriceBarReader` into `Closes` and `AdjustedCloses`, and puts no value in `Market` for a day the store lacks.
4. `strategy_replay_print.go` prints first the first and last stored quote day, then the header (lots set, as-of day, variant names with their `Params.Hash`, the lines "early assignment is not modelled; a day without a stored quote is a gap, not a guess", "the stored quote is the morning snapshot 90 minutes after the open; settlement is the close" and "a rolled lot's actual does not include the lot it was rolled into"), the coverage counts, the still-open count, the `Excluded` counts by reason, then one row per `Line` with its lot count beside each figure and the words `under 20 [T]: no conclusion` when `UnderMinimum` is set; it iterates no map.
5. Nothing is written to a store: the file set imports no writer interface and calls no `Insert`, `Upsert` or `Record` method; it opens the stores with `OT_PG_DSN_OPTION_LEDGER`, `OT_PG_DSN_PAGES`, `OT_PG_DSN_RUNNER` and `OT_PG_DSN_BAR` only, as `cmd/ot/optionledger.go:225-265` does; an open failure prints a fixed text naming the line and never the DSN.
6. The new tests `TestStrategyReplayDefaultsToShortPuts`, `TestLotsFateHeldAssignedClosedRolled`, `TestLotsAssignedUsesIntrinsicNotRealized`, `TestMarketPicksLatestReceivedAtThenCaptureID`, `TestMarketLeavesAMissingDayAbsent`, `TestStrategyReplayPrintsCountsBesideFigures`, `TestStrategyReplayPrintsRolledNote`, `TestStrategyReplayWiresEngineAndShortPutVariants`, `TestStrategyReplayDemoRefused`, `TestStrategyReplayOpenErrorWithheld` (all new) exist, each with `t.Parallel()`, over fake openers; the wiring test builds the real `replay.New()` and `shortput.New` variants (`expiry`-mode hold is the engine's own `hold`, the `early` variants come from the flags) and checks the variant names and `Params.Hash` printed; no database, no sleep.
7. `docs/contracts/ot/strategy-replay.md` states the flags, the output and that nothing is written.
8. The change edits only the paths of this entry's Files; the pull request body carries `Contracts: unchanged`.
9. The result matches docs/design/strategy-replay.md, Decisions 1, 2, 7, 8 and 10.

## Checked by CI

`go build ./...`, `go test ./internal/strategy/... ./cmd/ot/... -race -count=1`, `go run ./scripts/arch` (`strategy/replay` imports only root `strategy`, root `compute`, `strategy/replayapi`, `contracts`; `scripts/arch` finds `replayapi` as a declarations-only package from the tree and does not read the entry cell), `golangci-lint`, the size ratchet. What only a run shows: how many of his short-put lots have any stored quote after entry, read by the window from the first `ot strategy-replay` (read-only); it decides what part (b) may claim.

## Left out

- **As-of settings** (plan v2.5): settings, buy price and watch list of a past day are not stored per day; a past day is replayed with the settings as stored today.
- **A model for missing marks** (Black on a forward from put-call parity, `option-capture-need.md`): it would invent the numbers this design refuses to invent; needs the owner's word and a library decision.
- **Replaying the entry rule over stored days** (`strategy.result` proposals as cases, or `Evaluate` with other parameters): most proposals since 2026-11-07 have not expired, and re-evaluation needs the job's snapshot builder as a shared piece. Later, when the days have expired.
- **Fill models** beyond his own fills (mid less a share of the spread [V] Spintwig) and early assignment.
- **A page and a stored result** (a section of the candidates page; a table needs a migration the owner runs). The `Report` is the data it would render; the command is the first face.
- **Choosing the rule**: the report prints the record; the default stays hold to expiry [O].
- Other strategies (covered call, spreads, condor, strangle) use the same `Replayer` once ported. Result by market state tag, and a cross-currency total.

## Doubts

None. Printing before the quotes are deep was decided by the dispatcher (2026-11-14): print now, counts first, since part (a) is complete today and every early figure carries its count and the under-20 flag.

## Adversarial

Answers to the adversarial read `replay-adversarial.md` in the dispatcher window's scratchpad (points as the coordinator listed them).

1. **Split by data need.** Adopted: Decisions 2 and 3 and the Goal: hold against his actual on all lots without quotes; early rules on their own priced sets with counts and a minimum [T] 20 (Decision 4); no common-set rule.
2. **Drop `--source results` and the jobs package.** Adopted: no results source, no `internal/jobs/strategyreplay`; the loader and command are in `cmd/ot`; three issues. `replayapi` is kept (Decision 10: the interface rule of AGENTS.md for a module other code uses).
3. **False claims.** Adopted: newest row is by `received_at` with a written tie-break (Decision 7), not by capture id; `bar.PriceBarReader`; step 8 is not relied on (Today, Decision 7).
4. **Files.** Adopted: `cmd/ot/env_test.go` is in the Files of issue 3; the command is in new files; `wire.go` is not edited at all (the openers exist, including `openBar`).
5. **Like with like.** Adopted: fate per lot, actual gross beside net, `early_dte` ignored for a lot opened inside the window (Decisions 2, 6, 9). One part not adopted as worded: a rolled lot is compared with its actual close, as asked, but the replay does not follow the roll into the new lot; that lot is its own lot in the set.
6. **`markKeyOf` once.** Adopted: called in place from `cmd/ot/optionledger.go`, not moved, not copied (Decision 10, issue 3 item 3).
7. **Acceptance checkable by reading.** Adopted: runtime claims were removed or tied to a new test name written in the same item; where an item says a function reads or calls nothing, it is a read of the imports.

### Round 2 (advisor review r1)

1. **B1 assigned lots.** Adopted (Decision 2, Interface `Lot`, issue 2 item 8, issue 3 items 2 and 6): actual and hold are on one basis, credit minus intrinsic at the settle-day close for an assigned lot; fates `assigned` and `assigned_early` with their own rows and counts; commission is on neither side and net is printed beside it; new tests `TestAssignedItmLotActualEqualsHoldBasis` and `TestLotsAssignedUsesIntrinsicNotRealized`.
2. **B2 priced set of a variant.** Adopted (Decisions 3, 5, 7, 8; issue 2 items 4 and 8): priced only when hold is settled and every session day from the entry day plus one to the close day has an ok quote, else `gap_before_close`; still-open and excluded counts and the first and last quote day are printed first; an empty table today is stated as the honest result; new test `TestGapBeforeCloseIsUnpriced`. The stored slot (90 minutes after the open) is named in Decision 7.
3. **B3 rolls and totals.** Adopted (Decision 2, issue 2 items 6 and 8, issue 3 items 4 and 6): totals per fate only over lots hold can price, exclusions by reason with counts, rolled lots on their own row outside every total, the printed note about the rolled lot, `Line` carries `Assigned` and `Rolled`; new tests `TestRolledLotsOutsideTotals` and `TestStrategyReplayPrintsRolledNote`.
4. **B4 splits.** Adopted (Decision 8, `Market.AdjustedCloses`, issue 2 items 4 and 8, issue 3 item 3): `split_in_life` when the adjusted-to-trades ratio differs between entry day and settle day, counted; new test `TestSplitInLifeExcluded`.
5. **B5 entry 3 order and wiring.** Adopted: `Order: after 2` on its own line, and issue 3 item 6 names the wiring of `replay.New()` and the `shortput` variants into the command with new test `TestStrategyReplayWiresEngineAndShortPutVariants`; no fourth entry.
6. **Named, not blocking.** Done: the DSNs and roles are named read-only (Decision 10, issue 3 item 5); resolving a contract over a date range is new code of issue 3 `strategy_replay_market.go`, not `markKeyOf` (Decision 10, item 3); `env_test.go` takes a `leakExcluded` entry with its reason; the stored slot is in Decision 7.

