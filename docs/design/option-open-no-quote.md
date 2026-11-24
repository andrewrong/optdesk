# option open: a contract the broker answered with no quote is answered, not a failure

Options on the Tokyo exchange often have no market maker, so "no bid and no ask" can be the normal answer. The
option_open pull today treats it as a slow broker: the batch fails its ok share and the runner asks the same
contracts every 15 minutes. This design keeps two cases apart. (A) The broker answered and there is truly no quote:
normal, stored, counted as answered, left out of the ok-share test, never asked again. (B) We could not fetch
(error, timeout, disconnect, nothing arrived): a failure, retried, with a bound per day. It needs no migration, no
proto change and no pyside change. Owner direction "a", 2026-11-24. Plan: P2.

## Frame

Read on origin/main at 6f84b436.

- For a contract nobody quotes IB returns bid and ask as the -1 sentinel, market data type 1 (live), error code 0;
  IV, delta, sizes, close and volume arrive. pyside drops the sentinel and reports the field missing with reason
  INVALID, so `broker.Quote.Missing` carries `MissingInvalid` for `bid` and `ask` (`internal/broker/api.go`).
  Case A is therefore decided in Go from data that already arrives.
- `contentStatus` (`internal/jobs/optionopen/core.go`) gives such a row `stPartial`; `pull.go` and `jp.go` count it
  as partial, `underShare` ends the batch Retryable ("slow broker"), and the retry asks the same contracts.
- 2026-11-24: one JP batch of 29 items had 12 ok and 18 with no market; `min_ok_share = 0.5` failed it and the
  runner retried it 30 times. Retryable runs that end before the window close do not count in `nextAttempt`
  (`internal/runner/reconcile/reconcile.go`), so nothing bounds them.
- `option_candidates_jp` has `after = ["option_open_jp"]`. A dependent runs once every upstream key is stored or
  ended (`upstreamEnded`, `internal/runner/reconcile/upended.go`: a key is ended when `nextAttempt` would not start
  another run). Its source reads the rows of the whole day for the currency, whatever the batch
  (`internal/jobs/optioncandidates/source.go`, `begin`), so "usable rows of any batch" already holds once the
  dependent runs. The failed batch kept it from running.
- `obs.option_open.status` is `CHECK (status BETWEEN 1 AND 9)` (`migrations/0029_option_open.sql`). Status 3
  `no_data` already means "answered, nothing usable": the open view of `migrations/0062_base_fact_option_open.sql`
  counts 3 as `empty`, not as an error (errors are 5, 6, 8, 9). `error_code` is free text, written by the job
  (`TIMEOUT`, `ERROR`, a broker code).

## Modules

No row added or changed. Three modules: contracts (dataset doc), jobs (`optionopen`, `optioncandidates`), runner
(`reconcile`).

## Interface

No interface file changes. `docs/contracts/datasets/obs.option_open.md` gains one sentence (status 3 with error
code `NO_QUOTE`); `docs/contracts/runner.md` gains the retry cap in its counting rule. No proto, schema or
`internal/jobs/api.go` change.

## Decisions

1. **No migration: case A is status 3 `no_data` with error code `NO_QUOTE`.** No status number is added: the
   check stays 1..9, the views are untouched. Status 3 is the honest fit: the broker answered with nothing usable,
   and the open view already counts 3 as `empty`, not as an error. A new status 10 would need a migration of the
   check and of the 0062 views for the same meaning, so it is not taken. The marker is the error code because the
   table has no missing-reason column; adding one is a migration for a value the code can derive. Cost: a reader
   must read status and code together; the dataset doc says so (entry 1).
2. **When a row is `NO_QUOTE`.** The broker status is ok or partial, the market data type is 1, the error code is 0,
   `Bid` and `Ask` are nil and `Missing` holds `MissingInvalid` for both `bid` and `ask`. The row keeps every field
   that arrived (IV, delta, sizes, close, volume). Frozen or delayed types stay `no_data` with no code as today;
   a quote with one side INVALID stays `partial`; nothing arrived stays `no_data` with no code (case B).
3. **Answered means final and left out of the share.** `isFinal` (`pull_resume.go`) takes the stored row and is true
   for ok, not_listed, and status 3 with code `NO_QUOTE`; a retry or resume never asks such an item again, and no
   in-call retry applies (`retryable` already looks only at partial). In the batch count the row is neither ok nor
   partial nor unread. `postcondition` takes the outside-band rows apart first (all outside is still `NoValue`),
   then subtracts the no-quote rows from planned for the share test. A batch whose remaining items are all
   `NO_QUOTE` passes and ends done.
4. **Case B stays as it is, with a bound.** `disconnected`, `error`, `deadline`, `deferred` with a code, `partial`
   with a one-sided quote and nothing-arrived `no_data` still end the batch Retryable when under the share. The
   bound is the runner's, because the job does not see attempt counts: `nextAttempt` counts the key's retryable
   runs that ended before the close and, before the close, starts no new run once the count reaches 6 (constant
   `retryBeforeCloseCap`, a trial value: 6 runs at the 15-minute rhythm is about 90 minutes; IBKR account safety).
   The key is then ended, so `upstreamEnded` releases the dependents (`upstream_partial`) and the close table
   decides at the close as it does today. After the close the late run is unchanged.
5. **Candidates skip no-quote contracts.** `fromOpen` skips a row of case A like a `not_listed` one: it is not a
   quote of the chain and not a failure. The job does not end empty because of them, and the log line of the run
   gains `no_quote=<n>`. The page text is left out (see Left out).
6. **The cap is shared.** `nextAttempt` has no job identity, so the cap holds for every key whose retryable runs
   end before the close, not only option_open. 6 is above what the other windowed jobs use today (they end in one
   or two runs); the reviewer of entry 3 checks that no test of another job relies on more.

## Issues

### 1 obs-option-open-doc-no-quote

Module: contracts. Size: S.
Order: none.
Files: `docs/contracts/datasets/obs.option_open.md`
Acceptance: the status row names status 3 as "no_data: the broker answered with nothing usable", and one sentence says that status 3 with `error_code` `NO_QUOTE` is a contract the broker answered with no bid and no ask (IB's no-quote sentinel, live data, no error): stored, counted as answered, never asked again, and that status 3 with a null code is nothing usable arrived.

### 2 optionopen-no-quote

Module: jobs. Size: M.
Order: none.
Files: `internal/jobs/optionopen/core.go`, `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/jp.go`, `internal/jobs/optionopen/outside.go`, `internal/jobs/optionopen/pull_resume.go`, `internal/jobs/optionopen/no_quote_test.go`, `internal/jobs/optioncandidates/source.go`, `internal/jobs/optioncandidates/source_no_quote_test.go`
Acceptance: results as Decisions 2, 3 and 5. New tests: `TestRowStatusNoQuote` (both sides INVALID, type 1, no error: status 3 with code `NO_QUOTE`, IV and delta kept; one side INVALID: partial; frozen type: status 3 with no code), `TestNoQuoteRowIsFinalOnResume`, `TestNoQuoteLeavesOkShareDenominator` (US and JP: 12 ok and 18 no quote ends without error; all no quote ends done; all outside band is still `NoValue`), `TestCandidatesSkipNoQuote`.

### 3 reconcile-retry-cap

Module: runner. Size: S.
Order: none.
Files: `internal/runner/reconcile/reconcile.go`, `internal/runner/reconcile/retry_cap_test.go`, `docs/contracts/runner.md`
Acceptance: results as Decision 4. New tests: `TestRetryableRunsBeforeCloseAreCapped` (5 retryable runs before the close: a run starts; 6: it does not, and `upstreamEnded` calls the key ended), `TestRetryCapIgnoredAfterClose` (the late run still starts). `docs/contracts/runner.md` names the cap in the counting rule.

## Checked by CI

- `ci-go` for entries 2 and 3: `go build ./...`, `go vet ./...` and the tests named above.

## After merge (checked by the dispatcher, not by review)

- Next Tokyo open: a batch with no-market contracts ends done with `NO_QUOTE` rows stored, `option_candidates_jp`
  runs and its page is not empty, and the log holds no second ask of a `NO_QUOTE` contract.
- A batch that cannot fetch stops retrying after 6 runs.

## Left out

- A new status 10 and its migration: no honest need, see Decision 1.
- A pyside or proto change: the sentinel and the reason already arrive. The pyside spec still waits to the 15 s
  timeout for a bid that never comes; shortening that is a separate pyside design.
- Re-asking a `NO_QUOTE` contract later the same day: the owner's rule is never.
- The candidates page text naming the number skipped for no quote: a pages entry of its own if the owner wants it.
- A per-job cap: needs job identity in `nextAttempt`; Decision 6.
