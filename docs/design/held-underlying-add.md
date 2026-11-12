# Held underlying add: a held stock with no row on the list gets a pending row, or a named state on the page

Author: window (draft for advisor review)
Revision: 5
Variant: (b) of the advisor decision of 2026-11-12, with `Add` on its own one-method interface (not on `Settler`,
not through `Apply` from a wiring file; variant (a) is rejected). Contracts: unchanged.

## Problem

`listedJob.entries` loops over the stored rows only (`cmd/ot/listedjob.go:75-93`), so a stock the owner holds with no
row is never reached: no option chain, no EOD quote, no iv history, and no line says so. The held names come from the
newest positions of `heldAliases` (`cmd/ot/wire_underlying.go:41`) read by `heldSets` (`cmd/ot/underlyings_site.go:97-117`)
through `store.AccountReader.Positions` (`internal/store/api.go:784`); the rows are written by the `account_capture` job
(`internal/jobs/accountcapture`, wired at `cmd/ot/wire.go:41,251`) and by `cmd/ot/accountimport.go`, not by a Flex import.
A position carries `Symbol` and `Currency`, no market (`internal/store/api.go:708-722`). The role
`ot_w_underlying_resolve` already has INSERT and SELECT on `settings.underlying` (`migrations/0033_underlying.sql:52-55`),
so a row can be added through the resolve store with no migration.

## Decisions

1. New interface `underlying.HeldAdder` with two methods: the pure `Check(symbol, currency string) (key string, err error)`
   and `Add(ctx, symbol, currency string) (added bool, err error)`; both return an `InvalidError` (`api.go:82-91`, reason
   in `Why()`) for a refused name, so the reason travels typed and nothing outside the interface is called.
   `Settler`, `Writer`, `Store` and `Apply` keep their methods, so no fake of `Store` or `Settler` stops compiling.
   `underlyingpg.Store` implements it. In `cmd/ot` the store the resolve job opens (`cmd/ot/wire_underlying.go:61-66`)
   must satisfy it when the held-name wrapper is wanted (an account line is set): a store that does not is a start-up
   error ("underlying store cannot add held names"), never an unwrapped job without a word.
2. `Add` writes one pending row: kind stock, `Options` true, `Fundamentals` false, no issuer, `ConID` 0, exchange and
   currency the defaults of the market, `Note` = `underlying.NoteHeld`. It writes nothing and returns false when a row
   of the symbol exists in any status under either market, a retired one included, so it never resurrects a stopped
   name. Check and insert run under the table lock (`internal/store/underlyingpg/write.go:12-16,43`).
3. Market comes from the currency, once, in `underlyingpg`: `ValidateAdd` holds market to exchange and currency as two
   map literals (`internal/store/underlyingpg/validate.go:57-62`); one map `marketOfCurrency` (USD US, JPY JP) is added
   beside them. `Check` applies it and the symbol rules (`validate.go:44-54`); `Add` calls `Check`, and so do the wrapper
   and the page route, through the interface. Not master.instrument.calendar_id
   (`migrations/0008_master_instrument.sql:13`): a name with no row has no instrument. Not `rd.underlying_r1.market`:
   that is the row being made.
4. Origin: no column exists (`migrations/0033_underlying.sql:11-31`); the origin is the `Note` value `NoteHeld` ("Added:
   held position; check the con_id"; `validate.go:27` allows 200). The note column is on the page (`view.tsx:85`), and the
   row shows its con_id. The broker's resolve answer carries no name (`broker.Resolved`, `internal/broker/api.go:88-93`;
   `ContractSpec`, `contracts/proto/ibkr/v1/contract.proto:25-39`), so a resolved name cannot be shown; the note tells the
   owner to check the con_id, which is how a wrong match (decision 8) becomes visible. `Settle` copies the row (`write.go:112-121`), so it survives the
   resolve. An owner who overwrites the note takes the row over; code that reads the origin from `Note` is fragile.
5. Stopping. The row persists; no job removes or retires it. On the page the owner sets retired, paused or options off.
   For a held option the chain does not stop: `followedBy` keeps `options` while an option on the name is held
   (`r.Options || heldOption`, `cmd/ot/followfilter.go:28,41`), whatever the switch or status. For shares only: options
   off, paused or retired stops the chain (`followfilter.go:26-44`). `Add` skips a retired key.
6. Sold position: the row stays active with options on until the owner retires it. The design does not retire its own
   `NoteHeld` rows: the resolve role has no edit method, the owner may have taken the row over, and a silent retire is the
   silence this design removes. Named in Left out.
7. Wrapper `heldRows` in `cmd/ot` around the resolve job, `Plan` pure, the write in `Run`:
   - `Plan` calls the inner `Plan`, then, for today only, reads (no write) the newest positions of `heldAliases` (lookback
     `cmd/ot/companies_site.go:89`) and `Current`; for each distinct (symbol, currency) of a position whose stored `SecType`
     is `stock` or `option` (the words of `migrations/0019_account.sql:22`, written as literals like the sec_type fix
     does and moved to the `store` constants when they exist; `Depends on: #SECTYPE`; not the broker words `STK`/`OPT`;
     the tests use the stored words) with nonzero quantity
     (`companies_site.go:123`) that is not an index option (decision 8), has no row of the symbol and passes `Check`, it appends one expected key (`market:SYMBOL`, the inner dataset id), held keys first, at most 20
     (`max_names` of `job.toml`). The hash is the inner hash joined with the held keys.
     When the day's plan gains a key the reconciler takes it: every pass re-plans, merges the keys and, when the hash
     differs from the stored one, puts the day's expected keys again (`internal/runner/reconcile/reconcile.go:151-189,238-244`).
     A `Plan` error would make it skip the whole job (`reconcile.go:160-166`), so when the position read fails `Plan`
     returns the inner keys and hash unchanged (pending names still planned), logs one `Warn` with the job id and no text, and
     the page names the failure (decision 9). Other callers of `Plan` on the registered job are backfill
     (`internal/runner/reconcile/backfill.go:143`), the job console (`cmd/ot/jobconsole_turn.go:170`) and run-once
     (`cmd/ot/runoutcome.go:30`); they read the positions too, and a past day must not add today's names, so `Plan` plans held
     keys only when `day` is today in the job's zone (`America/New_York`, `job.toml`) and returns the inner keys alone otherwise.
     Marks: an added or resolved name is delivered; a store error is a Retryable result and the name is an inner pending key
     on the next plan; a name sold since the plan returns nothing (no value); a refused or index name has no key.
   - The runner calls `Run` once per key (`internal/runner/reconcile/runkey.go:100`), so a `Run`-side write needs a key:
     the held key is it. `Run` on a key with no row calls `Add` for its pair (read again from the positions; a name sold
     since the plan does nothing), then the inner `Run`, whose `due` read now holds the new row and resolves it in the
     same run (`internal/jobs/underlyingresolve/job.go:103,121`). One `Add` per run; a store error is a Retryable result.
   - The wrapper forwards `ID` and the optional methods `Schedule`, `RunBound`, `RestartSensitive`
     (`job.go:52-66`) by type assertion, as `listedJob` does (`cmd/ot/listedjob.go:56`). `dryrun.go:165` builds its own job and is not wrapped.
   - The resolve process gains a second DSN, `cfg.accountDSN`, to read positions, opened as `heldKeys` does
     (`wire_underlying.go:96-109`), beside `resolveDSN`. Without an account line the job is not wrapped. `heldSets` gives no
     currency, so the wrapper has its own reader of `AccountReader.Positions` (`internal/store/api.go:784`), written by
     account_capture (`cmd/ot/wire.go:41,251`) and `cmd/ot/accountimport.go`.
8. Index options. The position data has no underlying type: `store.Position` holds the position's own `SecType`
   (`stock`, `index`, `option`) and the option's `ConID` (`internal/store/api.go:708-724`; `internal/jobs/accountcapture/job.go:236-268`
   maps only the position's own word), so "index" cannot be read from an option position. The rule is one table of data, the
   `[[index]]` tables of `internal/jobs/barsrefresh/job.toml:13-37` (SPX, NDX, RUT, N225), read by `cmd/ot` through
   `barsrefresh.DefaultManifest().Indexes` (`internal/jobs/barsrefresh/manifest.go:46,85`), no symbol in code. Such a position of
   sec_type `option` is never planned or added. The page shows an index of currency JPY as "index option, covered by option_eod_jp"
   (that job lists N225 only, `internal/jobs/optioneod/job_jp.toml:24-25`; id `jobOptionEodJP`, `cmd/ot/preflight.go:21`) and any
   other as "not supported: index option" (`optioneod/job.toml` lists no index). Not covered: an index outside the set, say XSP or VIX, is added as a
   stock; the resolve asks for `STK` on SMART (`job.go:133-139`), so a stock with the same ticker would resolve to an
   unrelated con_id. A fix needs the underlying type in the position
   contract (`contracts/proto`), out of scope.
9. Page: every held name (shares, `Held`; option, new `HeldOptions`) not yet followed is a named gap `no_data`
   (`contracts/schemas/common.v2.json:15`; `view.tsx:224` prints every gap), sorted by symbol: no row ("X: held, not on the
   list"); a pending row ("X: pending; the first option chain comes at least one session day after the resolve"); an
   unresolved row ("X: unresolved, no option chain: <reason>"); and a refused name with its own text and reason from a new
   `Input.HeldRefused map[string]string` that `cmd/ot` fills with the `Why` of `Check` ("currency GBP has no market",
   "symbol does not match the pattern of market JP") or the index text of decision 8. Gone when the row is active. When
   the positions cannot be read the page builds anyway (today it fails, `cmd/ot/underlyings_site.go:76-78`) with one gap
   `no_data` "Held names could not be read from the positions; held names may be missing here" (`Input.HeldError`). A held-added row (`Note` `NoteHeld`, con_id above 0) shows "Resolved as US stock <SYM>, con_id N; check the con_id" in its note column. The page route reaches `Check` through `HeldAdder` on its store; a store without it is a start-up error.
10. Pending order: one term, `ORDER BY (note = $2) DESC, name_key` in `pendingSQL`
    (`internal/store/underlyingpg/read.go:22-23`), so held rows are not starved by the cut at `max_names` 20 (`job.go:77`).
    A row whose `ResolveContract` errors stays first each day; one the broker does not list is settled unresolved and
    leaves the front for `retry_days` 7 (`job.toml`).
11. A JP name bought on day D: the page shows "held, not on the list" from the capture of D. The job runs on XNYS days
    only, 45 minutes after the close (`job.toml`): on D if D is an XNYS day, else on the next one, and there it is added
    and resolved in one run; the page then shows active. The first JP chain is at the first JP open after that. So the
    first day shows the named gap, never a chain; a US holiday adds a day.

## Readers of the new row (Options true, also for shares only)

- Options true for shares only is kept: the owner sells options and held shares need a chain for covered calls; the
  covered-call rows exist only for an alias with shares (`internal/compute/optioncand/call.go:38`) and are listed by
  `internal/jobs/optioncandidates/calls.go:158`. Wanted.
- Candidates: the short-put rows of a `NoteHeld` row are blocked `no_buy_price` (empty buy price,
  `internal/compute/optioncand/put.go:18-21`), so they are listed but not ranked as a buy, while the owner leaves the buy price empty (the page edits it, `builder.go:24`). Wanted.
- Groups (`followfilter.go:22-46`): pending follows nothing; active `options` feeds option_candidates
  (`cmd/ot/candidates.go:77`), option_eod and JP (`cmd/ot/wire_optioneod.go:83,110`), iv_history
  (`cmd/ot/wire_ivhistory.go:63`), option_open resolve, plan and pull (`cmd/ot/wire_optionopen.go:100,145`). Wanted.
- option_list_preopen (`internal/jobs/optionopen/list.go:118-120`): not wired in `cmd/ot` on main; the new US row joins when it is.
- `jpListed` (`cmd/ot/candidates.go:165-182`): an active JP `NoteHeld` row is listed. Wanted: its JP candidates.
- `companies_site` (`cmd/ot/companies_site.go:47-60`): skips a row with no issuer, so the row does not appear there. Wanted.
- The import (`cmd/ot/underlying_import.go:69-75`) skips a key that exists: a file entry for a held name never reaches the
  row, so its issuer and con_id are not applied. Not fixed here (import is a separate path); the owner sets the issuer on the
  page (`editableBase`, `builder.go:24`) and the resolve gives the con_id. Named in Left out.
- Dividends and fundamentals: issuer empty, `Fundamentals` false: neither follows (`followfilter.go:25,31`).

## Modules

none. `internal/store/**`, `internal/pages/**` and `cmd/**` are existing rows (`docs/design/modules.md:90,95,101`); no
row, path, entry cell or depends cell changes; `ot` already depends on store and pages.

## Interface

Owning module: store. Interface file `internal/store/underlying/api.go`; contract test
`internal/store/underlying/api_test.go`. Added beside `Settler`:

    // HeldAdder is held by the held-name wrapper of cmd/ot over the resolve store (role ot_w_underlying_resolve).
    type HeldAdder interface {
        // Check applies the rules of Add without writing and returns the key (market:SYMBOL), or an InvalidError
        // whose Why says the reason (currency with no market, symbol refused).
        Check(symbol, currency string) (key string, err error)
        // Add writes one pending stock row (Options true, Note NoteHeld) for symbol in the market of currency. It writes
        // nothing and returns added false when a row of the symbol exists in any status under either market. It returns
        // an InvalidError for a currency with no market or a symbol the market refuses.
        Add(ctx context.Context, symbol, currency string) (added bool, err error)
    }

and the constant `NoteHeld = "Added: held position; check the con_id"`. The comment of `Settler.Pending` adds: rows with `Note` `NoteHeld`
come first. The contract test pins the written fields (status pending, kind stock, Options true, ConID 0, Note NoteHeld,
Fundamentals false, Issuer empty), added true then false, false after a retired edit, an InvalidError for GBP, and the
held-first order of `Pending`.

## Issues

1. (interface) store: `HeldAdder` (`Check`, `Add`), `NoteHeld`, held-first `Pending` and the `underlyingpg` implementation. Depends on: none.
   Design: docs/design/held-underlying-add.md#interface
   Files: internal/store/underlying/api.go, internal/store/underlying/api_test.go, internal/store/underlyingpg/write.go, internal/store/underlyingpg/validate.go, internal/store/underlyingpg/read.go, internal/store/underlyingpg/store_test.go, internal/store/underlyingpg/store_pg_test.go, internal/store/underlyingpg/validate_test.go
   1. `git grep -n "type HeldAdder interface" -- internal/store/underlying/api.go` prints one line, and `type Settler interface` lists the same three methods as before.
   2. `git grep -n "NoteHeld" -- internal/store/underlying/api.go` prints the constant, whose text contains "check the con_id", and `TestHeldAdderContract` pins that the written note is `NoteHeld`; the file has no `func` line.
   3. `api_test.go` contains `func TestHeldAdderContract` (the written fields, added true then false, retired false, `Check` and `Add` return an InvalidError with `Why()` for GBP) and `func TestPendingHeldFirst`.
   4. `git grep -n "func (s \*Store) Add(\|func (s \*Store) Check(\|marketOfCurrency" -- internal/store/underlyingpg` prints the two methods and the map holding USD and JPY only; `git grep -n "func [A-Z]" -- internal/store/underlyingpg/validate.go` lists no new exported function.
   5. `git grep -n "note = " -- internal/store/underlyingpg/read.go` prints the one ORDER BY term of `pendingSQL`.
   6. `git grep -n "func TestStoreCheck\|func TestAddWritesPendingHeldRow\|func TestAddSkipsExistingKey\|func TestPendingHeldFirstSQL" -- internal/store/underlyingpg` prints four lines; `store_pg_test.go` carries `//go:build pg` and `func TestAddThroughResolveRole`.
   7. No migration is part of the entry: its Files line holds none and `Add` uses the INSERT grant of `migrations/0033_underlying.sql:53`.
   8. `git grep -n "var _ underlying.HeldAdder" -- internal/store/underlyingpg` prints the compile-time assertion.
2. pages: the named states of a held name that is not followed. Depends on: none.
   Design: docs/design/held-underlying-add.md#decisions
   Files: internal/pages/underlyings/builder.go, internal/pages/underlyings/text.go, internal/pages/underlyings/builder_test.go
   1. `git grep -n "HeldOptions\|HeldRefused" -- internal/pages/underlyings/builder.go` prints both `Input` fields (`map[string]bool`, `map[string]string`).
   2. `git grep -n "no_data" -- internal/pages/underlyings/builder.go` prints the constant and the append; `text.go` holds four texts (no row, pending, unresolved, refused with the given reason), each built to at most 200 characters, the pending text containing "at least one session day".
   3. A row whose `Note` is `underlying.NoteHeld` and whose `ConID` is above 0 shows the note "Resolved as US stock <SYM>, con_id N; check the con_id" (JP likewise, market from the row); `git grep -n "Resolved as" -- internal/pages/underlyings/text.go` prints it and `builder_test.go` contains `func TestHeldAddedRowShowsResolvedText`. The exchange is not shown: `Outcome` has no exchange and `Settle` keeps only the con_id (`write.go:112-121`).
   4. `builder_test.go` contains `func TestHeldNameWithoutRowIsAGap`, `func TestHeldPendingRowIsAGap`, `func TestHeldUnresolvedRowIsAGap`, `func TestHeldRefusedNameShowsItsReason`, `func TestHeldActiveRowGivesNoGap`; the first also covers a name only in `HeldOptions`.
   5. `git grep -n "HeldError" -- internal/pages/underlyings/builder.go` prints the field; `builder_test.go` also contains `func TestHeldReadFailureIsAGap`.
   6. The page schema is unchanged: every new gap uses the reason `no_data` of `common.v2.json:15`, and no file under `contracts/` or `web/` is in Files.
3. ot: the wrapper, the index set, the start-up error and the page inputs. Depends on: 1, 2, #SECTYPE (stored words `stock`/`option`/`index` read from positions).
   Design: docs/design/held-underlying-add.md#decisions
   Files: cmd/ot/held_underlying_rows.go, cmd/ot/held_underlying_rows_test.go, cmd/ot/wire_underlying.go, cmd/ot/wire_underlying_test.go, cmd/ot/underlyings_site.go, cmd/ot/underlyings_site_test.go
   1. `git grep -n "underlying.HeldAdder" -- cmd/ot/wire_underlying.go` prints the assertion, followed by a returned error when it fails; `git grep -n "heldRows(" -- cmd/ot/wire_underlying.go` prints the wrap, guarded by `cfg.accountDSN`.
   2. `git grep -n "func (.*) Schedule(\|func (.*) RunBound(\|func (.*) RestartSensitive(" -- cmd/ot/held_underlying_rows.go` prints three lines, each calling the inner job by type assertion.
   3. `git grep -n "\.Add(" -- cmd/ot/held_underlying_rows.go` prints one line and it lies inside `Run`; `git grep -n "func (.*) Plan(" -A12 -- cmd/ot/held_underlying_rows.go` shows no call of `Add`; `git grep -n "underlyingresolve" -- cmd/ot/held_underlying_rows.go` prints nothing.
   4. `git grep -n "barsrefresh.DefaultManifest\|\.Indexes" -- cmd/ot/held_underlying_rows.go` prints the one read of the index table, `git grep -n "SPX\|NDX\|RUT\|N225" -- cmd/ot` prints nothing outside test files, and `git grep -n "option_eod_jp\|not supported: index option" -- cmd/ot/held_underlying_rows.go` prints both texts.
   5. `held_underlying_rows_test.go` contains `func TestHeldPlanAddsKeyWithoutWriting`, `func TestHeldRunAddsThenRunsInner`, `func TestHeldRunSoldNameDoesNothing`, `func TestHeldPlanSkipsIndexAndRefused`, `func TestHeldRowForwardsOptionals`, `func TestHeldPlanPastDayPlansInnerOnly` (a past day returns the inner keys alone), `func TestHeldPlanReadsStoredWords` (sec_type `stock`/`option` planned, `STK`/`OPT` not), `func TestHeldPlanReadFailureKeepsInnerKeys` (inner keys and hash returned, one Warn, no error); `wire_underlying_test.go` contains `func TestResolveRefusesStoreWithoutHeldAdder`.
   6. `git grep -n "HeldOptions\|HeldRefused" -- cmd/ot/underlyings_site.go` prints the lines that fill both from `heldSets` and `HeldAdder.Check` (the page store must satisfy `HeldAdder`, else a start-up error; the fakes of that file gain `Check` and `Add`); `underlyings_site_test.go` contains `func TestUnderlyingsPageGapForHeldOption` and `func TestUnderlyingsPageRefusedReason` and `func TestUnderlyingsPageHeldReadFailure` (the page is built, one gap).

## Checked by CI

`go test ./internal/store/... ./internal/pages/underlyings/... ./cmd/ot/...` (the pg test in GitHub CI), `go vet`,
`golangci-lint`, `scripts/arch`, the web build; each entry compiles and passes alone in the order 1, 2, 3; the size rule
(new files under 600 lines, `wire_underlying.go` and `underlyings_site.go` under 800).

## Left out

- A sold position: the row stays and the chain runs until the owner retires it (decision 6).
- The stronger check, that the con_id of a held option is in the listed chain of the name (else the state "held option not in this name's chain"): it needs `ListContracts` (`internal/broker/api.go:269`) per name in the resolve job and a place to keep the answer; the note and the con_id on the page are the cheap check.
- A resolved name on the page: the broker answer has none (decision 4); a name needs a `contracts/proto` change.
- An index outside the set of decision 8, and a stock whose ticker equals an index option's: needs the underlying type in the position contract.
- The import skipping a held key (readers section): the owner sets the issuer on the page.
- A held ETF is added as kind stock: the position has no kind; the owner edits it.
- A currency other than USD and JPY: no market (`migrations/0033_underlying.sql:15`); the page names it with the reason, no row.
- Short puts on a held stock are listed blocked, not suppressed (readers section).
- `docs/design/underlyings.md` Interface section (owner-only path) is not edited; the new interface is in this file.
- A column for the origin (decision 4). No account line: no wrap and no held data, as `heldKeys` and `heldSets` behave today (`wire_underlying.go:96-99`, `underlyings_site.go:101`).

## Adversarial

Full answer: `held-adversarial.md` in the dispatcher's scratchpad (local, not published). Revision 2, point by point (r2 below supersedes where it says so):
1. Gap against the goal (pending and unresolved rows silent, starvation, sold position): adopted. Named
   page states and the day-late line (decision 9), sold position named (decision 6); `Pending` order: r2 changed it (decision 10).
2. Plan with a side effect, wrapper hides optional methods: adopted (decision 7, grep item 3.2); second DSN named; r2 moved the write out of `Plan`.
3. A simpler way: variant (a) not adopted (advisor: an `Apply` call from a wiring file is a permission change); the
   one-method `HeldAdder` is adopted. "Gap alone first" is possible: entry 2 depends on nothing and may land first.
4. Lock-in (resolve type widened, Note as origin, permanent rows, two markets): the widening is gone (decision 1); `Note`
   as origin stays, fragility stated (decision 4); permanent rows and USD/JPY only are in decisions 5 and Left out.
5. False or unchecked claims: Flex claim removed (Problem); options-off claim corrected (decision 5); "table" wording is
   now "one map beside the literals" (decision 3); `max_names` order stated (decision 10); the JP and zone variants of
   option_list_preopen (`docs/design/option-open-gap-jp.md:129`) are not designed here: only the US job exists on main.
6. Slip risks: Design lines and Depends on added; acceptance items are file-reading or `git grep` forms; no line counts;
   entry 3 depends on 1 and 2. The pinned page fixture `contracts/fixtures/page.underlyings.v1.golden.json` is not in any
   Files line: its held name AAA has a row, so no gap is emitted and it stays unchanged; if a gap appears in it, the
   change is reported, not edited (module contracts).
The ten contradictions: 1 (Settler vs rejected (a)) and 2, 3, 5 of Slip (compile break, ordering, Depends): fixed by
`HeldAdder` and the three-entry order. 4 (options-off) fixed, decision 5. 5 (key under the other market): fixed, decision 2.
6 (STK and OPT of one symbol): fixed, decision 7. 7 (no account reader is silent): not adopted, it is today's behaviour,
named in Left out. 8 (header lines `Owner approved`): not adopted, the owner approves at merge. 9 (pg test checked by
review): not adopted as an item; entry 1 item 6 checks that the test exists, the run is under Checked by CI. 10 (Flex): fixed.

r2 (advisor review r1 of head 39ff94f8, all points adopted unless noted):
- Refused name: its own page state with the reason (decision 9, test `TestHeldRefusedNameShowsItsReason`); `Check` gives the reason.
- Index options: the position has no underlying type (decision 8, `internal/store/api.go:708-724`), so a symbol set; the
  ticker collision with an unrelated stock is possible outside the set and is named, not solved (Left out). Not adopted:
  reading the type from the position, because it does not exist.
- `Plan` pure, write in `Run` (decision 7). Beyond the request: `Run` is called per key, so with no pending row it would
  never run; the plan therefore carries a held key per missing name, and the same `Run` resolves the new row.
- Held rows first in `Pending` (decision 10); the JP name bought today is decision 11; the "unresolved forever" retry is bounded by `retry_days` 7.
- A store without `HeldAdder` is a start-up error (decision 1), test `TestResolveRefusesStoreWithoutHeldAdder`.
- Second DSN stated; Options true for shares only kept, with the covered-call reason; each reader answered; the import
  case is named, not fixed (separate path). Contract test pins the written fields; items 1.7 and 2.6 are results.

r3 (advisor lean): one held key per missing name kept, with the reconcile behaviour cited (reconcile.go:151-189,238-244) and
the read-failure path named: inner keys, one Warn, a page gap, three tests (decision 7, 9). The index set is the barsrefresh
`[[index]]` data read through `DefaultManifest().Indexes`, no literal (decision 8, item 3.4). Not adopted: the resolved name
of a held-added row on the page, because the resolve answer carries none; instead `NoteHeld` tells the owner to check the
con_id (decision 4, item 1.2).

r4 (advisor r2 and r3): B1 stored words `stock`/`option`/`index` in decisions 7 and 8 with `Depends on: #SECTYPE` in entry 3 only, tests on stored words;
B2 the false "con_id and name" removed, the page says "Resolved as <market> stock <SYM>, con_id N" (item 2.3), the stronger chain check is in Left out;
B4 `Check` is a method of `HeldAdder`, errors typed `InvalidError`, no function outside the interface; B5 other Plan callers named and held
keys planned for today only (decision 7, `TestHeldPlanPastDayPlansInnerOnly`); small: `builder.go:24`, `manifest.go:46,85` confirmed.
