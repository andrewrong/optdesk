# Underlyings: the owner adds, pauses and drops a name on one page; the jobs follow the stored list and spend no call on a name or job that does not need it

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-11
Revision: 2

Source: owner 2026-11-11: underlying (标的) management must exist; do not waste quota; a hand-edited config file is too hard to maintain. Owner 05:03Z: "a" (web page only, two switches per name) and "an underlying must have its basic attributes, including whether options are enabled" (Decision 3, the attribute table). Owner 05:05Z: "separate them where they should be separate" (indices and FRED series are not underlyings, Decision 4). The project advisor's proposal is the base; every path and symbol below was read on origin/main at f2b5473b and where the proposal was wrong it is corrected in `## Corrections to the proposal`.
Read: `AGENTS.md`, `docs/design/modules.md`, `docs/design/settings-form.md` (rev 5), `docs/design/macro-rates-view.md`, `docs/design/option-open-pull.md`, `migrations/0008`, `0025`, `cmd/ot/{watch,runonce,scheduler,wire,settings,jobreg,wire_optionopen}.go`, `internal/jobs/{api.go,barsrefresh,optioneod,optionopen,optioncandidates,ivhistory,dividendcapture,earningsrefresh,fundamentals}`, `internal/store/{instrument,ownerset,ownersetpg}`, `internal/sources/{api.go,jquants}`.

Goal: the owner opens `/underlyings` on `ot serve --owner-edit`, adds a name (symbol, market, kind, switches), pauses it, switches its options or fundamentals off, or drops it, and sees which job groups follow each name. The change is read by the next job plan with no restart. A name or job group that is off costs no broker, vendor or filing call. A contract is asked for once, when the name is added, and its `con_id` is stored. `$OT_HOME/watch.toml` is imported once and then no longer read.

## What main holds today

- The list is `$OT_HOME/watch.toml`, parsed by `parseWatch` in `cmd/ot/watch.go` into `[]jobs.WatchEntry` (`internal/jobs/api.go`: `Issuer`, `Market`, `ReportDateOverride`, `BuyPrice`, `SalePrice`, `Contract{SecType,Symbol,Exchange,Currency,ConID}`). `loadWatch` is called by `loadConfig` (`runonce.go:101`, used by `ot run-once` and, with `allowNoFredKey`, by `ot scheduler` at `scheduler.go:61`), by `cmd/ot/candidates.go:134`, `dryrun.go:133` and `companies_site.go:39`. It is read once at process start. A row needs a valid `issuer` (`cik:<10 digits>` or `edinet:<code>`) and a market `US` or `JP`; two rows with one issuer are refused.
- Jobs take the slice at construction: `bars_refresh`, `dividend_capture`, `earnings_refresh`, `fundamentals`, `iv_history`, `option_eod`, `option_eod_jp`, `option_candidates`, `option_open*` (through `optionopen.Stored.Watch`) and the valuation step of `compute_figures` (`Entries`). The ten uses are the `f.watch` of `jobFactory` (`cmd/ot/wire*.go`, `candidates.go:75`, `wire_optionopen.go:176`). The J-Quants source is built at start with `jpSymbols(cfg.watch)` (`runonce.go:523`) and makes one call per symbol per `Fetch` (`internal/sources/jquants/client.go`), so a name paused or added later changes nothing until a restart.
- `quote_snapshot` does not read the list: its instruments are the `[[instrument]]` of its own `job.toml`. `positionmarks` follows positions only.
- The scheduler and `run-once` build `wiring.jobs` once (`withWiring` / `wireJobs` in `runonce.go`); `contracts.Job` has `ID`, `Plan`, `Run` and jobs also carry optional methods (`Schedule`, `RunBound`, `RestartSensitive`, and for `option_candidates` `WithSettingsVersion`).
- `con_id`: `WatchEntry.Contract.ConID` comes from the file (0 when absent). With 0: `option_eod` and `option_eod_jp` end the key `NoValue` "no con_id in watch entry" (`optioneod/run.go:187`) and call nothing; `bars_refresh` calls `ResolveContract` once per name per run and caches in memory only (`barsrefresh/run.go`, `runState.conID`); `dividend_capture` takes `int(e.Contract.ConID)` as the instrument key (`dividendcapture/run.go:175,190,229`); `optionopen` passes the entry's `ConID` on (`inputs.go:89,106`); PR 1534 (open) makes `iv_history` call the broker for it. The `master.instrument` row of a name is created by the first job that runs with its con_id (`store.InstrumentResolver.ResolveInstrument(ibkr_conid, spec)`, `internal/store/instrument/resolve.go`), and the spec (calendar, increment) is the job's own: `XNYS`, `0.01` for US, the `[stock]` table of `job_jp.toml` for JP. No `master.instrument` row exists for a name no job has run for.
- Settings machinery on main (settings-form rev 5, built): schema `settings` with insert-only tables (`migrations/0025_settings.sql`), role `ot_w_settings` (INSERT and SELECT on the tables, SELECT on the `rd` views), `ot_ro_pages` and `ot_runner` read the views only, `internal/store/ownerset` (declarations) and `ownersetpg`, `POST /owner-settings` behind `--owner-edit` (loopback, `Host`/`Origin` check, HMAC token `OT_FORM_KEY`, 15 minutes, two-step confirm, history) in `cmd/ot/settings.go`, the line `OT_PG_DSN_SETTINGS` (`ownerset.DSNEnvSettings`), page `settings_page` and its view.
- Sizes: `internal/store/api.go` is at its limit (settings-form Decision 6); `cmd/ot/wire.go` is 790 lines and issue 9b only replaces its one `f.watch` use (the field is declared in `runonce.go`); `runonce.go` is 554, `site.go` 526.
- Next free migration: `0033` (main has `0032_option_contract.sql`; no open pull request touches `migrations/`).

## Corrections to the proposal

1. Key and identity. "Keyed by master instrument_id" cannot hold: a pending name has no `master.instrument` row, and the row is created by the first data job with that job's spec. The key is `name_key` (`US:AAPL`, market and upper-case symbol); the owner's contract request (symbol, market, kind, exchange, currency) is stored in the row, as the file held it. Nothing is written to `master` by this design.
2. `con_id` home. It is stored in the underlying row (set by the resolve job), not read from `master.instrument_identifier`: that identifier does not exist before a job ran. Master gets the same number later through the jobs' own `ResolveInstrument`. Decision 3 states the one-writer rule.
3. `report_date_override` (an owner field read by `earnings_refresh` and `option_candidates`) was missing; it is a column.
4. Quota: `option_eod` with con_id 0 costs no call, it fails the key; the call saving on main is `bars_refresh` (one `ResolveContract` per name per run; `iv_history` would save one only if PR 1534 merges), option legs are not saved by an underlying con_id, and the gains for `optionopen`, `dividend_capture` and the eight JP names are correctness. The J-Quants source symbol list is fixed at start; it must follow the list too (issue 9b).
5. Optional issuer is not free: `bars_refresh` keys each name by `Issuer` and refuses the whole job on an empty one (`barsrefresh/job.go`), `option_candidates` keys by issuer, and `obs.dividend_expectation` and `obs.earnings_date` have the pattern `^(cik:...|edinet:...)$` (so no placeholder issuer). Issue 7 gives `bars_refresh` a key for an issuer-less name; `option_candidates` skips it with a named reason (Left out).
6. Re-reading at plan time is not free either (the proposal's "not verified" point, checked): jobs take the slice at construction. Issue 9a adds one decorator in `cmd/ot` that rebuilds the inner job from the current list at each `Plan` (a run keeps its own snapshot); no job constructor changes. Issue 9b, the wiring, is the large one, and the optional methods of a job must be forwarded (Decision 9).
7. Settings role grants: a new table needs its own grants (the migration of issue 1 states them, as 0025 does). The resolve job runs with the runner DSN, which cannot INSERT into `settings`; it gets its own role `ot_w_underlying_resolve` and its own line `OT_PG_DSN_UNDERLYING_RESOLVE` (as `barDSN` and `openDSN` are per-job lines), with INSERT and SELECT on `settings.underlying` only: `ot_w_settings` has INSERT on every settings table (`0025_settings.sql`), so it is not used. The job is left out when the line is empty. The owner's word is one new env line.
8. `iv_history` follows the list (all entries, no filter today) and takes the `options` switch; `quote_snapshot` does not follow the list at all and stays as it is.
9. Line numbers: the `conID` function of `bars_refresh` is a function, cited by name.

## Decisions

1. One page, two switches, web only (owner "a"). No CLI editor; the one command is `ot underlying import`. Two switches per name: `options` (off: no `option_eod`, `option_eod_jp`, `option_open*`, `iv_history`, `option_candidates`) and `fundamentals` (off: no `fundamentals`, `earnings_refresh`, valuation of `compute_figures`). No per-job toggle, no tiers.
2. Store. One insert-only versioned table `settings.underlying`, newest row per `name_key` wins (ties by `id`), `set_at` from the database. The row carries every attribute of the table in Decision 3 that the settings schema holds. View `rd.underlying_r1` passes every column; `ot_ro_pages` and `ot_runner` read the view and no table; `ot_w_settings` has INSERT and SELECT on the table and SELECT on the view. Interface in the new declarations-only package `internal/store/underlying`, implementation in `internal/store/underlyingpg` (precedent `ownerset` and `ownersetpg`; `internal/store/api.go` untouched). Own implementation, because the existing writer is bound to two other tables and a library cannot give the versioned-settings shape; it is behind `underlying.Reader` and `underlying.Writer`.
3. The attributes of an underlying, one table. Stored = column of `settings.underlying`; "derived" = computed when read, never stored. Every field that `jobs.WatchEntry` or a job package reads has a row here (the check is the last column).

| attribute | meaning | type, allowed values | stored where | set by | editable after add | on the page | jobs that read it (through `WatchEntry` unless noted) |
|---|---|---|---|---|---|---|---|
| `name_key` | identity of the name | `<MARKET>:<SYMBOL>` | settings.underlying | derived at add | no | yes (symbol and market) | none (page, history) |
| `symbol` | IBKR symbol (JP: 4-digit code) | `^[A-Z0-9.]{1,12}$` (JP `^[0-9]{4}$`) | settings.underlying | owner at add | no (drop and add) | yes | all families (`Contract.Symbol`) |
| `market` | market of the listing | `US`, `JP` | settings.underlying | owner at add | no | yes | all (`WatchEntry.Market`: earnings vendor, JP split, dividends) |
| `kind` | what it is | `stock`, `etf` (index is not a kind, Decision 4) | settings.underlying | owner at add | no | yes | derives `Contract.SecType` (`STK` for both); `fundamentals` needs an issuer |
| `exchange` | IBKR routing exchange | text; default `SMART` (US) or `TSEJ` (JP) | settings.underlying | owner at add | only while status is `pending` or `unresolved` | yes | resolve job, `bars_refresh`, `option_eod*`, `optionopen` (`Contract.Exchange`) |
| `currency` | currency of the contract | `USD`, `JPY` (three upper-case letters) | settings.underlying | owner at add | only while `pending` or `unresolved` | yes | every job that builds the contract (`Contract.Currency`) |
| `name` | display name | text, at most 80; empty means the symbol | settings.underlying | owner (optional) | yes | yes | `companies`, `home` pages (`Symbol`/name); jobs use the symbol as instrument name |
| `issuer` | company id for filings and calendars | `cik:<10 digits>` or `edinet:<code>`; empty allowed | settings.underlying | owner at add | yes | yes | `fundamentals`, `earnings_refresh`, `dividend_capture`, valuation, `option_candidates` (issuer-less names are skipped by those that need it, issue 7), `bars_refresh` (key) |
| `con_id` | IBKR contract id | positive integer, or empty until resolved | settings.underlying (one writer: the resolve job, or the import of a file value) | resolve job | no (cleared when `exchange` or `currency` is corrected) | yes (read-only) | every job (`Contract.ConID`); with it set, no job resolves the underlying |
| `status` | lifecycle | `pending`, `active`, `paused`, `retired`, `unresolved` | settings.underlying | owner: `active`, `paused`, `retired`; system: `pending`, `unresolved`, and `active` after resolve | yes (owner states only) | yes | the filter (Decision 6) |
| `reason` | why `unresolved` | text, at most 200 | settings.underlying | resolve job | no | yes | none |
| `options` | switch | boolean, default true | settings.underlying | owner | yes | yes | filter: `option_eod`, `option_eod_jp`, `option_open*`, `iv_history`, `option_candidates` |
| `fundamentals` | switch | boolean, default true when an issuer is given, else false | settings.underlying | owner | yes | yes | filter: `fundamentals`, `earnings_refresh`, valuation |
| `held` | the owner holds shares of it | boolean | not stored: derived from the newest stock positions of the account aliases (the reader and look-back `optionopen.Stored.positions` uses) | derived | no | yes | filter (Decision 6) |
| `buy_price` | price the owner would buy at | positive decimal text, at most 4 places, or empty | settings.underlying | owner | yes | yes | `WatchEntry.BuyPrice`: `option_candidates` (owner's word wins), `option_eod_jp`, `companies` page |
| `sale_price` | price the owner would sell at | as `buy_price`, not below it | settings.underlying | owner | yes | yes | `WatchEntry.SalePrice` (companies page, home) |
| `report_date_override` | owner's own report date | `YYYY-MM-DD` or empty | settings.underlying | owner | yes | yes | `earnings_refresh`, `option_candidates` (`EarningsInfo.Override`) |
| `note` | free text | at most 200 | settings.underlying | owner | yes | yes | none |
| `set_at` | when this version was written | timestamptz, database `now()` | settings.underlying | database | no | yes (history) | none |

   Field-source check against `jobs.WatchEntry`: `Issuer`, `Market`, `ReportDateOverride`, `BuyPrice`, `SalePrice`, `Contract.SecType/Symbol/Exchange/Currency/ConID` each have a source in the table (kind gives SecType). Gaps found and what is done: (a) `ReportDateOverride` was missing in the proposal and is a column; (b) an empty `issuer` is valid in the table but three jobs assume a non-empty one: `bars_refresh` and `option_candidates` (issue 7), `dividend_capture`, `earnings_refresh` and valuation already skip it; (c) the instrument spec (calendar, MIC, increment, lot) that jobs pass to `ResolveInstrument` is not an owner attribute and not in the table: it stays in each job's manifest (`[stock]` of `job_jp.toml`), so the resolve job never creates a master row and no spec exists in two places; (d) `con_id` also lands in `master.instrument_identifier` later by the jobs' own call; the jobs read `con_id` only from the entry (the underlying row) and write the identifier from that entry (`ResolveInstrument(ibkr_conid, entry.ConID)`), so the two copies cannot differ unless the row's `con_id` is cleared and re-resolved, which only a corrected `exchange` or `currency` of an unresolved row does, before any job has run for it; and `name_key` is the owner's name, not an instrument identity: after the first job run the instrument is `ibkr_conid` in master, and a renamed symbol is a new name (drop the old, add the new; the old instrument keeps its history under its con_id); the resolve job writes only the underlying row. Edits to a field the table marks "no" are refused with a named field (422).
4. Indices and FRED series are not underlyings and stay out of this table (owner 05:05Z). They are not tradable names the owner follows: no switches, no issuer, no buy price, no options. Where they live today and whether they stay: the four indices SPX, NDX, RUT and N225 are the `[[index]]` of `internal/jobs/barsrefresh/job.toml` (sec_type `IND`, `con_id` 0, resolved per run in memory, four calls a run); they stay there, a fixed list in code. The FRED series are the `series` list of `internal/jobs/fredseries/job.toml` (and the `computefigures` and `macro_rates` lists); they stay there. The cost left is four `ResolveContract` calls per `bars_refresh` run for the indices; a small list of their own (with the `con_id` stored once) is a later cut, named in Left out. The page shows no index and no series.
5. Editing path. The POST route `POST /owner-underlyings` sits on the same guards as `/owner-settings` (flag `--owner-edit`, loopback listen address and remote address, `Host` and `Origin` names, form content type, 64 KiB, HMAC token with `OT_FORM_KEY`, 15 minutes, two steps), `guardOwnerPost` of `cmd/ot/settings.go` reused as is. The first post answers the page in confirm mode with a table old to new and, for any change that removes a job group (a pause, a retire, a switch off), a line naming the groups that stop for that name; the second post (`confirm=1`) applies. The page refuses a save that would leave no `active` name (422, field `status`, the text "at least one name must stay active"), so the owner cannot empty the list from the page. The `stops` lines also say that days missed while a group is off cannot be refilled (the daily history of a skipped day is not fetched later, `option_eod` and `option_open` rows of that day are not backfilled). A 303 goes to `/underlyings?saved=<n>`. The route needs `OT_PG_DSN_SETTINGS`. `ot serve` stores the row and computes nothing and has no broker; this adds one more table to the loopback writer exception of settings-form Decision 5 and nothing else.
6. Who follows a name (the filter). Four job groups: `bars`, `dividends`, `options`, `fundamentals`. A name with status `active` is followed by `bars` and `dividends`, by `options` when `options` is on, by `fundamentals` when `fundamentals` is on and an issuer is set. `pending` and `unresolved` are followed by nothing (no con_id). `paused` and `retired` names are followed by nothing, except that a held name keeps `bars` and `dividends`. The per-job table (every scheduled job and the two pages that read the list today; a job not named reads no list):

| job or page | group | statuses followed | held exception |
|---|---|---|---|
| `bars_refresh` (names; its four indices are fixed, Decision 4) | bars | `active` | a held `paused` or `retired` name stays |
| `dividend_capture` | dividends | `active` (needs an issuer, as today) | a held `paused` or `retired` name with an issuer stays |
| `option_eod`, `option_eod_jp` | options | `active` with `options` on | none: the job adds the short calls of held stocks itself (`NewWithHeld`), so the short-call quotes of a paused or options-off held name keep coming |
| `option_open_resolve`, `option_open_resolve_gap`, `option_open_plan`, `option_open` and the `_jp` four | options | `active` with `options` on | none: `optionopen.Stored.stocks` adds held stocks itself |
| `iv_history` | options | `active` with `options` on | none |
| `option_candidates` | options | `active` with `options` on and an issuer | none |
| `fundamentals`, `earnings_refresh` | fundamentals | `active` with `fundamentals` on and an issuer | none |
| `compute_figures` (the valuation step reads `Entries`) | fundamentals | `active` with `fundamentals` on and an issuer; its other steps read no list | none |
| `position_marks` | none: it follows the positions of the accounts, never the list | n/a | n/a (a held name is marked whatever its status) |
| `quote_snapshot` | none: its own manifest | n/a | n/a |
| page `companies` (`cmd/ot/companies_site.go`), page `home` company rows (`home_site.go`) | fundamentals | `active`, `paused` and `retired` names are shown with their status; names without an issuer are not shown (as today) | n/a |

   The rule is written twice, once in the page builder (the text of "followed by") and once in the `cmd/ot` filter, and both are tested against the one table `contracts/fixtures/underlyings.follow.json` (issue 3), so they cannot drift. Own implementation, because the rule is ours; it is a pure function.
7. Resolving, once. A name added on the page is written `pending` with no con_id. The job `underlying_resolve` (new package `internal/jobs/underlyingresolve`, manifest `job.toml`) reads the newest rows with status `pending`, and with status `unresolved` whose `set_at` is more than 7 days old (one retry a week, no new table; from option-trader: ask each contract once, remember not-listed), calls `ResolveContract` for each, and writes through `underlying.Writer.Settle`: listed gives `con_id` and status `active`; not listed or ambiguous gives status `unresolved` with a `reason`; a broker or transport error leaves the row as it is (retried the next run). `Settle` inserts only if the newest row of the key is still `pending` or `unresolved` with the same `id` the job read, so an owner who paused or retired the name meanwhile is not overwritten; this pending-only rule is part of the `Settle` contract and has a contract test (issue 1, both sides) and a pg test (issue 2). The job holds `underlying.Settler` (read and `Settle` only), not the `Writer`, so it cannot call `Apply`. Bounded: at most 20 names per run, the key is one name, run bound 10 minutes, once per trading day after the close (anchor `session_close`, offset 45 minutes, calendar XNYS); a name added in the day is therefore active from that evening, and the page says "resolves after the close" for a pending name. The job has no pacing of its own: it asks one contract at a time through the broker the other jobs share.
8. What costs fewer calls, as results. Savings on main: `bars_refresh` makes no `ResolveContract` for a watched name once its `con_id` is stored (today one per name per run, `barsrefresh/run.go` `runState.conID`); a paused or switched-off name is in no plan, so it costs no chain, quote, history, filing or J-Quants call; the J-Quants source symbols follow the list (Decision 9). Not saved by a stored underlying `con_id`: the option legs (`ResolveContract` per strike and expiry, `optioneod/run.go` `listed`; the option-open design caches those itself). Fixed, not saved: the four indices (Decision 4); `quote_snapshot` is unchanged. Correctness gains, not savings: `option_eod_jp` and `option_open` stop failing on a missing con_id (`optioneod/run.go:187`), and `dividend_capture` stops using 0 as an instrument key. `iv_history` would save a broker resolve only if PR 1534 (open, adds that resolve per run) merges; this design counts nothing for it.
9. Reading at plan time, no restart. `cmd/ot` gets one decorator `listedJob` (issue 9a): it holds the build function of a job family (`func([]jobs.WatchEntry) (contracts.Job, error)`), builds an inner job once at start (the list at start, so the optional methods and the schedule are answered), and at each `Plan` reads `Reader.Current`, derives the entries of its group with the filter of Decision 6 and the `WatchEntry` fields of Decision 3, builds a new inner job and plans with it. In-flight rule: a run keeps the list of the `Plan` it came from. Each `Plan` of a day adds a snapshot `{key set, inner job}` and never replaces one; `Run(day, keys)` uses the newest snapshot of that day whose key set contains all `keys` (none: it fails permanent "no plan for these keys"); snapshots of earlier days are dropped at the first `Plan` of a new day. A second `Plan` the same day therefore cannot change the inner job under a run in flight (a test starts a run, plans again with another list, and checks the run still uses its own). The optional methods (`Schedule`, `RunBound`, `RestartSensitive`) are forwarded to the newest inner job; `option_candidates` `WithSettingsVersion` is applied inside the build function, not on the decorator. Empty list: if the store holds no `active` name at all, `Plan` returns a permanent error `underlying list is empty` in every family. The runner records it as a failed run, which `ot scheduler status` and the failure notice show like any failed job; it is not a log line, and it never falls back to the file or a stale list. (A family whose group is off for every name plans zero keys without error: that is a choice, not an empty store.) A store read error at `Plan` returns a retryable error and plans nothing for the family. The earnings family also rebuilds its J-Quants source with the symbols of the current JP names that pass the filter (`jpSymbols`), so a paused JP name is not asked. `held` is read once per `Plan` from the positions reader.
10. Import and cutover. `ot underlying import <watch.toml>` parses the file with the existing `parseWatch` and writes one row per entry through `Writer.Apply`: status `active` when the file gives a non-zero `con_id`, else `pending`; `kind` `stock`; `options` and `fundamentals` true; a name already in the store (same `name_key`) is left as it is and counted. It prints the counts and the names left pending. It is idempotent and never writes the file. After issue 10 no code reads `watch.toml` at run time: `loadWatch`, its callers and `watchFileName` are removed in the last issue; `parseWatch` stays for the import until the import itself is removed in a later cleanup. At no point do two sources feed the jobs: issue 9b switches the jobs to the store in one step, after the import and its check (`## Rollout`), and issue 10 removes the reader.
11. `settings-form.md` is amended, not superseded: its Decision 4 ("`buy_price` and `sale_price` stay hand-edited in `watch.toml` until valuation lands") is replaced by "they are columns of `settings.underlying`, edited on `/underlyings`" and the line of `## Left out` that keeps `watch.toml` out of the form is struck. Its route, token, confirm and history are reused unchanged. `option_exclusion` keeps its meaning and table (an exclusion of an option name from candidates); the `options` switch of this design removes a name from every option job and is a different, coarser control. Both stay; the page `/settings` is not edited here.
12. The page `/underlyings`, schema `page.underlyings.v1`, English texts, one tanstack table (columns: name, kind, status, options, fundamentals, held, buy, sale, followed by, note), an add form and a per-row edit form, plain `<form method="post">` with hidden `expires` and `mac` as settings-form Decision 11 (no `fetch`, no form library). A pending name shows "resolves after the close" (after 36 hours: "the resolve job has not settled this name; check ot scheduler"), an unresolved one shows its `reason`. Read-only (no `--owner-edit`): the same page with `form` null and a gap text. The list read for the page is the store's `Current` plus the last 20 versions as history.
13. Libraries: none new. tanstack table and shadcn are installed (`web/src/components/data-table/data-table.tsx`). Own implementation, because there is no library for these versioned owner-list semantics; the store and the filter sit behind interfaces so a library can replace them.
14. Size and tests: every new file under 600 lines; every test calls `t.Parallel()`, writes under `t.TempDir()`, takes time from the fake clock, opens no port (handlers under `httptest`) and touches no live broker; PostgreSQL tests carry the build tag `pg` and run in GitHub CI only.

## Modules

No row is added. Changed cells of `docs/design/modules.md`, edited in this design's pull request (`arch.toml` is generated; `.go-arch-lint.yml` is tree-derived):

- store, entry cell: gains `internal/store/underlying`, a declarations-only sub-package (the rule of `market-bars.md`, as `ownerset`, `mark`, `openquote` stand there).
- ot: no cell changes; its import set is derived from the depends cell and the package kinds, and `cmd/ot` will import `internal/store/underlying` (declarations), `internal/store/underlyingpg`, `internal/jobs/underlyingresolve` and `internal/pages/underlyings`, all of rows in its depends cell.

New directories: `internal/store/underlying`, `internal/store/underlyingpg`, `internal/jobs/underlyingresolve`, `internal/pages/underlyings`, `web/src/pages/underlyings`. Every path of an entry belongs to one row by the first matching glob: store (`internal/store/**`, `migrations/**`, `docs/contracts/store.md`), contracts (`contracts/**`, `internal/contracts/**`, `web/src/generated/**`, `docs/contracts/page-schemas.md`), pages (`internal/pages/**`, `docs/contracts/pages.md`), jobs (`internal/jobs/**`), ot (`cmd/**`, `docs/contracts/ot/**`), web (`web/**`). Every entry is within one module.

Import checks: `jobs/underlyingresolve` imports `store/underlying` (declarations of a row in its depends cell), `broker`, `clock`, `contracts`; `pages/underlyings` imports `store/underlying`, `contracts`; `store/underlyingpg` imports `store/underlying`, `store` and pgx; `cmd/ot` imports the four packages above.

## Interface

Owner: store (`internal/store/underlying/api.go`, declarations only, with `api_test.go` calling both sides) and contracts (the page schema). Contracts line of the store issue: `Contracts: unchanged`; of the schema issue: `page.underlyings v1 (new)`.

```go
package underlying

const ( StatusPending = "pending"; StatusActive = "active"; StatusPaused = "paused"
        StatusRetired = "retired"; StatusUnresolved = "unresolved" )
const ( KindStock = "stock"; KindETF = "etf" )

// Row is one version of one name: every attribute of Decision 3 that the table stores. Text fields are the
// canonical text of the store ("" is unset); BuyPrice and SalePrice are decimal text, never a float.
type Row struct {
    ID int64; Key string          // name_key, "US:AAPL"
    Symbol, Market, Kind, Exchange, Currency, Name, Issuer string
    ConID int64                   // 0 until resolved
    Status, Reason string
    Options, Fundamentals bool
    BuyPrice, SalePrice, ReportDateOverride, Note string
    SetAt time.Time
}

// Reader is read through rd.underlying_r1.
type Reader interface {
    Current(ctx context.Context) ([]Row, error)               // newest row of every key, sorted by Key
    History(ctx context.Context, limit int) ([]Row, error)    // newest versions first
}

// Add is a new name, written pending (or active with a ConID, the import).
type Add struct { Symbol, Market, Kind, Exchange, Currency, Name, Issuer string; ConID int64
    Options, Fundamentals bool; BuyPrice, SalePrice, ReportDateOverride, Note string }
// Edit changes the editable fields of Decision 3 of one key; a nil field is unchanged.
type Edit struct { Key string; Status, Name, Issuer, Exchange, Currency, BuyPrice, SalePrice, ReportDateOverride, Note *string
    Options, Fundamentals *bool }
type Change struct { Adds []Add; Edits []Edit }

// Outcome is what the resolve job settles for one pending or unresolved row.
type Outcome struct { Key string; AfterID int64; ConID int64; Reason string } // ConID 0 = not resolved, Reason says why

// InvalidError names the field a value was refused for; nothing is written.
type InvalidError struct{ Field, Why string }

type Writer interface {   // page route and import; role ot_w_settings
    Apply(ctx context.Context, c Change) (written int, err error)   // one transaction; unchanged fields write nothing
    Close() error
}
type Settler interface {  // the resolve job; role ot_w_underlying_resolve
    Pending(ctx context.Context, retryBefore time.Time) ([]Row, error) // newest rows pending, or unresolved with SetAt before retryBefore
    Settle(ctx context.Context, o Outcome) (written bool, err error)   // inserts only if the newest row still has id AfterID and status pending or unresolved
    Close() error
}
type Store interface { Reader; Writer; Settler }
const DSNEnvResolve = "OT_PG_DSN_UNDERLYING_RESOLVE"  // the line of the resolve job's role
type OpenFunc func(ctx context.Context, dsn string) (Store, error)  // underlyingpg.Open
```

The page route and the import use the line `ownerset.DSNEnvSettings` (`OT_PG_DSN_SETTINGS`), passed by `cmd/ot` (this package imports no sibling); the resolve job uses `DSNEnvResolve` declared here. Validation (Decision 3 allowed values, the sale-not-below-buy rule, one `name_key` per row, the editable-field rule) lives in `underlyingpg/validate.go`, a pure file, so the page route and the import share it.

Migration `0033_underlying.sql`: `settings.underlying (id bigint identity primary key, name_key text not null, symbol, market, kind, exchange, currency, name, issuer, con_id bigint null, status, reason, options boolean, fundamentals boolean, buy_price text, sale_price text, report_date_override text, note text, set_at timestamptz default now())` with checks on market, kind, status; index `(name_key, set_at desc, id desc)`; owner `ot_owner`; view `rd.underlying_r1` (every column named); grants as in `0025_settings.sql`, one statement per role: `ot_w_settings` INSERT and SELECT on the table, SELECT on the view; `ot_w_underlying_resolve` (made by `internal/store/setup`, line `OT_PG_DSN_UNDERLYING_RESOLVE`) USAGE on schema `settings` and `rd`, INSERT and SELECT on `settings.underlying` and SELECT on `rd.underlying_r1`, nothing on any other settings table; `ot_ro_pages` and `ot_runner` SELECT on the view and no table. The Down part drops them. No dataset file in `contracts/datasets` (owner state, not an observation).

Page `page.underlyings.v1` (hand-written envelope as `page.macro_rates.v3`: `schema` constant `underlyings`, `schema_version` integer 1, `generated_at`, `session_day`, `title`, `sentence`, rows, `gaps`): `rows` list of `{key, symbol, market, kind, name, status, status_text, reason, issuer, con_id (integer or null), options, fundamentals, held, buy_price, sale_price, report_date_override, note, followed_by (list of enum bars, dividends, options, fundamentals), followed_text, set_at (owner-zone text), editable (list of field names)}`, `form` (`{post_path "/owner-underlyings", expires, mac}` or null), `confirm` (list of `{label, old, new}` and `stops` list of text, or null), `notice`, `history` (newest 20 `{at, key, text}`). Every visible string is a field; the view formats nothing. The English texts and the fixtures are pinned in issue 3.

## Rollout

The jobs read the store from the merge of issue 9b on, and an empty store fails every family, so the order is fixed. Numbered steps, who does each:

1. The window merges issues 1 to 8, 9a and 7 (store, resolve job, import, page, decorator, key fix; nothing changes in what the jobs follow; `underlying_resolve` is not wired yet).
2. The owner's window applies migration 0033 and the role: `ot setup pg`, then `ot migrate`, then `ot setup pg` again (the sequence of the migration rule), and adds the line `OT_PG_DSN_UNDERLYING_RESOLVE` that `ot setup pg` writes to the runner secrets dir through agent-crew (the window never edits `.env`).
3. The window runs `ot underlying import $OT_HOME/watch.toml` (needs `OT_PG_DSN_SETTINGS`), which writes `active` rows for names with a `con_id` and `pending` rows for the others.
4. The window runs `ot underlying import --check $OT_HOME/watch.toml` and compares: the count of `[[underlying]]` in the file equals the count of names in the store, and the command exits 0. A different count stops the rollout; the window re-runs the import (idempotent) and reports the difference.
5. The window merges issue 9b (the switch): from the first `Plan` after the auto-deploy the jobs follow the store, and `underlying_resolve` starts and settles the pending names in the evening. The owner opens `/underlyings` and checks the names and the pending rows.
6. After a week of runs with no `underlying list is empty` failure, the window merges issue 10, which removes the file loader. Until then the file is not read by any job, only by the import.

## Issues

Order (the day this design merges: 1, 3, 7 start; the rest as shown): 1, 3 and 7 first; 2 after 1; 4 after 1 and 3; 5 after 1; 6 after 2; 8 after 2 and 4; 9a after 1 and 3; 9b after 2, 5, 6, 7 and 9a, and only at step 5 of the rollout; 10 last, after 6, 8 and 9b; 11 after 3.

### 1 underlying-store-api (interface)

Module: store. Size: M.
Files: migrations/0033_underlying.sql, internal/store/underlying/api.go, internal/store/underlying/api_test.go, internal/store/setup/setup.go, internal/store/setup/setup_test.go, internal/store/setup/underlying_pg_test.go, docs/contracts/store.md
Design: docs/design/underlyings.md#interface
Contracts: unchanged
Order: first.
Goal: the declarations, the contract test from both sides and the migration of `## Interface` exist; `internal/store/api.go` is unchanged.
Acceptance:
0. The grants are one statement per role in the style of `0025_settings.sql` (so the greps below count one line each).
1. `grep -c "CREATE TABLE settings.underlying" migrations/0033_underlying.sql` prints 1, `grep -c "rd.underlying_r1" migrations/0033_underlying.sql` prints at least 1, `grep -c "GRANT SELECT ON rd.underlying_r1 TO ot_ro_pages" migrations/0033_underlying.sql` prints 1, `grep -c "ot_ro_pages\|ot_runner" migrations/0033_underlying.sql` counts no line that grants on `settings.underlying` to either (`grep -c "ON settings.underlying TO ot_ro_pages\|ON settings.underlying TO ot_runner"` prints 0); `git diff --name-only $(git merge-base HEAD origin/main) -- migrations` lists only `migrations/0033_underlying.sql`.
2. `internal/store/underlying/api.go` is declarations only (no function body: `grep -c "^func " internal/store/underlying/api.go` prints 0) with `Row`, `Reader`, `Writer`, `Settler`, `Store`, `DSNEnvResolve`, `Change`, `Add`, `Edit`, `Outcome`, `InvalidError`, `OpenFunc` and the five status constants; `wc -l` of it is under 600; `git diff --name-only` does not list `internal/store/api.go`.
3. `api_test.go` has a fake that implements `Store` and a caller that uses only the interface (`grep -c "func Test" internal/store/underlying/api_test.go` prints at least 2), and a case `TestSettleOnlyPending`: a fake whose newest row is no longer pending or unresolved, or has another id than `AfterID`, makes `Settle` return `written=false` and write nothing, and the caller treats that as no error.
4. `setup.go` gains the role `ot_w_underlying_resolve` and the line `OT_PG_DSN_UNDERLYING_RESOLVE` in its role list (`grep -c "ot_w_underlying_resolve" internal/store/setup/setup.go` prints at least 1) and `setup_test.go` pins the pair as it does for `ot_w_settings`; `underlying_pg_test.go` (`pg` tag) shows the role can INSERT and SELECT `settings.underlying` and cannot INSERT into `settings.rule_param` or `settings.option_exclusion`.
5. `docs/contracts/store.md` names the table, the view, the two roles and the grants; the pull request body carries `Contracts: unchanged`.
Checked by CI: ci-go (`go build ./...`, `go test ./internal/store/...`, go-arch-lint, hygiene); under the `pg` tag the migration test of `internal/store/underlyingpg` (issue 2) runs the file; the design verdict (an interface change).

### 2 underlying-store-pg

Module: store. Size: L.
Files: internal/store/underlyingpg/open.go, internal/store/underlyingpg/read.go, internal/store/underlyingpg/write.go, internal/store/underlyingpg/validate.go, internal/store/underlyingpg/validate_test.go, internal/store/underlyingpg/store_test.go, internal/store/underlyingpg/store_pg_test.go
Design: docs/design/underlyings.md#decisions
Contracts: unchanged
Order: after 1.
Goal: `underlyingpg.Open` is an `underlying.OpenFunc` over `rd.underlying_r1` and `settings.underlying`; `Apply` validates and inserts, `Settle` is compare-and-set.
Acceptance:
1. `grep -c "func Open" internal/store/underlyingpg/open.go` prints 1; `grep -rn "pgx" internal/store/underlyingpg/validate.go` prints nothing (the validator is pure); `grep -c "FROM rd.underlying_r1" internal/store/underlyingpg/read.go` prints at least 1 and no file reads `FROM settings.underlying` except the writer's insert and its newest-row check through the view.
2. `validate_test.go` has one table case per rule of Decision 3 (symbol and market patterns, kind, issuer forms, empty issuer, price text with 5 places, sale below buy, `report_date_override` not a date, an edit of a "no" field, a duplicate `name_key`); each case names the refused field in `InvalidError.Field`.
3. `store_test.go` (a fake `pgx` boundary or a pure statement test, no database) shows `Apply` of an unchanged edit writes 0 rows, an add writes one, a status edit to `active` of a name with `ConID` 0 is refused, and a correction of `exchange` of an `unresolved` row inserts a `pending` row with `ConID` cleared.
4. `store_pg_test.go` carries `//go:build pg` and covers: newest-row-wins with ties by `id`, `Pending` returns pending rows and unresolved rows older than `retryBefore` only, `Settle` writes when the newest row has `AfterID` and does nothing when the owner has paused it in between, `ot_ro_pages` cannot `SELECT` the base table.
5. Every new file under 600 lines; each test calls `t.Parallel()`; no sleep.
Checked by CI: ci-go (`go test ./internal/store/underlyingpg/...`, race, lint, arch); GitHub CI `pg` job.

### 3 underlyings-page-schema (interface)

Module: contracts (the generated type under `web/src/generated/**` is a path of contracts). Size: M.
Files: contracts/schemas/page.underlyings.v1.json, web/src/generated/underlyings.v1.ts, contracts/fixtures/page.underlyings.v1.golden.json, contracts/fixtures/page.underlyings.v1.confirm.json, contracts/fixtures/page.underlyings.v1.readonly.json, contracts/fixtures/page.underlyings.v1.bad-version.json, contracts/fixtures/underlyings.follow.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/underlyings_v1_test.go, docs/contracts/page-schemas.md
Design: docs/design/underlyings.md#interface
Contracts: page.underlyings v1 (new)
Order: first.
Goal: the schema, the generated type `Underlyings`, four page fixtures and the follow table exist and agree.
Acceptance:
1. `contracts/schemas/page.underlyings.v1.json` has `"$id": "https://optdesk.invalid/schemas/page.underlyings.v1.json"`, `"const": "underlyings"` for `schema`, `"const": 1` for `schema_version`, `additionalProperties: false` on every object, the fields of the page paragraph of `## Interface`, `followed_by` items an enum of exactly `bars`, `dividends`, `options`, `fundamentals`, and `status` an enum of the five statuses; `wc -l` at most 300.
2. The schema `title` is `Underlyings`; `web/src/generated/underlyings.v1.ts` is the output of `node scripts/generate-types.mjs page.underlyings.v1` (run from `web/`): line 1 is the banner and it has `export interface Underlyings `; no `index.ts` is made (`TestGeneratedTypes` refuses one) and `web/package.json`, `web/scripts/generate-types.mjs` and `internal/contracts/generated_ts_test.go` are not in the diff (the script and the test read the directory).
3. The fixtures: `golden` has the six names pinned here (invented symbols `AAA` US active held with options and fundamentals on; `BBB` US paused; `CCC` US active with options off; `DDD` JP pending; `EEE` US unresolved with reason `not listed`; `FFF` US etf active, no issuer, fundamentals off), `confirm` has a pause of `AAA` with `stops` naming `bars` is not stopped (held) and `options` stopped, `readonly` has `form` null and one `no_reader`-class gap, `bad-version` is the golden with `schema_version` 2. `underlyings.follow.json` is a list of cases `{status, options, fundamentals, issuer_set, held, followed_by}` that covers every status times both switches times held (at least 24 cases) and is the one table Decision 6 names. `MANIFEST.synthetic-v1.json` gains the five paths with their sha256 and loses none; `grep -c "@"` of each fixture prints 0.
4. `TestUnderlyingsV1Fixtures` validates golden, confirm and readonly and shows bad-version fails at `schema_version`; inline mutations (unknown status, unknown `followed_by` item, extra top-level field) each fail.
5. `docs/contracts/page-schemas.md` has a table row `| page.underlyings.v1 | 1 |`; `contracts/schemas/carryover.sha256` is unchanged. The body carries `Contracts: page.underlyings v1 (new)`.
6. Each new test calls `t.Parallel()`, never sleeps; the diff holds only the listed paths.
Checked by CI: ci-go (`go test ./internal/contracts/...`); ci-web `tsc`, `eslint` and the `type drift` step (byte-for-byte generated type); the design verdict.

### 4 underlyings-builder

Module: pages. Size: M.
Files: internal/pages/underlyings/builder.go, internal/pages/underlyings/follow.go, internal/pages/underlyings/text.go, internal/pages/underlyings/page.go, internal/pages/underlyings/builder_test.go, internal/pages/underlyings/follow_test.go, internal/pages/underlyings/page_test.go, docs/contracts/pages.md
Design: docs/design/underlyings.md#decisions
Contracts: unchanged
Order: after 1, 3.
Goal: `Build(Input)` returns bytes equal to the golden (and confirm and readonly) fixtures as JSON; the rule of "followed by" is `Follows`.
Acceptance:
1. `Build` takes an `Input` struct (rows of `underlying.Reader.Current`, the held symbol set, the history rows, `Form`, `Confirm`, `Notice`, `Now`, the owner zone) and returns `([]byte, error)`; `grep -c "database/sql\|pgx\|net/http" internal/pages/underlyings/*.go` prints 0 (no reader of its own, no write).
2. `TestFollowTable` reads `contracts/fixtures/underlyings.follow.json` with `readFileSync`-style `os.ReadFile` and checks every case against `Follows`; `TestBuildGolden` compares with `reflect.DeepEqual` against `page.underlyings.v1.golden.json` and validates against the schema with the jsonschema library already in `go.mod`; `TestBuildConfirm` and `TestBuildReadOnly` do the same for their fixtures.
3. `text.go` holds every visible string in English (status texts, "resolves after the close", the stops lines, the gap text); `grep -c "func Test" internal/pages/underlyings/*_test.go` is at least 8.
4. `page.go` returns Meta `underlyings`, schema `page.underlyings.v1`; `git diff --name-only` does not list `internal/pages/api.go`; each file under 600 lines; every test `t.Parallel()`.
Checked by CI: ci-go (`go test ./internal/pages/...`, lint, arch).

### 5 underlying-resolve-job

Module: jobs. Size: M.
Files: internal/jobs/underlyingresolve/job.go, internal/jobs/underlyingresolve/types.go, internal/jobs/underlyingresolve/manifest.go, internal/jobs/underlyingresolve/job.toml, internal/jobs/underlyingresolve/job_test.go, internal/jobs/underlyingresolve/manifest_test.go
Design: docs/design/underlyings.md#decisions
Contracts: unchanged
Order: after 1.
Goal: `underlying_resolve` of Decision 7: pending names and unresolved names older than 7 days are resolved once and settled; at most 20 per run.
Acceptance:
1. `grep -c 'id = "underlying_resolve"' internal/jobs/underlyingresolve/job.toml` prints 1, `grep -c 'anchor = "session_close"' internal/jobs/underlyingresolve/job.toml` prints 1, `grep -c 'offset = "45m"' internal/jobs/underlyingresolve/job.toml` prints 1, `grep -c "retry_days = 7" internal/jobs/underlyingresolve/job.toml` prints 1, `grep -c "max_names = 20" internal/jobs/underlyingresolve/job.toml` prints 1.
2. `grep -rn "internal/jobs/" internal/jobs/underlyingresolve/*.go` prints nothing (no other jobs package); `grep -c "time.Sleep\|time.Now" internal/jobs/underlyingresolve/*.go` prints 0.
3. `job_test.go` with a fake broker, fake store and fake clock covers: a listed answer calls `Settle` with the `con_id`; a not-listed answer settles `unresolved` with a reason; a transport error settles nothing and the run result is retryable for that key; an `unresolved` row of 3 days is not asked and one of 8 days is; a 21st pending name is left for the next run; `Settle` returning `written=false` is counted as "changed by the owner" and is no error.
4. The constructor is `func New(b broker.Broker, s underlying.Settler, clk clock.Clock) (contracts.Job, error)` and a nil argument is an error.
5. Each file under 600 lines, every test `t.Parallel()`.
Checked by CI: ci-go (`go test ./internal/jobs/underlyingresolve/...`, race, lint, arch).

### 6 underlying-import

Module: ot. Size: S.
Files: cmd/ot/underlying_import.go, cmd/ot/underlying_import_test.go
Design: docs/design/underlyings.md#decisions
Contracts: unchanged
Order: after 2.
Goal: `ot underlying import <watch.toml>` of Decision 10.
Acceptance:
1. The command is registered in `commands` as `underlying` with the sub-command `import`; `grep -c "parseWatch" cmd/ot/underlying_import.go` prints at least 1 and `git diff --name-only` does not list `cmd/ot/watch.go`.
2. The test, over a fake `underlying.Store` and a file under `t.TempDir()`, shows: a file row with a non-zero `con_id` is added `active`, one with 0 `pending`, both `kind` `stock` with both switches on; a second import adds 0 and counts the rest as existing; a refused row (bad issuer) writes nothing and names the issuer, not a path; the output lists the pending names.
3. The command needs `OT_PG_DSN_SETTINGS` and names that line when it is missing; it never writes the file. `ot underlying import --check <watch.toml>` writes nothing, prints the number of `[[underlying]]` entries of the file and the number of names of the store whose `name_key` the file would give, and exits non-zero unless the two numbers are equal and every file name is in the store (the check of rollout step 4); a test shows equal and unequal cases.
4. Each file under 600 lines; `t.Parallel()`.
Checked by CI: ci-go.

### 7 jobs-issuerless-keys

Module: jobs. Size: S.
Files: internal/jobs/barsrefresh/job.go, internal/jobs/barsrefresh/plan_test.go, internal/jobs/optioncandidates/job.go, internal/jobs/optioncandidates/job_test.go
Design: docs/design/underlyings.md#corrections-to-the-proposal
Contracts: unchanged
Order: first.
Goal: a name without an issuer (an ETF) no longer refuses `bars_refresh` and is skipped by `option_candidates` with a named reason; keys of names with an issuer do not change.
Acceptance:
1. In `bars_refresh` the key of an entry is its `Issuer`, and for an empty issuer `sym:` plus the symbol; a test builds the job with one issuer-less entry and one with an issuer and shows both are planned and the issuer entry's key is unchanged; two issuer-less entries with one symbol are still a duplicate error.
2. In `option_candidates`, `split` puts an entry with an empty `Issuer` into the skipped list with the text `entry <symbol>: no issuer, no candidates`; a test shows it is not planned and the other entries are.
3. `git diff --name-only` lists only the four files; no signature of an exported function changes (`grep` of the diff for `^-func [A-Z]` prints nothing).
Checked by CI: ci-go (`go test ./internal/jobs/barsrefresh/... ./internal/jobs/optioncandidates/...`).

### 8 ot-underlyings-route

Module: ot. Size: L.
Files: cmd/ot/underlyings_site.go, cmd/ot/underlyings_post.go, cmd/ot/underlyings_site_test.go, cmd/ot/underlyings_post_test.go, cmd/ot/site.go, cmd/ot/check_serve.go, cmd/ot/check_blocks.go, cmd/ot/check_blocks.json, docs/contracts/ot/pages/underlyings_page.md
Design: docs/design/underlyings.md#decisions
Contracts: unchanged
Order: after 2, 4.
Goal: `GET /underlyings` serves the page (read-only without `--owner-edit`), `POST /owner-underlyings` applies a change in two steps; the same guards as `/owner-settings`.
Acceptance:
1. `grep -c "guardOwnerPost" cmd/ot/underlyings_post.go` prints at least 1 and `grep -c "func guardOwnerPost" cmd/ot/underlyings_post.go` prints 0 (reused, not copied); `git diff` of `cmd/ot/settings.go` is empty; the page is registered with `registerPage` (route `underlyings`).
2. Tests under `httptest` with a fake store: without `--owner-edit` the route answers 404 for POST and the page has `form` null; a first post writes nothing and answers the confirm page whose `stops` name the groups that stop for a pause (and not `bars` for a held name); a second post applies and answers 303 to `/underlyings?saved=<n>`; a refused value is 422 with the field named; a bad token, a foreign `Host` or `Origin` is 403 with no page; a store error is 500 with a fixed text and a log line that carries no value.
3. `ot serve` refuses to start with `--owner-edit` when `OT_PG_DSN_SETTINGS` is empty only for the new route, as for `/owner-settings` (no new flag); `site.go` gains only the reader field and the route name.
4. `check_serve.go`, `check_blocks.go` and `check_blocks.json` gain the `underlyings` row as `settings_page` has one (a registered page is listed there).
5. `docs/contracts/ot/pages/underlyings_page.md` describes the route, the guards and the step texts; each file under 600 lines; `t.Parallel()`.
Checked by CI: ci-go.

### 9a ot-listed-job-decorator

Module: ot. Size: M. Not wired: the code is used by its tests only, nothing in the wiring calls it, so main stays green and behaviour does not change.
Files: cmd/ot/listedjob.go, cmd/ot/listedjob_test.go, cmd/ot/followfilter.go, cmd/ot/followfilter_test.go
Design: docs/design/underlyings.md#decisions
Contracts: unchanged
Order: after 1, 3.
Goal: the decorator `listedJob` and the filter of Decisions 6 and 9 exist and are tested.
Acceptance:
1. `followfilter_test.go` reads `contracts/fixtures/underlyings.follow.json` and checks every case against the filter (the table `pages/underlyings` also reads), and shows the `WatchEntry` fields come from the row as Decision 3 says (`SecType` `STK` for stock and etf, `ConID`, `ReportDateOverride`, `BuyPrice`, `SalePrice`; an empty `Issuer` stays empty).
2. `listedjob_test.go` (fake jobs, fake reader, fake clock, no sleep) shows: `Plan` builds the inner job from the list read at that call, so a name added or paused between two plans changes the second plan with no restart; the in-flight rule of Decision 9 (a run started on plan 1 still uses plan 1's inner job after plan 2 of the same day with another list; `Run` with keys of no snapshot fails permanent; a new day drops the old snapshots); `Schedule`, `RunBound` and `RestartSensitive` are forwarded; an empty store (no `active` name) returns a permanent error `underlying list is empty`; a read error returns a retryable error and no keys and never an older list; a family whose group is off for every name plans zero keys with no error.
3. Every test calls `t.Parallel()` and none sleeps; each file under 600 lines; `golangci-lint` passes (the `unused` check counts the tests as users).
Checked by CI: ci-go (`go test ./cmd/ot/...`, race, lint, arch).

### 9b ot-listed-jobs-wiring

Module: ot. Size: L.
Files: cmd/ot/wire_underlying.go, cmd/ot/wire_underlying_test.go, cmd/ot/wire.go, cmd/ot/runonce.go, cmd/ot/runonce_test.go, cmd/ot/runonce_option_test.go, cmd/ot/candidates.go, cmd/ot/candidates_test.go, cmd/ot/wire_earnings.go, cmd/ot/wire_earnings_test.go, cmd/ot/wire_fundamentals.go, cmd/ot/wire_fundamentals_test.go, cmd/ot/wire_ivhistory.go, cmd/ot/wire_ivhistory_test.go, cmd/ot/wire_dividend.go, cmd/ot/wire_dividend_test.go, cmd/ot/wire_optioneod.go, cmd/ot/jp_wire_test.go, cmd/ot/jp_stock_wire_test.go, cmd/ot/wire_optionopen.go, cmd/ot/wire_optionopen_test.go, cmd/ot/wire_computefigures.go, cmd/ot/wire_computefigures_test.go, cmd/ot/wire_test.go, cmd/ot/companies_site.go, cmd/ot/companies_site_test.go, cmd/ot/home_site.go, cmd/ot/home_site_test.go, docs/contracts/ot/jobs/underlying_resolve.md
Design: docs/design/underlyings.md#rollout
Contracts: unchanged
Order: after 2, 5, 6, 7, 9a; merges only at step 5 of `## Rollout`.
Goal: every family that took `f.watch` takes its group's entries from the store at each `Plan` through `listedJob`; `underlying_resolve` is a registered family; the J-Quants symbols follow the list; the companies and home pages read the store; the wiring no longer reads `watch.toml`.
Acceptance:
1. `grep -n "f\.watch\|jc\.f\.watch\|cfg\.watch" cmd/ot/*.go` (non-test files) prints nothing; the field `watch` of `jobFactory` and `runOnceConfig` is replaced by the list reader; `dryrun.go` and `loadConfig` keep `loadWatch` until issue 10 (so the tree compiles), and nothing else calls it.
2. `wire_underlying.go` registers the family `underlying_resolve` (job id, order a free step of ten) with `underlying.DSNEnvResolve`; an empty `OT_PG_DSN_UNDERLYING_RESOLVE` leaves only this job out, with a logged reason. The list itself is read through the `ot_runner` view grant (`OT_PG_DSN_RUNNER`), so no other job needs a new line. A test shows each wired family is a `listedJob` and that `option_candidates` still has its settings version.
3. A test over a fake store shows `earnings_refresh` builds its J-Quants source from the JP names that pass the filter at that `Plan` (a paused JP name absent, a newly added one present).
4. The companies and home pages read the store rows (status, issuer, symbol, prices) and show the same names they showed from the file for the same data; their existing tests are updated to a fake store.
5. `docs/contracts/ot/jobs/underlying_resolve.md` exists with an `Order: <n>` line (the same free step) and a `## Version` heading, as `TestJobDocsExist` requires.
6. An empty store makes every family's `Plan` fail with `underlying list is empty` (the test of 9a, now through the wiring). The rollout of `## Rollout` is stated in the pull request body.
Checked by CI: ci-go (`go test ./cmd/ot/...`, race, lint, arch).

### 10 ot-watch-cutover

Module: ot. Size: S.
Files: cmd/ot/watch.go, cmd/ot/watch_test.go, cmd/ot/dryrun.go, cmd/ot/runonce.go, docs/contracts/ot.md
Design: docs/design/underlyings.md#decisions
Contracts: unchanged
Order: after 6, 8, 9b (the rollout has finished).
Goal: nothing reads `watch.toml` at run time.
Acceptance:
1. `grep -c "func loadWatch\|watchFileName" cmd/ot/watch.go` prints 0 and `grep -rn "loadWatch(" cmd/ot` prints nothing; `parseWatch` and `watchRow` remain for the import.
2. `dryrun.go` reads its list through the store reader of issue 9b and the dry run still opens no store for writes.
3. `docs/contracts/ot.md` says the list is the store, edited on `/underlyings`, and names the import as the one use of the file.
4. Each file under 800 lines (`runonce.go` and `dryrun.go` shrink).
Checked by CI: ci-go.

### 11 web-underlyings-view

Module: web. Size: M.
Files: web/src/pages/underlyings/view.tsx, web/src/types/underlyings.ts, web/test/pages/underlyings/view.test.tsx
Design: docs/design/underlyings.md#decisions
Contracts: unchanged
Order: after 3.
Goal: a payload of `page.underlyings.v1` shows the table, the add form, per-row edit forms, the confirm step and the history, every string read from the payload.
UI parts (frontend-stack, ADR 0034):
- 按 frontend-stack：用 shadcn `Card`, `CardHeader`, `CardTitle` 做 the page frame.
- 按 frontend-stack：用 tanstack table (`web/src/components/data-table/data-table.tsx`) 做 the list of names with the columns name, kind, status, options, fundamentals, held, buy, sale, followed by and note.
- 按 frontend-stack：用 shadcn `Badge` 做 the status and the `followed_by` groups, `Alert` 做 the gaps, the notice and the `stops` lines of the confirm step.
- 按 frontend-stack：用 shadcn `Input`, `Label`, `Switch` and `Button` (already copied for `/settings`, no new package) 做 the add form and the row edit forms as a plain `<form method="post" action="/owner-underlyings">` with hidden `expires` and `mac`, no `fetch`.
- 按 frontend-stack：用 vitest and testing-library 做 the tests; zustand is not used.
Acceptance:
1. `web/src/types/underlyings.ts` has `SUPPORTED_VERSIONS = [1]`, `export type Payload = Underlyings` from `@/generated/underlyings.v1` and `validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)`.
2. `view.tsx` exports `registration = { schema: 'underlyings', page: page<Payload>(validate, UnderlyingsView) }`; no file of `web/src/shell/` is in the diff.
3. The view shows `title`, `sentence`, each row's fields and `followed_text`, the edit control only for the fields in `editable`, the form only when `form` is not null, the confirm table and `stops`, and the history; it formats nothing (`grep -n "toFixed\|Intl\.\|toLocale" web/src/pages/underlyings/view.tsx` prints nothing).
4. `view.test.tsx` reads the four page fixtures of `contracts/fixtures` with `readFileSync`, checks each expected string is read from the fixture, that a `pending` row shows its status text, that a read-only payload renders no `<form>`, and that `validate` refuses `bad-version` with `unsupported_version`; it stubs `ResizeObserver`, opens no port, sleeps never.
5. `web/package.json` is not in the diff; each file under 600 lines; no file outside the three is in the diff.
Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck`, `lint`, `test` and `build`; hygiene.

## Left out

- Tiers, cadence per name, per-job toggles, chain depth per name, a cap on the number of names, pools, the J-Quants back-to-back bug, adding a name automatically from positions (a held name that is not on the list is followed for bars and dividends only through `held` of an existing row; an unlisted held name is not added).
- Indices and FRED series (Decision 4): they are not underlyings. A small list of their own is a later cut: the four indices would move from `barsrefresh/job.toml` to a store list so their `con_id` is stored once and saves four `ResolveContract` calls per `bars_refresh` run; not done because the cost is four calls, they never change, and the owner does not manage them. FRED series stay in the manifests; the owner has not asked to manage them.
- A button "resolve now": `ot serve` has no broker (settings-form Decision 5) and the runner has no control socket; a pending name resolves after the close.
- Candidates for issuer-less names (`option_candidates` skips them, issue 7): the candidate rows and the datasets need an issuer; a key for them is a design of the candidates module.
- Removing `parseWatch` and the import command: kept until the owner says the import is done everywhere.
- Moving the instrument spec (calendar, MIC, increment) into the table: it is not an owner attribute and the jobs create `master.instrument` with their own spec (Decision 3, gap c).
- A page for `option_exclusion` (kept as it is, Decision 11) and a merge of it with the `options` switch.
- Editing `quote_snapshot`'s instruments: it keeps its own manifest and does not follow the list.
- Deleting a version: the table is insert-only; a drop is status `retired`.

## Risks

- A wrong click stops collection: the confirm step lists what stops, the held rule keeps `bars` and `dividends` of a held name, and a pause is one more row (a revert is "use this version", through the same confirm).
- The decorator hides the job type from code that type-asserts it (`WithSettingsVersion`): issue 9a applies it inside the build function and its test pins it.
- A read error at `Plan` stops a family for that run: chosen over running with a stale list; the runner retries.
- A name added on the page waits for the evening resolve; the page says so.
- The scheduler process holds a writer DSN for `underlying_resolve`: its own role with INSERT and SELECT on the one table (Decision 7); the owner's word is one new env line, `OT_PG_DSN_UNDERLYING_RESOLVE`.
- The serve process holds one more writable table on the settings DSN (same loopback exception).
- The rule of Decision 6 is written twice; the shared table `underlyings.follow.json` is the check.

## Adversarial

A fresh read-only sub-agent read this file against origin/main f2b5473b (main moved since e9e77892; no path of this design changed). Its answer is saved as `underlyings/adversarial.md` (the drafting scratchpad); the pull request description copies the six points.

1. Gap against the goal: adopted in part. A pending name older than 36 hours shows the status text "the resolve job has not settled this name; check ot scheduler" (a builder rule, `text.go`, one fixture case), which covers a job left out for an empty `OT_PG_DSN_SETTINGS`. Not adopted: editing `symbol` or `market` of an unresolved name; drop and re-add is one confirm action and keeps the key meaning clean. Not adopted: resolving at add time (serve has no broker). The 4 index calls stay and are named in Left out.
2. Biggest unnamed risk: adopted. Issue 9b has the rollout steps (migrate, import, check, then merge) and an empty store fails visibly. The writer DSN in the scheduler is now its own role with INSERT and SELECT on the one table (the advisor round below), so the owner's word is one new env line; the alternative, a resolve command run by hand, is not wanted.
3. Simpler way: not adopted. `kind`, `exchange` and `currency` are the contract request and cannot be dropped (the file carried them and the resolve job needs them); `reason` is what the owner sees for a failure; the 7-day retry is one date comparison, no table. Skipping the resolve job keeps today's per-run calls, which the owner said not to waste. The decorator stays.
4. Lock-in: not adopted as a change, accepted as stated cost. `name_key` is the owner's name and is not an instrument identity; the join to master is by `con_id` after the first job run, which is why `con_id` sits in both places (one writer each, same value). The two-places rule of the follow filter is guarded by the shared table `underlyings.follow.json`. Statuses and groups are frozen by the page schema v1; a new group is a v2.
5. Claim most likely false: adopted. `wire.go:521` does use `f.watch`; issue 9b now lists `cmd/ot/wire.go` (one `f.watch` use replaced). The field declaration is in `runonce.go`. Corrections 9 and Decision text that said wire.go is untouched are fixed in issue 9.
6. Likeliest slip: adopted. `check_serve.go`, `check_blocks.go` and `check_blocks.json` are in issue 8's Files; issue 9b's job document has the `Order:` line and `## Version` heading that `TestJobDocsExist` needs; issue 9 states that `dryrun.go` and `loadConfig` keep `loadWatch` until issue 10 so the tree compiles; issue 1 has item 0 (one grant statement per role in the style of 0025). Issue 9 stays one L entry with 19 files; splitting it would put two sources of the list into production between the halves, which Decision 10 forbids.

Advisor round (2026-11-11), all adopted in this revision: (1) an empty store fails every `Plan` visibly (Decision 9), the page refuses a save that leaves no active name (Decision 5), and `## Rollout` has numbered steps with their owners and a count check (`import --check`); (2) the resolve job has its own role `ot_w_underlying_resolve` and line `OT_PG_DSN_UNDERLYING_RESOLVE`, holds `underlying.Settler`, and the pending-only rule has a contract test (issue 1); (3) issue 9 is cut into 9a (decorator and filter, not wired) and 9b (wiring), the in-flight snapshot rule and its test are in Decision 9, and the line-count and "no other file" items are dropped; (4) the other designs that cite `watch.toml` carry one line that this design replaces it from issue 10; (5) Decision 8 counts only the savings on main; (6) Decision 6 has the per-job table; (7) Decision 3 (d) says which copy of `con_id` the jobs read and what a renamed symbol is; (8) issue 9b lists the same-package tests; (9) the confirm `stops` lines say missed days cannot be refilled.
