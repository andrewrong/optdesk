# Candidate evidence (`internal/compute/optionevidenceapi`)

Source: `docs/design/candidate-evidence.md`, entry 1. Every number is decimal text; the interface has no float, no clock and no I/O; an empty string is "no value".

**Empty figures have a reason.** An empty figure always has a `Why` (`no_iv`, `no_forecast`, `no_rate`, `no_fair_price`, `earnings_before_expiry`, `no_atm`, `no_buy_price`), and the page prints that reason where the number would be. A figure is never blank without a reason.

**The edge is a proxy.** `EdgePerShare` is the bid minus a price made from realised volatility (the mean of the 20, 60 and 120 day windows) in a European model without dividends. It is not a profit expectation, and its sign is often wrong. The page names it "bid vs forecast price", never "edge" or "advantage".

**Labels never gate.** A label (`no_edge_at_bid`, `delta_outside_band`, `ex_div_inside_life`) never changes whether a row passes a gate; it is one line of small text. The same row gives the same figures with every label set and with none.

**Promotion needs a replay.** No label is promoted to a gate before a replay shows that two groups of at least 30 trades differ. None is promoted today.

**No total, rank or order.** The package computes no total, no score, no rank and no order. Any ordering of rows or companies belongs to the page.

**One place.** The `Evaluator` is the only place these figures are computed (break-even, below buy, 1-sigma, edge, skew, labels). No page, job or test recomputes them.

**The ATM rule.** `ATMPicker.ATM` chooses the at-the-money row of one expiry. It looks at puts only (`Right` "P"), whose `Source` equals the Source of the candidate's own stored quote (`option_eod` for candidates, and `option_open` the other value), whose `IV` is a number above 0. Among them it takes the strike nearest to spot within 10% of spot (the difference at most `atmBand` times spot; 10% is a trial value), the lower strike on a tie, and returns that row's `IV` and `Strike` texts unchanged. With no such row, or a spot that is not a number above 0, it returns two empty strings, and the `Evaluator` answers `no_atm` for the skew. It does not sort or change the chain and reads no clock.

**Units.** `BelowBuyPct` and `OneSigmaPct` are percents; `BreakEven`, `BelowBuy`, `OneSigma`, `Fair` and the edges are prices; `Skew`, `ForecastErr`, `Row.IV`, `Row.ATMIV`, `Row.Forecast`, `Row.IVP` and `Row.VolEdge` are fractions (0.294 is 29.4%). The six new fields `SkewPct`, `ATMIVPct`, `ForecastPct`, `ForecastErrPct`, `IVPPct` and `VolEdgePct` are percents: each is its fraction (`Skew`, `Row.ATMIV`, `Row.Forecast`, `Row.ForecastErr`, `Row.IVP`, `Row.VolEdge`) times 100 at four decimals, empty when the input is empty or not a number. `Row.IVP` and `Row.VolEdge` are the two new inputs, name-level fractions, empty when none.
