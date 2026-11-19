# buyprice

- `buyprice.Apply(ctx, rd valuation.Reader, r buypriceapi.Resolver, entries []jobs.WatchEntry) ([]jobs.WatchEntry, error)` (implemented by `internal/jobs/buyprice`; design `docs/design/buy-price-source.md`, Decisions 4 and 5): returns a copy of the entries with `BuyPrice` and `BuySource` set and no other field changed; the input slice is not changed.
- One read per Plan: `rd.Current` is called once per `Apply` call, never once per entry. The state is copied field by field into `[]buypriceapi.Zone` (a profile's `Symbol`, `Currency`, `Pool`, `RMarket` as `Market`, and its current review's `V`, `P`, `VersionDate` as `ReviewDay`, `Reviewed` true when a current review exists) and `buypriceapi.Tables` (`Present` when the newest R version and margin table both exist).
- The key of a company is `WatchEntry.Issuer`, not the contract symbol. The currency is `WatchEntry.Contract.Currency`. The hand price given to the resolver is the entry's `BuyPrice` on input.
- Result per entry: `valuation` gives the resolver's price and source `valuation`; `hand` gives the hand price and source `hand`; `none` gives an empty price and an empty source. The price is the resolver's text, unrounded.
- A `Current` error is returned wrapped and no entry is returned.
- A nil reader makes no read and resolves with no zones, so a hand price gives `hand` and no price gives `none`. A nil resolver makes no read and returns the entries with the price unchanged and `BuySource` `hand` where a price exists, empty otherwise.
- The package imports no other jobs package.
- Tests: `internal/jobs/buyprice/apply_test.go` `TestApplyValuationWins`, `TestApplyHandAndNone`, `TestApplyReadErrorReturned`, `TestApplyNilReaderKeepsHand`, `TestApplyOneRead`, `TestApplyDoesNotChangeInput`, `TestApplyKeyIsIssuer`, `TestApplyCopiesState`.
