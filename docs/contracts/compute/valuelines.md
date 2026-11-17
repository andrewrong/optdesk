# Valuation lines (`internal/compute/valuelinesapi`)

Source: `docs/design/valuation-structure.md`, entry 1. Every number is decimal text; the interface has no float, no clock and no I/O.

**No rounding inside.** `Calculator` works in decimal text with no rounding, and every result is a decimal text at full precision. The only rounding of the system is `RoundText(value, places)`, half away from zero, which a page builder calls when it writes a display text. No page, test or job rounds a figure any other way.

**Each line judged alone.** Every flag of `Flags` compares the close price with its own line and with nothing else: the lines are not sorted and not repaired, so `AtTrim` and `AtClear` can both be true. A close equal to a line has reached it (`>=` for trim, clear and core; `<=` for the heavy point; inside the closed range `FairLow` to `FairHigh` for fair), and `InBuy` is `close <= BuyHigh` with no floor. `Zone` is a label picked from the flags by one fixed precedence: `core`, `clear`, `trim`, `fair`, `heavy`, `buy`, else `between`. The flags are the truth; the label is a convenience.

**Yield on an edge.** A bucket of the R table ends at `YieldUpTo`, inclusive, so a yield equal to an edge takes the lower bucket (4.5 is in the 4.0 to 4.5 bucket, 4.0 in the bucket that ends at 4.0). The top bucket has `YieldUpTo` empty and no upper bound.

**Hong Kong or China name.** A name with market `HKCN` gets every price line and no zone: `Zone` is empty and `Flags` is nil whatever the close price, because there is no price source for it. A US or JP name with no close price also gets no zone (`ZoneNote` `no_close`).

**Band positions.** The `Pos` of a `Mark` is a fraction of the axis from `Min` (0) to `Max` (1), as decimal text, returned by `Calculator.Band`. A page builder and the web print it as given and never compute a position.

**One place.** `Calculator` is the only place that computes these numbers (the lines, the band, the change percentage, the proposed R, the step warnings, the checks and the rounding). Every page calls it; no other package multiplies, divides, rounds or positions a figure of a valuation.
