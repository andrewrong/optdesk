# Option open gap, Japan: a stage that repairs the stored contracts live before the plan, and a postcondition that makes an empty stage visible

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 1

Source: owner ask optdesk-2940, 2026-11-12: "when the stored data is bad, collect on the spot; no one-wrong-step-then-all-wrong". Cause on main: the JP chain reads stored con_ids only (`docs/design/option-open-pull.md` Decision 1 and 21); the evening `option_open_resolve_jp` is the only stage that fills them, and when its input is wrong (a JP underlying with no con_id) the plan and the pull run on nothing and end green with zero rows.
Advisors (both read in the dispatcher scratchpad `designs/jp-gap-planb-inputs.md`): optdesk-advisor (00:10Z): a JP gap stage on the US pattern, one rule for both chains, a postcondition on resolve, plan and pull. Option-trader advisor (00:15Z): the old repo had no live fallback in the pull, only at close; cost: fully specified requests in groups of 30, pacer 5 in flight, 20 s timeout, 8170 contracts about 23 minutes; a 148-expiry open-strike refresh overloaded the gateway, hence a sigma band per name and a per-run cap; JP options are OSE.JPN, class = stock code, multiplier 100, taken from the chain definition and never SMART, monthly European, open slot 09:05 JST, empty books retried 09:15; at least 3 strikes per side; surfacing: stored 0 for an active underlying is a FAILED stage, and the pull states per name stored, fetched live, missing.
Read on origin/main at c96f513f: `AGENTS.md`, `docs/design/modules.md`, `docs/design/option-open-pull.md` (Decisions 1, 3, 5, 6, 12, 21, 22; issues 6, 7, 15, 17, 20), `internal/jobs/optionopen/{listed,jp,pull,plan,manifest,inputs}.go`, `internal/jobs/optionopen/job_{gap,resolve_jp,plan_jp,pull_jp}.toml`, `cmd/ot/wire_optionopen.go`, `internal/store/openquote/api.go`, `docs/contracts/runner.md` (Counting, Close table, optional methods), `docs/contracts/ot/jobs/option_open_resolve_gap.md`.
Not part of this design (registered drafts, assumed landed first): draft A (cmd/ot: the JP con_id of an underlying reaches the factory through its `entryFix`) and draft B (optionopen: an unresolved con_id is a key `Err`, and the summary of a run carries `run_err`).

Goal: on a JP session day, a watched JP name that has no stored option contract for the day is resolved live by a stage that runs before the plan, inside a cap that the gateway survives; and on both chains a stage that is given names and ends with nothing stored is a visible gap, never a green run. After this design the one rule reads: each stage reads its stored input, and the gap stage repairs that input live before the plan.

## What main holds today

- US: `option_open_resolve` (evening), `option_open_resolve_gap` (08:00 ET, `job_gap.toml`, `gap_only = true`, built by `NewGap` as `newListed(gapManifest, ...)`, `listed.go`), `option_open_plan` (08:45), `option_open` (09:45). The gap pass calls `ListChains` per stock and resolves only what neither the cache nor the marks hold; `GapOnly` is read at `listed.go` lines 122, 211 and 533.
- JP: `option_open_resolve_jp` (evening, `NewResolveJP`), `option_open_plan_jp` (08:15 JST), `option_open_jp` (09:05 JST). There is no JP gap pass: a JP name whose evening resolve failed or never ran has no stored contract at the plan and is planned as the underlying only.
- Wiring orders in `cmd/ot/wire_optionopen.go`: 120 resolve, 130 plan, 140 pull, 190 gap (US); 165, 170, 180 (JP).
- `contracts.Job` has the optional method `Postcondition(ctx, day, datasetID, subjectKey) (bool, error)` (`docs/contracts/runner.md`). A false or failing postcondition makes the run a verdict "done without a mark": the key ends `no_value` at the close and the ops page shows the gap through the existing Close table. No job of `internal/jobs/optionopen` has one.
- `master.option_list_run` (`openquote.ListRunRecord`, `OptionListStore.PutListRuns` and `ListRuns`, implemented by `internal/store/optionopenpg/lists.go`) holds one row per (session, underlying) and nothing in `internal/jobs` writes it today.

## Decisions

### The rule, stated once for both chains

1. Each stage of the chain (resolve, plan, pull) reads its stored input and nothing else. The gap stage is the only stage that may ask the broker for what the stored input lacks, and it runs before the plan: US `option_open_resolve_gap` at `session_open` minus 90 minutes (exists), JP `option_open_resolve_gap_jp` (new) at the same anchor on XTKS. The plan never asks the broker (Decision 1 of the pull design stands) and the pull never resolves at the open. This sentence goes into `docs/contracts/ot/jobs/option_open_resolve_gap.md` and the new JP file; it is the only place the rule is written.

### The JP gap stage

2. Job `option_open_resolve_gap_jp`, built by `NewGapJP` (new, `gapjp.go`) as `NewGap` over the JP rules, as `NewResolveJP` is `NewResolve` over them: `newListed(gapManifestJP, d, o)` with `d.Stored = r.stored(d.Stored)` and `d.Broker = jpBroker{...}`, and the JP rules are parsed from `planManifestJP` exactly as `NewResolveJP` does (the gap toml has no `[chain]` table, so the chain is written once). No new resolve code: the US gap logic is reused, and the JP rules (names without options, the chain of OSE.JPN with multiplier 100 and class = stock code, `N225` as the index of class 225) already exist in `jp.go`.
3. Manifest `job_gap_jp.toml`: zone `Asia/Tokyo`, calendar `XTKS`, `anchor = "session_open"`, `offset = "-90m"` (07:30 JST), `run_bound = "10m"`, `budget_minutes = 40` (ends 08:10, before the plan at 08:15), `gap_only = true`, `max_new = 300`, the band of `job_resolve_jp.toml` (`store_k = 1.75`, `store_drop = 0.25`, `min_dte = 14`, `max_dte = 90`, `calls = "held"`, `min_puts = 6`), and two new keys, `max_per_name = 60` and `dataset = "master.option_contract.gap_jp"` (its own dataset id, so that the first job to plan a key does not give a JP key the XNYS window of `master.option_contract.gap`; `Plan` of `listed.go` reads it, the other manifests keep their defaults) (the JP ceiling per stock of `job_plan_jp.toml`; the index keeps `max_per_index = 80` from the chain table).
4. Which names: the watched JP names (`jpRules.watch`, so names of `no_option` and the entries that are neither stock nor `N225` are out) that have no stored option contract of the band for the day. A name that has stored contracts for every expiry the chain lists inside the band gets no resolve. The stage is not free on a good day: `keyRun.run` calls `ListChains` once for every watched name before it looks at candidates (`listed.go`), so a good day costs one chain call per watched JP name and no resolve. Resolves are asked only for names with a gap.
5. The request shape, from the option-trader advisor: every request is fully specified (symbol, exchange `OSE.JPN`, currency `JPY`, multiplier `100`, trading class = stock code, expiry, strike, right), from the chain definition of `ListChains`, never SMART, one resolve in flight (`ResolveInFlight: 1`, as the US gap family), the 25 s call timeout of `broker.Options` (the service waits 20 s, `docs/design/broker.md`). The contracts of a name are those of the sigma band of Decision 3 of the pull design, nearest the spot first, so a name that hits its cap keeps the strikes the owner sells. A name is never asked as the whole chain (no 148-expiry refresh).
6. The caps, per run and per name, are values of the manifest: `max_new` (300 across all names, the first-day limit `OT_OPEN_RESOLVE_MAX_NEW` still overrides it) and `max_per_name` (60). A contract beyond a cap is `deferred`, counted in the summary line (`deferred=`, exists) and not an error; a run that defers is `Retryable` only through the existing `unsent` rule. Two timeouts in a row stop the run (the existing resolve rule), so a sick gateway is not hammered.
7. The gap stage writes one `ListRunRecord` per name it repaired (session = the key day, `Reason = "gap_live"`, `Listed` = the contracts it stored) through the optional port `ResolveDeps.Lists` (an `openquote.OptionListStore` reader and writer; nil means none are written). The same code path serves the US gap pass, so the pull can say which names were fetched live on both chains (Decision 10). No store or schema is added: the table and the PG implementation exist.

### The postcondition

8. A `Postcondition` on every stage, US and JP, on the key of the stage. The key of resolve and plan is one stock (names = 1; a `no_option` name is no key, `jpRules.watch` drops it); the key of the pull is one batch (names = the underlyings of its plan items). The method gets no run state, so each job keeps a small mutex-guarded map (day, key) to counts, filled by `Run` (`asked` of a resolve, rows asked of a pull) and read by `Postcondition` in the same process; after a restart the map is empty and only `stored` is tested. The rule is: false when names > 0 and stored = 0 and the run asked nothing to explain it. Where stored is read:
   - resolve and gap (`resolveJob`): stored = the stock's rows in `master.option_contract` (`Stored.Options.ListOptions`). True also when the run found the stored set complete (asked = 0 and stored > 0) or every contract of the band has a live mark in `master.option_not_listed` (an explained absence);
   - plan (`planJob`): stored = the stock's plan rows of the day and slot (`PlanReader.Plan`);
   - pull (`pull`, and `jpPull` by embedding): stored = rows of the batch with status `ok` or `partial` for the day (`openquote.OptionOpenReader.OptionOpen`; `no_data` and `not_listed` rows do not count) and the run asked at least one contract.
   The runner asks `Postcondition` only when the run ended without an error (`internal/runner/reconcile/runkey.go`): a run that returns `NoValue` or an error (draft B's unresolved con_id, the plan's "no stored close", the pull's "no plan") is already a verdict and is not asked. So the postcondition covers the case that the run ended clean and stored nothing, which is the failure of 2026-11-12; it adds nothing for an error that already reaches the key. A false postcondition is not an error of the run: the run ends `done` without a mark and the key is a gap through the existing Close table (`docs/contracts/runner.md`, row 4). That table acts at the close of the window (`after_close`, lateness 0 for these jobs), so the ops page shows the gap after the close, not at 08:15; the fault line (Decision 9) is the signal at the time of the run. The false postcondition does not stop the next stage: the stages are ordered by the clock only (Left out).
9. One fault line per false postcondition, through the `Log` the jobs already have (`ResolveDeps.Log`, `PullDeps.Log`; the plan gets the same field): `fault stage=<job id> day=<day> key=<key> names=<n> stored=<m> asked=<k>`. `ResolveDeps.Log` is a `func(string)` and `PullDeps.Log` is a `*slog.Logger`: the one builder returns the text, each job writes it with its own sink (`Info` on the logger), and the new `PlanDeps.Log` is a `func(string)` as the resolve. It is the line of the summary family (`option_open_resolve_gap day=... cap=...`), the same sink, one line per key and run, no new channel and no notification (the existing Telegram path of the runner stays as it is). The line names the stage and the count, so that the owner reads which stage was the first empty one.

### The pull states per name

10. The pull's run summary (`Summary.String`, one line per key) gains the state of each name, in the order of the plan, as separate tokens `state[<SYMBOL>]=stored|live|missing` (the existing `<SYMBOL>=done/total@time` tokens stay as they are). `stored` is a name with planned items whose contracts were stored before the session day's gap stage began (no `ListRunRecord` of reason `gap_live` for it); `live` means the contracts of the name were repaired live by the gap stage that day (it says nothing about the quotes, which the pull always fetches live); it is a name that has a `gap_live` record of the session (`PullDeps.Lists.ListRuns(day)`, nil means every name with items is `stored`); `missing` is a watched name of the batch with no planned item or whose rows are all `no_data` or `not_listed`. The counts of the three go into the one line (`names=3 stored=2 live=1 missing=0`) and a name that is `missing` while the watch list has it is a gap of the pull, through Decision 8.
11. The page side is unchanged: the option pages already show a gap line per name from the verdict path; a name `missing` is that gap. No pages entry, no new column, no contract change.

### What does not change

12. `Contracts: unchanged`: no schema, dataset or proto changes; no migration; `internal/jobs/api.go` and `internal/store/openquote/api.go` are not edited (the new ports are types of the `optionopen` package, as `PullDeps.Plans` is). `docs/design/modules.md` is not edited: the new files are inside the module row `jobs` and the wiring is inside the row `ot`.

## Interface

None. `Postcondition` is an optional method the runner already reads, `NewGapJP` has the type of `NewResolveJP` and the new dependency fields are types of the package `optionopen`.

## Issues

The entries are in one module each, in wave order. Entry 1 carries no interface change and is first because the other two use its fault line and its test helpers.

### 1 optionopen-postcondition

Module: jobs. Size: M.
Files: `internal/jobs/optionopen/postcondition.go` (new), `internal/jobs/optionopen/postcondition_test.go` (new), `internal/jobs/optionopen/listed.go`, `internal/jobs/optionopen/plan.go`, `internal/jobs/optionopen/plan_test.go`, `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/pull_test.go`, `internal/jobs/optionopen/fakes_test.go`
Design: docs/design/option-open-gap-jp.md#the-postcondition
Order: after drafts A and B (both land first). Before 2 and 3.
Goal: Decisions 8 and 9: `Postcondition` on `resolveJob` (so on resolve, gap, and both JP resolves), `planJob` (US and JP) and `pull` (so `jpPull`), each reading a stored reader; the fault line through the `Log`. The per-key counts map is on `resolveJob`, `planJob` and `pull` (a field and its filling in `Run`; `listed.go` gets the `asked` fill, under 20 added lines). New fields: `PlanDeps.Read PlanReader` and `PlanDeps.Log func(string)`, `PullDeps.Rows openquote.OptionOpenReader` (all optional; nil Rows or Read makes the postcondition true, so a job built without the port behaves as today).
Acceptance:
1. `grep -c "^func (.*) Postcondition(ctx context.Context, day, datasetID, subjectKey string) (bool, error)" internal/jobs/optionopen/postcondition.go` prints 3 (resolveJob, planJob, pull).
2. `grep -c "Postcondition(" internal/jobs/optionopen/listed.go internal/jobs/optionopen/jp.go` prints 0 for both (the method lives in the new file; `jpPull` gets it by embedding).
3. `grep -c "fault stage=" internal/jobs/optionopen/postcondition.go` prints 1 (one place builds the line) and `grep -c "names=%d stored=%d asked=%d" internal/jobs/optionopen/postcondition.go` prints 1.
4. `awk '/^type PlanDeps struct/,/^}/' internal/jobs/optionopen/plan.go | grep -c "Read PlanReader\|Log  *func(string)"` prints 2 and `awk '/^type PullDeps struct/,/^}/' internal/jobs/optionopen/pull.go | grep -c "Rows"` prints 1.
5. `grep -c "t.Parallel()" internal/jobs/optionopen/postcondition_test.go` equals `grep -c "^func Test" internal/jobs/optionopen/postcondition_test.go`, and `grep -c "time.Sleep\|time.Now" internal/jobs/optionopen/postcondition_test.go` prints 0.
6. `grep -rhE "^func (TestPostconditionResolveEmpty|TestPostconditionPlanEmpty|TestPostconditionPullNoRows|TestPostconditionNoDataRowsFalse|TestPostconditionNilReaderTrue|TestFaultLineNamesStageAndCount)\(" internal/jobs/optionopen | wc -l` prints 6.
7. `git diff --name-only origin/main` lists no path outside the Files line, and `wc -l internal/jobs/optionopen/postcondition.go` shows under 600.
8. The result matches docs/design/option-open-gap-jp.md#the-postcondition.
Checked by CI: ci-go; `go test ./internal/jobs/optionopen -race -count=1` (fake stores and fake clock: a stock with stored contracts is true; a name with no stored contract and no mark is false and logs one fault line with names=1 stored=0 asked=0; a pull batch with items and only `no_data` rows is false; the empty counts map after a restart tests stored only).

### 2 optionopen-gap-jp

Module: jobs. Size: M.
Files: `internal/jobs/optionopen/gapjp.go` (new), `internal/jobs/optionopen/gapjp_test.go` (new), `internal/jobs/optionopen/job_gap_jp.toml` (new), `internal/jobs/optionopen/manifest.go`, `internal/jobs/optionopen/listed.go`, `internal/jobs/optionopen/listed_test.go`, `internal/jobs/optionopen/gap_keys_test.go`, `internal/jobs/optionopen/jp.go`, `internal/jobs/optionopen/jp_test.go`, `internal/jobs/optionopen/fakes_test.go`
Design: docs/design/option-open-gap-jp.md#the-jp-gap-stage
Order: after 1.
Goal: Decisions 2 to 7: `NewGapJP`, the manifest `job_gap_jp.toml`, the manifest key `max_per_name` (read by the resolve path of every manifest; 0 means no per-name cap, so the US manifests are unchanged), the live resolve of the watched JP names with no stored contract of the band inside both caps, and the `gap_live` list-run record through `ResolveDeps.Lists`. Edits of `listed.go` are the two hooks (the per-name cap in `take` and the record at the end of a key) and stay under 40 added lines; the rules of the JP chain stay in `jp.go` and `gapjp.go`. `listed.go` is 602 lines on main: if the hooks do not fit without raising it past the ceiling of `docs/design/ratchet.json`, report it in the pull request description and do not split the file.
Acceptance:
1. `grep -c 'id = "option_open_resolve_gap_jp"\|gap_only = true\|zone = "Asia/Tokyo"\|calendar = "XTKS"\|offset = "-90m"\|budget_minutes = 40\|max_new = 300\|max_per_name = 60\|dataset = "master.option_contract.gap_jp"' internal/jobs/optionopen/job_gap_jp.toml` prints 10 (`calendar = "XTKS"` is in both tables; anchor, offset and budget as Decision 3).
2. `grep -c "func NewGapJP(d ResolveDeps, o ResolveOptions) (contracts.Job, error)" internal/jobs/optionopen/gapjp.go` prints 1, `grep -c "go:embed job_gap_jp.toml" internal/jobs/optionopen/gapjp.go` prints 1 and `grep -c "jpBroker{" internal/jobs/optionopen/gapjp.go` prints at least 1.
3. `grep -c "SMART" internal/jobs/optionopen/gapjp.go internal/jobs/optionopen/job_gap_jp.toml` prints 0 for both, `grep -c "\[chain\]" internal/jobs/optionopen/job_gap_jp.toml` prints 0 and `grep -c "parseJPRules(planManifestJP)" internal/jobs/optionopen/gapjp.go` prints 1 (the chain of OSE.JPN, multiplier 100 and class = code is written once, in `job_plan_jp.toml`).
4. `grep -c "MaxPerName" internal/jobs/optionopen/manifest.go` prints at least 2 (field and use) and `grep -c "max_per_name" internal/jobs/optionopen/job_gap.toml internal/jobs/optionopen/job_resolve.toml internal/jobs/optionopen/job_resolve_jp.toml` prints 0 for each (the US and the evening manifests keep no per-name cap).
5. `grep -c "gap_live" internal/jobs/optionopen/listed.go` prints 1 and `awk '/^type ResolveDeps struct/,/^}/' internal/jobs/optionopen/listed.go | grep -c "Lists"` prints 1.
6. `grep -rhE "^func (TestGapJPResolvesNameWithNoStoredContract|TestGapJPSkipsNameWithStoredBand|TestGapJPRequestsAreFullySpecified|TestGapJPPerNameCapDefers|TestGapJPRunCapDefers|TestGapJPTwoTimeoutsStop|TestGapJPWritesGapLiveRecord|TestGapJPNoOptionNameNotAsked)\(" internal/jobs/optionopen | wc -l` prints 8.
7. `git diff origin/main -- internal/jobs/optionopen/job_gap.toml internal/jobs/optionopen/job_resolve.toml internal/jobs/optionopen/job_resolve_jp.toml internal/jobs/optionopen/job_plan_jp.toml internal/jobs/optionopen/job_pull_jp.toml | wc -l` prints 0.
8. `git diff --name-only origin/main` lists no path outside the Files line, and `wc -l internal/jobs/optionopen/gapjp.go` shows under 600.
9. The result matches docs/design/option-open-gap-jp.md#the-jp-gap-stage.
Checked by CI: ci-go; `go test ./internal/jobs/optionopen -race -count=1` with a fake broker that records every request and a fake clock (no sleep): a watched JP name with no stored contract is asked, each request carries exchange OSE.JPN, currency JPY, multiplier 100, class = code, expiry, strike and right; a name with its band stored is not asked; 61 contracts of one name defer the sixty-first; the 301st of a run defers; two timeouts stop the run; a `gap_live` record is written for the repaired name only).

### 3 optionopen-pull-states

Module: jobs. Size: S.
Files: `internal/jobs/optionopen/pullstate.go` (new), `internal/jobs/optionopen/pullstate_test.go` (new), `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/pull_test.go`, `internal/jobs/optionopen/jp.go`, `internal/jobs/optionopen/jp_test.go`, `internal/jobs/optionopen/summary.go`, `internal/jobs/optionopen/core.go`, `internal/jobs/optionopen/core_test.go`, `internal/jobs/optionopen/fakes_test.go`
Design: docs/design/option-open-gap-jp.md#the-pull-states-per-name
Order: after 1 and 2 (it reads the `gap_live` record of 2 and the field `PullDeps.Rows` of 1).
Goal: Decision 10: the summary line of the pull names each name `stored`, `live` or `missing` and carries the three counts; `PullDeps.Lists` is the optional reader of `ListRuns`. `jpPull.runKey` (a copy of `pull.runKey` with the JP parameters) gets the same call, so the line is the same on both chains.
Acceptance:
1. `grep -c "stored|live|missing\|=stored\|=live\|=missing" internal/jobs/optionopen/pullstate.go` prints at least 1, and `grep -c "names=%d stored=%d live=%d missing=%d" internal/jobs/optionopen/summary.go internal/jobs/optionopen/pullstate.go` prints 1 for the first file and 0 for the second (`grep -c` prints a count per file).
2. `awk '/^type PullDeps struct/,/^}/' internal/jobs/optionopen/pull.go | grep -c "Lists"` prints 1, and `grep -c "ListRuns(" internal/jobs/optionopen/pullstate.go` prints 1.
3. `grep -c "nameStates\|pullStates" internal/jobs/optionopen/pull.go internal/jobs/optionopen/jp.go` prints at least 1 for each (both runKey paths call it).
4. `grep -rhE "^func (TestPullStateStored|TestPullStateLiveFromGapRecord|TestPullStateMissingNoItems|TestPullStateMissingAllNoData|TestPullStateNilListsAllStored|TestPullSummaryLineCarriesCounts)\(" internal/jobs/optionopen | wc -l` prints 6.
5. `git diff --name-only origin/main` lists no path outside the Files line, and `wc -l internal/jobs/optionopen/pullstate.go` shows under 600.
6. The result matches docs/design/option-open-gap-jp.md#the-pull-states-per-name.
Checked by CI: ci-go; `go test ./internal/jobs/optionopen -race -count=1`.

### 4 ot-gap-jp-wiring

Module: ot. Size: S.
Files: `cmd/ot/wire_optionopen.go`, `cmd/ot/wire_optionopen_test.go`, `docs/contracts/ot/jobs/option_open_resolve_gap_jp.md` (new), `docs/contracts/ot/jobs/option_open_resolve_gap.md`, `docs/contracts/ot/jobs/option_open_resolve.md`, `docs/contracts/ot/jobs/option_open_resolve_jp.md`, `docs/contracts/ot/jobs/option_open_plan.md`, `docs/contracts/ot/jobs/option_open_plan_jp.md`, `docs/contracts/ot/jobs/option_open.md`, `docs/contracts/ot/jobs/option_open_jp.md`
Design: docs/design/option-open-gap-jp.md#the-rule-stated-once-for-both-chains
Order: after 1, 2 and 3 (it passes their new fields).
Goal: register `option_open_resolve_gap_jp` at order 167 (between the JP resolve at 165 and the JP plan at 170) through `buildOpenResolve(..., optionopen.NewGapJP, false, broker.Options{ResolveInFlight: 1})`; pass `Lists` (the opened store as `openquote.OptionListStore`, by type assertion; a store that does not implement it leaves it nil and the family logs one startup line saying so, because a silent nil would drop the `live` state; the test of this entry has a store that implements it and one that does not) to the two gap jobs and to both pulls, `Rows` (the same store as `OptionOpenReader`) to both pulls, and `Read` and `Log` to both plans; the contract docs state the rule of Decision 1 once (in the gap docs; the other files link to them) and the postcondition and fault line of Decisions 8 and 9 for their job.
Acceptance:
1. `grep -c "option_open_resolve_gap_jp" cmd/ot/wire_optionopen.go` prints at least 2 and `grep -c "optionopen.NewGapJP" cmd/ot/wire_optionopen.go` prints 1.
2. `grep -c "order: 167" cmd/ot/wire_optionopen.go` prints 1 and `grep -c "ResolveInFlight: 1" cmd/ot/wire_optionopen.go` prints at least 4 (3 on main).
3. `grep -c "^Order: 167" docs/contracts/ot/jobs/option_open_resolve_gap_jp.md` prints 1, and `grep -c "each stage reads its stored input, and the gap stage repairs it live before the plan" docs/contracts/ot/jobs/*.md | grep -v ":0$"` prints exactly 2 lines (the US gap file and the JP gap file; no other file repeats the rule).
4. `grep -c "Postcondition" docs/contracts/ot/jobs/option_open_resolve.md docs/contracts/ot/jobs/option_open_resolve_gap.md docs/contracts/ot/jobs/option_open_resolve_jp.md docs/contracts/ot/jobs/option_open_resolve_gap_jp.md docs/contracts/ot/jobs/option_open_plan.md docs/contracts/ot/jobs/option_open_plan_jp.md docs/contracts/ot/jobs/option_open.md docs/contracts/ot/jobs/option_open_jp.md | grep -c ":0$"` prints 0 (all eight name it).
5. `grep -c "option_open_resolve_gap_jp" cmd/ot/wire_optionopen_test.go` prints at least 1.
6. `git diff --name-only origin/main` lists no path outside the Files line.
7. The result matches docs/design/option-open-gap-jp.md#the-rule-stated-once-for-both-chains.
Checked by CI: ci-go; `go test ./cmd/ot -race -count=1` (the family list holds the new id at its order; a missing socket or DSN leaves it out with the reason of its siblings).

## Checked by CI

ci-go runs `go test ./internal/jobs/optionopen ./cmd/ot -race -count=1` for entries 1 to 4. Nothing in these entries needs PostgreSQL, a live broker or a sleep. The first live check is the owner's: at 07:30 JST of a JP session day the line `option_open_resolve_gap_jp ... asked=` appears, and the line `fault stage=` appears for no stage when the chain is whole.

## Left out

- The JP resolve at the pull (a refresh of a name with no contract on the option pages): same reason as the pull design, the resolves cost seconds and cannot run inside the 15 seconds of the button.
- A second gap pass at 09:15 JST for empty books (the advisor's retry of empty books): the pull's own `retry_classes` already retry the planned classes; a gap pass after the plan would replan, which the pull design leaves out.
- A notification to Telegram for a fault line: the line goes to the same log as the summary, and the ops page shows the gap; a channel is the owner's to add.
- Calendar rules of the JP expiry (monthly European, last trading day the business day before the second Friday): the chain definition of `ListChains` carries the expiry, so the stage computes no date.
- A page column for the state of a name: Decision 11; the verdict path already shows a gap.
- Stopping or skipping the next stage on a false postcondition (the stages are ordered by the clock only), and a repair of a missing stored close (the plan keeps its `NoValue`) or of the underlying con_id (draft A).
- A US-only change of the gap pass beyond the shared hooks of entry 2: the US pass is not tuned here.

## Doubts

- "Fault line" has no sink on main under that name. This design uses the job `Log` (the same line family as the summary); if the owner means another channel, only the one place that builds the line (acceptance 3 of entry 1) changes.
- Decision 10 takes "fetched live" from `master.option_list_run`, a table that no job writes yet, not from a timestamp: `OptionRef` has no resolved-at field, and adding one is a store interface change that this design does not want. If the pull design's list run is redesigned, entry 3 follows.
- The postcondition of resolve counts a name with all contracts marked not listed as true (an explained absence). The advisor's rule says stored 0 for an active underlying is FAILED; a JP name that has options but whose marks cover the whole band is rare and is shown by the marks, not by a failed key. Owner call if it should fail.
- `listed.go` is 602 lines: over the 600 of a new file and under the 800 above which `ratchet.json` needs a ceiling (it lists no optionopen file), so entries 1 and 2 may add the small hooks; if the file passes 800 the entry reports instead of splitting.
- The store recounts `Listed` of a `ListRunRecord` from `master.option_list` (`openquote/api.go`), so the `Listed` of a `gap_live` row may be overwritten; the pull reads `Reason` only. `option_list_run` gets a second meaning (a gap record) beside the list run of the pull design.
- The window of the dataset `master.option_contract.gap_jp` is not registered anywhere on main; entry 4 or the runner's dataset table may need the XTKS window (unverified: not read in `internal/runner`).
- `store()` in the optionopen package creates the instrument of an option with `CalendarID: "XNYS"`; for JP options the calendar is wrong. Not changed here (unverified whether any reader uses it).
- A false postcondition becomes a gap on the ops page only at the close of the window, and does not stop the plan: the log line is the early signal. Owner call if the plan must stop on an empty gap stage.

## Adversarial

A fresh read-only sub-agent read this file and the repository at c96f513f and answered the six questions. Its full answer is the file `adversarial-gap-jp.md` in the dispatcher scratchpad.

1. Gap against the goal (a false postcondition stops nothing; the ops page shows it only at the close; only option contracts are repaired; "live" is not live quotes): adopted in part. The text now says that the gap is shown at the close and that the fault line is the early signal (Decision 8), that the next stage is not stopped (Left out and Doubts, owner call), that the close and the underlying con_id are not repaired (Left out), and that `live` means contracts repaired live, not quotes (Decision 10). Not adopted: stopping the plan, because it needs a runner change outside these modules.
2. Biggest unnamed risk (the postcondition has no run state; it is asked only when the run has no error, so it adds nothing for NoValue): adopted. Decision 8 now keeps a per-key counts map filled by `Run` and tests stored only after a restart, and states that the postcondition covers the clean-run-stored-nothing case only. The wrong premise "a `no_option` name is planned as the underlying only" is removed.
3. Simpler way (a JP manifest on the gap job, fail the plan loudly, take the pull state from the existing summary token): partly adopted. The JP gap is already the reuse of the gap job. Not adopted: failing the plan, since the plan already returns NoValue for the cases it can see and the owner asked for a visible postcondition on every stage; and the pull state from the existing token, since `done/total` cannot tell a repaired name from a stored one. The tokens are now separate (`state[SYMBOL]=...`) so they do not collide.
4. Lock-in (a second meaning of `option_list_run`, the token collision, the shared gap dataset, a silent nil `Lists`): adopted. Own dataset id and a manifest key `dataset`, separate tokens, a startup line when the store lacks `OptionListStore`, and a Doubt on the recount of `Listed`.
5. Claim most likely false ("the gap stage costs nothing on a good day"; the no_option plan row): adopted; both corrected (Decision 4: one `ListChains` per watched name per run).
6. Likeliest slip (entry 1: `listed.go` missing from Files, acceptance 4 printed 3, `Log` types differ, depends on drafts A and B; entry 2 acceptance 1 and 3; entry 3 acceptance 1; entry 4 acceptance 2 vacuous; the ratchet doubt wrong): adopted. `listed.go` is in the Files of entry 1, the Log types are reconciled in Decision 9, the acceptance commands are corrected (10 in entry 2, no `[chain]` table in the gap toml, per-file counts in entry 3, at least 4 in entry 4), and the ratchet doubt is rewritten. The dependency on drafts A and B stays: the design says they land first. Smaller points taken: `no_data` rows do not count as stored; the XNYS calendar of the option instrument and the unregistered dataset window are Doubts. The 10 minute `run_bound` against 300 resolves at one in flight (about 50 s at 0.16 s each) is not a risk and is left.
