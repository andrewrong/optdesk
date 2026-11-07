# Home page: the home page composes the v2 summaries

Author: the optdesk designer (first draft)
Owner approved: (pending)
Revision: 1

Source: the moves of the household, held companies and macro cards to their v2 schemas (issues #269 to #271, pull requests 272 to 274,
`docs/design/web-views-wave-2.md`), and `docs/design/web-views-wave.md` Left out, which defers `home_page` ("it composes the five cards and
links to reports and blocks"). Read on main at fefd76d: `contracts/schemas/page.home_page.v1.json`, `contracts/schemas/page.*_summary.v2.json`,
`contracts/schemas/page.market_summary.v3.json`, `contracts/schemas/page.options_summary.v3.json`, `contracts/schemas/common.v1.json`,
`docs/contracts/page-schemas.md`, `internal/contracts/common_test.go`, `internal/contracts/generated_ts_test.go`, `internal/contracts/carryover_test.go`,
`internal/pages/api.go`, `internal/pages/macro_page/builder.go`, `web/src/shell/pages.tsx`, `web/src/shell/App.tsx`, `web/src/shell/TopNav.tsx`,
`web/src/pages/home/*.tsx`, `web/src/types/*_summary.ts`, `web/src/generated/index.ts`, `web/package.json`, `web/fixtures/v3/*`,
`web/test/pages/home/*-v2/fixtures/*`.

Goal: one `home_page` payload that holds the five summaries in the shape their own pages use today, one Go function that composes it from
five summary documents, and one web view that shows the five existing cards from it. Nothing is read from a v1 card definition any more.

## What main holds today

- The stranded part is the schema only. `contracts/schemas/page.home_page.v1.json` embeds its own copies of the five cards (the defs named
  `option_trader__viewmodels__<module>__Card` and `HouseholdSummary`, `HeldCompaniesSummary`, `MacroSummary`, `MarketSummary`,
  `OptionsSummary`). It holds the envelope of ADR 0034, the texts `title`, `say`, `gap_line` and the lists `block_links` and `report_links`.
  Its `schema_version` is `{"const": 1}`; it is in `carryover.sha256` and in `docs/contracts/page-schemas.md`.
- No consumer exists. No Go code builds `home_page` (`internal/pages` holds `macro_rates`, `macro_page`, `option_data_page` and `embed`), and no
  file under `web/src` or `web/test` names `home_page` or `block_links`. The five cards are separate pages: each `web/src/pages/home/*Card.tsx`
  exports `registration` with its own schema (`household_summary`, `held_companies_summary`, `macro_summary`, `market_summary`,
  `options_summary`) and takes the whole summary payload as `payload`. The shell shows one registered page per HTML file (`App.tsx`).
- Which version each card reads: household, held companies and macro read v2 (`SUPPORTED_VERSIONS = [2]` in `web/src/types/*_summary.ts`);
  market and options read v3 (`SUPPORTED_VERSIONS = [3]`). So there is no single line of versions to compose.
- `internal/contracts/common_test.go` lists `page.home_page.v2.json` in `pageV2Files` and skips it while the file does not exist, so the v2 name is
  already reserved by the tests; `carryover_test.go` skips every name of `pageV2Files`, so a v2 file needs no hash line.
- The route names of `BlockLink` (`companies`, `us-close`, `jp-close`, `us-open`, `jp-open`) match no path in `web/src/shell/TopNav.tsx` and no
  page of `web/src/pages`.

## Decisions

1. New version, not an amendment. `page.home_page.v2.json` is a new file; v1 stays byte-identical (it has no consumer, but it is in the carry-over
   manifest and the v2 name is reserved). The Contracts line says so.
2. A member is a whole summary payload. `household`, `companies`, `macro`, `market` and `options` are each a `$ref` to the root of the summary file
   its card reads (`page.household_summary.v2.json`, `page.held_companies_summary.v2.json`, `page.macro_summary.v2.json`,
   `page.market_summary.v3.json`, `page.options_summary.v3.json`). Reason: the five cards take a whole payload today, so the home view reuses
   them unchanged, and each member keeps its own `data_times`, `stale_after` and `run_status`. A member that does not validate as its own page
   is not valid inside the home page either. Rejected: `$ref` to each `Card` def (the cards would need the envelope fields again).
3. The v2 line, with members of two lines. The file follows the v2 conventions of `page-schemas.md` (const 2, optional `stale_after`, shared defs of
   `common.v1`, no def name with `__`); the market and options members are v3 files because those are the versions their cards read. A later
   `home_page` v3 may take all five from v3 once household, held companies and macro have one (Left out).
4. Own fields kept from v1: `title`, `say`, `gap_line`, `block_links`, `report_links`, with the local def `BlockLink` copied (it is the only local def).
   The envelope uses `Id`, `Timestamp`, `RunStatus` and `SourceTime` of `common.v1` by `$ref` and gains the optional `stale_after`.
5. The composing function is plain and takes documents, not readers. `internal/pages/home_page.Compose` takes the five member documents as bytes plus
   the identity and texts of the run, and returns the home payload. No code builds any of the five summaries yet, so nothing can feed the function in
   production; wiring it into `cmd/ot` waits for those builders (Left out). The function is not a `pages.Builder` (no store read) and `internal/pages/api.go`
   is not edited.
6. What `Compose` decides, nothing else: each member must carry the `schema` name and the `schema_version` of its slot and the `run_id`, `build_id` and `scope`
   of the run, else it returns an error naming the slot; `run_status` of the home is the worst of the five (`failed`, then `partial`, then `ok`);
   `data_times` is the members' lists in the order household, companies, macro, market, options with a repeated pair of source and time dropped, an error
   past 20 entries (the schema's cap); `stale_after` is the earliest `stale_after` among the members that have one, omitted when none has. The members are
   written through as they came (`json.RawMessage`), never rebuilt.
7. The view is one page file, `web/src/pages/home/HomePage.tsx`, registered as `home_page`; the glob of `web/src/shell/pages.tsx` finds it, so no entry edits
   the shell. It renders the five existing card components with `payload` set to the member, in a grid, under a header of `title` and `say`, with `gap_line`
   as an alert and the two link lists as buttons. It computes nothing and changes no card. It validates the top level with the version check of
   `web/src/types/home_page.ts` and each member with the `validate` of its own `web/src/types/<summary>.ts`; a member that fails throws, so the error boundary
   puts the fail-closed notice (the rule of `App.tsx`).
8. Links by route name. The view holds one table from route name to path (the payload carries no path: "the browser holds the path"). The five names have no
   page today, so the paths in the table are the owner's to confirm (open question in the pull request); a link whose page is not served yet is the
   owner's to accept, the view does not hide it.

## Modules

None changed. Every path below belongs to a row of `docs/design/modules.md` as it stands: `contracts/**`, `internal/contracts/**`,
`docs/contracts/page-schemas.md` and `web/src/generated/**` to `contracts`; `internal/pages/**` to `pages`; `web/**` to `web`. A new Go package
`internal/pages/home_page` makes `scripts/arch` add a component (Generated output of `home-page-compose`).

| issue | module | order |
|---|---|---|
| home-page-v2-schema | contracts (interface; one path of web, see below) | first |
| home-page-compose | pages | after home-page-v2-schema |
| home-page-view | web | after home-page-v2-schema, parallel with home-page-compose |

## Interface

Owner: contracts. Frozen by `home-page-v2-schema`, read by `home-page-compose` (the schema file validates its output in tests) and `home-page-view`
(the generated type `HomePage`).

- `contracts/schemas/page.home_page.v2.json`: `$id` `https://optdesk.invalid/schemas/page.home_page.v2.json`, title `HomePage`, `additionalProperties` false.
  Required: `schema` (const `home_page`), `schema_version` (const 2), `run_id`, `build_id`, `scope`, `generated_at`, `data_times`, `run_status`, `title`,
  `say`, `gap_line`, `block_links`, `report_links`, `household`, `companies`, `macro`, `market`, `options`. Optional: `stale_after` (`Timestamp`).
  The five members are the `$ref` of Decision 2; `block_links` and `report_links` hold at most 10 `BlockLink`.
- `web/src/generated/home_page.v2.ts` (`HomePage`), made by `json2ts` with the banner of the existing files; `index.ts` re-exports `HomePage`.

## Issues

### home-page-v2-schema (interface)

- Module: contracts, with one path of web, `web/package.json` (the judged two-module exception of `docs/design/web-views-wave-2.md` entry `pages-types-wave-2`:
  the generator script is shared, so the entry cannot be split). Size: S. Depends on: none.
- Order: first.
- Design: docs/design/home-page.md#Interface
- Files: contracts/schemas/page.home_page.v2.json, docs/contracts/page-schemas.md, internal/contracts/home_page_v2_test.go, internal/contracts/generated_ts_test.go, web/src/generated/home_page.v2.ts, web/src/generated/index.ts, web/package.json
- Goal: write the schema of the Interface section, its generated type and its tests. `page-schemas.md` gains the row `page.home_page.v2 | 2` and one line saying that
  the file composes the five summary files by `$ref` (v2 for household, held companies and macro; v3 for market and options). `home_page_v2_test.go` (new,
  `t.Parallel`, no sleep) is the contract test: `TestHomePageV2Valid` builds the smallest valid instance and validates it (the pattern overrides of the existing
  v2 tests may be copied); `TestHomePageV2MemberChecked` changes `schema_version` of one member to a value its file does not allow and expects a failure at that member's
  path; `TestHomePageV2NoV1Defs` reads the file and fails when the text holds `option_trader__`. `generated_ts_test.go` gains the row `{"home_page.v2.ts", "./home_page.v2", "HomePage"}`.
  `web/package.json` gains one `json2ts` call in `generate:types` with `--cwd ../contracts/schemas` and the banner of the others.
- 按 frontend-stack：用 json-schema-to-typescript 做 the generated type `HomePage`。

Acceptance:

1. `test -f contracts/schemas/page.home_page.v2.json` holds and `grep -c '"const": "home_page"' contracts/schemas/page.home_page.v2.json` prints 1;
   `grep -c 'page.household_summary.v2.json\|page.held_companies_summary.v2.json\|page.macro_summary.v2.json\|page.market_summary.v3.json\|page.options_summary.v3.json' contracts/schemas/page.home_page.v2.json`
   prints at least 5 and `grep -c 'option_trader__' contracts/schemas/page.home_page.v2.json` prints 0.
2. `go test ./internal/contracts -run 'TestPageV2Conventions|TestPageV2FilesCheck|TestHomePageV2' -v` shows `TestPageV2Conventions/page.home_page.v2.json` and
   `TestPageV2FilesCheck/page.home_page.v2.json` as PASS (not SKIP) and the three `TestHomePageV2` tests as PASS.
3. `git diff $(git merge-base HEAD origin/main) -- contracts/schemas/page.home_page.v1.json contracts/schemas/carryover.sha256` is empty, and
   `go test ./internal/contracts -run TestCarryoverManifest` passes.
4. `test -f web/src/generated/home_page.v2.ts` holds, `grep -c "Code generated from contracts/, DO NOT EDIT" web/src/generated/home_page.v2.ts` prints 1,
   `grep -c "export type" web/src/generated/index.ts` prints 13 (twelve before) and `grep -c "export interface HomePage" web/src/generated/home_page.v2.ts` prints 1.
5. `grep -o "page\.[a-z_]*\.v[0-9]*\.json" web/package.json | sort -u | wc -l` prints 15 (fourteen before) and `grep -c "home_page.v2.ts" internal/contracts/generated_ts_test.go` prints 1.
6. `grep -c "page.home_page.v2" docs/contracts/page-schemas.md` is at least 1.
7. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the seven paths of the Files line and no other path; the pull request body carries the line
   `Contracts: page.home_page v2 added (new version; v1 has no consumer and is unchanged)`.
8. The result matches the Interface part of `docs/design/home-page.md`, judged from the diff and the design file.

Checked by CI: `pnpm --dir web run generate:types` with `git diff --exit-code web/src/generated` (ci-web drift check), tsc, eslint, vitest and the build; ci-go (`go test ./internal/contracts`);
checks (the `Contracts:` line).

### home-page-compose

- Module: pages. Size: M. Depends on: home-page-v2-schema.
- Order: after home-page-v2-schema.
- Design: docs/design/home-page.md#Decisions
- Files: internal/pages/home_page/compose.go, internal/pages/home_page/compose_test.go, internal/pages/home_page/testdata/household.json, internal/pages/home_page/testdata/companies.json, internal/pages/home_page/testdata/macro.json, internal/pages/home_page/testdata/market.json, internal/pages/home_page/testdata/options.json
- Goal: the function of Decisions 5 and 6. Signature: `func Compose(in Input, generatedAt time.Time) ([]byte, error)`, with `Input` holding `RunID`, `BuildID`, `Scope`, `Zone *time.Location`, `Title`,
  `Say`, `GapLine`, `BlockLinks`, `ReportLinks` (a slice of `Link{Route, Text string}`) and the five members as `[]byte` fields `Household`, `Companies`, `Macro`, `Market`, `Options`.
  `generated_at` is `generatedAt` written in `Zone`, as `macro_page` does; a nil `Zone` is an error. The package reads no clock, no store and no file outside tests, and imports only the standard library
  (`go.mod` is not edited). The five testdata files are hand-written, smallest valid instances of the five member schemas with the run identity `run-test`, `build-test`, `scope-test`.
- Generated output: the new package `internal/pages/home_page` makes `scripts/arch` add a component to `arch.toml` and `.go-arch-lint.yml` (the same as `internal/pages/macro_page` in `docs/design/pages-builders.md`); the entry carries the regenerated files and no hand edit. Those two files belong to the repo module and are the only paths outside the Files line.

Acceptance:

1. `go test ./internal/pages/home_page -race -count=1` passes; `grep -c "t.Parallel()" internal/pages/home_page/compose_test.go` is at least the number of `func Test` lines in that file, and `grep -c "time.Sleep" internal/pages/home_page/compose_test.go` prints 0.
2. `TestComposeValidatesAgainstSchema` composes from the five testdata files and validates the output with `github.com/santhosh-tekuri/jsonschema/v6` (already in `go.mod`) against `contracts/schemas/page.home_page.v2.json`, loading the member files by their `$id` host as `internal/contracts/common_test.go` does.
3. `TestComposeMemberMismatch` is table-driven over the five slots and over three faults each (wrong `schema`, wrong `schema_version`, a `run_id` different from the run's) and expects an error that contains the slot name; `TestComposeNilZone` expects an error.
4. `TestComposeRunStatus` checks the worst-of rule on three cases (all `ok`; one `partial`; one `partial` and one `failed`); `TestComposeDataTimes` checks the order, the dropped repeat and the error past 20 entries; `TestComposeStaleAfter` checks the earliest of two, and the omission when no member has one.
5. `TestComposeKeepsMembers` checks that each member in the output is byte-equal, after JSON compaction, to its input.
6. `grep -c "internal/store\|time.Now\|os.ReadFile" internal/pages/home_page/compose.go` prints 0; `wc -l internal/pages/home_page/compose.go` shows under 600; `git diff $(git merge-base HEAD origin/main) -- internal/pages/api.go internal/pages/api_test.go cmd go.mod go.sum` is empty.
7. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the seven paths of the Files line plus `arch.toml` and `.go-arch-lint.yml`, and no other path.
8. The result matches the Decisions 5 and 6 of `docs/design/home-page.md`, judged from the code and the design file.

Checked by CI: ci-go (`go test ./...`, go-arch-lint, golangci-lint, the `arch.toml` against `modules.md` check); the privacy scan.

### home-page-view

- Module: web. Size: M. Depends on: home-page-v2-schema.
- Order: after home-page-v2-schema.
- Design: docs/design/home-page.md#Decisions
- Files: web/src/pages/home/HomePage.tsx, web/src/types/home_page.ts, web/test/pages/home/HomePage.test.tsx, web/test/pages/home/home-v2/fixtures/ok.ts, web/test/pages/home/home-v2/fixtures/bad-version.json, web/test/pages/home/home-v2/fixtures/bad-member.json, web/test/pages/home/home-v2/fixtures/MANIFEST.synthetic-v1.json
- Goal: Decisions 7 and 8. `web/src/types/home_page.ts` takes `Payload` from `HomePage` of `@/generated/home_page.v2` with `SUPPORTED_VERSIONS = [2]` and the placeholder check
  (the pattern of `web/src/types/macro_summary.ts`). `HomePage.tsx` exports `registration = { schema: 'home_page', page: page<Payload>(validate, HomePage) }`; its `validate` runs the top-level check, then the `validate`
  of `@/types/household_summary`, `held_companies_summary`, `macro_summary`, `market_summary` and `options_summary` on the five members, and returns the first gap. The component renders `HouseholdCard`, `HeldCompaniesCard`,
  `MacroCard`, `MarketSummaryCard` and `OptionsSummaryCard` (imported, not edited) with `payload` set to the member; one const `ROUTE_PATHS` maps the five route names to paths. `ok.ts` is typed `satisfies HomePage`
  and builds its five members from the `ok` modules of `household-v2`, `held-v2` and `macro-v2` (imported) and the `ok.json` of `web/fixtures/v3/market-summary` and `options-summary` (read with `readFileSync`, as
  `MarketSummaryCard.test.tsx` does). `bad-version.json` has a `schema_version` other than 2; `bad-member.json` has a valid top level and a `macro` member whose `schema_version` is not 2.
- 按 frontend-stack：用 shadcn card, alert and button (`components/ui`) 做 the header, the `gap_line` and the two link lists。
- 按 frontend-stack：用 vite 的 `import.meta.glob`（`web/src/shell/pages.tsx`, unchanged）做 the self-registration of `home_page`。

Acceptance:

1. `grep -c "export const registration" web/src/pages/home/HomePage.tsx` prints 1, `grep -c "schema: 'home_page'" web/src/pages/home/HomePage.tsx` prints 1, `grep -c "SUPPORTED_VERSIONS = \[2\]" web/src/types/home_page.ts` prints 1 and `grep -c "home_page.v2" web/src/types/home_page.ts` is at least 1.
2. `grep -c "HouseholdCard\|HeldCompaniesCard\|MacroCard\|MarketSummaryCard\|OptionsSummaryCard" web/src/pages/home/HomePage.tsx` is at least 10 (five imports, five uses); `grep -c "fetch(" web/src/pages/home/HomePage.tsx` prints 0; `wc -l web/src/pages/home/HomePage.tsx` shows under 600.
3. `grep -c "satisfies HomePage" web/test/pages/home/home-v2/fixtures/ok.ts` prints 1; `web/test/pages/home/home-v2/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly `bad-member.json`, `bad-version.json` and `ok.ts`; `grep -c "@" web/test/pages/home/home-v2/fixtures/*` prints 0 for each file (the privacy rule for fixtures).
4. `web/test/pages/home/HomePage.test.tsx` renders `HomePage` with `ok` and checks that for each of the five members a text read from the fixture is on the page (`ok.household.card.title`, `ok.companies.card.title`, `ok.macro.card.title`, `ok.market.card.title`, `ok.options.card.title` each named in the file), that `ok.title`, `ok.say` and `ok.gap_line` are on the page, and that each of `ok.block_links` and `ok.report_links` is rendered as a link whose text is the `text` field; `grep -c "ok\.\(household\|companies\|macro\|market\|options\)\.card\.title" web/test/pages/home/HomePage.test.tsx` prints at least 5.
5. The same test checks that `registration.page.validate` refuses `bad-version.json` and refuses `bad-member.json` with a gap, and that it accepts `ok`; it opens no port and sleeps never; a clock the cards need is set with `vi.useFakeTimers` and `vi.setSystemTime`.
6. `pnpm --dir web run test -- web/test/pages/home` passes, together with `web/test/shell` (the registration test finds `home_page` and no duplicate schema).
7. `git diff $(git merge-base HEAD origin/main) -- web/src/shell web/src/pages/home/HouseholdCard.tsx web/src/pages/home/HeldCompaniesCard.tsx web/src/pages/home/MacroCard.tsx web/src/pages/home/MarketSummaryCard.tsx web/src/pages/home/OptionsSummaryCard.tsx web/src/pages/home/StaleAlert.tsx web/src/pages/home/CardNotes.tsx web/src/pages/home/DataTimesFold.tsx web/no-compute-allow.txt web/deps-allowlist.txt web/src/generated` is empty, and `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
8. The result matches the Decisions 7 and 8 of `docs/design/home-page.md`, judged from the code and the design file.

Checked by CI: `pnpm install --frozen-lockfile`, then tsc (`pnpm --dir web run typecheck`), eslint, vitest and the build; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

## Left out

- Amending v1 in place: it has no consumer, but it sits in `carryover.sha256`, the tests already reserve the v2 name, and a new file leaves the carried line untouched.
- Embedding the `Card` defs instead of whole payloads (Decision 2): rejected, the cards would need the envelope fields again and could not be reused unchanged.
- A `home_page` v3 with all five members at v3: waits until household, held companies and macro have v3 files; it is cheap then, since the view imports member types from their own modules.
- Retiring `page.home_page.v1.json` and its hash line: owner's decision, after v2 is consumed.
- Builders of the five summaries and the wiring of `Compose` into `cmd/ot` and the site: no code builds any summary yet; each needs its own readers and design. Until then `Compose` is covered by tests only.
- Dropping the five individual card registrations once the home page exists: the cards stay registered as pages of their own; a change there is a later decision.
- Real paths of the five route names and the pages behind them: the table `ROUTE_PATHS` holds the paths the owner confirms; the report and company pages are other designs.
- Updating the `web/src/shell/TopNav.tsx` link list: it already points at `/home`.
- json2ts naming: nested defs named `Card` in the five members become `Card`, `Card1` and so on in `home_page.v2.ts`; nothing imports them (only `HomePage` is re-exported), so the names are left as the tool makes them.

## Adversarial

pending
