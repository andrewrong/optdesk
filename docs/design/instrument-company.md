# instrument and company, step 5 of the storage plan: one id with a stock side and an option side, a symbol per instrument, per-source symbols in the identifier table, a small company table

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 1

Source: owner 2026-11-13 S6 (07:01Z: "three columns symbol, market, primary exchange; open the identifier table; a new company table; stock and option apart, I may hold both"; decided with the advisor: one shared id, two views, T 07:03Z) and "storage plan v2 OK" (07:11Z); storage plan v2 of the advisor, P8, section 3, cut order step 5, section 6. `data-repair-fx.md` (landed) is the precedent for column grants, optional interfaces and the deploy rule.

Goal: `con_id` stays the anchor and `master.instrument` stays the one id every table points at. It gains `symbol`. Stocks and options each have a side table and a view (`rd.stock`, `rd.option`). The identifier table opens its scheme list for per-source symbols (and keeps one current row per scheme). A new small `master.issuer` table is the company: one company, many instruments (two share classes, a stock and its options); 13F, filings and fundamentals will key on it. A Japanese stock is symbol `7203` on the Tokyo exchange, no `.T`.

## Today

All paths at origin/main 29c24a9a.

- `migrations/0008_master_instrument.sql`: `master.instrument (instrument_id, instrument_type, issuer_id bigint (no foreign key, never written), name, currency, mic, calendar_id, status, price_increment, lot_size, recorded_at)`; `master.instrument_identifier (instrument_id, scheme, value, valid_from, valid_to)`, key `(scheme, value, valid_from)`, check `instrument_identifier_scheme_ck` (`ibkr_conid, osi, ticker_exchange, jpx_option_code, isin`). `ot_w_master` has SELECT and INSERT on both; `0011` adds INSERT and SELECT on two columns of `master.option_terms`. `0010` creates `rd.instrument_r1 (instrument_id, instrument_type, name, currency)`, read by `internal/store/instrument/names.go`, `index_ids.go`, `stock_ids.go`.
- `master.instrument.name` holds the ticker (`jobs` pass `Name: c.Symbol`, for example `internal/jobs/dividendcapture/run.go:174`, `optionopen/inputs.go:112`). The `mic` is not reliable: the stock specs of `optionopen/inputs.go:112`, `optionopen/list.go:374` and `optioneod/watch.go:102` pass `CalendarID: "XNYS"` for every stock and no MIC, a Japanese one included.
- `internal/store/instrument/resolve.go:85` `ResolveInstrument` is the one writer of `master.instrument` and its identifiers (lock, lookup by `valid_to IS NULL`, insert, identifiers, option terms for an option). It is called from `barsrefresh/run.go:86`, `dividendcapture/run.go:175`, `optioneod/run.go:202,379`, `optionopen/inputs.go:114`, `list.go:379`, `listed.go:441` and `quotesnapshot/run.go:142`. The job `underlyingresolve` does not write the instrument tables: `internal/jobs/underlyingresolve/job.go:129` resolves a contract with the broker and `Settle`s the con_id into `settings.underlying`. `store.InstrumentResolver` and `InstrumentSpec` are in `internal/store/api.go:58-75`, which stands at 800 lines and may not grow (plan section 4).
- `master.option_contract` (`0032`, written by `ot_w_option_open`) has the underlying id, expiry, strike, right, multiplier, trading class, exchange; `master.option_terms` (`0009`, `rd.option_terms_r1`) has right, strike, multiplier, expiration. They overlap (plan P8); one option can be in either or both.
- `settings.underlying` (`0033`, view `rd.underlying_r1` with the same columns): `name_key, symbol, market (US|JP), kind (stock|etf), exchange, currency, name, issuer, con_id, status`; `issuer` is the watch-list text `cik:<10 digits>` or `edinet:<code>` (`cmd/ot/watch.go:25`) or empty.
- `internal/store/instrument/instrument_pg_test.go:123` builds its database from `0001` to `0008` only; the `pg` tests of the package run only in GitHub CI.

## Decisions

1. Common row (plan P8 item 1). `master.instrument` gains `symbol text NOT NULL DEFAULT ''` (the default stays: the old binary's insert still works during the deploy window). Backfill: for a stock, etf or index the current `name`; for an option the current `osi` identifier, else `name`. `name`, `issuer_id`, the keys and every foreign key stay. `issuer_id` gains a foreign key to `master.issuer`. A check: `currency <> 'JPY' OR symbol !~ '\.'` (a Tokyo symbol has no suffix; the currency, not the `mic`, which jobs leave empty). `rd.instrument_r1` is not changed.
2. Stock side (owner S6, plan P8). `master.stock (instrument_id bigint PRIMARY KEY REFERENCES master.instrument, name text NOT NULL, market text NULL CHECK (market IN ('US','JP','EU')), primary_exchange text NULL, issuer_id bigint NULL REFERENCES master.issuer, sector_fund text NULL)`; one row per stock, etf or index. `name` is the company or fund name. The `issuer_id` of the stock and of the instrument row are the same value (the migration and `RegisterStock` set both; the view reads the stock's).
3. Option side. `master.option_contract` stays the option side table as it is; `master.option_terms` stays; making the two one is the step that touches them (plan P8) and is not this one. Nothing about options is written differently here.
4. Two views for the pages. `rd.stock (instrument_id, symbol, name, market, primary_exchange, currency, mic, sector_fund, issuer_id, issuer_name)` over instrument, stock and issuer; `rd.option (instrument_id, symbol, currency, underlying_id, underlying_symbol, expiry, strike, "right", multiplier, trading_class, exchange)` over the instruments of type `option` left joined to `master.option_contract` and the underlying instrument (an option with terms only has NULL underlying columns). Both run with the rights of their owner; `SELECT` to `ot_ro_pages` and `ot_runner`. They are new names; no existing view changes.
5. Identifier table (plan P8 item 2). The scheme check becomes `ibkr_conid, osi, ticker_exchange, jpx_option_code, isin, cusip, figi, sym_fred, sym_alphavantage, sym_jquants, sym_sec, sym_eodhd` (the two old ones stay for the rows that exist). A unique index on `(instrument_id, scheme) WHERE valid_to IS NULL`: one current row per instrument and scheme, which makes `ibkr_conid` the single anchor. A reader asks "the value of scheme X for instrument N"; a missing row means the source does not have it, never a guess from the ticker. Filling the per-source schemes and `cusip` comes with the first design that needs one (Left out); `cusip` is only ever written for an instrument that already exists here, never for every CUSIP of a filing (plan P8 item 6).
6. Company (plan P8 item 3; owner: "a small company table"). `master.issuer (issuer_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, name text NOT NULL, country text NOT NULL, cik text NULL, edinet_code text NULL)`, a check that one of the two codes is set, unique indexes on each code where not null. The plan calls it `master.issuer`; the owner's word is the company table: one table, the plan's name. Fundamentals, earnings and 13F re-key to it in their own steps (7 and the 13F design), not here.
7. Backfill (the migration, owner DDL). For every instrument of type stock, etf or index: a `master.stock` row with `name` = the name of the settings row (the newest row of each `name_key` by `set_at`, then `id`; `rd.underlying_r1` passes every version) whose con_id equals the instrument's `ibkr_conid` and the name is not empty, else the instrument name; `market` from the currency (`USD` US, `JPY` JP, `EUR` EU, any other currency NULL, which the check accepts, so the migration never stops on it; the `mic` is not used, Today); (when two `name_key`s' newest rows carry one con_id, the one with the smaller `name_key` text is used: a tie rule, so the result does not depend on join order); `primary_exchange` = that settings row's `exchange` unless it is empty or `SMART`, else NULL; `issuer_id` from the settings row's `issuer` text (`cik:` gives `cik`, `edinet:` gives `edinet_code`; one issuer row per distinct code, named after the settings row, country from its market). A settings row with a malformed issuer text gives no issuer. The migration never merges two issuer rows: the issuer text carries one code, so a company with a US line and a JP line stays two issuers (Left out). An instrument with no settings row gets a stock row with the instrument name and no company.
8. Writes after the migration (plan P8 item 4; the plan's "underlyingresolve is the only writer" is not so today, see Today, and is made true for company data only). (a) `ResolveInstrument`, when it creates an instrument, also sets `symbol` (the `osi` identifier of an option, else `spec.Name`) and, for a stock, etf or index, inserts its `master.stock` row (`name = spec.Name`, `market` from the currency, NULL for a currency other than USD, JPY or EUR); when it finds one whose `symbol` is empty (made by the old binary between the migration and this code), it fills `symbol` and the stock row once; options are not touched on a hit. No caller and no interface changes. A symbol with a dot on a JPY instrument is refused as a Permanent error before the insert. (b) `RegisterStock` (the interface below) is the writer of the company data: it finds or creates the stock by `ibkr_conid` (creating as the jobs do today), overwrites `symbol`, `name`, `market`, `primary_exchange`, `sector_fund`, finds or creates the issuer by `cik` or `edinet_code` (a name change overwrites), and sets both `issuer_id` columns. A registration whose `cik` and `edinet_code` match two different issuer rows is refused as a Permanent error naming both ids and nothing is written. Equal input rewrites nothing.
9. The callers (owner S6: one resolver). `RegisterStock` is called on the two paths that make a name's con_id known, with the row of `settings.underlying` (symbol, market, kind, exchange, currency, name, issuer text; MIC empty and calendar `XNYS`, the values every job passes today, Left out). (a) The job `underlying_resolve`, after `Settle` wrote a resolved con_id (written true, con_id > 0). (b) `ot underlying import`, after `Apply` for each added entry that carries a con_id (those rows are written active and never pass the job; `cmd/ot/underlying_import.go`). The job cannot enumerate active rows: its `Settler` reads only pending rows and widening it is a change of `internal/store/underlying/api.go`, so the import path is used. Both run only when a registrar is wired. A Permanent refusal (Decision 8b: a registration whose `cik` and `edinet_code` match two issuer rows) is logged with its reason and does not fail the run; any other registrar error makes the job result Retryable (the import prints it and exits non-zero) and never undoes the `Settle` or the `Apply`. Names already resolved are covered by the migration backfill; a name whose instrument another job creates later gets its stock row from Decision 8a and its company at the next import or resolve of that name. There is no page action to correct a wrong instrument-to-company link (Left out).
10. Lookup (plan P8 item 5). `StockDirectory.LookupStock(symbol, market)` gives the instrument id of a stock by symbol and market; two stock rows with that symbol and market are an error (Permanent, naming both ids), never a pick: one lookup for the ticker joins to end, one reader at a time (Left out: no reader is changed here).
11. Roles. `ot_w_master` gets `SELECT, INSERT, UPDATE` on `master.stock` and `master.issuer` and `UPDATE (symbol, issuer_id)` on `master.instrument`. Closing an identifier row (`UPDATE (valid_to)`) is not granted: nothing here closes one; it comes with the first writer of a per-source scheme. No DELETE, no TRUNCATE. `ot_ro_pages` and `ot_runner` read the two new views only.
12. Deploy (the migration holds the auto-deploy). Entry 2 carries `0044_instrument_company.sql` (after `0043_series_registry.sql` of `series-registry.md`, itself after `0042` of `strategy-framework` and `0041_price_bar_repair.sql` of `data-repair-bars.md`); the window runs rebuild `ot`, `ot setup pg`, migrate, `ot setup pg` again. Entries 3 and 4 merge after it. The migration fails loudly rather than guess: a duplicate current identifier of one scheme on one instrument, or a JPY symbol with a dot stops it and changes nothing (one transaction); it is rehearsed on a copy first (plan section 5). The old binary keeps working on the migrated table (default symbol, no stock row; Decision 8a repairs both on the next resolve). The Down part drops the views, the two tables, the index, the check and the column, and restores the old scheme check; it refuses when a row carries a new scheme.
13. No old value is kept (owner S1): the company name, exchange and issuer are overwritten by the newest `RegisterStock`.

## Modules

`docs/design/modules.md`, row `store`: the entry cell gains `internal/store/stockreg` (declarations only, like `internal/store/bar`). Entries touch store (`internal/store/**`, `migrations/**`, `docs/contracts/store.md`; entries 1, 2), jobs (`internal/jobs/**`, `docs/contracts/jobs/**`; entry 3) and ot (`cmd/**`; entry 4). New import edges inside the rule of `modules.md` line 51: `internal/jobs/underlyingresolve` imports `internal/store/stockreg`; `internal/store/instrument` imports it.

## Interface

One interface change, entry 1, the new declarations-only package `internal/store/stockreg` (`api.go`, `api_test.go`; additive, nothing in `internal/store/api.go` changes):

    // Issuer is a company. Empty strings mean "not set"; at least one of CIK and EdinetCode is set.
    type Issuer struct {
        Name, Country, CIK, EdinetCode string
    }

    // Registration is what the owner's list and the broker say about one stock, etf or index.
    type Registration struct {
        ConID                                          int64
        Kind                                           string // stock or etf
        Symbol, Market, PrimaryExchange, SectorFund    string // Market is US, JP or EU; a Tokyo symbol has no suffix
        Name, Currency, MIC, CalendarID                string
        Issuer                                         *Issuer // nil: no company is known
    }

    // StockRegistrar writes the stock side and the company of an instrument, creating the instrument by its ibkr_conid
    // when none exists. Newest input wins; equal input changes nothing. It returns the instrument id. It is an optional
    // method of the value store.OpenInstrumentFunc returns; the job asserts it.
    type StockRegistrar interface {
        RegisterStock(ctx context.Context, r Registration) (int64, error)
    }

    // StockDirectory reads by symbol and by scheme. Optional method of the same value.
    type StockDirectory interface {
        // LookupStock returns the id of the stock with the symbol in the market; ok is false when there is none.
        LookupStock(ctx context.Context, symbol, market string) (id int64, ok bool, err error)
        // Identifier returns the value of the scheme valid now for the instrument; ok is false when there is none.
        Identifier(ctx context.Context, instrumentID int64, scheme string) (value string, ok bool, err error)
    }

`store.InstrumentResolver`, `InstrumentSpec`, `InstrumentStore`, `OpenInstrumentFunc` and every record in `contracts` are unchanged: no fake breaks.

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. Entry 2 holds the auto-deploy (migration); entries 3 and 4 merge after the window ran it (Decision 12). `Contracts: unchanged` (no file under `contracts/schemas`, `contracts/datasets` or `contracts/proto`). Test runs are in `## Checked by CI`.

### 1 instrument-company-api (interface)

Module: store. Size: S.
Order: first (design PR: after the series-registry design merge, which edits the same modules.md line).
Files: `internal/store/stockreg/api.go`, `internal/store/stockreg/api_test.go`
Design: docs/design/instrument-company.md#Interface
Acceptance:
1. `api.go` declares `Issuer`, `Registration`, `StockRegistrar` and `StockDirectory` as in the Interface section, with docs and no function body, and is the only non-test `.go` file of the package.
2. `grep -c 'internal/store/stockreg' docs/design/modules.md` prints 1 (the `store` row already holds the line on BASE, written by this design's pull request, which merges after the series-registry design); `git diff --name-only BASE -- docs/design/modules.md internal/store/api.go internal/store/api_test.go` prints nothing.
3. `api_test.go` has new contract tests (`t.Parallel()`): `TestStockRegistrarContract` (a fake returns one id for equal input and the caller reads it back) and `TestStockDirectoryContract` (a lookup that finds, one that does not, and an identifier that is missing).
4. `git diff --name-only BASE` lists only paths of the Files line.

### 2 instrument-company-store

Module: store. Size: L.
Order: after 1; after series-registry entry 2 (migration 0043, and both edit `docs/contracts/store.md` and `setup.go`); after data-repair-bars entry 2 (`0041`) and strategy-framework entry 5 (`0042`). Holds the auto-deploy: it carries a migration (Decision 12).
Files: `migrations/0044_instrument_company.sql`, `internal/store/instrument/resolve.go`, `internal/store/instrument/stock.go`, `internal/store/instrument/lookup.go`, `internal/store/instrument/resolve_test.go`, `internal/store/instrument/stock_test.go`, `internal/store/instrument/instrument_pg_test.go`, `internal/store/instrument/stock_pg_test.go`, `internal/store/setup/setup.go`, `docs/contracts/store.md`
Design: docs/design/instrument-company.md#Issues (Decisions 1 to 8, 10 to 13)
`resolve_test.go` and `instrument_pg_test.go` use the changed insert and the test database list; `internal/store/instrument/instrument_pg_test.go` is 269 lines and `resolve_test.go` 410: new tests go into the new files.
Acceptance:
1. `0044_instrument_company.sql` has an Up and a Down part, is owned by `ot_owner`, and in Up creates `master.issuer`, adds `symbol` with `DEFAULT ''`, creates `master.stock`, backfills `symbol`, the stock rows and the issuers as Decision 7 states (`git grep -n "settings.underlying\|rd.underlying_r1" migrations/0044_instrument_company.sql` finds the join), adds the foreign key and the check of Decision 1, replaces the scheme check as in Decision 5, adds the unique index, creates `rd.stock` and `rd.option` with the columns of Decision 4, and grants as in Decision 11 (`git grep -n "DELETE\|TRUNCATE" migrations/0044_instrument_company.sql` finds nothing outside the Down part).
2. `resolve.go` sets `symbol` and inserts the stock row on create and repairs an empty symbol on a hit as Decision 8a says, and refuses a dotted JPY symbol as Permanent; its signature and `store.InstrumentResolver` are unchanged; `git diff --name-only BASE -- internal/store/api.go` prints nothing.
3. `stock.go` holds `RegisterStock` (Decision 8b) and `lookup.go` holds `LookupStock` and `Identifier` on the store value `instrument.Open` returns; a compile-time assertion in each file shows the value satisfies `stockreg.StockRegistrar` and `stockreg.StockDirectory`.
4. `stock_test.go` (unit, fake database) has new tests: `ResolveInstrument` of a new stock inserts one stock row after the instrument and sets the symbol; of a new option sets the symbol from the osi identifier and inserts no stock row; of an existing stock with an empty symbol fills it once; a dotted JPY symbol is a Permanent error; `RegisterStock` finds the issuer by `cik`, creates it when absent, a second call with equal input sends no write; a registration whose `cik` and `edinet_code` name two issuer rows is a Permanent error and sends no write; `LookupStock` with two matching rows returns an error naming both ids and with one row returns it.
5. `instrument_pg_test.go` applies `0044_instrument_company.sql` and the migrations it reads (`0032`, `0033`, and the ones those need; the file list at `:123` names them); `stock_pg_test.go` (tag `pg`, new tests) shows: the backfill gives the stock row, market from the currency and NULL for a currency other than USD, JPY or EUR (the migration does not stop), a stock created by `ResolveInstrument` in such a currency gets a stock row with NULL market, the issuer from a `cik:` and an `edinet:` text, one issuer for two instruments with the same code; two `name_key`s whose newest rows share a con_id give the stock row of the smaller `name_key`, and an older version of a name does not win over the newest; a second current row of one scheme on one instrument is refused; `rd.stock` and `rd.option` return the stated columns and an option without `option_contract` has NULL underlying columns; the role `ot_w_master` runs `RegisterStock` with the grants alone and cannot DELETE; the old `insertInstrumentSQL` still works on the migrated table.
6. `setup.go` changes only comments (`git diff BASE -- internal/store/setup/setup.go` shows comment lines only) and `docs/contracts/store.md` states the tables, views, schemes, grants and `0044` (`git grep -n "0044" docs/contracts/store.md` finds it).
7. `git diff --name-only BASE` lists only paths of the Files line; every new source file is under 600 lines.

### 3 instrument-company-job

Module: jobs. Size: M.
Order: after 1; merge after the window ran the migration of entry 2.
Files: `internal/jobs/underlyingresolve/job.go`, `internal/jobs/underlyingresolve/register.go`, `internal/jobs/underlyingresolve/register_test.go`, `internal/jobs/underlyingresolve/job_test.go`, `docs/contracts/jobs/underlying_resolve.md`
Design: docs/design/instrument-company.md#Issues (Decision 9)
`job_test.go` builds the job with `New`; it stays valid because `New` does not change.
Acceptance:
1. `register.go` has `NewWithRegistrar(b broker.Broker, s underlying.Settler, reg stockreg.StockRegistrar, clk clock.Clock) (contracts.Job, error)`; `New` is unchanged and builds the job without a registrar (no call). `job.go` calls the registrar after a `Settle` that wrote a con_id above zero, with a `stockreg.Registration` made from the settled row; no call for `not listed`, `ambiguous` or a `Settle` that wrote nothing.
2. `register.go` exports `RegistrationOf(row underlying.Row) stockreg.Registration`, the one mapping the job and `ot underlying import` use. It is tested: `cik:0000320193` gives `Issuer{CIK: "0000320193", Country: "US"}`, `edinet:E02144` gives `EdinetCode` and country `JP`, an empty or malformed text gives a nil `Issuer`; a Tokyo name `7203` is registered as symbol `7203`, market `JP`.
3. A registrar error makes the result's `Err` Retryable and leaves the settled con_id as it is; a Permanent refusal is logged with its reason and leaves the result unchanged; tested.
4. `register_test.go` holds the new tests of 2 and 3 and of the no-call cases of 1 (fake broker, fake settler, fake registrar, no database).
5. `docs/contracts/jobs/underlying_resolve.md` states the registrar and the mapping.
6. `git diff --name-only BASE` lists only paths of the Files line.

### 4 instrument-company-wire

Module: ot. Size: S.
Order: after 3.
Files: `cmd/ot/wire_underlying.go`, `cmd/ot/wire_underlying_test.go`, `cmd/ot/underlying_import.go`, `cmd/ot/underlying_import_test.go`
Design: docs/design/instrument-company.md#Issues (Decision 9)
`underlying_import_test.go` calls `underlyingImport`, whose signature gains the optional registrar.
Acceptance:
1. `wire_underlying.go` opens the instrument store with `a.deps.openInstrument` and the master DSN when that DSN is set, asserts it to `stockreg.StockRegistrar`, closes it with the other stores (`jc.extra.addCloser`), and builds the job with `underlyingresolve.NewWithRegistrar`; with no master DSN, or a value without the method, it builds the job with `New` and logs that no company data is written.
2. `wire_underlying_test.go` has new tests for the three cases (DSN set and method present, DSN empty, method absent) with the fake `openInstrument`; `git diff --name-only BASE -- cmd/ot/wire.go` prints nothing (`wire.go` is 795 lines).
3. `underlying_import.go` takes an optional `stockreg.StockRegistrar` (the master DSN of the env file opened with `a.deps.openInstrument` and asserted; none: no call, a line says no company data is written) and, after `Apply`, calls it for each added entry with a con_id, with `underlyingresolve.RegistrationOf` of entry 3; a Permanent refusal is printed with its reason and does not change the exit status; `underlying_import_test.go` has new tests: a call per added entry with a con_id and none for a pending one, no call without a registrar, a refusal printed and exit status zero.
4. The console table is not edited (`git diff --name-only BASE -- cmd/ot/jobconsole_jobs.go` prints nothing): after `data-repair-bars.md` entry 4 and `series-registry.md` entry 5 it stays as it is after `series-registry.md` entry 5: 31 rows, 7 any-day, 16 today-only, 8 listed-only, four force-yes, 19 force-no.
5. `git diff --name-only BASE` lists only paths of the Files line.

## Checked by CI

`go test ./internal/store/... ./internal/jobs/underlyingresolve/... ./cmd/ot/... -race -count=1` (the `pg` tests of entry 2 with `0044` applied to a fresh database run only in GitHub CI), `go run ./scripts/arch --check`, `go build ./...`, `golangci-lint`, and the contracts generation check (no change expected). Results only a run shows: the backfill result on the test rows, the grants as the role `ot_w_master`, the two views' columns, the old insert on the migrated table.

## Left out

- A web page action to correct a wrong instrument-to-company link: today only the watch file and `ot underlying import` (which ignores existing names) or a migration change it. A follow-up brings the page action before the first page groups by company. One company with a US and a JP line stays two issuers until then.
- Filling the per-source schemes (`sym_fred`, `sym_alphavantage`, `sym_jquants`, `sym_sec`, `sym_eodhd`), `isin`, `figi` and `cusip`, and the writer and the `UPDATE (valid_to)` grant that closing a row needs: each comes with the first source or the 13F design that uses it (CUSIP only for instruments already held).
- Ending the ticker joins (`account.position.symbol`, `earnings_date.symbol`, the held-option match, the ops subject keys): one reader at a time through `LookupStock`; settings.underlying's `con_id` column is still written by `Settle` and is not read from the identifier table yet.
- Re-keying fundamentals, earnings and 13F to `master.issuer` (step 7 and the 13F design); `master.option_terms` and `master.option_contract` becoming one table; `instrument.name` (still the ticker) and `rd.instrument_r1`.
- The wrong `mic` and calendar (`XNYS`) on Japanese stocks created by other jobs (Today): not corrected here, because every reader of `calendar_id` would change behaviour; `rd.stock` shows the `mic` as it is. Europe: a stock per listing with its own con_id (plan P8 item 7) needs nothing more than the `EU` market value.
- A console row to re-register names that are already resolved: they are covered by the backfill; `underlying_resolve` stays today-only.

## Doubts

1. `symbol` of the existing rows is `name`, which today is the ticker the jobs passed; a row whose `name` is something else (a demo name, an old import) would carry it as symbol. The `pg` test shows the rule on rows; the first run on a copy of the real database (plan section 5) shows the data.
2. The currency decides `market`; a USD-quoted stock listed elsewhere would read `US`. Today's names are US and JP only (`settings.underlying` check), so it holds now.
3. The `settings.underlying` `exchange` is the owner's list value; whether it is IBKR's primary exchange code (`NASDAQ`, `TSEJ`) or a routing name is not verified, so `primary_exchange` can be NULL for names that say `SMART` and a routing name otherwise.
4. The migration number 0044 assumes `0041`, `0042` and `0043` land first; another migration that lands first moves the number and the file list of entry 2's test.
5. The grants follow the PostgreSQL rules as in the FX entry; no database was available here, the `pg` tests are the check.
6. `RegisterStock` creates a missing instrument with MIC empty and calendar `XNYS`, the values the jobs pass today (Left out); the job `underlying_resolve` is the first to create a Tokyo stock this way only if no other job created it before.

## Adversarial

Full answers are kept with the dispatcher (not in the repository).
1. Gap (7203 guard keyed on empty mic; etf kind lost; options without issuer): adopted for the guard (now keyed on currency JPY); etf and option issuer not adopted (the resolver takes stock, index, option only; options carry no company in the plan).
2. Risk (migration aborts on real data): adopted for unknown currency (market NULL, which the check accepts; no stop); duplicate identifiers still stop it by design, found on the rehearsal copy.
3. Simpler (columns on instrument, no master.stock): not adopted; the owner asked for two tables and the advisor kept the stock side table (S6).
4. Lock-in (scheme check, market list): not adopted; a new scheme costs one small migration, accepted.
5. False claim ("etf" reaches the resolver; name is the company name): noted, not fixed; ETFs are stored as type stock, names are the ticker where no settings row matches (Doubt 1).
6. Slip (entry 2 size L, shared files with two unmerged designs): not adopted; the Order lines pin it, the test database list is named in acceptance 5.
