# Rules for agents working in this repository

optdesk is written in three languages: Go (services and the command line), Python managed by uv
(`pyside/`) and a web front end in TypeScript managed by pnpm (`web/`). The module map is
`docs/design/modules.md`.

## Modules

- The packages are the modules. `docs/design/modules.md` lists each with its paths, its interface
  files, its tests and the modules it depends on. It is written before the code and judged by a design
  review on its pull request. A path belongs to the first row whose glob matches.
- A module issue edits only the paths of its module, as `docs/design/modules.md` and the `Files:` line of the
  issue give them. Needing another file is reported, not edited: name the file and the reason in the pull
  request description and leave it unchanged.

## Interfaces

- Go: an interface is one file per package, declarations only, with a test of the contract from both sides.
  Python: `src/<pkg>/<module>/api.py`, declarations only (signatures, types, docstrings, no logic), tested by
  `tests/contracts/test_<module>_api.py`. TypeScript: the generated types and the JSON schema of a page.
- Other modules use a module through its interface and nothing else. Editing an interface file is an
  interface change: the issue carries a Design line, and the interface issue lands before the issues that use it.
- Code generated from `contracts/` is never edited by hand; a change carries the source and the generated
  output together.

## Size

- A new source file stays under 600 lines. A file over 800 lines is listed with a ceiling in
  `docs/design/ratchet.json` and stays under it; a ceiling is never raised by an agent (the file is owner-only).
  When a change does not fit, report it in the pull request description; do not split the file yourself.

## Tests

- Tests run in parallel from the first test: no shared state between tests, a temporary directory for every
  file written, no port or file at a fixed place (Go: `t.Parallel()` and `t.TempDir()`; Python: `pytest -n auto`).
- A test never sleeps. Time comes from a fake clock in the test kit.
- No test connects to a live broker or places an order. A Go test that needs PostgreSQL carries the build tag
  `pg` and runs only in GitHub CI.
- Python: `uv sync --locked`, then `ruff check`, `mypy --strict` and `pytest`. The lockfile `uv.lock` is committed.
- Web: the front end renders data and computes nothing. pnpm is the one package manager, pinned in the
  `packageManager` field of the root `package.json`; install with `pnpm install --frozen-lockfile`; the
  lockfile `pnpm-lock.yaml` is committed. A change under `web/` passes `tsc`, `eslint`, `vitest` and the build in CI.

## Libraries

- Prefer a mainstream, maintained library over code of our own: one that is widely used, has a stable API,
  a permissive license and few transitive dependencies. Write our own only when no such library fits, and say
  so in the `Design:` line of the pull request ("own implementation, because ..."). A new direct dependency is
  named in the pull request description with its license; the reviewer checks the four criteria, and the
  owner's word is needed only for a library that touches secrets, the network or the sandbox.

## Pull requests

- The `test` check runs on every pull request; the workflows for each language run when their paths change.
  A scheduled workflow runs everything.
- Commit with the identity the history uses. No secrets, API keys, account ids, tokens, broker credentials,
  chat ids or personal data in code, commits, configuration or fixtures.
- The pull request body carries a line starting `Contracts: ` from the moment the pull request is opened
  (`Contracts: unchanged` when no file under `contracts/schemas`, `contracts/datasets` or `contracts/proto`
  changes). The `test` check reads the body of the event that started the run, so a body edited later is not
  seen until a new commit is pushed.
