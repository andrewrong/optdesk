"""Dev-only generator of the three golden files of internal/compute/valuation.

Run from this directory:  uv run gen.py /path/to/option-trader
The argument is a checkout of option-trader at 4704569f (read only; its src/ is put on sys.path).
The generator refuses a checkout whose six parameter files are not the ones pinned by git blob
id in docs/design/valuation.md (Decision 1). It builds synthetic filings inputs, calls the old
Python functions on them and writes, into internal/compute/valuation/testdata/:

  golden_fcf.json        8 cases: fcf_yield v2 (the band) and the reverse DCF (implied growth)
  golden_return.json    14 cases: price_return v1, with each refusal and GROWTH_LIMITED_BY_R
  golden_questions.json  6 cases: five_questions v1 (three) and diagnosis v1 (three)

Each file is {"source_commit": "4704569f", "cases": [...]}; a case is {"name", "input",
"expected"}. Every number is Decimal text (format "f", no exponent), a day is YYYY-MM-DD, an
absent figure is null. The inputs are in the shapes of the old functions (the Go tests adapt
them): fcf cases give the fiscal years as OCF, capex and SBC with the diluted shares, cover
shares and long-term debt; return cases give FiscalYear rows; question cases give the figures
by concept name of the parameter files and by fiscal-year end.

Nothing here is random: a second run writes the same bytes. It is not run in CI and nothing
imports it at run time; the committed JSON is what the tests read.
"""

from __future__ import annotations

import hashlib
import json
import sys
import tempfile
from datetime import date, datetime, timedelta, timezone
from decimal import ROUND_HALF_EVEN, Decimal
from pathlib import Path

HERE = Path(__file__).resolve().parent
TESTDATA = HERE.parents[2] / "internal" / "compute" / "valuation" / "testdata"
SOURCE_COMMIT = "4704569f"
PARAMS = "src/option_trader/valuation/params"
# git blob ids of the parameter files, as pinned in docs/design/valuation.md Decision 1.
PINNED = {
    "fcf_yield_v2.toml": "4952b59720ce",
    "reverse_dcf_v1.toml": "e1e1f731c7b8",
    "price_return_v1.toml": "8d7be25e35c3",
    "five_questions_v1.toml": "6755537f67d5",
    "diagnosis_v1.toml": "643178784313",
    "filing_rows_v1.toml": "967d8d64ff48",
}


def blob_id(data: bytes) -> str:
    return hashlib.sha1(b"blob %d\0" % len(data) + data).hexdigest()


def load_old(checkout: Path) -> dict[str, bytes]:
    """Check the pinned parameter files, put the checkout's src on sys.path, return the bytes."""
    files = {}
    for name, short in PINNED.items():
        path = checkout / PARAMS / name
        if not path.is_file():
            sys.exit(f"{path} not found: pass an option-trader checkout at {SOURCE_COMMIT}")
        data = path.read_bytes()
        if not blob_id(data).startswith(short):
            sys.exit(f"{name} is not the file pinned at {short}: the checkout is not {SOURCE_COMMIT}")
        files[name] = data
    sys.path.insert(0, str(checkout / "src"))
    return files


# ---- text helpers ----------------------------------------------------------------------


def t(value: Decimal | None) -> str | None:
    return None if value is None else format(value, "f")


def ts(values) -> list[str | None]:
    return [t(v) for v in values]


def d(text: str) -> Decimal:
    return Decimal(text)


def money(x: Decimal) -> Decimal:
    return x.quantize(Decimal(1), ROUND_HALF_EVEN)


def wobble(i: int, k: int) -> Decimal:
    """A deterministic -2%..+2% factor, so medians and ranges are not trivial."""
    return Decimal(1) + Decimal((i * 7 + k * 3) % 5 - 2) / 100


def write(name: str, cases: list[dict]) -> None:
    body = json.dumps({"source_commit": SOURCE_COMMIT, "cases": cases}, indent=2) + "\n"
    (TESTDATA / name).write_text(body)
    print(f"{name}: {len(cases)} cases")


# ---- golden_fcf.json -------------------------------------------------------------------

CIK = 1234567
ACCN = "0001234567-25-%06d"


def company_facts(case: dict) -> bytes:
    """The companyfacts JSON of one fcf case: one 10-K per fiscal year, as the SEC serves it."""
    us: dict[str, dict] = {}

    def add(concept: str, unit: str, start: str | None, end: str, val: str, filed: str, n: int):
        item = {"end": end, "val": int(val), "accn": ACCN % n, "form": "10-K", "filed": filed}
        if start is not None:
            item["start"] = start
        us.setdefault(concept, {"units": {}})["units"].setdefault(unit, []).append(item)

    years = case["years"]
    for n, y in enumerate(years):
        filed = (date.fromisoformat(y["end"]) + timedelta(days=50)).isoformat()
        add("NetCashProvidedByUsedInOperatingActivities", "USD", y["start"], y["end"], y["ocf"], filed, n)
        if y["capex"] is not None:
            add("PaymentsToAcquirePropertyPlantAndEquipment", "USD", y["start"], y["end"], y["capex"], filed, n)
        if y["sbc"] is not None:
            add("ShareBasedCompensation", "USD", y["start"], y["end"], y["sbc"], filed, n)
    last = years[-1]
    last_filed = (date.fromisoformat(last["end"]) + timedelta(days=50)).isoformat()
    last_n = len(years) - 1
    if case["shares"] is not None:
        add("WeightedAverageNumberOfDilutedSharesOutstanding", "shares", last["start"], last["end"],
            case["shares"], last_filed, last_n)
    if case["debt"] is not None:
        add("LongTermDebt", "USD", None, last["end"], case["debt"], last_filed, last_n)
    doc: dict = {"facts": {"us-gaap": us}}
    if case["cover_shares"] is not None:
        doc["facts"]["dei"] = {"EntityCommonStockSharesOutstanding": {"units": {"shares": [
            {"end": case["cover_date"], "val": int(case["cover_shares"]), "accn": ACCN % 99,
             "form": "10-K", "filed": case["cover_date"]}]}}}
    return json.dumps(doc).encode()


def fcf_years(n: int, first: int, base: int, grow: Decimal, capex_m: Decimal, sbc_m: Decimal, over: dict | None = None):
    """n fiscal years ending Dec 31 of first..first+n-1: OCF from base, growing by grow."""
    rows = []
    for i in range(n):
        y = first + i
        ocf = money(Decimal(base) * (1 + grow) ** i * wobble(i, 1))
        row = {"start": f"{y}-01-01", "end": f"{y}-12-31", "ocf": str(ocf),
               "capex": str(money(ocf * capex_m * wobble(i, 2))),
               "sbc": str(money(ocf * sbc_m * wobble(i, 3)))}
        row.update((over or {}).get(i, {}))
        rows.append(row)
    return rows


def fcf_case(name: str, years, *, shares="400000000", cover="402000000", debt="900000000",
             as_of="2025-03-10") -> dict:
    return {"name": name, "years": years, "shares": shares, "cover_shares": cover,
            "cover_date": "2025-02-01", "debt": debt, "as_of": as_of}


def fcf_cases() -> list[dict]:
    base = 5_000_000_000
    cases = [
        fcf_case("stable", fcf_years(5, 2020, base, d("0.04"), d("0.20"), d("0.05"))),
        fcf_case("growing_high_sbc_high_leverage",
                 fcf_years(5, 2020, base, d("0.15"), d("0.15"), d("0.30")), debt="60000000000"),
        # latest year's FCF is negative while the median is positive: the band is formed,
        # the reverse DCF refuses non_positive_base_fcf
        fcf_case("negative_base_year",
                 fcf_years(5, 2020, base, d("0.03"), d("0.20"), d("0.05"),
                           {4: {"capex": "7000000000"}})),
        # no debt fact, no cover count: leverage_unknown and shares_unverified
        fcf_case("unknown_leverage_unverified_shares",
                 fcf_years(5, 2020, base, d("0.05"), d("0.18"), d("0.04")), debt=None, cover=None),
        fcf_case("negative_capex_reported",
                 fcf_years(5, 2020, base, d("0.05"), d("0.18"), d("0.04"),
                           {2: {"capex": "-120000000"}})),
        fcf_case("shares_changed_since_base",
                 fcf_years(5, 2020, base, d("0.05"), d("0.18"), d("0.04")), cover="560000000"),
        fcf_case("insufficient_years", fcf_years(4, 2021, base, d("0.05"), d("0.18"), d("0.04"))),
        fcf_case("non_positive_median_fcf",
                 fcf_years(5, 2020, base, d("0.02"), d("1.10"), d("0.05"))),
    ]
    return cases


def run_fcf(old: dict, checkout: Path) -> list[dict]:
    from option_trader.sources.sec.companyfacts import RawCompanyFacts
    from option_trader.valuation.facts import DuckDbFactStore
    from option_trader.valuation.fcf_yield import FcfDraft, FcfParams, fcf_yield_draft
    from option_trader.valuation.reverse_dcf import ReverseDcfParams, implied_growth
    from option_trader.valuation.reverse_dcf_draft import (
        ReverseDcfDraft,
        reverse_dcf_draft,
    )

    fcf_params = FcfParams.from_toml(old["fcf_yield_v2.toml"])
    rd_params = ReverseDcfParams.from_toml(old["reverse_dcf_v1.toml"])
    retrieved = datetime(2025, 3, 10, tzinfo=timezone.utc)
    out = []
    with tempfile.TemporaryDirectory(dir=HERE) as tmp:
        for i, case in enumerate(fcf_cases()):
            body = company_facts(case)
            store = DuckDbFactStore(Path(tmp) / f"facts{i}.duckdb")
            store.save(RawCompanyFacts(CIK, body, hashlib.sha256(body).hexdigest(), retrieved))
            cutoff = date.fromisoformat(case["as_of"])
            band = fcf_yield_draft(store, CIK, cutoff, fcf_params)
            rev = reverse_dcf_draft(store, CIK, cutoff, rd_params, spec=fcf_params,
                                    spec_hash=fcf_params.param_hash)
            expected: dict = {}
            if isinstance(band, FcfDraft):
                expected["band"] = {
                    "state": "ok",
                    "fcf": ts(r.fcf for r in band.rows),
                    "shares": t(band.shares),
                    "median_fcf": t(band.median_fcf),
                    "values": {t(y): t(v) for y, v in band.values},
                    "primary_value": t(band.primary_value),
                    "flags": list(band.flags),
                }
            else:
                expected["band"] = {"state": "rejected", "reasons": list(band.reasons)}
            if isinstance(rev, ReverseDcfDraft):
                spots = reverse_spots(band, rev)
                expected["reverse"] = {
                    "state": "ok",
                    "base_fcf": t(rev.base_fcf),
                    "base_period_end": rev.base_period_end.isoformat(),
                    "shares": t(rev.shares),
                    "median_fcf": t(rev.median_fcf),
                    "flags": list(rev.flags),
                    "grid": [{"rate": t(p.rate), "growth": t(p.growth), "value": t(p.value),
                              "terminal_share": t(p.terminal_share),
                              "final_cash_flow": t(p.final_cash_flow)} for p in rev.grid],
                    "implied": [
                        {"spot": s, "rates": [
                            {"rate": t(r), "status": g.status.value, "growth": t(g.growth)}
                            for r in rd_params.discount_rates
                            for g in [implied_growth(rev.base_fcf, rev.shares, d(s), rate=r,
                                                     params=rd_params)]]}
                        for s in spots],
                }
            else:
                expected["reverse"] = {"state": "rejected", "reasons": list(rev.reasons)}
            out.append({"name": case["name"], "input": case, "expected": expected})
    return out


def reverse_spots(band, rev) -> list[str]:
    """Spot prices: one below the solver range, three around the band, one above it."""
    centre = band.primary_value if hasattr(band, "primary_value") else rev.base_fcf / rev.shares
    spots = ["0.01"]
    for k in ("0.5", "1", "2"):
        spots.append(t((centre * d(k)).quantize(Decimal("0.01"))))
    spots.append(t((centre * 1000).quantize(Decimal("0.01"))))
    return spots


# ---- golden_return.json ----------------------------------------------------------------

PRICE_DATE = date(2025, 3, 15)


def return_years(n=10, last=2024, *, rev0=10_000_000_000, g=Decimal("0.06"), ocf_m=Decimal("0.20"),
                 capex_m=Decimal("0.06"), dep_m=Decimal("0.03"), sbc_m=Decimal("0.02"),
                 shares0=1_000_000_000, share_g=Decimal("0.00"), plant_m: Decimal | None = Decimal("1.2"),
                 eq_m=Decimal("1.0")) -> list[dict]:
    rows = []
    for i in range(n):
        y = last - n + 1 + i
        rev = money(Decimal(rev0) * (1 + g) ** i)

        def f(m: Decimal, k: int) -> str:
            return str(money(rev * m * wobble(i, k)))

        rows.append({
            "period_end": f"{y}-12-31", "revenue": str(rev), "operating_cash_flow": f(ocf_m, 1),
            "capital_spending": f(capex_m, 2), "depreciation": f(dep_m, 3),
            "stock_compensation": f(sbc_m, 4),
            "diluted_shares": str(money(Decimal(shares0) * (1 + share_g) ** i)),
            "gross_plant": None if plant_m is None else f(plant_m, 5),
            "equity": f(eq_m, 6),
        })
    return rows


def analyst(reported: str, first: Decimal, growth: Decimal, n: int) -> list[str]:
    """n analyst revenues: the first at first x reported, then growing by growth."""
    return [str(money(Decimal(reported) * first * (1 + growth) ** i)) for i in range(n)]


def return_cases() -> list[dict]:
    def case(name, years, *, shares_now=None, costs=("20", "30", "45"), analyst_revenue=None,
             price_date=PRICE_DATE, expect=None):
        now = shares_now or years[-1]["diluted_shares"]
        return {"name": name, "years": years, "shares_now": now, "costs": list(costs),
                "price_date": price_date.isoformat(), "analyst_revenue": analyst_revenue,
                "expect": expect}

    std = return_years()
    reported = std[-1]["revenue"]
    dip = return_years()
    dip[-1]["operating_cash_flow"] = str(money(Decimal(dip[-1]["revenue"]) * d("0.10")))
    short = return_years()[1:]
    gap = return_years()
    gap[4]["period_end"] = "2020-06-30"
    missing = return_years()
    missing[3]["revenue"] = None
    broke = return_years(ocf_m=d("0.05"))
    fast = return_years(g=d("0.18"), eq_m=d("1.6"))
    return [
        case("steady_no_analyst", std, expect="ok"),
        case("steady_with_analyst", std, analyst_revenue=analyst(reported, d("1.05"), d("0.07"), 5),
             expect="ok"),
        case("growth_limited_by_r", fast,
             analyst_revenue=analyst(fast[-1]["revenue"], d("1.20"), d("0.20"), 5),
             expect="GROWTH_LIMITED_BY_R"),
        case("analyst_scale_omitted", std, analyst_revenue=analyst(reported, d("3"), d("0.05"), 4),
             expect="ANALYST_SCALE"),
        case("plant_not_reported", return_years(plant_m=None), expect="PLANT_NOT_REPORTED"),
        case("gate_spending_omits_earnings", return_years(capex_m=d("0.03")),
             expect="GATE_SPENDING"),
        case("r_below_min_omits_earnings", return_years(eq_m=d("3")), expect="R_BELOW_MIN"),
        case("latest_level_below_base", dip, expect="latest_base"),
        case("share_count_falls", return_years(share_g=d("-0.02")), expect="ok"),
        case("refusal_too_few_years", short, expect="TOO_FEW_YEARS"),
        case("refusal_not_consecutive", gap, expect="NOT_CONSECUTIVE"),
        case("refusal_stale_filing", std, price_date=date(2026, 6, 1), expect="STALE_FILING"),
        case("refusal_missing_figure", missing, expect="MISSING_FIGURE"),
        case("refusal_share_jump", std, shares_now=str(money(Decimal(std[-1]["diluted_shares"]) * d("1.5"))),
             expect="SHARE_JUMP"),
        case("refusal_base_not_positive", broke, expect="BASE_NOT_POSITIVE"),
    ]


def run_return(old: dict) -> list[dict]:
    from option_trader.valuation.price_return import (
        FiscalYear,
        PriceReturn,
        PriceReturnParams,
        price_return,
    )

    params = PriceReturnParams.from_toml(old["price_return_v1.toml"])

    def solved(items):
        return None if items is None else [
            {"cost": t(s.cost), "rate": t(s.rate), "beyond": t(s.beyond), "status": s.status.value}
            for s in items]

    out = []
    for case in return_cases():
        def opt(x):
            return None if x is None else d(x)

        years = [FiscalYear(date.fromisoformat(y["period_end"]), opt(y["revenue"]),
                            opt(y["operating_cash_flow"]), opt(y["capital_spending"]),
                            opt(y["depreciation"]), opt(y["stock_compensation"]),
                            opt(y["diluted_shares"]), opt(y["gross_plant"]), opt(y["equity"]))
                 for y in case["years"]]
        revenue = None if case["analyst_revenue"] is None else [d(v) for v in case["analyst_revenue"]]
        res = price_return(years, d(case["shares_now"]), [d(c) for c in case["costs"]],
                           price_date=date.fromisoformat(case["price_date"]),
                           analyst_revenue=revenue, params=params)
        expect = case.pop("expect")
        if not isinstance(res, PriceReturn):
            assert res.reason.value == expect, (case["name"], res.reason)
            expected = {"state": "refused", "reason": res.reason.value}
        else:
            seen = {f.value for f in res.flags}
            seen |= {f.value for s in res.scenarios for f in s.flags}
            seen |= {s.omitted.value for s in res.scenarios if s.omitted}
            if expect == "latest_base":
                assert any(s.latest_base is not None for s in res.scenarios), case["name"]
            elif expect != "ok":
                assert expect in seen, (case["name"], seen)
            expected = {
                "state": "ok",
                "period_end": res.period_end.isoformat(),
                "shares_now": t(res.shares_now),
                "revenue_record": t(res.revenue_record),
                "earnings_record": t(res.earnings_record),
                "earnings_record_years": res.earnings_record_years,
                "low_path": ts(res.low_path),
                "r": t(res.r),
                "flags": [f.value for f in res.flags],
                "analyst": None if res.analyst is None else
                {"growth": t(res.analyst.growth), "years": res.analyst.years},
                "plant": None if res.plant is None else
                {"usual": t(res.plant.usual), "latest": t(res.plant.latest)},
                "scenarios": [{
                    "key": s.key.value, "base": t(s.base), "growth": ts(s.growth), "r": t(s.r),
                    "omitted": s.omitted.value if s.omitted else None,
                    "flags": [f.value for f in s.flags], "solved": solved(s.solved),
                    "latest_base": t(s.latest_base), "latest_level": solved(s.latest_level),
                } for s in res.scenarios],
            }
        out.append({"name": case["name"], "input": case, "expected": expected})
    return out


# ---- golden_questions.json -------------------------------------------------------------


def series(ends: list[date], values: list[Decimal | None]) -> dict[str, str]:
    return {e.isoformat(): str(v) for e, v in zip(ends, values) if v is not None}


def ends_of(n: int, last: int = 2024, extra_before: int = 0) -> list[date]:
    return [date(last - n + 1 + i - extra_before, 12, 31) for i in range(n + extra_before)]


def figures(n: int, *, rev0=8_000_000_000, g=d("0.05"), gross_m: Decimal | None = d("0.45"),
            op_m=d("0.20"), tax=d("0.20"), ppe_m=d("0.40"), rec_m=d("0.10"), inv_m=d("0.08"),
            pay_m=d("0.07"), ocf_m=d("0.24"), capex_m=d("0.05"), sbc_m=d("0.02"),
            ni_m=d("0.15"), op_fade=d("0")) -> dict[str, dict[str, str]]:
    """The thirteen concept series of five_questions_v1 for n fiscal years."""
    ends = ends_of(n)
    revs = [money(Decimal(rev0) * (1 + g) ** i) for i in range(n)]

    def col(m: Decimal, k: int, fade=d("0")) -> list[Decimal]:
        return [money(r * (m - fade * i) * wobble(i, k)) for i, r in enumerate(revs)]

    op = col(op_m, 1, op_fade)
    pretax = col(op_m * d("0.97"), 2, op_fade)
    out = {
        "revenue": revs, "operating_income": op, "pretax_income": pretax,
        "income_tax": [money(p * tax) for p in pretax], "net_income": col(ni_m, 3),
        "ppe": col(ppe_m, 4), "receivables": col(rec_m, 5), "inventory": col(inv_m, 6),
        "payables": col(pay_m, 7), "ocf": col(ocf_m, 8), "capex": col(capex_m, 9),
        "sbc": col(sbc_m, 10),
    }
    if gross_m is not None:
        out["gross_profit"] = col(gross_m, 11)
    return {k: series(ends, v) for k, v in out.items()}


def balance(n: int, m: Decimal, revs: list[Decimal], k: int) -> list[Decimal]:
    return [money(r * m * wobble(i, k)) for i, r in enumerate(revs)]


def diag_figures(n: int, *, cyclical: bool, losses=(), rev0=6_000_000_000, g=d("0.03")) -> dict:
    """The figures of diagnosis_v1: balances (equity, assets) include the opening year."""
    ends = ends_of(n)
    revs = [money(Decimal(rev0) * (1 + g) ** i * (1 + (d("0.25") if i % 4 == 0 else d("-0.1"))))
            for i in range(n)]
    ni = [money(r * d("0.08") * wobble(i, 2)) for i, r in enumerate(revs)]
    for i in losses:
        ni[i] = -abs(ni[i])
    out = {"net_income": series(ends, ni)}
    if cyclical:
        out["revenue"] = series(ends, revs)
        out["operating_income"] = series(ends, [money(r * d("0.14") * wobble(i, 1) * (1 if i % 3 else d("0.4")))
                                                  for i, r in enumerate(revs)])
        out["inventory"] = series(ends, [money(r * d("0.12") * wobble(i, 3)) for i, r in enumerate(revs)])
    else:
        full = ends_of(n, extra_before=1)
        eq_revs = [money(Decimal(rev0) * (1 + g) ** (i - 1)) for i in range(n + 1)]
        out["equity"] = series(full, [money(r * d("0.9") * wobble(i, 4)) for i, r in enumerate(eq_revs)])
        out["assets"] = series(full, [money(r * d("9") * wobble(i, 5)) for i, r in enumerate(eq_revs)])
    return out


def question_cases() -> list[dict]:
    return [
        {"name": "five_strong_gross_basis", "kind": "five_questions", "figures": figures(10)},
        {"name": "five_operating_basis_no_new_capital", "kind": "five_questions",
         "figures": figures(10, gross_m=None, ppe_m=d("0.10"), rec_m=d("0.05"), inv_m=d("0"),
                            pay_m=d("0.30"), g=d("0.08"))},
        {"name": "five_not_enough_years", "kind": "five_questions", "figures": figures(4)},
        {"name": "diagnosis_cyclical_with_losses", "kind": "diagnosis", "sic": "3312",
         "figures": diag_figures(10, cyclical=True, losses=(1, 5))},
        {"name": "diagnosis_financial", "kind": "diagnosis", "sic": "6022",
         "figures": diag_figures(10, cyclical=False)},
        {"name": "diagnosis_cyclical_too_short", "kind": "diagnosis", "sic": "2911",
         "figures": diag_figures(4, cyclical=True)},
    ]


def run_questions(old: dict) -> list[dict]:
    from option_trader.valuation.diagnosis import DiagnosisParams, diagnose, kind_of
    from option_trader.valuation.five_questions import FiveParams, five_questions

    five = FiveParams.from_toml(old["five_questions_v1.toml"])
    diag = DiagnosisParams.from_toml(old["diagnosis_v1.toml"])

    def parsed(fig: dict) -> dict:
        return {name: {date.fromisoformat(k): d(v) for k, v in s.items()} for name, s in fig.items()}

    out = []
    for case in question_cases():
        fig = parsed(case["figures"])
        if case["kind"] == "five_questions":
            r = five_questions(fig, five)
            expected = {"status": r.status.value, "years": r.years}
            if r.q1 is not None:
                expected |= {
                    "q1": {"mean": t(r.q1.mean), "worst": t(r.q1.worst),
                           "capped_years": r.q1.capped_years},
                    "q2": {"value": t(r.q2.value), "new_capital": t(r.q2.new_capital),
                           "new_nopat": t(r.q2.new_nopat), "no_capital": r.q2.no_capital},
                    "q3": {"basis": r.q3.basis, "first": t(r.q3.first), "last": t(r.q3.last)},
                    "q4": {"value": t(r.q4.value)},
                }
        else:
            kind = kind_of(case["sic"], diag)
            assert kind is not None, case["name"]
            r = diagnose(kind, fig, diag)
            s = r.summary
            expected = {
                "kind": r.kind.value, "status": r.status.value,
                "rows": [{"period_end": row.period_end.isoformat(), "primary": t(row.primary),
                          "secondary": t(row.secondary), "scale": t(row.scale)} for row in r.rows],
                "summary": {"ranked": s.ranked, "latest_rank": s.latest_rank,
                            "primary_median": t(s.primary_median),
                            "secondary_median": t(s.secondary_median),
                            "secondary_latest": t(s.secondary_latest),
                            "revenue_vs_peak": t(s.revenue_vs_peak),
                            "loss_years": s.loss_years, "record_years": s.record_years},
            }
        out.append({"name": case["name"], "input": {k: v for k, v in case.items() if k != "name"},
                    "expected": expected})
    return out


def main() -> None:
    if len(sys.argv) != 2:
        sys.exit("usage: uv run gen.py /path/to/option-trader   (a checkout at 4704569f)")
    checkout = Path(sys.argv[1]).resolve()
    old = load_old(checkout)
    TESTDATA.mkdir(parents=True, exist_ok=True)
    write("golden_fcf.json", run_fcf(old, checkout))
    write("golden_return.json", run_return(old))
    write("golden_questions.json", run_questions(old))


if __name__ == "__main__":
    main()
