# readings, step 6 of the storage plan: one row per (figure, version, subject, day), a rerun replaces a differing value, the page view becomes a plain select

Author: the optdesk designer (draft)
Owner approved: Full answer is kept with the dispatcher (not in the repository).
1. Gap (fallback to live data is not in step 6; a forced rerun cannot blank a stale ok): not adopted, stays in Left out with the one owner question; the missing-over-ok rule is Decision 4 and the owner is asked in Left out.
2. Risk (a skipped write still counts usable; the warning fires on every rerun): adopted, entry 3 item 2 now skips the call and logs once.
3. Simpler (change `InsertReadings` itself, no new package, no entries 3 to 5): not adopted: it edits the comment of `internal/store/api.go` (an interface change at the 800 ceiling), loses the written and skipped counts, and lets a code merge before the migration run silently; the optional method follows the series and instrument designs. Entries 3 to 5 are small and can be dropped only together with 1 and 2.
4. Lock-in (`rev` as a counter, the optional-method pattern, UPDATE and DELETE grants, no undo): already in Decisions 1, 5 and 9 and Left out; the dataset text mismatch is named in Left out.
5. False claim (Decision 8, "a changed reading is not stored ... as before"): adopted, the sentence is corrected: a repair in the window is lost until a forced rerun. "No reader reads an old rev" holds for Go code (grep of `readings.reading` and `rd.reading_r1`); the pyside side was not checked, named in the report.
6. Slip (a code entry before the migration run; fakes without the method; delete order): adopted: delete order is in Decision 3; entry 3 lists the three fakes; the Order lines of 3 to 5 say merge after the window ran the migration, the only guard there is.
Date: 2026-11-13
Revision: 1

Source: owner 2026-11-13 S1 to S3 ("overwrite by key, a re-run replaces", "key = the data's own date, the capture date is a column", "use the new one, keep no old"), "storage plan v2 OK" (07:11Z); storage plan v2 of the advisor, P6 point 2 ("rd.reading_r1 ranks the whole table with a window function ... a plain select"), section 3 ("readings.reading, key without rev"), cut order step 6. The precedents are `data-repair-fx.md` (landed), `data-repair-bars.md` and the drafts `series-registry.md` (step 4) and `instrument-company.md` (step 5): column grants, an optional upsert interface that the job asserts at `Run`, `CREATE OR REPLACE VIEW`, and the any-day console row with Force are copied, not re-argued.

Goal: `readings.reading` and its three child tables are keyed without `rev`; a run replaces the stored reading of a (figure, version, subject, day) when it differs and leaves it when it equals; `rd.reading_r1` is a plain select with the same sixteen columns. `compute_figures` gets Force on its console row. For the owner: a repaired input and a forced rerun now show the repaired number (today a rerun adds a row that only a window function finds); no page, page schema or file of `web/` changes.

## Today

All paths at origin/main 2d62da95.

- `migrations/0013_readings.sql`: `readings.reading` key `(name, version, subject_kind, subject, day, rev)`; three children with the same key plus `ord`, `(ord, key_ord)` and `param`, each with a foreign key to its parent; `rd.reading_r1` ranks with `row_number() OVER (PARTITION BY name, version, subject_kind, subject, day ORDER BY (state = 'ok') DESC, rev DESC)`; `0016` adds `rd.reading_parameter_r1` over the child `readings.reading_parameter` (it names the column `rev`). `ot_w_readings` has INSERT and SELECT on the four tables and SELECT on `rd.reading_r1`; `ot_ro_pages` has SELECT on both views.
- `internal/store/reading/writer.go`: `InsertReadings` is `ON CONFLICT DO NOTHING` per key, children only for a new row. `reader.go` `LatestReadings` takes `DISTINCT ON (day)` ordered by version and rev. `store.ReadingWriter`, `ReadingReader`, `ReadingStore`, `ReadingViewReader` are in `internal/store/api.go`, which stands at 800 lines and may not grow.
- Three writers make revisions: `computefigures/steps.go:263-277` and `valuation.go:366` (read the newest rev, skip an equal state, reason and `rows_sha256`, else write `Rev = prev + 1`); `ivhistory/run.go:105` (`add` keeps a stored ok, replaces a stored missing by an ok at the next rev); `cmd/ot/approve.go:79` (owner command, `rev = prev + 1`). Every reader of values goes through `LatestReadings` or the view; none reads an old rev.
- `cmd/ot/jobconsole_jobs.go:82` `anyDay("compute_figures", "revision_rows", textRevisionRows)`: any day, no Force.
- A rerun that finds the input bad writes `missing_input` at the next rev; the winner rule hides it behind an earlier `ok` (`steps.go:271-275` logs a warning).

## Decisions

1. Key (plan section 3, owner S2). `readings.reading` key `(name, version, subject_kind, subject, day)`: the day is the data's own day, the capture time stays in `recorded_at` and the run in `job_run_id`. No column is added: `state`, `origin`, `recorded_at`, `job_run_id` already are the four columns of the plan. `rev` stays a plain `integer >= 1` column ("how many times this key changed"); `contracts.ReadingRecord.Rev` and the dataset text are unchanged, so `Contracts: unchanged`. A version is part of the key: a new algorithm version is a new row beside the old one, as today.
2. Children. `reading_input`, `reading_input_key`, `reading_parameter` keep their columns, including `rev` (a plain column, equal to the parent's), so `rd.reading_parameter_r1` is not touched. Their keys lose `rev`: `(parent key, ord)`, `(parent key, ord, key_ord)`, `(parent key, param)`; their foreign keys point at the parent's new key. When a reading is replaced its children are deleted and written again in the same transaction.
3. Migration `0045_reading_key.sql` (Up and Down, owner `ot_owner`). Up, in this order: delete `reading_input_key`, then `reading_input` and `reading_parameter`, then the readings that are not the winner of their key, by the rule of the old view (the highest `rev` of state `ok`, else the highest `rev`), so the page shows the same numbers before and after; drop the three foreign keys (their names are generated, the migration finds them in `pg_constraint`) and the four primary keys; add the four new primary keys and three foreign keys; `CREATE OR REPLACE VIEW rd.reading_r1` as `SELECT` of the sixteen columns from `readings.reading` with no `WHERE` (keeps owner and the grants to `ot_w_readings` and `ot_ro_pages`; no `DROP VIEW` in Up); the grants of Decision 5. Old revisions are not kept (owner S1). Down restores the old keys and the old view; the deleted rows do not come back.
4. The write. `UpsertReadings` runs in one transaction; per reading `INSERT ... ON CONFLICT (name, version, subject_kind, subject, day) DO UPDATE SET value, unit, currency, state, reason_code, origin, cutoff, rows_sha256, rev, recorded_at, job_run_id WHERE (stored value, unit, currency, state, reason_code, origin, cutoff, rows_sha256, rev) IS DISTINCT FROM (the new ones) AND NOT (stored state = 'ok' AND new state = 'missing_input') RETURNING 1`. A returned row is `Written`: its children are deleted and written again. A row not returned is `Skipped`: it equals the stored one, or it is a `missing_input` that yields to a stored `ok` (the rule of the old view, now kept in the write, and the same as "a no-value row never replaces a present one" in `series-registry.md`). `recorded_at` and `job_run_id` are not compared, or every run would differ. A batch that holds one key twice is a Permanent error before any statement. The old `InsertReadings` stays and is `ON CONFLICT DO NOTHING` on the new key: it adds no row for a stored day (Decision 8).
5. Roles. `ot_w_readings` keeps INSERT and SELECT and gets `UPDATE` on the eleven columns of Decision 4 (not on a key column) of `readings.reading`, and `DELETE` on the three children. No `DELETE` on `readings.reading`, no TRUNCATE. No new role or DSN.
6. The jobs. `computefigures` (both writes) and `ivhistory` assert the store to `readingkey.ReadingUpserter` at `Run` and return a Retryable error naming the missing method when it is absent (never a silent insert-only fallback). Their reading, skip and rev rules stay; the write changes from `InsertReadings` to `UpsertReadings` with `writtenBy` empty (the reading carries `job_run_id`). The `compute_figures` warning for a missing reading over an ok now says the ok was kept. `approve-band` (`cmd/ot/approve.go`) asserts the same and a second approval of a day replaces the first.
7. Console. `compute_figures` becomes `{ID: "compute_figures", Kind: kindAnyDay, Force: true}`: a forced past day recomputes from the stored inputs and replaces a differing reading; an equal one is not written. `textRevisionRows` and the reason `revision_rows` are no longer used by any row after this entry and are removed with it. `iv_history` stays force-no (`not_proved`).
8. Deploy. Entry 2 carries the migration and holds the auto-deploy; the window runs it after the merge (rebuild `ot`, `ot setup pg`, migrate, `ot setup pg`). Entries 3 to 5 merge after the window ran it: between the migration and them a changed reading is not stored (old insert on a stored key), so a repair of that window is lost, not stored; the jobs run daily and the window runs the migration in the same hour as the merge of entry 2, and a reading of a day not yet stored is unaffected. A forced rerun after entries 3 to 5 repairs it.
9. No old value is kept (owner S1): a wrong forced overwrite is repaired only by another run from inputs that are right. A forced rerun whose input is now missing does not blank a stored `ok` (Decision 4); see the owner question under Left out.

## Modules

`docs/design/modules.md`, row `store`: the entry cell gains `internal/store/readingkey` (declarations only, like `internal/store/bar`). Entries touch store (`internal/store/**`, `migrations/**`, `docs/contracts/store.md`; entries 1, 2), jobs (`internal/jobs/**`, `docs/contracts/jobs/**`; entries 3, 4) and ot (`cmd/**`; entries 5, 6). New import edges inside the rule of `modules.md` line 51: `internal/jobs/computefigures`, `internal/jobs/ivhistory` and `cmd/ot` import `internal/store/readingkey`; `internal/store/reading` imports it.

## Interface

One interface change, entry 1, the new declarations-only package `internal/store/readingkey` (`api.go`, `api_test.go`; additive, nothing in `internal/store/api.go` changes):

    // WriteCount is what one UpsertReadings call did: Written readings were inserted or replaced, Skipped ones
    // equalled the stored reading or were a missing_input that yields to a stored ok.
    type WriteCount struct{ Written, Skipped int }

    // ReadingUpserter writes readings by the key (Name, Version, SubjectKind, Subject, Day): a differing reading
    // replaces the stored one with its inputs, keys and parameters, an equal one changes nothing, a missing_input
    // never replaces an ok, no old value is kept. A batch with one key twice is a Permanent error. It is an optional
    // method of the value store.OpenReadingFunc returns; the caller asserts it.
    type ReadingUpserter interface {
        UpsertReadings(ctx context.Context, rows []contracts.ReadingRecord) (WriteCount, error)
    }

`store.ReadingWriter`, `ReadingReader`, `ReadingStore`, `ReadingViewReader`, `OpenReadingFunc` and `contracts.ReadingRecord` are unchanged: no fake breaks.

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. Entry 2 holds the auto-deploy (migration); entries 3 to 5 merge after the window ran it (Decision 8). `Contracts: unchanged` (no file under `contracts/schemas`, `contracts/datasets` or `contracts/proto`). Test runs are in `## Checked by CI`.

### 1 readings-api (interface)

Module: store. Size: S.
Order: first (design PR: after the instrument-company design merge, which edits the same modules.md line).
Files: `internal/store/readingkey/api.go`, `internal/store/readingkey/api_test.go`, `docs/design/modules.md`
Design: docs/design/readings.md#Interface
Acceptance:
1. `api.go` declares `WriteCount` and `ReadingUpserter` as in the Interface section, with docs and no function body, and is the only non-test `.go` file of the package.
2. `git diff BASE -- docs/design/modules.md` shows one changed line, the `store` row, with `internal/store/readingkey` added to its entry cell and nothing else changed; `git diff --name-only BASE -- internal/store/api.go internal/store/api_test.go` prints nothing.
3. `api_test.go` has a new contract test `TestReadingUpserterContract` (`t.Parallel()`): a fake counts a written and a skipped reading and the caller reads both back, an empty list returns zero counts, and a batch with one key twice returns a Permanent error.
4. `git diff --name-only BASE` lists only paths of the Files line.

### 2 readings-store

Module: store. Size: L.
Order: after 1; after instrument-company entry 2 (migration `0044`, and both edit `docs/contracts/store.md` and `setup.go`), which itself comes after series-registry entry 2 (`0043`), strategy-framework entry 5 (`0042`) and data-repair-bars entry 2 (`0041`); this entry takes `0045` and rebases on them. Holds the auto-deploy: it carries a migration (Decision 8).
Files: `migrations/0045_reading_key.sql`, `internal/store/reading/upsert.go`, `internal/store/reading/upsert_test.go`, `internal/store/reading/upsert_pg_test.go`, `internal/store/reading/pg_test.go`, `internal/store/reading/pg_view_test.go`, `internal/store/setup/setup.go`, `docs/contracts/store.md`
Design: docs/design/readings.md#Issues (Decisions 1 to 5, 8, 9)
Acceptance:
1. `0045_reading_key.sql` has an Up and a Down part, is owned by `ot_owner`, and in Up, in this order: deletes the child rows and then the `readings.reading` rows that are not the winner of their `(name, version, subject_kind, subject, day)` by the rule of Decision 3; drops the three foreign keys (found in `pg_constraint`) and the four primary keys; adds the primary keys of Decisions 1 and 2 and the three foreign keys; changes the view with `CREATE OR REPLACE VIEW rd.reading_r1` (the sixteen columns of `0013_readings.sql`, no `row_number`, no `WHERE`; `git grep -n "DROP VIEW" migrations/0045_reading_key.sql` finds only the Down part; no new owner or grant line for the view); grants of Decision 5. `git diff --name-only BASE -- migrations` lists only `0045_reading_key.sql`, so `0016` and the view `rd.reading_parameter_r1` are untouched.
2. `upsert.go` holds the statement of Decision 4 with `ON CONFLICT (name, version, subject_kind, subject, day) DO UPDATE ... WHERE ... IS DISTINCT FROM ... RETURNING 1` and the method `UpsertReadings` on the store value; the children of a written reading are deleted and written again in the same transaction; an empty list makes no call; a key twice is a Permanent error before any statement. `git diff BASE -- internal/store/reading/writer.go internal/store/reading/reader.go internal/store/reading/view.go` is empty, and `git diff --name-only BASE -- internal/store/api.go` prints nothing.
3. `upsert_test.go` (unit, fake database) has new tests: one transaction and one statement per reading with the children deleted and written for a written one only; the duplicate-key error; the empty list.
4. `upsert_pg_test.go` (tag `pg`, new tests) applies `0013`, `0016` and `0045` to a fresh database (it names the files, as `pg_test.go` does) and shows, as the role `ot_w_readings` with the grants alone: a differing reading replaces (1 written, children replaced, `rev` as sent); an equal reading is skipped; a `missing_input` does not replace an `ok` and an `ok` replaces a `missing_input`; the old `InsertReadings` adds no row for a stored key; the migration keeps per key only the winner of the old rule, tested with rows at rev 1 to 3 including an `ok` below a `missing_input`, and its children; the primary keys have no `rev` (a catalog query); the role cannot DELETE from `readings.reading` or UPDATE `name` (named in the test as denied); `ot_ro_pages` reads `rd.reading_r1` and `rd.reading_parameter_r1` after `0045` and gets the same rows as before it.
5. `pg_test.go` and `pg_view_test.go` are changed only where they insert two revs of one key and expect both or a winner by window function; their other assertions stay. `setup.go` changes only comments of the role `ot_w_readings` if any (`git diff BASE -- internal/store/setup/setup.go` shows comment lines only); `docs/contracts/store.md` states the new keys, the plain view, the grants and `0045` (`git grep -n "0045" docs/contracts/store.md` finds it) and no longer says "insert-only".
6. `git diff --name-only BASE` lists only paths of the Files line; every new source file is under 600 lines.

### 3 readings-computefigures

Module: jobs. Size: M.
Order: after 1; merge after the window ran the migration of entry 2.
Files: `internal/jobs/computefigures/steps.go`, `internal/jobs/computefigures/valuation.go`, `internal/jobs/computefigures/job.go`, `internal/jobs/computefigures/job_test.go`, `internal/jobs/computefigures/types_test.go`, `internal/jobs/computefigures/ivinput_test.go`, `docs/contracts/jobs/compute_figures.md`
Design: docs/design/readings.md#Issues (Decision 6)
Acceptance:
1. `Run` asserts `stores.Readings` to `readingkey.ReadingUpserter` and returns a Retryable error naming the missing upsert when it is absent; both write places (`steps.go` and `valuation.go`) call `UpsertReadings`; `git grep -n "InsertReadings" internal/jobs/computefigures` finds only test files.
2. The rev, skip and `rows_sha256` rules of `steps.go` and `valuation.go` are unchanged except one: when the stored reading is `ok` and the new one is `missing_input`, the job makes no upsert call and logs once that the ok was kept (a new test), so no run bumps `rev` for a write the store would skip; the run still counts the day as usable.
3. `job_test.go` has new tests: a store without the upsert method makes `Run` return a Retryable error; a fake upserter receives the reading and the valuation reading with the rev the job computed; a second run with equal input makes no upsert call. `types_test.go` and `ivinput_test.go` fakes gain the method.
4. `docs/contracts/jobs/compute_figures.md` states that readings are written by key and replaced when they differ.
5. `git diff --name-only BASE` lists only paths of the Files line.

### 4 readings-ivhistory

Module: jobs. Size: S.
Order: after 1; merge after the window ran the migration of entry 2; parallel with 3.
Files: `internal/jobs/ivhistory/run.go`, `internal/jobs/ivhistory/job_test.go`, `internal/jobs/ivhistory/run_test.go`, `docs/contracts/jobs/iv_history.md`
Design: docs/design/readings.md#Issues (Decision 6)
Acceptance:
1. `run.go` asserts the writer to `readingkey.ReadingUpserter` at `Run`, returns a Retryable error naming the missing upsert when it is absent, and writes the rows of the run in one `UpsertReadings` call; `git grep -n "InsertReadings" internal/jobs/ivhistory/run.go` finds nothing; `add` and `New` are unchanged.
2. `run_test.go` has new tests: a writer without the method makes `Run` return a Retryable error; a stored missing reading of the day is replaced by an ok one in the fake upserter's call; a stored ok is not sent again. `job_test.go` fake gains the method.
3. `docs/contracts/jobs/iv_history.md` states the upsert.
4. `git diff --name-only BASE` lists only paths of the Files line.

### 5 readings-approve

Module: ot. Size: S.
Order: after 1; merge after the window ran the migration of entry 2.
Files: `cmd/ot/approve.go`, `cmd/ot/approve_test.go`, `cmd/ot/wire_ivhistory_test.go`
Design: docs/design/readings.md#Issues (Decision 6)
Acceptance:
1. `approve.go` asserts the opened store to `readingkey.ReadingUpserter` and writes the approval with `UpsertReadings`; a store without it is an error text that names the method and writes nothing; `git grep -n "InsertReadings" cmd/ot/approve.go` finds nothing.
2. `approve_test.go` has new tests: a second approval of the same day sends the next `rev` to the upserter and the printed line shows it; a store without the method writes nothing and the command fails. `wire_ivhistory_test.go` fake gains the method.
3. `git diff --name-only BASE` lists only paths of the Files line.

### 6 readings-console

Module: ot. Size: S.
Order: after 3; after series-registry entry 5 (same table, same pinning tests, it comes after data-repair-bars entry 4).
Files: `cmd/ot/jobconsole_jobs.go`, `cmd/ot/jobconsole_jobs_test.go`, `cmd/ot/jobconsole_post_test.go`, `cmd/ot/jobconsole_site_test.go`
Design: docs/design/readings.md#Issues (Decision 7)
Acceptance:
1. `jobTable` has `{ID: "compute_figures", Kind: kindAnyDay, Force: true}` in place of `anyDay("compute_figures", "revision_rows", textRevisionRows)`; `textRevisionRows` is removed because no row uses it (`git grep -n "textRevisionRows" cmd/ot` finds nothing), and the reason code `revision_rows` is removed from the code list of `jobconsole_jobs_test.go` if the list names only reasons in use.
2. The numbers after `series-registry.md` entry 5 (31 rows: 7 any-day, 16 today-only, 8 listed-only; force-yes `calendar_refresh`, `fx_refresh`, `bars_refresh`, `fred_series`; 19 force-no) become: 31 rows, 7 any-day (`compute_figures` is already in the set), 16 today-only, 8 listed-only; force-yes five (those four and `compute_figures`); force-no 18. `jobconsole_jobs_test.go` pins them: the force-yes test of series entry 5 lists the five ids and says so in its name, `TestEveryForceNoJobHasReason` expects 18; `TestAnyDayIsExactlyTheSeven`, `TestTableIsTheWiredJobSet` (31) and `TestListedOnlyIsExactlyTheEight` are unchanged.
3. `jobconsole_site_test.go` expects the `compute_figures` row with `Force` true and no `ForceWhy`; `jobconsole_post_test.go` has a new test that a Force post of a past day to `compute_figures` is accepted (it is refused today as force-no); the last line of `TestFxRefreshRunValidatesPastDayWithForce` that series entry 5 left naming `compute_figures` as the force-no example names `account_capture`, and the test keeps its other lines.
4. A new test in `jobconsole_post_test.go` shows that the accepted forced post runs the job for that past day with its force flag set (as the fx test does).
5. `git diff --name-only BASE` lists only paths of the Files line.

## Checked by CI

`go test ./internal/store/... ./internal/jobs/computefigures/... ./internal/jobs/ivhistory/... ./cmd/ot/... -race -count=1` (the `pg` tests of entry 2 with `0045` applied to a fresh database run only in GitHub CI), `go run ./scripts/arch --check`, `go build ./...`, `golangci-lint`, and the contracts generation check (no change expected). Results only a run shows: the upsert counts as the role `ot_w_readings` with the column grants alone; the migration keeps the winner of the old rule per key with its children; the old insert on the migrated table; the two views return the same rows before and after `0045`.

## Left out

- Fundamentals, dividends and earnings keys (step 7), option captures (step 8), source rank and the void mark (step 9). The reading rows have no `void` state here: the four-column rule of the plan (source, status, updated_at, batch) is met by `origin`, `state`, `recorded_at`, `job_run_id`.
- Fallback to live data when the stored input of a chained job is bad (owner S1, "fallback, not cascade"): `compute_figures` reads stored bars and series and writes `missing_input` when they are bad. Making it collect live data is a change of that job's inputs, not of the readings key, and is not in step 6 of the plan. Owner question: should `compute_figures` fetch the input live when the stored one is missing, as its own step after this design (a/b: yes, no)?
- Force for `iv_history` (it stays `not_proved`) and any console row for `approve-band`.
- A forced rerun cannot replace a stored `ok` by `missing_input` (Decision 4). If a wrong `ok` must be blanked, that is a delete or a void mark, not built here.
- `contracts/datasets/readings.reading.r1.json` and its generated text still say "Primary key: (..., rev)" and "a later rev replaces an earlier one": a change of a file under `contracts/datasets` with its generated output, owner-reviewed; the database key and the dataset text differ until then.
- Dropping `rev` and the child `rev` columns, or `reading_input` tables themselves: contract changes.
- Recomputing readings after a replaced input (series or bars): a forced `compute_figures` day does it, nothing triggers it.

## Doubts

1. `recorded_at` and `job_run_id` are not compared in the upsert (Decision 4): two runs with equal values leave the first run's id. A reading whose children differ with the same `rows_sha256` and `rev` would be skipped; today's jobs bump `rev` whenever inputs or parameters differ (`valuation.go` compares the parameter text), and the pg test of entry 2 is the check.
2. The migration number `0045` assumes `0041` to `0044` land first; another migration that lands first moves the number and the file lists of entry 2.
3. The grants follow the PostgreSQL rules for `ON CONFLICT ... DO UPDATE` (UPDATE on the set columns, SELECT on the columns of the conflict target and the WHERE) as in the FX and series entries; no database was available here, the `pg` tests are the check.
4. Row counts and size of the online `readings.reading` were not read; the delete of Up is a single statement per table and runs while the auto-deploy is held. Back up first (plan section 5).
5. `docs/contracts/store.md` is also edited by series-registry entry 2 and instrument-company entry 2: the Order line of entry 2 puts this one last.

## Adversarial

Full answer is kept with the dispatcher (not in the repository).
1. Gap (fallback to live data is not in step 6; a forced rerun cannot blank a stale ok): not adopted, stays in Left out with the one owner question; the missing-over-ok rule is Decision 4 and the owner is asked in Left out.
2. Risk (a skipped write still counts usable; the warning fires on every rerun): adopted, entry 3 item 2 now skips the call and logs once.
3. Simpler (change `InsertReadings` itself, no new package, no entries 3 to 5): not adopted: it edits the comment of `internal/store/api.go` (an interface change at the 800 ceiling), loses the written and skipped counts, and lets a code merge before the migration run silently; the optional method follows the series and instrument designs. Entries 3 to 5 are small and can be dropped only together with 1 and 2.
4. Lock-in (`rev` as a counter, the optional-method pattern, UPDATE and DELETE grants, no undo): already in Decisions 1, 5 and 9 and Left out; the dataset text mismatch is named in Left out.
5. False claim (Decision 8, "a changed reading is not stored ... as before"): adopted, the sentence is corrected: a repair in the window is lost until a forced rerun. "No reader reads an old rev" holds for Go code (grep of `readings.reading` and `rd.reading_r1`); the pyside side was not checked, named in the report.
6. Slip (a code entry before the migration run; fakes without the method; delete order): adopted: delete order is in Decision 3; entry 3 lists the three fakes; the Order lines of 3 to 5 say merge after the window ran the migration, the only guard there is.
