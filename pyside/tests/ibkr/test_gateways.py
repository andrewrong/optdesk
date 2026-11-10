"""The gateways file and the single-gateway fallback."""

from __future__ import annotations

from pathlib import Path

import pytest

from optdesk.ibkr.gateways import ConfigError, from_env, load_gateways

EXAMPLE = Path(__file__).resolve().parents[2] / "examples" / "gateways.example.toml"


def entry(name: str = "a", port: int = 1, **more: str) -> str:
    lines = [
        "[[gateway]]",
        f'name = "{name}"',
        f"port = {port}",
        'account = "alias"',
        'collect = ["market_data"]',
        "required = true",
    ]
    lines += [f"{key} = {value}" for key, value in more.items()]
    return "\n".join(lines) + "\n"


def write(tmp_path: Path, text: str) -> Path:
    path = tmp_path / "gateways.toml"
    path.write_text(text)
    return path


def test_two_entries_with_defaults(tmp_path: Path) -> None:
    text = entry("a", 1) + entry("b", 2).replace("required = true\n", "")
    specs, default = load_gateways(write(tmp_path, text))
    assert [s.name for s in specs] == ["a", "b"] and default == "a"
    second = specs[1]
    assert second.host == "localhost" and second.client_id == 33
    assert second.market_data_type == 2 and second.required is False
    assert specs[0].required is True


def test_first_entry_is_default_without_key(tmp_path: Path) -> None:
    _, default = load_gateways(write(tmp_path, entry("x", 1) + entry("y", 2)))
    assert default == "x"


def test_default_key_names_an_entry(tmp_path: Path) -> None:
    _, default = load_gateways(
        write(tmp_path, 'default = "y"\n' + entry("x", 1) + entry("y", 2))
    )
    assert default == "y"


def test_default_without_market_data_is_error(tmp_path: Path) -> None:
    other = entry("y", 2).replace('["market_data"]', '["positions"]')
    text = 'default = "y"\n' + entry("x", 1) + other
    with pytest.raises(ConfigError, match="default"):
        load_gateways(write(tmp_path, text))


def test_first_entry_without_market_data_and_no_default_is_error(
    tmp_path: Path,
) -> None:
    first = entry("x", 1).replace('["market_data"]', '["positions"]')
    with pytest.raises(ConfigError, match="default"):
        load_gateways(write(tmp_path, first + entry("y", 2)))


def test_from_env_is_one_default_entry() -> None:
    (spec,) = from_env({"OT_IBKR_GATEWAY_PORT": "4002"})
    assert spec.name == "default" and spec.port == 4002
    assert spec.collect == ("market_data",) and spec.required is True
    assert spec.host == "127.0.0.1" and spec.client_id == 33
    with pytest.raises(ConfigError, match="OT_IBKR_GATEWAY_PORT"):
        from_env({})


def fails(tmp_path: Path, text: str, key: str, value: str | None = None) -> None:
    with pytest.raises(ConfigError, match=key) as caught:
        load_gateways(write(tmp_path, text))
    if value is not None:
        assert value not in str(caught.value)


def test_unknown_key(tmp_path: Path) -> None:
    fails(tmp_path, entry(portt="3"), "portt")
    fails(tmp_path, "colour = 1\n" + entry(), "colour")


def test_no_gateway(tmp_path: Path) -> None:
    fails(tmp_path, 'default = "x"\n', "gateway")


def test_none_required(tmp_path: Path) -> None:
    fails(tmp_path, entry().replace("required = true", "required = false"), "required")


def test_duplicate_name(tmp_path: Path) -> None:
    fails(tmp_path, entry("same", 1) + entry("same", 2), "name", "same")


def test_duplicate_host_port_client(tmp_path: Path) -> None:
    fails(tmp_path, entry("a", 7) + entry("b", 7), "port")


def test_bad_name_slug(tmp_path: Path) -> None:
    fails(tmp_path, entry("Bad_Name"), "name", "Bad_Name")


def test_bad_port(tmp_path: Path) -> None:
    for bad in ("0", '"text"', "-5"):
        text = entry().replace("port = 1", f"port = {bad}")
        fails(tmp_path, text, "port", bad.strip('"'))


def test_bad_collect_word(tmp_path: Path) -> None:
    text = entry().replace('["market_data"]', '["market_data", "orders"]')
    fails(tmp_path, text, "collect", "orders")


def test_executions_is_a_collect_word(tmp_path: Path) -> None:
    text = entry().replace('["market_data"]', '["market_data", "executions"]')
    specs, _ = load_gateways(write(tmp_path, text))
    assert specs[0].collect == ("market_data", "executions")


def test_empty_collect(tmp_path: Path) -> None:
    fails(tmp_path, entry().replace('["market_data"]', "[]"), "collect")


def test_account_shaped_like_an_id(tmp_path: Path) -> None:
    shaped = "abc" + "12345"
    fails(tmp_path, entry().replace('"alias"', f'"{shaped}"'), "account", shaped)


def test_missing_account(tmp_path: Path) -> None:
    fails(tmp_path, entry().replace('account = "alias"\n', ""), "account")


def test_example_file_is_valid() -> None:
    specs, default = load_gateways(EXAMPLE)
    assert len(specs) == 2 and default == "first"
    assert [s.required for s in specs] == [True, False]
