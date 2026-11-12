# broker contract

## Responsibility

The Go client of the pyside IBKR service. `broker.Broker` (`internal/broker/api.go`) is the port the jobs call; `grpcbroker.Dial` (`internal/broker/grpcbroker`) implements `broker.DialFunc` over the `ibkr.v1` services on a Unix socket and answers with plain Go types. Nothing outside `grpcbroker` imports the generated code.

## Interface

- `ListChains(ctx, underlying)` returns the chains; strikes are the union over all expirations, untouched.
- `ResolveContract(ctx, c)` returns `Resolved`; `Listed == false` with a nil error is the proto's NOT_LISTED. `Resolved.ErrorCode` and `ErrorText` carry the response's `error_code` and `error_text` (0 and empty when none). Several or differing results (the service's FAILED_PRECONDITION) are `Permanent`.
- `Snapshot(ctx, kind, cs)` returns one `Quote` per contract in the order asked, matched by `ConID`; a contract with no streamed answer gets `StatusDisconnected`. It does not split the call: the caller chunks. `Kind` is `KindOptionEod`, `KindUnderlyingIV`, `KindSnapshot` or `KindOptionOpen` (the last maps to `QUOTE_KIND_OPTION_OPEN`, value 4). An unknown `Kind`, `SecType` or `Right`, and `QUOTE_STATUS_UNSPECIFIED`, are `Permanent` errors; an unknown kind fails before any call.
- `Close()` releases the connection.
- `SecType` is `STK`, `IND` or `OPT`; `Right` is `P` or `C`; empty is unspecified.

## Reference client

`broker.ReferenceClient` is a separate port (`Broker` is unchanged): `Dividends(ctx, cs)` returns one `DividendProjection` per contract in the order asked, and `Close()`. `broker.NewReferenceFunc` is its dial type; `grpcbroker.DialReference(ctx, socketPath, o)` implements it over `ibkr.v1.ReferenceService` (`Retryable` when the socket is not reachable). `DividendProjection` has the contract, `Status` (`DividendOK`, `DividendNoValue`, `DividendTimeout`, `DividendError`), `Past12m`, `Next12m`, `NextAmount` (`*float64`, nil when unset), `NextExDate` (`*string`, `YYYYMMDD`, nil when unset), `ErrorCode` and `ReceivedAt`. Answers are matched to contracts by `ConID`, or by symbol, exchange and currency when the `ConID` is 0. A status the server did not set, a number that does not parse, an unknown `SecType` and a contract with no answer are `Permanent` errors; `SnapshotTimeout` bounds one call. No stored answers, no retries.

## History client

`broker.HistoryClient` is a separate port (`Broker` is unchanged on purpose: other types implement it): `Bars(ctx, q)` returns the daily bars of one contract, oldest first, and `Close()`. `broker.NewHistoryFunc` is its dial type; `grpcbroker.DialHistory(ctx, socketPath, o)` implements it over `MarketDataService.Bars` (`Retryable` when the socket is not reachable). `BarQuery` has the `Contract` (with `ConID`), `Kind` (`BarTrades`, `BarAdjustedLast`, `BarMidpoint`; the zero value is unset), `DurationDays` (1 to 730) and `End` (`YYYY-MM-DD`, or empty for now). `Bar` has `Day` (`YYYY-MM-DD`), `Open`, `High`, `Low`, `Close` and `Volume` (`*string`, nil when unset), all decimal text. An unset kind and an `End` that is neither `YYYY-MM-DD` nor empty fail before any call; a day that is not `YYYY-MM-DD` and a price or volume that is not a decimal are `Permanent`; `RESOURCE_EXHAUSTED` (the service's pacing limit) is `Retryable`. `Options.HistoryTimeout` (default 330 s) bounds one call. Contract tests: `TestBarsContract`, `TestBarsBadDayIsError`, `TestBarsBadPriceIsError`, `TestBarsPacedIsRetryable`, `TestBarsUnsetKindIsError`, `TestBarsBadEndDayIsError`, `TestHistoryTimeoutDefault`.

## Account reader

`broker.AccountReader` (`internal/broker/api.go`) is a separate port: `Account(ctx)`, `Executions(ctx)` and `Close()`. `Account` has the alias, `Generation`, `Values []AccountValue`, `Positions []Position` and the flags `ValuesCollected` and `PositionsCollected` (false: the gateway does not collect that part). `Executions` has the alias, `Generation`, `Collected`, `Pending` (fills left out for want of a commission report) and `Executions []Execution`. Money and quantities are decimal text; `Execution.Time` is UTC; no order id and no account id. `grpcbroker.AccountClient` implements it. Contract test: `TestAccountReaderContract`.

## Contract lister

`broker.ContractLister` (`internal/broker/api.go`) is a separate port from `Broker`; the client of `grpcbroker.Dial` implements both (`Dial` is unchanged). `ListContracts(ctx, underlying, expiry, right, tradingClass)` returns the real contracts of one underlying (a stock or index with a `ConID`) at one expiry, each with its `ConID`, `Strike` and `Multiplier`; `right` is `C`, `P` or empty for both; there is no strike. A contract of another trading class is dropped (every class is kept when `tradingClass` is empty). The `Strike` text is the one `ResolveContract` returns for the same contract. The multiplier is not filtered here: the caller compares it with the chain's. `broker.ErrNotServed` is a constant of the unexported string type `notServedError` (its `Error()` method is in `internal/broker/errors.go`): UNIMPLEMENTED from an older service is a `Permanent` error that matches it with `errors.Is`; any other gRPC error follows the error kinds below. `CallTimeout` bounds one call. Contract tests: `TestContractListerFromTheCaller`, `TestListContractsDropsOtherClasses`, `TestListContractsKeepsMultiplier`, `TestListContractsGRPCErrorIsReturned`, `TestListContractsUnimplementedIsErrNotServed`, `TestListContractsStrikeTextEqualsResolveContract`.

## Error kinds

Every error is a `contracts.TypedError`. `UNAVAILABLE` and `DEADLINE_EXCEEDED` are `Retryable`; `INVALID_ARGUMENT`, `FAILED_PRECONDITION` and every other code are `Permanent`. The text of an error holds the code and never the socket path. A cancelled context gives an error that wraps `context.Canceled` and is `Permanent`.

## Defaults

`broker.Options` fields at zero take: `CallTimeout` 25 s (`ListChains`, `ResolveContract`), `SnapshotTimeout` 60 s (`Snapshot`), `ResolveInFlight` 5 (at most this many `ResolveContract` calls of one client run at once; the others wait on their context). The in-flight wait is not part of `CallTimeout`.

## Invariants

- The socket path is the caller's: `Dial(ctx, socketPath, o)` is given `$OT_HOME/run/ibkr.sock` and never builds a path. `Dial` returns a `Retryable` error when the socket is not reachable.
- Unix socket only; no TCP.
- The client keeps no state between calls: no stored answers, no retries inside a call.
- No type or call exists for orders.
- The account client is a second, separate surface and is not part of `broker.Broker`: `DialAccount(ctx, socketPath, o)` returns an `*AccountClient` (`Retryable` when the socket is not reachable, defaults of `Options` as `Dial`). It has one call, `Account(ctx)`, which returns the plain types `Account` (alias, generation, `Values []AccountValue`, `Positions []Position`), `AccountValue` (tag, value, currency) and `Position` (a `broker.Contract`, quantity and average cost as the received decimal text, never floats), and `Close()`. `CallTimeout` bounds the call.
- Alias rule: the answer carries the alias of the gateway's entry and the client never holds an account id. A response with an empty alias or a negative generation is a `Permanent` error.
- `grpcbroker` is imported only by `ot`; a job needs a port in `internal/broker/api.go` first before it can use account data.

## Contract tests

`internal/broker/grpcbroker`: `TestSnapshotOrder`, `TestQuoteFields`, `TestKinds`, `TestQuoteKindMapsAllFourKinds`, `TestResolveNotListed`, `TestListChains`, `TestErrorKinds`, `TestResolveInFlight`, `TestTimeouts`, `TestDialRefuses`, `TestAccountValues`, `TestAccountPositions`, `TestAccountErrorKinds`, `TestAccountCallTimeout`, `TestDialAccountRefuses`, `TestAccountEmptyAliasRefused`, `TestDividendsContract`, `TestDividendsUnsetStatusIsError`.
