# Home page: the home page composes the v2 summaries

Author: the optdesk designer (first draft)
Owner approved: (pending)
Revision: 1

Source: the moves of the household, held companies and macro cards to their v2 schemas (issues #269 to #271, pull requests 272 to 274,
`docs/design/web-views-wave-2.md`), and `docs/design/web-views-wave.md` Left out, which defers `home_page` ("it composes the five cards and
links to reports and blocks"). Read on main at df7deae: `contracts/schemas/page.home_page.v1.json`, `contracts/schemas/page.*_summary.v2.json`,
`contracts/schemas/page.market_summary.v3.json`, `contracts/schemas/page.options_summary.v3.json`, `contracts/schemas/common.v1.json`,
`docs/contracts/page-schemas.md`, `internal/contracts/common_test.go`, `internal/contracts/generated_ts_test.go`, `internal/contracts/carryover_test.go`,
`internal/pages/api.go`, `internal/pages/macro_page/builder.go`, `web/src/shell/pages.tsx`, `web/src/shell/App.tsx`, `web/src/shell/TopNav.tsx`,
`web/src/pages/home/*.tsx`, `web/src/types/*_summary.ts`, `web/src/generated/index.ts`, `web/package.json`, `web/fixtures/v3/*`,
`web/test/pages/home/*-v2/fixtures/*`.

Goal: one `home_page` payload that holds the five summaries in the shape their own pages use today, one Go function that composes it from
five summary documents, and one web view that shows the five existing cards from it. Nothing is read from a v1 card definition any more.

Limits of this design, both stated so the goal is not read as a working page: nothing serves `/home` yet (`cmd/ot/site.go` serves `/` and `/options/data`;
`docs/design/option-page-serve.md` has the other top-bar routes answer 404), and nothing calls `Compose` (no builder of the five summaries exists). The page is
reachable only after the builders and a route land, each its own design.

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
- The route names of `BlockLink` in v1 (`companies`, `us-close`, `jp-close`, `us-open`, `jp-open`) match no path in `web/src/shell/TopNav.tsx` and no page of
  `web/src/pages`; `docs/design/option-page-serve.md` Decision 1 already chose that `ot` writes an `href` string, one route string in one Go file.

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
4. Own fields kept from v1: `title`, `say`, `gap_line`, `block_links`, `report_links`. The local def `BlockLink` (the only local def) changes: `route` (an enum of five names)
   becomes `href`, a string of 1 to 200 characters that starts with `/`, next to `text`, as `option-page-serve` Decision 1 does. v2 is a new file, so it may change the def.
   The envelope uses `Id`, `Timestamp`, `RunStatus` and `SourceTime` of `common.v1` by `$ref` and gains the optional `stale_after`.
5. The composing function is plain and takes documents, not readers. `internal/pages/home_page.Compose` takes the five member documents as bytes plus
   the identity and texts of the run, and returns the home payload. No code builds any of the five summaries yet, so nothing can feed the function in
   production; wiring it into `cmd/ot` waits for those builders (Left out). The function is not a `pages.Builder` (no store read) and `internal/pages/api.go`
   is not edited.
6. What `Compose` decides, nothing else: each member must carry the `schema` name and the `schema_version` of its slot and the `run_id`, `build_id` and `scope`
   of the run, else it returns an error naming the slot; `run_status` of the home is the worst of the five (`failed`, then `partial`, then `ok`);
   `data_times` is the members' lists in the order household, companies, macro, market, options with a repeated pair of source and time dropped, an error
   past 20 entries (the schema's cap); `stale_after` is the earliest `stale_after` among the members that have one, omitted when none has. The members are
   written through as they came (`json.RawMessage`), never rebuilt. `gap_line` is the caller's text and the schema requires at least one character, so `Compose`
   returns an error for an empty `GapLine`; on a healthy day the caller passes the fixed text `No gap today`, and the view shows it in the alert like any other.
7. The view is one page file, `web/src/pages/home/HomePage.tsx`, registered as `home_page`; the glob of `web/src/shell/pages.tsx` finds it, so no entry edits
   the shell. It renders the five existing card components with `payload` set to the member, in a grid, under a header of `title` and `say`, with `gap_line`
   as an alert and the two link lists as buttons. It computes nothing and changes no card. It validates the top level with the version check of
   `web/src/types/home_page.ts` and each member with the `validate` of its own `web/src/types/<summary>.ts`; a member that fails throws, so the error boundary
   puts the fail-closed notice (the rule of `App.tsx`).
8. Links by `href`. `BlockLink` carries the path the link goes to, written by whoever builds the payload; the view renders it as given and holds no route table, so the five
   route names and the open question about their paths are gone. A link whose page is not served yet is the owner's to accept; the view does not hide it.
9. What this locks in. The route set is open (an `href` is any path), so a new report route needs no new version. The members are pinned by `$ref` to two lines (v2 for
   household, held companies and macro; v3 for market and options), so a bump of any member schema needs `home_page` v3 and a change in `Compose`. The view checks only
   `schema_version` of a member (`web/src/types/placeholder.ts`); the full shape check of a member is the Go contract test.

## Modules

None changed. Every path below belongs to a row of `docs/design/modules.md` as it stands: `contracts/**`, `internal/contracts/**`,
`docs/contracts/page-schemas.md` and `web/src/generated/**` to `contracts`; `internal/pages/**` to `pages`; `web/**` to `web`. A new Go package
`internal/pages/home_page` makes `scripts/arch` add a component (Generated output of `home-page-compose`).

| issue | module | order |
|---|---|---|
| home-page-v2-schema | contracts (interface), plus web (judged two-module entry: the `generate:types` line in `web/package.json`) | after report-types (`docs/design/report-page.md`) |
| home-page-compose | pages (no caller yet) | after home-page-v2-schema |
| home-page-view | web | after home-page-v2-schema, parallel with home-page-compose |

## Interface

Owner: contracts. Frozen by `home-page-v2-schema`, read by `home-page-compose` (the schema file validates its output in tests) and `home-page-view`
(the generated type `HomePage`).

- `contracts/schemas/page.home_page.v2.json`: `$id` `https://optdesk.invalid/schemas/page.home_page.v2.json`, title `HomePage`, `additionalProperties` false.
  Required: `schema` (const `home_page`), `schema_version` (const 2), `run_id`, `build_id`, `scope`, `generated_at`, `data_times`, `run_status`, `title`,
  `say`, `gap_line`, `block_links`, `report_links`, `household`, `companies`, `macro`, `market`, `options`. Optional: `stale_after` (`Timestamp`).
  The five members are the `$ref` of Decision 2; `block_links` and `report_links` hold at most 10 `BlockLink`; `data_times` holds at most 20 `SourceTime` (the cap `Compose` enforces, Decision 6).
- `web/src/generated/home_page.v2.ts` (`HomePage`), made by `json2ts` with the banner of the existing files, through one new call in the `generate:types` script of `web/package.json` (the same entry writes both, so the CI drift check covers the file); `index.ts` re-exports `HomePage`. `BlockLink` is `{href, text}`.
- Size of the generated file: json2ts inlines each `$ref`, so the file is about 2,500 lines or more. Generated files are exempt from the 600-line rule: `scripts/ratchet/main.go`
  `isSource` skips a file whose first five lines hold a generated mark (`code generated`, `do not edit`, `@generated`), and every file of `web/src/generated` starts with
  `// Code generated from contracts/, DO NOT EDIT.`; AGENTS.md says "source file", and `structure_detail.v3.ts` (708 lines) is the precedent. So the schema keeps its `$ref`s and
  nothing is loosened.

## Issues

### home-page-v2-schema (interface)

- Module: contracts, plus web (judged two-module entry: the one `generate:types` line in `web/package.json` belongs to web, as `report-types` of `docs/design/report-page.md` and `pages-types-wave-2` of `docs/design/web-views-wave-2.md` do). Size: S. Depends on: report-types (by name; the dispatcher rewrites it to the issue number at cut time).
- Order: after report-types (`docs/design/report-page.md`). The chain is `macro-rates-v2-schema`, then `report-types`, then `home-page-v2-schema`, because all three append to
  `web/src/generated/index.ts` and `internal/contracts/generated_ts_test.go`; every count below is relative to main at cut time.
- Design: docs/design/home-page.md#Interface
- Files: contracts/schemas/page.home_page.v2.json, docs/contracts/page-schemas.md, internal/contracts/home_page_v2_test.go, internal/contracts/generated_ts_test.go, web/src/generated/home_page.v2.ts, web/src/generated/index.ts, web/package.json

- Goal: write the schema of the Interface section, its generated type and its tests. `page-schemas.md` gains the row `page.home_page.v2 | 2` and one line saying that
  the file composes the five summary files by `$ref` (v2 for household, held companies and macro; v3 for market and options). `home_page_v2_test.go` (new,
  `t.Parallel`, no sleep) is the contract test: `TestHomePageV2Valid` builds the smallest valid instance and validates it (the pattern overrides of the existing
  v2 tests may be copied); `TestHomePageV2MemberChecked` changes `schema_version` of one member to a value its file does not allow and expects a failure at that member's
  path; `TestHomePageV2NoV1Defs` reads the file and fails when the text holds `option_trader__`. `generated_ts_test.go` gains the row `{"home_page.v2.ts", "./home_page.v2", "HomePage"}`.
  `web/package.json` gains one `json2ts` call in `generate:types` for `page.home_page.v2.json` with `--cwd ../contracts/schemas` and the banner of the others; `home_page.v2.ts` is the output of that call, so this entry's own drift check covers it and `home-page-view` does not touch `web/package.json`.
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
   `grep -c "export type" web/src/generated/index.ts` prints one more than `git show origin/main:web/src/generated/index.ts | grep -c "export type"` and `grep -c "export interface HomePage" web/src/generated/home_page.v2.ts` prints 1.
5. `grep -c "home_page.v2.ts" internal/contracts/generated_ts_test.go` prints 1; `grep -c "home_page.v2.json" web/package.json` prints 1 and `grep -o "page\.[a-z_]*\.v[0-9]*\.json" web/package.json | sort -u | wc -l` prints one more than the same command on `git show origin/main:web/package.json`.
6. `grep -c "page.home_page.v2" docs/contracts/page-schemas.md` is at least 1.
7. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the seven paths of the Files line and no other path.

## Checked by CI

Checked by CI: `pnpm --dir web run generate:types` with `git diff --exit-code web/src/generated` (ci-web drift check: it regenerates `home_page.v2.ts` from the `generate:types` call this entry adds, so the file is checked here; json2ts cannot run in an offline sandbox, so no acceptance item claims it), tsc, eslint, vitest and the build; ci-go (`go test ./internal/contracts`); checks (the pull request body carries the line `Contracts: page.home_page v2 (new)` from the opening, the form `.github/workflows/checks.yml` accepts; v1 has no consumer and stays unchanged); the design verdict (the result matches the Interface part of `docs/design/home-page.md`, a judgement of the diff).

### home-page-compose

- Module: pages. Size: M. Depends on: home-page-v2-schema.
- Order: after home-page-v2-schema. `Compose` is a pure function and has no caller yet (the builders and the route are Left out); the owner wants this backend work in parallel with the view.
- Design: docs/design/home-page.md#Decisions
- Files: internal/pages/home_page/compose.go, internal/pages/home_page/compose_test.go, internal/pages/home_page/testdata/household.json, internal/pages/home_page/testdata/companies.json, internal/pages/home_page/testdata/macro.json, internal/pages/home_page/testdata/market.json, internal/pages/home_page/testdata/options.json, arch.toml, .go-arch-lint.yml

- Goal: the function of Decisions 5 and 6. Signature: `func Compose(in Input, generatedAt time.Time) ([]byte, error)`, with `Input` holding `RunID`, `BuildID`, `Scope`, `Zone *time.Location`, `Title`,
  `Say`, `GapLine`, `BlockLinks`, `ReportLinks` (a slice of `Link{Href, Text string}`) and the five members as `[]byte` fields `Household`, `Companies`, `Macro`, `Market`, `Options`.
  `generated_at` is `generatedAt` written in `Zone`, as `macro_page` does; a nil `Zone` is an error. The package reads no clock, no store and no file outside tests, and imports only the standard library
  (`go.mod` is not edited). The five testdata files are hand-written, smallest valid instances of the five member schemas with the run identity `run-test`, `build-test`, `scope-test`.
- Generated output: the new package `internal/pages/home_page` makes `scripts/arch` add a component to `arch.toml` and `.go-arch-lint.yml` (the same as `internal/pages/macro_page` in `docs/design/pages-builders.md`); the entry carries the regenerated files and no hand edit. Those two files are in the Files line (as in `docs/design/pages-builders.md`).

Acceptance:

1. `go test ./internal/pages/home_page -race -count=1` passes; `grep -c "t.Parallel()" internal/pages/home_page/compose_test.go` is at least the number of `func Test` lines in that file, and `grep -c "time.Sleep" internal/pages/home_page/compose_test.go` prints 0.
2. `TestComposeValidatesAgainstSchema` composes from the five testdata files and validates the output with `github.com/santhosh-tekuri/jsonschema/v6` (already in `go.mod`) against `contracts/schemas/page.home_page.v2.json`, loading the member files by their `$id` host as `internal/contracts/common_test.go` does.
3. `TestComposeMemberMismatch` is table-driven over the five slots and over three faults each (wrong `schema`, wrong `schema_version`, a `run_id` different from the run's) and expects an error that contains the slot name; `TestComposeNilZone` expects an error.
4. `TestComposeRunStatus` checks the worst-of rule on three cases (all `ok`; one `partial`; one `partial` and one `failed`); `TestComposeDataTimes` checks the order, the dropped repeat and the error past 20 entries; `TestComposeStaleAfter` checks the earliest of two, and the omission when no member has one.
5. `TestComposeKeepsMembers` checks that each member in the output is byte-equal, after JSON compaction, to its input.
6. `grep -c "internal/store\|time.Now\|os.ReadFile" internal/pages/home_page/compose.go` prints 0; `wc -l internal/pages/home_page/compose.go` shows under 600; `git diff $(git merge-base HEAD origin/main) -- internal/pages/api.go internal/pages/api_test.go cmd go.mod go.sum` is empty.
7. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the nine paths of the Files line and no other path.

## Checked by CI

Checked by CI: ci-go (`go test ./...`, go-arch-lint, golangci-lint, the `arch.toml` against `modules.md` check); the privacy scan; the design verdict (the result matches Decisions 5 and 6 of `docs/design/home-page.md`, a judgement of the code).

### home-page-view

- Module: web. Size: M. Depends on: home-page-v2-schema.
- Order: after home-page-v2-schema.
- Design: docs/design/home-page.md#Decisions
- Files: web/src/pages/home/HomePage.tsx, web/src/types/home_page.ts, web/test/pages/home/HomePage.test.tsx, web/test/pages/home/home-v2/fixtures/ok.ts, web/test/pages/home/home-v2/fixtures/bad-version.json, web/test/pages/home/home-v2/fixtures/bad-member.json, web/test/pages/home/home-v2/fixtures/MANIFEST.synthetic-v1.json

- Goal: Decisions 7 and 8. `web/src/types/home_page.ts` takes `Payload` from `HomePage` of `@/generated/home_page.v2` with `SUPPORTED_VERSIONS = [2]` and the placeholder check
  (the pattern of `web/src/types/macro_summary.ts`). `HomePage.tsx` exports `registration = { schema: 'home_page', page: page<Payload>(validate, HomePage) }`; its `validate` runs the top-level check, then the `validate`
  of `@/types/household_summary`, `held_companies_summary`, `macro_summary`, `market_summary` and `options_summary` on the five members, and returns the first gap. The component renders `HouseholdCard`, `HeldCompaniesCard`,
  `MacroCard`, `MarketSummaryCard` and `OptionsSummaryCard` (imported, not edited) with `payload` set to the member; each of `block_links` and `report_links` is a link to its `href`, with no route table. `web/package.json` and `web/src/generated/home_page.v2.ts` are not edited here: the generated file and the `generate:types` call come from `home-page-v2-schema`. `ok.ts` is typed `satisfies HomePage`, imports `HomePage` by the relative path `../../../../../src/generated/home_page.v2` and not by `@/generated/home_page.v2` (the fixture files hold no `@`, acceptance 3; the pattern of `web-views-wave-2.md` Decision 4 and `report-page.md`), and builds its five members from the `ok` modules of `household-v2`, `held-v2` and `macro-v2` (imported) and the `ok.json` of `web/fixtures/v3/market-summary` and `options-summary` (read with `readFileSync`, as
  `MarketSummaryCard.test.tsx` does). `bad-version.json` has a `schema_version` other than 2; `bad-member.json` has a valid top level and a `macro` member whose `schema_version` is not 2.
- 按 frontend-stack：用 shadcn card, alert and button (`components/ui`) 做 the header, the `gap_line` and the two link lists。
- 按 frontend-stack：用 vite 的 `import.meta.glob`（`web/src/shell/pages.tsx`, unchanged）做 the self-registration of `home_page`。

Acceptance:

1. `grep -c "export const registration" web/src/pages/home/HomePage.tsx` prints 1, `grep -c "schema: 'home_page'" web/src/pages/home/HomePage.tsx` prints 1, `grep -c "SUPPORTED_VERSIONS = \[2\]" web/src/types/home_page.ts` prints 1 and `grep -c "home_page.v2" web/src/types/home_page.ts` is at least 1.
2. `grep -c "HouseholdCard\|HeldCompaniesCard\|MacroCard\|MarketSummaryCard\|OptionsSummaryCard" web/src/pages/home/HomePage.tsx` is at least 10 (five imports, five uses); `grep -c "fetch(" web/src/pages/home/HomePage.tsx` prints 0; `wc -l web/src/pages/home/HomePage.tsx` shows under 600.
3. `grep -c "satisfies HomePage" web/test/pages/home/home-v2/fixtures/ok.ts` prints 1; `web/test/pages/home/home-v2/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly `bad-member.json`, `bad-version.json` and `ok.ts`; `grep -c "@" web/test/pages/home/home-v2/fixtures/*` prints 0 for each file (the privacy rule for fixtures).
4. `web/test/pages/home/HomePage.test.tsx` renders `HomePage` with `ok` and checks that for each of the five members a text read from the fixture is on the page (`ok.household.card.title`, `ok.companies.card.title`, `ok.macro.card.title`, `ok.market.card.title`, `ok.options.card.title` each named in the file), that `ok.title`, `ok.say` and `ok.gap_line` are on the page, and that each of `ok.block_links` and `ok.report_links` is rendered as a link whose text is the `text` field and whose `href` attribute is the `href` field; `grep -c "ROUTE_PATHS" web/src/pages/home/HomePage.tsx` prints 0; `grep -c "ok\.\(household\|companies\|macro\|market\|options\)\.card\.title" web/test/pages/home/HomePage.test.tsx` prints at least 5.
5. The same test checks that `registration.page.validate` refuses `bad-version.json` and refuses `bad-member.json` with a gap, and that it accepts `ok`; it opens no port and sleeps never; a clock the cards need is set with `vi.useFakeTimers` and `vi.setSystemTime`.
6. `pnpm --dir web run test -- web/test/pages/home` passes, together with `web/test/shell` (the registration test finds `home_page` and no duplicate schema).
7. `git diff $(git merge-base HEAD origin/main) -- web/package.json web/src/shell web/src/pages/home/HouseholdCard.tsx web/src/pages/home/HeldCompaniesCard.tsx web/src/pages/home/MacroCard.tsx web/src/pages/home/MarketSummaryCard.tsx web/src/pages/home/OptionsSummaryCard.tsx web/src/pages/home/StaleAlert.tsx web/src/pages/home/CardNotes.tsx web/src/pages/home/DataTimesFold.tsx web/no-compute-allow.txt web/deps-allowlist.txt web/src/generated` is empty, and `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.

## Checked by CI

Checked by CI: `pnpm install --frozen-lockfile`, then tsc (`pnpm --dir web run typecheck`), eslint, vitest and the build; hygiene (the manifest of the new `fixtures` directory and the privacy scan); the design verdict (the result matches Decisions 7 and 8 of `docs/design/home-page.md`, a judgement of the code).

## Left out

- Amending v1 in place: it has no consumer, but it sits in `carryover.sha256`, the tests already reserve the v2 name, and a new file leaves the carried line untouched.
- Embedding the `Card` defs instead of whole payloads (Decision 2): rejected, the cards would need the envelope fields again and could not be reused unchanged.
- A `home_page` v3 with all five members at v3: waits until household, held companies and macro have v3 files; it is cheap then, since the view imports member types from their own modules.
- Retiring `page.home_page.v1.json` and its hash line: owner's decision, after v2 is consumed.
- Builders of the five summaries and the wiring of `Compose` into `cmd/ot` and the site: no code builds any summary yet; each needs its own readers and design. Until then `Compose` has no caller and is covered by tests only.
- Dropping the five individual card registrations once the home page exists: the cards stay registered as pages of their own; a change there is a later decision.
- A route table and the real paths of the five old route names: dropped with `route` (Decision 8); the report and company pages are other designs.
- Updating the `web/src/shell/TopNav.tsx` link list: it already points at `/home`.
- json2ts naming: nested defs named `Card` in the five members become `Card`, `Card1` and so on in `home_page.v2.ts`; nothing imports them (only `HomePage` is re-exported), so the names are left as the tool makes them. The inlined size of the file is covered in the Interface section.

## Adversarial

Read in full in `home-page.adversarial.md`, in the dispatcher's scratchpad. The dispatcher's decisions:

1. Gap against the goal: adopted. The Goal names both limits (nothing serves `/home`, nothing calls `Compose`) and Decision 6 says what `gap_line` holds on a healthy day.
2. Size of the generated file: adopted as a check. Generated files are exempt from the 600-line rule (evidence in the Interface section), so the schema keeps its `$ref`s.
3. Simpler way: adopted in part. `BlockLink` carries `href` (Decision 4, 8), `ROUTE_PATHS` and the owner question are dropped; `home-page-compose` stays (backend work in parallel, a pure function with no caller yet).
4. Lock-in: adopted. Decision 9 names it.
5. Contracts line: adopted. The interface entry carries `Contracts: page.home_page v2 (new)`, counts are relative to main at cut time, and the compose entry lists `arch.toml` and `.go-arch-lint.yml`.
- (b) adopted: `home-page-v2-schema` is a judged two-module entry (contracts and web) and carries the `generate:types` line with the generated file, so its drift check covers `home_page.v2.ts`; `home-page-view` has no `web/package.json`.
- (c) adopted: `home-page-v2-schema` is ordered after `report-types` (`docs/design/report-page.md`) and depends on it by name.
