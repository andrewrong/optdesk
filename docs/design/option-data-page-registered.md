# Web: the option data page registers itself

Author: designer of optdesk (first draft)
Read on main at df7deae: `web/src/shell/pages.tsx`, `web/src/pages/optiondata/OptionDataPage.tsx`,
`web/test/shell/pages.test.tsx`, `docs/design/web-views-wave.md` (Decision 2, `## Left out`) and
`docs/design/web-views-wave-2.md`.

## What main does today

`pages.tsx` builds the registry as `{ ...explicit, ...discoverCards(import.meta.glob('../pages/*/*.tsx', { eager: true }), explicit) }`.
Every other page file exports `registration = { schema, page: page(validate, View) }` and is found by the glob. `explicit` holds one line,
`option_data_page: page<OptionDataPagePayload>(validateOptionDataPage, OptionDataPage)`, with the import of three names from
`@/pages/optiondata/OptionDataPage`.

Why it was left out: `web-views-wave.md` (Decision 2 and its `## Left out`) kept `option_data_page` explicit because its page file belonged to the
design `option-page-serve`, then in flight, and named the move "a one-line follow-up after it lands". That design is on main and
`OptionDataPage.tsx` has no open design on it (last touched by the top-bar and underlying-links change), so the reason is gone. `web-views-wave-2.md`
repeats the state ("`explicit` with `option_data_page` only") and does not change it.

## Modules

none: every path below belongs to the row `web` (`web/**`).

## Interface

no interface change. The registration shape (`CardRegistration`, `discoverCards(modules, taken)`) is unchanged; the page file only starts to use it.
`web/src/generated/**` is not touched.

## Issues

Items are facts read from the diff, by a grep or a named test. CI-only checks are under `Checked by CI`.

### web-option-data-registration

The option data page file exports its own registration and `pages.tsx` loses the last explicit entry.
Goal: `option_data_page` is found by the glob like the other pages; `pages.tsx` holds no page name and no page import.
Files: web/src/pages/optiondata/OptionDataPage.tsx, web/src/shell/pages.tsx, web/test/shell/pages.test.tsx
Depends on: none (merged work only)
Design: docs/design/option-data-page-registered.md#what-the-entry-does

- 按 frontend-stack：用 Vite `import.meta.glob` (the existing glob of `pages.tsx`) 做 the discovery of the page, with no new glob and no new package.
- 按 frontend-stack：用 the shell `page()` helper (`web/src/shell/page.ts`) 做 the registration `{ schema: 'option_data_page', page: page<Payload>(validateOptionDataPage, OptionDataPage) }`, written as in `web/src/pages/macro/MacroPage.tsx`.

Acceptance:

1. `grep -c "export const registration" web/src/pages/optiondata/OptionDataPage.tsx` prints 1, and the block holds `schema: 'option_data_page'` and `page(validateOptionDataPage, OptionDataPage)` (or `page<Payload>(...)`); the file adds the import `page` from `@/shell/page` and no other import.
2. `grep -c "OptionDataPage\|explicit\|page<" web/src/shell/pages.tsx` prints 0, so `web/src/shell/pages.tsx` has no `explicit` object, no import from `@/pages/optiondata` and no `page<` call; the import from `@/shell/page` is `type Page` only (no unused `page`); `grep -c "import.meta.glob('../pages/\*/\*.tsx'" web/src/shell/pages.tsx` prints 1; `pages` is `discoverCards(import.meta.glob(...))` with the default for `taken`.
3. `discoverCards` and `CardRegistration` keep their names, signatures and the duplicate check in `web/src/shell/pages.tsx` (the `Duplicate page schema` string is still in the file once); only the comment above the registry is rewritten so that it no longer contains the word `explicit`.
4. `web/test/shell/pages.test.tsx` keeps the tests `registers option_data_page with the option data page component` (the component of `pages['option_data_page']` is `OptionDataPage`) and `registers every page file by its glob`; `grep -c -i "explicit" web/test/shell/pages.test.tsx` prints at least 1 and the title of the second test and the comment above it say that no page is explicit (the current title "the ten pages and the five cards" is stale: 13 schemas are listed, so it is rewritten); the test `throws on a duplicate schema, among the modules or against a taken one` is unchanged.
5. `git diff --name-only origin/main` lists exactly the three files of the Files line, and no file under `web/src/generated/`, `contracts/`, `web/package.json` or `pnpm-lock.yaml`.
6. The result matches the design part `What the entry does` of `docs/design/option-data-page-registered.md`.

Checked by CI: `tsc`, `eslint`, `vitest` and the build of the `web` workflow pass.

## What the entry does

It is the one-line follow-up that `web-views-wave.md` named. There is one entry because the registry interface does not change (the glob and
`discoverCards` already exist), the move touches one page file, the shell registry and its test, and all three are in `web`. Splitting the
registry edit from the page edit would leave a state where the schema is registered twice (a throw at load) or not at all, so the two edits are
one pull request. After it lands the glob is the only way a page enters the shell, and no later page issue edits `pages.tsx`.

## Left out

- Shared files with the designs drafted in parallel (`macro-rates-view` PR 285, `report-page` PR 286, the home page): none. This entry touches no generated file; those designs add a page file with its own `registration` and, by the glob, need no line in `pages.tsx`.
- Removing the `taken` parameter of `discoverCards`: kept, because the duplicate-against-taken test and the shape of the helper are used by the tests; it costs nothing.
- A lint or test that forbids an explicit page map: not added; with `explicit` gone, a new explicit entry would be visible in review.
- `docs/design/web-views-wave.md` and `web-views-wave-2.md` are accepted designs and are not edited; this design supersedes the lines that contradict it (superseded, left on main): `web-views-wave.md` l.17 (eleven pages by hand in `explicit`), l.35 (the object `explicit` in `pages.tsx`), l.49 (`option_data_page` stays explicit), l.95 and l.105 (acceptance `page<` prints 1, nothing added to `explicit`); `web-views-wave-2.md` l.15 (`explicit` with `option_data_page` only).
- Moving the page to a `view.tsx` name: the glob matches any `*.tsx` of the directory, so the file name stays.

## Adversarial

1. Gap against the goal: adopted. Acceptance 4 had no grep for the title and comment, and the `page` import would be left unused; acceptance 2 now says the import is `type Page` only and acceptance 4 has a grep.
2. Biggest unnamed risk: adopted. The react-refresh claim was false (no such rule in `web/eslint.config.js`); it is deleted so the worker does not hunt for it. No import cycle: `page.ts` has type-only dependencies.
3. Simpler way: adopted. Acceptance 5 duplicated the glob test and the existing registration test; it is dropped and the list is renumbered (6 items).
4. Lock-in: adopted. The contradicting lines of `web-views-wave.md` and `web-views-wave-2.md` are named in Left out as superseded.
5. Claim most likely false: adopted. The react-refresh parenthesis is deleted; the other claims (read at df7deae, no open design on the page file) were confirmed.
6. Likeliest slip: adopted. The `git diff` check names `origin/main` as its base; the unused `page` import is closed by acceptance 2.

Answer: `option-data-page-registered.adversarial.md` in the dispatcher's scratchpad.
