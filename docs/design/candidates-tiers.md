# candidates tiers: the page options/candidates (with a leading slash) sorts the short puts into three tiers, "fits your conditions", "check" and a collapsed "not judged", and shows six columns first

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-18

Source: the plan `plan-2026-11-17.md` (owner approved), sections 早上：候选页, 候选的依据 (显示什么, 硬闸门, 软条件, 排序, 集中度), 分步开发 steps 9, 10 and 11, 不做的事; the mockup `candidates-table-mockup.html` (owner approved, answer "a", 2026-11-17), whose screenshots show the first view, the expanded row, the legend and the collapsed groups; the owner's tier rules of 2026-11-17 (Decision 2); the merged designs `docs/design/option-candidates.md` and `docs/design/candidates-evidence.md`. This design is plan step 11 pulled ahead of step 10; step 10 (the valuation buy line) is not here, see Left out. The adversarial review is in `## Adversarial`.

Checked on origin/main (dd0801c0): `option_candidates_page` has `Config{RunID, BuildID, Scope, Zone, Candidates, Figures, Names, Evidence, ATMFor, Readings}` (`page.go`, #2779); `Build` reads each kind with `readKind`, builds the short-put group as a main table of the ok rows and a second table of the blocked and unknown rows (`groups.go`), and writes the viewModel of `builder.go`. The evidence step (`docs/design/candidates-evidence.md`) is in flight: its compute entry (ATM picker, percent fields), page entry (`evidence.go`, `evcells.go`, `readings.go`, eight columns) and ot entry (`cmd/ot/evidence_site.go`, `candidateReaders`) are not on main; every entry of this design that edits those files is ordered after them. `internal/compute/optionevidence` (`New`, `DefaultParams`, `Evaluate` with `BelowBuy`, `BelowBuyPct`, `OneSigma`, `BreakEvenInSigmas`) is on main. `internal/compute/ruleparam` holds twelve keys (`registry.go`, 71 lines; `Resolve(stored)` fills defaults) and the settings form reads them from `settings.rule_param` through `ownerset.Reader.Params` (`cmd/ot/settings.go`); no consumer on a page reads one yet. `store.AccountReader` (`Values`, `Positions`) is used by the notices page and `cmd/ot` holds it as `rd.account` and the owner reader as `rd.owner`. `cmd/ot/wire.go` has 783 lines and no ceiling in `docs/design/ratchet.json` (the limit is 800); no entry below edits it.

## What you see

On the page options/candidates (with a leading slash) the first thing is the short-put view; the three tabs of today stay below it and their stored tables are unchanged. The numbers are invented (companies AAA, BBB, CCC; AAA spot 96.50, buy price 90.00; BBB spot 41.20, no buy price; cash of account A $120,000.00, none committed). The labels are English, the explanations are Chinese; the figures in the sample are what `optiontier` and `optionevidence` give for these rows.

    Option candidates   chain day 2026-11-17   rule put_v1
    AAA  Spot $96.50 (2026-11-17)  Buy price $90.00  Next earnings 2027-01-28    2 fit, 1 check, 2 collapsed
    BBB  Spot $41.20 (2026-11-17)  Buy price: not set   Next earnings unknown     0 fit, 0 check, 1 collapsed
    Earnings unconfirmed   本次运行的每一行都有：财报日来自数据商，没有人工核实。这不会改变任何一行的档位。
    [legend: ✓ Fits | ! Check | – Not judged | the cash line]   (each prints its conditions with today's values)

      TIER        CONTRACT                    MARGIN OF SAFETY          BELOW SPOT           PREMIUM / YIELD     BADGES
    2 ✓ fit your conditions
      ✓ Fits     AAA $85.00  12-18  31d      6.0%                      12.4%                $42.00              Thin premium
                                              BE $84.58 vs $90.00       1.3 moves*           5.8%/yr
      ✓ Fits     AAA $90.00  12-18  31d      1.0%                      7.7%                 $94.00
                                              BE $89.06 vs $90.00       0.9 moves*           12.3%/yr
    1 ! to check
      ! Check    AAA $87.50  12-18  31d      3.4%                      9.9%                 $60.00              No open interest
                                              BE $86.90 vs $90.00       1.1 moves*           8.1%/yr
    > 1 - not judged: break-even above buy price (collapsed)
    > 1 - in the money (collapsed)       AAA $100.00  12-18  Bid $5.40  intrinsic $3.50, time value $1.90  22.4%/yr
    > 1 - not judged: no buy price (collapsed)

    (a row opened with the arrow) IV (this contract) 30.5% | Delta (rough chance of assignment) −0.19 (about 19%) | ITM probability 21.8%
      | Vs cash 8.0% | Account cash A: enough, $120,000.00 free after $0.00 committed (2026-11-17) | Bid per share $0.94 | the eight evidence fields of candidates-evidence

    * moves = how many expected moves the break-even is below the spot: (spot − break-even) / (spot × this contract's IV × sqrt(years)).

The first view has seven columns: the arrow, the tier (word, colour and icon), the contract (name, strike, expiry, days to expiry), the margin of safety, the break-even below spot with its expected moves, the premium per contract with the time-value yield per year, and the badges. A tier that is empty shows its heading with the count 0 and no rows. A collapsed group lists its rows when opened, each as one line with the contract, the break-even, and the intrinsic and time value beside an in-the-money row. No row is missing: the counts of the groups sum to the rows read. A figure that cannot be formed is a gap cell with its reason in words, never a zero.

## Goal

The page answers "which short put fits what I have said I want, and which need a second look", with every judgement computed in Go from stored rows and named conditions, every condition printed with its current value, and no row hidden. The owner sets one of the conditions himself (the delta bound).

## Today

- The short-put group shows the ok rows, in the stored `rank` order, which the job writes by annualized yield (`rankRun`, `internal/jobs/optioncandidates/convert.go:185-224`); the rule's own order inside a company is by margin of safety (`internal/compute/optioncand/rank.go`), overwritten at write time. Blocked rows (a failed gate) and unknown rows (an unknown gate) go to a second table with their reasons. `docs/design/option-candidates.md` Decision 6 says "Moneyness is not filtered" and gives an in-the-money row the mark `itm`.
- All five put gates are evaluated for every row (no short-circuit, `evaluatePut`): `buy_price`, `quote`, `events`, `funding`, `cash_rate`; the row keeps its `Gates` (JSON), `Marks` (JSON words: `itm`, `earnings_unconfirmed`, `earnings_beyond_horizon`, `no_buy_price`, `no_open_interest`, `no_account`) and `Fits` (JSON, alias to fit, no_fit or unknown). `Spot`, `BuyPrice`, `Delta`, `Iv`, `Ask` are stored (pointers, empty when none); `BreakEven` is empty when there is no bid. A row with no buy price passes the gate with the mark `no_buy_price` (owner, 2026-11-12), so today every row of a name with no buy price is an ok row.
- The events gate blocks `earnings_before_expiry` for any next earnings date on or before expiry, whether the date is the vendor's or the owner's override; the stored row keeps no flag that says which (`input.go` writes `EarningsState` from the vendor row only).
- `funding` compares strike times multiplier with the alias's `AvailableFunds` and ignores short puts already open. The notices page and the report meter `cash_after_assignment` already define the honest figure: `TotalCashValue` minus the sum over the short puts of strike x multiplier x contracts (`docs/conventions/register.toml`, the entry of `report_page.account_moves`), from `AccountReader.Values` and `Positions` (positions carry `SecType`, `Right`, `Strike`, `Multiplier`, `Quantity`, `Currency`).
- `optionevidence.Evaluate` gives, per row, `BelowBuy` and `BelowBuyPct` (the margin of safety as a percent of the buy price), `OneSigma` and `BreakEvenInSigmas` = (spot − break-even) / one sigma, with one sigma = spot x IV x sqrt(DTE / 365): these are the owner's margin and expected moves, so this design reuses them.
- Rule parameters: twelve keys in `internal/compute/ruleparam/registry.go` with kind, bounds and a default; the owner changes them on the settings page; `Resolve(stored)` gives each key's stored text or default. A key is added there by a reviewed change.
- The web renders the short-put table by column id (`CandidateGroup.tsx`, shared `DataTable`); `OptionCandidatesPage.tsx` shows one tab per group. The generated type is `web/src/generated/option_candidates_page.v1.ts`; the schema has `additionalProperties: false`; the ADR of the front-end stack (`docs/adr/0034-frontend-stack.md`) says additive optional fields do not bump `schema_version`.

## Decisions

1. **What this design changes.** (a) The page ranks and groups the short puts by the owner's tiers (Decision 2), not by the stored `rank`; the stored order stays in the old table below. This changes the effect of Decision 6 of `docs/design/option-candidates.md` ("Moneyness is not filtered") on the page: the job still stores every strike and still marks `itm` and nothing in the job changes, but an in-the-money row is no longer listed with the others; it is counted in a collapsed group. (b) The five stored gates no longer decide which tier a row is in; the owner's conditions do, computed from the row's numbers. Each gate maps as: `buy_price` is recomputed from the numbers (grey `above_buy_price`); `quote` gives badges (Decision 7); `events` gives the badge `earnings_before_expiry` or an info badge; `funding` is replaced by the cash figure of Decision 5; `cash_rate` (yield above DGS3MO) decides nothing here and stays the stored column "Vs cash". The stored gate reasons still show in the old table. (c) The step-9 evidence call is widened from the ok rows to every row of the tiers Fits and Check (Decision 10); `docs/design/candidates-evidence.md` Decision 3 limited it to the ok rows and says step 11 may reopen it. (d) The `Labels` of the evidence step lose `delta_outside_band` in the expansion, because the tier rules have no lower delta bound (Decision 2); the stored-table column keeps it.
2. **The tiers, word for word.** A row is **green, "fits your conditions"** (never "recommended") when all hold: the buy price is set (above 0); break-even (strike − bid) is at most the buy price; the put is out of the money (strike not above spot, the stored `itm` mark's test, so an at-the-money put is not in the money); |delta| is at most the bound `put_delta_max` (Decision 6; trial; no lower bound; |delta| below 0.15 stays green and carries the note `thin_premium`); and, when the cash figure is built (Decision 5), at least one account's free cash covers the full strike times the multiplier. A row is **amber, "check"** when the first three hold (buy price set, break-even at most the buy price, out of the money) and one or more look-at badges are present (Decision 7). A row is **grey, collapsed**, when a hard condition fails or cannot be judged; the counts are in the group headings and in the run line. An unreadable delta (not stored) is a look-at badge `delta_unknown`, so such a row cannot be green.
3. **Grey, one reason per row, in this order of precedence**: `no_buy_price` ("N not judged: no buy price"), `no_spot` ("N not judged: no spot"), `in_the_money` ("N in the money"), `no_bid` (no bid, or a bid not above 0; "N not judged: no bid"), `above_buy_price` ("N break-even above buy price"), `bad_input` ("N not judged: unreadable numbers", an `ErrBadInput` of the evaluator). The groups are collapsed; the groups with a count of 0 are not drawn; no row of any status is dropped (the old behaviour that missing buy price hides nothing is kept, owner 2026-11-12). The counts per group and per company come from compute.
4. **Figures, all computed in compute** (`optiontier`, with the evaluator injected): margin of safety = `BelowBuyPct` of the evaluator (percent of the buy price, break-even under the buy price; negative when above); break-even below spot (percent) = (spot − break-even) / spot x 100; expected moves = `BreakEvenInSigmas` with the contract's own IV (empty, gap "no IV", when there is none; always labelled "moves*" with the formula in the hint); premium per contract = bid x multiplier; intrinsic = max(0, strike − spot); time value = bid − intrinsic; yield = time value / strike x 365 / DTE x 100 per year on the cash secured (strike); the intrinsic is shown beside an in-the-money row. Order inside a tier: margin of safety descending, then yield descending, then DTE, strike and key ascending. Greys are ordered by reason, then expiry, strike, key. The page sorts nothing and sums nothing; the web does not sort.
5. **Cash net of committed strikes is built from stored data.** Per account alias, `optiontier` takes `TotalCashValue` (USD) and the alias's option positions of the newest capture and gives `free = TotalCashValue − sum over positions with SecType OPT, Right put, Quantity below 0, Currency USD of strike x multiplier x |quantity|` (the formula the report meter `cash_after_assignment` defines; the meter takes the obligations from the replayed option ledger, this view from the positions of the newest capture). An alias figure is known when the value is present, in USD, the capture day is at most `MaxCashAgeDays` (3, trial: it spans a weekend) before the chain day, and no short put is in another currency (the register says an obligation in another currency is not netted, so this figure would be wrong). The run's cash is **built** when at least one alias is known; the aliases of `Config.Aliases` are shown in the expansion, each as enough, short or "no figure" (with the reason). A row's cash fits when a known alias has `free` at least strike x multiplier. When cash is not built, cash is a mark, not a condition: rows carry the info badge `cash_unknown` (or are summarised once above the table, Decision 7) and the legend says in Chinese that cash is only a mark because the figure cannot be built from the stored data. The page reads `AccountReader.Values` and `Positions` once per alias per build and passes the stored texts unchanged; `optiontier` does the sums. This does not recompute `AvailableFunds` fits of the stored rows; it replaces them on this view.
6. **The delta bound is a setting.** A thirteenth rule parameter `put_delta_max`: label "Maximum put delta", unit `delta`, kind decimal, default "0.3" (trial), minimum "0.05", maximum "0.5". It lives where every rule parameter lives: `settings.rule_param` through `ownerset.Reader.Params(ctx, at)`, shown and changed on the settings page with no page change (the form lists `Specs()`). The candidates page reads it per build with `Config.Owner.Params(ctx, generatedAt)` and `Config.RuleParams.Resolve`; a nil `Owner` or `RuleParams` gives the default 0.3 and the legend says "default" (not "set by you"). The thin-premium line 0.15 and the cash age 3 days are constants of `optiontier.DefaultParams`, trial values, printed in the legend.
7. **Badges.** Look-at (amber; the owner's list, from the first reason each stored gate keeps, so a row with two quote problems shows one badge): `no_open_interest` (stored mark `no_open_interest`), `low_open_interest` (quote reason `low_open_interest`), `one_sided_quote` (quote reasons `no_quote`, `no_bid`, `no_ask`, `small_bid_size`), `wide_quote` (`wide_spread`), `stale_quote` (quote reason `delayed_data`, or events reason `stale_chain`), `earnings_before_expiry` (events reason; the badge text names the stored next earnings date and says in Chinese that the stored row does not tell a confirmed date from a vendor date), `cash_short`, `delta_above_bound`, `delta_unknown`. Info (never demote, never amber by themselves): `earnings_unconfirmed`, `earnings_beyond_horizon`, `earnings_unknown` (events reasons `no_earnings`, `stale_earnings`, `earnings_unsupported`, `earnings_error`, `beyond_horizon`), `cash_unknown`. Note: `thin_premium`. An info mark carried by every row read is returned once in `Result.Common` and removed from the rows' badges; the page prints it once above the table with its Chinese gloss and "this never changes a row's tier" (the earnings_unconfirmed case of the owner). Concentration (a share of cash secured above a share he gives) is not delivered; see Left out.
8. **Who computes what.** `optiontier` (compute) is the one place that decides tier, grey reason, badges, order, counts, cash and the figures of Decision 4; it calls `optionevidenceapi.Evaluator` for margin and moves (one injected value; no second formula). `option_candidates_page` places texts: it passes the stored texts, calls `Tierer.Judge` once, calls the evidence builder of the evidence step for the Fits and Check rows, and writes cells from the answer; it adds, compares and sorts nothing except testing a stored text for empty. The web renders the payload and computes nothing.
9. **Payload.** One additive optional top-level field `short_put_tiers` (`null` when absent) of `page.option_candidates_page.v1`; `schema_version` stays 1 (ADR 0034: additive optional fields do not bump). The three groups of today stay unchanged, so the stored tables, the `ot page` output and every consumer of them are unchanged; the web shows the tier view first and the tabs below under the heading "Stored tables". Shapes in Interface. With a nil `Candidates` or no short-put rows the field is `null` and the group gap shows as today. The page writes every number as text; no JSON number is added.
10. **Rows, the strip and the expansion.** The first view is the seven columns of "What you see". The company strip shows once per company: ticker, spot with its data day, buy price (or "not set"), next earnings, and the counts "N fit, N check, N collapsed". The expansion of a Fits or Check row has these fields in order (label English, hint Chinese, each hint ends with the data day of its figure): `IV (this contract)`, `Delta (rough chance of assignment)` (|delta|, the word "rough" in the label), `ITM probability`, `Vs cash`, `Account cash` (one line per alias), `Bid per share`, then the eight evidence fields of `docs/design/candidates-evidence.md` (below buy, one-sigma move, bid vs forecast price, skew, forecast vol, IVP, name IV vs forecast, labels without `delta_outside_band`) with their gap texts and `as of` badges as that design writes them. ITM probability and Vs cash come from `Config.Figures` for these rows. The data day of the spot and of every figure is in a hint or a badge.
11. **Language.** Explanations and risk texts (tier explanations, grey reasons, badge glosses, legend lines, the run line, expansion hints, the cash note) are Chinese; labels (column titles, tier words, badge labels, field labels) and every number stay English. Texts, word for word (a value in braces is filled from the answer): tier Fits: 满足你的全部条件：已设买入价；保本点（行权价减 bid）不高于买入价；价外；|delta| 不高于 {delta_max}（{delta_source}）；{cash_clause}。这不是买入建议。; `{delta_source}` is 试用值，可在设置里改 or 默认值; `{cash_clause}` is 现金够付全额行权价（已扣除已卖出的 put 占用的行权价） or 现金只做标记，现有数据算不出全额行权价覆盖. Tier Check: 已设买入价、保本点不高于买入价、价外这三项都满足，但有需要再看一眼的徽标。 Grey group: 未评判，已折叠，只显示数量；点开可看每一行为什么在这里。 Grey reasons: no_buy_price 没有设买入价，无法判断保本点是否合适；不是被隐藏，只是没评判。 / no_spot 没有现价，无法判断是否价外。 / in_the_money 价内：行权价高于现价，卖出后几乎一定被指派（按行权价买入股票）；权利金里含内在价值，收益只按时间价值算。 / no_bid 没有 bid，无法算保本点。 / above_buy_price 保本点（行权价减 bid）高于你的买入价：被指派后的成本高于你愿意付的价。 / bad_input 数据读不出来，没有评判。 Badge glosses: no_open_interest 未平仓量没有记录或为零，可能难以成交。 / low_open_interest 未平仓量低于设置的最小值，成交可能困难。 / one_sided_quote 只有单边报价，看到的价格不一定能成交。 / wide_quote 买卖价差过大，实际成交价可能比 bid 差。 / stale_quote 报价延迟或不是最新交易日，数字可能已变。 / earnings_before_expiry 到期前有财报，财报日股价可能大幅跳空；存的行不区分日期是否已人工确认。 / cash_short 现金不够付全额行权价（已扣除已卖出的 put 占用的行权价）。 / delta_above_bound delta 高于你设的上限，被指派的概率较高（粗略等于 |delta|）。 / delta_unknown 没有存 delta，无法核对上限。 / thin_premium |delta| 低于 0.15，权利金薄，仍然满足条件。 / earnings_unconfirmed 财报日来自数据商，没有人工核实，不改变档位。 / earnings_beyond_horizon 近期刚报过财报，到期前不会再报，不改变档位。 / earnings_unknown 没有可用的财报日，不改变档位。 / cash_unknown 没有现金数字，现金只做标记。 Run line (a common mark): 本次运行的每一行都有：{label}。{gloss}这不会改变任何一行的档位。 Labels (column titles, tier words) are the English words of "What you see".
12. **Number text.** Money: `$`, thousands separators, two decimals, rounded half away from zero, U+2212 for a negative; a percent has one decimal; moves one decimal; the existing page helpers `fixedText` and the money helper of `format.go` are used and the page does not add, subtract, multiply or divide. A percent is a compute text at four decimals, as in the evidence design. The sort value of a cell is the decimal text, as `figValueCell` writes it.
13. **Web.** A new view `TierView` renders `short_put_tiers`: a company strip, the run line, a four-part legend, the tier table with one accordion item per row (the expansion), and the collapsed grey groups as accordions. The badge column is wider than the mockup's (at least 16rem, wrapping, so the 1160px width no longer cuts a badge). Colour never carries a tier alone: each tier has a word and a Lucide icon. At 640px and below a row wraps to two lines (contract, then the figures), as the phone mockup. The legend and every gloss come from the payload. The view counts, sorts, filters and formats nothing; it shows the checkbox of the mockup ("show today's real state") nowhere (it was a mockup device).
14. **Wiring.** `cmd/ot` builds the `Tierer` in a constructor whose return type is the receiver's api type, `newTierer() optiontierapi.Tierer`, returning `optiontier.New(newEvaluator())` where `newEvaluator()` returns the `optionevidenceapi.Evaluator` the evidence entry already builds; the page `Config` gets `Tiers`, `Account` (`rd.account`), `Aliases` (`rd.aliasList()`), `Owner` (`rd.owner`) and `RuleParams` (`deps.newRuleParams()`). The named path `ot page --name option_candidates_page` passes none of them (the tier field is `null`, the gap `no_reader`).
15. **Cost per request.** Per build: one `Judge` call over all short-put rows read (hundreds), per alias one `Values` and one `Positions` read for each day tried (the newest day with a value within 3 days before the group day: at most 4 days, so at most 8 reads per alias), one owner `Params` read. No chain or terms read is added (the evidence step's ATM reads stay as they are, for the Fits and Check rows only, which are fewer than the ok rows of today).
16. **Trial values.** `put_delta_max` default 0.3, the thin-premium line 0.15, the cash age 3 days; the owner sets them after seeing the page. The first is a setting, the others constants named in the compute contract doc.

## Modules

`docs/design/modules.md`: the interface cell of the compute row gains one package, no row is added. New packages: `internal/compute/optiontierapi` (interface) and `internal/compute/optiontier` (implementation) under the glob `internal/compute/**` of the compute row. This design's pull request adds `internal/compute/optiontierapi` to the interface cell of the compute row of `docs/design/modules.md` (the one line it edits, as `optionattrapi` and `optionevidenceapi` were added by their designs); the depends cell is unchanged. Paths and the first row whose glob matches: `internal/compute/optiontierapi/**`, `internal/compute/optiontier/**`, `internal/compute/ruleparam/**`, `internal/compute/settingsapi/api.go`, `docs/contracts/compute/optiontier.md` compute; `internal/pages/option_candidates_page/**` and `docs/contracts/pages.md` pages; `cmd/ot/**` and `docs/contracts/ot/pages/option_candidates_page.md` ot; `web/**` web; `contracts/schemas/**`, `internal/contracts/**` and `docs/contracts/page-schemas.md` contracts (the generated `web/src/generated/option_candidates_page.v1.ts` goes with its schema, as the P&L page entry 1 does). Imports against the `depends` column: pages imports `optiontierapi`, `settingsapi`, `store`, `store/ownerset`; not `optiontier`, `optionevidence`, `strategy`, `bsm`; compute imports `contracts` only (`optiontier` imports `optionevidenceapi` and `optiontierapi`, both compute); ot imports `optiontier` and what it already imports. Entry 3 edits `settingsapi/api.go`, an interface file, for one comment line ("thirteen keys"); that is an interface change with this design's Design line and no declaration changes.

## Interface

Entry 1 (module compute, new file `internal/compute/optiontierapi/api.go`, declarations only):

    // Params are the numbers of the tier rule; DefaultParams of optiontier gives the trial values.
    type Params struct {
        DeltaMax       string // "0.3": the bound of |delta| (the owner's setting put_delta_max)
        ThinDelta      string // "0.15": below it a green row carries the note thin_premium
        MaxCashAgeDays int    // 3: the oldest capture, in days before Day, a cash figure may have
        DaysInYear     int    // 365
    }

    // Position is one stored account position as text; the rule picks the short puts.
    type Position struct{ SecType, Right, Strike, Multiplier, Quantity, Currency string }

    // AccountCash is one alias's newest capture: the TotalCashValue text, its currency, the capture day and
    // the positions of that capture. Cash is "" when the value is not stored.
    type AccountCash struct {
        Alias, Cash, Currency string
        CaptureDay            contracts.Day
        Positions             []Position
    }

    // Row is one stored short-put row. Company is the key rows are counted by (the underlying id as text).
    // QuoteReason and EventsReason are the reason codes of the stored quote and events gates ("" when the gate
    // passed); Marks are the stored mark words.
    type Row struct {
        Key, Company                          string
        ChainDay, Expiry                      contracts.Day
        DTE                                   int
        Spot, Strike, Bid, BuyPrice           string
        IV, Delta, Multiplier                 string
        QuoteReason, EventsReason             string
        Marks                                 []string
    }

    type Input struct {
        Day      contracts.Day // the chain day
        Params   Params
        Rows     []Row
        Accounts []AccountCash
    }

    // CashCell is one alias's verdict for one row: Verdict is fits, short or unknown; Free and Committed are
    // money text; Why is the reason when unknown (stale_cash, no_cash, not_usd, other_currency).
    type CashCell struct{ Alias, Verdict, Free, Committed, Why string }

    // Verdict is the judgement of one row. Tier is fits, check or grey; Grey is the reason when Tier is grey
    // (no_buy_price, no_spot, in_the_money, no_bid, above_buy_price, bad_input). Place is the 1-based position
    // inside the tier. The money fields are decimal text; the percent fields are percents at four decimals;
    // an empty text is "no value" and Why names the figure that could not be formed.
    type Verdict struct {
        Key, Tier, Grey                    string
        Place                              int
        Badges, Infos, Notes               []string // codes, never nil
        BreakEven, BelowBuy, MarginPct     string
        BelowSpotPct, Moves                string
        PerContract, Intrinsic, TimeValue  string
        YieldPct                           string
        Cash                               []CashCell
        Why                                map[string]string // figure id to reason code
    }

    // CompanyCount is the tier counts of one company, in order of first appearance.
    type CompanyCount struct{ Company string; Fits, Check, Grey int }

    // Result: Verdicts are in input order; Order is the keys in display order (fits, check, then grey by
    // reason); Common is the info marks every row carries; CashBuilt says whether any alias figure is known;
    // Params echoes the numbers used, for the legend.
    type Result struct {
        Verdicts  []Verdict
        Order     []string
        Companies []CompanyCount
        Common    []string
        CashBuilt bool
        Params    Params
    }

    // Tierer judges all short-put rows of one build at once. It writes nothing, reads no clock and returns
    // optionevidenceapi.ErrBadInput's cause only inside a row (the row becomes grey bad_input); any other
    // error is returned.
    type Tierer interface{ Judge(in Input) (Result, error) }

Entry 3 (module compute, `internal/compute/ruleparam/registry.go`): the table `specs` gains, after `exit_dte`, `{Key: "put_delta_max", Label: "Maximum put delta", Unit: "delta", Kind: kindDecimal, Default: "0.3", Min: "0.05", Max: "0.5"}`; `Specs()` and `Resolve(nil)` have thirteen entries.

Entry 4 (module contracts, `contracts/schemas/page.option_candidates_page.v1.json`): the top-level property `short_put_tiers`, not in `required`, `oneOf` null or `TierView`; every new object has `additionalProperties: false` and every figure is a `Text`:

    TierView   { chain_day Text; companies [Company]; common [Mark]; legend [LegendItem]; columns [TierColumn];
                 tiers [Tier]; hidden_counts Text }
    Company    { key Text; ticker Text; spot Text; spot_day Text; buy_price Text | null; buy_price_note Text | null;
                 next_earnings Text; counts Text }
    Mark       { code Text; label Text; gloss Text }            // said once above the table
    LegendItem { tier Text; title Text; text Text }             // text is Chinese with the current values
    TierColumn { id Text; label Text; hint Text | null }
    Tier       { id Text (enum: fits, check, grey_no_buy_price, grey_no_spot, grey_in_the_money, grey_no_bid, grey_above_buy_price, grey_bad_input); word Text; icon Text (check, alert, minus); title Text;
                 explain Text; collapsed boolean; rows [TierRow] }
    TierRow    { id Text; company Text; contract Contract; margin Pair; below_spot Pair; premium Pair;
                 badges [Badge]; notes [Badge]; fields [Field]; line Text | null }
    Contract   { name Text; strike Text; expiry Text; dte Text }
    Pair       { main Text; sub Text | null; gap Gap | null }
    Badge      { code Text; label Text; gloss Text; tone Text (warn, info, none) }
    Field      { id Text; label Text; hint Text | null; value Text | null; gap Gap | null; badges [Badge] }

`line` is the one-line text of a grey row. Entry 5 (module pages, `page.go`): `Config` gains `Tiers optiontierapi.Tierer`, `Account store.AccountReader`, `Aliases []string`, `Owner ownerset.Reader` and `RuleParams settingsapi.RuleParams`, each documented as "nil gives the gap no_reader" except the last two ("nil gives the default 0.3").

## Issues

Build order: entries 1, 3, 4 and 5 are the interface entries and land first, in parallel; 2 (the rule) follows 1; 6 (pages) follows 1, 3, 4, 5 and the evidence entries it names; 7 (ot) follows 1, 2, 5 and the evidence ot entry; 8 (web) follows 4 only. The evidence entries ("candidates-evidence-atm", "-page", "-ot" of `docs/design/candidates-evidence.md`, in flight as the pull requests of that design) are named in each Order line. Companies in every test and fixture are AAA, BBB, CCC with invented numbers. No migration; nothing holds the auto-deploy.

### 1 tiers-api (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/optiontierapi/api.go`, `internal/compute/optiontierapi/api_test.go`
Contracts: unchanged
Design: docs/design/candidates-tiers.md#Interface
Plan: P2
Acceptance:
1. `api.go` (new) declares `Params`, `Position`, `AccountCash`, `Row`, `Input`, `CashCell`, `Verdict`, `CompanyCount`, `Result` and `Tierer` as in the Interface section, with docs, no function body and no import other than `internal/contracts`; the file is under 600 lines.
2. `api_test.go` (new) has `TestTiererContractFake` (new: a fake `Tierer` answers a green row, an amber row and a grey row and a caller reads `Tier`, `Grey`, `Place`, `Badges`, `Cash`, `Order`, `Common` and `CashBuilt`) and `TestVerdictListsNonNil` (new: the fake's answer for a row with no badge has non-nil empty `Badges`, `Infos` and `Notes`).
3. `git diff --name-only BASE` lists only the two files of the Files line; `internal/compute/api.go` and `docs/design/ratchet.json` are not in it.
4. The result matches docs/design/candidates-tiers.md#Interface.

### 2 tiers-rule

Module: compute. Size: L.
Order: after 1.
Files: `internal/compute/optiontier/tier.go`, `internal/compute/optiontier/grey.go`, `internal/compute/optiontier/badges.go`, `internal/compute/optiontier/cash.go`, `internal/compute/optiontier/figures.go`, `internal/compute/optiontier/order.go`, `internal/compute/optiontier/tier_test.go`, `internal/compute/optiontier/grey_test.go`, `internal/compute/optiontier/badges_test.go`, `internal/compute/optiontier/cash_test.go`, `internal/compute/optiontier/figures_test.go`, `internal/compute/optiontier/order_test.go`, `internal/compute/optiontier/purity_test.go`, `internal/compute/optiontier/contract_test.go`, `internal/compute/optiontier/testdata/tiers.json`, `internal/compute/optiontier/testdata/cash.json`, `docs/contracts/compute/optiontier.md`
Contracts: unchanged
Design: docs/design/candidates-tiers.md#Decisions
Plan: P2
Acceptance:
1. `tier.go` (new) declares `New(ev optionevidenceapi.Evaluator) optiontierapi.Tierer` with `var _ optiontierapi.Tierer = ...` and `DefaultParams() optiontierapi.Params` giving `DeltaMax` "0.3", `ThinDelta` "0.15", `MaxCashAgeDays` 3, `DaysInYear` 365, each with a comment saying trial; the package uses `shopspring/decimal` only (already a direct dependency; no new library), has no `float64`, no clock, no import of `internal/strategy`, `optionevidence` or `bsm`, and every non-test file is under 600 lines.
2. `Judge` puts a row in the tier Fits when the buy price is above 0, strike minus bid is at most the buy price, the strike is not above the spot, |delta| is at most `Params.DeltaMax` and, when `Result.CashBuilt`, some known alias has `Free` at least strike x multiplier; a row is Check when the first three hold and a look-at badge is present; |delta| below `ThinDelta` adds the note `thin_premium` and does not leave Fits; an empty delta adds `delta_unknown` (the names `DeltaMax`, `ThinDelta`, `CashBuilt`, `thin_premium` and `delta_unknown` are new).
3. `grey.go` (new) gives the grey reasons in the order of precedence of Decision 3 (`no_buy_price`, `no_spot`, `in_the_money`, `no_bid`, `above_buy_price`, `bad_input`); a row with no buy price and in the money is `no_buy_price`; a bid that is empty or not above 0 is `no_bid`; the mark `unreadable_gates` is `bad_input`; an `ErrBadInput` of the injected evaluator (`errors.Is`) makes that row `bad_input`, and any other evaluator error is returned by `Judge`; no row is dropped (`len(Verdicts)` equals `len(Rows)` and `Order` holds every key once).
4. `badges.go` (new) maps `QuoteReason`, `EventsReason` and the marks as Decision 7 lists (look-at, info, note), in a fixed order; a mark carried by every row goes to `Result.Common` and out of each row's `Infos`; `earnings_before_expiry` is a look-at badge for any events reason `earnings_before_expiry`, and `earnings_unconfirmed` never adds a look-at badge.
5. `cash.go` (new) builds `free` as `Cash` minus the sum of strike x multiplier x |quantity| over the positions with `SecType` "OPT", `Right` "put", a negative `Quantity` and `Currency` "USD"; an alias is unknown with `Why` `no_cash` (empty cash), `not_usd` (currency other than USD), `stale_cash` (capture older than `MaxCashAgeDays` before `Input.Day`, compared as day texts by a difference in days) or `other_currency` (a short put in another currency); `CashBuilt` is true when some alias is known; with it false no row gets `cash_short` and each row gets the info `cash_unknown`.
6. `figures.go` (new) writes `MarginPct` and `BelowBuy` from the evaluator's `BelowBuyPct` and `BelowBuy`, `Moves` from `BreakEvenInSigmas`, `BelowSpotPct` = (spot − break-even) / spot x 100, `PerContract` = bid x multiplier, `Intrinsic` = max(0, strike − spot), `TimeValue` = bid − intrinsic and `YieldPct` = time value / strike x `DaysInYear` / DTE x 100, percents at four decimals and money at the decimal result, with an empty text and a `Why` code where an input is empty (`no_iv` for `Moves`); it does not call `Evaluate` for a row already grey for `no_buy_price`, `no_spot` or `no_bid`.
7. `order.go` (new) orders inside a tier by `MarginPct` descending, then `YieldPct` descending, then DTE, strike and key ascending, sets `Place`, fills `Order` (fits, check, then grey by reason in the order of item 3, then expiry, strike, key) and `Companies` in order of first appearance.
8. `tier_test.go`, `grey_test.go`, `badges_test.go`, `cash_test.go`, `figures_test.go` and `order_test.go` (all new) have, reading their cases from `testdata/tiers.json` and `testdata/cash.json` (new, invented numbers: AAA spot 96.50, buy price 90.00, bid 0.94 at strike 90.00 gives margin "1.0400" percent, `YieldPct` "12.3000" per year): `TestFitsAllConditions` (new), `TestCheckNeedsHardThree` (new), `TestDeltaAboveBoundIsCheck` (new: |delta| 0.34 against 0.3), `TestDeltaBoundIsParam` (new: the same row is Fits with `DeltaMax` "0.4"), `TestThinPremiumStaysGreen` (new: delta −0.11 gives Fits with the note), `TestDeltaUnknownIsCheck` (new), `TestAtTheMoneyIsNotItm` (new), `TestGreyPrecedence` (new), `TestEvaluatorBadInputIsGrey` (new), `TestEvaluatorOtherErrorReturned` (new), `TestEveryRowInOrder` (new), `TestQuoteReasonsMapToBadges` (new), `TestEarningsBeforeExpiryBadge` (new), `TestCommonMarkOnceNeverDemotes` (new: every row has `earnings_unconfirmed`, it is in `Common` and no row left Fits for it), `TestCashNetOfCommittedStrikes` (new: cash 120000.00, committed 90000.00 by a short put at 90.00 x 100 x 10, a strike of 90.00 does not fit), `TestCashAliasUnknownReasons` (new: the four reasons), `TestCashNotBuiltIsMark` (new), `TestCashShortIsCheck` (new), `TestMoneyFiguresIntrinsicAndTimeValue` (new: strike 100.00, spot 96.50, bid 5.40 gives intrinsic "3.5" and time value "1.9" and the yield of the time value only), `TestOrderMarginThenYield` (new) and `TestCompaniesCounts` (new).
9. `contract_test.go` (new) has `TestTiererContractReal` (new: the cases of entry 1's `TestTiererContractFake` run against `New(fakeEvaluator)`), and `purity_test.go` has `TestNoFloatNoClockNoIO` (new: the non-test files import none of `math`, `os`, `net/http`, `database/sql`, `time`, and contain no `float64`).
10. `docs/contracts/compute/optiontier.md` (new) has one paragraph each for: the tier rules word for word, the grey reasons and their order, the badge sets, the cash figure and its four unknown reasons, the figures and their units (money decimal, percent at four decimals), the trial values, and the evaluator injection; `git diff --name-only BASE -- internal/compute` lists no file besides the Files line.
11. The result matches docs/design/candidates-tiers.md#Decisions.

### 3 tiers-delta-param (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/ruleparam/registry.go`, `internal/compute/ruleparam/doc.go`, `internal/compute/ruleparam/registry_test.go`, `internal/compute/ruleparam/normalize_test.go`, `internal/compute/settingsapi/api.go`
Contracts: unchanged
Design: docs/design/candidates-tiers.md#Interface
Plan: P2
Acceptance:
1. `registry.go` has the new entry of Interface entry 3 after the `exit_dte` entry; `git diff BASE -- internal/compute/ruleparam/registry.go` shows only added lines and the comment edits "twelve" to "thirteen"; `doc.go` and the comment on `Specs` in `settingsapi/api.go` say "thirteen" and the diff of `settingsapi/api.go` is that one comment line.
2. `registry_test.go` has `TestSpecsThirteenWithPutDeltaMax` (new: `Specs()` has 13 entries and the last is `put_delta_max`) and its two existing length checks of 12 are edited to 13; `TestResolveFillsDefaults` is edited only for that count.
3. `normalize_test.go` has `TestPutDeltaMaxNormalize` (new: "0.30" normalizes to "0.3", "0.04" and "0.51" are refused with `ErrRefused`, "" is valid and means the default, "abc" is refused).
4. No file of `internal/pages`, `cmd/ot` or `web` is in `git diff --name-only BASE`.
5. The result matches docs/design/candidates-tiers.md#Interface.

### 4 tiers-contract (interface)

Module: contracts. Size: M.
Order: first.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.option_candidates_page.v1 changed (additive optional field short_put_tiers)
Design: docs/design/candidates-tiers.md#Interface
Plan: P2
Acceptance:
1. The schema gains the property `short_put_tiers` (not in `required`; `oneOf` null or `TierView`) and the definitions `TierView`, `Company`, `Mark`, `LegendItem`, `TierColumn`, `Tier`, `TierRow`, `Contract`, `Pair`, `Badge` and `Field` of the Interface section, each with `additionalProperties: false`; `schema_version` stays the const 1 and `git diff BASE -- contracts/schemas/page.option_candidates_page.v1.json` has no removed line except the closing punctuation of an edited array; `git grep -n '"type": "number"'` finds no new line in the schema (the new definitions hold texts and one boolean).
2. `web/src/generated/option_candidates_page.v1.ts` contains `TierView`, `Tier`, `TierRow`, `Pair`, `Badge` and `Field` (all new names) and `git diff --name-only BASE` lists it together with the schema (never edited by hand).
3. `option_candidates_page_v1_test.go` gains `TestOptionCandidatesTiersNullValid` (new: the minimal instance with `short_put_tiers` null validates), `TestOptionCandidatesTiersAbsentValid` (new: an instance without the field validates), `TestOptionCandidatesTiersOneTierValid` (new: a tier `fits` with one row, a badge and a field with a gap validates), `TestOptionCandidatesTiersUnknownFieldFails` (new: an extra field of a `TierRow` fails) and `TestOptionCandidatesTiersNumberFails` (new: a JSON number where a `Text` is expected fails); the existing tests are not edited.
4. `docs/contracts/page-schemas.md` has the paragraph of `page.option_candidates_page.v1` extended with the sentence (new) that `short_put_tiers` is additive and optional and names this design's Interface.
5. `git diff --name-only BASE -- contracts/schemas/carryover.sha256 internal/contracts/common_test.go` prints nothing.
6. The result matches docs/design/candidates-tiers.md#Interface.

### 5 tiers-page-if (interface)

Module: pages. Size: S.
Order: after 1 (and on top of the merged `Config` of the evidence step, which is on main).
Files: `internal/pages/option_candidates_page/page.go`, `internal/pages/option_candidates_page/page_test.go`
Contracts: unchanged
Design: docs/design/candidates-tiers.md#Interface
Plan: P2
Acceptance:
1. `page.go` gains in `Config` the fields `Tiers optiontierapi.Tierer`, `Account store.AccountReader`, `Aliases []string`, `Owner ownerset.Reader` and `RuleParams settingsapi.RuleParams` with the comments of the Interface section (these five names are new); every existing field keeps its name, type and order (`git diff BASE -- page.go` has no removed line) and `NewPage` and `Build` are unchanged.
2. `page_test.go` has `TestConfigTierFieldsNilBuild` (new: a `Build` with the five new fields nil returns the page as before and the short-put output has no `short_put_tiers` text other than null, built with the fakes of the existing tests).
3. The package imports `optiontierapi` (a new package) and `settingsapi` and no `optiontier`, `optionevidence`, `strategy` or `bsm`; `git diff --name-only BASE` lists only the two files.
4. The result matches docs/design/candidates-tiers.md#Interface.

### 6 tiers-page

Module: pages. Size: L.
Order: after 1, 3, 4 and 5, and after the evidence entries "candidates-evidence-atm" and "candidates-evidence-page" (it edits `builder.go` and calls the builder of `evidence.go`).
Files: `internal/pages/option_candidates_page/builder.go`, `internal/pages/option_candidates_page/evidence.go`, `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/tierrows.go`, `internal/pages/option_candidates_page/tierfacts.go`, `internal/pages/option_candidates_page/tierstrings.go`, `internal/pages/option_candidates_page/tierlegend.go`, `internal/pages/option_candidates_page/builder_test.go`, `internal/pages/option_candidates_page/tiers_test.go`, `internal/pages/option_candidates_page/tierrows_test.go`, `internal/pages/option_candidates_page/tierfacts_test.go`, `internal/pages/option_candidates_page/tierlegend_test.go`, `internal/pages/option_candidates_page/purity_test.go`, `internal/pages/option_candidates_page/testdata/tiers_ok.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_gaps.golden.json`, `docs/contracts/pages.md`
Contracts: unchanged
Design: docs/design/candidates-tiers.md#Decisions
Plan: P2
Acceptance:
1. `builder.go` gains the optional field `ShortPutTiers *tiersView` with the JSON name `short_put_tiers` in `viewModel` and one call that fills it from the short-put rows (all statuses) already read; `page.go`, `groups.go` and `docs/design/ratchet.json` are not in `git diff --name-only BASE`; the other two groups and the old short-put tables are written as before: the existing tests of the package pass, `builder_test.go` being edited only as far as the new `Config` fields need.
2. `tierfacts.go` (new) builds the `optiontierapi.Input`: one `Row` per short-put row of every status with the stored texts unchanged (`Key` the row id, `Company` the underlying id as text, `QuoteReason` and `EventsReason` the first non-pass reason of the stored gate of that name via `parseGates`, `Marks` via `parseMarks`), one `AccountCash` per alias of `Config.Aliases` from `Account.Values` (the tag `TotalCashValue`) and `Account.Positions` of the newest session day within 3 days before the group day, with all positions passed as texts; and `Params` with `DeltaMax` from `Config.Owner.Params(ctx, generatedAt)` resolved by `Config.RuleParams.Resolve` under the key `put_delta_max`, the default "0.3" when `Owner` or `RuleParams` is nil; a reader error is returned by `Build`; a row whose stored gates or marks cannot be parsed is passed with empty reasons and the mark `unreadable_gates`, which the rule turns into grey `bad_input`, so no row is dropped; the file selects no position, sums nothing and compares only stored text with the empty text.
3. `tiers.go` (new) calls `Config.Tiers.Judge` once per build, writes one `Tier` per tier of the answer in the order fits, check, then the grey reasons that have rows, each with its `word`, `icon` (check, alert, minus), `title` ("N fit your conditions", "N to check", "N not judged: no buy price", "N in the money", "N not judged: no spot", "N not judged: no bid", "N break-even above buy price", "N not judged: unreadable numbers"; N from the answer), `explain` and `collapsed` (true for greys); the `companies` strip from `Result.Companies`, the stored spot and its group day, the buy price or "not set" and the next earnings; the `common` marks from `Result.Common`; a nil `Config.Tiers` or no short-put rows gives `short_put_tiers` null and no new gap.
4. `tierrows.go` (new) writes each `TierRow`: `contract`, `margin` (`main` the percent with one decimal by `fixedText`, `sub` "BE $84.58 vs $90.00"), `below_spot` (`main` percent, `sub` "1.3 moves*" or the gap text "no IV"), `premium` (`main` money per contract, `sub` "5.8%/yr"), the badges and notes with the label and the Chinese gloss of Decision 11, a grey row's `line` (contract, break-even, and for an in-the-money row "intrinsic $3.50, time value $1.90" and the yield), and for Fits and Check rows the `fields` of Decision 10: it calls the evidence builder of `evidence.go` for these rows whatever their stored status, and `evidence.go` is edited only to remove a status filter from that builder if it has one, not otherwise; ITM probability and Vs cash from `Config.Figures` for these rows; the `Account cash` field has one line per alias ("A: enough, $120,000.00 free after $0.00 committed (2026-11-17)", "short", or "no figure: <reason>").
5. `tierlegend.go` (new) writes the four legend items of Decision 11 with the values of `Result.Params` (the delta bound with 试用值，可在设置里改 or 默认值, the thin line 0.15, the cash age, counts), and the cash clause by `Result.CashBuilt`; `tierstrings.go` (new) holds every Chinese text of Decision 11 word for word and the English labels; every hint at most 200 characters and ends with its data day; every file under 600 lines.
6. `tiers_test.go` (new) has `TestTiersFromTierer` (new: a fake `Tierer` returns tiers, places and figures the page cannot produce and the cells show them unchanged), `TestTiersTierOrderAndCollapsed` (new), `TestEveryRowShownOnce` (new: the rows of all tiers equal the rows read, none missing), `TestNilTiersIsNull` (new), `TestCommonMarkSaidOnce` (new: a common mark is in `common` and in no row badge), `TestGreyRowLine` (new: an in-the-money row shows intrinsic and time value), `TestTiersValidatesSchema` (new: the golden `tiers_ok.golden.json` validates against `page.option_candidates_page.v1.json`) and `TestTiersGapsGolden` (new, golden `tiers_gaps.golden.json`: BBB with no buy price, a row with no IV, no cash figure).
7. `tierfacts_test.go` (new) has `TestDeltaBoundFromOwner` (new: a fake `ownerset.Reader` with `put_delta_max` "0.25" gives `Input.Params.DeltaMax` "0.25"), `TestDeltaBoundDefault` (new: nil `Owner` gives "0.3" and the legend says 默认值), `TestAccountInputIsText` (new: positions and cash pass as stored text and a short put of another currency is passed, not dropped), `TestAccountLookback3Days` (new), `TestReaderErrorReturned` (new) and `TestNoAccountReaderGivesNoAccounts` (new: the answer has `CashBuilt` false and the legend names cash as a mark); `tierrows_test.go` has `TestFieldsHaveEvidenceForBlockedAmber` (new: an amber row stored as blocked by the quote gate has the evidence fields), `TestExpansionNoDeltaOutsideBand` (new), `TestBadgeGlossIsChinese` (new: each gloss contains a Chinese character and each label none) and `TestHintsEndWithDataDay` (new); `tierlegend_test.go` has `TestLegendPrintsCurrentValues` (new) and `TestLegendCashClause` (new).
8. `purity_test.go` has `TestNoArithmeticInTiers` (new: the non-test `tier*.go` files import none of `github.com/shopspring/decimal`, `math`, `math/big`, `sort`, and call neither `percentText` nor `roundDigits`).
9. `docs/contracts/pages.md` gains one paragraph (new) for `short_put_tiers`: the tiers, the readers added to `Config`, the setting `put_delta_max`, the widened evidence call and the gap behaviour.
10. The result matches docs/design/candidates-tiers.md#Decisions.

### 7 tiers-ot

Module: ot. Size: M.
Order: after 1, 2 and 5, and after the evidence entry "candidates-evidence-ot" (it extends its `candidateReaders` and `newOptionCandidatesPage`).
Files: `cmd/ot/tiers_site.go`, `cmd/ot/tiers_site_test.go`, `cmd/ot/evidence_site.go`, `cmd/ot/candidates_site.go`, `cmd/ot/candidates_site_test.go`, `cmd/ot/evidence_site_test.go`, `docs/contracts/ot/pages/option_candidates_page.md`
Contracts: unchanged
Design: docs/design/candidates-tiers.md#Decisions
Plan: P2
Acceptance:
1. `tiers_site.go` (new) declares `newTierer() optiontierapi.Tierer` returning `optiontier.New(newEvaluator())` (the return type is the receiver's api type), and `newEvaluator() optionevidenceapi.Evaluator` only if the evidence entry did not already declare an equivalent, in which case that one is called; `wire.go` and `site.go` are not in `git diff --name-only BASE` (`newTierer` is a new name) and the evidence step's lines in `wire.go` are not edited.
2. `candidateReaders` (declared by the evidence ot entry in `evidence_site.go`) gains `Account store.AccountReader`, `Aliases []string` and `Owner ownerset.Reader`; `buildCandidates` fills them from `rd.account`, `rd.aliasList()` and `rd.owner`, passes `deps.newRuleParams()` as `RuleParams` and `newTierer()` (new) as `Tiers` into `option_candidates_page.Config`; `candidatesPageNamed` passes none of them.
3. `tiers_site_test.go` (new) has `TestTiererIsRealRule` (new: `newTierer()` is not nil and judges one AAA row green when given the real `optionevidence.New` evaluator), `TestCandidatesWiresTiers` (new: the fake constructor receives non-nil `Tiers`, `Account`, `Owner` and `RuleParams` from the site readers and a build through the route returns the page), `TestCandidatesNamedNoTiers` (new: the named path passes nil for the five fields and opens no store beyond those it opens today) and `TestDemoProfileBuildsNoTiers` (new); the existing fakes and calls of `candidates_site_test.go` and `evidence_site_test.go` are edited only as far as the new fields need.
4. `docs/contracts/ot/pages/option_candidates_page.md` names the readers the route passes for the tier view, the named path that passes none, and the new file `cmd/ot/tiers_site.go`.
5. The result matches docs/design/candidates-tiers.md#Decisions.

### 8 tiers-web

Module: web. Size: L.
Order: after 4.
Files: `web/src/pages/candidates/TierView.tsx`, `web/src/pages/candidates/TierRows.tsx`, `web/src/pages/candidates/CompanyStrip.tsx`, `web/src/pages/candidates/TierLegend.tsx`, `web/src/pages/candidates/OptionCandidatesPage.tsx`, `web/test/pages/optioncandidates/TierView.test.tsx`, `web/test/pages/optioncandidates/fixtures/tiers.ts`, `web/test/pages/optioncandidates/fixtures/tiers-null.ts`, `web/test/pages/optioncandidates/OptionCandidatesPage.test.tsx`
Contracts: unchanged
Design: docs/design/candidates-tiers.md#Decisions
Plan: P2
按 frontend-stack：用 shadcn Table 做 档位表（七列：箭头、档位、合约、安全边际、距现价、权利金与收益、徽标；徽标列加宽）。
按 frontend-stack：用 shadcn Accordion 做 每行的展开（IV、delta、证据字段）和灰色折叠组。
按 frontend-stack：用 shadcn Badge 做 档位字与徽标，用 shadcn Tooltip 做 列头与术语的白话注释，用 shadcn Alert 做 全行共有的标记行，用 Card 做 公司条与图例，用 Lucide 图标（Check、TriangleAlert、Minus、ChevronRight）做 档位图标；本页没有图，不用 Recharts；不新增 @tanstack/react-table 用法。
Acceptance:
1. `TierView.tsx` (new) renders `payload.short_put_tiers` when it is not null: `CompanyStrip` per company, the `common` marks as one Alert each, `TierLegend`, then the tiers in payload order; `OptionCandidatesPage.tsx` shows `TierView` before the tabs and puts the tabs under the heading "Stored tables"; with `short_put_tiers` null or absent the page renders exactly as today.
2. The components render every text from the payload (`word`, `title`, `explain`, `main`, `sub`, `label`, `gloss`, `hint`, `value`, `gap.text`, `line`) and contain no arithmetic on a figure, no `toFixed`, no `Intl.NumberFormat`, no sort and no filter of rows: `git grep -n "toFixed\|NumberFormat\|\.sort(\|\.reduce(\|\.filter(" -- web/src/pages/candidates/Tier*.tsx web/src/pages/candidates/CompanyStrip.tsx` finds no line; each new file is under 600 lines.
3. The tier word is shown with its icon chosen by `icon` (`check`, `alert`, `minus`) and the row's left border colour only adds to them; a tier with `collapsed` true renders as a closed accordion whose heading is `title`; a `Pair` with `gap` renders the gap text and no empty value; the badge column has the new class `min-w-64` (16rem) and its badges wrap.
4. A row's expansion lists `fields` in order, each label with its hint in a tooltip beside it; a field with `gap` shows the gap text; a field with badges shows them; a grey row renders `line` and no expansion.
5. `git diff --name-only BASE -- package.json web/package.json pnpm-lock.yaml web/dependency-allowlist.json frontend-whitelist.json` prints nothing, and `web/src/generated` is not in `git diff --name-only BASE`.
6. The result matches docs/design/candidates-tiers.md#Decisions.

## Checked by CI

- `go build ./...`, `go vet ./...`, `go test ./internal/contracts ./internal/compute/... ./internal/pages/option_candidates_page ./cmd/ot` with `-race`; every test of the new files runs in parallel (`t.Parallel()`), uses `t.TempDir()` for any file, and none sleeps or reads the clock (the page time comes from the arguments); the tests named in the entries pass.
- `go test ./internal/compute/ruleparam ./internal/compute/settingsapi`, the contract-docs check of `docs/contracts/compute/` and `docs/contracts/ot/`, go-arch-lint (the import edges of Modules and the injection of `newTierer()` into the page `Config`), the size check against `docs/design/ratchet.json`.
- `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest`, the web build; the frontend dependency check.
- The `test` check (reads `Contracts:` of the pull request body: entry 4 `Contracts: page.option_candidates_page.v1 changed (additive optional field short_put_tiers)`, the others `Contracts: unchanged`).
- `crew issue check` of each entry against the head of the design pull request.
- Entry 8, run by `vitest`: `TierView.test.tsx` (new) has `TestTierViewRendersTiers` (new: fixture `tiers.ts`, three tiers, heading counts), `TestTierWordAndIconPerTier` (new: each tier has its word and an icon with an accessible name), `TestGreyGroupsCollapsedWithCount` (new), `TestRowExpansionFields` (new), `TestCommonMarkAlertOnce` (new), `TestTierViewShowsTextUnchanged` (new: a fixture amount `−$1,234.50` appears exactly as given), `TestBadgeColumnWide` (new: the badge header has the class `min-w-64`) and `TestTierViewNullIsAbsent` (new: fixture `tiers-null.ts`); `OptionCandidatesPage.test.tsx` gains `TestStoredTablesBelowTiers` (new) and its existing tests pass without edit.
- After the merge, by the window and not an issue: a screenshot of the rendered page against the mockup, with every difference explained.

## Adversarial

Adversarial file: `state/optdesk-drafts/designs/candidates-tiers-adversarial.md`. The six points, answered:

1. Gap against the goal: adopted in part. Stored gates keep only the first reason, so a row shows one badge per gate, not every problem; Decision 7 now says so, and Left out names the job-time quote thresholds and the stored strike window (at most the strikes and DTE the job stores). Not adopted: more settable conditions; the owner gave one setting (delta), the rest are his stated rules or trial constants (Decision 16).
2. Biggest unnamed risk (a bid of 0 becoming a judged row): adopted. A bid that is empty or not above 0 is grey `no_bid` (Decision 3, entry 2 item 3). The sign and currency of `store.Position` are taken as stored and an unknown currency makes the alias unknown (Decision 5).
3. A simpler way (keep stored gates, add only the delta test, drop cash): not adopted. The owner's tiers need numbers the gates do not give (in the money, break-even against buy price for rows with no buy price) and the cash rule needs committed strikes; cash is already a mark when not built, and the account readers are optional.
4. Lock-in: named, not removed. The tier vocabulary and schema are additive beside the stored tables, so they can be dropped without touching the job; the key `put_delta_max` is one number by design; the removal of `delta_outside_band` in the expansion is Decision 1(d).
5. Most likely false claim: adopted. "2 x aliases reads" is replaced by the day probe (Decision 15); the report meter takes obligations from the replayed ledger, which Decision 5 now says; the comment edits of entry 3 are allowed by its item 1.
6. Slip causes: adopted. The tier ids and icons are an enum in Interface entry 4; an unparsable gate or mark text makes the row `bad_input` instead of failing or dropping it (entry 6 item 2); the evidence entries' names stay unconfirmed until they merge, which each Order line says. Percent texts: entry 6 rounds compute's four-decimal texts once with `fixedText` and the purity test forbids other rounding.

## Left out

- Plan step 10, the valuation buy line beside the hand-typed buy price: a later design; the company strip shows the hand-typed buy price only, and the tiers use it.
- Concentration above a share he gives (the owner's amber badge list): no share setting exists and the plan (集中度) says only to show shares, with no cap; a company's share of cash secured is not built here. Asked of the owner: the share number, as a second rule parameter.
- The confirmed-or-not flag of an earnings date: the stored row does not keep it (`input.go`), so `earnings_before_expiry` is amber for any date inside the life and its gloss says so; a job change that stores the flag is a later design.
- The stored `rank` and `rankRun` (`convert.go:185-224`): unchanged; the page orders itself. The `/options` overview cards still take `RankInUnderlying`.
- The shortlist of three delta rows per expiry of the plan: not built; the owner's tiers replace it, and the order inside a tier is by margin.
- The plan's yield-above-cash gate (`cash_rate`) as a condition: not a tier condition by the owner's rules; it stays the stored "Vs cash" column.
- The `ot page` named path with the tier readers: it shows `short_put_tiers` null.
- A web layout change of the stored tables (they keep the tabs).
- Per-company concentration stress lines and sector shares: no data (plan, 不做的事).
- Covered calls and held call guards: unchanged.
- Only stored rows are judged: the job stores a window of strikes and DTE 21 to 45, with the quote thresholds of the job's time; the page does not say more than that.
- A settings reader for `MaxCashAgeDays` and the thin-premium line: constants in `optiontier.DefaultParams`.
