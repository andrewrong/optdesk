"""Dev-only generator of the two generated golden fixtures of internal/compute.

Run from this directory:  uv run gen.py
It writes indicators.json (outputs of Python TA-Lib) and iv30_independent.json (Iv30 by an
independent numpy implementation of the glossary arithmetic) into
internal/compute/contracttest/testdata/. It is not run in CI and nothing imports it at run time.
Every random input comes from a seeded numpy generator, so only generated_at differs between runs.
"""

from __future__ import annotations

import hashlib
import json
import math
import sys
from datetime import datetime, timezone
from importlib import metadata
from pathlib import Path

import numpy as np
import talib

HERE = Path(__file__).resolve().parent
TESTDATA = HERE.parents[1] / "internal" / "compute" / "contracttest" / "testdata"

MINUTES_PER_DAY = 1440
MINUTES_PER_YEAR = 525600
TARGET = 30 * MINUTES_PER_DAY
AS_OF = "2024-03-04"
RULE = {"id": "test.iv30", "version": 1, "unit": "decimal", "scale": 6, "max_age_days": 4}


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def provenance() -> dict:
    return {
        "generator": "gen.py",
        "libraries": {
            "TA-Lib": metadata.version("TA-Lib"),
            "numpy": metadata.version("numpy"),
            "python": ".".join(str(p) for p in sys.version_info[:3]),
        },
        "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "gen_sha256": sha256(HERE / "gen.py"),
        "lock_sha256": sha256(HERE / "uv.lock"),
    }


def clean(a: np.ndarray) -> list:
    """A TA-Lib output as JSON values: null for a warm-up NaN."""
    return [None if math.isnan(v) else float(v) for v in a]


def write(name: str, cases: list[dict]) -> None:
    lines = [json.dumps(c, separators=(", ", ": ")) for c in cases]
    body = '{\n"provenance": ' + json.dumps(provenance()) + ',\n"cases": [\n'
    body += ",\n".join(lines) + "\n]\n}\n"
    (TESTDATA / name).write_text(body)


# ---- indicators.json -------------------------------------------------------------------

MA = {"sma": talib.MA_Type.SMA, "ema": talib.MA_Type.EMA, "wma": talib.MA_Type.WMA}
TOL = 1e-9
SEED_BASE = 6000  # shifts every seed; changed only to give a different draw
N_BARS = 60


def bars(seed: int) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    rng = np.random.default_rng(seed)
    close = np.round(100 + np.cumsum(rng.normal(0, 1.5, N_BARS)), 2)
    high = np.round(close + rng.uniform(0.1, 2.0, N_BARS), 2)
    low = np.round(close - rng.uniform(0.1, 2.0, N_BARS), 2)
    return high, low, close


def indicator_cases() -> list[dict]:
    cases = []

    def add(group: str, tag: str, inp: dict, expected: dict) -> None:
        cases.append({
            "name": f"{group.lower()}_{tag}", "group": group, "input": inp,
            "expected": {k: clean(v) for k, v in expected.items()}, "tolerance": TOL,
        })

    for i, seed in enumerate((SEED_BASE + 11, SEED_BASE + 12, SEED_BASE + 13)):
        _, _, close = bars(seed)
        v = close.tolist()
        for group, fn, periods in (
            ("SMA", talib.SMA, (3, 5, 10)), ("EMA", talib.EMA, (3, 5, 10)),
            ("WMA", talib.WMA, (3, 5, 10)), ("RSI", talib.RSI, (2, 5, 14)),
            ("ROC", talib.ROC, (1, 5, 10)),
        ):
            p = periods[i]
            add(group, f"p{p}", {"values": v, "period": p}, {"values": fn(close, timeperiod=p)})
        fast, slow, sig = ((3, 6, 4), (5, 10, 3), (12, 26, 9))[i]
        m, s, h = talib.MACD(close, fastperiod=fast, slowperiod=slow, signalperiod=sig)
        add("MACD", f"{fast}_{slow}_{sig}", {"values": v, "fast": fast, "slow": slow, "signal": sig},
            {"macd": m, "signal": s, "hist": h})
        period, up, dn, ma = ((5, 2.0, 2.0, "sma"), (10, 1.5, 2.5, "ema"), (5, 2.0, 2.0, "wma"))[i]
        u, mid, lo = talib.BBANDS(close, timeperiod=period, nbdevup=up, nbdevdn=dn, matype=MA[ma])
        add("BBands", f"p{period}_{ma}",
            {"values": v, "period": period, "up_dev": up, "down_dev": dn, "ma_type": ma},
            {"upper": u, "middle": mid, "lower": lo})
        high, low, close2 = bars(seed + 100)
        p = (3, 5, 14)[i]
        add("ATR", f"p{p}",
            {"high": high.tolist(), "low": low.tolist(), "close": close2.tolist(), "period": p},
            {"values": talib.ATR(high, low, close2, timeperiod=p)})
    return cases


# ---- iv30_independent.json -------------------------------------------------------------


def make_chain(rng: np.random.Generator, mode: str) -> tuple[list[dict], float]:
    """A seeded chain: strikes 80..120 by 2.5, mids with C-P = exp(-rT)(F-K), a smile of IV."""
    rate = round(float(rng.uniform(0.01, 0.06)), 4)
    spot = 100.0
    if mode == "exact30":
        days = [12, 30, 55]
    elif mode == "short_only":
        days = [9, 16, 25]
    elif mode == "long_only":
        days = [33, 47, 80]
    else:
        days = [int(rng.integers(2, 30)), int(rng.integers(31, 60)), int(rng.integers(60, 120))]
        days = sorted(set(days + [int(rng.integers(0, 2))]))
    chain = []
    for d in days:
        minutes = d * MINUTES_PER_DAY + (0 if d in (0, 1, 30) else int(rng.integers(0, MINUTES_PER_DAY)))
        t = minutes / MINUTES_PER_YEAR
        fwd = spot * math.exp(rate * t) + float(rng.normal(0, 0.8))
        base = float(rng.uniform(0.15, 0.45))
        strikes = []
        for k in np.arange(80.0, 120.01, 2.5):
            disc_diff = math.exp(-rate * t) * (fwd - k)
            tv = 1.0 + float(rng.uniform(0, 2))
            call_mid = tv + max(disc_diff, 0.0)
            put_mid = tv + max(-disc_diff, 0.0)
            smile = base + 0.0004 * (k - fwd) ** 2 / 10
            strikes.append({
                "strike": float(k),
                "call_iv": round(smile + float(rng.normal(0, 0.004)), 4),
                "put_iv": round(smile + float(rng.normal(0, 0.004)), 4),
                "call_mid": round(call_mid, 4), "put_mid": round(put_mid, 4),
                "has_call": True, "has_put": True,
            })
        chain.append({"minutes_to_expiry": minutes, "strikes": strikes})
    return chain, rate


def independent_iv30(chain: list[dict], rate: float) -> tuple[float | None, int]:
    """The glossary arithmetic ("Own IV30") in numpy, written without reference to the Go code."""
    usable = []  # (minutes, atm iv)
    for e in chain:
        n = e["minutes_to_expiry"]
        if n < 2 * MINUTES_PER_DAY:  # 0 and 1 day are ignored
            continue
        k = np.array([s["strike"] for s in e["strikes"]])
        c = np.array([s["call_mid"] for s in e["strikes"]])
        p = np.array([s["put_mid"] for s in e["strikes"]])
        order = np.argsort(k, kind="stable")
        k, c, p = k[order], c[order], p[order]
        i_star = int(np.argmin(np.abs(c - p)))  # first minimum: the lower strike on a tie
        fwd = k[i_star] + np.exp(rate * n / MINUTES_PER_YEAR) * (c[i_star] - p[i_star])
        iv_c = np.array([s["call_iv"] for s in e["strikes"]])[order]
        iv_p = np.array([s["put_iv"] for s in e["strikes"]])[order]
        j = int(np.argmin(np.abs(k - fwd)))
        usable.append((n, float((iv_c[j] + iv_p[j]) / 2)))
    for n, iv in usable:
        if n == TARGET:
            return iv, 1
    below = [u for u in usable if u[0] < TARGET]
    above = [u for u in usable if u[0] > TARGET]
    if not below or not above:
        return None, 0
    n1, s1 = max(below)
    n2, s2 = min(above)
    t1, t2 = n1 / MINUTES_PER_YEAR, n2 / MINUTES_PER_YEAR
    var = (t1 * s1**2 * (n2 - TARGET) / (n2 - n1) + t2 * s2**2 * (TARGET - n1) / (n2 - n1))
    return float(np.sqrt(var * MINUTES_PER_YEAR / TARGET)), 2


def iv30_cases() -> list[dict]:
    cases = []
    modes = ["bracket"] * 14 + ["exact30"] * 2 + ["short_only", "long_only"]
    for i, mode in enumerate(modes):
        rng = np.random.default_rng(SEED_BASE + 500 + i)
        chain, rate = make_chain(rng, mode)
        stale = i == 3
        rate_day = "2024-03-01" if stale else AS_OF
        value, n = independent_iv30(chain, rate)
        ok = value is not None
        cases.append({
            "name": f"iv30_seed{SEED_BASE + 500 + i}_{mode}", "group": "Iv30",
            "input": {"rule": RULE, "as_of": AS_OF, "rate": {"day": rate_day, "value": rate}, "chain": chain},
            "expected": {
                "status": "ok" if ok else "missing", "reason": "" if ok else "no_input",
                "value": value if ok else 0, "data_day": AS_OF if ok else "",
                "observations": n, "stale_rate": stale,
            },
            "tolerance": 1e-9,
        })
    return cases


def main() -> None:
    TESTDATA.mkdir(parents=True, exist_ok=True)
    write("indicators.json", indicator_cases())
    write("iv30_independent.json", iv30_cases())


if __name__ == "__main__":
    main()
