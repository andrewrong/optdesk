# option_open at a gap: puts outside the planned band do not fail the run, and a stock that gapped is quoted on the strikes of its live band

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-21
Revision: 1

Source: owner product goal (sells puts at the US open) and the finding of 2026-11-20, a broad selloff, AMD -9.8 percent. Amends `option-open-pull.md` Decisions 8, 12 and 22 (and Decision 1, one sentence). Read on origin/main at d6c75f8c: `internal/jobs/optionopen/{core.go:225-361,pull.go:270-307,jp.go:290-322,pull_resume.go,planfunc.go:398-432,listed.go,job_pull.toml}`, `internal/store/openquote/api.go` (`OptionContractStore`, `ListRecord`), `internal/broker/api.go` (`Chain`, `ListChains`, `ResolveContract`), `docs/design/option-open-pull.md`, `docs/design/option-open-gap-jp.md`. Status: draft. Not the same as `option-open-gap-jp.md`, which is the pre-open resolve pass; this is the open pull at a price gap. The gate of `option_candidates` is `candidates-after-partial-open.md`, not here.

摘要（中文）：11 月 20 日大跌（AMD -9.8%）。option_open 开盘时用实时现价重算价带，计划里落在新价带之外的合约被标 OutsideBand、不询价；但收尾检查仍把它们算进"计划数"，于是一批只有 8/52 成功就判失败，重试也没用（再问一次，价带还是丢掉它们，91 毫秒结束），option_open 永远"结束不了"。更糟的是产品面：下跌时卖方想卖的 put（行权价低于新现价）根本不在昨日收盘价算出的计划里，开盘快照里这只股票只剩约 4 个合约，候选页恰好在最需要的日子里最薄。本设计两点：(a) 价带外的条目单独计数，不进分母；(b) 现价跑出已存价带时，用已存合约表和一次链列表，为新价带补选行权价并询价，总数仍在每股 60 条的上限内；没有已存合约的行权价，开盘时限量解析，上限写明。

## Frame

Today (facts):

- The plan is made at 08:45 ET on the prior close (`planfunc.go`, Decision 3); it holds puts from `close * exp(-k sigma sqrt(T))` to around the close (k 1.4), and the resolve passes store contracts to k 1.75 or 25 percent (`ListRecord.BandLo/BandHi`). At 09:45 `Core.band` (`core.go:329`) runs `PlanFunc` again over the stored plan items with the live spot; an item it does not keep is written `deferred` with `OutsideBand=true` and not asked (`core.go:240-243`).
- Gap down: the live band reaches lower strikes than the stored one, and the strikes the plan never held are not candidates of `band`, so it can only drop. AMD on 11-20: the stock's open snapshot held about 4 contracts. Decision 12 accepted this ("quoted on what the band holds"); the owner's goal (a put seller on the stocks that fell) says it is wrong exactly on the days it matters.
- Postcondition (`pull.go:281-307`, `jp.go:296-322`): `planned` counts every item that is not `Deferred` in the stored plan; an outside-band row is `deferred` and counted in no numerator. The batch ends "8 of 52 planned ok, under share 0.50" and fails; `pull_resume.go:25-50` asks the same items, the band drops them again, 91 ms. No retry can end it ok.
- What is stored: `master.option_contract` holds every resolved contract to the stored band; `master.option_list` holds the band, not the strike list; the broker lists a chain (`ListChains`, reqSecDefOptParams, one call per stock, strikes per expiry) and resolves one contract per call (`ResolveContract`, about 0.16 s each, 37 percent not listed, Decision 1).

Goal: at 09:45 on a gap day, `option_open` for a stock that moved ends ok with quotes for the puts of its live band, and `/options/candidates` has candidates for that stock; a stock whose band moved is never a reason for a failed batch.

Owner's standing rules this design respects:

- "Same job one at a time" (`check-design-against-owner-rules`, 2026-11-19): nothing here adds a job, a lane or a second run; the added asks are inside the one pull run of the key.
- "Fallback, not cascade" (2026-11-12): a stock whose live band cannot be completed keeps its stored items and the row says why; no stage waits for another.
- "Dev mode: no time guards" (2026-11-12): the gap path has no clock rule; it is taken by the spot, not by the hour. `stop_after` of the pull stays as it is.
- "Think as product" (2026-11-13): the candidates page for a put seller on a fallen stock is the first thing checked (Decision 6).

## Modules

jobs (`internal/jobs/optionopen`); pages is read only to confirm the row fields (no edit). No interface file, contract or `modules.md` row changes. `modules.md` is unchanged: the new file is in the same package.

## Interface

None changed. `contracts.OptionOpen` keeps its fields; a row of an added contract is an ordinary row (`OutsideBand` false). The run summary line (`summary.go`) gains one integer `outside_band`.

## Decisions

1. **Outside-band items are counted apart** (amends Decision 22a). In `pull.go` and `jp.go`, `planned` is the not-deferred plan items minus the rows of the run with `OutsideBand` true, and `need = min_ok_share * planned`. The summary and the failure text list `outside_band=<n>` next to `ok`, `partial`, `unread`. A batch whose every item is outside the band has `planned == 0` and ends `NoValue` "all outside band" (the runner's gap, not a failure). The change is the same in the US and JP count; one helper in a new file, used by both.
2. **The live band is computed over the stored chain, not over the plan items** (amends Decisions 3 and 12). For each stock, `Core` reads `ListOptions(underlying)` (every stored resolved contract, not only the plan's items) and runs `PlanFunc` with them as candidates at the live spot. The contracts it keeps are the asks. A kept contract that is not in the stored plan is an added item: it is asked, and its row is written like any row of the batch. A plan item it does not keep stays `deferred` with `OutsideBand` true (Decision 1 counts it apart). Held contracts are kept as today.
3. **Strikes the store does not hold.** When the live band's lowest put strike is below the stored `BandLo` of the group (`ListRecord`), the stock is "gapped". For a gapped stock, once per stock and expiry, `ListChains` gives the strike list; the strikes inside the live band that no stored contract has are resolved with `ResolveContract` (stored on success by `PutOption`, a not-listed answer written as a mark exactly as the evening resolve does, Decision 6 of `option-open-pull.md`), then asked. This is a bounded exception to the sentence of Decision 1 "resolving must not run at the open": at most `gap_resolve_max` (a value of `job_pull.toml` and `job_pull_jp.toml`, 24) resolves per stock, taken from the strikes nearest the live around-strike downwards, at most `ResolveInFlight` at a time. If the chain call or a resolve fails, the stock keeps the asks of Decision 2 and its summary row names the reason `gap_chain_failed` (a fallback, not a failure of the batch).
4. **Same item cap.** Items asked for a stock never exceed `max_per_stock` (60, the plan's cap; held contracts are outside it as today). The cap is applied by the plan function over stored and resolved candidates together, so the live band uses the same expiry order and ceilings as the plan; nothing is added over it. An added item that falls over the cap is `deferred` with `OutsideBand` false (a ceiling, as Decision 3 already says).
5. **The pull stays one run.** Reading the stored chain and the gap resolve happen inside `runKey` before the asks of the stock; the resume of `pull_resume.go` treats an added row like a planned one (it is final or asked again). No new key, job, retry or wait. A stock that did not gap costs one extra `ListOptions` read and nothing else.
6. **What the owner sees.** For a gapped stock the open snapshot holds the puts of the live band (about as many as the plan holds on a normal day), and `option_candidates`, which reads the open snapshot, has candidates for it. The candidates page already prints the source count; a stock with `gap_chain_failed` shows "fewer strikes than usual" only through the existing "outside the planned band" note of Decision 12, which stays for the contracts that are dropped.
7. **No chain list is stored.** Nothing new is stored: `ListChains` is read at the open for gapped stocks only. If a later design wants the strike list at 08:00, `option_open_resolve_gap` already calls `ListChains` and could keep it (`## Left out`).

## Failure paths

- `ListOptions` fails: retryable, as every store read of the pull.
- `ListChains` fails or lists no strike below `BandLo`: the stock keeps Decision 2's asks, reason `gap_chain_failed`, the batch is judged on the other stocks (no failure of its own).
- Resolve not listed: a mark, the row is `not_listed`; it counts as planned, not ok (it is a real answer of the broker).
- Gateway down: as today, `disconnected` rows, retryable.
- Every stock outside its band and nothing added (stored chain and chain call empty): `NoValue` as in Decision 1.
- A re-run of the key the same morning: the added contracts are stored now, so Decision 2 finds them without a resolve; the resume treats final rows as final.

## Issues

### 1 optionopen-gap-count

Module: jobs. Size: S. Order: first; no interface issue. Files: `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/jp.go`, `internal/jobs/optionopen/outside.go` (new), `internal/jobs/optionopen/outside_test.go` (new), `internal/jobs/optionopen/pull_test.go`, `internal/jobs/optionopen/jp_partial_test.go`, `internal/jobs/optionopen/summary.go`.

Acceptance:

1. `grep -c "outsideBand(" internal/jobs/optionopen/pull.go internal/jobs/optionopen/jp.go` prints a count of at least 1 for each file.
2. A new test in `outside_test.go` runs the US pull with 52 planned puts of which 44 are dropped by the live band and 8 asked and ok, and expects no error (before: "8 of 52 planned ok").
3. A new test in `outside_test.go` runs the same for JP and expects no error.
4. A new test runs a batch whose every item is outside the band and expects `NoValue`, not a `Permanent` error.
5. A new test with 52 planned, 30 outside and 22 asked of which 5 are ok expects an `underShare` error naming `outside_band=30` (the share still bites on what was asked).

### 2 optionopen-gap-stored-chain

Module: jobs. Size: M. Order: after 1 (the same two files). Files: `internal/jobs/optionopen/core.go`, `internal/jobs/optionopen/gapband.go` (new), `internal/jobs/optionopen/gapband_test.go` (new), `internal/jobs/optionopen/fakes_test.go`, `internal/jobs/optionopen/core_test.go`.

Acceptance:

1. A new test in `gapband_test.go` stores puts down to the stored band and a plan on a close of 100, gives a live spot of 90 and expects the asks to include stored puts of the live band that the plan did not hold.
2. A new test expects an added item's row to carry `OutsideBand` false and a plan item it dropped to carry it true.
3. A new test expects the asks of one stock never to exceed `max_per_stock`, held contracts apart.
4. A new test runs a stock with a live spot inside the stored band and expects the same asks as before this change (new test of no change).
5. `go vet ./internal/jobs/optionopen/` and the tests of the package are listed under `## Checked by CI`.

### 3 optionopen-gap-resolve

Module: jobs. Size: M. Order: after 2. Files: `internal/jobs/optionopen/gapband.go`, `internal/jobs/optionopen/gapband_test.go`, `internal/jobs/optionopen/job_pull.toml`, `internal/jobs/optionopen/job_pull_jp.toml`, `internal/jobs/optionopen/manifest.go`.

Acceptance:

1. `grep -c "gap_resolve_max" internal/jobs/optionopen/job_pull.toml internal/jobs/optionopen/job_pull_jp.toml internal/jobs/optionopen/manifest.go` prints 1 for each file.
2. A new test fakes a chain with strikes below the stored `BandLo`, a live spot below the stored band, and expects resolves of at most `gap_resolve_max` strikes, nearest first, stored by `PutOption`, and their rows asked.
3. A new test expects a not-listed resolve to be written as a mark and a `not_listed` row.
4. A new test expects a failing `ListChains` to leave Decision 2's asks, the reason `gap_chain_failed` in the summary, and no error.
5. A new test expects a second run of the key to make no resolve call for the stored contracts.

## Checked by CI

`go test ./internal/jobs/optionopen/...`, `go vet`, `golangci-lint` for the package; none is an acceptance item above.

## Left out

- Keeping the strike list at 08:00 from `option_open_resolve_gap` (`ListChains` is already called there): cheaper at the open, but a new store field and an interface change; later if the open resolve is seen to be slow.
- Planning from the pre-market price at 08:45: no pre-market quote of the stock is stored; it would still miss the move between 08:45 and 09:45.
- A second pull after the open for the gapped stocks: breaks "one run, one key".
- The candidates gate (`candidates-after-partial-open.md`) and the candidates page text.
- Calls on a gap up: calls are planned for held names only (Decision 3); unchanged.

## Adversarial

- Gap vs goal: the goal is puts on the fallen stock; Decisions 2 and 3 give them, Decision 1 only stops the false failure. Decision 1 alone would end the batches but leave AMD thin, so it is not enough.
- Biggest unnamed risk: the gap resolve runs at the open, at 0.16 s a contract with 37 percent not listed; on a broad selloff 30 stocks gap at once, 30 x 24 resolves is about 2 minutes behind `ResolveInFlight` 5, against the pull's own pacing and the 10:00 stop. The cap of 24 is a guess; a probe on the next gap day sets it.
- Simpler way: drop Decision 3 and use only the stored contracts (Decision 2). Cheaper and no open-time resolve, but a gap beyond the stored band (k 1.75) is still thin; kept as the fallback when the chain call fails.
- Lock-in: `gap_resolve_max` is a manifest value; Decision 2 changes who the candidates of `band` are, behind the same `PlanFunc` type.
- Claim most likely false: "the stored contracts already hold the live band for most gaps". The stored band reaches 1.75 sigma, the live band starts 1.4 sigma below the new spot, so a gap above 0.35 sigma sqrt(T) (about 3.5 percent) leaves the stored band; AMD at 9.8 percent did. Issue 2 test 1 pins the case.
- Likeliest slip: counting an added row as planned in the postcondition twice (as item and as row) when a re-run finds it stored; issue 1 test 2 and issue 2 test 2 both run a re-run path.
