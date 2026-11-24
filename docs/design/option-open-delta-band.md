# option_open: expiries of 15 to 45 days, puts by Black-Scholes delta, no cap on puts, a run that is cut by rank, and the night line on the page

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-24
Revision: 1

Source: owner ask optdesk-5945 (2026-11-24), his words final; the owner's earlier decisions of 2026-11-10 (expiries 20 to 45, band k 1.4, ceilings) are amended where this file says so. Amends `option-open-pull.md` Decisions 3, 8, 10, 11 and 22 and `## Capacity`, and `option-open-band.md` Decision 4. Read on origin/main at eb9542ab: `internal/jobs/optionopen/{planfunc.go,band.go,gapband.go,core.go,pull.go,pull_resume.go,outside.go,list.go,list_run.go,manifest.go,job_plan.toml,job_resolve.toml,job_gap.toml,job_list_preopen.toml,job_list_zone.toml,job_pull.toml}`, `internal/pages/ops_today_page/builder.go`, `internal/store/runview/api.go`, `docs/design/{option-open-pull,option-open-band,modules}.md`. Read-only database checks of 2026-11-24 (`plan.option_open_item`, `obs.option_open`, `master.option_contract`, `master.option_list_run`, `master.option_not_listed`, `settings.underlying`) and the scheduler log of the day. Status: draft.

摘要（中文）：业主 2026-11-24 的决定：(1) 到期日取 15<=DTE<=45 的全部已上市到期日；(2) 看跌取 Black-Scholes delta 在 -0.40 到 -0.10 之间的全部行权价（用名称级前收盘 ATM IV 和计划价算，不用库里的 eod delta），外加 around_spot；(3) 看涨只给持仓名称，最先被砍；(4) 看跌和 around_spot 不再被每股 60 条或类别上限砍掉；(5) 跑不完时按秩次问：around_spot、再从价内向外轮流问看跌、最后看涨，run_bound 到点按这个秩次砍；(6) 页面每晚显示"问了 N / 计划 M，K 条因时间被砍"及原因。按 11 月 24 日的计划算，共 4,397 条，按 82 条/分需要约 54 分钟，25 分钟装不下，建议 run_bound 65 分钟。01-08 缺失的原因不是窗口也不是提前一天：当天绝大多数名称的合约链里还没有这个到期日。

## Frame

Today (facts, read on 2026-11-24):

- Windows. The plan, the evening resolve and the gap pass take 20 to 45 days (`min_dte = 20`, `max_dte = 45`; the resolve adds `lookahead_days = 4`), and the two list manifests take 21 to 45 (`[[window_dte]]`), one day apart. The session day of 2026-11-24 has no expiry at 17 days (12-11) in any of them.
- The missing expiry of 2027-01-08 (45 days on 11-24). The list run of the day was keyed 2026-11-24 (log line `option_list_preopen day=2026-11-24`, run early as a backfill at 14:02 +08, 5 hours before its 19:15 slot), so its window was 11-24 + 21 to 45 days, and `ChooseExpiries` keeps both ends (`dte >= dteMin && dte <= dteMax`). A day early or a floor of 21 does not explain it. What does: the chain the broker answered on 11-24 holds no 01-08 for every US name but one. The session's `expected` sets in `master.option_list_run` hold three expiries (12-18, 12-24, 12-31) for 41 to 45 names and none at 01-08; `master.option_contract` has no 01-08 contract; `master.option_not_listed` holds five 01-08 marks, all of one name, all under a trading class with a leading digit (a second class of that name), asked by the 21:00 gap pass and answered "no security definition". So the 01-08 weekly is not yet in the chain of the other names: the resolve asks every expiry of every chain within 49 days and found none. The weeklies reach about 6 weeks ahead (12-24 was first resolved at 43 days on 11-11) and 12-31, a quarter-end series, was seen at 49 days on 11-12. The chain itself is not stored, so this is read from the three stores and the log; issue 1 makes the next case checkable with one log field.
- Puts. The plan chooses puts by a band (`put_k` 1.4, about |delta| 0.08), then cuts: at least 6 and at most 25 per expiry on a one-dollar grid, the five-dollar rule for three index funds, 60 per stock (`capPerStock`, `planfunc.go:495-516`), and ceilings per class (`around_spot` 350, `puts` 1,400, `calls` 1,200). The plan of 2026-11-24 holds 3,118 items; 929 of them are written `deferred` by those caps before the run starts (`around_spot` 169, `puts` 529, `calls` 231), so the pull asks at most 2,189. The stored eod delta is no basis for a rule: it exists only for contracts collected before, and on 11-23 3,987 of 5,563 puts in the window had none (owner).
- The run. On 11-23 the pull asked 2,070 contracts (1,957 ok, 112 partial, 1 no data) in about 25 minutes: about 106 a minute for the first 15 minutes and 82 a minute on average (owner). `job_pull.toml` has `run_bound = "25m"` from open + 5 minutes and `stop_after = "1000"`.
- The cut by time. `pull.batches` cuts the plan into keys of at most 120 items in the order of the underlying ids and keeps a stock whole, so a stop by time drops the keys at the end of that order: whole stocks, whatever their rank. The plan's own rank (held, `around_spot` round-robin, puts nearest the spot outward round-robin, calls) is not what the pull follows.
- The page. `ops_today_page` shows the newest run of each key with a reason code; it has no line of how much of the night's plan was asked.

Goal: at each US open the owner has, for every name on the list, every expiry of 15 to 45 days, every listed put with a delta of -0.40 to -0.10 and the strikes around the spot, quoted, within the time the run has; when the run cannot fit, what is missing is the farthest puts and the calls, never an `around_spot` item, never a whole stock; and the owner reads on the page how much was asked and how much was cut by time.

Owner's standing rules this design respects:

- "Same job one at a time" (2026-11-19): no job, lane or second run is added; the pull is still one run of the key, in rank order.
- "Dev mode: no time guards" (2026-11-12): the only time rule is the run's own bound, the owner's `run_bound`, and a cut is shown, not hidden.
- "Standard sources, no made-up data" (2026-11-24): the delta of this design chooses strikes and is stored nowhere and printed nowhere; the pages keep showing the broker's delta (`option-open-pull.md` Decision 18, plan v2.5 ADR 0001).
- "Fallback, not cascade" (2026-11-12): the band is computed from stored inputs (the stored chain, the stored close, the stored sigma); nothing waits for a broker resolve.
- "Settings off is not a gap" (2026-11-24): names with options switched off are left out by the registered issue `option-open-skip-options-off`, not here (`## Left out`).
- "Check design against owner rules" (2026-11-19): `## Adversarial` and `## Decisions` quote them.

## Modules

jobs (`internal/jobs/optionopen`), pages (`internal/pages/ops_today_page`), ot (`cmd/ot/wire_optionopen.go`, one port). No interface file, no contract under `contracts/`, no migration and no `modules.md` change: the new files are in the package of the jobs row, `PullDeps` is a struct of `pull.go`, and the reason rows are `runview.Reason` rows of the existing `runview.ReasonWriter`.

## Interface

None changed. Three names are agreed between the producer (jobs) and the reader (pages, which may not import jobs), and pinned by one test on each side:

- the reason row of the night: dataset `obs.option_open`, subject `night`, scope empty, code `asked_of_planned`, and a detail of at most 200 characters in the form `asked <N> of <M> planned, <K> cut by time: <reason>`, with the reason text of Decision 7;
- `PullDeps` gains `Reasons runview.ReasonWriter` (nil: no line is written; `NewPull` does not require it);
- `PlanParams` gains `PutRule` (`band` or `delta`), `DeltaLo`, `DeltaHi` and `Rate`, with the manifest keys `put_rule`, `delta_lo`, `delta_hi`, `rate` in `job_plan.toml` table `[params]`.

## Decisions

### Decision 1 (owner ask optdesk-5945, 2026-11-24): expiries of 15 to 45 days

Owner: "every listed expiry with 15<=DTE<=45 (was 20-45, owner 2026-11-10). The listing and resolve windows follow."

1. `min_dte = 15` and `max_dte = 45` in `job_plan.toml`, `job_resolve.toml` and `job_gap.toml` (the resolve keeps `lookahead_days = 4` beyond 45, so an expiry that enters 45 on the plan day is resolved the evening before), and `[[window_dte]]` 15 to 45 in `job_list_preopen.toml` and `job_list_zone.toml` (the 21 of the lists, one day off the plan, goes). `DefaultPlanParams` takes `MinDTE` 15 (the pull core plans the live band with it). The Japan parameters (`job_plan_jp.toml`, `JPPlanParams`, 14 to 90) are not changed.
2. Every expiry in the window is kept; `(|DTE - 30|, DTE)` stays only the order that decides which expiry is cut first by time (Decision 5).
3. The cause of the missing 01-08 is in `## Frame`: not the day and not the floor, the chain. Nothing in the design makes the plan wait for an expiry the broker has not listed; the next day's list, resolve and gap pass take it as soon as a chain holds it (DTE 45 or less on the plan day, DTE 49 or less for the resolve). The list run summary line gains `chain_expiry_max=<date>` (the latest expiry any chain of the run showed) and `window_names=<n>` (names whose chain holds an expiry in the window), so a day on which an expiry at DTE 45 is missing is read from the line, not guessed.

### Decision 2 (owner ask optdesk-5945, 2026-11-24): puts by Black-Scholes delta

Owner: "the strikes whose put delta is between -0.40 and -0.10, plus around_spot (the put and call at the listed strike at or below spot and the one above). Delta is Black-Scholes from the name-level prior-close ATM IV (the plan's sigma, ResolveSigma) and the planned spot, for every listed strike. Never the stored eod delta."

1. New pure file `internal/jobs/optionopen/deltaband.go` (no I/O, no clock): `putDelta(spot, strike, sigma, days, rate)`, the Black-Scholes delta of a put, `N(d1) - 1` with `d1 = (ln(S/K) + (r + sigma^2/2) T) / (sigma sqrt(T))` and `T = days / 365`. `rate` is a plan value, 0 (the plan has no rate and no dividend input; with 0.04 the 2026-11-24 count moves by 20 of 4,373, under 0.5 percent). `sigma` is the plan's: `ResolveSigma` of the prior close's at-the-money implied vol, else the last known value, else 30 percent, and the row keeps `sigma_source` as today.
2. `PlanParams.PutRule = "delta"` for the US (`DefaultPlanParams`, `job_plan.toml`): a listed put of an expiry is a `puts` item when its delta is from -`delta_hi` (0.40) to -`delta_lo` (0.10), both ends in; "every listed strike" is every stored resolved contract of the expiry (`master.option_contract`, which holds only listed contracts), at the planned spot. A contract that is already an `around_spot` item is not also a put. `PutRule = "band"` is the rule of 2026-11-10 (k 1.4 and its guards) and stays for Japan (`JPPlanParams` sets it), so no Japan test or value moves.
3. Under `delta` the guards that cut puts do not apply: `min_puts` (6 per expiry), `max_per_expiry` (25 on a one-dollar grid) and the five-dollar rule are rules of the `band` rule; a name with no strike in the delta band has no put item for that expiry, which is not an error (its `around_spot` items remain).
4. The stored band follows: under `delta` the evening resolve and the gap pass store puts down to `store_k` 1.75 (about |delta| 0.04) and the 25 percent floor `store_drop` is not applied to puts, because for a name with sigma over about 0.87 a delta of -0.10 lies more than 25 percent below the spot and the floor hides those strikes (three names of the 2026-11-24 plan show it, up to 175 strikes). The extra resolves are new strikes only: the cache and the not-listed marks hold the rest, and the gap pass keeps `cap = 4000`.
5. At the open the live band is the same rule over the stored chain at the live spot with the plan's sigma (`option-open-band.md` Decisions 2 and 4); the pull core uses `DefaultPlanParams`, so one set of values serves both, and a test pins that `job_plan.toml` and `DefaultPlanParams` agree.
6. This amends `option-open-pull.md` Decision 3: "the plan computes no option value, no IV and no Greek of its own" no longer holds for the choice of strikes. The value is a filter, not a stored datum, and the pages still print the broker's delta; the sentence stays true of every stored and shown number.

### Decision 3 (owner ask optdesk-5945, 2026-11-24): calls

Owner: "only for held names (current rule), and they are the first thing cut."

`calls = "held"` and `call_k = 1.4` stay as they are. Calls rank after every put (Decision 5), so a cut by time takes them first.

### Decision 4 (owner ask optdesk-5945, 2026-11-24): nothing cuts puts or `around_spot` but time

Owner: "no per-stock cap of 60 (capPerStock) and no class ceiling may cut puts or around_spot."

1. For the US, `ceil_around_spot` and `ceil_puts` are 0 and `ceil_per_stock` is 0, and 0 means no ceiling for the three of them (`capPerStock` already returns on 0; `deferTail` of a class gets the same rule). Today a ceiling of 0 would defer everything, so issue 2 writes the rule and tests it.
2. `ceil_calls` stays 1,200: calls are the first thing cut and the plan keeps a bound on them; the farthest held calls above it are written `deferred` as today. `capPerStock` applies to nothing else. The ceilings of Japan do not change.
3. Held contracts stay outside every ceiling as today.

### Decision 5 (owner ask optdesk-5945, 2026-11-24): the run is cut by rank

Owner: "If the run cannot fit, items are asked by rank: around_spot first, then puts nearest the spot outward, across all names round-robin, then calls. The run_bound cut must follow that rank. Today pull.go batches cut in underlying-id order and drop whole stocks. Fix that."

1. The rank is the plan's own (`BuildPlan`: held, then `around_spot`, then `puts`, then `calls`; inside a class the names take turns, each name's next strike nearest the spot first with the expiries of a name interleaved by distance, the first name rotating by the day number). It does not change; Decision 4 only removes the deferral that put items at the end of it.
2. `pull.batches` cuts the plan into keys by rank: items sorted by `Rank`, a key is the next `max_per_key` items (240; was 120 by stock), keys run one after another in rank order. A stock appears in several keys; `Core.Pull` handles it per key as today (underlying snapshot, live band, expiry groups, chunks of 30). The pull keeps one start spot per stock for the whole run (a run-level cache filled at the first key the stock appears in), so the band of a stock is chosen once at the open spot and not 19 times; the end-of-group spot reads of `Core.group` (skew) stay per group. These reads are not in the 82 a minute and are an unmeasured cost: the first night measures it (night line `<r> a minute`) and `run_bound` is the owner's value to change.
3. `pull.run` today only logs a key started after `stop_after`; it becomes a real stop. The pull ends its own asking at its time bound: `stop_after` (clock, HHMM, a key not started by then is not started) and, inside a key, `Core` stops sending chunks when the run's deadline (`run_bound` less `cut_margin` of 1 minute, a `job_pull.toml` value) has passed. Every item with no final row at that moment gets a row with status `deferred` and `error_code` `TIME_CUT` (not asked; `outside_band` false); these are the highest ranks, never a part of the front of the plan. One sweep at the end of `pull.run`, under a fresh 10-second context so that it also runs after the runner cancels the run, writes the `TIME_CUT` rows of every item of every key not started or not finished. If the process is killed (a deploy; `restart_sensitive` holds the deploy back, the window does not deploy meanwhile) no sweep runs: the rows of those items are absent, the night line of the re-run counts them as cut by time (K = M - N - O - D), and a re-run the same morning resumes in rank order and asks them.
4. The postcondition of `option-open-pull.md` Decision 22a counts the asked items: `planned` is the not-deferred items minus the rows `TIME_CUT` and `OutsideBand`, so a night cut at 40 percent is not a failed night; the share (0.5) still bites on what was asked. A run with every item cut or outside the band is `NoValue` as in `option-open-band.md` Decision 1.
5. `outside.go` gains the helper that counts `TIME_CUT` rows next to `outsideBand(`; the Japan pull (`jp.go`) keeps its own batches and is not changed.

### Decision 6 (owner ask optdesk-5945, 2026-11-24): capacity and `run_bound`

Owner: "compute the total for the 2026-11-24 plan under rules 1-3 ... State the minutes it needs. If it exceeds 25, propose the run_bound that fits and say what is cut at 25."

`## Capacity` holds the computation. It needs about 54 minutes at the measured 82 a minute, so 25 minutes does not fit. In `job_pull.toml` `run_bound = "65m"`, `stop_after = "1035"` (60 minutes of asking from 09:35), `max_per_key = 240`, `cut_margin = "1m"`. The owner changes the number by editing the file; the page shows what each night cut (Decision 7), so the next number is read, not guessed. `option-open-pull.md` Decisions 10, 11 and 22a ("whole job window 25 minutes", "ends before 10:10 ET", "no new key after 10:00 ET") read as these values.

### Decision 7 (owner ask optdesk-5945, 2026-11-24): the night line

Owner: "the page that shows option_open (ops_today_page) shows per night 'asked N of M planned, K cut by time' with the reason."

1. At the end of every US pull run (ok, failed or cut) the pull writes one reason row, `ReplaceReasons(day, "obs.option_open", "night", ...)`, code `asked_of_planned`. `M` is every item of the day's plan (the plan's deferred calls included), `N` the rows that were sent to the broker (any status but `deferred`), `K` the rows `TIME_CUT`. The detail is `asked <N> of <M> planned, <K> cut by time: <reason>`; the reason is, in this order of precedence: `run ended at <HH:MM> ET at its time bound, ranks above <R> not asked, <r> a minute` when `K` > 0; `all asked` when `K` = 0 and nothing else was left; and, when a part was left for another cause, the counts after it: `<O> outside the open band, <D> deferred by the calls ceiling, <U> not answered, broker unavailable`. The text is cut to 200 characters.
2. `ops_today_page.Build` copies the detail of that reason, as it is, into the first entry of `notes` of the page (schema `notes` exists, no schema change); a day with no such reason has no such note. The page renders what a producer wrote and computes nothing.
3. `cmd/ot/wire_optionopen.go` gives the pull its `runview.ReasonWriter` (the one the plan job already receives).

## Capacity

Computed for the plan of 2026-11-24 (session day, slot 0945), read-only. Inputs: the plan's planned spot and sigma of each of the 46 names, the stored put and call strikes of 12-11, 12-18, 12-24 and 12-31 (`master.option_contract`), 01-08 taken from the 12-31 strike grid of the same name (01-08 is not stored; the grid of a quarter-end series may hold more strikes than a weekly, so this is an upper estimate), delta at `rate` 0 and days 17, 24, 30, 37 and 45, calls of the 25 names that carry calls today, up to the 1.4 band, held contracts 24 as the plan holds them.

| expiry | days | around_spot + puts + calls |
|---|---|---|
| 12-11 | 17 | 167 + 307 + 215 = 689 |
| 12-18 | 24 | 184 + 590 + 306 = 1,080 |
| 12-24 | 30 | 168 + 391 + 287 = 846 |
| 12-31 | 37 | 168 + 373 + 308 = 849 |
| 01-08 | 45 (estimated) | 168 + 406 + 335 = 909 |
| total | | 855 + 2,067 + 1,451 = 4,373 |

- With the 24 held contracts the plan holds 4,397 items (today 3,118, of which the pull could ask 2,189).
- Three names whose stored puts stop above a delta of -0.10 (a high-IV name among them) add up to 175 strikes once the 25 percent floor is gone (Decision 2.4): up to 4,572.
- The five names the registered issue `option-open-skip-options-off` removes carry 204 of the 4,373 (51 + 47 + 106): 4,169 + up to 24 held = about 4,193 after it.
- The 01-08 row is an estimate for an expiry that is not in the chain today. Without it the plan is 3,464 items (4,373 - 909) plus 24 held, 3,488: 42.5 minutes at 82 a minute and 32.9 at 106; with it 53.6. Either way 25 minutes does not fit, and 65 minutes holds the larger figure with the spot-read cost unmeasured.
- Minutes, at 82 a minute (the 11-23 average) and at 106 (its first 15 minutes): 4,397 needs 53.6 and 41.5; 4,572 needs 55.8 and 43.1; 4,193 needs 51.1 and 39.6. The spot reads of Decision 5.2 are not counted (unmeasured). The run does not fit 25 minutes at either rate.
- Proposed `run_bound` 65 minutes (asking until 10:35 ET, 60 minutes at 82 a minute is 4,920, over the 4,572): the run ends about 10:40 ET at the latest, an in-flight key finishing after `stop_after`.
- What 25 minutes would cut (kept for reading the page if the owner chooses to keep it): 25 minutes at 82 a minute asks about 2,050: the 24 held, all 855 `around_spot`, and about 1,171 of the 2,067 puts, the nearest ones of each name, down to about |delta| 0.21 (0.19 to 0.29 by name); cut: about 896 puts of |delta| 0.10 to 0.21 and all 1,451 calls, 2,347 items.

The ten largest names of the plan (names are not printed here; the full table with names is under `state/optdesk-drafts/designs/option-open-cuts-capacity-2026-11-24.md`), by items over the five expiries, sigma of the plan, `around_spot` + puts + calls:

| name | sigma | items |
|---|---|---|
| A | 0.35 | 20 + 124 + 140 = 284 |
| B | 0.97 | 20 + 81 + 124 = 225 |
| C | 0.72 | 20 + 56 + 133 = 209 |
| D | 0.58 | 20 + 65 + 86 = 171 |
| E | 0.52 | 20 + 60 + 85 = 165 |
| F | 0.80 | 20 + 56 + 81 = 157 |
| G | 0.38 | 20 + 54 + 79 = 153 |
| H | 0.85 | 20 + 128 + 0 = 148 |
| I | 0.70 | 20 + 49 + 73 = 142 |
| J | 0.49 | 20 + 38 + 71 = 129 |

The ten hold 1,783 items, 41 percent of the plan; the largest alone is 284, against the 60 per stock of today. The 11-23 rate is the only measure of the broker's speed with this request shape; the first night of the new plan measures its own (the night line has `<r> a minute`).

## Failure paths

- A run killed or cut by the scheduler before it writes the night line: the line of the earlier run of the day stays (`ReplaceReasons` replaces only at the end of a run), and the next run of the key writes it. The runner's own cut shows as the failed run it is today.
- The reason writer fails: logged, the run's result is not changed (the line is a display, not a postcondition).
- Gateway down at the start: the rows are `disconnected` as today, they count as sent (`N`), and the night line names them: `asked <N> of <M> planned, 0 cut by time: <U> not answered, broker unavailable`; the key is retried by the runner.
- A stock whose spot cannot be read in one key: its items of that key are `disconnected`/`error` as today and are asked again by a re-run; the other stocks of the key are asked.

## Issues

### 1 option-band-window

Module: jobs. Size: S. Order: first. Files: `internal/jobs/optionopen/job_plan.toml`, `internal/jobs/optionopen/job_resolve.toml`, `internal/jobs/optionopen/job_gap.toml`, `internal/jobs/optionopen/job_list_preopen.toml`, `internal/jobs/optionopen/job_list_zone.toml`, `internal/jobs/optionopen/planfunc.go`, `internal/jobs/optionopen/list.go`, `internal/jobs/optionopen/window_test.go` (new), `internal/jobs/optionopen/planfunc_test.go`, `internal/jobs/optionopen/list_test.go`.

Acceptance:

1. Every US manifest of the package names the same new window: `min_dte = 15` and `max_dte = 45` in the plan, the evening resolve and the gap pass manifests, and one `[[window_dte]]` of 15 to 45 in each of the two US list manifests; `job_plan_jp.toml`, `job_resolve_jp.toml` and `job_list_preopen_jp.toml` keep 14 to 90.
2. A new test `TestUSWindowIs15To45` in `window_test.go` plans on a day with one chain whose expiries lie 14, 15, 45 and 46 days out and expects the plan items to hold the expiries at 15 and 45 days and none at 14 or 46.
3. A new test in `window_test.go` expects the list job, the resolve candidates and the gap pass to keep the same set of expiries for the same chain (the window of the lists no longer differs by a day from the plan).
4. A new test in `list_test.go` expects the summary line of a list run to carry `chain_expiry_max=<date>` and `window_names=<n>` for a fake chain whose latest expiry is known, and for a run whose chains answered no expiry to carry `chain_expiry_max=none`.
5. The tests of `planfunc_test.go` that pin the 20-day floor (`TestExpiryWindowUSAndJP`) are changed to the floor of 15 for the US and keep 14 to 90 for Japan; no other expectation of that file is weakened.

### 2 option-band-delta-puts

Module: jobs. Size: M. Order: after 1. Files: `internal/jobs/optionopen/planfunc.go`, `internal/jobs/optionopen/deltaband.go` (new), `internal/jobs/optionopen/deltaband_test.go` (new), `internal/jobs/optionopen/manifest.go`, `internal/jobs/optionopen/band.go`, `internal/jobs/optionopen/gapband.go`, `internal/jobs/optionopen/job_plan.toml`, `internal/jobs/optionopen/planfunc_test.go`, `internal/jobs/optionopen/gapband_test.go`, `internal/jobs/optionopen/manifest_test.go`.

Acceptance:

1. For the US plan a listed put whose Black-Scholes delta at the planned spot and the plan's sigma lies from -0.40 to -0.10 (both ends in) is a `puts` item, whatever its stored eod delta is or whether it has one; a put outside that range and not at an `around_spot` strike is no item. A new test `TestPutsByDeltaNotByStoredDelta` in `deltaband_test.go` pins both with contracts that carry no stored delta.
2. A new test in `deltaband_test.go` pins `putDelta` against four known values (at the money, a put at delta near -0.25, near -0.10 and near -0.40) to three decimals, with `rate` 0.
3. No listed strike of the delta range is dropped by a count: a new test with one name of a half-dollar grid and 90 strikes in the range expects 90 put items for that expiry (no 25 per expiry, no 6 minimum, no five-dollar rule, no 60 per stock), and a new test expects `around_spot` items to be kept whatever the class ceilings are.
4. A ceiling of 0 means none for `ceil_around_spot`, `ceil_puts` and `ceil_per_stock`; the US set has all three at 0 and `ceil_calls` 1,200, so a new test with 1,400 calls expects the 200 farthest calls `deferred` and no put or `around_spot` item `deferred`; `JPPlanParams` keeps `PutRule` `band` and its ceilings, and its tests are unchanged.
5. The evening resolve and the gap pass store puts down to `store_k` 1.75 and not to the 25 percent floor under `delta`: a new test with sigma 1.0 and 30 days expects a put at 70 percent of the spot to be a candidate to store, and one with sigma 0.30 to be unchanged from before.
6. The live band at the open (`gapband.go`, `Core.band`) uses the same rule at the live spot and the plan's sigma, so a stock that gapped keeps every stored put of the delta range of the new spot; a new test in `gapband_test.go` replaces the cap test (`TestGapAsksOfOneStockNeverExceedTheCap`) with a stock of 90 stored puts in the range expecting 90 asks.
7. `job_plan.toml` and `DefaultPlanParams()` hold the same values (a new test compares every field of `[params]`), `put_rule = "delta"`, `delta_lo = 0.10`, `delta_hi = 0.40`, `rate = 0.0`.
8. `grep -c "time.Now\|net/http\|database/sql" internal/jobs/optionopen/deltaband.go` prints 0, and the pull request description names no new dependency (own implementation, because no library of the repository gives a Black-Scholes delta and it is 10 lines; it sits behind the existing `PlanFunc` type).

### 3 option-band-rank-cut

Module: jobs. Size: M. Order: after 2. Files: `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/pull_resume.go`, `internal/jobs/optionopen/core.go`, `internal/jobs/optionopen/outside.go`, `internal/jobs/optionopen/cut.go` (new), `internal/jobs/optionopen/cut_test.go` (new), `internal/jobs/optionopen/summary.go`, `internal/jobs/optionopen/job_pull.toml`, `internal/jobs/optionopen/pull_test.go`, `internal/jobs/optionopen/pull_resume_test.go`, `internal/jobs/optionopen/fakes_test.go`.

Acceptance:

1. The keys of the pull are rank slices: for a stored plan of 1,000 items of 40 stocks the keys hold items 1 to 240, 241 to 480 and so on in the order of `Rank`, a stock may be in several keys, and `Plan()` returns the same keys and the same hash for the same plan on every call. A new test `TestKeysAreRankSlices` in `cut_test.go` pins it.
2. A run stopped by its time bound after key k leaves the items of the keys after k, and the unsent items of key k, written `deferred` with error code `TIME_CUT` and `outside_band` false, and every item of a lower rank than the first cut item has a row from the broker. A new test `TestTimeCutTakesTheTailOfTheRank` in `cut_test.go` with a fake clock of the test kit (no sleep) expects that no `around_spot` item is cut while a put is asked and no put is cut while a call is asked.
3. The postcondition counts the asked items: a new test runs a plan of 400 items where 300 are asked (200 ok) and 100 are `TIME_CUT` and expects no error, and a new test with 300 asked of which 100 ok expects the under-share error naming `time_cut=100`.
4. A re-run of the key the same morning asks the `TIME_CUT` items again and treats final rows of the earlier run as final: a new test in `pull_resume_test.go` stores 120 final rows and 80 `TIME_CUT` rows and expects 80 asks.
5. At the end of a run the pull calls `ReplaceReasons` once with dataset `obs.option_open`, subject `night`, code `asked_of_planned` and a detail of the form `asked <N> of <M> planned, <K> cut by time: <reason>` of at most 200 characters; a new test pins the exact text of a cut night (`asked 2050 of 4397 planned, 2347 cut by time: run ended at 10:35 ET at its time bound, ranks above 2074 not asked, 82 a minute`), of a night with nothing cut (`asked 4397 of 4397 planned, 0 cut by time: all asked`) and that a nil `Reasons` writes nothing and does not fail the run.
6. `job_pull.toml` holds the new values `run_bound = "65m"`, `stop_after = "1035"`, `max_per_key = 240` and `cut_margin = "1m"`, and `NewPull` rejects a `cut_margin` that is not a duration or is not shorter than `run_bound`.
7. The Japan pull (`jp.go`) and its manifest are unchanged and its tests hold.

### 4 option-band-wire-night

Module: ot. Size: S. Order: after 3. Files: `cmd/ot/wire_optionopen.go`, `cmd/ot/wire_optionopen_test.go`.

Acceptance:

1. The pull job of the US built by `cmd/ot` receives the same `runview.ReasonWriter` the plan job receives, so after a run the reason row `obs.option_open` / `night` is stored for the session day; a new test `TestOpenWiringGivesThePullAReasonWriter` shows the writer is not nil in the US pull deps and that the Japan pull deps are as before.
2. No other file under `cmd/` changes, and the Japan pull is wired as before.

### 5 option-band-night-page

Module: pages. Size: S. Order: after 3 (the strings are agreed in `## Interface`, so it may run beside 4). Files: `internal/pages/ops_today_page/builder.go`, `internal/pages/ops_today_page/night.go` (new), `internal/pages/ops_today_page/night_test.go` (new), `internal/pages/ops_today_page/builder_test.go`.

Acceptance:

1. For a day whose reasons hold one row with dataset `obs.option_open`, subject `night` and code `asked_of_planned`, the first entry of `notes` of `page.ops_today_page.v1` is the detail of that row as stored (`asked 2050 of 4397 planned, 2347 cut by time: run ended at 10:35 ET at its time bound, ranks above 2074 not asked, 82 a minute`), and the row is not listed among the `today.rows` (it has no run of its own). A new test `TestNightLineIsTheFirstNote` in `night_test.go` pins both.
2. A day with no such reason has the notes it has today; a reason with another dataset, subject or code is not a night line; a detail over 200 characters is cut to 200 as the other notes are. New tests in `night_test.go` pin the three.
3. The page computes nothing: no number of the line is parsed or added by the builder, and the schema file `contracts/schemas/page.ops_today_page.v1.json` and every other file under `contracts/` are unchanged by the pull request.

## Checked by CI

- `go test ./internal/jobs/optionopen/... ./internal/pages/ops_today_page/... ./cmd/ot/...`, `go vet ./...`, `golangci-lint` and `scripts/arch` pass.
- The tests are parallel with a temporary directory per file, a fake clock, no sleep, no live broker and no PostgreSQL (build tag `pg` is not used).
- None of these is an acceptance item above.

## Left out

- Names with options switched off on /underlyings (held shares and held options included): the registered issue `option-open-skip-options-off` removes them from the plan, the list and the resolve; this design does not repeat it. The capacity above is computed with and without them (204 items).
- A separate bug, not this design: the plan of 2026-11-23 used a stale `prior_close` spot (one stock, underlying id 445: planned 142.39, the 2026-11-20 eod 157.27). It shifts the delta band of that name by the spot error, and 1,026 of the 1,814 rows deferred on 11-23 were outside the band at the open. It is its own issue; this design takes the plan's spot as the owner words it ("the planned spot").
- Japan: the pull, the plan and the lists keep the `band` rule, 14 to 90 days, their ceilings and their keys; the night line is for the US pull only.
- A broker's delta as the filter at the open, or an eod delta as a fallback: not chosen (owner: never the stored eod delta).
- The rate and the dividend of the Black-Scholes delta: `rate` 0 and no dividend; a value of `job_plan.toml` if the owner wants a rate later.
- The downstream of a later end: `option_candidates` and the pages read the open snapshot after the pull ends (`candidates-after-partial-open.md`); a run that ends about 10:40 ET moves nothing there, and the earliest ranks (around the spot, the near puts) are stored first. Whether the candidates should wait for the end is that design's question.
- Making the broker list the 01-08 weekly earlier: not ours; the plan takes it when a chain holds it.
- A per-name "the biggest names first" rule: the rank is round-robin by the owner's words.

## Adversarial

A fresh read-only agent, which had read no conversation, attacked the design. Full answer: `state/optdesk-drafts/designs/option-open-cuts-adversarial-2026-11-24.md`.

- Gap against the goal: the writer of `TIME_CUT` rows for keys never started was missing, and `stop_after` only warned. Adopted: Decision 5.3 names the end sweep and makes `stop_after` a stop. The 25-minute cut figure rests on the plan order (held, then per-name round-robin inside a class), as Decision 5.1 says; the delta figure is an average of per-name values, stated as such.
- Biggest unnamed risk: a 65-minute run killed by a deploy loses its night line. Adopted: Decision 5.3 (restart sensitive, the re-run counts the absent rows as cut).
- Simpler way: keys by stock sorted by best rank. Not adopted: it drops whole stocks again, which the owner named as the bug.
- Lock-in: rank-slice keys change with any plan change and the delta rule sits in the plan manifest and in `DefaultPlanParams`. Accepted; a test pins the two equal (issue 2).
- Claim most likely false: the cost of the spot reads ("one minute"). Adopted: the claim is removed, a run-level start-spot cache is added and the cost is called unmeasured. Also adopted: the capacity is stated without the unlisted 01-08 expiry (3,488, 42.5 minutes). The agent's 01-07 rows are Japan names, not adopted.
- Likeliest slip: issue 3 is large and 4 and 5 wait on its strings. Not split now (one module, one rank concept); 4 and 5 are ordered after 3. The meaning of 0 for a ceiling is written in Decision 4.1 and `JPPlanParams` keeps its ceilings explicit (issue 2 item 4).
