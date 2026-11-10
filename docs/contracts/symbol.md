# symbol contract

## Responsibility

Translate the stock symbol of the IBKR contract of `watch.toml` (the one canonical spelling optdesk keeps) into the spelling a vendor uses, and into the option root. Pure: no I/O, no clock, no state. The root package `internal/symbol` holds `api.go` only; the implementation is `internal/symbol/table` (`symbol.NewTranslatorFunc`), wired by `cmd/ot`.

## Inputs

- A symbol string as any source spells it: `BRK B`, `BRK-B`, `BRK.B`, `brk b`, with surrounding spaces.
- A `symbol.Vendor`. `symbol.AlphaVantage` is the only one defined.

## Outputs

- `ToVendor(symbol, v)`: the spelling `v` uses. For `AlphaVantage` a share class is joined with a dash: `BRK B` gives `BRK-B`, `BF B` gives `BF-B`; `AAPL` and `278A` are unchanged.
- `OSIRoot(symbol)`: the option root and trading class, separators removed: `BRK B` gives `BRKB`, `SPY` gives `SPY`.

## Invariants

- Canonical form: the IBKR contract symbol, share class separated by one space (`BRK B`). Pages and pyside contracts keep it.
- Normalisation, in both methods: trim, upper-case, and read a run of spaces, dots and dashes between two alphanumeric groups as one share-class separator. The canonical form and the vendor forms of one company therefore give the same answer.
- Errors: `ToVendor` returns an error for an empty (or blank) symbol and for an unknown vendor; it returns no partial value with an error.
- `OSIRoot` returns no error: its length is not checked (a root over six characters is the caller's concern) and it is not padded to the OSI width.
- No I/O: no network, no database, no file, no `time.Now`; no state between calls, so a Translator is safe for concurrent use.
- Adding a vendor adds one `Vendor` constant and one case in the implementation.

## Version history

- v1: `Vendor`, `AlphaVantage`, `Translator` (`ToVendor`, `OSIRoot`), `NewTranslatorFunc`.

## Contract tests

- `internal/symbol/contracttest`: `Run(t, newT)`, the suite every implementation runs.
- `internal/symbol/api_test.go`: `TestTranslatorFake` (the suite on a fake) and `TestConsumerThroughInterface` (a consumer using the interface only).
