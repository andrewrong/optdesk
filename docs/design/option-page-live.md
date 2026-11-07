# Option data page live: the page reads the collected option data, and the terms of an option are stored when it is first seen

Author: the optdesk dispatcher (revision 2)
Owner approved: (pending)
Revision: 2

Source: owner decision of 2026-11-07: wire the option data page to the real collected option data now. The database may hold no option
data yet (the pyside broker service is not deployed), so the page must then show an honest empty or gap state; once collection runs,
data flows in with no further code change. Sequel to `docs/design/option-page-serve.md`.
Read on origin/main at 85a8ac5 (the one sha every fact below is read at): `cmd/ot/site.go`, `cmd/ot/wire.go`, `cmd/ot/demo_option.go`,
`internal/pages/option_data_page/*.go`, `internal/store/api.go`, `internal/store/instrument/*.go`, `internal/jobs/optioneod/run.go`,
`migrations/0006` to `0009`, `docs/design/option-jobs.md`, `docs/design/store-master.md`, `docs/contracts/store.md`, `docs/contracts/ot.md`.

Goal: with the real profile, `ot serve --site` shows the contracts the jobs `option_eod` and `quote_snapshot` wrote, with strike,
expiry and right filled and the symbol of the underlying shown, and shows an honest empty state while the tables are empty. No code change is needed on the day collection starts.

## What main holds today

The page is already served from the store, not from demo data. The gap is smaller than "demo versus store":

- Closed: `openSiteReaders` (`cmd/ot/site.go`) opens `option_eod`, `quote_snapshot`, the `option_eod` catalog and the option terms with the
  line `OT_PG_DSN_PAGES` for every profile except `demo`; `newDeps` (`cmd/ot/wire.go`) binds the four real constructors; `buildSite` asks
  the catalog, picks the underlying and calls `option_data_page` with the real readers. The demo readers (`cmd/ot/demo_option.go`) are used
  only when the profile is `demo`. Role grants for the four reads exist (migrations 0006, 0007, 0009).
- Closed: the writers. `option_eod` and `quote_snapshot` write `obs.option_eod` and `obs.quote_snapshot` with the ids the instrument
  resolver gave (`option-jobs.md`); `ot run-once` and `ot backfill` run them (#239).
- Closed: the empty case in the builder. A read with no rows gives a `partial` page with the gap `no option data stored` and the sentence
  `0 contracts on <day>.`; `cmd/ot/site_test.go` already runs the empty catalog (id 0) with a fake. The web view renders `gaps` and an empty chain.
- Open 1, strike, expiry and right. The page takes them from `master.option_terms` through `rd.option_terms_r1`, and nothing writes that
  table: no writer role has a right on it (`0009`), `ResolveInstrument` writes only `master.instrument` and `master.instrument_identifier`,
  and the jobs keep an option's terms only in the identifier of scheme `osi` and in the name (`option-jobs.md`, `store-master.md` Left out 1).
  So with real collected data every chain row shows `expiry` and `right` null and `strike` as the gap `no strike`: honest, but the chain
  cannot be read, and nothing changes when collection starts. This is the one gap that needs code before the page is useful on real data.
- Open 2, the empty state is thin. With an empty catalog the page says `Underlying 0` and the one gap line; nothing tells the viewer that
  collection has not run. `Config.Notes` exists and the web view shows notes, but `ot` passes none.
- Open 3, the symbol. The label is `Underlying <id>` and `symbol` is null, so the switcher and the title show ids only and an owner cannot tell
  which id is which underlying. The page role has `SELECT` on `master.instrument` (`0008`) but no `USAGE` on schema `master`, so no reader can
  read a name; a view and a reader are needed (an interface change, below).
- Migration numbers: main ends at `0009`; no open pull request touches `migrations/` (read with `gh pr list --state open` at the time of writing).

## Decisions

1. The terms are written where the option is first seen. `ResolveInstrument` creates an option's instrument only once; in the same
   transaction it also inserts the current row of `master.option_terms`, read from the `osi` identifier that the jobs already pass (21
   characters: symbol padded to 6, `yymmdd` in the years 2000 to 2099, `C` or `P`, strike times 1000 in 8 digits; the exact inverse of the
   job's own formatter). The page, the jobs, the schema and the web stay as they are for terms. Rejected: a terms writer in the jobs (a new
   interface in `jobs` and `store`; the osi string is the one source the jobs have).
2. A terms failure never aborts the resolve. The terms insert runs between `SAVEPOINT option_terms` and `RELEASE`; on any error of that
   statement the transaction does `ROLLBACK TO SAVEPOINT option_terms` and goes on, so the instrument and its identifiers are still
   committed and the call returns the id and a nil error. The same holds for an `osi` that does not parse, a type other than `option`, or no
   `osi` in `extra`: no terms row, no error. A bad symbol or a missing grant never stops collection.
3. Migrate before deploying the code. The code that inserts terms needs migration 0011 (the grant); the savepoint makes a code-before-migration
   deploy harmless (the insert is refused, the option is still made, the page shows the gap), but terms are then missing for those options and
   are not written later (point 5), so the order is: `ot migrate`, then the new `ot` binary.
4. Defaults for the two columns the jobs cannot know: `exercise_style` and `settlement_type` are the text `unknown`; `strike_currency` and
   `multiplier` come from the spec (`Currency`, `LotSize`). Nothing shows these two. The grant is `INSERT` only, so a later correction of
   `unknown` needs a later writer with its own grant; today nobody closes or replaces a terms row.
5. First seen only, no backfill. Option instruments that exist already get no terms row: the database holds no collected option data yet, so
   a SQL parse and its failure surface are not worth it. An option made before this lands keeps the gap values.
6. The symbol comes from a new page-role view `rd.instrument_r1` (`instrument_id`, `instrument_type`, `name`, `currency`; no identifier, no
   issuer) and a new reader `store.InstrumentNameReader` (`## Interface`). The jobs set `Name` of a stock or index to its symbol
   (`optioneod/run.go`). `ot` reads the names of the catalog ids once per request, writes `Config.Symbol` and uses the name as the label of the
   underlying and of each choice (at most 50 ids, within the 500 cap); an id with no row, or an option type, keeps `Underlying <id>`. The builder
   and the web are unchanged.
7. The empty state is a note, not a new gap kind. When the catalog has no underlying, `ot` passes one note to the builder: `No option data is
   stored yet; collection has not run.` and the label `No underlying yet`. A non-empty catalog adds no note; an id with no rows keeps the
   builder's gap line.
8. No change to a schema, a fixture or a web file. Nothing sleeps and no clock is read by the new code: the parse is pure, the note is a
   constant, tests use the fake database seam of `internal/store/instrument` and the fake page and readers of `cmd/ot`.

## Modules

No row of `docs/design/modules.md` is added or changed, so `arch.toml` is unchanged. `migrations/**`, `internal/store/**` and
`docs/contracts/store.md` are store; `cmd/**` and `docs/contracts/ot.md` are ot. No new dependency.

| issue | module |
|---|---|
| option-live-interface (interface) | store |
| option-terms-written | store |
| option-names-reader | store |
| option-page-live-ot | ot |

## Interface

Owner: store. Contracts line: `Contracts: unchanged` (no file under `contracts/schemas`, `contracts/datasets` or `contracts/proto`).

`internal/store/api.go` gains, declarations only:

```go
// InstrumentName is the name of one instrument; for a stock or index it is the symbol.
type InstrumentName struct {
	InstrumentID int64
	Type, Name   string
}

// InstrumentNameReader reads names by instrument id.
type InstrumentNameReader interface {
	// InstrumentNames returns the names of ids ordered by InstrumentID; an unknown id is absent; an empty list gives
	// an empty answer and no database call; more than 500 ids or an id of 0 is Permanent.
	InstrumentNames(ctx context.Context, ids []int64) ([]InstrumentName, error)
}

// InstrumentNameStore is the reader of the names and Close.
type InstrumentNameStore interface {
	InstrumentNameReader
	Close() error
}

// OpenInstrumentNameFunc opens the name reader with the line DSNEnvPages (instrument.OpenInstrumentNames).
type OpenInstrumentNameFunc func(ctx context.Context, dsn string) (InstrumentNameStore, error)
```

Migration `0010_instrument_names_view.sql` creates `rd.instrument_r1` over `master.instrument` (owner `ot_owner`, so the view runs with its
owner's rights), and grants `SELECT` on it to `ot_ro_pages` (`USAGE` on schema `rd` exists since 0006). `ot_ro_pages` still gets nothing in
schema `master`. The grant is a change of who may read what: the issue carries the waiver lines.

## Issues

1. option-live-interface (interface), starts the day this design merges.
2. option-terms-written, Order: after 1 (both edit `internal/store/migrate/migrate_test.go` and `docs/contracts/store.md`; the migration numbers 0010 and 0011 follow each other).
3. option-names-reader, Order: after 1 (it implements the types of 1); independent of 2 (disjoint files).
4. option-page-live-ot, Order: after 3 (`newDeps` names the constructor `instrument.OpenInstrumentNames`); its tests use fakes only.

Entries 2 and 3 run together once 1 has merged. What is shown end to end (a real database, the service deployed, the page filled) needs
all four merged and the collection running, and is the owner's check: no test proves the owner-visible goal before the service runs.

### option-live-interface (interface)

Module: store. This issue carries a grant (read on a new view for the page role): its issue carries the waiver lines (advisor answers, the owner merges).

Files: internal/store/api.go, internal/store/api_test.go, migrations/0010_instrument_names_view.sql, internal/store/migrate/migrate_test.go, docs/contracts/store.md

Design: docs/design/option-page-live.md#interface

Acceptance:

1. `internal/store/api.go` holds the four declarations of `## Interface` and no function body: `grep -c '^func ' internal/store/api.go` prints the same number as on origin/main; `go build ./...` passes.
2. The contract test in `internal/store/api_test.go`, `TestInstrumentNameContract`, checks from both sides with a fake: a reader fake satisfies `InstrumentNameReader`, a caller that passes 501 ids or an id of 0 gets the documented `Permanent` kind, an empty list gives an empty answer; `go test ./internal/store -run TestInstrumentNameContract -count=1 -race` passes.
3. `migrations/0010_instrument_names_view.sql` has an Up and a Down part; Up creates `rd.instrument_r1` with the four columns, sets its owner to `ot_owner` and grants `SELECT` to `ot_ro_pages` only; Down drops the view.
4. `TestMigration0008` in `internal/store/migrate/migrate_test.go` is changed (it takes `names[len-2]` as 0008 and fails once 0010 exists: it must find 0008 by name), `TestMigrationsEndWith0009` is replaced by a test that ends with `0010_instrument_names_view.sql`, and `go test ./internal/store/... -count=1 -race` passes.
5. `docs/contracts/store.md` names the reader, the view, the grant and a version line `v8` (the last line is `v7`, migration 0009; the next line is `v8`).
6. No file outside the five listed is in the diff.

Checked by CI: the migration Up and Down against PostgreSQL (the `pg` tests of `internal/store/migrate`).

### option-terms-written

Module: store. This issue carries a grant (`INSERT` on `master.option_terms` to the role `ot_w_master`, which already writes `master.instrument`; no new role, no new schema right): its issue carries the waiver lines (advisor answers, the owner merges).

Files: migrations/0011_option_terms_writer.sql, internal/store/instrument/resolve.go, internal/store/instrument/osi.go, internal/store/instrument/osi_test.go, internal/store/instrument/resolve_test.go, internal/store/instrument/terms_pg_test.go, internal/store/migrate/migrate_test.go, docs/contracts/store.md

Design: docs/design/option-page-live.md#decisions

Acceptance:

1. `osi.go` holds one pure function that turns an osi value into right, strike (decimal text) and expiry (day, year 2000 plus `yy`), with an error for a value that is not 21 characters, a right other than `C` or `P`, or a date or strike that does not parse; `go test ./internal/store/instrument -run TestParseOSI -count=1 -race` passes with a table of a call, a put, a symbol shorter than 6 padded with spaces, a strike with a fraction (`00012500` is `12.5`), a strike of 8 digits at most, and each error case.
2. With a fake database, `ResolveInstrument` of a new option with a valid `osi` extra runs `SAVEPOINT`, one insert into `master.option_terms` (`right`, `strike`, `expiration_date`, `strike_currency` from `Currency`, `multiplier` from `LotSize`, `unknown` for style and settlement) and `RELEASE`, in the transaction of the instrument and identifier inserts; `TestResolveOptionTerms`.
3. With a fake database whose terms insert returns an error, the call returns the id and a nil error, the transaction runs `ROLLBACK TO SAVEPOINT` and commits, and the instrument and identifier inserts were run; `TestResolveTermsFailureKeepsInstrument`.
4. With a fake database, no terms statement runs for: an existing key, a type other than `option`, an option with no `osi` extra, an `osi` that does not parse; in the last three the instrument is created and the error is nil; `TestResolveNoTerms`.
5. Collision case, `TestResolveOsiCollision`: a second option whose `osi` already names another instrument (same symbol, expiry, right and strike, a different key, as an SPX and an SPXW option or an adjusted option give, because the job's `osiSymbol` ignores the trading class) returns a `Permanent` error as today, writes nothing, and runs no terms statement. This design does not change that behaviour (`## Left out`).
6. `migrations/0011_option_terms_writer.sql` has an Up part that grants `INSERT` on `master.option_terms` to `ot_w_master` and nothing else, and a Down part that revokes it. `internal/store/migrate/migrate_test.go` ends with 0011 (the tests changed by 1 of the interface issue are changed again); `go test ./internal/store/... -count=1 -race` passes.
7. `terms_pg_test.go` (build tag `pg`): its helper `newTermsDatabase` applies only 0009 today; it applies 0010 and 0011 as well, and the file gains `TestResolveWritesTermsPG` (a new option resolved by the role `ot_w_master` has a current terms row the page reader returns). `go vet -tags pg ./internal/store/instrument` passes offline.
8. `docs/contracts/store.md` states the side effect of `ResolveInstrument`, the two defaults, the savepoint, the grant, the order "migrate before deploying the code" and a version line `v9`.
9. `internal/store/api.go` does not change (`git diff --stat origin/main -- internal/store/api.go` prints nothing); every file stays under 600 lines.
10. The result matches the section Decisions (points 1 to 5) of docs/design/option-page-live.md.

Checked by CI: `TestResolveWritesTermsPG` and the migration 0011 Up and Down against PostgreSQL.

### option-names-reader

Module: store.

Files: internal/store/instrument/names.go, internal/store/instrument/names_test.go, internal/store/instrument/names_pg_test.go

Design: docs/design/option-page-live.md#interface

Acceptance:

1. `instrument.OpenInstrumentNames` satisfies `store.OpenInstrumentNameFunc` (`var _ store.OpenInstrumentNameFunc = OpenInstrumentNames`) and reads `rd.instrument_r1` only, never `master`: `grep -c 'master\.' internal/store/instrument/names.go` prints 0.
2. With a fake database: ids are returned ordered by id, an unknown id is absent, an empty list makes no database call, 501 ids and an id of 0 are `Permanent`, a database error is `Retryable`; `go test ./internal/store/instrument -run TestInstrumentNames -count=1 -race` passes, every test calls `t.Parallel()`.
3. `names_pg_test.go` (build tag `pg`) has `TestInstrumentNamesPG` with the role `ot_ro_pages`; `go vet -tags pg ./internal/store/instrument` passes offline.
4. No file outside the three listed is in the diff, each under 600 lines.
5. The result matches the section Interface of docs/design/option-page-live.md.

Checked by CI: `TestInstrumentNamesPG`.

### option-page-live-ot

Module: ot.

Files: cmd/ot/site.go, cmd/ot/wire.go, cmd/ot/demo_option.go, cmd/ot/site_test.go, cmd/ot/wire_test.go, docs/contracts/ot.md

Design: docs/design/option-page-live.md#decisions

Acceptance:

1. `deps` gains `openInstrumentName store.OpenInstrumentNameFunc`, bound to `instrument.OpenInstrumentNames` in `newDeps`; `openSiteReaders` opens it with the same line as the other four and closes it with them; the demo profile opens none and its demo readers answer names for the two demo underlyings. `go test ./cmd/ot -run 'TestSiteDemoOpensNoStore|TestSiteMissingDSN' -count=1 -race` passes.
2. With fakes, `GET /options/data` of a catalog of two underlyings whose names are `AAA` and `BBB` builds the page with `underlying.symbol` and `underlying.label` of the chosen one and the labels of both choices as the names; an id with no name, or of type `option`, keeps `Underlying <id>`; one name read per request with at most 50 ids; `TestSiteNames`.
3. With a fake catalog that is empty and fake readers that hold nothing, `GET /options/data` answers 200 with a page whose JSON has `run_status` `partial`, one gap, the note `No option data is stored yet; collection has not run.` and `underlying.label` `No underlying yet`, and makes no name read; `TestSiteEmptyStore`.
4. With a non-empty catalog the page has no such note; `/options/data/<id>` of an id with no rows keeps the gap `no option data stored` and adds no note; `TestSiteNoteOnlyWhenCatalogEmpty`.
5. A reader error of the fake catalog or of the fake name reader answers 500 with the fixed text and the log has `component=ot status=failed` only; `TestSiteReaderErrors`.
6. `sitePageInput` gains `Notes []string` and `Symbol string`, and `newOptionDataPage` passes them to `Config.Notes` and `Config.Symbol`; `cmd/ot/wire_test.go` checks both reach the built page.
7. `go test ./cmd/ot -count=1 -race` passes; every new test calls `t.Parallel()`, writes no file outside `t.TempDir()`, binds no fixed port and does not sleep.
8. `docs/contracts/ot.md` states the five stores, the names, the empty-catalog note and label in the `--site` line.
9. The result matches the section Decisions (points 6 and 7) of docs/design/option-page-live.md.

## Left out

1. Terms of option instruments that exist before this lands, and any correction of `unknown` style and settlement (Decisions 4 and 5): no backfill, no update grant. The page keeps the gap values for them.
2. The collision of the osi identifier. The job's `osiSymbol` uses the symbol only, not the trading class, so an SPX and an SPXW option of the same expiry, right and strike (or an adjusted option) give one osi for two contracts; `ResolveInstrument` then returns `Permanent` for the second and the job stops on that contract. This design neither causes nor fixes it; it needs the trading class in the identifier (a `jobs` change) and may be the reason an index underlying stays empty on real data. A strike of 100000 or more and a symbol longer than 6 also overflow the 21-character form; both fail the parse and give no terms row.
3. A terms writer in the jobs, and the real exercise style and settlement type: the broker chain answer has them; the jobs do not carry them yet.
4. A freshness note on the page (collection stopped for days): the page shows the day of its rows and the catalog window is 7 days; a stale label is a separate design with the runner status.
5. Deploying the pyside service, the `.env` line and the first real run: the owner's work (`docs/deploy/pyside.md`), not an item.
6. A failed terms insert is silent (no log in the store seam); the page shows the gap. A log line needs a logger in the store, its own design.

## Adversarial

Read: `adversarial-optlive.md`, kept in the dispatcher's scratchpad, on head ad2aa2f. Each of its six points:

1. Gap against the goal (no symbol): adopted. The owner needs the symbol now, so the design adds the interface entry, the view `rd.instrument_r1`, the reader and the `ot` wiring (Decision 6).
2. Biggest risk: adopted in three parts. The grants are named and the two entries that carry them say their issue carries the waiver lines; the terms insert is in a savepoint so a failure cannot abort the resolve, and the order is migrate before deploy (Decisions 2 and 3); the collision of the osi identifier is named, tested at the store (acceptance 5 of option-terms-written) and left out with its effect (`## Left out` 2). Not adopted: the column validation before the insert, because the savepoint covers every insert failure and `checkInput` already validates the currency.
3. Simpler way: adopted. The store-side parse stays and the SQL backfill is dropped (Decision 5).
4. Lock-in: adopted. The `unknown` values and the missing update grant are stated (Decision 4, Left out 1); migration numbers 0010 and 0011 are checked against open pull requests (none touches `migrations/`).
5. Claim most likely false: adopted. `TestMigration0008` is named in the interface entry, the helper of `terms_pg_test.go` applies 0010 and 0011, and the version lines are `v8` and `v9` after `v7` (verified in `docs/contracts/store.md`).
6. Likeliest slip: adopted. The CI-only tests are listed under Checked by CI, the owner-visible goal is stated as unproved before the service runs, and this section exists. Not adopted: a hardened backfill, there is none.
