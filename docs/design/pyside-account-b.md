# Pyside account service: the account summary and the positions of a gateway, read-only

Author: the optdesk dispatcher (draft)
Owner approved: (pending)
Date: 2026-11-07
Source: owner request of 2026-11-07: gateway B (the second account of the gateways file of `docs/design/pyside-gateways-health.md`, merged) serves the account summary and the positions of its
account, read-only, so that optdesk can show them. `## Left out` of that design names this as the next design. Read on main at fefd76d: `docs/design/pyside-gateways-health.md`,
`docs/design/pyside-ibkr-transport.md`, `docs/design/modules.md`, `docs/contracts/broker.md`, `docs/deploy/pyside.md`, `contracts/proto/ibkr/v1/*.proto`, `pyside/src/optdesk/ibkr/transport.py`
(owner-only), `server.py`, `gateways.py`, `ibgateway.py`, `model.py`, `pacing.py`, `timing.py`, `pyside/src/optdesk/contracts/api.py`, `pyside/tests/ibkr/test_guard.py`,
`pyside/tests/contracts/test_contracts_api.py`, `pyside/examples/gateways.example.toml`, `internal/broker/api.go`, `internal/broker/grpcbroker/` and `buf.gen.yaml`.

In this repository the two gateways are called gateway A and gateway B and their accounts account A and account B. A file or a test names an account only by its alias; no account id appears
anywhere.

## What main holds today

- `ibkr.v1` (`contracts/proto/ibkr/v1`) has `SessionService` (`Ping`), `ContractsService` (`ListChains`, `ResolveContract`) and `MarketDataService` (`Snapshot`). No account or position message exists.
- `transport.py` has `ALLOWED_IDS = {1, 2, 9, 20, 25, 49, 59, 71, 78}`; `ORDER_IDS = {3, 4, 21, 58}` are refused first; every other id raises `NotAllowed`. The ids of the position request (61) and the account summary
  request (62) are not on it (`pyside/tests/ibkr/test_guard.py` states both numbers). `install_guard` is per client, so the client of gateway B is guarded like the one of gateway A.
- `IbGateway.open_session` connects with `ib.client.connectAsync`, not `IB.connectAsync`, so `ib_async` does not start any account, position, order or execution fetch at connect.
- `gateways.py` already accepts the `collect` words `account_summary` and `positions` and keeps the alias in `GatewaySpec.account`. `server._socket_paths` returns no socket for a gateway whose `collect` lacks
  `market_data`, so gateway B, with `collect = ["account_summary", "positions"]`, has a session and a health entry and nothing a client can dial. `build_server(gateway, clock)` registers the three services
  on every socket.
- Go: `broker.Broker` (`internal/broker/api.go`) has `ListChains`, `ResolveContract`, `Snapshot` and `Close`; `grpcbroker.Dial(ctx, socketPath, broker.Options)` connects to a Unix socket, answers with plain
  Go types and maps every gRPC code to `Retryable` (`UNAVAILABLE`, `DEADLINE_EXCEEDED`) or `Permanent`. `docs/contracts/broker.md` says "No type or call exists for orders, positions or accounts."
- `modules.md` lists `contracts/proto/` and `pyside/src/optdesk/ibkr/transport.py` as owner-only paths: a pull request that touches one is merged by the owner.

## Decisions

1. A new service `AccountService` in a new proto file `contracts/proto/ibkr/v1/account.proto`, with exactly two read rpcs and no other:
   `AccountSummary(AccountSummaryRequest) returns (AccountSummaryResponse)` and `Positions(PositionsRequest) returns (PositionsResponse)`. Both requests are empty. Alternative rejected: add the two rpcs
   to `SessionService` or `ContractsService`: a service per kind of information lets pyside register a service only on the sockets of the gateways that collect it, and keeps `ContractsService` free of
   account data.
2. The messages (field numbers are the order shown):
   - `AccountSummaryResponse { string account = 1; int64 generation = 2; repeated AccountValue values = 3; }` and `AccountValue { string tag = 1; string value = 2; string currency = 3; }`.
   - `PositionsResponse { string account = 1; int64 generation = 2; repeated Position positions = 3; }` and `Position { ContractSpec contract = 1; string quantity = 2; string avg_cost = 3; }`, with `ContractSpec`
     from `contract.proto`.
   - `account` is the alias of the gateway's entry (`GatewaySpec.account`), never the account id the gateway reports; `generation` is the session generation of `PingResponse`, so two answers of two
     sessions can be told apart. Numbers are canonical decimal text (the way `strike` is), never floats. No message carries an order, an order id, an execution or a permission.
3. The tags are a fixed list in pyside, not a request field: `NetLiquidation`, `TotalCashValue`, `AvailableFunds`, `ExcessLiquidity`, `BuyingPower`, `GrossPositionValue`, `EquityWithLoanValue`, `InitMarginReq`,
   `MaintMarginReq`, `Cushion`. A caller cannot ask for another tag. The list is a constant of `accounts.py`; a change of it is a change of one file.
4. One account per gateway. The handler reads the accounts the gateway reports (`managedAccounts`); when there is not exactly one, the rpc fails `FAILED_PRECONDITION` with a fixed text and returns no
   value. Only rows of that one account are returned, and every row is rebuilt from the typed fields, so the id the gateway reports is dropped and the alias is written. A test feeds a fake whose rows carry an
   id-shaped string and checks that the serialized response contains none.
5. The allowlist of `transport.py` gains exactly four ids: 61 (positions request), 62 (account summary request), 63 (account summary cancel) and 64 (positions cancel). `ORDER_IDS` is unchanged and is still
   checked first; the account-update request (6), the executions request, the open-orders requests and every other id stay refused. The handler never subscribes for good: each rpc requests, reads until the
   end message of the gateway, and cancels (63 or 64) in a `finally`, so no subscription outlives the call and the 2-subscription limit of account summaries is not held. Calls to one gateway are serialized
   by a lock per kind. Each request goes through the pacer of that gateway like every other message (ADR 0003 lists account requests among the paced kinds) and ends after a deadline taken from the injected
   `Clock` (20 seconds, `DEADLINE_EXCEEDED`).
6. Which sockets and which services. A gateway is served on `ibkr-<name>.sock` when its `collect` holds any of the three words (today: `market_data` only). `SessionService` is on every socket. `ContractsService`
   and `MarketDataService` are on a socket only when `market_data` is in `collect`; `AccountService` only when `account_summary` or `positions` is. An rpc of a kind the gateway does not collect (`Positions` on
   a gateway that lists only `account_summary`) fails `FAILED_PRECONDITION`. The rule that the gateway on `ibkr.sock` has `market_data` is unchanged, so `ot run-once` is unaffected. The service is registered
   by `server.py` on the server `build_server` returns; `build_server` keeps its signature and its three services.
7. The log lines are unchanged in kind: `rpc_done` and `rpc_failed` with `rpc` set to `account_summary` or `positions` and `count` (the number of rows), through the whitelist of `transport.py`
   (`WORDS` gains the two names). No value, tag amount, symbol or alias is logged.
8. Go side: a client in the existing package `internal/broker/grpcbroker`, not a method on `broker.Broker`. `DialAccount(ctx, socketPath, broker.Options) (*AccountClient, error)` with `AccountSummary(ctx)`,
   `Positions(ctx)` and `Close()`, plain types `AccountSummary`, `AccountValue` and `Position` (a `broker.Contract`, quantity and average cost as text) declared in the package, errors and defaults as `Dial`
   (`CallTimeout`, `Retryable` for `UNAVAILABLE` and `DEADLINE_EXCEEDED`). The connect loop of `Dial` becomes one unexported function both use. Alternative rejected: add the two methods to `broker.Broker`:
   that is an edit of `internal/broker/api.go`, a second interface entry, and it breaks every fake of `broker.Broker` in jobs and ot for a consumer that does not exist yet. The port the first consumer
   needs (the job that stores an account snapshot, a later design) is declared by that design, over this client.
9. Nothing places orders and nothing here can. The guard refuses the order ids, the proto has no order message, the Go client has no such call, and the entries below say so in tests (the service has two
   rpcs; the real `Client` with a recording stub connection sends only 61, 62, 63 and 64 for the two rpcs).
10. Gateway B is configured, not coded: the example file `pyside/examples/gateways.example.toml` shows a gateway with `collect = ["account_summary", "positions"]` and no `market_data`; ports and aliases
    stay placeholders. The real file stays on the owner's machine.

## Modules

No row of `docs/design/modules.md` changes and no module is added. Paths by module:

- contracts: `contracts/proto/ibkr/v1/account.proto` (owner-only path), the Go output `internal/contracts/genpb/account.pb.go` and `internal/contracts/genpb/account_grpc.pb.go`, the Python output
  `pyside/src/optdesk/contracts/genpb/ibkr/v1/account_pb2.py`, `account_pb2.pyi`, `account_pb2_grpc.py` and `account_pb2_grpc.pyi`, the re-export file `pyside/src/optdesk/contracts/api.py`, its test
  `pyside/tests/contracts/test_contracts_api.py` and a new Go test `internal/contracts/account_proto_test.go`.
- pyside: `pyside/src/optdesk/ibkr/transport.py` (owner-only), `accounts.py` (new), `model.py`, `ibgateway.py`, `server.py`, `pyside/examples/gateways.example.toml`, and the tests
  `pyside/tests/ibkr/test_accounts.py` (new), `test_guard.py`, `test_server.py`. No new dependency: `pyside/pyproject.toml` and `pyside/uv.lock` do not change.
- broker: `internal/broker/grpcbroker/account.go` (new), `dial.go`, `account_test.go` (new) and `docs/contracts/broker.md`. `internal/broker/api.go` is not edited. No new Go package, so no regenerated arch file.
- docs: `docs/deploy/pyside.md`.

## Interface

The interface is the proto file `contracts/proto/ibkr/v1/account.proto` of decisions 1 to 3, owned by the contracts module, with the Go and Python output generated from it and committed together
(`sh contracts/generate.sh`). The Go port of the broker module and the Python `api.py` of `ibkr` are not edited. The interface entry is `account-proto`; the pyside entry and the broker entry use the
generated types and need nothing else from each other. The contract is tested from both sides: Python by `test_contracts_api.py` (servicer, stub and `add_..._to_server` of the new service, a fake and a
caller), Go by `internal/contracts/account_proto_test.go` (the service descriptor lists exactly the two rpcs; a fake server and a client compile against the generated code).

## Issues

Entry `account-proto` (interface) starts the day this design merges. Entries `pyside-account` and `go-account-client` each carry their own `Order:` line and need only `account-proto`; neither needs the
other's output. Entry `pyside-runbook-account` documents what `pyside-account` delivers. What is seen end to end (gateway B logged in, the service restarted with a gateway B file, a client dialing
`ibkr-<name>.sock` and getting the alias, values and positions) needs all three code entries merged and is the owner's check, not a test. Entries `account-proto` and `pyside-account` touch owner-only
paths (`contracts/proto/`, the allowlist of `transport.py`): each carries the waiver lines and the owner merges it.

### account-proto (interface)

Module: contracts. It adds a service to the proto of the broker transport (owner-only `contracts/proto/`).

Second opinion: waived by the owner — the advisor answered it (<date>, filled by the dispatcher)
The owner merges this pull request.
Advisor: agrees — (filled by the dispatcher)

Files: contracts/proto/ibkr/v1/account.proto, internal/contracts/genpb/account.pb.go, internal/contracts/genpb/account_grpc.pb.go, pyside/src/optdesk/contracts/genpb/ibkr/v1/account_pb2.py, pyside/src/optdesk/contracts/genpb/ibkr/v1/account_pb2.pyi, pyside/src/optdesk/contracts/genpb/ibkr/v1/account_pb2_grpc.py, pyside/src/optdesk/contracts/genpb/ibkr/v1/account_pb2_grpc.pyi, pyside/src/optdesk/contracts/api.py, pyside/tests/contracts/test_contracts_api.py, internal/contracts/account_proto_test.go

Design: docs/design/pyside-account-b.md#decisions

Depends on: none.

Order: first.

Acceptance:

1. `contracts/proto/ibkr/v1/account.proto` declares `package ibkr.v1`, imports `ibkr/v1/contract.proto`, and holds the service `AccountService` with exactly two rpcs: `grep -c 'rpc ' contracts/proto/ibkr/v1/account.proto`
   prints 2, and the names are `AccountSummary` and `Positions`; no message or rpc name contains `Order`, `Execution`, `Cancel` or `Place` (`grep -ci 'order\|execution\|cancel\|place' contracts/proto/ibkr/v1/account.proto` prints 0).
2. The messages are those of decision 2 of docs/design/pyside-account-b.md with the field numbers shown, every message and field has a comment, and `quantity` and `avg_cost` are `string`; `buf lint` and
   `buf build` pass on `contracts/proto`.
3. The generated Go files `internal/contracts/genpb/account.pb.go` and `account_grpc.pb.go` and the four Python files are exactly the output of `sh contracts/generate.sh` (the generated-code check of `checks.yml`
   finds no difference); none was edited by hand.
4. `pyside/src/optdesk/contracts/api.py` re-exports `AccountServiceServicer`, `AccountServiceStub`, `add_AccountServiceServicer_to_server`, `AccountSummaryRequest`, `AccountSummaryResponse`, `AccountValue`,
   `PositionsRequest`, `PositionsResponse` and `Position` in `__all__`, holds no logic, and imports no other new name.
5. `pyside/tests/contracts/test_contracts_api.py` gains a test, `test_account_service_surface`, that checks from both sides with a fake servicer and a stub: the servicer base has exactly the methods
   `AccountSummary` and `Positions`, a stub has the same two, and a caller reads `account`, `generation` and the repeated rows of a response built by the fake; `cd pyside && uv run pytest tests/contracts -n auto` passes.
6. `internal/contracts/account_proto_test.go` has `TestAccountServiceHasTwoReadRpcs` (the descriptor of `AccountService` lists exactly `AccountSummary` and `Positions`, neither streaming) and a test using a
   fake server on a Unix socket in `t.TempDir()` with `t.Parallel()`; `go test ./internal/contracts/... -count=1 -race` passes.
7. `ibkr.v1` of the other proto files is unchanged: `git diff --stat origin/main -- contracts/proto` lists only `account.proto`.
8. No file outside the ten listed is in the diff.
9. The result matches the sections Decisions (1 to 3) and Interface of docs/design/pyside-account-b.md, judged from the code and the design file.

Checked by CI: the generated-code check of `checks.yml` and `buf lint`.

### pyside-account

Module: pyside. It adds four ids to the allowlist of `transport.py` (owner-only) and serves account data of a gateway.

Second opinion: waived by the owner — the advisor answered it (<date>, filled by the dispatcher)
The owner merges this pull request.
Advisor: agrees — (filled by the dispatcher)

Files: pyside/src/optdesk/ibkr/transport.py, pyside/src/optdesk/ibkr/accounts.py, pyside/src/optdesk/ibkr/model.py, pyside/src/optdesk/ibkr/ibgateway.py, pyside/src/optdesk/ibkr/server.py, pyside/examples/gateways.example.toml, pyside/tests/ibkr/test_accounts.py, pyside/tests/ibkr/test_guard.py, pyside/tests/ibkr/test_server.py

Design: docs/design/pyside-account-b.md#decisions

Depends on: account-proto (the entry marked (interface)).

Order: after account-proto.

Acceptance:

1. `ALLOWED_IDS` in `pyside/src/optdesk/ibkr/transport.py` is `{1, 2, 9, 20, 25, 49, 59, 71, 78, 61, 62, 63, 64}` and `ORDER_IDS` is unchanged; `pyside/tests/ibkr/test_guard.py`
   `test_account_ids_pass` sends frames 61, 62, 63 and 64 through the real `Client` with the recording stub connection and all four are recorded, and `test_order_and_other_ids_still_refused` checks that 3, 4, 21, 58, 6 and 5
   still raise (`OrderRefused` for the first four, `NotAllowed` for 6 and 5).
2. `pyside/src/optdesk/ibkr/accounts.py` (new, under 600 lines) holds the tag tuple of decision 3 of docs/design/pyside-account-b.md and the two reads. Each read sends its request through the pacer, waits for
   the end message with a deadline from the injected `Clock`, and cancels (63 or 64) in a `finally`, also when the read fails or times out; `test_summary_cancels_after_failure` and `test_positions_cancel_after_timeout`
   in `pyside/tests/ibkr/test_accounts.py` use a fake `ib` and a fake clock and assert the cancel call.
3. `test_accounts.py` `test_real_client_sends_only_account_ids`: a real `ib_async` `Client` with the guard installed and a recording stub connection, driven through both reads, records frames whose ids are only
   61, 62, 63 and 64 (plus none of `ORDER_IDS`).
4. `model.py` declares a Protocol `AccountGateway` with the two async methods and the typed rows they return; the existing `Gateway` Protocol is unchanged, so no existing fake needs a new method
   (`git diff origin/main -- pyside/src/optdesk/ibkr/model.py` shows no removed line). `ibgateway.py` implements the two methods on `IbGateway` through `accounts.py`, using the session and generation of
   the connected client, with a lock per kind (`test_reads_of_one_kind_are_serialized`).
5. Account handling: with a fake whose `managedAccounts` has two entries, both rpcs fail `FAILED_PRECONDITION` and return no value (`test_two_accounts_refused`); with one entry, the response `account` is
   the alias of the spec, `generation` is the session generation, only the fixed tags are returned, and a fake row carrying an id-shaped string leaves no such string in `response.SerializeToString()`
   (`test_alias_replaces_id`). Numbers are decimal text with no exponent (`test_quantity_is_decimal_text`).
6. `AccountServicer` (in `transport.py`, built on the shared call helper) answers `AccountSummary` and `Positions` over a real `grpc.aio` server on a socket in a temporary directory; an rpc of a kind not in the
   gateway's `collect` is `FAILED_PRECONDITION` (`test_positions_refused_when_not_collected`); the log records of both rpcs hold only whitelisted fields and words and no value, tag, symbol or alias
   (`test_account_log_has_no_values`); `WORDS` gains `account_summary` and `positions` and nothing else.
7. `server.py`: `_socket_paths` returns the named socket for a gateway whose `collect` holds any of the three words; `ContractsService` and `MarketDataService` are registered only with `market_data`,
   `AccountService` only with `account_summary` or `positions`, and `SessionService` always. In `pyside/tests/ibkr/test_server.py`, `test_account_only_gateway_has_a_socket` (a gateway with the two account words
   and no `market_data` has `ibkr-<name>.sock` and answers the account rpcs with a fake, and `Snapshot` on it is `UNIMPLEMENTED`) and `test_default_socket_still_needs_market_data` pass; `build_server`
   keeps its signature.
8. `pyside/examples/gateways.example.toml` keeps its placeholders (no real port, alias or id), its second gateway lists `collect = ["account_summary", "positions"]`, and `pyside/tests/ibkr/test_gateways.py`
   still loads it (`cd pyside && uv run pytest tests/ibkr/test_gateways.py -n auto` passes).
9. Every test runs under `pytest -n auto` (a temporary directory for every socket, `kit.sockdir`), a fake clock from `pyside/tests/kit/fakeclock.py` and no sleep; none connects to a
   broker. `cd pyside && uv sync --locked && uv run ruff check && uv run mypy --strict && uv run pytest -n auto` passes.
10. No file outside the nine listed is in the diff; `pyside/pyproject.toml` and `pyside/uv.lock` are unchanged; no new source file is over 600 lines and `transport.py`, `ibgateway.py` stay under 800.
11. The result matches the section Decisions (points 3 to 7 and 9 to 10) of docs/design/pyside-account-b.md, judged from the code and the design file.

### go-account-client

Module: broker.

Files: internal/broker/grpcbroker/account.go, internal/broker/grpcbroker/dial.go, internal/broker/grpcbroker/account_test.go, docs/contracts/broker.md

Design: docs/design/pyside-account-b.md#decisions

Depends on: account-proto (the entry marked (interface)).

Order: after account-proto. It shares no file with `pyside-account` and needs none of its output.

Acceptance:

1. `internal/broker/grpcbroker/account.go` declares `DialAccount(ctx, socketPath, broker.Options) (*AccountClient, error)`, `(*AccountClient).AccountSummary`, `Positions` and `Close`, and the types
   `AccountSummary`, `AccountValue`, `Position` and `Positions`; `grep -c 'broker.Broker' internal/broker/grpcbroker/account.go` prints 0 and `git diff --stat origin/main -- internal/broker/api.go` prints nothing.
2. `dial.go` keeps `Dial` and `var _ broker.DialFunc = Dial` and moves the connect loop into one unexported function that `Dial` and `DialAccount` both call; the existing tests of the package still pass.
3. `account_test.go` uses a fake `AccountService` on a Unix socket in `t.TempDir()` with `t.Parallel()`: `TestAccountSummaryValues` (alias, generation, rows in order, a currency kept), `TestPositionsContract` (a
   position's `broker.Contract` fields, quantity and average cost as received text), `TestAccountErrorKinds` (`UNAVAILABLE` and `DEADLINE_EXCEEDED` are `Retryable`, `FAILED_PRECONDITION` and `INVALID_ARGUMENT`
   `Permanent`; the error text holds no socket path), `TestAccountCallTimeout` (a fake that blocks gives `DEADLINE_EXCEEDED` after a short `CallTimeout`, without a sleep in the test: the fake waits on the
   context) and `TestDialAccountRefuses` (an unreachable socket gives `Retryable`).
4. A response with no alias or a negative generation is a `Permanent` error (`TestAccountEmptyAliasRefused`); the client keeps no state between calls and does not retry inside a call.
5. `docs/contracts/broker.md` replaces the sentence "No type or call exists for orders, positions or accounts." by one that says no type or call exists for orders, and describes `DialAccount`, the two calls, the
   plain types, the alias rule (the answer carries the alias, the client never holds an account id) and the contract tests by name; the last line of the file stays a version line one above the present.
6. `go vet ./internal/broker/...` and `go test ./internal/broker/... -count=1 -race` pass; `go run ./scripts/arch --check` prints no difference (no new package).
7. No file outside the four listed is in the diff, and each new file is under 600 lines.
8. The result matches the section Decisions (point 8 and 9) of docs/design/pyside-account-b.md, judged from the code and the design file.

### pyside-runbook-account

Module: docs.

Files: docs/deploy/pyside.md

Design: docs/design/pyside-account-b.md#decisions

Depends on: pyside-account (it documents what that entry delivers).

Order: after pyside-account (no shared file; the order is prose).

Acceptance:

1. The section `## Sockets` of `docs/deploy/pyside.md` says that a gateway is served on `ibkr-<name>.sock` when its `collect` holds any of `market_data`, `account_summary` and `positions`, replaces the sentence
   that a gateway without `market_data` has no socket, and says that `ibkr.sock` still belongs to a gateway with `market_data`.
2. A new section `## Account data` says: which rpcs (`AccountSummary`, `Positions`), that the answer carries the alias and never an account id, that exactly one account per gateway is served and a login with
   several accounts is refused, that nothing is subscribed beyond a call, that the four ids 61 to 64 are the only additions to the allowlist and orders stay refused, and that the file with gateway B stays on the
   owner's machine.
3. `grep -c '[A-Za-z]\{1,\}[0-9]\{5,9\}' docs/deploy/pyside.md` prints the same count as on origin/main, and the file contains no port number, host address or account id.
4. No file outside `docs/deploy/pyside.md` is in the diff.
5. The result matches the section Decisions (points 5, 6 and 10) of docs/design/pyside-account-b.md, judged from the file and the design file.

## Left out

- A consumer: the job that calls the new client on a schedule and stores an account snapshot (the dataset, the `ot` command and its manifest), and the page that shows it. Each is a design of its own over this
  client; this design delivers the data path from gateway B to a Go caller and nothing that stores or shows.
- A port `broker.AccountReader` in `internal/broker/api.go`: declared by the design of the first consumer (decision 8).
- A stamp for how old an answer is (`received_at`): the caller stamps with its own clock when it stores.
- Account figures beyond the ten tags (margin by currency, `$LEDGER` segments), a tag chosen by the caller, a login with several accounts (an advisor structure), executions, open orders, trades, and the
  account-update stream: each needs its own allowlist id and is refused today.
- Fallback from gateway A to gateway B inside pyside: unchanged from the earlier design; a caller dials the socket it wants.
- Account figures for gateway A: the same service serves them when its entry lists the words; no extra work, and no decision to collect them is taken here.
- A change of `docs/adr/0003-market-data-architecture.md`: it names "a later issue may add them to the allowlist"; this design is that issue, and the ADR text stays the owner's to amend.
- Splitting the pyside entry in an allowlist entry and a handler entry: the handler is untestable against the real guard without the ids, the owner reviews the whole security surface in one pull request, and the
  two would share `transport.py`.

## Adversarial

pending
