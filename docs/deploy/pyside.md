# Deploying pyside as a crew host service

pyside (`python -m optdesk.ibkr`) is the IBKR service that `ot` talks to over Unix sockets. It
runs as a crew host service: its whole definition is one entry in `services.toml` of agent-crew.
This repository holds no `deploy/` file. Every value below in angle brackets is a placeholder the
owner fills in; none is written into this repository.

One process serves every IB Gateway the owner lists in a gateways file: one connection, one
supervisor and (for market data) one socket per gateway. A gateway that is down or being logged
into never blocks another. Which gateways exist, which account each serves (by alias) and what is
collected through each come from the file; nothing about them is in the code.

## The entry for services.toml

`services.toml` is owner-only; this runbook proposes the entry and changes nothing there.

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

The gateways are not in the entry: they are in the file that `OT_IBKR_GATEWAYS` names. The health
port is one number written twice, in crew's `health` URL and in `OT_IBKR_HEALTH_PORT`; it must
differ from every other port on the machine. Optional keys the owner may add to `env`:
`OT_IBKR_LINES` and `OT_IBKR_SNAPSHOTS` (they apply to every gateway).

## The gateways file

Copy `pyside/examples/gateways.example.toml` to the service's config directory (it is not
committed; the repository holds only the example) as the file `OT_IBKR_GATEWAYS` names, then fill
it in. The example's ports are the placeholders 1 and 2: replace each with the port of your
gateway. Shape:

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
collect = ["market_data", "account_summary", "positions"]
required = false
```

One `[[gateway]]` table per gateway. Keys:

- `name` (required): a slug, a lowercase letter then up to 15 lowercase letters, digits or
  hyphens, unique. It names the socket and the key in `/health`.
- `port` (required): a whole number above zero. `host` defaults to `localhost`, `client_id` to
  33 (a client id is per gateway), `market_data_type` to 2 (frozen: live data while the market is
  open, the last value when it is closed; live type 1 gives no bid or ask after the close, and the
  US option capture of 2026-11-09 stored 2272 rows without any). A value in the file wins.
- `account` (required): an alias, a label for you, never a real account id. A value shaped like an
  account id is refused. pyside does not send it to the gateway.
- `collect` (required): a non-empty list of `market_data`, `account_summary`, `positions`. Each of
  the three words is served: `market_data` by the market data services, `account_summary` and
  `positions` by `AccountService`.
- `required` (default false): whether this gateway's connection decides health (see below).
- `default` (top level, optional): the gateway that also answers on `ibkr.sock`.

The file is refused, and the process exits with status 2, when it has no gateway, no gateway with
`required = true`, an unknown key (a typo is an error), a duplicate name, a duplicate
host, port and client id, or when the gateway that answers on `ibkr.sock` lacks `market_data` in
`collect`. The message names the table and the key, never the value. A change of the file is a
restart: `crew cap --apply optdesk-pyside restart`.

## The no-file fallback

When `OT_IBKR_GATEWAYS` is unset, pyside runs the one gateway it ran before: one entry named
`default`, `collect = ["market_data"]`, required, served on `ibkr.sock` only. It reads
`OT_IBKR_GATEWAY_PORT` (required), `OT_IBKR_GATEWAY_HOST` (default `localhost`),
`OT_IBKR_CLIENT_ID` (default 33) and `OT_IBKR_MARKET_DATA_TYPE` (default 2, frozen; a
value in the variable wins). When `OT_IBKR_GATEWAYS` is set, these four keys are ignored.

## Keys pyside reads

| Key | Default | Meaning |
| --- | --- | --- |
| `OT_HOME` | none, required | the home of `ot`; the sockets are under `run/` |
| `OT_IBKR_GATEWAYS` | unset | path of the gateways file; unset gives the no-file fallback |
| `OT_IBKR_HEALTH_PORT` | unset | port of `GET /health`, a whole number above zero; unset means no listener |
| `OT_IBKR_LINES` | 70 | applies to every gateway |
| `OT_IBKR_SNAPSHOTS` | 50 | applies to every gateway |

A key that is set and unusable is an error that names the key only.

## Sockets

A gateway is served on `$OT_HOME/run/ibkr-<name>.sock` when its `collect` holds any of
`market_data`, `account_summary` and `positions`. The session service is on every such socket; the
market data services only when `collect` holds `market_data`, and `AccountService` only when it
holds `account_summary` or `positions`. `ibkr.sock` still belongs to a gateway with `market_data`:
one of those also answers on `$OT_HOME/run/ibkr.sock`, the path `ot` reads, the gateway `default`
names, or the first one of the file when `default` is absent. Name `default` explicitly, so that a
reordered file never switches the account `ot` collects market data from. In the no-file case
there is only `ibkr.sock`. The directory is mode 0700 and the files 0600.

Two rules about `OT_HOME`:

1. It must equal the `OT_HOME` of `ot`.
2. Each socket path under it must be shorter than 104 bytes, the macOS limit of a Unix socket
   path, or the bind fails.

Every gateway is read-only through the same guard; also set "Read-Only API" in IB Gateway.

## Account data

A gateway whose `collect` holds `account_summary` or `positions` (gateway B, for example, with
`collect = ["account_summary", "positions"]` and no `market_data`) answers the rpc `Account` of
`AccountService` on its `ibkr-<name>.sock`. The answer carries the alias of the gateway's entry and
never an account id; the account id the gateway reports is dropped.

- Exactly one account per gateway is served. When the login reports no account or several (an IRA
  beside a margin account, a linked or advisor structure), the rpc is refused on every call.
- Nothing is subscribed beyond a call: the rpc requests, reads until the gateway ends the data and
  cancels, so no subscription outlives it.
- The allowlist of the guard gains four ids only, 61 to 64 (the requests and cancels of positions
  and account summary). Orders stay refused, and so does every other id.
- The file with gateway B (its port and alias) stays on the owner's machine; the repository holds
  only the example with placeholders.

How a refused login shows: the log line in `optdesk-pyside.log` carries the word `multiple_accounts`
(no count and no id), the Go caller sees a `Permanent` error, and the health URL still says
connected. Recovery: a separate login for gateway B, or a later design that names one account by an
alias held on the owner's machine.

The live check of gateway B is the owner's: report what it lists, one account or several. Several
accounts mean a later design for selecting one, and its selector is an `.env` secret, never TOML or
code.

## Health

`GET /health` on `localhost` at `OT_IBKR_HEALTH_PORT`. The answer is 200 when every gateway with
`required = true` is connected and 503 otherwise, within 2 seconds. The body is
`{"gateways": {"<name>": true, "<name>": false}}`, one key per gateway, `true` when connected; no
alias, account id, host or port is in it. A gateway that is not `required` is reported and never
changes the status code. Any other path is 404 and any other method 405.

What crew does with it: it does not restart the service on a failed poll. After 3 failures in
5 minutes it opens a Fault, and it closes the Fault after a healthy poll. So the weekly logout of
a required gateway opens a Fault; that is the wanted signal. The settle of `crew cap --apply
optdesk-pyside deploy` and of `restart` polls the same URL and a non-2xx fails the action. The
first poll after a plain start can be non-2xx until the required gateways connect.

## First deploy

Owner lines, in order:

1. The main branch must already hold this runbook and the pyside code (deploy fetches the
   default branch of the repository).
2. Create `<service-dir>` of optdesk-pyside under the root of `services.toml`
   (`mkdir -p <service-dir>`). The directory must exist before the entry merges.
3. Make the gateways file from the example and put it in the service config directory.
4. Add the entry and merge it.
5. Start every IB Gateway, log in, and enable its API for connections from localhost on the port
   written in the file. Log the required gateways in before the deploy: the settle needs 2xx, so
   a required gateway that is not logged in fails it.
6. Run `crew cap optdesk-pyside deploy` as a dry run, then
   `crew cap --apply optdesk-pyside deploy`. The flag comes before the service name; apply
   needs a terminal and is the owner's. `boot = false` and `deploy = "owner"` mean crew never
   starts it at boot and never deploys it after a merge.
7. Run the check below.

## What status means, and the check

`crew cap` status ok means only a live tmux pane; the health URL is the proof that the required
gateways are connected. Besides it, check:

- the files `$OT_HOME/run/ibkr-<name>.sock` and `$OT_HOME/run/ibkr.sock` exist
  (`OT_HOME` of `ot`);
- the file `optdesk-pyside.log` under `logs` of the service directory holds a `connect_ok` line
  for each connected gateway. A `connect_failed` line means the port, the client id or the login
  is wrong; the supervisor retries for ever, so a not-required gateway left logged out keeps the
  service running with its key `false` in the body.

## Afterwards

- After the weekly IB Gateway login nothing is needed: the supervisor reconnects (a
  `session_lost` line, then `connect_ok`) and the Fault closes at the next healthy poll. If it
  does not reconnect, run `crew cap --apply optdesk-pyside restart`.
- A rollback is `crew cap --apply optdesk-pyside rollback`.
- A deploy is manual, after the required gateways are logged in. The optdesk project has no
  automatic deploy, and adding one would restart pyside on every merge.

## Left out

An automatic fallback from one gateway to another
(the caller dials another gateway's socket), a launchd unit, starting IB Gateway from crew, TLS or
a non-loopback bind for `/health`, and a script or env file under `deploy/`.
