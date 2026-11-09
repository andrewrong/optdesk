"""The gateways of the ibkr server: the file that names them, and the single-gateway fallback.

A gateway entry says which IB Gateway to connect to, which account it serves
(by alias, never by id), which information is collected through it and whether
its connection decides the health of the service. Every error names the table
by position and the key, never the value.
"""

from __future__ import annotations

import re
import tomllib
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any

COLLECT_WORDS = ("market_data", "account_summary", "positions", "executions")
DEFAULT_CLIENT_ID = 33
DEFAULT_MARKET_DATA_TYPE = 1
FALLBACK_NAME = "default"

_NAME = re.compile(r"[a-z][a-z0-9-]{0,15}")
_ALIAS = re.compile(r"[a-z][a-z0-9_-]{0,31}")
_ACCOUNT_ID = re.compile(r"[A-Za-z]+\d{5,9}")
_TOP_KEYS = frozenset({"default", "gateway"})
_KEYS = frozenset(
    {
        "name",
        "port",
        "host",
        "client_id",
        "market_data_type",
        "account",
        "collect",
        "required",
    }
)


class ConfigError(Exception):
    """A required variable is missing or a value is not usable; names the key only."""


@dataclass(frozen=True)
class GatewaySpec:
    """One configured gateway."""

    name: str
    port: int
    account: str
    collect: tuple[str, ...]
    host: str = "localhost"
    client_id: int = DEFAULT_CLIENT_ID
    market_data_type: int = DEFAULT_MARKET_DATA_TYPE
    required: bool = False


def positive_int(env: Mapping[str, str], name: str, default: int | None) -> int:
    """Read the whole number above zero in env[name]; default None means required."""
    raw = env.get(name)
    if raw is None or raw == "":
        if default is None:
            raise ConfigError(f"{name} is required")
        return default
    try:
        value = int(raw)
    except ValueError:
        raise ConfigError(f"{name} must be a whole number") from None
    if value <= 0:
        raise ConfigError(f"{name} must be above zero")
    return value


def from_env(env: Mapping[str, str]) -> tuple[GatewaySpec, ...]:
    """Return the one gateway of the single-gateway keys: named default, required."""
    return (
        GatewaySpec(
            name=FALLBACK_NAME,
            host=env.get("OT_IBKR_GATEWAY_HOST") or "127.0.0.1",
            port=positive_int(env, "OT_IBKR_GATEWAY_PORT", None),
            client_id=positive_int(env, "OT_IBKR_CLIENT_ID", DEFAULT_CLIENT_ID),
            market_data_type=positive_int(
                env, "OT_IBKR_MARKET_DATA_TYPE", DEFAULT_MARKET_DATA_TYPE
            ),
            account=FALLBACK_NAME,
            collect=("market_data",),
            required=True,
        ),
    )


def _whole(table: Mapping[str, Any], key: str, where: str, default: int | None) -> int:
    if key not in table:
        if default is None:
            raise ConfigError(f"{where}: {key} is required")
        return default
    value = table[key]
    if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
        raise ConfigError(f"{where}: {key} must be a whole number above zero")
    return value


def _text(table: Mapping[str, Any], key: str, where: str) -> str:
    if key not in table:
        raise ConfigError(f"{where}: {key} is required")
    value = table[key]
    if not isinstance(value, str):
        raise ConfigError(f"{where}: {key} must be text")
    return value


def _entry(table: object, position: int) -> GatewaySpec:
    where = f"gateway {position}"
    if not isinstance(table, dict):
        raise ConfigError(f"{where}: must be a table")
    for key in table:
        if key not in _KEYS:
            raise ConfigError(f"{where}: unknown key {key!r}")
    name = _text(table, "name", where)
    if not _NAME.fullmatch(name):
        raise ConfigError(f"{where}: name must be a slug of 1 to 16 characters")
    account = _text(table, "account", where)
    if not _ALIAS.fullmatch(account) or _ACCOUNT_ID.search(account):
        raise ConfigError(f"{where}: account must be an alias, never an account id")
    host = table.get("host", "localhost")
    if not isinstance(host, str) or not host:
        raise ConfigError(f"{where}: host must be text")
    collect = table.get("collect")
    if (
        not isinstance(collect, list)
        or not collect
        or not all(isinstance(word, str) and word in COLLECT_WORDS for word in collect)
    ):
        raise ConfigError(f"{where}: collect must be a non-empty list of known words")
    required = table.get("required", False)
    if not isinstance(required, bool):
        raise ConfigError(f"{where}: required must be true or false")
    return GatewaySpec(
        name=name,
        port=_whole(table, "port", where, None),
        account=account,
        collect=tuple(collect),
        host=host,
        client_id=_whole(table, "client_id", where, DEFAULT_CLIENT_ID),
        market_data_type=_whole(
            table, "market_data_type", where, DEFAULT_MARKET_DATA_TYPE
        ),
        required=required,
    )


def _check(specs: tuple[GatewaySpec, ...], default: str | None) -> str:
    if not specs:
        raise ConfigError("at least one [[gateway]] is required")
    if not any(spec.required for spec in specs):
        raise ConfigError("at least one gateway needs required = true")
    names: set[str] = set()
    seen: set[tuple[str, int, int]] = set()
    for position, spec in enumerate(specs, start=1):
        if spec.name in names:
            raise ConfigError(f"gateway {position}: name is a duplicate")
        names.add(spec.name)
        where = (spec.host, spec.port, spec.client_id)
        if where in seen:
            raise ConfigError(
                f"gateway {position}: host, port and client_id are a duplicate"
            )
        seen.add(where)
    chosen = (
        specs[0]
        if default is None
        else next((spec for spec in specs if spec.name == default), None)
    )
    if chosen is None:
        raise ConfigError("default must name a gateway")
    if "market_data" not in chosen.collect:
        raise ConfigError(
            "default: the gateway on ibkr.sock must have market_data in collect"
        )
    return chosen.name


def load_gateways(path: Path | str) -> tuple[tuple[GatewaySpec, ...], str]:
    """Read and validate the gateways file; return the specs and the default name."""
    try:
        with open(path, "rb") as handle:
            data = tomllib.load(handle)
    except OSError:
        raise ConfigError("OT_IBKR_GATEWAYS: the file cannot be read") from None
    except tomllib.TOMLDecodeError:
        raise ConfigError("OT_IBKR_GATEWAYS: the file is not valid TOML") from None
    for key in data:
        if key not in _TOP_KEYS:
            raise ConfigError(f"unknown key {key!r}")
    default = data.get("default")
    if default is not None and not isinstance(default, str):
        raise ConfigError("default must be text")
    tables = data.get("gateway", [])
    if not isinstance(tables, list):
        raise ConfigError("gateway must be a list of tables")
    specs = tuple(_entry(table, i) for i, table in enumerate(tables, start=1))
    return specs, _check(specs, default)
