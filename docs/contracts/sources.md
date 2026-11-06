# Contract: sources

## Responsibility

Fetch series from external data sources and return typed records with provenance. The first client is FRED (`internal/sources/fred`, series DGS10). It opens no database and knows neither the runner nor pages.

## Inputs

- `sources.FREDConfig`: `BaseURL`, `APIKey` (a string passed in by the caller; the package reads no environment), `Clock` (`clock.Clock`).
- `sources.FetchPlan`: series ids and an inclusive date range.
- FRED `series/observations` JSON: `{realtime_start, realtime_end, date, value}` per observation.

## Outputs

- `[]contracts.SeriesPoint` (`obs.series_point r1`), never a hand-written copy. A FRED value `"."` becomes a nil value with status `no_value`; other rows have status `present`. The unit `percent` is set in `mapping.go` only. `realtime_start`, `realtime_end` and `published_at` (the fetch instant on the injected clock) are kept, so the vintage is preserved; `source` is `fred`, `source_revision` is `realtime_start`.
- Errors are `contracts.TypedError` only: HTTP 429, 5xx, transport failures and cancellation are `Retryable`; other non-200 statuses, undecodable bodies and bad configuration are `Permanent`.

## Invariants

- At most one request per second per client (`requestInterval`), measured on the injected clock; the wait is a cancellable timer, never `time.Sleep`.
- The API key never appears in an error text or log line.
- Pure `net/http`; no `internal/` import other than contracts, clock and sources.

## Version history

- r1: FRED client for one series (issue 28).

## Contract tests

`internal/sources/fred`: `TestMapping`, `TestFixtureMapping`, `TestPacing`, `TestErrorKinds`, `TestRedaction`; the wiring `var _ sources.NewFREDFunc = New`.

## Jobs and slots

None. A job (`internal/jobs/fredseries`) calls `Source.Fetch`; this module schedules nothing.

## Forbidden

Opening the database; knowing the runner or pages; reading the environment; a live call in a test.
