# Pyside gateways and health: gateways from a config file in one process, and a health URL for crew

Author: the optdesk dispatcher (draft)
Owner approved: (pending)
Date: 2026-11-07
Source: owner decisions of 2026-11-07 (dispatcher window; revised the same day by optdesk-617, see below): (1) the pyside IBKR service must be able to use two IB Gateways on the one machine, gateway A (the owner's live account, with market data subscriptions)
and gateway B (a second account, no market data subscriptions); B collects that account's own information and is a fallback source of non-subscription information when A is down; both are read-only;
(2) no start at boot: the owner logs into the gateways, then pulls the service up with crew; (3) a health link that crew polls. Read on main at 85a8ac5: `pyside/src/optdesk/ibkr/server.py`,
`ibgateway.py`, `model.py`, `timing.py`, `transport.py` (owner-only), `docs/design/modules.md`, `docs/design/pyside-ibkr-transport.md`, `docs/design/broker.md`,
`docs/adr/0003-market-data-architecture.md`, `docs/GLOSSARY.md`, `cmd/ot/runonce.go`, and from agent-crew `internal/crew/health_poll.go` and `services.toml`.
Owner decision optdesk-617 (2026-11-07), which replaces the fixed A/B keys of the first version of this design: nothing about gateways is hard-coded. Which gateways exist, how many, which account each
serves (by alias) and which information is collected through each come from a configuration file. The two gateways of the first decision are two entries of that file; ports are configuration values and appear nowhere in the repository.

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

1. One process, one connection and one supervisor per configured gateway, not one service instance per gateway. A gateway that is down or being logged into never blocks another (each supervisor retries on its
   own backoff). Alternative rejected: one instance of `python -m optdesk.ibkr` per gateway with its own services.toml entry. It needs a new env key for the socket name, one health URL and one log per
   instance and one deploy per gateway of one release, and gives nothing in return: ADR 0003 already makes the pacer state per gateway inside one process, and the asyncio tasks isolate the failures.
2. The gateways come from a TOML file, read with `tomllib` (standard library). The path is the one env key `OT_IBKR_GATEWAYS`; the file lives in the service's own config directory of the owner's machine and is
   not committed (the repository holds only an example with placeholders, decision 10). Shape, one table per gateway under `[[gateway]]`, and an optional top-level `default`:
   - `name` (required): a slug, `[a-z][a-z0-9-]{0,15}`, unique; it names the socket (`ibkr-<name>.sock`) and the key in `/health`.
   - `port` (required, a whole number above zero); `host` (default `localhost`); `client_id` (default 33; a client id is per gateway); `market_data_type` (default 2, frozen: live data when the market is open, the last frozen value when closed; owner decision 2026-11-10).
   - `account` (required): an alias, `[a-z][a-z0-9_-]{0,31}`, never a real account id. A value shaped like an account id (letters then 5 to 9 digits, the shape `scrub` of `transport.py` removes) is refused, and the
     key is named, not the value. The alias is only a label for who reads the file and for the later account design; pyside does not send it to the gateway.
   - `collect` (required): a non-empty list from the fixed vocabulary `market_data`, `account_summary`, `positions`. Only `market_data` is served today (the three services of `ibkr.v1`). The other two are declared now and
     are served by the later account design (`## Left out`); a file may name them already, and they change nothing until that design lands.
   - `required` (default false): whether this gateway's connection decides health (decision 6).
   - `default` (top level, optional): the name of the gateway that is also served on `ibkr.sock` (decision 4).
   Validation: at least one `[[gateway]]`; at least one with `required = true` (else "the service works" would mean nothing); no unknown key (a typo is an error, not a silent default); no duplicate name;
   no duplicate `(host, port, client_id)`; the entry that answers on `ibkr.sock` (the one `default` names, or the first entry when `default` is absent) has `market_data` in `collect`. A failure is `ConfigError` with a message that names the table (by name or position) and the key and
   never the value; the process exits with status 2, as for a bad env key. `OT_IBKR_LINES`, `OT_IBKR_SNAPSHOTS` and `OT_HOME` stay env keys and apply to every gateway.
3. No file, no change. When `OT_IBKR_GATEWAYS` is unset the process reads the existing keys (`OT_IBKR_GATEWAY_PORT` required, `OT_IBKR_GATEWAY_HOST`, `OT_IBKR_CLIENT_ID`, `OT_IBKR_MARKET_DATA_TYPE`) and runs the one
   gateway it runs today: one entry, named `default`, `collect = ["market_data"]`, `required = true`, served on `ibkr.sock` only. When the file key is set, the single-gateway keys are ignored (the runbook says so).
   The A/B keys of the first version (`OT_IBKR_GATEWAY_B_*`) are not introduced. Alternative rejected: keep the single-gateway keys as defaults that file entries inherit (two sources for one value; a changed key
   would silently change a file entry).
4. Sockets, by name. Every entry whose `collect` holds `market_data` is served by its own grpc server of the three services, on `$OT_HOME/run/ibkr-<name>.sock`. One entry also answers on `$OT_HOME/run/ibkr.sock`, the
   path `ot` reads and builds (`cmd/ot/runonce.go` builds `OT_HOME/run/ibkr.sock` and nothing else, and it treats it as the market-data source): the entry named by `default`, or the first entry of the file when
   `default` is absent. The choice: an explicit key, because an order of entries must not decide which account `ot` collects market data from (a reordered file would silently switch it), and the first entry as the
   fallback so a small file needs no key. That server gets two ports (`add_insecure_port` twice), so there is no extra server for it. An entry without `market_data` has a session and a health entry and no
   socket. In the no-file case there is only `ibkr.sock`. The selector is the socket: `contracts/proto` does not change, no `account` or `gateway` field is added to any message, the generated code does not
   change, and `pyside/src/optdesk/ibkr/api.py` does not change, so there is no (interface) entry. Alternatives rejected: (a) a `gateway` field in every request: an interface change to a frozen `ibkr.v1`
   (docs/contracts/00-protocol.md), regenerated Go and Python, a Go client change, every caller must name one; (b) a new proto version: the same cost and nothing here needs it. Lock-in, accepted: a later account
   service is served on the sockets of the entries whose `collect` names it, and `ot` needs a socket choice. The fallback of non-subscription information (chains, contract resolution) from another gateway is
   available today to a caller that dials that gateway's socket; choosing it when the default is down is the caller's rule, not pyside's (pyside holds no business rules).
5. All gateways are read-only by the same guard. `install_guard` is installed on every gateway's client and `transport.py` is not edited (it is owner-only). The test builds a fake client object with `send` and
   `sendMsg` methods (no `IB(` and no order call, as `pyside-ibkr-transport.md` item 17 requires), passes it through the install function the factory gives each gateway and asserts an order id is refused for each of
   two entries. Each gateway uses the "Read-Only API" setting of IB Gateway. Account ids that the handshake delivers are not logged or stored for any gateway (the whitelist and scrub of `transport.py` are per process).
6. Health follows the standard of the other crew services, so the keeper side stays simple (owner addition of 2026-11-07): agent-crew `rules/service.md` ("a local URL that answers when the service works"),
   `internal/crew/health_poll.go` (an HTTP GET, 2xx is healthy, a 3 s limit) and the `health` lines of the existing `services.toml` entries (most are `http://localhost:<port>/health`). The contract is exactly
   `GET /health` on localhost only, 2xx when the service works and non-2xx otherwise, within 3 s, the port an env value (`OT_IBKR_HEALTH_PORT`; unset means no listener). "The service works" is: every gateway with
   `required = true` is connected. 200 then, else 503. The body is optional detail: `{"gateways": {"<name>": true, "<name>": false}}`, one key per configured gateway, `true` when connected; no account alias, account
   id, port or host appears in it. A gateway that is not `required` is reported and never changes the status code: it is logged in on the owner's schedule, and a down fallback must not open a Fault.
7. What a 503 does, read from agent-crew (`health_poll.go`, `cap_host.go`, `rules/service.md`): crew does not restart the service on a failed poll; it opens a Fault after 3 failures in 5 minutes and closes it after
   a healthy poll. So the weekly logout of a required gateway (and IB's daily restart if it exceeds the window) opens a Fault: that is the wanted signal, not a defect. The settle of `crew cap --apply
   optdesk-pyside deploy` and of `restart` polls the same URL and a non-2xx fails the action, so the required gateways are logged in before the deploy. Because a failed settle after an automatic deploy is rolled
   back, the entry carries `deploy = "owner"` (the optdesk project has no automatic deploy; the rule keeps it so for this service). Alternative rejected: 200 whenever the process and supervisors are alive, with the
   state only in the body (no Faults, no rollback, but the health would not reflect the gateway).
8. How health reads the gateways, so `supervise` is not edited. `health.py` takes a mapping from name to a pair: `required` and a plain callable `() -> bool`, and owns the one deadline, `PROBE_DEADLINE_S = 2.0`
   (under crew's 3 s). On each request the probes run concurrently (one worker thread per probe, `concurrent.futures.wait` with the deadline for all of them together), so any number of hanging probes costs one
   deadline, not one each; a probe that raises or has not answered at the deadline counts as "not connected". `serve` builds each callable as `asyncio.run_coroutine_threadsafe(gateway.status(), loop).result(PROBE_DEADLINE_S)`
   of `.connected` (the constant is imported from `health.py`, so the number lives in one place). A wedged event loop therefore gives 503, not a stale 200. `Supervised`, `supervise` and its log lines are unchanged. No
   `gateway` name is added to log fields (the whitelist `FIELDS` of the owner-only `transport.py` would drop it); a field for it is a later change of `transport.py`.
9. The listener. `http.server.ThreadingHTTPServer` on a thread, in a subclass that overrides `server_bind` to skip `socket.getfqdn` (it can stall for seconds on macOS) and `log_message` to write nothing (crew's poll
   would otherwise write a stderr line every 30 s). It binds the literal name `localhost` with an IPv4 socket (the name crew's `health` URL uses, which resolves to the loopback interface; the host is a constant of the
   code, not a setting; a name rather than a numeric address keeps the repository free of address shapes). `GET /health` answers JSON; any other path 404 and any other method 405. Own implementation, because the
   standard library covers one route; alternatives rejected: a grpc health service (crew polls HTTP only), a hand-written asyncio HTTP parser (more of our own code), aiohttp, FastAPI or any framework (a new direct
   dependency for one route; `uv.lock` holds no HTTP server library to reuse).
10. The example. `pyside/examples/gateways.example.toml` is a template with two sample entries (names `first` and `second`, aliases `alias_one` and `alias_two`). It must parse through the real loader, so its ports
    are the placeholder numbers 1 and 2 with a comment "replace with the port of your gateway"; no real port is written. A test reads it through the loader (no network) and asserts it is valid. The runbook (last
    entry) explains copying it to the service config directory and filling it in.
11. Seams and shutdown. `start_health(probes, port, wait=concurrent.futures.wait)` (the `wait` argument is the seam a test replaces; it is never set outside tests) returns a `HealthServer` with `port` (the bound port; `port=0` is allowed in this call so a test binds a free port, the config rejects 0) and a blocking `stop()`:
    `shutdown()` (it returns once `serve_forever` ends), `server_close()`, then `join()` of the thread (daemon threads for the request handlers). `serve` gains keyword parameters `make_gateway` (a factory
    `(spec: GatewaySpec) -> Gateway`; the default builds `IbGateway` with `install_guard`, the lines and snapshots of the config and the clock), `stop` (an `asyncio.Event`; when given, `serve` waits on it and installs no
    signal handler) and `on_ready` (called with the bound health port or None once the servers have started). `Config` holds `gateways: tuple[GatewaySpec, ...]`, `default: str` and `health_port: int | None`, so a test builds
    a `Config` with `health_port=0` and learns the port from `on_ready`. In its `finally`, `serve` calls `await asyncio.to_thread(health.stop)` before it stops the grpc servers.
12. Start and stop. `boot = false`: the owner logs into the gateways, then runs `crew cap --apply optdesk-pyside deploy` (the flag before the service name; apply needs a terminal and is the owner's). A first deploy needs
    the required gateways logged in (decision 7). After that, because the supervisors retry for ever, the service may run with a non-required gateway not logged in: health stays 2xx and its key is `false`. The first
    health poll after a plain start can be non-2xx until the required gateways connect.
13. Failure of a bind. A file that is not a socket at a socket path is a `ConfigError` raised by `prepare_socket`, and a health port already in use is the `OSError` of the bind; both end `serve` as the existing
    socket errors do on main (a traceback, no silent skip). This design adds no exit-status promise for them: status 2 stays what `main` gives for an error of `load_config`, which now includes every error of the
    gateways file. A stale socket file of the right kind is removed, as `prepare_socket` does. The 104-byte limit of a socket path (macOS) is a failure of the bind that already exists; no new length check is added.

## Modules

No row of `docs/design/modules.md` changes, and no module is added. Pyside paths (the row `pyside/**`): `pyside/src/optdesk/ibkr/gateways.py` (new), `pyside/src/optdesk/ibkr/health.py` (new),
`pyside/src/optdesk/ibkr/server.py`, `pyside/tests/ibkr/test_gateways.py` (new), `pyside/tests/ibkr/test_health.py` (new), `pyside/tests/ibkr/test_server.py` and `pyside/examples/gateways.example.toml` (new).
Docs path (the row `docs/**`): `docs/deploy/pyside.md`, the runbook merged as issue 262, rewritten by the last entry. Not touched: `pyside/src/optdesk/ibkr/transport.py` (owner-only),
`pyside/src/optdesk/ibkr/api.py` and `pyside/tests/contracts/test_ibkr_api.py` (the interface), `contracts/**` (contracts), `cmd/**` (ot), `pyside/pyproject.toml` and `pyside/uv.lock` (no new dependency).
Cut: the three Python pieces (the file loader, the health endpoint, the wiring in `server.py`) are one pyside entry. They are one module, they share the types `GatewaySpec` and the probe signature, and no
interface file of the module (`api.py`, declarations of the gRPC `Session`) carries them: an interface entry would have to edit `api.py` for types that are not part of the service interface, and a split into
siblings leaves the wiring entry unable to be written or verified without the unmerged code of the others. The runbook is a second module (docs) and follows the first entry.

## Interface

No interface file changes: `api.py` unchanged, `contracts/proto` unchanged, so there is no (interface) entry. The pieces below are internal to the pyside module (owned by it, used by no other module) and are
built in the one entry; the contracts a caller, crew or the owner can see are:

- The environment keys: `OT_IBKR_GATEWAYS` (path of the TOML file; unset gives the single-gateway behaviour of main), `OT_IBKR_HEALTH_PORT` (a whole number above zero; unset means no listener); the existing keys keep
  their meaning (`OT_IBKR_GATEWAY_PORT` and its companions only when `OT_IBKR_GATEWAYS` is unset). A key that is set and is not usable is a `ConfigError` naming the key only.
- The gateways file of decision 2: `default`, and `[[gateway]]` with `name`, `port`, `host`, `client_id`, `market_data_type`, `account`, `collect`, `required`. Code: `gateways.py` with `GatewaySpec` (a frozen
  dataclass of those fields), `load_gateways(path) -> tuple[tuple[GatewaySpec, ...], str]` (the specs and the default name) and `from_env(env) -> tuple[GatewaySpec, ...]` for the no-file case.
- The sockets: `$OT_HOME/run/ibkr-<name>.sock` per entry with `market_data`, and `$OT_HOME/run/ibkr.sock` for the default entry (or the only gateway of the no-file case); directory mode 0700, files 0600.
- `GET /health`, decisions 6, 8 and 9: status 200 or 503 is the contract; the body is `{"gateways": {"<name>": true|false}}`. Code: `health.py` with `PROBE_DEADLINE_S`, `render(states, required)` returning the
  status code and the body bytes, and `start_health(probes, port, wait=...)` returning the `HealthServer` of decision 11.

## Issues

Which entries run when: 1, then 2 (`Order: after 1`; the runbook documents what entry 1 delivers and shares no file with it), then 3 (`pyside-frozen-default`, `Order: after 1`, shares no file with 2). Entries 1 and 2 landed before the default of decision 2 became 2, so entry 3 carries that default into the code and entry 2 is reopened for item 7. Items are offline: a named test run from `pyside/` after `uv sync --locked`, or a grep, without network,
a live gateway or a fixed port. Tests use `pytest -n auto`, a fake clock and, for a socket, `kit.sockdir.short_socket_dir` (a `tmp_path` is too long for a Unix socket on macOS); a file the test writes goes in `tmp_path`; none
sleeps; none connects to a live broker; a test that starts the health server binds port 0.

### pyside-gateways

The gateways file, one supervisor and socket per gateway, and the health endpoint
  Goal: `gateways.py` reads and validates the file of decision 2 and builds the no-file entry of decision 3; `health.py` holds the handler and listener of decisions 6, 8, 9 and 11; `serve` builds one gateway and one supervisor per entry, serves the
  sockets of decision 4, starts the health listener over the probes of decision 8 and stops it on shutdown; `load_config` reads the file named by `OT_IBKR_GATEWAYS` or, when unset, the existing keys; with neither the file key nor the health key set nothing differs from main; `pyside/examples/gateways.example.toml` is a valid template.
  Files: pyside/src/optdesk/ibkr/gateways.py, pyside/src/optdesk/ibkr/health.py, pyside/src/optdesk/ibkr/server.py, pyside/tests/ibkr/test_gateways.py, pyside/tests/ibkr/test_health.py, pyside/tests/ibkr/test_server.py, pyside/examples/gateways.example.toml
  Design: docs/design/pyside-gateways-health.md#decisions
  Acceptance:
  1. From `pyside/`, `uv run pytest tests/ibkr -n auto` passes, and `uv run ruff check` and `uv run mypy --strict src tests` print no error.
  2. `test_gateways.py` has `test_two_entries_with_defaults` (host `localhost`, client id 33, market data type 2, `required` false), `test_first_entry_is_default_without_key`, `test_default_key_names_an_entry`,
     `test_default_without_market_data_is_error`, `test_first_entry_without_market_data_and_no_default_is_error` and `test_from_env_is_one_default_entry` (the existing keys give one entry named `default`, `market_data`,
     required; a missing port names `OT_IBKR_GATEWAY_PORT`).
  3. `test_gateways.py` has one test per failure, each asserting a `ConfigError` whose message holds the key and not the value: `test_unknown_key`, `test_no_gateway`, `test_none_required`, `test_duplicate_name`,
     `test_duplicate_host_port_client`, `test_bad_name_slug`, `test_bad_port` (zero, text, negative), `test_bad_collect_word`, `test_empty_collect`, `test_account_shaped_like_an_id` (the shape built from two string pieces at
     run time so no file carries it) and `test_missing_account`; and `test_example_file_is_valid` (reads `pyside/examples/gateways.example.toml` through `load_gateways`: two entries, one `required`).
  4. `test_health.py` has `test_all_required_connected_gives_200`, `test_a_required_down_gives_503`, `test_not_required_never_changes_the_code` (a not-required gateway up and down, with the required one up gives 200 and
     with it down gives 503), `test_body_has_one_key_per_gateway`, `test_body_holds_no_port_host_or_alias` and `test_probe_that_raises_counts_as_down`.
  5. `test_health.py` has `test_probes_run_concurrently` (two probes that each wait on a two-party barrier with a timeout: they pass only if both run at once) and `test_blocked_probe_counts_down_at_the_deadline` (a probe
     blocked on an event the test sets only at the end, and a `wait` argument that returns every probe as not done at once, as the deadline would: the answer is 503 and the gateway is `false`; no sleep).
  6. `test_health.py` has `test_other_path_is_404`, `test_other_method_is_405`, `test_listener_binds_localhost_only` (the bound address is the loopback interface, port 0 asked) and `test_stop_ends_the_thread` (after
     `stop()` the thread is not alive and a new connection is refused), each over a real request to the started server on a port the OS chose.
  7. `test_server.py` has `test_load_config_without_file_is_unchanged` (the existing keys give one gateway `default` on `ibkr.sock`; the existing assertions of the config tests hold for its fields),
     `test_load_config_reads_the_file` (`OT_IBKR_GATEWAYS` in `tmp_path`; `OT_IBKR_GATEWAY_PORT` is then not required), `test_bad_health_key_names_only_the_key` (zero, text and a negative number) and
     `test_main_exits_2_on_a_bad_gateways_file` (through `main`, with a file that has no gateway).
  8. `test_server.py` has `test_serve_sockets_per_name_and_default` (two entries: `ibkr-<name>.sock` for each and `ibkr.sock` for the default; with no `default` key the first entry answers on it; an entry without
     `market_data` has no socket; directory mode 0700, files 0600; a stale socket is replaced; a file that is not a socket at a socket path is a `ConfigError`), run through `serve` with `make_gateway` returning fakes, a
     fake clock, the `stop` event and an `OT_HOME` made with `kit.sockdir.short_socket_dir`; `test_one_supervisor_per_entry` (three entries with fake gateways: each fake gets `open_session` called); and
     `test_not_required_down_does_not_stop_the_others` (a fake whose connect always fails, a second that connects: health is 200 with that key `false`, and `serve` keeps running until `stop`).
  9. `test_server.py` has `test_serve_starts_and_stops_health` (a `Config` with `health_port=0`; `on_ready` gives the port; a real GET of `/health` answers 200 with the required fake connected and 503 with it down; after
     `stop` is set and `serve` returns the thread is gone and the port refuses a connection), `test_serve_without_health_port_starts_no_listener` and `test_every_gateway_gets_the_guard` (the factory is called per
     entry and the install function passed is `install_guard`; a fake client with `send` and `sendMsg`, passed through it, refuses an order id for each of two entries).
  10. `grep -v '^[[:space:]]*#' pyside/src/optdesk/ibkr/health.py | grep -c getfqdn` prints 0, `grep -c -E 'def log_message' pyside/src/optdesk/ibkr/health.py` prints 1, `grep -c -E '^(import|from) (aiohttp|fastapi|flask|starlette|uvicorn|tomli|yaml)' pyside/src/optdesk/ibkr/health.py pyside/src/optdesk/ibkr/gateways.py` prints 0 for each file,
      `grep -c -E 'reqPositions|reqAccount|reqExecutions|managedAccounts' pyside/src/optdesk/ibkr/server.py` prints 0, and `grep -c -E 'OT_IBKR_GATEWAY_B' pyside/src/optdesk/ibkr/server.py pyside/tests/ibkr/test_server.py` prints 0 for each file.
  11. `git diff origin/main --stat -- pyside/pyproject.toml pyside/uv.lock pyside/src/optdesk/ibkr/transport.py pyside/src/optdesk/ibkr/api.py contracts` prints nothing; `git diff --name-only origin/main` lists exactly the
      seven files of the Files line; each new file and `server.py` stay under 600 lines.
  12. The result matches decisions 1 to 13 and the interface part of docs/design/pyside-gateways-health.md, judged from the code and docs/design/pyside-gateways-health.md.
  Checked by CI: ci-pyside (ruff, mypy --strict, pytest -n auto), hygiene, and the `test` check.
  Pull request body: `Contracts: unchanged`; no new dependency (`tomllib` and `http.server` are the standard library). Size L, stated here: seven files, one module, because the pieces share their types (see `## Modules`).
  Depends on: none.
  Size: L

### pyside-runbook-gateways

The runbook rewritten to this design
  Goal: `docs/deploy/pyside.md` (merged as issue 262: one gateway, `boot = true`, no health) describes what the entries above deliver: the gateways file and how to fill in the example, `OT_IBKR_GATEWAYS`, the no-file fallback,
  `boot = false`, `deploy = "owner"`, the health URL `GET /health` on `OT_IBKR_HEALTH_PORT`, the services.toml entry of `## Proposed services.toml entry`, and the sockets.
  Files: docs/deploy/pyside.md
  Design: docs/design/pyside-gateways-health.md#proposed-servicestoml-entry
  Acceptance:
  1. `grep -c -E 'boot += +false' docs/deploy/pyside.md` prints at least 1 and `grep -c -E 'boot += +true' docs/deploy/pyside.md` prints 0 (the old line is replaced, and the text does not quote it); `grep -c -E 'deploy += +"owner"' docs/deploy/pyside.md` and `grep -c -E 'health += +"http://localhost:' docs/deploy/pyside.md` print at least 1.
  2. `grep -c -F -e OT_IBKR_GATEWAYS -e OT_IBKR_HEALTH_PORT -e gateways.example.toml docs/deploy/pyside.md` prints at least 3, `grep -c -F -e '[[gateway]]' -e collect -e required docs/deploy/pyside.md` prints at least 3,
     and `grep -c -F -e 'ibkr-<name>.sock' -e 'GET /health' docs/deploy/pyside.md` prints at least 2; `grep -c -F 'OT_IBKR_GATEWAY_B' docs/deploy/pyside.md` prints 0.
  3. `grep -c -F 'crew cap --apply optdesk-pyside deploy' docs/deploy/pyside.md` and `grep -c -F 'crew cap --apply optdesk-pyside restart' docs/deploy/pyside.md` each print at least 1, and `grep -c -F 'deploy --apply' docs/deploy/pyside.md` prints 0.
  4. `grep -c -F -e session_lost -e connect_failed docs/deploy/pyside.md` prints at least 2, `grep -c -i -E 'before.*(deploy|apply)' docs/deploy/pyside.md` prints at least 1 (the required gateways are logged in before the
     deploy, because the settle needs 2xx), and `grep -c -i -E 'non-2xx|until the required|Fault' docs/deploy/pyside.md` prints at least 2.
  5. `grep -c -E '/Users/|/Volumes/' docs/deploy/pyside.md` prints 0, `grep -c -E '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' docs/deploy/pyside.md` prints 0, and `git diff --name-only origin/main` equals the Files line.
  6. The result matches the `## Proposed services.toml entry` and decisions 2 to 4, 6, 7 and 12 of docs/design/pyside-gateways-health.md, judged from the code and docs/design/pyside-gateways-health.md.
  7. `grep -c -F 'market_data_type` to 2' docs/deploy/pyside.md` prints at least 1 (a gateway entry and the no-file fallback both say 2, frozen) and `grep -c -F 'market_data_type` to 1' docs/deploy/pyside.md` prints 0; the text of `OT_IBKR_MARKET_DATA_TYPE` says default 2 and that the variable is an override.
  Checked by CI: hygiene and the `test` check.
  Pull request body: `Contracts: unchanged`.
  Depends on: pyside-gateways (it documents what that entry delivers). Order: after 1 (no shared file; the order is prose).
  Size: S

### pyside-frozen-default

The market-data type defaults to 2 (frozen) in the code
  Goal: the default `market_data_type` of a gateway entry, of the no-file fallback (`OT_IBKR_MARKET_DATA_TYPE`) and of the `IbGateway` constructor is 2, as decision 2 says; a value in the file or in the variable still wins. Nothing else changes.
  Files: pyside/src/optdesk/ibkr/gateways.py, pyside/src/optdesk/ibkr/ibgateway.py, pyside/tests/ibkr/test_gateways.py, pyside/tests/ibkr/test_server.py, pyside/tests/ibkr/test_ibgateway.py
  Design: docs/design/pyside-gateways-health.md#decisions
  Acceptance:
  1. `grep -c -E '^DEFAULT_MARKET_DATA_TYPE = 2$' pyside/src/optdesk/ibkr/gateways.py` prints 1 and `grep -c -E 'market_data_type: int = 2' pyside/src/optdesk/ibkr/ibgateway.py` prints 1; `grep -c -E 'market_data_type: int = 1' pyside/src/optdesk/ibkr/ibgateway.py` prints 0.
  2. `test_gateways.py::test_two_entries_with_defaults` asserts market data type 2 for an entry without the key, and a test `test_explicit_market_data_type_wins` asserts that an entry with `market_data_type = 3` reads 3; `test_server.py::test_config_defaults` asserts 2 for the no-file fallback and a test `test_env_market_data_type_overrides` asserts that `OT_IBKR_MARKET_DATA_TYPE=1` gives 1; `test_ibgateway.py` has `test_default_market_data_type_is_frozen` (a gateway built without the argument sets 2 on the fake once, after the guard and the connect).
  3. `git diff --name-only origin/main` equals the Files line; no new file; `pyside/src/optdesk/ibkr/transport.py`, `api.py` and `contracts` are unchanged.
  4. The result matches decision 2 and the Configuration paragraph of docs/design/pyside-ibkr-transport.md, judged from the code and those documents.
  Checked by CI: ci-pyside (ruff, mypy --strict, pytest -n auto), hygiene, and the `test` check.
  Pull request body: `Contracts: unchanged`; no new dependency.
  Depends on: pyside-gateways (landed). Order: after 1.
  Size: S

## Proposed services.toml entry

The file is owner-only in agent-crew and is not changed here; the entry is the owner's to add, with the runbook `docs/deploy/pyside.md` (rewritten by the last entry). Values in angle brackets are placeholders, never
written in this repository. The simpler way adopted on 2026-11-07 runs `uv` directly, with no file under `deploy/`. The gateways are not in the entry: they are in the file that `OT_IBKR_GATEWAYS` names, which the owner
makes from `pyside/examples/gateways.example.toml` and keeps in the service's config directory.

```toml
[services.optdesk-pyside]
way    = "host"                # pyside must reach the IB Gateways, GUI applications the owner starts and logs into
repo   = "andrewrong/optdesk"
boot   = false                 # the owner logs into the gateways first, then pulls the service up with crew
deploy = "owner"               # no automatic deploy after a merge, so no automatic rollback (the settle needs the required gateways logged in)
build  = ["<uv path>", "sync", "--locked", "--no-dev", "--project", "pyside"]
run    = ["<uv path>", "run", "--no-sync", "--project", "pyside", "python", "-m", "optdesk.ibkr"]
health = "http://localhost:<health port>/health"
env    = { OT_HOME = "<OT_HOME of ot>", OT_IBKR_GATEWAYS = "<path of gateways.toml in the service config dir>", OT_IBKR_HEALTH_PORT = "<health port>" }
```

The gateways file, from the example (names, aliases and ports are the owner's values; the alias is a label, never an account id):

```toml
default = "<name of the gateway ot reads on ibkr.sock>"

[[gateway]]
name = "<name>"
port = <port of this gateway>
account = "<alias>"
collect = ["market_data"]
required = true

[[gateway]]
name = "<name>"
port = <port of this gateway>
account = "<alias>"
collect = ["market_data", "account_summary", "positions"]   # the last two are served by the later account design
required = false
```

Optional keys the owner may add to the entry: `OT_IBKR_LINES`, `OT_IBKR_SNAPSHOTS`. To run one gateway as today, leave out `OT_IBKR_GATEWAYS` and set `OT_IBKR_GATEWAY_PORT` and `OT_IBKR_CLIENT_ID` instead. The health
port is a value of the entry (it must differ from every other port on the machine; crew's `health` URL and `OT_IBKR_HEALTH_PORT` carry the same number). The deploy is `crew cap --apply optdesk-pyside deploy`, the restart
`crew cap --apply optdesk-pyside restart`.

## Left out

- Serving the account summary and the positions of any gateway: no allowlist entry, rpc or `ot` command exists for it. It is the next design (ADR 0003 foresees it); it needs the owner-only `transport.py` (allowlist)
  and `contracts/proto` (a service, an interface change with an interface entry first). The `collect` words `account_summary` and `positions` are declared now and change nothing until that design lands.
- Automatic fallback from one gateway to another inside pyside: the caller dials another gateway's socket when it decides to; pyside keeps no routing rule.
- Any `ot` change: `ot run-once` keeps reading `$OT_HOME/run/ibkr.sock` only.
- Startup grace for the first health poll and a different Fault threshold: crew's. The Fault on a required gateway's weekly logout is the wanted signal (decision 7).
- A field that names the gateway in the log lines: it needs the whitelist of the owner-only `transport.py` (decision 8).
- A stamp for how old the answer is: the probe goes through the event loop with a timeout, so a wedged loop gives 503.
- Reloading the gateways file without a restart: a change of the file is a restart (`crew cap --apply optdesk-pyside restart`).
- ADR 0003 says "the owner's dedicated IB Gateway session" (one). A note that further read-only gateway sessions exist is the owner's to add to the ADR; this design does not edit it.
- TLS, authentication or a non-loopback bind for `/health`: the listener is loopback only and the body holds no account id, alias, port or host.

## Adversarial

Read: `adversarial-gwh.md`, kept in the dispatcher's scratchpad (not in the repository), against the commit 16b2719 of this design.

1. Gap against the goal (the wiring of health in `serve` untestable, no gateway factory seam, thread shutdown unspecified). Adopted: decision 11 names `make_gateway`, `stop`, `on_ready` and `Config.health_port`
   (a test passes 0, the environment cannot), says how the thread ends (`shutdown`, `server_close`, `join`, through `asyncio.to_thread`), and entry 1 has `test_serve_starts_and_stops_health`.
2. Biggest unnamed risk (a weekly logout of A giving 503; the false claim that crew restarts). Adopted in part: decision 7 is corrected to what `health_poll.go` and `cap_host.go` do (a Fault after 3
   failures in 5 minutes, no restart; a settle that fails on non-2xx) and the entry carries `deploy = "owner"`; the runbook says to log in gateway A before the deploy. Not adopted: the reviewer's option of
   200 whenever the process is alive; the owner wants the health to reflect the gateway, so a Fault on A's logout is the wanted signal. The stale-200 risk of a wedged loop is closed by probing through the
   loop with a timeout (decision 8); `getfqdn` and the per-poll log line are removed (decision 9). The port collision is a start error (decision 13).
3. A simpler way (a body of two booleans; health reads status through callables). Adopted: decisions 6 and 8; `supervise` is not edited and the two entries share no file. The state words, generation and
   last-connect time are dropped; the log lines carry the distinction.
4. Lock-in (socket per gateway, `B` key names, body keys). Accepted and stated in decision 4; the cost is low and a later account design can serve both sockets.
5. Claims most likely false (crew restarts on a failed poll; "the runbook draft"; the 104-byte check against "nothing differs from main"). Adopted: decision 7 and the line of `## What main holds today` now say what
   crew does and name the merged runbook; decision 13 adds no length check, so nothing differs from main for A, and the old acceptance item that refused long paths for both is gone.
6. Likeliest slip in the drives (numbering and order; long socket paths; the guard test; fragile name-only diff; the `boot = true` grep; the dropped `gateway` log field). Adopted: the entries are `###` items that
   crew numbers by position, the runbook entry says `Order: after 1`; the tests use `kit.sockdir.short_socket_dir`; the guard test uses a fake client with `send` and `sendMsg`; entry 2 says the old
   `boot` line is replaced and not quoted; decision 8 states that the `gateway` log field is dropped, with the reason. Not changed: the `git diff --name-only` check stays, since each entry names files that its change must touch.

Owner decision optdesk-617 (2026-11-07), after this review: nothing about gateways is hard-coded. It replaced the fixed gateway A and gateway B of the first version: the `OT_IBKR_GATEWAY_B_*` keys, the single
`ibkr-b.sock`, the `{"a": ..., "b": ...}` body and the "A decides health" rule are gone; the gateways, their accounts (by alias), what each collects and whether each decides health come from the file of decision 2
(decisions 2, 4 and 6), and no file keeps today's single gateway (decision 3). Answers 1 to 3 and 5 stand with that change: the seams, the probes through the event loop, the `deploy = "owner"` entry and the
corrected crew facts apply to every gateway; the entries are now three (the pyside entry, then the runbook with `Order: after 1`, then `pyside-frozen-default`, added 2026-11-10 for the default market-data type 2). Account summary and position serving remains the next design.

Review of PR 275 at a31ba94 (verdict of the design session, kept in the dispatcher's scratchpad): adopted. The three non-interface pyside siblings are one entry (`## Modules` says why this cut and not an interface entry);
the probe deadline lives in `health.py`, runs all probes concurrently under one deadline and has a named test with a blocked probe and one with a barrier (decision 8); the socket-serving default must hold `market_data`,
also when it is the first entry (decision 2, a named test); decision 13 drops the exit-status promise and `main`'s status 2 for a bad file is tested; the numbers of this section are renumbered; the last item of the entry reads
"judged from the code and the design file".
