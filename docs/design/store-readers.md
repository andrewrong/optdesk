# Store readers: the current contract terms of an option and the latest day of option_eod

Author: the optdesk dispatcher (revision 2, after the adversarial review)
Owner approved: (pending)
Date: 2026-11-07
Source: `docs/design/option-page.md`, `## Left out`, lines 2 and 3 ("a reader of the contract terms by `instrument_id` and `underlying_id`" and "a reader that lists the underlyings with rows, and one for the latest `session_day`"), which say each needs a design of its own in module `store`. This design narrows the first (`## Left out`, 1).

Facts on main at 05fe334:

- The instrument types of `store-master.md` are on main: pull request 193 (#189) is merged, so `store.InstrumentReader`, `InstrumentSpec` and the stub package `internal/store/instrument` exist and no entry here waits for #189. Migration 0008, the role `ot_w_master` and `instrument/db.go` are not on main: they are issue #195 (the implementation entry of `store-master.md`). Migrations end at 0007.
- `OptionEodReader.OptionEod(ctx, underlyingID, from, to)` and `QuoteSnapshotReader` take a day window and nothing else (`internal/store/api.go:36-51`); the builder of `option-page.md` reads a fixed 7-day window ending on the day it is given, so it cannot find the latest day with rows and cannot list the underlyings.
- No record holds strike, expiry or right. `store-master.md` Left out 1 does not build `master.option_terms`; the v1 page schema has nullable `expiry`, `right` and `strike` cells waiting for a terms reader (`option-page.md`, `## Interface`). The symbol of an instrument comes from `store.InstrumentReader.Instruments` (`store-master.md`), not from a terms reader.
- `rd.option_eod_r1` is the view the pages and `ot_runner` read; today it passes every row through, and the follow-up named in `store-week2.md:156` and `:352` replaces it with the winner view under the same name. A reader of `rd.option_eod_r1` is therefore right before and after that change. `obs.option_eod` is a compressed hypertable on `session_day` with an index on `(instrument_id, session_day DESC)` (`migrations/0006_option_eod.sql`).
- Every grant to `ot_ro_pages` so far is `USAGE` on schema `rd` plus `SELECT` on an `rd` view (`migrations/0006_option_eod.sql`, `0007_quote_snapshot.sql`, `0002_series.sql`). `store-master.md` gives `USAGE` on schema `master` to `ot_w_master` only.
- The store sub-packages each hold a private copy of the pgx seam (`internal/store/snapshot/db.go`: `database`, `InTx`, `Select`); `store-master.md` gives `internal/store/instrument` the same (`db.go`, in its implementation entry).
- `docs/design/modules.md`: `internal/store/**`, `migrations/**` and `docs/contracts/store.md` are store; `internal/store/api.go` is its entry, so every declaration below is an interface change.

Goal: two small readers so the option page can show the strike, expiry and right of each contract, and can pick an underlying and its latest stored day, without a page-side guess. The terms get the owner's v3 table, read through an `rd` view; the latest-day reader is one grouped query over the existing view. There is no writer of the terms here. Nothing calls either reader yet: the builder change and the `ot` wiring are follow-ups (`## Left out`).

## Modules

No change to `docs/design/modules.md`, `arch.toml` or `.go-arch-lint.yml`: all entries are store only. The latest-day reader lives in the existing package `internal/store/snapshot`; the terms reader lives in `internal/store/instrument`, the package of #189 and #195. No new package directory, no new dependency, no new role and no new line name.

| issue | module |
|---|---|
| store-readers-interface | store |
| store-eod-latest | store |
| store-option-terms | store |

## Interface

Owner: store, `internal/store/api.go` (declarations only). Added after the instrument store types of `store-master.md`, and next to the option end-of-day types for the catalog rows:

| name | declaration |
|---|---|
| `store.OptionEodLatest` | `struct{ UnderlyingID int64; SessionDay contracts.Day }`: an underlying that has at least one row and the greatest `session_day` it has |
| `store.OptionEodCatalog` | `interface{ OptionEodLatest(ctx context.Context, since contracts.Day) ([]OptionEodLatest, error) }`: one entry per `underlying_id` with a row of `session_day` on or after `since`, ordered by `UnderlyingID`; `since` is a `session_day` date (`YYYY-MM-DD`); an empty `since` means no lower bound; no row gives an empty list, not an error |
| `store.OptionEodCatalogStore` | the catalog and `Close() error` |
| `store.OpenOptionEodCatalogFunc` | `func(ctx context.Context, dsn string) (OptionEodCatalogStore, error)`, implemented by `snapshot.OpenOptionEodCatalog` |
| `store.OptionTerms` | `struct{ TermsVersionID, InstrumentID int64; Right string; Strike, StrikeCurrency, Multiplier string; ExpirationDate contracts.Day; ExerciseStyle, SettlementType string; ValidFrom time.Time }`: the current version of the terms of one option; `Right` is `C` or `P`; `Strike` and `Multiplier` are decimal text, as `PriceIncrement` is in `store-master.md` |
| `store.OptionTermsReader` | `interface{ OptionTerms(ctx context.Context, instrumentIDs []int64) ([]OptionTerms, error) }`: the version with `valid_to IS NULL` of each id, ordered by `InstrumentID`; an id without a current version is absent from the answer; an empty list gives an empty answer; more than 500 ids is a `Permanent` error |
| `store.OptionTermsStore` | the reader and `Close() error` |
| `store.OpenOptionTermsFunc` | `func(ctx context.Context, dsn string) (OptionTermsStore, error)`, implemented by `instrument.OpenOptionTerms` |

Why a new interface and not a method on `OptionEodReader`: adding a method to an existing interface breaks every fake of it (the page builder's tests among them), and the reader and the catalog are used by different callers.

Why the terms are looked up by instrument id only: the builder holds the instrument ids of at most 500 rows it has read by underlying. The symbol comes from `InstrumentReader.Instruments` for the same ids; the list of underlyings comes from `OptionEodCatalog`. There is no writer: the producer is the chain refresh of the option jobs design (pull request 188), which declares the writer together with its caller (`## Left out`, 2).

`since` and its default: `since` is a `session_day` date, never an instant. The reader has no default window; the caller passes `since`. The default window is a setting with a code default (`macro-page-wiring.md` Left out, the owner-settings store with a code default), and the setting and its default are the caller's follow-up (`## Left out`, 1).

Table (migration 0009, owner: store): the owner's schema v3 `master.option_terms`, in the column names of v3, one row per version of the terms of one instrument.

```sql
master.option_terms(terms_version_id bigint generated always as identity PRIMARY KEY, -- addition: v3 says only "bigint PK"; identity is what master.instrument uses
  instrument_id bigint NOT NULL REFERENCES master.instrument(instrument_id),
  right char(1) NOT NULL CHECK (right IN ('C','P')), -- addition: v3 gives no codes for the column
  strike numeric NOT NULL, strike_currency char(3) NOT NULL, multiplier numeric NOT NULL,
  expiration_date date NOT NULL, exercise_style text NOT NULL, settlement_type text NOT NULL,
  valid_from timestamptz NOT NULL, valid_to timestamptz)
CREATE UNIQUE INDEX option_terms_current ON master.option_terms (instrument_id) WHERE valid_to IS NULL -- addition: the one-row-per-instrument invariant of this design, not a v3 object
```

The repository holds no copy of the v3 text for this table (`store-master.md` Left out 1 names only the columns and copies section 3.1 of the instrument tables), so the columns are the advisor's list of 2026-11-07; `underlying_id` is not in it and is not added (the underlying of an option is the `underlying_id` of its observation rows). `terms_version_id` of `obs.option_eod` and `obs.quote_snapshot` stays NULL; a foreign key from them to this table is a follow-up.

View: `rd.option_terms_r1` selects every column named, `WHERE valid_to IS NULL`; owner `ot_owner`.

Grants: `ot_ro_pages` `USAGE` on schema `rd` and `SELECT` on `rd.option_terms_r1`, and nothing on schema `master`; the view runs with its owner's rights, so the page role never touches `master`. No grant to `ot_w_master` here: the writer grant lands with the writer. A migration grants to a role that exists (`migrations/0006_option_eod.sql`).

Rules. Reader: the statement is `SELECT ... FROM rd.option_terms_r1 WHERE instrument_id = ANY($1) ORDER BY instrument_id`; decimals come back as the text the database prints for the numeric, with no trailing-zero rule beyond that (tests compare parsed values, `100` and `100.00` are equal); a zero id is `Permanent`; a database failure is `Retryable`. Catalog: `since` that is not `YYYY-MM-DD` and not empty is `Permanent`; the query is `SELECT underlying_id, max(session_day) FROM rd.option_eod_r1 [WHERE session_day >= $1] GROUP BY underlying_id ORDER BY underlying_id`; a database failure is `Retryable`.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is an offline grep or a named test, run in a repository copy without network, service or database. Every test uses `t.Parallel()` and `t.TempDir()`; none sleeps; a test that needs PostgreSQL carries the build tag `pg` and runs only in GitHub CI.

Which entries run when: the interface entry can start now, since #189 is merged. After it merges, `store-eod-latest` and `store-option-terms` are attached; their files are disjoint. `docs/contracts/store.md` is edited in this order: pull request 192 (#173) first, then #195 (migration 0008, role `ot_w_master`), then the terms entry, which takes migration number 0009.

### store-readers-interface (interface)

The declarations of the two readers
  Goal: `internal/store/api.go` declares the types of `## Interface` and its test uses them from both sides.
  Files: internal/store/api.go, internal/store/api_test.go
  Design: docs/design/store-readers.md#interface
  Acceptance:
  1. `git diff internal/store/api.go` shows additions only and they are exactly the eight rows of `## Interface`; `grep -cE '^func ' internal/store/api.go` prints 0.
  2. `api_test.go` tests the contract from both sides: a fake `OptionEodCatalogStore` that returns two underlyings ordered by id, and a caller that reads them; a fake `OptionTermsStore` that returns terms ordered by `InstrumentID`, and a caller that reads them back by ids with one unknown id absent, an empty list and 501 ids (the fake refuses them as `Permanent`).
  3. `git diff --name-only` lists the two files above and no others; `git diff -- go.mod go.sum` is empty; `grep -c 'PutOptionTerms\|OptionTermsWriter' internal/store/api.go` prints 0.
  4. Every test added calls `t.Parallel()` and none sleeps; `go run ./scripts/arch --check` prints `arch: ok`.
  Checked by CI: ci-go (build, vet, test, golangci-lint, go-arch-lint); the design verdict on the pull request (an interface change).
  Pull request body: `Contracts: unchanged`.
  Depends on: none (merged work only; #189 is merged). Order: before `store-eod-latest` and `store-option-terms`.
  Size: S

### store-eod-latest (store)

The catalog of underlyings and their latest day
  Goal: `snapshot.OpenOptionEodCatalog` returns a `store.OptionEodCatalogStore` over `rd.option_eod_r1`, the winner view once it replaces the pass-through.
  Files: internal/store/snapshot/catalog.go, internal/store/snapshot/catalog_test.go, internal/store/snapshot/catalog_pg_test.go
  Design: docs/design/store-readers.md#interface
  Acceptance:
  1. `catalog.go` holds `var _ store.OpenOptionEodCatalogFunc = OpenOptionEodCatalog`; it reuses `openDB` of `db.go` of its package and imports no other package of `internal/` than `store` and `contracts` (`go list -deps ./internal/store/snapshot | grep optdesk/internal`); `git diff -- migrations internal/store/api.go go.mod go.sum` is empty.
  2. `TestCatalog` (fake `database` seam as the other `snapshot` tests use): two underlyings in, `OptionEodLatest` out with the latest day of each, ordered by `UnderlyingID`; no row gives an empty non-nil list.
  3. `TestCatalogSince` (table): an empty `since` sends the statement without a `WHERE` and no argument; a day sends it with one argument equal to that day; a text that is not `YYYY-MM-DD` is `Permanent` and the seam sees nothing.
  4. `TestCatalogErrors`: a seam failure is `Retryable`; the statement text names only `rd.option_eod_r1` (no `obs.`, offline grep of the test's expectation and of `catalog.go`).
  5. Each new file is under 600 lines; no file outside the three listed is in the diff.
  Checked by CI: ci-go (go test, golangci-lint, go-arch-lint); the `pg` job (GitHub CI only): `catalog_pg_test.go` carries the build tag `pg`; with rows of two underlyings on three days, the answer holds each underlying's greatest day; `since` after an underlying's last day drops it; a connection as `ot_ro_pages` succeeds; the test seeds at most one capture per (session_day, instrument), so its expected answer does not change when `rd.option_eod_r1` becomes the winner view.
  Depends on: store-readers-interface merged (merged work only).
  Size: S

### store-option-terms (store)

Migration 0009, the `rd` view and the terms reader
  Goal: `instrument.OpenOptionTerms` returns a `store.OptionTermsStore` over `rd.option_terms_r1`, and `ot_ro_pages` can read it without any right on schema `master`.
  Files: migrations/0009_master_option_terms.sql, internal/store/instrument/terms.go, internal/store/instrument/terms_test.go, internal/store/instrument/terms_pg_test.go, internal/store/migrate/migrate_test.go, docs/contracts/store.md
  Design: docs/design/store-readers.md#interface
  Acceptance:
  1. `terms.go` holds `var _ store.OpenOptionTermsFunc = OpenOptionTerms`, uses the `database` seam of `db.go` of its package (`grep -c 'jackc/pgx' internal/store/instrument/terms.go` prints 0), imports no other package of `internal/` than `store` and `contracts`, and its statement text names `rd.option_terms_r1` and not `master.` (`grep -c 'master\.' internal/store/instrument/terms.go` prints 0).
  2. `0009_master_option_terms.sql` has an Up and a Down part; it creates the table of `## Interface` with the v3 column names, the CHECK on `right`, the foreign key and the NOT NULL columns, the unique partial index `option_terms_current` and the view `rd.option_terms_r1` with `WHERE valid_to IS NULL`; the Down part drops the view, the index and the table; `migrate_test.go` (no database) lists the file after 0008 and checks both parts.
  3. Grants, offline grep of `0009_master_option_terms.sql`: `grep -c 'GRANT USAGE ON SCHEMA rd TO ot_ro_pages'` prints 1; `grep -c 'GRANT SELECT ON rd.option_terms_r1 TO ot_ro_pages'` prints 1; `grep -i 'ot_ro_pages' ... | grep -c 'master'` prints 0; `grep -c 'ot_w_master'` prints 0; no `GRANT` line names a role other than `ot_ro_pages`.
  4. `TestOptionTerms`: ids in, terms out ordered by `InstrumentID`, an id without a current version absent, `Strike` and `Multiplier` equal by parsed value to the text the seam returns, an empty list gives an empty answer and no seam call, 501 ids is `Permanent` and the seam sees nothing, a zero id is `Permanent`, a seam failure is `Retryable`.
  5. `docs/contracts/store.md` lists the table, the view, `OptionTermsStore` and `OptionEodCatalog` (the catalog is documented here because the file is edited once, in the order of `## Issues`); each new file is under 600 lines; `git diff -- go.mod go.sum internal/store/api.go .go-arch-lint.yml` is empty.
  Checked by CI: ci-go (go test, golangci-lint, go-arch-lint); the `pg` job (GitHub CI only): `terms_pg_test.go` carries the build tag `pg` and seeds rows by SQL as the owner; the migration applies on a database holding 0001 to 0008; a row with an unknown `instrument_id` fails; a second current row of one instrument fails on the partial index while a closed one (`valid_to` set) is accepted; the reader returns the current version only; `ot_ro_pages` can select the view and cannot select `master.option_terms`, cannot insert.
  Depends on: #195
  Order: after `store-readers-interface`; `docs/contracts/store.md` after pull request 192 (#173) and #195, then this entry, migration 0009.
  Size: M

Width: two entries after the interface, in two different packages with disjoint files. The catalog needs no migration and no role; the terms need both; putting them in one entry would hold the one-query reader behind the migration and #195.

## Left out

1. The builder change that reads the two readers (fills `symbol` from `InstrumentReader.Instruments`, and `expiry`, `right`, `strike` from the terms; picks the latest day; lists underlyings) and the `ot` wiring (openers `snapshot.OpenOptionEodCatalog` and `instrument.OpenOptionTerms` in `deps`, an `--underlying` default, the setting of the default `since` window with its code default): follow-ups of `option-page.md`; they start after the entries here and the page builder have merged. `option-page.md` Left out asked for a terms reader by both `instrument_id` and `underlying_id`; this design narrows it to instrument ids only (2026-11-07), because the builder holds the ids and the symbol is the instrument reader's.
2. The writer of the terms (the former `PutOptionTerms` and `OptionTermsWriter`): removed. The producer is the chain refresh of `docs/design/option-jobs.md` (pull request 188), which declares the writer, its grant to `ot_w_master` and its caller together. Until then the table holds no row and the page shows `Gap` for the terms of every instrument; the pg test seeds rows by SQL.
3. A reader of terms as of a past day, of closed versions, and one keyed by `underlying_id` (all options of an underlying); a foreign key from the observation tables' `terms_version_id` to the table; corporate-action adjustments: not needed by the builder.
4. A catalog of datasets other than `option_eod` (there is one reader, for `option_eod`) and of `quote_snapshot`: the generalization is added when a page needs it. An index for the grouped query and any change of the compression settings: `since` bounds the scan by chunk; if the unbounded scan is slow on real data, an index or a summary table is a follow-up.
5. Textual risk: `internal/store/api.go` and `docs/contracts/store.md` are edited by pull request 192, by #195 and here; the entries are attached after the interface entry has merged and each rebases onto the last.
6. Owner-only cases: none of the paths is owner-only; migration 0009 takes effect only when the owner runs `ot migrate`. No network call, no new dependency and no new role.

## Adversarial

The full answer is in the author's scratchpad (`adversarial-store-readers.md`, 2026-11-07), with the advisor's decision of the same day.

1. Gap against the goal: adopted. The writer is removed and the terms half is stated as empty until the chain refresh of the jobs design fills it (Left out 2); the symbol comes from `InstrumentReader.Instruments`, the lookup is by instrument ids only and the narrowing of `option-page.md` is recorded (Left out 1).
2. Biggest unnamed risk (grants and the `rd` convention): adopted. The page reads through `rd.option_terms_r1`, `ot_ro_pages` gets `USAGE` on `rd` only, and terms acceptance 3 greps the grants of the migration offline. The second risk, two sources of truth (the `osi` text and a terms row): not adopted here; one producer (the chain refresh) writes both from the same values, and keeping them equal is the jobs design's to test.
3. Simpler way, (a) catalog only and (b) terms inside `ResolveInstrument`: (a) adopted in part, the writer and its grant are cut, the table and reader stay because the owner's v3 shape is cheap to land now; (b) not adopted, it would change `InstrumentSpec`, merged in #189, and a single write path is the jobs design's decision.
4. Lock-in (a shape that differs from the owner's table): adopted. The table takes the v3 shape and names now, with `terms_version_id`, `valid_from` and `valid_to`, so no replacement migration is needed; `since` is a date and the catalog is per underlying only (Left out 4).
5. False claim (#189 open): adopted. Pull request 193 is merged; the wait on #189 is gone, and the terms entry waits for #195 only.
6. Likeliest slip (the M entry with unmerged dependencies and shared files): adopted. The terms entry names `Depends on: #195` as its one start condition, the order of `docs/contracts/store.md` and migration 0009 is stated, the pg test seeds by SQL, and the catalog entry no longer differs from the terms entry in reading through an `rd` view.
