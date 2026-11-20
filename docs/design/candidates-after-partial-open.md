# candidates after a partial open: a dependent runs again when its upstream has ended for the day, not only when every key is stored

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-20
Revision: 1

Source: owner, twice (recurrence of `candidates-at-open.md`, #3024 read the open first with an eod fallback, #3060 `After` option_open). Status: draft. Read on origin/main at d6c75f8c: `internal/runner/reconcile/runkey.go:494-545` (`upstreamState`, `gate`), `:602-` (`runDependent`, `dependentRun`), `rerun.go` (`rerunMarked`), `reconcile.go:411-500` (`nextAttempt`, `closeRow`), `internal/jobs/optioncandidates/source.go`, `job.toml`, `internal/pages/option_candidates_page/strings.go`, `docs/contracts/runner.md:43`.

摘要（中文）：今天 20 日 13:30（+08，美股开盘前）有人手动补跑了 option_candidates，那时没有开盘链，33 个名字全部用了前一日收盘链。开盘后 option_open 的 002、011 两批一直失败重试，而调度器的闸门要求上游"每个 key 都已入库"才放行，所以带 upstream_missing 标记的那次结果被留了一整天。本设计把闸门从"上游全部成功"改成"上游今天已经结束"（每个 key 要么入库，要么失败且不会再重试），然后用已有的数据重算，失败的名字按 job 自己的规则回退到收盘链；盘前跑的结果因为带标记，上游结束后一定被替换；候选页顶部加一行"开盘链 N 个 / 收盘回退 M 个"。

## Frame

Today (facts):

- Day 2026-11-20, XNYS. A console backfill at 13:30+08 (before the 09:30 ET open) ran `option_candidates`: `open=0 eod_fallback=33`. `consoleMark` gave the run reason `upstream_missing` (`runkey.go:572`) because option_open had no key stored yet.
- After the open option_open batches 002 and 011 kept failing and retrying. `upstreamState` returns `upOK` only when every expected key is in `present` (`runkey.go:542`); a failed end is `upMissing` (`:495-500`). `rerunMarked` (`rerun.go:32-37`) needs `upOK` for every upstream, so the marked run was never replaced. The 13:30 result stayed on the page.
- The job already falls back per name: `[source] eod_fallback = true` (`job.toml`), mark `eod_fallback` (`source.go:55`), and a per-row hint (`strings.go:39`). The counts exist only in a log line (`source.go:118`). The page has no count of names by source [verified: no field in `page.option_candidates_page.v1.json` or `builder.go`].
- A run with `upstream_missing` is a normal run: one more run is allowed (`docs/contracts/runner.md:43`).

Goal, the owner's flow: five minutes after the open option_open pulls all chains; when that pull has ended, option_candidates computes from what the pull stored; only a name with no usable open chain takes the previous option_eod; a pre-open result never survives the open.

Owner's standing rules this design must respect (quoted from memory notes):

- "Same job one at a time": `check-design-against-owner-rules`, 2026-11-19, serial scheduler plus lanes broke it.
- "Dev mode: no time guards": `dev-mode-no-time-guards`, 2026-11-12, any job runs when triggered, only the same-job concurrency guard; except that a dependent must not compute before its upstream ran.
- "Fallback, not cascade": 2026-11-12, chained jobs collect live when stored input is bad.
- "Think as product": 2026-11-13, what the owner sees on the candidates page first.

## Modules

runner (the gate: `internal/runner/reconcile/`), jobs (none edited; `optioncandidates` is read to confirm its fallback), pages (`internal/pages/option_candidates_page/`), contracts and web (one optional field). Four modules, four entries.

## Interface

One shared item changes: `contracts/schemas/page.option_candidates_page.v1.json` gains an optional object `source_counts {open, eod_fallback, none}` (integers). Optional on v1, as `market_state_page.v2` did; the web tolerates its absence. No Go interface file changes: `runner.Dependent`, `After()` and the `upState` type are inside `reconcile`. `Contracts:` line of the PR is `changed` for entry 1 only.

## Decisions

1. **The gate asks "ended", not "ok".** A new state `upEnded` sits between `upOK` and `upMissing`: every expected key of the upstream for the day is stored, or has at least one run and `nextAttempt` (`reconcile.go:420`) would not start another (the key has its counted verdicts, a permanent error, or its window closed, `expireIfClosed`), or the upstream's run bound has passed since its slot (`runBound(d)`, option_open: slot 09:35 ET + `run_bound` 25m) so a retryable key is given up at the bound and not retried until the window close, which can be hours away. A key with no run at all, or a running run with a live lease, is not ended; a retryable run before the bound is not ended. `upOK` (all stored) and `upEnded` both let the dependent run; only `upEnded` marks the run (Decision 2).
2. **The mark says what was missing.** A run that starts on `upEnded` carries reason `upstream_partial`; a run that starts on `upMissing` (console, pre-open) carries `upstream_missing` as today. `rerunMarked` replaces a newest `upstream_missing` run once the upstreams are `upOK` or `upEnded`; it never replaces `upstream_partial` (no loop: ended is final for the day, a console run with force is the way to ask again). This is the one extra run, inside the existing rule of `runner.md:43`.
3. **The end of a failing upstream turn does not trigger by itself.** Today `trigger` passes `upEndedOK=false` after any failed turn and `gate` reads `ends[id]=false` as `upMissing`. New rule: `ends` only says "just ended in this call"; `upstreamState` still reads the ledger for the keys, so a turn whose key will be retried is `upRunning`-like (left, no mark) and the final retry or the expiry turn triggers the dependent again. The "ended" state is computed in one function `upstreamEnded` in a new file `upended.go`, called from `upstreamState`; `runkey.go` (785 lines, near the 800 line) gets only the call.
4. **No pre-open computing without the mark, and the mark is always replaced.** A dependent of an XNYS or XTKS day that runs before its upstream ran is `upMissing` already (no key has a run), so it carries `upstream_missing`; Decision 2 makes the post-open end replace it. Rejected: a time guard that refuses the pre-open run, which breaks "any job runs when triggered" (the owner runs a backfill on purpose). What the owner must not meet is a stale pre-open result after the open, and Decisions 1 and 2 remove it.
5. **One run at a time stays.** The rerun goes through `runDependent` under the same `TryLock(d.ID())` (`runkey.go:612`); a running console run makes the trigger leave, and `dc.want` records the recheck, as today. No new lock, no new lane.
6. **The job's own fallback is untouched.** `option_candidates` still reads open rows first and takes the previous option_eod only for a name with no usable open chain (`candidates-at-open.md` Decision 1). Failed batches 002 and 011 become `eod_fallback` names, not a missing day. Fallback, not cascade: the job does not wait for or repair option_open.
7. **The page shows the count.** The job logs `open`, `eod_fallback`, `none` (`source.go:118`); the producer derives the same three counts from the stored candidates' marks (a name with the mark `eod_fallback` counts to the fallback, a name with a candidate and no mark to open) and prints one line above the table: "Open chain: N names, last close: M names, none: K". Names with no candidate at all are `none` only when the stored row says so [unverified: if the dataset holds no row for a name with no put, the line prints N and M only and says "of the names with candidates"]. Entry 3 reads the stored shape first and states which. The gloss of "last close" sits next to it in the page's own strings.

## Failure paths

- Upstream key stuck running with a live lease: not ended; the dependent waits; at the lease end the crash path runs, then `nextAttempt` decides.
- Upstream ended with every key failed: the dependent runs once, all names `eod_fallback`, mark `upstream_partial`; the page line shows open 0, which is what the owner needs to see.
- Upstream window closed with a key retryable and never rerun: `expireIfClosed` makes it `no_value`, which is ended.
- Console force run of the dependent while an automatic rerun is due: the lock serialises; the later one sees a stored key and, with no mark, does not rerun.
- Upstream ends after midnight of its calendar for a dependent on another calendar: `upstreamDay` is unchanged.
- Stale pre-open result already stored today (2026-11-20): replaced by the next end of option_open for the day or by a console run with force; the design does not rewrite ledger rows.

## Issues

### 1 candidates-source-counts-schema (interface)

Module: contracts and web. Size: S.
Order: none.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/option_candidates_counts_test.go`
Acceptance: `source_counts` is optional with integer `open`, `eod_fallback`, `none`; the generated type has it; a new test `TestOptionCandidatesCountsOptional` validates an instance without and with it.

### 2 reconcile-upstream-ended

Module: runner. Size: M.
Order: none.
Files: `internal/runner/reconcile/upended.go`, `internal/runner/reconcile/runkey.go` (only `upstreamState`, `gate`, `trigger` arguments), `internal/runner/reconcile/rerun.go`, `internal/runner/reconcile/upended_test.go`, `internal/runner/reconcile/rerun_test.go`, `docs/contracts/runner.md`
Acceptance: Checked by CI, a new test `TestDependentRunsWhenUpstreamEndedPartial`: a fake ledger with upstream keys 001 stored and 002 failed with retries exhausted; the dependent runs once with reason `upstream_partial` and is not rerun on the next tick. A new test `TestPreOpenRunIsReplaced`: a console run before any upstream key has a run gets `upstream_missing`; when the upstream ends with one key failed, it runs once more with `upstream_partial`. A new test `TestRetryableUpstreamIsNotEnded`: a key whose newest run is retryable before the run bound does not release the dependent. A new test `TestRetryableUpstreamEndsAtRunBound`: the same key after slot + run_bound releases it. `runkey.go` stays at most 800 lines and `runner.md` names the three reasons.

### 3 candidates-page-source-counts

Module: pages. Size: S.
Order: after 1.
Files: `internal/pages/option_candidates_page/counts.go`, `internal/pages/option_candidates_page/builder.go`, `internal/pages/option_candidates_page/strings.go`, `internal/pages/option_candidates_page/counts_test.go`
Acceptance: a new test `TestSourceCountsFromMarks`: 5 stored names, 3 with a candidate and no mark, 2 with `eod_fallback`: the page carries open 3, eod_fallback 2. A new test `TestSourceCountsAbsentWithoutRows`: no rows, the field is absent. The line and its gloss are in `strings.go`, no other string file changes.

### 4 candidates-page-source-counts-web

Module: web. Size: S.
Order: after 1 and 3.
Files: `web/src/pages/candidates/OptionCandidatesPage.tsx`, `web/src/pages/candidates/OptionCandidatesPage.test.tsx`
Acceptance: `tsc`, `eslint`, `vitest` and the build pass; a new test renders the line from an instance with `source_counts` and renders none without it. After merge the window opens `/candidates` and checks, with a screenshot (owner rule of 2026-11-14), that the line equals the stored marks of the day.

## Checked by CI

- `ci-go` for entries 1 to 3, `ci-web` for entries 1 and 4.

## After merge (checked by the dispatcher, not by review)

- The window runs `ot` console runs on a past test day with one option_open key failed: the candidates run carries `upstream_partial` and the page line shows the split.
- Next US session: option_candidates ends after option_open although a batch failed; the page line shows open N, last close M.

## Left out

- A time guard that blocks a pre-open run (breaks the dev-mode rule). Rejected, Decision 4.
- Rewriting the stale 2026-11-20 result: the next end or a force run does it.
- Raising the retry count of option_open or fixing why batches 002 and 011 fail: a separate fault.
- The JP session (`optioncandidates` JP job): the gate is generic, so it follows; no JP-specific change.

## Adversarial

- Gap against the goal: (1) met by Decisions 1 to 3; (2) met by the mark and its replacement, not by a ban, so a pre-open result exists for a while and the page shows it until the open pull ends. Adopted: named, because a ban breaks the owner's dev-mode rule. (3) met by entries 3 and 4. The 2026-11-20 stale result is not rewritten (Left out).
- Biggest unnamed risk: "ended" is a new notion computed from ledger rows and `nextAttempt`; a wrong end (a key counted ended while option_open still retries) makes candidates run on a half chain with `upstream_partial`, and that run is never replaced. Mitigation: the run bound is the only time-based end, and the job's fallback covers the missing names; the cost is some names on the last close, shown by the count line.
- Simpler way: let `rerunMarked` accept any upstream that is not running and not missing a run. Not adopted: it cannot tell a retrying key from a given-up key, which is the bug of the retries.
- Lock-in: a third mark `upstream_partial` in the ledger vocabulary and one optional page field; both are additive. `upstreamState` now reads keys through `nextAttempt`, so a later change to the retry rule changes the gate too.
- Claim most likely false: "a pre-open console run is marked `upstream_missing`" (Frame, Decision 4). It rests on `consoleMark` and an upstream with no run rows; not run against a ledger. Second: that the stored candidates' marks give the open count (Decision 7, flagged [unverified]); entry 3 must read the dataset first and may need a job-side count, which is a jobs entry.
- Likeliest slip: `runkey.go` is at 785 lines and the edit crosses 800 if the logic is not kept in `upended.go`; a second slip is `trigger` still passing `upEndedOK=false` and `gate` still reading it as missing (Decision 3 changes both call sites).
