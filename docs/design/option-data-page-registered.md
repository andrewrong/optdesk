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
2. `grep -c "OptionDataPage\|explicit\|page<" web/src/shell/pages.tsx` prints 0, so `web/src/shell/pages.tsx` has no `explicit` object, no import from `@/pages/optiondata` and no `page<` call; `grep -c "import.meta.glob('../pages/\*/\*.tsx'" web/src/shell/pages.tsx` prints 1; `pages` is `discoverCards(import.meta.glob(...))` with the default for `taken`.
3. `discoverCards` and `CardRegistration` keep their names, signatures and the duplicate check in `web/src/shell/pages.tsx` (the `Duplicate page schema` string is still in the file once); only the comment above the registry is rewritten so that it no longer contains the word `explicit`.
4. `web/test/shell/pages.test.tsx` keeps the tests `registers option_data_page with the option data page component` (the component of `pages['option_data_page']` is `OptionDataPage`) and `registers every page file by its glob`; the test title of the second and its comment say that no page is explicit; the test `throws on a duplicate schema, among the modules or against a taken one` is unchanged.
5. `web/test/shell/pages.test.tsx` gains one test that `discoverCards({ file: moduleOfOptionDataPage })`, with the module imported from `@/pages/optiondata/OptionDataPage`, returns an object with the key `option_data_page` whose `component` is `OptionDataPage`.
6. `git diff --name-only` lists exactly the three files of the Files line, and no file under `web/src/generated/`, `contracts/`, `web/package.json` or `pnpm-lock.yaml`.
7. The result matches the design part `What the entry does` of `docs/design/option-data-page-registered.md`.

Checked by CI: `tsc`, `eslint`, `vitest` and the build of the `web` workflow pass (eslint includes the react-refresh rule: a page file that exports a component and a constant is already the pattern of `MacroPage.tsx`).

## What the entry does

It is the one-line follow-up that `web-views-wave.md` named. There is one entry because the registry interface does not change (the glob and
`discoverCards` already exist), the move touches one page file, the shell registry and its test, and all three are in `web`. Splitting the
registry edit from the page edit would leave a state where the schema is registered twice (a throw at load) or not at all, so the two edits are
one pull request. After it lands the glob is the only way a page enters the shell, and no later page issue edits `pages.tsx`.

## Left out

- Shared files with the designs drafted in parallel (`macro-rates-view` PR 285, `report-page` PR 286, the home page): this entry does not touch
  `web/src/generated/index.ts` or any generated file. It shares `web/src/shell/pages.tsx` only in principle: those designs add a page file with its own
  `registration` and, by the glob, need no line in `pages.tsx`; if one of them edits `web/test/shell/pages.test.tsx` (for example the list of schemas in
  `registers every page file by its glob`), the two entries share that file and a rebase is needed. No `Order:` line is written, because their entries are not cut yet; the dispatcher
  orders them when they are.
- Removing the `taken` parameter of `discoverCards`: kept, because the duplicate-against-taken test and the shape of the helper are used by the tests; it costs nothing.
- A lint or test that forbids an explicit page map: not added; with `explicit` gone, a new explicit entry would be visible in review.
- `docs/design/web-views-wave.md` and `web-views-wave-2.md` still say that `option_data_page` stays explicit: they are accepted designs and are not edited; this design supersedes that sentence.
- Moving the page to a `view.tsx` name: the glob matches any `*.tsx` of the directory, so the file name stays.

## Adversarial

pending
