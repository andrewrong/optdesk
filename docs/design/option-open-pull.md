# Option open pull: the contracts are decided before the open, quoted live at 09:45 ET, and a button on the option pages pulls the newest quotes of the symbols on screen

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-10
Revision: 1

Source: owner decision 2026-11-10 (dispatcher window): the owner sells options and trades at the US open, so option quotes must be fresh at the open, not only after the close. Wanted:
(1) decide the contract list in advance, then pull at the open (learned from option-trader); (2) on the option pages a refresh button that pulls the newest quotes on click, not streaming.
The after-close `option_eod` archive (history, IV rank) stays as it is. Owner addition the same day: a planned contract that does not exist is remembered with a validity period and not asked again until it ends.
Learned from option-trader (read, code and docs only): `scheduler/snapshot_jobs.py`, `snapshots/plan.py`, `params/plan_v4.toml`, `snapshots/listed_store.py`, `snapshots/listed_refresh.py`, `sources/ibkr/quote_port.py`, `reports/option_route.py`, ADR 0003 and 0004, issues #240 #241 #757 #775 #776 #793 #1811.
Read on origin/main at 08a89bc7: `AGENTS.md`, `docs/design/modules.md`, `docs/design/option-jobs.md`, `docs/design/option-page-builders.md`, `docs/design/scheduler.md`, `contracts/datasets/obs.option_eod.r1.json`, `contracts/datasets/obs.quote_snapshot.r1.json`, `contracts/proto/ibkr/v1/{marketdata,quote_types}.proto`, `pyside/src/optdesk/ibkr/{spec,ibgateway,gateways,pacing,server}.py`, `internal/jobs/optioneod/*`, `internal/jobs/optioncandidates/input.go`, `cmd/ot/serve.go`. Also read: the open design `docs/design/option-page-names.md` on `origin/design/option-page-names` (PR 1333); this design does not edit its files and its issues are ordered after it (Decision 11).

Goal: at 09:45 ET on a US session day the store holds a quote (bid, ask, size, model IV and Greeks, the data type and the quote time) of every contract the owner could sell or already holds, for the stocks of the watch list. On `/options` and `/options/stock/<symbol>` a button pulls the newest quotes of the contracts on screen, within 15 seconds, and the page shows the time and type of every quote it prints. The 16:15 `option_eod` job and its rows are not touched.

## What main holds today

- `option_eod` runs once, after the close (`job.toml`: anchor `session_close`, offset 0). Its quotes come from `QUOTE_KIND_OPTION_EOD` (ticks "100,101", release on bid, ask, close and the model values, 20 s, 5 s open-interest grace) over the pyside gateway, which sets one market-data type per connection and defaults it to 2, frozen (`gateways.py:20`, `ibgateway.py:216`, since #1143). That is right for the archive and wrong at the open.
- Plan today: `max_expiries` 3, band 0.10, two strikes below and two at-or-above (#1302), held calls through `NewWithHeldSymbols` (#1310); a watch-list variant has `min_dte` 21, `max_dte` 45, `max_strikes` 24. Contracts are resolved by `ResolveContract` (one con_id each, `Snapshot` demands a con_id) and cached in `master.instrument`.
- `option_eod` rows have a `capture_id` that is a random uuid, and `optioncandidates/input.go:157-163` takes "the newest capture of the day" per instrument by comparing `CaptureID` strings. A second capture of the day in that table would make that choice random. This is why the open rows do not go into `option_eod` (Decision 6).
- `obs.quote_snapshot` has a `slot` and a `market_data_type` but no IV and no Greeks; it is no fit for a seller's read.
- `ot serve` is read-only: the role `OT_PG_DSN_PAGES`, no broker, one POST route (`--owner-edit`, loopback only, `/owner-settings`). The pages `/options` and `/options/stock/<key>` are built from `store.Candidate` rows (computed after the close) and `option_eod` rows by `internal/pages/option_boards`.
- The runner has no control socket in code (`docs/contracts/runner.md` only names one), so a page cannot ask the scheduler process to run anything.

## Decisions

### Plan

1. Two jobs, in one new package `internal/jobs/optionopen`: `option_open_plan` (08:45 ET, `session_open` minus 45 minutes) and `option_open` (09:45 ET, `session_open` plus 15 minutes). The plan job resolves con_ids, which costs about 5 s per contract serialised (option-trader #776: 148 expiries overloaded the gateway); 300 resolves must not run inside the open slot. Its output is a stored plan; the pull job reads it. No plan rows for the day: the pull job ends `NoValue` "no plan" and the page shows a gap; it never plans on its own at 09:45.
2. Which stocks: the watch list and the held underlyings, the same two sources `optioneod` reads (the plan job takes the ports of `held.go` and `watch.go` through the store readers they use; it imports no other jobs package).
3. Which contracts of a stock, a pure function in a new compute package `internal/compute/optionplan` (compute has no I/O; same split as option-trader `plan.py`):
   - expiries: 21 to 45 calendar days to expiry (the window the watch-list variant already uses; it is also where a seller of short puts works), at most 3 of them (the same `max_expiries`).
   - strikes: puts from the spot down to `spot * exp(-k * sigma * sqrt(T))` with k = 1.5 and sigma the stored implied vol of the underlying (the reader `optioncandidates` uses); calls from the spot up to the same distance only for underlyings the owner holds (a seller of covered calls); in addition the two strikes nearest above the spot for puts (the #1302 rule). Without a stored sigma: a band of 25 percent of the spot (option-trader uses 30; a smaller band keeps the count down; it is a `job.toml` value). The spot is the last stored close; the open price is not known at 08:45 (Decision 12 says what happens when the open gaps).
   - every held option contract (put or call, any expiry) is always included, outside every ceiling.
   - ceilings: 40 contracts per stock, 1200 per run (about 45 stocks, owner 2026-11-10), counted without held contracts. Contracts over a ceiling are not dropped silently: the plan stores them with the reason `deferred` and the pull writes them as final status `deferred` (Decision 7). Order inside a stock: held first, then nearest to the spot.
   - only listed contracts: the chain comes from `ListChains` (reqSecDefOptParams); because its strikes are a union over expiries (option-trader #793: 8,170 contracts not listed), the plan job resolves each planned contract not yet in `master.instrument` (cache hit costs nothing) and applies Decision 5.
4. Plan rows go to a new dataset `plan.option_open_item` (Decision 6), written by the plan job before any pull request of the pull job. One row per planned contract: day, plan id, underlying id, instrument id, reason (`band`, `near_spot`, `held`, `deferred`), rank, planned spot and sigma. The plan job has a budget of 40 minutes (the lease of a run is 10 minutes and nothing renews it, so the key is one stock, not the whole plan), ends before the slot, and a stock not finished by then is reported as a gap of the job, not silently thinner.

### Listed check, remembered with a validity period (owner addition 2026-11-10)

5. What is stored, where, how long:
   - Table `master.option_not_listed` (store module, migration; Postgres next to `master.instrument`, not a file of the job). Columns: underlying id, expiry, strike, right, trading class, `checked_at`, `retry_at`, error code, error text. One row per contract that IBKR answered "none" for.
   - Validity: `retry_at = checked_at + 7 days`, plus a jitter drawn per row, uniform in minus 2 to plus 2 days, so the re-asks spread over days. These are option-trader's values after #793 (first 1 day, 8,000 contracts re-asked daily cost 23 minutes of gateway in a 30-minute job; owner decision there 2026-10-17). They are `job.toml` values. The plan job does not ask a contract before its `retry_at`.
   - The key of a mark is the full contract identity (underlying, currency, exchange, trading class, multiplier, expiry, strike text, right), as option-trader's `listed_absent`; a listed contract has no TTL (its con_id is fixed; `master.instrument` already holds it).
   - Early end of a mark, four triggers as option-trader (advisor 2026-11-10): (a) `retry_at` has passed; (b) the same strike is seen listed at another expiry or a later `ListChains` shows the strike now listed (the chain gained it after `checked_at`); (c) the stored spot of the stock moved 5 percent or more since `checked_at` (strikes may be listed near the new price): the marks of that stock are asked again. Adopted as they are: optdesk's shape (a daily plan of a few hundred contracts, one store) gives no reason to differ, and a seven-day mark with a one-day mark would only cost gateway time (#793: 23 of 30 minutes).
   - The refresh button and the pull never ask a not-listed contract and never write a mark: they only skip a contract that has a mark (option-trader's on-demand rule trusts a mark 1 day; here the plan already filtered).
6. Which answers count as "not listed" and which never do. A mark is written only for a final answer of IBKR about the contract itself: `ResolveContract` returns an empty list, or error 200 ("no security definition") for one fully specified contract with exactly one request in flight. Never marked, the contract is asked again at the next run: a timeout, a disconnect or session loss (10197, 1100 to 1102, "not connected"), a pacing or line error (101, 100, 162), a missing subscription (354, 10089 to 10091), a deadline or a stop of the run (two consecutive timeouts of the plan job's connection stop its resolves for the run, option-trader #757, and the rest are reported unsent, not marked), an answer that arrived after the run stopped, an ambiguous answer (error 200 with several results or a result that differs from the request: option-trader counts it `ambiguous`, not `not_listed`), and any unknown error. A test pins this table (issue 6). Sending a contract to the pull that the plan marked listed and that answers 200 at 09:45 gives status `not_listed` on that row only; the next plan job turns it into a mark when the answer was a clean 200 of one request (it reads those rows; it is the only writer of the table).

### Where rows go

7. A new dataset `obs.option_open` r1 (contracts, store migration), not `option_eod` with a slot column and not `quote_snapshot`. Reasons: the `option_eod` description says "frozen after the close" and the candidates job picks "the newest capture" by a random uuid (see above); the IV rank and the history read that table; adding a second capture per day changes both silently. `quote_snapshot` lacks IV and Greeks. The new table has the columns of `option_eod` that make sense at the open (instrument, underlying, generation, bid, ask, sizes, last, volume, underlying price and its time, the four IV columns, delta, gamma, theta, vega), without `open_interest` (stale intraday, the page joins the last `option_eod` value and prints its date), and adds: `slot` text (`0945`), `plan_id`, `market_data_type` (the type of the tick, Decision 8), `requested_at`, `sent_at`, `received_at`, `event_ts`, `connection_generation`, `status` and `error_code`. Primary key `(session_day, capture_id, slot, instrument_id)`; insert is `ON CONFLICT DO NOTHING`. Timescale hypertable on `session_day` like `option_eod`. The grant to `ot_ro_pages` is part of the migration.
8. Final status per item, exactly one row per planned item, written at the end of the run (a crash writes nothing and the runner retries the key): 1 `ok` (bid, ask and model IV), 2 `partial` (a part of them, kept), 3 `no_quote`, 4 `not_listed`, 5 `no_subscription`, 6 `deadline`, 7 `deferred` (over a ceiling or a line budget), 8 `error`, 9 `disconnected`. Statuses 6, 7, 8 and 9 are retried by nothing the same morning: a later pull is the owner's button. Order of `ok` rows: grouped by stock, then expiry, so that the spot of the stock sampled at the start and at the end of a group bounds the time skew (option-trader: a slow group or a spot move is flagged incoherent): the row carries `underlying_price` and `underlying_event_ts` of the group's end; a spot that moved 1 percent within a group sets a flag the page prints ("skew").

### Market data type and the request shape

9. Type live (1) at the open, and each row records the type that came with its own tick, not the type asked. How in pyside: `reqMarketDataType` is per connection in IB (`ibgateway.py:216` sets it once at connect), so changing it per call would change the type of a request running next to it. Decision: a second `[[gateway]]` entry in the pyside configuration (same host and port, another `client_id`, `market_data_type = 1`, name `live`); the new kind `QUOTE_KIND_OPTION_OPEN` is bound in `spec.py` to the gateway of that name (a new `KindSpec` field `gateway`), and the EOD kinds stay on the default frozen gateway. `gateways.py` already allows several entries with different client ids. The quote's `market_data_type` is already taken from the ib_async ticker (`_quote` in `ibgateway.py` reads `ticker.marketDataType`, falling back to the connection's type), so a connection whose subscription gives frozen or delayed data (354) is visible in the row; the plan is not failed by it. Open question the probe (issue 0) settles: IB serves live data under type 2 while the market is open and the last close only after it, so the existing frozen connection may already return ticks of type 1 at 09:45; if it does, the second gateway entry is dropped and `OPTION_OPEN` stays on the default gateway (one fewer client id and one fewer connection). Frozen turning live in Cboe extended hours (option-trader #1811) is why the tick's type is stored.
10. Request shape. One streaming `reqMktData` per contract, never `snapshot=True` (error 414 with generic ticks, option-trader). Ticks "100,101" as `OPTION_EOD`, release on bid, ask and the model values; no open-interest grace at the open. Pull job: item timeout 15 s, one retry on a timeout, key budget 8 minutes (the lease), whole job window 25 minutes. Line budget: the existing `LineBudget` (70 lines) and the in-flight cap `snapshots` (50, `OT_IBKR_SNAPSHOTS`) both apply, so at most 50 quotes run at once (`## Capacity`); error 101 shrinks the lines (existing, tested); at most 40 message starts a second, a cancel counting as a start (`pacing.py`). The line is cancelled in a `finally` and the ticker removed (ib_async keeps one ticker per conId). Reconnect on 10197: the live gateway reconnects and the key is retried by the runner; the rows written so far stay. No TWS or phone session may be logged in while the gateway is up (10197 kills the session): a note for the owner, not code.
11. Slot and early close: the schedule values are `anchor = "session_open"`, `offset = "15m"` (and `-45m` for the plan); the Japan slots are Decision 21, so the calendar moves both on an early-open day. The pull ends before 10:10 ET. Japan: left out (`## Left out`).
12. A gap at the open: the plan uses the last close, so a stock that gapped 8 percent opens with a band centred on yesterday. The pull records the open price in `underlying_price` and the page shows "outside the planned band" for a contract more than the band away. The button is the owner's way to a better set; it uses the stored plan only (Decision 14). A replan after the open is left out.

21. Japan (owner 2026-11-10: add). What is quoted: the Nikkei 225 monthly index options of OSE (class 225, the `option_eod_jp` manifest) and the JP stocks of the watch list (chain, class and multiplier as the JP section of the `optioneod` manifest names them). Calendar `XTKS`, zone `Asia/Tokyo`. Slots come from the calendar: plan job `option_open_plan_jp` at `session_open` minus 45 minutes (08:15 JST when the TSE opens at 09:00), pull job `option_open_jp` at `session_open` plus 15 minutes (09:15 JST). TSE hours: day session 09:00 to 11:30 and 12:30 to 15:30 (the close moved to 15:30 in November 2024; the calendar is the source, and the `option_eod_jp` slot at 15:50 is five minutes after the closing auction); 09:15 is inside the morning session for stocks and 30 minutes after the OSE day session opens (08:45) for the index options. A holiday or half day moves or removes the slots with the calendar. Same datasets (`obs.option_open` with currency JPY and slot `0915`), same statuses, same type-1 request; strike step and multiplier come from the stored terms, price increment 1 yen. Plan: the same function, DTE 21 to 45 (the archive section keeps its 14 to 75 and 12 strikes), at most 40 contracts per name and 60 for the index, puts only unless held. The JP count is small (the list is mostly US), one key of at most 120 contracts, 1 to 2 minutes; US and JP pulls never overlap (09:45 ET is 22:45 or 23:45 JST). The refresh button works for a JP symbol only inside the TSE session, by the same session check. Option-trader's JP details have not arrived and the owner said to ask nothing more: the values above are `job_jp.toml` values and change without a design.

### The refresh button

13. Path: the web button posts to `ot serve`, which asks pyside, directly, for the plan contracts of the symbols on the page and answers with the new rows; no database write. `POST /options/refresh` with a body `{"symbols": ["AAPL", ...]}`; the page JSON then is fetched again by the web and the builder reads the live rows from an overlay reader in `cmd/ot` that puts the in-memory rows over the stored `obs.option_open` rows. Rows of a refresh are not stored: a refresh is not an observation for the history, the 09:45 slot and the close are. (Storing them would need a writer role in `ot serve`; Decision 16.)
14. Scope of one call: the contracts of the plan of today for those symbols, at most 8 symbols and at most 40 contracts in all (held first, then by the rank of the page: the card contract of each symbol, then the chain rows near the spot). `/options` refreshes the symbols of its cards; `/options/stock/<key>` refreshes that stock. No plan for the day (the plan job has not run or failed): the answer is the notice "no plan today", and nothing is asked. Type 1 over the `live` gateway; item timeout 10 s, no retry, whole call deadline 15 s; rows that did not complete keep their stored values and the notice names how many ("27 of 31 quotes fresh, 4 from 09:45").
15. Freshness rule (option-trader: reuse under 60 s): a symbol whose newest in-memory refresh is younger than 60 s is not asked again; the answer says "reused, 41 s old". A call while another call of the same symbol runs waits for it (3 s), then reuses its rows. Outside the regular session of the clock (not between open and close of the calendar, early close included) the endpoint asks nothing and answers "market closed; stored quotes of <time>". No lock and no refusal between the button and the scheduled pull: they share the one line budget and the one in-flight cap of the gateway in pyside, and the clients partition it by request size. The pull sends its contracts in chunks of 30 (one chunk in flight), a refresh call in chunks of 20 (one in flight); 30 + 20 = 50 is the in-flight cap and is under the 70 lines, so neither starves the other (`LineBudget` wakes all waiters with no order, so the design does not rely on a queue). Two refresh calls never overlap (Decision 17).
16. Failure and fallback: pyside not reachable, the gateway not connected, or the deadline: the answer is HTTP 200 with the notice and the stored rows (never an error page, and never a cached "not listed"). A refresh row of status other than `ok` or `partial` is dropped, not shown.
17. The permission this adds, said exactly. Today `ot serve` has the pages role of the database and no network to the broker; the only thing it does besides reading is `POST /owner-settings` under `--owner-edit` on a loopback address. This design adds: `ot serve` (under a new flag `--live-quotes`, off by default, loopback address only, like `--owner-edit`) holds a `broker.Broker` client to the pyside socket and may call `Snapshot(QUOTE_KIND_OPTION_OPEN)` for contracts of today's plan. It cannot place an order (the broker port has no such method), cannot write the database (no new DSN), cannot read accounts, and the contracts it may ask are only those of the plan table. What changes is who may act on the gateway: a browser tab on the owner's machine, through a POST, can make the gateway spend lines and a 15 s of its pacing; an open page left on a refresh loop could starve the 09:45 job of lines. Priority of the 09:45 job: a call of the button that starts inside the pull job's window (09:44 to 10:10 ET) asks at most 10 contracts and in one chunk of 10, so the job's chunk of 30 always finds its lines; outside the window the chunk is 20. Limits: one call at a time per process, 60 s reuse, 40 contracts, and a count of at most 30 calls an hour. **Second opinion: a change of who may act; the owner waived the separate review, the advisor answers it (2026-11-10) and the owner gives the merge word.** Alternative that avoids it, not chosen: a control socket of the scheduler process; it does not exist in code, would give the web a way to start jobs, and so adds more power (`## Left out`).
18. Pages show: next to every bid, ask, IV and Greek of a contract the quote time (exchange-local `HH:MM:SS`, "09:45:12"), the data type as a word (live, frozen, delayed) and its source ("open pull", "refresh", "close archive"); older than 15 minutes grey, a type other than live in red words; a contract with no open or refresh row shows its `option_eod` value with the label "close archive, 11 Nov"; the open interest shows its own date. The builder only copies these fields; the web renders them. No yield or score is recomputed by the web; when the builder shows a yield at the new bid it calls the compute function of the candidates (no new formula; if that function is not exported, issue 9 names it). The schema change is Decision 19.
19. Page schema: `option_page` and `stock_option_page` gain, additively, per row a `quote` object (`as_of`, `data_type`, `source`, `skew`, `outside_band`) and per page a `refresh` object (`enabled`, `symbols`, `notice`, `fresh_count`, `total_count`); a new schema version (v2) for each; the web types are generated. This is a change of `contracts/schemas`, so `Contracts:` says so, and it comes first (issue 3).
20. The web button: shadcn `Button` with the lucide `RefreshCw` icon in the page header, disabled while a call runs and when `refresh.enabled` is false (the page says why in words); the response notice sits under the header. The web computes nothing and keeps no timer.

## Capacity

Can pyside carry this? From the code (`pyside/src/optdesk/ibkr/`), not from a guess:

- How it serves quotes today. `transport.py` `Snapshot` runs one `IbGateway.quotes` task per request and streams each row when it completes. `quotes` starts one task per contract at once; each takes a snapshot lease (`DEFAULT_SNAPSHOTS` 50, `OT_IBKR_SNAPSHOTS`), then a line lease (`DEFAULT_LINES` 70), then one message start from the `MessagePacer` (40 starts a second, a cancel counts as a start). So quotes are concurrent, at most 50 in flight, not one at a time. Each item is one streaming `reqMktData`, released when its fields are complete or at `timeout_s` 20 (the 5 s open-interest grace applies to `OPTION_EOD` only). One IB connection per `[[gateway]]` entry, one client id each (default 33); error 101 shrinks the line limit to the lines in use less one. The in-flight cap (50) is the effective limit, below the 70 lines; the account's own limit is 100 lines (option-trader), and an open TWS window eats from those 100, which is why 70 is the budget and 50 is what runs.
- What is not there. The session type is set once per connection (`ibgateway.py` `open_session`); a kind has no type of its own (Decision 9). `LineBudget` wakes all waiters with no order. `ResolveContract` goes through the `DetailsLane`: 2 in flight, rising to 5 when ten answers average under a second, halving over 3 s, and one request that runs 20 s drops it to 1 and pauses 30 s. There is no `Snapshot` batch size limit beyond the leases.
- The size of the open plan, an estimate to be checked by issue 0 and the first plan run. Per stock: with sigma 0.35 and 30 days, `k * sigma * sqrt(T)` is 0.15, so puts reach 15 percent below the spot: with a strike step of 2.5 on a stock of 150 that is about 9 strikes plus 2 above the spot, about 11 per expiry; 21 to 45 days holds 2 to 3 expiries (weeklies) or 1 (monthly only); say 25 to 35 contracts. The owner's list is about 45 stocks (2026-11-10), so 1,100 to 1,600 contracts before the not-listed cache; the per-stock ceiling is 40 and the per-run ceiling 1200, and the plan lists what is deferred (about 1,200 quoted is the working figure; a stock with deep strikes is cut at 40 nearest the spot, held contracts excluded). Option-trader's first run, 44,188 items, is the warning of how this grows.
- Wall time of the pull at that size. A quote holds its line from request to completion, typically 1 to 3 s at the open and up to 15 s when nothing comes. The pull keeps 30 in flight (Decision 15), at about 2 s each that is 15 items a second, under the 20 a second the pacer allows (two starts per item). So 1,200 items take about 80 s when all complete and about 3 to 4 minutes with a tenth waiting the 15 s and one retry. The key shape follows from this: a key of one stock (25 to 40 items) never fills 30 in flight, and 45 keys in sequence would cost 45 times the slowest item (3 to 30 s each: 2 to 22 minutes, too near the 25 minutes); so the pull key is a batch of stocks of up to 120 contracts (about 4 stocks, 12 keys), each `Run` sending chunks of 30 one after another, each key inside the 10-minute lease (typically 10 to 20 s). Total 2 to 5 minutes; the 50-in-flight cap does still finish inside the slot budget of 25 minutes, with a margin of about 5 times. The refresh button's share: 20 in flight (10 inside the window 09:44 to 10:10); 30 + 20 = 50, so the button can never take the job's lines. A refresh of 40 contracts in chunks of 20 takes two waves, 2 to 6 s typically, and may end partial inside 15 s (the notice says how many).
- Cost of the listed checks. `ResolveContract` is the slow call: option-trader measures about 5 s per (underlying, expiry), serialised, and its first run on 148 expiries overloaded the gateway (#776) and once stopped it answering (#757). Here the lane starts at 2 in flight, so a first day with no cache needs 500 to 700 resolves at 1 to 5 s over 2 to 5 lanes, about 5 to 30 minutes, possibly more; a day with the cache (`master.instrument` holds every con_id, `master.option_not_listed` every mark) resolves only the new strikes, a few dozen. The first day with about 1,300 uncached contracts needs 1,300 resolves at 1 to 5 s over 2 to 5 lanes, 5 to 55 minutes. Therefore the scheduled plan job caps new resolves per run at 300 (a `job.toml` value; a hand run by the window takes a larger value, 1500, since the plan key is one stock and each key is a minute or two inside the lease) and stops its resolves after two consecutive timeouts; what is left is `deferred` and reported. For the first day the owner's window runs the plan job by hand the evening before, and again if some stocks were deferred (`ot run-once`, allowed); after that only new strikes and expiries need resolves, a few dozen, and the nightly `option_eod` already resolves its contracts into the same cache.
- What must change in pyside: (1) a kind `OPTION_OPEN` with its own spec; (2) if issue 0 shows the frozen-type connection does not return live ticks at the open, a gateway named `live` of type 1 and a `gateway` field on the spec (Decision 9); (3) nothing in pacing: the concurrent, line-budgeted quote path and the 40-per-second window exist and are tested; the design only fixes how the two clients split the in-flight cap (Decision 15); (4) the configuration of the deployment is the owner's. Anything under 500 contracts and 40 messages a second is within what the code already does; the unknowns are the real latency at the open and the resolve time, which is what issue 0 prints.

## Interface

- Proto (owner merges): `QuoteKind` gets `QUOTE_KIND_OPTION_OPEN = 4`, with the specification text of Decision 10 in the comment of the enum (it is where the other kinds are written down). Nothing else in `contracts/proto` changes: `Quote` already has `market_data_type`, `event_ts` and the sizes. Generated Go and Python stubs come with it.
- Datasets: `obs.option_open.r1.json`, `plan.option_open_item.r1.json`, `master.option_not_listed.r1.json`; generated Go structs in `internal/contracts`.
- Store (declarations in `internal/store/api.go`): `OptionOpenWriter`, `OptionOpenReader` (rows of a day, newest of each instrument), `OptionOpenPlanStore` (write the plan, read it by day and symbol), `OptionNotListedStore` (marks: add, list not yet due, remove by key, remove by underlying).
- Jobs: `optionopen.NewPlan(...)` and `optionopen.NewPull(...)` and `optionopen.Refresh(ctx, broker, items, opts) (rows, notice)`, taking `broker.Broker`, the store ports and `clock.Clock`; `Refresh` is the same pull core the job calls, with the deadline and the retry count of Decision 14.
- Pages: a reader port `OptionQuoteReader` (the `OptionOpenReader` shape) given to the two builders.

## Issues

Order: 0 (measures, may run before anything else), 1, 0b, 2 (interfaces), then 3, 4, 5, 6 in parallel, then 7, 8, 9, 10, 11.

### 0 option-open-probe

Module: pyside. Size: S. Measures only; changes no behaviour.
Files: `pyside/tests/live/test_option_open_probe.py`
Design: docs/design/option-open-pull.md#Capacity
Order: first; the owner or the window runs it during a session with `OT_LIVE_PYSIDE=1` (skipped otherwise, never in CI, read-only: `ListChains`, `ResolveContract` and `Snapshot` only, no order and no other write rpc).
Goal: print the numbers `## Capacity` marks as unknown: the data type of the ticks the existing frozen-type gateway returns during the session, the seconds each of 40 puts of one stock takes to complete (median, 90th percentile, share that times out), the throughput of 50 at once, and the seconds per resolve of 20 uncached contracts of one expiry.
Acceptance:
1. `grep -c "OT_LIVE_PYSIDE" pyside/tests/live/test_option_open_probe.py` prints at least 1.
2. `grep -c "ResolveContract\|ListChains\|Snapshot" pyside/tests/live/test_option_open_probe.py` prints at least 3.
3. `grep -ci "order\|execution\|place" pyside/tests/live/test_option_open_probe.py` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vc "^pyside/tests/live/"` prints 0.
Checked by CI: ci-python (the file is collected and skipped; `ruff check`, `mypy --strict`).

### 0b broker-kind-option-open

Module: broker. Size: S. Interface issue.
Files: `internal/broker/**`, `docs/contracts/broker.md`
Design: docs/design/option-open-pull.md#Interface
Order: after 1 (the generated proto), before 6 and 7. Found by the adversarial review: without it the jobs cannot send the new kind or read the error of a resolve.
Goal: `KindOptionOpen` with its mapping to `QUOTE_KIND_OPTION_OPEN`; `Resolved` carries `ErrorCode` and `ErrorText` (the proto has them); the resolve in-flight limit of a client can be set to 1, and the plan job sets it (Decision 6 counts a clean 200 only for one request in flight; the default is 5). The client dials a socket path given by the caller, so the `live` gateway socket `ibkr-live.sock` is a value of the wiring (Decision 9).
Acceptance:
1. `grep -c "KindOptionOpen" internal/broker/api.go` prints at least 1.
2. `grep -c "ErrorCode" internal/broker/api.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vc "^internal/broker/\|^docs/contracts/broker.md"` prints 0.
Checked by CI: ci-go; `go test ./internal/broker/... -race -count=1` (contract test from both sides).

### 1 quote-kind-option-open

Module: contracts. Size: S. Interface issue.
Files: `contracts/proto/ibkr/v1/quote_types.proto`, `internal/contracts/genpb/**`, `pyside/src/optdesk/contracts/**`
Design: docs/design/option-open-pull.md#Interface
Order: first. The pull request is the owner's merge (`contracts/proto`).
Goal: `QUOTE_KIND_OPTION_OPEN = 4` exists with the comment of Decision 10, and the generated output.
Acceptance:
1. `grep -c "QUOTE_KIND_OPTION_OPEN = 4" contracts/proto/ibkr/v1/quote_types.proto` prints 1.
2. `grep -rc "QUOTE_KIND_OPTION_OPEN" internal/contracts/genpb | grep -v ":0" | wc -l` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) -- contracts/proto | wc -l` prints 1.
Checked by CI: ci-go, ci-python, the buf lint and generated-drift check; `Contracts: changed (proto)`.

### 2 option-open-datasets

Module: contracts. Size: M. Interface issue.
Files: `contracts/datasets/obs.option_open.r1.json`, `contracts/datasets/plan.option_open_item.r1.json`, `contracts/datasets/master.option_not_listed.r1.json`, `internal/contracts/**` (generated), `docs/contracts/datasets/**`
Design: docs/design/option-open-pull.md#Decisions
Order: with 1.
Goal: the three dataset files of Decisions 4, 5 and 7 and their generated types.
Acceptance:
1. `grep -c '"x-primary-key"' contracts/datasets/obs.option_open.r1.json` prints 1 and `grep -c '"slot"' contracts/datasets/obs.option_open.r1.json` prints at least 1.
2. `grep -c '"open_interest"' contracts/datasets/obs.option_open.r1.json` prints 0.
3. `grep -c '"requested_at"\|"sent_at"\|"received_at"\|"connection_generation"' contracts/datasets/obs.option_open.r1.json` prints at least 4.
4. `grep -c '"retry_at"' contracts/datasets/master.option_not_listed.r1.json` prints at least 1.
5. `git diff --name-only $(git merge-base HEAD origin/main) -- contracts/datasets/obs.option_eod.r1.json contracts/datasets/obs.quote_snapshot.r1.json | wc -l` prints 0.
Checked by CI: ci-go (generated drift, dataset schema checks); `Contracts: changed (datasets)`.

### 3 page-schema-quote-refresh

Module: contracts. Size: M. Interface issue.
Files: `contracts/schemas/page.option_page.v2.json`, `contracts/schemas/page.stock_option_page.v2.json`, `web/src/generated/**`, `docs/contracts/page-schemas.md`
Design: docs/design/option-open-pull.md#Decisions
Order: after the merge of the issues of `docs/design/option-page-names.md` (PR 1333), so that the two do not edit the same generated files at once.
Goal: Decision 19, additive over v1, with the v1 files unchanged.
Acceptance:
1. `grep -c '"quote"' contracts/schemas/page.option_page.v2.json contracts/schemas/page.stock_option_page.v2.json` prints at least 1 for each file.
2. `grep -c '"refresh"' contracts/schemas/page.option_page.v2.json` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) -- contracts/schemas/page.option_page.v1.json contracts/schemas/page.stock_option_page.v1.json | wc -l` prints 0.
Checked by CI: ci-go, ci-web (generated types, tsc); `Contracts: changed (schemas)`.

### 4 option-open-store

Module: store. Size: L.
Files: `migrations/0029_option_open.sql`, `migrations/0030_option_open_plan.sql`, `migrations/0031_option_not_listed.sql`, `internal/store/api.go`, `internal/store/optionopen/**`, `internal/store/optionopenpg/**`, `internal/store/setup/**`
Design: docs/design/option-open-pull.md#Interface
Order: after 2. An interface edit of `internal/store/api.go` is the interface part of this issue.
Goal: the writer, readers and plan and mark stores of `## Interface`, the migrations with the grant to `ot_ro_pages`, a role `ot_w_option_open` for the writes of the two jobs.
Acceptance:
1. `grep -c "CREATE TABLE obs.option_open\b" migrations/0029_option_open.sql` prints 1 and `grep -c "ot_ro_pages" migrations/0029_option_open.sql` prints at least 1.
2. `grep -c "retry_at" migrations/0031_option_not_listed.sql` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) -- migrations | grep -vc "0029\|0030\|0031"` prints 0.
4. `grep -c "OptionOpenWriter\|OptionOpenReader\|OptionNotListedStore" internal/store/api.go` prints at least 3.
Checked by CI: ci-go; `go test ./internal/store/... -race -count=1` and, under the `pg` tag in GitHub CI, the migration test; `Contracts: unchanged`.

### 5 option-plan-compute

Module: compute. Size: M.
Files: `internal/compute/optionplan/**`
Design: docs/design/option-open-pull.md#Decisions
Order: independent of 4.
Goal: the pure plan builder of Decision 3: expiries 21 to 45 days at most 3, strikes by k, sigma and the fallback band, held always, ceilings with the reason `deferred`, ordering. Own implementation, because no library knows the rule (it is a handful of lines; it sits behind a function so a library can replace it).
Acceptance:
1. `grep -rc "time.Now\|net/http\|database/sql" internal/compute/optionplan | grep -v ":0" | wc -l` prints 0.
2. `ls internal/compute/optionplan | grep -c "_test.go"` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vc "^internal/compute/optionplan/"` prints 0.
Checked by CI: ci-go; `go test ./internal/compute/optionplan -race -count=1` (golden cases: with sigma, without sigma, held outside the band, over each ceiling, a put and a call, the expiry at 21 and 45 days).

### 6 option-open-plan-job

Module: jobs. Size: L.
Files: `internal/jobs/optionopen/plan.go`, `internal/jobs/optionopen/listed.go`, `internal/jobs/optionopen/plan_test.go`, `internal/jobs/optionopen/listed_test.go`, `internal/jobs/optionopen/job_plan.toml`, `internal/jobs/optionopen/job_plan_jp.toml`, `internal/jobs/optionopen/doc.go`
Design: docs/design/option-open-pull.md#Decisions
Order: after 2, 4 and 5.
Goal: the jobs `option_open_plan` and `option_open_plan_jp` (Decision 21) of Decisions 1 to 6: key one stock, budget, listed check with the validity period and the answer table of Decision 6.
Acceptance:
1. `grep -c 'anchor = "session_open"' internal/jobs/optionopen/job_plan.toml` prints 1 and `grep -c 'offset = "-45m"' internal/jobs/optionopen/job_plan.toml` prints 1.
2. `grep -c "retry_days\|jitter_days" internal/jobs/optionopen/job_plan.toml` prints at least 2.
2a. `grep -c 'calendar = "XTKS"' internal/jobs/optionopen/job_plan_jp.toml` prints at least 1 and `grep -c 'offset = "-45m"' internal/jobs/optionopen/job_plan_jp.toml` prints 1.
3. `grep -c "func Test" internal/jobs/optionopen/listed_test.go` prints at least 6 (one per row group of the table of Decision 6: empty list, clean 200, timeout, disconnect, ambiguous 200, 354).
4. `grep -rc "internal/jobs/" internal/jobs/optionopen/*.go | grep -v ":0" | wc -l` prints 0.
Checked by CI: ci-go; `go test ./internal/jobs/optionopen -race -count=1` with a fake clock and a fake broker (no sleep; a timeout never writes a mark; a clean 200 writes `retry_at` inside seven plus or minus two days; a spot move of 5 percent asks again).

### 7 option-open-pull-job

Module: jobs. Size: L.
Files: `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/core.go`, `internal/jobs/optionopen/pull_test.go`, `internal/jobs/optionopen/core_test.go`, `internal/jobs/optionopen/job_pull.toml`, `internal/jobs/optionopen/job_pull_jp.toml`
Design: docs/design/option-open-pull.md#Decisions
Order: after 1, 4 and 6.
Goal: the jobs `option_open` and `option_open_jp` (a key is a batch of stocks of at most 120 contracts, `## Capacity`) and the shared `core` (also behind `Refresh`) of Decisions 7, 8, 10 and 12: grouped by stock and expiry, one final status per item, the skew flag, the type from the tick, the written rows.
Acceptance:
1. `grep -c 'offset = "15m"' internal/jobs/optionopen/job_pull.toml` and the same for `job_pull_jp.toml` print 1; `grep -c 'max_per_key = 120' internal/jobs/optionopen/job_pull.toml` prints 1.
2. `grep -c "QUOTE_KIND_OPTION_OPEN" internal/jobs/optionopen/core.go` prints at least 1 and `grep -c "QUOTE_KIND_OPTION_EOD" internal/jobs/optionopen/core.go` prints 0.
3. `grep -c "func Refresh" internal/jobs/optionopen/core.go` prints 1.
4. `grep -c "time.Sleep" internal/jobs/optionopen/*.go` prints 0 in every file.
Checked by CI: ci-go; `go test ./internal/jobs/optionopen -race -count=1` (every planned item has exactly one row; deferred rows; the type of the row is the type of the answer; a timeout retried once then `deadline`).

### 8 pyside-option-open-kind

Module: pyside. Size: M.
Files: `pyside/src/optdesk/ibkr/spec.py`, `pyside/src/optdesk/ibkr/server.py`, `pyside/src/optdesk/ibkr/gateways.py`, `pyside/src/optdesk/ibkr/ibgateway.py`, `pyside/tests/ibkr/test_option_open.py`
Design: docs/design/option-open-pull.md#Decisions
Order: after 1 and after the numbers of issue 0 (if the frozen-type gateway already returns live ticks, the `gateway` field and the second entry are dropped and this issue is only the kind). Not an interface issue if `api.py` is unchanged; if it must change, say so in the pull request.
Goal: the `OPTION_OPEN` kind of Decision 9 and 10 bound to the gateway named `live`; the row's `market_data_type` taken from the ticker; the kind's timeout 15 s and no open-interest grace.
Acceptance:
1. `grep -c "OPTION_OPEN" pyside/src/optdesk/ibkr/spec.py` prints at least 2.
2. `grep -c "gateway" pyside/src/optdesk/ibkr/spec.py` prints at least 2 (the new `KindSpec` field and the OPTION_OPEN row; the per-row type is already read from the ticker, `ibgateway.py` `_quote`, so a grep of it would check nothing new), or, if issue 0 drops the second gateway, `grep -c "OPTION_OPEN" pyside/tests/ibkr/test_option_open.py` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) -- pyside/src/optdesk/ibkr/api.py | wc -l` prints 0.
4. `git diff $(git merge-base HEAD origin/main) -- pyside/src/optdesk/ibkr/gateways.py | grep -c "^[-+]DEFAULT_MARKET_DATA_TYPE"` prints 0 (the default of the archive stays frozen).
Checked by CI: ci-python (`uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`); the live tests under `pyside/tests/live/` stay skipped. The deployed configuration (the second `[[gateway]]` entry) is the owner's, `## Owner questions`.

### 9 option-boards-quote-fields

Module: pages. Size: M.
Files: `internal/pages/option_boards/quote.go`, `internal/pages/option_boards/quote_test.go`, `internal/pages/option_boards/stock_page.go`, `internal/pages/option_boards/option_page.go`
Design: docs/design/option-open-pull.md#Decisions
Order: after 3 and 4 and after the issues of `option-page-names` have merged (they edit this package; if a file collides the pull request says so and waits).
Goal: Decisions 18 and 19: the builders take the `OptionQuoteReader`, fill `quote` and `refresh` per row and page, label stored close values "close archive", copy and never compute.
Acceptance:
1. `grep -c "OptionQuoteReader" internal/pages/option_boards/quote.go` prints at least 1.
2. `grep -rc "net/http\|database/sql\|pgx" internal/pages/option_boards | grep -v ":0" | wc -l` prints 0.
3. `grep -c "close archive" internal/pages/option_boards/quote.go` prints at least 1.
Checked by CI: ci-go; `go test ./internal/pages/option_boards -race -count=1` (golden pages with open rows, with a refresh overlay, with none; the schema validates).

### 10 ot-live-quotes

Module: ot. Size: L.
Files: `cmd/ot/live_quotes.go`, `cmd/ot/live_quotes_test.go`, `cmd/ot/serve.go`, `cmd/ot/wire_optionopen.go`, `cmd/ot/wire_optionopen_test.go`, `docs/contracts/ot.md`
Design: docs/design/option-open-pull.md#Decisions
Order: after 7 and 9. Second opinion: waived by the owner — the advisor answers it (owner 2026-11-10); the owner gives the merge word.
Goal: register the two jobs in the scheduler wiring; under `--live-quotes` (loopback only) `POST /options/refresh` with the 60 s reuse, the 40-contract and 8-symbol caps, the 15 s deadline, the session check, the hourly cap, the notice and the fallback; the overlay reader.
Acceptance:
1. `grep -c "live-quotes" cmd/ot/serve.go` prints at least 1.
2. `grep -c "loopback" cmd/ot/live_quotes.go` prints at least 1.
3. `grep -c "Place\|Order\|Execution" cmd/ot/live_quotes.go` prints 0.
4. `grep -c "OT_PG_DSN" cmd/ot/live_quotes.go` prints 0 (no new database line for serve).
5. `git diff --name-only $(git merge-base HEAD origin/main) -- cmd/ot/wire.go cmd/ot/site.go cmd/ot/pagereg.go | wc -l` prints 0.
Checked by CI: ci-go; `go test ./cmd/ot/... -race -count=1` (reuse under 60 s, a second call waits, market closed asks nothing, no plan asks nothing, a deadline falls back to stored rows with the notice, a call without the flag is 404).

### 11 web-refresh-button

Module: web. Size: M.
Files: `web/src/components/option-refresh.tsx`, `web/src/components/option-quote-cell.tsx`, `web/src/views/option-page.tsx`, `web/src/views/stock-option-page.tsx`, `web/test/option-refresh.test.tsx`
Design: docs/design/option-open-pull.md#Decisions
Order: after 3 (the types) and 10.
Goal: Decision 20 and the time-and-type text of Decision 18, from the JSON only.
Acceptance:
1. `grep -c "RefreshCw" web/src/components/option-refresh.tsx` prints at least 1.
2. `grep -c "setInterval\|setTimeout" web/src/components/option-refresh.tsx` prints 0.
3. `git diff --name-only $(git merge-base HEAD origin/main) -- package.json pnpm-lock.yaml | wc -l` prints 0 (no new dependency; `Button` and the icon exist, #1336).
Checked by CI: ci-web (`tsc`, `eslint`, `vitest`, the build); `Contracts: unchanged`.

## Checked by CI

Every test run named above runs in CI, not in acceptance. In addition: the test kit's fake clock in every job test (a test never sleeps); `t.TempDir()` and `t.Parallel()`; a Go test that needs PostgreSQL carries the tag `pg`. No test connects to a live broker. A live read-only check of the open kind belongs under `pyside/tests/live/` (skipped unless `OT_LIVE_PYSIDE=1`) and calls no write rpc.

## Left out

- A stock that is neither on the watch list nor held gets no open quote (the refresh button asks only the plan's contracts).
- The simpler way the review named (pull at 09:45 the contracts the 16:15 `option_eod` run resolved, no plan job): not chosen because it never quotes a new expiry or a new strike band before the first close that saw it, and the owner sells at the open; it stays the fallback when issue 6 slips: issue 7 reads `plan.option_open_item`, and a stand-in plan of the last `option_eod` instruments needs only a different writer.

- Streaming or an automatic refresh; the page has a button only, no timer.
- A replan after the open (a gap of the stock), and a refresh of a symbol with no plan (the resolves cost about 5 s each and cannot be done inside 15 s).
- Storing the rows of the refresh (needs a writer role in `ot serve`) and a history of intraday quotes beyond 09:45.
- A control socket of the scheduler for the page (more power than Decision 17 grants; not in code).
- Candidates (yield, rank) recomputed from open rows: `store.Candidate` rows stay the after-close result; the page shows the open quote next to them. A later design may feed open rows to the candidates job once the "newest capture" rule is fixed.
- Index options and spreads; the 15:10 pre-close pull (the owner dropped it 2026-11-09); the deploy of the second pyside gateway entry and the launchd slots (the owner's, `deploy/**`).
- Tuning of k, the band, ceilings and the validity period: values of `job.toml`, changed without a design.

## Owner decisions

Settled by the owner in the dispatcher window, 2026-11-10 (they replace the questions of revision 1):

- A, live quotes: yes, the account has live OPRA US option quotes. The open pull asks type 1.
- C, Japan: add the Japan open pull too (Decision 21).
- D, connection: a second client id on the same IB Gateway (Decision 9), unless issue 0 shows the frozen-type connection already returns live ticks at the open, in which case no second entry is made.
- F, validity of a not-listed mark: seven days plus or minus 2 (Decision 5).
- G, size: about 45 stocks on the list; ceilings, key shape and wall time are redone for it (Decisions 3 and 4, `## Capacity`).
- E, second opinion of Decision 17: settled by rule. Issue 10 carries `Second opinion: waived by the owner — the advisor answers it`; the owner gives the merge word.

## Owner questions

B. Calls: which names get calls and which only puts? Open: the owner says option-trader has a classification; the dispatcher has asked its advisor and will forward the answer. Until then Decision 3 plans calls only for held stocks. The plan function takes the class as an input (`calls: bool` per stock), so the answer changes only the plan job's reader, not an interface.

## Adversarial

A fresh read-only agent read this file and both repos; its answer is `adversarial-option-open-pull.md`. Point by point:

1. Gap (no broker issue, socket routing): adopted. Issue 0b is added (`KindOptionOpen`, `ErrorCode` and `ErrorText` on `Resolved`, a resolve in-flight limit of 1); the Go side dials a socket path from the wiring, `ibkr-live.sock` if issue 0 shows a second gateway is needed, `ibkr.sock` if not (Decision 9). Stocks off the list: kept in `## Left out`.
2. Unnamed risk, the plan job: adopted in part. Its key is one stock (as `option_eod`'s key is one underlying), the 300-resolve cap, the two-timeout stop and the hand-run the evening before (`## Capacity`) stay; the thin plan is a reported gap, not silent. Priority of the 09:45 job over the button: adopted (Decision 17, a 10-contract call inside the window).
3. Simpler way (reuse the 16:15 instruments, no plan job): not adopted as the main path, for the reason in `## Left out`; kept as the fallback.
4. Lock-in (Decision 17 ends "serve is read-only"): not removable, it is the owner's wish; named, flagged for the second opinion, behind a flag that is off by default, with no database line, no order method and a contract list limited to the plan. The second client id stays a deploy value (question D, issue 0).
5. False claim (issue 8 acceptance passes today): adopted; the acceptance now greps the `gateway` field and the unchanged default, and the text says the ticker type is already read. The "exactly one request in flight" claim: adopted into issue 0b (limit 1 set by the plan job).
6. Slip (broker issue missing, role file, deploy unowned): adopted for the broker issue; issue 4's Files gets `internal/store/setup/**`; the deploy side and question A stay the owner's and are named in `## Left out` and `## Owner questions`.
