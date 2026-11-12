# Held underlying add: a held stock with no row on the list gets a pending row, or a named state on the page

Author: window (draft for advisor review)
Revision: 2
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

1. New interface `underlying.HeldAdder` with one method `Add(ctx, symbol, currency string) (added bool, err error)`.
   `Settler`, `Writer`, `Store` and `Apply` are unchanged, so no fake of `Store` or `Settler` stops compiling. `underlyingpg.Store`
   implements it; `cmd/ot` reaches it by type assertion on the store the resolve job already opens
   (`cmd/ot/wire_underlying.go:61-66`); a store without it (a fake) leaves the job unwrapped.
2. `Add` writes one pending row: kind stock, `Options` true, `Fundamentals` false, no issuer, `ConID` 0, exchange and
   currency the defaults of the market, `Note` = `underlying.NoteHeld`. It writes nothing and returns false when the key
   has a row in any status under either market (US:X or JP:X), a retired one included, so it never resurrects a stopped name and the gap rule of decision 9 and `Add` agree. Check and insert run under
   the table lock as `Apply` and `Settle` do (`internal/store/underlyingpg/write.go:12-16,43`).
3. Market comes from the currency, once, in `underlyingpg`: `ValidateAdd` holds market to exchange and currency
   (`internal/store/underlyingpg/validate.go:57-62`); one map `marketOfCurrency` (USD US, JPY JP) is added beside it and
   `ValidateAdd` is called with the result, so the symbol rules (`validate.go:44-54`) apply. Not
   master.instrument.calendar_id (`migrations/0008_master_instrument.sql:13`): a name with no row has no instrument. Not
   `rd.underlying_r1.market`: that is the row being made. `cmd/ot/live_quotes.go:74-75` stays.
4. Origin: no column exists (`migrations/0033_underlying.sql:11-31`), a column needs a migration; the origin is the `Note`
   value `NoteHeld` ("Added: held position", `validate.go:27` allows 200). `Settle` copies the row (`write.go:112-121`),
   so it survives the resolve. If the owner overwrites the note the row is the owner's.
5. Stopping, stated as it is. The row persists; no job removes or retires it. On the underlyings page the owner sets
   retired, paused or options off. For a held option the chain does not stop: `followedBy` keeps `options` while the
   owner holds an option on the name (`r.Options || heldOption`, `cmd/ot/followfilter.go:28,41`), whatever the switch or
   status. For shares only: options off, paused or retired stops the chain (`followfilter.go:26-44`); retired or paused
   keeps bars. `Add` skips a retired key (decision 2).
6. Sold position: the row stays active with options on and the chain keeps running until the owner retires it. The
   design does not retire its own `NoteHeld` rows: the resolve role has no edit method, the owner may have taken the
   row over, and a silent retire is the same kind of silence this design removes. Simpler and honest: named in Left out.
7. Caller: a wrapper `heldRows` in `cmd/ot` around the resolve job (`cmd/ot/wire_underlying.go:66`). Its `Plan` first
   reads the newest positions of `heldAliases` (lookback `cmd/ot/companies_site.go:89`), takes each `STK` or `OPT`
   position of nonzero quantity (`companies_site.go:123`) with symbol and currency, calls `Add` once per distinct (symbol, currency) pair (an `STK` and an `OPT` of one symbol count once), then
   returns the inner `Plan`, so a new row is asked in the same run (`internal/jobs/underlyingresolve/job.go:71-100`).
   `heldSets` gives no currency, so the wrapper has its own reader of `AccountReader.Positions`; that is a second DSN
   (`cfg.accountDSN`) for this job beside `resolveDSN`, opened as `heldKeys` does (`wire_underlying.go:96-109`); with
   no account line the job is unwrapped. An `Add` error logs one `Warn` (symbol, reason) and never fails the plan. `Plan`
   writes, which no other Plan does; `Add` is idempotent so a repeated or replayed Plan adds nothing the second time.
8. The wrapper forwards every optional method of the inner job by type assertion: `Schedule`, `RunBound` and
   `RestartSensitive` (`internal/jobs/underlyingresolve/job.go:52-66`), as `listedJob` does (`cmd/ot/listedjob.go:56`);
   without them the job has no schedule. `ID` and `Run` pass through.
9. The page shows a named state for every held name (shares or option position) that is not yet followed, never silent,
   as a gap `{kind: gap, reason: no_data}` (in the enum, `contracts/schemas/common.v2.json:15`; the view prints every gap,
   `web/src/pages/underlyings/view.tsx:224`), sorted by symbol: no row under either market ("X: held, not on the list");
   a pending row ("X: pending; the first option chain comes at least one session day after the resolve"); an unresolved
   row ("X: unresolved, no option chain: <reason>"). Gone when the row is active. `Input.Held` (shares,
   `builder.go:49`) gets a sibling `HeldOptions` for option-only names (`heldSets` returns both).
10. Pending order is not changed: `Pending` sorts by `name_key` (`internal/store/underlyingpg/read.go:22-23`) and the job
    cuts at `max_names` 20 (`job.go:77`, `job.toml`). Not changed because the owner adds few names, the cut is a
    carry-over to the next session day, and a starved held name stays visible through decision 9 as pending.

## Side effects of `Options` true (checked)

- Pending: no group (`cmd/ot/followfilter.go:22-46`); the resolve runs 45 minutes after the XNYS close (`job.toml`), so
  the first chain is at least a day late. Wanted; said on the page (decision 9).
- Active: group `options` (`followfilter.go:28`) feeds option_candidates (`cmd/ot/candidates.go:77`), option_eod and JP
  (`cmd/ot/wire_optioneod.go:83,110`), iv_history (`cmd/ot/wire_ivhistory.go:63`), option_open resolve, plan and pull
  (`cmd/ot/wire_optionopen.go:100,145`). Wanted: this is the missing chain.
- Candidate ranking and sell-put page: short-put, covered-call and held-call-guard rows
  (`internal/jobs/optioncandidates/calls.go:158`); the name ranks like any, no extra weight. Wanted for covered calls;
  short puts on a held stock are not suppressed (Left out).
- option_list_preopen (`internal/jobs/optionopen/list.go:118-120`): every watched US stock; not wired in `cmd/ot` on
  main (no caller of `NewListPreopen`), so the new US name joins when it is wired. Wanted.
- Dividends and fundamentals: issuer empty, `Fundamentals` false: neither follows (`followfilter.go:25,31`).

## Modules

none. `internal/store/**`, `internal/pages/**` and `cmd/**` are existing rows (`docs/design/modules.md:90,95,101`); no
row, path, entry cell or depends cell changes; `ot` already depends on store and pages.

## Interface

Owning module: store. Interface file `internal/store/underlying/api.go`; contract test
`internal/store/underlying/api_test.go`. Added beside `Settler`:

    // HeldAdder is held by the held-name wrapper of cmd/ot over the resolve store (role ot_w_underlying_resolve).
    type HeldAdder interface {
        // Add writes one pending stock row (Options true, Note NoteHeld) for symbol in the market of currency. It writes
        // nothing and returns added false when the key has a row in any status. It returns an InvalidError for a currency
        // with no market or a symbol the market refuses.
        Add(ctx context.Context, symbol, currency string) (added bool, err error)
    }

and the constant `NoteHeld = "Added: held position"`. The fake of the contract test implements `HeldAdder`; the caller
side asserts added true, then false, false after a retired edit, and an InvalidError for GBP.

## Issues

1. (interface) store: `HeldAdder`, `NoteHeld` and the `underlyingpg` implementation. Depends on: none.
   Design: docs/design/held-underlying-add.md#interface
   Files: internal/store/underlying/api.go, internal/store/underlying/api_test.go, internal/store/underlyingpg/write.go, internal/store/underlyingpg/validate.go, internal/store/underlyingpg/store_test.go, internal/store/underlyingpg/store_pg_test.go, internal/store/underlyingpg/validate_test.go
   1. `git grep -n "type HeldAdder interface" -- internal/store/underlying/api.go` prints one line, and `git grep -n "type Settler interface" -A8` shows `Settler` without `Add`.
   2. `git grep -n "NoteHeld" -- internal/store/underlying/api.go` prints the constant; the file has no `func` line.
   3. `api_test.go` contains `func TestHeldAdderContract` (fake implements `HeldAdder`; caller side as in Interface).
   4. `git grep -n "func (s \*Store) Add(" -- internal/store/underlyingpg/write.go` prints one line; `git grep -n "marketOfCurrency" -- internal/store/underlyingpg/validate.go` prints the map with USD and JPY only.
   5. `git grep -n "func TestMarketOfCurrency\|func TestAddWritesPendingHeldRow\|func TestAddSkipsExistingKey" -- internal/store/underlyingpg` prints three lines; `store_pg_test.go` carries `//go:build pg` and `func TestAddThroughResolveRole`.
   6. `git grep -n "HeldAdder" -- internal/store/underlyingpg` prints a compile-time assertion `var _ underlying.HeldAdder = (*Store)(nil)`; `git diff --stat main -- migrations` is empty.
2. pages: the named state for a held name that is not followed. Depends on: none.
   Design: docs/design/held-underlying-add.md#decisions
   Files: internal/pages/underlyings/builder.go, internal/pages/underlyings/text.go, internal/pages/underlyings/builder_test.go
   1. `git grep -n "HeldOptions" -- internal/pages/underlyings/builder.go` prints the `Input` field `HeldOptions map[string]bool`.
   2. `git grep -n "no_data" -- internal/pages/underlyings/builder.go` prints the constant and the append; text.go holds three texts (no row, pending, unresolved), each at most 200 characters, the pending text containing "at least one session day".
   3. `builder_test.go` contains `func TestHeldNameWithoutRowIsAGap`, `func TestHeldPendingRowIsAGap`, `func TestHeldUnresolvedRowIsAGap` and `func TestHeldActiveRowGivesNoGap`, the first also with the name only in `HeldOptions`.
   4. `git diff --stat main -- contracts web` is empty.
3. ot: the wrapper, its forwarding and the option-held set. Depends on: 1, 2.
   Design: docs/design/held-underlying-add.md#decisions
   Files: cmd/ot/held_underlying_rows.go, cmd/ot/held_underlying_rows_test.go, cmd/ot/wire_underlying.go, cmd/ot/wire_underlying_test.go, cmd/ot/underlyings_site.go, cmd/ot/underlyings_site_test.go
   1. `git grep -n "underlying.HeldAdder" -- cmd/ot/wire_underlying.go` prints the type assertion; `git grep -n "heldRows(" -- cmd/ot/wire_underlying.go` prints the wrap, guarded by `cfg.accountDSN`.
   2. `git grep -n "func (.*) Schedule(\|func (.*) RunBound(\|func (.*) RestartSensitive(" -- cmd/ot/held_underlying_rows.go` prints three lines, each calling the inner job by type assertion.
   3. `git grep -n "\.Add(" -- cmd/ot/held_underlying_rows.go` prints one line; `git grep -n "Apply(\|underlyingresolve" -- cmd/ot/held_underlying_rows.go` prints nothing.
   4. `held_underlying_rows_test.go` contains `func TestHeldRowAdded`, `func TestHeldRowSkips`, `func TestHeldRowWarns`, `func TestHeldRowForwardsOptionals`; `wire_underlying_test.go` contains `func TestResolveUnwrappedWithoutHeldAdder`.
   5. `git grep -n "HeldOptions" -- cmd/ot/underlyings_site.go` prints the line that sets it from `heldSets`; `underlyings_site_test.go` contains `func TestUnderlyingsPageGapForHeldOption`.

## Checked by CI

`go test ./internal/store/... ./internal/pages/underlyings/... ./cmd/ot/...` (the pg test in GitHub CI), `go vet`,
`golangci-lint`, `scripts/arch`, the web build; each entry compiles and passes alone in the order 1, 2, 3, and the
size rule (new files under 600 lines, `wire_underlying.go` and `underlyings_site.go` under 800).

## Left out

- A sold position: the row stays and the chain runs until the owner retires it (decision 6).
- Priority of held names in the pending read (decision 10).
- A held ETF is added as kind stock: the position has no kind; the owner edits it.
- A currency other than USD and JPY: no market (`migrations/0033_underlying.sql:15` allows US and JP); named by the page, no row.
- Short puts on a held stock are not suppressed: no rule on main says so; owner question if wanted.
- `docs/design/underlyings.md` Interface section (owner-only path) is not edited; the new interface is in this file.
- A migration or column for the origin (decision 4).
- No account line: the job is unwrapped and the page has no held data, as `heldKeys` and `heldSets` behave today (`wire_underlying.go:96-99`, `underlyings_site.go:101`); not changed.

## Adversarial

Full answer: `held-adversarial.md` in the dispatcher's scratchpad (local, not published). Revision 2, point by point:
1. Gap against the goal (pending and unresolved rows silent, starvation, sold position): adopted except ordering. Named
   page states and the day-late line (decision 9), sold position named (decision 6); `Pending` order kept, reason in decision 10.
2. Plan with a side effect, wrapper hides optional methods: adopted (decisions 7 and 8, grep item 3.2); second DSN named.
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
review): not adopted as an item; entry 1 item 5 checks that the test exists, the run is under Checked by CI. 10 (Flex): fixed.
