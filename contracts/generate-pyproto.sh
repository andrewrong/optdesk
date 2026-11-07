#!/bin/sh
# Generate the Python output of contracts/proto into pyside/src/optdesk/contracts/genpb:
# messages, typed stubs, gRPC stubs and servicers (grpc_tools.protoc, mypy-protobuf plugin).
# Run on demand; contracts/generate.sh does not call it. Needs uv and the locked dev group of pyside
# (grpcio-tools, mypy-protobuf); --mypy_out writes the typed _pb2.pyi (protoc's own --pyi_out leaves bare Mapping types that mypy --strict refuses); the plugins are found on the PATH that `uv run` sets.
set -eu
cd "$(dirname "$0")/.."

OUT=pyside/src/optdesk/contracts/genpb
rm -rf "$OUT"
mkdir -p "$OUT"

uv run --project pyside --locked python -m grpc_tools.protoc \
  -I contracts/proto \
  --python_out="$OUT" \
  --mypy_out="$OUT" \
  --grpc_python_out="$OUT" \
  --mypy_grpc_out="$OUT" \
  contracts/proto/ibkr/v1/*.proto

# Rewrite the generated imports from the top-level ibkr package to the package they live in.
uv run --project pyside --locked python - "$OUT" <<'PY'
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
for path in sorted(root.rglob("*.py*")):
    text = path.read_text()
    new = text.replace("from ibkr.v1 import", "from optdesk.contracts.genpb.ibkr.v1 import")
    if new != text:
        path.write_text(new)
PY

: > "$OUT/__init__.py"
: > "$OUT/ibkr/__init__.py"
: > "$OUT/ibkr/v1/__init__.py"
