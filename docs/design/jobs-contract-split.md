# Jobs contract split: one document per job, so issues on different jobs never share a file

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-10
Revision: 1

Status: draft for design review.

Source: the window's reading of the queue on 2026-11-10: five open issues, #1275 (`optioneod`), #1291 (`computefigures`, valuation), #1294 (`accountcapture`), #1295 (`earningsrefresh`) and #1299 (`dividendcapture`), each edits only its own job's text, yet all five name `docs/contracts/jobs.md` in `Files:` and so run one after another. This is step c that `docs/design/jobs-api-slim.md` leaves out ("splitting `docs/contracts/jobs.md` into one document per job package: step c, a later design"); its sibling for the other hotspots is `docs/design/parallel-hotspots.md` (Decision 3, one contract document per package) and `docs/design/ot-wire-split.md`.

Read on origin/main at 994216f4: `docs/contracts/jobs.md` (208 lines, 22 `##` sections), `docs/design/modules.md` (jobs row, line 93; docs row, line 102), `docs/contracts/ot/jobs/*.md`, `cmd/ot/jobreg_test.go`, `docs/design/jobs-api-slim.md`, `docs/design/parallel-hotspots.md`, `docs/design/ot-wire-split.md`, `docs/design/contract-docs.md`, the Files lines of #1275, #1291, #1294, #1295 and #1299 (`gh issue view`), and `gh pr list --state open` (two open pull requests, #1301 and #1290; neither touches `docs/contracts/jobs.md`).

## What main holds today

`git grep -n "docs/contracts/jobs" origin/main -- . ':!docs/design'` finds, outside `docs/design/`, only these references (no Go file, script, workflow or test names the file; `cmd/ot/jobreg_test.go` reads `docs/contracts/ot/jobs/<id>.md`, a different path):

- `docs/contracts/ot/jobs/<id>.md`, line 9 of eleven files (`account_capture`, `calendar_refresh`, `compute_figures`, `dividend_capture`, `earnings_refresh`, `fred_series`, `option_candidates`, `option_eod`, `option_eod_jp`, `option_ledger`, `quote_snapshot`): "see `docs/contracts/jobs.md` for the job"; `option_eod_jp.md` names it twice in that line. These are paths of the `ot` row (`docs/contracts/ot/**`).
- `docs/design/modules.md`: the jobs row of the first table (line 93, paths cell) and the jobs row of the second table (line 120, the contract-document cell "job.toml per package and docs/contracts/jobs.md").
- 26 files under `docs/design/` name the path in prose or in a `Files:` line (19 in a line that starts `Files:` or `- Files:`). They are merged designs and stay as written.

A search for the string `jobs.md` alone also hits `option-jobs.md` (line 34 of `docs/contracts/jobs.md`), so every check below uses the full path `docs/contracts/jobs.md`.

Which section holds what, at 994216f4 (heading, first line): Responsibility 3, Inputs 9, Outputs 15, Invariants 23 (these three and the next are all the `fred_series` job), `option_eod` 32, Schedule 46 (the rule for three jobs), `compute_figures` 58, `SeriesHistoryLoader` 70, `calendar_refresh` 80, `earnings_refresh` 91, Version history 102 (17 `- v1: ` bullets in total, 16 here and one at line 208, each about one job), `dividend_capture` 121, Contract tests 131 (bullets that list tests of several jobs, and one paragraph about `fred_series`), Jobs and slots 143 (`fred_series`), Forbidden 147 (a rule for every job), Option ledger 153, Cut-over 159, Option candidates 166, `bars_refresh` 179, `fx_refresh` 187, `option_candidates settings version` 194, `fundamentals` 200 (its last line, 208, is a Version bullet).

The five issues show that the job text alone is not enough: #1275 and #1291 check a `- v1: ` Version bullet of their job in the shared section; #1294 adds `account_capture` text, for which no section exists yet.

## Goal

An issue that changes one job edits that job's contract document and no file another job's issue edits: the contract text of each job is a file of its own, and the lists that mix jobs (Version history, the test lists of Contract tests) are cut by job. Five issues on five jobs no longer share `docs/contracts/jobs.md`.

## Decisions

1. One document per job: `docs/contracts/jobs/<name>.md`, where `<name>` is the job id when there is a job id (`fred_series`, `option_eod`, ...) and the package name otherwise (`serieshistory`, `cutover`). The sections of `docs/contracts/jobs.md` move into these files with their text unchanged; nothing is rewritten, reordered inside a section, or deleted.
2. Where each section goes (heading of `docs/contracts/jobs.md` at 994216f4, then file):
   - `fred_series.md`: Inputs, Outputs, Invariants, Jobs and slots (all four describe `fred_series` alone), the `fred_series` test bullets of Contract tests (`plan_test.go`, `window_test.go`, `run_test.go`) with the paragraph "There is no test yet", and the `fred_series` Version bullet.
   - `option_eod.md`: `option_eod`, the `internal/jobs/optioneod` test bullet of Contract tests, and three Version bullets (the first release, the watch list with `min_dte`, `max_dte` and `max_strikes`, `jobs.OptionEodWindow`).
   - `option_eod_jp.md`: two Version bullets (`option_eod_jp`, `optioneod.NewJPWithWatch`); no section of its own exists today.
   - `compute_figures.md`: `compute_figures` and three Version bullets.
   - `serieshistory.md`: `SeriesHistoryLoader` and its Version bullet.
   - `calendar_refresh.md`: `calendar_refresh` and its Version bullet.
   - `earnings_refresh.md`: `earnings_refresh`, its test bullet of Contract tests (`internal/jobs/earningsrefresh/run_test.go`) and its Version bullet.
   - `dividend_capture.md`: `dividend_capture` and its Version bullet.
   - `option_ledger.md`: Option ledger and two Version bullets.
   - `cutover.md`: Cut-over.
   - `option_candidates.md`: Option candidates and `option_candidates settings version`.
   - `bars_refresh.md`: `bars_refresh`. `fx_refresh.md`: `fx_refresh`. `fundamentals.md`: `fundamentals`, with its Version bullet at the end as it stands.
   - `position_marks.md`: its Version bullet (no section exists today).
   That is 15 files. The 17 moved headings are Inputs, Outputs, Invariants, Jobs and slots, `option_eod`, `compute_figures`, `SeriesHistoryLoader`, `calendar_refresh`, `earnings_refresh`, `dividend_capture`, Option ledger, Cut-over, Option candidates, `bars_refresh`, `fx_refresh`, `option_candidates settings version`, `fundamentals`.
3. `docs/contracts/jobs.md` stays as the index: the title, Responsibility, a new `## Job documents` (one link line per file, each starting `- [docs/contracts/jobs/<name>.md](jobs/<name>.md)`), Schedule, Contract tests and Forbidden. Responsibility and Forbidden keep the text they have. Schedule keeps its text: it is one table of slots and one rule for three jobs (`option_eod`, `fred_series`, `quote_snapshot`), so it cannot be cut by line without rewriting it (Left out). Contract tests keeps two bullets, the shared test rules: the bullet of `internal/jobs/api_test.go` (the tests of the interface file `api.go`, which one line lists for six jobs) and the bullet `schedule_test.go of each of the three packages`; every other bullet of the section moves, whole and in order, under a `## Contract tests` heading of the file of its job (the `fred_series` bullets and paragraph, the `earningsrefresh` bullet, the `optioneod` bullet). Version history is cut by bullet: each bullet goes under a `## Version history` heading of the file of its job, in the order it has now, and the index has no such heading. In a job file the order is: the moved sections, then `## Contract tests`, then `## Version history`.
4. Nothing that reads the file breaks. No Go file, script or workflow names `docs/contracts/jobs.md`; the references that exist keep working because the path stays and now holds the index: the eleven `docs/contracts/ot/jobs/<id>.md` lines still point to a document that lists the job's file, and the merged designs stay as history. The cut edits no `ot` path (they belong to the `ot` row, AGENTS.md "needing another file is reported, not edited"); pointing the eleven lines at the job files is a later `ot` issue (Left out).
5. Module: the jobs row gains `docs/contracts/jobs/**` so that the new files are jobs paths and a job issue may edit them (a path belongs to the first row whose glob matches; before this, the files would fall to the docs row, `docs/**`, and a job issue editing them would edit another module's path). The edit is made in this pull request (as `jobs-api-slim` does for its bullet), not in an issue, so the cut is one issue of one module.
6. Order and the open issues: the move issue is attached only when no open pull request and no queued issue has `docs/contracts/jobs.md` in its Files, and it carries `Depends on:` every open issue that edits that file at the time it is cut (the rule, not fixed numbers). As of now (2026-11-10, `gh pr list --state open`): pull requests #1302, #1303, #1304 and #1306 are open and edit `docs/contracts/jobs.md`, and #1291 has no pull request yet and names the file in its Files; the move waits for all of them. So no started pull request is closed, nothing is retried, and no Files line of a running issue is rewritten. Issues cut after the move name `docs/contracts/jobs/<job>.md`.
7. Review of the move is by a concatenation check, not by reading 208 lines: every non-blank line of the old file appears again, once, in the index or in a job file, and the only new lines are named kinds (acceptance 2 of the issue). So a reviewer proves "unchanged" with `git` and `diff`.
8. After the move, an issue on one job names only `docs/contracts/jobs/<job>.md` (its section, its Contract tests, its Version history); it names `docs/contracts/jobs.md` only when a shared part changes (Responsibility, Schedule, Forbidden, the two shared Contract tests bullets, the link list). A new job adds its file and one link line to the index: a known small collision between two issues that add a job at the same time, accepted (a generated index is not worth the code, Left out).
9. Own code and libraries: none; documentation only.

## Modules

No new row. One cell of `docs/design/modules.md` changes, edited in this pull request: the paths cell of the jobs row (line 93) goes from `internal/jobs/** docs/contracts/jobs.md` to `internal/jobs/** docs/contracts/jobs.md docs/contracts/jobs/**`. The jobs row of the second table (line 120, "job.toml per package and docs/contracts/jobs.md") is not edited: the path still names the index. No row before the jobs row matches `docs/contracts/jobs/**` (the contracts row names `docs/contracts/00-protocol.md`, `docs/contracts/datasets/**`, `docs/contracts/figures/**` and `docs/contracts/page-schemas.md`, nothing else under `docs/contracts/`).

## Interface

None. No Go interface file, schema, proto or dataset changes; documentation only.

## Issues

Acceptance is by reading, grep and git in a copy of the repository; test runs are under "Checked by CI". `BASE` is `git merge-base HEAD origin/main`. If the file changed between this design and the issue, the headings, not the line numbers of this design, are the reference; the counts below are re-read at the issue's BASE by the author, and the pull request says so if one differs.

### 1 jobs-contract-split

Module: jobs. Size: S (documentation only; 16 paths).
Files: `docs/contracts/jobs.md`, `docs/contracts/jobs/fred_series.md`, `docs/contracts/jobs/option_eod.md`, `docs/contracts/jobs/option_eod_jp.md`, `docs/contracts/jobs/compute_figures.md`, `docs/contracts/jobs/serieshistory.md`, `docs/contracts/jobs/calendar_refresh.md`, `docs/contracts/jobs/earnings_refresh.md`, `docs/contracts/jobs/dividend_capture.md`, `docs/contracts/jobs/option_ledger.md`, `docs/contracts/jobs/cutover.md`, `docs/contracts/jobs/option_candidates.md`, `docs/contracts/jobs/bars_refresh.md`, `docs/contracts/jobs/fx_refresh.md`, `docs/contracts/jobs/fundamentals.md`, `docs/contracts/jobs/position_marks.md`
Design: docs/design/jobs-contract-split.md (Decisions 1 to 4 and 6; the jobs row of `modules.md` already holds `docs/contracts/jobs/**` from this design's pull request)
Order: after every open issue that edits `docs/contracts/jobs.md` when it is cut (`Depends on:`, Decision 6); no order with any other issue.
Goal: the text of each job is in `docs/contracts/jobs/<name>.md`, unchanged; `docs/contracts/jobs.md` is an index of the rules shared by all jobs and a link to every job document.
Acceptance: run in a copy of the repository; `BASE=$(git merge-base HEAD origin/main)`, and `sec` is the shell function `sec() { awk -v h="$1" '$0==h {p=1; next} /^## / {p=0} p' | grep -v '^$'; }` (the lines of a section without its heading, blank lines removed).
1. `ls docs/contracts/jobs/` lists exactly the 15 names of the Files line (`fred_series.md`, `option_eod.md`, `option_eod_jp.md`, `compute_figures.md`, `serieshistory.md`, `calendar_refresh.md`, `earnings_refresh.md`, `dividend_capture.md`, `option_ledger.md`, `cutover.md`, `option_candidates.md`, `bars_refresh.md`, `fx_refresh.md`, `fundamentals.md`, `position_marks.md`), and `for f in docs/contracts/jobs/*.md; do test "$(head -1 "$f")" = "# $(basename "$f" .md)" || echo "bad $f"; done` prints nothing (the first line of each is `# <name>`, the job id or package name of Decision 1).
2. Nothing is lost, changed or duplicated. `git show $BASE:docs/contracts/jobs.md | grep -v '^$' | sort > old.txt` and `cat docs/contracts/jobs.md docs/contracts/jobs/*.md | grep -v '^$' | sort > new.txt` (in a temporary directory), then `diff old.txt new.txt | grep -c '^<'` prints 0 (no old line is missing or altered), and `diff old.txt new.txt | grep '^>' | grep -v -e '^> # ' -e '^> ## Version history$' -e '^> ## Contract tests$' -e '^> ## Job documents$' -e '^> - \[docs/contracts/jobs/'` prints nothing (the only new lines are the title of a job file, the headings `## Version history` and `## Contract tests` of a job file, the heading `## Job documents` and its link lines).
3. The index keeps the shared text. `grep '^## ' docs/contracts/jobs.md` prints exactly, in this order, `## Responsibility`, `## Job documents`, `## Schedule`, `## Contract tests`, `## Forbidden`, and `grep -c '^# jobs contract$' docs/contracts/jobs.md` prints 1. For each of `## Responsibility`, `## Schedule` and `## Forbidden`, `diff <(git show $BASE:docs/contracts/jobs.md | sec "$h") <(sec "$h" < docs/contracts/jobs.md)` prints nothing (with `h` set to the heading). `sec '## Contract tests' < docs/contracts/jobs.md | wc -l` prints 2, `sec '## Contract tests' < docs/contracts/jobs.md | grep -c 'api_test.go'` prints 1 and the same with `schedule_test.go` prints 1.
4. Every moved section is in its file once. For each of the 17 headings of Decision 2 (set `h`, for example `## option_eod`, `## Option ledger`, `## option_candidates settings version`), `grep -lx "$h" docs/contracts/jobs/*.md` prints exactly the one file Decision 2 names, and `diff <(git show $BASE:docs/contracts/jobs.md | sec "$h") <(sec "$h" < docs/contracts/jobs/<that file>)` prints nothing. In `fred_series.md` the headings `## Inputs`, `## Outputs`, `## Invariants`, `## Jobs and slots`, `## Contract tests` come in that order (`grep -n '^## ' docs/contracts/jobs/fred_series.md`).
5. Contract tests are cut by job. `grep -c '^- `internal/jobs/fredseries/[a-z_]*_test.go`' docs/contracts/jobs/fred_series.md` prints 3 and `grep -c 'There is no test yet' docs/contracts/jobs/fred_series.md` prints 1; `grep -c '^- `internal/jobs/earningsrefresh/run_test.go`' docs/contracts/jobs/earnings_refresh.md` prints 1; `grep -c '^- `internal/jobs/optioneod`:' docs/contracts/jobs/option_eod.md` prints 1; `grep -c 'There is no test yet' docs/contracts/jobs.md` prints 0; `grep -c '^## Contract tests$' docs/contracts/jobs/*.md` prints 1 for `fred_series.md`, `earnings_refresh.md` and `option_eod.md` and 0 for the other twelve.
6. Version history is cut by bullet. `grep -c '^- v1: ' docs/contracts/jobs.md` prints 0, `grep -c '^## Version history$' docs/contracts/jobs.md` prints 0, and `grep -c '^- v1: ' docs/contracts/jobs/*.md` prints, per file, `fred_series.md:1`, `option_eod.md:3`, `option_eod_jp.md:2`, `compute_figures.md:3`, `serieshistory.md:1`, `calendar_refresh.md:1`, `earnings_refresh.md:1`, `dividend_capture.md:1`, `option_ledger.md:2`, `fundamentals.md:1`, `position_marks.md:1` and 0 for the other four (17 in all, as at BASE). In the ten files that carry them under a heading (all but `fundamentals.md`) the bullets follow `## Version history` in the order they have at BASE, and the `fundamentals` bullet stays the last line of `fundamentals.md`.
7. The index links every file. `grep -c '^- \[docs/contracts/jobs/' docs/contracts/jobs.md` prints 15, and each of the 15 names of acceptance 1 appears in one of those lines.
8. No other path changes. `git diff --name-only $BASE` lists exactly the 16 paths of the Files line; in particular no path under `docs/contracts/ot/`, no `docs/design/` file and no Go file. `grep -c "docs/contracts/jobs.md" docs/contracts/ot/jobs/*.md` prints the same numbers as at BASE (1 line in each of the 11 files; `option_eod_jp.md` names the path twice in that one line).
9. `wc -l docs/contracts/jobs/*.md docs/contracts/jobs.md` shows no file over 100 lines.
Checked by CI: the `test` check (`Contracts: unchanged`: no file under `contracts/schemas`, `contracts/datasets` or `contracts/proto` changes), ci-go (generation reads `modules.md`, which this design's pull request, not the issue, edits). No Go test reads the moved text: `cmd/ot/jobreg_test.go` `TestJobDocsExist` reads `docs/contracts/ot/jobs/<id>.md` and those files do not change.

## Checked by CI

`test` (the pull request body carries `Contracts: unchanged`), ci-go (it runs when `docs/design/modules.md` changes and generates the arch files from the table, so it reads the new glob), and the design verdict for this file and for the one cell of `docs/design/modules.md`; nothing else reads these documents.

## Left out

- The eleven `docs/contracts/ot/jobs/<id>.md` lines ("see `docs/contracts/jobs.md` for the job"): they stay correct because the index lists every job document; pointing each at `docs/contracts/jobs/<id>.md` is an `ot` issue for later (they are `ot` row paths and this cut edits none).
- Schedule stays in the index: it is one table of slots and one rule for three jobs, so cutting it by job means rewriting it, which this cut does not do. The bullet of `internal/jobs/api_test.go` in Contract tests and the `schedule_test.go` bullet stay for the same reason (one line lists the tests of six jobs, one line is about three packages); an issue that changes one of them edits the index and says so in its pull request.
- Registering a new job still edits the index once (one link line) and `cmd/ot` (see `docs/design/ot-wire-split.md`); a generated index was not designed here: nothing needs it and the generator would be code for a list of 15 lines.
- The jobs row of the second table of `modules.md` (contract-document cell): left naming `docs/contracts/jobs.md`. Whether crew reads that column to check an interface issue is unknown from this repository; if it compares to the one file, `docs/contracts/jobs/` may need a crew change (as `docs/design/parallel-hotspots.md` Left out names for `store`, `compute` and `ot`).
- The 26 merged designs that name `docs/contracts/jobs.md`: history; a design that is still open and names it in a `Files:` line is cut after the move and names `docs/contracts/jobs/<job>.md`.
- Splitting the job documents further (for example `option_eod` into a file for its JP part) and reordering any text: a job document that grows large is split by the issue that grows it.
- Rejected alternatives: one issue per job document (15 pull requests that all start from the same file and so serialise, the cost this design removes); keeping Version history in the index and asking each issue to edit it (two of the five open issues do exactly that, so the hotspot would stay); a rule that rejects a `Files:` line naming a shared file (not designed; a later design).
- `internal/jobs/api.go` and `internal/jobs/api_test.go` (800 lines) remain a shared hotspot for every job whose constructor type is an interface change (#1291 names both): `docs/design/jobs-api-slim.md` addresses it, not this design.

## Adversarial

Read: the full answer is the file `adversarial-jobs-split.md` of the dispatcher window's scratchpad (kept on the machine, not in the repository).

1. Gap against the goal (Contract tests, Schedule, Forbidden stay in the index; `api.go` and `api_test.go` shared). Adopted: the per-job Contract tests bullets move (Decision 3, acceptance 5); Schedule and the two shared test bullets stay, with the reason in Left out; `api.go` and `api_test.go` are named as the remaining hotspot in Left out.
2. Biggest unnamed risk ("frozen" is a convention only). Adopted: Decision 8 states that a job issue names only its job file and the index only for a shared part; the convention shrinks to the shared parts, whose edits are rare.
3. Simpler way (split only Version history). Not adopted: the section bodies would still be shared, and each of the five issues edits its own section; cutting the Contract tests bullets too was adopted.
4. Lock-in (index links, 15 file names, job id against package name, crew's contract-document column). Accepted: the names are job ids and package names (Decision 1); that crew may read one file for the column is unknown from here and stays in Left out.
5. Likeliest false claim (the five issues "will open pull requests"). Adopted: Decision 6 states the fact as of now (#1302, #1303, #1304 and #1306 open and editing the file, #1291 without a pull request) and keeps the rule form; the counts of the review (17 bullets, 9 "Contract tests:" lines, 22 sections, no code reads the file) are as checked.
6. Slip (acceptance 4 with blank lines, `<(OLD)` pseudo-syntax, unpinned title). Adopted: blank lines are removed with `grep -v '^$'` in `sec`; every command is a shell command with `git show $BASE:docs/contracts/jobs.md`; acceptance 1 pins the first line of each job file as `# <name>`.
