# Candidate evidence (`internal/compute/optionevidenceapi`)

Source: `docs/design/candidate-evidence.md`, entry 1. Every number is decimal text; the interface has no float, no clock and no I/O; an empty string is "no value".

**Empty figures have a reason.** An empty figure always has a `Why` (`no_iv`, `no_forecast`, `no_rate`, `no_fair_price`, `earnings_before_expiry`, `no_atm`, `no_buy_price`), and the page prints that reason where the number would be. A figure is never blank without a reason.

**The edge is a proxy.** `EdgePerShare` is the bid minus a price made from realised volatility (the mean of the 20, 60 and 120 day windows) in a European model without dividends. It is not a profit expectation, and its sign is often wrong. The page names it "bid vs forecast price", never "edge" or "advantage".

**Labels never gate.** A label (`no_edge_at_bid`, `delta_outside_band`, `ex_div_inside_life`) never changes whether a row passes a gate; it is one line of small text. The same row gives the same figures with every label set and with none.

**Promotion needs a replay.** No label is promoted to a gate before a replay shows that two groups of at least 30 trades differ. None is promoted today.

**No total, rank or order.** The package computes no total, no score, no rank and no order. Any ordering of rows or companies belongs to the page.

**One place.** The `Evaluator` is the only place these figures are computed (break-even, below buy, 1-sigma, edge, skew, labels). No page, job or test recomputes them.
