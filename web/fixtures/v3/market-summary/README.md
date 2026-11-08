# market-summary v3 fixtures

Invented data only: a Monday (2026-10-26), invented run and build ids, invented index names and
sources. No real reading, position or account. The files are written by hand; this README too.
Times are UTC with the suffix Z. Validated against `contracts/schemas/page.market_summary.v3.json`
by `TestFixturesV3Validate` in `internal/contracts/fixtures_v3_test.go`.

## Hand-worked figures of `ok.json`

- Row SPX: IV 20.0 − RV20 18.0 = +2.0 vol points, text `+2.0`, tone `up`.
- Row NDX: IV 24.5 − RV20 22.0 = +2.5 vol points, text `+2.5`, tone `up`.
- Row RUT: IV 22.0 − RV20 23.5 = −1.5 vol points, text `−1.5` (U+2212), tone `down`.
- Say: `隐含波动率高于已实现，差 +2.0 个波动点。` (the figure of row SPX).
- Volatility indices: VIX 18.20, VXN 21.00, RVX 23.50 index points, as published; not part of the table.
- Chart: 30 trading days, 09-14 to 10-23, every point with `close`, `ma200`, `band_low` and `band_high`
  (an invented index level, not a vol reading). For day i = 0..29: `ma200` = 98.0 + 0.1·i; the band is
  `ma200` ∓ 2.0; `close` = 100.0 + 0.4·i + w, with w = 0, +0.6, 0, −0.6 for i mod 4 = 0, 1, 2, 3.
  First point 100.0 / 98.0 / 96.0 / 100.0; last point (i = 29) 112.2 / 100.9 / 98.9 / 102.9.
  The three `series_names` are `收盘`, `200日均线`, `区间`. `ok.json` holds no Gap object.
- Every figure holds `kind` `value`; the stale label of `ok.json` has `is_stale` false.

## Other files

- `stale.json`: same figures, generated 2026-10-28 (UTC); the stale label has `is_stale` true.
- `gap.json`: the same chart and rows SPX, NDX, RUT, plus row N225 (RV20 19.0) whose `iv` and `iv_minus_rv20`
  are a Gap (`kind` `gap`, `no_data`); the RVX level is a Gap (`not_built`); `run_status` is `partial`.
- `bad-version.json`: `schema_version` 2; must fail at `/schema_version` with keyword `const`.
