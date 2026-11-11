# FX cash: the SecType CASH mapping that lets fx_refresh resolve USD.JPY

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-11
Revision: 4

Source: the `fx_refresh` job failed 3 of 3 runs. The owner asked to fix the FX contract type. The job itself is already designed (`docs/design/market-bars.md` Decision 9) and stays as it is; only the contract type it sends is unmapped.
Read on origin/main at 9a95c180: `AGENTS.md`, `docs/design/modules.md`, `docs/design/market-bars.md` (Decision 9), `internal/jobs/fxrefresh/run.go`, `internal/broker/grpcbroker/{convert,history}.go` and `convert_test.go`, `contracts/proto/ibkr/v1/contract.proto`, `pyside/src/optdesk/ibkr/{transport,ibgateway,history,spec}.py`, `pyside/src/optdesk/contracts/api.py`, `pyside/tests/contracts/test_contracts_api.py`, `pyside/tests/live/test_live_pyside.py`.

## Requirement

`fx_refresh` reaches IB: a `CASH` contract (IDEALPRO, symbol USD, currency JPY) resolves, and its daily MIDPOINT bars come back, through the Go broker client and the pyside service.

## What main holds today

- `internal/jobs/fxrefresh/run.go` (`runPair`) calls `ResolveContract` with `SecType: "CASH"`, `Symbol: "USD"`, `Exchange: "IDEALPRO"`, `Currency: "JPY"`, then `HistoryClient.Bars` with `Kind: broker.BarMidpoint` on the resolved contract. The values are right; the job needs no change.
- `internal/broker/grpcbroker/convert.go` `secTypeText` maps only STK, IND and OPT (`secTypeOfText` is its inversion), so `toSpec` returns Permanent "unknown sec type" and no call leaves the process. `history.go` converts the bars contract with the same `toSpec`.
- `contracts/proto/ibkr/v1/contract.proto` `SecType` has UNSPECIFIED, STOCK, INDEX, OPTION (0 to 3).
- `pyside/src/optdesk/ibkr/transport.py`: `_SEC_TYPES` has the same three words (its inverse `_SEC_TYPE_VALUES` follows it). `ResolveContract` refuses a spec whose `sec_type` is not in the map ("contract needs a type and a symbol"); `Bars` builds its `HistoryKey` with `_SEC_TYPES.get(c.sec_type, "")`, so a CASH spec would reach `ib_async` with an empty `secType`. `_bars_problem` needs a `con_id` above 0, which the resolve gives.
- `ibgateway.py` `resolve` builds `Contract(secType=sec_type, symbol, exchange, currency, ...)` from the text, and `bars` builds `Contract(conId, symbol, secType, exchange or "SMART", currency)`: both take the text as it comes, so `CASH` with exchange `IDEALPRO` needs no new code there. The resolve answer copies the request and takes only `con_id`, `exchange`, `trading_class`, `multiplier` from IB.
- `history.py` `_WHAT_TO_SHOW` already maps `midpoint` to `MIDPOINT`, and `transport.py` `_BAR_KINDS` maps `BAR_KIND_MIDPOINT` to `midpoint`: no change. `reqHistoricalDataAsync` is called with `useRTH` True for every contract; no STK, IND or OPT assumption exists elsewhere in `ibkr/` (no contract cache; the only sec-type tests are the `STK/IND` check of the chains rpc and `OPT` of the resolve, neither touches CASH).

## Decisions

1. `SEC_TYPE_CASH = 4` is added to `SecType` in `contract.proto` with its generated Go and Python code (entry 1). One value, no new enum or message: a pair is the existing `symbol` (base) and `currency` (quote).
2. `grpcbroker` maps `CASH` both ways, by adding it to `secTypeText` (entry 3); `secTypeOfText` follows by inversion.
3. pyside maps `SEC_TYPE_CASH` to the text `CASH` in `transport.py` `_SEC_TYPES` (entry 4). That one map serves the resolve path and the bars path, and `ibgateway.py` turns the text into an `ib_async` `Contract` with `secType="CASH"` and the exchange `IDEALPRO` the caller sends, so both paths work from this one line. `optdesk.contracts.api` exports the new name first (entry 2), because `transport.py` imports contracts only through that file. Files not edited, on purpose: `ibgateway.py`, `history.py`, `spec.py` (checked above). Known unverified point: `useRTH` is True for the bars call; IDEALPRO trades around the clock, and the live test of Decision 4 shows whether IB returns daily MIDPOINT bars with it. If it returns none, a later design adds a flag; this design does not.
4. A read-only live test `pyside/tests/live/test_live_cash.py` (skipped unless `OT_LIVE_PYSIDE=1`, the deployed socket of the gateway on port 4001 only, no order and no write rpc) resolves USD.JPY and pulls a few MIDPOINT bars, asserting a `con_id` above 0, at least one bar and a positive close. The window runs it once after the deploy.
5. No change to `internal/jobs/fxrefresh`: Decision 9 of `market-bars.md` stays as it is (daily MIDPOINT bar, 5-day backfill, source `ibkr_midpoint`), nothing is superseded, and no job entry exists.

## Modules

No row of `docs/design/modules.md` changes. Entries 1 and 2 are module contracts, 3 broker, 4 pyside; each stays in one row. `contracts/proto/` and `pyside/src/optdesk/ibkr/transport.py` are owner-only paths: the owner merges entries 1 and 4.

## Interface

- Proto (owner merges): `enum SecType` in `contracts/proto/ibkr/v1/contract.proto` gets `SEC_TYPE_CASH = 4;` with the comment "a cash pair of IDEALPRO: symbol is the base currency, currency the quote currency". The generated Go (`internal/contracts/genpb`, `pb.SecType_SEC_TYPE_CASH`) and Python (`pyside/src/optdesk/contracts/genpb/**`) come with it in the same pull request.
- Go: `internal/broker/grpcbroker/convert.go` `secTypeText` gets `pb.SecType_SEC_TYPE_CASH: "CASH"`. `broker.Contract.SecType` is a string and stays one.
- Python: `optdesk.contracts.api` imports `SEC_TYPE_CASH` from the generated `contract_pb2` and lists it in `__all__`; `transport.py` `_SEC_TYPES` gets `pb.SEC_TYPE_CASH: "CASH"`.

## Issues

### 1 sec-type-cash-proto (interface)

Module: contracts. Size: S.
Files: `contracts/proto/ibkr/v1/contract.proto`, `internal/contracts/genpb/**`, `pyside/src/optdesk/contracts/genpb/**`
Design: docs/design/fx-cash.md#Interface
Order: first.
Sequence: the pull request is the owner's merge (`contracts/proto`); entries 2, 3 and 4 follow it.
Goal: `SEC_TYPE_CASH = 4` exists in `SecType` with its comment, and in the generated Go and Python output.
Regenerate: Go with `buf generate` at the repository root (`buf.gen.yaml` has the Go plugins only, output `internal/contracts/genpb`). Python with, from `pyside/`: `uv run python -m grpc_tools.protoc -I ../contracts/proto --python_out=src/optdesk/contracts/genpb --mypy_out=src/optdesk/contracts/genpb --grpc_python_out=src/optdesk/contracts/genpb --mypy_grpc_out=src/optdesk/contracts/genpb ibkr/v1/contract.proto` (`grpcio-tools` and `mypy-protobuf` are in the dev group of `pyside/pyproject.toml`; no script or Makefile holds it, so this design names it; run on the unchanged `contract.proto` it reproduces the committed `contract_pb2.py` and `contract_pb2.pyi` byte for byte, checked 2026-11-11).
Acceptance:
1. `grep -c "SEC_TYPE_CASH = 4" contracts/proto/ibkr/v1/contract.proto` prints 1.
2. `grep -rl "SEC_TYPE_CASH" internal/contracts/genpb | wc -l` prints at least 1.
3. `grep -rl "SEC_TYPE_CASH" pyside/src/optdesk/contracts/genpb | wc -l` prints at least 1.
4. `git diff $(git merge-base HEAD origin/main) -- contracts/proto | grep -c "^-[^-]"` prints 0 (no existing value changes).
Checked by CI: ci-go, ci-python, the buf lint and generated-drift check; `Contracts: changed (proto)`.

### 2 pyside-contracts-sec-type-cash (interface)

Module: contracts. Size: S.
Files: `pyside/src/optdesk/contracts/api.py`, `pyside/tests/contracts/test_contracts_api.py`
Design: docs/design/fx-cash.md#Interface
Order: after 1.
Sequence: the generated `contract_pb2` holds the name (entry 1); entry 4 follows this one.
Goal: `optdesk.contracts.api` exports `SEC_TYPE_CASH`, and the contract test asserts its value.
Acceptance:
1. `grep -c "SEC_TYPE_CASH" pyside/src/optdesk/contracts/api.py` prints 2 (the import and `__all__`).
2. `grep -c "SEC_TYPE_CASH == 4" pyside/tests/contracts/test_contracts_api.py` prints 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e pyside/src/optdesk/contracts/api.py -e pyside/tests/contracts/test_contracts_api.py` prints nothing.
Checked by CI: ci-python (`uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`); `Contracts: unchanged`.

### 3 broker-cash-sec-type

Module: broker. Size: S.
Files: `internal/broker/grpcbroker/convert.go`, `internal/broker/grpcbroker/convert_test.go`
Design: docs/design/fx-cash.md#Decisions
Order: after 1.
Goal: a `broker.Contract` with `SecType` `CASH` converts to `SEC_TYPE_CASH` and back, so `ResolveContract` and `Bars` of the USD.JPY contract reach the service instead of failing Permanent in `toSpec`.
Acceptance:
1. `grep -c "SEC_TYPE_CASH" internal/broker/grpcbroker/convert.go` prints 1 and `grep -c '"CASH"' internal/broker/grpcbroker/convert.go` prints 1.
2. `grep -c "CASH" internal/broker/grpcbroker/convert_test.go` prints at least 2 (the round trip of a contract with symbol USD, exchange IDEALPRO, currency JPY, and the conversion back of the proto value), and `grep -c "unknown sec type" internal/broker/grpcbroker/convert_test.go` prints at least 1 (an unmapped text is still refused; the string is not in that test today, so this issue's test adds it).
3. `git diff $(git merge-base HEAD origin/main) -- internal/broker/grpcbroker/convert.go | grep -c "^-[^-]"` prints 0 (no existing mapping changes).
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e internal/broker/grpcbroker/convert.go -e internal/broker/grpcbroker/convert_test.go` prints nothing.
Checked by CI: ci-go; `go test ./internal/broker/... -race -count=1`.

### 4 pyside-transport-cash

Module: pyside. Size: S.
Files: `pyside/src/optdesk/ibkr/transport.py`, `pyside/tests/ibkr/test_transport_cash.py`, `pyside/tests/live/test_live_cash.py`
Design: docs/design/fx-cash.md#Decisions
Order: after 1, 2.
Sequence: `transport.py` is owner-only, so the owner merges this pull request; it follows entries 1 and 2.
Goal: a `ContractSpec` with `SEC_TYPE_CASH` reaches the gateway as `secType` `CASH` for the resolve and for the bars request (so `ib_async` gets `Contract(secType="CASH", symbol="USD", exchange="IDEALPRO", currency="JPY")`), a resolve answer of a CASH contract carries `SEC_TYPE_CASH` back, and a read-only live test of USD.JPY exists for the window to run.
Acceptance:
1. `grep -c "SEC_TYPE_CASH" pyside/src/optdesk/ibkr/transport.py` prints 1 and `grep -c '"CASH"' pyside/src/optdesk/ibkr/transport.py` prints 1.
2. `git diff $(git merge-base HEAD origin/main) -- pyside/src/optdesk/ibkr/transport.py | grep -c "^-[^-]"` prints 0.
3. `grep -c "SEC_TYPE_CASH" pyside/tests/ibkr/test_transport_cash.py` prints at least 2 (the resolve path with its answer, and the bars path, over the fake gateway of `pyside/tests/ibkr`), and `grep -c "sleep\|connect(" pyside/tests/ibkr/test_transport_cash.py` prints 0.
4. `grep -c "OT_LIVE_PYSIDE\|skip" pyside/tests/live/test_live_cash.py` prints at least 1, `grep -c "MIDPOINT\|midpoint" pyside/tests/live/test_live_cash.py` prints at least 1, `grep -c "4001" pyside/tests/live/test_live_cash.py` prints 0 (the socket comes from the existing live client, not a literal port), and `grep -ci "order\|place" pyside/tests/live/test_live_cash.py` prints 0 (read-only; follows `test_live_pyside.py`).
5. `grep -c "wife_client\|default_client" pyside/tests/live/test_live_cash.py` prints 0 and `grep -c "main_client" pyside/tests/live/test_live_cash.py` prints at least 1 (the fixtures of `pyside/tests/live/conftest.py` are `default_client`, `main_client` and `wife_client`; only `main_client`, the gateway on 4001, is used).
6. The live test asserts at least one MIDPOINT bar: `grep -c "len(.*bars.*) *>= *1\|len(.*bars.*) *> *0\|assert .*bars" pyside/tests/live/test_live_cash.py` prints at least 1. Zero bars is a failure of the test, not a pass: `internal/jobs/fxrefresh/run.go` treats an empty answer as success (returns 0 rows and no error), so this test is the only place that makes it visible.
7. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e pyside/src/optdesk/ibkr/transport.py -e pyside/tests/ibkr/test_transport_cash.py -e pyside/tests/live/test_live_cash.py` prints nothing.
Checked by CI: ci-python (`uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`); the live test is skipped there. Run once by the window after the deploy: `OT_LIVE_PYSIDE=1`, read-only.

## Checked by CI

Every test run named above runs in CI, not in acceptance. No test connects to a live broker; tests use the fake gateway and the fake clock, `tmp_path` and `t.TempDir()`, in parallel.

## After deploy

The window runs the live test (`OT_LIVE_PYSIDE=1`, entry 4), then checks two things: the first `fx_refresh` run after the deploy is a success, AND the stored USD/JPY row count rose. A success with zero new rows is not a fix: `fxrefresh/run.go` returns `(0, nil)` when IB sends no bars, and `useRTH` True for CASH (`ibgateway.py`) is unverified. Zero rows means the fix is not done, and the `useRTH` design follows at once (a flag on the bars rpc), without waiting for another run.

The job asks IB for `duration_days = 5`, so it refills only the last five days. Days of the failed period older than that are not recovered; no one-off backfill is planned (see Left out).

## Left out

- A live quote instead of the daily bar (option-trader style): a later design if a figure needs a decision-grade rate.
- MYR via FRED `DEXMAUS` through `fred_series`: IDEALPRO has no USD.MYR, and nothing reads MYR.
- Other pairs and any FX position or order: no cash order is placed.
- Zero bars treated as success in `internal/jobs/fxrefresh/run.go`: it returns `(0, nil)` for an empty answer, so a silent failure looks like a success. Not changed here because Decision 5 keeps the job as it is; the live test (entry 4, item 6) and the row-count check after deploy make it visible instead.
- A one-off backfill of the days of the failed period older than five days: none is planned; the daily job refills the last five days only.
- A `useRTH` flag for the bars rpc: only if the live run shows IB returns no daily MIDPOINT bars for CASH.

## Adversarial

1. Why not the live snapshot with a validity rule: scope creep (the owner asked for the contract type, and the daily bar design already works once CASH maps); its checks were weak (the data type defaults to 1 in `ib_async`, the identity check echoes the request, the age rests on a tick time that may be nil); and it lost the 5-day backfill that makes a missed run recoverable. The bar design stays.
2. Gap against the goal: only the mapping is claimed; whether IB returns bars for CASH with `useRTH` True is unproven until the live test (Decision 3 and Left out), which is why the window runs it once after the deploy.
3. Identity of the resolved contract: pyside echoes the request for `sec_type`, `symbol` and `currency`, so only `con_id` and `exchange` are IB's answer; `run.go` already refuses a not-listed answer or a zero `ConID`, and this design adds no second check.
4. Simpler way (FRED `DEXJPUS` instead of IB): not adopted, it is a noon reference rate and the job and its table exist for the IB midpoint.
5. Slip risk (owner-only chain): entries 1 and 4 need the owner's merge, so entry 3 can land first only after 1; the chain is stated in the Order lines, and the job needs no entry, so nothing waits on a fourth party.
6. Wrong sec-type word reaching IB: `transport.py` falls back to an empty text for an unmapped type, which IB would reject; entry 4's acceptance 3 covers both paths with a CASH spec, so an unmapped path fails in CI.
