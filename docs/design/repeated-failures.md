# repeated failures: the data health page lists, per dataset, the keys that keep failing or keep coming back empty

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-19

Source: outcome audit 2026-11-18, findings 9 and 10, plan item P1; issue #3083 (the `empty` outcome in the run log). Owner-visible result: on the data health page, under each dataset, the owner sees the keys that failed (or came back empty) on 3 or more distinct session days of the last 14 session days and whose newest run is still not ok: job, key, number of days, number of runs, first and last day, last error class. A key that returned rows but kept none is marked `empty`, so a run with usable 0 no longer looks like success there. Retry, close and the coverage cells do not change.

Read on origin/main at d4d59e7f: `AGENTS.md`, `docs/design/modules.md`, `docs/design/data-health.md`, `docs/design/option-eod-speed.md`, `internal/runner/reconcile/runkey.go`, `internal/store/api.go`, `internal/store/coverage/api.go`, `internal/store/ops/coverage.go`, `migrations/0003`, `0036`, `0051`, `internal/pages/data_health_page/{builder,page,strings,cells}.go`, `internal/pages/data_health_page/builder_test.go`, `cmd/ot/data_health_site_test.go`, `cmd/ot/english_test.go`, `contracts/schemas/page.data_health_page.v1.json`, `web/package.json`, `web/scripts/generate-types.mjs`.

## What a run row holds (fact, not a decision)

A row of `ops.capture_run` (`migrations/0003_ops_ledger.sql:5-17`, `store.Run` at `internal/store/api.go:282-291`) holds: `session_day`, `dataset_id`, `subject_key`, `status` (`running`, `done`, `partial`, `abandoned`; `internal/store/api.go:359-371`), `finished_at`, `returned_n`, `usable_n`, `error_code` and `reason_code`; every retry is a new row (`run_id` is the primary key). `error_code` is a `contracts.ErrorKind` (`retryable`, `permanent`, `unknown_calendar`; `internal/contracts/api.go:45-48`) and is set only when the job returned an error that is not `no_value` (`internal/runner/reconcile/runkey.go:232-236`). The text of the error is NOT stored: it goes to the log only, redacted (`runkey.go:215-219`). The row has no job id: the job is found from the dataset by the page (`builder.go:111-120`, the first job that lists the dataset; `JobID` is a `*string`, nil when none does). The `empty` outcome is derived in the log from the row (`runkey.go:196-202`: done, no error code, `returned_n > 0`, `usable_n = 0`), so the row side is reachable with no new column. The view `rd.capture_run_r1` (`migrations/0036_option_list_band_run_view.sql:30-32`) passes every one of those columns and is already granted to `ot_ro_pages` (0036:47), so no migration is needed. `CloseRunning` (`internal/store/ops/ledger.go:214`) closes a run that was killed by writing status `done`, no error code and `returned_n` 0.

## Decisions

1. A failed run is a row with `status = 'done'` and `error_code <> ''`; an empty run is a row with `status = 'done'`, `error_code = ''`, `returned_n > 0` and `usable_n = 0` (the rule of `runkey.go:196-202`). Rows of status `partial`, `abandoned` and `running` are not counted as runs of either kind, and are not looked at when the newest run is chosen.
2. A key (dataset and subject key) is listed in a kind when (a) it has runs of that kind on 3 or more distinct session days (`coverage.FailureMinDays`) among the last 14 session days of its dataset's calendar (`coverage.FailureWindowDays`, the only place they are written), AND (b) its newest done run in that window, by `finished_at`, is failed or empty, not ok: a key that has healed drops off at once. The count is of distinct days, not of runs: three retries on one afternoon are one day. The kinds stay separate: a key with 3 failed days and 3 empty days is two rows. The window is pinned by the builder: it already has the window days of each calendar (`windowsByCalendar`, `builder.go:66-95`, last `coverage.WindowSessions` days of the calendar); it takes the last `FailureWindowDays` of them, calls the reader once for each calendar a dataset with a cell uses, with `from` = the first of those days and `to` = `Today`, and keeps only the rows of the datasets of that calendar.
3. Limit of the `empty` kind, stated plainly: a run of a job that is judged by a `runner.Postcondition` is marked present whatever `usable_n` is (`runkey.go:239-246`), so a healthy key of such a job can have `returned_n > 0` and `usable_n = 0` and would be listed as `empty`. Neither the view nor the page can know which jobs those are: `runner.Postcondition` is a runner interface, pages does not import runner, and a flag on the page `Job` would edit `cmd/ot`. So the page does not hide them; the `class_text` of `empty` says that rows came back and none was kept, that it is normal for a source with nothing new and for a job that checks its own result, and that the key is only worth a look when it is also a gap in the grid. A finer rule (a per-job flag, or the present mark joined on the day) is a follow-up that needs `cmd/ot` or a view; it is left out below. Runs closed by `CloseRunning` (done, no error, `returned_n` 0) are neither failed nor empty and are not listed: a key killed by every restart shows in the grid as `expired`, not here, and the `failures_none_text` says so.
4. A row carries dataset, key, kind (`failed` or `empty`), number of distinct days, number of runs of that kind, first and last `session_day` of those runs, and the class of the newest done run of the key: its `error_code`, or `empty` for an empty run. No error text is shown, because none is stored (the log keeps it; storing it is a new column and is left out).
5. The store reads the view with one aggregating query (`rd.capture_run_r1`, grouped by dataset, key and kind; the newest done run chosen by `finished_at`, then `run_id`). It is a new method on a new interface `coverage.FailureReader`, not on `coverage.Reader`, so the fakes of `cmd/ot` and `base_data_page` tests (`coverage.Reader` implementers, `internal/pages/base_data_page/builder_test.go`, `cmd/ot/data_health_site_test.go`) still compile. The pg reader (`ops.coverageReader`, `internal/store/ops/coverage.go:24`) implements both. The page reaches the new one by a type assertion on `in.Cov`, made in `datasetRows` (`builder.go:41-63`), which `Build` (`builder.go:24-37`) calls; so `cmd/ot` is not touched. This is the seam the code already uses for `job.(runner.Postcondition)` (`runkey.go:239`). A reader that does not implement it is a named gap (Decision 7), never an empty list.
6. The builder orders a dataset's rows by days descending, runs descending, last day descending, key ascending, keeps the first 50, and writes `failures_total` (all qualifying keys), `failures_hidden` (`failures_total` minus the rows kept) and `failures_hidden_text` (the Chinese sentence that says how many are not shown, empty when none). A row of a dataset that has no cell on the page is dropped. The job column is the dataset's `job_id` the page already has (`page.go:27-36`); it can be null, and the web then shows `-`.
7. If the reader does not implement `coverage.FailureReader`, or the read fails, the page still shows the cells, with the new gap reason `failures_unavailable` and its text (`not implemented` and `read failed` are the same reason with the same text: the owner cannot act on the difference); the datasets show an empty list. The existing `coverage_unavailable` gap is unchanged (`builder.go:30-35`).
8. Every text the page prints has a field in the document and is Chinese hint text from the Go side: the top-level `failures_note` (what the table is, the 3 days in 14 rule and the newest-run rule), `failures_none_text` (shown for a dataset with no row; it says no key met the rule and that killed or cut runs are not counted here but show as `expired` in the grid), the per-dataset `failures_hidden_text`, the text of the gap `failures_unavailable`, and a `class_text` on every row (what `retryable`, `permanent`, `unknown_calendar` or `empty` means). The strings live in a new file `internal/pages/data_health_page/zh_texts.go` that holds string constants only (the one door of `cmd/ot/english_test.go:28-35`, `isZhTextsDoor`); `strings.go` stays English. Column names and labels are English in `web/src`. The web computes nothing: no count, no sum, no sort, no filter beyond what the table component offers on the given rows; it prints `failures_hidden_text` and never subtracts.
9. Schema version: the fields are added as OPTIONAL properties of `page.data_health_page.v1` and `schema_version` stays 1. This amends Decision 14 of `docs/design/data-health.md`, which says any change is a new version: an additive optional field is allowed because the page and the web ship together, so no reader of the page exists that this could break. The view `rd.coverage_day_r1` and the enums of Decision 14 are not changed by it. The design PR appends the sentence given in the next section to Decision 14. Entry 3 adds the fields not in `required`; entry 4 fills them on every document, and "always present" is an acceptance item of entry 4 (the schema is not tightened afterwards: a later tightening is a `v2` question).
10. Nothing is written and nothing in retry or close changes: the new code only reads; `internal/runner/**` and the scheduler are not touched, and no listed key is blocked, skipped or closed because it is listed (owner rule: our own guards never make data unrecoverable).

### Amendment to Decision 14 of docs/design/data-health.md

Appended to the end of Decision 14, as it is applied in the worktree:

> Amended by `docs/design/repeated-failures.md` (Decision 9): an optional field may be added to `page.data_health_page.v1` without a new version, because the page and the web ship together; `schema_version` stays 1, and the view and the enums above are unchanged.

## Modules

`docs/design/modules.md`: unchanged. First row that matches decides (checked against origin/main): store (`internal/store/coverage`, `internal/store/ops`, `docs/contracts/store/**`); contracts (`contracts/**` including `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/**`, `web/src/generated/**`, `docs/contracts/page-schemas.md`); pages (`internal/pages/**`, `docs/contracts/pages.md`); web (`web/**`). Every path in a Files line below exists at origin/main except the new files, and each entry stays in one module. `docs/contracts/ot/pages/data_health_page.md` matches only `docs/**` (module docs): no entry edits it (Left out). `docs/design/data-health.md` is edited by the design PR itself, not by an entry. No new module and no dependency edge: pages already depends on store and contracts, web on contracts.

## Interface

Owner: store, file `internal/store/coverage/api.go` (entry 1, `(interface)`). Declarations only:

```go
const FailureWindowDays = 14 // session days of the dataset's calendar
const FailureMinDays = 3     // distinct session days with a run of one kind

type FailureKind string
const (
	FailureFailed FailureKind = "failed"
	FailureEmpty  FailureKind = "empty"
)

// Failure is the runs of one kind of one key inside the window: Days >= FailureMinDays, and the newest done run
// of the key in the window is failed or empty.
type Failure struct {
	Dataset, Key string
	Kind         FailureKind
	Days         int // distinct session days with a run of Kind
	Runs         int // runs of Kind
	FirstDay     contracts.Day
	LastDay      contracts.Day
	LastClass    string // error_code of the newest done run; "empty" when that run was empty
}

// FailureReader reads the keys that meet the rule in from..to (session days, inclusive). Never nil; unordered.
// Implemented by the same value as Reader.
type FailureReader interface {
	RepeatedFailures(ctx context.Context, from, to contracts.Day) ([]Failure, error)
}
```

The page document (schema, entry 3) adds, all optional: top-level `failures_note`, `failures_none_text` (Text); per dataset `failures_total`, `failures_hidden` (Count), `failures_hidden_text` (Text, empty when none hidden) and `repeated_failures` (array of `{key, kind, days, runs, first_day, last_day, last_class, class_text}`); the gap reason enum gains `failures_unavailable`. Entries 4 and 5 read only that schema; the Go view-model types stay inside `internal/pages/data_health_page`.

## Issues

### 1 failure-reader-api (interface)

Declare `FailureReader`, `Failure`, `FailureKind` and the constants in the coverage package, with a contract test and a line in the store contract doc.

Design: Decisions 2, 4, 5.
Contracts: unchanged
Files: internal/store/coverage/api.go, internal/store/coverage/api_test.go, docs/contracts/store/coverage.md
Order: none
Acceptance:
- The `coverage` package declares `FailureReader`, `Failure`, `FailureKind`, `FailureFailed`, `FailureEmpty`, `FailureWindowDays` (14) and `FailureMinDays` (3), and `coverage.Reader` has the same methods as before.
- The contract test has a fake that implements `FailureReader` and a caller that uses it, and a check that a value implementing only `Reader` is accepted where `Reader` is asked (the fakes elsewhere keep compiling).

### 2 failure-reader-pg

Implement `RepeatedFailures` on the ops coverage reader with one query on `rd.capture_run_r1`.

Design: Decisions 1, 2, 3, 4, 5, 10.
Contracts: unchanged
Files: internal/store/ops/failures.go, internal/store/ops/failures_test.go, internal/store/ops/failures_pg_test.go
Order: after 1
Acceptance:
- `OpenCoverage` still returns a value that is a `coverage.Reader` and a `coverage.FailureReader`.
- For a key with done runs with an error code on 3 distinct session days in the window and a newest run that failed, the method returns one row of kind `failed` with Days 3, Runs equal to the number of those runs, the first and last `session_day` and the `error_code` of the newest run; with failed runs on only 2 days it returns none, also when the runs are 5.
- Three failed runs on one session day give no row (Days 1); a key whose newest done run is ok (`usable_n` above 0, no error) gives no row although it has failed runs on 3 days.
- A key with done runs of no error, `returned_n` 5 and `usable_n` 0 on 3 days gives one row of kind `empty` with `LastClass` `empty`; a `partial` run, an `abandoned` run, a `running` run and a done run with `returned_n` 0 are not counted in either kind and are not taken as the newest run.
- A run of `session_day` before `from` or after `to` is not counted; a key with 3 failed days and 3 empty days and a failed or empty newest run gives two rows.
- The query reads only `rd.capture_run_r1` and runs with the page role; no migration is added; a test with a fake `database` checks the arguments and the mapping, and the PostgreSQL test (build tag `pg`) checks the rows above.

### 3 page-schema

Add the repeated failures to the page schema as optional fields, with the fixtures, the manifest, the schema doc and the generated TypeScript type.

Design: Decisions 4, 6, 7, 8, 9.
Contracts: contracts/schemas/page.data_health_page.v1.json changes (additive and optional; `schema_version` stays 1, Decision 9).
Files: contracts/schemas/page.data_health_page.v1.json, contracts/fixtures/page.data_health_page.v1.ok.json, contracts/fixtures/page.data_health_page.v1.gap.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/data_health_page_v1_test.go, docs/contracts/page-schemas.md, web/src/generated/data_health_page.v1.ts
Order: none
Acceptance:
- The new properties (`failures_note`, `failures_none_text` at the top; `failures_total`, `failures_hidden`, `failures_hidden_text`, `repeated_failures` on a dataset) are in `properties` and NOT in `required`; a row of `repeated_failures` requires `key`, `kind` (`failed` or `empty`), `days` (3 or more), `runs` (at least `days`), `first_day`, `last_day`, `last_class` and `class_text`, all with a maximum length, no extra properties.
- The gap reason enum is `coverage_unavailable` and `failures_unavailable`.
- The `ok` fixture holds one dataset with one `failed` row (last class `retryable`) and one `empty` row, and the `gap` fixture holds the `failures_unavailable` gap; both are listed in `MANIFEST.synthetic-v1.json` and validate against the schema, and a row with `days` 2 or without `class_text` does not.
- The documents the current builder makes, which carry none of the new fields, still validate: `internal/pages/data_health_page/builder_test.go` (`validate`, lines 53-57 and `TestBuildLateTextValidatesSchema`) and `cmd/ot/data_health_site_test.go` (`validateDataHealth`, lines 68-80) stay green after this entry alone, and neither file is edited.
- `docs/contracts/page-schemas.md` says in the paragraph of `page.data_health_page.v1` that the fields are optional and added by `docs/design/repeated-failures.md` without a version change.
- `web/src/generated/data_health_page.v1.ts` is regenerated by the web script `generate:types` (`node scripts/generate-types.mjs`, run from `web/`, `json-schema-to-typescript`), not edited by hand, and is committed with the schema; its banner stays `// Code generated from contracts/, DO NOT EDIT.` (`internal/contracts/generated_ts_test.go`).

### 4 page-builder

The builder reads `FailureReader` when the reader has it, fills the new fields on every document and prints the Chinese texts.

Design: Decisions 2, 3, 4, 5, 6, 7, 8, 9, 10.
Contracts: unchanged
Files: internal/pages/data_health_page/failures.go, internal/pages/data_health_page/failures_test.go, internal/pages/data_health_page/zh_texts.go, internal/pages/data_health_page/builder.go, internal/pages/data_health_page/page.go, internal/pages/data_health_page/builder_test.go, docs/contracts/pages.md
Order: after 1, after 3
Acceptance:
- With a reader that gives two keys of one dataset (5 and 3 days), the document lists the 5-day key first, each row with its days, runs, first and last day, last class and a non-empty Chinese `class_text` for `retryable`, `permanent`, `unknown_calendar` and `empty`, and the dataset's `failures_total` is 2 and `failures_hidden` 0.
- With 60 qualifying keys the dataset lists 50, `failures_total` is 60, `failures_hidden` is 10 and `failures_hidden_text` is non-empty; with none hidden `failures_hidden_text` is empty.
- The reader is called once for each calendar the datasets use, with `from` the first of the last 14 window days of that calendar and `to` Today; a row of a dataset of another calendar or without a cell is dropped (a test with two calendars).
- Every document, with or without rows, carries `failures_note`, `failures_none_text` and, on every dataset, `failures_total`, `failures_hidden`, `failures_hidden_text` and `repeated_failures` (an empty list when none): the fields are always present.
- A `coverage.Reader` that does not implement `coverage.FailureReader` gives the gap `failures_unavailable` with its text and the cells unchanged, never empty lists without a gap; a `RepeatedFailures` error gives the same gap.
- The Chinese strings are constants in `zh_texts.go` and nowhere else; `strings.go` stays English and the English guard of `cmd/ot/english_test.go` passes.
- The built document validates against the schema of entry 3 (the page's contract test), and the Go json tags of the new fields equal the schema names (a test that decodes the `ok` fixture into the view-model types).
- `docs/contracts/pages.md` names the new fields and the gap reason.

### 5 web-table

Render the repeated failures of each dataset as a table with the data-table pattern.

Design: Decisions 4, 6, 7, 8.
Contracts: unchanged
Files: web/src/pages/data_health/FailuresTable.tsx, web/src/pages/data_health/DataHealthPage.tsx, web/test/pages/data_health/failures.test.tsx, web/test/pages/data_health/data_health.test.tsx
Order: after 3
Acceptance:
- Under each dataset the page shows a table with the columns job, key, kind, days, runs, first day, last day and last class, one row per `repeated_failures` item, built with the shadcn data-table pattern and `@tanstack/react-table`; the job is the dataset's `job_id` or `-` when it is null; the `class_text` is shown next to the class and `failures_note` once above the tables.
- A dataset with no row shows its `failures_none_text`; with a non-empty `failures_hidden_text` the page prints that text as it is (no number is computed on the page).
- An `empty` row is visibly not the same as a `failed` row (a different label) and nothing on the page is computed from the data beyond what the table does with the given rows; all column names and labels in `web/src` are English.
- A payload without the new fields (they are optional in the type) shows no table and no error; the gap `failures_unavailable` shows its text and the coverage grid is still drawn.
- One test loads `contracts/fixtures/page.data_health_page.v1.ok.json` and renders it, so the table is built from the payload the schema describes.
- `tsc`, `eslint`, `vitest` and the build pass (CI).

## Left out

- The error text (store a redacted text of the failed run): a new column and a writer change in the runner; ask only if the class is not enough.
- A finer `empty` rule that hides the healthy keys of Postcondition jobs (Decision 3): a per-job flag on the page `Job` (edits `cmd/ot`) or the present mark joined on the day (a view); the limit is stated in the `class_text` until then.
- Runs closed by `CloseRunning`, `partial` and `abandoned` runs (Decisions 1 and 3): not counted and not shown; the grid shows such keys as `expired`.
- A "recovered" mark: a key whose newest run is ok is not listed at all (Decision 2).
- A per-key retry or close button; the counts on the home page; changing what the runner counts as `ok` (finding 9 as a runner change).
- Failures of a dataset that has no cell on the page.
- `docs/contracts/ot/pages/data_health_page.md` describes the page and matches only the `docs` row of `docs/design/modules.md`; if it must change, the author of entry 4 reports it in the pull request.

## Adversarial

Full answer of the adversarial read: the dispatcher scratch file `optdesk-drafts/designs/repeated-failures/adversarial-raw.md` (runner home, not in the repository).

1. Gap, runs counted instead of days: adopted. Decision 2 counts distinct session days and shows runs as well; a healed key drops off.
2. Gap, crashed or cut keys never appear: adopted as a statement. Decision 3 and `failures_none_text` say they are not counted and show as `expired`; not counted as rows, because a `CloseRunning` row has no error and no rows.
3. Gap, the window and the calendars: adopted. Decision 2 pins `from`/`to` per calendar from the builder's window days.
4. Biggest risk, red main between entries 3 and 4: adopted. Entry 3 adds the fields as optional and says that `builder_test.go` and `data_health_site_test.go` stay green; entry 4 fills them and "always present" is its acceptance item.
5. Second risk, false `empty` for Postcondition jobs: partly adopted. The page cannot know them (Decision 3), so the limit is stated in the `class_text` and Left out; the newest-run rule removes healed keys. A join on the present mark is not adopted: it needs a view or `cmd/ot`.
6. Simpler way, `runview.RunsBetween` and grouping in the builder: not adopted. The SQL aggregate moves far fewer rows and keeps the grouping out of the builder; the type-assertion seam is kept (precedent `job.(runner.Postcondition)`), made safe by Decision 7: a reader without it is a named gap. The 50-row cap and the kind split are kept: the cap protects the page after an outage, and the kinds mean different things.
7. Lock-in, required fields: adopted (optional, Decision 9). Constants and `Failure` in the interface file: accepted, only this page uses them. "Runs" semantics: adopted by counting days from the first version. Error class only: not adopted; Left out names the new column.
8. Claim 5(a), the job can be null: adopted (Decision 6, entry 5). 5(c), entry 3 "Order: none" red in effect: adopted (point 4). 5(d), the generator is not named: adopted (entry 3 names `generate:types`, `web/scripts/generate-types.mjs`). 5(e), the cite of the assertion: adopted (`Build` is `builder.go:24-37`, `datasetRows` is `builder.go:41-63`, Decision 5).
9. Slip, schema and builder names typed twice: adopted. Entry 4 decodes the `ok` fixture into the view-model types; entry 5 renders the same fixture.

Advisor, points 1-5 and Q1-Q3: the text of the advisor's answer was not handed to this fix round; the dispatcher's rulings made after it are all applied and are the record: schema amendment of Decision 14 (adopted, Decision 9), Chinese texts through `zh_texts.go` (adopted, Decision 8), optional fields in entry 3 (adopted), Files with `MANIFEST.synthetic-v1.json`, `page-schemas.md` and `pages.md` checked against modules.md (adopted), no computing in the web (adopted, Decision 6 and 8), days not runs with the healed rule (adopted, Decision 2), the Postcondition limit (adopted, Decision 3), a named gap for a reader without `FailureReader` (adopted, Decision 7), the seam kept (adopted, Decision 5). A point of the advisor that is none of these is not recorded here.
