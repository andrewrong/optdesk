# Web: a second wave of self-registering page views

Author: advisor of optdesk
Source: owner request of 2026-11-07, as much parallel front-end work as the schemas honestly allow (the queue runs up to 8 at once).
Sequel to `docs/design/web-views-wave.md`. Read on main at b203a06: `web/src/shell/pages.tsx`, `web/src/types/*.ts`,
`web/src/generated/*`, `web/package.json` (`generate:types`), `internal/contracts/generated_ts_test.go`, `web/src/pages/home/*Card.tsx`,
`web/test/pages/home/*`, `web/fixtures/*`, `contracts/schemas/page.*.json`, `frontend-whitelist.json`, `docs/design/modules.md`,
`docs/design/option-page-serve.md`.

Depends on: `web-views-wave.md`, whose interface entry `pages-self-registering` (issue 241, pull request 245) landed at b203a06. A page file
under `web/src/pages/<dir>/` that exports `registration` is found by one glob, so no entry here edits `web/src/shell/pages.tsx`.

## What main holds today

- `pages.tsx` holds one glob `../pages/*/*.tsx` and `explicit` with `option_data_page` only. The registry is keyed by the schema name, not by
  the version: a schema registered twice throws at load, so a second view of an existing schema cannot sit beside the first. A new version of a
  page is a move of its view (`SUPPORTED_VERSIONS = [3]` in `web/src/types/market_summary.ts` is the precedent, pull request 159).
- Every page schema of `contracts/schemas` has a view, a view in the first wave, or a reason in `## Left out`, except three: `household_summary`,
  `held_companies_summary` and `macro_summary` have a v2 schema (made by `page-schemas-v2.md`) and their home cards (`HouseholdCard.tsx`,
  `HeldCompaniesCard.tsx`, `MacroCard.tsx`) still read v1 with hand-written placeholder types (`web/src/types/*.v1.types.ts`).
  No Go or Python code builds any of the three yet (only `internal/contracts` tests and fixtures name them), so a move breaks no producer.
- `web/src/generated/` has no v2 type of the three; `generate:types`, `web/src/generated/index.ts` and `TestGeneratedSummaryTypes` /
  `TestGeneratedV3Types` in `internal/contracts/generated_ts_test.go` are the shared files that list a generated type (wave 1, Decision 3).
- Fixtures of the three cards are v1 JSON in `web/fixtures/{household-summary,held-companies-summary,macro-summary}/`, listed in the shared
  manifest and read by Go tests; the card tests read them.
- The other schemas with no registered view are named in `## Left out`; no sixth or seventh candidate passed the tests of Decision 1.

## Decisions

1. Which pages. An entry is in the cut when its schema exists, its page is not excluded by `web-views-wave.md` (`settings_page`, `report_page`,
   `home_page`, `market_weather` as a page), the payload carries the `schema` field the shell dispatches on, no other design owns it, no older view
   of the same schema stays beside it, and the entry's files belong to no other entry. That gives three, not six: the v2 moves of the three summary cards. The
   owner asked for about six; the rest of the schemas fail one test each (`## Left out`), and a padded entry would be dishonest.
2. A version move, not a new page. The registry key is the schema name, so each card file keeps its `registration` and changes its type file to v2 with
   `SUPPORTED_VERSIONS = [2]`, as `market_summary` did. v1 support is dropped: no producer of v1 exists.
3. One interface entry for the generated types. The generator script, `index.ts` and the Go list are shared files, so the three v2 types come from one entry
   (`pages-types-wave-2`), the way wave 1 did for six. The three card entries then need no contracts path and no `web/package.json`.
4. Fixtures go in a `fixtures` directory of the card's own test subdirectory, `web/test/pages/home/<dir>/fixtures/` (`household-v2`, `held-v2`, `macro-v2`),
   each with its own manifest; `scripts/hygiene` needs the directory to be named `fixtures`, and three directories under `web/test/pages/home/` keep them apart. Fixtures are typed
   `.ts` literals (`satisfies` the generated type) with one untyped `bad-version.json`, as wave 1 Decision 4 and its `## Left out` explain. The v1 fixtures of `web/fixtures/` are not touched.
5. The shared helpers of the home cards (`StaleAlert.tsx`, `CardNotes.tsx`, `DataTimesFold.tsx`) are read by all three entries and edited by none; a needed change is reported, not made.
6. Each card entry owns its own test file in `web/test/pages/home/`. One other test file reads a card: `web/test/pages/household/householdPage.test.tsx` renders
   `HouseholdCard` with a v1 fixture, so it belongs to `household-summary-v2` and to no other entry.
7. A view is read-only and computes nothing (AGENTS.md); the three cards already follow this and keep doing so.

## Modules

No row is added or changed in `docs/design/modules.md`, so `arch.toml` is unchanged. Paths used: `web/**` and `web/test/**` belong to web;
`web/src/generated/**` and `internal/contracts/**` belong to contracts.

| issue | module | paths |
|---|---|---|
| pages-types-wave-2 (interface) | web, plus contracts (judged two-module entry, Decision 3) | `web/package.json`, `web/src/generated/**`, `internal/contracts/generated_ts_test.go` |
| household-summary-v2 | web | `web/src/pages/home/HouseholdCard.tsx`, `web/src/types/household_summary.ts`, `web/test/pages/home/HouseholdCard.test.tsx`, `web/test/pages/household/householdPage.test.tsx`, `web/test/pages/home/household-v2/**` |
| held-companies-summary-v2 | web | `web/src/pages/home/HeldCompaniesCard.tsx`, `web/src/types/held_companies_summary.ts`, `web/test/pages/home/HeldCompaniesCard.test.tsx`, `web/test/pages/home/held-v2/**` |
| macro-summary-v2 | web | `web/src/pages/home/MacroCard.tsx`, `web/src/types/macro_summary.ts`, `web/test/pages/home/MacroCard.test.tsx`, `web/test/pages/home/macro-v2/**` |

Order: `pages-types-wave-2` first; then the three cards, independent of each other and of the first-wave views (up to three at once, no shared file). Needing a
file outside an issue's Files is reported in the pull request description, not edited. A change under `web/` that touches no shared file does not wait for the six
entries of `web-views-wave.md`: none of them edits a file named here.

## Interface

Owner: contracts (the generated types). Frozen by `pages-types-wave-2`, read by the three cards. The entry is needed because the generated types are a
contracts output (`web/src/generated/**`) and the lists that name them are shared files; the views use no other new interface. The registration
of `web-views-wave.md` (`registration: { schema: string; page: Page }`, found by the glob, landed with issue 241) is used unchanged.

- `web/src/generated/` gains `household_summary.v2.ts` (`HouseholdSummary`), `held_companies_summary.v2.ts` (`HeldCompaniesSummary`) and `macro_summary.v2.ts`
  (`MacroSummary`), each made by `json2ts` with the banner of the existing files (names are the `title` of each schema; the entry checks them against the generated file and states them
  in its pull request if json2ts derives others); `index.ts` re-exports the three top-level names; `generate:types` lists the three.

## Issues

### pages-types-wave-2 (interface)

- Module: web and contracts. Size: S. Depends on: none.
- Design: Decision 3.
- Files: web/package.json, web/src/generated/**, internal/contracts/generated_ts_test.go
- What: add three `json2ts` calls to `generate:types` (each with `--cwd ../contracts/schemas`, as the existing ones, so the `common.v1.json` references resolve); commit the three generated files and the `index.ts` lines;
  extend `TestGeneratedSummaryTypes` with the three rows (its check of banner, top-level name, properties, consts and enums, and no `null`, applies to them). No new package: `json-schema-to-typescript` is whitelisted.
- 按 frontend-stack：用 json-schema-to-typescript 做 the three generated page types。

Acceptance:

1. `test -f` holds for `web/src/generated/household_summary.v2.ts`, `web/src/generated/held_companies_summary.v2.ts` and `web/src/generated/macro_summary.v2.ts`, and `grep -c "Code generated from contracts/, DO NOT EDIT" <file>` prints 1 for each.
2. `grep -c "export type" web/src/generated/index.ts` prints 12 (nine before, three after) and `grep -o "page\.[a-z_]*\.v[0-9]*\.json" web/package.json | sort -u | wc -l` prints 14 (eleven before, three after).
3. `grep -c "household_summary.v2.ts" internal/contracts/generated_ts_test.go` prints 1, and the same for `held_companies_summary.v2.ts` and `macro_summary.v2.ts`.
4. `git diff --name-only origin/main -- contracts/schemas contracts/proto contracts/datasets` prints nothing (the pull request says `Contracts: unchanged`) and `git diff --name-only origin/main` lists no path outside the Files line.
5. The result matches its part of `docs/design/web-views-wave-2.md` (Interface and Decision 3), judged from the code and the design file.

Checked by CI: `pnpm install --frozen-lockfile`, `pnpm --dir web run generate:types` with `git diff --exit-code web/src/generated` (ci-web drift check), tsc, eslint, vitest, the build; ci-go (`go test ./internal/contracts`); checks (the `Contracts:` line).

### household-summary-v2

- Module: web. Size: S. Depends on: `pages-types-wave-2` only.
- Design: Decisions 1, 2 and 4.
- Files: web/src/pages/home/HouseholdCard.tsx, web/src/types/household_summary.ts, web/test/pages/home/HouseholdCard.test.tsx, web/test/pages/household/householdPage.test.tsx, web/test/pages/home/household-v2/**
- What: the 家庭概览 card moves from `household_summary` v1 to v2: `web/src/types/household_summary.ts` takes `Payload` from `HouseholdSummary` of `@/generated/household_summary.v2` with `SUPPORTED_VERSIONS = [2]` (the pattern of `web/src/types/market_summary.ts`), and `HouseholdCard.tsx` reads the v2 shape (the `Gap` and the ids come from `common.v1.json`, the figure defs of v1 are not defs of v2, `stale_after` is a top-level field). The card keeps showing the fields of the payload and computes nothing. `web/test/pages/household/householdPage.test.tsx` renders `HouseholdCard` once with a v1 fixture (line 215); that line reads the v2 `ok` fixture of this entry instead and the rest of that file is left as it is. Four fixtures are written from the schema alone in `web/test/pages/home/household-v2/fixtures/`, in one mechanism: `ok`, `gap` and `stale` are TypeScript modules `ok.ts`, `gap.ts`, `stale.ts`, each `export const <name> = { ... } satisfies HouseholdSummary` (a literal, so `tsc` checks its shape; `schema` is `'household_summary'` and `schema_version` is 2); `bad-version.json` is plain JSON with `schema_version` set to another number, is not typed and is read with `readFileSync` and `JSON.parse`. `HouseholdCard.test.tsx` reads these four and no longer reads `web/fixtures/household-summary/`; the v1 fixtures stay for the Go tests.
- 按 frontend-stack：用 shadcn card and badge 做 the card, the four tiles (`net_worth`, `day_change`, `month_to_date`, `cash`) and the gap badges。
- 按 frontend-stack：用 shadcn alert, through the shared `StaleAlert` 做 the stale label。

Acceptance:

1. `grep -c "satisfies HouseholdSummary" web/test/pages/home/household-v2/fixtures/ok.ts web/test/pages/home/household-v2/fixtures/gap.ts web/test/pages/home/household-v2/fixtures/stale.ts` prints 1 for each, `test -f web/test/pages/home/household-v2/fixtures/bad-version.json` holds, and `grep -c "SUPPORTED_VERSIONS = \[2\]" web/src/types/household_summary.ts` prints 1.
2. `grep -c "household_summary.v2" web/src/types/household_summary.ts` is at least 1, `grep -c "export const registration" web/src/pages/home/HouseholdCard.tsx` prints 1 and `grep -c "schema: 'household_summary'" web/src/pages/home/HouseholdCard.tsx` prints 1; `git diff --name-only origin/main` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json`, `web/src/generated`, `web/fixtures/MANIFEST.synthetic-v1.json` or another card).
3. `wc -l web/src/pages/home/HouseholdCard.tsx` shows under 600 lines; `grep -c "fetch(" web/src/pages/home/HouseholdCard.tsx` prints 0; `git diff origin/main -- web/no-compute-allow.txt web/deps-allowlist.txt web/src/pages/home/StaleAlert.tsx web/src/pages/home/CardNotes.tsx web/src/pages/home/DataTimesFold.tsx` is empty (a needed change there is reported in the pull request description, not made).
4. `web/test/pages/home/household-v2/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else (`ok.ts`, `gap.ts`, `stale.ts`, `bad-version.json`); `grep -c "@" web/test/pages/home/household-v2/fixtures/*` prints 0 for each file; `grep -c '"schema_version": [0-9]*' web/test/pages/home/household-v2/fixtures/bad-version.json` prints 1 and `grep -c '"schema_version": 2,' web/test/pages/home/household-v2/fixtures/bad-version.json` prints 0.
5. `web/test/pages/home/HouseholdCard.test.tsx` imports `ok`, `gap` and `stale` from `./household-v2/fixtures/`, reads `bad-version.json` with `readFileSync`, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it opens no port and sleeps never; a clock it needs is set with `vi.useFakeTimers` and `vi.setSystemTime`.
6. Each field named under What of the card is asserted by name: for each of `title` `subtitle` `stale` `tiles` `notes` `gaps` `data_times_text`, `grep -c -E "(ok|gap|stale)\.card\.<field>\b" web/test/pages/home/HouseholdCard.test.tsx` prints at least 1, and `pnpm --dir web run test -- web/test/pages/home/HouseholdCard` passes together with `pnpm --dir web run test -- web/test/pages/household`.
7. The result matches its part of `docs/design/web-views-wave-2.md` (this entry, Decisions 1, 2 and 4), judged from the code and the design file.

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

### held-companies-summary-v2

- Module: web. Size: S. Depends on: `pages-types-wave-2` only.
- Design: Decisions 1, 2 and 4.
- Files: web/src/pages/home/HeldCompaniesCard.tsx, web/src/types/held_companies_summary.ts, web/test/pages/home/HeldCompaniesCard.test.tsx, web/test/pages/home/held-v2/**
- What: the 持仓公司 card moves from `held_companies_summary` v1 to v2: `web/src/types/held_companies_summary.ts` takes `Payload` from `HeldCompaniesSummary` of `@/generated/held_companies_summary.v2` with `SUPPORTED_VERSIONS = [2]`, and `HeldCompaniesCard.tsx` reads the v2 shape (`Gap` and the ids from `common.v1.json`, `stale_after` at the top level). The card keeps showing the fields of the payload and computes nothing; `value` stays the key of the user-started sort only. Four fixtures are written from the schema alone in `web/test/pages/home/held-v2/fixtures/`, in one mechanism: `ok`, `gap` and `stale` are TypeScript modules `ok.ts`, `gap.ts`, `stale.ts`, each `export const <name> = { ... } satisfies HeldCompaniesSummary` (a literal, so `tsc` checks its shape; `schema` is `'held_companies_summary'` and `schema_version` is 2); `bad-version.json` is plain JSON with `schema_version` set to another number, is not typed and is read with `readFileSync` and `JSON.parse`. `HeldCompaniesCard.test.tsx` reads these four and no longer reads `web/fixtures/held-companies-summary/`; the v1 fixtures stay for the Go tests.
- 按 frontend-stack：用 @tanstack/react-table, through `components/data-table` 做 the `rows` under the `columns`。
- 按 frontend-stack：用 shadcn card, badge and alert 做 the card, the earnings and volatility cells' badges, the stale label and the gap lines。

Acceptance:

1. `grep -c "satisfies HeldCompaniesSummary" web/test/pages/home/held-v2/fixtures/ok.ts web/test/pages/home/held-v2/fixtures/gap.ts web/test/pages/home/held-v2/fixtures/stale.ts` prints 1 for each, `test -f web/test/pages/home/held-v2/fixtures/bad-version.json` holds, and `grep -c "SUPPORTED_VERSIONS = \[2\]" web/src/types/held_companies_summary.ts` prints 1.
2. `grep -c "held_companies_summary.v2" web/src/types/held_companies_summary.ts` is at least 1, `grep -c "export const registration" web/src/pages/home/HeldCompaniesCard.tsx` prints 1 and `grep -c "schema: 'held_companies_summary'" web/src/pages/home/HeldCompaniesCard.tsx` prints 1; `git diff --name-only origin/main` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json`, `web/src/generated`, `web/fixtures/MANIFEST.synthetic-v1.json` or another card).
3. `wc -l web/src/pages/home/HeldCompaniesCard.tsx` shows under 600 lines; `grep -c "fetch(" web/src/pages/home/HeldCompaniesCard.tsx` prints 0; `git diff origin/main -- web/no-compute-allow.txt web/deps-allowlist.txt web/src/pages/home/StaleAlert.tsx web/src/pages/home/CardNotes.tsx web/src/pages/home/DataTimesFold.tsx` is empty (a needed change there is reported in the pull request description, not made).
4. `web/test/pages/home/held-v2/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else (`ok.ts`, `gap.ts`, `stale.ts`, `bad-version.json`); `grep -c "@" web/test/pages/home/held-v2/fixtures/*` prints 0 for each file; `grep -c '"schema_version": [0-9]*' web/test/pages/home/held-v2/fixtures/bad-version.json` prints 1 and `grep -c '"schema_version": 2,' web/test/pages/home/held-v2/fixtures/bad-version.json` prints 0.
5. `web/test/pages/home/HeldCompaniesCard.test.tsx` imports `ok`, `gap` and `stale` from `./held-v2/fixtures/`, reads `bad-version.json` with `readFileSync`, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it opens no port and sleeps never; a clock it needs is set with `vi.useFakeTimers` and `vi.setSystemTime`.
6. Each field named under What of the card is asserted by name: for each of `title` `say` `subtitle` `stale` `columns` `rows` `notes` `gaps` `data_times_text`, `grep -c -E "(ok|gap|stale)\.card\.<field>\b" web/test/pages/home/HeldCompaniesCard.test.tsx` prints at least 1, and `pnpm --dir web run test -- web/test/pages/home/HeldCompaniesCard` passes.
7. The result matches its part of `docs/design/web-views-wave-2.md` (this entry, Decisions 1, 2 and 4), judged from the code and the design file.

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

### macro-summary-v2

- Module: web. Size: S. Depends on: `pages-types-wave-2` only.
- Design: Decisions 1, 2 and 4.
- Files: web/src/pages/home/MacroCard.tsx, web/src/types/macro_summary.ts, web/test/pages/home/MacroCard.test.tsx, web/test/pages/home/macro-v2/**
- What: the 宏观 card moves from `macro_summary` v1 to v2: `web/src/types/macro_summary.ts` takes `Payload` from `MacroSummary` of `@/generated/macro_summary.v2` with `SUPPORTED_VERSIONS = [2]`, and `MacroCard.tsx` reads the v2 shape (`Gap` and the ids from `common.v1.json`, `stale_after` at the top level; `MacroCard.tsx` keeps exporting `validateMacro` and `MacroPayload`). The card keeps showing the fields of the payload and computes nothing. Four fixtures are written from the schema alone in `web/test/pages/home/macro-v2/fixtures/`, in one mechanism: `ok`, `gap` and `stale` are TypeScript modules `ok.ts`, `gap.ts`, `stale.ts`, each `export const <name> = { ... } satisfies MacroSummary` (a literal, so `tsc` checks its shape; `schema` is `'macro_summary'` and `schema_version` is 2); `bad-version.json` is plain JSON with `schema_version` set to another number, is not typed and is read with `readFileSync` and `JSON.parse`. `MacroCard.test.tsx` reads these four and no longer reads `web/fixtures/macro-summary/`; the v1 fixtures stay for the Go tests.
- 按 frontend-stack：用 shadcn card, progress and separator 做 the card, the `ease` marker and the divider above the events。
- 按 frontend-stack：用 @tanstack/react-table, through `components/data-table` 做 the `events` table。

Acceptance:

1. `grep -c "satisfies MacroSummary" web/test/pages/home/macro-v2/fixtures/ok.ts web/test/pages/home/macro-v2/fixtures/gap.ts web/test/pages/home/macro-v2/fixtures/stale.ts` prints 1 for each, `test -f web/test/pages/home/macro-v2/fixtures/bad-version.json` holds, and `grep -c "SUPPORTED_VERSIONS = \[2\]" web/src/types/macro_summary.ts` prints 1.
2. `grep -c "macro_summary.v2" web/src/types/macro_summary.ts` is at least 1, `grep -c "export const registration" web/src/pages/home/MacroCard.tsx` prints 1 and `grep -c "schema: 'macro_summary'" web/src/pages/home/MacroCard.tsx` prints 1; `git diff --name-only origin/main` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json`, `web/src/generated`, `web/fixtures/MANIFEST.synthetic-v1.json` or another card).
3. `wc -l web/src/pages/home/MacroCard.tsx` shows under 600 lines; `grep -c "fetch(" web/src/pages/home/MacroCard.tsx` prints 0; `git diff origin/main -- web/no-compute-allow.txt web/deps-allowlist.txt web/src/pages/home/StaleAlert.tsx web/src/pages/home/CardNotes.tsx web/src/pages/home/DataTimesFold.tsx` is empty (a needed change there is reported in the pull request description, not made).
4. `web/test/pages/home/macro-v2/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else (`ok.ts`, `gap.ts`, `stale.ts`, `bad-version.json`); `grep -c "@" web/test/pages/home/macro-v2/fixtures/*` prints 0 for each file; `grep -c '"schema_version": [0-9]*' web/test/pages/home/macro-v2/fixtures/bad-version.json` prints 1 and `grep -c '"schema_version": 2,' web/test/pages/home/macro-v2/fixtures/bad-version.json` prints 0.
5. `web/test/pages/home/MacroCard.test.tsx` imports `ok`, `gap` and `stale` from `./macro-v2/fixtures/`, reads `bad-version.json` with `readFileSync`, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it opens no port and sleeps never; a clock it needs is set with `vi.useFakeTimers` and `vi.setSystemTime`.
6. Each field named under What of the card is asserted by name: for each of `title` `say` `stale` `ease` `events` `notes` `gaps` `data_times_text`, `grep -c -E "(ok|gap|stale)\.card\.<field>\b" web/test/pages/home/MacroCard.test.tsx` prints at least 1, and `pnpm --dir web run test -- web/test/pages/home/MacroCard` passes.
7. The result matches its part of `docs/design/web-views-wave-2.md` (this entry, Decisions 1, 2 and 4), judged from the code and the design file.

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

## Left out

- `macro_rates`: its payload has no `schema` field (the schema has `schema_version` as the string "1" and no `schema`), and the shell dispatches on `payload.schema`; a view cannot be reached until the
  builder writes the field, which is a contracts and Go change. `option-page-serve.md` also keeps it out of the site and says its web view is its own design.
- `settings_page`, `report_page`, `home_page`, `market_weather` as a page: left out by `web-views-wave.md`, for the reasons it gives; not reopened.
- `market_summary` v1 and v2, `options_summary` v1 and v2, `structure_detail` v1 and v2, `household_summary` v1, `held_companies_summary` v1, `macro_summary` v1: older versions that a
  current view replaces or that an entry here replaces; a second view of one schema is refused by the registry (What main holds today).
- `positions_page` v3: `web/src/types/positions_page.ts` already accepts versions 1, 2 and 3.
- The schemas with a view today (`advice_page`, `companies_page`, `full_scan`, `household_page`, `macro_page`, `option_page`, `option_data_page`, `positions_page`, `structures_list`, `tracking`) and the six of wave 1:
  no view is missing.
- Removing the hand-written `web/src/types/*_summary.v1.types.ts` files and the v1 fixtures of `web/fixtures/`: the Go tests read the fixtures and the v1 types become unused only after the three entries land; a follow-up.
- A Go test that validates `web/test/pages/*/fixtures` against the schemas (Adversarial 2 of `web-views-wave.md`): it is a contracts change, not web-only.
- Not chosen: padding the wave to six with entries that split a card into parts. Each card is one file of under 150 lines; a split would make entries share it.
- Not chosen: hand-written v2 types in `web/src/types/` without generation. They would repeat the placeholder pattern that `generated-summary-types` retired, and drift from the schema unchecked.

## Adversarial

1. Gap against the goal: three entries and one interface entry, not six. Adopted as a limit set by the schemas, not by the cut (Decision 1, `## Left out`). Another wave needs new schemas (a builder that writes `macro_rates` with a `schema` field, a write path for `settings_page`) that are not web-only.
2. Claim most likely false: "the three moves touch only their own paths". Two things can break it: v2 changes a shape that `StaleAlert`, `CardNotes` or `DataTimesFold` read (they are shared and not edited; the entry stops and reports), and a generated v2 type that does not compile under `tsc --strict` (the interface entry runs first and stops on it).
3. Likeliest cause of a slip: `householdPage.test.tsx` renders `HouseholdCard` with a v1 fixture, which the v2 card may not render; it is in the Files of `household-summary-v2` and acceptance 6 runs both test directories.
4. Lock-in: three more `fixtures` directories under `web/test/pages/home/` as wave 1 has under `web/test/pages/`; adopted knowingly, each is one directory.
5. Risk not removed: the typed fixtures satisfy `tsc` but not `maxLength`, `pattern` or `x-rounding` constraints; no schema validator is whitelisted (as wave 1, Adversarial 2).
