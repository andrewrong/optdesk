# Short-put shortlist (`internal/compute/optionshortlist`)

Source: `docs/design/candidates-page-v2.md`, Decisions 3 and 6 to 11. The package implements `optionshortlistapi.Shortlister`. Exact rationals (`math/big`), decimal text, no float, no clock, no I/O, no rule parameter (no `ruleparam` import).

**Bounds** (constants of the package, not settings): strike 5 to 15 percent below spot (both ends kept), absolute delta 0.15 to 0.30 (both ends kept), at most 3 rows per underlying (`Company`).

**Distance.** (Spot - Strike) / Spot, from the `Strike` and `Spot` texts of the row, never from a break-even. `Placed.OTM` is the percent and `Placed.AbsDelta` the absolute delta, both at most four decimals, trailing zeros trimmed.

**Delta rule.** The absolute value of `Delta` is judged. An empty or unreadable `Delta` is `no_delta`.

**Drop order.** A row counts once, under the first code that applies: `bad_row` (mark `unreadable_gates`, or a `Bid` that is non-empty and unreadable, or an `Annualized` that is empty or unreadable), `no_bid` (bid empty or not above 0, or `QuoteReason` one of `no_quote`, `no_bid`, `no_ask`, `small_bid_size`), `no_open_interest` (any mark `no_open_interest`, or `QuoteReason` `low_open_interest`), `no_spot` (`Spot` or `Strike` empty, unreadable or not above 0), `otm_low`, `otm_high`, `no_delta`, `delta_out`, `over_cap`. `Dropped` lists the codes in this order, a zero count omitted; `Scanned` equals `Shown` plus the sum of the counts.

**Quote.** A row whose `QuoteReason` is `wide_spread` and nothing else drops it is kept with `Liquidity` `wide_spread`.

**Order.** Rows by the stored `Annualized`, highest first, ties by ticker, strike, key; groups by the stored `Annualized` of their best row (`Group.Best`) with the same ties. The cap of 3 keeps the best rows of each company.

**Result.** `StaleChain` counts scanned rows with events reason `stale_chain`; `DTEMin` and `DTEMax` are the smallest and largest readable `DTE` of the scanned rows; `Bounds` echoes the constants. `Select` returns no error.
