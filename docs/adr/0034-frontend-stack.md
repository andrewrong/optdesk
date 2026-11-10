# ADR 0034: Frontend stack, one design language

Date: 2026-10-23
Status: Accepted (owner 2026-10-23, advisor window: direction approved, then "按照你的来吧" on the open items; start = now, step 1a)
Second opinion: three-round battle 2026-10-23, claude-opus-5-5 vs gpt-6.1-sol (ids from the battle script, not recorded by the run), digest battle-frontend/digest.md; advisor third opinion battle-frontend/advisor.md (AGREES WITH CHANGES)

## Context

**Today.** Every report page is a Python string of HTML built in its own module under `reports/` (`render.py`,
`home_page.py`, `household_page.py`, and so on, about 20 page kinds). The theme is an inline CSS string
(`THEME`, `BASE_STYLE` in `render.py`). Pages are static and script-free (ADR 0010) except for ECharts 5.6.0,
vendored, the only JavaScript. Pages are published as immutable files under `<root>/<KL date>/<run_id>.html`,
validated, then written atomically with mode 0600 (`reports/files.py`). Access is by the existing serve path and
the owner's phone.

**Why replace it.** Each page re-implements tables, tiles, folds, toggles, banners and icons by hand. Behaviour
(sorting, expand, focus, keyboard, dark mode) is built page by page and differs between pages. Interaction the
owner now wants (resizable panels, drag, animated state) cannot be written reliably as string HTML. The owner's
decision (2026-10-22/23): reuse mature libraries, keep component source under our control, one design language,
no duplicate dependencies.

**What stays.** Python computes every number and every sentence; the browser only displays. Content rules are
unchanged: advice is never a decision; a data gap is named once; option terms in trader English; JPY shown with
approximate USD. Publishing keeps the same report directory, atomic 0600 write, access control and no new
service or port; the file set changes (see Build and publish flow). Core modules stay free of I/O (AGENTS.md);
the view-model writer is an adapter.

## Decision

**Hard rule (owner, 2026-10-23).** 严禁在已有依赖可以正确表达的情况下自行设计实现组件和行为. Never design or implement a component or
behaviour by hand when an existing dependency can express it correctly. The text is verbatim and stays.
Definitions for review (submitted to the owner as a rule clarification, see "Decide later"): *designated* means
named in the tables below or in `web/dependency-allowlist.json`; *composition* (arranging designated components,
passing props, layout with Tailwind utilities) is not hand-building; *thin adapters* (Recharts chart wrapper and theme,
the view-model loader, the Ajv validator call) are allowed and stay under 100 lines each. The ban applies to our
code, not to the internals of designated libraries. Upstream shadcn animations and reviewed accessibility fixes
in copied source are allowed.

**One library per job.** A second library for a job already in the tables is forbidden, in `package.json`, in
the resolved graph through a new direct dependency, and in code. `web/dependency-allowlist.json` lists every
package CI accepts, with its job and a denylist per job (for example Chart.js, Victory, Redux, MobX,
react-table, date-fns/dayjs/moment together).

*Runtime* (ships in the bundle):

| Job | Library | Major line | Licence |
|---|---|---|---|
| UI components | `radix-ui` primitives (shadcn `--base radix`), components copied by shadcn style **Nova** | radix-ui 1.x | MIT |
| Component helpers | `class-variance-authority`, `clsx`, `tailwind-merge` | as scaffolded | Apache-2.0 / MIT |
| Icons | Lucide (`lucide-react`) | 1.x | ISC |
| Animation | Motion (`motion`) | 14.x | MIT |
| Tables | TanStack Table through shadcn Data Table | 8.x (capped, see policy) | MIT |
| Charts | Recharts through the shadcn `chart` component (owner 2026-11-03) | 3.x | MIT |
| UI runtime | `react`, `react-dom` | 19.x | MIT |
| Schema validation | Ajv, standalone code generated at build time | 8.x | MIT |
| State | Zustand: designated, not installed until shared state exists | 5.x | MIT |
| Layout (later) | `react-resizable-panels`, shadcn `resizable` | 4.x | MIT |
| Drag (later) | `@dnd-kit/core` | 6.x | MIT |

*Toolchain and support* (not in the bundle except where noted):

| Job | Library | Major line | Licence |
|---|---|---|---|
| Component CLI | `shadcn` | 4.x | MIT |
| Styling | Tailwind CSS, `@tailwindcss/vite`, `tw-animate-css` (bundled CSS) | Tailwind 4.x | MIT |
| Language | TypeScript `strict` | 5.x (capped) | Apache-2.0 |
| Build | Vite, `@vitejs/plugin-react` | 8.x | MIT |
| Unit tests | Vitest | peer range of Vite decides | MIT |
| Test DOM | jsdom (Vitest environment, tests only; the one DOM environment) | scaffold-chosen | MIT |
| Layout tests | Playwright, Chromium only | scaffold-chosen | Apache-2.0 |
| TS types from schema | `json-schema-to-typescript` | scaffold-chosen | MIT |
| Lint, format | ESLint flat config, `typescript-eslint`, Prettier | ESLint 10.x; Prettier 3.x | MIT |
| Package manager | pnpm (via `packageManager`; `onlyBuiltDependencies` for lifecycle scripts) | 10.x (12.x fails in the worker sandbox with a store-lock EPERM, measured 2026-10-23) | MIT |
| Runtime for build | Node 24 LTS, provided by mise on the worker and the owner's machine; CI downloads the checksum-pinned tarball from nodejs.org | exact, see policy | MIT |

shadcn `chart` is the chart component (owner 2026-11-03); it adds Recharts, which is the one chart library. Registry components that add packages
(calendar, sonner, drawer, command, carousel, form, input-otp) are owner-only: an issue may use them only after
the owner approves the package. Components that add no package are designated once in the copied source.

**Version policy.** This ADR fixes the library per job and its major line. Step 1a produces a candidate set from
`pnpm create vite` and `shadcn init`; the candidate becomes the chosen set only after frozen install, lint,
typecheck, tests and build all pass. Exact versions are then frozen: exact pins (no `^` or `~`) in
`package.json`, committed `pnpm-lock.yaml`, `packageManager` field set to the exact pnpm, and
`web/dependency-allowlist.json` matching Appendix A. The chosen majors are capped at those the shadcn recipes
target that day: TypeScript 5.x and TanStack Table 8.x. A newer major (TypeScript 7, Table 9) needs an upgrade
issue and, for a change of a major line in the tables, an amendment to this ADR. Compatibility is judged by the
packages' declared peer ranges (for example Vitest against Vite), not by matching major numbers. Other rules:
- `minimumReleaseAge: 10080` (minutes, seven days) in the pnpm settings, so no package under a
week old installs.
- Lifecycle scripts blocked except a reviewed list (setting `onlyBuiltDependencies` in pnpm 10).
- Node: exact version pinned in `.nvmrc` and in `engines` (not `24` alone). Node 24 is the current LTS line
  ("Krypton"); Node 26 is a Current line without LTS status, and a build tool for a trading-adjacent repo should
  not run on a line that has no long-term fixes. The upgrade to the next LTS is an issue.
- The step 1a worker records chosen versions, commands and the date in Appendix A. Appendix B is the registry
  state on 2026-10-23 for reference only and pins nothing.
- Outbound destinations are the npm registry and the Node download, at install time only; record them where the
  repo lists approved destinations.

**Recharts 3.x (owner 2026-11-03).** Charts are Recharts through the shadcn `chart` component (3.10.1 in
`frontend-whitelist.json`); it is the one chart library, and no other is added. Imports come through the shadcn
`chart` source in `src/components/ui/`, with the Recharts parts used and no others. Colours come from the
design tokens through the chart config. Tooltip text is escaped and covered by a hostile-string test; ARIA text
comes from the view model, not auto-generated; no chart-side computation. Charts are built in TypeScript from
view-model data; Python no longer emits chart option objects. Old Python-rendered pages keep their vendored
chart script until each is retired; that is the only allowed duplicate, and it ends at step 6.

**Toolchain.**
- pnpm with `--frozen-lockfile` installs; Vite builds a static bundle; TypeScript `strict: true`,
  `noUncheckedIndexedAccess: true`.
- Tailwind 4 with CSS-first config, as shadcn's CLI requires; no second CSS framework, no CSS-in-JS.
- ESLint + Prettier; `pnpm lint`, `pnpm format:check`, `pnpm typecheck`, `pnpm test`, `pnpm build`, `pnpm test:e2e`
  (Playwright) are the six scripts CI calls. TypeScript types are generated from the JSON Schema by
  `json-schema-to-typescript` (the one designated generator).
- No SSR, no Next.js, no Storybook; a component sheet page built from fixtures replaces it.
- CI runs with no credentials and no secrets; the build produces files only. Production hosts have no Node: they
  receive only the built output, and nothing at runtime needs Node.

**Repository layout.**
- `web/` at the repo root: `package.json`, lockfile, `.nvmrc`, `dependency-allowlist.json`,
`src/`, `src/components/ui/` (shadcn source, the only place base components live), `src/pages/<page>/`,
`src/generated/` (types and Ajv code, checked in), `fixtures/`.
- `src/option_trader/viewmodels/`: one module per page with frozen dataclasses (core, no I/O),
plus a writer adapter that serialises and publishes.
- `docs/schemas/`: generated JSON Schema, one file per page and version, checked in.

**View-model contract.**
- Every payload has an envelope: `schema` (page id), `schema_version` (integer), `run_id`, `build_id`, scope and
  currency, `generated_at` and source data times (KL time, ISO), and run status. One typed payload per page per
  publish.
- Chain: Python dataclass, then JSON Schema, then generated TS types and Ajv standalone validator, by one named
  deterministic generator script in `scripts/`. CI regenerates and fails on any diff (the schema regeneration
  check). Fixtures are invented and validated against the schema in both languages.
- The browser validates the whole payload with Ajv before render; failure shows a named gap and "do not act",
  never a half page. Python writes with `allow_nan=False`; NaN and Infinity are rejected.
- Every displayed number is `{value, text, unit}`: Python formats `text`. The browser may sort or filter by
  `value` for presentation, started by the user, and never derives a number, sum, percentile, ranking or
  verdict. Precision, rounding mode and integer bounds are set in Python and written in the schema (for example
  a minimum, maximum and decimal places per field); the browser does not round.
- Missing is never zero: an absent value is a named gap with its reason. Field names use the GLOSSARY.md unit
  suffixes; conversion from broker units happens in Python at the boundary.
- A breaking change (field removed or renamed, new required field, type, unit or sign changed) is a new
  `schema_version`; the page accepts exactly the versions it names and shows a named gap otherwise. Additive
  optional fields do not bump it.
- Staleness (presentation only, no owner approval needed): Python sends `data_time`, `stale_after` and the label
  text on every block. The browser compares the clock with `stale_after` on first display, on a timer and on
  `visibilitychange`. If the device clock is behind `generated_at` by more than 5 minutes it shows a
  clock-skew message; a clock ahead of `generated_at` cannot be told from a page opened later, so it is not
  flagged. All freshness rules stay in Python.
- Names never contain account ids; account labels are the existing neutral labels.

**Build and publish flow.**
- The payload is embedded in the page as `<script type="application/json" id="vm">` with `<` escaped as `<`, so
  there is no fetch; a test uses a `</script>` string. The page is served over HTTP only (`file://` is
  unsupported).
- `pnpm build` produces `web/dist/`. Vite `base` is absolute (never `../`); an asset-resolution
test opens a built page and resolves every asset.
- Assets are content-hashed, shared by all pages, kept in `<root>/assets/`, append-only and retained while any
  report references them; mode 0600 as today.
- Order: assets first, then the HTML, each atomic and validated. The HTML name is unique per run, `<root>/<KL
  date>/<run_id>.html`; the latest/index pointer is advanced last; a committed run is never overwritten. A
  failed run leaves the previous run and pointer in place.
- Access control applies to every route (page, assets, index) exactly as today's pages; no new route is public.
- A static fail-closed notice sits outside the React root ("page did not load, do not act"), is hidden only
  after validation and first commit, and is restored by the error boundary; `<noscript>` says the same. A CSP
  with `default-src 'self'` and `connect-src 'none'` is set where the serve path allows it.
- No CDN, no web font download, no analytics. The hello page of step 1b lives under `_dev/`.
- The "STALE, do not act" banner, KL as-of times and run status of ADR 0010 are rendered from view-model fields
  by one shared component, so no page can omit them.

**Theming.** shadcn CSS variables carry the v2 design system colours for light and dark (mapping table made in
step 2b; the current `THEME` values are the starting reference). Nova gives the compact component style but its
default sizes conflict with the 16 px and 44 px rules, so `@theme` overrides set 44 px targets and a 16 px body.
Dark mode is CSS only: `@custom-variant dark` following `prefers-color-scheme`; no toggle script. Beyond
neutral, only the signal tokens the system has: ok, hold, unknown, banner states. Success and warning tokens
wait for the owner.

**Accessibility and phone rules.** Design width is 390 px with a floor of 360 px: one column, no horizontal page
scroll, wide tables scroll inside their container; on desktop the content is full width, no max-width column (owner 2026-11-10;
docs/design/web-shell-layout.md Decisions); the 390 px phone rules stay. Text at least 16 px, touch
targets at least 44 px, visible focus, colour never the only signal, contrast checked in both themes. Motion
respects reduced motion: `<MotionConfig reducedMotion="user">` (Motion defaults to "never"), chart animation
off, and `tw-animate-css` guarded by `motion-reduce`; no motion is needed to read a page.

**Not adopted now.** `react-resizable-panels` and dnd-kit enter `package.json` only in the issue of the first
desktop tool page that needs them (listed in the allowlist as "designated, not yet installed", as is Zustand).
No Storybook, SSR, Next.js, router library (one page per HTML file), form library, or data-fetching library.

## Enforcement

**(a) This ADR and AGENTS.md.** Add under "Rules for agents":
- `Frontend: use only the libraries and components designated in docs/adr/0034-frontend-stack.md. 严禁在已有依赖可以正确表达的情况下自行设计实现组件和行为. Do not add a frontend dependency the issue does not name.`
- `Frontend pages read one typed view-model made by Python; the browser computes no figure, rule or advice sentence.`

The definitions of "designated", "composition" and thin adapters, and the dispatcher change in (c), are a rule
change and go to the owner for approval before they are written into AGENTS.md.

**(b) CLAUDE.md review checklist.** Add item 8 and add it to the request-changes list:
- `8. Frontend: every component, icon, animation, table, chart and state use comes from the designated library of ADR 0034 (shadcn source in web/src/components/ui, Lucide, Motion, TanStack Table, Recharts through shadcn `chart`, Zustand). Name any hand-built component, behaviour or second library for a job already covered, and any figure or rule computed in the browser. Financial semantics are reviewed by a person; lint cannot prove them.`
- Change the last line to: `Request changes for any failure of items 2, 3, 4, 7 or 8.`

**(c) Dispatcher rule for issue drafting.** Every frontend issue names the designated components it uses (for
example "Card, Table, Badge from shadcn; Lucide `TriangleAlert`; Recharts line"), the view-model schema and
version, and the fixture. An issue without that list is not filed. A needed component that adds no package is
designated in the copied source; one that adds a package goes to the owner first. The worker does not choose.

**(d) CI (job `quick`, plus tests).** CI has no credentials.
- Allowlist test over the resolved dependency graph (not only `web/package.json`), reading
  `web/dependency-allowlist.json`: every package is allowed, versions exact and matching Appendix A, no per-job
  denylist hit, licences on an approved list, lockfile consistent with `--frozen-lockfile`.
- ESLint: no inline `<svg>` outside Lucide; no `@keyframes` or hand-written `transition`/`animation` where
  Motion applies; base components only from `src/components/ui`; `no-restricted-imports` and restricted syntax
  for any second state, table, icon, chart, animation or date library; no `fetch` or other network use; no
  `dangerouslySetInnerHTML`; no TanStack grouping, aggregation or footer sums and no `reduce` sums over
  displayed values.
- All six scripts (`lint`, `format:check`, `typecheck`, `test`, `build`, `test:e2e`) run in `quick`, plus the pytest schema
  regeneration check and a bundle size budget set at step 2b.
- Vitest: view-model mapping, keyboard use, both themes, unsupported `schema_version`, identity mismatch
  (`run_id` or scope), the hostile-string tooltip, the `</script>` embed. Expected values come from the
  Python-side fixture or the issue's reference values.
- Layout tests use Playwright (owner approved 2026-10-23), Chromium only, run in CI on the self-hosted runner
  only: Chromium cannot launch inside the worker sandbox on macOS (mach_port_rendezvous bootstrap_check_in
  denied, measured 2026-10-23), so workers write the tests and never run them, verify sessions do not run them,
  Chromium is in the owner's runner image of option-trader since 2026-10-24 (Playwright version pinned in the lockfile), and missing
  baselines are produced by CI with --update-snapshots, uploaded as an artifact and committed by the worker; `toHaveScreenshot` against committed baselines rendered from invented
  fixtures only, at 390 px and a desktop width in both themes, in the CI `quick` job; alternatives were weighed and
  rejected (Cypress: heavier, visual diff needs a paid cloud; Puppeteer: Chrome only, no assertions; Percy,
  Chromatic, Applitools: screenshots leave the machine; Vitest browser mode: Playwright underneath).
- Tests never touch the network; the build reaches the npm registry only at install time.

## Migration order

ADR 0034 step 4 depends on ADR 0033 change 9 (the facades, rows 9 to 11). Steps 1 to 3 do not touch the data
layer.

| # | Step | Worker |
|---|---|---|
| 1a | Toolchain, `web/`, allowlist, CI gates, AGENTS.md and CLAUDE.md lines (after owner approval of the rule wording) | Sonnet |
| 1b | Publisher: assets-first publish, unique name, latest pointer last, fail-closed shell, `_dev/` hello page | Sonnet |
| 2a | Schema, invented fixture and Python generator stub for the home 期权摘要 card view model (options_summary.v1: four tiles + six-month premium series), versioned; re-pointed 2026-10-23 after the owner's v2.2 feedback put the home page first | Sonnet |
| 2b | Theme mapping from v2 to shadcn variables; component sheet from fixtures (both themes, 360/390 px, chart-type check with Recharts) | Sonnet |
| 3 | First real page: the home page assembled block by block beside the old page; 3a 期权摘要 card from the 2a fixture, 3b 持仓提醒 payload for the open report, 3c.. 宏观/市场/家庭概览/持仓公司 in the owner's order; needs 2a and 2b | Sonnet |
| 4 | Python view-model generator fed by the change-9 facade | Opus (money figures, schema) |
| 5 | Remaining pages by the full inventory (including 持仓页 and 复盘页) in the top-down order (宏观, 市场, 家庭, 公司, 期权) | Sonnet; Opus for money or P&L tables |
| 6 | Remove Python HTML rendering, vendored ECharts 5.6.0, `THEME` and `BASE_STYLE`, and ADR 0010's script-free rule | Sonnet, owner-visible list first |

Retirement of an old page needs: stale, gap and error fixtures; independent expected values; parity with the old
page as regression evidence only; a tested rollback; and at least 5 sessions side by side, which is a minimum
and not proof. Step 6 keeps ADR 0010's access, privacy and STALE requirements and resolves its no-JavaScript
claim against the chart exception (the old pages' chart script is the only script before this ADR; React plus Recharts after).

Parallel: 1a first, then 1b; 2a and 2b run in parallel after 1a; step 3 needs both; step 4 runs in parallel with
2 and 3 once the facade exists. Inside step 5, pages run in parallel when they share no component not yet in the
sheet; the shared banner and table components land first. Step 6 only after every page is retired.

## Consequences

- One tested component set and one behaviour per widget; dark mode, focus and keyboard come from the libraries.
- Python and TypeScript both live in the repo; a schema change touches both, guarded by the generator check.
- Old and new pages co-exist during migration; the only allowed duplicates are the old Python renderer and
  vendored ECharts 5.6.0.
- Publishing changes: the JSON is inline, assets are shared and append-only (disk grows until a retention
  issue), and the file set is not the same as ADR 0010's single HTML.

**Risks.**
- Node toolchain on the Mac mini and in CI is new: exact Node, cached pnpm store, fail early.
- shadcn components are copied source and do not update; an upgrade is a deliberate issue that diffs the copied
  files, and the revision and licence of each copied component are recorded. Local edits to `src/components/ui`
  are limited to tokens and reviewed accessibility fixes.
- Tailwind 4 syntax differs from most 3.x examples; workers follow the shadcn docs for the pinned versions only.
- Recharts bundle size: import only what is used and keep to the budget.
- Supply chain: a large transitive tree; lockfile, frozen installs, exact pins, release age and blocked
  lifecycle scripts are the control; `pnpm audit` is advisory.
- Two languages raise the review load; checklist item 8 and generated types limit drift.

**Decided 2026-10-23 (owner: follow the advisor's recommendation).** Step 1a starts now, beside the running
data-layer work; v2 boards are the baseline for step 2b and the owner's detail changes are applied there as they
come; success and warning signal tokens are kept; the hard-rule definitions above stand as the AGENTS.md wording.
**Decided 2026-10-28 (issue #1185, step 5).** `/` is the composed React home (the run named by `_dev/home-page/latest`); the old Python home stays at `/legacy/` and answers `/` when that pointer or file is unreadable (the tested rollback). Retirement of the old home waits for the 5 side-by-side sessions.
**Decide later (owner).** Nothing pending. (Earlier text: the final wording of the hard rule and dispatcher rule in AGENTS.md (the definitions above are a
proposal).

## Amends

- ADR 0010 "Reports, phone access and Telegram": its rule of static script-free pages with inline HTML is
  superseded in steps; per page when retired in step 5, fully at step 6. Its file naming, 0600 atomic publish,
  validation, STALE banner and phone access remain.
- Any ADR about page publishing or page layout (for example 0028 page-build hold, 0026 market weather pages):
  the content rules stay; only the rendering moves.
- ADR 0033 gains a dependency on its change 9 facades for step 4 of this ADR; no change to its text.

- The CI runner is self-hosted and its image has no Node: CI downloads the Node tarball pinned in `web/.nvmrc` from
  nodejs.org and checks it against `web/node-sha256.txt` (`scripts/ci_node_bootstrap.sh`); mise stays the way workers and
  the owner get Node locally. A failed download, checksum or version fails the job, and a guard step at the end of `quick`
  fails it when a web gate was skipped (issue #1118). Chromium is in the owner's runner image of option-trader since 2026-10-24;
  e2e is required (`web/.e2e-required`), and a runner without Chromium fails the gate.

## Not verified

- No repository inspection by either battle model; version and registry facts after June 2026 (Vite 8,
  TypeScript 7, TanStack Table 9, pnpm 12 settings (not used: pnpm 10 is pinned after the sandbox measurement), shadcn Nova
  and CLI flags such as `--base radix`) come from the draft.
- Nova: its name, look and CLI flag are not confirmed in the shadcn docs; step 1a confirms them in shadcn 4.22.1
  and records the init command.
- Compatibility inside the scaffolded set (shadcn, Tailwind, TypeScript, Vite, typescript-eslint, React, motion,
  Zustand, Ajv standalone) is untested; step 1a verifies it before freezing.
- Ajv standalone and CSP behaviour with the chosen bundle are untested.
- The shadcn Data Table recipe targets TanStack Table 8 or 9; the cap to 8 follows the recipe.
- Node 24 on the Mac mini and the CI runner is not installed or checked.
- Bundle size is unmeasured.
- The npm approval of 2026-10-22/23 was not re-read; "about 20 page kinds" comes from an inventory that missed
  the 持仓页 and 复盘页 modules.

## Appendix A: Chosen set (filled by the step 1a worker)

Chosen on 2026-10-23 (issue #1100). The candidate set came from `create-vite` and `shadcn init`; it became the
chosen set after `pnpm install --frozen-lockfile`, `pnpm lint`, `pnpm format:check`, `pnpm typecheck`,
`pnpm test` and `pnpm build` all passed. The exact versions are frozen in `web/package.json` and
`web/pnpm-lock.yaml`, and `web/dependency-allowlist.json` matches this table (a test in `tests/web/` checks it).

**Runtime** (ships in the bundle):

| Package | Exact version | Job | Licence |
|---|---|---|---|
| `@tanstack/react-table` | `8.21.3` | tables | MIT |
| `ajv` | `8.20.0` | schema-validation | MIT |
| `class-variance-authority` | `0.7.1` | component-helpers | Apache-2.0 |
| `clsx` | `2.1.1` | component-helpers | MIT |
| `recharts` | `3.10.1` | charts | MIT |
| `lucide-react` | `1.61.0` | icons | ISC |
| `motion` | `14.6.0` | animation | MIT |
| `radix-ui` | `1.8.1` | ui-components | MIT |
| `react` | `19.3.1` | ui-runtime | MIT |
| `react-dom` | `19.3.1` | ui-runtime | MIT |
| `tailwind-merge` | `3.7.0` | component-helpers | MIT |

**Toolchain and support**:

| Package | Exact version | Job | Licence |
|---|---|---|---|
| `@playwright/test` | `1.64.1` | layout-tests | Apache-2.0 |
| `@tailwindcss/vite` | `4.3.3` | styling | MIT |
| `@types/node` | `24.19.2` | type-declarations | MIT |
| `@types/react` | `19.3.0` | type-declarations | MIT |
| `@types/react-dom` | `19.3.0` | type-declarations | MIT |
| `@vitejs/plugin-react` | `6.1.2` | build | MIT |
| `eslint` | `10.13.0` | lint | MIT |
| `jsdom` | `30.1.3` | test-dom | MIT |
| `json-schema-to-typescript` | `16.0.0` | schema-types | MIT |
| `prettier` | `3.9.10` | format | MIT |
| `shadcn` | `4.22.1` | component-cli | MIT |
| `tailwindcss` | `4.3.3` | styling | MIT |
| `tw-animate-css` | `1.4.0` | styling | MIT |
| `typescript` | `5.9.3` | language | Apache-2.0 |
| `typescript-eslint` | `8.72.0` | lint | MIT |
| `vite` | `8.3.5` | build | MIT |
| `vitest` | `5.0.5` | unit-tests | MIT |

`@types/react`, `@types/react-dom` and `@types/node` are type declarations only (job `type-declarations`); the
scaffold needs them for `strict` TypeScript and the ADR tables do not name them. `tailwindcss` is the Tailwind 4
package that `@tailwindcss/vite` and the shadcn CSS import require. Designated, not yet installed: `zustand`,
`react-resizable-panels`, `@dnd-kit/core` (listed in the allowlist, absent from `package.json`).

**Versions that follow the caps and peer ranges.** TypeScript 5.9.3 (cap 5.x; 7.0.2 was `latest`),
`@tanstack/react-table` 8.21.3 (cap 8.x; 9.3.1 was `latest`), Vite 8.3.5 with Vitest 5.0.5 (Vitest declares no
Vite peer range that excludes it; `pnpm install` reported no peer warning), ESLint 10.13.0 with
`typescript-eslint` 8.72.0, `@types/node` 24.x to match Node 24.

**Node and pnpm.** Node `24.22.1` (Node 24 LTS "Krypton"), pinned in `web/.nvmrc` and `engines.node`, downloaded in CI as a
checksum-pinned tarball (mise for workers and the owner locally). pnpm `10.18.0` (`packageManager` in `web/package.json`); pnpm 12.x was not used (store-lock EPERM in the
worker sandbox, see the table above).

**pnpm settings** (in the `pnpm` field of `web/package.json`; `save-exact=true` in `web/.npmrc`):
- `minimumReleaseAge: 10080` (seven days).
- `onlyBuiltDependencies: []`: no package may run a lifecycle script. After the install `pnpm ignored-builds`
  reported none, so nothing needs to be allowed (esbuild is not in the graph: Vite 8 builds with Rolldown, whose
  native binary arrives as an optional dependency, not as an install script; Playwright downloads Chromium with
  `playwright install`, not at install time).

**Commands run** (worker sandbox, `NODE_USE_ENV_PROXY=1` set for every command that reaches the network):
1. `node create-vite/index.js web --template react-ts --no-interactive` (create-vite 9.3.1 from npm). The
   `pnpm create vite` form failed in the sandbox with `ERR_PNPM_EPERM` while copying the `.vscode` files of the
   other templates into the dlx cache, so the same package was fetched with `npm i` and run directly; the output
   is the same template. The scaffold's `oxlint` and its config were removed (ESLint is the designated linter).
2. `pnpm add -E react react-dom radix-ui class-variance-authority clsx tailwind-merge lucide-react motion
   @tanstack/react-table@8 recharts@3 ajv@8`
3. `pnpm add -E -D typescript@5 vite@8 @vitejs/plugin-react tailwindcss @tailwindcss/vite tw-animate-css vitest
   jsdom eslint typescript-eslint prettier @playwright/test json-schema-to-typescript @types/react
   @types/react-dom @types/node@24 shadcn@4`
4. **Nova init command:** `pnpm exec shadcn init --template vite --base radix --preset nova --yes --no-monorepo`
   (shadcn 4.22.1). It wrote `components.json` with `"style": "radix-nova"`, so the Nova name and the
   `--base radix` flag are confirmed; `--preset nova` is the spelling that selects it (`--defaults` would pick
   `base-nova`, the Base UI variant).
5. `pnpm exec shadcn add button badge card table alert accordion toggle tabs separator tooltip skeleton
   scroll-area --yes`
6. Edits after the CLI: the `radix-nova` registry imports `cn` from an npm package named `cn` (0.5.0); that
   package is not designated, so it was removed and the components import `cn` from `src/lib/utils.ts`, the
   standard shadcn helper built on `clsx` and `tailwind-merge`. `shadcn init` also added `@fontsource-variable/geist`
   (a web font, ADR: no web font); it was removed and the font stack is the system one. The dark theme variables
   were moved from `.dark` into a `prefers-color-scheme: dark` media query (ADR Theming: dark mode is CSS only).
   Ranges written by `pnpm add` were rewritten to exact versions.
7. `pnpm install --frozen-lockfile`, `pnpm lint`, `pnpm format:check`, `pnpm typecheck`, `pnpm test`,
   `pnpm build`: all exit 0.

**Outbound destinations** (the repository has no other list of approved destinations, so they are recorded here):
`registry.npmjs.org` (npm registry, install time only), `ui.shadcn.com` (shadcn registry, only when a component
is added or `shadcn init` runs), and the Playwright browser hosts (`cdn.playwright.dev` and
`playwright.download.prss.microsoft.com`, only when the owner runs `playwright install chromium` on the runner).
Tests and `pnpm test:e2e` make no network request (Vite preview on 127.0.0.1 only). CI holds no credentials.

## Appendix B: Registry state on 2026-10-23 (reference only)

npm "latest" read from the registry on 2026-10-23. Not a pin. Three majors here are new (TypeScript 7, TanStack
Table 9, Vite 8) and shadcn recipes may not target them yet; the chosen TypeScript and Table lines are capped at
5.x and 8.x (policy above). Tailwind 4 is not new.

| Package | Latest | Licence |
|---|---|---|
| shadcn | 4.22.1 | MIT |
| radix-ui | 1.8.1 | MIT |
| tailwindcss | 4.3.3 | MIT |
| lucide-react | 1.61.0 | ISC |
| motion | 14.6.0 | MIT |
| zustand | 5.0.16 | MIT |
| @tanstack/react-table | 9.3.1 | MIT |
| recharts | 3.10.1 | MIT |
| react | 19.3.1 | MIT |
| typescript | 7.0.2 | Apache-2.0 |
| vite | 8.3.5 | MIT |
| vitest | 5.0.5 | MIT |
| eslint | 10.13.0 | MIT |
| prettier | 3.9.10 | MIT |
| pnpm | 12.15.0 | MIT |
| react-resizable-panels | 4.14.8 | MIT |
| @dnd-kit/core | 6.3.1 | MIT |
