# household family total: the household page and the home household card show the whole family by default, with a visible line for every account the total does not hold

Author: the optdesk designer (draft)
Owner ground: ask optdesk-5290 (owner 2026-11-19 "为什么household就我的account"); this design is pending
Date: 2026-11-19

Source: the live pages of 2026-11-19. `/household` builds one alias: `cmd/ot/household_site.go` takes the query `account` and falls back to the first alias of the book (`rd.aliasList()[0]`); `household_page.Build` puts that one alias in `scopes` (`internal/pages/household_page/builder.go`, the lines that choose the alias and build `scopes`); the home card is built for the first alias only (`cmd/ot/home_site.go`, `home_household.Config.Alias`; `docs/design/pages-real-data.md` entry 11 says "of the main account"). The book has three aliases and a capture is stored per alias, so the other two are on disk and never shown. The page shows no sum. Rules carried: the front end renders data and computes nothing; Chinese text of a package is in its `zh_texts.go` (checked by `TestZhTextsDoor`, `cmd/ot/zhdoor_test.go`); payload fields are added, never changed or removed; `docs/design/account-data.md` (2026-11-09: only the main account must work, the others may be unavailable) is kept, so an unavailable account is shown as such and is never a failure of the page.

Owner rules this design is judged by (quoted): owner 2026-11-19 a: "family total on top plus one row per account (pooling allowed)"; never silently drop an account (every account is a row or a named line); only the main account must work (`docs/design/builders-wave.md:11`, `docs/design/account-data.md`): a failing other account never fails the page.

Superseded on purpose: decision 5 of `docs/design/builders-wave.md` ("one account per household page, owner: never pools accounts") and the package comments "pools no account" of `household_page` and `home_household`. The owner's ask of 2026-11-19 replaces them for the default view. The scope convention of `docs/design/household-figures.md` and of the schema (#1680: `household` is me and spouse, the daughter is a scope apart, nothing is summed across scopes) is superseded too: the key `household` is reused (the web already knows it) and now holds every account of the book, the daughter included, summed by the figure of entry 3; entry 1 rewrites the `AccountScope` description of the schema to say so. The design PR itself edits the superseded text (see "Design PR also edits" under Modules). What stays: `?account=<alias>` is exactly the page of today (`TestOneScopeNoPooling` keeps its meaning: a named alias gives one scope), and the pages module still computes nothing: the sum is a compute figure (entry 2 and 3), the pages copy its text.

Read on origin/main: `AGENTS.md`, `docs/design/modules.md`, `docs/design/candidates-zh-headings.md`, `docs/design/builders-wave.md`, `docs/design/pages-real-data.md`, `cmd/ot/{household_site,home_site,accounts,site_positions}.go`, `cmd/ot/household_site_test.go`, `internal/pages/household_page/{page,blocks,builder,holdings,strings}.go` and `builder_test.go`, `internal/pages/home_household/member.go`, `internal/compute/blockapi/api.go`, `internal/compute/pagefig/{household,pagefig}.go`, `contracts/schemas/page.household_page.v1.json`, `contracts/schemas/page.household_summary.v2.json` (through its golden), `web/src/pages/household/HouseholdPage.tsx`, `web/src/pages/home/{HouseholdCard,CardNotes}.tsx`, `web/src/types/household_page.v1.types.ts`, `web/test/pages/household/householdPage.test.tsx`, `internal/contracts/fixtures_test.go`, `docs/design/ratchet.json`.

## What you see

`/household` (no query) of a book with two or more aliases: the title "家庭合计" and one grey sentence "全家各账户最新一次采集的净值相加，按美元显示；下表列出每个账户。" Under it a row of links "全家 · 账户（main） · 账户（wife） · 账户（daughter）" (one link per alias of the book, the current one marked; the alias words are the owner's own names from the book). Then, in grey lines under the sentence, one line for every account the total does not hold or holds from an older day, for example "账户（wife）今天未采集到，合计不含". The overview shows one row "家庭净值（美元）· 计入 2 / 3 个账户" with the sum (counted M of N accounts, next to the number, so a total that shrinks because the daughter's statement capture is old is visible at once). When a JPY account is counted, one more grey line says "日元账户按 USDJPY 汇率 R（日期 D，来源 S）折成美元". Under the cash figure and in the card notes one fixed line says "现金分放在各账户，只担保本账户的卖出看跌；合计现金不是可共用的保证金". The account table shows one row per account (net value, cash, data day) under the heading of the scope "全家"; an account that is not in the total has gap words in its cells ("未采集到") and no figure, never a zero. The other blocks (holdings, movers, trend, stress and so on) show the existing gap line, now worded "合计页不分项，点上方的账户看单个账户的明细". A book with one alias keeps today's page and today's card (no row of links, no count).

`/household?account=<alias>`: the page of that one account as it is today (same blocks, same English words, plus the one added key `views` of a book with two or more aliases), with the same row of links so the way back to "全家" is one click.

Home card "Household": its four tiles (net value, cash, previous trading day, month to date) are the family sums. The subtitle reads "全家合计 · 计入 2 / 3 个账户 · 最早数据日 2026-11-18"; under the tiles the same per-account lines, the JPY rate line when it applies and the cash line, in the card notes. The cash tile shows the family cash share of the family net value (the figure gives it). A change tile that one counted account cannot give (no earlier capture) is a gap that names it, never the sum of the others; the previous-trading-day tile is a gap with the words "各账户数据日不同，不显示单日变化" when the counted accounts have different data days, because a sum of one-day changes of different days is not a day change.

An account with no capture today is not hidden: if its last capture is within 3 trading days it is counted, with its own data day in the table and the line "账户（wife）用的是 2026-11-17 的采集，早于最新交易日，已计入合计"; older than 3 trading days (the stale rule the page has today), never captured, or whose reader failed, it is left out and the line says so ("账户（wife）读取失败，合计不含" for a failed read). The page's `data_times` has one `account` entry per counted account, `stale` is that of the oldest counted capture, `gaps` has one entry per left-out account and per detail block, and `run_status` is `partial` (the ten detail blocks are always gaps on the family page); `notes` holds the cash line and the statement line of each counted statement alias, as the named page does.

## Modules

`docs/design/modules.md` is not changed. Paths match the rows contracts, compute (`internal/compute/**`), pages (`internal/pages/**`), ot (`cmd/**`) and web (`web/**`); the first matching row owns each path. No store, dataset or migration change; nothing holds the auto-deploy. No file listed in `docs/design/ratchet.json` is touched.

Design PR also edits (docs only, in this pull request, not in an entry): `docs/design/builders-wave.md` line 11 (the Source phrase "the owner never pools accounts") and line 27 (decision 5), and the scope line of `docs/design/household-figures.md` (a line under its `Revision:` line), each gaining "superseded by household-family-total.md" for the default view; entry 7 edits the "summed across scopes" comment of the web types file.

- contracts: `contracts/schemas/page.household_page.v1.json`, the new test file `internal/contracts/household_page_family_test.go`, `docs/contracts/page-schemas.md` (entry 1). `web/src/generated/` holds no household_page type (the web type is a hand-written placeholder in `web/src/types/`, entry 7), so nothing is regenerated. `page.household_summary.v2` is not changed: the card uses its existing `subtitle`, `notes` and tile fields.
- compute: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `docs/contracts/compute.md` (entry 2); `internal/compute/pagefig/family.go` and `family_test.go`, both new (entry 3).
- pages: `internal/pages/household_page/{page,blocks,builder}.go`, the new `family.go`, `zh_texts.go` and `family_test.go`, `testdata/holdings.golden.json` and `docs/contracts/pages.md` (entry 4; it documents the `household_page` and `home_household` changes, entry 5 does not edit it); `internal/pages/home_household/member.go`, the new `family.go`, `zh_texts.go`, `family_test.go` (entry 5).
- ot: `cmd/ot/household_site.go`, `cmd/ot/home_site.go`, `cmd/ot/household_site_test.go`, `cmd/ot/home_site_test.go` (entry 6).
- web: `web/src/pages/household/HouseholdPage.tsx`, `web/src/types/household_page.v1.types.ts`, the new `web/test/pages/household/householdFamily.test.tsx` and the new fixture `web/fixtures/household-page/family.json` (entry 7); the directory has no MANIFEST file, and `TestFixturesValidate` (`internal/contracts/fixtures_test.go`) reads the whole directory against the schema, so the fixture must validate.

Sizes: `builder.go` is 506 lines, `blocks.go` 397, `home_household/member.go` 544, `household_site_test.go` 262, `home_site_test.go` 435, `householdPage.test.tsx` 462 (not edited); `builder_test.go` is 795 lines and is not edited, the new tests go to `family_test.go`. Every new file is under 600 lines. No ceiling in `ratchet.json` changes.

## Interface

Two interfaces change; each lands before its users.

Owner of the shape of the page: contracts. `contracts/schemas/page.household_page.v1.json`, v1 gains two optional properties (every `required` list unchanged, nothing removed, no JSON number added, not nullable):

    Page += views?:      View[]     // 2 to 21 items
    Page += lead_notes?: Text[]     // 1 to 20 items
    View  = { key, label: Text, href: Text, current: boolean }   // additionalProperties: false, all four required

`key` is `household` or an alias (the pattern of the scope key), `href` starts with `/household`; `label` and `href` are strings of 1 to 200 characters as the other texts of the schema. The family page uses the existing parts as they are (the six columns of the accounts block, `overview.totals` holding the one USD total cell as the other money blocks do): `scope` is `household`, `scopes` holds one scope `household` whose `accounts` list every account tag, the accounts block holds one row per account with that `scope`, `overview.rows` holds the one family row. `lead_notes` are the lines of the sentence "账户（x）…合计不含", its siblings "…已计入合计" and "…读取失败，合计不含", and the rate line of a counted JPY account; a page without gap or older day has no `lead_notes` key and never `"lead_notes": null`. A stored page read by an older reader has neither key.

Owner of the shape of the figure: compute. `internal/compute/blockapi/api.go` gains, as declarations only, a figure of its own that is not a method of `PageFigures` (so no existing implementation or fake breaks; the pattern of `NewHoldingFiguresFunc`):

    type FamilyAccountIn struct {
        Alias, Whose, Day string          // Day: data day of the capture, "" when none
        Reason            string          // "" counted, "no_data" never captured, "stale_input" too old, "read_error" the reader failed
        Values, Prior, MonthStart []ValueIn
    }
    type FamilyIn struct { Accounts []FamilyAccountIn; UsdJpy, RateDay, RateSource string }  // the rate as the Fx reader gives it
    type FamilyAccountOut struct { Alias, Reason, Day string; Net, Cash compute.FigureValue }
    type FamilyResult struct {
        Currency string                     // "USD"
        Included []string                   // aliases in the total, input order
        Excluded []FamilyAccountOut         // aliases left out, Reason: no_data, stale_input, read_error, unsupported_currency
        Rows     []FamilyAccountOut         // one per input account, input order
        Net, Cash, CashShare, DayAmount, DayPct, MonthAmount, MonthPct compute.FigureValue  // CashShare: Cash in percent of Net
        OldestDay string                    // oldest Day among Included, "" when none
        FxRate, FxDay, FxSource string      // set only when a JPY account is counted: the rate used, its day, its source
    }
    type FamilyFigure interface { Family(FamilyIn) FamilyResult }
    type NewFamilyFigureFunc func() FamilyFigure

The rules, written once in the doc comment of `Family`: an account is counted when `Reason` is empty and its NetLiquidation is USD, or JPY with a positive `UsdJpy` (the amount is divided by the rate through the existing `rate` function of `pagefig`, `holdings.go`, the one the holdings figure uses); any other currency, or JPY without a rate, is left out with reason `unsupported_currency`. `Net` is the sum over `Included`, `Cash` the sum of their TotalCashValue (a gap `missing_input` when one counted account has none), `CashShare` is Cash over Net in percent (a gap when either is a gap, none given for a negative Cash, as the card's share is today), `DayAmount` is the sum of net minus prior net (the gap `stale_input` when the counted accounts have different `Day`, because a sum of one-day changes of different days is not a day change) and `MonthAmount` the same against the month start (each a gap `missing_input` when one counted account lacks that earlier value), `DayPct` and `MonthPct` are those amounts over the sum of the earlier nets, all by exact decimal arithmetic as `pagefig` does today; with no account counted every figure is the gap `no_data`. An account listed twice (same `Alias`) is counted once, the first; the result `Rows` still has one row per distinct alias. The sum of the rest is never given in place of a gap.

## Decisions

1. `/household` without `account` shows the family: the sum of the latest capture of every alias of the book, in USD. A JPY-valued account is converted with the stored USDJPY rate of the newest data day among the accounts (`Config.Fx`, `LatestFxRate`, which gives the rate, its day and its source); the page says the rate, its day and its source in a line whenever a JPY account is counted; without a rate the account is left out and named. The total says "计入 M / N 个账户" next to the number on the page and in the card subtitle. The owner sees one family net value, not the main account's.
2. Every account of the book has a row in the account table of the family page, counted or not; an account left out shows gap words and no figure. No account is ever missing from the table, and no gap is a zero.
3. Every account that is left out has one grey line under the page sentence with the Chinese words of its reason (no capture today, capture older than 3 trading days, no USDJPY rate), and every account counted from an older day has one line with that day. The owner reads why the total is what it is without opening anything.
4. `?account=<alias>` shows that account exactly as today (English words unchanged); an alias not in the book is still the gap page "Unknown account"; the query `account=household` is the family page. A row of links on both views goes between the family and each account.
5. The family page does not sum the detail blocks (holdings, movers, trend, stress, income, month change, cash after, obligations, concentration, allocation): they keep the gap `not_built` with the words "合计页不分项，点上方的账户看单个账户的明细". The family view answers "how much does the family hold"; the detail stays one account at a time.
6. The home card shows the family sums in its four tiles with a Chinese subtitle and the same per-account lines in the card notes. A change tile whose counted accounts do not all have the earlier capture is a gap naming the account.
7. An account counts when its capture is at most 3 trading days old, the stale rule both pages already use; the data day of each account is in its table row. The total mixes data days on purpose and says so (decision 3), because the wife's capture often fails and the daughter's comes from the statement; the card's day-change tile is a gap when the days differ (Interface).
7b. Only the first alias (the main account) can fail the page. A reader error of any other alias is that alias's left-out row and line with reason `read_error`; an error of the session-days reader, or of the first alias, is an error of the build (the page of today for the main account fails the same way). This keeps "only the main account must work".
7c. Alias rules: the book already refuses a repeated alias and a repeated id (`internal/accounts/toml/book.go`, `checkAlias`); the page counts an alias listed twice in `Config.Aliases` once (test). An alias named `household` is not refused by the book: the page never reads it as an account, leaves it out with reason `read_error` and the not-read line, and `?account=household` is the family page, never an account (test); a book whose first alias is `household` gives the gap page `no_reader`.
8. Account labels are `账户（<alias>）`, whose words on the family page are "本人" for the first alias and "其他账户" for the others (Chinese, from `zh_texts.go`, the words the schema's `AccountTag` description already uses); the named-account page keeps its English "Me" / "Other account" until P7, so the two sets never meet on one page; the book has no Chinese display name per alias (see Left out).
9. The Chinese texts are constants of the two new `zh_texts.go` files; the web files carry no Chinese and no new visible English word; the page and the card gain only optional fields, and a payload without them (an older stored page) draws what it draws today. A book with one alias keeps today's page and card byte for byte (no `views`).
10. Family cash is shown with the fixed cash line (cash secures puts only in its own account) wherever it is shown, and no candidate or gate code reads `FamilyFigure` (Left out).

## Issues

Build order: entries 1 and 2 first, in parallel (different modules); entries 3, 4 and 5 after 2 (4 also after 1), in parallel, because 4 and 5 test with a fake `FamilyFigure` and need only the interface; entry 6 after 3, 4 and 5 (it wires `pagefig.NewFamily` and both builders); entry 7 after 1 only (it reads its own fixture; the live link "全家" opens the family page once 6 has merged, which no test of 7 needs). Entries are not merged (five modules). Each merge leaves main green: the schema fields are optional, the figure is a new interface, `Config` fields are added, and the route change is the last one.

### 1 household-family-contract (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/schemas/page.household_page.v1.json`, `internal/contracts/household_page_family_test.go` (new), `docs/contracts/page-schemas.md`
Contracts: changed (`contracts/schemas/page.household_page.v1.json`)
Design: docs/design/household-family-total.md#Interface
Acceptance:
1. The `AccountScope` description of the schema says that `household` holds the accounts of the book summed by the page figure and that every other scope is a single account; no property, `required` list or pattern of `AccountScope` changes. The schema has optional `views` (array of `View`, `minItems` 2, `maxItems` 21) and optional `lead_notes` (array of 1 to 200 character strings, `minItems` 1, `maxItems` 20), neither nullable and neither in any `required` list of the page; `View` has `key`, `label`, `href`, `current`, all four in its `required` list, `additionalProperties: false`, `href` with a pattern that starts with `^/household`.
2. No `required` list of the schema differs from main, nothing is removed, and no new property is a JSON number.
3. `household_page_family_test.go` (new file) has `TestFamilyFieldsValidate` (new: an instance with `views` and `lead_notes` passes), `TestFamilyOldShapeValidates` (new: an instance without them passes), `TestViewNeedsAllFour` (new: a view without `current` fails), `TestFamilyFieldsNullFail` (new: `"views": null` and `"lead_notes": null` each fail); the file builds its instance with the helpers of the other test files of the package (`minimal`, `readRaw`, `schemasDir`) and no helper is copied.
4. `docs/contracts/page-schemas.md` names the two new fields in one sentence under the page entry.
Checked by CI: `go test ./internal/contracts/...`.
Left out: any change to `page.household_summary.v2`.

### 2 family-figure-interface (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `docs/contracts/compute.md`
Contracts: unchanged
Design: docs/design/household-family-total.md#Interface
Acceptance:
1. `api.go` declares `FamilyAccountIn`, `FamilyIn`, `FamilyAccountOut`, `FamilyResult`, `FamilyFigure` and `NewFamilyFigureFunc` as in the Interface section and nothing else new; `PageFigures` has the same six methods as on main.
2. The doc comment of `FamilyFigure.Family` states the counting rule, the sum rules, the gap rule and the reasons `no_data`, `stale_input`, `read_error`, `unsupported_currency`, the `CashShare` and `FxRate`/`FxDay`/`FxSource` rules, the mixed-day gap of `DayAmount` and the once-per-alias rule as in the Interface section; the file holds declarations only. `docs/contracts/compute.md` names the new figure and its types in one paragraph.
3. `api_test.go` has a fake `FamilyFigure` and `TestFamilyFigureContract` (new): a constructor of type `NewFamilyFigureFunc` returns a value whose `Family` returns a `FamilyResult` with `Currency` USD for one counted input, and the fake of `PageFigures` that the file already holds still compiles unchanged.
4. `api.go` stays under 800 lines (it is 325 today); `docs/design/ratchet.json` is unchanged.
Checked by CI: `go test ./internal/compute/...`.

### 3 family-figure-pagefig

Module: compute. Size: M.
Order: after 2 (only).
Files: `internal/compute/pagefig/family.go` (new), `internal/compute/pagefig/family_test.go` (new)
Contracts: unchanged
Design: docs/design/household-family-total.md#Interface
Acceptance:
1. `family.go` declares `NewFamily() blockapi.FamilyFigure` and a compile-time check `var _ blockapi.FamilyFigure`; it uses `math/big` rationals as `household.go` does, no `float64`, no clock, no I/O.
2. For two USD accounts the result has `Net` equal to the exact decimal sum, `Included` of both aliases in input order, `Rows` of both, and `Excluded` empty.
3. An input with `Reason` `no_data` or `stale_input` is in `Excluded` with that reason and is not in `Net`; a JPY account with a positive `UsdJpy` is in `Net` divided by the rate; a JPY account with an empty `UsdJpy` and an account of another currency are in `Excluded` with `unsupported_currency`; an input with `Reason` `read_error` is in `Excluded` with that reason; the conversion calls the existing `rate` function of `pagefig` (no second parser of the rate); `FxRate`, `FxDay`, `FxSource` are the input rate, day and source when a JPY account is counted and empty otherwise; an alias listed twice is counted once.
4. `Cash` is a gap `missing_input` when a counted account has no TotalCashValue; `DayAmount` and `MonthAmount` are gaps `missing_input` when a counted account has no prior or month-start net, and the figure is never the sum of the others; with no counted account every figure is the gap `no_data` and `OldestDay` is "". `CashShare` is the exact percent of `Cash` over `Net` (a gap `no_data` without a usable net, none for a negative cash); `DayAmount` and `DayPct` are the gap `stale_input` when the counted accounts have different `Day`.
5. `OldestDay` is the least `Day` among the counted accounts.
6. `family_test.go` (new) has `TestFamilySumsTwoUsd`, `TestFamilyExcludesByReason`, `TestFamilyJpyNeedsRate`, `TestFamilyGapNotPartialSum`, `TestFamilyNoneCounted`, `TestFamilyCashShare`, `TestFamilyDayGapWhenDaysDiffer`, `TestFamilyOnceForRepeatedAlias` (all new); `TestFamilyJpyNeedsRate` also checks the rate fields; each is parallel and uses no fixed path.
7. `family.go` and `family_test.go` are each under 600 lines; the new files import nothing beyond `math/big`, `strings`, `compute` and `blockapi`.
Checked by CI: `go test ./internal/compute/...`.

### 4 household-family-pages

Module: pages. Size: M.
Order: after 1 and 2.
Files: `internal/pages/household_page/page.go`, `internal/pages/household_page/blocks.go`, `internal/pages/household_page/builder.go`, `internal/pages/household_page/family.go` (new), `internal/pages/household_page/zh_texts.go` (new), `internal/pages/household_page/family_test.go` (new), `internal/pages/household_page/testdata/holdings.golden.json`, `docs/contracts/pages.md`
Contracts: unchanged
Design: docs/design/household-family-total.md#Decisions
Acceptance:
1. `page.go` has `Config.Family blockapi.NewFamilyFigureFunc` (a new field, nil gives the gap page `no_reader` for the family view only) and the package comment no longer says "pools no account"; `NewPage` is unchanged for an alias that is named, and `Build` with the alias "" still builds the first alias as today (`TestOneScopeNoPooling` is not edited).
2. `family.go` (new) declares `BuildFamily(ctx, c Config, day contracts.Day, at time.Time) ([]byte, error)`: it reads the session days once, reads the capture of every alias of `c.Aliases` with the existing `capture`, marks an alias `no_data` when it has no capture and `stale_input` when its capture is more than `staleSessions` session days old, reads the rate with the existing `Fx` reader only when a capture holds JPY, calls `c.Family().Family(...)`, and copies its text; it passes the rate with its day and source as the reader gives them, and it adds, divides and compares no money value itself; with one alias in `c.Aliases` it returns the bytes of `Build` with the alias ""; an alias listed twice is passed once; an alias named `household` is not read and is passed as `read_error`.
3. The family page has `scope` `household`, one scope `household` whose `accounts` list every alias of the book, an accounts block with one row per alias (a left-out row has the gap words and no cell), `overview.rows` with one row named by `familyNetName`, `lead_notes` with one line per left-out alias and one per alias counted from a day earlier than the newest session day, the accounts block with the title, meaning and six column heads of `zh_texts.go` (账户, 净值, 现金, 股票, 期权, 数据日), the net and cash cells in USD as `FamilyResult.Rows` gives them (a JPY account is shown converted, not in yen), the stock and options cells of every row the gap `not_built` with the words `familyCellDetail` ("见单个账户"), `overview.totals` with the one USD total cell, `overview.rows` named by `familyNetName` with the counted M and N, the line of the rate, day and source in `lead_notes` when `FxRate` is set, `CashShare` shown with the cash line, and the ten detail blocks as the gap `not_built` with the words of `familyDetailGap`; a reader error of the first alias or of the session days is an error of `BuildFamily`, and a reader error of any other alias is that alias passed as `read_error` (a left-out row and line, never an error); `data_times` has one `account` entry per counted account, `stale` is that of the oldest counted capture, `gaps` has one entry per left-out account and one per detail block, `run_status` is `partial`, and `notes` holds the fixed cash line (`familyCashNote`) and the statement line of each counted statement alias; a `Config` with no `Account`, `Ledger`, `Sessions`, `Ledgers`, `Figures` or `Family` gives the gap page `no_reader` as `Build` does.
4. `blocks.go` has `Views` and `LeadNotes` on the page struct with `json:"views,omitempty"` and `json:"lead_notes,omitempty"`; `builder.go` fills `Views` in `base` when `Config.Aliases` has two or more entries (the one `household` link and one per alias, `current` on the page's own), and leaves it empty otherwise, so a one-alias `Config` builds the same bytes as on main. The shared test `config` of `builder_test.go` has two aliases, so `testdata/holdings.golden.json` (a whole page) gains the key `views` and nothing else in it changes; `docs/contracts/pages.md` names `BuildFamily`, `Config.Family`, the one-alias rule and the new tests where it names `TestOneScopeNoPooling`, and the `home_household` `Config` fields of entry 5.
5. `zh_texts.go` (new) holds, as string constants and nothing else, `familyTitle` ("家庭合计"), `familySentence`, `familyNetName` (a format with the counted M and the total N), `viewFamily` ("全家"), the label prefix and suffix of `账户（` and `）`, the whose words "本人" and "其他账户", the cell gaps ("未采集到", "采集过旧", "缺少汇率", "读取失败"), `familyCashNote` ("现金分放在各账户，只担保本账户的卖出看跌；合计现金不是可共用的保证金"), the rate line format ("日元账户按 USDJPY 汇率 %s（日期 %s，来源 %s）折成美元"), `familyDetailGap`, `familyCellDetail`, the accounts block title, meaning and six heads, and the five line formats ("%s今天未采集到，合计不含", "%s今天未采集到，最近一次是 %s（超过 3 个交易日），合计不含", "%s的币种缺少美元汇率或暂不支持，合计不含", "%s用的是 %s 的采集，早于最新交易日，已计入合计", "%s读取失败，合计不含"); `TestZhTextsDoor` still passes.
6. `family_test.go` (new) has `TestFamilyPageSumsAllAliases`, `TestFamilyPageNamesLeftOutAccount` (a wife-like alias with no capture has a row with gap words and a `lead_notes` line, and is not in the net), `TestFamilyPageOlderDayIsCountedAndSaid`, `TestFamilyPageStaleIsLeftOut`, `TestFamilyPageValidatesAgainstSchema`, `TestViewsOnNamedAccountPage`, `TestOneAliasConfigHasNoViews`, `TestFamilyPageOtherAliasReadErrorIsNamed` (a non-first alias whose reader errors is a left-out row with reason `read_error` and the page builds), `TestFamilyPageFirstAliasErrorFails`, `TestFamilyPageStatesCountAndRate`, `TestFamilyPageCashNote`, `TestFamilyPageRepeatedAliasCountsOnce`, `TestFamilyPageAliasNamedHouseholdIsLeftOut`, `TestFamilyPageOneAliasIsNamedPage` (all new); the tests use a fake `FamilyFigure`; `builder_test.go` and `holdings_test.go` are not edited.
7. `builder.go` stays under 600 lines and `family.go`, `zh_texts.go`, `family_test.go` are each under 600; `purity_test.go` of the package still passes; no ceiling in `ratchet.json` changes.
Checked by CI: `go test ./internal/pages/...`.

### 5 home-household-family-pages

Module: pages. Size: M.
Order: after 2.
Files: `internal/pages/home_household/member.go`, `internal/pages/home_household/family.go` (new), `internal/pages/home_household/zh_texts.go` (new), `internal/pages/home_household/family_test.go` (new), `internal/pages/home_household/testdata/family.golden.json` (new)
Contracts: unchanged
Design: docs/design/household-family-total.md#Decisions
Acceptance:
1. `member.go` `Config` gains `Aliases []string`, `Fx bar.FxRateReader` and `Family blockapi.NewFamilyFigureFunc` (new fields); when `Aliases` has two or more entries `Build` calls the family build of `family.go`, and when it has none or one `Build` is the code of today with `Alias`, so the existing tests and `testdata/ok.golden.json` are not edited; the package comment no longer says "pools no account".
2. `family.go` (new) reads, for each alias, the newest capture, the capture before it and the month start as `capture` does, marks `no_data` and `stale_input` as the page does (3 session days) and `read_error` for a non-first alias whose reader fails (a first-alias error is an error of `Build` as today), passes the rate with its day and source, calls `Family`, and copies the figures into the four tiles (the cash tile from `CashShare`, so the card keeps its share meaning and divides nothing); it adds and compares no money value itself; a change tile that is a gap in the result is a gap of the card: the words name the account for `missing_input` and are the words of `cardDayMixed` for the day tile's `stale_input`.
3. The card has title kept, subtitle "全家合计 · 计入 M / N 个账户 · 最早数据日 D" from `Included`, the number of aliases and `OldestDay`, and `notes` with one line per left-out alias, one per alias counted from an earlier day, the rate line when `FxRate` is set and the cash line, the same words as entry 4; the schema is `household_summary` version 2 and every field is one the schema has.
4. `zh_texts.go` (new) holds the subtitle format, the five line formats, the rate line, the cash line and `cardDayMixed` as string constants and nothing else; `TestZhTextsDoor` still passes.
5. `family_test.go` (new) has `TestFamilyCardSumsNetAndCash`, `TestFamilyCardNamesLeftOutAccount`, `TestFamilyCardChangeGapNamesAccount`, `TestFamilyCardSubtitle`, `TestFamilyCardNoneCounted`, `TestFamilyCardCashShare`, `TestFamilyCardDayGapWhenDaysDiffer`, `TestFamilyCardOtherAliasReadError`, `TestFamilyCardOneAliasIsToday`, `TestFamilyCardNotesHaveCashLine` (all new); `testdata/family.golden.json` (new) is the card of two counted accounts and one left out and validates as `household_summary` version 2.
6. `member.go` stays under 600 lines; the three new files are under 600; `purity_test.go` still passes.
Checked by CI: `go test ./internal/pages/...`.

### 6 household-family-routes

Module: ot. Size: S.
Order: after 3, 4 and 5.
Files: `cmd/ot/household_site.go`, `cmd/ot/home_site.go`, `cmd/ot/household_site_test.go`, `cmd/ot/home_site_test.go`
Contracts: unchanged
Design: docs/design/household-family-total.md#Decisions
Acceptance:
1. `householdConfig` sets `Family: pagefig.NewFamily`; `buildHousehold` calls `household_page.BuildFamily` when the query `account` is empty (or is `household`) and `household_page.Build` with the alias when it is not (a book of one alias gives the page of today inside `BuildFamily`); `householdPageNamed` builds the family page; the file no longer reads `rd.aliasList()[0]` as the default.
2. `home_site.go` sets `Aliases: rd.aliasList()`, `Fx: rd.fx` and `Family: pagefig.NewFamily` on `home_household.Config`.
3. In `household_site_test.go` the changed tests are `TestHouseholdRouteReadsAliasesOfTheBook` (two aliases: the request without query is the family page and asks both aliases; the request with `account=<alias>` asks that one), `TestHouseholdJpyHoldingIsUsdWithStoredRate` and `TestHouseholdJpyHoldingWithoutFxStaysYenWithGap` (their helper `jpyHouseholdBody` requests `?account=main`, because the holdings blocks are gaps on the family page); `TestHouseholdRouteServesStoredAccount` keeps its text (its book has one alias, so the request without query is the page of today), `TestHouseholdRouteWithoutReaderIsGap` keeps `no_reader` (BuildFamily with no reader), `TestHouseholdPageNamedDemoIsGap` keeps its `no_reader` expectation and is not edited, and three tests are new: `TestHouseholdRouteDefaultIsFamily`, `TestHouseholdRouteNamedAliasKeepsSingleView`, `TestHouseholdRouteFamilyNamesAliasWithoutCapture`.
4. `home_site_test.go` has `TestHomeRouteHouseholdCardIsFamily` (new: a book of two aliases, the household member of the home page has the family subtitle with "计入 1 / 2 个账户" and a note for the alias with no capture); the other tests are not edited (their lists hold one alias, so the card is the card of today, and `TestHomeRouteServesFiveMembers` only checks that the member exists); `cmd/ot/accounts_test.go` is not touched (it tests `bookAliasList`, unchanged).
5. Both site files hold wiring only: no sum, no text, no condition on a figure.
Checked by CI: `go test ./cmd/ot/...`.

### 7 household-family-web

Module: web. Size: S.
Order: after 1.
Files: `web/src/pages/household/HouseholdPage.tsx`, `web/src/types/household_page.v1.types.ts`, `web/test/pages/household/householdFamily.test.tsx` (new), `web/fixtures/household-page/family.json` (new)
Contracts: unchanged
Design: docs/design/household-family-total.md#Decisions
Acceptance:
1. `household_page.v1.types.ts` has `views?` and `lead_notes?` as optional members of the page type, matching the schema of entry 1, nothing removed, and the comment that says nothing is "summed across scopes" says that the `household` scope is the family sum of the accounts of the book.
2. `HouseholdPage.tsx` prints `payload.views` as a row of links (`href` and `label` as sent, the `current` one marked and not a link) in the header, and `payload.lead_notes` as grey paragraphs right under the sentence, each only when present; no new visible string literal; nothing is counted, joined or sorted.
3. `family.json` (new) is a family page of two counted accounts and one left out, validates against `page.household_page.v1.json` (`TestFixturesValidate` reads the directory), and carries the Chinese texts of entry 4 as data.
4. `householdFamily.test.tsx` (new) has one test (new) that renders `family.json` and finds the three links with the current one unlinked, the lead note of the left-out account, the account table with three rows and the gap words in the left-out row, the "计入 2 / 3" overview row name and the cash note as data, and one test (new) that renders `ok.json` (no `views`, no `lead_notes`) and finds none of them and no error.
5. `householdPage.test.tsx` and the other fixtures are not edited; table head code and the other files of `web/src/pages/household/` are unchanged.
Checked by CI: `tsc`, `eslint`, `vitest`, the build.

## Left out

- A Chinese display name per alias ("太太账户" instead of "账户（wife）"): the book has only the owner's alias; a name field is a change of `internal/accounts` (an interface) and of `accounts.toml`, and is one optional field away. Until then the lines say "账户（<alias>）".
- Family detail blocks (holdings, movers, trend, stress, income, month change, cash after, obligations, concentration, allocation summed over accounts): each needs its own figure and its own pooling rule (an underlying held by two accounts, a trend with different capture days). The family page says so in each block (decision 5); each is a later design on the owner's word.
- Chinese words for the rest of the household page and the card (title "Household", tile labels, the single-account page): P7 restyle of those pages; this design adds only the family texts.
- Converting other currencies than USD and JPY, and a rate other than the stored USDJPY of the newest data day.
- Candidates, gates and any risk or sizing code reading the family figure (`FamilyFigure` is read by the two pages only; cash pooled across accounts is display, never a margin or put-cover input).
- A "pooled risk" page: `/risk` is untouched.
- Removing any old field; any change to the single-account page beyond the row of links.

## Adversarial

Inputs: the advisor read is `state/optdesk-drafts/designs/advisor-2026-11-19-household.md` and the adversarial read `state/optdesk-drafts/designs/adversarial-2026-11-19-household.md`. Both are answered here in one round.

Advisor (6 points):
1. No `## Adversarial`: adopted, this section.
2. FX rate undated and unsourced: adopted. `FamilyIn` carries `RateDay`, `RateSource`; `FamilyResult` carries `FxRate`, `FxDay`, `FxSource`; one `lead_notes` line and one card note say them whenever a JPY account is counted; conversion goes through the existing `rate` function of `pagefig` (entry 3, test `TestFamilyJpyNeedsRate`).
3. One alias read error fails the page: adopted. Decision 7b: a non-first alias error is a left-out row with reason `read_error`; only the first alias or the session days fail the build (entries 3, 4, 5, tests named).
4. Pooled cash: adopted. Fixed cash line `familyCashNote` on the page and in the card notes with tests (`TestFamilyPageCashNote`, `TestFamilyCardNotesHaveCashLine`); Left out says no candidate or gate code reads `FamilyFigure`; Decision 10.
5. Superseded text: adopted. "Design PR also edits" lists `builders-wave.md` lines 11 and 27 and the `household-figures.md` scope line; entry 7 edits the types comment. Uncertain: `household-figures.md` has no line that says "scope" in those words, so the design names the place (under `Revision:`) and the design PR author fixes the exact line.
6. M of N and the alias collision rule: adopted. "计入 M / N 个账户" on the page overview row and in the card subtitle; Decision 7c: the book already refuses a repeated alias or id, the page counts a repeated alias once, an alias named `household` is left out and `?account=household` is the family page, with tests.
Not checked by the advisor, now covered: `holdings.golden.json` with two aliases (entry 4 item 4 says it gains `views` only); `cmd/ot/accounts_test.go` (entry 6 item 4, untouched); `docs/contracts/pages.md` (entry 4 Files).

Adversarial (3 blocking, minors):
B1. Existing ot tests that break: adopted. Entry 6 item 3 lists `TestHouseholdRouteReadsAliasesOfTheBook`, `TestHouseholdJpyHoldingIsUsdWithStoredRate` and `TestHouseholdJpyHoldingWithoutFxStaysYenWithGap` (helper uses `?account=main`); `TestHouseholdRouteServesStoredAccount` stays unedited because its book has one alias and a one-alias book gives today's page.
B2. Card cash share: adopted. `FamilyResult.CashShare`; entry 5 copies it, divides nothing.
B3. Reader error fails the page: adopted, same as advisor 3 (the existing `TestSecondAccountFailureLeavesMainAlone` meaning is kept).
Minors: required properties `data_times`, `stale`, `run_status`, `gaps`, `notes` stated in What you see and entry 4 item 3: adopted. Card day tile mixing days: adopted as a gap `stale_input` when counted days differ (`TestFamilyDayGapWhenDaysDiffer`, `TestFamilyCardDayGapWhenDaysDiffer`); this loses the day tile whenever one account is older, accepted as honest. One-alias book: adopted, family path only for two or more aliases, today's page and card otherwise. `views` on named pages and `Build` with alias "": adopted, stated in What you see and entry 4 item 1. Serialisation: adopted, entries 3, 4, 5 after 2, 6 after 3, 4, 5, 7 after 1. "本人" wording: adopted, Decision 8 (family page Chinese, named page English until P7). `docs/contracts/pages.md` in entry 4 and `docs/contracts/compute.md` in entry 2: adopted. Zhdoor check: nothing to change.
Six questions: gap against goal closed by B2, B3 and the M of N count. Lock-in on key `household` is accepted and written in the superseded paragraph. Not adopted: the "simpler way" (skip the card, skip day and month figures): the owner asked for the family on the home card too, and the day and month tiles exist on the card today; not adopted for that reason. Not adopted: sharing the 3-day stale rule through one function (copied by both pages, as today; a shared rule is its own change).
