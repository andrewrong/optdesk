# contracts

The shared schemas of optdesk: dataset schemas, page schemas, the Go records and the proto tree.

## Proto tree

`contracts/proto/ibkr/v1/` holds seven proto files, all of package `ibkr.v1`: `session`, `contract`,
`chains`, `resolve`, `quote_types`, `quote` and `marketdata`. They declare `SessionService` (`Ping`),
`ContractsService` (`ListChains`, `ResolveContract`) and `MarketDataService` (`Snapshot`). Their Go output
is `internal/contracts/genpb/*.pb.go` (messages only: no Go gRPC plugin is configured); their Python output
is `pyside/src/optdesk/contracts/genpb/` (messages, typed stubs, stubs and servicers). Generated files
are never edited by hand.

## Generating

Run `sh contracts/generate.sh`; it calls `sh contracts/generate-proto.sh`, which runs
`go tool buf lint` and then `go tool buf generate`. Both tools come from the `tool` block of
`go.mod`, so nothing is downloaded beyond the Go module cache. The source and its generated
output land in the same pull request.

The Python output comes from `sh contracts/generate-pyproto.sh`. It runs `grpc_tools.protoc` with the
mypy-protobuf plugins through `uv run --project pyside --locked`, rewrites the generated imports
`from ibkr.v1 import` to `from optdesk.contracts.genpb.ibkr.v1 import` and writes the empty
`__init__.py` files. `pyside/tests/contracts/test_contracts_api.py` fails when the committed output is stale.

## Versions

The version rule (record rN, page schema vN, proto v1, job manifest version) lives in
`docs/contracts/00-protocol.md`.
