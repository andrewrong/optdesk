# Buy price (`internal/compute/buypriceapi`)

Source: `docs/design/buy-price-source.md`, Decisions 1 and 2, entry 1. The interface has no float, no clock and no I/O.

**Order.** The buy price of a company is, in this order: (1) `valuation`, the buy-zone top of its current valuation review; (2) `hand`, the price typed by hand; (3) `none`, no price, so the company is not judged. No other source exists: no FCF band and no computed fallback.

**Exact text, never rounded.** `Result.Price` is exact decimal text, empty for `none`. The resolver never rounds, adds or compares a price; only a page rounds, for display.

**Pure and never failing.** `Resolver.Resolve` does no I/O, reads no clock and keeps no state. A bad input is a reason, not an error.

**Reasons.** `Result.ZoneReason` is empty when the zone was used, else why the valuation gave no price, in this order: `no_profile`, `ambiguous`, `removed`, `no_review`, `no_tables`, `currency`, `bad_input`. A hand price is still returned as `hand` after any reason.

**Mark words.** `buy_from_valuation` and `buy_from_hand` are declared in `buypriceapi` and written nowhere else. The job writes them on a stored row, the page reads them; they differ from each other and from `no_buy_price`.
