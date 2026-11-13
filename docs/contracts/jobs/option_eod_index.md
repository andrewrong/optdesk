# option_eod_index

## Version history

- v1: `option_eod_index` (trial): `optioneod.NewIndex` builds the job from the embedded `job_index.toml`; the keys SPX, XSP and RUT under the dataset `obs.option_eod_index` (additive).

## What it does

The job stores, once a day in the session (the slot of `option_eod`: XNYS, 90 minutes after the open, run bound 16 minutes, budget 8 minutes, chunks of 50), call-put pairs around the money of the monthly chains of three indexes: SPX (trading class `SPX`), XSP (class `XSP`) and RUT (class `RUT`). It is a job of its own so that an index failure never costs the stock day. The key order is SPX, XSP, RUT; the last key is the one a spent budget cuts, and a Retryable error of one key ends the run (the retry runs every key again; rows append). The rows are those of `mapRow`, the reason rows those of `reasons.go`.

## Keys of `job_index.toml` (every number is trial; none is sourced)

- Window: `min_dte` 0, `max_dte` 60, `max_expiries` 2, `chunk` 50, `budget_minutes` 8.
- Band: `sigma_days` 20, `sigma_fallback` 0.20, `band_k` 1.8.
- `[[index]]` row per class: `trading_class`, `strike_multiple` (SPX 50, XSP 5, RUT 5; the probes show steps of 5 and 1 near the money), `max_contracts` 80 (a cap per class and day), `expiry_kind` (`listed` or `third_friday`), `sigma_index`, `spot_index`, `spot_divisor` (SPX 1, XSP 10 on the SPX close, RUT 1).

## Selection

- Expiries: `listed` keeps every expiry the class lists within `min_dte`..`max_dte`; `third_friday` (XSP) keeps the third Friday of the month, or the Thursday before it when that Friday is not in the chain (19 June 2026 is a holiday, so 18 June). Then the first `max_expiries` are kept. Weeklies, 0DTE and the PM classes are not captured.
- Spot: the snapshot of the index (Last, else Close, else UndPrice). When it gives none and a close reader exists, the last stored close of `spot_index` divided by `spot_divisor`; with no reader the key ends Retryable `no price`.
- Sigma: the annualised (square root of 252) sample standard deviation of the last `sigma_days` log returns of the stored closes of `sigma_index`, read from 70 calendar days before the day. `sigma_fallback` is used with no reader, with fewer than `sigma_days` + 1 closes, with a close that is not a positive number, with a series of no deviation, or when the reader answers an error; a reader error never fails the key.
- Per expiry the half-width of the band is W = spot x `band_k` x sigma x sqrt(T), T the days over 365 with a floor of one day. N = `max_contracts` / (2 x expiries) strikes are taken per expiry among the strikes that are multiples of `strike_multiple`: the multiple nearest to spot always (a tie keeps the lower), the other N - 1 spread evenly over spot - W .. spot + W, each the nearest multiple of the band not yet taken (a tie keeps the lower); a band with fewer strikes takes all of them.
- Every strike is taken with both rights. After the contracts are resolved, a strike of which one right is not listed is dropped, so a stored pair is a listed pair.

## What is stored

- The rows of `mapRow`: bid, ask, sizes, volume, open interest where it comes, the market data type of the quote. A stored pair is a listed pair, not a quoted one: a quote with no bid and no ask is still stored with a status other than 1.
- `iv_decimal` and `delta` are NULL because the gateway sends no model values for these quotes; the reason rows record the missing fields. They are to be computed in a later step from the forward of each expiry by put-call parity, which is why the selection is a set of pairs around the money.
- `underlying_price` is the index value of the spot: the snapshot value (delayed by 15 minutes) or the last stored close (divided for XSP). The row does not say which.
- The expiry is keyed as the broker reports it, never moved to the Friday: the contract carries `20261217`, and the instrument name (`SPX 20261217 C 7585`) and the `osi` identifier (`SPX   261217C...`) follow it.
- The exercise style and the settlement (AM or PM) are not stored; the terms of every option are stored as `unknown`.
