# Option attribution (`internal/compute/optionattrapi`)

Source: `docs/design/option-attribution.md`, entry 1. Every number is decimal text; the interface has no float, no clock and no I/O.

**Seller amounts.** Every amount of a `Result` and of a `Step` is a seller amount: positive is earned. `Commission` is negative (a cost; the ledger's `Commissions` is positive) and `AssignedIntrinsic` is positive (the intrinsic value moved to the shares, zero unless the lot ended assigned or exercised). The identity: `Entry` + `Exit` + the sum of the steps (blocks and `Unsplit`) + `Commission` + `AssignedIntrinsic` equals the ledger figure `LedgerPnL`; `Difference` is `LedgerPnL` minus that sum rounded half-even to 4 places, and "0" for a well-formed lot.

**Ends are the stored mids.** The two ends of a step are the stored mids of its two quote days, not repriced values, and the convexity part and the time block are exact differences, so the blocks of a step sum to the value change times quantity times multiplier for any pricer. The pricer only decides how the move is shared.

**Gap days.** A gap day (a stored row with no ask above zero, no bid at or above zero, or no underlying price) is never filled and never replaced by a neighbour's price; the next quote day makes one multi-day step (`MultiDay`, in calendar days), and the gap day is listed in `GapDays`.

**Expiry day.** A step ending on the expiry day prices its end at the intrinsic value max(strike - spot, 0) at that row's underlying price, and does not ask the pricer for T = 0, so no expired-error reaches `Unsplit`.

**Not fills.** `Lot.Fills` holds only the ledger rows `open` (a sale) and `close` (a buy-back). The rows `expire`, `assign` and `exercise` are not fills: they are the `End` of the lot, with `EndDay` and `EndClose`.

**One place.** The `Attributor` is the only place the split is computed. A page reads the `Result` and never adds, rounds or splits a figure of its own.
