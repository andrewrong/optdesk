# Modules of optdesk

Author: advisor of optdesk (first draft)
Owner approved: (pending)
Revision: 1

Source: restart plan v2.5, parts 2 (three layers), 4 and 4.1 (module table), 8 (repository). The plan text
says "ten modules" and its table lists eleven rows (contracts, clock, broker, sources, store, compute, runner,
jobs, notify, pages, web); this map follows the table. The rows pyside, ci, deploy, repo, ot and docs are added, and `accounts` is a library module outside the layers that only `ot` depends on, so that every
path of the planned tree belongs to a row; they are not layer modules and no Go package depends on them, except that
`cmd/ot` (row ot) imports `internal/profile` of repo and `internal/accounts` of accounts.

How the table is read (`crew issue check`, `crew ask draft`): the table under `## Modules` has the columns
module, paths, entry, tests, depends. A path belongs to the first row one of whose globs matches it (`**` is any
depth), else to the first row whose tests match it. Rows are ordered so that a narrow glob comes before the
wide one that would swallow it (contracts before web and pyside, clock before pyside, every module before docs).
A path that no row matches is an error: `crew issue check` reports "<path> is in no module of
docs/design/modules.md" (checked in `moduleFindings` of agent-crew), so the last row `docs` takes `docs/**` and the
rows above name every other top-level path of the planned tree.

- **entry** is the interface file of the module: Go, one declarations-only file per package (`api.go`) with
  a test of the contract from both sides (`api_test.go`: one side a fake that implements it, the other a caller
  that uses it); Python, `api.py` of the package with `tests/contracts/test_<module>_api.py`; TypeScript, the
  generated types and the JSON schema of a page (they are owned by contracts, so web has no entry of its own).
  Editing an entry file is an interface change (AGENTS.md): the issue carries a Design line.
  Two exceptions, test kits: the entry cell of compute names `internal/compute/contracttest` and the entry cell of symbol names `internal/symbol/contracttest`, each a package of golden suites that imports `testing` and so holds behaviour, not declarations. It is listed so that a replacement implementation of that module's interface can run the suite; it is not imported by `cmd/ot`, and a package outside its module imports it from `_test.go` files only (go-arch-lint skips those; `docs/design/compute-core.md`, Decision 11).
  `scripts/arch` reads the kind of a package from the tree, not from an entry cell: for each `internal/<row>/<package>` directory it looks at the non-test `.go` files. A package is generated (`internal/contracts/gen` and `internal/contracts/genpb`, never an implementation package), declarations-only (`api.go` and no other non-test `.go` file except `doc.go`: no logic), a test kit (a non-test file imports `"testing"`, so it holds behaviour, not declarations) or an implementation package (any other package with a non-test `.go` file). The owning row of a package is the first row whose path glob matches it. A declarations-only package is listed so that other modules import an interface without importing its implementation (the implementation, `internal/store/barpg` for `internal/store/bar`, is a separate package). It is imported like the root package of its row: the root component and the sub-packages of any row that has its row in the depends cell may import it, and so may the sub-packages of its own row (jobs and `jobs/*` import `store/bar`). This is the general import rule for declarations-only packages, and this paragraph is its one statement; the `depends` paragraph below refers to it. A test kit is imported by the sub-packages of the rows that depend on its row, never by a root component, its own row's sub-packages or `cmd/ot` (`_test.go` files, as above). A package is a test kit only when a non-test file imports `"testing"`: a fake package that does not (such as `internal/notify/notifytest`) is an implementation package and `cmd/ot` may import it. The depends cell of `ot` is therefore the only control on what `ot` imports, besides the package kinds. A new `api.go` package or test kit belongs to the first matching row, and adding one is an interface change with a Design line. The entry cells are not read by `scripts/arch`.
  From entry 3 of `docs/design/jobs-api-slim.md` on, in the jobs row the entry file holds only the types that two or more packages other than `cmd/ot` and the root `internal/jobs` use (checked by `internal/jobs/api_users_test.go`, `docs/design/jobs-api-slim.md`); a job's constructor type, stores, config and result types live in the ordinary files of its package, and `cmd/ot` imports the package.
  A generated sub-package named in an entry cell (`internal/contracts/genpb`) is exempt from the declarations-only rule, because it is generated from `contracts/proto` and never edited by hand (AGENTS.md).
- **depends** is the only source of module dependencies. `arch.toml` and `.go-arch-lint.yml` (go-arch-lint) are derived from this column
  by `go run ./scripts/arch` at check time (`ci-go` runs it before go-arch-lint). From `arch-generated` entry 2 on they are in `.gitignore` and never committed, and only that entry
  names them in `Files:`; until it merges they are still committed (`docs/design/arch-generated.md`). The `ci-go` step fails when this column does not derive.
  The derivation: for a row named `<name>`, the root package `internal/<name>` (its `api.go`, the interface) is the
  go-arch-lint component `<name>`, and each sub-package `internal/<name>/<sub>` is its own component `<name>/<sub>`;
  `internal/<name>/**` is never one component. The component `<name>` may import the root components named in the
  row's depends cell, the standard library and external modules; a sub-package component `<name>/<sub>` may import
  the same and the root component `<name>`. A sub-package `<name>/shared` may be imported by its row's other sub-packages, and it imports no sibling. The root component `contracts` may also import its own sub-package component `contracts/gen`. The generated sub-packages `contracts/gen` and `contracts/genpb` have empty import lists, so the only edge between them and their root is contracts to contracts/gen. A sub-package component `<name>/<sub>` may also import the declarations-only packages and test kits of the rows in its depends cell, and the generated package `genpb`, as the paragraph on package kinds above says (for example `compute/contracttest`, the test kit, `store/bar` and `compute/trackapi`, declarations-only, and `contracts/genpb`); `genpb` is the one generated package that sub-packages of dependents import (`contracts/gen` is imported by the root `contracts` only); the root component of such a row receives only a declarations-only one, by the paragraph above, which is the one statement of it. `scripts/arch` discovers these packages from the tree and has no sub-package named in code except `genpb` (its constant `importableGenerated`), the two generated ones and `shared` (from `runner-due-gate` entry 2 on). No component may import a sub-package component of another row, so other
  modules import only the root package (the interface) of a module; the exceptions are the declarations-only packages, test kits and `genpb` that the previous sentences name, and `ot`, below. The one directory whose name differs from its row is
  `internal/profile/`, owned by repo: its component `profile` has an empty import list (standard library and
  external modules only, `log/slog` among them), which is also the depends cell of repo. Nothing imports `profile` except `cmd/ot`.
  The row `ot` (path `cmd/**`) is marked as the composition root of the program, as point 5 of the design rules provides: the one
  `package main` of the binary, which may import the implementation packages of the rows in its depends cell, and which no module imports; the program has
  no other such row. Its entry cell is empty (no row depends on `ot`, and `main.go` holds behaviour, so it is no interface file); its depends cell lists the rows whose root packages it may import (below). The reason it is not held to interface-only imports: Go cannot construct an
  implementation without importing the implementation's package, so a composition root must name those packages; `ot` is not a module that
  other modules use, and it holds no logic beyond wiring (flag parsing, the subcommand registry, the constructors of the wiring table in
  `docs/design/week1.md#interface`). The interface-only rule therefore binds the library modules (every row with an interface file in
  `internal/`), and for `ot` this map states the rule of its import list, not the list: the depends cell of `ot` plus the package kinds above, from which `scripts/arch` generates go-arch-lint component `ot` and
  `ci-go` lints against: the root packages of the rows in its depends cell (`internal/profile` for repo, a root directory and not a discovered sub-package) and every discovered implementation package of those rows, each found from the tree, with the declarations-only packages of those rows; a test kit and a generated package are never imported by `cmd/ot`.
  Any other import of a package of a module by `cmd/ot`, such as an implementation package of a row outside its depends cell, is a finding. A row whose paths hold no `internal/` directory (web, pyside, ci, deploy, docs) gives no go-arch-lint component; `ot` gives
  the component `ot` for `cmd/ot`.
  The bar jobs add four such imports to `cmd/ot` (docs/design/market-bars.md, entry 13): `internal/store/bar` (declarations only), `internal/store/barpg`, `internal/jobs/barsrefresh` and `internal/jobs/fxrefresh`; all four follow from the rule above and need no new row.
- **Layers** (plan 2): data = contracts, clock, broker, sources, gaphistory, symbol, store, compute (in that order);
  scheduling = runner, jobs, notify; front = pages, web. Within a layer a module may depend only on a module
  listed before it in its layer; across layers only scheduling to data and front to data are allowed. No layer
  module depends on the front, on ci or docs (only `ot`, outside the layers, depends on the front), and only `ot` depends on repo (its `internal/profile`).
- The pyside row follows the AGENTS.md layout (owner decision of 2026-11-06): the package is `optdesk`, the interface of a
  Python module is `pyside/src/optdesk/<module>/api.py` and its contract test is `pyside/tests/contracts/test_<module>_api.py`
  (docs/design/pyside-layout.md); the interface of contracts for Python is `pyside/src/optdesk/contracts/api.py` (it re-exports the generated package) and pyside imports only that file of contracts. The calendar generator is `pyside/src/optdesk/calendar/` and a path of clock (the clock row comes before the
  pyside row, so first-glob-wins keeps it clock's); the Python generated from the proto, `pyside/src/optdesk/contracts/genpb/**`, is a path of contracts.
- Size rule, one text: a source file that is new in a change, a copied file included, has at most 600 lines; a file of the base revision over 800 lines is listed in `docs/design/ratchet.json` with a ceiling and stays under it; a ceiling is never raised by an agent (the text of AGENTS.md). This map does not repeat ceilings.
- Ownership of shared paths, decided here: of the root files, `Makefile`, `go.mod`, `go.sum`,
  `.golangci.yml`, `.gitignore`, `AGENTS.md`, `README.md` and `LICENSE` belong to repo (the root paths of its row), so an issue that adds a Go
  dependency is a judged two-module entry (its module and repo) and names `go.mod` and `go.sum`; `package.json`,
  `pnpm-lock.yaml`, `pnpm-workspace.yaml` (pnpm 10.x pinned in `packageManager`) and `docs/adr/0034-frontend-stack.md` belong to web (every other ADR belongs to docs);
  `contracts/**`, `docs/contracts/00-protocol.md`, `docs/contracts/datasets/**`, `docs/contracts/figures/**`,
  `docs/contracts/page-schemas.md` and the code generated from contracts (`internal/contracts/gen/**`,
  `internal/contracts/genpb/**`, `web/src/generated/**`, `pyside/src/optdesk/contracts/genpb/**`) belong to contracts, so a source and its output are one module;
  each other `docs/contracts/<module>.md` belongs to its module; the migration runner and `migrations/**`
  belong to store; `web/test/**` belongs to web; `tests/**` (tests of the repo scripts) belongs to repo;
  `cmd/**` belongs to ot (the composition root); `scripts/check-frontend-deps.mjs` and `frontend-whitelist.json` belong to web (the web row comes before the `scripts/**` of repo); `pyproject.toml` and `uv.lock` of Python live under `pyside/` (the
  pyside row); `deploy/**` has its own row and no week-1 issue.
- Owner-only paths of the project declaration (an agent's pull request that touches one needs the owner's merge):
  `.github/` (ci), `contracts/proto/` (contracts), `deploy/` (deploy), `pyside/src/optdesk/ibkr/transport.py` (pyside),
  `docs/design/ratchet.json` (docs).
- Modules not touched in week 1: broker, compute and notify keep their paths, entry and tests cells (crew refuses a depends on a row
  with no entry), but none of their files exists until the weeks named in the second table (broker 3-4, compute 8-9; notify is stage 1, see below).
  The depends cells of jobs (added by `docs/design/option-jobs.md`, which follows the broker interface entry of
  `docs/design/broker.md`) and of ot (added by `docs/design/ot-collect.md`) name broker, so broker is no longer untouched in that sense. The depends cell of ot also names notify (added by `docs/design/notify.md`, which also adds `internal/notify/core`, `internal/notify/telegram` and `internal/notify/dryrun` to the import list of ot), so notify is no longer untouched either and its files come with that design (its interface entry first, stage 1, not week 11). The depends cells of jobs, ot, notify and pages name compute (jobs and ot by earlier designs, notify and pages added by `docs/design/risk-alerts.md`), so compute is no longer untouched in that sense; its packages are on main. No other depends cell names compute yet: the issue whose code first imports one of them adds it to the depends cell of the importing row in that issue.

## Modules

| module | paths | entry | tests | depends |
|---|---|---|---|---|
| contracts | contracts/** buf.yaml buf.gen.yaml internal/contracts/** web/src/generated/** pyside/src/optdesk/contracts/** pyside/tests/contracts/test_contracts_api.py docs/contracts/00-protocol.md docs/contracts/datasets/** docs/contracts/figures/** docs/contracts/page-schemas.md | internal/contracts/api.go internal/contracts/genpb web/src/generated/*.ts pyside/src/optdesk/contracts/api.py | internal/contracts/api_test.go internal/contracts/schema_test.go pyside/tests/contracts/test_contracts_api.py |  |
| clock | internal/clock/** docs/contracts/clock.md pyside/src/optdesk/calendar/** pyside/tests/contracts/test_calendar_api.py pyside/tests/calendar/** | internal/clock/api.go pyside/src/optdesk/calendar/api.py | internal/clock/api_test.go pyside/tests/contracts/test_calendar_api.py | contracts |
| broker | internal/broker/** docs/contracts/broker.md | internal/broker/api.go | internal/broker/api_test.go | contracts clock |
| sources | internal/sources/** docs/contracts/sources.md | internal/sources/api.go | internal/sources/api_test.go | contracts clock |
| gaphistory | internal/gaphistory/** docs/contracts/gaphistory.md | internal/gaphistory/api.go | internal/gaphistory/api_test.go | contracts |
| symbol | internal/symbol/** docs/contracts/symbol.md | internal/symbol/api.go internal/symbol/contracttest | internal/symbol/api_test.go internal/symbol/contracttest |  |
| accounts | internal/accounts/** docs/contracts/accounts.md | internal/accounts/api.go | internal/accounts/api_test.go |  |
| store | internal/store/** migrations/** docs/contracts/store.md docs/contracts/store/** | internal/store/api.go internal/store/jobrequest internal/store/hoursnotice internal/store/bar internal/store/seriesdef internal/store/stockreg internal/store/optionday internal/store/fundkey internal/store/ownerset internal/store/mark internal/store/flexsection internal/store/dividend internal/store/openquote internal/store/runview internal/store/underlying internal/store/readingkey internal/store/optionledger/api.go internal/store/strategyresult | internal/store/api_test.go | contracts gaphistory |
| compute | internal/compute/** docs/contracts/compute.md docs/contracts/compute/** tools/compute-golden/** | internal/compute/api.go internal/compute/contracttest internal/compute/trackapi internal/compute/settingsapi internal/compute/blockapi internal/compute/valuationapi | internal/compute/api_test.go internal/compute/contracttest | contracts |
| strategy | internal/strategy/** docs/contracts/strategy.md | internal/strategy/api.go | internal/strategy/api_test.go | contracts compute |
| runner | internal/runner/** docs/contracts/runner.md | internal/runner/api.go | internal/runner/api_test.go | contracts clock store |
| jobs | internal/jobs/** docs/contracts/jobs.md docs/contracts/jobs/** | internal/jobs/api.go | internal/jobs/api_test.go | contracts clock sources broker store compute strategy symbol |
| notify | internal/notify/** docs/contracts/notify.md | internal/notify/api.go | internal/notify/api_test.go | contracts store compute gaphistory |
| pages | internal/pages/** docs/contracts/pages.md | internal/pages/api.go | internal/pages/api_test.go | contracts store compute |
| web | web/** package.json pnpm-lock.yaml pnpm-workspace.yaml frontend-whitelist.json scripts/check-frontend-deps.mjs docs/adr/0034-frontend-stack.md |  | web/test/** | contracts |
| pyside | pyside/** pyside/pyproject.toml pyside/uv.lock | pyside/src/optdesk/ibkr/api.py | pyside/tests/contracts/test_ibkr_api.py | contracts |
| ci | .github/** |  |  |  |
| deploy | deploy/** |  |  |  |
| repo | go.mod go.sum Makefile internal/profile/** scripts/** tests/** arch.toml .go-arch-lint.yml .golangci.yml .gitignore AGENTS.md README.md LICENSE | internal/profile/api.go | internal/profile/api_test.go |  |
| ot | cmd/** docs/contracts/ot.md docs/contracts/ot/** tools/ot-flex-export/** tools/ot-history-export/** |  | cmd/ot/main_test.go | contracts clock sources broker store runner jobs notify pages repo compute strategy gaphistory symbol accounts |
| docs | docs/** |  |  |  |

## Contracts of the modules

Inputs, outputs and forbidden edges as the plan states them (part 4); the contract document is the page of the
module under `docs/contracts/` (template of plan 5.1). This table is read by people, not by crew.

| module | layer | inputs | outputs | forbidden | contract document |
|---|---|---|---|---|---|
| contracts | data | schemas, protos, job manifests written by people | generated Go structs, TS types, Python stubs; the edges of the generated arch.toml (derived from the table above) | any runtime logic | docs/contracts/00-protocol.md |
| clock | data | the calendar table master.session_day (a yearly generated file, cross-checked against IBKR trading hours) | Clock, Session, Slot, ET/JST/owner-zone conversion | hand-written open and close times (ADR 0004) | docs/contracts/clock.md |
| broker | data | the pyside gRPC service | typed records, account ids aliased | opening the database, arithmetic, retry amplification | docs/contracts/broker.md and contracts/proto/ibkr/v1 (first issue: week 3-4) |
| sources | data | FRED, SEC, HTTP, Stooq responses | typed records with source revision and published_at | opening the database; knowing the runner or pages | docs/contracts/sources.md |
| gaphistory | data | the rows of the view rd.job_gap_r1 as read by store | one read interface, `GapsSince`, with its own `Gap` row type | opening the database, arithmetic, writing | docs/contracts/gaphistory.md |
| symbol | data | a stock symbol of the IBKR contract | the spelling of a vendor, the option root | opening anything, any network, any state | docs/contracts/symbol.md |
| accounts | outside the layers | the file `$OT_HOME/accounts.toml` | the map from an IBKR account id to the owner alias, `Book` | any other file, any network, opening the database, logging an id | docs/contracts/accounts.md |
| store | data | records | one Writer and one Reader per dataset, migrations, roles rw and ro | being bypassed: only store imports pgx (depguard) | docs/contracts/store.md and docs/contracts/datasets/ |
| compute | data | typed records | figure values with register id and version | any I/O | docs/contracts/compute.md (first issue: week 8-9) |
| strategy | data | a `Snapshot` of stored inputs and an open `Position` | `Evaluation`, `Action`, `Params` of one strategy; one package per strategy behind `Strategy` | any I/O, any clock, importing store, jobs or pages | docs/contracts/strategy.md |
| runner | scheduling | job manifests (window rules), the key set each job Plan returns, the run table | run, attempt, job_event; ops.capture_expected_key and ops.dataset_window; control socket | knowing dataset content (keys are opaque) | docs/contracts/runner.md |
| jobs | scheduling | broker, sources, store, compute | dataset writes and a JobResult | importing another jobs package | job.toml per package and docs/contracts/jobs.md and docs/contracts/jobs/<job>.md |
| notify | scheduling | job_event, push list | morning report, push | account-derived information (ADR 0010) | docs/contracts/notify.md (first issue: `notify-interface` of `docs/design/notify.md`, stage 1) |
| pages | front | Reader, compute | vN JSON of a page | computing, writing the database | docs/contracts/pages.md |
| web | front | vN JSON | pages | computing, reading the database | docs/adr/0034-frontend-stack.md |
| pyside | outside the layers | IB Gateway through the TWS API | the gRPC services of contracts/proto/ibkr/v1 | opening the database, business numbers | contracts/proto/ibkr/v1 |
| ci | outside the layers | the repo | workflows, Dependabot | secrets in files | none |
| deploy | outside the layers | host service definitions (owner-only) | deployment files | secrets in files; no week-1 issue | none |
| repo | outside the layers | none | profile, scripts, lint and arch configuration | business logic | README.md |
| ot | composition root, outside the layers | the root packages of the rows in its depends cell and every discovered implementation package of those rows (`profile` is the root of repo) | the `ot` binary; and the dev-only exporters `tools/ot-flex-export/` and `tools/ot-history-export/` (each its own uv project, not part of the binary, run by hand, imported by nothing at run time) | any logic beyond wiring in `cmd/` | docs/contracts/ot.md |
| docs | outside the layers | design and decisions | design files, ADRs, ratchet (docs/design/ratchet.json is owner-only) | none | none |
