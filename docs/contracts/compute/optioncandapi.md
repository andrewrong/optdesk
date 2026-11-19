# Put rule in a currency (`internal/compute/optioncandapi`)

Source: `docs/design/candidates-jp.md`, Decisions 13 to 15. The package declares `CurrencyInput` (the `compute.CandidateInput` and a `Currency` text), `CurrencyRule` with `EvaluateIn` and the constructor type `NewCurrencyRuleFunc`; `optioncand.NewCurrency` implements it.

**An empty currency is USD.** `EvaluateIn` with an empty or `USD` currency returns the rows of `Evaluate`, row by row. Any currency other than `JPY` follows the USD rule. The stored row currency is set by the job from the run; `compute.Candidate` has no currency field.

**JPY puts.** Buy-price, spread, bid size and open interest gates are as for USD. The yield is `bid / strike x 365 / dte` to 6 places; the currency does not enter.
- Funding: with an account in JPY, strike times multiplier is compared with its yen funds (`fit` / `no_fit`). With accounts but none in JPY the gate passes and the row carries the mark `no_jpy_funds`; the `Fits` of a USD alias is `unknown` with reason `no_fx`, never `no_fit` for the currency. With no account the gate is as today (mark `no_account`).
- Cash rate: no yen short rate is stored, so the gate passes and the row carries the mark `no_jpy_rate`.

**A pass by mark is not a pass by check.** `no_jpy_funds` and `no_jpy_rate` say the gate was passed without the check being done. The stored status stays `ok`; no status is added. Readers (the tier) must treat these marks as look-at, never as green.
