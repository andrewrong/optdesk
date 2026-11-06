# Week 1 wiring: store DSN names, the other page schemas, fixture validation

Author: advisor of optdesk (third draft, cut narrowed after the design review)
Source: `## Left out` of `docs/design/week1.md` (the earlier issues 13 wire-run-once, 15 page-schemas-28, 16
web-fixtures), cut against the code on origin/main at 9b6f5f0. The cut holds only what needs packages that are on
origin/main; the `cmd/ot` wiring needs packages of store-session-day, store-series-ops and jobs-fred-series of week1.md,
which have not merged, and is left out (see `## Left out`).

Goal of the design: the interface constants for the DSN names exist in `internal/store/api.go`, the other 29 page
schemas are in `contracts/schemas`, and every page fixture of `web/fixtures` is validated against them by a Go test.

## Modules

No change to `docs/design/modules.md`: every path below belongs to an existing row.

| issue | module |
|---|---|
| store-dsn-names | store (interface) |
| schemas-28 | contracts |

Every entry lies in one module. `store-dsn-names` is the one entry marked (interface): it edits the interface file
`internal/store/api.go`. `schemas-28` adds new files under `contracts/schemas`, which are not an interface file of the
map; AGENTS.md calls the JSON schema of a page the TypeScript interface, so that issue carries a Design note and
lands before any use (see its entry). No issue of this design edits `go.mod`, `go.sum`, `package.json` or
`pnpm-lock.yaml`; the Go test uses the santhosh-tekuri library that is already a direct dependency.

## Interface

Frozen names, each read on origin/main. The owner of a name is the module of its file.

Added by `store-dsn-names` (the only interface issue of this design), in `internal/store/api.go`:

| name | meaning |
|---|---|
| `DSNEnvRunner`, `DSNEnvSeriesFred`, `DSNEnvPages` | string constants `OT_PG_DSN_RUNNER` (role `ot_runner`), `OT_PG_DSN_SERIES_FRED` (role `ot_w_series_fred`), `OT_PG_DSN_PAGES` (role `ot_ro_pages`): the names of the lines `Setup` writes to `$OT_HOME/.env` and `cmd/ot` reads. Two modules need them, store (the writer) and ot (the reader), so they are declared once, in the interface |
| `AdminDSNEnv` | string constant `OT_PG_ADMIN_DSN`: the process environment variable that carries `SetupConfig.AdminDSN`; it is read from the process environment only, never from `.env` and never from a flag |

Constants named `OT_*` and `*DSN*` in Go files are not environment variables; `scripts/hygiene` scans no identifier
names and the `environment` job of `hygiene.yml` greps the environment of the runner, so the names are free in code. They can never
be set in a workflow `env:` block; the pg tests build their DSN in the test from the `PG*` variables of the test-pg
job.

Not frozen (Left out): `Page` and `Builder` of pages; any window rule; the names of the `cmd/ot` subcommands, flags and
the `.env` key of the FRED source.

## Issues

Both issues are in wave A: nothing waits on a sibling. Items under "Checked by CI" are decided by the workflow of the
head commit (ci-go, checks, hygiene); the facts above them are read from the diff.

### store-dsn-names

(interface) The constants `DSNEnvRunner`, `DSNEnvSeriesFred`, `DSNEnvPages` and `AdminDSNEnv` of `## Interface`. Declarations only; no module issue waits on behaviour.
  Goal: `Setup` writes, and `cmd/ot` reads, the names of the `OT_PG_DSN_*` lines; the landed interface has no name for them.
  Files: internal/store/api.go, internal/store/api_test.go.
  Design: docs/design/week1-wiring.md#store-dsn-names; the constants are in store and not in `cmd/ot` because `internal/store/setup` writes the same lines and may not import `cmd/ot`.
  Acceptance:
  - `internal/store/api.go` declares the four string constants with the values of `## Interface`, each with a doc comment naming the role it belongs to or, for `AdminDSNEnv`, the process environment.
  - Its import list is unchanged, and the file still holds only package, import, type, alias, const and interface declarations (no `func`, no `var`).
  - `api_test.go` asserts that the three `DSNEnv*` constants are distinct and start with `OT_PG_DSN_`, and that `AdminDSNEnv` is not equal to any of them.
  - Checked by CI: `go build`, `go vet`, `go test` of `internal/store`; go-arch-lint (import list of `api.go` unchanged); checks (`Contracts: unchanged`). Offline substitute: `grep -n 'func TestDSNEnvNames' internal/store/api_test.go` finds the function, and `grep -c 't.Parallel()' internal/store/api_test.go` is at least 1.
  Depends on: none (everything it edits is landed).
  Size: S


### schemas-28

The other page schemas of the old repository, copied with a manifest of the copies, and a Go test that validates every page fixture of `web/fixtures` against them.
  Goal: The schemas the migrated front end already follows are in `contracts/schemas`, and the 13 fixture directories copied by web-migrate are checked against them. This is the carry-over of week1.md#left-out, not the cleanup pass of week 2. Reading the files of the web module is a read, not an edit.
  Files: contracts/schemas/page.advice_page.v1.json, page.companies_page.v1.json, page.full_scan.v1.json, page.held_companies_summary.v1.json, page.home_page.v1.json, page.household_page.v1.json, page.household_risk.v1.json, page.household_summary.v1.json, page.indicator_detail.v1.json, page.ledger_page.v1.json, page.macro_page.v1.json, page.macro_summary.v1.json, page.market_summary.v1.json, page.market_weather.v1.json, page.option_page.v1.json, page.option_result_page.v1.json, page.options_summary.v1.json, page.positions_page.v1.json, page.positions_page.v2.json, page.positions_page.v3.json, page.report_page.v1.json, page.report_page.v2.json, page.settings_page.v1.json, page.stock_option_page.v1.json, page.structure_detail.v1.json, page.structure_detail.v2.json, page.structure_detail.v3.json, page.structures_list.v1.json, page.tracking.v1.json (all under contracts/schemas/), contracts/schemas/carryover.sha256, internal/contracts/carryover_test.go, internal/contracts/fixtures_test.go, docs/contracts/page-schemas.md.
  Design: docs/design/week1-wiring.md#schemas-28; interface note: no file of this issue is an interface file of `docs/design/modules.md`, but AGENTS.md calls the JSON schema of a page an interface, so the issue only adds files, edits no existing schema and lands before any issue that reads them; the content is the bytes of the source, so `$id` still names the old host (`option-trader.invalid`) and the manifest is deleted by the week-2 pass that renames it. The fixture test is a Go test with santhosh-tekuri/jsonschema/v6, already a direct dependency of go.mod, so there is no new dependency and no whitelist change; it is a Go file and not a vitest file because the front end only renders data (AGENTS.md). It sits in the same issue as the schemas because it reads them and both are in contracts.
  Acceptance:
  - The Files line lists 29 new schema files, each named `page.<name>.v<N>.json` and listed once; week1.md says 28, the list here holds 29. `page.macro_rates.v1.json` is not among them.
  - `contracts/schemas/carryover.sha256` has one line per listed schema, in the order of the Files line, each line `<sha256>  contracts/schemas/<file>`; `page.macro_rates.v1.json` is not in it.
  - `carryover_test.go` computes the sha256 of every `page.*.json` of `contracts/schemas` except `page.macro_rates.v1.json`, compares it with the manifest, fails on a file missing from the manifest or a manifest line without a file, and compiles each schema with santhosh-tekuri/jsonschema/v6 (the `x-decimals`, `x-timezone` and `x-rounding` keywords are unknown keywords the compiler ignores). The expected hashes are in the manifest, not computed from the schemas the test checks.
  - `fixtures_test.go` holds a table, typed by hand, from fixture directory to schema file: `advice-page`, `companies-page`, `full-scan`, `held-companies-summary`, `household-page`, `household-summary`, `macro-page`, `macro-summary`, `market-weather`, `option-page`, `structures-list`, `tracking` map to the `v1` schema of the same name with `-` as `_`, and `positions-page` maps to `page.positions_page.v3.json`; it fails when a directory of `web/fixtures` has no row.
  - `fixtures_test.go` loops over every `*.json` file of every fixture directory, the five extra files of `market-weather` (`intraday*.json`, `mismatch.json`) included, and skips the `README.md` files. It holds a named list, a Go `var` in the test file, of the fixtures expected to fail; each entry gives the fixture path and the schema keyword it fails (for example `bad-version.json` files fail on `const` of `schema_version`). Every fixture not on that list validates against its mapped schema; each fixture on the list fails, and with the named keyword. The list may be empty. The test also fails on a list entry whose file does not exist.
  - The schemas are compiled with the `x-decimals`, `x-timezone` and `x-rounding` keywords unknown (ignored); the fixture paths are relative to the test file (`../../web/fixtures`, `../../contracts/schemas`).
  - `t.Parallel()` in each test and each subtest of `fixtures_test.go`; no write, no network, no sleep.
  - `docs/contracts/page-schemas.md` is created (it does not exist on origin/main) with the headings `## Responsibility`, `## Inputs`, `## Outputs`, `## Invariants`, `## Version history`; it lists the 30 schema ids with their `schema_version` const and says in a sentence that the copies are unchanged and the week-2 pass writes new versions only.
  - `page.macro_rates.v1.json`, its fixtures and `MANIFEST.synthetic-v1.json` of `contracts/fixtures` are not in the diff; no file of `web/` is in the diff.
  - Every `x-timezone` value of `contracts/schemas/page.*.json` is `owner`: `grep -ho '"x-timezone": *"[^"]*"' contracts/schemas/page.*.json | grep -vc '"x-timezone": "owner"'` prints 0.
  - Checked by CI: ci-go (the tests, which prove that each hash is that of the copy and that the fixtures validate as listed). Offline substitute: `grep -n 'func TestCarryoverManifest\|func TestCarryoverTimezoneIsOwner' internal/contracts/carryover_test.go` and `grep -n 'func TestFixturesValidate' internal/contracts/fixtures_test.go` find the functions, and `grep -c 't.Parallel()'` on each of the two files is at least 1; checks (generated code unchanged since nothing is generated from these files); hygiene (no account id, path or e-mail); the size rule (JSON data is not counted).
  Note for the pull request description (not an acceptance item): `Contracts: ` followed by one `page.<name>.vN (new)` entry per file, as the format check of `checks.yml` accepts; the description names the file that the earlier count of 28 left out, and any fixture that is on the expected-failure list with the reason.
  Note (not acceptance): the copies are byte copies except the substitutions the privacy gate requires: every `x-timezone` value became `owner` (see PR 53).
  Depends on: none.
  Size: M

## Left out

- The `cmd/ot` wiring (issues `store-wiring-support`, `ot-run-once`, `ot-page-serve-setup` of the earlier draft): they need
  packages of store-session-day, store-series-ops and jobs-fred-series of `docs/design/week1.md` that are not on origin/main.
  Cut by a follow-up design written after those three issues land, re-reading the identifiers then.
- `store-wiring-support` (the SELECT grant of `ot_runner` on `master.session_day`, `Setup` writing the DSN lines under the
  frozen names, `docs/contracts/store.md`): cut by that follow-up design, after store-session-day and store-series-ops land.
- `ot-run-once` (`app` environment and dependencies, `wire.go`, `.env` reader): cut by that follow-up design, after the
  three issues land.
- `ot-page-serve-setup` (`ot page`, `ot serve`, `ot setup pg`): cut by that follow-up design, after `ot-run-once`.
- TypeScript type generation from the schemas (`json-schema-to-typescript`, a `gen:types` script, the TS step of
  `contracts/generate.sh`, the drift check of `.github/workflows/checks.yml`, `web/src/generated/*`): a later design
  gated by the owner. No consumer exists yet (the migrated pages keep their local types in `web/src/types/*`) and neither
  package is in `frontend-whitelist.json` (revision 2026-11-05.1); this design asks for no whitelist change. When the
  owner opens it, the old generator `web/scripts/gen-validator.mjs` also wrote `*.validate.js` wrappers, which a new script drops.
- The window rule: `ot run-once` uses `runner.NewFunc`, that is `reconcile.New` with the zero rule (calendar `XNYS`,
  no offsets). Passing a rule from a job (`WindowRule`, a `[window]` table in `job.toml`, `reconcile.NewWithRule` behind a
  Func type) waits for a populated calendar and for jobs-fred-series to land; its trigger is the first run on a real database.
- Population of `master.session_day`: the table is empty on a real database in week 1 (the calendar generator is left out
  by week1.md); loading `contracts/fixtures/master.session_day.r1.json` through `store.SessionDayWriter` is one more `ot`
  subcommand and would take the design over the size of this wave. Until then `ot run-once` on a real database returns
  `UnknownCalendar`; its tests use fakes only.
- `docs/contracts/jobs.md`: not touched here (it exists on origin/main since the jobs issue; `docs/design/contract-docs.md` completes it).
- Fixtures for the 17 schemas that have no fixture directory (and fixtures regenerated for the carried schemas): the old ones
  are written by a Python registry that is not in this repository.
- The moves of the migrated pages from `web/src/types/*` to generated types, and the week-2 cleanup pass over the schemas.
- `Page` and `Builder` names of pages: `ot page` and `ot serve` call the one `BuildMacroRatesFunc`.
- Embedding the real web build into `internal/pages/embed/dist` (a build step that writes into the pages module and the `vm`
  element contract of the built page): a later issue of pages with the owner.
- A named adapter type from `store.SessionDayReader` to `clock.SessionSource`: not chosen, the store satisfies the interface.
- Calling `reconcile.New`, `reconcile.NewWithRule` or any implementation by name outside `wire.go`: not chosen, the `ot` row
  uses the functions of the wiring table.
- A dotenv library for `.env`: not chosen, it would touch secrets and load them into the process environment.
- The broker, compute and notify wiring: later weeks.

## Adversarial

Source: scratchpad file design-week1-wiring.adversarial.md of the dispatcher window.
Note: the cut was narrowed after the design review; the entries below that name `ot-run-once`, `ot-page-serve-setup`,
`store-wiring-support` or `contracts-fixture-validation` refer to the earlier draft, whose issues are left out or folded
into `schemas-28`.

- 1a `cmd/ot` cannot take the design (main.go, `app`, no getenv/deps): adopted; main.go and main_test.go are in `ot-run-once` Files and `app` gains `getenv` and `deps`.
- 1b run-once fails on an empty calendar: adopted as wording; the goal says wired, not run on real data, and Left out names the population of `master.session_day`.
- 1c fixture validation covers four files per directory only: adopted; the test loops over every `*.json` and only `bad-version.json` must fail.
- 2a wiring-interface crosses four modules on an unlanded window shape: adopted; the window rule, `runner-window-rule` and `jobs-window-rule` are gone.
- 2a (imports sentence false, `contracts/api.go` imports context): adopted; the acceptance is deleted with the entry.
- 2b store-wiring-support edits files of unmerged issues and cites missing docs: partly adopted; it waits for both issues, and `store.md` is created if absent; the unlanded names stay a risk named in its Design line.
- 2c hygiene names: adopted; the note on `OT_*` constants, the DSN built in the pg test from `PG*` parts, and no literal DSN string.
- 2d interface marks (ts-generate, schemas-28): adopted for schemas-28 (Design note); ts-generate is Left out; the "only interface issue" sentence is limited to `store-dsn-names`; schemas-28 lists explicit file names.
- 2e acceptance not judgeable from the diff: adopted; no source hash or count is judged by running, the manifest content and file list are diff facts, the hash proof is under Checked by CI, the sentinel DSN is tested by injection.
- 2f go-toml promotion and pnpm-lock: adopted by removal; no `[window]` reader, no pnpm-lock edit.
- 3 simpler way (Go test, cut window machinery, defer TS, merge setup): adopted in full; setup is folded into `ot-page-serve-setup` (seven items).
- 4 lock-in (DSN names, manifest churn, generator pin, `[window]` keys): DSN names accepted and frozen in `store-dsn-names`; manifest deletion by week 2 stated; the other two vanish with the cut.
- 5 claims likely false (only interface issue, init registration without main.go, imports sentence, generator options, headings of missing docs): all corrected above.
- 6 ordering of unlanded dependencies: adopted in part; `ot-run-once` depends on the unlanded issues only through `wire.go`'s imports, and no issue waits on the owner.
