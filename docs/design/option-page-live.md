# Option data page live: the page reads the collected option data, and the terms of an option are stored when it is first seen

Author: the optdesk dispatcher (revision 1)
Owner approved: (pending)
Revision: 1

Source: owner decision of 2026-11-07: wire the option data page to the real collected option data now. The database may hold no option
data yet (the pyside broker service is not deployed), so the page must then show an honest empty or gap state; once collection runs,
data flows in with no further code change. Sequel to `docs/design/option-page-serve.md`.
Read on origin/main at 85a8ac5 (the one sha every fact below is read at): `cmd/ot/site.go`, `cmd/ot/wire.go`, `cmd/ot/demo_option.go`,
`internal/pages/option_data_page/*.go`, `internal/store/api.go`, `internal/store/instrument/*.go`, `internal/jobs/optioneod/run.go`,
`migrations/0006` to `0009`, `docs/design/option-jobs.md`, `docs/design/store-master.md`, `docs/contracts/store.md`, `docs/contracts/ot.md`.

Goal: with the real profile, `ot serve --site` shows the contracts the jobs `option_eod` and `quote_snapshot` wrote, with strike,
expiry and right filled, and shows an honest empty state while the tables are empty. No code change is needed on the day collection starts.

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
- Not a gap of this design: the label `Underlying <id>` and the null `symbol`. The role `ot_ro_pages` has `SELECT` on `master.instrument`
  (`0008`) but no `USAGE` on schema `master`, so no reader of the page role can read a symbol; a symbol needs a view and a reader (`## Left out`).

## Decisions

1. The terms are written where the option is first seen. `ResolveInstrument` creates an option's instrument only once; in the same
   transaction it also inserts the current row of `master.option_terms`, read from the `osi` identifier that the jobs already pass (21
   characters: symbol padded to 6, `yymmdd`, `C` or `P`, strike times 1000 in 8 digits). The page, the readers, the jobs, the schema and
   the web stay as they are, so data flows in with no further change. Conflicts with the other way: a terms writer in the jobs would need a
   new interface and a change in `jobs` and `store`; rejected, the osi string is the one source the jobs have.
2. The parse lives in the store. It is a pure function over a fixed format (own implementation, because no library parses a 21-character
   option symbol of the form the jobs build). An `osi` that does not parse, a type other than `option`, or no `osi` in `extra` gives no terms row
   and no error: the instrument is still made (a bad symbol never stops collection).
3. Defaults for the two columns the jobs cannot know: `exercise_style` and `settlement_type` are the text `unknown`, `strike_currency` and
   `multiplier` come from the spec (`Currency`, `LotSize`). The page does not show these two.
4. A migration fills the terms of the option instruments that exist already, with the same parse in SQL, and grants `INSERT` on
   `master.option_terms` to the writer role of the master. It is idempotent (an instrument with a current terms row is skipped).
5. The empty state is a note, not a new gap kind. When the catalog has no underlying, `ot` passes one note to the builder: `No option data is stored yet; collection has not run.` and the label `No underlying yet`. The text is
   written once, in `ot`. A non-empty catalog adds no note; an id that has no rows keeps the builder's gap line.
6. No change to a schema, a fixture or a web file: the shapes already carry notes, gaps and filled terms (`contracts/fixtures/` `gap.json` and `terms.json`).
7. Nothing sleeps and no clock is read by the new code: the parse is pure, the note is a constant, tests use the fake database seam of
   `internal/store/instrument` and the fake page of `cmd/ot`.

## Modules

No row of `docs/design/modules.md` is added or changed, so `arch.toml` is unchanged. `migrations/**`, `internal/store/**` and
`docs/contracts/store.md` are store; `cmd/**` and `docs/contracts/ot.md` are ot. No new dependency.

| issue | module |
|---|---|
| option-terms-written | store |
| option-page-empty-state | ot |

## Interface

None. `store.InstrumentResolver.ResolveInstrument` keeps its signature and its documented contract in `internal/store/api.go` (the new
side effect is stated in `docs/contracts/store.md`); `option_data_page.Config` already has `Notes`; no file under `contracts/schemas`,
`contracts/datasets` or `contracts/proto` changes. `Contracts: unchanged`.

## Issues

Both entries start the day this design merges and need nothing from each other: the store entry is tested with the fake database seam,
the `ot` entry with a fake page and fake readers. What is shown end to end (a real database, the service deployed, the page filled) needs
both merged and the collection running, and is the owner's check, not an item.

### option-terms-written

Module: store.

Files: migrations/0010_option_terms_writer.sql, internal/store/instrument/resolve.go, internal/store/instrument/osi.go, internal/store/instrument/osi_test.go, internal/store/instrument/resolve_test.go, internal/store/instrument/terms_pg_test.go, internal/store/migrate/migrate_test.go, docs/contracts/store.md

Acceptance:

1. `internal/store/instrument/osi.go` holds one function that turns an osi value into right, strike (decimal text), and expiry (day), with an error for a value that
   is not 21 characters, has a right other than `C` or `P`, or a date or strike that does not parse; `go test ./internal/store/instrument -run TestParseOSI -count=1 -race`
   passes with a table that holds a call, a put, a symbol shorter than 6 padded with spaces, a strike with a fraction (`00012500` is `12.5`), and each error case.
2. With a fake database, `ResolveInstrument` of a new option with a valid `osi` extra runs one insert into `master.option_terms` in the same transaction as the
   instrument and identifier inserts, with `right`, `strike`, `expiration_date`, `strike_currency` from `Currency`, `multiplier` from `LotSize`, and `unknown` for
   the style and the settlement; `go test ./internal/store/instrument -run TestResolveOptionTerms -count=1 -race` passes.
3. With a fake database, no terms insert happens for: an instrument that already exists (the key is found), a type other than `option`, an option with no `osi` extra,
   an `osi` that does not parse; in the last three the instrument is still created and the error is nil. The test name is `TestResolveNoTerms`.
4. `go test ./internal/store/... -count=1 -race` passes, with `TestMigrationsEndWith0009` renamed and changed to end with `0010_option_terms_writer.sql`, and every test of
   the package that counts migration files updated; the file `0010` has an Up and a Down part, the Up part grants `INSERT` on `master.option_terms` to `ot_w_master`, and the backfill
   statement is guarded so a second run inserts nothing (`WHERE NOT EXISTS` on a current row).
5. `docs/contracts/store.md` states the new side effect of `ResolveInstrument`, the two defaults, the grant and the migration version (`v8`), and the line `ot_ro_pages` has
   nothing in schema `master` stays true.
6. `internal/store/instrument/terms_pg_test.go` (build tag `pg`) gains `TestResolveWritesTermsPG` and `TestBackfillTermsPG`: `go vet -tags pg ./internal/store/instrument` passes offline; the tests run in CI.
7. `internal/store/api.go` does not change: `git diff --stat origin/main -- internal/store/api.go` prints nothing; the new file stays under 600 lines and `resolve.go` under 600.
8. The result matches the section Decisions (points 1 to 4) of docs/design/option-page-live.md.

Checked by CI: the two `pg` tests above against PostgreSQL, which also runs the Up and Down parts of migration 0010.

### option-page-empty-state

Module: ot.

Files: cmd/ot/site.go, cmd/ot/wire.go, cmd/ot/site_test.go, cmd/ot/wire_test.go, docs/contracts/ot.md

Acceptance:

1. With a fake catalog that is empty and fake readers that hold nothing, `GET /options/data` answers 200 with a page whose JSON has `run_status` `partial`, one gap,
   the note `No option data is stored yet; collection has not run.`, and `underlying.label` `No underlying yet`; `go test ./cmd/ot -run TestSiteEmptyStore -count=1 -race` passes.
2. With a fake catalog of two underlyings the same route has no such note and the label `Underlying <id>`; with `/options/data/<id>` of an id that has no rows (catalog not
   empty) the page keeps the gap `no option data stored` and adds no note; test `TestSiteNoteOnlyWhenCatalogEmpty`.
3. A reader error of the fake catalog still answers 500 with the fixed text and the log has `component=ot status=failed` only (the existing test passes unchanged).
4. `sitePageInput` gains `Notes []string` and `newOptionDataPage` passes it to `Config.Notes`; the test in `cmd/ot/wire_test.go` checks a note given to the input is in the built page.
5. The demo profile still opens no store: `go test ./cmd/ot -run TestSiteDemoOpensNoStore -count=1 -race` passes unchanged.
6. `go test ./cmd/ot -count=1 -race` passes; every new test calls `t.Parallel()`, writes no file outside `t.TempDir()`, binds no fixed port and does not sleep.
7. `docs/contracts/ot.md` states the empty-catalog note and label in the `--site` line.
8. The result matches the section Decisions (points 5 to 7) of docs/design/option-page-live.md.

## Left out

1. A symbol and a readable label of an underlying (`SPY` instead of `Underlying <id>`). The page role cannot read `master`; it needs a view such as
   `rd.instrument_r1`, a reader in `internal/store/api.go` and a wire change in `ot`: a store interface change and its own design. Until then the
   label stays, as in `option-page-serve.md` Decision 3.
2. A terms writer in the jobs, and the real exercise style and settlement type. The broker chain answer has them; the jobs do not carry them yet. `unknown` is the honest value.
3. A freshness note on the page (collection stopped for days). The page shows the day of its rows and the catalog window is 7 days; a stale label is a separate design with the runner status.
4. Deploying the pyside service, the `.env` line and the first real run: the owner's work (`docs/deploy/pyside.md`), not an item.
5. An end-to-end check against a real database in CI beyond the two `pg` tests: needs seeded option rows and the roles; left to a later design.
