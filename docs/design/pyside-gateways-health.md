# Pyside gateways and health: two IB Gateways in one process, and a health URL for crew

Author: the optdesk dispatcher (draft)
Owner approved: (pending)
Date: 2026-11-07
Source: owner decisions of 2026-11-07 (dispatcher window): (1) the pyside IBKR service must be able to use two IB Gateways on the one machine, gateway A (the owner's live account, with market data subscriptions)
and gateway B (a second account, no market data subscriptions); B collects that account's own information and is a fallback source of non-subscription information when A is down; both are read-only;
(2) no start at boot: the owner logs into the gateways, then pulls the service up with crew; (3) a health link that crew polls. Read on main at 85a8ac5: `pyside/src/optdesk/ibkr/server.py`,
`ibgateway.py`, `model.py`, `timing.py`, `transport.py` (owner-only), `docs/design/modules.md`, `docs/design/pyside-ibkr-transport.md`, `docs/design/broker.md`,
`docs/adr/0003-market-data-architecture.md`, `docs/GLOSSARY.md`, `cmd/ot/runonce.go`, and from agent-crew `internal/crew/health_poll.go` and `services.toml`.
The two gateways are called gateway A and gateway B (account A, account B) here; ports are configuration values and appear nowhere in the repository.

## What main holds today

- `python -m optdesk.ibkr` (`pyside/src/optdesk/ibkr/__main__.py`) calls `server.main(os.environ)`. `load_config` reads `OT_HOME` and `OT_IBKR_GATEWAY_PORT` (both required), `OT_IBKR_GATEWAY_HOST`,
  `OT_IBKR_CLIENT_ID`, `OT_IBKR_LINES`, `OT_IBKR_SNAPSHOTS` and `OT_IBKR_MARKET_DATA_TYPE`. `serve` builds one `IbGateway`, one grpc server on the Unix socket `$OT_HOME/run/ibkr.sock`
  (`ot` reads the same path, `cmd/ot/runonce.go`), and one `supervise` task that connects, polls `status()` every second and reconnects with a capped backoff. It logs `connect_ok`, `connect_failed`,
  `session_lost` and `server_started`.
- One gateway, one connection. `IbGateway` is constructed per `(host, port, client id)`; `SessionState` holds `connected`, `generation` and `market_data_type`. The state is not readable from outside the
  process: there is no TCP listener and no health URL (the runbook draft says so and proposes `boot = true`; this design replaces both).
- The guard of `transport.py` is installed on every client by `install_guard` (allowlist of message ids 1, 2, 9, 20, 25, 49, 59, 71 and 78; orders refused, unknown ids refused). It is per client, so a second
  `IbGateway` built with the same `install_guard` is guarded the same way. Account, position and execution requests are not on the allowlist (ADR 0003: "a later issue may add them to the allowlist
  explicitly"); `docs/design/pyside-ibkr-transport.md` (Left out) repeats it. The proto `contracts/proto/ibkr/v1` has the services Session (`Ping`), Contracts (`ListChains`, `ResolveContract`) and
  MarketData (`Snapshot`) and no account or position message.
- Earlier designs that look ahead to the second account: ADR 0003 (accepted) keys the pacer limits "per gateway, by its `(host, port)`, shared by every connection of the process to it" and lists "account
  requests" among the paced kinds (amendments of 2026-10-16/17), so a second gateway in the same process already has its own pacer state; `docs/design/modules.md` gives the broker module "typed records,
  account ids aliased" and `docs/design/pyside-ibkr-transport.md` (Left out) names "the record types with aliased account ids" for the Go client; `docs/GLOSSARY.md` defines every account figure
  "per account alias" and "never across accounts" (the account snapshot of a day, with the readings built from it). No design file on main names two gateways or two ports; this is the first.

## Decisions

1. One process with two gateway connections, not two service instances. The process builds one `IbGateway` per configured gateway and one `supervise` task per gateway, so a gateway that is down or
   being logged into never blocks the other (each supervisor retries on its own backoff). Alternative rejected: two instances of `python -m optdesk.ibkr` with two services.toml entries. It needs a new
   env key for the socket name (the socket path is built from `OT_HOME` alone), two health URLs, two logs and two deploys of one release, and gives nothing in return: ADR 0003 already makes the
   pacer state per gateway inside one process, and the asyncio tasks isolate the failures. The one thing one process loses, isolation from a crash, is not a crash of B's connection (every
   connect failure is caught in `supervise`); a process crash restarts both, and crew restarts the one entry.
2. Gateway B is optional and off by default. B exists only when `OT_IBKR_GATEWAY_B_PORT` is set. With it unset the process behaves exactly as on main (one gateway, one socket), so the existing
   tests and any entry without B stay valid. Host: both gateways share `OT_IBKR_GATEWAY_HOST` (both are on this machine). Client id: `OT_IBKR_GATEWAY_B_CLIENT_ID`, default the value of
   `OT_IBKR_CLIENT_ID` (a client id is per gateway, so the same number does not collide). Market data type of B: `OT_IBKR_GATEWAY_B_MARKET_DATA_TYPE`, default 3 (delayed), because account B has no
   subscriptions and a live request on it only draws an error. Lines and snapshot limits are shared settings.
3. The selector is the socket, not a field of the proto. Gateway A is served on `$OT_HOME/run/ibkr.sock` exactly as today; gateway B, when configured, is served on `$OT_HOME/run/ibkr-b.sock` by
   a second grpc server of the same three services built over B's `IbGateway`. A caller picks the account by the socket it dials. So `contracts/proto` does not change, no `account` or `gateway` field is
   added to any message, the generated code does not change, and the interface file `pyside/src/optdesk/ibkr/api.py` does not change; there is no (interface) entry. Alternatives rejected:
   (a) a `gateway` field in every request: an interface change to a frozen `ibkr.v1` (docs/contracts/00-protocol.md), regenerated Go and Python, a Go client change, and every existing caller
   must say "A"; (b) a new proto version: the same cost and nothing here needs it. What the selector does not give, and this design does not claim: account B's account summary and positions need
   account and position requests, which are not on the allowlist and have no rpc. They are the later design that ADR 0003 foresees (an allowlist change in the owner-only `transport.py`, an account
   service in `contracts/proto`, an `ot` command that dials `ibkr-b.sock`). This design only makes B's session exist, supervised and visible, which that design needs first. The fallback of
   non-subscription information (chains, contract resolution) is available today to a caller that dials `ibkr-b.sock`; choosing it when A is down is the caller's rule, not pyside's (pyside holds no
   business rules).
4. Both gateways are read-only by the same guard. `install_guard` is passed to both `IbGateway` instances and `transport.py` is not edited (it is owner-only; nothing here needs it). The test of
   this design asserts the guard is installed on each client and an order id is refused on both (using the fakes the existing tests use). B uses the "Read-Only API" setting of IB Gateway like A.
   Account ids that the handshake delivers are not logged or stored for B either (the log whitelist of `transport.py` is per process, not per gateway).
5. Health follows the standard of the other crew services, so the keeper side stays simple (owner addition of 2026-11-07): agent-crew `rules/service.md` ("a local URL that answers when the service works"), `internal/crew/health_poll.go` (an HTTP GET, 2xx is healthy, a 3 s limit) and the `health` lines of the existing `services.toml` entries (most are `http://localhost:<port>/health`). So the contract is exactly `GET /health` on localhost only, 2xx when the service works and non-2xx otherwise, within 3 s, the port an env value; the JSON body is optional detail for the owner, not the contract. It is a small HTTP endpoint in the process, from the standard library. `http.server.ThreadingHTTPServer` on a daemon thread, bound to the loopback interface (the host is a
   constant of the code, not a setting), the port from `OT_IBKR_HEALTH_PORT`. Unset means no listener (tests and a bare `python -m optdesk.ibkr` are unchanged). `GET /health` answers JSON; any
   other path answers 404 and any other method 405. The handler reads one immutable snapshot that the supervisors replace (an assignment, no lock held across I/O), so a poll never touches the TWS
   connection and answers well inside crew's 3 s limit. Alternatives rejected: a grpc health service (crew polls an HTTP GET only, `internal/crew/health_poll.go`); an asyncio hand-written HTTP parser
   (more of our own code for the same result); aiohttp, FastAPI or any framework (a new direct dependency for one route; `uv.lock` holds no HTTP server library to reuse, so none is added).
   Own implementation, because the standard library covers one route and no maintained library is needed.
6. The response. Body (JSON, keys in this order): `status` (`"ok"` or `"down"`), `gateways` with the keys `a` and `b`, each an object of `state`, `generation` and `last_connect`. `state` is one of
   `connected`; `session_lost` (this process connected before and the session is down now, `generation` above 0); `not_logged_in` (no connect has succeeded since this process started: the gateway is not
   running, not logged in, or refuses the client; pyside cannot tell these apart and says so by one word); `disabled` (B only, when no B port is set). `generation` is the `IbGateway` counter (0 before the first connect). `last_connect`
   is the time of the latest `connect_ok` as ISO 8601 text in UTC taken from the `Clock` (so a test fixes it), or null. No account id, port or host appears in the body. Status 200 when gateway A's
   state is `connected`, else 503.
7. What B does to the code: nothing. B's state is reported and never changes the status code. Reasons: crew restarts the service when the poll fails, and a restart cannot log account B in; a 503
   for a B that is simply not logged in this week would keep crew restarting or marking a healthy A service red; B is the owner's second account and a fallback, A is the service's reason to exist (the
   gateway with the subscriptions). The owner reads B's state from the same body. Alternative rejected: 200 only when both are connected (it makes B's weekly login a precondition of the A service);
   a second status field or a second URL (crew polls one URL; the body already says it).
8. Supervision is shared by both gateways: `supervise` gains an optional observer (a `Board` that records the state words above) and keeps its behaviour and log events; the log lines gain a
   field `gateway` with the value `a` or `b` (the log whitelist of `transport.py` may refuse the field: then the lines stay as they are and the board is the only place of the name; the issue reports it rather than editing `transport.py`).
9. Start and stop. `boot = false` in the services.toml entry: the owner logs into the gateways, then runs `crew cap --apply optdesk-pyside deploy` (the flag before the service name; apply needs a
   terminal and is the owner's). Because the supervisors retry for ever, the service may be pulled up before B is logged in: A connected gives 200, B shows `not_logged_in` until the owner logs it in.
   The first poll after a start can be 503 until A's connect completes; this is the case where crew's health check should be allowed its startup grace (a crew setting, outside this repository).
10. Failure of the second bind: a port already in use for the health listener or a stale `ibkr-b.sock` of another kind is a `ConfigError` at start (exit status 2, as for the existing socket), not a silent
    skip. A stale socket file of the right kind is removed, as `prepare_socket` does for the first.

## Modules

No row of `docs/design/modules.md` changes, and no module is added. All paths are in the pyside row (`pyside/**`): `pyside/src/optdesk/ibkr/server.py`, `pyside/src/optdesk/ibkr/health.py` (new),
`pyside/tests/ibkr/test_health.py` (new) and `pyside/tests/ibkr/test_server.py`. Not touched: `pyside/src/optdesk/ibkr/transport.py` (owner-only), `pyside/src/optdesk/ibkr/api.py` and
`pyside/tests/contracts/test_ibkr_api.py` (the interface), `contracts/**` (contracts), `cmd/**` (ot), `pyside/pyproject.toml` and `pyside/uv.lock` (no new dependency). `docs/deploy/pyside.md` of the
runbook draft is a docs path; the runbook issue is updated to this design in its own right (see `## Left out`).

## Interface

No interface file changes: `api.py` unchanged, `contracts/proto` unchanged, so there is no (interface) entry and the issues below have no Design line of an interface change. The contracts a caller
or crew can see are these three, all outside the interface files:

- The environment keys (names; defaults in `## Decisions` 2 and 5): `OT_IBKR_GATEWAY_B_PORT`, `OT_IBKR_GATEWAY_B_CLIENT_ID`, `OT_IBKR_GATEWAY_B_MARKET_DATA_TYPE`, `OT_IBKR_HEALTH_PORT`; the
  existing keys keep their meaning. A new key that is set and is not a whole number above zero is a `ConfigError` naming the key only, as for the existing keys.
- The sockets: `$OT_HOME/run/ibkr.sock` (A, as today) and `$OT_HOME/run/ibkr-b.sock` (B, only when `OT_IBKR_GATEWAY_B_PORT` is set); both in the directory of mode 0700, files of mode 0600; each path
  shorter than 104 bytes.
- `GET /health` as in `## Decisions` 6:
  `{"status": "ok", "gateways": {"a": {"state": "connected", "generation": 1, "last_connect": "<UTC time>"}, "b": {"state": "not_logged_in", "generation": 0, "last_connect": null}}}`.
  The code is `health.py`: `Board` (records and returns the snapshot; takes a `Clock`), `render(snapshot)` returning the status code and the body bytes, and `start_health(board, port)` returning the
  running server (port 0 is allowed in this call so a test binds a free port; the config rejects 0).

## Issues

Items are offline: a named test run from `pyside/` after `uv sync --locked`, or a grep, without network, a live gateway or a fixed port. Tests use `pytest -n auto`, a fake clock and `tmp_path`;
none sleeps; none connects to a live broker; a test that starts the health server binds port 0.

### pyside-health-board

The status board and the `/health` handler
  Goal: `health.py` holds `Board`, `render` and `start_health` as in `## Interface`, and `supervise` reports the states of `## Decisions` 6 to an optional board without changing its behaviour.
  Files: pyside/src/optdesk/ibkr/health.py, pyside/src/optdesk/ibkr/server.py, pyside/tests/ibkr/test_health.py, pyside/tests/ibkr/test_server.py
  Design: docs/design/pyside-gateways-health.md#decisions
  Acceptance:
  1. From `pyside/`, `uv run pytest tests/ibkr/test_health.py tests/ibkr/test_server.py -n auto` passes, and `uv run ruff check` and `uv run mypy --strict src tests` print no error.
  2. `test_health.py` has tests named `test_connected_a_gives_200`, `test_session_lost_a_gives_503`, `test_not_logged_in_a_gives_503`, `test_b_state_never_changes_the_code` (B in each of its four words with A
     connected gives 200, and with A down gives 503), `test_b_disabled_by_default`, `test_last_connect_is_clock_time_in_utc` and `test_body_holds_no_port_or_host`.
  3. `test_health.py` also has `test_other_path_is_404`, `test_other_method_is_405` and `test_server_binds_loopback_only` (the socket name of the running server is the loopback interface, port 0 asked), each over a real
     request to the started server on a port the OS chose.
  4. `test_server.py` has a test that runs `supervise` with a fake gateway and a fake clock (connect fails twice, succeeds, then reports lost) and asserts the board goes `not_logged_in`, `connected` with
     `generation` 1 and a `last_connect`, then `session_lost`; and that `supervise` without a board still behaves as the existing tests assert.
  5. `grep -c -E '^(import|from) (aiohttp|fastapi|flask|starlette|uvicorn)' pyside/src/optdesk/ibkr/health.py` prints 0, `git diff --stat origin/main -- pyside/pyproject.toml pyside/uv.lock` prints nothing, and
     `git diff --name-only origin/main` equals the Files line.
  6. Every new file is under 600 lines (`wc -l`), and `server.py` stays under 600.
  7. The result matches Decisions 5 to 8 and the interface part of docs/design/pyside-gateways-health.md.
  Checked by CI: ci-pyside (ruff, mypy --strict, pytest -n auto), hygiene, and the `test` check.
  Pull request body: `Contracts: unchanged`; no new dependency (the standard library only).
  Depends on: none
  Size: M

### pyside-two-gateways (pyside)

Two gateways, two sockets, the health listener started
  Goal: with `OT_IBKR_GATEWAY_B_PORT` set, `serve` builds a second `IbGateway` with the guard, a second supervisor and a second grpc server on `ibkr-b.sock`; with `OT_IBKR_HEALTH_PORT` set it starts the health
  listener over the board of both; with neither set nothing differs from main.
  Files: pyside/src/optdesk/ibkr/server.py, pyside/tests/ibkr/test_server.py
  Design: docs/design/pyside-gateways-health.md#decisions
  Acceptance:
  1. From `pyside/`, `uv run pytest tests/ibkr/test_server.py -n auto` passes with `uv run ruff check` and `uv run mypy --strict src tests` printing no error.
  2. `test_server.py` has `test_load_config_without_b_is_unchanged` (the existing keys give the existing `Config`, B absent, health absent), `test_load_config_reads_b_and_health_keys` (the four new keys and their
     defaults: B client id equals the client id, B market data type 3), `test_bad_new_key_names_only_the_key` (zero, text and negative for each new key give a `ConfigError` whose message holds the key name and not
     the value) and `test_b_socket_path_is_next_to_the_first` (`ibkr-b.sock` under `$OT_HOME/run`, a path of 104 bytes or more is refused for both).
  3. `test_server.py` has `test_b_gets_the_guard` (the install function passed to the second `IbGateway` is `install_guard`, and with the real guard an order id is refused for the client of A and of B) and
     `test_b_down_does_not_stop_a` (a fake B whose connect always fails and a fake A that connects: A's board entry reaches `connected` and `serve` keeps running until the stop event; a tmp_path `OT_HOME`).
  4. `test_server.py` has `test_serve_without_b_binds_one_socket` and `test_serve_with_b_binds_two_sockets` (modes 0700 and 0600; both removed or replaced when stale; a file that is not a socket at
     `ibkr-b.sock` is a `ConfigError`), run with fake gateways, a fake clock and `tmp_path`.
  5. `grep -c -E 'reqPositions|reqAccount|reqExecutions|managedAccounts' pyside/src/optdesk/ibkr/server.py` prints 0; `git diff --name-only origin/main` equals the Files line; `git diff origin/main --stat -- pyside/src/optdesk/ibkr/transport.py contracts pyside/src/optdesk/ibkr/api.py` prints nothing.
  6. The result matches Decisions 1 to 4, 9 and 10 of docs/design/pyside-gateways-health.md.
  Checked by CI: ci-pyside, hygiene, and the `test` check.
  Pull request body: `Contracts: unchanged`; no new dependency.
  Depends on: pyside-health-board
  Size: M

## Proposed services.toml entry

The file is owner-only in agent-crew and is not changed here; the entry is the owner's to add, with the runbook of `docs/deploy/pyside.md` (updated, see `## Left out`). Values in angle brackets are
placeholders, never written in this repository. The simpler way adopted on 2026-11-07 runs `uv` directly, with no file under `deploy/`.

```toml
[services.optdesk-pyside]
way    = "host"                # pyside must reach the IB Gateways, GUI applications the owner starts and logs into
repo   = "andrewrong/optdesk"
boot   = false                 # the owner logs into the gateways first, then pulls the service up with crew
build  = ["<uv path>", "sync", "--locked", "--no-dev", "--project", "pyside"]
run    = ["<uv path>", "run", "--no-sync", "--project", "pyside", "python", "-m", "optdesk.ibkr"]
health = "http://localhost:<health port>/health"
env    = { OT_HOME = "<OT_HOME of ot>", OT_IBKR_GATEWAY_PORT = "<port of gateway A>", OT_IBKR_CLIENT_ID = "<client id>", OT_IBKR_GATEWAY_B_PORT = "<port of gateway B>", OT_IBKR_HEALTH_PORT = "<health port>" }
```

Optional keys the owner may add: `OT_IBKR_GATEWAY_B_CLIENT_ID`, `OT_IBKR_GATEWAY_B_MARKET_DATA_TYPE`, `OT_IBKR_GATEWAY_HOST`, `OT_IBKR_LINES`, `OT_IBKR_SNAPSHOTS`, `OT_IBKR_MARKET_DATA_TYPE`. The health
port is a value of the entry (it must differ from every other port on the machine, and crew's `health` URL and `OT_IBKR_HEALTH_PORT` carry the same number). The deploy is `crew cap --apply optdesk-pyside deploy`.
To run with gateway A only, leave out `OT_IBKR_GATEWAY_B_PORT`.

## Left out

- Account summary and position collection for account B (and for A): no allowlist entry, rpc or `ot` command exists for it. It is the later design that ADR 0003 foresees; it needs the owner-only
  `transport.py` (allowlist) and `contracts/proto` (a service, an interface change with an interface entry first). It builds on the sessions this design provides.
- Automatic fallback from A to B inside pyside: the caller dials `ibkr-b.sock` when it decides to; pyside keeps no routing rule.
- Any `ot` change: `ot run-once` keeps reading `$OT_HOME/run/ibkr.sock` only.
- The runbook issue draft (`docs/deploy/pyside.md`): it still says `boot = true` and "no health". It must be refreshed to this design (boot false, the health URL, the new keys, the two sockets, the
  meaning of `not_logged_in` against `session_lost`, "A connected is 200; B never changes the code") before it is registered; it is a docs issue, not an entry here.
- Startup grace for the first health poll, a restart policy on repeated 503, and starting or logging into IB Gateway from crew: crew's and the owner's.
- ADR 0003 says "the owner's dedicated IB Gateway session" (one). A note that a second, read-only gateway session exists is the owner's to add to the ADR; this design does not edit it.
- TLS, authentication or a non-loopback bind for `/health`: the listener is loopback only and the body holds no account id, port or host.
