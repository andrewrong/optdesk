# held_companies_summary fixtures

Invented data only: tickers and figures are made up, no real position, share or price. The files are
written by `scripts/gen_viewmodel_schema.py` (`pnpm gen:viewmodels`); CI fails on any difference. This
README is hand-written.

## Hand-worked figures of `ok.json`

Data day 2026-10-23 (US and JP). Nine held names, ordered by `share` (weight) descending; only the names with a results date inside 21 days are rows (rows 1 and 2), the other seven are counted in `say` only.

| Row | Name | Share | Results date | Day change | Closes | 5 日 |
|---|---|---|---|---|---|---|
| 1 | AMD | 0.30 | 10-28 (5 days) | `+2.1%` (0.021) | 100 101 102 103 104 105 | 105 / 100 − 1 = `+5.0%` |
| 2 | MSFT | 0.25 | 11-12 (20 days) | `−0.4% · 10-22` (−0.004 on the day before) | 400 402 401 399 398 396 | 396 / 400 − 1 = `−1.0%` |
| 3 | NVDA | 0.20 | 11-22 (30 days): Gap "21 日内无财报日" | Gap | five closes: Gap | Gap |
| 4 | AAPL | 0.08 | none: Gap | `0.0%` (0.0004, flat) | 100 … 100 100.5 | 100.5 / 100 − 1 = `+0.5%` |
| 5–9 | GOOG, AMZN, META, 6758 (JPY), 7203 (JPY) | 0.06 … 0.02 | none: Gap | GOOG `−1.2%`, the rest Gap | none: Gap | Gap |

Two names have a results date inside 21 days (AMD, MSFT), so the card sentence (`say`) is
`持有 9 家 · 2 家 21 日内财报 · 近 5 日消息：暂无数据源。`

The no-news-source Gap `no_news_source` ("近 5 日消息：暂无数据源") is listed once in `card.gaps`: there is no news source yet.

## Other files

- `stale.json`: the same card, generated after `stale_after`; the label says `数据已过期：数据日 10-23`.
- `gap.json`: the stored reading is not `ok`, so `rows` is one table-level Gap (`missing_input`), never rows of zeros.
- `bad-version.json` (`schema_version: 2`), `bad-nan.json` (a `value` written as the string `"NaN"`),
  `bad-extra-field.json` (an unknown field in `card`), `bad-missing-unit.json` (no `unit` in the first
  row's day change): each must fail validation.
