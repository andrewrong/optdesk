#!/bin/sh
# Lint and generate the Go output of contracts/proto into internal/contracts/genpb.
# Go tools only (go.mod tool directive); no download beyond the module cache.
set -eu
cd "$(dirname "$0")/.."

go tool buf lint
go tool buf generate
