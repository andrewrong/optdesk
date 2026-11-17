# candidate evidence: for each put candidate, the expected move, the edge at the bid, the skew and the soft labels, as a pure function

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-17

Source: the plan "卖 put 桌面（P2）——盈亏归因与候选依据" (`state/optdesk-drafts/designs/sellput-strategy/plan-2026-11-17.md`), sections 候选的依据 (每个候选显示什么, 硬闸门, 软条件), steps 7 and 8, appendix A and B; owner answers "aaac" of 2026-11-17 (candidates ordered by break-even distance below the buy price, concentration shown as shares only, buy price and valuation B_hi side by side with the gate unchanged, attribution and evidence in parallel). This design is plan steps 7 and 8 only: the interface and the rule. The candidate page columns (plan steps 9 to 11) are later designs.

## 你会看到什么

这份设计本身在页面上看不到东西，它做的是候选页（`/options/candidates`，每天"卖哪家的 put、卖哪个行权价、凭什么"那一页）以后每一行要加的"凭什么选它"那几格的计算。做完、接到页面上之后，每个候选行会多出：

- 比买入价低多少（below buy）：保本价（break-even，行权价减权利金）比你手填的买入价低多少，金额和百分比。这是你定的排序键（公司按最好一行的这个数排），没有买入价就写"no buy price"。
- 预期波动幅度（1-sigma：市场定价里一个标准差的股价变动，等于现价 × IV × √(到期天数/365)；IV 是隐含波动率，市场现在给未来波动开的价），以及保本价比现价跌了多少，是这个幅度的几倍（小于 1 说明保本价在一个标准差以内）。
- 买价对预测价（bid vs forecast price，代码里叫 edge）：你按 bid（买方出价）卖得的权利金，减去"用我们预测的波动率算出的价格"。**这不是真正的优势，只是一个代理指标**：预测波动率只是 20、60、120 个交易日实际波动（realised vol，已经发生的价格波动）的平均，模型是不带股息的欧式期权模型，所以正负号常常不准。页面会印出预测过去的误差，并且把它叫"bid vs forecast price"，不叫"优势"。到期前有财报、或没有预测，不印数字，只印原因。
- 偏斜（skew）：这个合约的 IV 减同一到期日平值合约（行权价最接近现价）的 IV。只展示，不筛选。名字级的"平值 IV 减预测"不在这里算，页面直接读已存的 `market.vol_edge`。
- 三个软标签，只是一行小字，不改这一行过没过闸门：没有 edge、delta 不在 0.15 到 0.30 之间（你自己的习惯区间，delta 是期权对股价的敏感度）、除息日在到期前。财报没核实、价差过宽沿用已有的标记和闸门，这里不重复。

闸门（买入价、报价、财报、资金、现金利率）不动。标签要等回放证明有用才可能升为闸门，今天一个也不升。

## Today

- The candidate rule evaluates each contract: five gates in a fixed order, flags, break-even and cushion (`internal/compute/optioncand/optioncand.go:21-116`, `put.go:18-118`). The gate for the buy price lets a row pass when there is no buy price and flags `no_buy_price` (`put.go:16-21`). The flag `earnings_unconfirmed` is already emitted (`internal/compute/optioncand/events.go:9`) and the spread is already gated at 10% (`internal/jobs/optioncandidates/job.toml:18`). This design changes none of it.
- Today a row shows rank, name, expiry, DTE, strike, bid, annual yield, excess over cash, break-even, delta, IV, ITM probability, flags, funds fit and earnings (`internal/compute/api.go:445-472`).
- The name-level readings `market.vol_forecast` (mean of RV20, RV60, RV120, `internal/compute/figures/combine.go:10-24`), `market.vol_edge` (`rules.json:25`) and `market.vol_forecast_err` are stored daily (`internal/jobs/computefigures/job.toml:18-20`).
- The pricer lives in the strategy module (`internal/strategy/risk/bsm`, `Value` at `bsm.go:107`) and compute depends on `contracts` only (`docs/design/modules.md`), so `cmd/ot` hands it in through an interface (plan step 9). `internal/compute/api.go` is at its ratchet ceiling, so new types go in new packages, as `companydataapi` did.

## Decisions

1. **One function per row, inputs as text.** It reads the row, the name-level readings the page already stored, the ATM IV of the same expiry, the buy price, the earnings and ex-dividend days and the cash rate; it writes nothing and reads no clock. The chain day comes in as an argument. It also runs for rows the hard gate blocks (the page folds them), so the reason `earnings_before_expiry` is seen on blocked rows.
2. **Below the buy price.** Break-even = strike - bid. `BelowBuy` = BuyPrice - break-even (positive: the break-even is below the buy price) and `BelowBuyPct` = BelowBuy / BuyPrice in percent. No buy price (empty or not above 0): both empty with reason `no_buy_price`. This is the owner's sort key (answer 1a of 2026-11-17: companies ordered by the best break-even distance below the buy price); the ordering itself is the page's (plan step 11) and reads this figure, so it has a golden case here.
3. **Expected move.** 1-sigma = spot x IV x sqrt(DTE / 365) (one standard deviation of a log-normal price; plan appendix A, "定义", citation marked 出处待核; 365 is the ITM probability's basis, `internal/compute/optionfig/figures.go:12`). The square root is `math/big.Float.Sqrt` (standard library, precision 128 bits) of DTE/365, formatted to 12 decimal places, then multiplied in `shopspring/decimal`; no float64 and no hand-written iteration. The share is (spot - break-even) / 1-sigma, named `BreakEvenInSigmas`. No IV, or IV not above 0: both empty with reason `no_iv`.
4. **Bid vs forecast price (edge), plainly a proxy.** Fair = `Valuer.Value("P", spot, strike, DTE/365, rate, forecast)` (European Black-Scholes without dividends, as `bsm`, at a volatility made of realised moves); `EdgePerShare` = bid - fair per share and `EdgePerContract` times the multiplier; positive means the bid is above that price. It is not an edge in the sense of a profit expectation: realised volatility ignores jumps and the model ignores dividends and early exercise, so its sign is often wrong. The zero threshold is a definition (theoretical edge against market price; Natenberg, chapter not verified, plan appendix B). The row carries the last forecast error beside it so the page prints both.
5. **No number, a reason.** An earnings date before expiry (`EarningsBeforeExpiry`): `EdgePerShare` empty with `earnings_before_expiry`; no forecast: `no_forecast`; no rate: `no_rate`; the pricer fails: `no_fair_price`. The page prints the reason where the number would be. The 1-sigma and below-buy figures are still given.
6. **Skew only.** Skew = contract IV - ATM IV of the same expiry (ATM = the stored strike nearest to spot of that expiry; its strike is carried so the page can name it); no ATM row: empty with `no_atm`. The name-level "ATM IV minus forecast" is the stored `market.vol_edge`; it is not recomputed here (no `NameGap`).
7. **Three labels, never a gate.** `no_edge_at_bid` (the edge has a number and is not above 0; absent when it has none), `delta_outside_band` (the absolute value of `Delta` outside 0.15 to 0.30: trial, the owner's habit from `docs/design/vol-edge.md`; using the absolute value makes the sign convention of the stored delta irrelevant; the band is a field of `Params` so a replay can change it), `ex_div_inside_life` (ex-dividend day after the chain day and not after expiry). `earnings_unconfirmed` and the wide-quote check are not repeated: the rule's mark and the gate exist. A label is promoted to a gate only after a replay shows two groups of at least 30 trades differ (plan, 软条件; 30 is trial); none is promoted now.
8. **No score, no ordering, no data days.** The package computes no total, rank or order. The data day of each stored reading stays with the page, which holds the reading.

## Modules

`docs/design/modules.md`: row `compute` (line 92), the interface cell gains `internal/compute/optionevidenceapi` (declarations only). No other row changes and no row is added: compute's sub-packages are not rows of their own (`internal/compute/**` matches their paths; their interface packages sit in the entry cell, as `companydataapi` does). This design's commit makes the one edit; the design `option-attribution.md` appends to the same cell, so whichever merges second is rebased. Paths of the entries: compute (`internal/compute/**`, `docs/contracts/compute/**`; entries 1 and 2).

Import edges against the `depends` column (compute: `contracts`): `optionevidenceapi` imports `internal/contracts` and the standard `errors`; `optionevidence` imports `optionevidenceapi`, `contracts`, `shopspring/decimal` (already a direct dependency, v1.6.0) and the standard `math/big`. No new library. It does not import `internal/strategy`; the production pricer reaches it through `Valuer`, filled in by `cmd/ot` later.

## Interface

Entry 1 (module compute, new package `internal/compute/optionevidenceapi`, `api.go` declarations only; decimal text, no float, no clock, no I/O; an empty string is "no value"):

    // Valuer is the Value half of stress.Pricer; years is DTE/365, rate a fraction, vol a fraction.
    type Valuer interface {
        Value(right, spot, strike, years, rate, vol string) (string, error)
    }

    // Params are the numbers of the rule; DefaultParams has the trial values below.
    type Params struct {
        DeltaLow, DeltaHigh string // "0.15", "0.30": trial; compared with the absolute delta
        DaysInYear          int    // 365
    }

    // Row is one put candidate with what the page read for it.
    type Row struct {
        ChainDay, Expiry, ExDiv contracts.Day // ExDiv "" when none
        DTE                     int
        Spot, Strike, Bid       string
        BuyPrice                string // "" when none
        IV, Delta, Multiplier   string
        RatePercent             string // cash rate, "" when none
        Forecast, ForecastErr   string // name-level, fractions; "" when none
        ATMIV, ATMStrike        string // same expiry; "" when no ATM row
        EarningsBeforeExpiry    bool
    }

    // Evidence is the answer. Each figure has its own Why: empty when the figure has a value, else one of
    // no_iv, no_forecast, no_rate, no_fair_price, earnings_before_expiry, no_atm, no_buy_price.
    // The edge is a proxy: the bid against a price made from realised volatility.
    type Evidence struct {
        BreakEven                                  string // strike - bid
        BelowBuy, BelowBuyPct, BelowBuyWhy         string
        OneSigma, OneSigmaPct, BreakEvenInSigmas   string
        MoveWhy                                    string
        Fair, EdgePerShare, EdgePerContract        string
        EdgeWhy                                    string
        Skew, SkewWhy                              string
        ForecastErr                                string // repeated beside the forecast
        Labels                                     []string
    }

    // Evaluator is the only place these figures are computed.
    type Evaluator interface {
        Evaluate(r Row) (Evidence, error)
    }

    // ErrBadInput: a spot, strike, bid or DTE that is not a usable number.
    var ErrBadInput = errors.New("optionevidenceapi: bad input")

Entry 2 (module compute, package `internal/compute/optionevidence`): `func New(v optionevidenceapi.Valuer, p optionevidenceapi.Params) optionevidenceapi.Evaluator` and `func DefaultParams() optionevidenceapi.Params`.

## Issues

Build order: entry 1 first, entry 2 after 1. Neither carries a migration, a schema or a write. Every entry compiles and its tests pass against main. Test runs are in `## Checked by CI`. All test numbers are made up (companies AAA and BBB, invented prices and dates).

### 1 candidate-evidence-api (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/optionevidenceapi/api.go`, `internal/compute/optionevidenceapi/api_test.go`, `docs/contracts/compute/optionevidence.md`
Contracts: unchanged
Design: docs/design/candidate-evidence.md#Interface
Acceptance:
1. `api.go` (new file of the new package `internal/compute/optionevidenceapi`) declares the new `Valuer`, `Params`, `Row`, `Evidence`, `Evaluator` (new) and `ErrBadInput` as in the Interface section, with docs and no function body (the file holds only type, const and var declarations; `ErrBadInput` is `errors.New(...)` as `valuelinesapi` does), and is the only non-test `.go` file of its package; its only repository import is `internal/contracts`.
2. `api_test.go` has `TestEvaluatorContractFake` (new: a fake `Evaluator` answers a made-up AAA row and a caller reads `BelowBuy`, `OneSigma`, `BreakEvenInSigmas`, `EdgePerShare`, `EdgeWhy`, `Skew` and `Labels`), `TestFakeValuerSatisfiesValuer` (new: a fake with a fixed fair price is assigned to `Valuer`, which has the shape of the `Value` method of `stress.Pricer`), `TestErrBadInputIsSentinel` (new) and `TestEmptyLabelsIsNonNil` (new).
3. `docs/contracts/compute/optionevidence.md` (new) states in one paragraph each: an empty figure always has a `Why` and the page prints it where the number would be; the edge is a proxy (the bid against a price from realised volatility) and the page names it "bid vs forecast price"; a label never changes whether a row passes a gate; no label is promoted to a gate before a replay shows two groups of at least 30 trades differ; the package computes no total, rank or order; the new `Evaluator` is the only place these figures are computed.

### 2 candidate-evidence

Module: compute. Size: M.
Order: after 1.
Files: `internal/compute/optionevidence/evidence.go`, `internal/compute/optionevidence/labels.go`, `internal/compute/optionevidence/evidence_test.go`, `internal/compute/optionevidence/labels_test.go`, `internal/compute/optionevidence/testdata/golden.json`
Contracts: unchanged
Design: docs/design/candidate-evidence.md#Decisions (Decisions 2 to 7)
Acceptance:
1. `evidence.go` implements `optionevidenceapi.Evaluator` (`var _ optionevidenceapi.Evaluator = ...`) with `shopspring/decimal` and `math/big.Float.Sqrt` at 128 bits formatted to 12 places, no `float64`, no clock, no import of `internal/strategy`: the 1-sigma of spot 100, IV 0.20 and 365 days is 20 and of spot 96.50, IV 0.312 and 31 days is 8.7744 to four places, with a break-even of 89.06 (strike 90.00, bid 0.94) `BreakEvenInSigmas` is 0.8479 to four places: `TestOneSigmaAndInSigmas` (new, made-up AAA row).
2. Below buy: strike 90.00, bid 0.94 and buy price 90.00 give `BreakEven` 89.06, `BelowBuy` 0.94 and `BelowBuyPct` 1.0444 to four places; a break-even above the buy price (strike 92.00, bid 0.94, buy price 90.00) gives `BelowBuy` -1.06; no buy price gives both empty with `no_buy_price` and the other figures unchanged: `TestBelowBuy` (new, table of the three; this is the figure the page sorts companies by).
3. With a fake `Valuer` that returns the fixed fair price 0.74, a bid of 0.94 gives `EdgePerShare` 0.20 and, with multiplier 100, `EdgePerContract` 20.00, and no label `no_edge_at_bid`; a fixed fair price 1.10 gives -0.16 and the label; a fair price equal to the bid gives 0 and the label: `TestEdgeAtBid` (new, table of the three).
4. Reasons, not numbers: an earnings date before expiry leaves the edge empty with `EdgeWhy` `earnings_before_expiry` and the 1-sigma still given; no forecast gives `no_forecast`; no rate gives `no_rate`; a `Valuer` error gives `no_fair_price`; no IV gives `no_iv` and empty 1-sigma: `TestEmptyFiguresHaveReasons` (new, one case each; it asserts that no empty figure has an empty `Why`).
5. Skew: contract IV 0.312 against ATM IV 0.300 gives `Skew` 0.012, no ATM row gives empty with `no_atm`, and the row's forecast error is returned unchanged in `ForecastErr`: `TestSkew` (new).
6. Labels: a delta of -0.19 sets no band label, -0.35 and -0.10 set `delta_outside_band`, and so does +0.35 (the sign of the stored delta is irrelevant); with chain day 2026-11-17, ex-dividend 2026-11-25 and expiry 2026-12-18 `ex_div_inside_life` is set, with ex-dividend 2026-12-19 or none it is not: `TestLabels` (new, in `labels_test.go`); `TestLabelsDoNotTouchFigures` (new): the same row with every label set and with none gives the same figures.

## Checked by CI

`go test ./internal/compute/...` (entries 1 and 2), `go run ./scripts/arch` and go-arch-lint (the import edges of Modules), the size ratchet check, the contract-docs check of `docs/contracts/compute/`. No page changes in this design; the first page that shows the columns is plan step 9, with its own design and its own screenshot check.

## Left out

- Plan steps 9 to 11: the candidate page columns, the valuation buy line beside the hand-typed buy price (the gate stays on the hand-typed price; owner answer 3 of 2026-11-17), the ordering of companies by `BelowBuy` of the best passing row, the shortlist (the three rows per expiry nearest delta 0.15, 0.22, 0.30, trial) and the concentration shares (companies only; no sector data, no cap). All page designs.
- The name-level ATM IV minus forecast (the stored `market.vol_edge`), the IVP number, the earnings-unconfirmed mark and the wide-quote check: read from what exists, not computed here.
- Promoting any label to a gate, a score, a total, a ranking by edge: not built. Replay as a page button (plan step 12): later.
- A pricer with dividends or early exercise (the model is European and the page says so), and a forecast other than the mean of three windows (a comparison with RV60 only and RV120 only belongs to the replay).
- Parameter sources: 1-sigma formula (definition, citation unverified in the plan), 365 days (the ITM probability's basis), edge zero (definition), delta band 0.15 to 0.30 (trial, the owner's habit), 30 trades to promote (trial). No academic source is claimed for any gate number; Goyal and Saretto (2009) is cited by the plan only as weak directional evidence for the labels and is not used to compute anything here.
- The gates themselves, the buy-price source, `settings.underlying`, and the stored candidate rank: untouched.

## Adversarial

Answers to the window's file adversarial-d1-d5-2026-11-17.md (outside the repository), section D5 and the top part.

- Top, PR conflict on the entry cell of line 92: kept, the window rebases the second. Precedent `optionfigapi`: adopted, replaced by `companydataapi`.
- 1 Gap, no `BuyPrice` and no below-buy figure: adopted (`Row.BuyPrice`, Decision 2, `TestBelowBuy`). Name gap recomputed although `market.vol_edge` is stored: adopted, dropped (Decision 6). Data days of the forecast and the ATM row not in `Row`: not adopted; the page holds each reading and its day (Decision 8).
- 2 The edge is a variance-premium proxy: adopted in naming (Chinese section, Decision 4, the contract note and the page string "bid vs forecast price"); printing past replay hit rates is not adopted, replay is plan step 12.
- 3 Drop `NameGap` and the duplicate labels `earnings_unconfirmed` and `wide_quote`: adopted (Decisions 6 and 7).
- 4 `Share` meaning two things: adopted, `EdgeShare` is `EdgePerShare` and `BreakEvenShare` is `BreakEvenInSigmas`. Free-string label set: not adopted; the three names are listed in Decision 7 and tested by name, and a new label is a design change.
- 5a Decimal square root: adopted, `math/big.Float.Sqrt` at 128 bits, 12 places (Decision 3); standard library, so no own implementation and no extra file. 5b `earnings_before_expiry` appears only on blocked rows: adopted as a statement in Decision 1. 5c Sign of the stored delta: adopted, the label uses the absolute value and a +0.35 test. 5d numbers check out: noted. 5e Nil sentinel: adopted (`errors.New`).
- 6 Command-run acceptance item 1 and the seven-assertion item: adopted, replaced by diff-checkable statements and split into items 1 to 6; sqrt precision is stated in item 1 and Decision 3.
