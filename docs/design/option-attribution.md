# option attribution: split the profit of a short put into direction, time, move cost and IV re-marking, as a pure function that sums to the ledger to the cent

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-17

Source: the plan "卖 put 桌面（P2）——盈亏归因与候选依据" (`state/optdesk-drafts/designs/sellput-strategy/plan-2026-11-17.md`), sections 盈亏归因怎么算, 取数, 特殊日子怎么处理, 缺数据时怎么办, 与券商盈亏对不上时的差额怎么显示, steps 2 and 3, appendix A; owner answers "aaac" of 2026-11-17 (attribution and evidence run as parallel lines). This design is plan steps 2 and 3 only: the interface and the rule. The page `/options/pnl` (plan step 4) is a later design.

## 你会看到什么

这份设计本身在页面上看不到东西，它做的是后面 `/options/pnl` 页（盈亏拆分页，计划第 4 步）要用的计算。做完之后，每个你卖出的 put（"卖 put" = 你收一笔权利金，到期前股价跌破行权价就按行权价买入股票）的盈亏会被拆成几块，而且几块加起来和账本里的盈亏分毫不差：

- 时间（theta，期权每过一天便宜一点，卖方收这份租金）。
- 波动幅度的代价（gamma，"凸性"：股价动得越大，put 对你越不利）。"时间"减"波动幅度的代价"就是"租金减代价"，也就是你卖保险实际赚了多少。
- 方向（delta：股价涨跌带来的盈亏，是你的估值判断，不是波动率上的优势）。
- IV 重估（vega；IV 是隐含波动率，市场现在给未来波动的定价，IV 降了你赚，但那是未实现的，到期前会回来）。
- 入场/出场与中价之差（成交价对买卖中间价的差）和手续费。

没有报价的日子不会被编出来；到期那一天的价格直接用内在价值（行权价减股价，不小于 0），不靠模型：写"gap day"，下一个有报价的日子合成一个多天步。IV 反解不出来的那一步放进"未拆分"，写原因。被行权的单子，股票那一侧的内在价值单列，不丢也不并进期权。所有数字都是读时现算，不建表、不加迁移。

## Today

- `compute.LedgerPosition` and `LedgerFill` carry quantity, multiplier, fills (day, action, quantity, price, commission), `Commissions` as a positive sum, `RealizedPnL` = premium minus commission rounded half-even to 4 places and `BrokerRealized` (`internal/compute/api.go:536-571,613-617`, `internal/compute/optionledger/premium.go:145-170`). Ledger actions are `open`, `close`, `expire`, `assign`, `exercise` (`internal/compute/optionledger/replay.go:16-20`); status of the position follows the last one (`lots.go:142-152`). The mid rule of the ledger: ask above 0 and bid not below 0 (`premium.go:94-113`).
- The pricer exists in the strategy module: `internal/strategy/risk/bsm` has `Value` and `Implied` with decimal text, an `ErrExpired` for no time left, and an error with `NoSolution()` (`bsm.go:14-32,107,121`); the interface it satisfies is `stress.Pricer` (`internal/strategy/risk/stress/stress.go:279-286`). Module compute depends on `contracts` only (`docs/design/modules.md`), so production code in compute cannot import either package; `cmd/ot` passes `bsm.New()` in plan step 4.
- Quotes are stored per contract per day in `obs.option_day` (`migrations/0047_option_captures.sql`), with bid, ask, the underlying price of the same row and the IBKR figures; nothing reads it yet (wired in plan steps 1 and 4, not here). `compute.StructureQuote` (`internal/compute/api.go:620`) has bid, ask, underlying close and the greeks but no IV, and repeats the contract terms on every row; it is not reused (Decision 9).
- No attribution exists. `internal/compute/api.go` is 840 lines and at its ceiling in `docs/design/ratchet.json`, so every new type goes in a new package, as `companydataapi` did (`docs/design/data-collection.md`, Modules).

## Decisions

1. **Daily re-pricing in a fixed order, per lot.** For two neighbouring quote days t0 and t1 (both quote days, Decision 8), V0 and V1 are the stored mids, S0 and S1 the underlying prices of the same two rows, sigma0 and sigma1 the implied volatilities of the two mids, T = calendar days to expiry / 365 (the ITM probability's basis, `internal/compute/optionfig/figures.go:6-18`), r the DGS3MO point of day t0 used at both ends and for every currency (trial: no yen rate is stored) (plan, step 3; appendix A). Then A = V0; B = Value(S1, sigma0, T0); C = Value(S1, sigma1, T0); D = V1. Stock step = B - A, split into a delta part Delta0 x (S1 - S0) and a convexity part, the exact remainder (B - A) minus the delta part; IV step = C - B; time step = D - C. Seller blocks are the negatives times the open quantity times the multiplier: direction, move cost, IV re-marking, time.
2. **The ends are the stored mids, not repriced values, and the remainder is exact.** A and D are V0 and V1 themselves (a solved IV reproduces its mid only to the solver's tolerance, `bsm.go` `Implied`), and the convexity part and the time step are exact differences, so the blocks of a step sum to -(V1 - V0) x quantity x multiplier for any pricer. The identity does not depend on the model or on how a division is carried (decimal divisions are at 16 places); the model only decides how the move is shared.
3. **Delta by central difference of `Value`.** Delta0 = (Value(S0 + h) - Value(S0 - h)) / 2h, h = 0.5% of S0 (trial, appendix A). The pricer needs no delta method and is not changed. A quadratic-in-price pricer gives the same Delta0 for every h (the central difference is exact), which the tests use instead of a numeric threshold.
4. **Quantity, fills and the ledger mapping.** `Lot.Fills` holds only the ledger rows with action `open` (Side `open`, a sale) and `close` (Side `close`, a buy-back); the rows `expire`, `assign` and `exercise` are not fills: the page sets `End` (`expired`, `assigned`, `exercised`, with `EndDay` and `EndClose`). The quantity of a step from quote day t0 to t1 is the quantity opened on days up to t0 minus the quantity closed on days before t1. An open fill is charged on the first quote day on or after its day: entry line = (price - V of that day) x quantity x multiplier, so a fill on a day with no quote carries the move up to that day in its entry line, flagged `LateEntry`. A close fill (also a partial close) is charged at the last quote day on or before its day: exit line = (V of that day - price) x quantity closed x multiplier, and the closed quantity leaves the steps from that day on. A fill with no quote day on the needed side makes the whole lot `NoQuotes`.
5. **Commission and assignment signs.** `Result.Commission` is minus the sum of the fills' commissions (a cost, negative; the ledger's `Commissions` is positive). `Result.AssignedIntrinsic` is plus intrinsic value I x quantity x multiplier moved to the shares (positive; the plan's "intrinsic moved to the shares"), zero unless the lot ended `assigned` or `exercised`. The identity: Entry + Exit + the sum of the steps (blocks and `Unsplit`) + Commission + AssignedIntrinsic = the ledger figure. The option side alone is that sum without `AssignedIntrinsic`, which is the plan's "option P&L = ledger realised minus I".
6. **Difference is the real check.** `Input.LedgerPnL` is the ledger's `RealizedPnL`, or `UnrealizedPnL` for an open lot. For an open lot the identity ends at the mid of the last quote day (`Result.MarkDay`); the ledger values an open lot at its own newest mark, whose day is `Input.LedgerMarkDay`. When the two days differ, `Difference` stays empty and `Reason` names both days, since the two figures are of different days and their gap is not an error. `Result.Difference` = `LedgerPnL` minus the identity sum above rounded half-even to 4 places (the ledger's own rounding; the only rounding in the package). "0" for a well-formed lot; a non-zero value is shown by the page with a reason (layer 1 of the plan).
7. **Expiry day, T = 0.** A quote on the expiry day is not a priced day: the pricer is not asked about T = 0 (so no `ErrExpired` reaches `Unsplit`). The step that ends on it prices B and C with sigma0 and T0 as usual, sets sigma1 unsolved, the IV step 0, and the end value D to the intrinsic value max(K - S1, 0) at that row's underlying price instead of its mid; the time step is D - C. Nothing is left over: after the last quote day n a terminal step runs to the intrinsic value I = max(K - S_T, 0) at `EndClose`. When day n is before the expiry day, the terminal step runs the stock step to `EndClose` (sigma_n, T_n) and the time step to I. When day n is the expiry day, the terminal step makes no pricer call: its stock step is I minus the intrinsic value at that row's underlying price, all of it Direction (seller sign, times quantity and multiplier), with move cost, IV step and time step 0. A lot that expires worthless has I = 0 and no exit line. A step whose IV cannot be solved for another reason (`NoSolution`, a bad number) is not split: all of it goes to `Unsplit` with a reason, and the result counts `UnsplitSteps` so the page shows the share. A step whose day t0 has no `Rate` within `MaxRateAgeDays` goes whole to `Unsplit` the same way, and its day is listed in `NoRate`.
8. **Gap days are never filled.** A quote day is a stored row with ask above zero, bid at or above zero and an underlying price (the ledger's own mark rule, `internal/compute/optionledger/premium.go:94-113`); its mark is the mid (bid + ask) / 2. A row with any of the three missing or out of range is a gap day. A gap day is not replaced by the previous price; the next quote day makes one multi-day step (calendar days, `MultiDay`). Wide quotes ((ask - bid) / mid above 10%, the candidate gate's own parameter) are flagged `Wide` and still used. Lots without stored quotes (before 2026-11-07, or none stored): `NoQuotes`; the result fills only `Currency`, `Commission`, `AssignedIntrinsic`, `LedgerPnL` and `Reason`; `Steps`, `Entry`, `Exit`, the block sums, `Unsplit` and `Difference` stay empty (no identity is claimed and nothing is estimated); the page shows the ledger's own premium beside it. A roll is a close and an open, attributed separately. One `Result` is one lot in one currency.
9. **Own `Quote` with the IBKR figures as `Ref`.** The IV is needed for the comparison and `StructureQuote` has none; its contract terms per row and its name `UnderlyingClose` (here the underlying of the same row) differ. IBKR IV, delta, gamma, theta and vega are comparison only and never enter the sums. The model is European Black-Scholes without dividends (`bsm.go:1-5`): an ex-dividend drop lands in the solved IV and the early-exercise premium is not modelled; the page says this one sentence.
10. **No migration, no write, no clock.** The function takes everything it reads as arguments. `Pricer` is declared here with the shape of `stress.Pricer` because compute cannot import it; `cmd/ot` meets the shape at compile time in plan step 4.

## Modules

`docs/design/modules.md`: row `compute` (line 92), the interface cell gains `internal/compute/optionattrapi` (declarations only). No other row changes and no row is added: compute's sub-packages are not rows of their own (`internal/compute/**` matches their paths; their interface packages sit in the entry cell, as `companydataapi` does). This design's commit makes the one edit; the design `candidate-evidence.md` appends to the same cell, so whichever merges second is rebased. Paths of the entries: compute (`internal/compute/**`, `docs/contracts/compute/**`; entries 1 and 2).

Import edges against the `depends` column (compute: `contracts`): `optionattrapi` imports `internal/contracts` only (`contracts.Day`) and the standard `errors`; `optionattr` imports `optionattrapi`, `contracts` and `github.com/shopspring/decimal` (already a direct dependency, v1.6.0; no new library). Nothing in compute imports `internal/strategy`; the tests use stub pricers, so no test imports it either.

## Interface

Entry 1 (module compute, new package `internal/compute/optionattrapi`, `api.go` declarations only; decimal text, no float, no clock, no I/O; an empty string is "no value"):

    // Pricer has the shape of stress.Pricer in internal/strategy/risk/stress; years is calendar days/365,
    // rate is the continuously compounded rate as a fraction, right is "P". An error with
    // NoSolution() bool true means no volatility matches the price.
    type Pricer interface {
        Value(right, spot, strike, years, rate, vol string) (string, error)
        Implied(right, spot, strike, years, rate, price string) (string, error)
    }

    // Fill is a ledger row with action open (Side "open", a sale) or close (Side "close", a buy-back).
    type Fill struct {
        Day                         contracts.Day
        Side                        string
        Quantity, Price, Commission string // Commission positive, as the ledger
    }

    // Ref is the IBKR figures of one stored row, for comparison only.
    type Ref struct{ IV, Delta, Gamma, Theta, Vega string }

    // Quote is one stored row of the contract: Spot is the underlying price of the same row.
    type Quote struct {
        Day            contracts.Day
        Bid, Ask, Spot string
        Ref            Ref
    }

    // Rate is a DGS3MO point in percent.
    type Rate struct {
        Day     contracts.Day
        Percent string
    }

    // Lot is one short put position: the ledger's view. End is open, closed, expired, assigned or exercised.
    type Lot struct {
        Currency, Strike, Multiplier string
        Expiry                       contracts.Day
        Fills                        []Fill
        End                          string
        EndDay                       contracts.Day // "" when open
        EndClose                     string        // underlying close of EndDay for expired, assigned, exercised
    }

    // Input is everything Attribute reads. Quotes ascend by Day. LedgerPnL is the ledger's RealizedPnL
    // (UnrealizedPnL for an open lot). MaxRateAgeDays is how old a Rate may be (7).
    type Input struct {
        Lot            Lot
        Quotes         []Quote
        Rates          []Rate
        LedgerPnL      string
        LedgerMarkDay  contracts.Day // the ledger's mark day for an open lot; "" when closed
        MaxRateAgeDays int
    }

    // Step is one pair of neighbouring quote days (or the terminal step). Blocks are seller amounts:
    // positive is earned. Reason is set when the step is Unsplit.
    type Step struct {
        From, To                                    contracts.Day
        CalendarDays                                int
        MultiDay, Wide, Terminal                    bool
        Direction, MoveCost, IVRemark, Time, Unsplit string
        IVFrom, IVTo                                string // solved from the mids; "" when not solved
        RefIVFrom, RefIVTo                          string
        Reason                                      string
    }

    // Result is one lot. Commission is negative (a cost); AssignedIntrinsic is positive (the intrinsic
    // value moved to the shares). Difference is LedgerPnL minus (Entry + Exit + the sum of Steps +
    // Commission + AssignedIntrinsic) rounded half-even to 4 places.
    type Result struct {
        Currency                              string
        Steps                                 []Step
        Entry, Exit, Commission               string
        Direction, MoveCost, IVRemark, Time   string // sums over Steps
        Unsplit, AssignedIntrinsic            string
        LedgerPnL, Difference                 string
        UnsplitSteps                          int
        MarkDay                               contracts.Day // last quote day of an open lot; "" when closed
        LateEntry                             bool
        GapDays                               []contracts.Day
        NoQuotes                              bool
        NoRate                                []contracts.Day // days that could not be split for lack of a rate; their amount is in Unsplit
        Reason                                string
    }

    // Attributor is the only place the split is computed.
    type Attributor interface {
        Attribute(in Input) (Result, error)
    }

    // ErrBadInput: a lot with no open fill, a quantity, price or LedgerPnL that is not a number.
    var ErrBadInput = errors.New("optionattrapi: bad input")

Entry 2 (module compute, package `internal/compute/optionattr`): `func New(p optionattrapi.Pricer) optionattrapi.Attributor`. Parameters in the package, each marked in its doc comment: h = 0.5% of spot (trial), wide-quote 10% (the candidate gate's parameter), 365 days.

## Issues

Build order: entry 1 first, entry 2 after 1. Neither carries a migration, a schema or a write. Every entry compiles and its tests pass against main. Test runs are in `## Checked by CI`. All test numbers are made up (companies AAA and BBB, invented prices).

### 1 option-attribution-api (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/optionattrapi/api.go`, `internal/compute/optionattrapi/api_test.go`, `docs/contracts/compute/optionattr.md`
Contracts: unchanged
Design: docs/design/option-attribution.md#Interface
Acceptance:
1. `api.go` (new file of the new package `internal/compute/optionattrapi`) declares the new `Pricer`, `Fill`, `Ref`, `Quote`, `Rate`, `Lot`, `Input`, `Step`, `Result`, `Attributor` (new) and `ErrBadInput` as in the Interface section, with docs and no function body (the file holds only type, const and var declarations; `ErrBadInput` is `errors.New(...)` as `valuelinesapi` does), and is the only non-test `.go` file of its package; its only repository import is `internal/contracts`.
2. `api_test.go` has `TestAttributorContractFake` (new: a fake `Attributor` answers a made-up AAA lot of 2 contracts sold at 1.80 and a caller reads `Steps`, `GapDays`, `Unsplit`, `LedgerPnL` and `Difference`), `TestFakePricerSatisfiesPricer` (new: a fake pricer with fixed table prices and a `NoSolution() bool` error is assigned to `Pricer`, so the shape of `stress.Pricer` is kept), `TestErrBadInputIsSentinel` (new: `errors.Is` of the fake's error against `ErrBadInput` is true) and `TestEveryListIsNonNil` (new: the fake's empty answers are non-nil slices).
3. `docs/contracts/compute/optionattr.md` (new) states in one paragraph each: amounts are seller amounts (positive is earned), `Commission` is negative and `AssignedIntrinsic` positive, and the identity sum equals the ledger figure; the ends of a step are the stored mids, so the blocks of a step sum to the value change for any pricer; a gap day is never filled and makes a multi-day step; a step ending on the expiry day prices the end at the intrinsic value and does not ask the pricer for T = 0; `expire`, `assign` and `exercise` are not fills; the new `Attributor` is the only place the split is computed.

### 2 option-attribution

Module: compute. Size: M.
Order: after 1.
Files: `internal/compute/optionattr/attr.go`, `internal/compute/optionattr/steps.go`, `internal/compute/optionattr/attr_test.go`, `internal/compute/optionattr/identity_test.go`, `internal/compute/optionattr/pricer_test.go`, `internal/compute/optionattr/testdata/golden.json`
Contracts: unchanged
Design: docs/design/option-attribution.md#Decisions (Decisions 1 to 8)
Acceptance:
1. `attr.go` implements `optionattrapi.Attributor` (`var _ optionattrapi.Attributor = ...`) with `shopspring/decimal`, no `float64`, no clock and no import of `internal/strategy`; its only rounding call is the 4-place half-even rounding that computes `Difference` (one call site): `TestAttributeNoRounding` (new: a made-up AAA lot, 2 contracts, multiplier 100, sold at 1.80, quotes on three days, keeps exact decimal blocks).
2. Expiry golden: a lot sold at 1.80 (2 contracts, multiplier 100, strike 38.00) that expires worthless with commissions 2.60 and `LedgerPnL` 357.40 gives `Commission` -2.60, no `Exit`, `AssignedIntrinsic` 0, entry + steps = 360.00 and `Difference` "0"; assignment golden: the same lot assigned with `EndClose` 36.10 gives `AssignedIntrinsic` 380.00, entry + steps + exit = -20.00 (the option side) and `Difference` "0": `TestIdentityExpiredGolden` (new), `TestIdentityAssignedGolden` (new), both in `identity_test.go`. `TestDifferenceShowsLedgerGap` (new): a `LedgerPnL` of 357.00 on the expiry case gives `Difference` -0.40 (ledger minus our sum).
3. Stub pricers in `pricer_test.go` (new): one with value linear in vol and quadratic in price, closed-form `Implied`, so every golden number is hand-checkable, and a second with another curvature. Properties pinned by golden cases: with the quadratic stub the move cost of a step is exactly minus the curvature times the squared price move times quantity times multiplier (zero when the price did not move, not solver noise) and direction is exactly minus Delta0 times the move: `TestMoveCostIsCurvatureTimesMoveSquared` (new), `TestDirectionIsDeltaTimesMove` (new); Delta0 and every block are identical for h and h/2 on the quadratic stub: `TestDeltaIndependentOfStep` (new); the two stubs give equal totals and different splits: `TestTotalIndependentOfPricer` (new).
4. Golden cases in `testdata/golden.json` (all invented), each its own named test case run by `TestGoldenCases` (new): ordinary two-day step; gap day (one `MultiDay` step of 2 calendar days, the gap day in `GapDays`); wide quote (`Wide`, still split); IV not solvable (that step all `Unsplit` with a reason and counted in `UnsplitSteps`, the others split); quote on the expiry day (end value is the intrinsic value, the stub is never called with T = 0, `Unsplit` stays 0); terminal step after a quote on the expiry day with `EndClose` different from that row's underlying price (all of the intrinsic move in `Direction`, no pricer call, `Difference` "0"); open lot whose `LedgerMarkDay` differs from its last quote day (`Difference` empty, `Reason` names both days); `NoQuotes` lot (only the fields of Decision 8 filled); no stored quotes (`NoQuotes`); two currencies (two `Result`s, never added); no rate within `MaxRateAgeDays` (the day in `NoRate`).
5. Fills: `TestPartialCloseGolden` (new: 3 contracts sold, 1 bought back on a quote day, the remaining 2 expire; the closed contract leaves the steps from its quote day and the identity holds with `Difference` "0"), `TestFillOnDayWithoutQuote` (new: an open fill on a day with no quote is charged on the next quote day and `LateEntry` is true), and `TestFillSideMapping` (new: a lot built from ledger rows `open`, `close`, `expire` has two fills and `End` `expired`).
6. Each new test calls `t.Parallel()` and reads `testdata/golden.json` only; `attr.go` and `steps.go` are each under 600 lines (the diff shows their line counts).

## Checked by CI

`go test ./internal/compute/...` (entries 1 and 2), `go run ./scripts/arch` and go-arch-lint (the import edges of Modules), the size ratchet check, the contract-docs check of `docs/contracts/compute/`. No page changes in this design; the first page that shows the split is plan step 4, with its own design and its own screenshot check.

## Left out

- Everything of plan steps 1, 4, 5 and 6 (quote coverage on the tracking page, the page `/options/pnl`, the monthly block, cash due if assigned): later designs; the reader of `obs.option_day`, the mapping of ledger rows to `Lot`, display rounding, the three reconciliation lines (ours vs ledger, ledger vs broker's `BrokerRealized`, the "looks like a roll" label) and the share of `Unsplit` steps shown on the page belong to the page.
- Layer 3 of the plan's reconciliation (the account page's IBKR unrealised P&L against our mid P&L): not done, IBKR's pricing basis is not read.
- A binomial tree for early-exercise premium, dividend yield in the pricer, IBKR greeks as inputs to the sums: not built.
- The sample sentence ("fewer than 20 closed lots: no conclusion", 20 is trial) and monthly or per-company sums: the page's.
- A table or a cache of daily attribution, a migration, a write: none.
- Whether `obs.option_day` was backfilled before 2026-11-07: not known here (plan, 没有核实的事项); the function gets fewer quotes and says `NoQuotes`.
- Parameter sources: h 0.5% (trial), rate of day t0 (convention), 10% wide quote (the candidate gate's own), 365 days (the ITM probability's basis), 4-place rounding of `Difference` (the ledger's own). No academic source is claimed for any of them (plan, appendix A).

## Adversarial

Answers to the window's file adversarial-d1-d5-2026-11-17.md (outside the repository), section D1 and the top part.

- Top, test-side import: adopted in wording: the claim is that production compute may not import strategy; tests use stub pricers, so no test imports it either (Modules).
- Top, PR conflict on the entry cell of line 92: kept, both designs append there, the window rebases the second. Precedent `optionfigapi`: adopted, replaced by `companydataapi`.
- 1 Gap, contract test from both sides: not adopted beyond the precedent (`companydataapi` has a fake and a caller); the real function meets the interface by `var _` in entry 2. Partial closes, first fill on a day with no quote, ledger action to Side mapping: adopted (Decision 4, tests `TestPartialCloseGolden`, `TestFillOnDayWithoutQuote`, `TestFillSideMapping`).
- 2 T = 0 step: adopted (Decision 7, the expiry-day golden case); the share of `Unsplit` steps is `UnsplitSteps` in the result for the page.
- 3 Reuse `StructureQuote`: not adopted, one sentence in Decision 9 (no IV, contract terms on every row).
- 4 Lock-in of the field set and the duplicate `Pricer` and `Valuer`: not adopted; the fields are the page's schema by design and change by a new design; the two shape copies are pinned by `TestFakePricerSatisfiesPricer` and met at compile time in plan step 4.
- 5a Difference compares the function with itself: adopted (`Input.LedgerPnL`, Decision 6, `TestDifferenceShowsLedgerGap`). 5b Commission sign: adopted (negative, Decision 5). 5c Assigned identity and sign: adopted (Decision 5, golden numbers 357.40 and 380.00). 5d A = V0 and "no rounding inside": adopted (Decision 2 now says the identity holds for any pricer and any division precision; the noise-sign problem is removed by exact stub pricers in tests). 5e Numeric thresholds: adopted, replaced by exact properties on stub pricers. 5f Own test-side Black-Scholes: adopted, replaced by stub pricers. 5g Nil sentinel: adopted (`errors.New`).
- 6 Command-run acceptance items and grep: adopted, replaced by named tests or diff-checkable statements; item 3 bundle: split into items 2 to 5.
