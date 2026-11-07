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
- The wire shape of v1 and v2 is the same for the three: the `Gap` and `StaleLabel` defs become refs to `common.v1.json`, a date pattern is written with `[0-9]`, and an optional top-level `stale_after` is new (`card.stale` already carries its own `stale_after`, which `StaleAlert` reads). So each card diff is a few lines: the type file, the literal schema name where needed, and the fixtures and tests. A type error from a generated type (json2ts writes arrays with a `maxItems` as tuple unions) is reported in the pull request, not patched in `web/src/generated`.
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
   `.ts` literals (`satisfies` the generated type, imported by the relative path `../../../../../src/generated/<name>.v2`, not by `@/`, so that the item `grep -c "@"` is 0; wave 1 does the same) with one untyped `bad-version.json`, as wave 1 Decision 4 and its `## Left out` explain. The v1 fixtures of `web/fixtures/` are not touched.
5. The shared helpers of the home cards (`StaleAlert.tsx`, `CardNotes.tsx`, `DataTimesFold.tsx`) are read by all three entries and edited by none; a needed change is reported, not made.
6. Each card entry owns its own test file in `web/test/pages/home/`. `HeldCompaniesCard.test.tsx` also scans every file of `src/pages/home` for formatting calls, so each of the three runs the whole `web/test/pages/home` directory (acceptance 6). One other test file reads a card: `web/test/pages/household/householdPage.test.tsx` renders
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

- Module: web and contracts (the judged two-module exception of `web-views-wave.md` Decision 3: the generator script is shared, so the entry cannot be split). Size: S. Depends on: none.
- Design: Decision 3.
- Files: web/package.json, web/src/generated/**, internal/contracts/generated_ts_test.go
- What: add three `json2ts` calls to `generate:types` (each with `--cwd ../contracts/schemas`, as the existing ones, so the `common.v1.json` references resolve); commit the three generated files and the `index.ts` lines;
  extend `TestGeneratedSummaryTypes` with the three rows. That test checks only the banner, `export interface <Name>` and the line of `index.ts` (properties, consts and enums are checked by `TestGeneratedV3Types` for the v3 pair only, and no `null` rule applies: the v2 schemas hold `null` types, so the generated files hold `| null`); the content of the three files is guarded by the CI drift diff. No new package: `json-schema-to-typescript` is whitelisted.
- 按 frontend-stack：用 json-schema-to-typescript 做 the three generated page types。

Acceptance:

1. `test -f` holds for `web/src/generated/household_summary.v2.ts`, `web/src/generated/held_companies_summary.v2.ts` and `web/src/generated/macro_summary.v2.ts`, and `grep -c "Code generated from contracts/, DO NOT EDIT" <file>` prints 1 for each.
2. `grep -c "export type" web/src/generated/index.ts` prints 12 (nine before, three after) and `grep -o "page\.[a-z_]*\.v[0-9]*\.json" web/package.json | sort -u | wc -l` prints 14 (eleven before, three after).
3. `grep -c "household_summary.v2.ts" internal/contracts/generated_ts_test.go` prints 1, and the same for `held_companies_summary.v2.ts` and `macro_summary.v2.ts`.
4. `git diff --name-only $(git merge-base HEAD origin/main) -- contracts/schemas contracts/proto contracts/datasets` prints nothing (the pull request says `Contracts: unchanged`) and `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
5. The result matches its part of `docs/design/web-views-wave-2.md` (Interface and Decision 3), judged from the code and the design file.

Checked by CI: `pnpm install --frozen-lockfile`, `pnpm --dir web run generate:types` with `git diff --exit-code web/src/generated` (ci-web drift check), tsc, eslint, vitest, the build; ci-go (`go test ./internal/contracts`); checks (the `Contracts:` line).

### household-summary-v2

- Module: web. Size: S. Depends on: `pages-types-wave-2` only.
- Design: Decisions 1, 2 and 4.
- Files: web/src/pages/home/HouseholdCard.tsx, web/src/types/household_summary.ts, web/test/pages/home/HouseholdCard.test.tsx, web/test/pages/household/householdPage.test.tsx, web/test/pages/home/household-v2/**
- What: the 家庭概览 card moves from `household_summary` v1 to v2: `web/src/types/household_summary.ts` takes `Payload` from `HouseholdSummary` of `@/generated/household_summary.v2` with `SUPPORTED_VERSIONS = [2]` (the pattern of `web/src/types/market_summary.ts`), and `HouseholdCard.tsx` reads the v2 shape (the wire shape is the same as v1, see What main holds today). `HouseholdCard.tsx` registers `schema: ['household', 'summary'].join('_')` today (the rule of issues 1132 and 1133 to spell the name once); the entry replaces it with the literal `'household_summary'` and the rule is retired, the registration test stays green. The card keeps showing the fields of the payload and computes nothing. `web/test/pages/household/householdPage.test.tsx` renders `HouseholdCard` once with a v1 fixture (line 215); that line reads the v2 `ok` fixture of this entry instead and the rest of that file is left as it is. Four fixtures are written from the schema alone in `web/test/pages/home/household-v2/fixtures/`, in one mechanism: `ok`, `gap` and `stale` are TypeScript modules `ok.ts`, `gap.ts`, `stale.ts`, each `export const <name> = { ... } satisfies HouseholdSummary` (the type is imported by the relative path `../../../../../src/generated/household_summary.v2`, never by `@/`) (a literal, so `tsc` checks its shape; `schema` is `'household_summary'` and `schema_version` is 2); `bad-version.json` is plain JSON with `schema_version` set to another number, is not typed and is read with `readFileSync` and `JSON.parse`. `HouseholdCard.test.tsx` reads these four and no longer reads `web/fixtures/household-summary/`; the v1 fixtures stay for the Go tests.
- 按 frontend-stack：用 shadcn card and badge 做 the card, the four tiles (`net_worth`, `day_change`, `month_to_date`, `cash`) and the gap badges。
- 按 frontend-stack：用 shadcn alert, through the shared `StaleAlert` 做 the stale label。

Acceptance:

1. `grep -c "satisfies HouseholdSummary" web/test/pages/home/household-v2/fixtures/ok.ts web/test/pages/home/household-v2/fixtures/gap.ts web/test/pages/home/household-v2/fixtures/stale.ts` prints 1 for each, `test -f web/test/pages/home/household-v2/fixtures/bad-version.json` holds, and `grep -c "SUPPORTED_VERSIONS = \[2\]" web/src/types/household_summary.ts` prints 1.
2. `grep -c "household_summary.v2" web/src/types/household_summary.ts` is at least 1, `grep -c "export const registration" web/src/pages/home/HouseholdCard.tsx` prints 1 and `grep -c "schema: 'household_summary'" web/src/pages/home/HouseholdCard.tsx` prints 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json`, `web/src/generated`, `web/fixtures/MANIFEST.synthetic-v1.json` or another card).
3. `wc -l web/src/pages/home/HouseholdCard.tsx` shows under 600 lines; `grep -c "fetch(" web/src/pages/home/HouseholdCard.tsx` prints 0 and `grep -c "household-summary" web/test/pages/household/householdPage.test.tsx` prints 0 (the v1 fixture is no longer read); `git diff $(git merge-base HEAD origin/main) -- web/no-compute-allow.txt web/deps-allowlist.txt web/src/pages/home/StaleAlert.tsx web/src/pages/home/CardNotes.tsx web/src/pages/home/DataTimesFold.tsx` is empty (a needed change there is reported in the pull request description, not made).
4. `web/test/pages/home/household-v2/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else (`ok.ts`, `gap.ts`, `stale.ts`, `bad-version.json`); `grep -c "@" web/test/pages/home/household-v2/fixtures/*` prints 0 for each file; `grep -c '"schema_version": [0-9]*' web/test/pages/home/household-v2/fixtures/bad-version.json` prints 1 and `grep -c '"schema_version": 2,' web/test/pages/home/household-v2/fixtures/bad-version.json` prints 0.
5. `web/test/pages/home/HouseholdCard.test.tsx` imports `ok`, `gap` and `stale` from `./household-v2/fixtures/`, reads `bad-version.json` with `readFileSync`, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it opens no port and sleeps never; a clock it needs is set with `vi.useFakeTimers` and `vi.setSystemTime`.
6. Each field named under What of the card is asserted by name: for each of `title` `subtitle` `stale` `tiles` `notes` `gaps` `data_times_text`, `grep -c -E "(ok|gap|stale)\.card\.<field>\b" web/test/pages/home/HouseholdCard.test.tsx` prints at least 1, and `pnpm --dir web run test -- web/test/pages/home` passes together with `pnpm --dir web run test -- web/test/pages/household`.
7. The result matches its part of `docs/design/web-views-wave-2.md` (this entry, Decisions 1, 2 and 4), judged from the code and the design file.

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

### held-companies-summary-v2

- Module: web. Size: S. Depends on: `pages-types-wave-2` only.
- Design: Decisions 1, 2 and 4.
- Files: web/src/pages/home/HeldCompaniesCard.tsx, web/src/types/held_companies_summary.ts, web/test/pages/home/HeldCompaniesCard.test.tsx, web/test/pages/home/held-v2/**
- What: the 持仓公司 card moves from `held_companies_summary` v1 to v2: `web/src/types/held_companies_summary.ts` takes `Payload` from `HeldCompaniesSummary` of `@/generated/held_companies_summary.v2` with `SUPPORTED_VERSIONS = [2]`, and `HeldCompaniesCard.tsx` reads the v2 shape (the wire shape is the same as v1, see What main holds today). The card keeps showing the fields of the payload and computes nothing; `value` stays the key of the user-started sort only. `HeldCompaniesCard.tsx` registers `['held', 'companies', 'summary'].join('_')` today (the rule of issues 1132 and 1133); the entry replaces it with the literal `'held_companies_summary'` and the rule is retired, the registration test stays green. Four fixtures are written from the schema alone in `web/test/pages/home/held-v2/fixtures/`, in one mechanism: `ok`, `gap` and `stale` are TypeScript modules `ok.ts`, `gap.ts`, `stale.ts`, each `export const <name> = { ... } satisfies HeldCompaniesSummary` (the type is imported by the relative path `../../../../../src/generated/held_companies_summary.v2`, never by `@/`) (a literal, so `tsc` checks its shape; `schema` is `'held_companies_summary'` and `schema_version` is 2); `bad-version.json` is plain JSON with `schema_version` set to another number, is not typed and is read with `readFileSync` and `JSON.parse`. `HeldCompaniesCard.test.tsx` reads these four and no longer reads `web/fixtures/held-companies-summary/`; the v1 fixtures stay for the Go tests.
- 按 frontend-stack：用 @tanstack/react-table, through `components/data-table` 做 the `rows` under the `columns`。
- 按 frontend-stack：用 shadcn card, badge and alert 做 the card, the earnings and volatility cells' badges, the stale label and the gap lines。

Acceptance:

1. `grep -c "satisfies HeldCompaniesSummary" web/test/pages/home/held-v2/fixtures/ok.ts web/test/pages/home/held-v2/fixtures/gap.ts web/test/pages/home/held-v2/fixtures/stale.ts` prints 1 for each, `test -f web/test/pages/home/held-v2/fixtures/bad-version.json` holds, and `grep -c "SUPPORTED_VERSIONS = \[2\]" web/src/types/held_companies_summary.ts` prints 1.
2. `grep -c "held_companies_summary.v2" web/src/types/held_companies_summary.ts` is at least 1, `grep -c "export const registration" web/src/pages/home/HeldCompaniesCard.tsx` prints 1 and `grep -c "schema: 'held_companies_summary'" web/src/pages/home/HeldCompaniesCard.tsx` prints 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json`, `web/src/generated`, `web/fixtures/MANIFEST.synthetic-v1.json` or another card).
3. `wc -l web/src/pages/home/HeldCompaniesCard.tsx` shows under 600 lines; `grep -c "fetch(" web/src/pages/home/HeldCompaniesCard.tsx` prints 0; `git diff $(git merge-base HEAD origin/main) -- web/no-compute-allow.txt web/deps-allowlist.txt web/src/pages/home/StaleAlert.tsx web/src/pages/home/CardNotes.tsx web/src/pages/home/DataTimesFold.tsx` is empty (a needed change there is reported in the pull request description, not made).
4. `web/test/pages/home/held-v2/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else (`ok.ts`, `gap.ts`, `stale.ts`, `bad-version.json`); `grep -c "@" web/test/pages/home/held-v2/fixtures/*` prints 0 for each file; `grep -c '"schema_version": [0-9]*' web/test/pages/home/held-v2/fixtures/bad-version.json` prints 1 and `grep -c '"schema_version": 2,' web/test/pages/home/held-v2/fixtures/bad-version.json` prints 0.
5. `web/test/pages/home/HeldCompaniesCard.test.tsx` imports `ok`, `gap` and `stale` from `./held-v2/fixtures/`, reads `bad-version.json` with `readFileSync`, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it opens no port and sleeps never; a clock it needs is set with `vi.useFakeTimers` and `vi.setSystemTime`.
6. Each field named under What of the card is asserted by name: for each of `title` `say` `subtitle` `stale` `columns` `rows` `notes` `gaps` `data_times_text`, `grep -c -E "(ok|gap|stale)\.card\.<field>\b" web/test/pages/home/HeldCompaniesCard.test.tsx` prints at least 1, and `pnpm --dir web run test -- web/test/pages/home` passes.
7. The result matches its part of `docs/design/web-views-wave-2.md` (this entry, Decisions 1, 2 and 4), judged from the code and the design file.

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

### macro-summary-v2

- Module: web. Size: S. Depends on: `pages-types-wave-2` only.
- Design: Decisions 1, 2 and 4.
- Files: web/src/pages/home/MacroCard.tsx, web/src/types/macro_summary.ts, web/test/pages/home/MacroCard.test.tsx, web/test/pages/home/macro-v2/**
- What: the 宏观 card moves from `macro_summary` v1 to v2: `web/src/types/macro_summary.ts` takes `Payload` from `MacroSummary` of `@/generated/macro_summary.v2` with `SUPPORTED_VERSIONS = [2]`, and `MacroCard.tsx` reads the v2 shape (the wire shape is the same as v1, see What main holds today; `MacroCard.tsx` keeps exporting `validateMacro` and `MacroPayload`). The card keeps showing the fields of the payload and computes nothing. Four fixtures are written from the schema alone in `web/test/pages/home/macro-v2/fixtures/`, in one mechanism: `ok`, `gap` and `stale` are TypeScript modules `ok.ts`, `gap.ts`, `stale.ts`, each `export const <name> = { ... } satisfies MacroSummary` (the type is imported by the relative path `../../../../../src/generated/macro_summary.v2`, never by `@/`) (a literal, so `tsc` checks its shape; `schema` is `'macro_summary'` and `schema_version` is 2); `bad-version.json` is plain JSON with `schema_version` set to another number, is not typed and is read with `readFileSync` and `JSON.parse`. `MacroCard.test.tsx` reads these four and no longer reads `web/fixtures/macro-summary/`; the v1 fixtures stay for the Go tests.
- 按 frontend-stack：用 shadcn card, progress and separator 做 the card, the `ease` marker and the divider above the events。
- 按 frontend-stack：用 @tanstack/react-table, through `components/data-table` 做 the `events` table。

Acceptance:

1. `grep -c "satisfies MacroSummary" web/test/pages/home/macro-v2/fixtures/ok.ts web/test/pages/home/macro-v2/fixtures/gap.ts web/test/pages/home/macro-v2/fixtures/stale.ts` prints 1 for each, `test -f web/test/pages/home/macro-v2/fixtures/bad-version.json` holds, and `grep -c "SUPPORTED_VERSIONS = \[2\]" web/src/types/macro_summary.ts` prints 1.
2. `grep -c "macro_summary.v2" web/src/types/macro_summary.ts` is at least 1, `grep -c "export const registration" web/src/pages/home/MacroCard.tsx` prints 1 and `grep -c "schema: 'macro_summary'" web/src/pages/home/MacroCard.tsx` prints 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json`, `web/src/generated`, `web/fixtures/MANIFEST.synthetic-v1.json` or another card).
3. `wc -l web/src/pages/home/MacroCard.tsx` shows under 600 lines; `grep -c "fetch(" web/src/pages/home/MacroCard.tsx` prints 0; `git diff $(git merge-base HEAD origin/main) -- web/no-compute-allow.txt web/deps-allowlist.txt web/src/pages/home/StaleAlert.tsx web/src/pages/home/CardNotes.tsx web/src/pages/home/DataTimesFold.tsx` is empty (a needed change there is reported in the pull request description, not made).
4. `web/test/pages/home/macro-v2/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else (`ok.ts`, `gap.ts`, `stale.ts`, `bad-version.json`); `grep -c "@" web/test/pages/home/macro-v2/fixtures/*` prints 0 for each file; `grep -c '"schema_version": [0-9]*' web/test/pages/home/macro-v2/fixtures/bad-version.json` prints 1 and `grep -c '"schema_version": 2,' web/test/pages/home/macro-v2/fixtures/bad-version.json` prints 0.
5. `web/test/pages/home/MacroCard.test.tsx` imports `ok`, `gap` and `stale` from `./macro-v2/fixtures/`, reads `bad-version.json` with `readFileSync`, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it opens no port and sleeps never; a clock it needs is set with `vi.useFakeTimers` and `vi.setSystemTime`.
6. Each field named under What of the card is asserted by name: for each of `title` `say` `stale` `ease` `events` `notes` `gaps` `data_times_text`, `grep -c -E "(ok|gap|stale)\.card\.<field>\b" web/test/pages/home/MacroCard.test.tsx` prints at least 1, and `pnpm --dir web run test -- web/test/pages/home` passes.
7. The result matches its part of `docs/design/web-views-wave-2.md` (this entry, Decisions 1, 2 and 4), judged from the code and the design file.

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

## Left out

- `macro_rates`: its payload has no `schema` field (the schema has `schema_version` as the string "1" and no `schema`), and the shell dispatches on `payload.schema`; a view cannot be reached until the
  builder writes the field, which is a contracts and Go change. `option-page-serve.md` also keeps it out of the site and says its web view is its own design.
- `settings_page`, `report_page`, `home_page`, `market_weather` as a page: left out by `web-views-wave.md`, for the reasons it gives; not reopened.
- `market_summary` v1 and v2, `options_summary` v1 and v2, `structure_detail` v1 and v2, `household_summary` v1, `held_companies_summary` v1, `macro_summary` v1: older versions that a
  current view replaces or that an entry here replaces; a second view of one schema is refused by the registry (What main holds today).
- `home_page` v1 embeds the v1 card defs of the three cards (`household_summary`, `held_companies_summary`, `macro_summary`); after the moves no view reads that embedded shape, and a v1 card payload is a visible gap. It stays stranded until the design of `home_page` (left out above).
- `positions_page` v3: `web/src/types/positions_page.ts` already accepts versions 1, 2 and 3.
- The schemas with a view today (`advice_page`, `companies_page`, `full_scan`, `household_page`, `macro_page`, `option_page`, `option_data_page`, `positions_page`, `structures_list`, `tracking`) and the six of wave 1:
  no view is missing.
- Removing the hand-written `web/src/types/*_summary.v1.types.ts` files and the v1 fixtures of `web/fixtures/`: the Go tests read the fixtures and the v1 types become unused only after the three entries land; a follow-up.
- A Go test that validates `web/test/pages/*/fixtures` against the schemas (Adversarial 2 of `web-views-wave.md`): it is a contracts change, not web-only.
- Not chosen: padding the wave to six with entries that split a card into parts. Each card is one file of under 150 lines; a split would make entries share it.
- Not chosen: hand-written v2 types in `web/src/types/` without generation. They would repeat the placeholder pattern that `generated-summary-types` retired, and drift from the schema unchecked.

## Adversarial

Reviewed by an independent adversarial pass; its full text is `adversarial-wave2.md`, kept in the scratchpad of the drafting session (the review of the first draft). The six points:

1. Gap against the goal. Adopted: the two-module interface entry is labelled as the judged exception of wave 1 Decision 3; the diff checks name `$(git merge-base HEAD origin/main)`; the pnpm items sit under Checked by CI and need an installed tree (`pnpm install --frozen-lockfile`). The count is three entries plus the interface because the schemas allow no more (Decision 1, `## Left out`); not adopted: padding.
2. Biggest unnamed risk. Adopted: the fixture import is relative, not `@/` (Decision 4 and each What), and a generated type that fails `tsc` in a card is reported, not edited. Not adopted: a schema validator for fixtures, which is a contracts change and no validator is whitelisted (wave 1 Adversarial 2).
3. A simpler way. Adopted in part: the wire shape of v1 and v2 is the same, so the Whats say the diff is a few lines. Not adopted: folding the three type-file swaps into the interface entry, which would break "interface first, cards independent" and the cards' own `tsc`; not adopted: pointing the tests at the v1 fixtures, which the type no longer matches.
4. Lock-in. Adopted: the three more `fixtures` directories stay named, and `home_page` v1 with its embedded v1 card defs is named in `## Left out` as stranded; the dead `*_summary.v1.types.ts` files are a named follow-up.
5. Claim most likely false. Adopted: the text on `TestGeneratedSummaryTypes` is corrected (banner, interface name and index line only; the v2 files hold `| null`). The drift diff of ci-web is the real guard of the content. Held: the counts 12 and 14 and the names of the three types.
6. Likeliest cause of a slip. Adopted: the registration of `HouseholdCard.tsx` and `HeldCompaniesCard.tsx` is a `join('_')` today, so each What says to replace it with the literal (acceptance 2), the household entry gains the grep that its test no longer reads the v1 fixture, and every entry runs the whole `web/test/pages/home` directory.
