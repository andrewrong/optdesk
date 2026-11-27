---
type: design
title: "not connected answer: every pyside rpc answers a session that is not connected with UNAVAILABLE, and the Go broker marks a row the server never sent as a retryable error, not as disconnected"
status: accepted
date: "2026-11-27"
need: [none]
none_reason: "bug from the pyside lifecycle audit of 2026-11-26 (hole H8); no need record exists"
---

# not connected answer: every pyside rpc answers a session that is not connected with UNAVAILABLE, and the Go broker marks a row the server never sent as a retryable error, not as disconnected

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-27
Plan: none

Source: `state/optdesk-drafts/audit-pyside-lifecycle-2026-11-26.md`, hole H8. Every fact below was read on origin/main `33b3e4c2` after `git fetch` on 2026-11-27; nothing was run.

Owner-visible result: a job that needs the gateway sees one answer when the gateway session is not connected, whatever call it makes: a retryable error that its runner retries and its row or message calls "disconnected". Today a snapshot taken while the session is down comes back as a normal answer with every row `disconnected`, and a row the server never sent also reads `disconnected`.

## Frame

- Seven rpcs read the gateway. Six (`ListChains`, `ResolveContract`, account values, positions, executions, dividends, bars; all through `self._session()` at `pyside/src/optdesk/ibkr/ibgateway.py:333-336`, which raises `ConnectionError`) end in gRPC `UNAVAILABLE` with the fixed word `session_reconnecting` (`retryable_status`, `pyside/src/optdesk/ibkr/transport_health.py:33-35`). Go turns `UNAVAILABLE` into a `Retryable` error (`kindOf`, `internal/broker/grpcbroker/errors.go`).
- `Snapshot` is the odd one: `IbGateway.quotes` (`ibgateway.py:469-476`) reads `self._generation` without asking the session, and `_quote` answers `STATUS_DISCONNECTED` for every row when `_current(generation)` is false (`ibgateway.py:533-539`). The RPC then ends OK.
- Go `Snapshot` (`internal/broker/grpcbroker/dial.go:145-148`) pre-fills every slot with `StatusDisconnected`; a contract the stream never answered (early end of stream) keeps it and is told apart from a real disconnect by no one. `docs/contracts/broker.md` line 11 and `TestSnapshotOrder` (`client_test.go:107`) state it as the contract.
- Callers already treat a `Snapshot` error as "disconnected and retry": `positionmarks` answers `fail(contracts.Retryable, "disconnected", alias)` (`internal/jobs/positionmarks/run.go:285-288`); `quotesnapshot` wraps it `asTyped(err, contracts.Retryable)` (`run.go:150-152`); `optioneod` and `optionopen` pass the typed error up.

## Owner rules and how this design meets them

1. "Gaps stay gaps with a reason, never invented values" (owner 2026-11-24). A row nobody sent is no quote and no "disconnected" fact; it is an error of the call. Self-check: entry 2 removes the pre-fill.
2. "A file over 800 lines is listed in `docs/design/ratchet.json` and a ceiling is never raised by an agent; when a change does not fit, report it" (`AGENTS.md`, Size). `ibgateway.py` is 799 lines (not listed; at 800 it is still allowed). Entry 1 replaces one line by one line and adds none. `transport.py` (795) is not touched. Self-check: entry 1 acceptance 3.
3. "Tests run in parallel, never sleep, a fake clock, no fixed port" (`AGENTS.md`, Tests). New tests use `FakeClock` and `t.Parallel()`/`pytest -n auto`; no acceptance item runs a test.
4. "Other modules use a module through its interface and nothing else" (`AGENTS.md`, Interfaces). `internal/broker/api.go` and `contracts/proto/ibkr/v1/*.proto` are not edited (Decision 3).
5. "One issue, one module" (`AGENTS.md`, Modules). Entry 1 is pyside, entry 2 is broker.
6. "Obvious bug: decide, don't ask" (owner 2026-11-11). The style is derived below, not offered as a choice.

## Decisions

1. **The one style is gRPC `UNAVAILABLE` (word `session_reconnecting`), Go `Retryable`.** Derivation: the requirement is that callers see "disconnected" uniformly. Six rpcs carry no status field (chains, resolve, account, executions, dividends, bars), so for them only a call error can say it; only `Snapshot` could say it by a row. A style every rpc can use is the error. It also costs least: six rpcs and every Go caller already use it, and `positionmarks` already maps a `Snapshot` error to its row word "disconnected".
2. **`quotes` asks the session before it starts.** `IbGateway.quotes` calls `self._session()` in place of reading `self._generation`; a session that is not connected raises `ConnectionError` before any row is made, and `MarketDataServicer.Snapshot` already maps a failure of the work to `UNAVAILABLE` (`transport.py:507-510` through `_status_of`). The line `generation = self._generation` becomes `_, generation = self._session()`: the file stays 799 lines.
3. **A loss after the call started keeps the per-row `DISCONNECTED`.** Rows already answered `ok` must not be thrown away, and `job.lost`, the 10197 deadline and the "stop after the pacer wait" paths (`ibgateway.py:533-539, 595-606`, tests `test_stop_checked_after_pacer_wait`, `test_disconnect_marks_unanswered_disconnected`, `test_2110_ends_a_waiting_quote_disconnected`) say "this contract was not quoted because the session went away", a per-contract fact. The proto and `internal/broker/api.go` keep `QUOTE_STATUS_DISCONNECTED` / `StatusDisconnected` for exactly that. The rule in one sentence: not connected at the call start is an error of the call; lost during the call is a status of the row.
4. **Go: a contract with no answer when the stream ends is a `Retryable` error of `Snapshot`, no row.** `Snapshot` no longer pre-fills `StatusDisconnected`; after `io.EOF`, if any slot is unfilled it returns `nil` and a `Retryable` error `broker: snapshot: stream ended before every contract was answered` (no contract id, no socket path in the text). `errors.go` gets a `retryable(text string) error` next to `permanent`, same `brokerError` type. The `broker.Broker` interface is unchanged; the doc sentence of `docs/contracts/broker.md` line 11 changes.
5. **No proto and no schema change.** `Contracts: unchanged`. The server sends the same messages; only the case "not connected at the start" moves from rows to a status code.

## Modules

`docs/design/modules.md` is unchanged. First row that matches decides: pyside (`pyside/**`: entry 1), broker (`internal/broker/** docs/contracts/broker.md`: entry 2). No import edge is added.

## Interface

No interface file changes: `pyside/src/optdesk/ibkr/api.py`, `internal/broker/api.go` and `contracts/proto/ibkr/v1/marketdata.proto` stay as they are (Decision 5; the comment "one `Quote` per contract in the order asked" of `broker.Broker.Snapshot` stays true because a call that cannot answer every contract now fails). The changed contract is the prose of `docs/contracts/broker.md` (entry 2, not an entry file of the broker module).

## Issues

Build order: the entries are independent; each compiles and passes against main alone. Either lands first. Test runs are in `## Checked by CI`. Each entry is one module and lists only paths of that module.

### 1 notconn-pyside-quotes-session

Module: pyside. Size: S.
Order: any.
Files: `pyside/src/optdesk/ibkr/ibgateway.py`, `pyside/tests/ibkr/test_ibgateway_quote_errors.py`
Contracts: unchanged
Design: docs/design/not-connected-answer.md#1 notconn-pyside-quotes-session
Acceptance:
1. In `ibgateway.py` the first statements of `IbGateway.quotes` after `row_spec = spec.kind_spec(kind)` are `_, generation = self._session()`, and `grep -c 'generation = self._generation' pyside/src/optdesk/ibkr/ibgateway.py` prints 0 inside `quotes`; the `_current(generation)` branches of `_quote` and the 10197 deadline branch of `quotes` are unchanged, and the tests `test_stop_checked_after_pacer_wait`, `test_disconnect_marks_unanswered_disconnected` and `test_2110_ends_a_waiting_quote_disconnected` are not edited.
2. `test_ibgateway_quote_errors.py` gains `test_quotes_while_not_connected_raise_connection_error` (new) asserting that a gateway that was never opened raises `ConnectionError` from `quotes` and calls `on_quote` zero times, and `test_snapshot_not_connected_aborts_unavailable` (new) asserting that the `MarketDataServicer.Snapshot` of that gateway aborts with `UNAVAILABLE` and details `session_reconnecting` and yields no message; the file stays under 800 lines and imports its fixtures from `test_ibgateway` as `test_session_codes.py` does.
3. `ibgateway.py` has at most 800 lines (`wc -l`), no entry of `docs/design/ratchet.json` changes, and no other file of `pyside/src` is edited.

### 2 notconn-broker-unsent-row

Module: broker. Size: S.
Order: any.
Files: `internal/broker/grpcbroker/dial.go`, `internal/broker/grpcbroker/errors.go`, `internal/broker/grpcbroker/client_test.go`, `docs/contracts/broker.md`
Contracts: unchanged
Design: docs/design/not-connected-answer.md#2 notconn-broker-unsent-row
Acceptance:
1. `dial.go` no longer contains `StatusDisconnected`: `grep -c StatusDisconnected internal/broker/grpcbroker/dial.go` prints 0; `Snapshot` makes its slots with the contract only and, after `io.EOF`, returns `nil` and `retryable("snapshot: stream ended before every contract was answered")` when a slot is unfilled; a stream that answered every contract returns the slots in the order asked as before.
2. `errors.go` declares `retryable(text string) error` returning a `*brokerError` of kind `contracts.Retryable` with text `"broker: " + text`, next to `permanent`; no other function of the package changes.
3. `client_test.go` changes `TestSnapshotOrder` so the fake answers all four contracts (still in reverse order) and asserts the slots in the order asked.
4. `client_test.go` also adds `TestSnapshotUnsentContractIsRetryableError` (new) asserting that a fake which skips contract 3 gives `nil` quotes and an error whose `Kind()` is `contracts.Retryable` and whose text holds neither a contract id nor a socket path, and `TestSnapshotNoPrefilledStatus` (new) asserting an empty contract list still returns an empty non-nil slice and no error.
5. `docs/contracts/broker.md` line 11 no longer says "a contract with no streamed answer gets `StatusDisconnected`"; it says that a contract the stream did not answer makes the call a `Retryable` error, that `StatusDisconnected` is only what pyside sends for a session lost during the call, and the test list names the two new tests.
6. `internal/broker/api.go` and every file under `contracts/` and `internal/contracts/` are unchanged, and `internal/broker/grpcbroker/dial.go` stays under 600 lines.

## Checked by CI

`go test ./...` and go-arch-lint (entry 2); `uv sync --locked`, `ruff check`, `mypy --strict` and `pytest -n auto` (entry 1); `go run ./scripts/precheck`. After deploy the window may check read-only on the main gateway (port 4001 only) that `ot run-once --dry-run quote_snapshot` with the gateway stopped reports a retry with "disconnected" and no stored rows; that check is not an acceptance item.

## Left out

- **Per-row `DISCONNECTED` for a loss during the call.** Kept on purpose (Decision 3); turning a half-answered stream into an error would drop `ok` rows. A caller cannot tell "lost mid-call" from "not connected" by the row alone; it need not, both are retried.
- **`UNAVAILABLE` text of other causes.** `retryable_status` also gives `UNAVAILABLE` for `OSError` and `HistoryUnavailable`; not touched.
- **Other pyside holes of the audit (H1 to H7, H9, H10).** Own designs.
- **A Go `Quote` field that says why a row is disconnected.** No caller asks for it.

## Doubts

1. `quotesnapshot` stored a row with word `DISCONNECTED` for a down session (`mapQuote`, `map.go:61`); it now returns a retryable error and stores nothing for that chunk until the retry succeeds or the runner gives up. The runner's own ledger and gap say it; `positionmarks` already behaved so. Whether the owner wanted a stored "disconnected" row per quote is not stated anywhere read.
2. Whether a `ConnectionError` raised from `quotes` before the first message reaches `context.abort` before any yield: read in `transport.py:500-510` (the queue gets `None` from the done callback, `failure` is read, abort is called), not run. Entry 1 acceptance 2 has the test.

## Adversarial

Self-review by the drafting session on origin/main `33b3e4c2`; each point is answered below.

1. **Gap vs goal.** Adopted in part. The goal says callers see "disconnected" uniformly; a loss during the call still shows as a row status. Named in Decision 3 and Left out, with the one-sentence rule. Not adopted: turning that into an error, because it throws away answered rows.
2. **Biggest unnamed risk.** Adopted. A job that stored the `DISCONNECTED` rows (`quotesnapshot`) now stores nothing for that chunk on a down session; named in Doubts 1. The other four callers already map a `Snapshot` error to a retry.
3. **Simpler way.** Not adopted: "make every rpc answer rows/flags" cannot work, six rpcs have no status field; "only fix Go" leaves a down session answering OK with rows. The chosen change is one replaced line in pyside and one condition in Go.
4. **Lock-in.** Accepted: callers may now rely on "a `Snapshot` error is retryable and means no answer for the call". The proto keeps the enum value, so the other direction stays open.
5. **Claim most likely false.** "The file stays 799 lines and `_session()` raises before any row": `_session` raises only when `self._connected` is false; a session in `soft_lost` or `_opening` may still count as connected (`ibgateway.py:285-287`), and then rows are made as today. Not a regression; the per-row path covers it. Entry 1 acceptance 3 checks the line count so a slip shows.
6. **Likeliest slip.** The worker adds a helper or a comment line to `ibgateway.py` and reaches 801 lines. Acceptance 3 of entry 1 states at most 800 and says to report rather than split the file. Second: the worker leaves `TestSnapshotOrder` as it is, which fails; entry 2 acceptance 3 names it.

## 摘要

pyside 的行情快照在网关会话未连接时，现在返回一个"全部行都是 disconnected"的正常应答，而其他六个调用都返回 UNAVAILABLE（Go 侧是可重试错误）。本设计统一为一种：调用开始时会话未连接，一律 UNAVAILABLE；只有调用进行中才断线时，已回答的行保留，未回答的行仍标 disconnected。Go 侧不再预填 disconnected，流结束时仍有没答的合约就返回可重试错误。只改 `ibgateway.py` 一行（替换，不增行）、`dial.go`、`errors.go` 和文档，不改 proto。
