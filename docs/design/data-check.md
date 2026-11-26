---
type: design
title: "data check: a daily job that tests stored data for wrong values and silent gaps, keeps its findings, shows them on the data health page and sends one message when a finding is new"
status: accepted
date: "2026-11-26"
need: [none]
none_reason: "owner ask 2026-11-26: a separate feature that checks every day at a fixed time whether the data is healthy, first only reports; no need record exists"
---

# data check: a daily job that tests stored data for wrong values and silent gaps, keeps its findings, shows them on the data health page and sends one message when a finding is new

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-26
Plan: P1

Source: owner ask 2026-11-26, "a separate feature that checks every day at a fixed time whether the data is healthy, and repairs if not, or at first only reports it"; the dataset sweep of the same day (`state/optdesk-dataset-sweep-2026-11-26.md`, rows 1, 23, 32, 36, 37 and the gap rows). Decision of this design: the first version REPORTS ONLY. Repair is under Left out; the repair button of `docs/design/data-health.md` stays the repair path. Every fact below was read on origin/main `8241d7a0` after `git fetch` on 2026-11-26; nothing was run and no database was read.

Owner-visible result: every day two hours after the US close one job, `data_check`, runs a fixed list of six rules over the last 5 session days of each market. A finding says which dataset, which day, which rule, how bad (`error`: wrong data stored as good; `warn`: a gap; `unknown`: the rule could not decide) and the counts it rests on. The findings are on the data health page in a block "数据检查" under the coverage grid. When a finding appears that no earlier run reported, one Telegram message lists the new ones (at most 10 lines, then "还有 N 条见页面"). Nothing is repaired, deleted or overwritten by the job.

## Frame

- The coverage page (`docs/design/data-health.md`) answers "did the runner record the key". Its own legend says `ok` means "the runner recorded the key as stored", not that the values are right (Today, first bullet). The sweep found three cases the ledger shows green or does not show: readings of rv20, rv60, rv120 and hv30 repeating the previous day while the bars were stale (row 32; the cause is addressed by open pull request #4150, the check stays as the net for any other path), `obs.option_open` rows whose receipt time is a later day than the session day (row 23), candidates computed before a later change (rows 36, 37). It also found gaps nobody was told about: bars of 11-24 and 11-25 not written (row 1; the ledger cell is `expired` and `ops.job_gap` is open, but only the page shows it), and datasets with no plan at all for a day.
- Today nothing reads the data and says "wrong". `internal/notify` pushes open gaps (`NotifyGaps`, `GapSource.OpenGaps`), slow runs, gateway state and the morning report; none of them looks at values.
- A job that checks is not new in the repo: `recon_bars` (`internal/jobs/reconbars`, design `docs/design/base-data-page.md` entry 21) compares our bars with a second source and stores a run in its own schema `recon` with its own role `ot_w_recon` (`migrations/0060_recon.sql`, `internal/store/reconpg`, `internal/store/recon/api.go`, wiring `cmd/ot/wire_reconbars.go`, line `reconDSN` in `cmd/ot/runonce.go:64-65,100`). This design copies that shape: one job, one schema, one role, one wiring file.

## Owner rules and how this design meets them

1. "The front end renders data and computes nothing" (`AGENTS.md`, Tests; `docs/design/data-health.md` principles). The rule texts, labels and evidence sentences are built by the page builder from stored counts; `web/` prints strings and flags. Self-check: entry 10 has no arithmetic, no sorting rule, no wording table; entry 9 owns them.
2. "Dev mode: no time guards except one run of the same job at a time" (`docs/design/data-health.md` Decision 13). The job has no "only after" guard besides its schedule slot; the console may run it for any day, with force, at any time; the runner's one-run-per-job rule is the only limit. Self-check: no entry adds a time condition.
3. "Tests run in parallel, never sleep, time from a fake clock, a temporary directory for every file, no fixed port" (`AGENTS.md`, Tests). Every rule test builds synthetic facts and a fake `Probe`/`coverage.Reader` and a `clockwork` fake clock; `t.Parallel()`; PostgreSQL tests carry the `pg` tag. Self-check: no acceptance item below runs a test; runs are in "Checked by CI".
4. "A new source file stays under 600 lines; a file over 800 lines is listed in `docs/design/ratchet.json` and a ceiling is never raised by an agent" (`AGENTS.md`, Size). The only file listed is `internal/compute/api.go` (840), which no entry touches. Files edited here are at: `cmd/ot/runonce.go` 576, `cmd/ot/wire.go` 791 (not touched; the wiring is a new file), `cmd/ot/data_health_site.go` 231, `internal/pages/data_health_page/builder.go` 213, `internal/store/setup/setup.go` 332. `cmd/ot/runonce.go` gains a few lines and stays under 800. Each rule file is one file of at most 3 rule functions.
5. "Own code behind an interface" (owner 2026-11-08). The reads are behind `finding.Probe`, the writes behind `finding.Writer`, the page read behind `finding.Reader`, the message behind `datacheck.Announcer`; a later library or a second implementation replaces any of them without touching a rule.
6. "Pages are in Chinese" (owner 2026-11-17: hints in Chinese). Block title, rule labels, severity words, evidence sentences and the message are Chinese strings in `internal/pages/data_health_page/zh_texts.go` and `internal/jobs/datacheck/announce.go`; a technical word is explained next to it (owner 2026-11-14), through the page glossary.
7. "Gaps stay gaps with a reason, never invented values" (owner 2026-11-24). A rule that cannot read its input stores a finding of severity `unknown` with the reason `read_failed`; it never stores "no problem" for what it could not look at, and a run in which every rule is `unknown` is a failed run (Decision 8). The job writes no data value anywhere but its own tables.
8. "One issue, one module" (`AGENTS.md`, Modules). Each entry below carries one `Module:` and its Files are inside that module's paths of `docs/design/modules.md` (checked in "Modules").
9. "Dev mode, the job runs when asked; the owner works on the web, the terminal is a thin wrapper" (`docs/design/data-health.md` principles). The run button is the existing console row (Decision 9); no new command.
10. "No secrets, account ids or personal data in code, rows or messages" (`AGENTS.md`, Pull requests). A finding holds a rule id, a dataset id, a day, a severity and counts. Entry 4 has a test that a stored finding has no free-text field longer than the fixed vocabulary.

## Decisions

1. **One job, `data_check`, one run per day, report only.** It reads, judges, stores, sends. It never calls a repair, never writes a dataset of another job, never enqueues a request (Left out). The first version's rules are six functions, three for wrong values and three for gaps (Decisions 3 and 4).
2. **Schedule.** `calendar = "XNYS"`, `anchor = "session_close"`, `zone = "America/New_York"`, `offset = "120m"`, `run_bound = "10m"`, `restart_sensitive = false` in `internal/jobs/datacheck/job.toml`, the shape of `internal/jobs/bookrisk/job.toml` and `internal/jobs/reconbars/job.toml`. 120 minutes is after `book_risk` (45m) and `recon_bars` (30m) and after the Japanese close; about 06:00 Singapore time in summer, 07:00 in winter, so the message falls at or after the end of the notify quiet hours (`DefaultQuietTo = "07:00"`) in winter. The slot is a proposal the owner may move by editing the one line. A day when XNYS is closed has no run; the next run looks back 5 session days of each market, so a Japanese-only day is checked a day later (Doubts, 1).
3. **Rules that find wrong data (severity `error`).** Each is one function in `internal/jobs/datacheck`, over the last `CheckWindowSessions = 5` session days of the dataset's market (`coverage.Reader.Sessions`), the check day excluded for rules that depend on a finished day (Decision 5).
   - `reading_repeat` (sweep row 32). For the names `market.hv30_own`, `market.rv20`, `market.rv60`, `market.rv120` (`internal/jobs/computefigures/job.toml:9,10,16,17`): for a reading day D, the subjects whose ok reading of D has exactly the value of the same subject's ok reading of the previous day that has one. Fires when at least 5 subjects have an ok reading on D and at least half of them repeat. Evidence: `ok`, `same`. Since `migrations/0045_reading_key.sql` `readings.reading` has one row per (name, version, subject_kind, subject, day) (a rerun updates the row in place) and `rd.reading_r1` passes it through with no winner filter; the probe therefore takes the highest `version` of a name that has a row for the day, and compares ok rows only.
   - `option_open_late_quote` (sweep row 23). Rows of `rd.option_open_r1` whose `received_at`, read in the zone of the row's currency (`USD` America/New_York, `JPY` Asia/Tokyo), falls on a day after `session_day`. Fires when at least 1 row. Evidence: `rows`, `late`.
   - `candidates_older_than_settings` (sweep rows 36, 37, the part data can show). For the newest session day that has candidate rows, per `kind`: the newest run's `computed_at` is earlier than the newest `set_at` of `rd.rule_param_r1` and `rd.option_exclusion_r1`. Severity is `warn`, not `error`: the rows are right for the settings they used. Evidence: `rows`, `hours_behind` (whole hours). A rule change made in code without a change of a stored setting is not visible in data; `ruleVersion` of `internal/jobs/optioncandidates/convert.go:22` is the constant `"put_v1"` and does not move with a code change. That case is under Left out, with its fix.
4. **Rules that find gaps (severity `warn`).**
   - `capture_gap` (sweep row 1). For each session day in the dataset's window before the check day: a cell of `coverage.Reader.Cells` with `Missing + Expired > 0`. One finding per (dataset, day). Evidence: `keys`, `missing`, `expired`. It reports what the page already shows, and is the reason a bars write failure reaches the owner without opening the page.
   - `plan_absent` (sweep: a job never built logs nothing). For each dataset of the expected list (Decision 6) and each session day of its calendar's window before the check day: no cell at all. One finding per (dataset, day). Evidence: `cells` = 0. The page's `not_run` state shows it per cell; nothing tells the owner.
   - `gap_unclosed` (sweep: `job_gap` never closed). Rows of `rd.job_gap_r1` with `cleared_at` empty, `first_seen_at` older than 24 hours at the run's `Now` and `day` inside the window of the dataset's calendar (older open gaps are history, not news; the sweep counted 55 open rows). Evidence: `age_hours`. A gap whose (dataset, day) is already a `capture_gap` finding of the same run is dropped, so one problem is one finding (the job does this, a pure function with a test).
5. **A finding is keyed by (rule, dataset, day, subject), not by its counts.** A condition that gets worse is not announced again; the page shows the current counts. The dataset `datacheck.run` is skipped by `capture_gap` and `plan_absent`, so the job does not report on itself (a failed run is seen by the existing gap message). `subject` is empty in all six rules of this version (the column exists for later rules). The window of days is the rule's; a finding for a day inside the window is produced again on the next run while the condition holds, so the page always shows the current state, and a repaired day disappears from the next run's findings.
6. **The expected list of `plan_absent` comes from `cmd/ot`, not from the job.** `jobs` may not import another job or `cmd/ot` (`docs/design/modules.md`, row jobs). `cmd/ot` builds `[]datacheck.ExpectedDataset{Dataset, Calendar}` from `dataHealthManifests` (`cmd/ot/data_health_site.go:122`) for the jobs of `jobTable` that are not in `dataHealthNoLedger` and not in a new fixed list `dataCheckNotDaily` with a reason per entry (a job that is not planned on every session day, such as `calendar_refresh`). `TestEveryLedgerJobIsDailyOrReasoned` (entry 11) fails for a job that is in neither list, as `TestEveryJobHasRowOrReason` does for the page.
7. **Findings are stored insert-only in a schema of their own, `datacheck`.** `datacheck.run (run_id identity, check_day, started_at, finished_at, rules_total, rules_unknown)`, `datacheck.finding (finding_id identity, run_id, rule, dataset, day, subject, severity, evidence, is_new)`, `datacheck.notice (run_id, noticed_at)`. No UPDATE, DELETE or TRUNCATE for any role but the owner (the rule of `0042_strategy_result.sql` and `0060_recon.sql`). `evidence` is text holding a JSON list of `{name, n}` with `name` from a fixed vocabulary and `n` a count. The page reads two views, `rd.data_check_run_r1` and `rd.data_finding_r1`, as `ot_ro_pages`.
8. **"New" means never delivered.** `is_new` is true when no finding of the same key exists in an earlier run that has a `datacheck.notice` row. The writer computes it at insert, in one statement. The job sends one message for the findings with `is_new`, then writes the notice row after the send returned nil (at-least-once, as `MarkSent` is called "after the send, never before", `internal/store/api.go:481-483`). If the send fails the job returns a retryable error, the notice is not written, and the next run finds the same findings new again. A run in which every rule is `unknown` returns `Usable = 0` and so is a failed run the runner retries; a run in which some rules decided returns `Returned = rules`, `Usable = rules decided`.
9. **Console and ledger.** The job's Plan names one key of ledger dataset `datacheck.run`, as `recon_bars` names `recon.run` (`internal/jobs/reconbars/job.go:22,106-109`). `jobTable` gets `{ID: "data_check", Kind: kindAnyDay, Force: true}` (as `book_risk`: insert-only under a new run id; a force re-run after a repair is how the owner sees the check again), and `dataHealthNoLedger` gets `"data_check": "it stores a check run and no ledger dataset"` (as `recon_bars`).
10. **One role.** `ot_w_datacheck`, line `OT_PG_DSN_DATACHECK`, made by `setup pg` (`internal/store/setup/setup.go`, the role table at lines 120-131). Grants (entry 3): INSERT and SELECT on the three tables; SELECT on `rd.coverage_day_r1`, `rd.session_day_r1`, `rd.reading_r1`, `rd.option_open_r1`, `rd.option_candidate_r1`, `rd.rule_param_r1`, `rd.option_exclusion_r1`, `rd.job_gap_r1` and the two new views; USAGE on schemas `datacheck` and `rd`. The page role `ot_ro_pages` gets the two new views only. While the line is empty the job is held with a stated reason (`startOff`, as `reasonNoReconDSN`).
11. **The message.** The scheduler and the console build jobs through `registerJob` (`cmd/ot/wire_reconbars.go`); the new wiring file passes `telegramSender(a)` (`cmd/ot/notify.go:107`, reads `OT_TELEGRAM_TOKEN` and `OT_TELEGRAM_CHAT_ID`) as the `Announcer`. Without the two lines the announcer is nil: findings are stored, the page shows them, the run is ok, and the job logs one line "no announcer". The text carries rule label, dataset, day and counts in Chinese; no id of an account, an order or a contract.
12. **The page block.** `page.data_health_page.v1` gains one optional field `data_check` and one value of the gap `reason` enum, `data_check_unavailable`, under the rule of `docs/design/repeated-failures.md` Decision 9 (an additive optional field, `schema_version` stays 1, because page and web ship together). The builder fills it on every document; "always present" is an acceptance item. The block shows: the last run (check day, finish time, rules decided of total), a count per severity, and one row per finding with `新` on `is_new`. The words `error`, `warn`, `unknown` are shown as `错误`, `缺口`, `无法判断` with a one-sentence gloss each in the glossary.
13. **Versions.** View names end `_r1`; any column change is `_r2`. `datacheck` rule ids and the evidence vocabulary are fixed strings in `internal/store/finding/api.go`; a new rule adds a string and a label, never changes one.
14. **Migration.** One migration, owner-only DDL, `migrations/0073_data_check.sql` with Up and Down: the last on origin/main is `0071_instrument_conid_repair.sql`; the open design `earnings-fallback` (pull request #4142) names 0072, so this one takes 0073, and no other open branch holds a number above 0071 on 2026-11-26. A later design that lands first takes the next number and this one is renumbered in its pull request. Deploy: the window rebuilds `ot`, then `setup pg`, `migrate`, `setup pg` (owner 2026-11-09), because the role is made by `setup pg` and its grants by the migration; the code that uses them merges after the migration (Order lines).

## Modules

`docs/design/modules.md`: the row `store` gains `internal/store/finding` in its entry cell (declarations only, a package of its own because `internal/store/api.go` is 800 lines and is not touched); this design's commit makes that edit. No other row changes and no dependency edge is added to a depends cell. First row that matches decides: store (`internal/store/**`, `migrations/**`, `docs/contracts/store/**`: entries 1, 3, 4, 5), contracts (`contracts/**`, `internal/contracts/**`, `web/src/generated/**`, `docs/contracts/page-schemas.md`: entry 2), jobs (`internal/jobs/**`, `docs/contracts/jobs.md`, `docs/contracts/jobs/**`: entries 6, 7, 8), pages (`internal/pages/**`: entry 9), web (`web/**`: entry 10), ot (`cmd/**`, `docs/contracts/ot/**`: entry 11). New import edges, all inside the map: `internal/jobs/datacheck` imports `internal/store/finding`, `internal/store/coverage`, `internal/contracts`, `internal/clock`; `internal/store/findingpg` implements `finding`; `internal/pages/data_health_page` imports `internal/store/finding`; `cmd/ot` imports `internal/jobs/datacheck`, `internal/store/findingpg` and `internal/store/ops` (already). `internal/store/setup` may not import `internal/store/finding`, as it may not import `store/recon` (`setup.go:89`); it holds its own constant for the line name and a test that the two are equal.

## Interface

Landed in #4181

Entry 1 (module store, new package `internal/store/finding`, `api.go` declarations only, `api_test.go` the contract from both sides: a fake writer, reader and probe, and a caller):

    // DSNEnvDataCheck is the line of the role ot_w_datacheck.
    const DSNEnvDataCheck = "OT_PG_DSN_DATACHECK"

    type Severity string
    const (
        SeverityError   Severity = "error"   // wrong data stored as good
        SeverityWarn    Severity = "warn"    // a gap
        SeverityUnknown Severity = "unknown" // the rule could not decide
    )

    // Rule ids (fixed strings; a new rule adds one).
    const (
        RuleReadingRepeat   = "reading_repeat"
        RuleOpenLateQuote   = "option_open_late_quote"
        RuleCandidatesStale = "candidates_older_than_settings"
        RuleCaptureGap      = "capture_gap"
        RulePlanAbsent      = "plan_absent"
        RuleGapUnclosed     = "gap_unclosed"
    )

    // Evidence names (fixed): ok, same, rows, late, hours_behind, keys, missing, expired, cells, age_hours, read_failed.
    type Count struct {
        Name string
        N    int
    }

    // Finding is one problem: a rule, a dataset (or a reading name), a day, a severity and counts. No free text.
    type Finding struct {
        Rule, Dataset string
        Day           contracts.Day
        Subject       string // empty in this version
        Severity      Severity
        Evidence      []Count
        IsNew         bool // set by the writer, ignored on input
    }

    type Run struct {
        RunID                       int64
        CheckDay                    contracts.Day
        StartedAt, FinishedAt       time.Time
        RulesTotal, RulesUnknown    int
        Findings                    []Finding
    }

    // Writer stores one run and says which findings are new (Decision 8).
    type Writer interface {
        InsertRun(ctx context.Context, r Run) (Run, error) // the returned Run has RunID and IsNew set
        MarkNoticed(ctx context.Context, runID int64) error
        Close() error
    }

    // Reader reads the newest run and its findings as ot_ro_pages. Never nil slices; no run gives a zero Run.
    type Reader interface {
        Latest(ctx context.Context) (Run, error)
        Close() error
    }

    // Probe answers the questions of the rules with counts, never rows. window days are inclusive.
    type Probe interface {
        RepeatedReadings(ctx context.Context, names []string, from, to contracts.Day) ([]ReadingRepeat, error)
        LateOpenQuotes(ctx context.Context, from, to contracts.Day) ([]LateQuotes, error)
        NewestCandidateRuns(ctx context.Context, from, to contracts.Day) ([]CandidateRun, error)
        SettingsChangedAt(ctx context.Context) (time.Time, bool, error) // false: no setting stored
        OpenGaps(ctx context.Context) ([]OpenGap, error)
        Close() error
    }
    type ReadingRepeat struct{ Name string; Day contracts.Day; OK, Same int }
    type LateQuotes   struct{ Day contracts.Day; Currency string; Rows, Late int }
    type CandidateRun struct{ Day contracts.Day; Kind string; Rows int; ComputedAt time.Time }
    type OpenGap      struct{ JobID, Dataset string; Day contracts.Day; Reason string; FirstSeenAt time.Time }

    type OpenWriterFunc func(ctx context.Context, dsn string) (Writer, error)
    type OpenReaderFunc func(ctx context.Context, dsn string) (Reader, error)
    type OpenProbeFunc  func(ctx context.Context, dsn string) (Probe, error)

Entry 2 (module contracts): the optional field of `page.data_health_page.v1`:

    data_check: null or {
      check_day, finished_at, none_text (Optional: empty allowed), rules_total, rules_unknown (Count), title, note (Text),
      counts: {error, warn, unknown} (Count),
      findings: [{rule, label, dataset, day, severity, severity_text, is_new, evidence_text, why_text}]
    }

`data_check` is null only when the builder has no reader or the reader fails; when `datacheck.run` has no run, `data_check` is an object with `none_text` "还没有检查结果", empty `findings`, zero counts and empty `check_day` and `finished_at`, and the block then shows `none_text`; when there is a run the builder writes `none_text` empty, so the web shows `none_text` exactly when it is non-empty and decides nothing itself; the gap enum gains `data_check_unavailable`. Entries 9 and 10 read only this schema; the Go view-model types stay inside `internal/pages/data_health_page`.

Entry 6 (module jobs): package `internal/jobs/datacheck`, exported surface only:

    const CheckWindowSessions = 5
    const GapGrace = 24 * time.Hour

    type ExpectedDataset struct{ Dataset, Calendar string }
    type Announcer interface{ Announce(ctx context.Context, text string) error } // cmd/ot adapts notify.Sender
    type Stores struct {
        Cov      coverage.Reader
        Probe    finding.Probe
        Out      finding.Writer
        Expected []ExpectedDataset
        Announce Announcer // nil: findings are stored, nothing is sent
    }
    type Rule struct {
        ID  string
        Run func(ctx context.Context, in Input) ([]finding.Finding, error)
    }
    type Input struct {
        CheckDay contracts.Day
        Now      time.Time
        Windows  map[string][]contracts.Day // calendar id to its session days of the window, ascending, check day excluded
        Cov      coverage.Reader
        Probe    finding.Probe
        Expected []ExpectedDataset
    }
    func New(st Stores, clk clock.Clock) (contracts.Job, error) // an error names a key, never a value

A rule that returns an error is stored as one finding of severity `unknown` with the rule id as its rule and as its dataset and the check day as its day, because a `Rule` names no dataset. With no rule the job returns the error `no_rules` and writes nothing.

## Issues

Build order: the design is cut into entries by the numbers below; a draft names an entry as `Design: docs/design/data-check.md#<n>`. Every entry compiles and its tests pass against main. Entry 3 is the only migration and holds the auto-deploy. Every entry carries a `Contracts:` line; entry 2 alone changes a file under `contracts/schemas`. Order: 1 and 2 first; 3 after 1; 4 and 5 after 1 and 3 (parallel); 6 after 1; 7 after 5 and 6; 8 after 7; 9 after 1 and 2; 10 after 2; 11 after 4, 5, 6, 8 and 9. Test runs are in `## Checked by CI`. Each entry is one module and lists only paths of that module.

### 1 data-check-store-api (interface)

Module: store. Size: S.
Order: first.
Files: `internal/store/finding/api.go` (new), `internal/store/finding/api_test.go` (new), `docs/contracts/store/finding.md` (new)
Contracts: unchanged
Design: docs/design/data-check.md#Interface
Acceptance:
1. `api.go` declares what the Interface section lists for entry 1 (`DSNEnvDataCheck`, `Severity` and its three constants, the six rule ids, `Count`, `Finding`, `Run`, `Writer`, `Reader`, `Probe`, the four fact types and the three `Open...Func` types) with docs and no function body, and is the only non-test `.go` file of its package; `grep -c 'internal/store/finding' docs/design/modules.md` prints 1 (written by this design's pull request); `git diff --name-only BASE -- internal/store/api.go docs/design/ratchet.json` prints nothing.
2. `api_test.go` has `TestFindingContractFake` (new) asserting: a fake writer stores a run, the fake reader returns it, a caller reads `IsNew`), `TestReaderNonNilSlices` (new) asserting: the fake's empty answer has a non-nil `Findings` and `Evidence`), `TestRuleIDsAreDistinct` (new) and `TestDataCheckDSNEnvName` (new) asserting: the constant equals `OT_PG_DSN_DATACHECK`).
3. `docs/contracts/store/finding.md` (new) lists the package, the three severities with their meaning, the rule ids, the evidence vocabulary and the sentence "a finding holds counts and fixed words, never free text, an id or a secret", one paragraph each.

### 2 data-check-contract (interface)

Module: contracts. Size: M.
Order: first.
Files: `contracts/schemas/page.data_health_page.v1.json`, `web/src/generated/data_health_page.v1.ts`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/data_health_page_v1_test.go`, `docs/contracts/page-schemas.md`
New files: `contracts/fixtures/page.data_health_page.v1.datacheck.json` (new)
Contracts: page.data_health_page v1 (optional field `data_check` added; `schema_version` stays 1)
Design: docs/design/data-check.md#Interface
Acceptance:
1. The schema gains the optional property `data_check` with the fields of Entry 2 of the Interface section and no others, `additionalProperties: false` at every new level, `severity` as an enum of `error`, `warn`, `unknown`, `data_check` nullable and not in `required`, and the gap `reason` enum gains `data_check_unavailable`; the generated type is produced from the schema and not edited by hand; no existing property or enum value changes.
2. `ok.json` is unchanged in content except that it passes as before; `datacheck.json` carries a `data_check` with one finding of each severity, one with `is_new` true, and `gaps` empty; it is labelled invented in `MANIFEST.synthetic-v1.json`.
3. `data_health_page_v1_test.go` has `TestDataCheckFixtureValidates` (new) asserting: `datacheck.json` passes), `TestDataCheckRejectsUnknownField` (new) asserting: a finding with an extra key fails), `TestDataCheckSeverityEnum` (new) asserting: `severity: "fatal"` fails) and `TestOldFixturesStillValidate` (new) asserting: `ok.json`, `gap.json`, `gateways.json` pass).
4. `docs/contracts/page-schemas.md` states that the field is optional under `docs/design/repeated-failures.md` Decision 9 and that every severity value has a glossary row.

### 3 data-check-schema

Module: store. Size: M.
Order: after 1.
Files: `internal/store/setup/setup.go`, `internal/store/setup/setup_test.go`
New files: `migrations/0073_data_check.sql` (new), `internal/store/setup/data_check_pg_test.go` (new), `docs/contracts/store/finding.md` (new)
Contracts: unchanged
Design: docs/design/data-check.md#Decisions (7, 10, 14)
Acceptance:
1. `migrations/0073_data_check.sql` (new) has Up and Down; Up creates schema `datacheck` with the tables `run`, `finding`, `notice` of Decision 7 (`finding.severity` has a CHECK of the three values; `finding` has a unique key on `(run_id, rule, dataset, day, subject)`) and the views `rd.data_check_run_r1` and `rd.data_finding_r1` (every column named, owner `ot_owner`), with the grants of Decision 10 and no UPDATE, DELETE or TRUNCATE for any role but the owner; Down drops what Up made and revokes what it granted.
2. The role table of `setup.go` gains the role `ot_w_datacheck` with its line `OT_PG_DSN_DATACHECK`, and its own constant for the line name; `setup_test.go` expects 22 roles. `TestDataCheckLineEqualsStoreConstant` (new) asserts that the constant equals `finding.DSNEnvDataCheck`, imported by the test only.
3. `data_check_pg_test.go` (tag `pg`) has `TestDataCheckGrants` (new) asserting: `ot_w_datacheck` can INSERT into the three tables and cannot UPDATE or DELETE; `ot_ro_pages` can SELECT the two views and cannot SELECT the tables; `ot_w_datacheck` can SELECT the eight views of Decision 10) and `TestMigrate0073Down` (new) asserting: after Down the schema and views are gone).
4. `git diff --name-only BASE -- migrations` lists only the one new migration file.

### 4 data-check-writer-reader

Module: store. Size: M.
Order: after 1 and 3.
Files: `internal/store/findingpg/writer.go` (new), `internal/store/findingpg/reader.go` (new), `internal/store/findingpg/writer_test.go` (new), `internal/store/findingpg/findingpg_pg_test.go` (new)
Contracts: unchanged
Design: docs/design/data-check.md#Decisions (7, 8)
Acceptance:
1. `OpenWriter` and `OpenReader` have the types `finding.OpenWriterFunc` and `finding.OpenReaderFunc`; `InsertRun` stores the run and its findings in one transaction and returns the run with `RunID` and each `IsNew` set; `Latest` returns the run with the highest `check_day`, then the highest `run_id` (a forced run of an old day does not replace the newest day on the page) and its findings through the views, ascending by severity (`error`, `warn`, `unknown`), then dataset, day, rule; with no run it returns a zero `Run` with an empty non-nil `Findings`.
2. `IsNew` follows Decision 8: a finding whose key is in no earlier run is new; a finding whose key is in an earlier run that has no notice row is new again; a finding whose key is in an earlier run that has a notice row is not new; after `MarkNoticed` of the run, an equal finding in the next run is not new (`findingpg_pg_test.go`: `TestIsNewUntilNoticed` (new)).
3. `MarkNoticed` twice for one run writes one row and returns nil `TestMarkNoticedTwice` (new).
4. `writer_test.go` has `TestEvidenceIsFixedVocabulary` (new) asserting: `InsertRun` returns an error for an evidence name outside the vocabulary of `finding` and for a `Subject`, `Dataset` or `Rule` longer than 120 bytes), so no free text and no id can be stored.

### 5 data-check-probe

Module: store. Size: M.
Order: after 1 and 3.
Files: `internal/store/findingpg/probe.go` (new), `internal/store/findingpg/probe_test.go` (new), `internal/store/findingpg/probe_pg_test.go` (new)
Contracts: unchanged
Design: docs/design/data-check.md#Decisions (3, 4)
Acceptance:
1. `OpenProbe` has the type `finding.OpenProbeFunc` and reads only the views of Decision 10, never a table.
2. `RepeatedReadings` takes for each name and day of the window the rows with `state = 'ok'` of the highest `version` from `rd.reading_r1`, compares each `value` with the same subject's ok row of the previous day that has one, and returns `OK` and `Same` `TestRepeatedReadingsHighestVersion` (new) under the `pg` tag, two versions of a name on one day, only the higher counts; `TestRepeatedReadingsSkipsMissingInput` (new) under the `pg` tag).
3. `LateOpenQuotes` counts per session day and currency the rows of `rd.option_open_r1` and those whose `received_at` falls on a later local day than `session_day` in `America/New_York` for `USD` and `Asia/Tokyo` for `JPY`; a currency with no zone is counted in `Rows` and never in `Late` `TestLateOpenQuotesZones` (new) under the `pg` tag, a JPY row received at 09:17 Tokyo the day after its session day is late, a USD row received at 22:00 New York the same day is not).
4. `NewestCandidateRuns` returns per (day, kind) the newest run's row count and `computed_at` from `rd.option_candidate_r1`; `SettingsChangedAt` returns the newest `set_at` over `rd.rule_param_r1` and `rd.option_exclusion_r1`, `false` when both are empty; `OpenGaps` returns the rows of `rd.job_gap_r1` with an empty `cleared_at`, ascending by `first_seen_at`.
5. Every method returns non-nil slices and no row of any table, only the counts of the fact types (`probe_test.go`: `TestProbeReturnsCountsOnly` (new), reflection over the fact types: no string field other than a name, a currency, a kind, a reason, a day or a job or dataset id).

### 6 data-check-job

Module: jobs. Size: M.
Order: after 1.
Files: `internal/jobs/datacheck/doc.go` (new), `internal/jobs/datacheck/job.go` (new), `internal/jobs/datacheck/job.toml` (new), `internal/jobs/datacheck/manifest.go` (new), `internal/jobs/datacheck/rule.go` (new), `internal/jobs/datacheck/rules.go` (new), `internal/jobs/datacheck/dedupe.go` (new), `internal/jobs/datacheck/announce.go` (new), `internal/jobs/datacheck/job_test.go` (new), `internal/jobs/datacheck/manifest_test.go` (new), `internal/jobs/datacheck/dedupe_test.go` (new), `internal/jobs/datacheck/announce_test.go` (new), `docs/contracts/jobs/data_check.md` (new)
Contracts: unchanged
Design: docs/design/data-check.md#Decisions (1, 2, 5, 8, 9, 11)
Acceptance:
1. `New(st, clk)` returns an error naming a key when `Cov`, `Probe`, `Out` or the clock is nil, and accepts a nil `Announce`; `job.toml` has the schedule of Decision 2 and `id = "data_check"`; `Plan` returns one key of dataset `datacheck.run` and a hash of the manifest version and the day `TestPlanOneKey` (new).
2. `rules.go` holds `defaultRules()` returning the rules in order; at this entry it returns an empty slice, and with no rule `Run` returns `Returned` 0, `Usable` 0 and the error `no_rules` and never calls `Out.InsertRun`, so no run is stored (`TestNoRulesIsAnError` (new)).
3. `Run` builds the windows (the last `CheckWindowSessions` session days of each calendar of `Expected` and of XNYS and XTKS, before the check day, from `Cov.Sessions`), runs each rule, turns a rule error into one finding of severity `unknown` with the evidence `read_failed` 1, the rule id as its rule and as its dataset, and the check day as its day, stores one run through `Out.InsertRun`, and returns `Returned` = rules, `Usable` = rules without error `TestRuleErrorBecomesUnknown` (new; `TestAllUnknownIsUsableZero` (new)).
4. `dedupe.go` drops a `gap_unclosed` finding whose (dataset, day) equals a `capture_gap` finding of the same run and keeps every other finding `TestDedupeKeepsOneProblemOnce` (new).
5. `announce.go` builds one Chinese text from the findings with `IsNew`: at most 10 lines ordered `error` before `warn` before `unknown`, each line rule label, dataset, day and counts, then "还有 N 条见页面" when more than 10; no new finding, no call to `Announce` `TestAnnounceCapsTenLines` (new; `TestNoNewNoMessage` (new); `TestTextHasNoIDs` (new): the text built from findings of every rule holds only the fixed words, the dataset, the day and digits).
6. After `InsertRun` and a successful `Announce` the job calls `MarkNoticed` once; when `Announce` fails the job returns a retryable error and does not call `MarkNoticed` `TestSendFailureNoNotice` (new); with a nil `Announce` it calls `MarkNoticed` never and returns ok `TestNilAnnouncerStoresOnly` (new).
7. The tests use a fake `finding.Writer`, `finding.Probe`, `coverage.Reader`, a `clockwork` fake clock and `t.Parallel()`; no test sleeps.

### 7 data-check-rules-wrong

Module: jobs. Size: M.
Order: after 5 and 6.
Files: `internal/jobs/datacheck/rules.go` (new), `internal/jobs/datacheck/rules_wrong.go` (new), `internal/jobs/datacheck/rules_wrong_test.go` (new)
Contracts: unchanged
Design: docs/design/data-check.md#Decisions (3)
Acceptance:
1. `rules_wrong.go` has the functions `reading_repeat`, `option_open_late_quote` and `candidates_older_than_settings` (as Go identifiers `ruleReadingRepeat`, `ruleOpenLateQuote`, `ruleCandidatesStale`) and `defaultRules()` in `rules.go` returns them in that order.
2. `ruleReadingRepeat`: for the facts `{name market.rv20, day D, OK 35, Same 35}` it returns one finding of severity `error`, dataset `market.rv20`, day D, evidence `ok` 35 and `same` 35; for `{OK 4, Same 4}` none (below 5 subjects); for `{OK 40, Same 5}` none; for `{OK 10, Same 5}` one. `TestReadingRepeat` (new) is a table of these rows.
3. `ruleOpenLateQuote`: `{day D, USD, Rows 5548, Late 0}` gives none; `{day D, JPY, Rows 183, Late 183}` gives one `error` finding, dataset `obs.option_open`, evidence `rows` 183 and `late` 183 `TestOpenLateQuote` (new).
4. `ruleCandidatesStale`: with the newest candidate day D, a run computed at 09:00 and `SettingsChangedAt` 11:30 the same day gives one `warn` finding, dataset `candidates.option_candidate`, evidence `rows` and `hours_behind` 2; a run computed after the change gives none; `SettingsChangedAt` false gives none; candidate runs of an older day than the newest give none `TestCandidatesOlderThanSettings` (new).
5. A `Probe` method that returns an error makes the rule return that error (the job turns it into `unknown`, entry 6); `TestRuleReturnsProbeError` (new) covers each of the three.

### 8 data-check-rules-gap

Module: jobs. Size: M.
Order: after 7.
Files: `internal/jobs/datacheck/rules.go` (new), `internal/jobs/datacheck/rules_gap.go` (new), `internal/jobs/datacheck/rules_gap_test.go` (new)
Contracts: unchanged
Design: docs/design/data-check.md#Decisions (4, 5, 6)
Acceptance:
1. `rules_gap.go` has `ruleCaptureGap`, `rulePlanAbsent` and `ruleGapUnclosed`, and `defaultRules()` returns the six rules, the three of entry 7 first.
2. `ruleCaptureGap`: a cell `{dataset obs.price_bar, day D, Keys 63, OK 0, Missing 60, Expired 3}` with D in the window gives one `warn` finding with evidence `keys` 63, `missing` 60, `expired` 3; a cell with `Missing 0` and `Expired 0` gives none; a cell on the check day gives none (the window excludes it) `TestCaptureGap` (new).
3. `rulePlanAbsent`: with `Expected` `{obs.option_eod, XNYS}` and no cell of that dataset on one session day of the XNYS window, one `warn` finding for that (dataset, day) with evidence `cells` 0; a dataset with a cell on every window day gives none; a dataset of calendar `XTKS` is judged by the XTKS window only. `TestPlanAbsent` (new) includes a US holiday that is not in the XNYS window.
4. `ruleGapUnclosed`: an open gap of a window day first seen 30 hours before `Now` gives one `warn` finding (dataset of the gap, or the job id when the dataset is empty, day of the gap, evidence `age_hours` 30); one first seen 20 hours before gives none, and an open gap of a day before the window gives none. `TestGapUnclosed` (new) covers these.
5. `TestRuleGapReturnsProbeError` (new) covers `OpenGaps` and `Cov.Cells` errors.

### 9 data-check-page-builder

Module: pages. Size: M.
Order: after 1 and 2.
Files: `internal/pages/data_health_page/page.go`, `internal/pages/data_health_page/builder.go`, `internal/pages/data_health_page/zh_texts.go`
New files: `internal/pages/data_health_page/datacheck.go` (new), `internal/pages/data_health_page/datacheck_test.go` (new)
Contracts: unchanged
Design: docs/design/data-check.md#Decisions (12, 13)
Acceptance:
1. `Input` gains `Checks finding.Reader` (nil: `data_check` is null and no gap; a reader with no run: `data_check` is the object with `none_text`, empty `findings`, zero counts); the builder always writes the key `data_check` on every document ("always present", `TestDataCheckKeyAlwaysPresent` (new): nil reader, empty reader, a run).
2. With a run of two findings the document's `data_check` has `counts` per severity, `rules_total` and `rules_unknown`, and one finding row each with `label` and `severity_text` from `zh_texts.go` (`错误`, `缺口`, `无法判断`), `evidence_text` built from the counts as a Chinese sentence naming each count (for `reading_repeat` with `ok` 35 and `same` 35: "35 个读数中 35 个与前一日相同"), `why_text` one sentence for the rule, and `is_new` carried `TestDataCheckRows` (new) decodes the fixture `datacheck.json` into the view-model).
3. Findings are ordered `error`, `warn`, `unknown`, then dataset, day, rule, and the order is the builder's `TestDataCheckOrder` (new).
4. A reader error gives `data_check` null and one `gaps` entry `{kind: gap, reason: data_check_unavailable, text: ...}`; the rest of the page is built as before `TestDataCheckUnavailable` (new).
5. The glossary gains rows for `错误`, `缺口`, `无法判断` and `新`, each one sentence; `TestSeveritiesHaveGlossary` (new) asserts a glossary row for each of the three severities.
6. No string of the block is English except dataset and rule identifiers (the rule identifiers are shown next to the Chinese label); `cmd/ot/english_test.go` is not edited by this entry and is run in CI.

### 10 data-check-web-block

Module: web. Size: S.
Order: after 2.
Files: `web/src/pages/data_health/DataHealthPage.tsx`
New files: `web/src/pages/data_health/DataCheckBlock.tsx` (new), `web/test/pages/data_health/data_check.test.tsx` (new)
Contracts: unchanged
Design: docs/design/data-check.md#Decisions (12)
Acceptance:
1. `DataCheckBlock` renders the `title`, the `note`, the last run line (`check_day`, `finished_at`, `rules_unknown` of `rules_total`), the three counts, and one row per finding with the label, dataset, day, `severity_text`, `evidence_text` and `why_text` as given, and a `新` mark when `is_new`; it computes, sorts and words nothing.
2. With `data_check` whose `none_text` is non-empty (the builder's no-run object) the block shows `none_text` and no findings table; with `data_check` null the block is not rendered; with `data_check` absent (an older document) the block is not rendered; `DataHealthPage.tsx` renders it under the coverage grid.
3. `data_check.test.tsx` has `TestRendersFindingsAsGiven` (new) asserting: the fixture `datacheck.json` renders every `evidence_text`), `TestNoRunShowsNoneText` (new: a `data_check` with non-empty `none_text` and empty `findings` shows `none_text`), `TestNullRendersNothing` (new: a null `data_check` renders no block) and `TestAbsentRendersNothing` (new); `tsc`, `eslint`, `vitest` and the build pass (CI).

### 11 data-check-wiring

Module: ot. Size: M.
Order: after 4, 5, 6, 8 and 9.
Files: `cmd/ot/runonce.go`, `cmd/ot/jobconsole_jobs.go`, `cmd/ot/data_health_site.go`, `cmd/ot/data_health_site_test.go`
New files: `cmd/ot/wire_datacheck.go` (new), `cmd/ot/wire_datacheck_test.go` (new), `docs/contracts/ot/jobs/data_check.md` (new)
Contracts: unchanged
Design: docs/design/data-check.md#Decisions (6, 9, 10, 11)
Acceptance:
1. `wire_datacheck.go` registers the job family `data_check` with `registerJob` (`order: 240`: `recon_bars` holds 230, orders are unique steps of ten, `cmd/ot/jobreg.go:18`); while `dataCheckDSN` (new field of `runOnceConfig`, line `OT_PG_DSN_DATACHECK`, 2 added lines in `runonce.go`) is empty the job is held with the reason "OT_PG_DSN_DATACHECK is not set (run setup pg)" `TestDataCheckHeldWithoutDSN` (new); with it set the job is built from `findingpg.OpenWriter`, `findingpg.OpenProbe`, `ops.OpenCoverage`, the expected list of Decision 6 and `telegramSender(a)` wrapped as an `Announcer`, nil when the sender cannot be built `TestNilAnnouncerWithoutToken` (new).
2. `jobTable` has `{ID: "data_check", Kind: kindAnyDay, Force: true}` and `dataHealthNoLedger` has `"data_check"` with its reason; the existing `TestEveryJobHasRowOrReason` passes.
3. `dataCheckNotDaily` is a fixed list with a reason per entry and `TestEveryLedgerJobIsDailyOrReasoned` (new) fails for a `jobTable` job that has a ledger dataset in `dataHealthManifests` and is in neither the expected list nor `dataCheckNotDaily`.
4. The data health site opens the finding reader with the pages line (`store.DSNEnvPages`) and passes it as `Input.Checks`; when it cannot be opened the page is built with a nil reader and no gap, as entry 9 acceptance 1 gives for a nil reader (`data_health_site_test.go`: `TestDataHealthSiteChecksReader` (new)).
5. `docs/contracts/ot/jobs/data_check.md` (new) states the schedule, the six rules with their severity, "reports only", the line `OT_PG_DSN_DATACHECK` and the message rule.

## Checked by CI

`go test ./...` (entries 1, 3, 4, 5 (the `pg` tests run only in GitHub CI), 6, 7, 8, 9, 11), `go run ./scripts/arch` and go-arch-lint (the new import edges of Modules), the schema test of entry 2, `ci-web` (`tsc`, `eslint`, `vitest`, the build; entry 10), `go run ./scripts/precheck`, and the migration check with PostgreSQL 17 and TimescaleDB (the pin used by the migration check workflow). After deploy the window runs `ot setup pg`, `migrate`, `setup pg`, then runs `data_check` once from `/job-console` for the latest session day, renders `/data-health` and checks the block with a screenshot (owner rule of 2026-11-14); that check is not an acceptance item.

## Left out

- **Repair (the owner's second wish).** The first version only reports (owner: "or at first only reports it"). The repair stays the button of `docs/design/data-health.md`. A later design may let a finding carry a "repair" button that enqueues the existing request (`settings.job_request`, `jobrequest.Poster.Enqueue`) for the finding's dataset and day; it needs the verdict rule of data-health Decision 5 per finding and is not started here.
- **A rule for a change of rule code.** `ruleVersion` in `internal/jobs/optioncandidates/convert.go` is `"put_v1"` and does not move when the rule's code changes, so "candidates not recomputed after a code change" (the buy-price hint build of sweep rows 36 and 37) cannot be seen in data. The fix is a follow-up in the jobs module: the job bumps its rule version with each rule change and exports it; `datacheck` then takes it as an expected value like `Expected`. Until then rule 3 covers stored settings only.
- **Held contracts without an end-of-day quote and earnings coverage** (sweep rows 9, 10, 17, 18). Each needs account-derived input (positions) or a provider-specific expectation ("not announced within 3 months" versus a matching fault) that the sweep itself did not settle. Rules are one function each; these are the first two to add once their expectation is stated.
- **Row-level value checks** (price outside a band, IV outside a band). No stated band; "numbers sourced or trial" (owner 2026-11-13) applies, so a band needs its own entry with a source.
- **Quiet hours.** The message goes out through the sender directly at the slot; it does not read `notify/core/quiet.go`. Moving the slot is one line of `job.toml`.
- **Wrong candidates from wrong inputs (sweep row 37), stale readings other than the four named (row 33), and rows that vanish (row 23, the scheduled rows missing).** Each needs a rule with an expected value; the late-quote rule catches the mislabelled inputs of row 37 at the source.
- **A check on a day XNYS is closed.** The job's calendar is XNYS; a Japanese-only day is checked on the next US session day (Doubts, 1).
- **Settings switches of underlyings.** The names switched off on `/underlyings` are not in any plan, so no rule reports them (owner rule "settings off is not a gap"); the candidates rule does not read `settings.underlying`.

## Doubts

1. On a US holiday with an open Japanese session (today, 2026-11-26) no run happens; the next run covers it by its 5-session window. A second manifest on XTKS would double the messages; not done unless the owner asks.
2. `recon_bars` names ledger dataset `recon.run` and is listed in `dataHealthNoLedger`; how the page treats a ledger dataset with no row in `dataHealthManifests` was not run. Entry 11 copies the precedent and its acceptance 2 runs the existing test; if the page shows a `datacheck.run` row with no job, that is a page change inside entry 9, to be reported in the pull request.
3. `ReadingViewReader`'s comment in `internal/store/api.go` still describes `rd.reading_r1` as a winner view with revisions; since 0045 it is not (one row per key). Entry 5 does not rely on the comment; correcting it is an interface change of its own and is not part of this design.
4. The first run after deploy finds every old problem of the window as new; the message lists 10 and the rest is on the page only (Decision 8). The owner may prefer a silent first run; one flag of `Announce` would do it and is not built unless asked.

## Adversarial

Review by a fresh read-only sub-agent on origin/main `8241d7a0`; each point is answered below and the design text above was changed where the answer says so.

1. **Gap vs goal.** Adopted in part. Row 37 (wrong inputs) and the vanished rows of row 23 are named under Left out; the late-quote rule catches the mislabelled inputs at the source. Row 36 (code change) stays Left out with its fix (bump the rule version). `capture_gap` is kept on purpose: its value is the message, since the owner said he is not told today.
2. **Biggest risk.** Adopted. `gap_unclosed` now only reads gaps whose day is inside the window (Decision 4). `datacheck.run` is skipped by `capture_gap` and `plan_absent` (Decision 5). The first-run flood is bounded by the 5-session window and the 10-line cap; that the rest is on the page only is stated in Decision 8 and Doubts 4, with the silent-first-run flag as the owner's option.
3. **Simpler way.** Not adopted. The notify gap path only knows ledger gaps (`GapSource.OpenGaps`) and has no wrong-data rule; `alerts.sent` is keyed by (rule, day), which cannot say "this finding is new" per dataset and day; computing in the page builder gives no daily message and no history. The role and schema copy `recon_bars` (`ot_w_recon`, schema `recon`), the repo's own precedent for a checking job. The cost is the 11 entries, each one module.
4. **Lock-in.** Accepted as stated in Decisions 7, 10, 13. Decision 5 now says a finding is keyed by rule, dataset, day and subject, not by counts, so a worsening condition is not re-announced; that is the chosen behaviour (the page shows current counts). The empty `subject` stays for later per-key rules.
5. **False claims.** Corrected: `readings.reading` has one row per (name, version, subject_kind, subject, day) since `0045_reading_key.sql`; Decision 3, Doubts 3 and entry 5 now say so and the test is `TestRepeatedReadingsHighestVersion`. "`internal/store/api.go` is over 800 lines" now says it is 800 lines. The migration number is 0073: the open design `earnings-fallback` (pull request #4142) holds `0072_earnings_source_yfinance.sql`. `TestEveryEnumHasGlossary` is replaced by a new `TestSeveritiesHaveGlossary`. #4150 is called "open" and only as the cause fix of row 32.
6. **Likeliest slip.** Fixed: the job order is `240` (entry 11). Entry 11 is after entries 7 and 8, so no wired job runs without rules; entry 6 alone is not wired. `Latest` takes the highest `check_day` then `run_id`, so a forced old day does not replace the page (entry 4). Entry 11 says "a few lines" of `runonce.go`. The setup test imports `finding` in a test file only, as `setup_test.go` already imports `store/recon` (line 13), which arch-lint accepts. `dataCheckNotDaily` is judged by hand per job with a reason, and `TestEveryLedgerJobIsDailyOrReasoned` forces the decision; a wrong judgement gives a `plan_absent` finding the owner can read and the list is one place to fix.

## 摘要

新增一个每天固定时间（美股收盘后两小时）运行的只报告不修复的数据检查任务 `data_check`。六条规则：三条查"数据是错的却标成正常"（读数与前一日相同、期权开盘报价的接收日晚于交易日、候选在设置变更后未重算），三条查"缺口没人发现"（账本里 missing/expired 的格子、某数据集整天没有计划、`job_gap` 长期未关闭）。结果存入独立的 `datacheck` 库表（迁移 0073，新角色 `ot_w_datacheck`），在数据健康页新增"数据检查"区块，只要出现从未发送过的新问题就发一条 Telegram 消息。修复留在原有的数据健康页按钮，本版不做。
