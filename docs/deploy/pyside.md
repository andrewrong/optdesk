# Deploying pyside as a crew host service

pyside (`python -m optdesk.ibkr`) is the IBKR service that `ot` talks to over a Unix socket.
It runs as a crew host service: its whole definition is one entry in `services.toml` of
agent-crew. This repository holds no `deploy/` file. Every value below in angle brackets is a
placeholder the owner fills in; none is written into this repository.

## The entry for services.toml

`services.toml` is owner-only; this runbook proposes the entry and changes nothing there.

```toml
# way = "host": pyside must reach IB Gateway, a GUI application the owner starts and logs
# into weekly. Crew does not manage IB Gateway. No health: pyside listens on a Unix socket only.
[services.optdesk-pyside]
way   = "host"
repo  = "andrewrong/optdesk"
build = ["<uv path>", "sync", "--locked", "--no-dev", "--project", "pyside"]
run   = ["<uv path>", "run", "--no-sync", "--project", "pyside", "python", "-m", "optdesk.ibkr"]
boot  = true   # the owner may choose false
env   = { OT_HOME = "<OT_HOME of ot>", OT_IBKR_GATEWAY_PORT = "<port>", OT_IBKR_CLIENT_ID = "<client id>" }
```

## Facts the entry rests on

- Crew runs `build` with the release directory as working directory and without
  `CREW_SERVICE_DIR`. So build and run use `--project pyside`, relative to the release, and the
  absolute `uv` path.
- `run` runs in `current`.
- The `env` table is for values that are not keys. pyside holds no secret; the IB login is in
  IB Gateway.
- `--no-dev` keeps the dev group (mypy, ruff, grpcio-tools) out of a release.
- The absolute `uv` path is baked into the entry. After a move of `uv`, the next deploy fails
  at the build; update the path then.

## Keys pyside reads

| Key | Default | In the entry |
| --- | --- | --- |
| `OT_HOME` | none, required | set |
| `OT_IBKR_GATEWAY_PORT` | none, required | set |
| `OT_IBKR_GATEWAY_HOST` | `localhost` | the owner may add |
| `OT_IBKR_CLIENT_ID` | 33 | set |
| `OT_IBKR_LINES` | 70 | the owner may add |
| `OT_IBKR_SNAPSHOTS` | 50 | the owner may add |
| `OT_IBKR_MARKET_DATA_TYPE` | 1 | the owner may add |

## Two rules about OT_HOME

1. It must equal the `OT_HOME` of `ot`. `ot` reads `OT_HOME/run/ibkr.sock`, and pyside creates
   the socket there.
2. `OT_HOME/run/ibkr.sock` must be shorter than 104 bytes, the macOS limit of a Unix socket
   path, or the bind fails.

## First deploy

Owner lines, in order:

1. The main branch must already hold this runbook and the pyside code (deploy fetches the
   default branch of the repository).
2. Create `<service-dir>` of optdesk-pyside under the root of `services.toml`
   (`mkdir -p <service-dir>`). The directory must exist before the entry merges.
3. Add the entry with the three values and merge it.
4. Start IB Gateway, log in, and enable its API for connections from localhost on the port
   written in the entry.
5. Run `crew cap optdesk-pyside deploy` as a dry run, then
   `crew cap --apply optdesk-pyside deploy`. The flag comes before the service name; apply
   needs a terminal and is the owner's.
6. Run the check below.

## What status means, and the check

`crew cap` status ok means only a live tmux pane. pyside binds the socket and retries the broker
for ever, so a wrong port or a client id already in use still shows ok. The proof of a
connection is two things:

- the file `$OT_HOME/run/ibkr.sock` (`OT_HOME` of `ot`) exists;
- the file `optdesk-pyside.log` under `logs` of the service directory holds a line with
  `connect_ok`. A `connect_failed` line means the port, the client id or the login is wrong.

The same-`OT_HOME` check against `ot`: the path printed by `ls $OT_HOME/run/ibkr.sock`
(`OT_HOME` of `ot`) is the one pyside reports in its `server_started` line, and `ot run-once` no
longer reports the broker as unreachable.

## Afterwards

- After the weekly IB Gateway login nothing is needed: the supervisor reconnects (a
  `session_lost` line, then `connect_ok`). If it does not, run
  `crew cap --apply optdesk-pyside restart`.
- A rollback is `crew cap --apply optdesk-pyside rollback`.
- A deploy is manual. The optdesk project has no automatic deploy, and adding one would restart
  pyside on every merge.

## Left out

A health URL (a TCP listener is out of scope), a launchd unit, starting IB Gateway from crew,
and a script or env file under `deploy/`.
