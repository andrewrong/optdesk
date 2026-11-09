# gaphistory contract

## Responsibility

The rows of the view `rd.job_gap_r1` as read by store, behind one read interface with its own row type. It opens no database, does no arithmetic and writes nothing. Design: `docs/design/option-notices.md#Interface`.

## Outputs

- `gaphistory.Gap{JobID, DatasetID string; Day contracts.Day; Reason string; FirstSeenAt, ReportedAt, ClearedAt time.Time}`: the columns of `store.Gap`; a zero time is NULL.
- `gaphistory.Reader`: `GapsSince(ctx, since)` and `Close()`.
- `gaphistory.OpenFunc`: `func(ctx, dsn) (Reader, error)`.

## Invariants

- `GapsSince` returns the rows with `first_seen_at >= since` or `cleared_at >= since`, ascending by `FirstSeenAt`, and never nil (an empty slice for none).
- `internal/gaphistory` imports only `context`, `time` and `contracts`; it does not import `store`.

## Version history

- v1: interface declared.
