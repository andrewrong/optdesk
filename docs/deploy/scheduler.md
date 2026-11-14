# Deploying ot scheduler as a crew host service

`ot scheduler` is the resident process that owns the instance lock and runs the ticker: it calls
`Reconcile` for each job at its slot. It is a subcommand of the one `ot` binary, a separate process
from `ot serve`. It runs as a crew host service: its whole definition is one entry in
`services.toml` of agent-crew. This repository holds no `deploy/` file. Every value below in angle
brackets is a placeholder the owner fills in; none is written into this repository. The design is
`docs/design/scheduler.md`; the commands are in `docs/contracts/ot.md`.

Nothing else starts it: launchd is not used, no cron runs it, and `ot serve` never starts, stops or
embeds it.

## Why a host service

The way was decided by a trial of 2026-11-08. A Unix socket listening on the host, in a directory
bind-mounted into a container (OrbStack running; `docker run --rm -v <dir>:/sock ...` connecting to
`/sock/s.sock`), gave `ConnectionRefusedError` twice and the host listener never saw a connection.
OrbStack does not forward a host Unix socket through a bind mount here. The broker jobs reach
pyside only by the sockets under `$OT_HOME/run/` (`docs/deploy/pyside.md`), so a container cannot
reach them and the scheduler cannot run in one. `ot serve` is also a host service, see `docs/deploy/serve.md`: it reads PostgreSQL
only and names no broker and no socket.

## The entry for services.toml

`services.toml` is owner-only; this runbook proposes the entry and changes nothing there.

```toml
[services.optdesk-scheduler]
way = "host"                 # the broker sockets are Unix sockets on the host; a container cannot connect to them
repo   = "andrewrong/optdesk"
boot   = true                  # in crew: started and restarted by the keeper the owner starts, not a start at machine boot
deploy = "auto"                # crew deploys on merge and rolls back by itself; set only after the first deploy below
deploy_paths = ["<deploy path>", "<deploy path>"]   # the paths of the repository whose change redeploys the scheduler
build  = ["env", "PATH=<go dir>", "HOME=<home>", "sh", "-c", "go build -o bin/ot ./cmd/ot"]
run    = ["bin/ot", "scheduler"]
health = "http://127.0.0.1:<health port>/health"
env    = { OT_HOME = "<OT_HOME of ot>", OT_SCHED_HEALTH_PORT = "<health port>" }
```

`boot = true` is needed because the keeper skips every host service with `boot = false`, and
no other key gives a restart after a crash. It does not start anything at machine boot.

Until the first deploy is done, write `deploy = "owner"` in its place (the first deploy below); the
owner changes it to `"auto"` afterwards and no `deploy = "owner"` stays.

`build` and `run` are explained in `docs/deploy/serve.md`, The build line. `deploy_paths` lists the paths whose change
redeploys the scheduler; the owner fills it in.

The secrets are not in the entry: the DSNs and `FRED_API_KEY` are lines of `$OT_HOME/.env`, which
`ot` reads. The health port is one number written twice, in crew's `health` URL and in
`OT_SCHED_HEALTH_PORT` (unset gives 8765, so `<health port>` is 8765 unless the owner sets another); it must differ from every other port on the machine, and
the listener is on `127.0.0.1` only. `OT_HOME` must equal the `OT_HOME` of `ot` and of pyside,
because the broker socket is `$OT_HOME/run/ibkr.sock`.

## Crash recovery

The owner starts the keeper; the keeper brings the scheduler up and starts it again after a crash.
The machine itself starts nothing: after a boot the IB gateways need a re-login anyway, so the owner
starts the keeper then. While the scheduler is down no slot is run; on restart the ticker reads the
ledger and runs the keys still missing. A start closes the previous process's `running` rows at once
as `crashed` (once the instance fence is held, before the first tick), whatever their lease, and the
keys are started again (up to the brake of three, which a deploy mid-run counts toward like a lapsed
lease). A row the old process wrote after that close, in the moment between its fence ending and its
stop, is still closed `crashed` when its lease lapses.

A job whose broker is down at start (pyside not up, a gateway logged out) is left out and logged
once; it comes back on the first tick whose probe passes, with no restart. `fred_series` with an
empty `FRED_API_KEY` is left out until a restart with the key.

## Restart procedure

crew has no hook that asks a service before a restart, so the check is by hand:

1. Run `ot scheduler can-restart`. Exit 0 and `ok` mean a restart is safe now. Exit 1 prints the
   reason: a run is not finished, or a job marked `restart_sensitive` is between its slot start and
   its window close. Wait, and run it again.
2. Run `crew cap --apply optdesk-scheduler restart`. The flag comes before the service name; apply
   needs a terminal and is the owner's.
3. Run `ot scheduler status` and read it as below.

`can-restart` is advice. After the first deploy crew's automatic deploy and rollback restart the
scheduler on every merge without calling it, and `crew cap --apply ... restart` bypasses it. So a
merge during a sensitive slot is not stopped: it costs a drain, and the run in hand ends `partial`
(counted, and run again by the tick) or `abandoned` (not counted, and its key is repeated on
restart). Until this is enforced (crew would need a pre-deploy hook; lesson 8 of the design), merge
changes of the scheduler outside 09:30-16:30 America/New_York.

A replaced binary needs no command: the scheduler compares the identity of its executable
(size, mtime, inode) at each tick, drains when it changed and exits with status 75, the contract of
crew for one uncounted keeper restart per new identity.

## SIGTERM

On SIGTERM (or interrupt) the loop stops and the worker gets a 30-second grace. The run in hand is
finished or cut at the end of the grace: a cut run that returned rows ends `partial`, one that
returned none ends `abandoned` with reason `abandoned_stop`. Then the lock is closed, the
run's lease is released and the process exits. Any other stop (the lock held, the lock lost, a
frozen loop) exits non-zero, never 75.

## Reading ot scheduler status

`ot scheduler status` reads the `ops` tables with the runner DSN alone and opens no dataset store
and no broker, so it works while the scheduler is down. It prints:

- `build`: the build the running scheduler recorded; `(none)` before any scheduler has run;
- `runs`: the runs not finished, each with its day and the age of its last heartbeat; a heartbeat
  that keeps growing means a stuck run;
- `jobs`: the next slot of each job, and `left out: <reason>` for a job the scheduler could not
  prepare (a missing key, an unreachable broker);
- `gaps`: the open gaps, a dataset and day for a key gap, a job for a preflight gap, each with the
  time it was reported.

Exit 0 is green: no gap is open and no job is left out. Exit 1 is red, with the reasons in the
error line. The live proof is the health URL: 200 while the lock is held and the last tick is under
60 seconds old, else 503. crew opens a Fault after 3 failures in 5 minutes and does not restart the
service on a failed poll; `crew cap` status ok means only a live tmux pane.

## First deploy

Owner lines, in order:

1. The main branch must already hold this runbook and the scheduler code (deploy fetches the
   default branch of the repository), and the migrations of the `ops` tables must be applied
   (`ot migrate`).
2. Create `<service-dir>` of optdesk-scheduler under the root of `services.toml`
   (`mkdir -p <service-dir>`). The directory must exist before the entry merges.
3. Check that `$OT_HOME/.env` holds the DSN lines `ot scheduler` needs, and that pyside runs with
   its required gateways logged in (a job without its broker is left out, not an error).
4. Add the entry with `deploy = "owner"` and merge it.
5. Run `crew cap optdesk-scheduler deploy` as a dry run, then
   `crew cap --apply optdesk-scheduler deploy`.
6. Run `ot scheduler status` and the health URL: both must be green.
7. Change the entry to `deploy = "auto"` and merge it. From then on crew deploys on merge and rolls
   back by itself, subject to the merge window above.

## A second process

Only `ot scheduler` runs jobs, and only one scheduler runs. The instance fence, a PostgreSQL advisory
lock taken before anything else is opened, is the one-process guarantee. A
second `ot scheduler` retries every 5 seconds for 60 seconds, then exits non-zero with the text
`the scheduler lock is held or cannot be taken after 60s: is another ot scheduler running?`; it
touches nothing. Once it holds the fence, a start closes every `running` row of the previous process as
`crashed`. `ot run-once` and `ot backfill` run no job: they only enqueue console requests, which the
scheduler's console runs, so they need the console on (`ot scheduler status`); without it a request
stays queued. During a restart the old process releases the lock when it exits, so the
new one gets it within its wait. If the old process died, the database releases the lock when its
connection ends.

## What calendar_refresh needs

The job `calendar_refresh` asks pyside's own generator for the trading calendar by running
`uv run --no-sync --project pyside python -m optdesk.calendar ...` as a child process of the
scheduler (`docs/design/compute-calendar.md#Decisions`). The child starts in the working directory
of the scheduler, so the host needs:

- `uv` on `PATH` of the scheduler, or the path of `uv` in the `.env` line `OT_CALENDAR_UV` (the
  default is `uv` found on `PATH`);
- the working directory of the scheduler is the checkout root, the directory that holds `pyside/`.
  This is the same need as the pyside unit, which starts with
  `uv run --no-sync --project pyside python -m optdesk.ibkr` (`docs/deploy/pyside.md` line 23-24);
- two lines in `$OT_HOME/.env`: `OT_PG_DSN_LOADER` (the DSN of the role `ot_loader`, the only role
  that may write `master.session_day`) and `OT_CALENDAR_UV` (the `uv` path above).

A failed command ends the run in one of two ways:

- a failure to start or wait for the process, or a timeout at `run_bound` (5 minutes), is a
  retryable run;
- exit code 1, exit code 2 and any other nonzero exit are permanent.

Either way nothing is written for that run, and the next session day's refresh covers the span
again (today minus 7 to today plus 400 days).

## Left out

Not done here: starting the scheduler at machine boot, a launchd unit (not used), a cron entry,
a container, a check of `can-restart` by crew before a deploy, TLS or a non-loopback bind for `/health`, and a script or
env file under `deploy/`.
