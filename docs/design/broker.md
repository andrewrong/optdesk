# Broker: the Go client of the IBKR service

Author: the optdesk dispatcher (draft)
Owner approved: (pending)
Date: 2026-11-07
Source: owner request optdesk-485 (option collection), second of three designs (`docs/design/store-master.md`, `docs/design/broker.md`, `docs/design/option-jobs.md`).
Read on main at 0c3d9e6: `AGENTS.md`, `docs/design/modules.md`, `pyside-ibkr-transport.md`, `backfill.md` (form), `internal/clock/api.go`,
`internal/runner/reconcile/reconcile.go`, `scripts/arch/main.go`, `arch.toml`, `.go-arch-lint.yml`, `go.mod`, `buf.gen.yaml`.
Settled elsewhere: PR 184 puts the snapshot kind (`QUOTE_KIND_SNAPSHOT`), the sizes and `event_ts` of `Quote`, the second `buf` plugin, the generated `*_grpc.pb.go`
files and the `google.golang.org/grpc` requirement into #166; this design edits no proto, no `buf.gen.yaml`, no generated file and no `go.mod`, and its client entry is attached
after #166 has merged with PR 184's content (the generated stubs and `go.mod` must exist; the owner's word on the library is given there, not here).

Goal: a Go client (module broker) that speaks `ibkr.v1` over the pyside Unix socket and answers with plain Go types, so the jobs of `docs/design/option-jobs.md` call a port
and never the generated code. Nothing calls it yet.

Facts the design rests on:

- `internal/broker/` does not exist (`pyside-ibkr-transport.md:27`); the row `broker` has paths `internal/broker/**` and `docs/contracts/broker.md`, entry `internal/broker/api.go`,
  depends `contracts clock` (`modules.md:65`). The Go client is named as a separate design there (`pyside-ibkr-transport.md:346`).
- Nothing may import `internal/contracts/genpb` today: `scripts/arch/main.go:111` lists it as generated with an empty import list and `arch.toml:11-13` gives it `may_import = []`.
  `arch.toml` comes from the table of `modules.md` and the implementation list of `ot` alone (`scripts/arch/main.go:4`); the directory components of a new package reach
  `.go-arch-lint.yml` only (`deriveTree`, `main.go:231`, which builds a sub-package's imports with `subDeps`, `:177`, from the row's roots and the root itself). The exception for the one package that may read `genpb` is therefore code in `deriveTree` and a line of `.go-arch-lint.yml`; `arch.toml` does not change.
- `clock.Clock` has one method, `Now() time.Time` (`internal/clock/api.go:12-13`): it cannot arm a timer, so a call timeout is a `context.WithTimeout`, and a test uses a one-nanosecond timeout against a server that blocks on a channel, not a sleep.
- The runner calls `job.Run(ctx, ...)` with the caller's context and no timeout (`reconcile.go:272`); a lease lasts 10 minutes (`reconcile.go:23`), so the client bounds every call.
- The service keeps no cache and holds at most 5 contract details and 50 snapshots in flight (`pyside-ibkr-transport.md:41-45`, `:163`). One `ot run-once` is one process, so an in-memory cache of not-listed answers would
  never survive to the next run; this design builds none (`## Left out`, point 4).
- Generated Go and Python are produced by two scripts: `sh contracts/generate.sh` (Go, calls `contracts/generate-proto.sh`) and `contracts/generate-pyproto.sh` (Python, freshness tested by `test_generated_python_is_current`, not by `generate.sh`). This design touches neither.

## Modules

`docs/design/modules.md` changes by one sentence in the derivation paragraph (line 16, where `contracts/genpb` is said to have an empty import list): the sub-package `broker/grpcbroker` is the one package that may import `contracts/genpb`. The design
pull request makes the edit. No row changes. `internal/broker/**` and `docs/contracts/broker.md` are broker; `scripts/**` and `.go-arch-lint.yml` are repo (`modules.md:77`).

| issue | module |
|---|---|
| broker-interface | broker and repo (licensed, below) |
| broker-grpc | broker |

Licence: the interface entry lies in two modules because the arch exception is repo code (`scripts/arch/main.go`, its test and `.go-arch-lint.yml`) and the stub package `internal/broker/grpcbroker/doc.go`
must be listed there at the moment the directory gets its first Go file (the precedent is `store-week2.md:225-230`). The client entry lies in broker only.

## Interface

Owner: broker, `internal/broker/api.go`, declarations only; no account, position or order type exists, and the proto has none.

| name | declaration |
|---|---|
| `broker.Kind` | `type Kind int`: `KindOptionEod`, `KindUnderlyingIV`, `KindSnapshot` (the three kinds of `ibkr.v1`) |
| `broker.Contract` | `struct{ SecType, Symbol, Exchange, Currency, Expiry, Strike, Right, Multiplier, TradingClass string; ConID int64 }`; `Expiry` is `YYYYMMDD`, `Strike` canonical decimal text |
| `broker.Chain` | `struct{ Exchange, TradingClass, Multiplier string; Expirations, Strikes []string }`; strikes are the union over all expirations, as the proto comment says |
| `broker.Status` | `type Status int`: `StatusOK`, `StatusPartial`, `StatusNoData`, `StatusNoSubscription`, `StatusTimeout`, `StatusError`, `StatusDisconnected` |
| `broker.Quote` | `struct{ Contract Contract; Status Status; MarketDataType int; Generation int64; ReceivedAt time.Time; EventTs *time.Time; Bid, Ask, Last, Close, Volume, OpenInterest, BidSize, AskSize, LastSize, ImpliedVol, Delta, Gamma, Vega, Theta, UndPrice *float64; ErrorCode int; ErrorText string }` |
| `broker.Resolved` | `struct{ Listed bool; Contract Contract }`; `Listed` false is the proto's NOT_LISTED |
| `broker.Broker` | `interface{ ListChains(ctx, Contract) ([]Chain, error); ResolveContract(ctx, Contract) (Resolved, error); Snapshot(ctx, Kind, []Contract) ([]Quote, error); Close() error }`; `Snapshot` returns one `Quote` per contract in the order asked and does not split the call (the caller chunks); errors are `contracts.TypedError`: `UNAVAILABLE` and `DEADLINE_EXCEEDED` are `Retryable`, every other code `Permanent` |
| `broker.Options` | `struct{ CallTimeout, SnapshotTimeout time.Duration; ResolveInFlight int }`; a zero field takes its default |
| `broker.DialFunc` | `func(ctx context.Context, socketPath string, o Options) (Broker, error)`; implemented by `grpcbroker.Dial` |

Defaults: `CallTimeout` 25 s (above the 20 s the service waits for a contract detail, `pyside-ibkr-transport.md:43`), `SnapshotTimeout` 60 s, `ResolveInFlight` 5 (the service's contract-detail ceiling).
`ListChains` and `ResolveContract` use `CallTimeout`; `Snapshot` uses `SnapshotTimeout`; at most `ResolveInFlight` `ResolveContract` calls of one client run at once, the others wait on the caller's context.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is an offline grep or a named test, run without network, service or database. Every test uses
`t.Parallel()` and `t.TempDir()`; none sleeps; no test touches a live broker or the gateway.

### broker-interface (interface)

The broker declarations, the arch exception and the stub package
  Goal: `internal/broker/api.go` declares the types of `## Interface`, and `broker/grpcbroker` may import `contracts/genpb` while no other package may.
  Files: internal/broker/api.go, internal/broker/api_test.go, internal/broker/grpcbroker/doc.go, scripts/arch/main.go, scripts/arch/main_test.go, .go-arch-lint.yml
  Design: docs/design/broker.md#interface
  Cross-module: broker and repo, licensed by this design (see `## Modules`).
  Acceptance:
  1. `api.go` holds exactly the types, `Broker`, `Options` and `DialFunc` of `## Interface`, declarations only (`grep -cE '^func ' internal/broker/api.go` prints 0); `grep -ciE 'order|account|position' internal/broker/api.go` prints 0.
  2. `api_test.go` tests the contract from both sides: a fake `Broker` that returns a `Quote` per contract and a `Retryable` `TypedError`, and a caller that branches on `errors.As` to `contracts.TypedError`.
  3. `grpcbroker/doc.go` is a package comment and the package clause (`wc -l` is at most 3).
  4. `go run ./scripts/arch --check` prints `arch: ok`; `.go-arch-lint.yml` has the component `broker/grpcbroker` with `contracts`, `clock`, `broker` and `contracts/genpb` as the imports it may use; `git diff -- arch.toml go.mod go.sum` is empty.
  5. A test in `scripts/arch/main_test.go` derives the components of a scratch tree and shows that `broker/grpcbroker` has the `contracts/genpb` import and that another sub-package (for example `jobs/optioneod`) does not; the existing test that `contracts/genpb` has an empty import list still passes.
  6. Every test added calls `t.Parallel()` and none sleeps.
  Checked by CI: ci-go (build, vet, test, golangci-lint, go-arch-lint), `checks` (arch); the design verdict on the pull request (an interface change).
  Pull request body: `Contracts: unchanged`.
  Depends on: none (merged work only). Order: before `broker-grpc`.
  Size: M

### broker-grpc (broker)

The client over the Unix socket
  Goal: `grpcbroker.Dial` returns a `broker.Broker` that speaks `ibkr.v1`, bounds every call and limits contract details in flight.
  Files: internal/broker/grpcbroker/dial.go, internal/broker/grpcbroker/convert.go, internal/broker/grpcbroker/errors.go, internal/broker/grpcbroker/pace.go, internal/broker/grpcbroker/client_test.go, internal/broker/grpcbroker/convert_test.go, internal/broker/grpcbroker/pace_test.go, docs/contracts/broker.md
  Design: docs/design/broker.md#interface
  Cross-module: none. It is attached after #166 merged with the content of PR 184 (the generated services, the new `Quote` fields and the grpc requirement); it adds no dependency and no generated file (`git diff -- go.mod go.sum buf.gen.yaml internal/contracts pyside` is empty).
  Acceptance:
  1. `dial.go` holds `var _ broker.DialFunc = Dial`; `go list -deps ./internal/broker/grpcbroker | grep optdesk/internal` lists only `broker`, `clock`, `contracts` and `contracts/genpb`.
  2. `TestSnapshotOrder`: a fake `MarketDataServiceServer` on a Unix socket in `t.TempDir()` streams its answers in reverse; `Snapshot` returns one `Quote` per contract in the order asked, and a contract with no streamed answer gets `StatusDisconnected`.
  3. `TestQuoteFields` (table): each `QuoteStatus` of the proto maps to its `broker.Status` and `QUOTE_STATUS_UNSPECIFIED` is an error; every optional double is nil when unset and a pointer when set (a set zero stays a pointer to 0); the sizes and `event_ts` of PR 184 map; `Generation`, `MarketDataType` and `ReceivedAt` (UTC) map.
  4. `TestKinds`: each `broker.Kind` maps to its proto `QuoteKind` and an unknown `Kind` is a `Permanent` error before any call; `TestResolveNotListed`: NOT_LISTED gives `Listed == false` and a nil error; several or differing results (the service's FAILED_PRECONDITION) give `Permanent`; `TestListChains` returns the strike union untouched.
  5. `TestErrorKinds` (table): `UNAVAILABLE` and `DEADLINE_EXCEEDED` give `Retryable`, `INVALID_ARGUMENT`, `FAILED_PRECONDITION` and every other code `Permanent`.
  6. `TestResolveInFlight`: with `ResolveInFlight` 2 and a fake server that blocks on a channel, at most 2 `ResolveContract` calls are in flight at once (the fake counts them; the test releases them by channel); a waiting call returns the context's error when its context is cancelled.
  7. `TestTimeouts`: with `CallTimeout` and `SnapshotTimeout` of one nanosecond and a fake server that never answers, each call fails with `Retryable`; a cancelled context ends a stream and leaves no goroutine (the fake's done channel is closed); no test sleeps.
  8. `TestDialRefuses`: a missing socket gives a typed error whose text holds no path (the test greps it).
  9. No TCP and no cache: `grep -c '"tcp"' internal/broker/grpcbroker/*.go` prints 0 per file; `grep -ciE 'cache' internal/broker/grpcbroker/*.go` prints 0 outside comments of `dial.go`; `grep -ciE 'order|account|position' internal/broker/grpcbroker/dial.go` prints 0; each new file is under 600 lines.
  10. `docs/contracts/broker.md` states the interface, the error kinds, the defaults, that the socket path is the caller's (`$OT_HOME/run/ibkr.sock`, `pyside-ibkr-transport.md:73`) and that the client keeps no state between calls.
  Checked by CI: ci-go (go test, golangci-lint, go-arch-lint component `broker/grpcbroker`).
  Depends on: broker-interface merged and #166 merged with PR 184 (merged work only).
  Size: M

Width: one entry after the interface. The client is one package whose files share the dial and the conversion, so a split would put two entries in one directory.

## Left out

1. Wiring in `cmd/ot` and the runner, the follow-up `ot-collect`, after the three designs have merged: `deps` for `DialFunc` and the constructors, the socket path, the import list and depends cell of `ot` (`modules.md:28`, `:78`). It names real constructors of every package, so it cannot be a parallel entry. No data flows until it lands.
2. `runner-window-per-dataset` (`store-week2.md:347-348`): `WindowRule` is one rule for all jobs (`reconcile.go:37`; `cmd/ot/runonce.go:96`). A prerequisite of `ot-collect`; an interface change of runner.
3. The pyside serving of `QUOTE_KIND_SNAPSHOT` and the new `Quote` fields (`ibkr-service`): until it lands the service answers `INVALID_ARGUMENT` to a `KindSnapshot` call.
4. A not-listed cache: dropped here because one `ot run-once` is one process and an in-memory cache would never be read twice. The 7 days plus or minus 2 days of ADR 0003 (`pyside-ibkr-transport.md:45`) need a store table written and read by the job; a follow-up design `not-listed-cache`. Until then every run pays for every not-listed pair.
5. The runner's consequence for any caller of this client: `NoValue` is marked at once and final (`reconcile.go:278-279`), and a run that ends in another error is not run again by `Reconcile` (only an empty answer before the window close and one crash re-run are, `reconcile.go:226-244`). A transport error of this client therefore leaves its key unmarked until someone runs `ot backfill`. Retries inside one call (for example a second dial) are not built; the caller decides.
6. Which strikes and expiries to ask and what the numbers mean: the jobs (`docs/design/option-jobs.md`). Time-of-day line budgets (`pyside-ibkr-transport.md:352`) and the index IV job (`store-week2.md:356-358`).
7. Scheduling (launchd), any live run against the gateway or a database, and any real call to the socket: the owner's; no entry runs one. Orders, positions, accounts and funds are never part of this: no type, rpc or method exists for them (`broker-interface` 1, `broker-grpc` 9).
8. Owner-only and second-opinion cases: `contracts/proto/` and `go.mod` are not touched here; the grpc library and its words from the owner belong to #166. The client reaches pyside by a local socket only and adds no outside path, since the gateway is reached by pyside alone, already a second-opinion case (`pyside-ibkr-transport.md:89-90`).
9. Textual risk: `.go-arch-lint.yml` is edited by the interface entries of all three designs and PR 185 edits `arch.toml` and `modules.md`; the entries are attached one after another so each rebases onto the last.

## Adversarial

1. Gap against the goal and stale against PR 184: adopted; the client entry no longer lists `buf.gen.yaml`, generated files or `go.mod`, and is attached after #166 merges with PR 184's content; wiring, per-dataset window and pyside serving are named in Left out.
2. Unnamed risk, the not-listed cache that never survives one process: adopted by dropping the cache (Left out 4, follow-up `not-listed-cache`); lease and one-key `Run`: bounded by call timeouts here and by the job budget in `docs/design/option-jobs.md`.
3. Simpler way, fold the proto and gRPC plugin into #166: adopted (settled by the advisor, PR 184); dropping the Go gRPC layer is impossible because pyside serves gRPC on a Unix socket.
4. Lock-in (`Broker` shape, no partial result): adopted in part; `Snapshot` no longer chunks, the caller chunks and writes per chunk, so a transport error costs one chunk; the interface stays small and plain-typed; stubs of other designs are no longer created here.
5. False claims (`arch.toml` lists directory components, one-module siblings): adopted; the exception is code in `scripts/arch` and a line of `.go-arch-lint.yml`, `arch.toml` is unchanged, and the stub package is in the interface entry.
6. Slip causes (stale Files and acceptance, `go.mod` conflicts, wide interface entry): adopted; `broker-grpc` is broker-only, the interface entry is two modules, and the three interface entries edit `.go-arch-lint.yml` and are attached one after another (Left out 9).

Full answer: scratchpad adversarial-broker-jobs.md and adversarial-option-collection.md of the dispatcher window (2026-11-07)
