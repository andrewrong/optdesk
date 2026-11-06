#!/bin/sh
# Regenerate everything derived from contracts/: Go records, dataset pages, TS stub.
# Go tools only (go.mod tool directive and contracts/gendoc); no npx, no download beyond the module cache.
set -eu
cd "$(dirname "$0")/.."

D=https://optdesk.invalid/datasets
go tool go-jsonschema -p gen --capitalization ID \
  --schema-root-type "$D/master.session_day.r1.json=SessionDay" \
  --schema-root-type "$D/obs.series_point.r1.json=SeriesPoint" \
  -o internal/contracts/gen/records.go \
  contracts/datasets/master.session_day.r1.json contracts/datasets/obs.series_point.r1.json
gofmt -w internal/contracts/gen/records.go

go run ./contracts/gendoc -in contracts/datasets -out docs/contracts/datasets

if [ -f contracts/generate-proto.sh ]; then
  sh contracts/generate-proto.sh
fi
