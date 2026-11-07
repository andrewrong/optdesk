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
  process: there is no TCP listener and no health URL (the merged runbook, `docs/deploy/pyside.md`, issue 262, says so, with one gateway and `boot = true`; this design replaces both).
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
   pacer state per gateway inside one process, and the asyncio tasks isolate the failures. A crash of the process restarts both, and the failure of B's connection is no crash (every connect
   failure is caught in `supervise`).
2. Gateway B is optional and off by default. B exists only when `OT_IBKR_GATEWAY_B_PORT` is set. With it unset and `OT_IBKR_HEALTH_PORT` unset the process behaves as on main: one gateway, one socket, no
   listener. Host: both gateways share `OT_IBKR_GATEWAY_HOST` (both are on this machine). Client id: `OT_IBKR_GATEWAY_B_CLIENT_ID`, default the value of `OT_IBKR_CLIENT_ID` (a client id is per
   gateway, so the same number does not collide). Market data type of B: `OT_IBKR_GATEWAY_B_MARKET_DATA_TYPE`, default 3 (delayed), because account B has no subscriptions and a live request on it only
   draws an error. Lines and snapshot limits are shared settings.
3. The selector is the socket, not a field of the proto. Gateway A is served on `$OT_HOME/run/ibkr.sock` exactly as today; gateway B, when configured, is served on `$OT_HOME/run/ibkr-b.sock` by
   a second grpc server of the same three services built over B's `IbGateway`. A caller picks the account by the socket it dials. So `contracts/proto` does not change, no `account` or `gateway` field is
   added to any message, the generated code does not change, and `pyside/src/optdesk/ibkr/api.py` does not change; there is no (interface) entry. Alternatives rejected:
   (a) a `gateway` field in every request: an interface change to a frozen `ibkr.v1` (docs/contracts/00-protocol.md), regenerated Go and Python, a Go client change, and every existing caller
   must say "A"; (b) a new proto version: the same cost and nothing here needs it. Lock-in, accepted: a later account service must be served on both sockets, `ot` needs a socket choice, and the key
   names fix "gateway A and gateway B" (a third gateway is a rename). What the selector does not give, and this design does not claim: account B's account summary and positions need account and position
   requests, which are not on the allowlist and have no rpc. They are the next design (an allowlist change in the owner-only `transport.py`, an account service in `contracts/proto`, an `ot` command
   that dials `ibkr-b.sock`). This design only makes B's session exist, supervised and visible. The fallback of non-subscription information (chains, contract resolution) is available today to a caller
   that dials `ibkr-b.sock`; choosing it when A is down is the caller's rule, not pyside's (pyside holds no business rules).
4. Both gateways are read-only by the same guard. `install_guard` is passed to both `IbGateway` instances and `transport.py` is not edited (it is owner-only). The test builds a fake client object with
   `send` and `sendMsg` methods (no `IB(` and no order call, as `pyside-ibkr-transport.md` item 17 requires), passes it through the install function the factory gives each gateway and asserts an order id is
   refused for both. B uses the "Read-Only API" setting of IB Gateway like A. Account ids that the handshake delivers are not logged or stored for B either (the whitelist and scrub of `transport.py`
   are per process, not per gateway).
5. Health follows the standard of the other crew services, so the keeper side stays simple (owner addition of 2026-11-07): agent-crew `rules/service.md` ("a local URL that answers when the service
   works"), `internal/crew/health_poll.go` (an HTTP GET, 2xx is healthy, a 3 s limit) and the `health` lines of the existing `services.toml` entries (most are `http://localhost:<port>/health`). So the
   contract is exactly `GET /health` on localhost only, 2xx when the service works and non-2xx otherwise, within 3 s, the port an env value (`OT_IBKR_HEALTH_PORT`; unset means no listener); the JSON body is
   optional detail for the owner, not the contract.
6. "The service works" is "gateway A is connected" (owner decision of 2026-11-07: the health reflects the gateway). 200 when A is connected, 503 otherwise. Body, the simpler form: `{"a": true, "b": null}`
   where `a` is whether A is connected and `b` is whether B is connected, or null when no B is configured. The words "session lost" and "not logged in" are no longer in the body: the log lines
   `session_lost` and `connect_failed` already tell them apart. The body holds no account id, port or host. Alternatives rejected: a state word, a generation and a last-connect time per gateway (more
   words and a board that `supervise` must feed, for detail the owner can read in the log); 200 whenever the process is alive (it would say nothing about the gateway).
7. What B does to the code: nothing. B is reported and never changes the status code. Reason: B is the owner's second account and a fallback, A is the service's reason to exist (the gateway with the
   subscriptions), and B is logged in on the owner's schedule. What a 503 does, read from agent-crew (`health_poll.go`, `cap_host.go`, `rules/service.md`): crew does not restart the service on a failed
   poll; it opens a Fault after 3 failures in 5 minutes and closes it after a healthy poll. So the weekly logout of A (and IB's daily restart if it exceeds the window) opens a Fault: that is the wanted
   signal, not a defect. The settle of `crew cap --apply optdesk-pyside deploy` and of `restart` polls the same URL and a non-2xx fails the action, so gateway A is logged in before the deploy.
   Because a failed settle after an automatic deploy is rolled back, the entry carries `deploy = "owner"` (the optdesk project has no automatic deploy; the rule keeps it so for this service).
   Alternative rejected: 200 whenever the process and supervisors are alive, with the state only in the body (no Faults, no rollback, but the health would not reflect the gateway).
8. How health reads the gateways, so `supervise` is not edited. `health.py` takes callables: `a: Callable[[], bool]` and `b: Callable[[], bool] | None`. `serve` builds each as
   `asyncio.run_coroutine_threadsafe(gateway.status(), loop).result(timeout)` of `.connected`, with a timeout of 2 s (under crew's 3 s); a timeout or any exception counts as "not connected". This also covers a
   wedged event loop: the answer then is 503, not a stale 200. `Supervised`, `supervise` and its log lines are unchanged. The `gateway` name is not added to log fields (the whitelist `FIELDS` of the
   owner-only `transport.py` would drop it); lines of the two gateways are told apart by the generation and by the order of events, and a field for it is a later change of `transport.py`.
9. The listener. `http.server.ThreadingHTTPServer` on a thread, in a subclass that overrides `server_bind` to skip `socket.getfqdn` (it can stall for seconds on macOS) and `log_message` to write nothing (crew's
   poll would otherwise write a stderr line every 30 s). It binds the literal name `localhost` with an IPv4 socket (the name crew's `health` URL uses, which resolves to the loopback interface; the host is
   a constant of the code, not a setting; a name rather than a numeric address keeps the repository free of address shapes). `GET /health` answers JSON; any other path 404 and any other method 405.
   Own implementation, because the standard library covers one route; alternatives rejected: a grpc health service (crew polls HTTP only), a hand-written asyncio HTTP parser (more of our own code),
   aiohttp, FastAPI or any framework (a new direct dependency for one route; `uv.lock` holds no HTTP server library to reuse).
10. Seams and shutdown. `start_health(a, b, port)` returns a `HealthServer` with `port` (the bound port; `port=0` is allowed in this call so a test binds a free port, the config rejects 0) and a
    blocking `stop()`: `shutdown()` (it returns once `serve_forever` ends), `server_close()`, then `join()` of the thread (daemon threads for the request handlers). `serve` gains keyword parameters
    `make_gateway` (a factory `(host, port, client_id, market_data_type) -> Gateway`; default builds `IbGateway` with `install_guard`, lines, snapshots and the clock of `config`), `stop` (an
    `asyncio.Event`; when given, `serve` waits on it and installs no signal handler) and `on_ready` (called with the bound health port or None once the servers have started). `Config` gains `health_port: int | None`,
    so a test builds a `Config` with `health_port=0` and learns the port from `on_ready`. In its `finally`, `serve` calls `await asyncio.to_thread(health.stop)` before it stops the grpc servers.
11. Start and stop. `boot = false`: the owner logs into the gateways, then runs `crew cap --apply optdesk-pyside deploy` (the flag before the service name; apply needs a terminal and is the owner's). A
    first deploy needs gateway A logged in (decision 7). After that, because the supervisors retry for ever, the service may run with B not logged in: A connected gives 200 and `"b": false`. The first
    health poll after a plain start can be non-2xx until A connects.
12. Failure of a bind. A port already in use for the health listener, or a file that is not a socket at either socket path, is a `ConfigError` at start (exit status 2, as for the existing socket), not a
    silent skip. A stale socket file of the right kind is removed, as `prepare_socket` does for the first. The 104-byte limit of a socket path (macOS) is a failure of the bind that already exists
    for A; for B it is the same bind failure at start, and no new length check is added for either gateway (so nothing differs from main for A).

## Modules

No row of `docs/design/modules.md` changes, and no module is added. Pyside paths (the row `pyside/**`): `pyside/src/optdesk/ibkr/health.py` (new), `pyside/tests/ibkr/test_health.py` (new),
`pyside/src/optdesk/ibkr/server.py` and `pyside/tests/ibkr/test_server.py`. Docs path (the row `docs/**`): `docs/deploy/pyside.md`, the runbook merged as issue 262, rewritten by the third entry.
Not touched: `pyside/src/optdesk/ibkr/transport.py` (owner-only), `pyside/src/optdesk/ibkr/api.py` and `pyside/tests/contracts/test_ibkr_api.py` (the interface), `contracts/**` (contracts), `cmd/**` (ot),
`pyside/pyproject.toml` and `pyside/uv.lock` (no new dependency). The entries share no file: entry 1 writes `health.py` and its test, entry 2 writes `server.py` and its test and imports `health.py`.

## Interface

No interface file changes: `api.py` unchanged, `contracts/proto` unchanged, so there is no (interface) entry. The contracts a caller or crew can see are these three, all outside the interface files:

- The environment keys (names; defaults in decisions 2 and 5): `OT_IBKR_GATEWAY_B_PORT`, `OT_IBKR_GATEWAY_B_CLIENT_ID`, `OT_IBKR_GATEWAY_B_MARKET_DATA_TYPE`, `OT_IBKR_HEALTH_PORT`; the existing keys keep their
  meaning. A new key that is set and is not a whole number above zero is a `ConfigError` naming the key only, as for the existing keys.
- The sockets: `$OT_HOME/run/ibkr.sock` (A, as today) and `$OT_HOME/run/ibkr-b.sock` (B, only when `OT_IBKR_GATEWAY_B_PORT` is set), in the directory of mode 0700, files of mode 0600.
- `GET /health`, decisions 5 and 6: status 200 or 503 is the contract; the body is `{"a": true, "b": null}` in its form. The code is `health.py`: `render(a_connected, b_connected)` returning the status code and
  the body bytes, `probe` helpers as in decision 8, and `start_health(a, b, port)` returning the `HealthServer` of decision 10.

## Issues

Which entries run when: 1, then 2, then 3, in that order (`Order: after`). Items are offline: a named test run from `pyside/` after `uv sync --locked`, or a grep, without network, a live gateway or a fixed
port. Tests use `pytest -n auto`, a fake clock and, for a socket, `kit.sockdir.short_socket_dir` (a `tmp_path` is too long for a Unix socket on macOS); none sleeps; none connects to a live broker; a test
that starts the health server binds port 0.

### pyside-health-endpoint

The `/health` handler and the listener
  Goal: `health.py` holds `render`, the server subclass and `start_health` / `HealthServer` as in decisions 8 to 10; `server.py` is not touched.
  Files: pyside/src/optdesk/ibkr/health.py, pyside/tests/ibkr/test_health.py
  Design: docs/design/pyside-gateways-health.md#decisions
  Acceptance:
  1. From `pyside/`, `uv run pytest tests/ibkr/test_health.py -n auto` passes, and `uv run ruff check` and `uv run mypy --strict src tests` print no error.
  2. `test_health.py` has `test_a_connected_gives_200`, `test_a_down_gives_503`, `test_b_never_changes_the_code` (B true, false and absent, with A connected gives 200 and with A down gives 503),
     `test_b_absent_is_null_in_the_body`, `test_probe_error_counts_as_down` (a callable that raises or exceeds the timeout gives 503) and `test_body_holds_no_port_or_host`.
  3. `test_health.py` has `test_other_path_is_404`, `test_other_method_is_405`, `test_listener_binds_localhost_only` (the bound address is the loopback interface, port 0 asked) and
     `test_stop_ends_the_thread` (after `stop()` the thread is not alive and a new connection is refused), each over a real request to the started server on a port the OS chose.
  4. `grep -c -E 'getfqdn' pyside/src/optdesk/ibkr/health.py` prints 0 outside comments (`grep -v '^[[:space:]]*#'`), `grep -c -E 'def log_message' pyside/src/optdesk/ibkr/health.py` prints 1, and
     `grep -c -E '^(import|from) (aiohttp|fastapi|flask|starlette|uvicorn)' pyside/src/optdesk/ibkr/health.py` prints 0.
  5. `git diff --stat origin/main -- pyside/pyproject.toml pyside/uv.lock pyside/src/optdesk/ibkr/server.py` prints nothing, `git diff --name-only origin/main` lists exactly the two files of the Files line, and each is under 600 lines.
  6. The result matches decisions 5 to 10 and the interface part of docs/design/pyside-gateways-health.md.
  Checked by CI: ci-pyside (ruff, mypy --strict, pytest -n auto), hygiene, and the `test` check.
  Pull request body: `Contracts: unchanged`; no new dependency (the standard library only).
  Depends on: none. Order: first.
  Size: S

### pyside-two-gateways

Two gateways, two sockets, the health listener started by serve
  Goal: with `OT_IBKR_GATEWAY_B_PORT` set, `serve` builds a second gateway with the guard, a second supervisor and a second grpc server on `ibkr-b.sock`; with `OT_IBKR_HEALTH_PORT` set it starts the health
  listener over the two gateways and stops it on shutdown; with neither set nothing differs from main.
  Files: pyside/src/optdesk/ibkr/server.py, pyside/tests/ibkr/test_server.py
  Design: docs/design/pyside-gateways-health.md#decisions
  Acceptance:
  1. From `pyside/`, `uv run pytest tests/ibkr/test_server.py -n auto` passes, with `uv run ruff check` and `uv run mypy --strict src tests` printing no error.
  2. `test_server.py` has `test_load_config_without_b_is_unchanged` (the existing keys give the existing fields; B and health absent), `test_load_config_reads_b_and_health_keys` (the four new keys and their
     defaults: B client id equals the client id, B market data type 3) and `test_bad_new_key_names_only_the_key` (zero, text and a negative number for each new key give a `ConfigError` whose message holds the
     key name and not the value).
  3. `test_server.py` has `test_serve_without_b_binds_one_socket` and `test_serve_with_b_binds_two_sockets` (directory mode 0700, socket files 0600, stale sockets replaced, a file that is not a socket at
     `ibkr-b.sock` is a `ConfigError`), run through `serve` with `make_gateway` returning fakes, a fake clock, the `stop` event and an `OT_HOME` made with `kit.sockdir.short_socket_dir`.
  4. `test_server.py` has `test_serve_starts_and_stops_health` (a `Config` with `health_port=0`; `on_ready` gives the port; a real GET of `/health` answers 200 with fake A connected and 503 with it down;
     after `stop` is set and `serve` returns the thread is gone and the port refuses a connection) and `test_serve_without_health_port_starts_no_listener`.
  5. `test_server.py` has `test_b_gets_the_guard` (the factory is called for A and for B and the install function passed is `install_guard`; a fake client with `send` and `sendMsg`, passed through it, refuses an
     order id for both) and `test_b_down_does_not_stop_a` (a fake B whose connect always fails and a fake A that connects: the health answer is 200 with `"b": false` and `serve` keeps running until `stop`).
  6. `grep -c -E 'reqPositions|reqAccount|reqExecutions|managedAccounts' pyside/src/optdesk/ibkr/server.py` prints 0; `git diff origin/main --stat -- pyside/src/optdesk/ibkr/transport.py pyside/src/optdesk/ibkr/api.py contracts`
     prints nothing; `git diff --name-only origin/main` lists exactly the two files of the Files line; `server.py` stays under 600 lines.
  7. `grep -c -E '\b[0-9]{4,5}\b' pyside/src/optdesk/ibkr/server.py` prints 0 outside the `BACKOFF` and `POLL` constants (no port number in code or fixtures: ports are configuration).
  8. The result matches decisions 1 to 4 and 8 to 12 of docs/design/pyside-gateways-health.md.
  Checked by CI: ci-pyside, hygiene, and the `test` check.
  Pull request body: `Contracts: unchanged`; no new dependency.
  Depends on: pyside-health-endpoint. Order: after 1 (pyside-health-endpoint; `server.py` imports `health.py`).
  Size: M

### pyside-runbook-two-gateways

The runbook rewritten to this design
  Goal: `docs/deploy/pyside.md` (merged as issue 262: one gateway, `boot = true`, no health) describes what the two entries above deliver: gateway A required and gateway B optional through `OT_IBKR_GATEWAY_B_PORT`, `boot = false`, `deploy = "owner"`, the health URL `GET /health` on `OT_IBKR_HEALTH_PORT`, the services.toml entry of `## Proposed services.toml entry`, and the two sockets.
  Files: docs/deploy/pyside.md
  Design: docs/design/pyside-gateways-health.md#proposed-servicestoml-entry
  Acceptance:
  1. `grep -c -E 'boot += +false' docs/deploy/pyside.md` prints at least 1 and `grep -c -E 'boot += +true' docs/deploy/pyside.md` prints 0 (the old line is replaced, and the text does not quote it); `grep -c -E 'deploy += +"owner"' docs/deploy/pyside.md` and `grep -c -E 'health += +"http://localhost:' docs/deploy/pyside.md` print at least 1.
  2. `grep -c -F -e OT_IBKR_GATEWAY_B_PORT -e OT_IBKR_HEALTH_PORT -e OT_IBKR_GATEWAY_B_CLIENT_ID -e OT_IBKR_GATEWAY_B_MARKET_DATA_TYPE docs/deploy/pyside.md` prints at least 4, and `grep -c -F -e ibkr-b.sock -e 'GET /health' docs/deploy/pyside.md` prints at least 2.
  3. `grep -c -F 'crew cap --apply optdesk-pyside deploy' docs/deploy/pyside.md` and `grep -c -F 'crew cap --apply optdesk-pyside restart' docs/deploy/pyside.md` each print at least 1, and `grep -c -F 'deploy --apply' docs/deploy/pyside.md` prints 0.
  4. `grep -c -F -e session_lost -e connect_failed docs/deploy/pyside.md` prints at least 2, `grep -c -i -E 'log in.*(gateway A|A).*before|before.*(deploy|apply)' docs/deploy/pyside.md` prints at least 1 (gateway A is logged in before the deploy, because the settle needs 2xx), and `grep -c -i -E 'non-2xx|until A connects|Fault' docs/deploy/pyside.md` prints at least 2.
  5. `grep -c -E '/Users/|/Volumes/' docs/deploy/pyside.md` prints 0, `grep -c -E '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' docs/deploy/pyside.md` prints 0, and `git diff --name-only origin/main` equals the Files line.
  6. The result matches the `## Proposed services.toml entry` and decisions 5 to 7 and 11 of docs/design/pyside-gateways-health.md.
  Checked by CI: hygiene and the `test` check.
  Pull request body: `Contracts: unchanged`.
  Depends on: pyside-two-gateways (it documents what the entries above deliver). Order: after 2 (the two share no file; the order is prose).
  Size: S

## Proposed services.toml entry`, and the two sockets.
  Files: docs/deploy/pyside.md
  Design: docs/design/pyside-gateways-health.md#proposed-servicestoml-entry
  Acceptance:
  1. `grep -c -E 'boot += +false' docs/deploy/pyside.md` and `grep -c -E 'boot += +true' docs/deploy/pyside.md` print at least 1 and 0; `grep -c -E 'health += +"http://localhost:' docs/deploy/pyside.md` prints at least 1.
  2. `grep -c -F -e OT_IBKR_GATEWAY_B_PORT -e OT_IBKR_HEALTH_PORT -e OT_IBKR_GATEWAY_B_CLIENT_ID -e OT_IBKR_GATEWAY_B_MARKET_DATA_TYPE docs/deploy/pyside.md` prints at least 4, and `grep -c -F -e ibkr-b.sock -e 'GET /health' docs/deploy/pyside.md` prints at least 2.
  3. `grep -c -F 'crew cap --apply optdesk-pyside deploy' docs/deploy/pyside.md` and `grep -c -F 'crew cap --apply optdesk-pyside restart' docs/deploy/pyside.md` each print at least 1, and `grep -c -F 'deploy --apply' docs/deploy/pyside.md` prints 0.
  4. `grep -c -i -F -e not_logged_in -e session_lost docs/deploy/pyside.md` prints at least 2, and `grep -c -i -E 'non-2xx|until A connects' docs/deploy/pyside.md` prints at least 1 (the first poll may fail until gateway A connects).
  5. `grep -c -E '/Users/|/Volumes/' docs/deploy/pyside.md` prints 0, `grep -c -E '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' docs/deploy/pyside.md` prints 0, and `git diff --name-only origin/main` equals the Files line.
  6. The result matches the `## Proposed services.toml entry` and `## Decisions` 5 to 9 of docs/design/pyside-gateways-health.md.
  Checked by CI: hygiene and the `test` check.
  Pull request body: `Contracts: unchanged`.
  Depends on: pyside-two-gateways (prose: it documents what the two entries above deliver and lands after them; it shares no file with them)
  Size: S

## Proposed services.toml entry

The file is owner-only in agent-crew and is not changed here; the entry is the owner's to add, with the runbook `docs/deploy/pyside.md` (rewritten by the third entry). Values in angle brackets are
placeholders, never written in this repository. The simpler way adopted on 2026-11-07 runs `uv` directly, with no file under `deploy/`.

```toml
[services.optdesk-pyside]
way    = "host"                # pyside must reach the IB Gateways, GUI applications the owner starts and logs into
repo   = "andrewrong/optdesk"
boot   = false                 # the owner logs into the gateways first, then pulls the service up with crew
deploy = "owner"               # no automatic deploy after a merge, so no automatic rollback (the settle needs gateway A logged in)
build  = ["<uv path>", "sync", "--locked", "--no-dev", "--project", "pyside"]
run    = ["<uv path>", "run", "--no-sync", "--project", "pyside", "python", "-m", "optdesk.ibkr"]
health = "http://localhost:<health port>/health"
env    = { OT_HOME = "<OT_HOME of ot>", OT_IBKR_GATEWAY_PORT = "<port of gateway A>", OT_IBKR_CLIENT_ID = "<client id>", OT_IBKR_GATEWAY_B_PORT = "<port of gateway B>", OT_IBKR_HEALTH_PORT = "<health port>" }
```

Optional keys the owner may add: `OT_IBKR_GATEWAY_B_CLIENT_ID`, `OT_IBKR_GATEWAY_B_MARKET_DATA_TYPE`, `OT_IBKR_GATEWAY_HOST`, `OT_IBKR_LINES`, `OT_IBKR_SNAPSHOTS`, `OT_IBKR_MARKET_DATA_TYPE`. The health
port is a value of the entry (it must differ from every other port on the machine, and crew's `health` URL and `OT_IBKR_HEALTH_PORT` carry the same number). The deploy is `crew cap --apply optdesk-pyside deploy`.
To run with gateway A only, leave out `OT_IBKR_GATEWAY_B_PORT`.

## Left out

- Account summary and position collection for account B (and for A): no allowlist entry, rpc or `ot` command exists for it. It is the next design (ADR 0003 foresees it); it needs the owner-only
  `transport.py` (allowlist) and `contracts/proto` (a service, an interface change with an interface entry first). It builds on the sessions this design provides.
- Automatic fallback from A to B inside pyside: the caller dials `ibkr-b.sock` when it decides to; pyside keeps no routing rule.
- Any `ot` change: `ot run-once` keeps reading `$OT_HOME/run/ibkr.sock` only.
- Startup grace for the first health poll and a different Fault threshold: crew's. The Fault on A's weekly logout is the wanted signal (decision 7).
- A field that names the gateway in the log lines: it needs the whitelist of the owner-only `transport.py` (decision 8).
- A stamp for how old the answer is: the probe goes through the event loop with a timeout, so a wedged loop gives 503.
- ADR 0003 says "the owner's dedicated IB Gateway session" (one). A note that a second, read-only gateway session exists is the owner's to add to the ADR; this design does not edit it.
- TLS, authentication or a non-loopback bind for `/health`: the listener is loopback only and the body holds no account id, port or host.

## Adversarial

Read: `adversarial-gwh.md`, kept in the dispatcher's scratchpad (not in the repository), against the commit 16b2719 of this design.

1. Gap against the goal (the wiring of health in `serve` untestable, no gateway factory seam, thread shutdown unspecified). Adopted: decision 10 names `make_gateway`, `stop`, `on_ready` and `Config.health_port`
   (a test passes 0, the environment cannot), says how the thread ends (`shutdown`, `server_close`, `join`, through `asyncio.to_thread`), and entry 2 has `test_serve_starts_and_stops_health`.
2. Biggest unnamed risk (a weekly logout of A giving 503; the false claim that crew restarts). Adopted in part: decision 7 is corrected to what `health_poll.go` and `cap_host.go` do (a Fault after 3
   failures in 5 minutes, no restart; a settle that fails on non-2xx) and the entry carries `deploy = "owner"`; the runbook says to log in gateway A before the deploy. Not adopted: the reviewer's option of
   200 whenever the process is alive; the owner wants the health to reflect the gateway, so a Fault on A's logout is the wanted signal. The stale-200 risk of a wedged loop is closed by probing through the
   loop with a timeout (decision 8); `getfqdn` and the per-poll log line are removed (decision 9). The port collision is a start error (decision 12).
3. A simpler way (a body of two booleans; health reads status through callables). Adopted: decisions 6 and 8; `supervise` is not edited and the two entries share no file. The state words, generation and
   last-connect time are dropped; the log lines carry the distinction.
4. Lock-in (socket per gateway, `B` key names, body keys). Accepted and stated in decision 3; the cost is low and a later account design can serve both sockets.
5. Claims most likely false (crew restarts on a failed poll; "the runbook draft"; the 104-byte check against "nothing differs from main"). Adopted: decision 7 and the line of `## What main holds today` now say what
   crew does and name the merged runbook; decision 12 adds no length check, so nothing differs from main for A, and the old acceptance item that refused long paths for both is gone.
6. Likeliest slip in the drives (numbering and order; long socket paths; the guard test; fragile name-only diff; the `boot = true` grep; the dropped `gateway` log field). Adopted: the entries are `###` items that
   crew numbers by position, entry 2 says `Order: after 1` and entry 3 `Order: after 2`; the tests use `kit.sockdir.short_socket_dir`; the guard test uses a fake client with `send` and `sendMsg`; entry 3 says the old
   `boot` line is replaced and not quoted; decision 8 states that the `gateway` log field is dropped, with the reason. Not changed: the `git diff --name-only` check stays, since each entry names files that its change must touch.
