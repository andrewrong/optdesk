# Deploying ot serve as a crew docker service

`ot serve` serves the whole site over HTTP from one process. It is a subcommand of the one `ot`
binary, a separate process from `ot scheduler`. It runs as a crew docker service: its definition is
one entry in `services.toml` of agent-crew, a compose file and a Dockerfile. Every value below in
angle brackets is a placeholder the owner fills in; no key value, no DSN and no address is written
into this repository. The commands are in `docs/contracts/ot.md`; the rules of crew for a docker
service are `rules/service.md` of the agent-crew repository.

The owner decided (2026-11-09) that `ot serve` runs in docker and `ot scheduler` on the host
(`docs/deploy/scheduler.md`). `ot serve` reads PostgreSQL only and names no broker and no socket, so
a container is enough for it.

## The entry for services.toml

`services.toml` is owner-only; this runbook proposes the entry and changes nothing there.

```toml
[services.optdesk-serve]
way     = "docker"
repo    = "andrewrong/optdesk"
health  = "http://<bind>:<port>/health"   # the host address and the published port of the container
secrets = ["optdesk.env"]                  # the file under the secrets directory that holds the DSN lines
root    = "<root of services.toml>"        # the service directory is created under it
deploy  = "owner"                          # set to "auto" only after the first deploy below
```

Start at boot is the default of a docker service, the restart policy `unless-stopped`; the entry
sets nothing for it. `<port>` is one number written twice, in the `health` URL and in the published
port of the compose file; it must differ from every other port on the machine.

The compose file and the Dockerfile are those of `serve-deploy-files`; they take these values from
crew:

- `CREW_IMAGE`: the image name crew builds and runs;
- `CREW_SERVICE_DIR`: the service directory under `<root>`;
- `CREW_SECRETS`: the secrets directory that holds `optdesk.env`.

## The container command

The command of the container is:

```
serve --addr :8080
```

`ot serve` listens on `127.0.0.1:8080` by default, which a published port cannot reach, so the
container sets `--addr :8080`. The compose file publishes container port 8080 on `<bind>:<port>`.
`GET /health` is the route of issue #926 (200 when the database answers, 503 when it does not); crew polls it.
The route is not on the main branch until #926 merges, and this service is not deployed before that.

## How the keys reach it

`ot` reads `OT_PG_DSN_PAGES` from the file `$OT_HOME/.env`, not from the process environment. So an
`environment:` or `env_file:` entry of compose does not give it the line. Compose instead mounts
`${CREW_SECRETS}/optdesk.env` read-only as the file `.env` in the OT_HOME directory, and sets
`OT_HOME` to that directory:

```yaml
    environment:
      OT_HOME: <OT_HOME directory in the container>
    volumes:
      - ${CREW_SECRETS}/optdesk.env:<OT_HOME directory in the container>/.env:ro
```

`optdesk.env` is the owner's file and stays outside the repository. For `ot serve` it needs the one
line `OT_PG_DSN_PAGES` (the DSN of the role `ot_ro_pages`). No key value is written in this
repository, in the compose file or in the entry.

## The image

The web bundle is built before the Go binary: `pnpm --dir web build` writes it into
`internal/pages/embed/dist`, and `go build` of `./cmd/ot` embeds that directory
(`internal/pages/embed/embed.go`). The image therefore holds one binary and no `web/dist`; the
Dockerfile runs the pnpm build stage first and copies only `internal/pages/embed/dist` into the Go
stage.

## PostgreSQL stays outside the container

PostgreSQL is not part of this service. The DSN in `OT_PG_DSN_PAGES` names the host address
`<pg-host>`, not `localhost` and not `127.0.0.1`, because inside the container those are the
container itself. The database must accept the connection from the docker network on that address.

## First deploy

The first deploy is the owner's. Owner lines, in order:

1. The main branch must already hold this runbook, the compose file and the Dockerfile (deploy
   fetches the default branch of the repository), and the migrations must be applied (`ot migrate`).
2. Create `optdesk.env` under the secrets directory with the line `OT_PG_DSN_PAGES` and `<pg-host>`
   in its DSN, and create the service directory under `<root>`.
3. Add the entry with `deploy = "owner"` and merge it.
4. Run `crew cap optdesk-serve deploy` as a dry run, then `crew cap --apply optdesk-serve deploy`;
   apply needs a terminal.
5. Request `http://<bind>:<port>/health` and `/home`: both must answer 200.
6. Change the entry to `deploy = "auto"` and merge it. From then on crew deploys on merge and rolls
   back by itself.

## Left out

Not done here: the compose file and the Dockerfile themselves (`serve-deploy-files`), TLS, a
non-loopback exposure beyond `<bind>`, a PostgreSQL container, and any key value.
