# Store week 2: tables and writers for obs.option_eod and obs.quote_snapshot

Author: the optdesk dispatcher (draft for the design review)
Source: the project advisor's goal for week 2 (migrations and writers in the store module for the two datasets, interface
first, so that the week 3-4 jobs have tables and the Go side can be tested on synthetic fixtures before any broker code
exists) and the advisor's schema v3 for the two tables, restated below in this file's own words. No broker code is touched.

Read at the base (origin/main caacdb4, which includes #135; every item read or grepped in the worktree of this branch):

- `docs/design/modules.md`: row `store` owns `internal/store/** migrations/** docs/contracts/store.md`; row `contracts`
  owns `contracts/**`, `internal/contracts/**` and `docs/contracts/datasets/**`; row `repo` owns `arch.toml` and
  `.go-arch-lint.yml` (and `scripts/**`). A new package under `internal/store/` is a new component of both lint files.
- `arch.toml` lists `store/ops`, `store/series`, `store/sessionday` and `store/setup` as components;
  `scripts/arch` writes both lint files from `modules.md` and the tree (`go run ./scripts/arch`), and
  `go run ./scripts/arch --check` runs in `checks.yml`; `.go-arch-lint.yml` rejects a component whose directory is absent
  and a package that belongs to no component. So a new package and the regenerated lint files must land in one pull request.
- `internal/store/api.go`: `SeriesWriter`, `SeriesReader`, `SeriesStore`, `OpenSeriesFunc`, the line names
  `DSNEnvRunner`, `DSNEnvSeriesFred`, `DSNEnvPages`, `DSNEnvLoader`, `AdminDSNEnv`, `Window`, `Run`, `OpsLedger`; it imports
  `context`, `time` and `contracts`.
- `internal/contracts/api.go` aliases generated records (`SessionDay`, `SeriesPoint`, `SeriesUnit`); they come from
  `contracts/datasets/*.r1.json` through `contracts/generate.sh` (go-jsonschema into `internal/contracts/gen/records.go`,
  `contracts/gendoc` into `docs/contracts/datasets/*.md`); `contracts/fixtures/MANIFEST.synthetic-v1.json` lists the fixtures
  with sha256 and `scripts/hygiene` checks it. Nothing for `option_eod` or `quote_snapshot` exists in the tree.
- `internal/store/series/writer.go` is the model of a writer: one transaction, no call for empty input, `ON CONFLICT DO
  NOTHING`, a `database` seam faked in unit tests, `contracts.TypedError` kinds, pgx only under `internal/store/`.
- `migrations/` holds 0001 to 0005 (`0005_session_day_loader.sql` from #135). `0002_series.sql` is the pattern for a table,
  a view in schema `rd`, owners and grants. `0003_ops_ledger.sql` gives `ot_w_series_fred` `REFERENCES, SELECT` on
  `ops.capture_run`.
- `ops.capture_run` exists (0003) with `run_id text` as primary key and no `capture_id`, `kind` or `partial` column; its
  `status` is text with only `running` and `done` used by the runner. Schema v3 has `capture_id uuid` as its key, with
  `kind`, `generation` and `attempt` columns; the table on main is a different shape, so a foreign key to `run_id` would
  have to be redone.
- `internal/store/setup/setup.go`: `roles` has five entries (`ot_owner`, `ot_runner`, `ot_w_series_fred`, `ot_ro_pages`,
  `ot_loader`), one `GRANT CONNECT` statement names the four login roles, `ensureRole` sends a SCRAM-SHA-256 verifier
  (`scramVerifier`), and `extensions` is `timescaledb`. `TestRolesTable` expects five roles.
- `internal/store/setup/migrate_pg_test.go` and `loader_pg_test.go` migrate all of `migrations/` into scratch databases made
  by `newScratch` (which creates the roles of `roles` under `pg_advisory_lock(54)` and does not create the Timescale
  extension). CI runs `timescale/timescaledb:2.30.2-pg17` (`.github/workflows/ci-go.yml`).
- `docs/contracts/store.md` has the eight contract headings and versions v1 to v4; v4 is the loader role.
- `ops.dataset_window`: see the next section.

## What exists for ops.dataset_window

The table is already materialised, so this design adds no ops table, no ops migration and no ops row.

- `migrations/0003_ops_ledger.sql` creates `ops.dataset_window (dataset_id, session_day, open_at, close_at)` with primary
  key `(dataset_id, session_day)`; `ot_runner` has INSERT, UPDATE and DELETE on it.
- `internal/store/ops/ledger.go` implements `PutWindow` and `Window` over it.
- `internal/runner/reconcile/reconcile.go`, `reconcileDay`, calls `PutWindow` once per distinct `DatasetID` among the keys
  the registered jobs plan for the day, with the window from `windowFor` (calendar open plus `OpenOffset`, close plus
  `CloseOffset`, none for a non-session day). A dataset gets its window as soon as a job plans a key with its `DatasetID`
  (`obs.option_eod`, `obs.quote_snapshot`); nothing is seeded.
- Limit found, not fixed here: `runner.NewWithRuleFunc` takes one `WindowRule` for all jobs and `cmd/ot/runonce.go` builds it
  from the single registered job. Two jobs with different windows cannot coexist yet (`## Left out`).

## Brief

Background. Week 1 left one dataset (`obs.series_point`) with its writer role, view and ledger. The next two datasets are
the frozen end-of-day option snapshot and the timed quote snapshot; both are not obtainable again once the day has gone,
so their tables are append-only and permanent. The advisor's schema v3 fixes their columns, keys, hypertable and
compression settings, the sentinel rule and one writer role per job. This design cuts the store side of that schema.

Proposal.

- Two new records in `contracts` (r1, new, so no version bump), the store declarations, and one new package
  `internal/store/snapshot` holding both writers and readers, with migrations 0006 (`obs.option_eod`) and 0007
  (`obs.quote_snapshot`).
- Two new writer roles, `ot_w_option_eod` and `ot_w_quote_snapshot`, one `.env` line each, created by `Setup` with a SCRAM
  verifier like `ot_loader`; the password is only in `.env`.
- Scope cut, stated: r1 stores every column of v3 and keeps the keys and identity fields, but (a) `capture_id` is a uuid and part of
  the v3 keys now, but has no foreign key yet: `ops.capture_run` on main has `run_id text` as its key, v3 has `capture_id
  uuid` plus `kind`, `generation` and `attempt`, and a key on `run_id` now would be redone; (b) the views `rd.option_eod_r1` and `rd.quote_snapshot_r1`
  pass every row through instead of picking the best row per key, because the pick needs the capture ledger's status and
  finish time; (c) `underlying_id`, `instrument_id` and `terms_version_id` are plain `bigint` columns because the master
  tables do not exist. A follow-up design (`capture-ledger`, `## Left out`) adds the ledger columns, the three constraints
  and the winner views; the view names and the table keys do not change.

Questions for the reviewer and the owner.

1. This is a second-opinion case: two new roles and two new credentials (generated passwords written to `$OT_HOME/.env`).
   The owner's word is needed before `store-week2-roles` merges.
2. Theta: the CHECK of theta rejects NaN and infinities only, as for delta; the writer applies the same -1 to NULL screen
   to theta as to delta, so a broker "no value" never lands as -1.0 (answered in `## Second opinion`).
3. Storage: only the chunk interval and the compression delay are provisional; `segmentby` and `orderby` are fixed by
   schema v3 and cannot change once a table has data, so the pg tests compress a chunk of synthetic rows as the proof.
4. The index IV figure is neither on `obs.quote_snapshot` nor on `obs.option_eod`; it has its own follow-up design
   (`## Left out`).

## Second opinion

The project advisor answered this design (model Claude Fable 5.1, 2026-11-06). The answer binds nothing and the merge
waits for the owner's word. This is a second-opinion case by one clause: two new roles and two new credentials.

1. Foreign keys and views. The reason to defer them is not that a key is impossible: `ops.capture_run` on main has
   `run_id text` as its key while schema v3 has `capture_id uuid` with `kind`, `generation` and `attempt`, so a key to
   `run_id` now would be redone. The tables keep `capture_id uuid` and the v3 primary keys now. `capture-ledger` is a
   `Depends on` of the first job that writes real rows, so no live row references nothing.
2. Theta. The CHECK stays at NaN and infinities only; the writer applies the same -1 to NULL screen to theta as to delta, so
   a broker "no value" never lands as -1.0. The delta acceptance items cover theta.
3. Storage. Only the chunk interval and the compression delay are provisional; `segmentby` and `orderby` are fixed by
   schema v3. The pg compression test on synthetic rows stays the proof.
4. Index IV. A follow-up design, not `obs.quote_snapshot` and not `obs.option_eod`; it defines `market.index_iv`, keyed
   like the quote table.

Advisor: agrees, with the conditions above, all adopted.

Revision note: the cut was reordered after the first drive of the interface entry stopped (it needs `contracts.OptionEod` and `contracts.QuoteSnapshot`, which the schemas entry made, while every other entry waits for the interface entry). The schemas entry is folded into the interface entry, and the sentinel screen and the lint files moved to `store-option-eod`, with no change of substance: no table, role, grant or acceptance item changed, only which entry carries it. Crew allows exactly one "(interface)" entry per design and holds every other entry until that entry lands, so chaining two interface issues (schemas first, then the store `api.go`) was not possible; that is why the cut folds them into one entry.

## Interface

Owners: `contracts` for the records, `store` for the declarations and the package `internal/store/snapshot`.

The tables, in words (identifiers are the column names; every price, size, volume and Greek column is a nullable `numeric`):

`obs.option_eod` (one option contract on one session day, frozen after the close):

- key `(session_day, capture_id, instrument_id)`; `session_day` is the market-zone date; `capture_id` is a uuid;
  `instrument_id` is a bigint and is what tells apart contracts that share underlying, expiry, strike and right (an index
  option and its weekly, an adjusted contract).
- identity and provenance: `underlying_id`, `generation`, `market_data_type` (smallint), `received_at` (timestamptz),
  `event_ts`, `status` (smallint 1 to 5: ok, no quote, not listed, stale, error), `error_code`, `currency` (three
  upper-case letters), `snapshot_kind`, `terms_version_id`.
- quote: `bid`, `ask`, `bid_size`, `ask_size`, `last`, `volume`, `open_interest`, `oi_asof_date`.
- underlying: `underlying_price`, `underlying_ccy` (three letters, not null), `underlying_event_ts`.
- vol and Greeks: `iv_bid_decimal`, `iv_mid_decimal`, `iv_ask_decimal`, `iv_decimal` (0.25 is 25 percent), `delta`, `gamma`,
  `theta_per_calendar_day`, `vega_per_vol_point`.
- storage: a hypertable partitioned on `session_day` with one-month chunks; compression after 7 days, segmented by
  `underlying_id`, ordered by `session_day DESC, capture_id, instrument_id`; an index on `(instrument_id, session_day DESC)`;
  no retention. Only the chunk interval and the compression delay are provisional; the segment and order columns are fixed.

`obs.quote_snapshot` (a planned time slot of quotes):

- key `(session_day, capture_id, slot, instrument_id)`; `slot` is text.
- `received_at`, `generation`, `market_data_type`, `event_ts`, `status`, `error_code`, `currency`, `bid`, `ask`, `bid_size`,
  `ask_size`, `last`, `last_size`, `volume`. No IV column.
- storage: a hypertable on `session_day` with 7-day chunks; compression after 7 days, segmented by `instrument_id`, ordered
  by `session_day DESC, slot, capture_id`; no retention. Only the 7-day chunk interval and the 7-day compression delay are provisional; segmented by `instrument_id` and ordered by `session_day DESC, slot, capture_id` are fixed by schema v3.

CHECK rules (the contracts generate the numeric ones, the migration repeats them):

- price, size, volume, open interest, underlying price: NULL, or at least 0 and neither NaN nor an infinity.
- `iv_*`, `gamma`, `vega_per_vol_point`: NULL, or not NaN, not an infinity and not -1.
- `delta` and `theta_per_calendar_day`: NULL, or not NaN and not an infinity (-1 is a legal value of both, so the CHECK
  cannot reject it). One rule for both: the writer turns a -1 into NULL before the insert, because the broker marks "no value"
  with -1; a real value of exactly -1.0 is stored as NULL, a loss accepted on advice (`## Second opinion`).
- `status` between 1 and 5.

Sentinel rule of the writer: the broker boundary turns a "no value" marker into NULL and sets the row status; the store
writer is a second guard and does it per column. Every sentinel found (-1 in any numeric column, delta and theta
included, so a broker "no value" never lands as -1.0; NaN or an infinity anywhere) is stored as NULL, the status of the row is left as the caller set it, and the other columns
of the row are kept. A row is never rejected or marked as a whole because one column was a sentinel.

Retry rule: the key contains `capture_id`, and the insert is `ON CONFLICT DO NOTHING`. A retry of the same capture
(same `capture_id`, same instrument) adds no row and keeps the first one; a better observation of the same contract is a new
capture with a new `capture_id`, which is a different key and is never blocked by the first. Which row a reader prefers
among several captures is the winner view of the follow-up design; until then the views return all rows.

Roles and grants (migrations; the roles are made by `Setup`):

- `ot_w_option_eod`: `USAGE` on schemas `obs` and `ops`, `INSERT` on `obs.option_eod`, `SELECT, REFERENCES` on
  `ops.capture_run`; nothing else (no UPDATE, DELETE or TRUNCATE).
- `ot_w_quote_snapshot`: the same on `obs.quote_snapshot`.
- `ot_runner`: `SELECT` on both tables. `ot_ro_pages` and `ot_runner`: `SELECT` on `rd.option_eod_r1` and
  `rd.quote_snapshot_r1`, `USAGE` on `rd`; `ot_ro_pages` has no grant on a base table.
- Line names: `OT_PG_DSN_OPTION_EOD`, `OT_PG_DSN_QUOTE_SNAPSHOT`.

Declarations added to `internal/store/api.go` (declarations only; `<Record>` types come from `contracts`):

```go
// OptionEodWriter appends option end-of-day rows; a row whose key exists is left as it is.
type OptionEodWriter interface {
	InsertOptionEod(ctx context.Context, rows []contracts.OptionEod) error
}

// OptionEodReader reads the rows of an underlying with session_day between two days, inclusive,
// ordered by session_day, instrument_id and capture_id.
type OptionEodReader interface {
	OptionEod(ctx context.Context, underlyingID int64, from, to contracts.Day) ([]contracts.OptionEod, error)
}

// QuoteSnapshotWriter appends quote snapshots idempotently.
type QuoteSnapshotWriter interface {
	InsertQuoteSnapshots(ctx context.Context, rows []contracts.QuoteSnapshot) error
}

// QuoteSnapshotReader reads the snapshots of an instrument with session_day between two days, inclusive,
// ordered by session_day, slot and capture_id.
type QuoteSnapshotReader interface {
	QuoteSnapshots(ctx context.Context, instrumentID int64, from, to contracts.Day) ([]contracts.QuoteSnapshot, error)
}

// OptionEodStore is the Reader and the Writer of obs.option_eod.
type OptionEodStore interface {
	OptionEodReader
	OptionEodWriter
	Close() error
}

// QuoteSnapshotStore is the Reader and the Writer of obs.quote_snapshot.
type QuoteSnapshotStore interface {
	QuoteSnapshotReader
	QuoteSnapshotWriter
	Close() error
}

// The lines of $OT_HOME/.env for the two writer roles.
const (
	DSNEnvOptionEod     = "OT_PG_DSN_OPTION_EOD"
	DSNEnvQuoteSnapshot = "OT_PG_DSN_QUOTE_SNAPSHOT"
)

// OpenOptionEodFunc opens the option end-of-day store (internal/store/snapshot.OpenOptionEod).
type OpenOptionEodFunc func(ctx context.Context, dsn string) (OptionEodStore, error)

// OpenQuoteSnapshotFunc opens the quote snapshot store (internal/store/snapshot.OpenQuoteSnapshot).
type OpenQuoteSnapshotFunc func(ctx context.Context, dsn string) (QuoteSnapshotStore, error)
```

Shared by both writers: one transaction per call, no database call for empty input, the SQL names every column and never
uses `SELECT *`, `session_day` is the market-zone date, instants are UTC, errors are `contracts.TypedError` (`Retryable`
for the database, `Permanent` for a malformed day), the caller's context reaches every call, no clock and no environment read.

## Modules

No change to `docs/design/modules.md`: the package `internal/store/snapshot` falls under the row `store`. The lint files do
change: `arch.toml` and `.go-arch-lint.yml` belong to `repo` and are regenerated by `go run ./scripts/arch`; the
option_eod issue, which creates the package directory, carries them as a judged two-module entry, as the go.mod issues do
(`modules.md`, ownership of shared paths).

Licence for two-module entries (docs/design/modules.md and AGENTS.md otherwise allow an issue only the paths of its module). Two entries edit files of two modules, and this design licenses exactly those. (1) `store-week2-interface` edits `contracts` (the two schemas, the generated records, the documents, the fixtures) together with `store` (`internal/store/api.go` and `api_test.go`). Reasons: (i) `api_test.go` reads the fixtures through `contracts.OptionEod` and `contracts.QuoteSnapshot`, so the entry cannot be built before the schemas are on main; (ii) crew allows exactly one "(interface)" entry per design and holds every other entry until that entry lands, so a separate schemas entry would wait for the interface entry that needs it; (iii) AGENTS.md says generated code and its source land together, and they do. (2) `store-option-eod` edits `store` together with the lint files `arch.toml` and `.go-arch-lint.yml` of `repo`. Reasons: (i) `.go-arch-lint.yml` rejects a package that belongs to no component and `scripts/arch --check` rejects a stale file, so the new package and the regenerated lint files land in one pull request; (ii) this entry creates the package. Nothing else edits files of two modules.

| issue | module | order |
|---|---|---|
| store-week2-interface | contracts, store | first |
| store-week2-roles | store | after store-week2-interface |
| store-option-eod | store, with the lint files of repo | after store-week2-roles |
| store-quote-snapshot | store | after store-option-eod |
| store-week2-doc | store | last |

The cut: the interface entry holds the two dataset schemas with their records and fixtures and the store declarations that read them, so it can be built on main alone; it is the only entry marked (interface). The option_eod entry holds the new package with its sentinel screen and the lint files. The roles issue makes the roles and repairs the
pg test helpers that migrate the whole `migrations/` directory. The two writer issues run one after the other: the first creates the shared pgx seam
`snapshot/db.go` that the second uses, and 0006 must merge before 0007 because goose refuses a lower version applied
after a higher one.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the diff,
with an offline grep or a named test. Every test added calls `t.Parallel()`, uses `t.TempDir()` for any file, never sleeps
and never reads the real clock; every new source file stays under 600 lines. Pull request description conventions (the `Contracts:` line, and for the interface entry that it names the two new files under `contracts/datasets`) are merge conditions under AGENTS.md, not acceptance items.

### store-week2-interface (interface)

The two dataset schemas with their records and fixtures, and the store declarations that use them.
  Goal: `contracts` holds `obs.option_eod r1` and `obs.quote_snapshot r1` as source and generated output, with synthetic fixtures; `store` names both stores, the two line names and the two open functions through `internal/store/api.go` alone.
  Files: contracts/datasets/obs.option_eod.r1.json, contracts/datasets/obs.quote_snapshot.r1.json, contracts/generate.sh, internal/contracts/gen/records.go, internal/contracts/api.go, internal/contracts/schema_test.go, docs/contracts/datasets/obs.option_eod.md, docs/contracts/datasets/obs.quote_snapshot.md, contracts/fixtures/obs.option_eod.r1.json, contracts/fixtures/obs.quote_snapshot.r1.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/store/api.go, internal/store/api_test.go
  Design: docs/design/store-week2.md#interface. A judged two-module entry (`contracts` and `store`): `api_test.go` reads the fixtures through `contracts.OptionEod` and `contracts.QuoteSnapshot`, which exist only once the schemas land, so the schemas and the declarations that use them are one pull request and this is the single interface entry.
  Acceptance:
  1. Each schema has `x-dataset`, `x-revision` `r1`, `x-primary-key` equal to the key of `## Interface`, `additionalProperties: false`, every column of `## Interface` and no other; the non-key price, size, volume, IV and Greek columns accept `null`; `status` is an integer from 1 to 5; `currency` and `underlying_ccy` match `^[A-Z]{3}$`; `capture_id` has format `uuid`. Offline: `grep -c '"x-primary-key"'` prints 1 for each file, and `grep -cE 'iv_(bid|mid|ask)_decimal|iv_decimal' contracts/datasets/obs.quote_snapshot.r1.json` prints 0.
  2. `contracts/generate.sh` names both schemas; `internal/contracts/gen/records.go` and both `docs/contracts/datasets/*.md` are the unedited output of `sh contracts/generate.sh` (offline: running it leaves `git diff --exit-code` clean; both md files start with `<!-- Code generated by contracts/gendoc`).
  3. `internal/contracts/api.go` gains exactly the aliases `OptionEod` and `QuoteSnapshot` (offline: `grep -c 'gen\.' internal/contracts/api.go` grows by 2).
  4. Each fixture file has one record per line, at least four records with unique keys, one record with status 3 or 2 and null prices, one with a null delta, and uses capture ids and instrument ids that are plainly synthetic; neither contains the marker `synthetic-v1`; the manifest lists both with their sha256 (offline: `sha256sum` equals the entry; `go run ./scripts/hygiene` passes).
  5. `TestDatasetFixtures` in `schema_test.go` names both datasets; a case per schema shows a record with an extra property, a status of 6 and a currency of `usd` is rejected (offline: `grep -c 'obs.option_eod' internal/contracts/schema_test.go` is at least 1).
  6. `api.go` declares the types and constants of `## Interface` and gains no function body (offline: `grep -c '^func ' internal/store/api.go` prints the same number as at caacdb4); it still imports only `context`, `time` and `contracts`.
  7. `api_test.go` has two halves: a fake in the test that implements both stores (a repeat of one key adds no row; a second capture with a new `capture_id` adds one; inclusive range; documented order), and a caller written against the interfaces only that reads the fixtures of this entry through `contracts.OptionEod` and `contracts.QuoteSnapshot`. Tests `TestOptionEodStoreContract` and `TestQuoteSnapshotStoreContract`.
  8. No file outside the thirteen listed is in the diff. `internal/contracts/gen/records.go` has 184 lines at the base; two generated records add roughly 150 lines, so it stays near 350 lines, under the 800-line threshold (offline: `wc -l` after generation is below 800).
  Checked by CI: ci-go, and the `checks` step that runs `sh contracts/generate.sh` and compares.
  Depends on: none. Order: first.
  Size: M

### store-week2-roles

The two writer roles, their lines in `.env`, and the pg test helpers.
  Goal: after `ot setup pg` the roles `ot_w_option_eod` and `ot_w_quote_snapshot` exist with DSNs under their lines, and every pg test that migrates the whole `migrations/` directory has the roles and the Timescale extension in its scratch database.
  Files: internal/store/setup/setup.go, internal/store/setup/setup_test.go, internal/store/setup/setup_pg_test.go, internal/store/setup/migrate_pg_test.go, internal/store/setup/loader_pg_test.go
  Design: docs/design/store-week2.md#brief; the roles are made by `Setup` like `ot_loader` (a SCRAM verifier, the password only in `.env`); the grants are in the migrations of the writer issues. Written against the tree at caacdb4 (after #135).
  Owner's word: needed before merge (two new roles, two new credentials). Pending. The pull request description names the two new lines, says `Setup` touches no other line of `.env`, and carries the owner's word; this is a merge condition, not an acceptance item.
  Acceptance:
  1. `setup.go` declares `roleOptionEod = "ot_w_option_eod"` and `roleQuoteSnapshot = "ot_w_quote_snapshot"`, appends `{roleOptionEod, store.DSNEnvOptionEod}` and `{roleQuoteSnapshot, store.DSNEnvQuoteSnapshot}` to `roles`, and adds both to the `GRANT CONNECT` statement; `Setup` grants nothing on a table (offline: `grep -c 'roleOptionEod\|roleQuoteSnapshot' internal/store/setup/setup.go` is at least 4; `grep -c 'GRANT' internal/store/setup/setup.go` is unchanged).
  2. `ensureRole` is unchanged: no plain password reaches SQL (offline: `grep -c 'literal(password)' internal/store/setup/setup.go` prints 0, and `git diff` shows no line of `ensureRole` or `scram.go`).
  3. `TestRolesTable` expects seven roles, only `ot_owner` with an empty key, and the lines of the two new roles and of `ot_loader` by their `store` constants.
  4. In `migrate_pg_test.go` and `loader_pg_test.go` every scratch database that `migrateWith` or `MigrateUp` migrates gets `CREATE EXTENSION IF NOT EXISTS timescaledb` through the admin connection to that database before the migration (offline: `grep -c 'timescaledb' internal/store/setup/migrate_pg_test.go internal/store/setup/loader_pg_test.go` is at least 1 for each); a database that `loader_pg_test.go` creates itself is covered too; the role creation still runs under `pg_advisory_lock(54)` (offline: `grep -c 'pg_advisory_lock(54)'` unchanged).
  5. No file outside the five listed is in the diff.
  Checked by CI: `TestSetupPG` extended: after `Setup` on a seeded `.env` the file gains two lines whose users are the new roles and whose database is `optdesk`, other lines and comments are byte-equal, a second run changes nothing, and neither log nor error holds a password or verifier; the whole pg job stays green with migrations 0001 to 0005 only.
  Depends on: store-week2-interface (the two line names). #135 is merged at the base. Order: after the interface.
  Size: S

### store-option-eod

Migration 0006, the package `internal/store/snapshot` with its sentinel screen and the lint files for it, and the writer and reader of `obs.option_eod`.
  Goal: the option_eod job can append frozen end-of-day rows and read them back; the table is a compressed hypertable that no writer role can update.
  Files: migrations/0006_option_eod.sql, internal/store/snapshot/screen.go, internal/store/snapshot/screen_test.go, internal/store/snapshot/db.go, internal/store/snapshot/optioneod.go, internal/store/snapshot/optioneod_test.go, internal/store/snapshot/optioneod_pg_test.go, arch.toml, .go-arch-lint.yml
  Design: docs/design/store-week2.md#interface; shape of `internal/store/series`; a judged two-module entry (`store` and the lint files of `repo`): `arch.toml` and `.go-arch-lint.yml` must change in the pull request that creates the package (go-arch-lint rejects an unlisted package and `scripts/arch --check` rejects a stale file); this issue adds the sentinel screen `snapshot/screen.go` and the shared seam `snapshot/db.go` (the `database` interface with InTx, Select and Close, its pgx adapter, and the `retryable` and `permanent` helpers) for itself and for `store-quote-snapshot`.
  Acceptance:
  1. `0006_option_eod.sql` creates `obs.option_eod` with the columns, types, primary key and CHECKs of `## Interface`, `NOT NULL` on the key, identity and `currency`, `underlying_ccy`, `status` columns, `create_hypertable` on `session_day` with a one-month chunk, the compression settings, a compression policy of 7 days, the index on `(instrument_id, session_day DESC)`, and no retention policy (offline: `grep -c 'create_hypertable\|add_compression_policy\|compress_segmentby\|compress_orderby' migrations/0006_option_eod.sql` is 4 or more, and `grep -c 'add_retention_policy' migrations/0006_option_eod.sql` prints 0).
  2. The same file creates `rd.option_eod_r1` naming every column; sets `OWNER TO ot_owner` on table and view; grants as in `## Interface` to `ot_w_option_eod`, `ot_runner` and `ot_ro_pages`; no UPDATE, DELETE or TRUNCATE to anyone but the owner; its Down part drops view then table.
  3. `optioneod.go` declares `OpenOptionEod(ctx, dsn) (store.OptionEodStore, error)` with `var _ store.OpenOptionEodFunc = OpenOptionEod`; the insert names every column and ends `ON CONFLICT DO NOTHING`; no `SELECT *` in the package.
  4. `optioneod_test.go`, against a fake of the seam: the SQL strings; one transaction for several rows; no call for empty input; the context reaches every call; a table test where -1 in `bid`, `iv_decimal`, `gamma` and `delta`, -1 and NaN in `theta_per_calendar_day` and +Inf in `volume` each become NULL while every other column and the status of the row are unchanged; a database error is `Retryable`; a malformed day in the reader is `Permanent`; reader range and order.
  5. `snapshot/screen.go` holds one screen, `screenSentinel(v *float64) *float64` (NULL for NaN, an infinity and -1), used for every numeric column including delta and theta; it declares no `Open` function, no pgx import and no seam. The writer in this pull request uses it, so the `unused` linter has no finding.
  6. `screen_test.go` is a table test of `screenSentinel` on -1, NaN, +Inf, -Inf, 0, -0.5, 0.25 and nil, with the expected result for each (-0.5 passes through and meets the CHECK of the table); the writer also uses it.
  7. `arch.toml` and `.go-arch-lint.yml` are the output of `go run ./scripts/arch` (offline: `go run ./scripts/arch --check` prints nothing and exits 0; `grep -c 'store/snapshot' arch.toml` is 1 or more).
  8. The package reads no environment and no clock (offline: `grep -rn 'os.Getenv\|time.Now\|time.Sleep' internal/store/snapshot` prints nothing).
  Checked by CI: `optioneod_pg_test.go` (first line `//go:build pg`; scratch database with a random name and the Timescale extension created through the admin connection; the test creates the roles `ot_owner`, `ot_runner`, `ot_w_series_fred`, `ot_ro_pages` and `ot_w_option_eod` itself (`ot_w_series_fred` because 0003 grants to it and applying 0003 fails with a missing role otherwise), by name, when missing, under advisory lock 54, and applies the Up part of 0002, 0003 and 0006 with a helper of its own, as `internal/store/series/series_pg_test.go` does; it does not import `store/setup`, which a store sub-package may not): `TestOptionEodRoundTripPG` writes `contracts/fixtures/obs.option_eod.r1.json` and reads it back equal; `TestOptionEodRetryPG` inserts one key twice with other prices (first kept) and a second `capture_id` with the same contract (both kept); `TestOptionEodCompressPG` compresses a chunk of the fixture rows and reads them back equal; `TestOptionEodGrantsPG` shows UPDATE, DELETE and SELECT on the table refused for `ot_w_option_eod` and the base table refused but the view readable for `ot_ro_pages`; `TestOptionEodChecksPG` shows a direct INSERT of -1 into `bid` and of NaN into `delta` refused, and of -1 into `delta` accepted by the table (the writer never sends it).
  Depends on: store-week2-interface, store-week2-roles. Order: after both; merges before store-quote-snapshot.
  Note: 0006 refers to schemas `obs`, `rd` and `ops` and to roles of 0002 and 0003 only (`ot_owner`, `ot_runner`, `ot_w_series_fred`, `ot_ro_pages` and its own writer), never to `master`, so applying 0002, 0003 and 0006 is enough once those roles exist.
  Size: L

### store-quote-snapshot

Migration 0007 and the writer and reader of `obs.quote_snapshot`.
  Goal: the snapshot job can append timed quote snapshots and read them back.
  Files: migrations/0007_quote_snapshot.sql, internal/store/snapshot/quotesnapshot.go, internal/store/snapshot/quotesnapshot_test.go, internal/store/snapshot/quotesnapshot_pg_test.go
  Design: docs/design/store-week2.md#interface; as `store-option-eod`.
  Acceptance:
  1. `0007_quote_snapshot.sql` creates `obs.quote_snapshot` with the columns and key of `## Interface` (no IV column; offline: `grep -cE 'iv_(bid|mid|ask)_decimal|iv_decimal' migrations/0007_quote_snapshot.sql` prints 0), the CHECKs, `create_hypertable` on `session_day` with 7-day chunks, the compression settings and policy, no retention policy, and `rd.quote_snapshot_r1`; owners and grants as acceptance 2 of `store-option-eod` with `ot_w_quote_snapshot`; its Down part drops view then table.
  2. `quotesnapshot.go` declares `OpenQuoteSnapshot` with `var _ store.OpenQuoteSnapshotFunc = OpenQuoteSnapshot`; the insert ends `ON CONFLICT DO NOTHING`; no `SELECT *`.
  3. `quotesnapshot_test.go` covers the cases of acceptance 4 of `store-option-eod` for this table (every numeric column uses `screenSentinel`), and two slots of one instrument and capture are both kept.
  Checked by CI: `quotesnapshot_pg_test.go` (build tag `pg`; same setup as `store-option-eod`, so the roles `ot_owner`, `ot_runner`, `ot_w_series_fred`, `ot_ro_pages` and `ot_w_quote_snapshot` are created by name, with the role `ot_w_quote_snapshot` and the Up part of 0002, 0003 and 0007): `TestQuoteSnapshotRoundTripPG`, `TestQuoteSnapshotRetryPG`, `TestQuoteSnapshotCompressPG`, `TestQuoteSnapshotGrantsPG`.
  Depends on: store-week2-interface, store-week2-roles, store-option-eod (for `snapshot/db.go`, and 0006 before 0007). Order: after all three.
  Size: M

### store-week2-doc

The store contract document.
  Goal: `docs/contracts/store.md` states the two datasets, their writers and rules.
  Files: docs/contracts/store.md
  Design: docs/design/store-week2.md#interface. Written against the landed trees of the four issues above (v4 is the loader line).
  Acceptance:
  1. The file keeps the eight contract headings (offline: `grep -cE '^## (Responsibility|Inputs|Outputs|Invariants|Version history|Contract tests|Jobs and slots|Forbidden)$' docs/contracts/store.md` prints 8).
  2. `## Inputs` and `## Outputs` name the two stores, `snapshot.OpenOptionEod`, `snapshot.OpenQuoteSnapshot`, the two line names and the two views; `## Invariants` states the append-only rule and the retry rule, the per-column sentinel rule, the grants of the two roles (no write on `ops`, `SELECT, REFERENCES` on `ops.capture_run`), that `ops.dataset_window` needs no row of its own, and that a database made before 0006 needs `ot setup pg` before `ot migrate`.
  3. `## Version history` has `- v5:` naming migrations 0006 and 0007 and the two roles; `## Contract tests` names the tests of the writers and the pg tests, each of which exists in the tree (offline: grep for each name under `internal/store`).
  4. No file but `docs/contracts/store.md` is in the diff.
  Depends on: store-option-eod, store-quote-snapshot, store-week2-roles. Order: last.
  Size: S

## Rollout

Merge order is the table of `## Modules`. 0006 merges before 0007 (the second writer issue depends on the first) and the owner does not run `ot migrate` between the two
merges, because goose does not apply a lower version after a higher one. Before the owner runs `ot migrate` on a real
database, `store-week2-roles` has merged and `ot setup pg` has run (0006 and 0007 grant to roles that must exist); the
owner is told the two new `.env` lines. The two tables are hypertables, so the real database needs the extension that
`Setup` already creates.

## Left out

- Job skeletons (the option_eod freeze job, the index IV job): they need a settled writer interface and sources that do not
  exist. A follow-up design after the interface issue and the first writer land.
- A per-dataset window rule in the runner (`WindowRule` is one rule for all jobs): its own interface issue in a follow-up design,
  `runner-window-per-dataset`, changing `internal/runner/api.go` and `docs/contracts/runner.md`.
- A shared fake store for the jobs' tests (a non-test package under `internal/store`): a follow-up design, `store-testkit`,
  once the interface has merged; until then each test file keeps its own fake, as `api_test.go` does for series.
- The capture ledger and what depends on it: `capture_id`, `kind`, a finish time and the `complete`-style statuses on
  `ops.capture_run`, the foreign keys from both tables to it, the winner views that replace the pass-through views, and
  `ops.job_event`: a follow-up design, `capture-ledger`. It is a `Depends on` of the first job that writes real rows
  (weeks 3-4), so no live row holds a `capture_id` that references nothing.
- The master tables and the foreign keys of `instrument_id`, `underlying_id` and `terms_version_id` to them.
- The index IV figure: it is stored neither in `obs.quote_snapshot` nor in `obs.option_eod`. Schema v3 leaves it as an
  open question under the name `market.index_iv` (a table whose rows can be fetched again). The design of the index IV job
  defines that table, keyed like `obs.quote_snapshot` by `(session_day, capture_id, slot, instrument_id)`, the same four columns.
- The tick tables and wiring in `cmd/ot`.
- Any ops table or row, any broker, sources or compute code.
- Alternatives not chosen, with the reason: one role for both writers (the advisor's rule is one writer role per job, so a
  credential is as wide as one job); one migration for both tables (a Down part per table stays simple and the order 0006
  before 0007 is checkable); starting with the option table only (both jobs of weeks 3-4 need their table); two packages
  instead of one (a second lint component and a second copy of the pgx seam for no gain).

## Adversarial

The full adversarial review of the first draft is `adv-store-week2.md` in the dispatcher scratchpad (a local path, kept
out of this public file). Its six points, each answered:

1. Gap against the goal. Adopted in part. The window rule and a shared fake store are real gaps; they are named in
   `## Left out` with the follow-up designs `runner-window-per-dataset` and `store-testkit`, because each is its own
   interface change. The IV gap is stated, not closed: v3 has no IV column on the quote table. Wiring in `cmd/ot` waits for a job.
2. The unnamed risk (CI red outside the Files lines). Adopted. The lint files are now carried by `store-option-eod`, the issue that creates the package, as
   a judged two-module entry (store and repo; the first cut put them in the interface issue, and the cut was revised after #163), and `migrate_pg_test.go` and `loader_pg_test.go` are in `store-week2-roles` with the advisory
   lock and the Timescale extension in the scratch databases; goose ordering is in `## Rollout`; the frozen-key retry is
   defined in `## Interface`. The reviewer's other point, that a first write could freeze a bad capture, is answered by the
   key: a better observation is a new `capture_id`, a different key.
3. A simpler way. Adopted in part. One package instead of two (one lint component, one pgx adapter) and the contract
   document as one last issue; not adopted: one role (the advisor's rule is one writer role per job, and the owner agreed), one
   migration (one table per migration keeps each Down part simple and the 0006 before 0007 order checkable), and starting
   with the option table only (the owner wants both tables for the two jobs).
4. Lock-in. Adopted. The key now holds `instrument_id`, which tells apart contracts that share underlying, expiry,
   strike and right (an index option and its weekly, adjusted contracts), so the second row is no longer dropped;
   the quote key holds `capture_id`, `slot` and `instrument_id`; a sentinel clears one column and never the row. Tables are
   hypertables as the advisor fixed; the migrations are applied by the owner, so a later change is a new migration.
5. The false claim. Adopted: `arch.toml` and `.go-arch-lint.yml` do list the store sub-packages and need regeneration; the
   claim is gone and the regeneration belongs to `store-option-eod`. The other checks of the reviewer (record names, the
   count of aliases, `TestRolesTable` coming from #135) are corrected: no enum type is generated, so two aliases, and the
   roles issue is written against the tree after #135.
6. Likeliest slip. Adopted. The lint files and the pg test helpers have an owner; the first cut split the contracts and store entries; after #163 the schemas and the store declarations are one interface entry (contracts and store) and the lint files are in `store-option-eod` (store and repo), so there are exactly two licensed two-module entries, with the reasons in `## Modules`; the roles and
   doc issues are written against the landed #135 tree; the SQL is checked by named pg tests, with greps only for presence.
