# spread legs: one page shows each spread the account holds as one strategy with its legs, net credit, max loss and breakevens

Author: the optdesk designer (draft 2026-11-21)
Owner ground: owner request "价差腿" (option positions that form a spread are shown grouped as one strategy, not as unrelated rows); plan P2
Date: 2026-11-21

Source: the main of 2026-11-21. Open option contracts are listed one row each on `/positions` (`positions_page`, `internal/pages/positions_page/rows.go`: one `optionRow` per stored position, so a short put and a long put of one spread are two rows with no link). The matcher already exists: `optionstruct` (`internal/compute/optionstruct`, a port of Lean's OptionStrategyMatcher, owner decision 2026-11-20) turns the replayed ledger positions into `compute.Structure` values with `Kind` (`put_spread`, `call_spread`, `put_debit_spread`, `call_debit_spread`, `calendar_spread`, `diagonal_spread`, straddles, strangles, `iron_condor`, `collar`, `covered_call`, singles and `other`), `Legs` (`StructureLeg`: right, strike, expiry, signed quantity, multiplier, `OpenPrice`) and the figures `MaxLoss`, `MaxGain`, `BreakEven` (`compute.StructureFigures`). `structures_page` (`internal/pages/structures_page`) shows them at `/options/structures` as one row per structure (kind, expiry, max loss, max profit, floating P/L) and one detail page per structure with a legs table, tiles and a payoff chart. Rules carried: the front end renders and computes nothing; Go computes; only stored data is read; Chinese text of a package is only in its `zh_texts.go` and each term is glossed next to it; no ticker and no account id in this file; a missing figure is a gap with its reason, never a zero; `internal/compute/api.go` is at its ratchet ceiling (840 lines) and is not edited.

## Today

Stored, so nothing new is read: the ledger trades (Flex rows) and the replayed positions with `OpenPrice` per leg, the cycles (for covered calls), the session days. `optionstruct.Evaluate` needs none of the quotes for max loss, max gain and break-even (they come from the leg terms).

Owner-visible today: on `/positions` the options table lists every contract on its own row (code, role, quantity, cost, price, day, unrealised, open premium); nothing says which rows belong together. On `/options/structures` the spreads are already one row each, but the row has no legs, no net credit and no break-even; the legs and the single break-even are one click away on the detail page, and a structure with two break-evens (an iron condor) shows "no single breakeven price". The net credit of a spread is shown nowhere.

What is missing is only: a view that puts, for each spread, the legs, the net credit (new figure), max loss, max gain and all break-evens (new figure) in one place.

New page `/options/spread-legs` (menu Options, label "价差腿"). Example, all names and numbers are made up:

    价差腿 Spread legs             data day 2026-11-20
    把同一标的、同一到期日的期权腿合成一个策略：看净收入、最大亏损、盈亏平衡价。
    共 3 个组合，另有 2 个单腿持仓不在此页（见"持仓结构"页）。

    AAA  认沽价差 Put spread（卖出价格较高的认沽、买入价格较低的认沽）   到期 2026-12-18（27 天）
        腿 Leg                      数量 Qty   行权价 Strike   开仓价 Open price
        卖出认沽 Short put              -2          95.00          2.40
        买入认沽 Long put               +2          90.00          1.10
        净收入 Net credit +260.00 美元（正数为收入，负数为支出）   最大亏损 Max loss -740.00   最大盈利 Max gain +260.00
        盈亏平衡价 Break-even 93.70（到期时股价高于它则不亏）
    BBB  铁鹰 Iron condor（一个认沽价差加一个认购价差）                   到期 2026-12-18（27 天）
        4 legs ...
        净收入 +310.00 美元   最大亏损 -690.00   最大盈利 +310.00
        盈亏平衡价 Break-even 86.90 和 113.10
    CCC  备兑认购 Covered call（持有正股、卖出认购）                      到期 2026-12-18（27 天）
        卖出认购 Short call             -1         120.00          1.80
        净收入 +180.00 美元   最大亏损 无上限规则（含股票，此页不算）   最大盈利 +1,420.00   盈亏平衡价 此页不算（含股票）
    未识别组合 Unidentified: one block per unit the matcher could not name, legs only, no figure, with the reason.
    注：净收入 = 各腿（-数量 x 开仓价 x 合约乘数）之和，只含开仓价，不含手续费和平仓。

Each figure cell shows its reason in words when it cannot be given; no cell shows 0 for a missing number.

## Decisions

1. A new page, not a change of `/positions` and not of the structures list. The `positions_page` schema is v4 with a 1,166-line file and a 1,138-line fixture; a v5 would copy files far over 600 lines and break the flat contract tests, and the options table there is a contract list (price, day change) that stays as it is. The structures list is sorted by max loss and day to expiry, a different job; its schema has no legs. The page sits beside them and links each group to the structure detail page of the same structure (the href function of the list, reused).
2. One new package of compute, `internal/compute/spreadlegs` with its declarations in `internal/compute/spreadlegsapi/api.go`. `internal/compute/api.go` is at its ceiling (840, `docs/design/ratchet.json`) and never raised, so `StructureFigures` gets no new field. The figure takes the `compute.Structure` values `Evaluate` already gave and returns, per structure id, the net credit and the break-evens. It does the day count and the display order, so the page counts and sorts nothing. It does not call `optionstruct` and does not repeat max loss and max gain: those two are copied from `Structure.Figures` by the page, so there is one rule for them.
3. Net credit = sum over the legs of (-Quantity x OpenPrice x Multiplier), exact decimals (`math/big`), printed with 2 places half-even. Positive is a credit, negative a debit (a debit spread shows a negative number and the Chinese gloss says so). It uses `StructureLeg.OpenPrice` of the replayed ledger, so it is the price paid at the open, not the premium left after a partial close and not net of commission. A leg with an empty or non-numeric `OpenPrice` makes the net credit the gap `missing_input`; a structure whose currency is not USD gives `unsupported_currency` as the matcher does for its own figures. The sign of `Quantity` is the stored one (short is negative).  A leg that is only a part of a position (the matcher splits one position over two units) uses the quantity of the leg, never the position's.
4. Break-evens = every price at which the expiry payoff of the legs is zero, exact, ascending, found from the leg terms (right, strike, signed quantity, multiplier, open price): the payoff is piecewise linear with corners at the strikes, so a crossing is a corner or lies inside a segment, and the segment above the top strike is extended by its slope (the signed call quantity). Zero crossings only; a payoff that touches zero at a corner and stays on one side is one break-even. Only for a structure of two or more legs of one expiry and not `other`, as the matcher's payoff (`optionstruct/payoff.go`): any other gives the gap `no_rule` (calendar, diagonal, different expiries, covered call and collar, which hold shares the structure does not list, and `other`). For a structure with exactly one crossing the number equals `Structure.Figures.BreakEven`; a test pins that.
5. Which structures are listed. Every structure of the matcher with two or more legs, and `covered_call`. A single naked leg (`short_put`, `long_put`, `long_call`, `naked_call`) is not a spread: it is counted in the sentence ("另有 N 个单腿持仓") and stays on the structures page. `Other` structures (units the matcher did not name) go in their own last group with their legs, the reason line the structures page already writes, and no figure. Order: by days to the shortest expiry, nearest first, then by underlying; the figure returns the rows in this order and with the day count (`DaysToExpiry`), the page copies both. The structures page orders by max loss; this page is about what to watch at a glance, so the order differs on purpose and the sentence says so.
6. A covered call is shown with its call leg only and the net credit of the call. The stock leg is not a `StructureLeg`; its shares come from the ledger cycle, which this page does not read. Max loss and break-even are the gaps `no_rule` as the matcher gives them, with the words "含股票，此页不算". The max gain is the matcher's figure.
7. One account per page, as `/positions`: the account is the query `account` word (the first alias when absent), only the main account must work, an unknown alias gives the gap page. Accounts are never pooled: the same underlying in two accounts is two blocks on two pages.
8. The page loads through the same `Config.load` as the structures pages (ledger, session days, replay, `Evaluate`), so the legs of this page and of the structures page are the same legs on the same data day. The quote read in `load` is not used by this page (its figures are not shown); it stays because `load` is one function and a second loader would be a copy.
9. The page carries the pages purity rule: `structures_page` imports no `math/big`, no `sort`, no `time` arithmetic on expiries and no `optionstruct` (it receives `NewOptionStructuresFunc` and the new `NewSpreadFunc` in `Config`). It copies every number, text, day count and the order of the rows from the figure and the structure.
10. Texts: kind names with a Chinese gloss (a table in `zh_texts.go`, one entry per kind of `kindTexts`), the column heads of the legs with a hint, the sentence and the note above. A gap is always the reason in words (`missing_input`: "有一条腿缺开仓价", `unsupported_currency`: "非美元，此页不算", `no_rule`: "此结构没有这项规则").
11. Web: a table of legs inside a card per structure; Go sends every cell and sentence. Menu: Options, label "价差腿", after "Structures".

## Modules

`docs/design/modules.md` is not changed. Paths match the rows contracts, compute, pages, ot and web (the first matching row owns each path). No store, dataset or migration change; the auto-deploy is not held. `cmd/ot/wire.go` (791 of 800 lines) and `cmd/ot/site.go` are not edited: `registerPage` in `init` is enough, as for `held_family_page`.

- compute (entry 1): new `internal/compute/spreadlegsapi/api.go` and `api_test.go`, new `internal/compute/spreadlegs/spreadlegs.go` and `spreadlegs_test.go`, `docs/contracts/compute.md`.
- contracts (entry 2): `contracts/schemas/page.spread_legs_page.v1.json`, `contracts/fixtures/page.spread_legs_page.v1.ok.json`, `web/src/generated/spread_legs_page.v1.ts` (generated), `docs/contracts/page-schemas.md`, new `internal/contracts/spread_legs_page_test.go`.
- pages (entry 3, with ot): in `structures_page`, new `spreads.go`, `spreads_test.go`, `testdata/spreads.golden.json`; edited `page.go` (Config field `Spreads`, `NewSpreadsPage`), `zh_texts.go`, `docs/contracts/pages.md`. Exception, stated: a third page in this package, because `load`, the href function and the kind texts are unexported and a second package would copy them. No file is near the ratchet: `list.go` 365, `load.go` 287, the largest, stay unchanged.
- ot (entry 3, with pages): new `cmd/ot/spread_legs_site.go` and `spread_legs_site_test.go`; edited `cmd/ot/check_serve.go`, `cmd/ot/pagereg_test.go`, `cmd/ot/pagelist_test.go`. The config of the page is built from the same site readers as `site_positions.go` builds for the structures pages, in the new file; `site_positions.go` is not edited.
- web (entry 4): new `web/src/pages/spreadlegs/SpreadLegsPage.tsx`, `web/src/types/spread_legs_page.ts`, test `web/test/pages/spreadlegs/SpreadLegsPage.test.tsx` and fixture `web/test/pages/spreadlegs/fixtures/spreads.ts` (a typed copy of the schema example of entry 2); edited `web/src/shell/TopNav.tsx`. Frontend stack: table = TanStack Table through the shadcn Data Table; no chart; Tailwind for layout.

## Interface

Owner of the shape: contracts. New `page.spread_legs_page.v1.json` (`additionalProperties: false`, schema name `spread_legs_page`): the common page keys of `page.structures_list.v1.json` (`schema`, `schema_version`, `run_id`, `build_id`, `scope`, `generated_at`, `data_times`, `run_status`, `title`, `stale`) plus `sentence`, `note` (text or null), `groups` (exactly 2: spreads, unidentified; each `{ title, structures }`), where a structure is `{ symbol, href, kind_text, kind_gloss, expiry_text, legs, net_credit, max_loss, max_gain, break_even, reason }`: `legs` is 1 to 8 of `{ side_text, strike, expiry_text, quantity, open_price }` (texts), the four figures are `Cell` = `{ kind: value|gap, text, value?, unit?, reason? }` except `break_even`, which is `{ kind: values|gap, text, values: [numbers], reason? }`, and `reason` is text or null (the unidentified-group line). All texts are 1 to 200 characters; at most 200 structures.

Owner of the figure: compute. New file `internal/compute/spreadlegsapi/api.go`, declarations only (a package of its own, as `blockapi`; `internal/compute/api.go` is not edited):

    type SpreadIn struct { Structures []compute.Structure; AsOf contracts.Day } // AsOf is the data day, injected
    type SpreadFigures struct { ID string; Expiry contracts.Day; DaysToExpiry int; NetCredit compute.FigureValue; BreakEvens []string; BreakEvenGap string } // Expiry is the shortest expiry of the legs, DaysToExpiry its calendar days from AsOf (0 or more); BreakEvens ascending, decimal text; BreakEvenGap is "" when BreakEvens is set
    type SpreadResult struct { Rows []SpreadFigures } // one row per input structure, already in the display order of Decision 5: DaysToExpiry ascending, then underlying (via the structure id), then id
    type SpreadFigure interface { Spreads(SpreadIn) SpreadResult } // pure: no I/O, no clock, exact decimals
    type NewSpreadFunc func() SpreadFigure

`NetCredit.Gap` is one of `missing_input`, `unsupported_currency`; `BreakEvenGap` is one of `no_rule`, `missing_input`, `unsupported_currency`. The implementation `internal/compute/spreadlegs` (`New() SpreadFigure`) is built by `cmd/ot` and handed to the page through `structures_page.Config.Spreads` (`spreadlegsapi.NewSpreadFunc`); a nil `Spreads` gives the page the gap `no_reader`, not an error.

Owner of the page: pages. `structures_page.NewSpreadsPage(c Config, alias string) pages.Page` (name `spread_legs_page`, schema `page.spread_legs_page.v1`).

## Issues

Build order: each entry compiles and passes its tests against main. Entries 1 and 2 are interface entries, first and independent; 3 (pages and ot, one judged entry) and 4 (web) depend only on them. Only entry 2 changes a file under `contracts/schemas`; the others say `Contracts: unchanged`.

### 1 legs-1-figure (interface)

Module: compute. Order: first. Files: `internal/compute/spreadlegsapi/api.go`, `internal/compute/spreadlegsapi/api_test.go`, `internal/compute/spreadlegs/spreadlegs.go`, `internal/compute/spreadlegs/spreadlegs_test.go`, `docs/contracts/compute.md`.

### 2 legs-2-schema (interface)

Module: contracts. Order: first. Files: `contracts/schemas/page.spread_legs_page.v1.json`, `contracts/fixtures/page.spread_legs_page.v1.ok.json`, `web/src/generated/spread_legs_page.v1.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/spread_legs_page_test.go`.

### 3 legs-3-page

Module: pages and ot. Order: after 1, 2. Files: `internal/pages/structures_page/spreads.go`, `internal/pages/structures_page/spreads_test.go`, `internal/pages/structures_page/testdata/spreads.golden.json`, `internal/pages/structures_page/page.go`, `internal/pages/structures_page/zh_texts.go`, `docs/contracts/pages.md`, `cmd/ot/spread_legs_site.go`, `cmd/ot/spread_legs_site_test.go`, `cmd/ot/check_serve.go`, `cmd/ot/pagereg_test.go`, `cmd/ot/pagelist_test.go`.

### 4 legs-4-web

Module: web. Order: after 2. Files: `web/src/pages/spreadlegs/SpreadLegsPage.tsx`, `web/src/types/spread_legs_page.ts`, `web/test/pages/spreadlegs/SpreadLegsPage.test.tsx`, `web/test/pages/spreadlegs/fixtures/spreads.ts`, `web/src/shell/TopNav.tsx`.

## Checked by CI

- `ci-go`: `go build ./...`, `go vet ./...`, `go test ./internal/compute/... ./internal/contracts/... ./internal/pages/... ./cmd/ot/...`.
- `ci-web`: `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest`, build.

## Left out

- The stock leg of a covered call (shares, basis) and so its max loss and break-even: the shares are in the ledger cycle, which this page does not read. Asked: should a covered call show the stock as a leg and the break-even as basis minus the call premium? The matcher's rule says the premium is never deducted from the basis, so this needs a decision on that rule first.
- Floating profit and loss, Greeks and the close cost of a spread (stored quotes could give them; the structures page already has them per structure). Not asked for here.
- Margin or buying-power of a spread (the broker's figure is not stored by account summary per structure).
- Multi-expiry structures (calendar, diagonal): max loss and break-even stay gaps `no_rule`, as the matcher.
- Spread history (closed spreads, how a spread was rolled) and a payoff chart on this page: the detail page has the chart; each group links to it.
- Pooling the same underlying across accounts, alerts on a spread, a search or filter box.
- A live quote or any new source: nothing here is fetched; a structure with no stored open price shows the gap.

## Adversarial

1. Duplicate of the structures page: the structures list already has one row per spread. Answer: it lacks legs, net credit and a multi-point break-even, and its order and columns serve the weekly decision; a third column set there would change a schema (`page.structures_list.v1`) other issues use (Decision 1). The new page reuses its loader and href so the two cannot disagree on the legs.
2. Net credit definition: remaining premium (`PremiumIncome`, which includes closes) versus open price. Chosen: open price times quantity (Decision 3), because a spread's net credit is what it was opened for; a half-closed spread uses the leg quantities of the matcher. Commission excluded and said in the note.
3. Two break-evens (iron condor, strangle): a single `FigureValue` cannot hold them, so the figure returns a list; for one crossing a test pins equality with the matcher's number (Decision 4), so the two never disagree.
4. A covered call looks wrong without the stock: stated in Decision 6 and Left out; its figures are gaps with the reason, not numbers that look like a spread.
5. `api.go` ceiling: no field is added to `StructureFigures`; a package of its own carries the new figure (Decision 2).
6. Likeliest slip: a pages package computing the break-even or the net credit itself (the structures page imports no `math/big`; entry 3 acceptance has a purity item) and an issue editing `internal/compute/api.go`.
7. Unit mix: a structure in JPY: the matcher already gives gaps, the figure copies that (Decision 3), never a sum across currencies.
8. The hidden cost of reusing `load`: a quote read whose result is unused (Decision 8); accepted for one loader; if it proves slow the owner can ask for a quotes-free `load` as its own issue.
9. Review point 5 (Decision 9 against Decision 5 and the example): adopted; the figure carries `Expiry`, `DaysToExpiry` (from an injected `AsOf`) and returns rows in display order, so the page counts and sorts nothing (Interface, Decisions 2, 5, 9). Collar added to the `no_rule` break-even list (Decision 4).
