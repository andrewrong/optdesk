# serieshistory

## SeriesHistoryLoader

`serieshistory.SeriesHistoryLoader` has `Load(ctx, sources.FetchPlan) ([]serieshistory.SeriesLoaded, error)`; `serieshistory.SeriesLoaded` is `{SeriesID string; Fetched int}`; the constructor type is `serieshistory.NewSeriesHistoryFunc` (`serieshistory.New(sources.Source, store.SeriesWriter)`). The design is `docs/design/compute-fred-series.md#series-history-loader`; `ot series-load` only wires it.

- `Load` refuses an empty series list, `From` after `To` and a range over 5 years as `Permanent`, before any fetch.
- One `Fetch` per series over `From`..`To`; every series is fetched before the first write, so a source error writes nothing.
- The points of a series are written once, with one `InsertSeriesPoints` call (idempotent), and `Load` returns the count fetched per series.
- A source or writer error is passed on by class only: a `TypedError` keeps its kind, an untyped fetch error is `Permanent`, an untyped write error `Retryable`; the text is not passed on. Only `contracts.TypedError` values are returned.
- No clock and no environment are read.

## Version history

- v1: `SeriesHistoryLoader`, with `internal/jobs/serieshistory`.
