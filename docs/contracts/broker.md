# broker contract

## Responsibility

The Go client of the pyside IBKR service. `broker.Broker` (`internal/broker/api.go`) is the port the jobs call; `grpcbroker.Dial` (`internal/broker/grpcbroker`) implements `broker.DialFunc` over the `ibkr.v1` services on a Unix socket and answers with plain Go types. Nothing outside `grpcbroker` imports the generated code.

## Interface

- `ListChains(ctx, underlying)` returns the chains; strikes are the union over all expirations, untouched.
- `ResolveContract(ctx, c)` returns `Resolved`; `Listed == false` with a nil error is the proto's NOT_LISTED. Several or differing results (the service's FAILED_PRECONDITION) are `Permanent`.
- `Snapshot(ctx, kind, cs)` returns one `Quote` per contract in the order asked, matched by `ConID`; a contract with no streamed answer gets `StatusDisconnected`. It does not split the call: the caller chunks. An unknown `Kind`, `SecType` or `Right`, and `QUOTE_STATUS_UNSPECIFIED`, are `Permanent` errors; an unknown kind fails before any call.
- `Close()` releases the connection.
- `SecType` is `STK`, `IND` or `OPT`; `Right` is `P` or `C`; empty is unspecified.

## Error kinds

Every error is a `contracts.TypedError`. `UNAVAILABLE` and `DEADLINE_EXCEEDED` are `Retryable`; `INVALID_ARGUMENT`, `FAILED_PRECONDITION` and every other code are `Permanent`. The text of an error holds the code and never the socket path. A cancelled context gives an error that wraps `context.Canceled` and is `Permanent`.

## Defaults

`broker.Options` fields at zero take: `CallTimeout` 25 s (`ListChains`, `ResolveContract`), `SnapshotTimeout` 60 s (`Snapshot`), `ResolveInFlight` 5 (at most this many `ResolveContract` calls of one client run at once; the others wait on their context). The in-flight wait is not part of `CallTimeout`.

## Invariants

- The socket path is the caller's: `Dial(ctx, socketPath, o)` is given `$OT_HOME/run/ibkr.sock` and never builds a path. `Dial` returns a `Retryable` error when the socket is not reachable.
- Unix socket only; no TCP.
- The client keeps no state between calls: no stored answers, no retries inside a call.
- No type or call exists for orders, positions or accounts.

## Contract tests

`internal/broker/grpcbroker`: `TestSnapshotOrder`, `TestQuoteFields`, `TestKinds`, `TestResolveNotListed`, `TestListChains`, `TestErrorKinds`, `TestResolveInFlight`, `TestTimeouts`, `TestDialRefuses`.
