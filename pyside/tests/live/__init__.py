"""Read-only live integration tests of the deployed pyside sockets (local only).

They call the sockets of the running ibkr service and pin the answers that only the
real gateway shows. Every test but test_guard.py is skipped unless OT_LIVE_PYSIDE=1,
so CI and a plain `pytest -n auto` never connect.

Run:

    cd pyside && OT_LIVE_PYSIDE=1 OT_HOME=<ot home> OT_IBKR_HEALTH_PORT=<port> uv run pytest tests/live -n0
"""
