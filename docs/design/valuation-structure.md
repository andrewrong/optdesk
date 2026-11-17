# valuation structure: five insert-only tables, one function that gives the prices of a valuation, four pages to enter and read them

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-17

Source: the plan "optdesk 估值系统：底层数据结构与分步计划" (Claude Doc GvmvBALVmTrFhVioDZ3nB6), sections 本轮范围, 第一件：估值底层结构, 数据结构 groups 1, 2, 5, 6 and 录入页面, approved by the owner 2026-11-17 through optdesk-advisor; owner answers Q0 to Q6 of 2026-11-17 (see Decisions). Field inventory: `state/optdesk-drafts/designs/valuation/field-inventory-2026-11-16.md`. The owner's own numbers (his R table, margin table, valuations, accounts) are not in this file, in a fixture, in a test or in a migration; every number below is a made-up test number or a rule of the method, and the first parameters are entered on a page.

Goal: the owner opens a web page, enters the valuation of one company (V, predictability P, review type, version date are the only required values), and the system stores it and shows, computed on the spot, the fair price, the heavy point, the buy zone, the fair zone, the trim line, the clear line, the core line and the zone the close price is in. Nothing derived is stored. This is the first thing of the plan only (steps 2 to 10 are Left out).

## Today

- No table holds a valuation. `settings.underlying` (migration `0033_underlying.sql`) holds the names the owner follows with a hand-typed `buy_price`; the put candidates read it. It stays as it is.
- The old FCF yield band lives in `internal/compute/valuation` and shows on the companies page; it reads no V, P or R. It stays as it is.
- The pattern for a page that the owner edits exists: an insert-only settings table, the newest row per key wins, a store package that declares `Reader` and `Writer` (`internal/store/underlying`), the pg implementation in its own package, the pages read through `rd.*_r1` views as `ot_ro_pages`, the write role `ot_w_settings` (`ownerset.DSNEnvSettings`), the owner post behind `guardOwnerPost` (`cmd/ot/ownertoken.go`), a refusal re-rendered with 422 and a write answered 303 (`docs/design/price-lines.md`).
- The latest 10-year yield is the stored FRED series `DGS10` (`store.SeriesReader`). A close price is read in two steps: a symbol to an instrument id by the stock id reader (`StockIDs(ctx, names)`, opened for the market state page by `openMarketStateIDs`, `cmd/ot/market_state_site.go:36`, through `reportStockIDOpener`; the reader of the report is wired at `cmd/ot/report.go:170`), then the bars of that id by `bar.PriceBarReader.PriceBars(ctx, instrumentID, basis, from, to)` (`internal/store/bar/api.go:47`). A Hong Kong or Chinese A-share name has no price source and is not on `settings.underlying`.
- The owner posts are dispatched by hard-coded branches in `cmd/ot/site.go` (`ownerPostRoute` at line 389, `priceLinesRoute` at line 397); a post route with no branch is not served. The writer is opened through a field of `deps` in `cmd/ot/wire.go` (`openUnderlying`, lines 178 and 258). `wire.go` is 772 lines and `site.go` 672, both under the 800-line ceiling; this design adds a few lines to each and keeps them under 800.
- The migrations on origin/main end at 0050; `docs/design/data-health.md` entry 3 holds 0051. This design takes 0052.

## Decisions

1. Q0, review record built full (owner: as recommended). The table `valuation.review` has every field of group 2 of the plan: the header, the valuation part and the five scores with the reason of each. Required for a record that is not a draft: review type, version date, V, P; the draft may leave them empty. The rule is stated once, as a CHECK of the table (Decision 12): `status = 'draft'` or all four present. Every other column is nullable and unchecked this round. The checks of the plan's step 3 (trigger type, change memo, score reasons, lock of the multiple, adjustment caps, scenario odds) are not built; the columns for them are, so step 3 needs no second entry of each company. A column `source` (`owner` or `program`) says who made V; this round the writer writes only `owner`, so the program V of the plan's step 9 needs no migration.
2. Q1, no rounding inside. The function works in decimal text (`shopspring/decimal`, already a direct dependency of go.mod) with no rounding; its results are decimal text at full precision. The only rounding is `RoundText(value, places)`, a method of the same interface, which the page builders call when they write a display text (2 places for a price, an R and a yield). No page, test or job rounds a figure any other way.
3. Q2, Hong Kong and China names. A profile with R market `HKCN` can be entered and gets every price line; it gets no zone (`Zone` empty and `Flags` null) whatever the close price, because there is no price source for it. A US or JP name gets the zone when a close price is passed. A name with no stored price gets prices and no zone (`ZoneNote` `no_close`).
4. Q3, each line judged alone. Every flag of `Flags` compares the close price with its own line and with nothing else: no sorting of the lines, no repair when the trim line lies above the clear line (R near 1 with a large trim premium gives that), so both flags can be true at once. `Zone` is a label picked from the flags by one fixed precedence, `core`, `clear`, `trim`, `fair`, `heavy`, `buy`, else `between`; the flags are the truth, the label is a convenience for the table. A close price equal to a line counts as having reached it: `>=` for the trim, clear and core lines, `<=` for the heavy point, inside the closed range from `FairLow` to `FairHigh` for the fair zone, and `InBuy` is `close <= BuyHigh` (the buy zone has no floor: `BuyLow` is only the price of the second batch of buying, printed as a line, never a lower limit of the flag). So a close between the heavy point and `BuyLow` is `heavy` or `buy` by the precedence and never `between`; `between` is only a close above `BuyHigh` and below `FairLow` (or any close above `BuyHigh` that no higher flag takes). The page prints every true flag, not only the label.
5. Q4, R is proposed, the owner clicks. `ProposeR(rows, yield)` gives the three R values of the bucket of a 10-year yield; the market parameters page shows the proposal when it differs from the R of the newest R version and has one button "Update R". The click inserts one R version row; nothing else ever writes an R version, no job proposes or saves one.
6. Q5, a step above 0.05 warns and never blocks. `RStepWarnings(previous, next)` returns one text per market whose R moves by more than `RStepWarn` ("0.05"); the page prints it before the click and again on the saved R version; the row is saved either way.
7. Q6, a yield on a bucket edge falls in the lower bucket. Bucket edges are the upper bound of a bucket and inclusive: a yield equal to a bucket's `YieldUpTo` takes that bucket (4.5 is in the 4.0 to 4.5 bucket, 4.0 is in the bucket that ends at 4.0). The top bucket has no upper bound.
8. Insert-only, and a review can be superseded. No table has an update or a delete. A change is another row; the newest row per key wins (profile: `company_key`; review: `review_id`; R version and the two parameter tables: the newest version by `effective_date` then `set_at`). A draft is edited by inserting another row of the same `review_id` with `status` `draft`. A row with `status` `effective` closes its `review_id` against further drafts or effective rows (a unique partial index on `review_id` where `status = 'effective'`). A mistake in an effective review is undone by one more row of the same `review_id` with `status` `void` and a `void_reason` (one void row per `review_id`, a second unique partial index; the writer refuses a void row for a `review_id` with no effective row). The current valuation of a company is the effective row, with no void row of its `review_id`, whose review type is not `backfill`, with the latest `effective_at` (set by the database at insert, not typed), ties by `id`; `version_date` is shown and never decides which review is current, so a mistyped version date cannot hide later reviews, and a `backfill` entered late cannot replace the current one. The owner fixes a wrong V or date by voiding the row and entering a new review; the history keeps both rows and says which is void. A draft and a void review are read by the review pages and by nothing that computes.
9. The two parameter tables are versioned data, one row per bucket; the rows of one version are inserted in one transaction so they share `effective_date` and `set_at`. The pages always compute with the newest version of each table and the newest R version, that is with today's parameters, also for an older review; the function takes the tables as arguments, so an old result can be recomputed with the tables of its day by a caller that passes them, but no page takes an as-of date (Left out). The first version of each table is entered on the market parameters page (owner: the first parameters are entered on a page). The plan says in one place that this entry exists and in another that the parameter tables are read-only; this design builds the add-a-version form (one post carrying the whole table as text) and no edit or delete of a row.
10. The company key is a text the owner types on the entry form for a name that is not on `settings.underlying` (a Hong Kong or A-share name), and the field `Key` of `underlying.Row`, `market:SYMBOL` such as `US:XXX`, for a name that is; the profile row stores the key, the display name, the symbol used to find a close price (empty for none), the currency and the R market column (default from the market of the key: US to US, JP to JP, anything else must be chosen). The form refuses a typed key that equals a `Key` on `settings.underlying`, so one company has one key. The rows of the valuation table are the union of every current `settings.underlying` row and every profile: a name on the list that has no profile is a row with a profile defaulted from the list row (key, symbol, market US to US and JP to JP, currency, pool state `in_pool`), valued false and "not entered", and it counts in `unvalued_count` like a profile without a review; its entry link opens the company page and form of its key. The first save of a review (draft or effective) inserts the profile row when the key has none, from the same defaults (or from the typed fields for a free key), in the same request; no separate profile step is needed. This design does not add a name to `settings.underlying`.
11. The AI switch and its expiry condition are stored on the R version (the expiry text is required when the switch is on) and have no effect on any price this round, because the AI chain weight of a company is a field of the plan's step 8.
12. Checks before a review becomes effective, this round only. One rule decides what is required, and it is in the table: a CHECK `status = 'draft' OR (type, version_date, v, p are all not null)`, so a draft can be saved with any of the four empty and an effective or void row cannot (a void row carries a copy of the four values of the effective row it voids). `Check` repeats the rule in words for the page, before the insert: review type, version date, V and P present; V above 0; P between 1 and 5 in steps of 0.5; and the buy zone upper line below the fair zone lower line (`Check` of the interface). The last check depends only on the margin row of P (the margin above the fair half width), not on V or R, so a wrong margin table blocks every effective review of that P until a new table version is entered; the page says which row fails. A failing record stays a draft and the page lists each failed check with its text; nothing is refused. No score other than P is checked.
13. Page order of this design (the build order of the plan's step 1.5): the market parameters page, the entry form, the company valuation page, the valuation table. The plan numbers them the other way round (table first); the content of each page is compared with the plan's section 录入页面 in the pull request of its entry. The four routes are served by two page documents: `page.valuation_params_page.v1` (the market parameters page) and `page.valuation_page.v1` with a field `view` (`table`, `company` or `form`), because the table, company and form views share the company, the lines and the form shapes; a schema per view would repeat them and the pages are read by one binary.
14. The pages compute nothing beyond calling the interface: every number on a page is a text the builder wrote from the function's result, and the web prints it. This includes the positions of the lines on the price axis, which `Calculator.Band` returns (decision 17), never the builder, and the change percentage of a version history row, which `Calculator.ChangePercent` returns.
15. The entry form's live preview follows the plan ("immediately"): the page sends the typed V and P by a debounced `fetch` (400 ms after the last key) to the preview post (`action=preview`), which writes nothing and answers the `valuation_page` document of view `form` as JSON; the web prints `lines` of that document and computes nothing. The form also has a "Calculate" button that posts the same action, for a browser without script.
16. The write goes to the repository's own PostgreSQL through the existing role `ot_w_settings`; no file or service outside the repository is written, so no entry names `docs/writes.md`. The migration entry holds the auto-deploy (migrations hold it). The Down part of the migration never drops owner data: as migration `0044` does for a check it cannot undo, it refuses (raises) when any of the five tables holds a row, and drops the views and tables only when all are empty, so Down serves the CI check on an empty database and is not a way to lose data.
17. One chain, one place. The chain of a company (profile, R market column, newest R version, margin table, the effective review, the close price, the lines, the band) is written once, as the exported function `Resolve` of the package `internal/pages/valuation_page`, used by the three views and by no other code in this design. Step 2 of the plan (the put gate reads the prices) will use it or move it behind an interface in its own design; this design does not publish it through `internal/pages/api.go` (Left out). The pure part, choosing the R of the market column and the margin row, is inside `Calculator.Lines`, so a second consumer only has to read the store snapshot (`Reader.Current`) and pass plain strings.
18. Close price: `valuation_page` takes a `CloseReader` interface of its own (`StockIDs(ctx, names)` and `PriceBars(ctx, id, basis, from, to)` of the two existing readers); `cmd/ot` fills it with the stock id reader opened by `reportStockIDOpener(a)` (as `openMarketStateIDs`, `cmd/ot/market_state_site.go:36`, does) and `siteReaders.bars`. The profile's symbol is the name given to `StockIDs`; the newest `trades` bar at or before today is the close. No id found or no bar gives `ZoneNote` `no_close`.
19. Lines stays one flat struct this round. Steps 3 and 8 (quality total, position cap, core position, AI chain weight, effective buy price) will add fields through their own designs, each an interface change with a Design line; adding fields is compatible, and the zone label values are not renamed by this design.

## Modules

`docs/design/modules.md`: row `store`, the interface cell gains `internal/store/valuation` (declarations only); row `compute`, the interface cell gains `internal/compute/valuelinesapi` (declarations only). No other row changes and no row is added; this design's commit makes the two edits. Paths of the entries: store (`internal/store/**`, `migrations/**`, `docs/contracts/store/**`; entries 2 and 5), compute (`internal/compute/**`, `docs/contracts/compute/**`; entries 1 and 4), contracts (entry 3), pages (`internal/pages/**`, `docs/contracts/pages.md`; entry 6), ot (`cmd/**`, `docs/contracts/ot/**`; entry 7) and web (entry 8). Tests: each interface package has an `api_test.go` with the contract from both sides.

Import edges, checked against the `depends` column of `docs/design/modules.md` (store: `contracts gaphistory`; compute: `contracts`; pages: `contracts store compute`; ot: every module): `internal/store/valuation` imports `internal/contracts` only (`contracts.Day`); `internal/compute/valuelinesapi` and `internal/compute/valuelines` import `contracts` at most and no store package; the table types are declared twice, once in each interface package, as plain text structs, and only the pages copy one into the other, so there is no store to compute edge; `internal/store/valuationpg` imports `internal/store/valuation` and `contracts`; `internal/pages/valuation_params_page` and `internal/pages/valuation_page` import `internal/store/valuation`, `internal/compute/valuelinesapi`, `internal/store` and `internal/store/bar` (the close reader types) and `contracts`; `cmd/ot` imports all of them and `internal/compute/valuelines`. No page imports `valuelines` or `valuationpg`; `internal/compute/valuation` (the old band) is not imported or edited.

## Interface

Entry 1 (module compute, new package `internal/compute/valuelinesapi`, `api.go` declarations only, decimal text everywhere, no float, no clock, no I/O; plain strings so that no other package's type is needed):

    const (
        MarketUS   = "US"
        MarketJP   = "JP"
        MarketHKCN = "HKCN"
    )
    const RStepWarn = "0.05"

    // RRow is one bucket of the R table: the bucket ends at YieldUpTo, inclusive; "" is the top bucket (no upper bound).
    type RRow struct{ YieldUpTo, US, JP, HKCN string }
    // MarginRow is one bucket of the safety margin table: the row with the greatest PFrom at or below P is used.
    type MarginRow struct{ PFrom, M, W, H, S string } // margin, fair half width, heavy discount, trim premium, as fractions
    type RValues struct{ US, JP, HKCN string }

    type Input struct {
        V, P   string
        Market string      // Market*: the column of R to use
        R      RValues     // the newest R version
        Margin []MarginRow // ascending PFrom
        Close  string      // "" when no close price
    }
    // InBuy is close <= BuyHigh; BuyLow is the second batch price, not a floor.
    type Flags struct{ AtHeavy, InBuy, InFair, AtTrim, AtClear, AtCore bool }
    type Lines struct {
        C                                         string // fair price V*R
        R                                         string // the R of the market column that was used
        Heavy, BuyLow, BuyHigh, FairLow, FairHigh string
        Trim, Clear, Core                         string // C*(1+s), V*1.2, V*1.5
        M, W, H, S                                string // the margin row used
        Zone                                      string // "", core, clear, trim, fair, heavy, buy, between
        ZoneNote                                  string // "", no_close, no_zone_market
        Flags                                     *Flags // nil when Zone is empty
    }
    // Band is the price axis of the company page: the positions are fractions of the axis from 0 to 1, as text.
    type Mark struct{ Name, Value, Pos string }
    type Band struct {
        Min, Max string
        Marks    []Mark // every line of Lines, and the close when given
    }
    type Problem struct{ Field, Text string }
    // CheckInput is the review as typed: every field text, "" when empty.
    type CheckInput struct {
        Type, VersionDate, V, P string
        Margin                  []MarginRow
    }

    // Calculator is the one place that computes these numbers; every page calls it.
    type Calculator interface {
        // Lines: V, P and the R of the market column to Lines. Heavy C*(1-h); BuyHigh C*(1-m); BuyLow C*(1-m-0.05);
        // FairLow and FairHigh C*(1-w) and C*(1+w). ErrBadInput when V is not above 0, P is outside 1..5, or Margin has no row for P.
        Lines(in Input) (Lines, error)
        // Band: the axis for Lines and the close; the lines and the close sit inside Min and Max.
        Band(l Lines, close string) Band
        // ChangePercent: (to-from)/from*100 as decimal text; ErrBadInput when from is 0.
        ChangePercent(from, to string) (string, error)
        // ProposeR: the R values of the bucket of the 10-year yield (a yield on an edge takes the lower bucket).
        ProposeR(rows []RRow, yield string) (RValues, error)
        // RStepWarnings: one text per market whose R moves by more than RStepWarn.
        RStepWarnings(previous, next RValues) []string
        // Check: the checks of Decision 12 for a review that is to become effective.
        Check(in CheckInput) []Problem
        // RoundText: round a decimal text half away from zero to places digits; the only rounding of the system.
        RoundText(value string, places int) string
    }
    var ErrBadInput error

Entry 2 (module store, new package `internal/store/valuation`, `api.go` declarations only; the pg implementation is `internal/store/valuationpg`). Decimal columns are decimal text, dates are `contracts.Day`. The table rows are declared here as plain text structs; the compute package's `RRow` and `MarginRow` are not imported.

    type Profile struct { // table valuation.company_profile
        ID int64; CompanyKey, Name, Symbol, Currency string
        RMarket  string // US, JP, HKCN
        Pool     string // in_pool, watch, removed
        PoolDate contracts.Day
        Note     string
        SetAt    time.Time
    }
    type Adjustment struct{ Percent, Reason string }
    type Scenario struct{ Label, Probability, EPS, Multiple string } // label: bear, base, bull, tail
    type Review struct { // table valuation.review
        ID int64; ReviewID, CompanyKey string
        Type        string // first, post_earnings, event, backfill
        VersionDate contracts.Day
        FiscalPeriodEnd, ReportDate contracts.Day // zero when unset
        Status      string // draft, effective, void
        Source      string // owner (program is reserved for the plan's step 9)
        V, P        string
        Trigger     string // a, b, c, d or ""
        EarningsBase, BaseBasis, ForwardFactor, LockedMultiple string
        MethodResults []string     // up to three
        Adjustments   []Adjustment
        Scenarios     []Scenario
        ChangeMemo, Reaction string
        Moat, Understand, Management, Financial string
        Reasons    map[string]string // per dimension, key p, moat, understand, management, financial
        ReportText string
        VoidReason string            // set on a void row
        EnteredAt  time.Time
        EffectiveAt *time.Time       // set by the database on the effective row
    }
    type RVersion struct { // table valuation.r_version
        ID int64; EffectiveDate contracts.Day
        YieldUsed        string
        RUS, RJP, RHKCN  string
        AISwitch bool; AIExpiry, Note string
        SetAt time.Time
    }
    type RTableRow struct{ YieldUpTo, US, JP, HKCN string }      // tables valuation.r_table
    type MarginTableRow struct{ PFrom, M, W, H, S string }       // table valuation.margin_table
    type RTableVersion struct { EffectiveDate contracts.Day; SetAt time.Time; Rows []RTableRow }
    type MarginTableVersion struct { EffectiveDate contracts.Day; SetAt time.Time; Rows []MarginTableRow }
    type Entry struct { Profile Profile; Current *Review } // Current is nil when the company has no current valuation
    // State is everything the pages need, read in one call.
    type State struct {
        Entries []Entry              // newest profile row per company_key, ascending key
        RVersion *RVersion           // newest, nil when none
        RTable *RTableVersion; Margin *MarginTableVersion // newest, nil when none
    }
    type Reader interface {
        Current(ctx context.Context) (State, error)
        Reviews(ctx context.Context, companyKey string) ([]Review, error) // every row of a company, newest first
        RVersions(ctx context.Context) ([]RVersion, error)                // newest first
    }
    type Writer interface { // every call is one transaction, one insert (a table version: all its rows)
        PutProfile(ctx context.Context, p Profile) error
        PutReview(ctx context.Context, r Review) error // ErrClosed: the ReviewID has an effective row and r is not its void row; ErrNotEffective: a void row for a ReviewID with no effective row
        PutRVersion(ctx context.Context, v RVersion) error
        PutRTable(ctx context.Context, t RTableVersion) error
        PutMarginTable(ctx context.Context, t MarginTableVersion) error
    }
    var ErrClosed, ErrNotEffective error
    type OpenFunc func(dsn string) (Reader, Writer, io.Closer, error)

Entry 3 (module contracts): two page documents, `additionalProperties: false` at every level, numbers as text, the owner form as in `page.price_lines_page.v1` (`post_path` and the token fields, null without owner edit):
- `page.valuation_params_page.v1`: `latest_yield` (text, day), `current_r` (the newest R version: date, yield used, three R, AI switch, expiry text), `proposal` (three R, `step_warnings`, null when equal to the current), the two parameter tables as read-only rows (each with its `effective_date`), `r_history`, `gaps`, `error`, `form`.
- `page.valuation_page.v1`: `view` (`table`, `company`, `form`), `rows` (view `table`: one per name of the union of every current `settings.underlying` row and every profile, with `valued` false, the text "not entered" and an entry link when no current review: company, V, version date, R, C, heavy, buy zone, fair zone, trim, clear, core, close, zone, flags, P, hand-typed buy price, `status`; null in other views), `unvalued_count`, `company` (views `company` and `form`: the profile fields), `current` (view `company`: the fields of the current review), `lines` (C, the lines, the four rates used, `zone`, `zone_note`, `flags`; views `company` and `form`, in view `form` it is the preview), `band` (`min`, `max` and for each mark `name`, `value`, `pos`, all text), `close` (text, day), `history` (view `company`: one row per review row: date, V, V from and to as text, `change_percent`, trigger type, memo, status, void reason, and `detail`: every other stored field of the row with the report text, shown on expand), `required` (view `form`: the four field names), `problems` (each `field`, `text`), `error`, `saved`, `gaps`, `form`.

## Issues

Build order: the cut is by the numbers below; a draft names an entry as `Design: docs/design/valuation-structure.md#<n>`. Every entry compiles and its tests pass against main. Entries 1, 2 and 3 first, in parallel; 4 after 1; 5 after 2; 6 after 1, 2 and 3; 7 after 4, 5 and 6; 8 after 3 and 7. Entry 5 is the only migration; it holds the auto-deploy. Every entry carries its own `Contracts:` line; entry 3 alone changes a file under `contracts/schemas`. Every entry's Files are in one module (entry 3 is the contracts module with its generated web types, as the data-health design cut it). Entries 1 and 2 edit nothing of `docs/design/modules.md`; this design's pull request does. Test runs are in `## Checked by CI`. All test numbers are made up.

### 1 valuation-compute-api (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/valuelinesapi/api.go`, `internal/compute/valuelinesapi/api_test.go`, `docs/contracts/compute/valuelines.md`
Contracts: unchanged
Design: docs/design/valuation-structure.md#Interface
Acceptance:
1. `api.go` declares the constants, `RRow`, `MarginRow`, `RValues`, `Input`, `Flags`, `Lines`, `Mark`, `Band`, `Problem`, `CheckInput`, `Calculator` and `ErrBadInput` as in the Interface section, with docs and no function body, and is the only non-test `.go` file of its package; it imports no package of this repository other than `contracts` (none is needed); `internal/compute/valuelinesapi` is in the interface cell of the row `compute` of `docs/design/modules.md` (written by this design's pull request, so `grep -c 'valuelinesapi' docs/design/modules.md` prints 1).
2. `api_test.go` has `TestCalculatorContractFake` (new: a fake `Calculator` is called from a caller that reads `Lines`, `Band`, `ChangePercent`, `ProposeR`, `RStepWarnings`, `Check` and `RoundText`), `TestRStepWarnIs005` (new) and `TestMarketNames` (new: US, JP and HKCN).
3. `docs/contracts/compute/valuelines.md` states in one paragraph each: no rounding inside; each line judged alone and the fixed zone precedence; the edge rule of a yield (lower bucket); a Hong Kong or China name gets no zone; the positions of `Band` are fractions of the axis; `Calculator` is the only place that computes these numbers.

### 2 valuation-store-api (interface)

Module: store. Size: S.
Order: first.
Files: `internal/store/valuation/api.go`, `internal/store/valuation/api_test.go`, `docs/contracts/store/valuation.md`
Contracts: unchanged
Design: docs/design/valuation-structure.md#Interface
Acceptance:
1. `api.go` declares `Profile`, `Review`, `Adjustment`, `Scenario`, `RVersion`, `RTableRow`, `MarginTableRow`, `RTableVersion`, `MarginTableVersion`, `Entry`, `State`, `Reader`, `Writer`, `ErrClosed`, `ErrNotEffective` and `OpenFunc` as in the Interface section, with docs and no function body, and is the only non-test `.go` file of its package; its only import of this repository is `internal/contracts` (`grep -c 'internal/compute' internal/store/valuation/api.go` prints 0); `internal/store/valuation` is in the interface cell of the row `store` of `docs/design/modules.md` (written by this design's pull request).
2. `api_test.go` has `TestReaderWriterContractFake` (new: a fake keeps rows in memory; a caller writes a profile, a draft, an effective review and a void row and reads them back through `Current` and `Reviews`), `TestVoidedReviewIsNotCurrent` (new, in the fake the test defines: a void row of the effective review leaves `Entry.Current` nil) and `TestEveryMethodReturnsNonNil` (new: empty answers are non-nil slices).
3. `docs/contracts/store/valuation.md` lists the five tables, the key of each, the "newest row wins" rule, the current valuation rule of Decision 8 (latest `effective_at`, no void row, not `backfill`), and states that no row is updated or deleted.

### 3 valuation-page-contracts (interface)

Module: contracts. Size: M.
Order: first.
Files: `contracts/schemas/page.valuation_params_page.v1.json`, `contracts/schemas/page.valuation_page.v1.json`, `web/src/generated/valuation_params_page.v1.ts`, `web/src/generated/valuation_page.v1.ts`, `contracts/fixtures/page.valuation_params_page.v1.ok.json`, `contracts/fixtures/page.valuation_page.v1.table.json`, `contracts/fixtures/page.valuation_page.v1.company.json`, `contracts/fixtures/page.valuation_page.v1.hkcn.json`, `contracts/fixtures/page.valuation_page.v1.form.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/valuation_pages_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.valuation_params_page v1, page.valuation_page v1 (both new)
Design: docs/design/valuation-structure.md#Interface
Acceptance:
1. Each schema has the fields of Entry 3 of the Interface section and no others, `additionalProperties: false` at every level, every figure typed as text, `view` an enum of `table`, `company`, `form`, and `form`, `proposal`, `rows`, `current`, `lines`, `band`, `history` nullable; the generated types are produced from the schemas and not edited by hand.
2. The fixtures use made-up companies and numbers and are labelled invented in `MANIFEST.synthetic-v1.json`; `hkcn.json` has `lines` with prices, `zone` empty and `flags` null; the table fixture has one valued row, one profile with no review and one list name with no profile ("not entered"); the company fixture has a history with one void row and a `detail`.
3. `valuation_pages_v1_test.go` has `TestValuationFixturesValidate` (new: every fixture passes its schema), `TestValuationRejectsUnknownField` (new: an extra key on a `lines` object fails), `TestValuationFiguresAreText` (new: a number where a text is required fails) and `TestValuationViewEnum` (new: `view: "chart"` fails).
4. `docs/contracts/page-schemas.md` lists the two pages and states that a change is `v2`.

### 4 valuation-lines

Module: compute. Size: M.
Order: after 1.
Files: `internal/compute/valuelines/calc.go`, `internal/compute/valuelines/rtable.go`, `internal/compute/valuelines/check.go`, `internal/compute/valuelines/calc_test.go`, `internal/compute/valuelines/rtable_test.go`, `internal/compute/valuelines/check_test.go`
Contracts: unchanged
Design: docs/design/valuation-structure.md#Decisions (Decisions 2, 3, 4, 7, 12, 14)
Acceptance:
1. `calc.go` implements `valuelinesapi.Calculator` (`var _ valuelinesapi.Calculator = ...`) with `shopspring/decimal` and no rounding: `TestLinesNoRounding` (new: made-up V 100, P 3, R 0.9 in the market column, margin row m 0.18, w 0.07, h 0.30, s 0.22 give C 90, BuyHigh 73.8, BuyLow 69.3, Heavy 63, FairLow 83.7, FairHigh 96.3, Trim 109.8, Clear 120, Core 150; a V of 100.07 keeps its exact products), and no call to a rounding function of `decimal` appears outside `RoundText` (`grep -rn 'Round\|Truncate\|Floor\|Ceil' internal/compute/valuelines/*.go` prints only `RoundText` lines and tests).
2. `TestZoneFlagsJudgedAlone` (new: R 1 with s large so Trim is above Clear gives both `AtTrim` and `AtClear` true for a close above both, and only `AtClear` true for a close between them; the label follows the precedence of Decision 4), `TestLineBoundaries` (new: a close equal to each line takes the side Decision 4 names; with the lines of item 1, a close of 66 (between the heavy point 63 and `BuyLow` 69.3) gives `InBuy` true and `Zone` `buy`, a close of 63 gives `AtHeavy` and `Zone` `heavy`, a close of 50 gives `AtHeavy` and `InBuy` and `Zone` `heavy`, and none of the three gives `between`; a close of 80 (above `BuyHigh` 73.8, below `FairLow` 83.7) gives `between`), `TestHKCNHasNoZone` (new: market HKCN with a close gives prices, `Zone` "" and `Flags` nil), `TestNoCloseNoZone` (new: `ZoneNote` `no_close`), `TestBadInput` (new: V 0, V negative, P 0.5, P 5.5 and a margin table with no row for P give `ErrBadInput`), `TestMarginRowChosenByP` (new: P on the edge of two rows takes the row whose `PFrom` is at or below P) and `TestMarketColumnChosen` (new: the same R values with market JP and US use their own column).
3. `TestBandPositions` (new: for the lines of item 1 and a close of 80, `Band` returns `Min` below the lowest of the lines and the close and `Max` above the highest, every `Pos` is between 0 and 1, the marks are in the order of the values, and a trim above clear keeps both marks) and `TestChangePercent` (new: from 100 to 88 gives -12, from 0 gives `ErrBadInput`).
4. `rtable.go` has `ProposeR` and `RStepWarnings`: `TestYieldOnEdgeTakesLowerBucket` (new: a made-up table with edges 4.0 and 4.5 gives the 4.0 to 4.5 bucket for 4.5 and the first bucket for 4.0, and the top bucket for 4.51), `TestStepWarning` (new: 0.06 warns, 0.05 does not, per market, text names the market) and `TestRoundTextHalfAwayFromZero` (new: 2.345 to 2 places gives 2.35, -2.345 gives -2.35, the input text is not changed).
5. `check.go` has `Check`: `TestCheckRequiredAndRanges` (new: missing type, version date, V or P each give one `Problem` with its field; V 0 fails; P 3.3 fails; P 3.5 passes), `TestCheckBuyBelowFair` (new: a margin row whose margin is not above its fair half width gives one `Problem` naming the row) and `TestCheckIgnoresOptionalFields` (new: a review with only the four required values passes).
6. Each new file is under 600 lines; the package imports no package of this repository other than `valuelinesapi` and `contracts`; `internal/compute/valuation` is unchanged.

### 5 valuation-tables

Module: store. Size: M.
Order: after 2.
Files: `migrations/0052_valuation.sql`, `internal/store/valuationpg/store.go`, `internal/store/valuationpg/read.go`, `internal/store/valuationpg/write.go`, `internal/store/valuationpg/write_test.go`, `internal/store/valuationpg/read_test.go`, `internal/store/valuationpg/valuation_pg_test.go`, `docs/contracts/store/valuation.md`
Contracts: unchanged
Design: docs/design/valuation-structure.md#Decisions (Decisions 1, 8, 9, 10, 16)
Acceptance:
1. The migration `0052_valuation.sql` (new file; `0050_fx_rate_source.sql` is the last on main and `0051_coverage_view.sql` is held by data-health entry 3) has Up and Down, creates the schema `valuation` (owner `ot_owner`, `USAGE` to `ot_w_settings`) and in it the five tables `company_profile`, `review`, `r_version`, `r_table`, `margin_table` with `ALTER ... OWNER TO ot_owner`, `GRANT INSERT, SELECT` to `ot_w_settings` and no `UPDATE` or `DELETE` grant to anyone, the views `rd.valuation_company_profile_r1`, `rd.valuation_review_r1`, `rd.valuation_r_version_r1`, `rd.valuation_r_table_r1` and `rd.valuation_margin_table_r1` with `SELECT` to `ot_ro_pages` and `ot_w_settings`, a unique partial index on `valuation.review (review_id) WHERE status = 'effective'` and another where `status = 'void'`, a check that `status` is `draft`, `effective` or `void` and that `source` is `owner` or `program`, and `effective_at` set by `now()` on an effective row; `review` has the CHECK of Decision 12 (`status = 'draft'` or `type`, `version_date`, `v` and `p` all not null); every column of `review` other than `review_id`, `company_key`, `status` and `source` is nullable, so a draft can be saved with the four empty; the migration uses no feature newer than PostgreSQL 15 (the pair of `ci-go.yml`, see Checked by CI); no row is inserted and no number of the owner appears in the file.
2. The Down part raises an exception, as `0044_instrument_company.sql` does, when any of the five tables holds a row, and drops the views, tables and schema only when all five are empty (`TestValuationDown` below).
3. `write.go` implements `valuation.Writer` (`var _ valuation.Writer = ...`) one transaction per call; a table version is inserted in one transaction so its rows share `set_at`; `PutReview` of an effective `ReviewID` returns `valuation.ErrClosed` unless the row is its void row, and a void row of a `ReviewID` with no effective row returns `valuation.ErrNotEffective`; `write_test.go` has `TestPutRTableOneTransaction` (new: a recording `database/sql` driver defined in the test file sees one begin and one commit for a three-row table), `TestPutReviewAfterEffectiveIsClosed` (new), `TestVoidNeedsEffective` (new) and `TestNoUpdateOrDeleteStatements` (new: the SQL strings the writer sends, which the test lists from a variable of `write.go`, contain no `UPDATE` and no `DELETE`).
4. `read.go` implements `valuation.Reader` over the views only (`grep -c 'FROM valuation\.' internal/store/valuationpg/read.go` prints 0), opens with the pages DSN as `OpenGapReader` does; `read_test.go` has `TestCurrentValuationRule` (new: two effective reviews give the one with the later `effective_at` whatever their version dates, equal times the later `id`; a draft is never current; an effective review with a void row is not current and the earlier effective review of the company is), `TestBackfillNeverCurrent` (new), `TestNewestTableVersionWins` (new) and `TestProfilesNewestPerKey` (new).
5. `valuation_pg_test.go` carries the build tag `pg` and has `TestValuationTablesAsRoles` (new: `INSERT` works as `ot_w_settings`, `UPDATE` and `DELETE` are denied on every table, `SELECT` on the views works as `ot_ro_pages` and on the tables is denied), `TestRequiredColumnsUnlessDraft` (new: a draft row with null `v` inserts; an effective row and a void row with null `type`, `version_date`, `v` or `p` are each refused by the CHECK), `TestEffectiveAndVoidPartialIndexes` (new: a second effective row and a second void row of one `review_id` are each refused), `TestValuationDown` (new: Down on empty tables removes them; Down with one profile row raises and leaves the tables).
6. Each new file is under 600 lines; `internal/store/api.go` and `docs/design/ratchet.json` are unchanged.

### 6 valuation-pages

Module: pages. Size: L.
Order: after 1, 2 and 3.
Files: `internal/pages/valuation_params_page/page.go`, `internal/pages/valuation_params_page/builder.go`, `internal/pages/valuation_params_page/builder_test.go`, `internal/pages/valuation_page/page.go`, `internal/pages/valuation_page/resolve.go`, `internal/pages/valuation_page/table.go`, `internal/pages/valuation_page/company.go`, `internal/pages/valuation_page/form.go`, `internal/pages/valuation_page/resolve_test.go`, `internal/pages/valuation_page/table_test.go`, `internal/pages/valuation_page/company_test.go`, `internal/pages/valuation_page/form_test.go`, `internal/pages/valuation_page/purity_test.go`, `docs/contracts/pages.md`
Contracts: unchanged
Design: docs/design/valuation-structure.md#Decisions (Decisions 3, 4, 5, 6, 8, 9, 12, 13, 14, 15, 17, 18)
Acceptance:
1. `valuation_params_page.Build` takes the stored versions, the newest `DGS10` point, the stored tables, the `valuelinesapi.Calculator` and the owner form, and emits a document that validates against `page.valuation_params_page.v1`: `TestProposalShownWhenDifferent` (new: made-up tables and a yield whose bucket differs from the newest R version give a `proposal`; the same R gives null), `TestStepWarningsInDocument` (new: a proposal that moves one market by 0.10 carries the warning text of `RStepWarnings`), `TestNoTablesGivesGap` (new: no stored R table gives the gap `no_r_table` and no proposal, never a failed page) and `TestFormNullWithoutOwnerEdit` (new).
2. `resolve.go` has the exported `Resolve`, the one place of the chain of Decision 17: it takes a `valuation.Entry`, the `valuation.State` tables, the `CloseReader` and the calculator and gives the `Lines`, the `Band` and the close; it copies the table rows of the store types into the plain `valuelinesapi` types and does nothing else with them: `TestResolveChain` (new: a fake store state and a fake `Calculator` that records its `Input` show the market column of the profile, the newest R version, the margin rows and the close of the newest stored bar reach `Lines` unchanged), `TestResolveCloseFromSymbol` (new: the symbol is given to `StockIDs`, the id to `PriceBars` with basis `trades`; no id or no bar gives `ZoneNote` `no_close` from the calculator) and `TestResolveNoCurrentReview` (new: an entry with no current review gives no lines).
3. The three views of `valuation_page` validate against `page.valuation_page.v1`. Table: `TestUnvaluedCompaniesListed` (new: a profile with no current review is a row with `valued` false and an entry link), `TestNamesWithoutProfileListed` (new: two made-up `settings.underlying` rows, one with a profile and one without, give two rows; the second has a defaulted profile, "not entered", an entry link with its key and counts in `unvalued_count`; a profile with no list row is listed too), `TestRowsUseOneResolvePerCompany` (new: one `Lines` call per valued company), `TestHandBuyPriceNextToBuyHigh` (new: the hand-typed buy price of `settings.underlying` is printed beside the buy-zone upper line and nothing is computed from the two) and `TestUnvaluedCount` (new: counts profiles without a current review and names without a profile). The table builder takes the current `underlying.Row` list (`underlying.Reader.Current`) beside `valuation.State`. Company: `TestCompanyLinesFromCalculator` (new: a fake `Calculator` answer is printed unchanged apart from `RoundText`), `TestHKCNNoZone` (new), `TestBandFromCalculator` (new: `band` marks equal the `Band` the fake returns, text for text), `TestHistoryRows` (new: three review rows, one void, give date, V from and to, `change_percent` as the fake `ChangePercent` returns it, trigger type, memo, status, void reason and a `detail` with the report text, newest first) and `TestNoCurrentReviewShowsDrafts` (new: a company with only drafts shows them in `history` and no `lines`). Form: `TestPreviewUsesCalculator` (new: a fake `Calculator` records one `Lines` call for the typed V and P, and `lines` equals its answer, every figure written through `RoundText`), `TestProblemsListedNotRefused` (new: a draft with V 0 gives `problems` and no error), `TestOnlyFourRequired` (new: `required` is exactly review type, version date, V and P) and `TestMarketDefaultFromKey` (new: the key `US:XXX` gives R market US, a free key asks for a choice).
4. Both packages import no `database/sql`, `os`, `net/http`, store implementation or clock (`purity_test.go` and a purity test in `builder_test.go`); no builder multiplies, divides, rounds or positions a figure itself (`TestNoArithmeticInBuilders` (new): the files contain no `decimal` import, no `strconv.ParseFloat` and no `math` import).
5. `docs/contracts/pages.md` lists the two packages in one paragraph each and names `Resolve` as the one place of the chain; each new file is under 600 lines.

### 7 valuation-wire-posts

Module: ot. Size: L.
Order: after 4, 5 and 6. It edits `cmd/ot/pagereg_test.go` and `cmd/ot/pagelist_test.go`, so it goes after any earlier page entry that edits them and before the next one; the two lists are edited in this one entry only. It adds a few lines to `cmd/ot/site.go` (672 lines) and `cmd/ot/wire.go` (772 lines), which stay under 800 (`price-lines.md` entry 2 is the precedent).
Files: `cmd/ot/valuation_site.go`, `cmd/ot/valuation_post.go`, `cmd/ot/valuation_post_review.go`, `cmd/ot/valuation_site_test.go`, `cmd/ot/valuation_post_test.go`, `cmd/ot/valuation_post_review_test.go`, `cmd/ot/site.go`, `cmd/ot/wire.go`, `cmd/ot/pagereg_test.go`, `cmd/ot/pagelist_test.go`, `docs/contracts/ot/pages/valuation.md`
Contracts: unchanged
Design: docs/design/valuation-structure.md#Decisions (Decisions 1, 5, 8, 9, 12, 15, 16, 18)
Acceptance:
1. `valuation_site.go` registers four families, named `valuation_params`, `valuation_table`, `valuation_company` and `valuation_form`, on the routes `valuation-params`, `valuation`, `valuation-company` and `valuation-form` (each with a `name`, an `order` after the highest one pinned in `pagereg_test.go` and a `build`, as `market_state_site.go` does), opens `valuation.Reader` with the pages DSN through `siteOpeners`, reads the latest `DGS10` with the series reader, reads the current `settings.underlying` rows with the reader that `site.go` opens (`openUnderlyingReader`, line 270), fills the `CloseReader` with the stock id reader opened through `reportStockIDOpener(a)` (as `openMarketStateIDs` does) and `siteReaders.bars`, builds the owner form when owner edit is on, and passes `valuelines.New()` as the `Calculator`; with no pages DSN each page answers with a gap, never a 500: `TestValuationRoutes` (new: each route answers 200 with a document that validates, over fake readers), `TestCompanyKeyFromQuery` (new: `?company=US:XXX` selects the company, an unknown key answers an empty page with a gap) and `TestPageListHasValuation` (new: the pinned lists in `pagereg_test.go` and `pagelist_test.go` each gain the four names, in the order of `registerPage` `order`).
2. `site.go` gains the branches that route the POST paths `valuation-post` and `valuation-params-post` to the handlers, only when `rd.edit != nil` (the form of `priceLinesRoute`, `cmd/ot/site.go:397`); `wire.go` gains the field `openValuation valuation.OpenFunc` of `deps` and its assignment `valuationpg.Open` (as `openUnderlying`, `wire.go:178` and `258`); each file gets only those lines and stays under 800: `TestValuationPostRoutesNeedOwnerEdit` (new: a POST to either path with owner edit off is not served by the handler; with it on, it reaches the handler), `TestPostsNeedGuard` (new: a post without the token writes nothing, nothing is written before `guardOwnerPost` passes) and `TestOpenValuationWired` (new: the real deps have a non-nil `openValuation`).
3. The form `valuation-form` posts `action` `preview` (writes nothing; answers the page document as JSON when the request asks for JSON, else the page), `draft` (inserts a review with status `draft`), `effective` (runs `Calculator.Check`; with no `Problem` inserts a review with status `effective`, otherwise inserts it as `draft` and answers 422 with the problems listed), `void` (inserts a void row with the reason; the reason is required) and `profile` (inserts a profile row, refusing a typed key equal to a `Key` on `settings.underlying`): `TestPreviewWritesNothing` (new), `TestFailedCheckStaysDraft` (new: an `effective` post with V 0, and one with V empty, each stores a draft row and answers the problems; a `draft` post with V and P empty stores a draft row and no problem), `TestFirstSaveInsertsProfile` (new: a draft post for a key with no profile calls `PutProfile` with the defaults of Decision 10 before `PutReview`, once; a second save calls `PutProfile` no more), `TestEffectiveWritesOnce` (new: a valid record gives one `PutReview` with status `effective` and `Source` `owner`), `TestReviewIdKeptAcrossDrafts` (new: two saves of one record carry one `ReviewID`; a save after `effective` answers 422 from `ErrClosed`), `TestVoidNeedsReason` (new) and `TestProfileKeyRefusedWhenOnList` (new).
4. The form `valuation-params` posts `action` `r_table` and `margin_table` (each inserts one table version from the posted whole table as text, one bucket per line; the effective date is required and an R bucket order that is not ascending is refused with the line named), `accept_r` (inserts one R version from the proposal recomputed on the server with `ProposeR` from the posted yield and date; a step above `RStepWarn` is saved and warned) and `r_version` (inserts an R version with the AI switch and its expiry text, the text required when the switch is on): `TestAcceptRRecomputesOnServer` (new: the posted R values are ignored and the stored ones equal `ProposeR` of the yield), `TestStepAboveLimitSavesWithWarning` (new), `TestAISwitchNeedsExpiry` (new: switch on with an empty expiry text is refused) and `TestTableRowsMustAscend` (new). A refusal re-renders the page with 422 and the text in `error`; a write answers 303 with `saved=1`.
5. `docs/contracts/ot/pages/valuation.md` says that a valuation the owner already holds in his own table is entered as review type `first` (`backfill` is only for an earlier version of a company that has a later one), and lists the four routes, the two post paths, the actions, the 422 refusals, the 303 answers and the query parameters; no file other than the eleven named is edited; `cmd/ot/jobconsole_post.go` is unchanged; each new file is under 600 lines.

### 8 valuation-web

Module: web. Size: L.
Order: after 3 and 7 (the nav link must not point at a route that is not served).
Files: `web/src/pages/valuation_params/ValuationParamsPage.tsx`, `web/src/pages/valuation_params/ParamTables.tsx`, `web/src/pages/valuation/ValuationPage.tsx`, `web/src/pages/valuation/ValuationTable.tsx`, `web/src/pages/valuation/CompanyView.tsx`, `web/src/pages/valuation/PriceBand.tsx`, `web/src/pages/valuation/ReviewHistory.tsx`, `web/src/pages/valuation/ReviewForm.tsx`, `web/src/types/valuation_params_page.ts`, `web/src/types/valuation_page.ts`, `web/src/components/ui/select.tsx`, `web/src/components/ui/SOURCES.md`, `web/src/shell/TopNav.tsx`, `web/test/shell/TopNav.test.tsx`, `web/test/pages/valuation_params/valuation_params.test.tsx`, `web/test/pages/valuation/valuation.test.tsx`
Contracts: unchanged
Design: docs/design/valuation-structure.md#Decisions (Decisions 4, 5, 6, 9, 13, 14, 15)
Acceptance:
1. 按 frontend-stack：用 shadcn `Card` 和 `Alert` 做 the page frames and the `gaps`, `error` and `step_warnings` lines; 按 frontend-stack：用 shadcn `Table` 做 the read-only R table, margin table and R history; 按 frontend-stack：用 `@tanstack/react-table` 做 the valuation table and the review history (the row expands to `detail` and the report text); 按 frontend-stack：用 shadcn `Button` (type submit) 做 "Update R", "Calculate" and every save button; 按 frontend-stack：用 shadcn `Input`, `Label` 和 `Select` 做 the fields of both forms (plain HTML forms posting to `form.post_path`, no form library); 按 frontend-stack：用 shadcn `Badge` 做 the zone and flag marks; 按 frontend-stack：用 `lucide-react` `Plus` 做 the entry link of an unvalued row. `select.tsx` is added with `shadcn add select` (it uses `radix-ui`, already in `frontend-whitelist.json`), and `components/ui/SOURCES.md` gains its line; no package is added to `package.json`.
2. 按 frontend-stack：用 `recharts` 的 `ReferenceLine` 和 `ReferenceArea`，经 `web/src/charts/Chart.tsx` 做 the price band, as the other pages do; the axis domain is `band.min` and `band.max` and the position of every mark is `band.marks[].pos`, read as given: the chart computes none (`TestNoLogicInWeb` (new) in each test file: no arithmetic on a document value is exported from the page folders and no mark position is computed).
3. Each of `ValuationParamsPage.tsx` and `ValuationPage.tsx` exports `registration = { schema, page }` with `schema` equal to its schema name (`page.valuation_params_page.v1`, `page.valuation_page.v1`), as `web/src/pages/advice/AdvicePage.tsx:65` does, so the glob of `web/src/shell/pages.tsx` finds it: `TestRegistrationsExported` (new: both modules export `registration` with those schema names).
4. The params page prints the proposal sentence with its three R values and its warnings next to the "Update R" form (hidden when `proposal` is null or `form` is null), the add-a-version forms for the two tables (one text area each) and for the R version: `TestProposalAndWarnings` (new) and `TestFormsHiddenWithoutOwnerEdit` (new).
5. `ValuationPage` switches on `view`. Form: the four required fields first, marked required, the optional groups (scores with reasons, origin of V, report text) collapsed below; typing V or P sends one debounced `fetch` (400 ms) to `form.post_path` with `action=preview` and Accept JSON and prints `lines` as given (`TestPreviewIsDebounced` (new, with the fake timers of vitest: three key presses in 100 ms give one request; the printed lines are the answer text)); "Save draft", "Make effective" and "Void" post `draft`, `effective` and `void`; `problems` are listed per field; `TestFourFieldsRequired` (new) and `TestProblemsListed` (new). Company: every true flag is printed, not only `zone`, and for an empty `zone` the `zone_note` text and the prices are printed (`TestEveryTrueFlagPrinted` (new: both trim and clear true print both), `TestHKCNNoZoneText` (new)); the history prints date, V from and to, `change_percent`, trigger type, memo, status and void reason per row and expands to `detail` (`TestHistoryColumnsAndExpand` (new)). Table: one row per company, an entry button on an unvalued row, a column for the hand-typed buy price next to the buy-zone upper line (`TestUnvaluedRowHasEntryLink` (new), `TestHandPriceColumn` (new)).
6. `TopNav.tsx` gains one entry "Valuation" with route `valuation`, following the pinned list in `TopNav.test.tsx`, which is edited in the same entry; the other pages are reached by links inside the pages. The generated types in `web/src/generated/` are imported and not edited; every new file is under 600 lines.

## Checked by CI

`go test ./...` (entries 1, 2, 4, 6, 7; the `pg` tests of entry 5 run only in GitHub CI), `go run ./scripts/arch` and go-arch-lint (the import edges of Modules), the schema test of entry 3, `ci-web` (`tsc`, `eslint`, `vitest`, the build; entry 8), and the migration check of `ci-go.yml` on `timescale/timescaledb:2.19.0-pg15` (the production pair; the migration is written for PostgreSQL 15). After deploy the window renders the four pages and compares each with the plan's section 录入页面 by screenshot (owner rule of 2026-11-14 and the plan's step 1.5); the screenshot check is not an acceptance item. The plan's step 1.6 (the owner enters the first R and margin parameters on the market parameters page, enters three companies and compares them with his own table) is the owner's, after the last entry lands.

## Left out

- Steps 2 to 10 of the plan: no change to the put candidates or to `settings.underlying.buy_price` (the hand-typed price is only shown beside the new buy line), no checks before publishing beyond Decision 12, no quality total, no position cap (S5) or core position, no breakpoints, no verification list (v3), no program-computed V (the `source` column is there, only `owner` is written), no model, no expiry reminder, no portfolio limits, no behaviour record.
- The effective buy price of the plan (the lower of the buy-zone upper line and the hand-typed price): not computed; the table page prints the two side by side as the plan's page 1 asks, and the number is the plan's step 2.
- An as-of date on a page: every page uses today's parameters, also for an old review, and a history row shows V but not the price lines of its day (Decision 9).
- `Lines` as a flat struct stays; the later steps change the interface through their own designs (Decision 19). `Resolve` is not published through `internal/pages/api.go` (Decision 17).
- The old FCF yield band of `internal/compute/valuation` and its figures on the companies page: untouched; two valuations exist side by side until a later design stops one.
- Data collection steps 2.2 to 2.4 (single-quarter values, quarterly table, coverage page) and 2.5: a later separate design (step 2.1, the SEC concepts, is its own issue). Japanese financials this round: not built (owner Q15).
- The AI switch has no effect on R (Decision 11); the AI chain weight is not stored.
- Profile fields of group 1 that belong to later steps (cyclical flag, earnings type, pricing-power type, percentile indicator, themes, AI weight, country, fiscal-year end month, manual next report date): not built.
- An edit or delete of a row of a parameter table, and a delete of any row: not built (insert-only, a new version replaces).
- A close price for a Hong Kong or A-share name and adding such a name to `settings.underlying`: not built; these names have prices and no zone.
- A new write outside the repository: none, so no `docs/writes.md` entry.

## Doubts

- The plan numbers the pages table, company page, form, parameters; the build order here is the dispatcher's (parameters first). The content is the same.
- The plan says the parameter tables are read-only and also that the first version is entered on the market parameters page; Decision 9 builds the add-a-version form only.
- The plan lists two checks as step 1 (V above 0 and scores in range; buy upper below fair lower) and also says all checks wait for step 3; Decision 12 builds V above 0, P in range and the buy-below-fair check, and no other score check.
- "The five prices" is not listed in the plan; this design gives C, the heavy point, the buy zone, the fair zone, the trim line, the clear line and the core line, all of which the page prints.
- The clear and core lines are V times 1.2 and V times 1.5 in the plan, and the buy zone lower line is C times (1 - m - 0.05); the 1.2, the 1.5 and the 0.05 buy-zone width are constants of the method in the function, not table rows. If the owner changes one of them, it is a code change of `valuelines`.
- The inclusive or exclusive side of a close price on a line is not stated in the plan; Decision 4 chooses it.
- The preview post answers JSON when the request asks for JSON; whether the site already serves a page document that way for a post is not checked here and entry 7 item 3 must settle it with the existing handlers.
- `ot_w_settings` is reused; a separate role would need `internal/store/setup`, which is not in the files of any entry.
- A `backfill` review is never current (Decision 8); the plan says only that it is a historical entry without the "against the previous" check. A valuation the owner already holds is entered as `first`, not `backfill`.

## Adversarial

The full answer of the adversarial review is `state/optdesk-drafts/designs/valuation/adversarial-2026-11-17.md` (the scratchpad copy of the coordinator). The six points, in its order, and the decisions taken:

1. Gap against the goal. Adopted. The history now has the change percentage (`Calculator.ChangePercent`), the trigger type, an expandable `detail` with the report text (the old `TestHistoryChangeText` and its "no percentage" rule are gone, replaced by `TestHistoryRows` and `TestChangePercent`); no existing test of main bans a percentage on a page, since these pages are new. The live preview is a debounced fetch to the preview post (Decision 15). The path from a symbol to a close price is named (Decision 18, and Today: `cmd/ot/market_state_site.go:36`, `cmd/ot/report.go:170`, `internal/store/bar/api.go:47`). The effective buy price is in Left out; the duplicate-key risk is closed by refusing a typed key equal to a `Key` on `settings.underlying` (Decision 10).
2. A review cannot be undone. Adopted, in the smaller form: current is the latest `effective_at`, not `version_date`, with a void row (a new row status, no sixth table) and `backfill` never current (Decision 8). The margin-table failure of the buy-below-fair check is named (Decision 12). Today's parameters for every page, as-of date left out and named in Left out (Decision 9).
3. A simpler way. Adopted in part. Entries 6+7 (pages) and 8+9 (ot) are merged, the two web entries are merged, and the schemas are cut from four to two because the three views share the company, lines and form shapes (Decision 13); eleven entries are now eight. The table types are declared in the store package and plain strings in the compute package (Interface), which removes the store to compute edge. The two table forms are one post each carrying the whole table as text, no bucket parser beyond the order check. Not adopted: dropping `valuelinesapi`, which stays the compute interface; the history from/to stays (it is the plan's column).
4. Lock-in. Partly adopted. A `source` column for V (Decision 1) and one place for the chain, `Resolve` (Decision 17), are in. Not adopted: a richer `Lines` or a published resolve interface; `Lines` stays flat this round and the later steps change the interface through their own designs (Decision 19). The migration Down refuses when it would drop rows (Decision 16, as `0044`).
5. The false claims. Adopted. The store to compute edge is removed and every edge is checked against the `depends` column (Modules). `cmd/ot/site.go` and `cmd/ot/wire.go` are in the Files of entry 7 with the post branches and the `deps` field, the acceptance items that forbade them are gone, and both stay under 800 lines. The pinned lists: the four families are named and ordered, so `pagereg_test.go` and `pagelist_test.go` are edited in entry 7. `underlying.Row.Key` is a field and is named so. The shadcn `Select` is added with `shadcn add select` and recorded in `SOURCES.md`; the band goes through `web/src/charts/Chart.tsx`.
6. Likeliest slip. Adopted. Entries 8 and 9 are one entry with the missing files; the arch check names the edges; pages entries no longer share `docs/contracts/pages.md` with a parallel entry; the web entry names the `registration` export; `TestNoUpdateOrDeleteStatements` scans a list of the writer's SQL strings, not comments; the recording driver of `TestPutRTableOneTransaction` is defined in the test file; the band positions come from `Calculator.Band` (Decision 14). The out-of-order migration number is noted: setup migrate allows it, so 0052 landing before 0051 is not blocked.

## Advisor round 2026-11-17

Four blocking points, all fixed in one round: (1) the buy flag is `close <= BuyHigh` with no floor and the precedence is `core`, `clear`, `trim`, `fair`, `heavy`, `buy`, else `between` (Decision 4, Interface, `TestLineBoundaries`); (2) the valuation table lists every current `settings.underlying` name united with the profiles, and the first save of a review inserts a missing profile (Decision 10, entries 3, 6 and 7); (3) one rule for the four required values, a table CHECK that exempts drafts (Decision 12, entry 5, `TestFailedCheckStaysDraft`, `TestRequiredColumnsUnlessDraft`); (4) the preview text is the field `lines` of view `form` (entries 6 and 8). Also: an existing valuation is entered as `first`; the CI sentence follows `ci-go.yml` (TimescaleDB 2.19.0 on PostgreSQL 15); the 0.05 buy-zone width is named in Doubts.
