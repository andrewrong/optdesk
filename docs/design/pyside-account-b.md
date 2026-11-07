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

1. A new service `AccountService` in a new proto file `contracts/proto/ibkr/v1/account.proto`, with exactly one read rpc: `Account(AccountRequest) returns (AccountResponse)`. The request is empty. One rpc, not
   two: the owner reads the summary and the positions of an account together (the glossary keeps figures per account alias), one rpc means one lock, one failure, one cancel path and half the proto, the Go
   surface and the tests; no code reason needs two (the two requests of the gateway have their own request ids, and the handler sends both inside one call). Alternative rejected: two rpcs, or the rpc added to
   `SessionService` or `ContractsService`: a service per kind of information lets pyside register it only on the sockets of the gateways that collect it, and keeps `ContractsService` free of account data.
2. The messages (field numbers are the order shown):
   - `AccountResponse { string account = 1; int64 generation = 2; repeated AccountValue values = 3; repeated Position positions = 4; }`, `AccountValue { string tag = 1; string value = 2; string currency = 3; }`
     and `Position { ContractSpec contract = 1; string quantity = 2; string avg_cost = 3; }`, with `ContractSpec` from `contract.proto`.
   - `account` is the alias of the gateway's entry (`GatewaySpec.account`), never the account id the gateway reports; `generation` is the session generation of `PingResponse`, so two answers of two
     sessions can be told apart. Numbers are canonical decimal text (the way `strike` is), never floats. No message carries an order, an order id, an execution or a permission.
   - `values` is filled when `account_summary` is in the gateway's `collect` and `positions` when `positions` is; the other list is empty, and an empty `positions` of a gateway that collects it means no
     open position.
3. The tags are a fixed list in pyside, not a request field: `NetLiquidation`, `TotalCashValue`, `AvailableFunds`, `ExcessLiquidity`, `BuyingPower`, `GrossPositionValue`, `EquityWithLoanValue`, `InitMarginReq`,
   `MaintMarginReq`, `Cushion`. A caller of the rpc cannot ask for another tag. The guard checks the id of a frame, not its content, so the list is a rule of the handler, not of the guard: anything holding the
   client could send id 62 with other tags. `accounts.py` therefore calls `client.reqAccountSummary(reqId, "All", tags)` itself with exactly these ten tags, with `wrapper.startReq(reqId)` for the end future (and
   `startReq("positions")` for the positions); it does not use `IB.reqAccountSummaryAsync`, which sends all tags including `$LEDGER:ALL` and never cancels, nor `IB.reqPositionsAsync`, which never cancels. A test
   checks the frame that `accounts.py` sends carries exactly the ten tags. The list is a constant of `accounts.py`; a change of it is a change of one file.
4. One account per gateway. The handler reads the accounts the gateway reports (`managedAccounts`); when there is not exactly one, the rpc fails `FAILED_PRECONDITION` with a fixed text and returns no value, and
   writes the log word `multiple_accounts` (added to `WORDS`; no count of accounts and no id is logged). A login with several accounts (an IRA beside a margin account, a linked or advisor structure) is
   therefore refused on every call while health still says connected; the runbook says how the owner sees it (the log line with that word, and the Go caller's `Permanent` error) and the recovery: a separate
   login for gateway B, or a later design that names one account by an alias held on the owner's machine. Only rows of the one account are returned, and every row is rebuilt from the typed fields, so the id the
   gateway reports is dropped and the alias is written. The handler does not read `wrapper.positions` or `wrapper.acctSummary`: those dicts are keyed by account id, persist across calls and a position closed
   between two calls is not re-sent. The handler collects its own rows per call with its own callbacks (or clears the wrapper dicts before the request), so a closed position cannot reappear
   (`test_second_positions_call_drops_closed_position`). A test feeds a fake whose rows carry an id-shaped string and checks that the serialized response contains none.
5. The allowlist of `transport.py` gains exactly four ids, as `ib_async` 2.1.0 sends them (`client.py`): 61 (`reqPositions`), 62 (`reqAccountSummary`), 63 (`cancelAccountSummary`) and 64 (`cancelPositions`).
   `ORDER_IDS` is unchanged and is still checked first; the account-update request (6), the executions request, the open-orders requests and every other id stay refused. The handler never subscribes for good:
   the rpc requests, reads until the end messages of the gateway, and cancels (63 and 64) in a `finally`, so no subscription outlives the call and the 2-subscription limit of account summaries is not held. Calls
   to one gateway are serialized by one lock. Each request goes through the pacer of that gateway like every other message (ADR 0003 lists account requests among the paced kinds) and ends after a deadline
   taken from the injected `Clock` (20 seconds, `DEADLINE_EXCEEDED`). `transport.py` is owner-only, so the entry is one pull request the owner merges. The existing test
   `test_guard.py::test_unknown_id_refused`, which lists 61 and 62 as refused with a comment naming them, is edited: 61 and 62 leave its tuple and the comment.
6. Which sockets and which services. A gateway is served on `ibkr-<name>.sock` when its `collect` holds any of the three words (today: `market_data` only). `SessionService` is on every socket.
   `ContractsService` and `MarketDataService` are on a socket only when `market_data` is in `collect`; `AccountService` only when `account_summary` or `positions` is. `build_server(gateway, clock=None, *, spec=None)`
   in `transport.py` gets the keyword `spec` (the gateway's `GatewaySpec`): with `spec=None` it registers today's three services and no `AccountService`, so every existing caller and test is unchanged; with a
   spec it registers the services by `collect`, and builds `AccountServicer` with the alias `spec.account` and the `collect` words from that spec. `server.py` passes `spec=` for each socket. The
   rule that the gateway on `ibkr.sock` has `market_data` is unchanged, so `ot run-once` is unaffected.
7. The log lines are unchanged in kind: `rpc_done` and `rpc_failed` with `rpc` set to `account` and `count` (the number of rows), through the whitelist of `transport.py` (`WORDS` gains `account` and
   `multiple_accounts`). No value, tag amount, symbol or alias is logged. The `ib_async` INFO line `position: ...` carries the account id; it stays harmless only because `configure_logging` sets the `ib_async`
   logger to WARNING and the record factory scrubs id-shaped words; tests pin both (acceptance of `pyside-account`). The scrub works on the shape of a word, so a login id of another shape would pass it; the
   response is built from typed fields, so only logs are at risk. The guard checks ids, not tags.
8. Go side: a client in the existing package `internal/broker/grpcbroker`, not a method on `broker.Broker`. `DialAccount(ctx, socketPath, broker.Options) (*AccountClient, error)` with `Account(ctx)` and
   `Close()`, plain types `Account`, `AccountValue` and `Position` (a `broker.Contract`, quantity and average cost as text) declared in the package, errors and defaults as `Dial` (`CallTimeout`, `Retryable`
   for `UNAVAILABLE` and `DEADLINE_EXCEEDED`). The connect loop of `Dial` becomes one unexported function both use. This is a new exported surface used across modules, though not an interface file by the
   letter of AGENTS.md (`grpcbroker` has none; `api.go` is not edited). `arch.toml` lets only `ot` import `grpcbroker`; `jobs` may import only `broker`, so no job may build on `grpcbroker` directly: the first
   consumer needs a port in `internal/broker/api.go`, which is the interface change of that later design, declared over this client. Alternative rejected: add the method to `broker.Broker`: that is an edit of
   `internal/broker/api.go`, a second interface entry, and it breaks every fake of `broker.Broker` in jobs and ot for a consumer that does not exist yet.
9. Nothing places orders and nothing here can. The guard refuses the order ids, the proto has no order message, the Go client has no such call, and the entries below say so in tests (the service has one
   rpc; the real `Client` with a recording stub connection sends only 61, 62, 63 and 64 for the rpc, and sends both cancels).
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

The interface is the proto file `contracts/proto/ibkr/v1/account.proto` of decisions 1 and 2, owned by the contracts module, with the Go and Python output generated from it and committed together
(`sh contracts/generate.sh`). The Go port of the broker module and the Python `api.py` of `ibkr` are not edited. The interface entry is `account-proto`; the pyside entry and the broker entry use the
generated types and need nothing else from each other. The contract is tested from both sides: Python by `test_contracts_api.py` (servicer, stub and `add_..._to_server` of the new service, a fake and a
caller), Go by `internal/contracts/account_proto_test.go` (the service descriptor lists exactly the one rpc; a fake server and a client compile against the generated code).

## Issues

Entry `account-proto` (interface) starts the day this design merges. Entries `pyside-account` and `go-account-client` each carry their own `Order:` line and need only `account-proto`; neither needs the
other's output. Entry `pyside-runbook-account` documents what `pyside-account` delivers and carries `Depends on: none`, with its order in the `Order:` line. What is seen end to end (gateway B logged in, the
service restarted with a gateway B file, a client dialing `ibkr-<name>.sock` and getting the alias, values and positions) needs all three code entries merged and is the owner's check, not a CI item and not a
test (CI decides only the proto items). Entries `account-proto` and `pyside-account` touch owner-only paths (`contracts/proto/`, the allowlist of `transport.py`): each carries the waiver lines and the owner
merges it.

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

1. `contracts/proto/ibkr/v1/account.proto` declares `package ibkr.v1`, imports `ibkr/v1/contract.proto`, and holds the service `AccountService` with exactly one rpc: `grep -c 'rpc ' contracts/proto/ibkr/v1/account.proto`
   prints 1 and its name is `Account`; no message, rpc or field identifier contains `Order`, `Execution`, `Cancel` or `Place` (checked on the declaration lines, not the comments: `grep -v '^ *//' contracts/proto/ibkr/v1/account.proto | grep -ci 'order\|execution\|cancel\|place'` prints 0).
2. The messages are those of decision 2 of docs/design/pyside-account-b.md with the field numbers shown, every message and field has a comment, and `quantity` and `avg_cost` are `string`; `buf lint` and
   `buf build` pass on `contracts/proto`.
3. The generated Go files `internal/contracts/genpb/account.pb.go` and `account_grpc.pb.go` and the four Python files are exactly the output of `sh contracts/generate.sh` (the generated-code check of `checks.yml`
   finds no difference); none was edited by hand.
4. `pyside/src/optdesk/contracts/api.py` re-exports `AccountServiceServicer`, `AccountServiceStub`, `add_AccountServiceServicer_to_server`, `AccountRequest`, `AccountResponse`, `AccountValue` and `Position`
   in `__all__`, holds no logic, and imports no other new name.
5. `pyside/tests/contracts/test_contracts_api.py` gains a test, `test_account_service_surface`, that checks from both sides with a fake servicer and a stub: the servicer base has exactly the method
   `Account`, a stub has the same, and a caller reads `account`, `generation` and the repeated rows of a response built by the fake; `cd pyside && uv run pytest tests/contracts -n auto` passes.
6. `internal/contracts/account_proto_test.go` has `TestAccountServiceHasOneReadRpc` (the descriptor of `AccountService` lists exactly `Account`, not streaming) and a test using a fake server on a Unix
   socket in `t.TempDir()` with `t.Parallel()`; `go test ./internal/contracts/... -count=1 -race` passes.
7. `ibkr.v1` of the other proto files is unchanged: `git diff --stat origin/main -- contracts/proto` lists only `account.proto`.
8. No file outside the ten listed is in the diff.
9. The result matches the sections Decisions (1 and 2) and Interface of docs/design/pyside-account-b.md, judged from the code and the design file.

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

1. `ALLOWED_IDS` in `pyside/src/optdesk/ibkr/transport.py` is `{1, 2, 9, 20, 25, 49, 59, 71, 78, 61, 62, 63, 64}` and `ORDER_IDS` is unchanged. In `pyside/tests/ibkr/test_guard.py` the existing
   `test_unknown_id_refused` is edited (61 and 62 leave its tuple `(61, 62, 5, 7, 6, 0)` and the comment "positions request 61, account summary request 62"); `test_account_ids_pass` sends frames 61, 62, 63 and 64
   through the real `Client` with the recording stub connection and all four are recorded, and `test_order_and_other_ids_still_refused` checks that 3, 4, 21, 58, 6 and 5 still raise (`OrderRefused` for the
   first four, `NotAllowed` for 6 and 5).
2. `pyside/src/optdesk/ibkr/accounts.py` (new, under 600 lines) holds the tag tuple of decision 3 of docs/design/pyside-account-b.md and the read. It calls `client.reqAccountSummary` with the ten tags itself (not
   `IB.reqAccountSummaryAsync`), sends each request through the pacer, waits for the end messages with a deadline from the injected `Clock`, and cancels (63 and 64) in a `finally`, also when the read fails or
   times out; `test_account_cancels_after_failure` and `test_account_cancel_after_timeout` in `pyside/tests/ibkr/test_accounts.py` use a fake `ib` and a fake clock and assert both cancel calls, and
   `test_summary_frame_carries_exactly_the_ten_tags` asserts the frame.
3. `test_accounts.py` `test_real_client_sends_only_account_ids`: a real `ib_async` `Client` with the guard installed and a recording stub connection, driven through the read, records frames whose ids are only
   61, 62, 63 and 64 (none of `ORDER_IDS`), and the recorded frames hold both cancels 63 and 64 (the cancel assertion is in this real-client test, not only in the fake-`ib` tests).
4. `model.py` declares a Protocol `AccountGateway` with the one async method and the typed rows it returns; the existing `Gateway` Protocol is unchanged, so no existing fake needs a new method
   (`git diff origin/main -- pyside/src/optdesk/ibkr/model.py` shows no removed line). `ibgateway.py` implements the method on `IbGateway` through `accounts.py`, using the session and generation of the
   connected client, with one lock (`test_reads_are_serialized`).
5. Account handling: with a fake whose `managedAccounts` has two entries, the rpc fails `FAILED_PRECONDITION`, returns no value and writes the log word `multiple_accounts` and no count or id
   (`test_two_accounts_refused`); with one entry, the response `account` is the alias of the spec, `generation` is the session generation, only the fixed tags are returned, and a fake row carrying an
   id-shaped string leaves no such string in `response.SerializeToString()` (`test_alias_replaces_id`); the id-shaped test string is built at run time from two pieces so no file carries it. A position
   present in a first call and closed before the second is absent from the second answer (`test_second_positions_call_drops_closed_position`). Numbers are decimal text with no exponent
   (`test_quantity_is_decimal_text`).
6. `AccountServicer` (in `transport.py`, built on the shared call helper) answers `Account` over a real `grpc.aio` server on a socket in a temporary directory; an rpc to a gateway whose `collect` lacks both account
   words is not served (`test_account_unimplemented_without_words`), and a gateway with only `account_summary` answers with an empty `positions` (`test_positions_empty_when_not_collected`); the log records of
   the rpc hold only whitelisted fields and words and no value, tag, symbol or alias (`test_account_log_has_no_values`); `WORDS` gains `account` and `multiple_accounts` and nothing else.
7. Log safety of `ib_async`: `test_ib_async_position_log_cannot_carry_an_id` feeds a real wrapper `position()` call and checks that the `ib_async` logger level set by `configure_logging` is above INFO and that
   the scrub regex of the record factory removes an id-shaped string (built at run time from two pieces) from every formatted record.
8. `server.py` and `build_server`: `build_server(gateway, clock=None, *, spec=None)` as in decision 6; `_socket_paths` returns the named socket for a gateway whose `collect` holds any of the three words;
   `ContractsService` and `MarketDataService` are registered only with `market_data`, `AccountService` only with `account_summary` or `positions`, and `SessionService` always; with `spec=None` the three
   services of today are registered. In `pyside/tests/ibkr/test_server.py`, `test_account_only_gateway_has_a_socket` (a gateway with the two account words and no `market_data` has `ibkr-<name>.sock` and
   answers the account rpc with a fake, and `Snapshot` on it is `UNIMPLEMENTED`) and `test_default_socket_still_needs_market_data` pass.
9. `pyside/examples/gateways.example.toml` keeps its placeholders (no real port, alias or id), its second gateway lists `collect = ["account_summary", "positions"]`, and `pyside/tests/ibkr/test_gateways.py`
   still loads it (`cd pyside && uv run pytest tests/ibkr/test_gateways.py -n auto` passes).
10. Every test runs under `pytest -n auto` (a temporary directory for every socket, `kit.sockdir`), a fake clock from `pyside/tests/kit/fakeclock.py` and no sleep; none connects to a
    broker. `cd pyside && uv sync --locked && uv run ruff check && uv run mypy --strict && uv run pytest -n auto` passes.
11. No file outside the nine listed is in the diff; `pyside/pyproject.toml` and `pyside/uv.lock` are unchanged; no new source file is over 600 lines and `transport.py`, `ibgateway.py` stay under 800.
12. The result matches the section Decisions (points 3 to 7 and 9 to 10) of docs/design/pyside-account-b.md, judged from the code and the design file.

### go-account-client

Module: broker.

Files: internal/broker/grpcbroker/account.go, internal/broker/grpcbroker/dial.go, internal/broker/grpcbroker/account_test.go, docs/contracts/broker.md

Design: docs/design/pyside-account-b.md#decisions

Depends on: account-proto (the entry marked (interface)).

Order: after account-proto. It shares no file with `pyside-account` and needs none of its output.

Acceptance:

1. `internal/broker/grpcbroker/account.go` declares `DialAccount(ctx, socketPath, broker.Options) (*AccountClient, error)`, `(*AccountClient).Account` and `Close`, and the types `Account`, `AccountValue`
   and `Position`; `grep -c 'broker.Broker' internal/broker/grpcbroker/account.go` prints 0 and `git diff --stat origin/main -- internal/broker/api.go` prints nothing.
2. `dial.go` keeps `Dial` and `var _ broker.DialFunc = Dial` and moves the connect loop into one unexported function that `Dial` and `DialAccount` both call; the existing tests of the package still pass.
3. `account_test.go` uses a fake `AccountService` on a Unix socket in `t.TempDir()` with `t.Parallel()`: `TestAccountValues` (alias, generation, rows in order, a currency kept), `TestAccountPositions` (a
   position's `broker.Contract` fields, quantity and average cost as received text), `TestAccountErrorKinds` (`UNAVAILABLE` and `DEADLINE_EXCEEDED` are `Retryable`, `FAILED_PRECONDITION` and `INVALID_ARGUMENT`
   `Permanent`; the error text holds no socket path), `TestAccountCallTimeout` (a fake that blocks gives `DEADLINE_EXCEEDED` after a short `CallTimeout`, without a sleep in the test: the fake waits on the
   context) and `TestDialAccountRefuses` (an unreachable socket gives `Retryable`).
4. A response with no alias or a negative generation is a `Permanent` error (`TestAccountEmptyAliasRefused`); the client keeps no state between calls and does not retry inside a call.
5. `docs/contracts/broker.md` replaces the sentence "No type or call exists for orders, positions or accounts." by one that says no type or call exists for orders, and describes `DialAccount`, the one call, the
   plain types, the alias rule (the answer carries the alias, the client never holds an account id), that `grpcbroker` is imported only by `ot` and a job needs a port in `api.go` first, and the contract tests by name in
   the contract-tests line (the last line of the file today), which gains the new test names.
6. `go vet ./internal/broker/...` and `go test ./internal/broker/... -count=1 -race` pass; `go run ./scripts/arch --check` prints no difference (no new package).
7. No file outside the four listed is in the diff, and each new file is under 600 lines.
8. The result matches the section Decisions (points 8 and 9) of docs/design/pyside-account-b.md, judged from the code and the design file.

### pyside-runbook-account

Module: docs.

Files: docs/deploy/pyside.md

Design: docs/design/pyside-account-b.md#decisions

Depends on: none.

Order: after pyside-account is merged (it documents what that entry delivers; no shared file, so the order is prose and the dispatcher starts this issue after that merge).

Acceptance:

1. The section `## Sockets` of `docs/deploy/pyside.md` says that a gateway is served on `ibkr-<name>.sock` when its `collect` holds any of `market_data`, `account_summary` and `positions`, replaces the sentence
   that a gateway without `market_data` has no socket, and says that `ibkr.sock` still belongs to a gateway with `market_data`.
2. A new section `## Account data` says: which rpc (`Account`), that the answer carries the alias and never an account id, that exactly one account per gateway is served and a login with several accounts is
   refused, that nothing is subscribed beyond a call, that the four ids 61 to 64 are the only additions to the allowlist and orders stay refused, and that the file with gateway B stays on the owner's machine.
3. The same section tells the owner how a refused multi-account login shows and how to recover: the log line carries the word `multiple_accounts` (no count and no id), the Go caller sees a `Permanent`
   error while the health URL still says connected, and the recovery is a separate login for gateway B or a later design that names one account by an alias held on the owner's machine.
4. `grep -c '[A-Za-z]\{1,\}[0-9]\{5,9\}' docs/deploy/pyside.md` prints the same count as on origin/main, and the file contains no port number, host address or account id.
5. No file outside `docs/deploy/pyside.md` is in the diff.
6. The result matches the section Decisions (points 4, 6 and 10) of docs/design/pyside-account-b.md, judged from the file and the design file.

## Left out

- A consumer: the job that calls the new client on a schedule and stores an account snapshot (the dataset, the `ot` command and its manifest), and the page that shows it. Each is a design of its own over this
  client; this design delivers the data path from gateway B to a Go caller and nothing that stores or shows, and that path is not seen working by CI (the owner's end-to-end check).
- A port `broker.AccountReader` in `internal/broker/api.go`: declared by the design of the first consumer (decision 8); until then no job may use `grpcbroker` (`arch.toml`).
- A stamp for how old an answer is (`received_at`): the caller stamps with its own clock when it stores.
- Account figures beyond the ten tags (margin by currency, `$LEDGER` segments), a tag chosen by the caller, a login with several accounts (an advisor structure; refused with the word `multiple_accounts`),
  executions, open orders, trades, and the account-update stream: each needs its own allowlist id or a proto change and is refused today.
- Permanent choices of `ibkr.v1` taken here, to be changed only by a new version: the field numbers of `AccountResponse`; the rule that `account` is the alias, which fixes one account per socket (the socket
  per gateway is the only selector, with no gateway field); the ten tags fixed in code, so a `$LEDGER` or currency tag later is a proto change; `quantity` and `avg_cost` as `string`, which cannot become a typed
  decimal without a v2; and the exported surface of `grpcbroker` named in decision 8.
- Fallback from gateway A to gateway B inside pyside: unchanged from the earlier design; a caller dials the socket it wants.
- Account figures for gateway A: the same service serves them when its entry lists the words; no extra work, and no decision to collect them is taken here.
- A change of `docs/adr/0003-market-data-architecture.md`: it names "a later issue may add them to the allowlist"; this design is that issue, and the ADR text stays the owner's to amend.
- Splitting the pyside entry in an allowlist entry and a handler entry: the handler is untestable against the real guard without the ids, the owner reviews the whole security surface in one pull request, and the
  two would share `transport.py`.

## Adversarial

Answer of the adversarial pass: `pyside-account-b.adversarial.md` in the dispatcher's scratchpad. One line per question.

- (a) Orders and id leak: adopted; no order path, but the guard checks ids not tags, so decision 3 and acceptance 2 pin the ten tags in the frame, acceptance 7 pins the `ib_async` `position:` log, and the test id is built at run time.
- (b) Ids 61 to 64: adopted; verified against `ib_async` 2.1.0, cited in decision 5, and `accounts.py` calls `reqAccountSummary` itself with fixed tags since the library helper sends all tags and never cancels.
- (c) Multi-account login: adopted; refusal kept with the fixed log word `multiple_accounts` (no count, no id) and the runbook says how the owner sees and recovers it (decision 4, runbook acceptance 3).
- (d) Cross-module surface: adopted; decision 8 names it as a new exported surface and that only `ot` may import `grpcbroker`.
- 1 Gap against the goal: adopted; the multi-account word and runbook line close the silent case, and the end-to-end check is stated as the owner's, not CI.
- 2 Stale wrapper state: adopted; the handler collects its own rows per call and a test covers a closed position (decision 4, acceptance 5).
- 3 Simpler way: adopted; collapsed to one rpc `Account` (decision 1), no code reason for two.
- 4 Lock-in: adopted; recorded in `## Left out`.
- 5 False claims: adopted; `build_server` gets `spec=None` (decision 6), the broker.md version-line claim is dropped, and the pyside entry names the edit of `test_guard.py::test_unknown_id_refused`.
- 6 Slips: adopted; `pyside-runbook-account` has `Depends on: none` with the order in `Order:`, `AccountServicer` gets the alias and `collect` from `spec`, and the cancel assertion is in the real-client test.
