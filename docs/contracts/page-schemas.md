# page schemas contract

## Responsibility

Hold the JSON schemas of the pages in `contracts/schemas/page.*.json`. The schemas are of two kinds: those written for this repository (their `$id` is `https://optdesk.invalid/schemas/page.<name>.json`) and 29 copies of `docs/schemas/<name>.v<N>.json` of the old option-trader repository at commit b00b4ed, the commit the web migration used. The copies are unchanged except that every `x-timezone` value is the literal string `owner` (a privacy rule of this public repository), and the week-2 pass writes new versions only.

The generated TypeScript types follow `docs/design/generated-index.md#generated-index`: one file `web/src/generated/<name>.ts` per type, generated from `page.<name>.json` with the banner on line 1 and `interface <title>` (the schema `title`), no index file, the set of types being the `.ts` files of the directory (`pnpm --dir web run generate:types [page.<name>]` regenerates them and creates the file of a named schema); the contract tests read the directories and no list.

## Inputs

- `contracts/schemas/carryover.sha256`: one line `<sha256>  contracts/schemas/<file>` for each of the 29 copies, in file name order. `page.macro_rates.v1.json` is not in it.
- `web/fixtures/<dir>/*.json`: the page fixtures that the migrated front end follows.

## Outputs

The schema ids with the `const` of their `schema_version`:

| schema id | schema_version |
|---|---|
| page.advice_page.v1 | 1 |
| page.companies_page.v1 | 1 |
| page.companies_page.v2 | 2 |
| page.flex_import.v1 | 1 |
| page.full_scan.v1 | 1 |
| page.held_companies_summary.v1 | 1 |
| page.home_page.v1 | 1 |
| page.home_page.v2 | 2 |
| page.household_page.v1 | 1 |
| page.household_risk.v1 | 1 |
| page.household_summary.v1 | 1 |
| page.indicator_detail.v1 | 1 |
| page.indicator_detail.v2 | 2 |
| page.job_console.v1 | 1 |
| page.ledger_page.v1 | 1 |
| page.macro_page.v1 | 1 |
| page.macro_rates.v1 | "1" (a string) |
| page.macro_rates.v2 | 2 |
| page.macro_rates.v3 | 3 |
| page.macro_summary.v1 | 1 |
| page.market_summary.v1 | 1 |
| page.market_weather.v1 | 1 |
| page.option_page.v1 | 1 |
| page.option_page.v2 | 2 |
| page.option_result_page.v1 | 1 |
| page.options_summary.v1 | 1 |
| page.positions_page.v1 | 1 |
| page.positions_page.v2 | 2 |
| page.positions_page.v3 | 3 |
| page.report_page.v1 | 1 |
| page.report_page.v2 | 2 |
| page.settings_page.v1 | 1 |
| page.settings_page.v2 | 2 |
| page.stock_option_page.v1 | 1 |
| page.stock_option_page.v2 | 2 |
| page.structure_detail.v1 | 1 |
| page.structure_detail.v2 | 2 |
| page.structure_detail.v3 | 3 |
| page.structures_list.v1 | 1 |
| page.tracking.v1 | 1 |
| page.tracking.v2 | 2 |
| page.underlyings.v1 | 1 |

Also `common.v1` (`contracts/schemas/common.v1.json`, `$id` `https://optdesk.invalid/schemas/common.v1.json`): only `$defs` `Id`, `Timestamp`, `RunStatus`, `SourceTime`, `StaleLabel`, `Gap`, `UsdFigure`, `PercentFigure`, `SharePercentFigure`, `UsdSignedFigure`, used as `common.v1.json#/$defs/<Name>`.

Also `common.v2` (`contracts/schemas/common.v2.json`, `$id` `https://optdesk.invalid/schemas/common.v2.json`) for the v3 line: only `$defs` `Id`, `Timestamp`, `RunStatus`, `SourceTime`, `Gap`, `StaleLabel`, `UsdFigure`, `PercentFigure`, `SharePercentFigure`, `UsdSignedFigure`, used as `common.v2.json#/$defs/<Name>`. `Gap` requires `kind` (`gap`), `reason` and `text`; `StaleLabel` requires `data_time`, `text` and `is_stale`; each figure requires `kind` (`value`); units are `usd`, `percent`, `share_percent`, `usd_signed`.

The v3 files, `schema_version` const 3: `page.market_summary.v3` (title `MarketSummaryV3`) and `page.options_summary.v3` (title `OptionsSummaryV3`); `stale_after` is required in both.

Also `page.option_data_page.v1` (`schema_version` 1, title `OptionDataPage`), written for this repository by `docs/design/option-page.md#interface`: the page of stored option data (chain of the last stored day and up to three quote series). The page name is `option_data_page`; it is not `option_page` (`page.option_page.v1` is a carried copy and is unchanged). It refs `common.v2.json` for `Id`, `Timestamp`, `RunStatus`, `Gap`, `SourceTime` and `StaleLabel`. Its fixtures are `contracts/fixtures/page.option_data_page.v1.{input,ok,terms,gap,stale,bad-version,choices,terms-input,jpy}.json`, listed in `MANIFEST.synthetic-v1.json` and checked by `TestOptionDataPageFixtures` (`choices` validates; an `href` with a scheme fails); the optional `underlyings` field (list of `UnderlyingChoice` `{id, label, href, current}`, `href` matching `^/[A-Za-z0-9/_-]{1,100}$`, absent when the builder has no choices) was added by `docs/design/option-page-serve.md#interface` without a version change; the optional `Underlying.vol_line`, `Underlying.earnings_text`, `Underlying.earnings_tone` (`in_term`, `outside`, `none`) and `Figure.approx_usd_text` (the USD approximation of a JPY figure) were added by `docs/design/option-pages-v22.md#interface` without a version change, with the fixture `jpy`; the optional `Underlying.off_session` (boolean, absent means false: the quotes of that underlying were taken off session) was added by `docs/design/option-chain-daily.md#2 option-eod-contract` without a version change and without a fixture; its type is generated into `web/src/generated/option_data_page.v1.ts` by `generate:types`. It is not in `carryover.sha256` (`classifyPages` in `carryover_test.go` takes a page file without a `carryover.sha256` line and with an `$id` of this repository as new).

Also four pages written for this repository by `docs/design/missing-pages.md#interface`, each `schema_version` 1, with the envelope of `page.option_data_page.v1` and `Id`, `Timestamp`, `RunStatus`, `SourceTime`, `StaleLabel` and `Gap` taken from `common.v2` by `$ref`. None is in `carryover.sha256`; none has a producer, a route or a TopNav item. Their types are generated into `web/src/generated/<name>.v1.ts` by `generate:types`.

| schema id | title | board | content |
|---|---|---|---|
| page.market_page.v1 | MarketPage | 7 | `weather` (`chart` a `Chart` of `ChartPoint {label, close, ma200}` or a `Gap`), `indices` (`IndexRow`, each figure a `Figure` or `Gap`), `term` (`TermPoint`) or a `Gap`, `breadth` (`BreadthRow`) or a `Gap` |
| page.price_lines_page.v1 | PriceLinesPage | 14 | `form` (`post_path` const `/plan/price-lines`, `expires`, `mac`), `rows` (`PriceLineRow`, `market` enum `US` `JP`), `add`, `unset` (`UnsetRow`), `saved`, `error`, `footnote` |
| page.core_page.v1 | CorePage | 15 | `basis_note`, `scale` (`treasury.marker_percent`), `rows` (`CoreRow`; `returns` 1 to 6 `ReturnPoint` or a `Gap`), `jp_note`, `legend`, `snapshot_text` |
| page.diagnosis_page.v1 | DiagnosisPage | 16 | `basis_note`, `groups` (`DiagGroup` with a string `key`; `DiagRow` of at most 12 `YearRow`, or a `Gap`) |

Numbers a producer will write say "metric id: assigned by compute" in their description; no schema computes anything.

Also `page.tracking.v2` (title `Tracking`) and `page.indicator_detail.v2` (title `IndicatorDetail`), written for this repository by `docs/design/tracking-household-reports.md#interface`; the v1 files are carried copies and stay byte-identical. Both follow the v2-line conventions and take `Gap`, `SourceTime` and `StaleLabel` from `common.v1`. `page.tracking.v2` replaces `rows` and `pending` by `versions` (1 to 10 `RuleVersion`, current first: `label`, `expired_text`, 1 to 6 `RuleGroup`, `covered_calls` or null, `my_trades` or a `Gap`); a `RuleGroup` has `key`, `title`, `summary` (1 to 8 `{label, text}` tiles or a `Gap`), `note` and `rows` (`TrackingRow`); `Usd` stays a local def. `page.indicator_detail.v2` keeps the v1 content and requires `meta_line` (the source line) and `recent` (a `RecentBlock` of at most 12 rows `{period, released, value, change}`, or a `Gap`). Their types are generated into `web/src/generated/tracking.v2.ts` and `web/src/generated/indicator_detail.v2.ts`; neither is in `carryover.sha256`.

## Invariants

Conventions of every file of the v2 line (`page.<name>.v2.json`, checked by `TestPageV2Conventions`):

1. `schema_version` is `{"const": 2}` with `"type": "integer"`, the number being that of the file name.
2. `stale_after` is an optional top-level property, `$ref` `Timestamp`: declared, not in `required`.
3. The page uses the shared defs of `common.v1` by `$ref` and has no local def of the same name, except the local `Gap` of `options_summary`.
4. No def name contains `__`; a def named `option_trader__viewmodels__<module>__<Name>` is renamed `<ModulePascal><Name>`, and one with a counterpart in `common.v1` becomes the `$ref`.
5. `ROUND_HALF_UP` does not occur; every `x-rounding` value is `ROUND_HALF_EVEN`. A money value stays a JSON number.
6. Every `x-timezone` value is `owner`.

Conventions of every file of the v3 line (`page.<name>.v3.json`, checked by `TestPageV3Conventions`; a missing file is skipped):

1. Shared defs come from `common.v2.json` by `$ref`, with no local def of their names, except the local `Gap` of `options_summary` (reasons `missing_quote`, `no_data`, `unsupported_currency`, `kind` `gap`).
2. The string `"null"` is not a type and there is no `anyOf`; every `oneOf` has branches that each require a `kind` with a distinct `const`.
3. No def name contains `__`, `ROUND_HALF_UP` does not occur, every `x-timezone` is `owner`, every `unit` const matches `^[a-z][a-z0-9_]*$`, and a money value is a JSON number.
4. `schema_version` is `{"const": 3, "type": "integer"}` and `stale_after` is in `required`; staleness is the flag `is_stale` of the stale label, set by the producer.
5. The fixtures of `web/fixtures/v3/<dir>` are checked by `TestFixturesV3Validate`, the generated types by `TestGeneratedV3Types`.

Invariants of the carried files:

- The 29 copies are unchanged except the `x-timezone` substitution (every value is `owner`, checked by the test); the hash of each edited copy is in the manifest, and `internal/contracts/carryover_test.go` fails on a file without a line, a line without a file, or a hash that differs.
- The keywords `x-decimals`, `x-timezone` and `x-rounding` are unknown to the compiler and ignored.
- Every fixture of `web/fixtures` validates against the schema that `internal/contracts/fixtures_test.go` maps its directory to (`positions-page` to `page.positions_page.v3.json`), except the `bad-version.json` files, which fail at `/schema_version` with the keyword `const`.
- No schema and no fixture is edited to make a fixture pass.
- `page.positions_page.v3.json` (edited in place, `schema_version` stays 3; `docs/design/pages-real-data.md#Interface`): `views` has 1 to 8 `PositionsView`; `key` is the account alias and `label` the account label, each a non-empty string of at most 64 characters; `RoleCell.text` and `value` are the account label of the row (non-empty, at most 64 characters). The enums of the three tables' column ids and `Gap.reason` are unchanged. The money `unit` names the kind of amount (`amount`, `amount_per_share`, `amount_signed`; the generic `Cell.unit` says `amount_signed`) and `currency` alone names the money (`docs/design/positions-currency-units.md`). A stock quantity (`StockQuantityCell`, unit `shares`) is a number with at most 6 places and its text drops trailing zeros; an option quantity (`OptionQuantityCell`, unit `contracts`) is an integer. Checked by `internal/contracts/positions_page_v3_test.go`.
- The `const` of `schema_version` is an integer in the copies and a string in `page.macro_rates.v1`; unifying it is for week 2.

## Version history

- v1: the 29 carried schemas (`page.macro_rates.v1` was added before them), the manifest and the fixture validation test.
- v2 line: `common.v1` is added; the `page.<name>.v2.json` files of the five summaries and `home_page` are added by later issues, and the existing files are unchanged.
- `page.home_page.v2` composes the five summary files by `$ref`: `page.household_summary.v2`, `page.held_companies_summary.v2` and `page.macro_summary.v2` (v2), `page.market_summary.v3` and `page.options_summary.v3` (v3). `BlockLink` is `{href, text}`.
- v2 TypeScript types: the types of `page.market_summary.v2` and `page.options_summary.v2` are generated into `web/src/generated/` by the web script `generate:types` (`json-schema-to-typescript` 16.0.0), not by `contracts/generate.sh`; fixtures `web/fixtures/market-summary` and `web/fixtures/options-summary` are checked against these files.
- v3 line: `common.v2`, `page.market_summary.v3` and `page.options_summary.v3` are added (tagged unions with a required `kind` on every figure and gap, `is_stale` set by the producer, lower snake case units, no `null`); the v2 files and `common.v1` are unchanged. Their types are generated into `web/src/generated/*.v3.ts` by `generate:types`; fixtures are in `web/fixtures/v3/`.
- Home page revision (`docs/design/home-page-revise.md#interface`): `page.home_page.v3` (`schema_version` 3; `overview` `{items: OverviewLine {label, text, href}[1..10], gaps: GapItem {card, text}[0..20]}` replaces `block_links`; refs `common.v2`, `page.held_companies_summary.v3` and `page.options_summary.v4`), `page.held_companies_summary.v3` (`Row` gains `price` of `PriceFigure` or `Gap`, `day_sharp` and `five_day_sharp` of `SharpMark` or `NoMark`; `Column.id` gains `price` and `Card.columns` has five entries; `Card` gains `sharp_note`; `rows` is an array only, `EarningsCell` gains `kind` `value`, so that every `oneOf` branch carries a `kind`) and `page.options_summary.v4` (`Card` gains required `closed_series_text`; `schema_version` 4, so the v3-line header rule of `schema_version` 3 does not apply to it). Types: `web/src/generated/{home_page.v3,held_companies_summary.v3,options_summary.v4}.ts`. The three v4 options fixtures `contracts/fixtures/page.options_summary.v4.{ok,gap,stale}.json` are listed in `MANIFEST.synthetic-v1.json` and checked by `TestFixturesV4Options`. Every v2 and v3 file is unchanged.
- Unified site (`docs/design/unified-site.md#interface`): `page.macro_rates.v3` is `page.macro_rates.v2` with `schema_version` 3 and `unit` gaining `index_points` (the five index series; text has two decimals and no suffix); it is not in the v3 line: `macro_rates` keeps the v2-line conventions under version 3, so `pageV3Files` of `TestPageV3Conventions` does not list it. `page.sample_page.v1` is the marker of a sample route, `{schema: "sample_page", schema_version: 1, page}` with `page` one of `home_page`, `household_page`, `positions_page`, `household_risk`, `option_page`, `advice_page`; it is closed and all three fields are required. Types: `web/src/generated/{macro_rates.v3,sample_page.v1}.ts`; fixtures `contracts/fixtures/page.macro_rates.v3.{golden,gap,stale,bad-version,seven}.json` and `page.sample_page.v1.{ok,bad-version}.json` are listed in `MANIFEST.synthetic-v1.json`. The v2 files and fixtures are unchanged.
- `page.companies_page.v2` (title `CompaniesPageV2`, `docs/design/valuation.md#Decisions` 12 and 13): `companies_page` v1 plus the required blocks `core` (`CoreRow` rows, the Treasury yield or a `Gap`, a Japan `jp_rate` `Gap`), `diagnosis` (`DiagGroup` groups) and `questions` (Q1 to Q4 `Figure` or `Gap`, owner text or `Gap`), each `anyOf` the block or `Gap`; a price line `Cell` gains `price_state` (`band`, `owner override`, `awaiting approval`, `expired`) and `hand_price`. `Gap`, `SourceTime`, `StaleLabel` come from `common.v1`. Its fixtures are `contracts/fixtures/page.companies_page.v2.{input,ok,gap}.json`, listed in `MANIFEST.synthetic-v1.json` and checked by `TestCompaniesPageV2Fixtures`: `input` is not a page but what the builder reads (subjects with a hand price, readings, facts, `series` with a `DGS10` point, `instruments`, quotes, option rows with strikes, owner text) and a `figures` section per symbol that fake compute functions return; `ok` is the page built from it with those fakes. Type: `web/src/generated/companies_page.v2.ts`. `page.companies_page.v1` is unchanged.

Also `page.option_candidates_page.v1` (`schema_version` 1, title `OptionCandidatesPage`), written for this repository by `docs/design/option-candidates-page.md#interface`: the stored short puts, covered calls and guard rows of the last run. The page name is `option_candidates_page`. It refs `common.v2.json` for `Id`, `Timestamp`, `RunStatus`, `Gap`, `SourceTime` and `StaleLabel`. Envelope as `market_page`; page fields `chain_day`, `computed_at`, `rule_version`, `estimate_note` and `groups` (1 to 3 `Group` of `kind` `short_put`, `covered_call`, `held_call_guard`, with `counts`, `main` and `other` `Table`s or null, and a `gap` that is non-null exactly when both tables are null). A `Reason` has `gate` (`^[a-z_]{1,32}$`), `status` (`block`, `unknown`), `code` (`^[a-z_]{1,48}$`) and `text`; gate and code are patterns, not enums. Checked by `internal/contracts/option_candidates_page_v1_test.go`; its type is generated into `web/src/generated/option_candidates_page.v1.ts` by `generate:types`. It is not in `carryover.sha256`; it has no producer, route or fixture yet.

Also `page.option_notices_page.v1` (`schema_version` 1, title `OptionNoticesPage`), written for this repository by `docs/design/option-notices.md#interface`: the notices of each account alias. The page name is `option_notices_page`. It refs `common.v2.json` for `Id`, `Timestamp`, `RunStatus`, `SourceTime` and `StaleLabel`. Envelope as `market_page`; page fields `day` (ISO date) and `accounts` (`AliasBlock` of `alias`, `main`, `status` `ok|no_capture|stale`, `data_day` or null, `notices`, `cash_by_date` of `{day, currency, amount_text}`, `cushion_text` or null, `note` or null). A `NoticeRow` has `id` (`^notice-[A-Za-z0-9_-]{1,64}$`), `code` (`^[a-z_]{1,32}$`), `severity` (`info`, `warn`, `act`), `title`, `contract`, `expiry` or null, `facts` of `{label, text}` and `reasons`; `code` is a pattern, not an enum, so a new code needs no schema change. Checked by `internal/contracts/option_notices_page_v1_test.go`; its type is generated into `web/src/generated/option_notices_page.v1.ts` by `generate:types`. It is not in `carryover.sha256`; it has no producer, route or fixture yet.

Also `page.option_tracking_page.v1` (`schema_version` 1, title `OptionTrackingPage`), written for this repository by `docs/design/option-tracking.md#interface`: per account the open short positions, the recent closes and the reconciliation checks of the ledger. The page name is `option_tracking_page`. It refs `common.v2.json` for `Id`, `Timestamp`, `RunStatus`, `Gap`, `SourceTime` and `StaleLabel`. Envelope as `market_page`; page fields `as_of`, `mark_note` and `accounts` (1 to 4 `Account` with `alias` (`^[a-z0-9_-]{1,16}$`), `open`, `recent` and `recon` `Table`s or null, and a `gap` that is non-null exactly when the three tables are null). Row ids start `pos-` in `open` and `recent`, `chk-` in `recon`. The `Table`, `Row` and `Cell` shapes are written out in this file, with no `$ref` to another page schema. Checked by `internal/contracts/option_tracking_page_v1_test.go`; its type is generated into `web/src/generated/option_tracking_page.v1.ts` by `generate:types`. It is not in `carryover.sha256`; it has no producer, route or fixture yet.

Quote and refresh (`docs/design/option-open-pull.md#3`, Decisions 18 to 20): `page.option_page.v2` and `page.stock_option_page.v2` are `page.option_page.v1` and `page.stock_option_page.v1` with `schema_version` 2, additively. Every row that carries a price gains a required `quote` object (`Quote`: `as_of`, exchange-local `HH:MM:SS` or a date text for a close archive value; `data_type`, `live`, `frozen`, `delayed` or null for a close archive value; `source`, `open_pull`, `refresh` or `close_archive`; the booleans `skew` and `outside_band`): the chain `Row` of `option_page`, and `ContractLine` and `MatrixCell` of `stock_option_page`. Each page gains a required `refresh` object (`Refresh`: `enabled`, `symbols` (at most 8), `notice` (the reason in words when `enabled` is false, or null), `fresh_count`, `total_count`). The builder copies these fields and the web renders them; nothing is computed. The ids use `https://optdesk.invalid/schemas/page.<name>.v2.json`; the v1 files are unchanged. Types: `web/src/generated/{option_page.v2,stock_option_page.v2}.ts`. The v1 type files `web/src/types/option_page.v1.types.ts` and `web/src/types/stock_option_page.ts` are outside the paths of this change and unchanged.

The `Contracts:` line of the pull request of these two files names them as new: `page.option_page.v2 v2 (new), page.stock_option_page.v2 v2 (new)`.

`page.stock_option_page.v3` (`docs/design/vol-edge.md#interface`, item 2, Decision 4): `schema_version` const 3, title `StockOptionPage`. `page.stock_option_page.v2` is kept unchanged. The v3 file is a root object of its own with the 21 root properties of v2 plus a required `vol_view` (`VolView` `{name, method, rows, coaching}`, `rows` at most 20 `VolRow` `{label, text, data_day, gap}`; `label` matches `^[a-z0-9_]{1,32}$`, `data_day` is a date or empty, `text` and `gap` at most 200 characters and may be empty; never null). Its members that v2 defines in `$defs` are refs `page.stock_option_page.v2.json#/$defs/<Name>` (no copy of the v2 `$defs`, so the generated file may hold `null`: it refs a v2 page file); its type is generated into `web/src/generated/stock_option_page.v3.ts` by `generate:types page.stock_option_page.v3`. The `Contracts:` line names it `page.stock_option_page.v3 v3 (new)`.

Also `page.underlyings.v1` (`schema_version` 1, title `Underlyings`), written for this repository by `docs/design/underlyings.md#interface`: the followed names with their switches, the owner form, the confirm step with its `stops` lines, and the history. `gaps` are `common.v2` `Gap`s. Type: `web/src/generated/underlyings.v1.ts`; fixtures `contracts/fixtures/page.underlyings.v1.{golden,confirm,readonly,bad-version}.json` and the follow table `contracts/fixtures/underlyings.follow.json` (the one table of Decision 6; `dividends` needs an issuer) are listed in `MANIFEST.synthetic-v1.json`.

Also `page.flex_import.v1` (`schema_version` 1, title `FlexImport`), written for this repository by `docs/design/flex-xml-import.md#6 flex-import-page-schema`: the upload form (null when the route is off), the results of one post (`files`) and the recent imports (`recent`). `status` is one of `stored`, `present`, `nothing_found`, `unconfigured`, `failed`; `reason` is one of `not_report`, `too_large`, `too_many_files`, `bad_gzip`, `store_error` or empty; the counts are integers of at least 0; `gaps` are `common.v2` `Gap`s. Type: `web/src/generated/flex_import.v1.ts`; fixtures `contracts/fixtures/page.flex_import.v1.{golden,result,disabled,bad-version}.json` are listed in `MANIFEST.synthetic-v1.json` and checked by `TestFlexImportV1Fixtures`.

Also `page.ops_today_page.v1` (`schema_version` 1, title `OpsTodayPage`), written for this repository by `docs/design/option-open-gap-jp.md#Same-morning visibility` (Decision 11, entry 5): the runs of the day in one section. The page name is `ops_today_page`. It refs `common.v2.json` for `Id`, `Timestamp`, `RunStatus`, `SourceTime` and `StaleLabel`. Envelope as `option_notices_page` (its `notes` hold the page notes); page fields `day` (ISO date, the session day) and `today` (`TodaySection`: `rows`). A `Row` has `dataset`, `key`, `market`, `status`, `error_code` or null, `reason_code` or null, `returned_n` and `usable_n` (integers, 0 or more, as the run recorded them) and `detail` or null; a `not_wired` row carries the NAME of the missing key in `detail`, never a value. Every text is the producer's: the schema holds no number the view must compute, no gap list and no ranking. Checked by `internal/contracts/ops_today_page_v1_test.go`; its type is generated into `web/src/generated/ops_today_page.v1.ts` by `generate:types`. It is not in `carryover.sha256`; it has no producer, route or fixture yet.

Also `page.job_console.v1` (`schema_version` 1, title `JobConsole`), written for this repository by `docs/design/job-console.md#Decisions` (Decision 12, entry 2): the job console of `ot serve --owner-edit`. Fields `schema`, `schema_version`, `generated_at`, `title`, `sentence`, `form` (null when the route is off, else `post_path`, `expires`, `mac`), `gaps` (a local `Gap` `{kind, reason, text}` whose `reason` is `job_requests_unavailable` or `no_such_request`; `common.v2` `Gap` cannot carry them), `jobs`, `requests` (the newest 20), `selected` (the request id of `?request=` or null) and `refresh_seconds` (5 or 0). A job has `id`, `kind` (`any_day`, `today_only`, `listed_only`), `today`, `note` or null, `force`, `force_why`, `why_text` and `needs_text`. A request has `id`, `job`, `day`, `force`, `state` (`queued`, `running`, `done`, `refused`, `failed`, `interrupted`, `withdrawn`), `state_text`, `at_text` (UTC, `2006-01-02 15:04 UTC`), `reason` and `keys`. A key line has `dataset`, `subject`, `outcome` (`already_present`, `waiting`, `skipped`, `not_run`, `failed`, `partial`, `produced`, `unconfirmed`, `nothing_found`), `outcome_text`, `present` and `reason`; the builder derives them and the view computes nothing. Type: `web/src/generated/job_console.v1.ts`; fixtures `contracts/fixtures/page.job_console.v1.{golden,bad-version}.json` are listed in `MANIFEST.synthetic-v1.json` and checked by `TestJobConsoleGolden` and `TestJobConsoleBadVersion`. The `Contracts:` line names it `page.job_console v1 (new)`.
