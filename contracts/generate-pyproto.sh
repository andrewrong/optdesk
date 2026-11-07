#!/bin/sh
# Generate the Python output of contracts/proto into pyside/src/optdesk/contracts/genpb.
# grpc_tools.protoc (grpcio-tools) with the typed-stub plugin of mypy-protobuf, both
# from the locked environment of pyside. The generated imports `from ibkr.v1 import`
# are rewritten to the package path; the rewrite is done by uv run python, with no in-place sed.
set -eu
cd "$(dirname "$0")/.."

out=pyside/src/optdesk/contracts/genpb
rm -rf "$out"
mkdir -p "$out"

uv run --project pyside --locked python -m grpc_tools.protoc \
  -I contracts/proto \
  --python_out="$out" \
  --mypy_out="$out" \
  --grpc_python_out="$out" \
  --mypy_grpc_out="$out" \
  contracts/proto/ibkr/v1/*.proto

uv run --project pyside --locked python - "$out" <<'PY'
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
for path in sorted(root.rglob("*.py*")):
    text = path.read_text()
    fixed = text.replace("from ibkr.v1 import", "from optdesk.contracts.genpb.ibkr.v1 import")
    if fixed != text:
        path.write_text(fixed)
for directory in [root, root / "ibkr", root / "ibkr" / "v1"]:
    (directory / "__init__.py").write_text("")
PY
