# store/gatewaystate

Package `internal/store/gatewaystate` declares the read and write of the gateway connection state
(`docs/design/gateway-down-since.md`, entry 2, Decisions 1 and 2). It holds declarations only;
`internal/store/gatewaystatepg` implements it over `ops.gateway_state` (write) and `rd.gateway_state_r1` (read).

## Rows

A row holds an alias, a flag and two times, never free text, an id or a secret.

`Reader.States` returns every row ordered by alias; no rows is an empty non-nil slice.

## Observe (Decision 2)

`Writer.Observe(ctx, alias, down, at)` is one observation per gateway per tick, one statement:

- no row: insert `(down, since = at, observed_at = at)`;
- a row with the same `down`: set only `observed_at = at`;
- a row with another `down`: set `down`, `since = at`, `observed_at = at`;
- an `at` earlier than the stored `observed_at` changes nothing, so a late tick never moves `since` back.

The store reads no wall clock; every stored time is the `at` of the caller.
