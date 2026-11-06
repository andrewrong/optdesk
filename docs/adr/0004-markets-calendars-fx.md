# ADR 0004: Markets, trading calendars, FX and USD as the base currency

Date: 2026-10-02
Status: accepted

## Context

The owner also sells options on Japanese single stocks, for example four-digit codes on Osaka Exchange, and has a real-time subscription. Everything so far assumes the US: third-Friday monthlies, January LEAPS, 100-share contracts, USD prices and New York hours.

The owner works in one fixed local time zone without DST, set in configuration. They trade shortly after each open and want a snapshot then, plus one before the close. Open and close times must come from a maintained calendar, not from hand-written times. All money is finally judged in USD.

The owner's previous project fetched FX on demand from free web APIs, kept no history, and fell back silently to hard-coded rates. That is rejected.

Two advisor models debated this over two three-round sessions; the record is kept outside the repo.

## Decision

**Markets**
- A `Market` record describes each venue family:
  - id, timezone, currency, calendar code;
  - option-session overlay;
  - exercise style;
  - expiry labeller;
  - quality policy;
  - snapshot slots;
  - whether tiers apply.
- Broker routing (SMART for the US, OSE.JPN for Japan) stays in the IBKR adapter, keyed by market id.
- Contract unit, multiplier, strike grid, tick ladder (`reqMarketRule`) and the expiry dates are facts of each contract, read from IBKR definitions. They are not market constants.
- The watchlist identifies a name by `(market, symbol)`. Japanese symbols are strings, for example `"1234"`.
- Standard contracts only. FLEX and adjusted deliverables are excluded.

**Expiries**
- Listed expiries from definitions are the authority. `chain.py` stops hard-coding third Fridays and January LEAPS. A per-market labeller only classifies them (monthly, weekly, quarterly, LEAPS).
- Each expiry stores last trading day, exercise date, settlement date and the broker cutoff explicitly.
- Japanese single-stock options:
  - monthly series only;
  - European exercise;
  - last trading day is the business day before the second Friday, holiday-adjusted.

  These are JPX rules and must not be confused with Nikkei 225 SQ conventions. They are verified against definitions in the first Japanese probe.

**Calendars**
- Planning uses `exchange_calendars` (XNYS, XTKS) behind a `SessionCalendar` port: holidays, early closes and the Tokyo lunch break.
- XTKS describes the cash market only. A versioned JPX overlay adds option hours: 09:00–11:30 and 12:30–15:40 JST, with closing auctions at 11:35 and 15:45.
- At runtime, IBKR `tradingHours`/`liquidHours` (sampled per hours regime, cached daily) are the check. This is the "service-backed" source.
- Expected differences, such as the option close being later than the cash close, are not conflicts.
- Unexpected disagreement on whether the market is open:
  - collection still runs on the best schedule available;
  - observations are flagged;
  - the report shows the disagreement.
- If IBKR hours are unavailable, planning falls back to the library calendar with a degraded flag.
- "Today" is always the market's own session date.

**Snapshot slots**

| Market | Slot | Local time | KL time |
|---|---|---|---|
| US | open | 09:45 New York | 21:45 (summer) / 22:45 (winter) |
| US | close | close − 30 min (12:30 on half days) | 03:30 / 04:30 |
| US | after_close | close + 5 min (13:05 on half days) | 04:05 / 05:05 |
| JP | open | 09:05 JST; retry empty books at 09:15 | 08:05 |
| JP | close | 15:15 JST, before the 15:30 cash close | 14:15 |

- The US `after_close` slot (owner decision 2026-10-13) quotes the stocks only: no option is planned or quoted, and the run writes no report. It collects the accounts and, when readings are recorded (ADR 0030), publishes the Core page.
- There are no Japanese snapshots during lunch.
- Tiers from ADR 0003 apply to both markets. For Japan, the volatility band keeps at least three listed strikes per side, because Japanese strike grids are wide.
- Gateway auto-restart is set to 17:00 KL, outside every slot.

**Quote quality**
- **Stored:** everything received, including one-sided quotes, as received or NULL.
- **Recommendation-eligible:** all of the following:
  - live data type;
  - a positive bid with size at least the market's minimum;
  - fresh option and underlying quotes;
  - spread within the market's policy.
- A model price never makes a quote eligible.
- Spread limits are tick-aware and economically capped. Japanese limits are set from one week of measured spreads before Japan is enabled.

**Competing session (error 10197)**
1. Keep every quote already received; each quote has its own observation time.
2. Cancel the collector's subscriptions.
3. Probe one underlying plus one option at increasing intervals, requiring live data.
4. When the probe passes, retry unfinished items in tier order, and refresh any retained candidate older than its maximum age.
5. At the slot deadline, close the run as partial: what is missing, why, how long it was blocked, and the observation span.
- Loss of authentication (`auth_required`) and capacity limits (error 101) are separate statuses. They are reported, not retried blindly.

**FX service**
- A permanent `FxService` with these ports:
  - `FxFeed`: IBKR IDEALPRO USD.JPY over the guarded connection;
  - `ReferenceFeed`: ECB daily reference rates over HTTPS, with bounded timeout and size;
  - `FxStore`: DuckDB.
- The service holds one streaming USD.JPY line and stores every observation, including rejected ones, with:
  - source and contract;
  - bid and ask as `Decimal`;
  - receipt time, labelled as receipt time;
  - market-data type;
  - connection generation;
  - reason and policy version.
- **Decision-grade** requires all of the following:
  - live data type;
  - `0 < bid < ask`;
  - spread within policy;
  - the FX market open;
  - the current connection generation;
  - age within the limit (initially 180 s).

  These are re-checked when rules evaluate, not only when the run starts.
- **There is no hard-coded or default fallback rate.** If no decision-grade observation exists, the result is `FxUnknown`:
  - FX-dependent decisions are held;
  - local collection continues.
- **ECB cross-check:**
  - The ECB rates are crossed through EUR to get USD/JPY for one reference date.
  - They are compared with stored IDEALPRO mids around the 14:10 CET concertation.
  - The threshold is set after at least four weeks of shadow-mode measurement.
  - A large move against the previous fix is a market-move warning, not an integrity failure.
  - Until the first aligned check passes, the feed is "unverified".
- A manual override is an `fx_override` row (actor, reason, expiry), shown in reports.
- An internal daily fix is the median valid mid between 16:27 and 16:30 New York, recorded with its member observations.
- Rules and reports never fetch rates themselves. The service returns a frozen `ValuationContext`, and each conversion records which observation it used.

**USD as the base currency**
- A `Money(Decimal, Currency)` type refuses mixed-currency arithmetic.
- **Stays local or unitless:**
  - strikes, moneyness, ticks, spreads, the price-based volatility band;
  - IV and Greeks.
- **Converted to USD:**
  - capital, premium, fees, concentration, P&L.
- **Side by purpose**, for USD.JPY bid `b`, ask `a`, mid `m`:

| Purpose | USD value |
|---|---|
| Valuation | JPY / m |
| JPY proceeds (premium) | JPY / a |
| JPY obligation (assignment) | JPY / b |
| Funding with FX buffer `h` | JPY / (b × (1 − h)) |

- **Funding:**
  - The obligation is net of JPY cash actually available and not already allocated.
  - The buffer `h` starts at **10%** (owner decision), which adds about 11.1% to the USD needed. It is validated against historical yen moves before Japan is enabled.
  - The figure is an analytical reserve, not IBKR margin or buying power, and is labelled "FX-dependent".
- **Yield:**
  - Economic yield uses mid-valued premium over gross assignment exposure, consistently across markets.
  - The funding check is separate from the yield.
  - Reports show JPY originals next to USD values.
- **Conversion timing:**
  - A valuation batch uses one frozen FX context.
  - Cash flows convert at their event time, using the latest valid observation at or before it.
  - Later rates are labelled retrospective and never feed decisions.
  - USD P&L shows the local move and the FX effect separately.

**Order of work**
- US capture and US scheduling come first.
- Japan is enabled in the last step, only after:
  - the Japanese definitions and entitlement probe;
  - one week of measured spreads;
  - currency-aware rules;
  - the FX shadow period.

## Consequences

- `quotes.py`, `rules.py`, `chain.py`, `watchlist.py` and the store schema change. The glossary's "Money: USD" and US-only scope are revised.
- FX-dependent Japanese decisions can be held for "FX unknown". That is intended: no decision is better than one made on a stale rate.
- The ECB call is the only outbound HTTP the collector makes. It sends no account or position data.
- The owner's subscription may not cover OSE derivatives. The first Japanese probe checks this before anything else is built on it.

## Amendment (2026-10-02)

- The collector connects to a dedicated IB Gateway session using live data on the default port 4001, with "Read-Only API" enabled and connections limited to localhost.
- The owner does not use TWS while the Gateway session is active. Logging in to TWS or the mobile app to trade may displace the Gateway session or cause competing-session error 10197; collection then follows the handling above. Afterwards, the owner logs Gateway back in by hand. Login is never automated, and credentials never live in configuration or code.
- The owner logs in to trade after the open snapshot, so that snapshot informs the decision.
- Gateway auto-restart is set to 17:00 KL, outside every slot.
- TWS windows previously consumed the account's 100 simultaneous market-data lines; a 119-line warning was observed on 2026-10-02. Gateway has no windows, leaving the full allowance available to the tool.

## Amendment (2026-10-05)

- JP standard series: `OSE.JPN`, trading class equal to the stock code, multiplier 100 (observed 2026-10-05).
- US standard series (2026-10-09): `SMART`, multiplier 100, trading class equal to the stock symbol with its spaces removed. Measured 2026-10-09: the stock `BRK B` has the option trading class `BRKB` on all 21 definition rows, one of them `SMART`; a symbol without a space is unchanged.
- JP filters currently enforce exchange and trading class; explicit multiplier 100 enforcement is tracked in a follow-up.
- Owner decision `2026-10-05-unify-serve`: JP open/close snapshots and pre-open chain refresh are registered in `serve` using XTKS, separately from US sessions. Missing JP chains still allow underlying capture; this adds no JP option-definition logic.

On 2026-10-05, collection was changed so store calls never run on the event loop
while collection is running: reads use worker threads and writes are serialised and batched
in arrival order, with observations persisted before final item statuses. Contracts are
resolved once per underlying/expiry; resolution time is outside the quote timeout, with
separate hard limits for preparation and streaming. This change retains serial expiry loads
under one resolver-wide lock, including the pause between requests. The collector's default
60 s preparation limit includes definition lookups and time queued for that lock; the default
30 s request timeout starts after acquiring it. Two slow expiry loads ahead, or many distinct
underlying/expiry pairs taking 1–2 s each, can exhaust that preparation limit before a queued
item sends its request; existing timeout, retry and deadline handling then applies. When the
last waiter times out, its queued load is cancelled; a retry joins the lock queue at the back
and can exhaust its preparation limit again. Retries provide no fairness guarantee. The
single-underlying reproduction does not establish throughput for plans distributing 70 lines
across many pairs; those plans require reassessing this queue bound and request pacing.
The first real US open snapshot had
400 option items all end no_data/timeout with zero observations, while 34 underlyings were
fine. A 100-item reproduction at line budget 70 yielded 78 no_data and 22 OK in 95.7 s;
event-loop stalls reached 19.64 s (79.3 s total, 14 over 1 s). Synchronous per-item DuckDB
connections blocked request delivery, serial per-contract resolution consumed item timeouts,
and selecting open interest from a streamed contract without a right discarded tick 101.

**2026-10-09 — the stored USD.JPY series is a sample of the stream.** The feed used to emit
an observation on every change of bid or ask and once a second for an unchanged quote, and
each one is a synchronous database write on the service's event loop. Measured on a 280 MB
database: 0.9 write cycles a second around the clock, about 75,000 rows a day, the event
loop blocked more than half of the time; broker handshakes of the other connections timed
out and scheduled scans failed with a healthy gateway. The feed now emits the first
observation of a connection, every change of state (a side present or missing, the
market-data type, the connection generation) at once, and otherwise at most one observation
per 15 s; a price change inside that gap is dropped and the next emitted observation carries
the then-current quote and its own receipt time. "Stores every observation, including
rejected ones" above means every observation the feed emits. The acceptance rules and the
180 s age limit are unchanged.

**2026-10-14 — no service through a gateway not in Read-Only mode (owner decision).**
The system refuses service through an IB Gateway whose API is not in Read-Only mode. Right
after the handshake of every connection, before any other request, `transport.connect` opens
a second short connection with client id 0 to the same gateway, requests the open orders
(`reqOpenOrders`) and closes it. A gateway in Read-Only mode rejects that request (IBKR TWS
API documentation, "API client's orders"); on the owner's gateways the rejection is error
`321`, reqId `-1`, with the words `Read-Only mode` (observed 2026-10-14). The rejection within
5 seconds is `read_only`, reused for 60 seconds by connections to the same gateway; the
open-orders end without it is `not_read_only`; neither, or a `321` without those words, is
`unknown`, a failed connection. The open-orders end gives `not_read_only` only after a grace
of 250 ms without the rejection; a rejection within it gives `read_only`. The 5 seconds
cover the whole check, handshake and grace included. Checks of one
gateway run one at a time, since client id 0 is held by one connection at a time. That lock
holds within one process only: another process probing the same gateway at the same moment
fails its connection as `unknown`. On `not_read_only` one Telegram message goes at once to the
alerts topic (not held by the quiet hours), the gateway watch's state `not_read_only` is kept,
and the process ends with exit code 3, whatever the command. The state is kept in `--db` when
given, otherwise in the default database, resolved only at the refusal. Whatever starts
`serve` must not restart it on exit code 3. Binding note: on a gateway not in
Read-Only mode the client-0 request binds manual orders to the API; that gateway can already
take orders from any client and the process ends at once, so the binding adds no new
exposure. No order is ever sent, modified or cancelled. This guard is the system's own
refusal, not the broker-side guarantee: the Read-Only switch lives on the same machine as the
agents and may be switched off.

**2026-10-14 — the Read-Only check runs once per established connection (owner decision).**
The Read-Only check runs once per established connection to a gateway: on the first connection
of the process, and on the first connection after a failed one, because each check makes the
gateway show a window. A `read_only` result is held, with no time limit, until a handshake to
that gateway fails (timeout, refused, any exception of the connection; `transport.connect`
then makes the guard forget it), or until a gateway watch check finds the port closed or the
handshake failed. The 60-second reuse above no longer applies; `unknown` is still not held,
and the refusal on `not_read_only` is unchanged. The 5-minute watch check asks the gateway
only when no result is held. Accepted limit (owner's choice): the Read-Only switch turned off
on a gateway that keeps running and keeps answering is found only at the next re-established
connection (the gateway's daily restart, a re-login, any failed connection to it), not within
5 minutes.

**2026-10-17 — a Read-Only result lives only while a checked connection stays up (owner decision).**
A `read_only` result of a gateway is held only while at least one guarded connection to that
gateway has stayed connected without a break since the check. The guard counts the open
guarded connections of each gateway: a connection that passes the check adds one, its
disconnect of either kind takes one away, and at zero the result is dropped, so the next
connection is checked again. An unexpected disconnect (not started by our own `disconnect()`)
drops the result at once, even while other connections to the gateway stay open, because a
lost socket may mean a restarted gateway; those connections are not closed. A failed
handshake and `forget` still drop it, and a drop during a check still wins. The handshake's
connection time cannot tell two gateway runs apart, hence the tie to connections. Each check
logs one line: the port, the reason (`first`, `count_zero`, `unexpected_disconnect`,
`handshake_failed` or `forgotten`) and the result. This replaces "held, with no time limit,
until a handshake fails" of the note of 2026-10-14.

**2026-10-17 — a settings change is checked by one Read-Only probe (owner decision 2026-10-16 and 2026-10-17).**
A gateway's IB Gateway settings folder may be named in the local configuration through an
environment variable (`settings_dir_env` in `[gateways.<name>]`, or
`OPTION_TRADER_IB_SETTINGS_DIR` without `[gateways]`; the variable holds the path, the
repository only its name). A small task of `serve`, the settings watcher, lists each such
folder every 2 seconds and keeps a snapshot of the names and modification times
(`st_mtime_ns`) of its files matching `ibg*.xml` and `*.ini`, read from the folder listing
only: the files are encrypted and never opened. Any added, removed or changed file marks the
gateway pending; once its files have stayed unchanged for 5 seconds the watcher calls the
guard's `recheck` (reason `settings_changed`): the held result is dropped and, when a guarded
connection to the gateway is open, `require` probes at once under the rule of the 2026-10-17 Read-Only note; with none
open, the next connection checks. So a settings change is normally checked by one Read-Only
probe within about 10 seconds (a 2-second scan, then 5 seconds without further change), and
there is at most one such probe per gateway per minute: two rechecks of one gateway are at
least 60 seconds apart, a change inside that time waits for it. Each such probe may show a
pop-up on the gateway. `read_only` clears pending, `not_read_only` is refused as before, and
an unknown result is retried after 60 seconds (one WARNING after 3 in a row); a third recheck
of one gateway within 10 minutes logs one WARNING that its settings files keep changing. The
timings live in the gateway watch parameter file. Without a configured folder the limit of
the note of 2026-10-14 is unchanged.
