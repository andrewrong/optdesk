# gateway health: a tolerant check per gateway session and the state of each data farm on /data-health

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-20. Checked head: `origin/main` `2c8f3dd6`.

Owner words: 2026-11-20 (the health check per gateway session is tolerant, not zero-error; a broken farm shows on the page and waits; the owner is told once). Plan: none — incident repair, no plan item yet (the owner may name one). Advisor decisions of 2026-11-20 are carried: this design is health only; the yfinance bars fallback is its own design (Left out).

## Goal

The owner opens `/data-health` and sees, for each gateway (by its alias `main` or `wife`), one line: the API session is up, or down and being retried; and under it each data farm the gateway reported (name, `ok` or `down`). When the history farm of `main` is broken for longer than a grace time, the line says in Chinese that the owner must log in at that gateway again, and one text goes to the owner once per incident and once on recovery. A farm that the gateway itself lost never makes us reconnect in a loop; a job call that meets a reconnect is retried, never failed for good.

## Today

- The incident of 2026-11-19 to 2026-11-20: the gateway reported the HMDS farm `ushmds` broken (code 2105). Our API session stayed connected, contract queries worked, every bars request timed out for about 20 hours, and the logs showed only `DeadlineExceeded` (the deadline is the Go caller's; pyside waits with no deadline of its own). A restart of pyside did not help, because the farm is the gateway's; only the owner's login to the gateway did.
- Farm codes are dropped: `spec.is_informational` (2100 to 2199) in `pyside/src/optdesk/ibkr/spec.py`, and `ibgateway._on_error` returns on it; `transport.py` sets the `ib_async` logger to WARNING. Only 1100, 2110 (soft loss) and 1101, 1102 (restore) are tracked (`_connected`, `_soft_lost`).
- `server.supervise` (`pyside/src/optdesk/ibkr/server.py`) reconnects only when the socket ends or a soft loss lasts `RESTORE_WAIT_S` (300 s); its backoff is `BACKOFF_FIRST_S` 1 doubling to `BACKOFF_CAP_S` 60 (`8b8e03b6`, #2549), reset after `STABLE_SESSION_S`. It sends nothing on the wire to test the session. `health.py` already has `PROBE_DEADLINE_S = 2.0` and `server.py` has `_probe` (line 200): they call the local `status()` for `/health`, not the wire.
- `ibgateway.bars` has no timeout of its own. `transport.Ping` returns `connected`, `generation`, `market_data_type` only; nothing in Go reads `SessionService.Ping`, so `/data-health` knows nothing of a gateway. `reqCurrentTime` is used nowhere in `pyside/src`.
- `DataHealthPage.tsx` (`web/src/pages/data_health/`) renders fixed sections only (grid, repeated failures, legend; lines 22 to 54): a new `gateways` field is not shown by any existing generic list, so the page needs its own component.
- FACT, read-only probe of 2026-11-20 08:47Z: a fresh client connected to port 4001 received the farm status messages 2104, 2105, 2106 and 2158 at connect, among them "2105 HMDS data farm connection is broken: ushmds". Farm state is therefore visible at session start once the 21xx codes are no longer dropped.

## Decisions

1. Three layers of a gateway state, kept apart. (a) API session: connected, soft lost (1100, 2110), or ended. (b) Farms: each farm the gateway named, `ok` or `down`. (c) The history probe of Decision 5. A reconnect answers (a) only; (b) and (c) are shown and notified, and the session keeps waiting.
2. Farm codes. Down: 2103 (market data), 2105 (historical data), 2157 (security definition). Ok: 2104, 2106, 2158; they clear the `down` of the same farm. The farm name is the word after the last colon of the message, kept only when it is lower-case letters, digits and dots up to 16 characters, else `other`. This is the fast path: the codes are read at connect and whenever they come. The rest of 2100 to 2199 stays not an error of a request, as today. `spec.is_informational` is unchanged.
3. Clock probe, per gateway (`main` and `wife`). Every `PROBE_EVERY_S` the session asks the gateway for its current time (`reqCurrentTime`, no data farm) under `PROBE_WAIT_S`. A failed probe is one failure; `PROBE_FAILS` consecutive failures close that session and open a new one; a good probe clears the count. Each gateway has its own count: one gateway's failures change nothing of the other's state. The wife gateway (port 4002) gets this probe only: no data request, and no reconnect driven by `main`'s state.
4. Backoff stays as today: doubling from `BACKOFF_FIRST_S` to the 60 s cap of `server.py`. No cap on the number of opens and no `ReconnectPolicy` protocol. The noise is cut instead: one log line per state change, not per attempt. A gateway that stays down logs one `session_down` line when it goes down, and one `session_up` line when it comes back; attempts in between add none. The reason of the last failed attempt is a field of the next state line, not a line of its own.
5. Farm probe, `main` only (the session used for history, port 4001). Besides the clock probe, every `FARM_PROBE_EVERY_S` one tiny history request goes out on that session under `PROBE_WAIT_S`: one daily bar (or the head timestamp) of one fixed liquid index or ETF, whose symbol is named in code only (`pyside/src/optdesk/ibkr/farm_probe.py`), not a name of any watch list. `FARM_PROBE_FAILS` consecutive failures while the clock probe is good set the state `farm_broken`, naming the farm (`ushmds` for US). It is the slow path behind the code of Decision 2; either one sets the state, and the first good answer or an ok code clears it. The probe request waits on the history pacer like any request.
6. History wait. `ibgateway.bars` waits `HISTORY_WAIT_S` per request and then raises `HistoryUnavailable`, which the transport maps to `UNAVAILABLE` with the message `history_unavailable`. The Go caller sees a Retryable error (Decision 11), not its own deadline. Nothing else depends on this error in this design.
7. Grace. A farm broken for less than `FARM_GRACE_S` raises no notice and no `farm_down` verdict (IB resets farms at night and they come back by themselves); the page shows the farm as `down` (the since-time stays in pyside and in the notice, not on the page), and pyside sets the state `farm_broken` only when the grace has passed; the Go side holds no grace. Times come from the injected `Clock`.
8. Constants, trial values (named, not yet measured; the owner can change them without a design), all in one new file `pyside/src/optdesk/ibkr/health_constants.py`: `PROBE_EVERY_S` = 60, `PROBE_FAILS` = 3, `HISTORY_WAIT_S` = 60, `FARM_PROBE_EVERY_S` = 300, `FARM_PROBE_FAILS` = 3, `PROBE_WAIT_S` = 10, `FARM_GRACE_S` = 600. `PROBE_WAIT_S` is a new name because `health.py` already has `PROBE_DEADLINE_S` (2 s, local `/health`).
9. Log lines, one per change, with the configured gateway name only: `farm_code gateway=<name> code=<int> farm=<word>` per code of 2100 to 2199 when the farm state changes, `soft_loss`, `restore`, `session_down`, `session_up`, `farm_broken gateway=<name> farm=<word>`, `farm_ok gateway=<name> farm=<word>`. No account id, no port. The farm word passes the rule of Decision 2.
10. The payload. `PingResponse` gains, additive and numbered after the existing fields: `state` (enum `up`, `soft_lost`, `down`, `farm_broken`), `farms` (repeated `{name, up, since_unix_ms}`). The old fields keep their numbers. `ot serve` reads one `Ping` per gateway socket per request of `/data-health`, under a 2 s deadline; a gateway that does not answer shows `unreachable`, never an error page. The numbers and states are pyside's; the page builder maps them to verdict keys and Chinese text and computes nothing (the front end renders data).
11. A job call in flight during a reconnect gets a Retryable class. pyside answers such a call `UNAVAILABLE` with the message `session_reconnecting` (a different message from `history_unavailable`); `grpcbroker` maps both to `contracts.Retryable`, never to a permanent key failure.
12. What the owner sees, per gateway, is a verdict key of the builder: `session_up`, `session_retrying`, `farm_down` (text: log in at this gateway again; a reconnect does not help), `unreachable`. Each key has its Chinese text in `zh_texts.go` with the term explained nearby. The gateway is shown by alias (`main`, `wife`), never an account id or port.
13. Notice. The existing notify channel is `notify.Sender` (Telegram, `internal/notify/telegram`) with `SentStore` (`alerts.sent`), as `AlertNotifier` uses them. A new `GatewayNotifier` gets one `GatewayFact` per gateway on every tick and decides by its marks: one text when a fact is `Down` (state `farm_broken`, so after the grace), marked by incident with the key `gateway_<alias>_<farm>_<since_unix>` (the farm's `since` time is in the fact, so a second incident on the same day sends), and one text when a gateway with a marked incident is no longer `Down`, marked `<key>_up`. The text names the gateway by alias, no account id, no port, and says the owner must log in again at the gateway; the Chinese text is a constant of Go in `internal/notify/core` (repo rule). Mark only after a confirmed send. No job and no time guard (dev mode).
14. Version of the page. `gateways` is an additive OPTIONAL property of `page.data_health_page.v1`, never in `required`; the schema version does not change. This amends `docs/design/data-health.md` Decision 14 ("Versions are named"), whose exception clause allows an additive field only by amendment; this design is that amendment, and the older readers read the page as before.
15. A new web component renders the gateway block (Decision 12): `GatewayBlock.tsx` is shown by `DataHealthPage.tsx` above the grid when `gateways` is non-empty, and shows nothing else when it is empty or absent.
16. No time guard (dev mode): probes, notices and the page run when due or asked, at any hour.

## Modules

No row of `docs/design/modules.md` changes; every path falls in an existing row by the first matching glob. New edges: none (`pages` gets plain fields, `notify` gets plain fields and `Sender`, `ot` already wires `broker`, `pages` and `notify`).

| paths | row |
|---|---|
| `contracts/proto/ibkr/v1/session.proto`, `internal/contracts/genpb/session.pb.go`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/session_pb2.py`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/session_pb2.pyi`, `contracts/schemas/page.data_health_page.v1.json`, `web/src/generated/data_health_page.v1.ts`, `contracts/fixtures/**`, `internal/contracts/data_health_page_v1_test.go`, `docs/contracts/page-schemas.md` | contracts |
| `pyside/src/optdesk/ibkr/**`, `pyside/tests/ibkr/**`, `pyside/tests/contracts/test_ibkr_api.py` | pyside |
| `internal/broker/**`, `docs/contracts/broker.md` | broker |
| `internal/pages/data_health_page/**`, `docs/contracts/pages.md` | pages |
| `internal/notify/**`, `docs/contracts/notify.md` | notify |
| `cmd/ot/**`, `docs/contracts/ot/**` | ot |
| `web/src/pages/data_health/**`, `web/test/pages/data_health/**` | web |
| `docs/design/gateway-health.md` | docs |

## Interface

1. Proto `ibkr.v1.PingResponse`: additive `state` and `farms` of Decision 10; enum `SessionState`; message `FarmState {string name; bool up; int64 since_unix_ms}`. Existing fields and numbers unchanged.
2. Page `page.data_health_page.v1` gains the optional array `gateways` (`[{name, verdict, farms: [{name, up}], note}]`, `additionalProperties` false, `verdict` an enum of the keys of Decision 12, not in `required`).
3. Python (`pyside/src/optdesk/ibkr/api.py`): protocol `FarmBoard` (`record(code, text, now) -> None`, `snapshot() -> tuple[model.Farm, ...]`); exception `HistoryUnavailable`. `pyside/src/optdesk/ibkr/model.py`: the existing `SessionState` gains `state` (`up`, `soft_lost`, `down`, `farm_broken`; default `up`) and `farms: tuple[Farm, ...]` (default empty); new `Farm` is `{name: str, up: bool, since: int}` (`since` in unix ms); the `Gateway.status` protocol returns them. `api.py` declares no second `SessionState`; `model.SessionState` is its one owner.
4. Go (`internal/broker/api.go`): `type Farm struct { Name string; Up bool; Since time.Time }`, `type Health struct { Name, State string; Farms []Farm }` (`State` is one of `up`, `soft_lost`, `down`, `farm_broken`), `type HealthReader interface { Health(ctx context.Context) (Health, error); Close() error }`, `NewHealthFunc`.
5. Pages: `data_health_page.Input` gains `Gateways []GatewayInput`; `GatewayInput` is `{Name string; Answered bool; State string; Farms []GatewayFarm}` and `GatewayFarm` is `{Name string; Up bool}` (`State` as in `broker.Health`; `Answered` false when the reader failed).
6. Notify: `notify.GatewayNotifier` with `NotifyGateways(ctx, []GatewayFact) (int, error)`; `GatewayFact` is `{Gateway, Farm string; Since time.Time; Down bool}` (`Farm` and `Since` are those of the first farm that is down, empty when `Down` is false). `notify.FactOf(alias string, answered bool, state string, farms []GatewayFarm) GatewayFact` is the declared mapping, owned by notify: `Down` is true when `answered` and `state` is `farm_broken`; `GatewayFarm` is `{Name string; Up bool; Since time.Time}`.

## Issues

Each entry is one module. `(interface)` entries land first. All entries: `Plan: none — incident repair`, `Design: docs/design/gateway-health.md#issues`, `Standing: none`, `Consultation: none`. Entries 1 and 5 touch an owner-only path (`contracts/proto/`, `transport.py`): The dispatcher writes this pull request. The owner merges this pull request.

Decision coverage: 1, 2, 3, 5, 7, 8, 9 entry 3; 6 entries 3 and 4; 10 entries 1 and 4; 11 entries 4 and 5; 12 entry 6; 13 entry 7; 14 entry 1; 15 entry 8; 4 entry 3 item 8 (the backoff itself is unchanged code, so only the log rule has an item); 16 no entry — it adds no code. The wire of entry 9 carries 10, 12 and 13.

### 1 gateway-health-contract (interface)

Module: contracts. Size: M. Order: first. Owner-only: `contracts/proto/` is an owner-only path. The dispatcher writes this pull request. The owner merges this pull request.
Owner case: a change of an interface in an owner-only path
Files: `contracts/proto/ibkr/v1/session.proto`, `internal/contracts/genpb/session.pb.go`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/session_pb2.py`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/session_pb2.pyi`, `contracts/schemas/page.data_health_page.v1.json`, `web/src/generated/data_health_page.v1.ts`, `contracts/fixtures/page.data_health_page.v1.gateways.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/data_health_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: ibkr.v1 PingResponse (additive), page.data_health_page v1 (additive optional `gateways`)
Acceptance:
1. `contracts/proto/ibkr/v1/session.proto` adds the fields and messages of Interface 1 after the existing field numbers and renames no existing field.
2. `contracts/schemas/page.data_health_page.v1.json` has the `gateways` property of Interface 2 with `additionalProperties` false at every object, `verdict` a new enum of exactly `session_up`, `session_retrying`, `farm_down`, `unreachable` (all new words), and `gateways` is new and not in `required`.
3. `web/src/generated/data_health_page.v1.ts` names `gateways` as optional and nothing else new; the `ok` and `gap` fixtures are unchanged.
4. The fixture `page.data_health_page.v1.gateways.json` (new) has two invented gateways named `main` and `wife` (one `farm_down` with a farm `down`, one `unreachable`) and is listed in `contracts/fixtures/MANIFEST.synthetic-v1.json`.
5. `internal/contracts/data_health_page_v1_test.go` has `TestGatewaysFixtureValidates` (new) and `TestGatewaysVerdictOutsideEnum` (new): a document with a `verdict` outside the enum is refused.
6. `docs/contracts/page-schemas.md` names `gateways` as an amendment of `docs/design/data-health.md` Decision 14.

### 2 gateway-health-pyside-api (interface)

Module: pyside. Size: S. Order: after 1.
Files: `pyside/src/optdesk/ibkr/api.py`, `pyside/src/optdesk/ibkr/model.py`, `pyside/tests/contracts/test_ibkr_api.py`
Acceptance:
1. `model.py` gives `SessionState` the fields `state` and `farms` and declares `Farm` (new; fields of Interface 3); `api.py` declares the protocol `FarmBoard` and the exception `HistoryUnavailable` (new) and no `SessionState`, signatures, types and docstrings only.
2. `TestFarmBoardContract` (new) in `pyside/tests/contracts/test_ibkr_api.py`: a fake class is an instance of the runtime-checkable `FarmBoard`; `TestSessionStateDefaults` (new): `model.SessionState` built with the old arguments still works through the defaults.

### 3 gateway-health-pyside-probe

Module: pyside. Size: L. Order: after 2.
Files: `pyside/src/optdesk/ibkr/health_constants.py`, `pyside/src/optdesk/ibkr/farms.py`, `pyside/src/optdesk/ibkr/farm_probe.py`, `pyside/src/optdesk/ibkr/session_probe.py`, `pyside/src/optdesk/ibkr/server.py`, `pyside/src/optdesk/ibkr/ibgateway.py`, `pyside/tests/ibkr/test_farms.py`, `pyside/tests/ibkr/test_probe.py`, `pyside/tests/ibkr/test_server_health.py`
Acceptance:
1. `health_constants.py` (new) holds the seven constants of Decision 8 with their values and no other file of the package defines one of them; `PROBE_DEADLINE_S` of `health.py` is not renamed or reused.
2. `farms.py` (new) implements `FarmBoard`: `TestFarmCodes` (new) shows 2103, 2105 and 2157 set the named farm `down`, 2104, 2106 and 2158 set it `up`, a farm name outside the character rule is `other`, and a code of 2100 to 2199 outside the six changes no farm; a 2105 text naming `ushmds` read at connect shows the farm `ushmds` `down`.
3. `session_probe.py` (new): `TestClockProbe` (new) shows two failures keep the session, the third closes it and opens a new one, and a good probe between clears the count; every wait goes through the injected `Clock`.
4. `TestWifeClockOnly` (new) shows the `wife` gateway gets the clock probe only: no history request is sent for it, and three failed probes of `main` change nothing of the state or the session of `wife`.
5. `farm_probe.py` (new): `TestFarmProbe` (new) shows three failed history probes with a good clock probe set `farm_broken` naming `ushmds`, a good answer clears it, and a failing clock probe sets no `farm_broken`.
6. `TestFarmGrace` (new) shows with the fake clock that a farm down for less than `FARM_GRACE_S` gives no `farm_broken` verdict and one down for longer does.
7. `TestHistoryWait` (new) shows `ibgateway.bars` raises `HistoryUnavailable` after `HISTORY_WAIT_S` of fake clock.
8. `TestLogOnStateChange` (new) drives `supervise` against a gateway that stays down for 24 hours of fake clock and asserts exactly one `session_down` line and no line per attempt, then one `session_up` after it returns; the lines hold `gateway=<name>` and no account id and no port.
9. `server.py` and `ibgateway.py` get only the hook calls, all new code is in the new files of Files, and a test (`TestSizeCeiling`, new) asserts each of the two files stays under 800 lines (`ibgateway.py` is 791 at the checked head).

### 4 gateway-health-pyside-transport

Module: pyside. Size: S. Order: after 2.
Owner-only: `pyside/src/optdesk/ibkr/transport.py` is an owner-only path. The dispatcher writes this pull request. The owner merges this pull request.
Owner case: a change in an owner-only path
Files: `pyside/src/optdesk/ibkr/transport.py`, `pyside/src/optdesk/ibkr/transport_health.py`, `pyside/tests/ibkr/test_transport_health.py`
Acceptance:
1. `transport.py` (794 lines at the checked head) gets only the calls into the new `transport_health.py` (new); the mapping code is in the new file, and a test (`TestSizeCeiling`, new) asserts `transport.py` stays under 800 lines (794 at the checked head).
2. `TestPingCarriesFarms` (new) builds against `model.SessionState` from a fake `Gateway` (no `ibgateway.py` read) and shows `Ping` returns `state` and every farm of the board with `up` and `since_unix_ms`, and the old fields as before.
3. `TestHistoryUnavailableStatus` (new) shows `HistoryUnavailable` becomes `UNAVAILABLE` with the message `history_unavailable`.
4. `TestInFlightReconnect` (new) shows a call in flight when its session closes is answered `UNAVAILABLE` with the message `session_reconnecting`, not `INTERNAL` and not a key-specific error.

### 5 gateway-health-broker (interface)

Module: broker. Size: M. Order: after 1.
Files: `internal/broker/api.go`, `internal/broker/api_test.go`, `internal/broker/grpcbroker/health.go`, `internal/broker/grpcbroker/health_test.go`, `internal/broker/grpcbroker/errors.go`, `internal/broker/grpcbroker/errors_test.go`, `docs/contracts/broker.md`
Acceptance:
1. `internal/broker/api.go` declares `Health`, `Farm`, `HealthReader` and `NewHealthFunc` of Interface 4, declarations only; `TestHealthReaderContract` (new) in `api_test.go`: a fake implements it and a caller reads one `Health`.
2. `grpcbroker/health.go` (new) implements `HealthReader` over `SessionService.Ping`; `TestHealthMapsPing` (new) shows every new field maps to the field of the same meaning and an older answer (new fields zero) maps to state `up` when `connected`.
3. `TestHealthUnreachable` (new) shows a call whose context is cancelled by the test (a fake server, no wall clock) returns a retryable `contracts.TypedError`.
4. `TestReconnectIsRetryable` (new) in `errors_test.go`: a status `UNAVAILABLE` with the message `session_reconnecting` and one with `history_unavailable` map to `contracts.Retryable`, never to a permanent key failure.
5. `docs/contracts/broker.md` names `HealthReader` (new) and the two messages.

### 6 gateway-health-page

Module: pages. Size: M. Order: after 1.
Files: `internal/pages/data_health_page/builder.go`, `internal/pages/data_health_page/gateways.go`, `internal/pages/data_health_page/gateways_test.go`, `internal/pages/data_health_page/zh_texts.go`, `docs/contracts/pages.md`
Acceptance:
1. `gateways.go` (new) maps `Input.Gateways` to the `gateways` array by the fields of `GatewayInput` only: `TestGatewayVerdicts` (new) shows `farm_broken` gives `farm_down`; `soft_lost` or `down` gives `session_retrying`; `Answered` false gives `unreachable`; otherwise `session_up`.
2. `TestGatewayTexts` (new) shows each verdict key has a Chinese text from `zh_texts.go` with the term explained beside it (the `farm_down` text says to log in at that gateway again and that a reconnect does not help), and no text holds an account id or a port.
3. `TestNoGatewaysOmitsField` (new) shows `Build` with no gateways returns a document without the `gateways` key.

### 7 gateway-health-notify (interface)

Module: notify. Size: M. Order: after 1.
Files: `internal/notify/api.go`, `internal/notify/api_test.go`, `internal/notify/core/gateways.go`, `internal/notify/core/gateways_text.go`, `internal/notify/core/gateways_test.go`, `docs/contracts/notify.md`
Acceptance:
1. `internal/notify/api.go` declares `GatewayNotifier`, `GatewayFact`, `GatewayFarm`, `FactOf` and `NewGatewayNotifierFunc` of Interface 6, declarations only; `TestGatewayNotifierContract` (new) in `api_test.go`; `TestFactOf` (new) shows the mapping of Interface 6.
2. `core/gateways.go` (new) sends through `notify.Sender` and marks through `SentStore`: `TestGatewayOncePerIncident` (new) shows two passes on one incident (same gateway, farm and `Since`) send one text, and a second incident of the same farm on the same day (a later `Since`, after a `Down: false` pass) sends again.
3. `TestGatewayRecoveryOnce` (new) shows a `Down: false` fact after a sent incident sends one recovery text, a second one sends none.
4. `TestGatewayMarkAfterSend` (new) shows a failed send leaves no mark and the next pass sends again.
5. `TestGatewayText` (new) shows the text, a constant of `gateways_text.go`, names the gateway by alias and says to log in again at the gateway, and holds no account id and no port.

### 8 gateway-health-web

Module: web. Size: S. Order: after 1.
Files: `web/src/pages/data_health/GatewayBlock.tsx`, `web/src/pages/data_health/DataHealthPage.tsx`, `web/test/pages/data_health/GatewayBlock.test.tsx`
Acceptance:
1. `GatewayBlock.tsx` (new) renders for each gateway its name, its verdict text and its farms with `ok` or `down`, from payload fields only; it computes no verdict.
2. `DataHealthPage.tsx` shows `GatewayBlock` (new) when `payload.gateways` (new) is non-empty and renders as before when it is absent or empty.
3. `GatewayBlock.test.tsx` (new) renders the fixture `page.data_health_page.v1.gateways.json` and finds both gateway names and the down farm, and renders the `ok` fixture without a gateway block.

### 9 gateway-health-wire

Module: ot. Size: M. Order: after 5, 6 and 7.
Files: `cmd/ot/data_health_site.go`, `cmd/ot/gateway_health.go`, `cmd/ot/gateway_health_test.go`, `cmd/ot/data_health_site_test.go`, `cmd/ot/notify.go`, `docs/contracts/ot/pages/data_health_page.md`
Acceptance:
1. `cmd/ot/gateway_health.go` (new) reads one `Health` per configured gateway socket through `broker.HealthReader` under a 2 s deadline and copies it to `Input.Gateways` by alias (`Answered` false on an error); `TestGatewayHealthUnreachable` (new) shows a reader that errors gives `unreachable` and the page still answers 200 with a document that validates.
2. `TestDataHealthGateways` (new) in `data_health_site_test.go`: two fake readers (one with state `farm_broken`) give the `gateways` array of the fixture `gateways`.
3. `TestGatewayNotifyPass` (new): the scheduler tick passes one `GatewayFact` per gateway on every tick, built by `notify.FactOf` from the `Health` just read, and keeps no previous verdict; `GatewayNotifier` and its marks decide what is sent.
4. No line of `cmd/ot` holds a rule: the verdict words come from `Health` and the builder.

## Checked by CI

- `ci-go` for entries 1, 5, 6, 7, 9: build, vet, tests of the packages, the architecture checks, `buf breaking` and the generation check for entry 1, `TestGatewaysFixtureValidates` and the existing builder, `purity_test.go` and `cmd/ot/english_test.go` tests unchanged.
- `ci-pyside` for entries 2, 3, 4: `uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto` including `tests/guards` (size and layers) and the existing `test_spec.py`.
- `ci-web` for entries 1 and 8: `tsc`, `eslint`, `vitest`, build.
- No test sleeps or connects to a gateway: every wait goes through the injected `Clock`; a test that needs PostgreSQL carries the tag `pg`.

## Left out

- bars fallback to yfinance: its own design (docs/design/bars-fallback.md, next)
- Auto re-login of the gateway (IBC or any login script): the credentials are the owner's, so the owner logs in; the design only tells the owner when and where.
- A reconnect cap per hour and a `ReconnectPolicy` protocol: not adopted (Decision 4).
- A data request or a reconnect for the wife gateway driven by `main`: none (Decision 3).
- No change to the 1100/2110/1101/1102 soft-loss rule or `RESTORE_WAIT_S`, to the history pacer or the line budgets.
- No new fault table and no history of farm outages: the page shows the present state.
- No automatic restart of pyside or the gateway process; a restart does not heal a farm the gateway lost.

## Doubts

1. The constants of Decision 8 are trial values; the farm probe request (one daily bar of a fixed index or ETF) is the least known part: if the probe symbol has no permission on `main`, the probe fails by data and not by farm. Entry 3 names the symbol in code only, to be changed in one place.
2. The grace of 10 minutes may be short or long against the real nightly reset; it is a constant.

## Adversarial

Raw text: the scratchpad file `gateway-health-adversarial.md` in `state/optdesk-drafts/designs/` (not committed).

1. Gap against the goal (blind first minutes, false alarms at the IB reset, hourly cap delaying recovery, page not showing the fallback): adopted. The 21xx codes are read at connect (FACT in Today), the farm probe covers silent farms (Decision 5), the grace stops false alarms (Decision 7), the hourly cap is dropped (Decision 4); the fallback is another design.
2. Biggest unnamed risk (yfinance rows read as IBKR rows; a capped gateway not reconnecting after login): adopted for the second part, the cap is gone; the first part leaves with the fallback to `docs/design/bars-fallback.md`, which must name every reader of `rd.price_bar_r1`.
3. A simpler way (split in two; skip the cap and the policy protocol; maybe skip the notice): adopted for the split and the cap. Not adopted for dropping the notice: the owner wants to be told once and the page is not looked at all day (Decision 13, entry 7).
4. Lock-in (additive proto and page fields, enum words, `alerts.sent` keys, UNAVAILABLE plus a message string): partly adopted. The additive fields and keys are accepted and named (Decision 14); the string match is made two fixed messages with a test on the Go side (Decision 11, entry 5 item 4); the CHECK migration leaves with the fallback.
5. Claim most likely false (a farm down at pyside start is not seen): adopted, it was false. The read-only probe of 2026-11-20 08:47Z showed 2105 `ushmds` at connect; Decision 15 of the draft is replaced by the FACT in Today and the history-stall counter is dropped. The restate runner-up leaves with the fallback.
6. Likeliest slip (the 800-line ceilings of `ibgateway.py` and `transport.py`, owner-only paths, entry size): adopted. New code goes in new files, the two files get only hook calls and a test holds each under 800 lines (entry 3 item 9, entry 4 item 1), the owner-only entries say so, and the large pyside entry is cut in two (3 and 4). The acceptance items that are runs by CI are under `## Checked by CI`, and no test waits on a wall clock.
