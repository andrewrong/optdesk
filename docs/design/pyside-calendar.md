# Pyside calendar: the generator of `master.session_day` and the workflow ci-pyside

Author: the optdesk dispatcher (first draft)
Source: the advisor goal "the calendar generator that populates master.session_day from exchange_calendars, plus a ci-pyside
workflow"; `docs/design/week1-run.md` (`## Left out`: the generator and its yearly files follow `pyside-layout.md`) and
`docs/design/pyside-layout.md` (`calendar` is a module of clock; ci-pyside must land before any calendar issue).

Goal (scope: the calendar XNYS only): the owner runs one command offline and gets a file in the format `ot calendar load` reads (this cut proves it with the Python copy of the gate; see `## Left out`), with its provenance header filled by the
generator, and every later pyside issue has a CI that runs `uv sync --locked`, `ruff check`, `mypy --strict` and `pytest -n auto`.
No broker, no secret, no network at test time; no trading hour is written by hand anywhere.

Read at the base (a384084, each by grep or read):

- `cmd/ot/calendar.go` decodes `{"provenance": {...}, "rows": [...]}`; provenance has the five strings `generator`,
  `generator_version`, `source_library`, `source_library_version`, `generated_at`, each non-empty, `generated_at` RFC 3339
  (`checkProvenance`); `rows` must be non-empty; `checkRows` demands a non-empty `calendar_id`, `session_day` strictly ascending
  within a calendar id, `open_at` before `close_at` on a session day, `open_at` equal to `close_at` and `early_close` false on a
  day that is not a session. The loader opens the store only after every file is checked.
- `cmd/ot/testdata/calendar_synthetic.json` is the format model: a non-session row has both instants at 00:00:00Z of its day.
- `contracts/datasets/master.session_day.r1.json`: six required fields (`calendar_id`, `session_day`, `is_session`, `open_at`,
  `close_at`, `early_close`), `additionalProperties: false`; `docs/contracts/datasets/master.session_day.md` says `session_day`
  is the market-zone date and the instants are UTC.
- `docs/contracts/clock.md`: "a known calendar has a row for every day, session or not".
- `docs/design/week1-run.md` says the envelope is defined there and that the clock row will only be a producer.
- `pyside/pyproject.toml`: `requires-python >=3.12`, `dependencies = []`, dev group `pytest pytest-xdist mypy ruff`, `[tool.mypy]
  strict = true`, pytest `addopts = "-n auto"`, `testpaths = ["tests"]`; the tree has only `optdesk/ibkr` and
  `tests/contracts/test_ibkr_api.py`; `pyside/uv.lock` exists.
- `.github/workflows/` has `checks.yml ci-go.yml ci-web.yml hygiene.yml` and no ci-pyside; `ci-web.yml` pins
  `actions/checkout` by commit sha and sets `permissions: contents: read`.
- `docs/design/modules.md`: the clock row already holds `pyside/src/optdesk/calendar/**` and
  `pyside/tests/contracts/test_calendar_api.py` and precedes the pyside row; `pyside/tests/calendar/` is not in any row but pyside's.
  `go run ./scripts/arch --check` prints `arch: ok` after the change below.

Checked in a copy at the base with uv 0.12.19: a directory `pyside/tests/calendar/` without `__init__.py`, holding
`test_calendar_x.py`, beside `tests/contracts/`, passes `uv run pytest -q` (5 passed) and `uv run mypy --strict src tests`
(no issues), so the layout is sound; test file names stay unique across directories.

Not verified here (library not installed in this worktree; the adversarial review installed it in a scratch environment and
reports): exchange_calendars 4.13.2, Apache-2.0 in the metadata field `License-Expression`, python `>=3.10`, no `py.typed`
(so `mypy --strict` reports `import-untyped`), a default range of 20 years back to about one year ahead unless `start` and `end`
are passed to `get_calendar` (which works for later years), `InvalidCalendarName` for an unknown id.

## Modules

One row changes, the clock row of `docs/design/modules.md` (done in this pull request; every other row is as it is):

| module | paths | entry | tests | depends |
|---|---|---|---|---|
| clock | internal/clock/** docs/contracts/clock.md pyside/src/optdesk/calendar/** pyside/tests/contracts/test_calendar_api.py pyside/tests/calendar/** | internal/clock/api.go pyside/src/optdesk/calendar/api.py | internal/clock/api_test.go pyside/tests/contracts/test_calendar_api.py | contracts |

Why: `pyside/tests/calendar/**` joins clock so the issue that writes the generator also owns its tests (a path under `pyside/tests/`
otherwise belongs to the pyside row); the entry and tests cells gain the calendar interface and its contract test, as
`pyside-layout.md` promised. `depends` is unchanged: the generator imports no generated code (it writes JSON). `scripts/arch`
generates `arch.toml` and `.go-arch-lint.yml` from the depends column only, so neither file changes; `go run ./scripts/arch --check`
prints `arch: ok`. No ot file is changed by this design.

## Interface

Owner: clock (`pyside/src/optdesk/calendar/api.py`, declarations only, standard library only). The file format is the one of
`week1-run.md` and `cmd/ot/calendar.go`; the dataset owner stays contracts. The generator is a producer and adds no field.

Declarations (frozen dataclasses and Protocols; no function with a body, so the contract test never calls a stub):

- `CalendarProvenance`: `generator`, `generator_version`, `source_library`, `source_library_version`, `generated_at` (all `str`).
- `SessionRow`: `calendar_id: str`, `session_day: datetime.date`, `is_session: bool`, `open_at: datetime.datetime`,
  `close_at: datetime.datetime` (both timezone-aware, UTC), `early_close: bool`.
- `CalendarFile`: `provenance: CalendarProvenance`, `rows: tuple[SessionRow, ...]`.
- `GenerateError(Exception)`: an id outside the supported set, an id the library rejects, a bad range or a duplicate id.
- `Builder` (Protocol, `__call__`): `(calendar_ids: Sequence[str], first: date, last: date, generated_at: datetime) -> CalendarFile`.
  Raises `GenerateError`. `generated_at` is an argument, so a test needs no clock.
- `Renderer` (Protocol, `__call__`): `(file: CalendarFile) -> str`, the JSON text of the file format, keys in the order of `contracts/datasets/master.session_day.r1.json` (`calendar_id`, `session_day`, `is_session`, `open_at`,
  `close_at`, `early_close`; the synthetic fixture orders them differently, and the loader reads by name), instants as RFC 3339 with the suffix `Z`, one row per line, a final newline.
- `Command` (Protocol, `__call__`): `(argv: Sequence[str], now: Callable[[], datetime], out: TextIO, err: TextIO) -> int`, the
  command line `python -m optdesk.calendar --calendar ID [--calendar ID ...] --from YYYY-MM-DD --to YYYY-MM-DD --out PATH`;
  returns 0, 2 for a usage or range error (including a repeated `--calendar`, an id outside the supported set), 1 otherwise.
- `SUPPORTED_CALENDARS: tuple[str, ...]`: `("XNYS",)`. Tokyo is not here, see `## Left out`.

Implementation (clock, `pyside/src/optdesk/calendar/`): `__init__.py`, `generate.py` (the one file that imports
exchange_calendars: `build_file`, `render_file`, `GENERATOR_VERSION`), `cli.py` (`run`), `__main__.py` (wires `run` to
`datetime.now`; the only place that reads the clock). `generate.py` calls the library directly and has no source seam: the library needs
no network and is fast, so the tests run against it and a seam would only be a second copy of its API.
Public names of the module equal the Protocols: `build_file` is a `Builder`, `render_file` a `Renderer`, `run` a `Command`.

Rules every producer keeps (the gate of `ot calendar load`, restated so tests check them):

1. One row for every calendar day from `first` to `last` inclusive, per calendar id, ordered by id as given and then by
   `session_day` ascending; `session_day` is the market-zone date the library reports. Each id appears once.
2. A session day: `is_session` true, `open_at` and `close_at` are the library's open and close in UTC, `open_at` before
   `close_at`; `early_close` is true exactly when the library lists the session as an early close.
3. A day that is not a session: `is_session` false, `early_close` false, `open_at` and `close_at` both the instant 00:00:00Z of its
   `session_day` (built with `time.min` and UTC, no time of day typed). This is a sentinel, not an instant: a consumer of a
   row with `is_session` false reads only that flag and never `open_at` or `close_at` (to be written into `docs/contracts/clock.md`
   by a later clock issue; see `## Left out`).
4. Provenance: `generator` is `optdesk.calendar`; `generator_version` is the constant `GENERATOR_VERSION` of `generate.py`, a
   string that is changed by hand in the same pull request as any change of the rules above (the version of the package
   `optdesk` is static and would never differ, so it is not used); `source_library` is `exchange_calendars`;
   `source_library_version` is the installed version of the library (`importlib.metadata`); `generated_at` is `now()` in UTC,
   RFC 3339. Renaming `generator` or a key means reloading files, so the names are final with this design.
5. No trading hour, holiday or early-close date is written in the code; each comes from the library, which needs no network.
   `build_file` passes `start=first` and `end=last` to `get_calendar`, so a file for a year ahead of the library's default range can be made.
   A library error is raised as `GenerateError` with a message that names the id or the range and nothing else.

Dependency (AGENTS.md, Libraries): a new direct runtime dependency `exchange_calendars`, licence Apache-2.0, mainstream, maintained,
stable API from major version 4, no network. Its transitive dependencies (numpy, pandas, python-dateutil, pyluach,
korean-lunar-calendar, toolz, tzdata) are many; that fails the "few transitive dependencies" criterion, so the pull request of the
dependency issue lists the full set from `uv.lock` with licences and the reviewer decides. Own implementation is not chosen:
a hand-kept holiday table is what the goal forbids. The library ships no `py.typed`: the dependency issue adds a
`[[tool.mypy.overrides]]` block for module `exchange_calendars` (`ignore_missing_imports = true`) to `pyside/pyproject.toml`. The values of the library are
`Any` for mypy, so `generate.py` converts every instant through one function that rejects a naive datetime, and a test covers it.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is checkable offline from a copy of the
repository after the packages are installed (`cd pyside` first for the Python commands). Locking a new dependency needs the package
index once; that step is under `Checked by CI`, not an offline item.

### ci-pyside-workflow

Goal: a workflow runs the four Python checks on every pull request that touches `pyside/`, so every later pyside issue has CI
(`pyside-layout.md` says it lands before any calendar issue).
Files: .github/workflows/ci-pyside.yml
Depends on: none
Design: docs/design/pyside-calendar.md#issues. The path is owner-only under `.github/` and the worker cannot edit it: this entry is
opened by the dispatcher (not as a worker issue), reviewed by the design review of its pull request and merged by the owner;
the issues below depend on it, so they wait for that merge.
Acceptance:
1. The workflow is named `ci-pyside`, triggers on `pull_request` and on `push` to `main` with the paths `pyside/**` and
   `.github/workflows/ci-pyside.yml`, and sets `permissions: contents: read` and a `timeout-minutes`, with no secret.
2. The job runs on `ubuntu-latest`, checks out with `persist-credentials: false` using the commit sha that `ci-web.yml` pins for
   `actions/checkout`, and installs uv with an action that `uses:` a full 40-character commit sha:
   `grep -c 'uses: ' .github/workflows/ci-pyside.yml` equals `grep -c -E 'uses: [^@]+@[0-9a-f]{40}' .github/workflows/ci-pyside.yml`.
3. From `working-directory: pyside` the steps run in order `uv sync --locked`, `uv run ruff check`, `uv run mypy --strict src tests`,
   `uv run pytest -n auto`.
4. No step sets a variable named `OT_*` or `*DSN*` (the `environment` job of `hygiene.yml` forbids them).
5. `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci-pyside.yml'))"` parses the file.
Checked by CI: the workflow runs on its own pull request (its path is in its filter) and is green. That the sha is the commit of
the tagged release is checked by the reviewer at the pull request; making the check required is the owner's act.

### calendar-interface (interface)

Goal: the calendar interface, its contract test and the locked dependency `exchange_calendars` (with the mypy override) exist, so the
generator needs nothing from a sibling issue.
Files: pyside/src/optdesk/calendar/__init__.py, pyside/src/optdesk/calendar/api.py, pyside/tests/contracts/test_calendar_api.py, pyside/pyproject.toml, pyside/uv.lock
Depends on: ci-pyside-workflow
Design: docs/design/pyside-calendar.md#interface. A judged two-module interface entry of clock (the first three files) and pyside
(the last two), as `modules.md` treats an issue that adds a Go dependency (its module and repo with `go.mod` and `go.sum`); the
dependency is part of the interface entry because the generator cannot import the library before the lock has merged. Own
implementation of the library is not chosen, see `## Interface`, Dependency.
Acceptance:
1. `api.py` holds the declarations of `## Interface` and no logic, and imports only the standard library:
   `grep -E '^(import|from) ' pyside/src/optdesk/calendar/api.py` lists only `__future__`, `collections.abc`, `dataclasses`, `datetime`,
   `typing`.
2. `api.py` has no function or method with a body other than a docstring or `...`, and the Protocols are the only callables:
   `grep -c -E '^def ' pyside/src/optdesk/calendar/api.py` prints 0.
3. The contract test has, from both sides, a fake `Builder`, `Renderer` and `Command` (plain classes that implement the Protocols) and
   a caller that uses them through `optdesk.calendar.api` only; it asserts rules 1 to 3 of the interface on the rows the fake builder
   returns, that `GenerateError` carries a message, that `SUPPORTED_CALENDARS` is `("XNYS",)`, and that the text of the fake renderer
   decodes with `json.loads` to the keys `provenance` and `rows`. It imports nothing from `optdesk.calendar` but `api`, and it
   never calls the implementation.
4. `pyside/pyproject.toml` lists `exchange_calendars` once under `dependencies` with the lower bound `4` and an upper bound below `5`, and
   no other direct dependency changes; it holds one `[[tool.mypy.overrides]]` block with `module = ["exchange_calendars"]` and
   `ignore_missing_imports = true`, and no other mypy setting changes.
5. `pyside/uv.lock` names `exchange-calendars` (`grep -c 'name = "exchange-calendars"' pyside/uv.lock` is at least 1).
6. `uv run python -c "import socket; socket.socket.connect = None; import exchange_calendars as x; print(len(x.get_calendar('XNYS').sessions))"`
   prints a number: loading a calendar needs no network.
7. In a copy, `uv run ruff check`, `uv run mypy --strict src tests` and `uv run pytest -n auto` pass.
Note for the reviewer (not acceptance; the pull request description is not a file): the description names the dependency with its
licence, printed by `uv run python -c "import importlib.metadata as m; print(m.metadata('exchange_calendars')['License-Expression'])"`,
lists every new package of the lock (expected: numpy, pandas, python-dateutil, pyluach, korean-lunar-calendar, toolz, tzdata) with
its licence, and reports any difference from the claims of `## Interface`.
Checked by CI: `uv sync --locked` (a stale lock fails; locking needs the package index once), `ruff check`, `mypy --strict`, `pytest -n auto`.

### calendar-generator

Goal: `python -m optdesk.calendar` writes a file that satisfies the rules of `## Interface`, from the library only, for XNYS.
Files: pyside/src/optdesk/calendar/generate.py, pyside/src/optdesk/calendar/cli.py, pyside/src/optdesk/calendar/__main__.py, pyside/tests/calendar/test_calendar_generate.py, pyside/tests/calendar/test_calendar_cli.py
Depends on: calendar-interface
Design: docs/design/pyside-calendar.md#interface
Acceptance:
1. On the real library for `XNYS` over one full calendar year of the past (`test_calendar_generate.py`): every day has a row; a
   Saturday is not a session; the first Monday of September (computed with `datetime` in the test, not typed) is not a session; at
   least one early close exists; every early-close session is shorter than the longest session of that year; every session has
   `open_at` before `close_at`; every instant is timezone-aware UTC; the rows pass `check_like_loader`, a function of the test file
   that repeats the rules of `checkRows` and `checkProvenance` of `cmd/ot/calendar.go`. No test types an hour of a session or a fixed
   UTC offset.
2. A range in a year later than the library's default range (computed from the test's own fixed range, a year far enough ahead)
   builds without error, because `start` and `end` are passed (`test_calendar_generate.py`).
3. `build_file` raises `GenerateError` for an id outside `SUPPORTED_CALENDARS` (a known library id such as another exchange is
   among them), for a repeated id, and for `last` before `first`; each message names the id or the range and holds no path or
   environment value.
4. A naive datetime from the library layer is rejected by the conversion function (a test passes one directly).
5. `run` (`test_calendar_cli.py`) with a fake `now` and `tmp_path`: writes the file to `--out`, returns 0, provenance holds the five
   values of rule 4 with `generated_at` equal to the fake clock and `generator_version` equal to `GENERATOR_VERSION`; an invalid
   date, `--to` before `--from`, a missing `--out`, no `--calendar`, a repeated `--calendar` or an unsupported id returns 2 and
   writes no file; two runs with the same fake clock write byte-identical files.
6. No network and no clock in tests: one test replaces `socket.socket.connect` (through `monkeypatch`) with a function that raises
   and runs the item 1 build under it; `grep -n -E 'time\.sleep|datetime\.now|date\.today|utcnow' pyside/tests pyside/src/optdesk/calendar`
   lists only the one call in `__main__.py`.
7. No time of day or zone literal in the source: `grep -c -E '[0-9]{1,2}:[0-9]{2}' pyside/src/optdesk/calendar/generate.py pyside/src/optdesk/calendar/cli.py pyside/src/optdesk/calendar/__main__.py`
   prints 0 for each; `grep -n -E 'ZoneInfo|pytz|[A-Z][a-z]+/[A-Z][a-z_]+' pyside/src/optdesk/calendar/generate.py pyside/src/optdesk/calendar/cli.py pyside/src/optdesk/calendar/__main__.py`
   lists nothing (the only zone name in the package is `UTC`, from `datetime.timezone.utc`).
8. Tests write only under `tmp_path`, use no fixed path or port, share no state; `wc -l` of each new file is below 600;
   `uv run ruff check`, `uv run mypy --strict src tests` and `uv run pytest -n auto` pass without any change of
   `pyside/pyproject.toml`.
Checked by CI: the same commands (ci-pyside).

## Left out

- Tokyo (XTKS): the calendar has a lunch break and `master.session_day` has one open and close per day, so one pair would silently
  cover the break; the project record also names a separate hours overlay for it. Tokyo needs a decision on the dataset (break
  columns, a contracts change) and is a later design, not "no new design".
- Yearly data files are not committed: a file carries `generated_at`, and the owner runs the
  generator and `ot calendar load` (`week1-run.md`); which years is the owner's choice.
- The contracts document of the file envelope (`week1-run.md` named it for this design) is a contracts file and a later contracts issue;
  so is the sentence of `docs/contracts/clock.md` that says a consumer reads only `is_session` of a closed day (rule 3).
- The Go test that loads a file made by the generator through `ot calendar load`'s reader is a follow-up after the generator has
  merged (it needs a sample produced by `python -m optdesk.calendar`, an output of an unmerged sibling, so it cannot be cut here).
  Until then the acceptance of this cut proves only the Python copy of the gate (`check_like_loader`); the follow-up is an ot issue
  whose sample is checked by rerunning `build_file` and `render_file` with the file's own `generated_at` and comparing bytes.
- A cross-check of the library's hours against the broker's hours (the broker design, `modules.md` clock row) stays out.
- Making ci-pyside a required check and branch protection are the owner's.
- A JSON-schema validation of rows with `jsonschema` is not added: a second new dependency for a check the Go loader already makes.

## Adversarial

Full answer: `adv-pyside-calendar.md` in the dispatcher scratchpad (a local path, kept out of this public file).

1. Gap against the goal: adopted. `start` and `end` are passed to `get_calendar` (rule 5) and tested for a later year; the Go test of a generated sample is a
   follow-up after the generator merges (`## Left out`), and this cut states that its acceptance proves only the Python copy of the gate; Tokyo moves to `## Left out` with the lunch-break reason; the broker cross-check is listed there.
2. Untyped library and the mypy fix: adopted. The override is in the interface entry, which carries `pyproject.toml`; the generator
   issue changes no pyproject line (acceptance 8); the `Any` values go through one checked conversion.
3. A simpler way: adopted in part. The `CalendarSource` seam and the `build`/`source` split are dropped (one `generate.py`, tests on the
   real library, which is offline and fast), and the skip-then-unskip dance is gone because the contract test uses fakes of Protocols.
   Dependency and interface are merged into one judged two-module interface entry (clock and pyside), as the design review asked.
4. Lock-in: adopted. `generator_version` is a hand-kept constant changed with the rules (rule 4, the package version is static); the
   closed-day sentinel is named as a sentinel with the consumer rule (rule 3); the names `generator` and the five keys are final.
5. False claims: adopted. No stub is called; the licence is read from `License-Expression`; python-dateutil is listed; a repeated id is
   exit 2 and `GenerateError`; the stub-calling and early-close-comparison items are replaced.
6. Slip causes: adopted. The interface and generator depend on `ci-pyside-workflow` (as `pyside-layout.md` requires), which is an
   owner-merged entry; locking is under `Checked by CI`; the manual run item is dropped; the `tests/calendar`
   layout was run once in a copy (`## Read at the base`).

Design review of this pull request (head b8ca06d), answered:

1. calendar-generator without the lock: adopted. The dependency moves into the interface entry as a two-module entry; the
   generator depends only on it.
2. calendar-sample-load needs an unmerged sibling: adopted. The issue is removed and listed under `## Left out`; the follow-up checks
   its sample by rerunning the builder with the file's `generated_at`.
3. Zone-name check: adopted, a named grep in generator acceptance 7.
4. Licence check in the description: adopted as a note for the reviewer, with the command; the file items of the entry carry the decidable checks.
5. Renderer key order: adopted, the order of the dataset schema is named.
