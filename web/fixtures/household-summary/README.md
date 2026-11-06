# household_summary fixtures

Invented data only: no real account, position, balance or report. The dollar figures are fixture values.
The JSON files are written by `scripts/gen_viewmodel_schema.py` (`pnpm gen:viewmodels`); CI fails on any
difference. This README is hand-written.

## Hand-worked figures of `ok.json`

| Tile (key) | Hand value | Text |
|---|---|---|
| `net_worth` (净值) | 412,300.00 USD, some stock converted at the broker's ledger rate | `≈$412,300`, caption `USD` |
| `day_change` (上一交易日) | +1,860.00 USD; 1860 / 410,440 = 0.004532 = +0.45 % | `+$1,860`, caption `+0.45%` |
| `month_to_date` (本月至今（不含出入金）) | stored change (end − start − deposits) +11,700.00 USD; 11,700 / 398,600 = 0.02935 = +2.94 % | `+$11,700`, caption `+2.94%` |
| `cash` (现金) | 156,700 / 412,300 = 0.3800 = 38 % | `38%`, caption `+$156,700` |

The subtitle is `两户合计 · 数据日 10-21`; the members are counted, never named.

## Other files

- `stale.json`: generated after `stale_after`; the label reads `数据已过期 · 数据日 10-21`. On a US market
  holiday the label reads `休市 · 数据日 mm-dd` and never 过期.
- `gap.json`: the month has no day-end yet (`本月还没有日结单`), and the allocation rows are missing, so the
  cash share is a gap while its dollar caption stays.
- `bad-*.json`: a version 2, a `"NaN"` value, an unknown field, a figure without `unit` and a share above
  100; each must fail validation.
