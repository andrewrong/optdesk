# store/valuation

Package `internal/store/valuation` declares the store of the valuation structure (`docs/design/valuation-structure.md`, Interface entry 2 and Decision 8). It holds declarations only; `internal/store/valuationpg` implements it. Decimal columns are decimal text and dates are `contracts.Day`.

## Tables

| Table | Key | Newest row wins by |
| --- | --- | --- |
| `valuation.company_profile` | `company_key` | the newest row of the `company_key` |
| `valuation.review` | `review_id` (rows of one id are drafts, one effective row, one void row) | the newest row of the `review_id`; the current valuation is chosen by the rule below |
| `valuation.r_version` | one row per version | `effective_date`, then `set_at` |
| `valuation.r_table` | `effective_date`, `set_at` and the bucket (`yield_up_to`) | the newest version (all rows of one `effective_date` and `set_at`) |
| `valuation.margin_table` | `effective_date`, `set_at` and the bucket (`p_from`) | the newest version, as for `r_table` |

## Insert-only

No row is updated or deleted, and the interface has no method that does. A change is another row, and the newest row per key wins as the table above says. A draft is edited by another draft of the same `review_id`. An effective row closes its `review_id`: `PutReview` returns `ErrClosed` for any further draft or effective row of it. A mistake is undone by one void row of the same `review_id` with a `VoidReason`; `PutReview` returns `ErrNotEffective` for a void row whose `review_id` has no effective row. The rows of a table version are inserted in one transaction.

## Current valuation

The current valuation of a company (`Entry.Current`) is the review row with status `effective`, with no void row of its `review_id`, whose type is not `backfill`, with the latest `effective_at` (set by the database at insert, ties by `id`). `version_date` is shown and never decides which review is current. It is nil when no row qualifies. A draft and a void row are read through `Reviews` and are never current.

## Empty answers

`Current`, `Reviews` and `RVersions` return non-nil slices when there is nothing to return; the newest R version and the two table versions of `State` are nil when none exists.
