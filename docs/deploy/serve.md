# Deploying ot serve as a crew host service

`ot serve` is the web server of the one `ot` binary: it serves the pages and `/health`, and reads
PostgreSQL only. It is a separate process from `ot scheduler`. Like `ot scheduler` and optdesk-pyside it
runs on the host under the keeper of agent-crew: its whole definition is one entry in `services.toml`.
This repository holds no `deploy/` file. Every value below in angle brackets is a placeholder the owner
fills in; none is written into this repository. The commands and flags are in `docs/contracts/ot.md`;
the form of this runbook is `docs/deploy/scheduler.md`.

## The entry for services.toml

`services.toml` is owner-only; this runbook proposes the entry and changes nothing there.

```toml
[services.optdesk-serve]
way = "host"                # a host service under the keeper, like optdesk-pyside and optdesk-scheduler
repo   = "andrewrong/optdesk"
boot   = true                  # in crew: started and restarted by the keeper the owner starts, not a start at machine boot
deploy = "owner"               # until the first deploy below; the owner then changes it to "auto"
deploy_paths = ["<deploy path>", "<deploy path>"]   # the paths of the repository whose change redeploys ot serve
build  = ["env", "PATH=<mise shims and go dir>", "HOME=<home>", "sh", "-c", "pnpm install --frozen-lockfile && pnpm --dir web build && go build -o bin/ot ./cmd/ot"]
run    = ["bin/ot", "serve", "--addr", "<addr>"]
health = "http://<loopback>:<port>/health"
env    = { OT_HOME = "<OT_HOME of the scheduler and pyside>" }
```

- `<addr>` is `<loopback>:8080` to listen on the loopback only, or `:8080` to listen on every
  interface. `<port>` in `health` is the port of `<addr>`, one number written twice; it must differ
  from every other port on the machine.
- `deploy_paths` lists the paths whose change redeploys the service; the owner fills it in. The
  paths of the web front end (`web/`), of the embedded bundle and of the Go code `ot serve` builds
  from belong in it.
- `boot = true` is needed because the keeper skips every host service with `boot = false`. It does
  not start anything at machine boot.
- `OT_HOME` must equal the `OT_HOME` of the scheduler and of pyside, so the three programs read
  one home.

## The secrets are not in the entry

`ot` reads `OT_PG_DSN_PAGES` from `$OT_HOME/.env` (`cmd/ot/env.go` reads the file;
`cmd/ot/site.go` names the variable), so the entry holds no key and no DSN. A missing line makes
`ot serve` refuse at start and name the variable.

## The build line

crew's `build` is one argv, started in the release directory. It cannot hold a `&&` of its own, so
the entry is `env PATH=<mise shims and go dir> HOME=<home> sh -c "pnpm install --frozen-lockfile &&
pnpm --dir web build && go build -o bin/ot ./cmd/ot"`:

1. `env PATH=... HOME=...` gives the build the mise shims (`pnpm`, `node`) and the Go directory on
   `PATH`, which the keeper does not have, and a `HOME`.
2. `pnpm install --frozen-lockfile` installs the packages from the committed lockfile.
3. `pnpm --dir web build` builds the web bundle into `internal/pages/embed/dist`
   (`outDir: '../internal/pages/embed/dist'` in `web/vite.config.*`). This comes before `go build`
   because the directory is `go:embed`ded (`//go:embed all:dist` in `internal/pages/embed/embed.go`)
   and is not committed.
4. `go build -o bin/ot ./cmd/ot` writes the binary into the release directory.

The relative `run[0]`, `bin/ot`, resolves against `current`, the symlink to the release directory
the build ran in, so `run` needs no absolute path.

## Health

`health` is `http://<loopback>:<port>/health`. `/health` answers 200 while the server is up and
its database is reachable, else 503. crew opens a Fault after 3 failures in 5 minutes and does not
restart the service on a failed poll; `crew cap` status ok means only a live tmux pane. A replaced
binary is picked up by crew's deploy, which restarts the service.

## First deploy

The first deploy is the owner's. Owner lines, in order:

1. The main branch must already hold this runbook and the code of `ot serve` (deploy fetches the
   default branch of the repository), and the migrations must be applied (`ot migrate`).
2. Create the service directory of optdesk-serve under the root of `services.toml`:
   `mkdir -p <root>/optdesk-serve`. The directory must exist before the entry merges.
3. Check that `$OT_HOME/.env` holds the line `OT_PG_DSN_PAGES`, and that `pnpm`, `node` and `go`
   are found under the `PATH` of the `build` entry.
4. Add the entry with `deploy = "owner"` and merge it.
5. Run `crew cap optdesk-serve deploy` as a dry run, then `crew cap --apply optdesk-serve deploy`.
6. Open the health URL: it must answer 200.
7. Change the entry to `deploy = "auto"` and merge it. From then on crew deploys on merge and rolls
   back by itself, and no `deploy = "owner"` stays.

## Left out

Not done here: starting `ot serve` at machine boot, a launchd unit, a cron entry, TLS, and a script
or env file under `deploy/`.
