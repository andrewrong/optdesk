# Option attribution (`internal/compute/optionattrapi`)

Source: `docs/design/option-attribution.md`, entry 1. Every number is decimal text; the interface has no float, no clock and no I/O.

**Seller amounts.** Every amount of a `Result` and of a `Step` is a seller amount: positive is earned. `Commission` is negative (a cost; the ledger's `Commissions` is positive) and `AssignedIntrinsic` is positive (the intrinsic value moved to the shares, zero unless the lot ended assigned or exercised). The identity: `Entry` + `Exit` + the sum of the steps (blocks and `Unsplit`) + `Commission` + `AssignedIntrinsic` equals the ledger figure `LedgerPnL`; `Difference` is `LedgerPnL` minus that sum rounded half-even to 4 places, and "0" for a well-formed lot.

**Ends are the stored mids.** The two ends of a step are the stored mids of its two quote days, not repriced values, and the convexity part and the time block are exact differences, so the blocks of a step sum to the value change times quantity times multiplier for any pricer. The pricer only decides how the move is shared.

**Gap days.** A gap day (a stored row with no ask above zero, no bid at or above zero, or no underlying price) is never filled and never replaced by a neighbour's price; the next quote day makes one multi-day step (`MultiDay`, in calendar days), and the gap day is listed in `GapDays`.

**Expiry day.** A step ending on the expiry day prices its end at the intrinsic value max(strike - spot, 0) at that row's underlying price, and does not ask the pricer for T = 0, so no expired-error reaches `Unsplit`.

**Not fills.** `Lot.Fills` holds only the ledger rows `open` (a sale) and `close` (a buy-back). The rows `expire`, `assign` and `exercise` are not fills: they are the `End` of the lot, with `EndDay` and `EndClose`.

**One place.** The `Attributor` is the only place the split is computed. A page reads the `Result` and never adds, rounds or splits a figure of its own.

**One book per currency.** `Booker.Combine` returns one `Book` per `Result.Currency`, in order of first appearance; for no results it returns a non-nil empty slice. `Book.Lots` counts the lots added.

**Lots counted apart.** A result with `NoQuotes` is counted in `NoQuoteLots`, and one with an empty `Difference` (an open lot whose ledger mark day differs from its last quote day) in `UnreconciledLots`. Both add nothing to any sum field of the `Book`; their ledger P&L is added to `NoQuoteLedgerPnL` and `UnreconciledLedgerPnL`, and to no other field.

**Rent minus cost.** `RentMinusCost` is `Time` plus `MoveCost` (seller amounts: `MoveCost` is negative).

**Total and the book identity.** `Total` is `Entry` + `Exit` + `Direction` + `MoveCost` + `IVRemark` + `Time` + `Unsplit` + `Commission`, the option side; it excludes `AssignedIntrinsic`, which is its own field. For the lots added, `LedgerPnL` = `Total` + `AssignedIntrinsic` + `Difference`. An empty `Exit` counts as zero.

**Exact sums, one place.** Sums are exact decimal addition and nothing is rounded; the page rounds for display. The `Booker` is the only place results are added; a page never adds figures of its own.

**Lot card.** The card of one lot uses `Combine` of one result, so one rule makes the lot figures and the book figures.
