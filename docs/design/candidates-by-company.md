# candidates by company: the page options/candidates shows each company as one block, its verdict and then its best few contracts inside the owner's filter by annualized yield, with every other contract folded under it by reason

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-19

Source: three owner words that the live page does not follow, and the decisions it must keep.

- optdesk-5015, 2026-11-18T14:44Z: "但是我觉得你需要将相同的股票进行group by，并且按照年化再各个股票进行排序，这样比较好，过滤一定要做" (group the same stock; within each stock sort by annualized yield; filtering must be done).
- optdesk-5300, 2026-11-19: "这个按照公司groupby，这个我之前不是说过吗，为什么实现的好像不一样" (he sees it is not grouped).
- optdesk-5299, 2026-11-19: a company whose price is far above its buy price shows "未判定"; he asks why. The state must be stated plainly.
- Owner 2026-11-18 (`docs/design/buy-price-source.md`): the gate's buy price is the valuation buy-zone top, else the hand price, else the company is not judged.
- Owner 2026-11-19: the Chinese-first look of the approved mockup (fonts, Chinese hints) stays; the web computes nothing; Chinese texts come from the Go file `zh_texts.go`.

Read on origin/main (cbaec93d, after `candidates-zh-headings` #3252, #3254 and #3255 merged): `AGENTS.md`, `docs/design/modules.md`, `docs/design/ratchet.json`, `docs/design/candidates-tiers.md`, `docs/design/candidates-page-v2.md`, `docs/design/candidates-zh-headings.md`, `docs/design/buy-price-source.md`, `contracts/schemas/page.option_candidates_page.v1.json`, `internal/compute/optiontier/*.go`, `internal/compute/optiontierapi/api.go`, `internal/compute/optionshortlist/*.go`, `internal/pages/option_candidates_page/{builder,tiers,tierrows,tierstrings,zh_texts,shortlist}.go`, `cmd/ot/candidates_site.go`, `web/src/pages/candidates/*.tsx`, `scripts/ratchet/main.go`.

## What you see

First live page, honestly: until the owner re-imports the buy prices, no company has a judged buy price, so the blocks that exist (companies with a valuation figure) mostly read "未设买入价·未判定" with all their contracts folded, and the companies with neither stay in the one idle line below. The filters and yields below show only for a company that has a buy price; the sentence on each block says which case it is.

The page keeps its head, the buy-price hint, the legend and the kept line. Under the heading "公司" there is one block per company that has a buy price or a valuation figure. A block with a buy price is:

    公司甲   [有可卖]   买入价 $90.00（估值）  现价 $96.50  下次财报 12-03     最高年化 87.5%
    估值买入线 ...（the valuation line, unchanged）
    筛选范围内，按年化从高到低，最多 3 个：适合 2 个，待查 1 个；其余在下面折叠。
      ✓ 适合  $90 12-18 31d  ...  权利金 $94 / 年化 87.5%  ...
      ✓ 适合  $85 12-18 31d  ...  权利金 $70 / 年化 60.1%  ...
      ! 待查  $88 12-18 31d  ...  权利金 $80 / 年化 49.1%  ...
    ▸ 未显示 69：价内 16 · 保本点高于买入价 40 · 价外不足 5% 8 · 超出每家 3 个 5      (closed; opens to the rows)

    公司乙   [价格高于买入价·暂无可卖]   买入价 $40.00  现价 $130.00
    现价 $130.00 高于买入价 $40.00：有报价的价外合约，保本点（行权价减 bid）都高于买入价，所以暂无可卖；价内和没有 bid 的合约在下方折叠。这不是没判定，是判定过了。
    ▸ 未显示 54：价内 9 · 保本点高于买入价 45

The filter (owner word 5015, "过滤一定要做"): a block shows the 适合 and 待查 rows that lie inside the band, at most 3 per company, by annualized yield. Band: strike 5 to 15 percent below spot, absolute delta 0.15 to 0.30, open interest present and not low (bid present is already a tier gate). Everything else of the company, the rows the gate blocked and the rows outside the band or beyond the cap, is folded under the company with one count per reason beside the tier reasons; the shown rows plus all the counts are all the company's rows. Which numbers are the owner's and which are trial:

- Owner's (his goal of 2026-11-18, recorded in `candidates-page-v2.md` Source and Decisions 7 and 8): strike 5 to 15 percent below spot; absolute delta 0.15 to 0.30; open interest present and not low. The upper delta bound is his setting `put_delta_max` (default 0.3, which the tier rule already reads); 0.15 and the two OTM numbers are fixed values of the rule, changed by a design, not a setting.
- Trial, not decided by him: at most 3 rows per company (`zhHintCap` calls it a supplement to the approved design). It is one parameter (`PerCompany`).

Order of the blocks: companies with shown rows first, by the best annualized yield of their shown rows, highest first; then the companies with nothing shown: "筛选范围内暂无可卖", "价格高于买入价·暂无可卖", "暂无可卖", "无报价", "未设买入价·未判定". Inside a block the contracts are sorted by annualized yield, highest first; each shown row keeps its tier as a small badge (适合 or 待查), not a band. The word "未判定" is drawn only for a company with no buy price, the one case where nothing was judged. The page no longer shows one table of all 待查 contracts of all companies, and no "合约分档" heading.

Yield basis, said once in the legend: annualized yield is the time value on the bid, annualised over the row's own days to expiry, on the cash secured (the strike). Rows of different expiries are therefore comparable. The yen companies are the Japanese tab's own blocks built by the same builder; they are ranked among yen companies only and are never ranked against dollar companies, and the legend says so.

## Why the page is not grouped today (found on origin/main)

1. `docs/design/candidates-page-v2.md` (Decisions 1 to 3; owner approved on 5008 and 5015) built a grouped short list: `short_put_list`, `internal/compute/optionshortlist`, `shortlist.go`, `ShortList.tsx`, wired in `cmd/ot/candidates_site.go` (`Shortlist: newShortlister()`). The deployed head has that line, so the list is not a missing deploy.
2. `docs/design/candidates-tiers.md` (written from the 2026-11-17 mockup, Decisions 1(a), 10, 13) replaced the first view with company cards and tier bands: the cards are one column of companies, then ONE table per tier whose rows mix all companies, ordered by margin (`optiontier/order.go sortTier`). It does not cite 5015. Nobody reconciled the two, so the grouping that 5015 asked for was never in the first view the owner opens.
3. `short_put_list` is absent from the live payload because of `fillShortPut` in `internal/pages/option_candidates_page/builder.go`: `Build` calls it for every group that has rows, and for a group that is not a short put it sets `vm.ShortPutList = nil`, so a later covered-call or held-call group with rows wipes the list the short-put group just wrote. #3175 kept that on purpose. The yen list is written by its own spec and survives. This is a real bug; it is fixed by the shortputlist bug issue (drafted separately), ahead of this design, and is not fixed here.
4. A company with every row blocked is "未判定" because `companyTierOf` (`tiers.go`) knows three counts and gives grey, whose word is "未判定", to any company with 0 fit and 0 check. Its rows were judged: `greyReason` (`optiontier/grey.go`) returns `in_the_money` when the strike is above spot and `above_buy_price` when strike minus bid is above the buy price. Both are verdicts, not "not judged". The text "折叠 = 未判定" (`zhStripGreyHint`, `zh_texts.go`) says the same wrong thing under every card.

## Modules

`docs/design/modules.md` is not changed. By the first row whose glob matches:

- compute: `internal/compute/optiontierapi/api.go`, `internal/compute/optiontierapi/api_test.go`, `docs/contracts/compute/optiontier.md` (entry 1); `internal/compute/optiontier/tier.go`, `internal/compute/optiontier/order.go`, new `internal/compute/optiontier/blocks.go`, new `internal/compute/optiontier/blocks_test.go` (entry 3).
- contracts: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md` (entry 2).
- pages: `internal/pages/option_candidates_page/{tiers,tierrows,zh_texts}.go`, new `bycompany.go`, new `bycompany_test.go`, new `testdata/by_company.golden.json` (entry 4).
- web: `web/src/pages/candidates/{TierView,CompanyStrip,TierRows,OptionCandidatesPage}.tsx`, new `CompanyBlock.tsx`, tests and fixtures under `web/test/pages/optioncandidates/` (entry 5).
- ot: no file. `candidateReaders` already wires the tier judge; `ot page` builds no tier view (`short_put_tiers` null), so it builds no `by_company` either. `check_blocks.go` lists top-level keys only and `by_company` is inside the tier view.
- docs (this pull request, no issue): one line at the head of `docs/design/candidates-tiers.md` and of `docs/design/candidates-page-v2.md` saying which decisions this design supersedes (Decision 10).

The band is computed in compute (`optiontier`, which already holds every input: strike, spot, delta and the quote reason of each row), not in pages or web. `optionshortlist` keeps its own constants for the list that stays in the payload; this design does not import it (compute sub-packages do not depend on each other here) and does not change it. Its numbers and the new `Params` defaults are the same numbers until the list is removed (Left out); `TestBandDefaults` pins the defaults of `optiontier`.

No store, dataset, migration, job or setting change; nothing holds the auto-deploy. Size (scripts/ratchet counts `.go`, `.ts`, `.tsx` source, not JSON): every edited source file is under 800 lines today (`tiers.go` 476, `tierrows.go` 338, `zh_texts.go` 259, `tier.go` 156, `order.go` 159, `TierRows.tsx` 227, `CompanyStrip.tsx` 92, `TierView.tsx` 71, `OptionCandidatesPage.tsx` 145) and none is in `ratchet.json`; every new file is planned under 300 lines. `ratchet.json` is not touched.

## Interface

Owner of the shape: compute for the Go result (entry 1), contracts for the page (entry 2). All additions are add-only: no existing field changes value, no `required` list of an existing object changes, `schema_version` stays 1.

`internal/compute/optiontierapi/api.go` (declarations only; decimal text; no float, clock or I/O):

    // Params += the band of the shown rows (empty or unreadable gets the default in optiontier.fillParams)
    //   BandOTMMinPct, BandOTMMaxPct, BandDeltaMin string  // "5", "15", "0.15"; the upper delta bound is DeltaMax
    //   PerCompany int                                       // 3, trial
    // Company states of a block, one per company (total rule: Decision 3).
    const (
        StateSellable   = "sellable"      // a fit or check row is inside the band and shown
        StateOutBand    = "out_band"      // some row fits or is to check, none is inside the band
        StateAboveBuy   = "above_buy"     // all rows grey; every priced out-of-the-money row has a break-even above the buy price
        StateNoBuyPrice = "no_buy_price"  // all rows grey, one has no buy price
        StateNoQuote    = "no_quote"      // all rows grey, one has no spot or no bid (and the above_buy rule does not hold)
        StateBlocked    = "blocked"       // any other all-grey company
    )
    // Reasons a row is folded: the grey codes of the tier rule, then the band codes below, one reason per row.
    const (
        FoldNoOpenInterest = "no_open_interest"
        FoldOTMLow         = "otm_low"
        FoldOTMHigh        = "otm_high"
        FoldNoDelta        = "no_delta"
        FoldDeltaOut       = "delta_out"
        FoldOverCap        = "over_cap"
    )
    type GreyCount struct { Code string; N int }     // a fold reason and its rows
    type CompanyBlock struct {
        Company string       // as CompanyCount.Company
        State   string       // one of the States above
        Best    string       // YieldPct of Shown[0]; "" when Shown is empty or has no yield
        NFit    int          // shown rows whose tier is fits
        NCheck  int          // shown rows whose tier is check
        NOpen   int          // fit or check rows not shown (outside the band or over the cap)
        Shown   []string     // keys of the in-band fit and check rows, at most PerCompany, by YieldPct descending, empty last, then DTE, strike, key ascending
        Folded  []string     // keys of every other row of the company: grey rows first, then band-folded rows, each by the same rule
        Why     []GreyCount  // fold reasons with N > 0: grey codes in the order of precedence of Decision 3 of candidates-tiers.md, then the band codes in the order above; sum of N = len(Folded)
    }
    Result += Blocks []CompanyBlock   // companies in the order of Decision 2; never nil

`contracts/schemas/page.option_candidates_page.v1.json`, v1 gains optional properties (every new object has `additionalProperties: false`, texts only, no JSON number):

    TierView += by_company?: ByCompany            // optional, not nullable
    TierRow  += tier?: { id: fits|check, word, glyph: Text }   // same shape as Company.tier; set only on shown rows
    ByCompany { heading_note: Text, legend: [LegendItem], columns: [TierColumn], blocks: [CompanyBlock] }
    CompanyBlock { company: Text, verdict: Verdict, verdict_text: Text, best_yield: Text|null,
                   row_ids: [Text], folded: Folded|null }
    Verdict { id: sellable|out_band|above_buy|no_buy_price|no_quote|blocked, word: Text, glyph: Text, tone: success|warn|muted }
    Folded  { title: Text, summary: Text, reasons: [{ code, label, count: Text }], row_ids: [Text] }

`company` is `Company.key`; `row_ids` are `TierRow.id` values (`tierrows.go` sets `ID: v.Key`) of rows that stay in `tiers[].rows` (rows are not copied: the stored rows number in the thousands, so copying them would multiply the payload; the blocks carry ids only). Order inside `blocks`, `row_ids` and `folded.row_ids` is the order to draw; the web does not sort. `Text` is `#/$defs/Text` (1 to 200 characters), so every sentence below is at most 200 characters.

Texts (all in `zh_texts.go` as string constants; each is a gloss next to the word it explains):

    verdict.word    有可卖 | 筛选范围内暂无可卖 | 价格高于买入价·暂无可卖 | 未设买入价·未判定 | 无报价 | 暂无可卖
    verdict_text    sellable   筛选范围内，按年化从高到低，最多 {cap} 个：适合 {n} 个，待查 {m} 个；其余在下面折叠。
                    out_band   有 {k} 个合约能卖，但都不在筛选范围内（价外 {otmmin}% 到 {otmmax}%，delta {dmin} 到 {dmax}，持仓量够），原因见下方折叠。
                    above_buy  现价 {spot} 高于买入价 {buy}：有报价的价外合约，保本点（行权价减 bid）都高于买入价，所以暂无可卖；价内和没有 bid 的合约在下方折叠。这不是没判定，是判定过了。
                    no_buy_price  没有买入价（估值买入区间和手填价格都没有），所以不判定；设了买入价才会判定。
                    no_quote   没有现价或没有 bid，所以没法判定。
                    blocked    所有合约都被挡住了，原因见下方折叠。
    best_yield      最高年化 {x}%
    folded.title    未显示 {n}；summary  价内 16 · 保本点高于买入价 40 · 价外不足 5% 8 · 超出每家 3 个 5   (grey labels are `greySpecs[...].keptLabel`; band labels are new constants, below)
    band labels     no_open_interest 持仓量没有或太低 | otm_low 价外不足 {otmmin}% | otm_high 价外超过 {otmmax}% | no_delta 没有 delta | delta_out delta 不在 {dmin} 到 {dmax} | over_cap 超出每家 {cap} 个
    heading_note    每家公司一块：先看结论和价格，下面是这家公司筛选范围内年化最高的合约；其余折叠在最后。公司按自己最高的年化排序。
    columns         the seven `tiers.columns`, with the premium column relabelled "权利金 / 年化" and hinted "每份合约的权利金；下一行是年化收益，也是每家公司内排序的依据。"
    legend          four `LegendItem`s of the existing shape plus the existing cash item: 适合, 待查, 未显示 ("已判定但不能卖（价内，或保本点高于买入价，或没有 bid），或不在筛选范围内，或超出每家 {cap} 个；点开每个公司下的折叠看原因。"), 年化 ("年化 = bid 里的时间价值，按这一行的到期天数折成一年，除以占用的现金（行权价）；公司按自己最高的年化排序；日元公司在日本标签页，不和美元公司排在一起。"), then the cash item

`{spot}`, `{buy}`, `{n}`, `{m}`, `{k}`, `{x}`, `{cap}`, `{otmmin}`, `{otmmax}`, `{dmin}`, `{dmax}` are filled with the page's existing text helpers (`moneyText`, `fixedText`) from the strip, the block counts (`NFit`, `NCheck`, `NOpen`) and the echoed `Result.Params`; the page adds, subtracts and compares nothing.

## Decisions

1. One layout. The first view is the company blocks; the flat tier bands, the short list and the strip's count line are not drawn when `by_company` is present. A payload without `by_company` (an older stored page, a build without a tier judge, `ot page`) renders as today.
2. Order, all decided in compute (`optiontier`, new `blocks.go`) because `modules.md` forbids pages and web to compute and `candidates-tiers.md` Decision 8 makes `optiontier` the one place of order and counts. Shown rows by `YieldPct` descending (empty last), ties by DTE, strike, key ascending; folded rows by the same rule, grey first. Blocks: first the companies with shown rows by `Best` descending (companies whose shown rows have no yield after them, by company text), then the rest by state rank `out_band`, `above_buy`, `blocked`, `no_quote`, `no_buy_price`, then company text. The existing `Order`, `Place` and `Companies` are unchanged.
3. The company state is a total rule over its rows, first match wins: a fits or check row inside the band and shown gives `sellable`; else any fits or check row gives `out_band`; else (all rows grey) any `no_buy_price` row gives `no_buy_price`; else `above_buy` when at least one row is `above_buy_price` and no row is `no_spot` or `bad_input`; else any `no_spot` or `no_bid` row gives `no_quote`; else `blocked`. Why the `above_buy` sentence is true for every such company: a grey row that has a buy price, a spot, a strike not above spot and a bid can only be `above_buy_price` or `bad_input` (`greyReason` order: no_buy_price, no_spot, in_the_money, no_bid, above_buy_price, bad_input), so with no `no_spot` and no `bad_input` row every other row is in the money or has no bid, and those are not priced out-of-the-money contracts; and an `above_buy_price` row has strike not above spot and strike minus bid above the buy price, so spot exceeds the buy price. A mixed company (a no-bid out-of-the-money row whose strike is below the buy price, in-the-money rows, above-buy rows) is `above_buy` and its sentence says the folded rest are in the money or without bid. The company's old `tier` (fits, check, grey with the word "未判定") is not changed and no longer drawn; Decision 6 says where "未判定" is left.
4. Sort figure: the row's `YieldPct` (time-value yield per year), the figure the premium cell already prints under the premium. A shown row is out of the money, so the intrinsic value is 0 and `YieldPct` is the bid over the strike times 365 over the days, the stored `annualized_yield` that `candidates-page-v2.md` Decision 3 named (`optiontier/figures.go writeTimeValue`). Rows of different expiries compare on this figure because it is already per year; the legend says so. The figure is made visible by the relabelled column "权利金 / 年化", not by a new column.
5. Filter (5015). The band, computed in `optiontier` from the row texts on exact decimals: out of the money by (spot minus strike) over spot, from `BandOTMMinPct` to `BandOTMMaxPct` percent (below the minimum, in the money included, is `otm_low`; above the maximum `otm_high`); absolute delta from `BandDeltaMin` to `DeltaMax`, a row with an empty or unreadable delta is `no_delta`, outside is `delta_out`; open interest absent or low (the quote reason `low_open_interest` or the mark `no_open_interest`, the codes `optionshortlist` reads) is `no_open_interest`. These apply to the fit and check rows; a row that fails goes to the fold under the first reason in the order `no_open_interest`, `otm_low`, `otm_high`, `no_delta`, `delta_out`. Of the rows that pass, the best `PerCompany` by the order of Decision 2 are shown and the rest are `over_cap`. Grey rows are folded under their grey reason (never twice). So `len(Shown)` plus the sum of `Why` is the company's row count, tested in compute and again in pages. The delta bound is the setting `put_delta_max` through `DeltaMax`, so the band follows the owner's setting; `optionshortlist` still ignores it, which is why this design does not reuse it. The labels are the existing `keptLabel`s for grey reasons ("价内", "保本点高于买入价", "没有 bid", ...); the label for `above_buy_price` stays "保本点高于买入价".
6. "未判定" is said once and truly. The verdict `no_buy_price` is the only place the word is drawn in this layout. Old texts that say a blocked row is "not judged" stay in the payload as values of old fields and the web does not draw them when `by_company` is present: the grey tier titles and word (`zhWordGrey`, `explainGrey`), the strip count line with `zhStripGreyHint` ("折叠 = 未判定"), the legend item `legendGreyText`, `companies_note` and the "none" explanations (`noneFitsAllGrey`). `by_company.legend` and `heading_note` replace the legend and the note. Where the word remains: only in a payload without `by_company` (the fallback: older stored pages, `ot page`), through `zhStripGreyHint` and `legendGreyText`, which are not edited (editing them changes the existing goldens; they are accurate only as old-page text and the fallback is a transition). The idle line (`idleNoteTail`, `buyHintZh`) does not contain the word. Folded rows carry no row `tier` badge, so the word is not drawn on an in-the-money row; the folded line shows its reason. The subtitle ("按公司分组；只有你设了买入价的公司才会被评判") stays true.
7. The strip is the header of the block. `CompanyStrip` keeps its first line and the valuation line; in a block it draws the verdict pill (`verdict.word`, glyph, tone) in place of the old tier pill and draws `verdict_text` as a third line in place of the count line. Buy price and its source follow `buy-price-source.md` unchanged (valuation buy-zone top, else hand price, else none): the strip already prints `buy_price` and `source_badge`, and `no_buy_price` is its "没有买入价" state.
8. Rows are the existing `TierRow` objects, found by id; each shown row carries its own `tier` badge (fits or check) so the row needs no enclosing band. The expansion of a shown row and the line of a folded row are unchanged. The web looks rows up by id in `tiers[].rows` (a join, no sorting, counting or filtering) and draws the ids in the given order.
9. Fate of the short list. `short_put_list` and `short_put_list_jpy` stay in the payload and are not changed; the web draws the list only when the tier view has no `by_company`. The `fillShortPut` wipe is fixed by the shortputlist bug issue (drafted separately, ahead of this design), not here. After that fix the US list is written again and, with `by_company`, drawn by no page: the blocks replace it. Removing the list is a later design (Left out).
10. Supersedes. `candidates-tiers.md`: Decision 1(a) in part (rows are grouped by company, not by tier), Decision 10 (the strip's count line "N fit, N check, N collapsed" is replaced by the verdict and the fold line) and Decision 13 (the tier tables and grey accordions as the first view; kept only as the fallback for a payload with no `by_company`). Decisions 2 to 9, 11 and 12 stand. `candidates-page-v2.md`: Decision 1 (the list as the first view) is replaced; Decisions 3 (group by company, rows by annualized yield, groups by best yield), 7, 8 and 10 (OTM band, delta band, cap) and the open-interest filter are carried into the blocks by Decision 5 (the cap as a trial parameter) with the dropped-by-reason count kept as the fold; the separate "Not shown K" line and the extra columns (OTM %, Delta, Liquidity) are not carried; Decision 14's buy-price slot stands. `candidates-zh-headings.md`: `heads.companies` stands; `heads.companies_note` is replaced in this layout by `heading_note`; `heads.tiers` is not drawn; the subtitle is kept.
11. Chinese-first look: the blocks use the existing card, badge, table and accordion components, fonts and colours (`success`, `warn`, muted); no new colour or font; no new Chinese in `web/src`. The window compares a screenshot of the built page with the approved mockup before the owner sees it, as the standing UI rule says; that is a window action, not an acceptance item.
12. Cost: no new read. `blocks` is built from the rows `Judge` already holds, the page step from the verdicts and strips it already has.

Owner-rule self-check:

| rule | where it is met |
|---|---|
| 5015 group same stock | Decision 1; Interface `blocks`; entry 5 draws one block per company |
| 5015 sort by annualized within each stock | Decisions 2 and 4; entry 3 acceptance 2; the column and the legend say what the figure is |
| 5015 filtering must be done | Decision 5: the band (owner's numbers named, cap 3 named trial), at most 3 shown, everything else folded with a count per reason, shown plus counts equals all rows; entry 3 acceptance 4 to 6 |
| 5300 why not grouped | Why section: items 1 and 2 say how two designs diverged; this design replaces the divergent layout and cites 5015 in Source |
| 5299 plain state for "far above the buy price" | Decisions 3 and 6; verdict "价格高于买入价·暂无可卖" and its sentence, true for a mixed company; entries 3 and 4 acceptance |
| gate buy price: valuation, else hand, else not judged (`buy-price-source.md`) | Decision 7; no change to the rule; "未设买入价·未判定" is its third case |
| web computes nothing | Decisions 2, 5, 8; the band, counts and order are in compute; entry 5 acceptance forbids any sort, count or arithmetic |
| Chinese texts in `zh_texts.go`, Chinese-first look | Decision 11; entry 4 puts every text in `zh_texts.go`; entry 5 acceptance forbids new visible literals |

## Issues

Build order: entries 1 and 2 first (interfaces, parallel); entry 3 after 1; entry 4 after 1 and 2; entry 5 after 2. The shortputlist bug issue (drafted separately) is ahead of the design and is not an entry here. Entries are not merged (four modules). Each merge leaves main green: every new field is optional or empty. The page shows the new layout only when entries 3 and 4 are both on main (before entry 3 `Blocks` is empty, so `by_company` is not written).

### 1 candidates-by-company-compute-api (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/optiontierapi/api.go`, `internal/compute/optiontierapi/api_test.go`, `docs/contracts/compute/optiontier.md`
Contracts: unchanged
Design: docs/design/candidates-by-company.md#Interface
Acceptance:
1. `api.go` declares the six `State*` and the six `Fold*` string constants, `GreyCount`, `CompanyBlock` with the fields of the Interface section (`NFit`, `NCheck`, `NOpen` included), the four new `Params` fields, and `Result.Blocks`; no existing declaration changes; the file stays declarations only (types and constants, no function).
2. `api_test.go` has `TestResultBlocksContract` (new): a fake `Tierer` returns a `Result` with two `CompanyBlock` values; the test reads `Blocks` back with `Shown`, `Folded`, `Why`, `State`, `NFit`, `NCheck` and `NOpen`, and checks that the six state constants and the six fold constants are distinct non-empty strings.
3. `docs/contracts/compute/optiontier.md` has a paragraph "Company blocks" stating the state rule of Decisions 3 and 5 in the order of the rule, the band and its parameters, the sort key of Decision 4, that one row has one fold reason, and that `Blocks` is never nil.
Checked by CI: `go test ./internal/compute/optiontierapi/...`.
Left out: any change of `CompanyCount`, `Order` or `Place`.

### 2 candidates-by-company-contract (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: changed (`contracts/schemas/page.option_candidates_page.v1.json`)
Design: docs/design/candidates-by-company.md#Interface
Acceptance:
1. The schema has optional, non-null `by_company` on `TierView` and optional `tier` on `TierRow`; the new `$defs` are `ByCompany`, `CompanyBlock`, `Verdict` and `Folded`, each with `additionalProperties: false` and the properties of the Interface section, every text `$ref: #/$defs/Text`, `verdict.id` an enum of the six ids and `tone` an enum of three; row `tier.id` an enum of `fits` and `check`; `row_ids` items are `Text`; no `required` list of an existing object differs from main; nothing is removed; no new property is a JSON number.
2. `option_candidates_page.v1.ts` is regenerated in the same change; it holds the new optional members and nothing removed.
3. `option_candidates_page_v1_test.go` has `TestByCompanyValidates` (new: a tier view with one block, one folded group and a row `tier` passes), `TestByCompanyAbsentValidates` (new: the same view without `by_company` passes), `TestByCompanyNullFails` (new: `"by_company": null` fails), `TestVerdictIdEnum` (new: `"id": "grey"` in a verdict fails), `TestRowTierGreyFails` (new: `"id": "grey"` in a row `tier` fails) and `TestBlockNeedsRowIds` (new: a block without `row_ids` fails); the existing tests still pass.
4. `docs/contracts/page-schemas.md` names `by_company` and the row `tier` in one sentence under the page entry.
Checked by CI: `go test ./internal/contracts/...`; `tsc` on the regenerated type.
Left out: removing any old field.

### 3 candidates-by-company-compute

Module: compute. Size: M.
Order: after 1.
Files: `internal/compute/optiontier/tier.go`, `internal/compute/optiontier/order.go`, new `internal/compute/optiontier/blocks.go`, new `internal/compute/optiontier/blocks_test.go`
Contracts: unchanged
Design: docs/design/candidates-by-company.md#Decisions
Acceptance:
1. `blocks.go` (new file) has one function that builds `Result.Blocks` from the row states; `order.go` calls it once in `assemble` and changes no existing line of the ordering of `Order`, `Place` or `Companies`; `tier.go` changes only `DefaultParams` (trial values "5", "15", "0.15", 3 for the four new fields) and `fillParams` (empty or unreadable gets the default), and `res.Params` echoes them.
2. A company with 3 in-band fit or check rows of yields 12, 80 and no yield, and 2 grey rows, has `Shown` in the order 80, 12, no-yield and `Folded` with the two grey keys by the same rule; ties keep DTE, strike, key ascending (`TestBlockRowsByYield`, new).
3. State by the rule of Decision 3, one case per state: a shown fit gives `sellable` even with grey rows; only check rows in the band give `sellable`; fit rows all outside the band give `out_band`; all grey with a `no_buy_price` row gives `no_buy_price`; all grey with `above_buy_price` rows gives `above_buy` when the other rows are in the money or have no bid; in the money plus a no-spot row, and in the money plus no bid only, give `no_quote`; `above_buy_price` plus `bad_input` and in the money only give `blocked` (`TestBlockStateRule`, new, a table of those cases). `TestAboveBuyMixedCompany` (new): a company with an in-the-money row, a row with no bid whose strike is below the buy price, and rows whose break-even is above the buy price is `above_buy`, and the same company plus one row with no spot is `no_quote`.
4. The band by Decision 5, one case per code: a fit row 4 percent out, 16 percent out, with delta 0.10, with delta 0.35, with no delta, with the quote reason `low_open_interest` and with the mark `no_open_interest` is folded under `otm_low`, `otm_high`, `delta_out`, `delta_out`, `no_delta`, `no_open_interest`, `no_open_interest`; a row on each edge (5, 15, delta 0.15 and 0.30) is shown; a row failing two checks counts once under the first in the order (`TestBandCodes`, new). The delta upper bound follows `Params.DeltaMax` (`TestBandFollowsDeltaMax`, new). `TestBandDefaults` (new) pins the defaults "5", "15", "0.15", "0.3" and 3.
5. The cap: five in-band rows with `PerCompany` 3 show the three best by yield and fold two under `over_cap`; with `PerCompany` 5 none (`TestBlockCap`, new).
6. Counts add up: for every company in a mixed table of 12 rows (fits, check, grey of each reason, each band code), `len(Shown)` plus `len(Folded)` equals the company's row count, `Why` has each reason with N above 0 in the order of Decision 5 and the sum of `N` equals `len(Folded)`, `NFit` plus `NCheck` equals `len(Shown)`, and `NOpen` equals the fit and check rows folded (`TestBlockCountsAddUp`, new).
7. Block order: a company with `Best` 80 before one with `Best` 12 before one whose shown rows have no yield; then `out_band`, `above_buy`, `blocked`, `no_quote`, `no_buy_price` companies; ties by company text (`TestBlockOrder`, new). A company with no rows read does not appear.
8. `Blocks` is a non-nil empty slice for a `Judge` call with no rows; the existing `purity_test.go` passes for `blocks.go` (no float, no clock, no I/O); `blocks.go` and `blocks_test.go` are new files under 600 lines and `order.go` and `tier.go` stay under 800.
Checked by CI: `go test ./internal/compute/optiontier/...`.

### 4 candidates-by-company-pages

Module: pages. Size: M.
Order: after 1 and 2 (shows data only once 3 is on main).
Files: `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/tierrows.go`, `internal/pages/option_candidates_page/zh_texts.go`, new `internal/pages/option_candidates_page/bycompany.go`, new `internal/pages/option_candidates_page/bycompany_test.go`, new `internal/pages/option_candidates_page/testdata/by_company.golden.json`
Contracts: unchanged
Design: docs/design/candidates-by-company.md#Interface
Acceptance:
1. `bycompany.go` (new) holds the view types of the Interface section and one function that turns `Result.Blocks` into `by_company`; it adds, subtracts, compares and sorts nothing: it keeps the order of `Blocks`, takes the counts of each sentence from `NFit`, `NCheck`, `NOpen` and the echoed `Params`, and the money texts from the strip and the verdict figures.
2. `tiers.go` gains `ByCompany *byCompanyView` with `json:"by_company,omitempty"` on `tiersView` and one call in `buildTiers` after `fillStrips`; `tierrows.go` gains `Tier *tierBadge` with `json:"tier,omitempty"` on `tierRow`, set only for shown rows. A `Result` with no `Blocks` marshals with no `by_company` and no row `tier`, so the existing golden files of `testdata/` (tiers_ok, tiers_gaps, shortlist_ok, shortlist_gaps, evidence_ok, evidence_gaps, valueline, jp_rows) are unchanged.
3. Blocks are written only for the companies in `tiersView.Companies` (those with a buy price or a valuation figure); a company that `splitIdle` names in the idle line has no block. `row_ids` are the keys of `Shown`; `folded.row_ids` are the keys of `Folded`; each equals a `TierRow.id` in `tiers`.
4. `zh_texts.go` holds the texts of the Interface section (verdict words and sentences, band labels, heading note, legend items, relabelled column and hint) as string constants and nothing else new, with the verdict words exactly as written there; `zhStripGreyHint`, `legendGreyText` and the other old texts of Decision 6 are not edited; `TestZhTextsDoor` still passes. The folded `summary` uses `greySpecs[code].keptLabel` for grey codes, the band labels for the band codes, and the order of `Why`.
5. `bycompany_test.go` (new, with its own fake `Tierer` that returns `Blocks`; no shared fake is edited) has: `TestVerdictWordsAndTexts` (new: the six states give the six words, glyphs and tones; the `above_buy` sentence holds the strip's spot and buy texts, the clause about priced out-of-the-money contracts and the sentence "这不是没判定，是判定过了"; the `sellable` sentence holds `{cap}`, `{n}`, `{m}` filled from the block; `no_buy_price` is the only verdict whose word contains "未判定"), `TestFoldedSummaryCounts` (new: 16 in the money, 40 above the buy price, 8 `otm_low` and 5 `over_cap` give "价内 16 · 保本点高于买入价 40 · 价外不足 5% 8 · 超出每家 3 个 5" and title "未显示 69"), `TestShownPlusFoldedIsAllRows` (new: for the golden table the row ids of `Shown` and `Folded` of every block cover that company's `TierRow` ids once each, and the counts of `folded.reasons` add up to `folded.row_ids`), `TestBlocksOnlyForListedCompanies` (new), `TestRowTierBadge` (new: a shown fits row and a shown check row carry their own badge; a folded row carries none), `TestByCompanyAbsentWithoutBlocks` (new), `TestJapaneseViewHasBlocks` (new), `TestLegendYieldBasis` (new: the legend holds the yield-basis item with the sentence on days and cash secured and the sentence that yen companies are not ranked against dollar companies), `TestHeadingNote` (new: `heading_note` is present, `heads.tiers` is still in the payload, `companies_note` is unchanged), and `TestByCompanyGolden` (new, compares `testdata/by_company.golden.json`, which holds a sellable company with a fold, an above-buy company of mixed rows, an out_band company and a no-buy-price company).
6. The built page with `by_company` validates against the schema of entry 2.
7. `tiers.go` (476 lines) and `tierrows.go` (338) stay under 800 and, as `bycompany.go` and `bycompany_test.go` are new files under 600, no ceiling in `docs/design/ratchet.json` changes; `zh_texts.go` (259) stays under 800.
Checked by CI: `go test ./internal/pages/...`.

### 5 candidates-by-company-web

Module: web. Size: M.
Order: after 2.
Files: new `web/src/pages/candidates/CompanyBlock.tsx`, `web/src/pages/candidates/TierView.tsx`, `web/src/pages/candidates/CompanyStrip.tsx`, `web/src/pages/candidates/TierRows.tsx`, `web/src/pages/candidates/OptionCandidatesPage.tsx`, new `web/test/pages/optioncandidates/CompanyBlock.test.tsx`, `web/test/pages/optioncandidates/TierView.test.tsx`, `web/test/pages/optioncandidates/CompanyStrip.test.tsx`, `web/test/pages/optioncandidates/OptionCandidatesPage.test.tsx`, new `web/test/pages/optioncandidates/fixtures/bycompany.ts`, `web/test/pages/optioncandidates/fixtures/MANIFEST.synthetic-v1.json`
Contracts: unchanged
Design: docs/design/candidates-by-company.md#Decisions
Acceptance:
1. `CompanyBlock.tsx` (new) draws, for one `by_company.blocks` entry: the `CompanyStrip` of the company with the verdict pill (`verdict.word`, glyph, tone) and `verdict_text` as its third line, `best_yield` beside the pill when not null, the rows named by `row_ids` in the given order, each found by id in `tiers[].rows` and drawn with the existing row component and its own `tier` badge, then, when `folded` is not null, one closed accordion titled `folded.title` with `folded.summary` beside it and the folded rows (their `line`) inside.
2. `TierView.tsx`, when `tiers.by_company` is present, draws in this order: buy hint, common marks, kept line and note, `by_company.legend` (in place of `tiers.legend`), the "公司" heading with `by_company.heading_note` (in place of `heads.companies_note`), one `by_company.columns` head once above the blocks, the blocks in payload order, the card note, the idle line; it draws no flat company cards, no tier section, no `heads.tiers` heading and so no "合约分档" heading. When `by_company` is absent the file draws exactly what it draws today.
3. `CompanyStrip.tsx` draws the old tier pill and the count line only when it gets no verdict; `TierRows.tsx` exports the row component and takes the row's own `tier` for its border, word and icon; the grey and fits rendering of the old tier sections is unchanged.
4. `OptionCandidatesPage.tsx` draws `ShortList` for the US page and the yen tab only when the tier view has no `by_company`; the stored-tables accordion and the group tabs stay.
5. No new string literal that is visible text in any of the files; they sort, count, filter and join nothing (the row lookup by id is a join and the only data step). The files are checked by reading: no `.sort(`, `.filter(`, `.reduce(` or arithmetic on payload values is added.
6. Tests: `CompanyBlock.test.tsx` (new) has `TestBlockRowsInPayloadOrder`, `TestVerdictPillAndText`, `TestFoldedClosedWithSummary`, `TestNotJudgedWordOnlyForNoBuyPrice` (the word "未判定" is found only in a block whose verdict id is `no_buy_price`), `TestRowCarriesOwnTierBadge`; `TierView.test.tsx` has `TestByCompanyReplacesBands` (new: no `data-tier` section, no flat card list, legend from `by_company`), `TestNoTierTablesHeading` (new: with `by_company` the text "合约分档" is not found; without it the existing fixture still draws it) and `TestNoByCompanyRendersAsToday` (new: the existing fixture `tiers.ts` renders its sections); `CompanyStrip.test.tsx` has one new test that the count line is not drawn with a verdict; `OptionCandidatesPage.test.tsx` has one new test that `ShortList` is hidden for a tier view with `by_company` and shown without. The fixture `bycompany.ts` (new; the manifest lists it) has four companies: sellable with a fold, above the buy price, out_band, no buy price.
7. The existing tests in the four test files still pass unchanged.
Checked by CI: `tsc`, `eslint`, `vitest`, the build.

## Left out

- The separate "Not shown K" line and the extra columns (OTM %, Delta, Liquidity as columns) of the approved v2 mockup: the fold carries the counts per reason and the numbers are in the row expansion. Whether to add the columns is the owner's choice after he has seen the blocks.
- The cap of 3 is a trial parameter, not the owner's number; the owner may raise or drop it (`PerCompany`), one value.
- The band numbers 5, 15, 0.15 are fixed values of the rule, not settings; `put_delta_max` is the one setting (the upper bound).
- Removing `short_put_list`, `short_put_list_jpy`, `shortlist*.go`, `ShortList*.tsx`, the `optionshortlist` package and its wiring: a later design once this page is accepted; until then they are payload fields no page draws when `by_company` is present. Fixing the `fillShortPut` wipe is the shortputlist bug issue (drafted separately), not this design.
- The old company `tier` field and its word "未判定" for an all-blocked company, and `zhStripGreyHint` and `legendGreyText`: kept as values for a payload without `by_company`, not drawn in the new layout.
- Companies with neither a buy price nor a valuation figure keep the one idle line; giving each its own block with "未设买入价·未判定" is one change in `splitIdle` if the owner wants it.
- A tie between companies with equal best yield is broken by the company key, not the ticker (compute has no ticker); the page does not re-sort.
- A company order that puts all "有可卖" companies before those whose best row is a check row: the owner said best annualized yield, so a company whose best row is a check row can rank above one whose rows all fit.
- Any change of the tier rule, the buy-price rule, the grey reasons, the column set other than the premium label, theme, font or size.

## Adversarial

Inputs: `state/optdesk-drafts/designs/advisor-2026-11-19-bycompany.md` (the advisor) and part A of `state/optdesk-drafts/designs/adversarial-2026-11-19-bycompany-scheduler.md` (the review; its part B is the scheduler design and is not answered here). Every point, adopted or not:

Advisor
1. Keep the filters of 5015: adopted (Decision 5, What you see, entries 3 to 5). Correction of fact: the band numbers are constants of `internal/compute/optionshortlist` (`OTMMinPct`, `OTMMaxPct`, `DeltaMin`, `DeltaMax`, `PerUnderlying`), not parameters in the page's `shortlist.go` (which only labels the drop codes); the band is therefore computed in `optiontier` (compute, pages compute nothing) with four add-only `Params` fields rather than moved or imported. Owner's versus trial values are named; delta upper bound is the setting. Rows outside the band or over the cap are folded with one count per reason; shown plus counts equals all rows, tested (entry 3 acceptance 6, entry 4 `TestShownPlusFoldedIsAllRows`).
2. Block bound closed by the cap: adopted (`PerCompany` 3, shown at most 3).
3. `above_buy` false for mixed companies: adopted (Decision 3 defines the state so the sentence holds, sentence reworded to "有报价的价外合约"; `TestAboveBuyMixedCompany`). Not adopted as worded in the advisor: the sentence is not weakened to "有合约的保本点高于买入价"; the state is narrowed instead, with `bad_input` and no-spot rows sending the company to `blocked` or `no_quote`.
4. Rank across expiries: adopted (legend item "年化", Decision 4, company order = best shown row; yen companies stand in the Japanese tab and are not ranked against dollar companies, said in the legend and tested by `TestLegendYieldBasis`).
5. Adversarial must answer: dropped filters (point 1), mixed expiries (point 4), yen versus USD (point 4), first live page (What you see opens with it: blocks mostly read 未设买入价·未判定 until the buy prices are re-imported). Adopted. Softened wording: "mostly", not "every", because a company with a valuation figure and a hand price can already be judged; the idle line holds those with neither.
6. Entry 4 acceptance 2 "five existing golden files": adopted; it now lists the eight existing goldens of `testdata/` by name (the other four files there are inputs).
Headings: adopted (`heads.companies` stands, `companies_note` replaced by `heading_note`, `heads.tiers` not drawn, subtitle kept; entry 5 acceptance 2 and `TestNoTierTablesHeading`: no "合约分档" with `by_company`).
fillShortPut: adopted as a separate bug issue ahead of this design; referenced, not fixed here (Why item 3, Decision 9).
Owner instruction items: `NFit`/`NCheck` (and `NOpen`) added in entry 1; "未判定" only for no buy price with the remaining places stated exactly (Decision 6: fallback payload only, texts not edited because that changes the existing goldens; idle line does not contain the word); all new files marked "new" in the Files lines.

Review, part A
- Verified true: no action. Its note on `cmd/ot/candidates_site.go:49` is moot, the line number is dropped from the text.
- Files check: new files marked "new" in every Files line (adopted); module assignment unchanged. `tier.go` is added to entry 3 for `DefaultParams` and `fillParams`.
- Gap 1 ("过滤一定要做"): adopted by keeping the band, the cap and the fold (it chose "keep cap-3 / delta band as Shown plus a fold"); no question to the owner first, as decided.
- Gap 2 (sort by `YieldPct`): no change, as it says; legend and column state the figure.
- Gap 3 (`above_buy` sentence): adopted, see advisor 3.
- Biggest unnamed risk (counts in `verdict_text`): adopted, `NFit`, `NCheck`, `NOpen` in entry 1.
- Simpler way (order and ids only, drop the second copy of legend, columns and heading note): not adopted. The legend and heading note must change text (yield basis, filter, no "未判定" under every card), and a payload can hold a layout only if its texts travel with it; the cost is one object. Rows are already not copied (ids only).
- Lock-in (two layouts side by side, dead `short_put_list` fields): accepted and named (Decisions 9 and 10, Left out); schema is add-only.
- Claim most likely false (row ids): checked, `tierrows.go` sets `ID: v.Key`, so a key is a row id; entry 4 acceptance 3 and `TestShownPlusFoldedIsAllRows` test it. Its second claim (the sentence generalises): fixed by Decision 3.
- Likeliest slip (idle line still says "未判定"): checked; `idleNoteTail` and `buyHintZh` do not contain the word, `zhStripGreyHint` and `legendGreyText` do and remain in the fallback only; stated in Decision 6 and Left out; blocks exist only for listed companies, so a company with neither is in the idle line, said in What you see.
- Short list C (candidates): all five points done as above.
