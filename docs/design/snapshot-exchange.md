# Snapshot exchange: carry the requested exchange through Gateway.quotes so JP contracts get quotes

Author: the optdesk dispatcher (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 1

Source: the owner's wish for subscribed JP option data (2026-11-09) and the live probe of 2026-11-09 (kept as lines, the probe directory is not in the repository).
Probe: `MarketDataService.Snapshot` for conId 14087999 (the N225 index) answers `QUOTE_STATUS_ERROR`, `errorCode 200`, "No security definition has been found for the request";
`ResolveContract` for the N225 option 20261112 / 50000 / call answers `RESOLVE_STATUS_FOUND` with `exchange OSE.JPN`, `currency JPY`, `trading_class 225`, conId 831054033, and chains list.
Read on main at acaea12: `pyside/src/optdesk/ibkr/ibgateway.py` (line 372 builds `Contract(conId=con_id, exchange="SMART")`), `transport.py` (`MarketDataServicer.Snapshot` keeps only
`[c.con_id for c in request.contracts]`), `model.py` (`Gateway.quotes(kind, con_ids, on_quote)`), `contracts/proto/ibkr/v1/contract.proto` (`ContractSpec` has `exchange`, `currency`,
`trading_class`), `docs/design/pyside-ibkr-transport.md`, `docs/design/modules.md`, `AGENTS.md`. Closed issue #458 added the SMART default and said that carrying the exchange is a separate
interface change; this is that change.

## What main holds today

- The request already carries what is needed: `SnapshotRequest.contracts` is a list of `ContractSpec`, and the Go caller fills the spec it got from `ResolveContract`.
  The server drops everything but `con_id` at the first line of `Snapshot`.
- `Gateway.quotes(kind, con_ids, on_quote)` in `model.py` takes `Sequence[int]`. `IbGateway._quote` builds `Contract(conId=con_id, exchange="SMART")`. SMART routes US
  contracts; IB has no security definition for a JP conId on SMART, hence error 200.
- The Gateway Protocol has three implementations or stand-ins: `IbGateway` (`ibgateway.py`), the fake `Gateway` of `tests/ibkr/test_seams.py:44` and the fake of `tests/ibkr/test_transport.py:81`.
  `tests/ibkr/test_ibgateway.py` calls `gateway.quotes` directly (lines 244, 405, 462). No other `def quotes` exists in `pyside/`.
- `QuoteRow` carries `con_id` and stays unchanged; the Go side matches rows to contracts by `con_id`.

## Decisions

1. A small frozen record `QuoteTarget(con_id: int, exchange: str = "", currency: str = "", trading_class: str = "", sec_type: str = "")` in `model.py` (`sec_type` is the TWS word, "OPT", "IND", "STK"). Empty means "not given". It is a record, not four
   parallel lists, so a later field (a primary exchange) is one more default field.
2. `Gateway.quotes(kind, targets: Sequence[QuoteTarget], on_quote)` replaces `con_ids`. The call is not kept in two forms: both callers and all three implementations move in one issue,
   because a Protocol with two shapes would fail `mypy --strict` on whichever side lags.
3. `IbGateway._quote` builds `Contract(conId=t.con_id, exchange=t.exchange or "SMART")` and sets `currency`, `tradingClass` and `secType` only when non-empty. A target with all three empty builds exactly
   the contract of today, so US requests are byte-for-byte unchanged.
4. `MarketDataServicer.Snapshot` builds one `QuoteTarget` per `ContractSpec` (`con_id`, `exchange`, `currency`, `trading_class`, and `sec_type` mapped from `ContractSpec.sec_type` with the mapping `resolve` already uses; unspecified is empty) and keeps the existing check that every contract carries a
   `con_id > 0`. No new rejection: an exchange the gateway does not know comes back as the row's own error status (code 200 or the like), as it does today.
5. Rows stay keyed by `con_id`. Two targets with the same `con_id` and different exchanges in one request are not supported (the `_Job` bookkeeping keys on `con_id`); no caller sends that, and the
   issue does not add a check for it.
6. `transport.py` is an owner-only path (`docs/design/modules.md`, pyside row): the pull request of the issue needs the owner's merge. The issue says so and does not split the change.

## Modules

No row of `docs/design/modules.md` changes and no module is added. All paths are in the pyside row (`pyside/**`). `go run ./scripts/arch --check` is not affected.

## Interface

Owning module: pyside. Contract: unchanged (no file under `contracts/schemas`, `contracts/datasets` or `contracts/proto` changes; `ContractSpec` already has the fields).

| Name | File | Change |
|---|---|---|
| `QuoteTarget` | `pyside/src/optdesk/ibkr/model.py` | new frozen dataclass: `con_id: int`, `exchange: str = ""`, `currency: str = ""`, `trading_class: str = ""`, `sec_type: str = ""` |
| `Gateway.quotes` | `pyside/src/optdesk/ibkr/model.py` | `quotes(self, kind: str, targets: Sequence[QuoteTarget], on_quote: Callable[[QuoteRow], Awaitable[None]]) -> None`; docstring: SMART when `exchange` is empty |

`pyside/src/optdesk/ibkr/api.py` (the `Session` probe) is not touched; `Gateway` is the internal seam between the servicers and `IbGateway`, tested from both sides by `tests/ibkr/test_seams.py`.

## Issues

### quotes-exchange (interface)

- Module: pyside. Size: S. Depends on: none.
- Order: first.
- Design: docs/design/snapshot-exchange.md#Interface (interface change of the `Gateway` Protocol with all its implementations; `transport.py` is owner-only, the owner merges)
- Files: pyside/src/optdesk/ibkr/model.py, pyside/src/optdesk/ibkr/ibgateway.py, pyside/src/optdesk/ibkr/transport.py, pyside/tests/ibkr/test_ibgateway.py, pyside/tests/ibkr/test_seams.py, pyside/tests/ibkr/test_transport.py

- Goal: the Interface table; `IbGateway` builds the contract from the target; `Snapshot` passes the spec fields; the two fakes and the direct callers move to `QuoteTarget`.

Acceptance (all offline; a fake IB client, no live gateway, no sleep, `tmp_path` for any file):

1. `grep -c "class QuoteTarget" pyside/src/optdesk/ibkr/model.py` prints 1; `grep -rn "con_ids" pyside/src/optdesk/ibkr/model.py pyside/src/optdesk/ibkr/ibgateway.py` prints no `quotes` parameter.
2. In `tests/ibkr/test_ibgateway.py` a test calls `gateway.quotes("snapshot", [QuoteTarget(7, "OSE.JPN", "JPY", "225")], keep)` and asserts that the fake IB client's `reqMktData` received a
   `Contract` with `conId == 7`, `exchange == "OSE.JPN"`, `currency == "JPY"`, `tradingClass == "225"` and, for a target with `sec_type="OPT"`, `secType == "OPT"`.
3. A test with `QuoteTarget(5)` asserts the contract has `exchange == "SMART"` and empty `currency` and `tradingClass`, the contract of today (the US case is unchanged).
4. In `tests/ibkr/test_transport.py` a test sends a `SnapshotRequest` whose `ContractSpec` carries `con_id`, `exchange`, `currency`, `trading_class` and asserts the fake gateway received the
   same `QuoteTarget`; the existing test that a `con_id <= 0` is `INVALID_ARGUMENT` still passes.
5. `tests/ibkr/test_seams.py` and the fake in `test_transport.py` implement the new signature; `cd pyside && uv run ruff check && uv run mypy --strict && uv run pytest -n auto` passes.
6. `git diff --name-only $(git merge-base HEAD origin/main)` lists only paths of the Files line, and no file under `contracts/`.

## Left out

- The JP option capture job (which JP contracts, which expiries, when to run, the store dataset for JPY quotes) is a later design; this one only lets a JP `Snapshot` reach the gateway with an exchange.
- Unproven by the probe: the failing snapshot was the N225 index, not the option, so exchange (and secType) forwarding is necessary but not shown sufficient. Before the capture job design, the owner runs one live `ot` snapshot of conId 831054033 (and a hand `reqMktData` if it fails); a failure there is a second small design, not a reason to hold this one.
- Error routing in `IbGateway._on_error` matches on `conId` only (`ibgateway.py:187-190`); a wrong exchange may surface as a timeout rather than error 200. Left as is; the live check will show it.
- Checking that the owner's IBKR account is subscribed to OSE and TSE data. A live `ot` snapshot of conId 831054033 after the merge is the owner's check; no test depends on it.
- A primary exchange, the underlying index (`SEC_TYPE_INDEX`) quirks (an index may need `secType` and `exchange` rather than a bare conId), and a duplicate-conId check, until a real request needs them.
- Chain and resolve behavior: they already work for JP (probe).

## Adversarial

Review file: dispatcher scratchpad `snapex-adversarial.md` (not in the repository). Answers:

1. Gap, the probe was an index and the live check is deferred: adopted as a statement (Left out: forwarding is necessary, not shown sufficient; the owner's live snapshot is named); `sec_type` added to `QuoteTarget` (adopted). Not adopted: a gating live test, since AGENTS.md forbids live tests and the owner runs it. Market data type and subscription: left out, named.
2. `_on_error` keyed on conId, wrong exchange may time out: adopted as a named risk in Left out, not fixed here (a separate change of error routing, no evidence yet).
3. Simpler way, optional kwarg or a resolve cache: not adopted. A `Mapping` beside `con_ids` is a parallel structure that drifts, and a cache is hidden state that fails after a restart; the caller already sends the spec. The hand probe first: adopted as the owner's check.
4. Lock-in, a third copy of the spec and con_id keying: adopted as stated (Decision 5); the record is the minimum and takes one default field per addition.
5. False claims: the SMART cause rests on an index probe, adopted (Left out says so); same conId with different exchanges unchecked, not adopted as a check (Decision 5 states the limit).
6. Slip, a second change after live use and green fake tests: adopted (Left out says a failure is a second small design); six files in one issue kept, because the Protocol and its three users must move together.
