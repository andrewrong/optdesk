# candidates valuation line: each company strip of `/options/candidates` shows the valuation buy line B_hi beside the hand-typed buy price the gate uses, and changes no judgement

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-18

Source: the plan `state/optdesk-drafts/designs/sellput-strategy/plan-2026-11-17.md` (owner approved 2026-11-17, "aaac"), sections 早上：候选页 (the line "Valuation buy line 91.20 (review 2026-11-10) [shown only; the gate uses 90.00]"), 分步开发 step 10, 买入价与估值的关系 (question 3, the owner's answer a: side by side, the gate unchanged), 不做的事; the merged designs `docs/design/candidates-tiers.md` (the company strip, the Chinese door) and `docs/design/valuation-structure.md` (the store `internal/store/valuation`, the calculator `internal/compute/valuelinesapi`). This design is plan step 10 only. Step 13 (the buy price source reaching the gate) is not here; see Left out.

Checked on origin/main (796f5ec5): the candidates page `option_candidates_page` has the tier view merged (`tiers.go`, `tierrows.go`, `tierfacts.go`, `tierstrings.go`, `tierlegend.go`, `zh_texts.go`, #2822, #2834 and the ot wiring #2835). `companyView` in `tiers.go` (JSON `key`, `ticker`, `spot`, `spot_day`, `buy_price`, `buy_price_note`, `next_earnings`, `counts`) is the company strip and is written by `companiesOf`; the web shows it in `web/src/pages/candidates/CompanyStrip.tsx`. `Config` (`page.go`) has no valuation field. `valuation.Reader.Current(ctx)` gives the `State` (newest profile row per company key, the current review of each, the newest R version and margin table); `valuelinesapi.Calculator.Lines` gives `BuyHigh` from `V`, `P`, the market column, the R values and the margin table, and needs no close (`Input.Close` may be empty, `api.go`). The valuation page copies the store rows into the calculator input in `valuation_page.Resolve` (page code of another package; this page does not import it). `cmd/ot/valuation_site.go` opens the valuation reader for every site route (`openValuationReader`, key `siteValuationKey`, nil in the demo profile and without the pages line) and `valuationReaderOf(rd)` returns it. `valuelines.New()` returns the concrete `valuelines.Calc`. The gate reads only the stored hand-typed `buy_price` (`put.go`); nothing here touches it.

## What you see

On the first view of the candidates page, each company strip gets two more lines under the spot and buy price. The numbers are invented (AAA buy price 90.00 hand-typed; AAA valuation review of 2026-11-10 gives 91.20; BBB has no effective valuation; CCC's valuation is in JPY).

    AAA   Spot $96.50 (2026-11-17)   Buy price $90.00   Next earnings 2027-01-28   3 fit, 1 check, 2 collapsed
          Valuation buy line B_hi   $91.20 (review 2026-11-10)       [hint, Chinese: B_hi is the buy ceiling the valuation page computes from your review; shown beside the hand-typed price only]
          Buy price used by the gate (hand-typed)   $90.00           [hint, Chinese: the gate and the tiers use only this hand-typed number; nothing changes if you edit the valuation]
    BBB   Spot $41.20 (2026-11-14)   Buy price not set   ...
          Valuation buy line B_hi   (Chinese reason: no effective valuation review)
          Buy price used by the gate (hand-typed)   not set
    CCC   ...
          Valuation buy line B_hi   (Chinese reason: the valuation is not in USD, not shown beside a USD option)

Labels, the figures and the review day are English; every hint and every reason is Chinese, held as string constants in `zh_texts.go` (Decision 6). The two prices are shown as they are; the page does not subtract them and does not say which is better (Left out).

## Goal

The plan's step 10: put the valuation buy line next to the hand-typed buy price, so that the owner can see for some weeks how far apart the two are before he decides (question 3) whether the gate should take B_hi. The page must say, for every company, which price the gate used, and must say why there is no B_hi when there is none. No gate change, no change of which rows are in which tier, no migration, no write, no new store.

## Today

- `buildTiers` (`tiers.go`) writes `short_put_tiers.companies` through `companiesOf(res, rows, day)`, one `companyView` per company of the tier answer, with the ticker from `store.Candidate.Issuer` (the instrument name read through `Config.Names`, `tickersOf` in `readings.go`, `namedRows` in `names.go`). The company key is the underlying id as text.
- The buy price of the strip is the stored row's `BuyPrice` through `moneyText`, or the text `not set` (`notSet`, `tierstrings.go`); the strip does not say where the price comes from.
- `Config` (`page.go`) holds the readers the page uses (candidates, figures, names, evidence, ATM, readings, tiers, account, aliases, owner, rule params); `cmd/ot/evidence_site.go` fills it from `candidateReaders` and `buildCandidates` (`candidates_site.go`) fills `candidateReaders`; the named path (`ot page`) and the demo profile pass none of the tier readers, so `short_put_tiers` is null there.
- The schema `page.option_candidates_page.v1` has `Company` with `additionalProperties: false` and seven required fields; the optional property `short_put_tiers` was added without a version change (`docs/contracts/page-schemas.md`).
- `valuation.Entry{Profile{CompanyKey, Name, Symbol, Currency, RMarket, Pool}, Current *Review}`: `Current` is the effective review with no void row and a type other than backfill; a review has `V`, `P` and `VersionDate`. `Calculator.Lines` accepts an empty `Close` (it then writes only the zone note `no_close`, which this page ignores) and returns `ErrBadInput` when `V` is not above 0, `P` is outside 1 to 5 or the margin table has no row for `P`.
- Line counts: `tiers.go` 276, `page.go` 47, `zh_texts.go` 72, `tierstrings.go` 131, `evidence_site.go` 149, `candidates_site.go` 128, `CompanyStrip.tsx` 21; no file is in `docs/design/ratchet.json` for this change. A new source file stays under 600 lines.

## Decisions

1. **One more block on the company strip, nothing else changes.** `Company` of `page.option_candidates_page.v1` gets the optional property `valuation` (`ValueLine` or null, not in `required`), added without a version change as `short_put_tiers` was. The tiers, the rows, the counts, the stored tables, the gate, the order of companies and every existing field are as before; the goldens change only by the new key.
2. **Where B_hi comes from.** `Config` gets `Valuation valuation.Reader` and `ValueLines valuelinesapi.Calculator`. Once per build the page calls `Valuation.Current(ctx)`; for each company of the strip with a matching entry it passes `V` and `P` of `Entry.Current`, `Entry.Profile.RMarket`, the R values of `State.RVersion`, the rows of `State.Margin` (copied field by field into the plain `valuelinesapi` types, as `valuation_page.Resolve` does) and an empty `Close` to `Calculator.Lines`, and takes `Lines.BuyHigh` unchanged. The page adds, compares and rounds nothing: the money text is the existing `moneyText`, and `BuyHigh` is the calculator's. The close is left empty on purpose: `BuyHigh` does not use it, so no price read is added.
3. **Which valuation belongs to a company.** The company's ticker (the name the page already holds) equals `Entry.Profile.Symbol` ignoring letter case (`strings.EqualFold`). If two companies of the strip have the same ticker, both get the reason `ambiguous` and no figure (a name shared by two instruments cannot say which one the valuation is for). The symbol and the ticker share one vocabulary: `valuation_page.Resolve` turns `Profile.Symbol` into an instrument id through the stock name reader (`StockIDs(names)`), and `Config.Names` reads the same instrument names; a symbol with another spelling gives `no_profile`, which the strip says in words, never a wrong figure. A strip with no ticker (no name reader) matches nothing.
4. **Reasons, in this order of testing.** The field `reason` is null when a figure is shown and else one of: `no_profile` (no entry has the ticker), `ambiguous` (Decision 3), `removed` (the entry's `Pool` is `removed`), `no_review` (`Current` is nil), `no_tables` (`RVersion` or `Margin` is nil), `currency` (`Profile.Currency` is not `USD`: B_hi is in the valuation's currency and the page does not convert), `bad_input` (`Lines` returned an error), `read_failed` (`Current` returned an error; every company gets it). `Valuation` or `ValueLines` nil gives `valuation` null for every company (the named path, the demo profile, and a site without the pages line, where `openValuationReader` gives no reader): the strip shows no valuation lines, as it shows no tiers there; the owner sees the block missing, which Left out names.
5. **A valuation failure never fails the page.** The valuation line is an addition; a reader error of `Current` gives `read_failed` on every strip and the page is built as without it. This is unlike the evidence and tier readers, whose errors are returned, because those readers feed the judgement and this one does not.
6. **Texts.** Labels (English): "Valuation buy line B_hi" and "Buy price used by the gate (hand-typed)". `ValueLine` = `label`, `text`, `hint`, `reason`, `gate_label`, `gate_text`, `gate_hint`. `text` is the money text and `(review YYYY-MM-DD)` of `Current.VersionDate` (English), or for a reason the Chinese reason sentence; `gate_text` is the strip's own `buy_price` text or `not set`. The two hints and the eight reason sentences are Chinese string constants of `internal/pages/option_candidates_page/zh_texts.go` (names `valueHintZh`, `gateHintZh` and `valueReason<Code>Zh`), each 1 to 200 characters, no number inside a constant. The hints say that B_hi is shown only and the gate uses the hand-typed price, and one sentence of `valueHintZh` says that B_hi is computed with today's R version and margin table and the review's V and P, so the review day beside it is the day of V and P, not of the tables.
7. **The price shown is the price the gate used.** `gate_text` is read from the same stored row as `buy_price`, so the two lines on the strip cannot disagree with the tiers; the label says "hand-typed" because the stored `buy_price` comes only from the setting `settings.underlying.buy_price` today (plan 现状); when step 13 changes the source, this label and `gate_hint` are the one place to change (Left out).
8. **Wiring.** `cmd/ot` gets `newValueLines() valuelinesapi.Calculator` (returns `valuelines.New()` as the receiver's api type, in a new file); `candidateReaders` gains `Valuation valuation.Reader`; `buildCandidates` fills it with `valuationReaderOf(rd)` and `ValueLines` with `newValueLines()` unless the profile is `demo`; `candidatesPageNamed` passes none. No new store or opener: the site already opens the valuation reader for every route.
9. **Cost.** One `Current` read and at most one `Lines` call per company per build; the page is built per request as today.
10. **Web** renders and computes nothing: `CompanyStrip.tsx` shows `valuation` when it is not null or absent as two rows (label, text, hint in a tooltip beside the label; the same for the gate row); a payload without `valuation` renders as today.
11. **No migration, no write, no job change, no gate change.** `docs/contracts/pages.md` gets one paragraph and `docs/contracts/ot/pages/option_candidates_page.md` one sentence.

## Modules

`docs/design/modules.md`: no row is added or changed. Paths and the first row whose glob matches: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go` and `docs/contracts/page-schemas.md` contracts (entry 1), `internal/pages/option_candidates_page/**` and `docs/contracts/pages.md` pages (entries 2 and 3), `cmd/ot/**` and `docs/contracts/ot/pages/option_candidates_page.md` ot (entry 4), `web/src/pages/candidates/**` and `web/test/pages/optioncandidates/**` web (entry 5). Imports against the `depends` column: pages imports `store/valuation` (store) and `valuelinesapi` (compute), both allowed; `cmd/ot` imports `valuelines` (compute); web imports generated types only.

## Interface

Entry 1 (module contracts): `contracts/schemas/page.option_candidates_page.v1.json` gains the definition `ValueLine` (`additionalProperties: false`; required `label`, `text`, `hint`, `reason`, `gate_label`, `gate_text`, `gate_hint`; `label`, `text`, `gate_label`, `gate_text` are `Text`; `hint` and `gate_hint` are `Text` or null; `reason` is null or the enum `no_profile`, `ambiguous`, `removed`, `no_review`, `no_tables`, `currency`, `bad_input`, `read_failed`) and the optional property `valuation` of `Company` (`ValueLine` or null, not in `required`). The generated type `web/src/generated/option_candidates_page.v1.ts` is regenerated, never edited by hand. `schema_version` stays 1.

Entry 2 (module pages, interface `internal/pages/option_candidates_page/page.go`): `Config` gains two fields after the existing ones:

    // Valuation reads the current valuation of each company; nil gives no valuation block on the strips.
    Valuation valuation.Reader
    // ValueLines computes B_hi from a review; nil gives no valuation block on the strips.
    ValueLines valuelinesapi.Calculator

`NewPage` and `Build` are unchanged.

## Issues

Ordering against the design `tiers-amend-1` (branch `design/tiers-amend-1`, head 7aa35f87), which edits the same `companyView` and `tiers.go`, the `Company` definition of the schema and `CompanyStrip.tsx`: valline-contract lands after its entry 10, valline-page after its entry 11 and valline-web after its entry 12; the window adds `Depends on: #<n>` when it registers these drafts. Build order: entries 1 and 2 are the interface entries and land first, in parallel; entry 3 (pages) needs both; entry 4 (ot) needs entry 2; entry 5 (web) needs only the generated types of entry 1. The candidates tier entries are on main, so no entry waits for an unmerged step. Companies in every test and fixture are AAA, BBB, CCC with invented numbers (no valuation number of the owner appears in the repository). No migration; nothing holds the auto-deploy. A test names a Chinese constant and never repeats its text.

### 1 valline-contract (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.option_candidates_page v1 additive
Design: docs/design/candidates-valuation-line.md#Interface
Plan: P2
Acceptance:
1. The schema has the new definition `ValueLine` with the fields and the `reason` enum of the Interface section and `additionalProperties: false`, and `Company` has the property `valuation` (`ValueLine` or null) while `required` of `Company` is unchanged; `schema_version` is still the const 1; `git diff BASE -- contracts/schemas/page.option_candidates_page.v1.json` has no removed line except the closing punctuation of an edited object; `git grep -n '"type": "number"'` finds no new line in the schema. The text named here is new.
2. `web/src/generated/option_candidates_page.v1.ts` contains `ValueLine` (a new name) and `git diff --name-only BASE` lists it together with the schema (never edited by hand).
3. `option_candidates_page_v1_test.go` gains `TestOptionCandidatesValueLineValid` (new: a company with a `valuation` of a figure and a null `reason` validates), `TestOptionCandidatesValueLineReasonValid` (new: a `valuation` with reason `no_review` validates), `TestOptionCandidatesValueLineNullValid` (new: `valuation` null validates), `TestOptionCandidatesNoValuationStillValid` (new: a company without the key validates, which is the additive rule), `TestOptionCandidatesValueLineReasonEnum` (new: reason `stale` fails) and `TestOptionCandidatesValueLineUnknownField` (new: an extra key of `ValueLine` fails); the existing tests are not edited.
4. `docs/contracts/page-schemas.md` extends the `page.option_candidates_page.v1` paragraph with one sentence (new): `Company.valuation` is additive and optional and its definition is named by this design's Interface.
5. `git diff --name-only BASE -- contracts/schemas/carryover.sha256 internal/contracts/common_test.go` prints nothing.
6. The result matches docs/design/candidates-valuation-line.md#Interface.

### 2 valline-page-if (interface)

Module: pages. Size: S.
Order: first, in parallel with 1.
Files: `internal/pages/option_candidates_page/page.go`, `internal/pages/option_candidates_page/page_test.go`
Contracts: unchanged
Design: docs/design/candidates-valuation-line.md#Interface
Plan: P2
Acceptance:
1. `page.go` gains in `Config` the fields `Valuation valuation.Reader` and `ValueLines valuelinesapi.Calculator` after the existing ones, with the comments of the Interface section (both names are new); every existing field keeps its name, type and order (`git diff BASE -- internal/pages/option_candidates_page/page.go` has no removed line other than the import block's edit) and `NewPage` and `Build` are unchanged.
2. `page_test.go` has `TestConfigValuationFieldsNilBuild` (new: a `Build` with both new fields nil returns the page as before, built with the fakes of the existing tests, and no company of `short_put_tiers` has a `valuation` text other than null).
3. The package imports `internal/store/valuation` and `internal/compute/valuelinesapi` and no `valuelines`, `valuation_page` or `valuationpg` package; `git diff --name-only BASE` lists only the two files.
4. The result matches docs/design/candidates-valuation-line.md#Interface.

### 3 valline-page

Module: pages. Size: M.
Order: after 1 and 2.
Files: `internal/pages/option_candidates_page/valueline.go`, `internal/pages/option_candidates_page/valueline_test.go`, `internal/pages/option_candidates_page/valuefakes_test.go`, `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/tiers_test.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/testdata/tiers_ok.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/valueline.golden.json`, `docs/contracts/pages.md`
Contracts: unchanged
Design: docs/design/candidates-valuation-line.md#Decisions
Plan: P2
Acceptance:
1. `valueline.go` (new) declares `valueLineView` with the JSON names of `ValueLine` and one function that fills the `valuation` field of each `companyView` from `Config.Valuation` and `Config.ValueLines` by Decisions 2 to 5; `tiers.go` gains the field `Valuation *valueLineView` with the JSON name `valuation` in `companyView` and one call to that function in `buildTiers`; `page.go`, `groups.go`, `builder.go` and `docs/design/ratchet.json` are not in `git diff --name-only BASE`, and every new non-test file is under 600 lines. The text named here is new.
2. The figure of a strip is `Calculator.Lines(...).BuyHigh` from the review, profile market, R version and margin table of the entry whose `Symbol` equals the ticker ignoring case, written with `moneyText` and ` (review YYYY-MM-DD)`; a fake `Calculator` that answers a `BuyHigh` the page cannot produce shows that text unchanged; `Close` of the input is empty. The text named here is new.
3. A strip carries exactly one reason, null when a figure is shown, tested in the order of Decision 4: no matching entry `no_profile`, two companies with one ticker `ambiguous` on both, `Pool` `removed` `removed`, no current review `no_review`, no R version or no margin table `no_tables`, a currency other than `USD` `currency`, an error of `Lines` `bad_input`, an error of `Current` `read_failed` on every company; a nil `Valuation` or nil `ValueLines` leaves `valuation` null on every company and a `Current` error does not make `Build` return an error. The text named here is new.
4. `gate_text` equals the `buy_price` text of the same strip, or `not set` when the strip has no buy price; `label` and `gate_label` are the English labels of Decision 6; the two hints and the reason sentences are the Chinese constants `valueHintZh`, `gateHintZh` and `valueReason<Code>Zh` of `zh_texts.go` (all new constants), each 1 to 200 characters, and `zh_texts.go` holds only const declarations with string literals; no other non-test file of the package holds a Chinese character.
5. `valueline_test.go` (new) has `TestValueLineFromCalculator` (new), `TestValueLineMatchIgnoresCase` (new), `TestValueLineAmbiguousTicker` (new: AAA and a second underlying named AAA both get `ambiguous`), `TestValueLineReasonOrder` (new: a removed pool with no review says `removed`), `TestValueLineNoReview` (new), `TestValueLineNoTables` (new), `TestValueLineCurrency` (new: a JPY profile shows no figure), `TestValueLineBadInput` (new: a fake `Calculator` returning an error gives `bad_input`), `TestValueLineReadErrorIsSoft` (new: `Build` returns the page and every strip says `read_failed`), `TestValueLineNilReadersNull` (new), `TestValueLineGateTextIsBuyPrice` (new: a company with no buy price says `not set`), `TestValueLineTextsFromZhConsts` (new: the hints and the reason of `no_review` equal their constants and are 1 to 200 characters), `TestValueLineDoesNotChangeTiers` (new: the tiers, rows and counts of a build with the valuation readers equal those of a build without them), `TestNoArithmeticInValueLine` (new: `valueline.go` imports none of `github.com/shopspring/decimal`, `math`, `math/big`, `sort` and calls neither `percentText` nor `roundDigits`) and `TestValueLineValidatesSchema` (new: the golden `valueline.golden.json` validates against `page.option_candidates_page.v1.json`); `tiers_test.go` is edited only where it names `companyView` or the goldens.
6. `tiers_ok.golden.json` and `tiers_gaps.golden.json` gain the key `valuation` on each company (null) and change nowhere else.
7. `docs/contracts/pages.md` gains one paragraph (new) for `Company.valuation`: the readers added to `Config`, the match by ticker, the reasons, that the line is shown only and the gate and tiers are unchanged, and that a valuation read error does not fail the page.
8. `git diff --name-only BASE` lists only files under `internal/pages/option_candidates_page` and `docs/contracts/pages.md`.
9. The result matches docs/design/candidates-valuation-line.md#Decisions.

### 4 valline-ot

Module: ot. Size: S.
Order: after 2.
Files: `cmd/ot/valline_site.go`, `cmd/ot/valline_site_test.go`, `cmd/ot/evidence_site.go`, `cmd/ot/evidence_site_test.go`, `cmd/ot/candidates_site.go`, `cmd/ot/candidates_site_test.go`, `docs/contracts/ot/pages/option_candidates_page.md`
Contracts: unchanged
Design: docs/design/candidates-valuation-line.md#Decisions
Plan: P2
Acceptance:
1. `valline_site.go` (new) declares `newValueLines() valuelinesapi.Calculator` returning `valuelines.New()` (the return type is the receiver's api type; `newValueLines` is a new name); `wire.go` and `site.go` are not in `git diff --name-only BASE`.
2. `candidateReaders` and `candidatesConfig` are declared in `evidence_site.go`, so that file and its test are edited here; `candidateReaders` gains the field `Valuation` (new name); `buildCandidates` fills it with `valuationReaderOf(rd)` and `candidatesConfig` passes it and `newValueLines()` as `Valuation` and `ValueLines` (both new names, set in `page.go` by entry 2) into `option_candidates_page.Config` unless the profile is `demo`; `candidatesPageNamed` passes neither; no opener is added and `valuation_site.go` is not in `git diff --name-only BASE`.
3. `valline_site_test.go` (new) has `TestValueLinesIsRealCalculator` (new: `newValueLines()` is not nil and `Lines` of a sample input gives the `BuyHigh` the calculator gives), `TestCandidatesWiresValuation` (new: the fake constructor, which takes `candidateReaders`, receives a non-nil `Valuation` from the site readers and a build through the route returns the page) and `TestCandidatesConfigHasValueLines` (new: `candidatesConfig` returns a `Config` whose `ValueLines` is not nil and whose `Valuation` is the one of the readers), `TestCandidatesNamedNoValuation` (new: the named path passes nil for both and opens no store beyond those it opens today) and `TestDemoProfileBuildsNoValuation` (new); the fakes and calls of `candidates_site_test.go` and `evidence_site_test.go` are edited only as far as the new field needs.
4. `docs/contracts/ot/pages/option_candidates_page.md` names the valuation reader the route passes, the calculator constructor and the named path that passes none.
5. The result matches docs/design/candidates-valuation-line.md#Decisions.

### 5 valline-web

Module: web. Size: S.
Order: after 1.
Files: `web/src/pages/candidates/CompanyStrip.tsx`, `web/test/pages/optioncandidates/CompanyStrip.test.tsx`, `web/test/pages/optioncandidates/fixtures/valueline.ts`
Contracts: unchanged
Design: docs/design/candidates-valuation-line.md#Decisions
Plan: P2
按 frontend-stack：用已有的 shadcn Card 做公司条，估值线与闸门买入价各占一行（标签、数字、注释三列），用 shadcn Tooltip 把 hint 放在标签旁；原因文字用 Alert 的弱化样式；用 Lucide 图标；本块没有图，不用 Recharts，也不用 @tanstack/react-table。
Acceptance:
1. `CompanyStrip.tsx` renders two extra rows from the new payload field `valuation` of the company when it is not null or absent (label, text, and the hint in a tooltip beside the label; the gate row the same with the new fields `gate_label`, `gate_text` and `gate_hint`) and renders as it does today when `valuation` is null or absent; no other file under `web/src` is in `git diff --name-only BASE`.
2. The component renders every text from the payload and contains no arithmetic on a figure, no `toFixed`, no `Intl.NumberFormat`, no sort and no filter: `git grep -n "toFixed\|NumberFormat\|\.sort(\|\.reduce(\|\.filter(" -- web/src/pages/candidates/CompanyStrip.tsx` finds no line.
3. A strip with a `reason` shows `text` (the Chinese sentence) in the place of the figure: in the new fixture the company with the new reason value of the `no_review` case shows its sentence and the page text of that strip holds no `$` after the label.
4. `CompanyStrip.test.tsx` (new) has `TestStripShowsValueLine` (new: fixture `valueline.ts` shows `$91.20 (review 2026-11-10)` exactly as given, and the gate row `$90.00`), `TestStripShowsReasonText` (new: a company with reason `no_review` shows its text and no `$`), `TestStripNoValuationUnchanged` (new: a company with `valuation` null and one without the key render the same text as today) and `TestStripHintBesideLabel` (new), with the fixture `fixtures/valueline.ts` (new, invented numbers); the existing fixtures and `TierView.test.tsx` are not in `git diff --name-only BASE`.
5. `git diff --name-only BASE -- package.json web/package.json pnpm-lock.yaml frontend-whitelist.json` prints nothing, and `web/src/generated` is not in `git diff --name-only BASE`.
6. The result matches docs/design/candidates-valuation-line.md#Decisions.

## Checked by CI

- `go build ./...`, `go vet ./...`, `go test ./internal/contracts ./internal/pages/option_candidates_page ./cmd/ot` with `-race`; every test of the new files runs in parallel (`t.Parallel()`), uses `t.TempDir()` for any file, and none sleeps or reads the clock; the tests named in the entries pass; `go test ./cmd/ot` includes the Chinese door tests (`TestZhTextsDoor`, `TestGoTreesAreEnglish`).
- go-arch-lint (the import edges of Modules and the injection of `newValueLines()` into the page `Config`), the size check against `docs/design/ratchet.json`.
- `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest`, the web build; the frontend dependency check.
- The `test` check (reads `Contracts:` of the pull request body: entry 1 `Contracts: page.option_candidates_page v1 additive`, the others `Contracts: unchanged`).
- `crew issue check` of each entry against the head of the design pull request.
- After the merge, by the window and not an issue: a screenshot of the rendered strip against the sample above.

## Adversarial

Adversarial file: `state/optdesk-drafts/designs/candidates-valuation-line-adversarial.md` (one fresh read-only sub-agent given the design file and the repository, six questions).

1. Gap against the goal: adopted in part. The reasons are words, not a silent gap. Not adopted: a difference or a mark between the two prices and any history; the plan's question 3 is decided by the owner looking for weeks, and a difference is a later design (Left out). The gate row is kept: the plan names the label "buy price used by the gate", and the label is what step 13 changes.
2. Unnamed risk, the match key: adopted. Decision 3 says why the symbol and the ticker share a vocabulary and that another spelling gives `no_profile` in words; a nil reader (no pages line) is named in Decision 4 and Left out. A valuation with no row on the page gets no line by design (the strip exists per candidate company).
3. A simpler way (fewer reasons, no gate fields, three entries): not adopted. `ambiguous`, `removed` and `bad_input` each prevent a wrong figure that the simple form would show or hide without a word; the gate fields keep the strip honest after step 13; the interface entries stay separate because the schema is a shared file that lands first. The extra cost is a few constants.
4. Lock-in: named. The `reason` enum and `gate_*` fields are additive, and a change is a schema change by design; the symbol rule is one function; Left out says step 13 changes the label and `gate_hint` only.
5. Claim most likely false: adopted. "Every route" is restated with the nil cases (Decision 4); the empty `Close` is stated with its zone note and tested in entry 3 item 2 (`Close` of the input is empty and the figure is the calculator's).
6. Likeliest slip: adopted. The purity test of the new file is in `valueline_test.go` (the tier purity test covers only `tier*` files), so `purity_test.go` is not touched; entry 4 states why `evidence_site.go` is edited; web item 3 states the fixture result. The Chinese golden is a JSON file, outside the Go guard, and holds the constants' text as the page writes it.

## Left out

- Step 13 (the buy price source reaching the gate, owner question 3, answer a for now): the gate keeps the hand-typed price; the strip says "hand-typed" and changes with that step. Collection of contracts follows the buy price (`job.toml`), which is why the source is not changed here.
- The difference between B_hi and the hand-typed price (an amount or a percent), and a mark for which is lower: a computation the plan does not ask for and a judgement the owner wants to make by looking for some weeks; a later small design with `percentText` if he asks.
- A valuation close, zone or price axis on this page, and the valuation page's other lines (BuyLow, fair, trim, clear, core): not shown; only B_hi.
- How old a review may be: the line shows the review day and applies no age rule or stale mark.
- A currency conversion for a non-USD valuation (shown as the reason `currency`), and a profile match by instrument id instead of the symbol: the symbol is the name the page already has; a shared name is the reason `ambiguous`.
- The old stored tables and the `/options` overview: no valuation line there.
- Chinese hints of the existing strip fields and the legend: unchanged.
- A mark when the whole block is missing because no valuation reader exists (no pages line): the owner sees no valuation lines; the data-health page is where a missing store shows.
