# Account data: positions, balances and executions of the owner's IBKR accounts, stored and readable (stage 1); seller features named, not cut

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 13 (the account migration is named NNNN_account.sql, the next free number at cut time, with glob greps; main took 0017 for fundamental_fact; migrations apply out of order, advisor 2026-11-09); revision 12 (the Contracts form of account-proto is `ibkr.v1.account v1 additive`, accepted by the checks since pull request 661); revision 11 (account-wire calls the Config of account-job); revision 10 (revision 10 answers the design review of bda56bc: one meaning of alias, the constructor in account-job, the source of `OT_IBKR_GATEWAYS`; revision 9 revision 9 answers the design review of ffc7983: main's label is the `account` field, the constructor, the refusal tests; revision 8 revision 8 answers the design review of ca0996c: one source of main, no new event store, the socket list form; revision 7 revision 7 designates main and lets a non-main alias fail without failing the run, owner decision 2026-11-09; revision 6 writes the Order cells as entry numbers so that the shared-file pair check reads them; advisor amendments: rd views for pages, transport.py back in account-pyside-executions, the Contracts form; revision 4 was the Order cells name issues, the table follows the section order, Files equal the repository; revision 3 was the answer to the design review of 90176f9; revision 2 was after the adversarial pass, `account-adversarial.md` in the scratchpad of the review)
Source: owner decisions of 2026-11-08, binding: (1) account data, all of it, before cut-over (a); (2) the margin cushion and the assignment cash are shown only on a page or as a local notice, never on Telegram (5 a; ADR 0010 stands for account data); (3) tracking is stage 1 (7 a). The advisor named the seller needs that are missing: an early-assignment and ex-dividend guard, cost basis and wheel, assignment cash by date per account, the margin cushion. Plan: `docs/design/alignment-v25.md` on `origin/design/alignment-v25` (rows `account job`, `Flex job`, `account collection with partial status`, `ot import`; "stage 2" of `docs/design/risk-alerts.md` Decision 12).
Read on main at eb7321d: `AGENTS.md`, `docs/design/modules.md`, `broker.md`, `pyside-account-b.md`, `pyside-ibkr-transport.md`, `risk-alerts.md`, `store-week2.md`, `ot-collect.md`, `contracts/proto/ibkr/v1/account.proto`, `internal/broker/grpcbroker/account.go`, `pyside/src/optdesk/ibkr/accounts.py`, `migrations/0015_alerts.sql`, `internal/store/api.go`, `internal/jobs/quotesnapshot/`. option-trader (read-only): `src/option_trader/sources/ibkr/account_reader.py` (reqPositions, accountSummary, portfolio prices, orders) and `src/option_trader/private_book/executions.py` (executions and cash events come from a stored Flex report, not from the API). No credential, `.env` or service data was read.

## What main holds today

- The account service is landed: `AccountService.Account` returns the summary rows (`TAGS` of `accounts.py`: NetLiquidation, TotalCashValue, AvailableFunds, ExcessLiquidity, BuyingPower, GrossPositionValue, EquityWithLoanValue, InitMarginReq, MaintMarginReq, Cushion) and the open positions of one gateway, by alias, decimal text, read-only. `grpcbroker.AccountClient` is its Go client. Nothing stores or reads it: no dataset, table, role or job (alignment rows `account job` and `account collection with partial status`).
- The pyside transport allows ids `{1, 2, 9, 20, 25, 49, 59, 71, 78, 61, 62, 63, 64}` after `pyside-account-b.md`; the executions request is refused (that design's Left out). `pyside/tests/ibkr/test_guard.py` holds its own tuple of the allowed ids and asserts the guard against it, so the two files change together. `contracts/proto/ibkr/v1` and `transport.py` are owner-only paths.
- `docs/contracts/broker.md` keeps the account client out of `broker.Broker`; the account is a second surface.
- In the example config, gateway B collects `account_summary` and `positions` and gateway A collects `market_data` only (`pyside/examples/gateways.example.toml`); stage 1 needs both to collect the account words, which is the owner's deploy file (Decision 8). Which gateway holds which owner account is that deploy file of the owner, not in this repository.
- option-trader has the history (Flex reports with executions, cash events, assignments) in its own store. This repository has none.

## Decisions

1. Stage 1 is three datasets, each stored by a job and readable by a Reader: `account.position` (open positions per capture), `account.value` (summary rows per capture: balances and margin tags) and `account.execution` (one row per execution). Positions and values come from `AccountService.Account`. Stage 1 builds no figure, page or notice, and "readable" means the Go `store.AccountReader` that stage 2 builders use; no `ot` subcommand or page reads it yet (Left out). Position prices, unrealized P&L and orders are not collected (Left out).
2. Collected is a field, not a guess. `AccountResponse` gains `bool values_collected = 5` and `bool positions_collected = 6`, and the new `ExecutionsResponse` has `collected`, so "the gateway does not collect it" differs from "none open". The job writes nothing for a kind that is not collected. An empty `positions` with `positions_collected` true is a capture with zero positions, written.
3. Executions from the API, written only complete. `AccountService` gains one read rpc `Executions` and the gateway word `executions` in `collect`. The handler sends the one executions request (`reqExecutions`, id 7), waits for the end marker and for the commission reports until the deadline of `accounts.py`, and answers only executions whose commission report is in; the count of those still waiting is `pending`. The job writes the complete rows and, when `pending` is above zero for the main alias (Decision 8), ends the run as failed with the status "pending", so a retry the same day (insert-only, `ON CONFLICT DO NOTHING`) picks the rest up. For a non-main alias `pending` above zero is a degraded state (Decision 8), the complete rows are written and the run stays `ok`. Rows are never updated, so `ot_w_account` needs INSERT and SELECT only.
4. The API gives the current trading day only (assumed; the repository holds no evidence for more: option-trader reads its executions from Flex, not the API). A missed run therefore loses that day's executions. Mitigations: the schedule is a daily anchor on `session_close` with `run_bound` and `restart_sensitive` like the other jobs, so a missed run is a gap the scheduler reports (`scheduler.md` Decisions 12 and 13), and the real fix is Flex or `ot import`, both deferred (Left out).
5. Identity. `account.capture(capture_id text PK, alias, captured_at, session_day, ...)`; `capture_id` is `<alias>:<session_day>:<run key>` of the scheduler run, so a retry of the same run writes the same id and the capture insert is `ON CONFLICT DO NOTHING` (then no child rows are written). An execution row is keyed by `(alias, exec_id)`. Every row carries the alias of the gateway's entry, never an account id; money and quantities are `numeric`, decimal text in Go, never floats. Rows are permanent and append-only.
6. A role of its own: `ot_w_account`, INSERT and SELECT on the tables of schema `account`, no UPDATE, DELETE or TRUNCATE; DSN line `OT_PG_DSN_ACCOUNT`, `store.DSNEnvAccount`. Pages never read a base table: the migration makes one pass-through view per account table in schema `rd` (`rd.account_capture_r1`, `rd.account_position_r1`, `rd.account_value_r1`, `rd.account_execution_r1`, the pattern of `rd.series_point_r1` in `migrations/0002_series.sql`), and `ot_ro_pages` gets SELECT on the views only (and USAGE on `rd`, which it has), no grant on schema `account` or its tables; `ot_w_account` has INSERT and SELECT on the base tables, and, for its own Reader (Decision 10), USAGE on schema `rd` and SELECT on the four views, the pattern of `ot_w_readings` (SELECT on `rd.reading_r1`, `docs/contracts/store.md`). The model of the grants is `migrations/0015_alerts.sql` (role `ot_w_alerts`, INSERT and SELECT, no grant on a page view beyond its own views). Account and market data never share a role.
7. Delivery rule, binding: nothing in this design reaches notify. A job result or `job_event` carries a job id, a status and an alias only (an alias is the `account` field of the gateway's entry, Decision 8, a label and not an account figure); no holding, balance or size. The margin cushion and the assignment cash, when built, go to a page block or a local notice, never `Sender`. `internal/notify` does not import `internal/store/account` or `internal/jobs/accountcapture` (`scripts/arch` enforces it).
8. Stage 1 captures both gateways, but only main must work (owner decision 2026-11-09: "only my main IBKR account must work; the second account may be unavailable, its gateway may be logged out; do not require all accounts; one account failing is fine as long as the main one works"). Gateway A and gateway B are logged into two different owner accounts. The job iterates the list of sockets the owner's deploy file gives, which names both; each socket carries the deploy file's label (the alias of Decision 5, the `account` field of its gateway entry, such as `a` and `b`), and the `collect` words of both gateways in that file include `account_summary`, `positions` and `executions`: a deploy change, not a code change. The label is the only identity of an account in the repository: no account id, account name or person appears in code, fixtures, logs or this design, and a test fixture uses the labels `a` and `b` with invented figures. The rows of the two gateways are told apart by `alias` (Decision 5: in `account.capture`, in the id of the capture whose children carry it, and in the key of `account.execution`); there is no second column to add. Main, the single designation, one source: the one `[[gateway]]` entry with `required = true` in the gateways file named by `OT_IBKR_GATEWAYS` (`pyside/src/optdesk/ibkr/gateways.py`, the field that already decides gateway health; gateway b stays `required = false`); the label of main is the `account` field of that entry (the alias of Decision 5, the same string as `AccountResponse.account` and as the label in `OT_ACCOUNT_SOCKETS`; the entry's `name` is a gateway name and is not used by the job). `OT_ACCOUNT_SOCKETS` carries no flag of its own: its form is a comma list of `label=socket path`, such as `a=<path>,b=<path>`, and the wiring reads the label of main (the `account` field of the required entry) from the gateways file and refuses to start the job when no entry, or more than one entry, is required, or when the label of main is not in the list (an error naming the key only; `gateways.py` is not edited here, another design owns that file). Run status: a failed, logged-out or not-configured non-main alias (including `pending` above zero for it) ends the run `ok`, writes what it can and emits one `degraded` log record per run with the alias and a status only; it is never `failed`, never a gap row (`ops.job_gap`) and never a notification (Decision 7). No `job_event` row, field or contract exists for it on main (`docs/contracts/runner.md` has none), and this design adds no state: `degraded` is a log record of the job, one per run per such alias, not deduplicated between runs (a fresh `run-once` process has no memory). Only main failing (or main `pending` above zero) ends the run `failed`.
9. The seller features and what they need (not cut):
   - Margin cushion: `Cushion`, `ExcessLiquidity` and `MaintMarginReq` are stored per capture; the figure, its page block and the local notice are later issues.
   - Assignment cash by date per account: an assignment shows in the API as a position change, not as an execution; a pure compute figure over consecutive `account.position` captures (an option position gone on its expiry day while a stock position appears) gives `strike x multiplier x quantity` by date and alias. It is only as reliable as the captures (Decision 4); Flex marks assignments explicitly and is the check.
   - Early-assignment and ex-dividend guard: needs the stage 1 positions and ex-dividend dates, which no dataset holds; the dividend source is the open item of that later design.
   - Cost basis and wheel: needs the full execution and assignment history, so it waits for `ot import` and Flex.
10. `store.AccountReader`: `Positions(ctx, alias, day)`, `Values(ctx, alias, day)`, `Executions(ctx, alias, from, to)`, rows sorted by key; it reads the `rd.account_*_r1` views, never the base tables, and the role that opens it (`ot_w_account`) holds SELECT on those views (Decision 6).

## Second opinion

Second-opinion cases for the advisor (it answers later; no Advisor line is written here), named in the issues that carry them:
- A new outside account access: the job reads the owner's positions, balances and executions from IBKR for the first time and writes them to PostgreSQL under a new role. Affects `account-proto`, `account-guard-id`, `account-pyside-executions`, `account-store-migration` and `account-job`. Questions: (a) the allowlist grows by one read id and the order ids stay refused first; (b) the role cannot update or delete; (c) account data never reaches `Sender`.
- `account-guard-id` is in addition an owner-only path (`transport.py`): the owner merges it.
- Flex, if the owner wants it, is a new outside account access (token and HTTPS call to IBKR); it is not in this design and each of its issues will carry the case.

## Modules

New directories: `internal/store/account` and `internal/jobs/accountcapture`. This pull request edits `docs/design/modules.md` by one list: the exact import list of `cmd/ot` gains `internal/store/account` and `internal/jobs/accountcapture` (the precedent is `internal/store/alert`) and `arch.toml` is regenerated with it by `go run ./scripts/arch` so that `--check` passes; no depends cell changes (jobs already names broker and store, ot names both). The interface entry lies in contracts only (the proto, the datasets and the output generated from them, committed together); the other interface files are edited by the issue of their own module, each with a Design line. This design has one entry marked (interface), account-proto (contracts, registrable on its own); the broker declarations (account-broker-api) and the store declarations (account-store-api) are ordinary entries after it, each declarations only (the pattern of `docs/design/home-page-revise.md`). The two judged two-module entries are the ones modules.md licenses by precedent: a new sub-package needs `.go-arch-lint.yml`, a repo path, regenerated by `go run ./scripts/arch` (`risk-alerts.md` `alerts-store-interface`). No Go dependency is added (pgx and gRPC are in `go.mod`).

| # | issue | module | size | order |
|---|---|---|---|---|
| 1 | account-proto (interface) | contracts | M | first |
| 2 | account-guard-id | pyside (owner-only path, owner merges) | S | any time |
| 3 | account-pyside-executions | pyside (owner-only path, owner merges) | M | after account-proto and account-guard-id (same file `transport.py`) |
| 4 | account-broker-api | broker | S | after account-proto |
| 5 | account-broker-impl | broker | M | after account-broker-api |
| 6 | account-store-api | store | S | after account-proto |
| 7 | account-store-migration | store | M | after account-store-api |
| 8 | account-store-impl | store and repo (judged, `.go-arch-lint.yml`) | M | after account-store-migration |
| 9 | account-job | jobs and repo (judged, `.go-arch-lint.yml`) | M | after account-broker-api, account-store-api and account-store-impl (both regenerate `.go-arch-lint.yml`: account-job rebases on account-store-impl) |
| 10 | account-wire | ot | S | after account-broker-impl, account-store-impl and account-job |

## Interface

| name | owner and file | declaration |
|---|---|---|
| `AccountService.Executions`; `ExecutionsRequest{}`; `ExecutionsResponse{account, generation, collected, pending, executions}`; `Execution{exec_id, perm_id, time, contract, side, quantity, price, commission, commission_currency}`; `AccountResponse.values_collected = 5`, `positions_collected = 6` | contracts, `contracts/proto/ibkr/v1/account.proto` (owner-only, the owner merges) | `time` is RFC 3339 UTC text, numbers are decimal text, `commission` is always set, no order id and no account id; the change is additive (no field is removed or renumbered) |
| datasets `account.position` r1, `account.value` r1, `account.execution` r1 | contracts, `contracts/datasets/account.*.r1.json` | position: `capture_id, alias, captured_at, session_day, symbol, sec_type, exchange, currency, expiry, strike, multiplier, right, con_id, quantity, avg_cost`; value: `capture_id, alias, captured_at, session_day, tag, value, currency`; execution: `alias, exec_id, perm_id, executed_at, session_day, symbol, sec_type, expiry, strike, right, multiplier, currency, side, quantity, price, commission, commission_currency` |
| `broker.AccountReader` (`Account(ctx) (Account, error)`, `Executions(ctx) (Executions, error)`, `Close() error`); types `Account` (with `ValuesCollected`, `PositionsCollected`), `AccountValue`, `Position`, `Execution`, `Executions` (with `Collected`, `Pending`) | broker, `internal/broker/api.go` (account-broker-api) | declarations only, additions only; `Broker` unchanged; implemented by `grpcbroker.AccountClient` (account-broker-impl) |
| `store.AccountWriter` (`WriteCapture(ctx, Capture) error`, `WriteExecutions(ctx, alias string, []Execution) error` (`Execution` is `store.Execution`)), `store.AccountReader` (Decision 10, reads the `rd.account_*_r1` views), `store.AccountStore` (both and `Close() error`), `store.DSNEnvAccount = "OT_PG_DSN_ACCOUNT"`, `store.OpenAccountFunc func(ctx, dsn) (AccountStore, error)`, `Capture{ID, Alias, At, SessionDay, Status, Values []AccountValue, Positions []Position}`; row types `store.Execution`, `store.AccountValue`, `store.Position` (plain structs of the dataset columns, decimals as text, declared by store; store imports neither broker nor the generated records, the job maps broker types to them) | store, `internal/store/api.go` (account-store-api) | declarations only, additions only; implementation `internal/store/account.Open` |

## Issues

### account-proto (interface)

- Module: contracts. Size: M. Depends on: none.
- Order: first.
- Contracts: `account.position r1 (new), account.value r1 (new), account.execution r1 (new), ibkr.v1.account v1 additive` (the id `ibkr.v1.account` is the one pull request 315 used, `ibkr.v1.account (new)`; the proto change only adds an rpc, messages and fields, which is the `additive` form that `checks.yml` accepts since pull request 661; the form `ibkr.v1 account.proto (additive)` fails the regex).
- Design: docs/design/account-data.md#Interface (second-opinion case: the new rpc and datasets)
- Files: contracts/proto/ibkr/v1/account.proto, contracts/datasets/account.position.r1.json, contracts/datasets/account.value.r1.json, contracts/datasets/account.execution.r1.json, contracts/generate.sh, docs/contracts/datasets/account.position.md, docs/contracts/datasets/account.value.md, docs/contracts/datasets/account.execution.md, internal/contracts/gen/records.go, internal/contracts/genpb/account.pb.go, internal/contracts/genpb/account_grpc.pb.go, internal/contracts/genpb/account_proto_test.go, pyside/src/optdesk/contracts/genpb/ibkr/v1/account_pb2.py, pyside/src/optdesk/contracts/genpb/ibkr/v1/account_pb2.pyi, pyside/src/optdesk/contracts/genpb/ibkr/v1/account_pb2_grpc.py, pyside/src/optdesk/contracts/genpb/ibkr/v1/account_pb2_grpc.pyi, pyside/src/optdesk/contracts/api.py, pyside/tests/contracts/test_contracts_api.py

- Goal: the proto messages and rpc and the three datasets of the Interface table; `contracts/generate.sh` lists the three datasets; the generated Go and Python are committed with their source (`sh contracts/generate.sh`, the pattern of `pyside-account-b.md` `account-proto`); the Python contract test covers the new servicer, stub and `add_..._to_server`.

Acceptance:

1. `grep -c "rpc Executions" contracts/proto/ibkr/v1/account.proto` prints 1; `grep -ci "order_id\|account_id" contracts/proto/ibkr/v1/account.proto` prints 0; `grep -c "values_collected = 5\|positions_collected = 6" contracts/proto/ibkr/v1/account.proto` prints 2.
2. `git diff $(git merge-base HEAD origin/main) -- contracts/proto/ibkr/v1/account.proto | grep -c '^-[^-]'` prints 0 (additive).
3. `ls contracts/datasets/account.*.r1.json | wc -l | tr -d ' '` prints 3; `grep -c "account\.\(position\|value\|execution\)\.r1" contracts/generate.sh` is at least 3.
4. `grep -c "account\.\(position\|value\|execution\)" internal/contracts/gen/records.go` is at least 3 (the generated output is committed with its source; that it equals the regenerated one is checked by CI).
5. `cd pyside && uv run pytest tests/contracts/test_contracts_api.py -q` passes.
6. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxE 'contracts/(proto/ibkr/v1/account\.proto|datasets/account\.[a-z]+\.r1\.json|generate\.sh)|docs/contracts/datasets/account\.[a-z]+\.md|internal/contracts/(gen/records\.go|genpb/account.*)|pyside/src/optdesk/contracts/(genpb/ibkr/v1/account.*|api\.py)|pyside/tests/contracts/test_contracts_api\.py' | wc -l | tr -d ' '` prints 0.

Checked by CI: the generated-code check (`sh contracts/generate.sh` leaves no diff); pyside and ci-go workflows; the design verdict. The proto path is owner-only: the owner merges it.

### account-guard-id

- Module: pyside (owner-only path). Size: S. Depends on: none.
- Order: any time; entry 3 (account-pyside-executions) waits for it.
- Design: docs/design/account-data.md#Decisions (Decision 3; second-opinion case, owner merges)
- Files: pyside/src/optdesk/ibkr/transport.py, pyside/tests/ibkr/test_guard.py

- Goal: `ALLOWED_IDS` gains id 7 (`reqExecutions`) and nothing else; `ORDER_IDS` is unchanged and still checked first; `test_guard.py` states id 7 as allowed and ids 3, 4, 21 and 58 as refused, in the same pull request so that CI is green on it alone. The commission report is an inbound callback, not a request, and needs no id.

Acceptance:

1. `cd pyside && uv run pytest tests/ibkr/test_guard.py -n auto -q` passes.
2. `git diff $(git merge-base HEAD origin/main) -- pyside/src/optdesk/ibkr/transport.py | grep '^[-+][^-+]' | grep -c "ORDER_IDS"` prints 0, and `git diff $(git merge-base HEAD origin/main) -- pyside/src/optdesk/ibkr/transport.py | grep -c '^-[^-]'` is at most 1 (the one line of `ALLOWED_IDS` rewritten).
3. `cd pyside && uv run python -c "from optdesk.ibkr.transport import ALLOWED_IDS, ORDER_IDS; print(7 in ALLOWED_IDS, sorted(ORDER_IDS))"` prints `True [3, 4, 21, 58]`.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e pyside/src/optdesk/ibkr/transport.py -e pyside/tests/ibkr/test_guard.py | wc -l | tr -d ' '` prints 0.

Checked by CI: the pyside workflow (`uv sync --locked`, ruff, mypy --strict, pytest); the design verdict; the owner's merge.

### account-pyside-executions

- Module: pyside (owner-only path, owner merges: the Files include `transport.py`, which account-guard-id edits first, and `ibgateway.py`). Size: M. Depends on: account-proto and account-guard-id.
- Order: after 1, 2 (account-proto, account-guard-id).
- Design: docs/design/account-data.md#Decisions (Decisions 2 and 3; second-opinion case)
- Files: pyside/src/optdesk/ibkr/accounts.py, pyside/src/optdesk/ibkr/model.py, pyside/src/optdesk/ibkr/server.py, pyside/src/optdesk/ibkr/gateways.py, pyside/src/optdesk/ibkr/transport.py, pyside/src/optdesk/ibkr/ibgateway.py, pyside/tests/ibkr/test_executions.py, pyside/tests/ibkr/test_gateways.py

- Goal: the new execution row type and the collected flags live in `model.py` (where `accounts.py` imports `AccountRows`, `PositionRow` and `AccountValueRow` from; `api.py` of the package is not edited, it declares no account type); the handler of `Executions` over a fake gateway as Decision 3 says (complete rows only, `pending` counted, empty list with `collected` true when none, `collected` false and no request sent when the gateway's `collect` lacks `executions`), the `executions` word accepted by `gateways.py`, the request and the commission-report callback carried by `transport.py` and `ibgateway.py` (after the id of account-guard-id), and `values_collected` and `positions_collected` set from `collect` in `Account`; no test connects to a live broker.

Acceptance:

1. `cd pyside && uv run pytest tests/ibkr -n auto -q` passes; `grep -c "def test_" pyside/tests/ibkr/test_executions.py` is at least 5 (complete, pending, empty, not collected, order ids still refused).
2. `cd pyside && uv run ruff check . && uv run mypy --strict src tests` pass.
3. `grep -c "time.sleep\|asyncio.sleep" pyside/tests/ibkr/test_executions.py` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e pyside/src/optdesk/ibkr/accounts.py -e pyside/src/optdesk/ibkr/model.py -e pyside/src/optdesk/ibkr/server.py -e pyside/src/optdesk/ibkr/gateways.py -e pyside/src/optdesk/ibkr/transport.py -e pyside/src/optdesk/ibkr/ibgateway.py -e pyside/tests/ibkr/test_executions.py -e pyside/tests/ibkr/test_gateways.py | wc -l | tr -d ' '` prints 0.

Checked by CI: the pyside workflow; the design verdict.

### account-broker-api

- Module: broker. Size: S. Depends on: account-proto.
- Order: after 1 (account-proto).
- Design: docs/design/account-data.md#Interface (broker row; interface change of `internal/broker/api.go`)
- Files: internal/broker/api.go, internal/broker/api_test.go, docs/contracts/broker.md

- Goal: the broker declarations of the Interface table, declarations only, with a contract test from both sides (a fake `AccountReader` and a caller; `internal/broker/api_test.go` is 70 lines, so it may grow); `docs/contracts/broker.md` records the new surface and keeps the alias rule.

Acceptance:

1. `grep -c "type AccountReader interface" internal/broker/api.go` prints 1; `git diff $(git merge-base HEAD origin/main) -- internal/broker/api.go | grep -c '^-[^-]'` prints 0; `grep -c "var _ broker.AccountReader" internal/broker/api_test.go` prints 1.
2. `go test ./internal/broker -run 'TestAccount' -race -count=1` passes; `grep -c "t.Parallel()" internal/broker/api_test.go` is at least 2.
3. `grep -c "AccountReader" docs/contracts/broker.md` is at least 1.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e internal/broker/api.go -e internal/broker/api_test.go -e docs/contracts/broker.md | wc -l | tr -d ' '` prints 0.

Checked by CI: ci-go; the design verdict.

### account-broker-impl

- Module: broker. Size: M. Depends on: account-broker-api (the interface; account-proto comes with it).
- Order: after 4 (account-broker-api).
- Design: docs/design/account-data.md#Interface (broker row; implementation of the interface of account-broker-api)
- Files: internal/broker/grpcbroker/account.go, internal/broker/grpcbroker/account_test.go, internal/broker/grpcbroker/convert.go, internal/broker/grpcbroker/convert_test.go

- Goal: `AccountClient` satisfies `broker.AccountReader`: `Account` as today with the two collected flags, `Executions` with the same rules (no retry, empty alias or negative generation is `Permanent`, time parsed as UTC, decimals kept as text), tested against an in-process fake gRPC server on a unix socket in `t.TempDir()`.

Acceptance:

1. `grep -c "var _ broker.AccountReader" internal/broker/grpcbroker/account_test.go` prints 1.
2. `go test ./internal/broker/... -race -count=1` passes; `grep -c "t.Parallel()" internal/broker/grpcbroker/account_test.go` is at least 3; `grep -c "time.Sleep" internal/broker/grpcbroker/account_test.go` prints 0.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxE 'internal/broker/grpcbroker/(account|account_test|convert|convert_test)\.go' | wc -l | tr -d ' '` prints 0.

Checked by CI: ci-go; the design verdict.

### account-store-api

- Module: store. Size: S. Depends on: account-proto (for the dataset names only).
- Order: after 1 (account-proto).
- Design: docs/design/account-data.md#Interface (store row; interface change of `internal/store/api.go`)
- Files: internal/store/api.go, internal/store/account_api_test.go, docs/contracts/store.md

- Goal: the store declarations of the Interface table, including the row types `Execution`, `AccountValue` and `Position`, declarations only, with a contract test from both sides (a fake `AccountStore` and a caller) in the new file `internal/store/account_api_test.go` (`internal/store/api_test.go` has 800 lines and is not in `docs/design/ratchet.json`, so it cannot grow); `docs/contracts/store.md` records the schema `account`, the keys of Decision 5, the insert-only rule and the delivery rule of Decision 7.

Acceptance:

1. `grep -c "type AccountStore interface\|type AccountReader interface\|type AccountWriter interface\|type OpenAccountFunc " internal/store/api.go` prints 4; `grep -c "DSNEnvAccount = \"OT_PG_DSN_ACCOUNT\"" internal/store/api.go` prints 1; `git diff $(git merge-base HEAD origin/main) -- internal/store/api.go | grep -c '^-[^-]'` prints 0.
2. `grep -c "var _ store.OpenAccountFunc" internal/store/account_api_test.go` prints 1; `grep -c "^type \(Execution\|AccountValue\|Position\) struct" internal/store/api.go` prints 3; `go test ./internal/store -run 'TestAccount' -race -count=1` passes.
3. `grep -c "account.capture\|insert-only" docs/contracts/store.md` is at least 2.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e internal/store/api.go -e internal/store/account_api_test.go -e docs/contracts/store.md | wc -l | tr -d ' '` prints 0.

Checked by CI: ci-go; the design verdict.

### account-store-migration

- Module: store. Size: M. Depends on: account-store-api.
- Order: after 6 (account-store-api) and after the store issue that enables out-of-order migrations (WithAllowOutofOrder).
- Design: docs/design/account-data.md#Decisions (Decisions 5 and 6; second-opinion case)
- Files: migrations/NNNN_account.sql (NNNN the next free number at cut time), internal/store/setup/setup.go, internal/store/setup/setup_test.go, internal/store/setup/account_pg_test.go

- Goal: schema `account` with `account.capture`, `account.position`, `account.value` (foreign key to the capture) and `account.execution`, columns of the datasets (`numeric` for money and quantity); the role `ot_w_account` (INSERT and SELECT only) made by `setup` with the DSN line `OT_PG_DSN_ACCOUNT`; the views `rd.account_capture_r1`, `rd.account_position_r1`, `rd.account_value_r1` and `rd.account_execution_r1` (pass-through, owner `ot_owner`) with SELECT for `ot_ro_pages` on the views only; `ot_w_account` has INSERT and SELECT on the base tables, USAGE on schema `rd` and SELECT on the four views (its Reader reads them). The migration writes one statement per line, in this form: `CREATE VIEW rd.account_<name>_r1 ...` (plain `CREATE VIEW`), `GRANT SELECT ON rd.account_<name>_r1 TO ot_ro_pages;`, `GRANT SELECT ON rd.account_<name>_r1 TO ot_w_account;` for each of the four names, and `GRANT USAGE ON SCHEMA rd TO ot_w_account;`.

Acceptance:

1. `M=$(ls migrations/*_account.sql)`; `test -f $M` succeeds; `grep -cE "CREATE TABLE( IF NOT EXISTS)? account\.(capture|position|value|execution)" $M` prints 4; `grep -v '^--' $M | grep -ci "UPDATE\|DELETE\|TRUNCATE"` prints 0; `grep -ci "float\|double" $M` prints 0.
2. `grep -v '^--' $M | grep -c "CREATE VIEW rd.account_"` prints 4; `grep -v '^--' $M | grep -c "GRANT SELECT ON rd.account_[a-z]*_r1 TO ot_ro_pages;"` prints 4; `grep -v '^--' $M | grep -c "GRANT SELECT ON rd.account_[a-z]*_r1 TO ot_w_account;"` prints 4; `grep -v '^--' $M | grep -c "GRANT USAGE ON SCHEMA rd TO ot_w_account;"` prints 1; `grep -v '^--' $M | grep "ot_ro_pages" | grep -c "account\."` prints 0 (no grant on a base table); `grep -v '^--' $M | grep -c "ot_w_account"` is at least 2.
3. `grep -c "ot_w_account" internal/store/setup/setup.go` is at least 2; `go test ./internal/store/setup -run 'TestRoles|TestAccountRole' -race -count=1` passes.
4. `grep -c "//go:build pg" internal/store/setup/account_pg_test.go` prints 1.
5. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxE -e 'migrations/[0-9]{4}_account\.sql' -e 'internal/store/setup/setup\.go' -e 'internal/store/setup/setup_test\.go' -e 'internal/store/setup/account_pg_test\.go' | wc -l | tr -d ' '` prints 0.

Checked by CI: ci-go; the `pg` job in GitHub CI (`TestAccountMigration`: it applies on a clean database, `ot_w_account` has the grants above and no more, and can read back through the four views, `ot_ro_pages` can SELECT the four views and cannot SELECT a base table); the design verdict.

### account-store-impl

- Module: store and repo (judged two-module entry: `.go-arch-lint.yml` is a repo path, regenerated by `go run ./scripts/arch`; precedent `risk-alerts.md` `alerts-store-interface`). Size: M. Depends on: account-store-migration.
- Order: after 7 (account-store-migration). Entry 9 (account-job) follows it (both regenerate `.go-arch-lint.yml`).
- Design: docs/design/account-data.md#Decisions (Decisions 5 and 10)
- Files: internal/store/account/doc.go, internal/store/account/store.go, internal/store/account/read.go, internal/store/account/store_test.go, internal/store/account/store_pg_test.go, .go-arch-lint.yml

- Goal: `account.Open` returns the `store.AccountStore` over pgx with the role `ot_w_account`: `WriteCapture` is one transaction (a capture whose id exists writes nothing, children included), `WriteExecutions` is insert-only `ON CONFLICT DO NOTHING`, the Reader returns rows sorted by key. The non-`pg` tests run against a fake `database` as in `internal/store/snapshot`.

Acceptance:

1. `go test ./internal/store/account -race -count=1` passes; `grep -c "t.Parallel()" internal/store/account/store_test.go` is at least 4.
2. `grep -c "//go:build pg" internal/store/account/store_pg_test.go` prints 1; `grep -rl "pgx" internal --include=*.go | grep -vc '^internal/store/'` prints 0.
3. `grep -ci "update \|delete from\|truncate" internal/store/account/store.go` prints 0.
4. `wc -l internal/store/account/*.go | grep -v total | awk '$1>600' | wc -l | tr -d ' '` prints 0.
5. `grep -c "store/account" .go-arch-lint.yml` is at least 1 (that `go run ./scripts/arch --check` prints `arch: ok` is checked by CI).
6. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxE 'internal/store/account/[a-z_]+\.go|\.go-arch-lint\.yml' | wc -l | tr -d ' '` prints 0.

Checked by CI: ci-go; the `pg` job (`TestAccountStore`: write, rewrite the same capture id, executions twice, read back); the `arch.toml` check; the design verdict.

### account-job

- Module: jobs and repo (judged two-module entry: `.go-arch-lint.yml`, as above). Size: M. Depends on: account-broker-api, account-store-api and account-store-impl (the interfaces only for code: the job imports neither implementation; account-store-impl is a dependency only because both issues regenerate `.go-arch-lint.yml`; the implementations are wired in account-wire).
- Order: after 4, 6, 8 (account-broker-api, account-store-api, account-store-impl; so that `.go-arch-lint.yml` is regenerated once on top of account-store-impl).
- Design: docs/design/account-data.md#Decisions (Decisions 2 to 5, 7 and 8; second-opinion case: the new account access)
- Files: internal/jobs/accountcapture/job.go, internal/jobs/accountcapture/job.toml, internal/jobs/accountcapture/schedule.go, internal/jobs/accountcapture/job_test.go, internal/jobs/accountcapture/schedule_test.go, .go-arch-lint.yml

- Goal: the job `account_capture` through the interfaces only, built by `accountcapture.New(accountcapture.Config{Main string; Readers map[string]broker.AccountReader; Writer store.AccountWriter; Clock clock.Clock}) (contracts.Job, error)` in `job.go` (`Main` is the alias of Decision 8 and must be a key of `Readers`, else `New` returns an error; `Readers` holds one `broker.AccountReader` per alias; `store.AccountWriter`; `clock`): per alias one `Account` and one `Executions` call; the capture id of Decision 5; nothing written for a kind not collected; `pending` above zero for main ends the run failed with the status "pending" after the complete rows are written (for a non-main alias it is `degraded`); a failed alias does not stop the others; main fails the run, a non-main alias ends it `ok` with one `degraded` log record (alias and status only); a result names a failed alias and a status only; `job.toml` carries the schedule of Decision 4 and no account id, socket path or value.

Acceptance:

1. `go test ./internal/jobs/accountcapture -race -count=1` passes with fakes of both interfaces and the test kit's fake clock; the tests include a retry of the same run key (no second capture), a kind not collected (no row), a failing alias among two, `pending` above zero on main (rows written, run failed) and on a non-main alias (degraded, the run stays ok).
1a. `grep -c "func New(" internal/jobs/accountcapture/job.go` prints 1; `grep -c "Main string" internal/jobs/accountcapture/job.go` prints 1; a test `TestNewRefusesMainNotInReaders` is present once.
2a. Main fails the run; non-main failure leaves it ok with one degraded log record; no gap row: `go test ./internal/jobs/accountcapture -run 'TestMainFailsRun|TestNonMainFailureDegraded|TestNonMainPendingDegraded|TestNoGapForNonMain' -race -count=1` passes, each name present once, and `grep -c "degraded" internal/jobs/accountcapture/job.go` is at least 1.
2. `grep -c "time.Sleep" internal/jobs/accountcapture/*.go` prints 0; `grep -c "^\[schedule\]" internal/jobs/accountcapture/job.toml` prints 1; `grep -ciE "DU[0-9]|U[0-9]{6,}|socket|token" internal/jobs/accountcapture/job.toml` prints 0.
3. `grep -rln "internal/notify\|internal/store/account\|grpcbroker" internal/jobs/accountcapture | wc -l | tr -d ' '` prints 0; `grep -c "jobs/accountcapture" .go-arch-lint.yml` is at least 1 (that `go run ./scripts/arch --check` prints `arch: ok` is checked by CI).
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxE 'internal/jobs/accountcapture/[a-z_.]+|\.go-arch-lint\.yml' | wc -l | tr -d ' '` prints 0.

Checked by CI: ci-go; the `arch.toml` check; the design verdict.

### account-wire

- Module: ot. Size: S. Depends on: account-broker-impl, account-store-impl and account-job.
- Order: after 5, 8, 9 (account-broker-impl, account-store-impl, account-job).
- Design: docs/design/account-data.md#Decisions (Decision 8)
- Files: cmd/ot/account.go, cmd/ot/account_test.go, cmd/ot/runonce.go, cmd/ot/wire.go, docs/contracts/ot.md

- Goal: the registry entry `account_capture` (constructor with `grpcbroker.DialAccount` per socket of the owner's list, read by the key `OT_ACCOUNT_SOCKETS`, a comma list of `label=socket path` with no flag, and main (the `account` field of the required entry) read from the gateways file whose path is the key `OT_IBKR_GATEWAYS` (as `OT_ACCOUNT_SOCKETS`, `OT_PG_DSN_ACCOUNT` and the other keys, read through `deps.readEnv` from `$OT_HOME/.env`, never `os.Getenv`; an unset key refuses the job like a file with no required entry) by Decision 8; the wiring passes it to `accountcapture.New(accountcapture.Config{Main string; Readers map[string]broker.AccountReader; Writer store.AccountWriter; Clock clock.Clock})` (the Config of account-job: one `grpcbroker.DialAccount` per `label=socket path` of `OT_ACCOUNT_SOCKETS`, passed as `Readers` keyed by label, `Main` from the gateways file, `Writer` from `account.Open`), the one exported constructor of the job package (declared in `internal/jobs/accountcapture/job.go`, not in `internal/jobs/api.go`, so no interface issue is added and the Files of account-job stay as they are) (zero or several required entries, or a main label missing from the list, refuse the job; `docs/contracts/ot.md` gets the same wording and one row each for `OT_ACCOUNT_SOCKETS` and `OT_IBKR_GATEWAYS` in the table of keys with the Source `$OT_HOME/.env`, no value), and `account.Open` with `OT_PG_DSN_ACCOUNT`) so that `ot run-once account_capture` and the scheduler find the job: the job list of `wireJobs` in `cmd/ot/runonce.go` (432 lines; the `jobs: []contracts.Job{...}` of `wiring`, which both `run-once` and the scheduler read) gains the entry, `cmd/ot/wire.go` (233 lines) gains the constructor fields in `deps` (the pattern of `newQuoteSnapshot`), and the new `cmd/ot/account.go` holds the constructor; there is no `jobs.go`; a dry run with fakes in the test; the contract document names the two keys and no value. The two new import paths are already in the `cmd/ot` list of `modules.md` by this design.

Acceptance:

1. `go test ./cmd/ot -run 'TestAccount' -race -count=1` passes.
2. `grep -c "OT_PG_DSN_ACCOUNT\|DSNEnvAccount" cmd/ot/account.go` is at least 1; `grep -c "internal/notify" cmd/ot/account.go` prints 0.
3. `go test ./cmd/ot -run 'TestAccountRefusesNoMain|TestAccountRefusesTwoMains|TestAccountRefusesMainNotInSockets|TestAccountGatewaysKeyReadThroughDeps' -race -count=1` passes, each name present once in `cmd/ot/account_test.go` (`TestAccountGatewaysKeyReadThroughDeps` builds `deps` with a fake `readEnv` giving the path); `grep -c "os.Getenv" cmd/ot/account.go` prints 0; `grep -c "OT_IBKR_GATEWAYS\|OT_ACCOUNT_SOCKETS" docs/contracts/ot.md` is at least 2. `grep -c "account" cmd/ot/runonce.go` is at least 1 (the entry is in `wireJobs`).
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxE 'cmd/ot/(account|account_test|runonce|wire)\.go|docs/contracts/ot\.md' | wc -l | tr -d ' '` prints 0.

Checked by CI: ci-go; the `arch.toml` check; the design verdict. Running the job against the real gateway is the owner's (`ot run-once`), not a test.

## Checked by CI

- `go test ./internal/broker/... ./internal/store/... ./internal/jobs/accountcapture ./cmd/ot -race -count=1` (ci-go: parallel, `t.TempDir()`, fake clock, no sleep).
- `go test -tags pg ./internal/store/...` (the `pg` job, GitHub CI only).
- `go run ./scripts/arch --check` printing `arch: ok` (account-store-impl, account-job and account-wire); the generated-code check of `contracts/generate.sh` (account-proto). Both run Go tools that the acceptance checker refuses, as in `compute-core.md`.
- Pyside: `uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`.

## Left out

- Rejected: every alias required (a spouse's gateway that is logged out would fail the run and open a gap every day); main marked by a fixed label in code (a name of an account in the repository, Decision 8); a second flag in `OT_ACCOUNT_SOCKETS` or a state table for the last state of an alias (two sources, new state for a log line).
- A non-main alias that is down (owner decision 2026-11-09) loses its executions of that day, as in the next item; only main is guaranteed. The run is `ok`, so no gap is reported for it; the `degraded` log record is the only trace.
- A missed day of executions: the API holds the current day only (Decision 4), so a missed run loses it. Mitigation: the scheduler reports the gap; Flex or `ot import` fills it. Flex (executions, cash events and assignments with full history) is deferred: it is a new outside account access with a token. Rejected as stage 1 because the API already carries the day's executions without a second credential.
- `ot import` of option-trader's history: the cut-over design (alignment Decision 3.8).
- The seller features of Decision 9, the local notice and the dividend source: each its own later design over these tables.
- Readers beyond Go: an `ot` subcommand or page that shows the stored rows, position prices, unrealized P&L, per-currency ledger, open orders, the account-update stream (orders stay refused by the guard). Pages, compute figures and `report_page` blocks are stage 2.
- Rejected: an update of the commission after the first write (a UPDATE grant, a nullable column); a "current" overwrite table instead of captures (captures are the history that assignment detection and tracking read); one four-module interface entry (the review's R1, below); a random capture id.
- Lock-ins, named: the `capture_id` text shape and the tag rows are permanent keys of append-only tables, the alias is the identity of every row (renaming an alias in the deploy file orphans history), and the proto field numbers are fixed once the owner merges.
- Telegram text about accounts: never (owner decision 5 a).

## Adversarial

Review: `account-adversarial.md` (scratchpad of this session). Each point:

- Q1 goal gaps: readable is the Go Reader only (adopted: stated in Decision 1 and Left out); executions start from the first run and the history waits for Flex or `ot import` (adopted: Decision 4, Left out); no prices or P&L (adopted: Left out).
- Q2 and Q5 partial status and "collected" (adopted: Decision 2, proto fields 5 and 6, job tests). The claim "a few days of executions" is replaced by the current day only (adopted: Decision 4).
- Q3 simpler cut: interface entry for contracts only, broker and store declarations in their own issues (adopted); executions written complete and insert-only, no UPDATE grant (adopted: Decision 3 and 6); one run per session day (adopted: Decision 4).
- Q4 lock-ins named (adopted: Left out). The role per data class is kept: account data must not share a role with market data.
- Q6 and R2 owner-only merges, amended in revision 5: `transport.py` had been moved out of `account-pyside-executions` into the small owner-merge issue `account-guard-id`, but the handler needs it (the `AccountServicer` method at about `transport.py:461`, the registration in `build_server` at about 519 to 541, and `ibgateway.account` at about line 301 in `ibgateway.py`), so `transport.py` and `ibgateway.py` are in the Files of `account-pyside-executions` again and that entry is an owner merge. The owner merges one pull request more or less either way (guard-id, executions and the contracts issue are owner merges); `account-guard-id` stays a one-id change (`, 7` on the first line of `ALLOWED_IDS`) so that the guard change is read alone, and executions follows it. The vacuous grep is dropped (adopted).
- R1 four-module entry: refused and removed, with the self-licence paragraph (adopted). The two repo pairs left (account-store-impl and account-job) are the `.go-arch-lint.yml` precedent only; account-job is ordered after account-store-impl because both regenerate that file.
- R3 weak acceptance: baselines are now `git merge-base` diffs, Files-bounded diff commands per issue, the generate check names `contracts/generate.sh` and its output paths, the `ot_ro_pages` grant is a migration line and a `pg` test, the collected rule has job tests (adopted). The grants themselves are verified only by the `pg` job, which is GitHub CI by AGENTS.md (not adopted as an offline check, because no offline tool can).
- R4 dependencies: the job depends on the interfaces (account-broker-api and account-store-api) only, plus account-store-impl for the shared `.go-arch-lint.yml` ordering; the broker is split into the interface issue account-broker-api and the implementation issue account-broker-impl, so the job never waits for the implementation; account-wire depends on account-broker-impl, account-store-impl and account-job (adopted, revision 3).
- R5 contradictions: the `cmd/ot` import list of `modules.md` is edited by this design (adopted); `.go-arch-lint.yml` is in the Files of issues account-store-impl and account-job (adopted); the capture id derives from the run key (adopted: Decision 5); the status rule is computable (adopted: Decision 2); the alias is the `account` field of the gateway entry, a label and not an account figure (checked, recorded in Decisions 7 and 8); "Owner approved: (pending)" stays until the second-opinion cases are answered.
- Gateway A (owner decision 2026-11-09): both gateways are captured in stage 1 (Decision 8, revised: replaces "gateway B only" and the question for the owner). The rows of the two accounts are already told apart by `alias` (Decision 5), so no dataset, migration, Reader or issue changes; the lock-in on the alias now covers two accounts.
