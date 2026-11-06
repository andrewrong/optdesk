# market_summary v2 fixtures

Invented data only: a Monday (2026-10-26), invented run and build ids, invented index names and
sources. No real reading, position or account. The files are written by hand; this README too.
Validated against `contracts/schemas/page.market_summary.v2.json` by `internal/contracts/fixtures_test.go`.

## Hand-worked figures of `ok.json`

- Row 甲: IV 20.0 − RV20 18.0 = +2.0 vol points, text `+2.0`, tone `up`.
- Row 乙: IV 15.5 − RV20 16.0 = −0.5 vol points, text `−0.5` (U+2212), tone `down`.
- Say: `隐含波动率高于已实现，差 +2.0 个波动点。` (the figure of row 甲).
- Volatility indices: VIX 18.20, VXN 21.00, RVX 23.50 index points, as published; not part of the table.
- Chart: five days, the last (`10-24`) has no data, so all five levels are `null`.

## Other files

- `stale.json`: same figures, generated 2026-10-28 after `stale_after`.
- `gap.json`: row 乙 has `iv` and `iv_minus_rv20` as a common Gap (`no_data`), the RVX level is a Gap
  (`not_built`); `run_status` is `partial`.
- `bad-version.json`: `schema_version` 1; must fail at `/schema_version` with keyword `const`.
