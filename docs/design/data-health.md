# data health: a page that shows, for every stored dataset and every recent session day, whether the data is there, and a button that repairs the gaps

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-17

Source: owner proposal `state/optdesk-drafts/designs/data-repair/proposal.md` (approved 2026-11-14), the answer of optdesk-advisor 2026-11-14 ~14:10Z (cut and points P1 to P4), owner word "a" on question 3 (an option-chain day missed past the next open is accepted as lost; the page says the day is lost: the source gives only the present; no degraded fill). Owner principles: every stored dataset is repairable and easy to look at; every job runs alone and twice with the same result; the owner works on the web page, the terminal is a thin wrapper; dev mode has no time guards except one run of the same job at a time; a technical word is explained next to it; the front end shows data and computes nothing. Every fact below was read on origin/main `e18b41bd` (after `git fetch`, 2026-11-17); nothing was run and no database was read.

Goal: one page, route `data-health`. Per dataset a row of the last 30 session days (`coverage.WindowSessions`), each cell one of `ok`, `no_value` (the source answered "nothing for this key"), `expired` (the runner closed the key at its window after a failed, partial or crashed run: a real gap, not an answer), `missing` (the runner expected the key and it is not there) or `not_run` (the runner recorded no plan for that day). A gap day (`missing` or `expired`) has one repair button when its job can run that day, else the cell says why: "lost: the source gives only the present", or, for a `not_run` day, "not planned: repair of an unplanned day is not built yet" (never "lost at the source": our own guard is not a reason to call data unrecoverable). Today's cell before the job's slot says "due at HH:MM" and has no button (Decision 16). Each button names its number of runs. Each dataset row has one button "repair N days" (N = the runs it will start, one per gap day, run one after another). Nothing is stored twice: the page reads the runner's ledger through one new read-only view and queues runs through the queue that exists.

## Today

- The ledger exists and is complete for the question. `ops.capture_expected_key (session_day, plan_hash, dataset_id, subject_key)` is replaced per day by `PutExpected`; `ops.capture_present (session_day, dataset_id, subject_key, status)` holds `present` or `no_value` per key (`internal/store/ops/ledger.go:24-30`, `store.StatusPresent`, `store.StatusNoValue`, `internal/store/api.go:212-213`); `ops.capture_run` holds the runs (migration `0003_ops_ledger.sql`). `ot_ro_pages` has no grant on an `ops` table (0003, 0012); the page role reads views of schema `rd`, owned by `ot_owner`, which run with the owner's rights (`0036_option_list_band_run_view.sql:30-48`, `0028_session_day_view.sql`). Two ledger views are readable today: `rd.job_gap_r1` and `rd.expectation_r1` (0012), both about a dataset window, not a day and a dataset. `rd.capture_run_r1` (0036) gives the runs. What `present` means (read on origin/main): `ops.capture_present` is written only by the key runner, `MarkPresent` after `job.Run` returned and the verdict found usable data (`internal/runner/reconcile/runkey.go:148`), or as `no_value` when a key is closed at its window after failed, partial or crashed runs (`closeRow` `reconcile.go:432-462`, `expireIfClosed` `:466-486`, run reasons `expired` `:38` and `crashed_repeatedly` `:40`); that second kind is the state `expired`, not an answer of the source (Decision 3). It is the runner's record that the data was stored in that turn, not a count of data rows. The view cannot count data rows of every dataset (each has its own table and key shape), so the page says "ok = the runner recorded the key as stored" in its legend; a table restored or pruned outside the runner leaves the ledger green and is out of scope (Left out).
- The queue exists. `settings.job_request` (0035) takes one row per job, day and force; `jobrequest.Poster.Enqueue` stores it unless an open one of the same job, day and force exists (`internal/store/jobrequest/api.go`); the scheduler's worker runs a request as one turn, so requests run one at a time (`docs/design/job-console.md#Decisions` 1 and 8). `POST /owner-job-console` (`cmd/ot/jobconsole_post.go`, route `jobConsolePostRoute` in `cmd/ot/jobconsole_site.go:20`) validates `job`, `day`, `force` against `jobTable` (`cmd/ot/jobconsole_jobs.go:75-111`, 33 rows) and enqueues. The command line is already a thin wrapper of the same queue: `ot backfill --day --job --force` calls `enqueue` with `buildRequests` (`cmd/ot/backfill.go:44`, `cmd/ot/enqueue_wait.go:41,77`).
- The console rows: any-day with force: `fred_series`, `fundamentals`, `compute_figures`, `option_candidates`, `book_risk`, `earnings_refresh`, `dividend_capture`, `fx_refresh`, `bars_refresh`, `macro_events`; today-only with force: `option_eod`, `option_eod_jp`, `option_eod_index`, `calendar_refresh`; any-day without force: `statement_capture`; today-only without force: `account_capture`, `position_marks`, `quote_snapshot`, `calendar_hours_us`, `calendar_hours_jp`, `underlying_resolve`, `iv_history`, `option_list_preopen`, `option_list_zone`, `option_list_preopen_jp`; listed only: `option_open`, `option_open_jp`, `option_open_plan`, `option_open_plan_jp`, `option_open_resolve`, `option_open_resolve_jp`, `option_open_resolve_gap`, `option_ledger`.
- Dataset ids are not job ids: `fred_series` writes dataset `obs.series_point` (`internal/jobs/fredseries/plan.go:22`), the option list jobs `master.option_list` and `master.option_list.jp` (`job_list_preopen*.toml`), `option_eod` the manifest's `LedgerDataset` (`internal/jobs/optioneod/plan.go:31`). A map from dataset to job is needed and does not exist as a table.
- Option chains: the job refuses a day other than the clock's local date in its zone, `Permanent`, "a past option chain cannot be collected" (`internal/jobs/optioneod/run.go:118-121`, test `TestPastDayRefuses`, `run_test.go:390`); the console accepts only today for a today-only row (`readJobRun`, `jobconsole_post.go`, test `TestPostTodayOnlyTokyoZone`). `docs/design/option-captures.md` Decision 11 rests on this: a re-run never meets a compressed chunk, because compression starts after 7 days (`0006_option_eod.sql:74`). The runs are at 11:00 ET (`job.toml`, session open + 90 minutes) and 10:30 JST (`job_jp.toml`).
- A forced run of a stored day does what the console says: enqueue without force of a day that is present is a no-op, with force it replaces by natural key (`docs/design/option-captures.md`, the upserts of the bar, FX, series, event and fact stores use `ON CONFLICT ... DO UPDATE`).

## Decisions

1. **The ledger is the expected side; one view, aggregated per (day, dataset) in SQL.** `rd.coverage_day_r1` groups `ops.capture_expected_key` left-joined to `ops.capture_present` and, for the `no_value` keys, to the newest `ops.capture_run` row of the key (Decision 3), by session day and dataset: key counts and up to 20 missing keys. No per-key rows reach Go, no FRED column, no table, no write path changes. Days with no expected row at all are marked `not_run` by the page builder from `rd.session_day_r1` (already granted to `ot_ro_pages`).
2. **One day-window constant.** `coverage.WindowSessions = 30` in `internal/store/coverage/api.go` is the only place the number exists; the page builder, the repair and the reader take it from there. There is no `PastDays`, no deadline, no `button_until`. The days of the window come from one function: the builder asks `Reader.Sessions(calendar, Today minus coverage.LookbackDays, Today)` for each calendar its jobs use (`LookbackDays = 60` calendar days always hold 30 sessions), keeps the last `WindowSessions` days with `IsSession` at or before Today, and passes the oldest day of all calendars as `from` of `Reader.Cells(from, Today)`. The function is the exported `WindowDays` of the pages package, so the repair (entry 8) covers the same days as the page.
3. **A cell has a state and a repair verdict.** The view counts per (day, dataset): `ok`, `no_value`, `expired`, `missing`. `no_value` = the key is marked `no_value` and the newest `ops.capture_run` row of the key is a clean finish (the verdict `no_value` writes no reason and no error, `runkey.go:221-222,242`); `expired` = the key is marked `no_value` and the newest run row of the key is not clean: its `status` is not `done`, or its `error_code` is not empty, or its `reason_code` is `expired` or `crashed_repeatedly` (the rewrite of `expireIfClosed`, `reconcile.go:481-486`). A later clean run changes the cell: a repair that gets a value marks the key `present` (`runkey.go:235-236`, `:148`); one that gets nothing leaves a clean `no_value` run. State of a cell (builder, one precedence for a mixed cell, first match wins): `missing` if any key is missing > `expired` if any key is expired > `no_value` if any key is `no_value` > `ok`; a (day, dataset) with no `Cell` is `not_run` (it has no keys, so it never mixes). The cell prints `ok` of `keys` ("n of m") whenever the state is not `ok`. Verdict (builder): `none` (state `ok` or `no_value`, or no owner form), `button`, or `not_refetchable` with a text. The web renders both and decides nothing.
4. **Gaps are `missing` and `expired` cells; the repair is a plain run, never forced.** A plain run of a missing key fills it. A plain run of an `expired` key also runs: `already_present` is answered only for a key marked `present` (`isPresent := present[k] == store.StatusPresent`, `backfill_force.go:179-184`); a key marked `no_value` takes the ordinary run (`backfill_force.go:212-239`, `runMissingKey` `:274-283`, `RunKey` verdict `runkey.go:217-242`: a usable answer marks it `present`, so the cell turns `ok`). So no cell needs force. `ok` and `no_value` cells have no button on this page and are never overwritten by it; a forced run stays on `/job-console`, where it is today. So the page posts `force` never. `not_run` days are not gaps with a button (Decision 5).
5. **Verdict rule (one rule, in the builder).** `button` when the dataset's job is `any_day` and not present-only, or when it is `today_only` (or present-only) and the cell's day is today in the job's zone; otherwise `not_refetchable`. Present-only (`Job.PresentOnly`, a fixed list in `cmd/ot`: `earnings_refresh`, `dividend_capture`): their past-day run stores what the source says today under day D (replay, `earningsrefresh/run.go:211-214,260`; `dividendcapture/run.go:212-216,238`), so the cell would turn `ok` with data that is not D's; they are treated like the account data, which is simpler than a "filled later" mark on the page. This is the console's own rule (`readJobRun`) plus that flag, so a button never posts what the console refuses. Texts (the builder writes them, the web prints them): a past day of a today-only or present-only job, and a dataset with no job: "lost: the source gives only the present"; a `listed_only` job: the job's `WhyText`. The verdict applies to the states `missing` and `expired` only. A `not_run` day is always `not_refetchable`, with the text "not planned: repair of an unplanned day is not built yet" and never the word "lost": the console refuses a day with no plan at or before the runner's cursor (`no_plan`: `planned` `cmd/ot/jobconsole_turn.go:189-207`, `BackfillWith` `backfill_force.go:46-49`, `classify` `jobconsole_turn.go:224-225`), that is our guard and the source still has the day; the page does not read the cursor, so it offers no button. Planning such a day is the first item of Left out. Account data (`account_capture`, `position_marks`, `quote_snapshot`: today-only; `statement_capture`: any day, plain run) gets rows like every dataset: a past day of a present-only source shows "lost: the source gives only the present", a missed `statement_capture` day shows a button.
6. **Option chains: repairable on the same session day only.** `option_eod`, `option_eod_jp`, `option_eod_index` are today-only; their past days show "not refetchable" (owner word "a", question 3). No window, no change to the job: the job refuses a past day because the broker answers for the present only (`internal/jobs/optioneod/run.go:118-121`), and a re-run of a past day would store an `off_session` quote under it (`map.go:57-71`). Entry for a repair window is under Left out.
7. **Every dataset a job stores is a row.** The page lists every ledger dataset that has expected keys in the window, each with its job id from the map `dataHealthJobs()` (built in `cmd/ot` from `jobTable` and the manifests). `TestEveryJobHasRowOrReason` requires that each of the 33 `jobTable` ids is the job of a dataset or is named in a fixed list of jobs that store no ledger dataset, with the reason. The jobs list itself stays on `/job-console` (no `jobs[]` here).
8. **Repair all gaps of a dataset queues one request per gap day, oldest first.** `action=repair` with `dataset=<id>` on `/owner-job-console`; the server recomputes the gap days (`missing` and `expired` cells) from the view and takes nothing from the form but the dataset id and the token. Cap: the 30 sessions the page shows. The button prints the number of runs it starts, "repair N days", and N is `gap_days`, which the page and the server take from the same exported `GapDays` (entry 4), so the number printed is the number queued; the runs go one after another (the worker takes one request at a time). `bars_refresh` pulls 400 days per name and `fundamentals` the whole document per issuer for each gap day, so a large N is a long run; the number is on the button so the owner sees it before pressing. A ledger mark for days a run already covered is under Left out. One request per gap day for every job, window jobs included, because the ledger marks only the day a run was asked for: `MarkPresent(ctx, day, key, status)` in `RunKey` (`runkey.go:148`), so a later day's run that happens to re-fetch day D never turns D present. The table is what a plain run (no force) for a past day D does, read on origin/main `fef91db2`:

| job (dataset) | past-day plain run fetches | refetchable for past days |
|---|---|---|
| `fred_series` (`obs.series_point`) | plain series for D only; windowed series from D minus `lookback_days` (at most 31) to D (`fredseries/run.go:80,85`, `plan.go:56`) | yes |
| `fundamentals` | the whole document of each issuer up to D, no From bound (`fundamentals/run.go:100-115`) | yes |
| `bars_refresh` | a repair run: End = D, `backfill_days` (400) back, no adjusted step, no deepening (`barsrefresh/run.go:16-19,114-116`, `job.toml:4`); heavy per name | yes |
| `fx_refresh` | MIDPOINT bars with End = D, `duration_days` 5 (`fxrefresh/run.go:198-201`, `job.toml:4`), upsert by day | yes |
| `earnings_refresh` | one fetch per source of the current calendar, rows stamped D (a replay: `replay := day < today`, `earningsrefresh/run.go:211-214,260`); an instance returns its first result of day D without a fetch | no for a past day: present-only (Decision 5); yes for today (a repeat in the same process may return the first result, see Doubts) |
| `dividend_capture` | one Dividends call per entry, rows stamped D (replay, `dividendcapture/run.go:212-216,238`); same per-instance memo | no for a past day: present-only (Decision 5); yes for today (same remark) |
| `macro_events` | FRED release dates from D onward (`macroevents/run.go:96`, `release.go:46-47`) and the file kinds, `ReplaceFuture` from D (`run.go:64`) | yes |
| `statement_capture` | every alias's report for D (`statementcapture/job.go:151-158`); insert only | yes |
| `compute_figures` | no fetch: recomputes from stored rows of D (`computefigures/job.go:276-277`) | yes |
| `option_candidates` | no fetch: reads stored chains of D (`option_eod`); an alias with no chain day of D gets no row (`optioncandidates/job.go:163-180`) | yes, only where the stored chain of D exists |
| `book_risk` | no fetch: reads stored put candidates and positions of D (`bookrisk/job.go:194-215`); insert only under a new run id (`jobconsole_jobs.go:80`) | yes |
| `option_eod`, `option_eod_jp`, `option_eod_index`, `calendar_refresh`, `calendar_hours_jp`, `option_list_preopen_jp` (today-only rows, `jobconsole_jobs.go:87-89,91,93,99`) | refused for a past day (`optioneod/run.go:118-121`) | no |
| `account_capture`, `position_marks`, `quote_snapshot`, `calendar_hours_us`, `underlying_resolve`, `iv_history`, `option_list_preopen`, `option_list_zone` (today-only, `jobconsole_jobs.go:82,84,90,92,94,96-98`) | not asked for a past day | no |
| the eight listed-only `option_open*` and `option_ledger` rows (`jobconsole_jobs.go:101-108`) | never run from the console | no |

The two "no" groups, the past days of the two present-only rows and the listed-only rows give `not_refetchable` by the rule of Decision 5.
9. **Moved to Left out** (rate-limit probe): no entry delivers it, so the repair queues every gap day and the page does not say "queued one".
10. **FRED rows say how old the newest value is.** A FRED series publishes on its own calendar, so the row `obs.series_point` shows per series the newest observation date and its age in days (builder-computed) from `rd.series_point_r1`, next to the ledger cells.
11. **Audit and old values.** Who repaired which day is `settings.job_request`. The data rows keep `written_by` and `updated_at`. No old value is kept (owner 2026-11-13); the page says "a repair overwrites by key and keeps no earlier value".
12. **Words are explained where they stand.** A glossary list carries `ok`, `no_value`, `expired`, `missing`, `not_run`, `not refetchable` (no button: the text says "lost" when the source gives only the present and "not planned" when our repair of that day is not built), `ledger`, `session day`, each with its one-sentence meaning beside the legend (owner 2026-11-14).
13. **Dev mode.** No time guard; the only limit is the one open request of the same job, day and force that `Enqueue` enforces.
14. **Versions are named.** The view is `rd.coverage_day_r1` and the page `page.data_health_page.v1`; their columns and enums are kept to what the Interface lists (four states `ok`, `no_value`, `expired`, `missing` plus `not_run`; verdicts `none`, `button`, `not_refetchable`), and any change is `_r2` and `v2` as a new version, never an edit.
15. **Migration.** One migration, owner-only DDL, `migrations/0051_coverage_view.sql` (the last on origin/main is `0050_fx_rate_source.sql`, no open branch holds 0051 on 2026-11-17; a later design that lands first takes the next number and this one is renumbered in its pull request), Up and Down, adds the view and its grant, touches no row. The window deploys it with `setup pg`, `migrate`, `setup pg`; the code that reads it merges after it (Order lines).
16. **Today's cell before the slot.** `Job.DueAt` ("HH:MM", from the `slot` of the job manifest: `option_eod` and `option_eod_index` "1100", `option_eod_jp` "1030"; empty for every other job, which has no slot field) is the local time in `Job.Zone` at which the day's run starts. A cell of Today in state `missing` or `not_run` whose `Now` in that zone is before `DueAt` has `repair` `none` and `due_text` "due at 11:00 EST" (zone abbreviation of `Now`); at or after it the normal rule applies. A job with an empty `DueAt` has no `due_text`.
17. **Two writers of one dataset.** `master.option_list` is written by `option_list_preopen` and `option_list_zone` (`job_list_preopen.toml:8`, `job_list_zone.toml:10`). The dataset's `job_id` is the first job of `jobTable` order that lists it (`option_list_preopen`); both are today-only, so the choice changes no verdict.

## Modules

`docs/design/modules.md`, row `store`: the interface cell gains `internal/store/coverage` (declarations only; a package of its own because `internal/store/api.go` is at 800 lines and is not touched); this design's commit makes that edit. No other row changes. The entries touch store (`internal/store/**`, `migrations/**`, `docs/contracts/store/**`; entries 1 and 3), contracts (entry 2), pages (`internal/pages/**`, `docs/contracts/pages.md`; entries 4 and 5), ot (`cmd/**`, `docs/contracts/ot/**`; entries 6 and 8) and web (entry 7). New import edges, all inside the map: `internal/pages/data_health_page` imports `internal/store/coverage` and `internal/contracts`; `internal/store/ops` implements `coverage`; `cmd/ot` imports the three. No package of jobs or runner imports `coverage`; no jobs file is edited.

## Interface

Entry 1 (module store, new package `internal/store/coverage`, `api.go` declarations only, `api_test.go` the contract from both sides: a fake reader and a caller):

    // WindowSessions is the number of session days the page shows and the repair covers; the only place it is written.
    const WindowSessions = 30

    // LookbackDays is how many calendar days back the builder asks Sessions, enough for WindowSessions sessions.
    const LookbackDays = 60

    type State string
    const (
        StateOK      State = "ok"       // present, and the source gave a value
        StateNoValue State = "no_value" // marked no_value, and the newest run of the key finished clean
        StateExpired State = "expired"  // marked no_value, and the newest run of the key is not a clean finish
        StateMissing State = "missing"  // expected by the plan of the day, not present
    )

    // Cell is one (day, dataset) of the ledger, counted by state.
    type Cell struct {
        Day         contracts.Day
        Dataset     string
        Keys        int      // Keys == OK + NoValue + Expired + Missing
        OK, NoValue, Expired, Missing int
        MissingKeys []string // up to 20 missing subject keys, ascending
    }

    // Session is one row of rd.session_day_r1.
    type Session struct {
        Calendar  string
        Day       contracts.Day
        IsSession bool
    }

    // Newest is the newest stored observation of one series, from rd.series_point_r1.
    type Newest struct {
        SeriesID string
        Day      contracts.Day
    }

    // Reader reads the coverage as the page role (ot_ro_pages). Every slice is non-nil.
    type Reader interface {
        Cells(ctx context.Context, from, to contracts.Day) ([]Cell, error)               // ascending by day then dataset
        Sessions(ctx context.Context, calendar string, from, to contracts.Day) ([]Session, error)
        NewestObservations(ctx context.Context) ([]Newest, error)
        Close() error
    }
    type OpenFunc func(ctx context.Context, dsn string) (Reader, error)

The view (entry 3, SQL; the columns are the contract of `Cells`):

    rd.coverage_day_r1(session_day date, dataset_id text, keys int, ok int, no_value int, expired int, missing int, missing_keys text[])

one row per (session day, dataset); `ok` counts keys whose `capture_present.status = 'present'`, `no_value` and `expired` those with `'no_value'`, split by the newest `ops.capture_run` row of the key as in Decision 3, `missing` those with no present row; `missing_keys` holds at most 20, ascending; owner `ot_owner`; `GRANT SELECT ... TO ot_ro_pages`.

Entry 2 (module contracts): `page.data_health_page.v1` (envelope of `page.ops_today_page.v1.json`: `schema`, `schema_version`, `run_id`, `built_at`), with `form` (null or `{post_path, expires, mac}` as in `page.job_console.v1`; null when no owner edit, and then no cell and no dataset has a button); `datasets[]` `{dataset_id, label, job_id (string or null), calendar, gap_days (int, builder-computed), repair_all ("button" or "none"), newest[] ({series_id, day, age_days}; only the FRED row), cells[]}`; `cells[]` `{day, state (ok, no_value, expired, missing, not_run), keys, ok, no_value, expired, missing, missing_keys[], repair (none, button, not_refetchable), why_text ("" unless not_refetchable), due_text ("" unless Decision 16)}`; `glossary[]` `{term, text}`; `overwrite_note`; `gaps[]` (`coverage_unavailable`).

Entry 4 and 5 (module pages, package `internal/pages/data_health_page`): `Build(ctx, in Input) ([]byte, error)`:

    type Job struct {        // from jobTable and the manifests, filled by cmd/ot
        ID       string
        Kind     string      // "any_day", "today_only", "listed_only"
        Zone     string      // zone of "today" for a today_only job
        WhyText  string      // why a listed_only job or a past day cannot be run
        Datasets []string    // ledger datasets the job writes
        Calendar string      // "XNYS", "XTKS"
        PresentOnly bool     // past-day run stores today's source under the day (Decision 5)
        DueAt    string      // "HH:MM" in Zone, from the manifest slot; "" without a slot (Decision 16)
    }
    type Input struct {
        Now   time.Time
        Today contracts.Day
        Cov   coverage.Reader
        Jobs  []Job
        Form  *Form          // nil without owner edit
    }

The builder holds the rules of Decisions 2 (`WindowDays`), 3, 4, 5, 8 (`GapDays`), 10, 16 and nothing in `web/` repeats them. Entry 8 (module ot): the form field `action=repair`, `dataset`, the token fields, posted to `/owner-job-console`; the answer is the existing 303 to `/job-console?request=<first id>`, the page that shows the request's own end state (queued, running, done, failed). A cell button is the existing `action=run` with `job`, `day`, force off.

## Issues

Build order: the design is cut into entries by the numbers below; a draft names an entry as `Design: docs/design/data-health.md#<n>`. Every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. Entry 3 is the only migration; it holds the auto-deploy. Every entry carries its own `Contracts:` line: entry 2 alone changes a file under `contracts/schemas`. Order: 1 and 2 first; 3 and 4 after 1 (4 also after 2), parallel; 5 after 4; 6 after 3 and 5; 7 after 2 and 6; 8 after 6. No entry depends on a change to a job. Test runs are in `## Checked by CI`.

### 1 data-health-store-api (interface)

Module: store. Size: S.
Order: first.
Files: `internal/store/coverage/api.go`, `internal/store/coverage/api_test.go`, `docs/contracts/store/coverage.md`
Contracts: unchanged
Design: docs/design/data-health.md#Interface
Acceptance:
1. `api.go` declares `WindowSessions` (30), `LookbackDays` (60), `State` and its four constants, `Cell`, `Session`, `Newest`, `Reader` and `OpenFunc` as in the Interface section, with docs and no function body, and is the only non-test `.go` file of its package; `grep -c 'internal/store/coverage' docs/design/modules.md` prints 1 (written by this design's pull request); `git diff --name-only BASE -- internal/store/api.go docs/design/ratchet.json` prints nothing.
2. `api_test.go` has `TestReaderContractFake` (new: a fake `Reader` returns two cells and a caller reads them, `Keys == OK + NoValue + Expired + Missing`), `TestEveryMethodReturnsNonNil` (new: the fake's empty answers are non-nil slices) `TestWindowSessionsIs30` (new) and `TestLookbackIs60` (new).
3. `docs/contracts/store/coverage.md` lists the package, the four states, the meaning of every `Cell` field and the sentence "ok means the runner recorded the key as stored" in one paragraph each.

### 2 data-health-contract (interface)

Module: contracts. Size: M.
Order: first.
Files: `contracts/schemas/page.data_health_page.v1.json`, `web/src/generated/data_health_page.v1.ts`, `contracts/fixtures/page.data_health_page.v1.ok.json`, `contracts/fixtures/page.data_health_page.v1.gap.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/data_health_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.data_health_page v1 (new)
Design: docs/design/data-health.md#Interface
Acceptance:
1. The schema has the fields of Entry 2 of the Interface section and no others, `additionalProperties: false` at every level, `state` and `repair` as enums of the values named there, `form` nullable; the generated type is produced from it and not edited by hand.
2. `ok.json` carries one dataset with cells in each state and each repair value, a FRED row with `newest`; `gap.json` carries `gaps` with `coverage_unavailable` and no dataset; both fixtures are labelled invented in `MANIFEST.synthetic-v1.json`.
3. `data_health_page_v1_test.go` has `TestDataHealthFixturesValidate` (new: both fixtures pass the schema), `TestDataHealthRejectsUnknownField` (new: a cell with an extra key fails) and `TestDataHealthRepairEnum` (new: `repair: "button_until"` fails).
4. `docs/contracts/page-schemas.md` lists the page, states that a change is `v2`, and lists the glossary rule (every enum value of a cell has a `glossary` row).

### 3 data-health-view

Module: store. Size: M.
Order: after 1.
Files: `migrations/0051_coverage_view.sql`, `internal/store/ops/coverage.go`, `internal/store/ops/coverage_test.go`, `internal/store/ops/coverage_pg_test.go`, `docs/contracts/store/coverage.md`
Contracts: unchanged
Design: docs/design/data-health.md#Decisions (Decisions 1 and 15)
Acceptance:
1. The migration has Up and Down, creates the new view `rd.coverage_day_r1` with the eight columns of the Interface section (one row per session day and dataset), `ALTER VIEW ... OWNER TO ot_owner`, `GRANT SELECT ... TO ot_ro_pages`, and touches no table and no row; its file name is `0051_coverage_view.sql` (new file; `0050_fx_rate_source.sql` is the last before it).
2. `coverage.go` implements `coverage.Reader` (`var _ coverage.Reader = ...`, `var _ coverage.OpenFunc = ...`), reading `rd.coverage_day_r1`, `rd.session_day_r1` and `rd.series_point_r1` only (`grep -c 'FROM ops\.\|FROM obs\.\|FROM master\.' internal/store/ops/coverage.go` prints 0), opens with the pages DSN like `OpenGapReader`, and holds no fold loop over keys; `coverage_test.go` has `TestCellsFromRows` (new: two view rows scan to two cells with counts and `MissingKeys`) and `TestSessionsAscending` (new).
3. `coverage_pg_test.go` carries the build tag `pg` and has `TestCoverageViewStates` (new: three expected keys of one dataset and day, one with no present row, one `present`, one `no_value`, give one row with `keys` 3, `ok` 1, `no_value` 1, `missing` 1 and `missing_keys` of the one key), `TestCoverageViewExpired` (new: three `no_value` keys of one dataset and day, one whose newest `ops.capture_run` row has `reason_code` `expired`, one whose newest row has `error_code` `retryable` and no reason, one with a clean `done` row and no reason, give `expired` 2 and `no_value` 1; a later clean `done` row for the first turns it to `no_value`), `TestCoverageViewAsPagesRole` (new: `SELECT` on the view works as `ot_ro_pages` and `SELECT` on `ops.capture_present` is denied) and `TestCoverageDown` (new: Down removes the view).
4. The new file is under 600 lines and `internal/store/ops/` keeps no file over 800 lines.

### 4 data-health-page-rules

Module: pages. Size: M.
Order: after 1 and 2.
Files: `internal/pages/data_health_page/cells.go`, `internal/pages/data_health_page/repair.go`, `internal/pages/data_health_page/cells_test.go`, `internal/pages/data_health_page/repair_test.go`
Contracts: unchanged
Design: docs/design/data-health.md#Decisions (Decisions 2, 3, 4, 5, 8, 16)
Acceptance:
1. `cells.go` turns `coverage.Cell` rows and the sessions of a calendar into the cells of a dataset for the last `coverage.WindowSessions` sessions: a session day with no `Cell` of the dataset is `not_run` (`TestNotRunFromCalendar`, new), and a (day, dataset) with several keys in different states takes the state of Decision 3 by one precedence, `missing` > `expired` > `no_value` > `ok` (`TestMixedCellPrecedence`, new: 499 `ok` and 1 `missing` give `missing` and the text "499 of 500"; 1 `expired` and 499 `ok` give `expired`; 1 `no_value` and 499 `ok` give `no_value`; `missing` and `expired` together give `missing`), and no literal 30 appears in the package (`grep -rn '\b30\b' internal/pages/data_health_page/*.go` prints no non-test line).
2. `repair.go` sets `repair` by one rule, `TestRepairVerdicts` (new, a table): `ok` and `no_value` give `none`; `missing` or `expired` with an `any_day` job gives `button`; `not_run` always gives `not_refetchable` with the text "not planned: repair of an unplanned day is not built yet", which never contains the word "lost" (`TestNotRunNeverButton`, new: a `not_run` day of an `any_day` job, before and on `Today`, gives `not_refetchable` and that text); with a `today_only` job gives `button` for today in the job's zone and `not_refetchable` with the job's text for a past day (`option_eod`, Friday cell, `Now` Saturday: `not_refetchable`); a past day of a today-only job or of a present-only job (`TestPresentOnlyPastDay`, new: `earnings_refresh` with `PresentOnly`, a missing past day gives `not_refetchable` with "lost: the source gives only the present", the same job on `Today` gives `button`) carries that text; a `listed_only` job gives `not_refetchable` with its `WhyText`; no job gives `not_refetchable` with the "lost" text; no owner form gives `none` for every cell; `TestTodayBeforeSlot` (new: `option_eod` with `DueAt` "11:00", `Zone` America/New_York, a `missing` cell of `Today` at `Now` 10:00 gives `none` and `due_text` "due at 11:00 EST", at 11:00 gives `button`, and a job with empty `DueAt` has an empty `due_text`).
3. `cells.go` has the new exported `WindowDays(sessions []coverage.Session, today contracts.Day) []contracts.Day` (`TestWindowDaysLast30Sessions`, new: 45 sessions in the input give the last 30 at or before `Today`, ascending, none after `Today`) and `repair.go` the new exported `GapDays` (`TestGapDaysAndRepairAll`, new: `GapDays` and the field `gap_days` count `missing` and `expired` cells whose verdict is `button`, oldest first; `repair_all` is `button` only when that count is above 0 and the form is present).
4. Each new file is under 600 lines.

### 5 data-health-page-build

Module: pages. Size: M.
Order: after 4.
Files: `internal/pages/data_health_page/page.go`, `internal/pages/data_health_page/builder.go`, `internal/pages/data_health_page/strings.go`, `internal/pages/data_health_page/builder_test.go`, `internal/pages/data_health_page/purity_test.go`, `docs/contracts/pages.md`
Contracts: unchanged
Design: docs/design/data-health.md#Decisions (Decisions 7, 10, 11, 12)
Acceptance:
1. `Build` takes the `Input` of the Interface section, derives `from` with `WindowDays` per calendar (Decision 2), and emits a document that validates against `page.data_health_page.v1`; a `coverage.Reader` error gives the `gaps` entry `coverage_unavailable` and no dataset, never a failed page (`TestReaderErrorGivesGap`, new).
2. `TestDatasetRows` (new: two datasets from the fake reader give two rows, each with its `job_id` from `Job.Datasets` (the first job listing it when two do); a dataset no job lists has `job_id` null and its cells are `not_refetchable`).
3. `TestFredNewest` (new: `NewestObservations` rows of two series give `age_days` from `Today`, computed in the builder, not by the web).
4. `TestGlossaryCoversEnums` (new: every `state`, `repair` value and the words of Decision 12 have a glossary row) and `TestOverwriteNote` (new: the document carries the line of Decision 11).
5. `purity_test.go` shows no import of `database/sql`, `os`, `net/http` or a store implementation, and no read of the clock (`Now` is the input); `docs/contracts/pages.md` lists the package and the rules in one paragraph; each new file is under 600 lines.

### 6 data-health-wire

Module: ot. Size: M.
Order: after 3 and 5. It also edits the pinned lists of `cmd/ot/pagelist_test.go` and `cmd/ot/pagereg_test.go`, so it goes after any earlier page entry that edits them and before the next one; the two lists are edited in this one entry only.
Files: `cmd/ot/data_health_site.go`, `cmd/ot/data_health_site_test.go`, `cmd/ot/pagereg_test.go`, `cmd/ot/pagelist_test.go`, `docs/contracts/ot/pages/data_health_page.md`
Contracts: unchanged
Design: docs/design/data-health.md#Interface
Acceptance:
1. `data_health_site.go` has `siteDataHealthRoute = "data-health"`, registers a `pageFamily` as `market_state_site.go` does (name `data_health_page`), opens `coverage.Reader` with the pages DSN through `siteOpeners` and, when the owner edit is on, builds the `Form` with `jobConsolePostRoute` as `PostPath` like `buildJobConsolePage`.
2. `dataHealthJobs()` builds the `[]data_health_page.Job` from `jobTable` (every row carried: kind, zone, force text) and the manifests of the jobs (calendar, datasets of `Plan` for a fixed day, `DueAt` from the manifest `slot`: "11:00" for `option_eod` and `option_eod_index`, "10:30" for `option_eod_jp`, empty for the rest) and sets `PresentOnly` for the fixed list `earnings_refresh`, `dividend_capture`: `TestEveryJobHasRowOrReason` (new: each of the 33 `jobTable` ids is the job of a dataset or is in a fixed list of jobs that store no ledger dataset, each with a reason text; the four account jobs follow the same rule) `TestDatasetsResolveToFirstJob` (new: for every job whose plan is built in the test wiring, each `DatasetID` of its keys is listed by at least one `Job.Datasets`; `master.option_list` is listed by `option_list_preopen` and `option_list_zone` and the dataset row takes `option_list_preopen`, the first in `jobTable` order, Decision 17) and `TestDueAtFromSlot` (new: the three option chain jobs carry "11:00", "11:00" and "10:30", `fx_refresh` carries "").
3. `TestDataHealthRoute` (new: the route answers 200 with a document that validates, over fake readers; with no pages DSN it answers the page with the gap `coverage_unavailable`); `TestPageListHasDataHealth` (new, the pinned lists in the two test files each gain `data_health_page`, in the order of `registerPage` `order`).
4. `docs/contracts/ot/pages/data_health_page.md` is a new file that lists the route `data-health`, its gap `coverage_unavailable`, the repair button and the form rules in one paragraph each; no file other than the five named is edited; `git diff --name-only BASE -- cmd/ot/wire.go cmd/ot/site.go` prints nothing.

### 7 data-health-web

Module: web. Size: M.
Order: after 2 and 6 (the nav link must not point at a route that is not served).
Files: `web/src/pages/data_health/DataHealthPage.tsx`, `web/src/pages/data_health/CoverageGrid.tsx`, `web/src/pages/data_health/RepairButton.tsx`, `web/src/types/data_health_page.ts`, `web/src/shell/TopNav.tsx`, `web/test/shell/TopNav.test.tsx`, `web/test/pages/data_health/data_health.test.tsx`
Contracts: unchanged
Design: docs/design/data-health.md#Decisions (Decisions 3, 8, 12)
Acceptance:
1. 按 frontend-stack：用 `@tanstack/react-table` 做 the grid of datasets and 30 session cells of `ok.json`; 按 frontend-stack：用 shadcn `Card` and `Alert` 做 the page frame and the `gaps` line; 按 frontend-stack：用 shadcn `Button` (type submit) 做 every repair button, whose label is "repair {gap_days} days" for a dataset and "repair" for a cell; 按 frontend-stack：用 `lucide-react` `Info` 做 the icon beside each glossary term. The page holds no arithmetic on a value of the document and no rule of repair (`TestNoLogicInWeb` (new) in the same file: no age, count or date computation is exported from the page folder; state, verdict, `gap_days` and `age_days` are printed as given).
2. A cell with `repair: "button"` renders a form that posts to `form.post_path` with `action=run`, `job` (the dataset's `job_id`), `day` and the token fields and no `force` field; `not_refetchable` renders `why_text` and no form; `form: null` renders no form anywhere. A dataset with `repair_all: "button"` renders one form with `action=repair`, `dataset=<id>` and the token fields.
3. Each state and verdict is printed with its glossary text in the legend (`TestLegendShowsGlossary`, new); the page prints `overwrite_note`.
4. `TopNav.tsx` gains one entry "Data health" with route `data-health`, following the pinned list in `TopNav.test.tsx`, which is edited in the same entry. `data_health.test.tsx` has `TestDueTextPrinted` (new: a cell with `due_text` prints it and renders no form).

### 8 data-health-repair

Module: ot. Size: M.
Order: after 6.
Files: `cmd/ot/data_health_repair.go`, `cmd/ot/data_health_repair_test.go`, `cmd/ot/jobconsole_post.go`, `cmd/ot/jobconsole_post_test.go`, `docs/contracts/ot/pages/job_console_page.md`
Contracts: unchanged
Design: docs/design/data-health.md#Decisions (Decisions 4, 5, 8)
Acceptance:
1. `postOwnerJobConsole` handles `action=repair` after the same guard (`guardOwnerPost`) and opens the pages-role `coverage.Reader` per post; `repairGaps(ctx, ...)` in `data_health_repair.go` reads the sessions with `data_health_page.WindowDays` and the cells of those days, takes the days `data_health_page.GapDays` returns for the one dataset posted (the same function that gives the `gap_days` printed on the button), maps the dataset to its job with `dataHealthJobs()` and calls `Poster.Enqueue` with `Force` false, oldest day first, once for each of those days; the action `run` of a single cell is the existing one and is not edited.
2. Tests over a fake `coverage.Reader` and a fake `Poster`: `TestRepairQueuesOldestFirst` (new: three missing days give three requests in ascending day order, none with force); `TestRepairSkipsOkAndNoValue` (new: `ok`, `no_value` and `not_run` cells queue nothing; an `expired` cell queues one request with `Force` false); `TestRepairWindowJobOnePerDay` (new: for `bars_refresh`, `fred_series` and `fundamentals` three gap days give three requests, one per day, none merged, as the table of Decision 8 requires); `TestRepairTodayOnlyPastDay` (new: `option_eod` with a missing past day queues nothing); `TestRepairIgnoresFormDays` (new: a posted `day` field is not read); `TestRepairExistingRequestIsKept` (new: `EnqueueExisting` is not an error); `TestRepairNoPoster` (new: 503 with the fixed text of `textStoreFailure`); `TestRepairCountEqualsGapDays` (new: the number of requests queued equals `gap_days` of the same cells, the number the button prints); `TestRepairPresentOnlyPastDay` (new: `earnings_refresh` with a missing past day queues nothing, with a missing day today queues one).
3. The answer is the 303 of `redirectToRequest` to the first queued request, whose page shows the request's own end state (a queued or running request is not shown as repaired; a request that ends failed leaves the cell `expired` or `missing`, as for `earnings_refresh` and `dividend_capture`, whose per-instance memo may return a remembered failure); with nothing queued it is the 303 to `/data-health?repair=none`; `TestRepairAnswerNeverSaysRepaired` (new: no text written by `data_health_repair.go` contains "repaired").
4. `docs/contracts/ot/pages/job_console_page.md` lists `repair` as the third value of the field `action`, with its field `dataset`, its 422 for an unknown dataset and its answer.
5. `jobconsole_post.go` stays under 300 lines and no new file reaches 600; `git diff --name-only BASE -- cmd/ot/jobconsole_jobs.go cmd/ot/wire.go cmd/ot/site.go` prints nothing.

## Checked by CI

`go test ./...` (entries 1, 3 (the `pg` tests run only in GitHub CI), 4, 5, 6, 8), `go run ./scripts/arch` and go-arch-lint (the new import edges of Modules), `ci-web` (`tsc`, `eslint`, `vitest`, the build; entry 7), the schema test of entry 2, and the migration check with PostgreSQL 17 and TimescaleDB 2.30.2 (the pin of PR 2417). The view is proved on that pair only. After deploy the window renders `/data-health` and checks the page against the mockup with a screenshot (owner rule of 2026-11-14); that check is not an acceptance item.

## Left out

- **Planning a `not_run` day (first follow-up issue, not a later design).** The page says "not planned: repair of an unplanned day is not built yet" for such a day; the console refuses it `no_plan` at or before the cursor, which is our guard and not a fact about the source (owner 2026-11-12). Letting the console plan such a day is a change to `jobconsole_turn.go` with its own entry and test, and a day on which the machine was off is the most likely gap the owner will see, so this follows right after the entries here.
- **Option repair window job (former entry 7): a past option day is not repairable.** Reason: that the broker answers for the last session day after the close is unverified (the repo says it answers the present only, `run.go:118-121`); a forced past-day `option_eod` re-run would overwrite the session capture with an `off_session` quote (`optioneod/map.go:57-71`); and two runs of a past day with different capture ids would break "twice, same result" and `option-captures.md`. The window runs a read-only probe on gateway 4001 before any design of it. Until then option days are repairable on the same session day only; past days show "not refetchable" (owner Q3 a). No entry of this design depends on it.
- **Rate-limit probe (former Decision 9): not delivered.** Queuing one request when the newest finished request of the job is `failed` or `interrupted` would read `jobrequest.Reader` (`internal/store/jobrequest/api.go:88-94`), which no entry opens, and a page text "queued one" that no acceptance item names. Until the owner asks, the repair queues every gap day (at most 30); a failed request ends `failed` and the worker goes on to the next. Which `reason_code` a rate limit leaves is not read.
- A degraded fill of an option day from historical bars (no greeks, no IV): only if the owner asks.
- A button for `no_value` cells and a repair of `ok` cells: stay on `/job-console` (force). `expired` cells are not in this list: they have a plain-run button (Decision 4).
- The `not_proved` and `listed_only` console rows get no button for a past day; making them forceable is each its own small issue.
- A check that data rows exist (a restored or pruned table the ledger does not know): the view counts the ledger; a per-dataset row count is a later design if the owner asks.
- A ledger mark for days a run already covered (a window run of `bars_refresh` or `fundamentals` per gap day repeats work): only if a real backlog shows hours.
- A table `ops.coverage_day` written by the runner: only if the view proves slow.
- Automatic repair (a gap fills itself): needs a rate design first.
- Old values or a version history (owner 2026-11-13).
- Changes to the worker, the runner, `settings.job_request`, the jobs, `internal/store/api.go`, `cmd/ot/wire.go`, `cmd/ot/site.go` and `cmd/ot/jobconsole_jobs.go`.
- A new write outside the repository: none, so no `docs/writes.md` entry.

## Doubts

- Not read: the ledger datasets of `account_capture`, `position_marks`, `quote_snapshot` and `statement_capture` (entry 6 `TestEveryJobHasRowOrReason` decides; a job with no ledger keys cannot show gaps and is named in the fixed list).
- Not verified: whether the console host rebuilds its job set per request (`cmd/ot/jobconsole_host.go:71,95` keeps the set it was given). `earnings_refresh` and `dividend_capture` return their first result of a day from memory on a repeat in the same instance (`earningsrefresh/run.go:213-214`, `dividendcapture/run.go:214-215`); with the present-only rule only today's repair can meet it, and it fails loudly (the request ends failed, the cell is not turned `ok`); entry 8 item 3 shows the request's end state.
- Pending code in flight at the time of reading: #2550 touches `internal/jobs/optioneod`; #2548 merged (`1262b35e`) and the citations of `optioneod/run.go:118-121` and `map.go:57-71` were re-read after it and hold; they shift only if #2550 moves those lines.
- Not verified: whether a genuine source answer of nothing always leaves a clean newest run row (read: `runkey.go:221-222,242` set no reason and no error); `TestCoverageViewExpired` pins the view's rule on rows, not on a live run.
- The order against other open design pull requests (`modules.md` conflicts) and a clash of migration number 0051 are settled when the pull request opens.

## Adversarial

Critique: `state/optdesk-drafts/designs/data-repair/adversarial-2026-11-15.md`.

1. Gap against the goal, account data and jobs without a row: adopted. Decisions 5 and 7 and entry 6 give every ledger dataset a row, account data included, "not refetchable" for past days of present-only sources, a test that every `jobTable` id has a row or a named reason.
2. Gap, "complete means ledger present": adopted as a statement. `capture_present` is written only by the runner after a usable result (`runkey.go:148`), so it means "the runner recorded the key as stored", not a row count; the legend says so, and a row-count check is Left out with the reason (no generic count across dataset tables).
3. Gap, datasets with no ledger keys: not adopted as rows (nothing to count); named in the fixed list of entry 6.
4. Gap, `not_run` button for unplanned days: adopted. A button exists only when the job accepts the day (Decision 5) and the day has keys; a `not_run` day gets no button because the console refuses it `no_plan` at or before the cursor (`jobconsole_turn.go:189-207`).
5. Risk 2, option_eod overwrite with off_session data: adopted. Entry 7 removed; past option days are not refetchable; Left out names the reason and the probe.
6. Risk 2, tens of thousands of rows: adopted. The view aggregates per (day, dataset) in SQL; no Go fold.
7. Simpler, aggregate in SQL / drop jobs[] / drop button_until, PastDays / one button per dataset-day and per dataset: adopted. The `no_value` cell button is dropped too (force stays on `/job-console`). Window jobs get one request per gap day.
8. Lock-in, frozen view and enums: adopted. Minimal columns and enums, `_r1`/`v1` named, a change is a new version (Decision 14).
9. Lock-in, `action=repair` on the console: not adopted as a change; it adds one action to the existing post and edits no accepted-day rule (`jobConsole` rows untouched).
10. Lock-in, option_eod day rule and `PastDays` three times: adopted. The job is not changed and `PastDays` does not exist; the only constant is `coverage.WindowSessions`.
11. Claim 5, broker answers for the last session day: adopted. The claim is removed and the capture-id change with it.
12. Slip, entry 7 unprobeable: adopted by removal; no entry depends on it (entry 8 follows 6 only).
13. Slip, entry 4 L: adopted. Split into entries 4 (rules) and 5 (build), same module.
14. Slip, pinned lists: adopted. Both lists are edited in entry 6 only.
15. Slip, `modules.md` conflicts with parallel design PRs: not adopted as a design change; settled at PR open (Doubts).

## Fix round 2026-11-15 (advisor)

Origin/main read at `fef91db2`. Only the five blocking points of `advisor-2026-11-15.md`.

1. `no_value` after a failed window: new state `expired` (Goal, Today, Decision 3, Decision 4, Decision 14, Interface entry 1 and 2 and the view, entries 1, 2, 3, 4, 8, Left out). It is repaired by a plain run, not force: `already_present` is answered only for a `present` key (`backfill_force.go:179-184`), so `never force` holds for every cell.
2. `not_run` day: no button, always `not_refetchable` (Decision 5, entry 4 `TestNotRunNeverButton`, entry 8, Left out "Planning a `not_run` day", Adversarial 4).
3. Mixed cell: one precedence `missing` > `expired` > `no_value` > `ok`, `not_run` has no keys (Decision 3, entry 4 `TestMixedCellPrecedence`).
4. Per-job table of what a plain past-day run fetches, with file:line (Decision 8; entry 8 `TestRepairWindowJobOnePerDay`; Doubts).
5. Decision 9 moved to Left out (Decision 9 stub, entry 8 Design line and its test, Left out "Rate-limit probe", Doubts).

## Fix round 2026-11-17 (advisor r2, non-blocking points)

Origin/main read at `e18b41bd` and `1262b35e`; every file:line citation was re-read and holds (`optioneod/run.go:118-121`, `TestPastDayRefuses` `run_test.go:390`, `reconcile.go`, `runkey.go`, `backfill_force.go`, `jobconsole_jobs.go:75-110`, `jobconsole_turn.go`, the manifests); the table cites `fef91db2` text unchanged.

- a. A `not_run` day prints "not planned: repair of an unplanned day is not built yet", never "lost" (Goal, Decision 5, entry 4); "Planning a `not_run` day" is the first Left out item.
- b. `earnings_refresh` and `dividend_capture` are present-only (Decision 5, table, entries 4, 6, 8), chosen over a "filled later" mark because it is one flag and no page text.
- c. The memo failure is loud; entry 8 item 3 shows the request's end state and `TestRepairAnswerNeverSaysRepaired` (new) pins it.
- d. The button prints "repair N days", N from the same `GapDays` the server uses (Decision 8, entries 4, 7, 8); the ledger mark is Left out.
- e. Files of entries 6 and 8 carry the contract docs; "due at" is Decision 16; `from` is `WindowDays` (Decision 2, entry 4); the two writers of `master.option_list` are Decision 17 and `TestDatasetsResolveToFirstJob`.
