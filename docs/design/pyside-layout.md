# Design: pyside layout

Author: advisor of optdesk (first draft)
Owner approved: (pending)
Revision: 1

Source: owner decision of 2026-11-06 (optdesk-234, "1. a"): pyside follows the AGENTS.md layout
`src/<pkg>/<module>/api.py` (declarations only), tested by `tests/contracts/test_<module>_api.py`, managed by uv under
`pyside/`. This design settles the paths and cuts the one issue that creates the skeleton. It replaces the open point of
docs/design/week1.md (Left out), which is deleted there in the same pull request.

## Modules

The rows of docs/design/modules.md that change (every other row is as it is). The clock row stays before the pyside row
and the contracts row stays first, so first-glob-wins gives the calendar paths to clock and the generated Python to contracts.

| module | paths | entry | tests | depends |
|---|---|---|---|---|
| contracts | contracts/** buf.yaml buf.gen.yaml internal/contracts/** web/src/generated/** pyside/src/optdesk/contracts/genpb/** docs/contracts/00-protocol.md docs/contracts/datasets/** docs/contracts/figures/** docs/contracts/page-schemas.md | internal/contracts/api.go web/src/generated/index.ts | internal/contracts/api_test.go internal/contracts/schema_test.go |  |
| clock | internal/clock/** docs/contracts/clock.md pyside/src/optdesk/calendar/** pyside/tests/contracts/test_calendar_api.py | internal/clock/api.go | internal/clock/api_test.go | contracts |
| pyside | pyside/** pyside/pyproject.toml pyside/uv.lock | pyside/src/optdesk/\<module\>/api.py | pyside/tests/contracts/test_\<module\>_api.py | contracts |

Changes against the base revision: `pyside/gen/**` of contracts becomes `pyside/src/optdesk/contracts/genpb/**` (never
edited by hand, like `internal/contracts/genpb`); `pyside/tools/**` leaves clock and `pyside/src/optdesk/calendar/**` joins it
(with the contract test of the calendar interface, so the clock issue that writes the calendar generator owns both files; its entry and tests
cells gain the calendar `api.py` and test in the design that creates them); the pyside row gets its entry, test and the two
Python project files. The owner-only path of pyside in the project declaration becomes
`pyside/src/optdesk/ibkr/transport.py`; the declaration change is made by agent-crew separately (a note, not an issue here).

## Interface

Layout (package `optdesk`, src layout, one uv project at `pyside/`):

- `pyside/pyproject.toml`, `pyside/uv.lock` (committed), `pyside/src/optdesk/__init__.py`, `pyside/src/optdesk/py.typed`.
- One directory per module under `pyside/src/optdesk/<module>/` with `__init__.py` and `api.py` (signatures, types and docstrings,
  no logic) and its implementation files beside it. Another module imports `optdesk.<module>.api` and nothing else of it.
- Contract tests in `pyside/tests/contracts/test_<module>_api.py`, one per interface file, testing from both sides: a fake that implements the
  interface and a caller that uses it.
- Python modules of week 1 to 3-4: `ibkr` (interface `pyside/src/optdesk/ibkr/api.py`; the owner-only implementation file is
  `pyside/src/optdesk/ibkr/transport.py`, created by the broker design, not here), `calendar` (module clock, later design), `contracts/genpb`
  (generated from `contracts/proto/` by the contracts module, later design; Python generation needs `buf.gen.yaml` and `contracts/`, both owner-only or contracts files).

The `ibkr` interface, derived from the landed `contracts/proto/ibkr/v1/session.proto` (one rpc, `Ping`, response with `connected`):

- `SessionStatus`: a frozen dataclass with one field, `connected: bool`.
- `Session`: a runtime-checkable `typing.Protocol` with one method, `ping(self) -> SessionStatus`.
- The package name `optdesk` is final (it is the import path of all generated code and the path in the owner-only declaration; the build backend needs the project name `optdesk`). Never put `pyside/src/optdesk` itself on `sys.path` (the module `calendar` would shadow the standard library one); the package is installed by uv.
- `api.py` imports only the standard library: it does not import generated code. The hand-written `SessionStatus` and `Session` mirror the proto and are not a consumer in the sense of docs/contracts/00-protocol.md; only a generated import is. The first import of the generated `ibkr.v1` Python
  freezes `ibkr.v1` (docs/contracts/00-protocol.md: a version freezes at its first consumer), so that import belongs to the
  broker design with `transport.py`, which also adds order-free account and market data calls. The declarations here add no rpc and no order, account or
  symbol type.

Tooling (Python rules of AGENTS.md): `requires-python` 3.12 or later; dev dependency group `pytest`, `pytest-xdist`, `mypy`, `ruff`; `mypy`
runs `--strict` (set in `[tool.mypy]`); pytest settings in `[tool.pytest.ini_options]`: `testpaths = ["tests"]`, `addopts = "-n auto"`.
Libraries (AGENTS.md): all five are mainstream, maintained, permissively licensed (MIT); the build backend is `uv_build`
(MIT or Apache-2.0, the backend of the uv tool itself). No runtime dependency in this issue.

Checks of the existing scripts, run on the base revision for this design: `scripts/ratchet` counts a file by extension from the set `.go .py .ts .tsx .js .mjs .sh`
over `git ls-files --cached --others --exclude-standard`, so `.py` under `pyside/src` and `pyside/tests` is counted; `scripts/hygiene` scans every tracked file
whatever its extension; `.gitignore` already ignores `.venv/` and `__pycache__/`. No repo issue is needed.

Note for later (not an issue here): the workflow ci-pyside (owner-only, under `.github/`) filters on `pyside/**` and runs, from `pyside/`,
`uv sync --locked`, `ruff check`, `mypy --strict` and `pytest -n auto`.

## Issues

### pyside-skeleton

1. Pyside: the uv project, the empty package and the ibkr interface with its contract test (interface).
  Goal: Create the uv project under `pyside/` with the package `optdesk` and the first Python interface, `optdesk.ibkr.api`, declaring the session probe of `ibkr.v1` without importing generated code. Write `pyproject.toml` and the package files by hand with plain uv defaults (no `authors`, no extra index, no `exclude-newer`); do not run `uv init`; produce `uv.lock` only with `uv lock`. With no network the worker stops and reports, and never hand-writes `uv.lock`. The body rule of a declaration is one text: a docstring, optionally followed by `...`, and nothing else. This is the one (interface) entry of this design: the interface file and the project files it needs land together, so no sibling issue is needed. It adds no implementation, no `transport.py` and no code that contacts a broker.
  Files: pyside/pyproject.toml, pyside/uv.lock, pyside/src/optdesk/__init__.py, pyside/src/optdesk/py.typed, pyside/src/optdesk/ibkr/__init__.py, pyside/src/optdesk/ibkr/api.py, pyside/tests/contracts/test_ibkr_api.py.
  Design: docs/design/pyside-layout.md#interface
  Acceptance:
  - `pyside/pyproject.toml` declares `requires-python`, the dev group with `pytest`, `pytest-xdist`, `mypy` and `ruff`, `strict = true` under `[tool.mypy]`, and `-n auto` in the pytest options: `grep -n 'requires-python' pyside/pyproject.toml`, `grep -n 'strict = true' pyside/pyproject.toml` and `grep -n 'addopts' pyside/pyproject.toml` each find one line.
  - `pyside/uv.lock` is committed and locks the five tools: `grep -c '^name = "pytest-xdist"' pyside/uv.lock` prints 1, and the same grep for `mypy`, `ruff`, `pytest` and `optdesk` prints 1 each; `pyside/pyproject.toml` lists no `[project] dependencies` entry (no runtime dependency): `grep -n '^dependencies' pyside/pyproject.toml` prints nothing or `dependencies = []`.
  - `pyside/src/optdesk/ibkr/api.py` declares `class SessionStatus` (a frozen dataclass with the field `connected: bool`) and `class Session` (a `typing.Protocol` with the method `ping`): `grep -n '^class SessionStatus' pyside/src/optdesk/ibkr/api.py`, `grep -n '^class Session(' pyside/src/optdesk/ibkr/api.py`, `grep -n 'def ping' pyside/src/optdesk/ibkr/api.py` and `grep -n 'connected: bool' pyside/src/optdesk/ibkr/api.py` each find one line.
  - `api.py` holds declarations only and imports no generated code and no third-party module: `grep -nE '^(import|from) ' pyside/src/optdesk/ibkr/api.py` lists only standard-library modules (`dataclasses`, `typing`, `__future__`), and `grep -c 'genpb' pyside/src/optdesk/ibkr/api.py` prints 0; the file has no `def` with a body other than a docstring, optionally followed by `...` (checked by the test `test_api_declares_only` below).
  - `pyside/tests/contracts/test_ibkr_api.py` tests the contract from both sides with these functions, each found by `grep -n 'def test_' pyside/tests/contracts/test_ibkr_api.py`: `test_fake_satisfies_session` (a fake class with a `ping` method passes `isinstance(fake, Session)`), `test_caller_reads_connected` (a caller function that takes a `Session` returns `status.connected` from a fake that returns `SessionStatus(connected=True)`), `test_session_status_is_frozen` (assigning `connected` raises `dataclasses.FrozenInstanceError`) and `test_api_declares_only` (parses `api.py` with `ast` and asserts each function body is a docstring, optionally followed by `...`, and nothing else).
  - The tests are parallel-safe and offline: the test file holds no module-level mutable state, no `time.sleep` (`grep -c 'sleep' pyside/tests/contracts/test_ibkr_api.py` prints 0), no socket or `grpc` import (`grep -cE 'socket|grpc|ib_insync|ibapi' pyside/tests/contracts/test_ibkr_api.py` prints 0) and writes no file (`grep -c 'open(' pyside/tests/contracts/test_ibkr_api.py` prints 0); a test that needs a file uses `tmp_path`.
  - `grep -cE '@|/Users/|/Volumes/' pyside/pyproject.toml pyside/uv.lock` prints 0 for each file (no authors e-mail, no local path).
  - Each of the seven files is under 600 lines (`wc -l` of each prints less than 600), and `git diff --name-only origin/main` lists exactly `pyside/pyproject.toml`, `pyside/uv.lock`, `pyside/src/optdesk/__init__.py`, `pyside/src/optdesk/py.typed`, `pyside/src/optdesk/ibkr/__init__.py`, `pyside/src/optdesk/ibkr/api.py` and `pyside/tests/contracts/test_ibkr_api.py`.
  - `git status --porcelain` is empty after the four commands have run. `.gitignore` belongs to repo, another module, so this issue does not edit it: the caches `.pytest_cache`, `.mypy_cache` and `.ruff_cache` stay untracked because each tool writes an inner `.gitignore` into its own directory (not by an entry of the root `.gitignore`).
  Checked by CI: `uv sync --locked`, `ruff check`, `mypy --strict` and `pytest -n auto`, each run from `pyside/` (workflow ci-pyside, created by the owner outside this design; until it exists, the author runs the four commands and states the results in the pull request); `uv sync --locked` fails when `pyside/uv.lock` and `pyside/pyproject.toml` disagree; `pytest -n auto` runs the four tests in parallel and passes; `mypy --strict` passes on `src` and `tests`.
  Note (not acceptance): the pull request description pastes the output of `uv --version` and the four commands, because no CI runs Python until ci-pyside lands. `Contracts: unchanged`; the new dev dependencies and the build backend are named in the pull request with their licenses (all MIT or Apache-2.0).
  Depends on: none. Order: first and only.
  Size: S

## Left out

- The calendar generator, `pyside/src/optdesk/calendar/` with its `api.py` and contract test (module clock): a later design, which cross-checks the table against IBKR trading hours (ADR 0004). This design only gives clock the paths.
- `pyside/src/optdesk/ibkr/transport.py` and any call of a live broker: the broker design (weeks 3-4); `transport.py` is owner-only, and the crew declaration change to that path is made by agent-crew separately.
- Python generated from the proto, `pyside/src/optdesk/contracts/genpb/**`, with its `buf.gen.yaml` and `contracts/` changes (owner-only paths): a later design of contracts; the first import of it freezes `ibkr.v1`.
- The workflow ci-pyside (`.github/`, owner-only, merged by the owner) must land before any broker or calendar issue; until then no CI runs Python. Likewise the crew declaration change to the new owner-only `transport.py` path is a precondition of the broker design.
- Drift between the hand-written `api.py` types and the proto: the contracts or broker design must either generate the `api.py` types or add a field-comparison test against the generated message, so drift is caught; until then the proto is the owner. The first generated import freezes `ibkr.v1`.
- Any change of `.gitignore`, `scripts/ratchet` or `scripts/hygiene`: not needed or owner-made; the extension check is in `## Interface`.
- Account, market-data and order messages of the proto, and gRPC runtime dependencies: broker design.

## Adversarial

Answers to the adversarial review (kept on the dispatcher's machine as pyside-layout.adversarial.md):

1. Gap against the goal: adopted. The hand-written mirror is not a consumer and the generated import freezes `ibkr.v1` (`## Interface`); `week1.md` names the new calendar path; ci-pyside before broker or calendar issues is in `## Left out`; the `sys.path` warning is in `## Interface`.
2. Biggest unnamed risk: adopted. `uv.lock` only by `uv lock`, never by hand, stop without network; the e-mail and local-path grep is an acceptance item; the pull request description pastes the command output (a note outside acceptance).
3. Simpler way: not adopted. One size-S issue is kept so the interface lands with the files it needs; a project without a build backend would break the later console scripts.
4. Lock-in: adopted. The name `optdesk` is final; the drift between the mirror and the generated types is recorded in `## Left out`; the `ping` signature is an interface change when a deadline is added, accepted.
5. Claim most likely false: adopted. `git status --porcelain` empty is an acceptance item and `.gitignore` is left to repo (inner `.gitignore` files of the tools are relied on, said in the issue); one body rule for declarations; the declaration change is a precondition of the broker design.
6. Likeliest slip: adopted. The Goal forbids `uv init`; the seven files are named exactly in acceptance.
