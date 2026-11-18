# candidates page v2: the page options/candidates shows the short puts a cash-secured put seller reads, as a short list grouped by underlying, 8 columns with Delta, and counts what the filter left out

Author: the optdesk designer (draft)
Owner approved: the mockup `candidates-v2-mockup.html` with Delta added and the grouping below (2026-11-18T14:44Z, asks 5008 and 5015); this text is pending
Date: 2026-11-18

Source: the owner's goal of 2026-11-18 (a cash-secured put seller: the list shows puts with the absolute delta from 0.15 to 0.30, the strike 5 to 15 percent below spot, 21 to 45 days to expiry, a bid, and an open interest that is present and not low; grouped by underlying, groups by their best annualized yield, rows inside a group by annualized yield; at most 8 columns; "filtering is a must", today 2343 rows are unreadable); the mockup `candidates-v2-mockup.html` and its screenshot `mockup-1440.png`; the adversarial read `adversarial-raw.md` beside this file (answered in `## Adversarial`); the merged designs `docs/design/candidates-tiers.md`, `docs/design/option-candidates.md`, `docs/design/candidates-evidence.md` and `docs/design/ui-english.md` (decision 1).

Checked on origin/main (ded5e555, 2026-11-18): `internal/compute/optioncand/optioncand.go` keeps a put row when its days to expiry are inside `min_dte` to `max_dte` and nothing else (the window is `internal/jobs/optioncandidates/job.toml`, 21 and 45, read only by the job; it is not a setting, the settings keys of the same names move nothing); the quote gate (`put.go`) returns the first blocking reason (`no_quote`, `no_bid`, `no_ask`, `small_bid_size`, `wide_spread`, `low_open_interest`) and sets the status, so all rows of the chain are stored (2343 on 2026-11-18); an open interest of 0 is only the mark `no_open_interest` (0 means not captured). A stored row (`rd.option_candidate_r1`) carries `strike`, `spot`, `delta`, `iv`, `bid_eod`, `ask`, `buy_price`, `annualized_yield` (the fraction bid over strike times 365 over the days, 6 decimals, `put.go`), `gates`, `marks` and no open-interest column. The page already reads every row (`readKind`), calls `optiontierapi.Tierer.Judge` once (`tiers.go`) whose verdict has `PerContract`, `BreakEven` and `MarginPct`; its `BelowSpotPct` is the break-even below spot (`internal/compute/optiontier/figures.go`) and its `YieldPct` is the yield of the time value only, so this design uses neither. `format.go` of the page already has the text rounding helpers `fixedText`, `percentText` and `moneyText`. The calendar table holds 2026-11-18 as a session, so a missing calendar row is not the cause of `stale_chain` (Decision 13).

## What you see

The first block of the page is one list, above the tier view, which stays below it, closed. The numbers are those of the mockup (33 underlyings of the stored run, 99 rows before the Delta filter). Labels, chips, counts and the dropped line are English; the hints (tooltips) are Chinese (`docs/design/ui-english.md` decision 1).

    Put candidates
    Run day 2026-11-18   Scanned 33 underlyings / 2,343 puts   Shown N rows, M underlyings
    (caveat, one sentence, Chinese risk text)
    [Expiry 21-45 d] [Strike below spot 5-15%] [|Delta| 0.15-0.30] [Max per underlying 3] [Bid present] [Open interest present]
    Not shown K: No bid a; No open interest b; No spot c; OTM below 5% d; OTM above 15% e; No delta f; Delta out of band g; Over 3 per underlying h
      STRIKE   EXPIRY      OTM %   DELTA   PREMIUM (BID)     ANNUALIZED   BUY PRICE      LIQUIDITY
    MU  spot $686.97                                           3 rows, best 87.5%
      $645     12-11 24d   6.1%    0.22    $37.10 $3,710     87.5%       No buy price
      ...
    > All 2,343 puts (tier view and stored tables)   (closed)

In the mockup the Delta digits are dashes because the page payload has no delta yet.

## Goal

The owner, who sells cash-secured puts, opens options/candidates and sees at most a few dozen readable rows, grouped by underlying, best annualized yield first, with the figures he decides on (strike, expiry, how far out of the money, delta, premium, annualized yield, the buy price and where it comes from, whether the quote is thin), and a count of what was left out and why.

## Today

2343 put rows of 33 underlyings are shown in tier tables; the payload has no delta, no mid and no spot field of its own (spot is in the company strip); every row carries `stale_chain`. The only stored filter is the days-to-expiry window.

## Decisions

1. The first view is a list `short_put_list` written above the tier view. The tier view (`short_put_tiers`) and the three stored tables are kept unchanged and, when the list is present, move under one closed section "All 2,343 puts" (the number is the count of stored put rows); a page without the list renders as today; nothing is deleted in this step (Left out).
2. The list has eight columns, in this order, English labels, Chinese hints: Strike, Expiry (date with days to expiry under it), OTM % (how far the strike is below spot), Delta (absolute value of the put delta, two decimals), Premium (bid) (the bid, and the money per contract under it), Annualized (the stored annualized yield, bid over strike times 365 over the days), Buy price (with its source tag), Liquidity. The underlying and its spot are the header of its group, not a column.
3. The rows are grouped by underlying. Groups are ordered by the best stored annualized yield of the group, highest first; rows inside a group by their stored annualized yield, highest first. A tie is broken by ticker, then by strike, then by row key, so the order is the same on every build. The figure is the stored `annualized_yield` of the row, not the tier verdict's yield.
4. A row is shown only when it passes the rule of Decisions 5 to 9; the rule is one compute package `optionshortlist` (declared in `optionshortlistapi`), called once per build with the rows already read. The page shows the kept rows and writes no condition of its own; the web page does not filter, sort or count.
5. Where the filter lives: in the compute package, at read time over the stored rows, not in `optioncand.Evaluate`. Reason: the stored rows are also read by the tier view, the counts and the strategy shadow, and the owner needs the count of what was left out by reason, which needs the rows.
6. The bounds are constants of `optionshortlist`, each with a comment that names this design: strike below spot 5 to 15 percent, absolute delta 0.15 to 0.30, at most 3 rows per underlying. They are not rule parameters: no key is added to `ruleparam`, the settings form is unchanged, and the package does not read `put_delta_max`. The tier view keeps its own `put_delta_max` (owner setting, default 0.3); the list does not follow it. To change a bound is a code change until the owner has used the list and asks for a setting (Left out).
7. Distance rule: OTM % is (spot - strike) / spot, computed by `optionshortlist` on exact rationals from the stored `Strike` and `Spot` texts of the row (not the tier verdict's `BelowSpotPct`, which is the break-even). A row below 5 percent (in the money included) is `otm_low`, above 15 percent `otm_high`; a row with no readable spot or strike is `no_spot`.
8. Delta rule: the absolute value of the stored put delta must be from 0.15 to 0.30. A row whose delta is empty cannot be judged and is dropped with the reason `no_delta`, counted in the dropped line as "No delta" (the stored delta is the capture's text; how many rows have none is an open measurement, Left out).
9. Quote rule, computed from the row's own fields and all its marks and reasons, not from the first reason only: a row is dropped when its bid is empty or not above 0 or its quote reason is `no_quote`, `no_bid`, `no_ask` or `small_bid_size` (`no_bid`); when any mark is `no_open_interest` or its quote reason is `low_open_interest` (`no_open_interest`); or when its marks hold `unreadable_gates` or its bid or annualized text cannot be read (`bad_row`). A row whose only quote reason is `wide_spread` is kept and carries the badge "Wide spread" with a Chinese gloss ("买卖价差大：bid 偏低，实际可挂在中间价附近成交，别按 bid 成交"). Limit: the stored quote gate holds one reason, so a row that is both wide and below the minimum open interest shows `wide_spread` only and is kept; only the mark `no_open_interest` (open interest 0) is seen behind it (Left out). A delayed quote is kept without a badge (the caveat of Decision 12 covers it).
10. Cap: at most 3 rows per underlying, the highest stored annualized yield first, the rest `over_cap`. This is an addition of the approved mockup to the owner's words of 2026-11-18, which name grouping and order only; the page says so in the hint of the chip "Max per underlying" and counts the rows it hides.
11. The count line: "Not shown K" with one item per reason in one fixed precedence order, which is also the order of the checks: `bad_row`, `no_bid`, `no_open_interest`, `no_spot`, `otm_low`, `otm_high`, `no_delta`, `delta_out`, `over_cap`. A row counts under the first code that applies in that order, so the items add up to K and K + shown = scanned. An item with 0 rows is not written. The decision to drop is made from all of a row's fields; the order only chooses the label.
12. One caveat line for the whole page, not a mark per row: when stored rows carry the events reason `stale_chain`, the line says how many rows and that the quote times must be checked at the broker; with none it is not written. It is the events reason, not the page's group `Stale` flag, and does not repeat it.
13. Cause of `stale_chain` on every row: the chain freshness (`optioncand/events.go chainFresh`) compares the run day and the chain day against the session list the job reads (`optioncandidates/input.go sessionDays`, trimmed by `trimSessionDays`). Checked: the calendar table has 2026-11-18 as a session, so a missing calendar row is not the cause. A chain of the previous session was marked stale until #2883 (merged 2026-11-18 09:25Z, "a chain of the previous session is fresh") and rows stored by a run before that deploy keep the mark; whether today's rows are such rows is not read from code (it needs `computed_at` of the stored rows). This design therefore adds a test-only issue of the three cases (chain on the run day, one session before, two sessions before), which changes no source file, and prints "Run day", not a chain day, on the list. After the issue merges the window re-runs the job for 2026-11-18 and reads `computed_at`; that is a window action, not an acceptance item.
14. The Buy price column shows the buy price with its source tag and, under it, the break-even and its margin to the buy price (the tier verdict figures `BreakEven` and `MarginPct`). The buy price has a source slot now: the row carries a value and a source code (`buy_source`, a `Badge` with code, label and Chinese gloss); today the only code is `hand` (the price the owner set), returned by one function `buySourceOf` of the page. A later design makes the price come from the valuation's buy-zone top, else the hand price, else the automatic SEC FCF band; it adds codes to that function and its texts with no schema change. The link to `/plan/price-lines` shows only when no source gives a price, and its text is "No buy price", which does not say whose price is missing. The list does not judge fit; the tier view keeps that (its grey reasons `no_buy_price` and `above_buy_price` change with the later design, not here).
15. All figures of the list come from the stored row text, the tier verdict and the `optionshortlist` result; the page formats text (rounding the digits it shows, with `format.go`) and the web page shows it. No filter, sort or count in `option_candidates_page` beyond placing rows in the order the compute package returns, and none in `web`.
16. A page built without the shortlist reader (nil) has no `short_put_list` (the property is omitted, so the page bytes and the golden files of today do not change) and renders as today; no row is lost from the tier view or the stored tables by this design.
17. Days to expiry: the rows exist only inside `min_dte` to `max_dte` of `job.toml` (21 and 45); the mockup said 20 to 50. The list shows the window it has: the chip reads the smallest and largest days of the scanned rows (`DTEMin`, `DTEMax` of the result) and its hint says plainly that the window is `job.toml` of the job and not the settings page. This design does not change the job window.

## Modules

`docs/design/modules.md`: the interface cell of the compute row gains one package, `internal/compute/optionshortlistapi`; no row is added (the amended file is `modules-amended.md` beside this design, one line changed). New packages `optionshortlistapi` (interface) and `optionshortlist` (implementation) are under the glob `internal/compute/**` of the compute row; `ruleparam` is not touched. Paths by the first row whose glob matches: compute `internal/compute/optionshortlistapi/**`, `internal/compute/optionshortlist/**`, `docs/contracts/compute/optionshortlist.md`; contracts `contracts/schemas/**`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/**`, `docs/contracts/page-schemas.md`; pages `internal/pages/option_candidates_page/**`, `docs/contracts/pages.md`; ot `cmd/ot/**`, `docs/contracts/ot/pages/option_candidates_page.md`; web `web/**`; jobs `internal/jobs/optioncandidates/**`. Imports against the `depends` column: pages imports `optionshortlistapi` and `optiontierapi` (compute), not `optionshortlist`; compute imports `contracts` only; ot imports `optionshortlist`; web imports nothing new. Shared files: `zh_texts.go` and `builder.go` of the page are edited by this design in one entry only; any other design that edits them is ordered after or before it by the pull request that lands first, and a conflict is reported in the pull request description.

## Interface

`internal/compute/optionshortlistapi/api.go` (new, declarations only; decimal text, no float, no clock, no I/O; an empty text is "no value"):

    type Bounds struct {
        OTMMinPct, OTMMaxPct string // "5" and "15", echo of the constants
        DeltaMin, DeltaMax   string // "0.15" and "0.30"
        PerUnderlying        int    // 3
    }
    type Row struct {
        Key, Company, Ticker string
        Strike, Spot         string // the stored texts; the rule computes the distance
        Delta, Bid           string // the stored texts; the rule takes the absolute delta
        Annualized           string // the stored annualized yield (fraction text), the ordering figure
        DTE                  string
        QuoteReason, EventsReason string // first non-pass reason of the stored gates
        Marks                []string
    }
    type Input struct { Rows []Row }
    type Drop struct { Code string; N int }       // Code in the fixed order of Decision 11
    type Placed struct { Key, OTM, AbsDelta, Liquidity string } // OTM and AbsDelta decimal text; Liquidity "" or "wide_spread"
    type Group struct { Company, Best string; Rows []Placed }   // Best is the stored Annualized of the best row
    type Result struct {
        Groups []Group; Dropped []Drop; Scanned, Shown, StaleChain int
        DTEMin, DTEMax string // smallest and largest days of the scanned rows
        Bounds Bounds
    }
    type Shortlister interface { Select(in Input) (Result, error) } // an unreadable number in one row drops that row as bad_row, no_bid or no_spot by its field; any other error is returned

No registry change: `ruleparam` and the settings form are untouched.

Page schema `page.option_candidates_page.v1`, additive and optional property `short_put_list` (absent, null or `ShortList`), `additionalProperties: false`, texts only, no JSON number:

    ShortList { run_day: Text, counts: Text, caveat: Text|null, filters: [Filter{id, label, value, hint}],
                dropped: Text|null, dropped_items: [DroppedItem{code, label, count, hint}], columns: [TierColumn],
                groups: [ShortGroup], folded_title: Text }
    ShortGroup { ticker: Text, spot: Text, summary: Text, rows: [ShortRow] }
    ShortRow { key: Text, strike: Pair, expiry: Pair, otm: Pair, delta: Pair, premium: Pair, annualized: Pair, buy: Pair,
               buy_source: Badge|null, buy_link: Text|null, badges: [Badge] }

`Pair`, `Badge` and `TierColumn` are the existing definitions; a gap in a `Pair` shows its gap text. `ShortList`, `Filter`, `DroppedItem`, `ShortGroup` and `ShortRow` are new names.

`Config` of `option_candidates_page` gains `Shortlist optionshortlistapi.Shortlister` (nil omits `short_put_list`, Decision 16).

## Issues

### 1 shortlist-api (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/optionshortlistapi/api.go`, `internal/compute/optionshortlistapi/api_test.go`
Contracts: unchanged
Design: docs/design/candidates-page-v2.md#Interface
Plan: P2
Acceptance:
1. `api.go` declares `Bounds`, `Row`, `Input`, `Drop`, `Placed`, `Group`, `Result` and `Shortlister` (all new names) with the fields of the Interface section and no function body; it imports `contracts` at most.
2. `api_test.go` has `TestShortlisterContractBothSides` (new: a fake `Shortlister` returns groups, drops, the bounds and the counts and a caller reads them unchanged; a compile-time check that the fake satisfies `Shortlister`).
3. `git diff --name-only BASE` lists only the two files.
4. The result matches docs/design/candidates-page-v2.md#Interface.

### 2 shortlist-rule

Module: compute. Size: M.
Order: after 1.
Files: `internal/compute/optionshortlist/shortlist.go`, `internal/compute/optionshortlist/reasons.go`, `internal/compute/optionshortlist/shortlist_test.go`, `internal/compute/optionshortlist/purity_test.go`, `docs/contracts/compute/optionshortlist.md`
Contracts: unchanged
Design: docs/design/candidates-page-v2.md#Decisions
Plan: P2
Acceptance:
1. `shortlist.go` (new) declares `New() optionshortlistapi.Shortlister` and the bounds as constants of the package (strike 5 to 15 percent below spot, absolute delta 0.15 to 0.30, at most 3 rows per underlying), each with a comment that names `docs/design/candidates-page-v2.md`; the package reads no rule parameter and imports no `ruleparam`.
2. `Select` computes the distance as (Spot - Strike) / Spot on exact rationals (`math/big`) from the `Strike` and `Spot` texts of the row, never from a break-even figure, and judges the absolute value of `Delta` on the same rationals.
3. `Select` decides a row from its own fields and all its marks and reasons, not from the first reason only: a row is dropped when its bid is empty or not above 0 or its `QuoteReason` is `no_quote`, `no_bid`, `no_ask` or `small_bid_size`; when any mark is `no_open_interest` or its `QuoteReason` is `low_open_interest`; when `Delta` is empty; when `Spot` or `Strike` is empty or unreadable; or when the distance or the absolute delta is outside the bounds; a row with `QuoteReason` `wide_spread` and nothing else is kept with the liquidity code `wide_spread`.
4. `Select` counts each dropped row once under the first code of the fixed order `bad_row`, `no_bid`, `no_open_interest`, `no_spot`, `otm_low`, `otm_high`, `no_delta`, `delta_out`, `over_cap` (`reasons.go` (new) holds the order and the mapping), applies the cap of 3 to the kept rows by stored annualized yield, and `Scanned` equals `Shown` plus the sum of the dropped counts for every input.
5. The result orders groups by the best stored annualized yield of the group and rows by their stored `Annualized` (the bid over the strike times 365 over the days, as the job stored it, not a tier figure), highest first, ties by ticker, strike, key; `Placed` carries the distance and the absolute delta as decimal text and `Result` carries `StaleChain`, `DTEMin` (new) and `DTEMax` (new) of the scanned rows and the `Bounds` echo.
6. `shortlist_test.go` has `TestDistanceFromStrikeAndSpot` (new: strike 645, spot 686.97 gives 6.1 percent and is kept, a row whose break-even is 11.5 percent below spot but whose strike is 3 percent below spot gives `otm_low`), `TestAnnualizedIsStoredYield` (new: groups follow the stored `Annualized` text), `TestDeltaBandUsesAbsoluteValue` (new: -0.22 is kept, -0.35 gives `delta_out`), `TestEmptyDeltaIsNoDelta` (new), `TestWideSpreadWithLowOpenInterestMarkDropped` (new: a wide-spread row that also has the mark `no_open_interest` is dropped as `no_open_interest`), `TestWideSpreadAloneKeptWithBadge` (new), `TestDropOrderIsFixed` (new: a row with no bid and a delta out of the band counts once, as `no_bid`), `TestCapKeepsBestYield` (new: five rows of one company keep three and count two `over_cap`), `TestTieBreakStable` (new: two row orders give one result), `TestCountsAddUp` (new) and `TestUnreadableBidDropsRow` (new: a bad `Bid` text drops that row only).
7. `purity_test.go` has `TestNoClockNoIO` (new: the non-test files import none of `time`, `os`, `net`, `database/sql`, `math/rand`); `docs/contracts/compute/optionshortlist.md` (new) states the bounds, the drop order and the delta rule.
8. The result matches docs/design/candidates-page-v2.md#Decisions.

### 3 shortlist-contract (interface)

Module: contracts. Size: M.
Order: first.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.option_candidates_page v1 additive
Design: docs/design/candidates-page-v2.md#Interface
Plan: P2
Acceptance:
1. The schema gains the optional property `short_put_list` (not in `required`; `oneOf` null or `ShortList`) and the definitions `ShortList`, `Filter`, `DroppedItem`, `ShortGroup` and `ShortRow` of the Interface section, each with `additionalProperties: false`, reusing `Pair`, `Badge` and `TierColumn`; `schema_version` stays 1 and `git diff BASE` on the schema has no removed line except the closing punctuation of an edited array; no new `"type": "number"`.
2. `web/src/generated/option_candidates_page.v1.ts` contains `ShortList`, `ShortGroup` and `ShortRow` (new names) and is listed in `git diff --name-only BASE` with the schema, never edited by hand.
3. `option_candidates_page_v1_test.go` gains `TestShortListNullValid` (new), `TestShortListAbsentValid` (new), `TestShortListOneGroupValid` (new: one group, one row with a gap `Pair`, a `buy_source` badge and a liquidity badge), `TestShortListUnknownFieldFails` (new: an extra field of a `ShortRow`) and `TestShortListNumberFails` (new: a JSON number for `strike.main`); no existing test of that file is edited.
4. `docs/contracts/page-schemas.md` extends the paragraph of `page.option_candidates_page.v1` with one sentence (new) naming `short_put_list` as additive and optional.
5. `git diff --name-only BASE -- contracts/schemas/carryover.sha256 internal/contracts/common_test.go` prints nothing.
6. The result matches docs/design/candidates-page-v2.md#Interface.

### 4 shortlist-page-if (interface)

Module: pages. Size: S.
Order: after 1.
Files: `internal/pages/option_candidates_page/page.go`, `internal/pages/option_candidates_page/page_test.go`
Contracts: unchanged
Design: docs/design/candidates-page-v2.md#Interface
Plan: P2
Acceptance:
1. `page.go` gains in `Config` the field `Shortlist` of type `optionshortlistapi.Shortlister` (new, both names) with the comment "nil gives no short_put_list"; every existing field keeps its name, type and order (`git diff BASE -- page.go` has no removed line); `NewPage` and `Build` are unchanged.
2. `page_test.go` has `TestConfigShortlistNilBuild` (new: a `Build` with `Shortlist` nil returns the page as before and its text has no `short_put_list`).
3. The package imports `optionshortlistapi` (new) and not `optionshortlist`; `git diff --name-only BASE` lists only the two files.
4. The result matches docs/design/candidates-page-v2.md#Interface.

### 5 shortlist-page

Module: pages. Size: L.
Order: after 1, 2, 3 and 4.
Files: `internal/pages/option_candidates_page/builder.go`, `internal/pages/option_candidates_page/shortlist.go`, `internal/pages/option_candidates_page/shortlistfacts.go`, `internal/pages/option_candidates_page/shortlistrows.go`, `internal/pages/option_candidates_page/shortlisttexts.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/shortlist_test.go`, `internal/pages/option_candidates_page/shortlistfacts_test.go`, `internal/pages/option_candidates_page/shortlistfakes_test.go`, `internal/pages/option_candidates_page/testdata/shortlist_ok.golden.json`, `internal/pages/option_candidates_page/testdata/shortlist_gaps.golden.json`, `docs/contracts/pages.md`
Contracts: unchanged
Design: docs/design/candidates-page-v2.md#Decisions
Plan: P2
Acceptance:
1. `builder.go` gains the optional field `ShortPutList` of type `*shortListView` with the JSON name `short_put_list,omitempty` (all three names new) in `viewModel` and one call that fills it from the stored rows and the tier verdicts already built; with the field nil the page bytes are those of today, so the golden files of `tiers_test.go`, `evidence_test.go` and `valueline_test.go` and `builder_test.go` are not edited and pass; `page.go`, `groups.go`, `tiers.go`, `format.go`, `purity_test.go` and `docs/design/ratchet.json` are not in `git diff --name-only BASE`.
2. `shortlistfacts.go` (new) builds the `optionshortlistapi.Input`: one `Row` per short-put row of every status with `Strike`, `Spot` (`deref(c.Spot)`), `Delta`, `Bid`, `Annualized` (the stored `AnnualizedYield`) and `DTE` as stored, `QuoteReason` and `EventsReason` by `parseGates`, `Marks` by `parseMarks`, `Ticker` from `Config.Names`, and `Key` the `tierKey` of the row; it reads no owner setting and no rule parameter; a row whose gates cannot be parsed is passed with the mark `unreadable_gates`, never dropped by the page.
3. `shortlist.go` (new) calls `Config.Shortlist.Select` once per build and writes `ShortList`: `run_day` (the group day), `counts`, `caveat` (from `Result.StaleChain`, null when 0), the six `filters` (`expiry` from `DTEMin` and `DTEMax` of the result, the other five from `Bounds`; the hint of `expiry` says in Chinese that the window is `min_dte` and `max_dte` of `job.toml`, not a setting), `dropped` and `dropped_items` from `Result.Dropped` in its order (a zero count is not written), `columns` with the eight column ids and English labels and Chinese hints, `folded_title`, and the groups in the order of the result; a nil `Config.Shortlist` or no short-put rows omits the property and adds no gap beyond the existing `no_reader`.
4. `shortlistrows.go` (new) writes each `ShortRow` with the formatters of `format.go` (`fixedText`, `percentText`, `moneyText`; the rounding the page needs is allowed, it computes no figure): `strike`, `expiry` (`main` the date, `sub` the days with `d`), `otm` (`Placed` distance as a percent with one decimal), `delta` (`Placed` absolute delta with two decimals; the gap text when the stored delta is empty), `premium` (`main` the bid, `sub` the money per contract from the tier verdict), `annualized` (the stored `Annualized` as a percent with one decimal), `buy` (`main` the buy price, `sub` the break-even and its margin from the tier verdict), `buy_source` (a `Badge` whose code comes from one function `buySourceOf` that returns `hand` for a stored buy price and nothing otherwise, the label and Chinese gloss of each code in `shortlisttexts.go` and `zh_texts.go`), `buy_link` (the price-lines path of the plan, `plan/price-lines` with a leading slash, only when the row has no buy price from any source; the link text is the source-neutral "No buy price") and the badge `wide_spread` with label "Wide spread" and a Chinese gloss.
5. The English texts are constants of `shortlisttexts.go` (new) and every Chinese text is a hint or gloss constant of `zh_texts.go` of 1 to 200 characters (labels, chips, counts and the dropped line are English, ui-english decision 1); no CJK character is in the other new files; every new file is under 600 lines and `zh_texts.go` stays under 800 lines.
6. `shortlist_test.go` has `TestShortListFromShortlister` (new: a fake `Shortlister` returns groups and drops the page cannot produce and the output shows them in that order), `TestShortListCountsLine` (new), `TestDroppedItemsOrderAndZeroOmitted` (new), `TestCaveatOnlyWithStaleChain` (new), `TestNilShortlistOmitsProperty` (new), `TestBuyLinkOnlyWithoutBuyPrice` (new), `TestBuySourceSlotIsHand` (new: a row with a stored buy price has the `buy_source` code `hand` and no `buy_link`), `TestDeltaGapWhenEmpty` (new), `TestExpiryChipFromResult` (new: the chip value is the `DTEMin` and `DTEMax` of the fake result), `TestShortListValidatesSchema` (new: the golden `shortlist_ok.golden.json` validates against the schema) and `TestShortListGapsGolden` (new, golden `shortlist_gaps.golden.json`: a row with no delta, a row with no buy price); `shortlistfacts_test.go` has `TestRowsCarryStoredFields` (new: `Strike`, `Spot`, `Annualized` and `Delta` equal the stored texts), `TestUnreadableGatesPassedWithMark` (new) and `TestReaderErrorReturned` (new).
7. `docs/contracts/pages.md` gains one paragraph (new) for `short_put_list`: the list, the reader added to `Config`, the constants of the rule and the omitted-when-nil behaviour.
8. The result matches docs/design/candidates-page-v2.md#Decisions.

### 6 shortlist-ot

Module: ot. Size: S.
Order: after 1, 2 and 4.
Files: `cmd/ot/shortlist_site.go`, `cmd/ot/shortlist_site_test.go`, `cmd/ot/candidates_site.go`, `cmd/ot/candidates_site_test.go`, `cmd/ot/check_blocks.go`, `cmd/ot/check_blocks_test.go`, `docs/contracts/ot/pages/option_candidates_page.md`
Contracts: unchanged
Design: docs/design/candidates-page-v2.md#Decisions
Plan: P2
Acceptance:
1. `shortlist_site.go` (new) declares `newShortlister() optionshortlistapi.Shortlister` returning `optionshortlist.New()`; `wire.go` and `site.go` are not in `git diff --name-only BASE`.
2. `buildCandidates` in `candidates_site.go` passes `newShortlister()` (new) as `Shortlist` (the new field of `page.go`) into `option_candidates_page.Config`; `candidatesPageNamed` passes none.
3. `shortlist_site_test.go` has `TestShortlisterIsRealRule` (new: `newShortlister()` is not nil, keeps one row with delta 0.22, 6 percent below spot and a bid, and drops one with delta 0.40 as `delta_out`), `TestCandidatesWiresShortlist` (new: the fake constructor receives a non-nil `Shortlist` and a build through the route returns the page) and `TestCandidatesNamedNoShortlist` (new); the existing fakes of `candidates_site_test.go` are edited only as far as the new field needs.
4. `check_blocks.go` adds `short_put_list` to the `pageMetaKeys` of `option_candidates_page`; `check_blocks_test.go` gains `TestCheckAcceptsShortPutListKey` (new: a payload with the key `short_put_list` has no unknown block); `check_blocks.json` is not in `git diff --name-only BASE`.
5. `docs/contracts/ot/pages/option_candidates_page.md` names the reader the route passes for the list, the named path that passes none, and the new file `cmd/ot/shortlist_site.go`.
6. The result matches docs/design/candidates-page-v2.md#Decisions.

### 7 shortlist-web

Module: web. Size: M.
Order: after 3.
Files: `web/src/pages/candidates/ShortList.tsx`, `web/src/pages/candidates/ShortListRows.tsx`, `web/src/pages/candidates/OptionCandidatesPage.tsx`, `web/test/pages/optioncandidates/ShortList.test.tsx`, `web/test/pages/optioncandidates/fixtures/shortlist.ts`, `web/test/pages/optioncandidates/fixtures/MANIFEST.synthetic-v1.json`, `web/test/pages/optioncandidates/OptionCandidatesPage.test.tsx`
Contracts: unchanged
Design: docs/design/candidates-page-v2.md#Decisions
Plan: P2
按 frontend-stack：用 shadcn Table 做 分组列表（八列：Strike、Expiry、OTM %、Delta、Premium (bid)、Annualized、Buy price、Liquidity；组头整行跨列）。
按 frontend-stack：用 shadcn Badge 做 Wide spread 徽标、买入价来源标签和筛选条件的 chip，用 shadcn Tooltip 做 列头、chip 和徽标的白话注释，用 shadcn Accordion 做 “全部 N 张”折叠区（里面放原有的档位视图和原始表格，默认关闭），用 shadcn Alert 做 全页的一句注意；本页没有图，不用 Recharts；不新增 @tanstack/react-table 用法。
Acceptance:
1. `ShortList.tsx` (new) renders `payload.short_put_list` when it is present and not null: the counts line, the caveat as one Alert when not null, the `filters` as chips with their `hint` as the tooltip, the `dropped` line with its items, then `ShortListRows`; with `short_put_list` null or absent the page renders exactly as today.
2. `ShortListRows.tsx` (new) renders one group header row per `groups[]` item (`ticker`, `spot`, `summary`) and one table row per `rows[]` item in payload order with the eight columns of `columns`; a `Pair` with a gap shows the gap text and no empty value; `buy_source` renders as a small tag with its gloss as the tooltip when not null; `buy_link` renders as a link when not null.
3. `OptionCandidatesPage.tsx` shows `ShortList` first and, only when `short_put_list` is present, puts the tier view and the stored tables inside one closed Accordion whose heading is `folded_title` (a new payload field of the schema); with the property absent the tier view, the stored-tables accordion and their order are those of today, so `OptionCandidatesPage.test.tsx` and `TierView.test.tsx` keep every existing test unedited (the added test of item 5 is the only change to the first).
4. The components render every text from the payload and contain no arithmetic on a figure, no `toFixed`, no `Intl.NumberFormat`, no sort and no filter: `git grep -n "toFixed\|NumberFormat\|\.sort(\|\.reduce(\|\.filter(" -- web/src/pages/candidates/ShortList.tsx web/src/pages/candidates/ShortListRows.tsx` finds no line; each new file is under 600 lines.
5. `ShortList.test.tsx` has `renders groups in payload order` (new), `renders the gap text for an empty delta` (new), `renders the buy link only when buy_link is set` (new), `renders the buy source tag` (new) and `renders nothing for a null short_put_list` (new); `OptionCandidatesPage.test.tsx` has `folds the tier view under the closed accordion` (new: with the list present the tier view is not in the document until the accordion is opened); `fixtures/shortlist.ts` is listed in `fixtures/MANIFEST.synthetic-v1.json`.
6. `git diff --name-only BASE -- package.json web/package.json pnpm-lock.yaml web/dependency-allowlist.json frontend-whitelist.json` prints nothing, and `web/src/generated` is not in `git diff --name-only BASE`.
7. The result matches docs/design/candidates-page-v2.md#Decisions.

### 8 chain-fresh-cases

Module: jobs. Size: S.
Order: first.
Files: `internal/jobs/optioncandidates/input_test.go`, `internal/jobs/optioncandidates/job_test.go`
Contracts: unchanged
Design: docs/design/candidates-page-v2.md#Decisions
Plan: P2
Acceptance:
1. `job_test.go` has `TestRunChainOnRunDayNotStale` (new: session days up to the run day, a chain on the run day, no earnings row and no account: no stored row has the events reason `stale_chain`), `TestRunChainOfPreviousSessionNotStale` (new: the chain one session before the run day) and `TestRunChainTwoSessionsOldIsStale` (new: the chain two sessions before the run day gives `stale_chain` on every row).
2. `input_test.go` has `TestTrimSessionDaysKeepsRunDay` (new: with the chain day as the only input date, the trimmed list still ends with the run day and holds the session before it).
3. `input.go`, `events.go` and every other non-test file are not in `git diff --name-only BASE`; if a test fails on the unchanged code the pull request description names the failing test and the file that needs the fix, and leaves it unchanged.
4. `git diff --name-only BASE` lists only the two test files.
5. The result matches docs/design/candidates-page-v2.md#Decisions.

## Checked by CI

- `ci-go`: `go build ./...`, `go vet ./...` and `go test ./internal/compute/... ./internal/pages/... ./internal/contracts/... ./internal/jobs/optioncandidates/... ./cmd/ot/...` pass (the compute, contracts, pages, ot and jobs issues).
- `ci-web`: `tsc`, `eslint`, `vitest` and the build pass (the web issue); `ui-shots` takes the page at 1440 px and the window compares it with `mockup-1440.png` after deploy.
- After deploy the window re-runs `option_candidates` for 2026-11-18 (reads `computed_at` of the rows) and looks at the live page at 1440 px: the list has groups, the Delta column has values and the count line adds up.

## Left out

- Removing the tier view and the stored tables: they stay folded; deleting them is a later step after the owner has used the list.
- A filter control on the page and settings for the bounds (Decision 6): a page control would need a write path, and a setting would add rows to the owner's settings history; both wait until the owner has used the list.
- The days-to-expiry window 20 to 50 of the mockup and any change of `job.toml` (Decision 17).
- Dropping rows at write time in `optioncand.Evaluate` (Decision 5), and a stored `chain_day` column (a migration; the list prints the run day instead).
- Open measurement: how many of today's 2343 rows have an empty delta. The window's read-only psql was not available to the designer (no database socket and no secrets of the owner were touched); until it is read, the count of "No delta" on the live page is the first measurement, and if it dominates the list the capture of the delta is the next design.
- Open limit: a row that is both wide spread and low open interest is kept with the badge, because the stored gate keeps one reason (Decision 9); storing the open interest is a migration for a later design.
- Mid price of the quote (not in the stored rows; the bid is shown and the wide-spread badge says to use the middle).
- The buy price chain of the later design (valuation buy-zone top, else hand price, else the automatic SEC FCF band) and the tier rule's use of it; only the source slot is here (Decision 14).
- Covered calls and held-call guards: the list is for short puts only.

## Adversarial

The read is `adversarial-raw.md` beside this file (origin/main 2c7e99b5 when it was written, answered against ded5e555). The six points of its section 1, with its other findings:

1. Gap against the goal (distance, window, chips, cap, open interest, language): adopted. The distance is now (spot - strike) / spot from `Strike` and `Spot` in `optionshortlist` (Decision 7), the column, the filter and the test names follow (`TestDistanceFromStrikeAndSpot`), and the tier `BelowSpotPct` is not used. The window is `job.toml`, said plainly in the chip hint and Decision 17, and the false claim that the settings page widens it is gone; the chip reads its numbers from the scanned rows, so it has a source. The cap of 3 is called an addition of the mockup (Decision 10). Open interest is judged from the marks and every reason (Decision 9); the one case that stays invisible, wide spread behind low open interest, is named as a limit. Chinese is kept to hints; chips, counts and the dropped line are English.
2. Biggest unnamed risk (silent wrong filter, delta availability): adopted for the filter (a test pins that a row 3 percent below spot with a break-even 11.5 percent below spot is `otm_low`); delta availability is adopted as a stated rule (empty delta is dropped as `no_delta` and counted, Decision 8) and the count of empty deltas is an open measurement (Left out), because the read-only database was not reachable here.
3. Simpler way: adopted in part. `optionshortlist` stays beside `optiontier` (the tier judge's verdict is a per-row colour, the list is a filter with counts), but it takes `Strike` and `Spot` and computes the distance itself, the page no longer copies `BelowSpotPct` or `YieldPct`, no ruleparam key is added (the settings form stays at 13 keys), and the issue count fell from nine to eight (the params issue is gone).
4. Lock-in: adopted. No registry rows or tests change (Decision 6); `put_delta_max` moves only the tier rule, not the list; the schema field names are additive and frozen by the first consumer, accepted; the buy price has a source slot and a source-neutral link text, so the later buy price design needs no schema bump (Decision 14); rows stored before that design keep their stored `buy_price`, which the page calls `hand`.
5. Claims most likely false: adopted. The yield is the stored `annualized_yield`, not the verdict's time-value yield (Decision 3); the settings claim is removed; the ui-english claim is corrected; the `stale_chain` cause is now stated as unproven from code and the re-run and `computed_at` check are the window's after merge (Decision 13).
6. Likeliest slip in the issue cut: adopted. The registry tests no longer change; the rounding the page needs is allowed through `format.go` (no purity test bans it); every Files line lists the files the change touches and states which existing tests stay unedited because the property is omitted when nil and the old page path is kept when the list is absent (web tests `TestStoredTablesBelowTiers` and `TestStoredTablesClosedByDefault`, the page goldens), plus the fixtures manifest the hygiene check needs; shared paths carry `Order: after` lines; all `Checked against` lines are re-pinned to ded5e555 and use one header format; the stale-chain issue accepts tests and an unchanged `input.go` only, and the re-run is not an acceptance item; the ui-shots comparison is under Checked by CI as a window action.
