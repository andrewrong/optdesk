# Option open, plan B: contracts computed, confirmed by listing, cached valid and invalid for seven days, listed on the spot when the cache lacks them; US and Japan

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 2 (revision 1 was a JP gap stage with a Postcondition; the owner's own shape and the advisor's review r1 replace it)

Source: owner ask optdesk-2940, 2026-11-12: "when the stored data is bad, collect on the spot; no one-wrong-step-then-all-wrong". Owner shape, same day: compute the candidate contracts; before the open confirm with IBKR which are valid and which are not; store both with a validity of 7 days; after 7 days check again, and a contract still invalid stays cached as invalid; collect only the valid ones; a contract the cache lacks or whose validity ended is checked on the spot, with no step after step; IBKR's real limit is the concurrent market-data lines, not a daily cap. Advisor review r1 (PR 1750, head 710386a4): NoValue only on a gateway answer; a missing input is never a silent no_value; the CHECK constraint of `option_list_run` forbids a new reason; the Telegram and ops views show a failed key only after the window close. Option-trader advisor (00:15Z): JP options are OSE.JPN, class = stock code, multiplier 100, from the chain definition, never SMART; open slot 09:05 JST; at least 3 strikes per side.
Read on origin/main at 746e8aed: `AGENTS.md`, `docs/design/modules.md`, `docs/design/option-open-pull.md`, `docs/design/option-contract-lists.md` (Decisions 4, 6, 8, 9, 10; entries 1 to 6), `migrations/0034_option_list.sql`, `internal/jobs/optionopen/{listed,plan,pull,jp}.go` and the tomls, `internal/store/openquote/api.go`, `internal/runner/reconcile/{reconcile,runkey}.go`, `internal/runner/sched/{gaps,ticker}.go`, `pyside/src/optdesk/ibkr/{pacing,ibgateway}.py`, `cmd/ot/{wire_optionopen,check_scheduler,notify}.go`. "[unverified]" marks a claim not checked in code or docs.

Goal: for a watched name of either market, on a session day, the plan and the pull read contracts that a gateway answer has confirmed (valid) or refused (invalid, remembered for 7 days); a name with no fresh answer is listed on the spot before the plan; a run that cannot get an answer is a retrying key that the owner sees the same morning, never a green no_value.

## What main holds today, against the owner's model

| Owner step | On main |
|---|---|
| compute the candidates | done: `candidates` of the sigma band over the chain (`listed.go`), `ChooseExpiries` and the strike rules of `plan.go` and `band.go`, US and JP (`jp.go`) |
| confirm with IBKR before the open | US: evening `option_open_resolve` (per contract) and the 08:00 ET `option_open_resolve_gap`. JP: the evening `option_open_resolve_jp` only. Listing by expiry (`ListContracts`, `master.option_list`) is designed (`option-contract-lists.md`), its tables and store are on main (`0034_option_list.sql`, `optionopenpg/lists.go`), its rpc, broker method and job are not (entries 1 to 6 of that design, not built; US only, JP "out of this design") |
| store valid and invalid | valid: `master.option_contract` (no end date: a listed contract stays valid until its expiry). Invalid: `master.option_not_listed` with `retry_at` 7 plus or minus 2 days (`retry_days = 7`, `jitter_days = 2` in the resolve tomls); written only for "no security definition" (`classify`, `listed.go`) |
| check again after 7 days, invalid stays cached | done for the per-contract path: the mark is due at `retry_at`, asked again, marked again |
| collect valid ones only | done: the plan reads stored contracts only (Decision 1 of the pull design), the pull sends the planned items |
| check what is missing on the spot | US: the 08:00 gap pass only. JP: none. The plan and the pull never ask, and a stage that lacks its input ends `NoValue` (`listed.go:328` "no stored close", `plan.go:202` and `:209` "no stored close", "no resolved chain"), which `verdict()` turns into a final no_value mark (`runkey.go:176`) that `sched/gaps.go` never shows |

So about four of the six steps exist for the US. The delta of this design: the same pre-open confirmation for JP, freshness of seven days for a listing, the on-the-spot listing of a name with no fresh answer, the rule that only a gateway answer ends a key `NoValue`, and a same-morning view of a key that is still retrying.

## Decisions

### The model, stated once

1. Each stage reads its stored input; the listing stage is the only stage that asks the broker for what the stored input lacks, and it runs before the plan, at the pre-open pass and again as the on-the-spot pass (Decision 5). Valid and invalid are both stored; collection uses the valid ones only. This sentence is written once, in `docs/contracts/ot/jobs/option_open_resolve_gap.md`, and the other contract files point to it.

### Confirm by listing, not per contract (advisor)

2. One `ListContracts` per (underlying, expiry, class) returns every listed strike (`option-contract-lists.md` Decision 1, one `reqContractDetails` with no strike). Validity is membership in that answer: a strike in it is valid (stored in `master.option_contract`), a computed candidate of a listed expiry that is not in it is invalid for the day. One answer fills the positive and the negative cache of the whole expiry. A lookup of a contract that does not exist, one by one, is the expensive path: it waits the details lane timeout of 20 s and then a pause of 30 s (`pacing.py`: `DETAILS_TIMEOUT_S`, `DETAILS_PAUSE_S`). So the per-contract resolve stays only where it is today (the US evening pass and gap) and is not extended to JP.
3. What the negative cache stores: the contract key, the day D of the answer, and the reason `not_in_list` (the mark's existing checked_at and error text, `contracts.OptionNotListed`; no new column). Only "the gateway answered and it was not listed on day D" is cached. A timeout, no answer, a disconnect, a `ErrNotServed` and a list that is `short` (`option-contract-lists.md` Decision 6, "real list") are never cached as invalid: they leave the group unlisted and the key retrying. This is the rule the per-contract path already has (`classify`: only `outNotListed` is ever marked); the listing job gets the same test.
4. Freshness, seven days. A group is fresh when `master.option_list` has a row with outcome `listed` whose `listed_session` is within `list_ttl_days` = 7 days before D (for the 06:15 pass: the group is listed again when it is stale, and not before). The valid cache has no seven-day end: a listed contract cannot stop being valid before its expiry, so re-asking it costs lookups and finds nothing. The invalid cache ends with the freshness of its group, so after seven days the group is listed again and a strike still not listed stays invalid; a strike the list now holds becomes valid. `list_ttl_days` is a manifest value of the list job; `1` is the daily behaviour of `option-contract-lists.md` and stays its default, this design's tomls set 7.
5. The consequence of seven days, and the owner's open choice. A strike or expiry that IBKR adds after a big move of the stock stays invisible for up to seven days if its group is fresh. Two ways, to be chosen by the owner and not decided here:
   - (a) a plain seven days; nothing else is read. Nothing to build beyond Decision 4.
   - (b) the group is re-listed early when the spot leaves the band it was listed for. This needs the spot at the listing time stored with the group, which `master.option_list` does not have (a migration and an interface change of the store, an owner-only path), and the stages read the stored close, so the exit test runs on the stored close of the day. Entry 2 builds (a) and carries the manifest key `relist_on_band_exit = false`; (b) is a later entry once the owner chooses it and the column exists.
6. The on-the-spot pass: the list job code runs a second time with a manifest of its own at the pass before the plan (US 08:00 ET, JP 07:30 JST), `gap_only = true`: it lists the groups that are not fresh (a name added late, a group the pre-open pass did not finish, a stale group), nothing else, in the order of need of `option-contract-lists.md` Decision 4 (held names, then priority names with a buy price, then the rest; inside a class the nearest to 30 DTE for the US and to the middle of 14 to 90 for JP, then the earlier expiry; never by symbol alone). A name with every group fresh costs no broker call, as the `ListChains` of today's gap does not (it asks once per name; the list job reads `master.option_list` first and asks `ListChains` only for a name that has a stale group).
7. JP. The same job code, a JP rule set (`jpRules`, `jp.go`: exchange OSE.JPN, multiplier 100, class = stock code, `N225` as the index of class 225, the names without options excluded), a JP expiry window of DTE 14 to 90 (`job_plan_jp.toml`), and two slots on XTKS: `option_list_preopen_jp` at 06:30 JST and `option_list_gap_jp` at 07:30 JST with `stop_time` 08:10 JST so that it ends before the plan at 08:15. Whether the pyside `ListContracts` serves OSE names is unverified (Doubt 1): if it does not, the JP listing ends every name `unserved` and the JP side stays on the evening per-contract resolve of today, and the key is retrying and visible (Decision 11), not silent.

### NoValue only on a gateway answer; a missing input is never a silent no_value

8. The rule, for resolve, gap, list, plan and pull, US and JP: a key ends `NoValue` only when a gateway answer proves the market has nothing (an empty chain for the name, an expiry list that is empty after a real answer, "no security definition"). A missing stored input is not such an answer. The cases on main: "no stored close" (`listed.go:328`, `plan.go:202`): `Retryable` with the text "no stored close of X", re-run until the window close, because the bar of the day may arrive later; "no resolved chain" (`plan.go:209`): `Retryable` when the name has no stored contract and no fresh list (the on-the-spot pass fills it before the plan), and `NoValue` only when the stored list of the name is fresh and empty; the pull's "no plan" (`errNoPlan`, `pull.go`): `Retryable`, since the plan job is the stage that failed; "no con_id" is already `Retryable` in `listed.go:307` and the list job takes the same line. `no_option` names and names off the watch list stay `NoValue`: the owner's own words are the answer.
9. A stage whose inputs were fine and stored nothing: the read-back at the end of `Run` (the stock's rows in `master.option_contract`, the plan rows of the day, the pull rows `ok` or `partial`) turns a clean result with stored 0 into `Retryable` "stored 0 of the names asked", with one line `fault stage=<id> day=<day> key=<key> stored=<m> asked=<k>` through the job's `Log`. No `Postcondition` method is added: a false `Postcondition` is a counted run that is never re-run (`runkey.go:182-188`, `reconcile.go:349-357`), the opposite of what is wanted. A `Retryable` run before the window close is not counted (`reconcile.go:349`) and is run again at every turn: every 15 minutes (`sched/ticker.go:32`) and at every slot instant, until the close (for the JP datasets the XTKS close, 15:30 JST; the pull stops at `stop_after = "0930"` with a `Permanent` error). A re-run of a stage that has nothing to ask costs no broker call (Decision 6 reads the lists first), so the loop is cheap and ends when the key is present or the window closes.
10. Without fix A (the JP con_id of an underlying) the design still holds: a JP name with con_id 0 is a `Retryable` key of the list stages and of the gap, shown by Decision 11 the same morning, and every other JP name proceeds. With fix A the same names list.

### Same-morning visibility

11. What shows a failed key today: `sched/gaps.go` `keyKind` gives a missing key a gap only after the window close (`!closed || running` gives none, except the crash brake), and a `no_value` key none at all unless the runner wrote `expired`; the readers of open gaps on main are `ot check scheduler` (`cmd/ot/check_scheduler.go`) and the Telegram notifier (`cmd/ot/notify.go`, `NotifyGaps`); no web page calls `internal/pages/gaps` `Lines` (only tests do) [unverified beyond `grep` of non-test callers]. So the ops page the owner means has no code on main to extend here; the existing gap path reaches Telegram and `ot check scheduler`. Entry 3 changes `keyKind`: before the close, a missing key whose last two runs ended `Retryable` (about 30 minutes of retrying; a single transient is retried silently) is an open gap with the reason `retrying`, ranked below `expired`; at the close it becomes `expired`; a present key clears it. It is the one change of the runner module. A page that lists the gaps of the day is not in this design (Left out, Doubt 2).

### Load: IBKR's limits, and which limit bounds which stage

12. There is no daily cap and no per-name count in this design. The limits, from the code that enforces them and from IBKR's documented numbers:
   - Message rate: the TWS API allows 50 messages per second by default [unverified: IBKR documentation, from memory]; `MessagePacer` of the pyside transport holds 40 starts per second for every message (a cancel counts), `DEFAULT_STARTS_PER_SECOND` in `pacing.py`. It binds everything on the one connection, and is far above what the lookups need.
   - Contract-detail style requests (`ListChains`, `ListContracts`, `ResolveContract`, dividends, bars) go through `DetailsLane`: in flight 2 at the start, rising to at most 5 when ten answers in a row average under 1 s, falling to 1 on slow answers; a request that waits 20 s times out, drops the limit to 1 and pauses starts for 30 s; more than 2 unanswered requests block new starts. The Go client of these jobs dials with `ResolveInFlight: 1` and a call timeout of 25 s (`wire_optionopen.go`, `broker.Options`). The probe quoted in `option-contract-lists.md` found list calls finishing in 5.0 s steps with no overlap [unverified: probe files are not in the repository], so one list request is taken as 5 s. This lane bounds the pre-open pass, the gap pass and the on-the-spot pass.
   - Concurrent market-data lines: 100 by default per IBKR account [unverified: IBKR documentation]; `LineBudget` holds 70 lines and 50 snapshots in flight (`DEFAULT_LINES`, `DEFAULT_SNAPSHOTS`) and shrinks on error 101. This bounds the pull and the refresh button, not the lookups; the lookups subscribe to no market data.
   - Worst case, JP, from these: 20 watched JP names [assumption] with 4 expiries in the window is 80 list requests at 5 s: 400 s, under 7 minutes, in the first pre-open pass; a typical gap pass lists only the stale groups. A sick gateway: each request can cost 20 s plus a pause of 30 s, 50 s; the run stops at two timeouts in a row (the rule of `option-contract-lists.md` Decision 4 and of `resolveAll`, `listed.go`), so at most about 100 s are lost before the stage ends retrying; and the stage's own clock bounds the rest: `stop_time` 08:10 JST for the gap, `stop_time` 07:00 JST for the pre-open pass. The US figures are those of `option-contract-lists.md` Decision 9 and are not changed.
   - The turn against the US evening resolve. One worker runs one key at a time, in the order of DatasetID and then subject (`reconcile.go:183-190`), and a busy worker retries a due slot after 5 s (`ticker.go`, `busyRetry`). US `option_open_resolve` runs from 17:00 ET with a budget of 40 minutes (`job_resolve.toml`), so to about 17:41 ET with the last call. 07:30 JST is 17:30 ET in winter and 18:30 ET in summer. In winter the JP gap waits for the worker until about 17:41 ET (07:41 JST); its `stop_time` of 08:10 JST then leaves 29 minutes for work that needs under 7 in the first pass and much less after it. Because the stop is a clock time and not a duration from the start, a late start cannot push the gap past the plan at 08:15. In summer there is no overlap. A retried gap key runs before the plan key of the same turn because `master.option_list` sorts before `plan.option_open_item`; the pull keys of a turn are those of the plan rows read at the start of the turn, so the pull never runs in the turn that stores the plan.

### What does not change

13. `Contracts: unchanged`: no schema, dataset or proto change. Not edited: `internal/jobs/api.go`, `internal/store/openquote/api.go`, `migrations/**`. `option_list_run` is not written by this design (its CHECK of reasons does not hold a new one, and its `PutListRuns` overwrites on conflict, `lists.go:33-40`; the list job of `option-contract-lists.md` owns it). `docs/design/modules.md` is not edited: the new files sit in the rows jobs and ot.

## Interface

None. No Go, Python or contract interface file changes: the new job constructors have the type of `NewResolveJP` and the manifest keys are values of the package `optionopen`. The entries that depend on `option-contract-lists.md` use its interfaces (entries 1 to 4 of that design) as they land.

## Issues

### 1 optionopen-no-silent-no-value

Module: jobs. Size: S.
Files: `internal/jobs/optionopen/listed.go`, `internal/jobs/optionopen/listed_test.go`, `internal/jobs/optionopen/plan.go`, `internal/jobs/optionopen/plan_test.go`, `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/pull_test.go`, `internal/jobs/optionopen/jp.go`, `internal/jobs/optionopen/jp_test.go`, `internal/jobs/optionopen/fakes_test.go`
Design: docs/design/option-open-gap-jp.md#novalue-only-on-a-gateway-answer-a-missing-input-is-never-a-silent-no_value
Order: independent of the other entries; first.
Goal: Decisions 8 and 9 for the existing resolve, gap, plan and pull, US and JP (`jpPull.Run` is a copy of `pull.Run`): "no stored close", "no resolved chain" of a name with no fresh list, and "no plan" are `Retryable` with a stored reason; the read-back turns stored 0 after a clean run into `Retryable` with the fault line; no `Postcondition` method.
Acceptance:
1. `grep -c 'NoValue, "no stored close' internal/jobs/optionopen/listed.go internal/jobs/optionopen/plan.go` prints 0 for both, and `grep -c 'Retryable, "no stored close' internal/jobs/optionopen/listed.go internal/jobs/optionopen/plan.go` prints 1 for both.
2. `grep -c 'NoValue, "no resolved chain' internal/jobs/optionopen/plan.go` prints 0 and `grep -c "errNoPlan" internal/jobs/optionopen/pull.go internal/jobs/optionopen/jp.go` shows no `NoValue` kind on the line of its use (`grep -n "NoValue.*errNoPlan" internal/jobs/optionopen/pull.go` prints nothing).
3. `grep -rc "Postcondition(" internal/jobs/optionopen | grep -v ":0$"` prints nothing.
4. `grep -c "fault stage=" internal/jobs/optionopen/listed.go internal/jobs/optionopen/plan.go internal/jobs/optionopen/pull.go` prints a total of 3 lines and `grep -c "names=%d" internal/jobs/optionopen/listed.go` prints 0.
5. `grep -rhE "^func (TestNoStoredCloseIsRetryable|TestNoChainWithoutFreshListIsRetryable|TestNoPlanIsRetryable|TestStoredZeroAfterCleanRunIsRetryable|TestEmptyFreshListIsNoValue|TestRetryableRerunsBeforeClose)\(" internal/jobs/optionopen | wc -l` prints 6.
6. `git diff --name-only origin/main` lists no path outside the Files line.
7. The result matches docs/design/option-open-gap-jp.md#novalue-only-on-a-gateway-answer-a-missing-input-is-never-a-silent-no_value.
Checked by CI: ci-go; `go test ./internal/jobs/optionopen -race -count=1` with fake stores and a fake clock (a key with no bar is Retryable and is run again on the next fake turn; an answered empty chain is NoValue).

### 2 optionopen-list-ttl-jp

Module: jobs. Size: M.
Files: `internal/jobs/optionopen/list.go`, `internal/jobs/optionopen/list_test.go`, `internal/jobs/optionopen/list_jp.go` (new), `internal/jobs/optionopen/list_jp_test.go` (new), `internal/jobs/optionopen/job_list_preopen_jp.toml` (new), `internal/jobs/optionopen/job_list_gap_jp.toml` (new), `internal/jobs/optionopen/job_list_gap.toml` (new), `internal/jobs/optionopen/manifest.go`, `internal/jobs/optionopen/fakes_test.go`
Design: docs/design/option-open-gap-jp.md#confirm-by-listing-not-per-contract-advisor
Order: after the entries 3, 4 and 5 of docs/design/option-contract-lists.md (the store interface, its implementation and the list job, whose `list.go` this entry edits); `list_ttl_days` and the gap manifest need that job.
Goal: Decisions 3, 4, 6 and 7: the manifest key `list_ttl_days` (default 1, so the US pre-open pass is unchanged) and `gap_only` for the list job; the freshness test from `master.option_list` (`outcome` `listed` and `listed_session` within the days); the negative cache written from an answered list only; `NewListGapUS` (`job_list_gap.toml`, 08:00 ET) and the JP pair `NewListPreopenJP`, `NewListGapJP` over `jpRules` with the JP expiry window, `stop_time` 08:10 JST for the gap and 07:00 JST for the pre-open pass; a name with every group fresh is not asked. Choice (b) of Decision 5 is not built; `relist_on_band_exit` is read and must be false.
Acceptance:
1. `grep -c "list_ttl_days" internal/jobs/optionopen/manifest.go internal/jobs/optionopen/list.go` prints at least 1 for each, and `grep -c "list_ttl_days = 7" internal/jobs/optionopen/job_list_gap.toml internal/jobs/optionopen/job_list_preopen_jp.toml internal/jobs/optionopen/job_list_gap_jp.toml` prints 1 for each of the three.
2. `grep -c 'zone = "Asia/Tokyo"\|calendar = "XTKS"' internal/jobs/optionopen/job_list_gap_jp.toml` prints at least 2, `grep -c 'stop_time = "0810"' internal/jobs/optionopen/job_list_gap_jp.toml` prints 1, `grep -c 'stop_time = "0700"' internal/jobs/optionopen/job_list_preopen_jp.toml` prints 1 and `grep -c "gap_only = true" internal/jobs/optionopen/job_list_gap.toml internal/jobs/optionopen/job_list_gap_jp.toml` prints 1 for each.
3. `grep -c "SMART" internal/jobs/optionopen/list_jp.go internal/jobs/optionopen/job_list_preopen_jp.toml internal/jobs/optionopen/job_list_gap_jp.toml` prints 0 for each and `grep -c "parseJPRules(planManifestJP)" internal/jobs/optionopen/list_jp.go` prints 1 (the chain of OSE.JPN is written once).
4. `grep -c "relist_on_band_exit" internal/jobs/optionopen/manifest.go` prints at least 1 and `grep -c "relist_on_band_exit = false" internal/jobs/optionopen/job_list_gap.toml` prints 1.
5. `grep -c "not_in_list" internal/jobs/optionopen/list.go` prints at least 1, and `grep -n "AddNotListed" internal/jobs/optionopen/list.go` shows no call outside the branch that has an answered list (the test of acceptance 6 is the check).
6. `grep -rhE "^func (TestFreshGroupIsNotAsked|TestStaleGroupAfterSevenDaysIsListed|TestTimeoutIsNeverCachedInvalid|TestNotServedIsNeverCachedInvalid|TestShortListIsNeverCachedInvalid|TestAnsweredAbsentIsCachedInvalid|TestGapPassListsOnlyStaleGroups|TestGapJPStopTimeEndsBeforePlan|TestOrderIsByNeedNotSymbol|TestJPRequestsAreFullySpecified)\(" internal/jobs/optionopen | wc -l` prints 10.
7. `git diff origin/main -- internal/jobs/optionopen/job_gap.toml internal/jobs/optionopen/job_resolve.toml internal/jobs/optionopen/job_resolve_jp.toml internal/jobs/optionopen/job_plan_jp.toml internal/jobs/optionopen/job_pull_jp.toml | wc -l` prints 0.
8. `git diff --name-only origin/main` lists no path outside the Files line.
9. The result matches docs/design/option-open-gap-jp.md#confirm-by-listing-not-per-contract-advisor.
Checked by CI: ci-go; `go test ./internal/jobs/optionopen -race -count=1` (a fake broker recording every request: fully specified OSE.JPN requests of class = code and multiplier 100, no broker call for a name with fresh groups, the stop time ends a run with a fake clock, no sleep).

### 3 sched-retrying-gap

Module: runner. Size: S.
Files: `internal/runner/sched/gaps.go`, `internal/runner/sched/gaps_test.go`
Design: docs/design/option-open-gap-jp.md#same-morning-visibility
Order: independent of 1 and 2.
Goal: Decision 11: `keyKind` returns `retrying` for a key that is missing before its window close and whose last two runs (not counting `empty_before_close` and crashed runs) ended `done` with `ErrorCode` `retryable`; `gapRank` gets the kind below `expired`; the close, the clear and the rule that a gap already open with the same kind is not written again are unchanged. The kind reaches `ot check scheduler` and the Telegram notifier through the existing gap path.
Acceptance:
1. `grep -c "gapRetrying" internal/runner/sched/gaps.go` prints at least 3 and `grep -c '"retrying"' internal/runner/sched/gaps.go` prints 1.
2. `grep -rhE "^func (TestKeyKindRetryingTwoRetryableBeforeClose|TestKeyKindOneRetryableIsNoGap|TestKeyKindRetryingBecomesExpiredAtClose|TestKeyKindRetryingClearsWhenPresent|TestSyncWritesRetryingGap|TestRetryingRankBelowExpired)\(" internal/runner/sched | wc -l` prints 6.
3. `grep -c "t.Parallel()" internal/runner/sched/gaps_test.go` is at least `grep -c "^func Test" internal/runner/sched/gaps_test.go`, and `grep -c "time.Sleep" internal/runner/sched/gaps_test.go` prints 0.
4. `git diff origin/main -- internal/runner/api.go docs/contracts/runner.md internal/pages internal/notify | wc -l` prints 0.
5. `git diff --name-only origin/main` lists no path outside the Files line.
6. The result matches docs/design/option-open-gap-jp.md#same-morning-visibility.
Checked by CI: ci-go; `go test ./internal/runner/sched -race -count=1` with a fake clock and fake ledgers.

### 4 ot-list-jp-wiring

Module: ot. Size: S.
Files: `cmd/ot/wire_optionopen.go`, `cmd/ot/wire_optionopen_test.go`, `docs/contracts/ot/jobs/option_open_resolve_gap.md`, `docs/contracts/ot/jobs/option_list_preopen_jp.md` (new), `docs/contracts/ot/jobs/option_list_gap.md` (new), `docs/contracts/ot/jobs/option_list_gap_jp.md` (new)
Design: docs/design/option-open-gap-jp.md#the-model-stated-once
Order: after 1, 2 and 3 (it registers what 2 builds); after entry 6 of docs/design/option-contract-lists.md (the wiring file of the list job).
Goal: register `option_list_gap` (US, 08:00 ET), `option_list_preopen_jp` (06:30 JST) and `option_list_gap_jp` (07:30 JST) through the family of the list job, one resolve in flight, the list store of the opened option open store; a store that lacks the list port leaves these families out with one startup line, not silently; the contract docs: the rule of Decision 1 once in `option_open_resolve_gap.md`, the others point to it, and each new file states its slot, its `stop_time`, the freshness days and the fault line.
Acceptance:
1. `grep -c "option_list_gap_jp\|option_list_preopen_jp\|option_list_gap\b" cmd/ot/wire_optionopen.go` prints at least 3 and `grep -c "ResolveInFlight: 1" cmd/ot/wire_optionopen.go` prints at least 6 (3 on main).
2. `grep -c "^Order: " docs/contracts/ot/jobs/option_list_preopen_jp.md docs/contracts/ot/jobs/option_list_gap.md docs/contracts/ot/jobs/option_list_gap_jp.md` prints 1 for each.
3. `grep -l "each stage reads its stored input" docs/contracts/ot/jobs/*.md` prints exactly `docs/contracts/ot/jobs/option_open_resolve_gap.md`.
4. `grep -c "option_list_gap_jp" cmd/ot/wire_optionopen_test.go` prints at least 1.
5. `git diff --name-only origin/main` lists no path outside the Files line.
6. The result matches docs/design/option-open-gap-jp.md#the-model-stated-once.
Checked by CI: ci-go; `go test ./cmd/ot -race -count=1`.

## Checked by CI

ci-go runs `go test ./internal/jobs/optionopen ./internal/runner/sched ./cmd/ot -race -count=1` for entries 1 to 4. Nothing here needs PostgreSQL, a live broker or a sleep. The first live check is the dispatcher's: one read-only probe of `ListContracts` on an OSE name on port 4001 once the rpc entry and the pyside restart are in (Doubt 1).

## Left out

- Choice (b) of Decision 5 (re-list when the spot leaves the band): needs a column on `master.option_list`; built only after the owner chooses it.
- A web page that lists today's gaps and failed runs: no page reads `internal/pages/gaps` on main; the same-morning view is `ot check scheduler` and the Telegram notifier (Decision 11). A page is a pages entry the owner decides.
- A per-contract resolve for JP, a daily or per-name cap, and any write of `master.option_list_run` by the gap: the owner's real limit is the lane (Decision 12), and the table's reasons are the list job's.
- A live spot from the broker when the stored bar is missing: the stage is retrying and visible; a live snapshot of the underlying is a later change if the owner wants it (`Broker.Snapshot` has only `KindOptionEod`).
- Stopping the next stage when a key is retrying: the stages are ordered by the clock; the plan reads what is stored and its own `Retryable` shows a name that has nothing.
- The pull's per-name state (stored, fetched live, missing) of revision 1: dropped; the on-the-spot listing happens before the plan, and the pull collects what the plan holds.

## Doubts

1. Whether the pyside `ListContracts` serves the OSE names (class = stock code, OSE.JPN, multiplier 100) is unverified. The dispatcher probes port 4001 once entry 1 of `option-contract-lists.md` (#1742) and the pyside restart are in. If it does not, entry 2's JP jobs end every name `unserved` (retrying, visible) and JP stays on the evening per-contract resolve.
2. There is no web ops page that lists failed runs or open gaps on main (Decision 11). The owner's "same morning" is met on Telegram and `ot check scheduler`; whether a page is wanted is the owner's.
3. The seven days for the invalid cache are a group freshness here, not a per-contract date, because one listing answers a whole expiry. The owner's words ("store both with 7 days") are met for the invalid side; the valid side ends at the contract's expiry, not at seven days (Decision 4). Owner call if valid contracts must also be re-listed every seven days: the list job already re-lists a stale group, so the valid ones are refreshed then.
4. IBKR's numbers (50 messages per second, 100 market-data lines) are from memory of the TWS documentation and not checked here; the numbers that bind the code are pyside's (40 per second, 70 lines, 50 snapshots, the details lane).
5. `list.go` and `ErrNotServed` are not on main yet; entry 2's Files name `list.go` because that design's entry 5 creates it. If that entry changes the file name, entry 2's Files follow.
6. `store()` in `listed.go` gives every option instrument `CalendarID: "XNYS"`, also the JP ones. Not changed here; no reader of it was found [unverified: `internal/store/instrument` and `internal/pages` only].
7. The 7 plus or minus 2 days of the per-contract marks (`jitter_days`) differ from the exact 7 of the listing freshness; the owner's text says 7.

## Adversarial

(filled after the review)
