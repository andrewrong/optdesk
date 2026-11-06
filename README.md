# optdesk

optdesk is a personal options desk: a Go binary `ot` that runs data jobs and serves pages, a
Python side for the broker gateway and a TypeScript front end. The design is under `docs/design/`;
`docs/design/modules.md` is the module map and the single source of dependencies.

## Try it

```sh
go run ./cmd/ot version        # starts under the demo profile when $OT_HOME is absent
```

`ot` reads only `OT_HOME` and `OT_PROFILE`. Without `OT_HOME` it runs the demo profile and loads no real value.

## Checks

```sh
go run ./scripts/arch          # regenerate arch.toml and .go-arch-lint.yml from modules.md
go run ./scripts/arch --check  # fail when they differ from modules.md
go tool go-arch-lint check     # the import limits
go tool golangci-lint run ./...
go run ./scripts/ratchet       # file size rule (600 new, 800 base, docs/design/ratchet.json)
go run ./scripts/hygiene       # account ids, local paths, e-mail addresses, fixture manifests
```

Rules for contributors and agents are in `AGENTS.md`.

## Licence

Apache License 2.0; see `LICENSE`.
