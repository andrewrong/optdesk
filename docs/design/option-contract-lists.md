# Option contract lists: the real contract list of every watched US stock, taken in the evening and again before the open

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-11
Revision: 4 (design A of two; design B, the ladder, accounting, floor and page, is a file not on main yet and cites this one; revisions 1 to 3 are kept beside this file as `option-contract-lists.v1.md` to `.v3.md`, outside the repository; the reviews of revs 1 to 3 are folded in and answered in `## Adversarial`). This design supersedes no sentence of `option-open-pull.md`: its per-strike path stays true and keeps running.
Status: under review. Read on origin/main at c3503656. Probe files in `probes/listchains/`: `raw.txt`, `per-expiry.txt`, `parallel.txt`, `probes-5-6.txt` (read-only, port 4001). "[unverified]" marks a claim not checked in code, docs or a probe.

Goal. The seller needs the real contracts, with con_ids, of every watched US stock stored before the open, without depending on the day before (owner, 12:36Z: "必须有兜底方案"). Today the per-strike resolve guesses strikes and asks IBKR one by one. One `reqContractDetails` per (underlying, class, expiry) with no strike returns the real list, calls and puts together (`probes-5-6.txt`). This design adds that call and two new jobs that use it. It changes no existing job, no page and no plan; main's per-strike path keeps running beside it as the fallback. Design B consumes the stored lists. The indexes SPX, NDX and RUT are not in this design (`## Left out`).

## What main holds today

- `ContractsService` has `ListChains` and `ResolveContract` only. `broker.Broker` has no list call; `ReferenceClient` and `HistoryClient` are separate interfaces beside it.
- The details lane (`pacing.py`): timeout 20 s, then a 30 s pause, so a timeout costs 50 s; shared with dividends and bars. `parallel.txt`: 8 and 16 parallel calls finish in exact 5.0 s steps, so list calls do not overlap. **Every number below is 5.0 s per request.**
- The runner gives `Run` one key per call (`runkey.go:76`); `reconcileOn` runs the keys of a plan one after the other. Main's resolve jobs plan one key per stock, so each stock has its own budget clock and there is no order across stocks. Without `Bounded` a run bound is 30 minutes, and the deadline is max(window close, now) + bound (`runkey.go:253`). A key is present only with `Usable > 0`; `Err` nil with `Usable` 0 before the window close is `empty_before_close` and is retried every turn until the close; a run cut with `Returned > 0` is `partial` and runs again the same day, a crashed lease is re-run (`reconcile.go` `nextAttempt`).
- `runner.Scheduled`, `runner.Windowed` and `runner.Bounded` are optional methods with plain values on a job (`internal/runner/api.go:92-113`; `resolveJob` has them, `listed.go:97-104`), so a job package needs no edit of `internal/jobs/api.go`. A window is the session close plus a non-negative offset (`window.go`), so it closes at 16:00 at the earliest. `parseManifest` needs `budget_minutes` above 0, `[params]` and an `after_close` window (`manifest.go:80-100`). `stocks()` keeps US stocks only (`inputs.go:80`). `jobs.WatchEntry` carries `BuyPrice` (`internal/jobs/api.go:23`) in `Stored.Watch`.
- `option_open_resolve` runs at close+60m (17:00 ET), `option_open_resolve_gap` at open-90m (08:00 ET), `option_eod` at close (16:00, `run_bound` 16m, so cut near 16:16), `fundamentals` at 08:00 ET, `option_ledger` at close+60m (no window), the JP plan at open-45m (18:15 ET in winter); one worker (`ticker.go`), `reconcileTimeout` 2 h.
- `store()` writes the broker's strike text (`req.Strike`, listed.go:419), which is the broker's canonical decimal text; `strikeKey` (`inputs.go:130`, `RatString`) is for matching only: "12.5" becomes "25/2". `putOptionSQL` sets `resolved_at = now()` on every upsert and nothing reads it.
- `transport.py` (owner-only; 771 lines) registers `ContractsServicer` in `build_server`, so a new rpc is a method on that class. `ibgateway.py` has 799 lines; the ratchet fails a file over 800 not in `ratchet.json`, which is owner-only. `transport.py` imports generated types only through `pyside/src/optdesk/contracts/api.py`; the generated servicer's methods are abstract for mypy, so every subclass (`ContractsServicer`, `_FakeContracts` in `tests/contracts/test_contracts_api.py`) gains the method in the same change. gRPC has its default 4 MB message limit (no option on main); the largest answer probed is 516 contracts, about 50 KB.
- pyside deploy is the owner's (`docs/deploy/pyside.md`).

## Decisions

1. **The rpc.** `ContractsService.ListContracts(underlying, expiry, right, trading_class) -> ListContractsResponse{ repeated ContractSpec }`: one `reqContractDetails` with no strike; an empty `right` means both rights in one answer (Decision 7); con_id, right, strike, multiplier, class per contract. Read only. A timeout, disconnect or gateway error is a gRPC error, never a partial list (IB returns the whole detail list or an error). `list_contracts` joins the log `WORDS`, and `ListContracts` the live `ALLOWED_RPCS` (rpc names, checked against the stubs by `test_guard.py`), as a read.
2. **Where the pyside code lives.** New file `pyside/src/optdesk/ibkr/lister.py`: `ListingGateway(IbGateway)` adds `list_contracts` on the details lane as `resolve` does, formatting every strike with the existing `_decimal_text` (the table's CHECK at `migrations/0032_option_contract.sql:18` is `^[0-9]+(\.[0-9]+)?$` on `strike` text, and the key has `strike` as text, so "12.5" and "12.50" would be two rows); `ibgateway.py` is not edited. New file `list_rpc.py` (under 200 lines): validation, the answer, the proto conversion. A protocol `ListerGateway` in `model.py`, as `AccountGateway` and `ExecutionsGateway`, not a method of `Gateway`, so no fake of `Gateway` breaks. `server.py`'s default `build` is `ListingGateway`. **The edit of `transport.py` is the delegating method and nothing else**: an import, the method on `ContractsServicer` (UNIMPLEMENTED when the gateway has no `list_contracts`; else `self._shared.call("list_contracts", context, list_rpc.answer(...))`) and the word in `WORDS`; at most 15 lines, 771 to at most 786 before `ruff format`; the pull request states the count after formatting, and the entry fails if it is over 800.
3. **Go side.** `broker.ContractLister` (`ListContracts(ctx, underlying Contract, expiry Day, right, tradingClass string) ([]Contract, error)`, `right` empty for both) beside `Broker`; `Dial` still returns `Broker`; the concrete client implements both. UNIMPLEMENTED from an old service is `broker.ErrNotServed` (a sentinel in `api.go`, wrapped in a Permanent error so `errors.Is` works). The Go side keeps only contracts of the requested class and the chain's multiplier. Strike text is the form main already stores: the Python side formats the float strike of a contract detail with `_decimal_text` (`ibgateway.py:143`: "660", "12.5", no trailing zeros), as it does for `ListChains` strikes that `ResolveContract` echoes.
4. **Two new jobs, one runner key each, their own parser and values.**
   - *Jobs.* `option_list_evening` (close+30m, 16:30 ET, order 115: after `option_candidates` 110, before `option_open_resolve` 120) and `option_list_preopen` (open-120m, 07:30 ET, order 185: before the gap's 190). `Plan` returns **one** key: dataset `master.option_list`, subject `evening` or `preopen`. `Run` gets all names in one call (`runkey.go:76`), builds every group, sorts the calls once and stops at one clock: held names, then **priority names** (a `Stored.Watch` entry with a non-empty `BuyPrice`, the owner's price), then the rest; inside a class, nearest expiry, then symbol; a name whose `ListChains` expiry set differs from the stored lists goes first in its class. Priority is bounded by the budget only; held is guaranteed (Decision 9). The jobs do not go through `cmd/ot/listedjob.go`: the one key does not depend on the names, and `Run` reads the stocks at its start.
   - *Parser, window, bound.* `list.go` has its own parser and its own `Schedule()`, `Window()` and `RunBound()`. Evening: window `after_close`, `lateness_hours` 5 (as the resolve), `RunBound` 92 minutes. Pre-open: window `after_close`, lateness 0 (closes 16:00), `RunBound` 31 minutes. A bound is the budget plus 50 s plus slack. The evening bound never binds: the deadline is the window close (21:00) plus the bound, and the 90-minute budget cuts first. The pre-open bound (16:00 + 31 minutes for a 07:30 start) never cuts a normal run either; it matters only for a start after the close. The job's own budget and `stop_time` are the cuts. Without `RunBound` the default 30 minutes would apply.
   - *Budget and stop time.* `list_budget_minutes` 90 (evening) and 30 (pre-open: 07:30 to 08:00). The pre-open job also has `stop_time` = 08:30 ET (initial value), a clock time in code: a window cannot end before the plan at 08:45; the stop time bites only on a late start (a down gateway at 07:30, a long earlier turn).
   - *Usable and the end states.* `Usable` is the number of names whose run row for today's session and this pass has `listed` above 0 (`listed_session` today), not the number of rows; so a harmless re-run after a pass that did its work stays present (`verdict`, `runkey.go:172-196`: present only with `Err` nil and `Usable` above 0, l.188; `Err` nil with `Usable` 0 before the close is `empty_before_close` and is retried each turn, l.190-193; after the close it is `no_value`, l.195; no `Postcondition`). **`Err` is nil only when `Returned` is above 0 or the reason is `stop_time`.** A pass that lists nothing (`Returned` 0) because of timeouts, `listchains_failed`, `broker_down` or `budget` ends Retryable before `stop_time`: before the window close a Retryable run is not counted and is retried every turn; after the close it counts and the key expires as `no_value`, which is visible. A run that starts after `stop_time` makes no call, writes the rows with reason `stop_time` and ends Err nil with `Usable` 0 (unless an earlier run of the day listed names, then `Usable` is above 0 and the key is present): the runner retries it each turn as a no-op that returns before any broker call or store read (its whole cost is one run row write per turn), and the key expires as `no_value` after the close, visible: that day's pre-open pass did not run. This is intended. A pass that lists something and then ends on the budget, `list_max_new` or two consecutive timeouts ends Err nil with `Returned` = groups listed. Tests: `TestPreopenStopsAtStopTime` (fake clock 08:29:55, a request of 10 s, one more group: the second request is not made, reason `stop_time`); `TestLateStartMakesNoCallAndStaysMissing` (clock 08:31, nothing listed today: Err nil, zero broker calls, every row `stop_time`, `Usable` 0; and a second case where an earlier partial run listed names: the late re-run has `Usable` above 0 and is present); `TestPassListingNothingIsRetryable` (a fake broker whose every call times out, and one whose `ListChains` always fails, at 07:30: the result is Retryable with `Usable` 0, and Err nil with the reason `stop_time` only once the clock is past 08:30).
   - *Reconcile re-runs.* A run cut at `reconcileTimeout` with `Returned` above 0 (`partial`) or a crashed lease runs again the same day. The re-run skips groups listed today, and before `stop_time` it continues, after it it is the late-start case; rows are updated monotonically (Decision 6), so a no-op re-run never lowers a `listed` count. A pass that listed something and ended on its budget is present and is not re-run; the next pass continues.
5. **Until pyside is restarted.** The first `ErrNotServed` ends the pass: Permanent with the text `list_unserved: the pyside service has not been restarted`, and every name's run row gets reason `unserved`. The ledger keeps only the kind and `safeErrText` cuts at 300 bytes, so names live in the run rows and the job's summary line (`Log`). Main's per-strike jobs run at their own slots as today. The owner restarts pyside (Rollout).
6. **The writer and the records.**
   - *Writer.* Its own function in `list.go` (`listed.go`'s `store` needs a `keyRun`, `resolveJob` and `candidate`): per contract `Instruments.ResolveInstrument` (instrument from `osiOf`, same package) and `Options.PutOption` (`ListDeps` embeds `Stored`, which has both). **The strike is stored as the broker gave it, unchanged, as `store()` does (`listed.go:419`)**; **matching uses `strikeKey`** (`refKey`, `listed.go:343`), never the stored text. "Already stored" is one `ListOptions` read per name, matched on (right, strike by `strikeKey`, expiry, class); only new contracts are written, and the marks of what it lists are removed.
   - *Bounds.* Writes run inside the budget clock and stop at `list_max_new` = 60000 contracts per pass (initial): the first pass on an empty store lists about 180 groups of 200 to 500 contracts (both rights), 36 to 90 thousand, each a `ResolveInstrument` and a `PutOption` of unmeasured time, so it may end on the budget or the cap and the next pass continues.
   - *Real list.* A list is real when the call succeeds, both rights are present, it is not empty for an expiry `ListChains` lists, and it holds at least half (`list_min_ratio` 0.5) of the stored contracts of that group; otherwise the group keeps its older contracts and the outcome is `short` or `error`.
   - *Tables* in migration `0034_option_list.sql`. **`master.option_list`**, one row per (underlying, expiry, class) asked: `listed_at`, `listed_session` (the trading day), `count`, `tried_at`, `outcome` (`listed`, `short`, `error`). **`master.option_list_run`**, one row per (session, pass, underlying): `expected` (the expected groups, a list of `expiry|class` from the name's `ListChains` of that run), `listed` (groups with `listed_session` today), `reason` (`not_finished`, `none`, `listchains_failed`, `budget`, `stop_time`, `timeouts`, `unserved`, `broker_down`, `list_max_new`), `updated_at`. Rows are written at pass start with reason `not_finished` and updated per name as it ends, with writes that survive a cancelled context (`context.WithoutCancel`), so a crash or a cut leaves rows that read not finished. `expected` is set once; `listed` never decreases; `reason` is the latest, but **a re-run writes a reason only when `listed` is below `expected`**, so a no-op re-run never replaces `none` on a complete row. **A watched US stock with no row for a session and pass is a gap** (the pass did not reach it or died before writing). B reads both tables.
   - *Order of writes.* Contracts first, then the `option_list` row, then the run row: a failure between leaves the older row and the group is listed again. `short` and `error` update `tried_at` and `outcome` and keep `listed_at`; a second `short` on a later session is accepted. "Listed today" is `listed_session` equal to today's trading day, never `resolved_at`. Age is in trading sessions: `NextSession` (`internal/clock/api.go:31`) is the first session strictly after a day, so age is a loop of at most 5 calls. Role `ot_w_option_open` (INSERT, UPDATE, SELECT on both; no page role); B's migration is numbered after 0034.
7. **Classes and rights; one request per (underlying, class, expiry).** Class per stock: the symbol's own class; adjusted classes (`2SPY`, `2NVDA` in `raw.txt`; multiplier 100, so a multiplier filter would not drop them) are never asked and B names them as left out. **The right is always left empty**: SPY 20261218 unset gave 516 contracts (258 C, 258 P) in 0.84 s, against 4.49 s for the puts alone and 4.98 s for the calls; strike sets equal, con_ids distinct, the unset answer is their union; a contract is matched on (right, con_id). So calls of every name cost nothing extra: both rights are stored, the plan uses puts for non-held names and calls for held ones as today, and there is no per-right key and no carve-out for held calls. It is one sample of the unset case: an answer with only one right for an expiry is `error` and the group keeps its older contracts. A month string (`202612`) returns every expiry of the month (SPY: 5 expiries, 1034 puts, 15.6 s, the sum of five calls), gains nothing and comes near the 20 s timeout: **requests stay per expiry**. Latency is 1 to 7 s whatever the rows (QQQ daily 161 puts 6.5 s, QQQ monthly 352 puts 7.2 s); 5.0 s stays the planning figure.
8. **Expiry choice, `ChooseExpiries`** (a pure function in `list.go`; B applies it in the plan so a list is never taken for an expiry the plan drops). `max_expiries` is **pending the owner**: (a) the nearest N to DTE 30; (b) weekly expiries up to N weeks and the next two monthlies; (c) everything within N days. Initial value: (a) with N = 6 plus the next two monthlies if not among them (`keep_monthlies = 2`), so the expiry a seller uses is never dropped; a name with fewer keeps all. Held contracts of dropped expiries are not lost (the held path unions them); watched ones are named in B.
9. **Arithmetic at 5.0 s, per (underlying, class, expiry).** Inputs are assumptions: 45 US stocks, 12 held, 5 daily-expiry names (SPY, QQQ, IWM and the like) at 8 expiries each, 40 others at **3.5 expiries each (an assumption, not a measurement)**: 40 + 140 = **typical 180 requests, 15 minutes** (22 at 7.3 s). Worst, every name 8 expiries: 360 requests, 30 minutes (44 at 7.3 s). The 90-minute evening pass holds both. The budget is checked between calls, so a lane timeout (50 s) overshoots it by that much. These are IB calls only; Decision 6 states the writes. Pre-open: `ListChains` for 45 names as 5.0 s calls is 3.75 minutes [unmeasured], leaving 26.25 minutes = **315 requests** inside the 30-minute budget: the typical 180 are listed today by 07:49 even if the evening failed entirely; in the worst case 315 of 360. Held is guaranteed: 12 names x 8 expiries = 96 requests = 8 minutes. Priority has no bound but the budget. Two consecutive timeouts stop the pass (`stopAfterTimeouts`, `listed.go:35`).
10. **Collisions on the one worker, named.** *Pre-open:* 07:30 to 08:00 by budget, at most 08:01 (one 50 s timeout), so it ends before `fundamentals` and the gap at 08:00 and the plan at 08:45. If the gateway is not up at 07:30 (probe 3 pending) the key is retried each turn and never starts after 08:30 (Decision 4); a late run delays `fundamentals` and the gap (budget 40 minutes from their start, unchanged on main), so the gap can end near 09:10 and the plan waits behind it: the cost of a late gateway, bounded by the stop time. *Evening:* `option_eod` is cut near 16:16, so the pass starts at 16:30 and runs to 18:00 at most, ahead of `option_ledger` and `option_open_resolve` (17:00); typically it ends at 16:45 and nothing waits; in the worst case they start up to an hour late and the JP plan at 18:15 ET waits behind them (a delay, not a failure). A turn beyond `reconcileTimeout` (2 h) is cut and the run is `partial`.
11. **Probes pending, and what holds without them.** Probe 3 (gateway at 06:00 to 08:00 ET): default 07:30 with the retry and stop time of Decision 4; nothing claims the gateway answers early. Probe 5 (daily-expiry counts): the arithmetic uses the cap. Probes 6 and the QQQ and SPX parts of 5 are done (Decision 7). An empty answer for an expiry `ListChains` lists is `error`, recorded and named. Probes 7 and 8 belong to B.

## Modules

No new row. First matching glob of `docs/design/modules.md`.

| module | paths touched | change |
|---|---|---|
| contracts + pyside (judged cut, entry 1) | `contracts/proto/ibkr/v1/resolve.proto`, `internal/contracts/genpb/**`, `pyside/src/optdesk/contracts/genpb/**`, `pyside/src/optdesk/contracts/api.py`, `pyside/tests/contracts/test_contracts_api.py`, `docs/contracts/00-protocol.md`, `pyside/src/optdesk/ibkr/{model,lister,list_rpc,transport,server}.py`, `pyside/tests/ibkr/**`, `pyside/tests/live/**` | the rpc, its generated code, its pyside side; owner-only path |
| broker | `internal/broker/api.go`, `api_test.go`, `grpcbroker/**`, `docs/contracts/broker.md` | `ContractLister`, `ErrNotServed`, the client |
| store | `internal/store/openquote/api.go`, `api_test.go`, `internal/store/optionopenpg/**`, `migrations/0034_option_list.sql`, `docs/contracts/store.md` | the list rows and run rows |
| jobs | `internal/jobs/optionopen/**` | the two list jobs, new files only |
| ot | `cmd/ot/wire_optionlist.go` (new), its test, `docs/contracts/ot/jobs/option_list_*.md` | wiring |

## Interface

- Entry 1: `ListContracts` and its messages; `ListContractsRequest`, `ListContractsResponse` exported in `pyside/src/optdesk/contracts/api.py`; `ListerGateway` in `model.py`. Both-sides tests: `test_list_contracts.py` (servicer with a fake `ListerGateway`, and with a gateway without it: UNIMPLEMENTED) and `_FakeContracts` in `test_contracts_api.py`.
- Entry 2: `broker.ContractLister`, `broker.ErrNotServed`, declarations only; `api_test.go` and `grpcbroker/lister_test.go` test it from both sides.
- Entry 3: `openquote.ListRecord`, `openquote.ListRunRecord` and `openquote.OptionListStore` (`PutLists`, `Lists`, `PutListRuns`, `ListRuns`), separate from `Stores`.
- Jobs: `ListDeps` (embeds `Stored`; types of the package; `internal/jobs/api.go` is not touched).

## Issues

Six entries; entries 1 and 3 are interfaces and 1 is first. Each lists every file it touches and builds alone on main plus the entries it is ordered after; no two entries share a file. The tomls carry only keys that `list.go` reads in the same entry. Acceptance states results checkable with git; test runs are under `## Checked by CI`.

### 1 list-rpc (interface; the design's judged cut: contracts and pyside, owner-only)

Files: `contracts/proto/ibkr/v1/resolve.proto`, `internal/contracts/genpb/resolve.pb.go`, `resolve_grpc.pb.go`, `resolve_proto_test.go` (same directory), `pyside/src/optdesk/contracts/genpb/ibkr/v1/resolve_pb2.py`, `resolve_pb2.pyi`, `resolve_pb2_grpc.py`, `resolve_pb2_grpc.pyi` (same directory), `pyside/src/optdesk/contracts/api.py`, `pyside/tests/contracts/test_contracts_api.py`, `docs/contracts/00-protocol.md`, `pyside/src/optdesk/ibkr/model.py`, `lister.py`, `list_rpc.py`, `transport.py`, `server.py` (same directory), `pyside/tests/ibkr/test_list_contracts.py`, `pyside/tests/ibkr/test_lister.py`, `pyside/tests/live/allowed.py`, `pyside/tests/live/client.py`
Design: docs/design/option-contract-lists.md#Decisions (1, 2)
Order: first.
Contracts: ibkr.v1.resolve v1 additive
Owner case: `transport.py` is an owner-only path and the files span contracts and pyside, the shape of `account-data.md:74` (the judged two-module entry whose owner-only `transport.py` stub the owner merges). One pull request: the generated servicer's methods are abstract, so the proto, `api.py`, the fake, the servicer and the lister cannot pass `mypy --strict` apart. The owner merges this pull request.

Acceptance:
- `git grep -c "rpc ListContracts" contracts/proto/ibkr/v1/resolve.proto` prints 1; no existing message or rpc changes; the generated files carry the source change.
- `ibgateway.py` and `ratchet.json` are not in the diff; `transport.py` gains at most 15 lines and the pull request gives `wc -l` after formatting (at most 800); `lister.py` is under 300 lines and `list_rpc.py` under 200.
- `test_lister.py` has a case that a float strike 660.0 is listed as "660" and 12.5 as "12.5" (never "660.0" or an exponent), through `_decimal_text`.
- `Gateway` in `model.py` is unchanged; `test_seams.py` and `test_transport.py` are not edited; `_FakeContracts` has the method.
- `list_contracts` is in `WORDS` and `ListContracts` in `ALLOWED_RPCS`; the live client adds no write rpc.

### 2 list-broker (interface, broker)

Design: docs/design/option-contract-lists.md#Decisions (3)
Files: `internal/broker/api.go`, `internal/broker/api_test.go`, `internal/broker/grpcbroker/lister.go`, `internal/broker/grpcbroker/lister_test.go`, `docs/contracts/broker.md`
Order: after 1.
Contracts: unchanged

Acceptance:
- `api.go` declares `ContractLister` and `ErrNotServed` and nothing else new; `Broker` and `Dial` are unchanged.
- `lister_test.go` has one test per case: contracts of other classes dropped, a gRPC error returned, UNIMPLEMENTED is `ErrNotServed` through `errors.Is`, and the strike text of a listed contract equals that of `ResolveContract` for the same contract.

### 3 list-store-api (interface, store)

Design: docs/design/option-contract-lists.md#Decisions (6)
Files: `internal/store/openquote/api.go`, `internal/store/openquote/api_test.go`, `docs/contracts/store.md`
Order: first (touches no file of another entry).
Contracts: unchanged

Acceptance:
- `api.go` adds `ListRecord` (underlying id, expiry, class, `ListedAt`, `ListedSession`, `Count`, `TriedAt`, `Outcome`), `ListRunRecord` (underlying id, session, pass, `Expected`, `Listed`, `Reason`, `UpdatedAt`) and `OptionListStore` with the four methods; `Stores` is unchanged; `api_test.go` has a fake and a caller.
- `store.md` documents both tables, the reasons of Decision 6, "listed today is `ListedSession`, never `resolved_at`", and "a watched US stock with no run row is a gap".

### 4 list-store-impl (store, migration)

Files: `internal/store/optionopenpg/lists.go`, `lists_test.go`, `lists_pg_test.go` (same directory), `migrations/0034_option_list.sql`
Order: after 3.
Contracts: unchanged

Acceptance:
- The migration creates `master.option_list` keyed by (underlying, expiry, class) with CHECKs on class and outcome, and `master.option_list_run` keyed by (session, pass, underlying) with a CHECK on `reason` (the nine values of Decision 6), owner `ot_owner`, INSERT, UPDATE, SELECT on both to `ot_w_option_open`, nothing to a page role; `git grep -c "0034_option_list" migrations` prints 1.
- `lists_test.go` shows `short` and `error` keep `listed_at`, `PutListRuns` never lowers `Listed` and keeps `Expected` once set, and `ListRuns` returns the rows of a session and pass; `lists_pg_test.go` carries the build tag `pg` and applies 0034 itself.

### 5 list-jobs (jobs; new files only)

Files: `internal/jobs/optionopen/list.go`, `list_run.go`, `list_test.go`, `list_run_test.go`, `job_list_evening.toml`, `job_list_preopen.toml` (same directory)
Order: after 2 and 3.
Contracts: unchanged

Acceptance:
- `listed.go`, `manifest.go` and the four existing tomls are not in the diff; each new file is under 600 lines.
- `Plan` of each job returns exactly one key (`master.option_list`, `evening` or `preopen`); a test drives `Run` once with that key and asserts the call order of Decision 4 across names and one budget clock.
- Own parser and values: a test parses both tomls with `list.go` and not with `parseManifest`; `Window()` is `after_close` lateness 5 (evening) and 0 (pre-open); `RunBound()` is 92 and 31 minutes; the tomls carry `list_budget_minutes` (90, 30), `max_expiries = 6`, `keep_monthlies = 2`, `list_min_ratio = 0.5`, `list_max_new = 60000` and `stop_time = "08:30"` (pre-open only), each read by `list.go`.
- `TestPreopenStopsAtStopTime` and `TestLateStartMakesNoCallAndStaysMissing` of Decision 4 exist; a re-run after a cut skips groups listed today and does not lower a run row's `Listed`.
- A list row and a per-strike row of one contract are one row: a test stores the contract once as `ResolveContract` returned it and once from a list (the same strike text "660" or "12.5", as the Python side formats it; a variant spelling "660.0" is also listed once, to show `strikeKey` matching keeps one row) and the fake store holds one row, and `refKey` of main's evening resolve finds the listed contract in `cached` (`listed.go:343`); the stored strike text is the broker's, never `strikeKey`'s ("25/2").
- Run rows: written at pass start with `not_finished` and updated per name; a failed `ListChains`, a stop at the budget or stop time, two consecutive timeouts, `ErrNotServed`, a down broker and `list_max_new` each leave the reason of Decision 6 and the expected groups; the summary line names them; the error text carries none; no more than `list_max_new` contracts are written in a pass.
- `ErrNotServed` ends the key Permanent with `list_unserved`; a `short` list keeps the contracts and a second `short` on a later session is stored; an answer with one right is `error`; no `Sleep`.

### 6 list-ot-wire (ot)

Files: `cmd/ot/wire_optionlist.go`, `cmd/ot/wire_optionlist_test.go`, `docs/contracts/ot/jobs/option_list_evening.md`, `docs/contracts/ot/jobs/option_list_preopen.md`
Order: after 4 and 5; merges after `ot setup pg` and migrate have run (Rollout).
Contracts: unchanged

Acceptance:
- The two jobs are registered with orders 115 and 185 (`option_candidates` is 110, `option_open_resolve` 120, the gap 190), which put the evening list before the resolve and the pre-open list before the gap; they do not go through `listedjob.go`; the broker is asserted to `ContractLister` and the store to `OptionListStore`; a broker without them gives `list_absent`, logged once, and the key ends Permanent for the day; `wire_optionopen.go` is not in the diff; the docs state the keys of entry 5.

## Rollout

Entries 2 to 5 need no owner step. Entry 1 is the owner's merge. After entry 4 merges the window runs `ot setup pg`, migrate, `ot setup pg` (migration 0034); then entry 6 merges and `ot` deploys itself. **The owner restarts pyside** after entry 1 merges (`docs/deploy/pyside.md`: deploy is the owner's, gateways logged in first); before it both list jobs end `list_unserved` and main's per-strike jobs run as today. No secret, role or env line is new.

## Checked by CI

`go test` of `internal/contracts/genpb`, `internal/broker/...`, `internal/store/...` (tag `pg` in GitHub CI), `internal/jobs/optionopen`, `cmd/ot`; `buf` generation matches the committed files; `uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`; the ratchet; the `Contracts:` line check.

## Left out

- The ladder, rung names, `list_old`, accounting reasons, the floor, the plan and pull use of `ChooseExpiries`, the page: design B, which cites this one. B names what is not listed: dropped expiries, adjusted classes (`2SPY`), and the indexes.
- **The indexes SPX, NDX and RUT**: `stocks()` keeps US stocks only (`inputs.go:80`), so they are not in the sources of the jobs, and an index list needs its own name source and class rules. Who does it: a later design C, to be drafted after this one lands. Probe facts for it: `ListChains` of SPX shows the classes SPX (20 expiries), SPXO (2) and SPXW (40); **SPXW is verified** (SPXW 20261211: 185 puts, 3.9 s; SPX 20261217: 543 puts, 1.6 s); NDXP and RUTW are [unverified]; no NDX or RUT list was probed.
- A union rung from `ListChains` alone (it over-lists, `raw.txt`). Starting before 07:30: only after probe 3.

## Adversarial

Rev 4 answers the review of rev 3, then earlier points.
- *Strike text (B2).* Adopted: the broker's text is stored unchanged, `strikeKey` only matches ("25/2" would break `strike::numeric`); acceptance: one row for a list and a per-strike contract (Decision 6, entry 5).
- *Window, bound, Usable, end states (B1).* A pass that lists nothing is a failure to show: `Err` is nil only when `Returned` is above 0 or the reason is `stop_time`; a pass listing nothing ends Retryable before `stop_time` (retried each turn, `no_value` after the close, visible); a run after `stop_time` makes no call, ends Err nil with `Usable` 0 and stays missing (retried as a no-op, `no_value` after the close: that day's pre-open pass did not run, intended); `Usable` counts names with `listed` above 0 today for this pass, so a re-run after a pass that did its work stays present; tests `TestPassListingNothingIsRetryable` and `TestLateStartMakesNoCallAndStaysMissing`. The values of Decision 4 stand: the evening bound never binds, the pre-open bound matters only for a start after the close.
- *Reconcile re-runs (S2).* Corrected: a partial or crashed run runs again the same day; a re-run respects `stop_time`, never lowers `listed` and writes a reason only when `listed` is below `expected`; a pass that listed something and hit its budget is present and not re-run.
- *Run table in the interface and store entries (S1).* Adopted: `ListRunRecord` and four methods in entry 3, migration, grants and tests in entry 4; rows written at start as `not_finished`, `expected` is a list, no row is a gap, `list_max_new` is a reason.
- *Indexes.* Left out by name with the probe facts (the jobs' sources are US stocks only).
- *One-worker collisions.* The evening pass 16:30 to 18:00 can delay the ledger, the resolve and the JP plan by up to an hour (a delay, not a failure); the pre-open pass is done by 08:01 normally and bites the 08:00 jobs and the plan only on a late gateway, bounded by 08:30.
- *Earlier points, standing.* Gap against the goal: expiries pending the owner (Decision 8), `ListChains` at 5.0 s counted. Entries 1 and 3 cannot land apart: one owner-merged entry. A simpler way (band-only, the existing gap): not adopted, no measured gap-pass duration exists and the owner asked for no dependence on yesterday; the new jobs touch nothing existing, so it stays as the fallback. Lock-in: `list_min_ratio`, "second short", `listed_session` and the run reasons are what B reads. The ratchet: `transport.py` at most 786 before formatting, stated after. The pyside restart is the owner's second step and the jobs say `list_unserved` until it.
- *Text fixes.* `ListContracts` in `ALLOWED_RPCS` in entry 1 too; one size limit for `list_rpc.py` (200); precedent `account-data.md:74` cited; `listedjob.go` decided (not used); pre-open budget 30 minutes with 08:30 only on a late start; the 4 MB gRPC limit and the largest answer stated; tie order; 3.5 expiries marked an assumption; `BuyPrice` is on `jobs.WatchEntry`.

## Doubts (least sure)

1. Whether a one-key `Run` with a sort across 45 names stays under 600 lines in `list_run.go`.
2. `ListChains` time (45 calls) and the write time of the first pass are unmeasured.
3. Whether `ruff format` keeps `transport.py` under 800 (the entry fails if not, and then goes to the owner).
4. The unset-right case has one sample (SPY).
