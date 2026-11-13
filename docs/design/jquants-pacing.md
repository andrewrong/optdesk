# J-Quants pacing: the JP earnings leg spaces its calls 12 s apart and retries a 429 once after 61 s

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 2

Source: live failure, 2026-11-12 18:22Z: the JP leg of `earnings_refresh` failed with `rate_limited` for all 8 JP subjects in 178 ms. Owner rules: development mode (no time-of-day guard), prefer a mainstream library over our own code when one fits, own glue code sits behind an interface, the smallest design. Read on origin/main at 6b2f9662: `AGENTS.md`, `docs/design/modules.md`, `internal/sources/api.go`, `internal/sources/api_test.go`, `internal/sources/jquants/{client.go,client_test.go}`, `internal/jobs/earningsrefresh/{run.go,job.toml}`, `docs/contracts/sources.md`, `cmd/ot/{wire.go,runonce.go}`. J-Quants limits (https://jpx-jquants.com/en/spec/rate-limits, not fetched by the author, quoted from the owner's ask): Free 5 requests a minute, Light 60, Standard 120, Premium 500; a 429 carries no `Retry-After`; wait at least one minute. The plan in use is unknown, so the spacing fits Free. "[unverified]" marks a claim not checked in code or a probe.

Goal: on any plan, including Free, the JP leg of one run fetches every symbol: calls are 12 seconds apart (at most 5 in any minute), and a 429 waits 61 seconds and asks the same symbol once more.

## What main holds today

| Piece | On main |
|---|---|
| calls | `client.Fetch` (`internal/sources/jquants/client.go:60`) makes one GET per symbol in a row, no space between calls |
| 429 | `one` returns `rate_limited` of kind `Retryable` (`client.go:95`) and `Fetch` stops at the first failure |
| job | `earningsrefresh` wraps a failed market as `Permanent` (`run.go:297`, market-agnostic); an `error` row is fetched again by the next run (`run.go:200-233`, `TestErrorRowIsFetchedAgain`) |
| config | `sources.EarningsConfig` has `BaseURL`, `APIKey`, `Symbols`, `Doer`: no way to wait |
| wiring | `cmd/ot/runonce.go:578` builds the J-Quants config with `BaseURL`, `APIKey`, `Symbols` and `cmd/ot/wire.go:338` passes `jquants.New` |
| bound | `job.toml` `run_bound = "10m"` |

## Decisions

1. Spacing: the J-Quants client waits `callGap` = 12 seconds before every call after the first of a `Fetch`, so any 60 seconds hold at most 5 calls (the Free rate, in a sliding or a fixed-minute window). It is a package constant, not a config value.
2. Retry: a 429 waits `retryWait` = 61 seconds and asks the same symbol once more. If the second answer is also 429 the symbol fails as `rate_limited` of kind `Retryable` and the remaining symbols are not called (the first-failure rule stays). The retry is not spaced by `callGap` (it already waited 61 s). Other failures (5xx, `network`, 4xx, bad body) are not retried: the rule is about 429 only.
3. One injected wait: `sources.EarningsConfig` gains one field, `Sleep func(ctx context.Context, d time.Duration) error`. Nil means the real wait of the package (a timer and a select on `ctx.Done()`, returning `ctx.Err()` on cancel, about 8 lines). Both waits of Decisions 1 and 2 go through it, so a test passes a fake that records the durations and never sleeps. A `Sleep` error is reported as the existing `network` code (`jquants: request cancelled`, kind `Retryable`). Alpha Vantage ignores the field, as it ignores `Symbols`. No other field, no interface type.
4. Own implementation, because the whole pacing is one constant gap and one retry, about 15 lines in `Fetch`; a limiter library (`golang.org/x/time/rate`) would bring a `go.mod` and `go.sum` change and a second module into the entry, and its `Wait` (real clock, real sleep) could not be used under the no-sleep test rule, so only a hand-spelled fixed gap would remain of it. The glue is behind a func-typed config field, so a library can replace the body later without a caller change.
5. Time budget: 8 symbols take 7 x 12 = 84 s; one 429 adds 61 s; a persistent 429 on the first symbol ends after 61 s plus two calls. All are far under `run_bound = "10m"`. A watch list of about 49 symbols (48 x 12 = 576 s) is the most that fits on Free [computed]; above that the owner raises the bound, not guarded here. The run holds the runner fence as any run does, off the tick worker (`e9a1e2fd`); nothing here changes who holds it.
6. Wiring: `cmd/ot/wire.go:338` and `cmd/ot/runonce.go:578` need no change: they pass no `Sleep`, so the real wait is used. The job `earningsrefresh` is unchanged.

## Interface

Owning module: sources (entry 1). `internal/sources/api.go`: one field on `EarningsConfig` (`Sleep func(ctx context.Context, d time.Duration) error`, nil means the real wait; J-Quants only), the doc of `EarningsSource` ("never retries" becomes: "Alpha Vantage never retries; J-Quants spaces its calls and retries a 429 once"), and the doc of `NewJQuantsFunc`. Additive: every existing caller and fake compiles unchanged.

## Issues

Build order: every entry compiles and its own tests pass against main. BASE is `git merge-base origin/main HEAD`. A grep that "prints nothing" prints nothing and exits 1. An acceptance names a test by function name, so that `git grep -n "func TestName"` finds it; what the test asserts is in the same item. Every pull request of this design, and its own, carries `Contracts: unchanged`.

### 1 jquants-pacing-source (interface)

Module: sources. Size: S.
Files: `internal/sources/api.go`, `internal/sources/api_test.go`, `internal/sources/jquants/client.go`, `internal/sources/jquants/client_test.go`, `docs/contracts/sources.md`
Design: docs/design/jquants-pacing.md#interface (Decisions 1, 2, 3, 4)
Order: first.
Goal: add `EarningsConfig.Sleep`; in the J-Quants client wait 12 s between calls and retry a 429 once after 61 s, both through `Sleep`; make the package doc, the `EarningsSource` doc and the contract text say so; replace the test that pinned "no retry" and give the multi-symbol tests a fake `Sleep`, so none sleeps.
Acceptance:
1. `git grep -n "Sleep func(ctx context.Context, d time.Duration) error" -- internal/sources/api.go` prints one line, and `git grep -n "J-Quants spaces its calls" -- internal/sources/api.go` prints one line (the `EarningsSource` doc; it still says "Alpha Vantage never retries").
2. `git diff --name-only BASE -- go.mod go.sum` prints nothing, and `git grep -n "golang.org/x/time" -- '*.go'` prints nothing.
3. In `client.go` the two waits are constants and go through the injected func: `git grep -n "callGap\s*=\s*12 \* time.Second\|retryWait\s*=\s*61 \* time.Second" -- internal/sources/jquants/client.go` prints two lines; `git grep -n "time.Sleep" -- internal/sources/jquants` prints nothing (the real wait uses a timer and `ctx.Done()`); a nil `Sleep` in `New` becomes that real wait (read in `New`).
4. In `client.go`, `Fetch` still returns at the first failed symbol; the retry loop reads `http.StatusTooManyRequests` and no other status or code, and runs at most once per symbol; after the second 429 it returns `rate_limited` of kind `Retryable` (read in `Fetch` and `one`). `git grep -n "no retry" -- internal/sources/jquants/client.go` prints nothing (the package doc and the `Fetch` comment are rewritten).
5. `client_test.go`: `TestJQuantsRateLimitedNoRetry` is replaced: `git grep -n "func TestJQuantsRateLimitedNoRetry" -- internal/sources/jquants/client_test.go` prints nothing, and the new tests are `TestJQuantsRateLimitedRetriesOnce` (a doer that answers 429 then 200 on one symbol: two calls with that symbol's code, one recorded sleep of 61 s, rows returned), `TestJQuantsRateLimitedTwiceFails` (always 429, symbols 1111 2222 3333: two calls, one sleep of 61 s, code `rate_limited`, kind `Retryable`, the symbol 2222 never called), `TestJQuantsServerErrorNotRetried` (503, three symbols: one call, no sleep; it carries the 503 half of the replaced test), `TestJQuantsPacesCalls` (three symbols all 200: the recorded sleeps are exactly 12 s, 12 s; one symbol records none) and `TestJQuantsSleepCancelled` (a fake `Sleep` that returns `context.Canceled`: code `network`, kind `Retryable`, no further call); `TestJQuantsRealSleepCancelled` calls the package's real wait with an already cancelled context and a one-hour duration and gets `context.Canceled` back at once.
6. The tests that the change touches, all of `internal/sources/jquants/client_test.go`: `TestJQuantsOneCallPerSymbol` (three symbols) stays with its text unchanged, and stays green and fast because `newSource` installs a fake `Sleep` (it would otherwise take 24 s on the real wait); `TestJQuantsRateLimitedNoRetry` is the only test that turns red and is replaced as above; every other test of the package uses one symbol, makes no 429, and calls no wait. `newSource` keeps its signature and installs a recording fake; a second helper that returns the recorder serves the tests of item 5. `git grep -n "time.Sleep\|time.After" -- internal/sources/jquants/client_test.go` prints nothing. `internal/sources/api_test.go`: `TestNewJQuantsFuncContract` passes a config with `Sleep` set and the fake source ignores it.
7. `docs/contracts/sources.md` J-Quants section: the sentence "It never retries; the first failure (429 included) returns at once and no further call is made (Decision 12)" is replaced: `git grep -n "429 included" -- docs/contracts/sources.md` prints nothing; `git grep -n "12 s\|61 s\|EarningsConfig.Sleep" -- docs/contracts/sources.md` prints lines that name the 12 s gap, the 61 s wait, the single retry, the first-failure rule and `Sleep`; the contract-test list names the five new tests, the real-wait test and no longer names `TestJQuantsRateLimitedNoRetry` (`git grep -n "TestJQuantsRateLimitedNoRetry" -- docs/contracts internal` prints nothing). The Alpha Vantage line "It never retries and follows no redirect" stays.
8. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `go test ./internal/sources/... -race -count=1`, `go run ./scripts/arch --check`, `go build ./...` (the wiring in `cmd/ot` compiles unchanged).

Landed in: not yet.

## Left out

- Treating `rate_limited` as `Retryable` in `earningsrefresh` (`run.go:297`): the wrapper is market-agnostic, so it would also make a US Alpha Vantage `rate_limited` retryable and risk a re-run storm through the scheduler and reconcile (`runner/sched/gaps.go:170`, `runner/reconcile/reconcile.go`); a paced run should not see a 429, and a remaining 429 stays `Permanent` for the day, while its `error` rows are fetched again by the next run.
- A higher plan's rate from config or from `cmd/ot`: the 12 s gap works on every plan; a faster plan is a later change that makes the gap a field.
- The stale text of `docs/contracts/jobs/earnings_refresh.md` ("No retry the same day" and test names that do not exist): that file belongs to the jobs module, not to sources, so this design leaves it out; the owner may ask for a docs-only entry.
- A limiter library, a persistent request log or a cross-run limiter: two runs in one minute (a console run beside the scheduled one) can still exceed 5 a minute, and then the 61 s retry covers it once.
- Retrying 5xx or network failures, a per-symbol result (a failed symbol stops the others), `Retry-After` handling (J-Quants sends none), raising `run_bound`, a time-of-day guard, a plan probe.

## Doubts

1. The Free rate (5 a minute) is quoted from the owner's ask, not fetched; the 12 s gap holds in both a sliding and a fixed-minute window (first call at second 59 gives 2 calls in that window and at most 5 in the next).
2. The 61 s retry on a key already limited by another run can fail twice and end the run `Permanent` for the day; the next run's `error` rows fetch again. Accepted: the owner's dev mode runs jobs on trigger.

## Adversarial

The full answer is `jquants-pacing-adversarial.md` in the dispatcher's scratchpad (the read of revision 1).

1. Gap against the goal: adopted in part. (b) entry 2 is dropped (Left out); (c) the fence time is stated in Decision 5; (a) a fresh limiter on a second run stays, covered by the single 61 s retry and named in Left out.
2. Retryable re-run storm: adopted. The Retryable change is gone, so no scheduler behaviour changes; the "plan lacks the endpoint" case is not separated and stays a `Permanent` http_status or a failed 429 (Doubt 2).
3. Simpler way: adopted. No library, no `go.mod` or `go.sum` change, no `Waiter`, no `PerMinute`, `MaxRetries` or `Clock`; one `Sleep` field (Decisions 3, 4).
4. Lock-in: adopted. One func field instead of an interface and four fields; the 12 s and 61 s stay constants, named as such; no `x/time` dependency; no wiring claim left that `cmd/ot` sets a rate.
5. False claims: adopted. No grep expects "never retries" to vanish from a file that still says it for Alpha Vantage; each grep is scoped to a line that exists after the change; the `.Wait(` grep is removed.
6. Likeliest slip: adopted for entry 1 (the replaced test is named, `TestJQuantsOneCallPerSymbol` stays, every other test checked in item 6); entry 2 and its `TestPerMarketBothFail` case are gone with the entry; the `modules.md` two-module question is gone because `go.mod` is not edited.
