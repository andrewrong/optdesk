# contracts

The shared schemas of optdesk: dataset schemas, page schemas, the Go records and the proto tree.

## Proto tree

`contracts/proto/ibkr/v1/` holds three proto files, all package `ibkr.v1`: `session.proto`
(`SessionService`, rpc `Ping`), `contracts.proto` (`ContractsService`, rpcs `ListChains` and
`ResolveContract`) and `marketdata.proto` (`MarketDataService`, rpc `Snapshot`). Their Go output is
`internal/contracts/genpb/*.pb.go`: messages only, because no Go gRPC plugin is added. Those files are
generated and never edited by hand.

## Generating

Run `sh contracts/generate.sh`; it calls `sh contracts/generate-proto.sh`, which runs
`go tool buf lint` and then `go tool buf generate`. Both tools come from the `tool` block of
`go.mod`, so nothing is downloaded beyond the Go module cache. The source and its generated
output land in the same pull request.

The Python output is generated on demand by `sh contracts/generate-pyproto.sh`, which runs
`grpc_tools.protoc` through `uv run --project pyside --locked` into
`pyside/src/optdesk/contracts/genpb/` and rewrites the generated imports to that package.
`pyside/tests/contracts/test_contracts_api.py` fails when the committed output is stale.
Python code imports the generated names through `optdesk.contracts.api` only.

## Versions

The version rule (record rN, page schema vN, proto v1, job manifest version) lives in
`docs/contracts/00-protocol.md`.
