# contracts

The shared schemas of optdesk: dataset schemas, page schemas, the Go records and the proto tree.

## Proto tree

`contracts/proto/` holds exactly one proto file in week 1: `contracts/proto/ibkr/v1/session.proto`
(package `ibkr.v1`, one rpc `Ping`). Its Go output is `internal/contracts/genpb/session.pb.go`;
that file is generated and never edited by hand.

## Generating

Run `sh contracts/generate.sh`; it calls `sh contracts/generate-proto.sh`, which runs
`go tool buf lint` and then `go tool buf generate`. Both tools come from the `tool` block of
`go.mod`, so nothing is downloaded beyond the Go module cache. The source and its generated
output land in the same pull request.

## Versions

The version rule (record rN, page schema vN, proto v1, job manifest version) lives in
`docs/contracts/00-protocol.md`.
