# strategy risk, step 3: the book stress at the FINRA moves, and the capital fit of every candidate

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-14
Revision: 4
Revision note: revision 2 answers the adversarial read of revision 1: the types sit in the root package `internal/strategy` and `cmd/ot` injects the implementation (Decision 11); the loader is described as `risk_page/load.go` really reads (Decision 4); no "cushion" is printed from `ExcessLiquidity` (Decision 5); cash-settled European options are never assigned (Decision 6); a stale mark is a reason word (Decision 3); partial totals are "incomplete" (Decision 4); the FINRA ladder per class replaces the joint scenarios and the concentration check is out (Decisions 2 and 7, Left out).
Revision 3 answers the second read (advisor notes r1): the output sentence and the (g)(7) and (g)(8) sources (Decisions 1 and 2); `no_capture` for an alias without a capture (Decision 4); totals per alias and currency, no number shown unless complete, the Doubt removed (Decisions 2 and 4); `open_put_strikes` and the words of `fit_capital` (Decision 8); the oldest spot day per class (Decision 2); the job-console list (entry 5); row identity and status (Decision 12); intrinsic valuation (Decision 3); index options told apart by the class table (Decision 6).

Source: owner 2026-11-13 (record `optdesk-owner-2026-11-13.md`, P2 to P4): every rule from professional practice with a source, every number tagged; the three accounts are `mine`, `wife`, `daughter`; advisor plan `option-strategy-plan.md` section 6 step 3, with sections 2 (rows "Portfolio-margin stress", "Cash-secured put ... Short broad-index option", "Buying power of a short strangle"), 3, 4 (capital column), 8 and 9.
Tags: **[V]** read on the source, **[S]** snippet or memory of a secondary page, not confirmed, **[O]** the owner's own decision as recorded in the repo, **[T]** trial: our choice, **[NF]** looked for, no source found. The tags are the advisor's and the adversarial reader's; this design re-read no source.
Existing design: `strategy-framework.md` (module `strategy`, `strategy.result` r1; its "Left out" names this step), `option-positions.md`, `option-ledger.md`, `position-marks.md`, `account-data.md`, `account-identity.md`, `option-candidates.md` (the `funding` gate of `put_v1`). Nothing of them changes behaviour.

Goal: (1) one pure stress of an alias's open positions over the FINRA portfolio-margin ladder, per alias and for the household; (2) one pure capital fit, which says for every ok candidate and every alias with funds what capital it needs by the exchange formulas, and what it adds to the book's worst loss; (3) a job that stores both in `strategy.result` beside the candidates, so a page later shows "worst loss of the book X, cash needed for assignments Y" and what a candidate adds. No page here, no new table, no new source or secret.

## Today

All paths at origin/main 832c799b.

- Open positions: `compute.OptionStructures` groups one alias's positions into `Structure` values (`Legs` with `Strike`, `Expiry`, `Quantity`, `Multiplier`, `Mid`, `MarkStatus`). Its input is built by `internal/pages/risk_page/load.go` (`load`): ledger trade rows (`FlexTradeRows`) go through `Ledgers().Replay` with `MarkMaxAgeSessions` 5 and no marks, giving `Positions` and `Cycles`; `capture` reads `Account.Positions(alias, day)` of the newest of the last session days and keeps the rows with `SecType` OPTION as `Held` and STOCK as stocks with their `ConID`; `quotes` joins each position to a chain row of `obs.option_eod` through the stored option terms (`store.OptionTerms`, which carries `ExerciseStyle` and `SettlementType`) and returns `StructureQuote` with bid, ask, `UnderlyingClose`, delta, gamma, theta, vega of one day. So an option's price today is the chain mid of a day and the spot beside it is that same row's `UnderlyingClose`. A job cannot import a page.
- Store `store/mark` (`account.position_mark`: `Price`, `Basis`, `Status`, `MarkedAt`, keyed by `ConID`) is not read by `risk_page`; only the stock lines would use it.
- Funds: `Account.Values(alias, day)` tags NetLiquidation, TotalCashValue, AvailableFunds, ExcessLiquidity, MaintMarginReq, InitMarginReq (and others), each row with a currency. The `daughter` alias comes from Flex only (owner O4): ledger positions and no `account.value` rows.
- Candidates: `strategy.result` rows of strategy `short_put` (`legs` JSON; `facts` `spot`, `iv`, `delta`, `bid`, `ask`, `underlying_id`; `internal/strategy/shortput/map.go`), read by `strategyresult.Reader.Results(day, strategy)`, written by `Writer.InsertResults`.
- `funding` exists inside the put rule (`strike x multiplier` at most `AvailableFunds`); it stays and this step adds nothing to `put_v1` rows.
- Jobs import only the root package of `strategy` (`docs/design/modules.md`, interface cell of row strategy: `internal/strategy/api.go`); the short put is built in `cmd/ot/strategy_wire.go` and handed to the job. `internal/compute/api.go` is at its ceiling of 840 lines and is not touched.

## Decisions

1. **The moves are FINRA's, not ours.** FINRA Rule 4210(g)(2)(F) [V]: the portfolio-margin valuation uses ten equidistant intervals (valuation points) over the range of each product class; the range is +6 percent / -8 percent for a high-capitalization broad-based index, +-10 percent for a non-high-capitalization broad-based index, +-15 percent for equity, narrow-based index and other equity-based products. The class of an underlying is a table of the job manifest (`[[class]] symbol, class`; owner config [O]): SPX, XSP, NDX `broad_high`, RUT `broad_other`, every other symbol `equity`; that RUT is "other broad" and NDX "high cap" is [S] (footnote 2 of the rule defines high capitalization through SEA Rule 15c3-1a(b)(1)(i)(B), not read). Japanese and other non-US names are not in the rule's scope; they are stressed at the `equity` range, +-15 percent [T], and the row says so (`range_basis` `finra_equity_applied`). The owner trades SPX, XSP and the Russell 2000 index, not NDX (P5 [O]). The rule's per-contract minimum (a floor under each requirement) is not applied: this is a scenario, not a requirement.
2. **The figure is the largest loss of the ten intervals, per class, summed over classes.** Rule 4210(g)(7) [V]: the requirement of a portfolio is the greater of the largest theoretical loss among the valuation points and the per-contract minimum; (g)(8) [V]: positions are grouped by class into portfolios, netted fully inside a portfolio at each point, and the total is the sum of the greatest loss of each portfolio, with offsets between portfolios only as SEA Rule 15c3-1a allows. So here: for each class every underlying of that class moves by the same percentage at each point, the profit and loss of the alias's holdings of that class and currency is summed, the class's figure is the worst (lowest) point, and the total `worst_loss` is the sum of the class worsts with no offset between classes [V, (g)(8)]. The points lie on ten equal intervals of the class's range: eleven points from the low end to the high end, ends included, `low + k * (high - low) / 10` for k = 0 to 10; for the asymmetric +6/-8 range the step is 1.4 percent and the points are -8, -6.6, ..., +6, and the zero move is not forced onto the grid (that the rule's wording means ten intervals and not ten points is [S]; the intervals reading is used). The points and their profit and loss are stored (`ladder_<class>`), the worst of each class with its move (`worst_<class>`), and the oldest spot day among the class's holdings (`oldest_spot_day_<class>`) is stored and printed beside the figure. A volatility shift has no standard [NF] and is not modelled. **Every page that prints a figure prints this sentence: "this is the broker-dealer portfolio-margin ladder used as a sourced scenario, not your account's margin."** The sentence is a fact of the row (`basis_note`), so the page words nothing.
3. **Revaluation, not a Taylor sum.** At +-15 percent a delta-gamma sum misprices a short option, so each option leg is repriced at the moved spot with Black-Scholes-Merton and the volatility implied by the leg's own price (bisection on `Mid`). Both the price and the spot of a leg come from the same chain row, so they are of one day; the leg is rejected as `stale_mark` when that row's day is older than `max_mark_age_sessions` sessions before the run day (the value `risk_page` uses, 5 [O]) or when the leg's status is not a usable mark; shares valued from `store/mark` are rejected as `stale_mark` when `Status` is not ok, `Basis` is `none`, or `MarkedAt` is of an earlier session day than the run day. A leg whose price is at or below the intrinsic value at the spot of its row (an American in-the-money put quoted under the European intrinsic, so no volatility can be solved) is valued at the intrinsic value at the moved spot, not excluded [T]: it is the leg's lower value bound at that spot and it is the leg most at risk, so dropping it would hide the loss; a leg with no price at all is `no_vol`. A rejected holding is listed, never dropped silently. Rate is DGS3MO as the put rule reads it; dividend yield zero [T]; the shock is instantaneous, no time passes [T]. American equity options are priced as European, which understates a short put's loss a little [S]; index options are European [V, Cboe index-options page]. Own implementation, because no mainstream maintained Go option-pricing library with a permissive licence and few dependencies is known to the advisor [NF]; it sits behind the `Pricer` interface of package `stress` so a library can replace it (owner 2026-11-08). `float64` and `math.Erf` are used only inside package `bsm`; every sum of profit and loss in `stress` and every capital in `fit` is `math/big` `Rat` (as `internal/compute/optionstruct/rat.go` does), and every text that leaves is rounded to cents.
4. **What is stressed, and what the loader reads.** Per alias: its open option legs from `OptionStructures`, built by the same sequence as `risk_page/load.go` (ledger rows, `Replay`, capture of OPTION and STOCK rows, `quotes` through the option terms), plus its shares. Shares are the STOCK rows of the newest capture of the alias (quantity times the spot of the symbol: the `UnderlyingClose` of a chain row of that symbol when there is one, else the mark of `store/mark` joined to the STOCK row by `ConID`; this join is new in this job). An alias that has no capture (today `daughter`) has no STOCK rows: if its ledger carries stock positions they are stressed from the ledger, otherwise its shares are unknown and the row carries the exclusion `no_capture` and has no total; a capture position absent from the ledger is excluded as `not_in_ledger`. Other exclusions: `no_class`, `no_vol`, `stale_mark`, `expired`. **Totals are per alias and per currency.** A USD alias's total is in USD. A holding in another currency (a JPY option, a JPY share) with a class row and a mark is stressed in its own currency at the `equity` range (Decision 1) and gives a second total in that currency; without a class row or a mark it is not stressed and is counted by currency (`not_stressed_<currency>`). The USD total is complete on its own when no USD holding is excluded; a currency's total exists only when complete for that currency. The household row sums only rows of one currency and carries one total per currency. When a total is not complete the row stores no total at all: it stores each class's worst loss over the holdings that were stressed (`worst_<class>`, `ladder_<class>`) and the exclusion list with reasons and counts, and the page prints the class worsts and the list, never a number marked incomplete (an incomplete sum is not a bound: an excluded line may hold a loss or a gain).
5. **No invented cushion.** The broker's `ExcessLiquidity` already deducts the margin of the current shorts and the broker's requirement grows in a fall, so `ExcessLiquidity - loss` is not a cushion. Stored instead, labelled by the key name: `equity_loss_vs_excess_liquidity` = the complete total of the alias in the funds currency (absent when the total is not complete, Decision 4) as text beside `ExcessLiquidity`, with the fact `margin_change_not_modelled` = `true` [T: our reading; margin at stress is not modelled because the broker's model (TIMS, OCC) was not read, 403 [NF]]. No check passes or blocks on it. An alias with no `account.value` of the day or the day before (today `daughter`) has no such pair, reason `no_funds`.
6. **Assignment cash by exercise style.** In each ladder point, each short put with strike above the moved spot is assigned and needs `strike x multiplier x quantity`; the sum is `assignment_cash` of the worst point of its class. A leg is never assigned when it is European and cash settled. The job sets `RiskLeg.CashSettled` true when either `store.OptionTerms` says `ExerciseStyle` European and `SettlementType` cash, or the underlying's class is `broad_high` or `broad_other` (the owner's class table of Decision 1, which lists SPX, XSP, NDX and RUT: index options are European and cash settled [V, Cboe index-options page]). The columns `exercise_style` and `settlement_type` are written by `internal/store/instrument/resolve.go` (line 44) from the contract the resolver is given; its `insertTermsSQL` writes the literal `'unknown'` for both columns of every option it creates [V, read in `resolve.go`] (only the demo profile, `cmd/ot/demo_option.go:102`, sets `american` and `physical`). So the rule is: `unknown` or empty counts as assignable unless the class table names the underlying an index [V for the writer, O for the table]; the class table, not the terms, keeps SPX, XSP, NDX and RUT legs out of assignment cash, and an equity option counts as assignable [T: the cash need is then overstated, not understated]. SPX, XSP and RUT legs add to the loss and never to assignment cash.
7. **Capital of a candidate is the exchange's formula.** `Capital`: cash-secured short put: strike times multiplier times quantity [V, Cboe strategy-based margin page]; short equity option, uncovered: option value plus 20 percent of the underlying less the out-of-the-money amount, floor option value plus 10 percent of the underlying (puts: of the strike) [V for 20 percent and the 10 percent minimum as the plan records them; the OTM deduction and the put floor [S]]; short broad-index option: 15 percent of the index value less the out-of-the-money amount, minimum 10 percent [V, Cboe, FINRA 4210]; put credit spread: width less credit, times multiplier [arithmetic]; covered call: nil, the shares are the cover. A structure none of them covers has `Gap` `no_rule`. The account's real requirement is `MaintMarginReq` and `InitMarginReq` of `account.value`; the formula is for a candidate not yet in the account.
8. **Account fit is one check and facts, written as a result row.** For each ok candidate row of the day and each alias with funds, a row `fit`, key `<candidate key>:<alias>`, carries one check `fit_capital`: `Capital` at most `AvailableFunds` of the alias, else `block capital`. The check is against the broker's available funds, not against cash net of the puts already open. `RiskFunds` carries its `Currency` and the candidate its currency; a different one is `unknown no_fx` (as the put's `funding` gate does); no funds is `unknown no_funds`. Facts: `capital`; `open_put_strikes` (the sum of `strike x multiplier x quantity` of the alias's open short puts in that currency, so a page can say what the puts already open would take on assignment); `worst_loss_with` (the alias's `worst_loss` in the candidate's currency with the candidate's legs added to the same class ladder, repriced with the same `Pricer`, from the row's `spot`, `iv` and `legs`; absent with `no_vol` when the row has no `iv` and absent when the alias has no complete total in that currency, Decision 4). There is no concentration check (Left out).
9. **One job, runnable alone and idempotent.** `book_risk` runs once per XNYS session day, offset 45 minutes after the close against 30 for `option_candidates` [T]; it reads stores only; it writes `strategy.result` rows with `Strategy` `book_risk` (keys `<alias>:<currency>` and `household:<currency>`) and `fit`, insert-only under one run id per run; a rerun writes a new run id and readers take the newest run (`Reader.Results`). The identity and the status of the rows are Decision 12. With no candidate rows it writes book rows only; with no positions it writes zero rows and succeeds. It reads no live source and calls no broker.
10. **Front end computes nothing.** The facts carry every figure; their keys are in `docs/contracts/strategy.md`. A page is Left out.
11. **Where the code lives, and the module rule.** Other modules use a module through the root package only, so the types are declared in the existing interface file `internal/strategy/api.go` (an interface change: issue 1 carries the Design line), prefixed `Risk`, and the implementations are packages `internal/strategy/risk/bsm`, `risk/stress` and `risk/fit` that the job never imports. `cmd/ot` builds them and hands the job a `strategy.RiskStresser` and `strategy.RiskFitter`, as `cmd/ot/strategy_wire.go` hands over the short put. This needs no change of `docs/design/modules.md` (owner-only) and the fewest files: the other way, a declarations-only `internal/strategy/risk/api.go` named in the interface cell of the strategy row, costs a modules.md edit and a second interface file. The job never sees `Pricer`; it lives in `stress`. No file of `internal/strategy/replayapi` (design `strategy-replay`, PR 2307) is touched.
12. **Row identity and status in `strategy.result` r1 (no dataset change).** The dataset requires `underlying_id` as an integer and `status` ok, blocked or unknown (`contracts/datasets/strategy.result.r1.json`). A `book_risk` row has no underlying: `underlying_id` is 0 (the job's value for "none"; whether any underlying row has id 0 was not read [S], the key never joins on it) and `key` is `<alias>:<currency>` or `household:<currency>`. A `fit` row takes the `underlying_id` of its candidate row. Status: a `book_risk` row is `ok` when its total for that currency is complete, `unknown` when it is not (class worsts and exclusions stay in the facts), never `blocked`; a `fit` row is `ok` when `fit_capital` passes, `blocked` when it blocks, `unknown` otherwise. No contracts entry is needed and `Contracts: unchanged` holds.

## Modules

| module | change |
|---|---|
| strategy | `internal/strategy/api.go` (types), `risk/bsm`, `risk/stress`, `risk/fit`, `docs/contracts/strategy.md` |
| jobs | `internal/jobs/bookrisk` |
| ot | `cmd/ot/bookrisk.go` |

`strategy` imports `contracts` and `compute` only; the risk packages import neither store nor jobs. After the interface, entries 2, 3 and 4 are buildable in parallel; entry 5 follows 2, 3 and 4.

## Interface

Owner: module `strategy`, file `internal/strategy/api.go` (declarations added; existing ones unchanged). All numbers are decimal text.

```go
type RiskClass string // "equity", "broad_high", "broad_other"

// RiskLeg is the option part of a holding; CashSettled is true when the leg is European and cash settled.
type RiskLeg struct {
	Right, Strike, Expiry, QuoteIV string
	CashSettled                    bool
}

// RiskHolding is one open line: a share line (Option nil) or one option leg.
type RiskHolding struct {
	Underlying, Currency string
	Class                RiskClass // "" when the symbol has no class
	Option               *RiskLeg
	Quantity, Multiplier string // quantity negative when short
	Spot, Mark           string // Mark "" when unknown
	MarkDay, SpotDay     contracts.Day
	MarkUsable           bool // false for a status or basis that is not a usable mark
}

type RiskFunds struct{ Currency, NetLiquidation, Available, ExcessLiquidity string }

// RiskBook is one alias on one run day; Funds is nil with no account.value of the day or the day before.
type RiskBook struct {
	Alias, Currency string
	RunDay          contracts.Day
	Rate            string
	Holdings        []RiskHolding
	Funds           *RiskFunds
	HasCapture      bool // false for an alias with no capture: its STOCK rows are unknown unless the ledger carries them
	SessionsOld     func(contracts.Day) int // sessions between a day and RunDay, supplied by the caller
}

type RiskPoint struct{ Move, PnL, AssignmentCash string }
type RiskClassResult struct {
	Class  RiskClass
	Currency string
	Ladder []RiskPoint // eleven points on ten equal intervals
	Worst  RiskPoint
	OldestSpotDay contracts.Day
}
type RiskExcluded struct{ Underlying, Currency, Reason string } // no_class, no_vol, stale_mark, expired, not_in_ledger, no_capture

// RiskTotal is the sum of the class worsts in one currency, a loss as a positive number; it exists only when
// that currency is complete (nothing of it excluded).
type RiskTotal struct{ Currency, WorstLoss string }

// RiskOutcome is the stress of one book. Classes are per class and currency over the holdings stressed; Totals
// only for complete currencies; NotStressed counts holdings not stressed by currency.
type RiskOutcome struct {
	Alias       string
	Classes     []RiskClassResult
	Totals      []RiskTotal
	Excluded    []RiskExcluded
	NotStressed map[string]int
}

// RiskStresser revalues a book over the FINRA ladder, purely.
type RiskStresser interface{ Stress(RiskBook) RiskOutcome }

// RiskCandidate is the structure of one ok candidate row; Spot and QuoteIV come from its facts.
type RiskCandidate struct {
	Key   string
	Class RiskClass
	Legs  []RiskHolding
}

// RiskFit is the verdict of one candidate in one book: the check and the facts.
type RiskFit struct {
	Check Check // Name "fit_capital"
	Facts map[string]string
}

// RiskFitter is the capital fit, purely.
type RiskFitter interface{ Fit(RiskBook, RiskOutcome, RiskCandidate) RiskFit }
```

`SessionsOld` is the one injected function so the packages hold no calendar. Reason words: `no_class`, `no_vol`, `stale_mark`, `expired`, `not_in_ledger`, `no_capture`, `no_funds`, `no_fx`, `no_rule`, `capital`.

## Issues

### 1 risk-types (interface)

Module: strategy. Size: S.
Order: first.
Files: `internal/strategy/api.go`, `internal/strategy/api_test.go`, `docs/contracts/strategy.md`
Design: docs/design/strategy-risk.md#Interface
Acceptance:
1. `internal/strategy/api.go` gains exactly the names of the Interface section (`RiskClass`, `RiskLeg`, `RiskTotal`, `RiskHolding`, `RiskFunds`, `RiskBook`, `RiskPoint`, `RiskClassResult`, `RiskExcluded`, `RiskOutcome`, `RiskStresser`, `RiskCandidate`, `RiskFit`, `RiskFitter`) as declarations and doc comments only; every existing declaration of the file is unchanged.
2. `api_test.go` gains the new tests `TestRiskAPINewFakeSatisfiesStresserAndFitter` (a fake implements both interfaces, checked by assignment) and `TestRiskAPINewCallerUsesOnlyTheInterface` (a caller runs one `Stress` and one `Fit` against fakes), `t.Parallel()`.
3. `docs/contracts/strategy.md` gains a section "Book risk" that quotes Rule 4210(g)(2)(F) for the three ranges and the ten intervals, (g)(7) and (g)(8) for the largest loss and the sum over portfolios, each with its tag, states the sentence "this is the broker-dealer portfolio-margin ladder used as a sourced scenario, not your account's margin", lists the fact keys of the rows `book_risk` (`worst_loss_<currency>` only for a complete currency, `ladder_<class>`, `worst_<class>`, `oldest_spot_day_<class>`, `assignment_cash`, `excluded`, `not_stressed_<currency>`, `range_basis`, `basis_note`, `equity_loss_vs_excess_liquidity`, `margin_change_not_modelled`) and `fit` (`capital`, `open_put_strikes`, `worst_loss_with`), the key and status of each row (Decision 12) and the reason words; the rest of the file is unchanged.
4. No file outside the Files line changes; `Contracts: unchanged`.
5. The result matches docs/design/strategy-risk.md, Interface.

### 2 stress

Module: strategy. Size: M.
Order: after 1.
Files: `internal/strategy/risk/bsm/bsm.go`, `internal/strategy/risk/bsm/bsm_test.go`, `internal/strategy/risk/stress/stress.go`, `internal/strategy/risk/stress/ladder.go`, `internal/strategy/risk/stress/stress_test.go`, `internal/strategy/risk/stress/testdata/**`
Design: docs/design/strategy-risk.md#Decisions (1, 2, 3, 6)
Acceptance:
1. `bsm.New()` implements the `Pricer` interface declared in `stress` (`Value`, `Implied`, decimal text in and out); the package comment says "own implementation, because no mainstream Go library fits" and that `float64` is used only in this package; a price below intrinsic or an expired option is an error.
2. `stress.New(p Pricer)` returns a `strategy.RiskStresser`; `ladder.go` holds the three ranges as constants each with the comment "Rule 4210(g)(2)(F) [V]" (high cap +6/-8, other broad +-10, equity +-15) and builds the eleven points of ten equal intervals, `low + k * (high - low) / 10`; every sum in the package is `math/big` `Rat` and `float64` does not appear in `stress`.
3. `Stress` gives for each class and currency present the eleven points and the worst point with its `OldestSpotDay`, a `RiskTotal` (sum of the class worsts) only for a currency with no excluded holding, and `AssignmentCash` per point as the sum of `strike x multiplier x quantity` of short puts with strike above the moved spot whose `Option.CashSettled` is false; a holding of a non-USD currency with a class row and a mark is stressed at the `equity` range in its own currency, else counted in `NotStressed`.
4. A holding is excluded with the reason `no_class`, `expired`, `no_vol` (no price at all) or `stale_mark` (mark or spot day more than the caller's allowed sessions old by `SessionsOld`, `MarkUsable` false, or `MarkDay` different from `SpotDay`), and a book with `HasCapture` false and no ledger shares is excluded as `no_capture`; an option priced at or below its intrinsic value is valued at the intrinsic value at the moved spot and is not excluded; excluded holdings are in no sum.
5. `stress_test.go` has the new tests `TestStressNewShortPutWorstPointEqualsHandValue` (the expected number written in the test from the closed form), `TestStressNewWorstPointOfBroadHighIsTheMinusEightEnd`, `TestStressNewClassesAreSummedWithoutOffset`, `TestStressNewCashSettledIndexLegHasNoAssignmentCash`, `TestStressNewEmptyExerciseFieldsCountAsAssignable`, `TestStressNewStaleMarkIsExcludedNotPriced`, `TestStressNewMarkDayOtherThanSpotDayIsStale`, `TestStressNewIncompleteCurrencyHasClassWorstsAndNoTotal`, `TestStressNewDeepItmPutAtIntrinsicIsValuedNotExcluded`, `TestStressNewOtherCurrencyHasItsOwnTotal`, `TestStressNewUsdTotalIsCompleteWhileJpyIsNot`, `TestStressNewOldestSpotDayPerClass`, `TestStressNewElevenPointsOnTenEqualIntervalsForAsymmetricRange`, `TestStressNewNoCaptureAliasHasNoTotal`, `TestStressNewSharesOnlyBookIsLinear`; `bsm_test.go` has `TestBsmNewPutCallParity` and `TestBsmNewImpliedRoundTripsToFourDecimals`; all `t.Parallel()`, no clock, no file outside `testdata`.
6. The change edits only the paths of this entry's Files; new files under 600 lines; `Contracts: unchanged`.
7. The result matches docs/design/strategy-risk.md, Decisions 1 to 3 and 6.

### 3 fit

Module: strategy. Size: S.
Order: after 1.
Files: `internal/strategy/risk/fit/fit.go`, `internal/strategy/risk/fit/capital.go`, `internal/strategy/risk/fit/fit_test.go`, `internal/strategy/risk/fit/testdata/**`
Design: docs/design/strategy-risk.md#Decisions (7, 8)
Acceptance:
1. `fit.New(st strategy.RiskStresser)` returns a `strategy.RiskFitter`; `capital.go` holds `capital` with one comment per formula naming its tag and source from Decision 7 (cash-secured put, uncovered equity option, short broad-index option, put credit spread, covered call); a structure none covers gives the reason `no_rule`; all arithmetic is `math/big` `Rat`.
2. `Fit` returns the one check `fit_capital`: `capital` at most `Funds.Available` passes (the comparison is with available funds, the code comment says it is not cash net of open puts), over blocks with the reason `capital`, `Funds` nil is `unknown no_funds`, a `Currency` of the funds different from the candidate's currency is `unknown no_fx`; `Facts` holds `capital`, `open_put_strikes` (sum of `strike x multiplier x quantity` of the book's short put holdings in that currency) and `worst_loss_with`, the latter from `Stress` of the book with the candidate's legs added and absent when the book has no complete total in that currency; no check is made on the facts.
3. `fit_test.go` has the new tests `TestFitNewCashSecuredPutCapitalIsStrikeTimesMultiplier`, `TestFitNewIndexShortOptionUsesFifteenPercentLessOtm`, `TestFitNewEquityNakedUsesTwentyPercentWithTenPercentFloor`, `TestFitNewSpreadCapitalIsWidthLessCredit`, `TestFitNewCoveredCallNeedsNoCapital`, `TestFitNewNoFundsIsUnknownNotBlock`, `TestFitNewOtherCurrencyIsUnknownNoFx`, `TestFitNewCapitalOverAvailableBlocks`, `TestFitNewOpenPutStrikesSumsTheOneOpenShortPut`, `TestFitNewIncompleteBookHasNoWorstLossWith`, `TestFitNewWorstLossWithAddsTheCandidateToItsClass`; expected numbers written in the tests from the formulas; fake stresser; `t.Parallel()`.
4. The change edits only the paths of this entry's Files; `Contracts: unchanged`.
5. The result matches docs/design/strategy-risk.md, Decisions 7 and 8.

### 4 job-bookrisk-and-wire

Module: jobs. Size: M.
Order: after 1.
Files: `internal/jobs/bookrisk/job.go`, `internal/jobs/bookrisk/load.go`, `internal/jobs/bookrisk/rows.go`, `internal/jobs/bookrisk/job.toml`, `internal/jobs/bookrisk/job_test.go`, `internal/jobs/bookrisk/load_test.go`, `internal/jobs/bookrisk/testdata/**`, `docs/contracts/jobs/book_risk.md`
Design: docs/design/strategy-risk.md#Decisions (4, 5, 8, 9, 11)
Acceptance:
1. `job.toml` has `id = "book_risk"`, calendar `XNYS`, anchor `session_close`, `version = 1`, zone `America/New_York`, offset `45m`, `run_bound = "10m"`, `restart_sensitive = false` (the fields of the `option_candidates` manifest), `max_mark_age_sessions` with the comment that 5 is the value of `risk_page`, a `[[class]]` table (SPX, XSP, NDX `broad_high`; RUT `broad_other`) with a tag comment per number, and no `[[need]]` row.
2. The job holds a `strategy.RiskStresser` and a `strategy.RiskFitter` in its stores or config type and imports from `internal/strategy` only the root package; a grep for `internal/strategy/` with a sub-path, for `internal/pages` and for another `internal/jobs` package finds nothing in the job.
3. `load.go` builds one `strategy.RiskBook` per alias that has a ledger row or a capture, in the sequence of `internal/pages/risk_page/load.go` (ledger rows, `Replay` with the structure marks rule, capture of OPTION and STOCK rows, quotes through `store.OptionTerms`), groups with `compute.OptionStructures`, sets `RiskLeg.CashSettled` true when `ExerciseStyle` is European and `SettlementType` cash or the underlying's class is `broad_high` or `broad_other`, sets `HasCapture` false for an alias with no capture row, takes share prices from the quote `UnderlyingClose` or else from `store/mark` joined to the STOCK row by `ConID`, lists a capture position absent from the ledger as `not_in_ledger`, and sets `Funds` with its `Currency` from the newest `account.value` of the day or the day before, nil otherwise.
4. `job.go` writes, per alias and currency, one `book_risk` row with key `<alias>:<currency>` and, per currency, one with key `household:<currency>` (sum of the complete aliases' totals of that currency; the row is `unknown` and carries no total when any alias row of the currency is not complete), `underlying_id` 0 and status `ok` for a complete total and `unknown` otherwise (Decision 12); and for each ok row of `Reader.Results(day, "short_put")` and each alias with funds one `fit` row with the `underlying_id` of its candidate and status ok, blocked or unknown by its check; all through `strategyresult.Writer.InsertResults` with one run id per run, `legs`, `checks` and `facts` as JSON text with the keys of `docs/contracts/strategy.md`; with no candidate rows only book rows; a failed read of one alias is recorded on its row (`status` unknown, reason `read_failed`) and the others still run; no broker and no network source is used.
5. `job_test.go` and `load_test.go` have the new tests `TestBookRiskNewWritesOneRowPerAliasAndHousehold`, `TestBookRiskNewHouseholdIsSumOfCompleteAliasTotalsPerCurrency`, `TestBookRiskNewIncompleteAliasGivesHouseholdNoTotal`, `TestBookRiskNewAliasWithoutCaptureIsNoCapture`, `TestBookRiskNewBookRowHasUnderlyingIdZeroAndStatusByCompleteness`, `TestBookRiskNewIndexClassLegIsCashSettledWithEmptyTerms`, `TestBookRiskNewFitRowsOnlyForOkCandidates`, `TestBookRiskNewAliasWithoutValuesGetsNoFitRow`, `TestBookRiskNewOneAliasReadFailureKeepsTheOthers`, `TestBookRiskNewRerunWritesANewRunId`, `TestBookRiskNewCashSettledFromTermsFields`, `TestBookRiskNewCaptureOnlyPositionIsNotInLedger`, `TestBookRiskNewLegsEqualRiskPageStructureLegs` (the same fake stores through this loader and through the `risk_page` sequence give the same legs; the page package is named only in the test as a copy of its sequence, not imported); fakes, `t.Parallel()`, fake clock, `t.TempDir()`.
6. `docs/contracts/jobs/book_risk.md` states the manifest keys, the reads and the rows; the change edits only this entry's Files; new files under 600 lines; `Contracts: unchanged`.
7. The result matches docs/design/strategy-risk.md, Decisions 4, 5, 8, 9 and 11.

### 5 ot-wire

Module: ot. Size: S.
Order: after 2, 3, 4.
Files: `cmd/ot/bookrisk.go`, `cmd/ot/bookrisk_test.go`, `cmd/ot/wire.go`, `cmd/ot/jobconsole_jobs.go`, `cmd/ot/jobconsole_jobs_test.go`, `docs/contracts/ot.md`, `docs/contracts/ot/jobs/book_risk.md`
Design: docs/design/strategy-risk.md#Decisions (9, 11)
Acceptance:
1. `cmd/ot/bookrisk.go` builds `bsm.New()`, `stress.New(...)` and `fit.New(...)` (the only place that imports the three risk packages), hands the job the stresser and the fitter, and opens the stores as `candidates.go` does (a failed open names the key, never a value); `jobconsole_jobs.go` gains the row `anyDay("book_risk", "new_run_id", textNewRunID)` (the rows are insert-only under a new run id, as `option_candidates`; so the job is any-day and force-no) and its comment; `cmd/ot/jobconsole_jobs_test.go` has three pinned changes, each stated as the result at landing: the job-table count is one more than main holds at registration (31 on main today, so 32 if nothing else landed first), `TestAnyDayIsExactlyTheSeven` lists eight ids (its expected list gains `book_risk`; the test name is kept), and `TestEveryForceNoJobHasReason` counts one more force-no job (18 on main today, so 19); `wire.go` at most two lines (795 lines; if more is needed the pull request says so and leaves it).
2. `bookrisk.go` calls `registerJob` with a `jobFamily` (as `candidates.go` does), so `ot run-once book_risk` and `ot backfill --job book_risk` run the job; the job has no dry run (a `--job` dry run exists only for the `dryRunJobs` of `cmd/ot/dryrun.go`, which this entry does not edit); the demo profile refuses the job as `candidates` is refused.
3. `bookrisk_test.go` has the new tests `TestBookRiskNewIsRegisteredByRunOnce`, `TestBookRiskNewIsRegisteredForBackfill`, `TestBookRiskNewDemoProfileRefused`, `TestBookRiskNewMissingDSNNamesTheKeyOnly`; fakes, `t.Parallel()`.
4. `docs/contracts/ot.md` names the id in its table of job documents and `docs/contracts/ot/jobs/book_risk.md` is its document (as `docs/contracts/ot/jobs/option_candidates.md`); no other pinned list of job ids is touched (a grep for `option_eod_index` finds the others only in tests of those jobs); the change edits only this entry's Files; `Contracts: unchanged`.
5. The result matches docs/design/strategy-risk.md, Decisions 9, 11 and 12.

## Checked by CI

`go build ./...`, `go vet ./...`, the Go tests of `internal/strategy/...`, `internal/jobs/bookrisk` and `cmd/ot`, the architecture lint (jobs reach `strategy` through its root package only), `internal/jobs/api_users_test.go`, the size check against `docs/design/ratchet.json`.

## Left out

- A page or block for the book risk (the facts are stored so it computes nothing).
- A concentration check (an underlying's share of the stress loss against a limit): no source for any limit [NF], the tastylive 25 to 50 percent of buying power is one firm's line [S]. If the owner names a number it is a later entry on the same facts; until then nothing is shown.
- A cushion from `ExcessLiquidity` and margin at stress (Decision 5); a volatility shift [NF]; the broker's margin model; early-exercise value of American options; dividends in the pricer; non-US underlyings and other currencies (excluded and counted); Flex cash and margin of `daughter` (owner: not now), so her fit is `unknown no_funds`.
- Moving the `funding` gate out of `put_v1`; beta-weighted delta of single names; sizing from the stress; automatic blocking; replay of the stress over past days (the replay design owns `internal/strategy/replayapi`).
- The rule's per-contract minimum, the 15c3-1a offsets between classes, and the zero-move point on the grid.
- Fit for strategies other than `short_put`: the job reads the rows of `short_put` only; a strategy added later is one more name in its list.

## Doubts

None. The one question of revision 2 (show an incomplete household total) is answered by the owner's rule that a total is shown only when complete (Decisions 2 and 4).

## Adversarial

A fresh read-only read of the first draft (the file `risk-adversarial.md` in the dispatcher window's scratchpad) raised six blocking points and a simpler way; all are adopted in this text.

1. Module rule (jobs may import only the root `strategy` package): adopted. The `Risk*` types live in `internal/strategy/api.go`; `cmd/ot` builds the implementation and hands it to the job, as `strategy_wire.go` does for the short put. No `modules.md` change.
2. The loader as `internal/pages/risk_page/load.go` really reads (chain mid through the option terms, `Replay` with no marks, capture OPTION and STOCK rows): adopted; `store/mark` only for share prices, joined by ConID, named as new.
3. No cushion figure from `ExcessLiquidity − loss`: adopted; the row carries `equity_loss_vs_excess_liquidity` with `margin_change_not_modelled` [T] and no check on it.
4. Cash-settled European index options are never assigned: adopted, from `store.OptionTerms` exercise style and settlement type; `unknown` or empty fields count as assignable unless the class table names an index (Decision 6).
5. Stale IV: adopted, reason `stale_mark`.
6. An incomplete total is never a lower bound: adopted; `RiskFunds` carries `Currency`, a mismatch gives `unknown no_fx`.
Simpler way: the joint scenarios are replaced by the FINRA ten-point ladder per class, the concentration check and unused interface types are dropped. Not adopted: five entries remain (types, stress, fit, job, wiring), because each is one module and the job cannot hold the pricer.

Second read (advisor notes r1 on revision 2, `strategy-risk-review-r1.md`): all points adopted in revision 3.

1. Source: (g)(7) and (g)(8) make the no-offset sum [V]; the output sentence "this is the broker-dealer portfolio-margin ladder used as a sourced scenario, not your account's margin" is a fact of the row; Japanese names at +-15 percent are [T]; ten equal intervals, eleven points, stated (Decisions 1 and 2).
2. An alias with no capture: exclusion `no_capture` or shares from the ledger (Decision 4).
3. Totals per alias and currency, no number unless complete, class worsts and the exclusion list otherwise; the Doubt is removed (Decisions 2 and 4).
4. `fit_capital` is against available funds, with `open_put_strikes` beside it (Decision 8).
5. Oldest spot day per class stored and printed (Decision 2).
6. The job-console table, its test (31 to 32) and the `docs/contracts/ot/jobs/` document are in entry 5; the manifest fields are named in entry 4.
7. `strategy.result` identity and status of the new rows: Decision 12, no dataset change.
8. A leg at or below intrinsic is valued at intrinsic (Decision 3). The terms columns are written by `instrument/resolve.go` and `insertTermsSQL` writes `'unknown'` for both on every option [V]; the owner's class table keeps SPX, XSP, NDX and RUT out of assignment cash (Decision 6).

Third read (design verdict `changes` at 42cd8964), answered in revision 4: entry 5 acceptance 2 now says `ot run-once` and `ot backfill --job book_risk` run the job through `registerJob` and the job has no dry run (`dryRunJobs` of `cmd/ot/dryrun.go` is not edited), and `cmd/ot/runonce.go` leaves the Files line; the three pinned changes of `jobconsole_jobs_test.go` are stated as results at landing (entry 5 acceptance 1); the Modules paragraph matches the Order lines; Decision 9 keys are `<alias>:<currency>` and `household:<currency>`; Decision 6 states the `'unknown'` literal as [V].
