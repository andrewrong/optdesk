# Account data: positions, balances and executions of the owner's IBKR accounts, stored and readable (stage 1); seller features named, not cut

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Source: owner decisions of 2026-11-08, binding: (1) account data, all of it, before cut-over (a); (2) the margin cushion and the assignment cash are shown only on a page or as a local notice, never on Telegram (5 a; ADR 0010 stands for account data); (3) tracking is stage 1 (7 a). The advisor named the seller needs that are missing: an early-assignment and ex-dividend guard, cost basis and wheel, assignment cash by date per account, the margin cushion. Plan: `docs/design/alignment-v25.md` on `origin/design/alignment-v25` (rows `account job`, `Flex job`, `account collection with partial status`, `ot import`; "stage 2" of `docs/design/risk-alerts.md` Decision 12).
Read on main at eb7321d: `AGENTS.md`, `docs/design/modules.md`, `broker.md`, `pyside-account-b.md`, `pyside-ibkr-transport.md`, `risk-alerts.md`, `store-week2.md`, `ot-collect.md`, `contracts/proto/ibkr/v1/account.proto`, `internal/broker/grpcbroker/account.go`, `pyside/src/optdesk/ibkr/accounts.py`, `migrations/0015_alerts.sql`, `internal/store/api.go`, `internal/jobs/quotesnapshot/`. option-trader (read-only): `src/option_trader/sources/ibkr/account_reader.py` (reqPositions, accountSummary, portfolio prices, orders) and `src/option_trader/private_book/executions.py` (executions and cash events come from a stored Flex report, not from the API). No credential, `.env` or service data was read.

## What main holds today

- The account service is landed: `AccountService.Account` returns the summary rows (`TAGS` of `accounts.py`: NetLiquidation, TotalCashValue, AvailableFunds, ExcessLiquidity, BuyingPower, GrossPositionValue, EquityWithLoanValue, InitMarginReq, MaintMarginReq, Cushion) and the open positions of one gateway, by alias, decimal text, read-only. `grpcbroker.AccountClient` is its Go client. Nothing stores or reads it: no dataset, table, role or job (alignment rows `account job` and `account collection with partial status`).
- The pyside transport allows ids `{1, 2, 9, 20, 25, 49, 59, 71, 78, 61, 62, 63, 64}` after `pyside-account-b.md`; the executions request is refused (that design's Left out). `contracts/proto/ibkr/v1` and `transport.py` are owner-only paths.
- `docs/contracts/broker.md` keeps the account client out of `broker.Broker`; the account is a second surface.
- Gateway B collects `account_summary` and `positions`; gateway A collects `market_data` only (`pyside/examples/gateways.example.toml`). Which gateway holds which owner account is a deploy file of the owner, not in this repository.
- option-trader has the history (Flex reports with executions, cash events, assignments) in its own store. This repository has none.

## Decisions

1. Stage 1 is three datasets, each stored by a job and readable by a Reader: `account.position` (open positions per capture), `account.value` (the summary rows per capture, balances and margin tags) and `account.execution` (one row per execution). Positions and values come from `AccountService.Account` as it is. Stage 1 builds no figure, page or notice.
2. Executions from the API, not from Flex. `AccountService` gains one read rpc `Executions` (the request carries nothing; the answer lists the executions the gateway still holds, with the commission when its report has arrived) and pyside allows the one request id of the executions (`reqExecutions`, 7) and the commission report callback, and nothing else (order ids stay refused first). The API holds a few days of executions, so a job that misses more than that loses rows: the gap is real and is closed by `ot import` of option-trader's stored history at cut-over (the cut-over design, `docs/design/alignment-v25.md` Decision 3.8), and that import is not in this design. Flex (an HTTPS call to IBKR with the owner's token) is the other way and is deferred (Left out): it is a new outside account access.
3. Row identity. A position or value row belongs to a capture: `(capture_id, alias, ...)`; the job writes one capture per run, in one transaction, and a partial answer (a gateway that collects only one of the two lists, or does not answer) writes what it has with `status` of the capture saying which list is missing, never a silent empty list: an empty `positions` of a gateway that collects them means no position and is written as a capture row with zero positions. An execution row is keyed by `(alias, exec_id)`, insert-only; a later commission arrives as `ON CONFLICT (alias, exec_id) DO UPDATE SET commission, commission_currency WHERE commission IS NULL` and nothing else changes.
4. Account ids never appear: every row carries the alias of the gateway's entry (the broker rule). Money and quantities are `numeric` in the database and canonical decimal text in Go (as the proto has them); nothing is a float.
5. A role of its own: `ot_w_account` (INSERT and SELECT on schema `account` tables; UPDATE only on the two commission columns of `account.execution`; no DELETE, no TRUNCATE), DSN line `OT_PG_DSN_ACCOUNT`, `store.DSNEnvAccount`. The reader role is the existing read-only role. Rows are permanent and append-only like `obs.*`. Account data and market data never share a role.
6. Delivery rule, binding: nothing in this design reaches notify. The job writes no `job_event` text that carries a holding, balance or size (its failure text is a job id and a status, as ADR 0010 says); the margin cushion and the assignment cash, when built, go to a page block or a local notice (a macOS notification on the owner's machine, a later design), never `Sender`. `internal/notify` does not import the account reader (its depends cell does not name it; `scripts/arch` enforces it).
7. Capture schedule: one run after the US close (anchor `session_close`, offset after, as the scheduler's manifests do) so that the executions of the day are held and the end-of-day positions are taken once; a second run before the open is a deferral. The job reads each configured account gateway socket in turn, one at a time; the sockets come from the owner's deploy file.
8. The seller features and what they need (not cut):
   - Margin cushion: stage 1 already stores `Cushion`, `ExcessLiquidity`, `MaintMarginReq` per capture; the figure and its page block are a later compute and pages issue, with the local notice.
   - Assignment cash by date per account: an assignment shows in the API as a position change, not as an execution of the option; detecting it from consecutive captures (an option position gone on its expiry day while a stock position appears or changes) is a pure compute figure over `account.position`, and the cash is then `strike x multiplier x quantity` by date and alias. This is reliable only with a capture every session day; Flex marks assignments explicitly and is the check.
   - Early-assignment and ex-dividend guard: needs the positions (stage 1) plus the ex-dividend dates, which no dataset holds; the source of dividends is the open question of that later design.
   - Cost basis and wheel: needs full history of executions and assignments, so it waits for `ot import` and Flex.
9. The first reader is the store's own: `store.AccountReader` with `Positions(ctx, alias, day)`, `Values(ctx, alias, day)`, `Executions(ctx, alias, from, to)`, so that stage 2 builders (household, positions, ledger, tracking) have one place to read. Stage 1 adds no page.

## Second opinion

Two items are second-opinion cases for the advisor (it answers later; no Advisor line is written here):
- A new outside account access in the sense of data volume: the job reads the owner's account (positions, balances, executions) from IBKR for the first time and writes it to PostgreSQL, a new write of account data outside any existing role. Affects `account-interface` (the rpc and the datasets), `account-pyside-executions` (a new allowlisted request id), `account-store-migration` (the role `ot_w_account`) and `account-job`. Questions: (a) the allowlist grows by one read id and the order ids stay refused first; (b) the role cannot delete and updates only the commission; (c) account data never reaches `Sender`.
- Flex, if the owner wants it, is a new outside account access (a token and an HTTPS call to IBKR). It is not in this design; each of its issues will carry the case.

## Modules

New directories: `internal/store/account` (the Writer and Reader), `internal/jobs/accountcapture` (the job). Other edits are to existing packages. This pull request adds nothing to `docs/design/modules.md`; the issue `account-job` adds `broker` to nothing (jobs already names it) and `account-wire` edits only `cmd/**`. New Go imports are none of third parties (pgx and gRPC are in `go.mod`).

| # | issue | module | size | order |
|---|---|---|---|---|
| 1 | account-interface (interface) | contracts, broker, store and repo (licensed, below) | M | first |
| 2 | account-pyside-executions | pyside | M | after 1 |
| 3 | account-broker-grpc | broker | S | after 1 |
| 4 | account-store-migration | store | M | after 1 |
| 5 | account-store-impl | store | M | after 4 |
| 6 | account-job | jobs | M | after 1, 3 and 5 |
| 7 | account-wire | ot | S | after 6 |

Licence: the interface entry lies in four modules because one change of the account surface is one contract: the proto and the datasets are contracts (`contracts/proto/**`, `contracts/datasets/**` and the generated Go, Python and TypeScript, which are never edited by hand and are committed with their source), the call is broker (`internal/broker/api.go`), the store types are store (`internal/store/api.go`), and `.go-arch-lint.yml` for the new sub-package `internal/store/account` is repo (`scripts/arch`, the precedent is `docs/design/broker.md:38` and `docs/design/risk-alerts.md` `alerts-store-interface`). The pairs licensed are contracts with broker (a rpc and the Go declaration that mirrors it), store with repo, and contracts with store (a dataset and the store type that carries it), and no other. Each later issue lies in one module.

## Interface

| name | owner and file | declaration |
|---|---|---|
| `AccountService.Executions`, messages `ExecutionsRequest{}`, `ExecutionsResponse{account, generation, executions}`, `Execution{exec_id, perm_id, time, contract, side, quantity, price, commission, commission_currency}` | contracts, `contracts/proto/ibkr/v1/account.proto` (owner-only, merged by the owner) | one read rpc; `time` is RFC 3339 UTC text, numbers are decimal text, `commission` is empty until its report arrives, no order id, no account id |
| datasets `account.position` r1, `account.value` r1, `account.execution` r1 | contracts, `contracts/datasets/account.*.r1.json` | position: `capture_id, alias, captured_at, session_day, symbol, sec_type, exchange, currency, expiry, strike, multiplier, right, con_id, quantity, avg_cost`; value: `capture_id, alias, captured_at, session_day, tag, value, currency`; execution: `alias, exec_id, perm_id, executed_at, session_day, symbol, sec_type, expiry, strike, right, multiplier, currency, side, quantity, price, commission, commission_currency` |
| `broker.AccountReader` (`Account(ctx) (Account, error)`, `Executions(ctx) (Executions, error)`, `Close() error`) with the types `Account`, `AccountValue`, `Position`, `Execution`, `Executions` | broker, `internal/broker/api.go` | declarations only; additions only (the existing `Broker` is unchanged); `grpcbroker.AccountClient` is the implementation |
| `store.AccountWriter` (`WriteCapture(ctx, Capture) error`, `WriteExecutions(ctx, alias string, []Execution) error`), `store.AccountReader` (Decision 9), `store.AccountStore` (both plus `Close() error`), `store.DSNEnvAccount = "OT_PG_DSN_ACCOUNT"`, `store.OpenAccountFunc func(ctx, dsn) (AccountStore, error)` | store, `internal/store/api.go` | declarations only, additions only; implementation `internal/store/account.Open`; `Capture{ID, Alias, At, SessionDay, Status, Values, Positions}` |

## Issues

### account-interface (interface)

- Module: contracts, broker, store and repo (judged four-module entry, Licence above). Size: M.
- Order: first.
- Design: docs/design/account-data.md#Interface (second-opinion case: the new rpc and the new datasets)
- Files: contracts/proto/ibkr/v1/account.proto, contracts/datasets/account.position.r1.json, contracts/datasets/account.value.r1.json, contracts/datasets/account.execution.r1.json, docs/contracts/datasets/account.position.md, docs/contracts/datasets/account.value.md, docs/contracts/datasets/account.execution.md, internal/contracts/genpb/**, internal/contracts/gen/**, pyside/src/optdesk/contracts/genpb/**, internal/broker/api.go, internal/broker/api_test.go, docs/contracts/broker.md, internal/store/api.go, internal/store/api_test.go, internal/store/account/doc.go, docs/contracts/store.md, .go-arch-lint.yml

- Goal: the declarations of the Interface table, declarations only; the generated code committed with its source; a contract test from both sides for `broker.AccountReader` (a fake and a caller) and for `store.AccountStore` (a fake and a caller); the contract documents say the alias rule, the append-only rule and the delivery rule of Decision 6.

Acceptance:

1. `grep -c "rpc Executions" contracts/proto/ibkr/v1/account.proto` prints 1; `grep -ci "order_id\|account_id" contracts/proto/ibkr/v1/account.proto` prints 0.
2. `ls contracts/datasets/account.*.r1.json | wc -l | tr -d ' '` prints 3; `scripts/` generation check: `bash contracts/generate.sh && git diff --exit-code -- internal/contracts pyside/src/optdesk/contracts` prints nothing and exits 0.
3. `grep -c "type AccountReader interface" internal/broker/api.go` prints 1; `git diff $(git merge-base HEAD origin/main) -- internal/broker/api.go internal/store/api.go | grep -c '^-[^-]'` prints 0 (additions only); `go test ./internal/broker ./internal/store -run 'TestAccount' -race -count=1` passes.
4. `grep -c "DSNEnvAccount = \"OT_PG_DSN_ACCOUNT\"" internal/store/api.go` prints 1; `grep -c "store/account" .go-arch-lint.yml` is at least 1; `go run ./scripts/arch --check` prints `arch: ok`; `grep -c "internal/notify" .go-arch-lint.yml` is unchanged and no `notify` line names `store/account` (`grep -n "store/account" .go-arch-lint.yml | grep -c notify` prints 0).
5. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vE '^(contracts/|docs/contracts/|internal/contracts/|pyside/src/optdesk/contracts/|internal/broker/api|internal/store/api|internal/store/account/doc.go|\.go-arch-lint\.yml)' | wc -l | tr -d ' '` prints 0.

Checked by CI: ci-go (`go test ./internal/broker ./internal/store`); the generated-code check; the `arch.toml` check; the design verdict. The proto path is owner-only: the owner merges it.

### account-pyside-executions

- Module: pyside. Size: M. Depends on: account-interface.
- Order: after 1.
- Design: docs/design/account-data.md#Decisions (Decision 2; second-opinion case)
- Files: pyside/src/optdesk/ibkr/accounts.py, pyside/src/optdesk/ibkr/server.py, pyside/tests/ibkr/test_executions.py, pyside/tests/ibkr/test_guard.py, pyside/tests/contracts/test_ibkr_api.py. `pyside/src/optdesk/ibkr/transport.py` is owner-only and needs the one line that adds id 7: reported in the pull request description, not edited.

- Goal: the handler of `Executions` over a fake gateway: it sends the one executions request, waits for the end marker and the commission reports within the deadline of `accounts.py`, answers by alias with decimal text, an empty list when there is none, and refuses nothing else; no test connects to a live broker.

Acceptance:

1. `cd pyside && uv run pytest tests/ibkr/test_executions.py tests/ibkr/test_guard.py tests/contracts/test_ibkr_api.py -n auto -q` passes.
2. `cd pyside && uv run ruff check . && uv run mypy --strict src tests` pass.
3. `grep -c "7" pyside/tests/ibkr/test_guard.py` is at least 1 and the guard test asserts ids 3, 4, 21 and 58 are still refused first (`cd pyside && uv run pytest tests/ibkr/test_guard.py -k order -q` passes).
4. `git diff --name-only $(git merge-base HEAD origin/main) -- pyside/src/optdesk/ibkr/transport.py | wc -l | tr -d ' '` prints 0.

Checked by CI: the pyside workflow (`uv sync --locked`, ruff, mypy, pytest); the design verdict; the owner's merge of `transport.py`.

### account-broker-grpc

- Module: broker. Size: S. Depends on: account-interface.
- Order: after 1.
- Design: docs/design/account-data.md#Interface
- Files: internal/broker/grpcbroker/account.go, internal/broker/grpcbroker/account_test.go, internal/broker/grpcbroker/convert.go, internal/broker/grpcbroker/convert_test.go

- Goal: `AccountClient` satisfies `broker.AccountReader`: `Account` as today (its types become the broker types), `Executions` with the same rules (no retry, an empty alias or a negative generation is `Permanent`, time parsed as UTC, decimals kept as text); tested against an in-process fake gRPC server on a unix socket in `t.TempDir()`.

Acceptance:

1. `grep -c "var _ broker.AccountReader" internal/broker/grpcbroker/account_test.go` prints 1; `go test ./internal/broker/... -race -count=1` passes.
2. `grep -rn "t.Parallel()" internal/broker/grpcbroker/account_test.go | wc -l | tr -d ' '` is at least 3; `grep -c "time.Sleep" internal/broker/grpcbroker/account_test.go` prints 0.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vc '^internal/broker/grpcbroker/'` prints 0.

Checked by CI: ci-go (`go test ./internal/broker/...`); the design verdict.

### account-store-migration

- Module: store. Size: M. Depends on: account-interface.
- Order: after 1.
- Design: docs/design/account-data.md#Decisions (Decisions 3 to 5; second-opinion case)
- Files: migrations/0016_account.sql, internal/store/setup/setup.go, internal/store/setup/setup_test.go, internal/store/setup/account_pg_test.go

- Goal: schema `account` with `account.capture(capture_id uuid PK, alias text, captured_at timestamptz, session_day date, status text)`, `account.position` and `account.value` keyed by `(capture_id, ...)` with a foreign key to the capture, and `account.execution` keyed by `(alias, exec_id)`, all columns of the datasets of the Interface table (`numeric` for money and quantity); the role `ot_w_account` and its grants of Decision 5, made by `setup`, DSN line `OT_PG_DSN_ACCOUNT`.

Acceptance:

1. `M=migrations/0016_account.sql`; `test -f $M` succeeds; `grep -cE "CREATE TABLE( IF NOT EXISTS)? account\.(capture|position|value|execution)" $M` prints 4; `grep -v '^--' $M | grep -ci "DELETE\|TRUNCATE"` prints 0; `grep -v '^--' $M | grep -ci "GRANT UPDATE"` prints 1 and that line names only the two commission columns.
2. `grep -c "ot_w_account" internal/store/setup/setup.go` is at least 2; `go test ./internal/store/setup -run 'TestRoles|TestAccountRole' -race -count=1` passes.
3. `grep -c "//go:build pg" internal/store/setup/account_pg_test.go` prints 1.
4. `grep -ci "float\|double" migrations/0016_account.sql` prints 0.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists only the four Files paths.

Checked by CI: ci-go; the `pg` job in GitHub CI (`TestAccountMigration`: it applies on a clean database, the role holds the grants above and no more); the design verdict.

### account-store-impl

- Module: store. Size: M. Depends on: account-store-migration.
- Order: after 4.
- Design: docs/design/account-data.md#Decisions (Decisions 3 and 9)
- Files: internal/store/account/store.go, internal/store/account/read.go, internal/store/account/store_test.go, internal/store/account/store_pg_test.go

- Goal: `account.Open` returns the `store.AccountStore` over pgx with the role `ot_w_account`: `WriteCapture` is one transaction (the capture row and its rows, an idempotent retry of the same `capture_id` writes nothing twice), `WriteExecutions` is the insert-only upsert of Decision 3, the Reader returns rows sorted by key. The tests of the non-`pg` file run against a fake `database` as in `internal/store/snapshot`.

Acceptance:

1. `go test ./internal/store/account -race -count=1` passes; `grep -c "t.Parallel()" internal/store/account/store_test.go` is at least 4.
2. `grep -c "//go:build pg" internal/store/account/store_pg_test.go` prints 1; `grep -rc "pgx" internal/store/account/store.go` is at least 1 and `grep -rln "pgx" internal --include=*.go | grep -vc '^internal/store/'` prints 0.
3. `grep -ci "delete from\|truncate" internal/store/account/store.go` prints 0.
4. `wc -l internal/store/account/*.go | awk '$1>600' | wc -l | tr -d ' '` prints 0.

Checked by CI: ci-go; the `pg` job (`TestAccountStore`: write, rewrite the same capture, a commission arriving after its execution, read back); the design verdict.

### account-job

- Module: jobs. Size: M. Depends on: account-interface, account-broker-grpc and account-store-impl.
- Order: after 1, 3 and 5.
- Design: docs/design/account-data.md#Decisions (Decisions 3, 6 and 7; second-opinion case: the new account access)
- Files: internal/jobs/accountcapture/job.go, internal/jobs/accountcapture/job.toml, internal/jobs/accountcapture/schedule.go, internal/jobs/accountcapture/job_test.go, internal/jobs/accountcapture/schedule_test.go

- Goal: the job `account_capture` through the interfaces only (`broker.AccountReader` per configured alias, `store.AccountWriter`, `clock`): for each alias one `Account` and one `Executions` call, one capture and one execution write, `status` naming a missing list, a failed alias does not stop the others and the result of the job says which alias failed by alias only; the manifest `job.toml` carries the schedule of Decision 7 and no account id, socket path or value; no text of the result carries a holding, balance or size.

Acceptance:

1. `go test ./internal/jobs/accountcapture -race -count=1` passes with fakes of both interfaces and the fake clock of the test kit; `grep -c "time.Sleep" internal/jobs/accountcapture/*.go` prints 0.
2. `grep -c "^\[schedule\]" internal/jobs/accountcapture/job.toml` prints 1; `grep -ciE "DU[0-9]|U[0-9]{6,}|socket|token" internal/jobs/accountcapture/job.toml` prints 0.
3. `grep -rn "internal/notify" internal/jobs/accountcapture | wc -l | tr -d ' '` prints 0; `go run ./scripts/arch --check` prints `arch: ok`.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vc '^internal/jobs/accountcapture/'` prints 0.

Checked by CI: ci-go; the `arch.toml` check; the design verdict.

### account-wire

- Module: ot. Size: S. Depends on: account-job.
- Order: after 6.
- Design: docs/design/account-data.md#Decisions (Decision 7)
- Files: cmd/ot/account.go, cmd/ot/account_test.go, cmd/ot/jobs.go, docs/contracts/ot.md

- Goal: the registry entry `account_capture` (constructor with `grpcbroker.DialAccount` per alias of the owner's file `OT_ACCOUNT_SOCKETS` read by name, and `account.Open` with `OT_PG_DSN_ACCOUNT`), so that `ot run-once account_capture` and the scheduler find the job; a dry run with fakes in the test; the contract document names the two environment keys and no value.

Acceptance:

1. `go test ./cmd/ot -run 'TestAccount' -race -count=1` passes.
2. `grep -c "OT_PG_DSN_ACCOUNT\|DSNEnvAccount" cmd/ot/account.go` is at least 1; `grep -rn "internal/notify" cmd/ot/account.go | wc -l | tr -d ' '` prints 0.
3. `go run ./scripts/arch --check` prints `arch: ok`.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vcE '^(cmd/ot/|docs/contracts/ot.md$)'` prints 0.

Checked by CI: ci-go; the `arch.toml` check; the design verdict. Running the job against the real gateway is the owner's (`ot run-once`), not a test.

## Checked by CI

- `go test ./internal/broker/... ./internal/store/... ./internal/jobs/accountcapture ./cmd/ot -race -count=1` (ci-go, parallel, `t.TempDir()`, fake clock, no sleep).
- `go test -tags pg ./internal/store/...` (the `pg` job, GitHub CI only).
- `go run ./scripts/arch --check` printing `arch: ok`.
- The generated-code check of `contracts/generate.sh`.
- Pyside: `uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`.

## Left out

- Flex (executions, cash events and assignments with full history): a new outside account access with a token; deferred, and the cost basis and wheel depend on it or on `ot import`. Rejected as stage 1 because the API already carries the day's executions and positions without a second credential.
- `ot import` of option-trader's history: the cut-over design (alignment Decision 3.8); it reads the three tables of this design and `bars`.
- The seller features of Decision 8 (margin cushion figure and block, assignment cash by date per account, the early-assignment and ex-dividend guard, cost basis and wheel), the local notice, and the dividend source: each is its own later design over these tables. Named so that nothing is silently dropped.
- The portfolio prices and open orders that option-trader's `account_reader.py` also reads, and the account-update stream: not in stage 1; orders stay refused by the guard.
- A position and value history by overwrite (a "current" table): rejected, because captures are the history that assignment detection and tracking read.
- One generic key-value `account.value` of arbitrary tags: kept as tag rows (the proto's shape), with the tag list fixed by pyside.
- Pages (positions, household, ledger, tracking), compute figures and the `report_page` blocks: stage 2 designs; this design only makes the data readable.
- Which owner account sits behind which gateway, and whether gateway A also collects account data (a deploy-file change of the owner): see the open question.
- Telegram text about accounts: never (owner decision 5 a); a local notice is a later design.

## Open questions for the owner

1. Should gateway A (market data today) also collect account data, so that both accounts are captured? a: yes, change its `collect` in the deploy file; b: no, only gateway B in stage 1.
