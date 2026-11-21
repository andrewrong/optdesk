# held companies: one page lists each company the family holds with its accounts, market value, share of the total and next earnings date

Author: the optdesk designer (draft, revised 2026-11-21 after the adversarial review and the dispatcher decisions)
Owner ground: ask optdesk-5618, owner UI doc row 6 ("持仓公司": next earnings date, which accounts hold it, share of the portfolio)
Date: 2026-11-21

Source: the main of 2026-11-21. Held stocks are read per alias by `heldPositions` (`cmd/ot/underlyings_site.go`); `household_page.BuildFamily` reads the newest capture of every alias, counts an account or names why not, and gives the family net in USD with one USDJPY rate (`FamilyResult.FxRate`, `readBook` in `internal/pages/household_page/family.go`); `blockapi.HoldingFigures.Holdings` values one stock row from its stored mark; stored earnings rows are read by `store.EarningsDateReader` (`obs.earnings_date`, states `listed`, `absent`, `unsupported`, `error`) and the owner override is `underlying.Row.ReportDateOverride`. Rules carried: the front end computes nothing; Chinese text of a package is only in its `zh_texts.go`; no account id anywhere (an account is its alias word); only the main account must work, another account failing is a named line, never a failed page; a missing figure is a gap with its reason, never a zero.

New page `/held-family` (menu Portfolio, label "Held companies"). Example, all names and numbers are made up:

    Held companies 持仓公司         family total 340,000 USD (main + wife + daughter), data day 2026-11-20
    把全家持有的股票按公司合并：哪些账户持有、值多少钱、占全家总资产几成、下次财报几号。
    Company   Held in           Market value (USD)  Share of total  Next earnings
              持有账户          市值（美元）         占全家总资产     下次财报（距今天数）
    AAA       main, wife        41,230              12.1 %          2026-11-25 (in 4 days)
    BBB       main              30,900               9.1 %          2026-12-03 (in 12 days)
    CCC       wife              12,400               3.6 %          no date: the earnings job has no row for this company
    EEE       main              -2,000              -0.6 %          no date: no underlying row for this symbol
    DDD       main, daughter    gap: no mark        gap             2026-11-21 (today)
    Sum of the rows with a value 82,530 USD = 24.3 % of 340,000
    注：市值 = 持股数 x 最近一次收盘价（最多早 5 天，日元按全家同一个 USDJPY 折成美元），总资产 = 各账户采集日的净值之和（含现金和期权），所以各行占比之和小于 100 %。空头股票显示负数。
    只有期权、没有股票的公司不在此表。账户（daughter）没有采集，未计入，其持仓不在上表。2 行的收盘价早于采集日。

Each column header has the Chinese hint under it. Rows are sorted by market value, rows with a gap last. A cell that cannot be given shows its reason in words; no cell shows 0 for a missing number.

## Modules

`docs/design/modules.md` is not changed. Paths match the rows contracts, compute, pages, ot and web; the first matching row owns each path. No store, dataset or migration change; the auto-deploy is not held. `cmd/ot/wire.go` (791 of 800 lines) is not edited: `registerPage` in `init` is enough.

- contracts (entry 2): `contracts/schemas/page.held_family_page.v1.json`, `contracts/fixtures/page.held_family_page.v1.ok.json` (new, the example payload of the schema: value, short, gap and each earnings reason), `web/src/generated/held_family_page.v1.ts` (generated), `docs/contracts/page-schemas.md`, new `internal/contracts/held_family_page_test.go`.
- compute (entry 1): `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `internal/compute/pagefig/held.go` and `held_test.go` (new), `docs/contracts/compute.md`.
- pages (entry 3, with ot): in `household_page`, new `held.go`, `held_test.go`, `testdata/held.golden.json`; edited `page.go` (Config), `zh_texts.go`, `docs/contracts/pages.md`. Exception, stated: a second page in this package, because `readBook`, `distinctAliases` and `whoseOf` are unexported and a second package would copy them.
- ot (entry 3, with pages): new `cmd/ot/held_family_site.go` and `held_family_site_test.go`; edited `cmd/ot/check_serve.go`, `cmd/ot/pagereg_test.go`, `cmd/ot/pagelist_test.go`.
- web (entry 4): new `web/src/pages/heldfamily/HeldFamilyPage.tsx`, `web/src/types/held_family_page.ts`, test `web/test/pages/heldfamily/HeldFamilyPage.test.tsx` and fixture `web/test/pages/heldfamily/fixtures/held.ts` (a typed copy of the schema example of entry 2); edited `web/src/shell/TopNav.tsx`. Frontend stack (ADR 0034): table = TanStack Table through the shadcn Data Table; no chart; Tailwind for layout.

## Interface

Owner of the shape: contracts. New `page.held_family_page.v1.json` (`additionalProperties: false`, schema name `held_family_page`): the common page keys of `page.household_page.v1.json`, plus `total` (`{ text, currency, day, accounts_text }`), `columns` (exactly 5, each `{ id, label, hint }`), `rows` (0 to 200 of `{ company, accounts: [{ alias, label }], value: Cell, share: Cell, earnings: EarningsCell }`), `rows_sum` (`Cell`), `lead_notes` (0 to 20 text) and `gaps`. `Cell` is `{ kind: value|gap, text, reason? }`; `EarningsCell` is `{ kind: date|gap, date?, days_to?, text, reason? }` (`days_to` an integer from 0). All texts are 1 to 200 characters.

Owner of the figure: compute. `blockapi/api.go` gains, declarations only, a figure of its own (not a method of `PageFigures`):

    type HeldLineIn struct { Alias, Underlying, Currency string; Value compute.FigureValue; MarkDay contracts.Day } // Value in USD, from Holdings
    type HeldEarningsIn struct { Rows []HeldEarningsRow; Override contracts.Day; Read bool } // Read false: no reader
    type HeldEarningsRow struct { Status string; Report, ObservedOn contracts.Day }
    type HeldCompanyIn struct { Underlying, Currency string; Resolve string; Earnings HeldEarningsIn } // Resolve: ok, no_underlying or no_issuer
    type HeldIn struct { Lines []HeldLineIn; Companies []HeldCompanyIn; TotalUSD compute.FigureValue; Today contracts.Day }
    type HeldRow struct { Underlying, Currency string; Aliases []string; Value, Share compute.FigureValue; OldestMark contracts.Day; Next contracts.Day; DaysTo int; EarnReason string }
    type HeldResult struct { Rows []HeldRow; RowsSum compute.FigureValue }
    type HeldFigure interface { HeldCompanies(HeldIn) HeldResult }   // pure: no I/O, no clock, exact decimals

`NewHeldFigureFunc` constructs it (`internal/compute/pagefig`). Rules, each stated once and only here: lines of one `(Underlying, Currency)` merge into a row; value is the sum of its lines, and one line with a gap makes the row value the gap `missing_quote`; a negative line (short stock) counts as its negative value, so the share is negative; share is value over `TotalUSD` in percent, a gap when either is a gap; rows sort by value descending, gaps last, ties by underlying. Earnings, the one copy of the rule for this page: the newest row by `ObservedOn` (a tie keeps the later) is the state; `Override` first, else its `Report`; a date before `Today` is `past_date`; `DaysTo` counts calendar days; state `absent` or no rows gives `no_row`, `unsupported` gives `unsupported`, `error` gives `row_error`, `Read` false gives `no_reader`, `Resolve` is checked before the rows: `ok` reads the rows as above, `no_underlying` gives `no_underlying` (no underlying row for the symbol), `no_issuer` gives `no_issuer` (a row without issuer); any other value is `no_underlying`.

Owner of the page builder: pages, declared here and landed together with its only user, entry 3 (pages and ot in one entry, so no module entry waits on a sibling module entry). In `internal/pages/household_page/held.go`: `func BuildHeld(ctx context.Context, cfg Config, day contracts.Day) (HeldPage, error)`; `HeldPage` is the Go struct of the schema of entry 2. `Config` in `page.go` gains `Underlying underlying.Reader`, `Earnings store.EarningsDateReader`, `Held blockapi.HeldFigure` (nil gives gaps `no_reader`). `cmd/ot` calls `BuildHeld` and nothing else of the package. The web page reads the payload through the schema and the example of entry 2 only.

## Decisions

1. New page, not a section elsewhere. Weighed: (a) `home_companies` (card `held_companies_summary`): one cell per name for one member, no accounts, no value, no share, no family read, and its `earningsCell` is unexported and shows only a past window; widening it changes a schema three versions old. (b) A block on the `household_page` family view: it holds the book and `Included`, but the owner's list needs a sorted company table and a new block means a schema v2 and a change of a live page; a new schema with no existing page touched is the smaller change. (c) `/positions`: one account at a time. The page is named `held_family_page`, not `held_companies_page`, so it is not confused with `held_companies_summary`; the code sits in `household_page` because it reuses its book reader.
2. Accounts: only the accounts the family total counts (`FamilyResult.Included`) have rows; an account left out is one grey line with its reason in Chinese. Rows and denominator cover the same accounts, not always the same day (a capture may be older, as `/household` already says).
3. Market value: quantity x the stored mark of the capture day or the newest mark within the household fallback of 5 days. One FX rule: every JPY line is valued with `FamilyResult.FxRate`, the same rate the family net uses, passed into `HoldingsIn`; no rate per alias day. Options and cash are not in a row.
4. Share = market value over the family net liquidation (cash and options in the denominator), so rows sum below 100 %. The hint names the basis: marks are the latest close up to 5 days old, the total is the net of each account's capture day; one lead note counts the rows whose mark is older than the data day (`OldestMark`). It is not the share among stocks, which `/household` shows per account.
5. Short stock: kept as a row with negative value and share, sorted last among rows with a value, the hint says "空头显示负数". Options-only companies have no row and the page says so (the owner sells puts; their exposure is on `/options`).
6. Earnings: the page reads the stored rows through `store.EarningsDateReader` for the 120 days up to the data day (the window `home_companies` uses) and hands them to the figure; the window rule and the override rule live only in `pagefig/held.go`. Not a third copy: the page imports nothing from `internal/jobs`, and `household_page` stays free of `sort`, `math/big`, `internal/jobs` (its purity test).
7. Company and issuer come from `underlying.Reader.Current(symbol)`. A miss shows its real reason, carried as `HeldCompanyIn.Resolve` (`no_underlying`: "no underlying row for symbol X"; `no_issuer`: "underlying row has no issuer"), never a generic "not resolved". Risk named: broker symbols of JP and HK names may not match an underlying row, and the earnings job covers few JP names, so some rows will be gaps with those reasons. A lead note counts the rows without a date, so the owner sees coverage; before cut the window counts the gaps on live data (outcome check) and reports the number.
8. No price line, buy price or sell price: owner data, not shown. Chinese words are consts of `zh_texts.go`; column labels and numbers are English; `reason` fields are the English words of the figure.

## Issues

Build order: each entry compiles and passes its tests against main. Entries 1 and 2 are interface entries, first and independent; 3 (pages and ot, one judged entry) and 4 (web) depend only on them. Only entry 2 changes a file under `contracts/schemas`; the others say `Contracts: unchanged`.

### 1 held-co-1-figure (interface)

Module: compute. Order: first. Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `internal/compute/pagefig/held.go`, `internal/compute/pagefig/held_test.go`, `docs/contracts/compute.md`.

### 2 held-co-2-schema (interface)

Module: contracts. Order: first. Files: `contracts/schemas/page.held_family_page.v1.json`, `contracts/fixtures/page.held_family_page.v1.ok.json`, `web/src/generated/held_family_page.v1.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/held_family_page_test.go`.

### 3 held-co-3-page

Module: pages and ot. Order: after 1, 2. Files: `internal/pages/household_page/held.go`, `internal/pages/household_page/held_test.go`, `internal/pages/household_page/testdata/held.golden.json`, `internal/pages/household_page/page.go`, `internal/pages/household_page/zh_texts.go`, `docs/contracts/pages.md`, `cmd/ot/held_family_site.go`, `cmd/ot/held_family_site_test.go`, `cmd/ot/check_serve.go`, `cmd/ot/pagereg_test.go`, `cmd/ot/pagelist_test.go`.

### 4 held-co-4-web

Module: web. Order: after 2. Files: `web/src/pages/heldfamily/HeldFamilyPage.tsx`, `web/src/types/held_family_page.ts`, `web/test/pages/heldfamily/HeldFamilyPage.test.tsx`, `web/test/pages/heldfamily/fixtures/held.ts`, `web/src/shell/TopNav.tsx`.

## Checked by CI

- `ci-go`: `go build ./...`, `go vet ./...`, `go test ./internal/compute/... ./internal/contracts/... ./internal/pages/... ./cmd/ot/...`.
- `ci-web`: `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest`, build.

## Adversarial

Full answer path: this section with Decisions 1 to 7, for each point of the review `designs/held-companies.adversarial.md` in the draft store (not committed).

1. Gap vs goal (quantity, basis, FX, short stock, options-only, section vs page): adopted. `quantity_text` and `Name` and `RowsShare` dropped; basis in the hint and a lead note (4); one FX rate (3); short and options-only rules (5); the two nearer homes weighed and the page renamed (1).
2. Symbol-to-issuer risk: adopted; real reasons and a coverage note, live count before cut (7).
3. Simpler way: adopted for the dead fields; the figure interface stays (pure sort and decimals must be in `pagefig`); four issues stay (pages and ot are one entry, so the builder needs no interface entry of its own).
4. Lock-in: the second page in `household_page` is stated as an exception (Modules); the fixed 5 columns lock a v2 for a new column, accepted.
5. False claim (reuse of the jobs `earnings`): adopted; no reuse, one copy of the rule in `pagefig` (6). Rows and denominator may differ in day: stated in 2 and 4.
6. Likeliest slip: adopted; `Order:` in every draft (entry numbers only), `wire.go` out of Files, purity respected (6).
