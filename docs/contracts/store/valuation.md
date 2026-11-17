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

## PostgreSQL implementation

`internal/store/valuationpg` implements `Reader` and `Writer` over migration `0052_valuation.sql`.

- Schema `valuation` (owner `ot_owner`, `USAGE` to `ot_w_settings`) holds the five tables; `ot_w_settings` has `INSERT` and `SELECT` on them and nobody has `UPDATE` or `DELETE`. The writer reads the tables only to learn whether a `review_id` is closed.
- The pages read the views `rd.valuation_company_profile_r1`, `rd.valuation_review_r1`, `rd.valuation_r_version_r1`, `rd.valuation_r_table_r1` and `rd.valuation_margin_table_r1` (`SELECT` to `ot_ro_pages` and `ot_w_settings`). `read.go` names no table; the newest-row and current-valuation rules are applied in Go over the rows the views return.
- `valuationpg.Open(dsn)` is the `OpenFunc` (one pool, both interfaces; the role of the dsn decides what works). `valuationpg.OpenReader(ctx, dsn)` opens the reader alone with the pages dsn.
- `review` carries the CHECKs of Decision 12 (`status = 'draft'` or `type`, `version_date`, `v` and `p` present), of `status` (`draft`, `effective`, `void`) and of `source` (`owner`, `program`); two unique partial indexes allow one effective and one void row per `review_id`. `effective_at` is set by a `BEFORE INSERT` trigger to `now()` on an effective row and is null on every other row.
- `PutReview` reads the status of the `review_id` and inserts in one transaction; a unique violation of either partial index (a concurrent writer) is also `ErrClosed`. A `Put*` of a table version inserts all its rows in one transaction, so they share `set_at`.
- The Down part raises when any of the five tables holds a row and drops the views, tables, trigger function and schema only when all five are empty.
