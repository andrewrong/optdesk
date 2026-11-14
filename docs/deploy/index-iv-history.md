# Loading the history of the three index series (VIX, VXN, RVX)

The market page (`market_page`) shows the 3-year percentile of VIX, VXN and RVX only when the stored
FRED history of the three series spans 3 years before the day. This runbook loads that history once.
The design is `docs/design/market-page-producer.md` (Decisions 9 and 10). The run is the owner's: no
agent and no test runs it. Nothing below holds a key, an account id or a connection string; they stay in
the `.env` file of the host, which `ot` reads.

## The command

```
ot series-load --series VIXCLS --series VXNCLS --series RVXCLS --from DAY --to DAY
```

`DAY` is `YYYY-MM-DD`. The three ids are the Cboe series VIXCLS (VIX), VXNCLS (VXN) and RVXCLS (RVX).
`--to` is the newest day to load, normally the last session day.

## The rule for --from

- `--from` is at least 3 years and 10 days before the first day a percentile is wanted. The first stored
  point must be on or before that day minus 3 years, and that date may fall on a weekend.
- The loader refuses a range over 5 years. One run therefore covers 3 years and a margin; for a longer
  span run the command more than once with ranges of at most 5 years.
- Example: percentiles wanted from 2026-11-10 need `--from` on or before 2023-10-31.

## After the load

Run `ot run-once` for the day. The command only enqueues console requests and returns; the scheduler
runs them, so the console must be on (`--wait` polls until the requests end). The `compute_figures` job
then stores the readings when the scheduler has run its request; without it they appear with the next
scheduled run, which computes the as-of day only.

## The check

For each of the three ids (`market.vix_pctile_3y`, `market.vxn_pctile_3y`, `market.rvx_pctile_3y`) the
stored reading of the day is one of:

- `ok`: the percentile is stored and the page shows it.
- `short_history`: `--from` was too late. The page then shows `历史不满 3 年，不给分位`, which is true.
  Load again with an earlier `--from` and run `ot run-once` again (it enqueues; the scheduler runs it).

## Not part of this

The IBKR live index value is not loaded here. It waits for a probe of a CBOE market-data subscription
and is a separate owner decision.
