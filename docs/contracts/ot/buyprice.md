# ot: the buy price of the option jobs

Design: docs/design/buy-price-source.md#7 (Decisions 4 and 5). Module: `cmd/ot` (`buyprice_wire.go`).

## The valuation read

The listed jobs of the option families (`groupOptions`: `option_eod`, `option_open` and the others that follow
the options group) read the valuation once at each Plan. The step `prices` of `listedJob` calls
`internal/jobs/buyprice.Apply`, which reads `valuation.Reader.Current` once and resolves one price for each entry
with `buyprice.New(valuelines.New())`. The bars, fundamentals and dividend families have no such step and never
read the valuation, so a valuation outage cannot stop them.

## The pages line

The reader is opened by `openJobValuation` with the pages line (`OT_PG_DSN_PAGES`) through `deps.openValuation`,
because `ot_runner` has no grant on the valuation views. No new DSN line, role or `deps` field exists. An open
error is a static text that holds no DSN. Its closer goes to the closer set of the run.

## The three moments

1. **Start.** The build at start never reads the valuation and never fails for it: the entries carry hand prices
   and one Info line `buyprice: start builds with hand prices, each Plan reads the valuation` is logged.
2. **A Plan.** A `Current` error makes the Plan fail with a Retryable error ("read valuation failed"); the run
   plans nothing and is retried. Judging with a hand price when a valuation exists would be silent.
3. **No reader.** With no pages line or no opener the step runs with no zones: each entry keeps its hand price
   with the source `hand`, and one Info line `buyprice: no valuation reader, hand prices only` is logged at
   start, never per run.

## No other source

The price of an entry is the valuation buy-zone top, else the hand price, else none. No other source exists: no
FCF band and no computed price.
