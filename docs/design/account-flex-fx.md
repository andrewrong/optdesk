# Account Flex sections and daily FX: cash events, open positions, net asset value and IBKR conversion rates from the Flex statement, stored; the daily FX job stays in market-bars

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 1

Status: draft, not yet a pull request. It builds on main at 7959e72: `sources.FlexSource`, `FlexStatement`, `FlexLogin`, `LoadFlexFileFunc` (`internal/sources/api.go`), `jobs.FlexRun`, `FlexAttributesSHA256Func` and `NewOptionLedgerJobFunc` (`internal/jobs/api.go`), `store.OptionLedgerWriter` and `OptionLedgerReader` (`internal/store/api.go`, 799 lines), the datasets `optionledger.flex_trade_row` and `optionledger.flex_fetch` (`contracts/datasets`), `store.DSNEnvOptionLedger`, the account tables of migration `0019_account.sql`, and `docs/design/account-data.md`, `docs/design/option-ledger.md`.
Not on main yet, and waited for: (1) `docs/design/option-ledger.md` issues 5, 6, 8 and 9 (the Flex client `internal/sources/flex`, the schema `optionledger` with its role `ot_w_optionledger` and its migration, and the job `option_ledger`: the packages `internal/sources/flex`, `internal/store/optionledger` and `internal/jobs/optionledger` hold only a stub or the shared hash on main); issues 5 and 6 here edit or extend what those create, so they are cut after those land; (2) `docs/design/market-bars.md` (pull request 768), which carries the daily USD.JPY job `fx_refresh`, the table `obs.fx_rate` and the first declarations-only package `internal/store/bar` with the `scripts/arch` change that lets a root component import such a package; the paragraph of `docs/design/modules.md` that states the rule is copied below from that pull request word for word, and entry 3 here is cut only after that pull request has merged.
Needs the owner: no owner-only path is edited (`contracts/proto/**`, `transport.py`, `.github/**`, `docs/design/ratchet.json` and `services.toml` stay as they are). Three things are the owner's and not an agent's, named in Decision 12: the edit of his Flex query in the IBKR Client Portal, the reading of more account data through the existing Flex token, and the new tables in his PostgreSQL. No ratchet ceiling is needed: the store interface goes in the new declarations-only package `internal/store/flexsection`, not in `internal/store/api.go`.
Source: owner 2026-11-09: all account data before cut-over; the ledger truth is IBKR Flex; the 15 s FX stream is dropped (daily FX only); only the main IBKR account must work; the margin cushion and the assignment cash never go to Telegram; own glue code sits behind an interface so a library can replace it; the front end renders data and computes nothing. Plan v2.5 section 11 rows W5-6 "FX job" and "Flex job" as written in `docs/design/alignment-v25.md` (the rows "FX job" and "Flex job" and Decision 3 item 4). `docs/design/account-data.md` Left out deferred Flex; `docs/design/option-ledger.md` Left out names the other Flex sections ("a later design adds them as registry entries") and is this design's origin for the Flex half.
Read on main at 7959e72: `AGENTS.md`, `docs/design/modules.md`, `alignment-v25.md`, `account-data.md`, `option-ledger.md`, `option-positions.md`, `option-candidates.md`, `docs/adr/0004-markets-calendars-fx.md`, `internal/sources/api.go`, `internal/jobs/api.go`, `internal/store/api.go`, `contracts/datasets/optionledger.flex_fetch.r1.json`, `optionledger.flex_trade_row.r1.json`, `contracts/generate.sh`, `internal/jobs/fredseries/job.toml`, `docs/contracts/ot.md`. On a branch in review (read, not on main): `docs/design/market-bars.md` and its change to `docs/design/modules.md`. option-trader was not read: the Flex attribute names below are from `option-ledger.md`, which read it, and are checked by the first real fetch (Decision 4).

## Goal

After each trading day the owner's Flex statement gives, for each account, the rows that the Trade section does not carry: cash events (deposits, withdrawals, dividends, interest, fees, withholding tax), the open positions IBKR itself reports, the net asset value and its change, and IBKR's own daily conversion rates. This design stores them, as sent, append-only, keyed by alias, and makes them readable by a Go `Reader` for the stage 2 builders. The daily FX job is not rebuilt here: it is `fx_refresh` of `docs/design/market-bars.md`, and the owner's wish "all account data before cut-over" is met for FX by that job plus the Flex conversion rates stored here. This design builds no figure, no page, no notice and no `ot` command.

## What main holds today

- The Flex client, the loader of `$OT_HOME/flex.toml`, the `option_ledger` job and the tables of schema `optionledger` are declared (interfaces and datasets for trades and fetches) and not built. `sources.FlexStatement` carries `Trades` as text maps and counts the rows of other levels in `OtherLevels`; every other section of a report is dropped by the parser that option-ledger issue 6 will write.
- No cash event, open position, net asset value or conversion rate of the broker's statement is stored anywhere. The API capture of `account-data.md` holds positions and balances of the capture instant, not IBKR's end-of-day statement, and no cash event.
- The daily FX figure has no producer on main; `market-bars.md` (not on main) designs it from the IBKR MIDPOINT bar.
- `internal/store/api.go` has 799 lines and cannot take a declaration; the declarations-only package rule is on `docs/design/market-bars.md`, not yet on main.

## Decisions

1. Scope of Flex: the sections of a statement other than `Trades`, which `option-ledger.md` stores: `CashTransaction`, `OpenPosition`, `ChangeInNAV`, `EquitySummaryByReportDateInBase`, and (Decision 2) `ConversionRate`. Each row is stored with all its attributes as sent, as text (a JSON object in one text column), the way trades are (option-ledger Decision 5). Nothing is computed, converted, summed or classified: no dividend figure, no cash balance, no reconciliation of the replayed ledger against `OpenPosition`. Those are stage 2 (Left out).
2. FX: the daily FX job is `fx_refresh` of `docs/design/market-bars.md` Decision 9 (IBKR daily MIDPOINT of USD.JPY, table `obs.fx_rate`, no fallback rate). This design adds no FX source and no FX job; it stores the `ConversionRate` section of the Flex statement (IBKR's rate for its own base-currency statement, per report date and currency pair) as raw rows, so a later check can put IBKR's statement rate beside the stored midpoint. Window default — owner may change: (a) store `ConversionRate` as raw rows and take the daily FX from market-bars only; (b) do not store it. Window default — owner may change: the FX source is the IBKR midpoint of market-bars and nothing else, (a) yes, (b) also a second daily source (ECB reference rates over HTTPS as ADR 0004 names, or FRED `DEXJPUS`); (b) is a new outside service for ECB and needs the owner and a second opinion, and is not designed here.
3. A job of its own, `flex_sections`, not an extension of `option_ledger`: the two designs land in separate pull requests, `option_ledger` decides its windows from its own fetch records (`LatestOkFetch`), and a second kind of record in that table would corrupt them. Cost: one more request pair per query per run, paced by the pacer of `internal/sources/flex` (option-ledger Decision 4, at least 7 seconds between any two requests of the process); the Flex request limits are not verified (open point 2 of `option-ledger.md`). The job takes the same logins, accounts and history start as `option_ledger`, from the same `$OT_HOME/flex.toml` read by `LoadFlexFile`; it adds no key to `flex.toml` and no field to `sources.FlexLogin`. Window default — owner may change: the sections come from the same Flex queries as the trades (`TradeQueries` of the login), so the owner adds the five sections to his existing query in the Client Portal, (a) same query, (b) a separate query per login in a new `section_queries` key (a change of `FlexLogin` and of the loader of option-ledger issue 6, cut after this design).
4. Identity and registry. A row is keyed by `(alias, section, row_key, revision)`. `row_key` is built by a registry of Go data, one entry per section (name and the attributes that make the key, joined by `|`): `CashTransaction` by `transactionID`; `OpenPosition` by `reportDate`, `conid` and `levelOfDetail`; `ChangeInNAV` by `fromDate` and `toDate`; `EquitySummaryByReportDateInBase` by `reportDate`; `ConversionRate` by `reportDate`, `fromCurrency` and `toCurrency`. These attribute names come from `option-ledger.md` and the Flex documentation as remembered by it and are unverified until the first real fetch (as its open point 1); a row that lacks a key attribute is refused and counted (`rows_refused`), never given a guessed key. A later section is one more registry entry and no schema change (the dataset does not enumerate sections), which is the way option-trader's `sections.py` does it.
5. Revision rule, the same as trades: a new key is revision 1; a row whose `attributes_sha256` equals the latest stored revision is not written; a changed row is the next revision. The hash is the recipe of `jobs.FlexAttributesSHA256Func` (declared in `internal/jobs/api.go`, implemented by `internal/jobs/optionledger.AttributesSHA256`); a jobs package cannot import another jobs package, so the job takes the function as an argument and `cmd/ot` passes it. The attributes `accountId` and `acctAlias` are removed before a row is hashed or stored, and a row whose attributes still hold either is an error that writes nothing (option-ledger Decision 5). Window default — owner may change: all other attributes are kept as sent, free text of a cash event included, (a) keep, (b) also drop the free-text attributes `description` and `notes`. The database is the owner's own and not this repository.
6. A statement is mapped to an alias by the `[[account]]` allow-list of `flex.toml`; a statement of an account not mapped is not stored and is counted (`statements_unmapped`). No account number, name or person is in code, a fixture, a log or a record: the alias is the only identity, and a fixture uses the labels `a` and `b` and invented figures.
7. A section absent from a statement is "not collected", not "none": nothing is written for it, the count `sections_found` of the fetch record says how many of the registry's sections the report held, and the run stays `ok`. A report with none of the registry sections at all is also `ok` with `sections_found` 0 and a `degraded` log record naming the login and the query id, never the query's content; the owner sees it before he sees a missing figure.
8. Main only, as for the ledger: the login marked `required` in `flex.toml` is main. If it is missing a token or its fetch fails, the job result is `failed`; any other login that cannot run or fails is a `degraded` log record (a login name and the kind of error) and the result stays `ok`. A non-main alias is never required to work (owner decision 2026-11-09).
9. Store: schema `optionledger`, two insert-only tables, `flex_section_row` and `flex_section_fetch` (a record of each request pair of the section job: login, query id, window, status, error kind and code, `raw_sha256`, parser version, `statements`, `statements_unmapped`, `sections_found`, `rows_parsed`, `rows_new`, `rows_revised`, `rows_unchanged`, `rows_refused`). No new role and no new DSN line: the role `ot_w_optionledger` of option-ledger gets INSERT and SELECT on the two tables and no UPDATE, DELETE or TRUNCATE, and `store.DSNEnvOptionLedger` is the line. Pages never read a base table: the migration makes `rd.flex_section_row_r1` and `rd.flex_section_fetch_r1`, `ot_ro_pages` gets SELECT on the views only, and `ot_w_optionledger` gets SELECT on the views for its Reader (the pattern of `docs/design/account-data.md` Decision 6). The migration number is the next free at cut time and is greater than the number of the option-ledger migration it depends on; 0021 today, re-cut if another lands first.
10. Interface and own glue: the declarations are one declarations-only package `internal/store/flexsection` (`api.go` and its test) because `internal/store/api.go` is at 799 lines; its implementation is the separate package `internal/store/flexsectionpg`, which is the only code that holds SQL. The job reaches the store only through `flexsection.Writer` and `flexsection.Reader`, so a library or another store can replace the implementation. The section registry and the window rule are own code in `internal/jobs/flexsections`: no library fits (a registry of five rows, a date rule), and the job is behind `jobs.NewFlexSectionsJobFunc`; own implementation, because the pacing, the parser and the hash it uses are already behind interfaces and the rest is a table. The window rule is a small copy of the one in `option_ledger` (a jobs package cannot import another jobs package).
11. Delivery rule, binding: nothing in this design reaches notify. A job result or `job_event` carries a job id, a status and an alias only (an alias is a label); no cash amount, balance, net value or position. `internal/notify` imports none of `internal/store/flexsection`, `internal/store/flexsectionpg` or `internal/jobs/flexsections`; the grep of entry 10 checks it. When a figure is built later (dividend income, net value), it goes to a page or a local notice, never to `Sender`; the margin cushion and the assignment cash stay as `account-data.md` Decision 7 says.
12. Credentials and second opinion. Keys only, no value anywhere: the job reads what option-ledger already reads: `$OT_HOME/flex.toml`, whose `token_env` and `account_env` entries name the variables that hold the token and the account number, and the line `OT_PG_DSN_OPTION_LEDGER` of `$OT_HOME/.env`. No new variable, no new file and no new key is introduced. Needs the owner and a second opinion (the advisor answers later; no Advisor line is written here): (a) a read of more of the owner's account data (cash events, net value, positions) through the existing Flex token: the service and the token are those of `option-ledger.md` (itself a second-opinion case), the data classes are new; (b) the edit of his Flex query in the Client Portal to add the five sections, which only he can do (Decision 3); (c) the new tables in his PostgreSQL under an existing role, insert and select only (a write outside the repository). No new outside service, account or credential is added by this design under the window defaults; the (b) answer of Decision 2 would add one and would need the owner first.
13. The job. Id `flex_sections`; it runs after `option_ledger` in `ot run-once` and in the scheduler is anchored at `session_close` plus 75 minutes (`option_ledger` is at plus 60; a statement is produced after the close). Keys of its `job.toml`: `history_from` (the same as `option_ledger`, 2025-12-01) and `overlap_days` (30, so a Flex correction arrives as the next revision). Per login, per query, per window: one request pair, parse, map accounts, build the rows from the registry, write the record and the rows in one transaction (`RecordFetch`: a failure leaves neither). The run key is `{optionledger.flex_section_row, <alias>}` per mapped alias; it is present when the fetch for the alias's window was recorded `ok`.
14. Order of sections in a report is not relied on, and neither is the order of rows within a section: `row_index` is the position in the report and carries no meaning beyond it. Money and quantities are text as sent; no float and no decimal parse in this design.

## Modules

New directories: `internal/store/flexsection` (declarations only), `internal/store/flexsectionpg`, `internal/jobs/flexsections`; new files in `internal/sources/flex` (a package that option-ledger issue 6 creates). No new row. This pull request edits `docs/design/modules.md` in three places and regenerates `arch.toml` with `go run ./scripts/arch` so that `--check` passes: (1) the paragraph that states the declarations-only rule is added to the list under the entry bullet, copied word for word from `origin/design/market-bars` (pull request 768), where `docs/design/market-bars.md` carries the same change; this design waits for that pull request, and whichever lands second merges the two copies of the paragraph (they are the same text, so the conflict is the position only); (2) the entry cell of the store row gains `internal/store/flexsection` (the same cell in which pull request 768 adds `internal/store/bar`; whichever lands second merges the two names); (3) the sentence that lists the implementation packages `cmd/ot` may import gains `internal/store/flexsection`, `internal/store/flexsectionpg` and `internal/jobs/flexsections` (as pull request 768 adds `store/bar`, `store/barpg`, `jobs/barsrefresh` and `jobs/fxrefresh`). No depends cell changes: sources depends on contracts and clock, jobs already names sources, store and clock, store names contracts and gaphistory.

`scripts/arch` follows the rule only after the interface entry `bar-store-api` of `docs/design/market-bars.md` has landed; entry 3 here adds this package to the list `declarationsOnly` in `scripts/arch/main.go` after it. The entries that edit `.go-arch-lint.yml` (3, 4 and 7) are one serial chain in that order, and they edit it after the entries of `docs/design/market-bars.md`, `docs/design/option-ledger.md` and `docs/design/account-data.md` that edit the same file; each rebases on whichever lands first (the file is regenerated by `go run ./scripts/arch`). Entry 10 shares `cmd/ot/runonce.go`, `cmd/ot/wire.go` and `docs/contracts/ot.md` with `optionledger-wire` of `docs/design/option-ledger.md` and with the wire entries of two other designs; it rebases on whichever lands first. Entry 1 shares `contracts/generate.sh`, `internal/contracts/gen/records.go` and `internal/contracts/schema_test.go` with the dataset entries of other designs; the generator rewrites the output on rebase.

| # | issue | module | order |
|---|---|---|---|
| 1 | flexsection-datasets (interface) | contracts | first |
| 2 | flexsection-sources-api (interface) | sources | first |
| 3 | flexsection-store-api (interface) | store, plus repo (licensed pair: `scripts/arch`, `.go-arch-lint.yml`) | after 1; before 4, 7 |
| 4 | flexsection-jobs-api (interface) | jobs, plus repo (licensed pair: `.go-arch-lint.yml`) | after 2, 3; before 7 |
| 5 | flexsection-source-parse | sources | after 2 |
| 6 | flexsection-store-migration | store | after 1, 3 |
| 7 | flexsection-store-impl | store, plus repo (licensed pair: `.go-arch-lint.yml`) | after 3, 4, 6 |
| 8 | flexsections-registry | jobs | after 4 |
| 9 | flexsections-job | jobs | after 4, 8 |
| 10 | flexsections-wire | ot | after 5, 7, 9 |

## Interface

| name | owner and file | declaration |
|---|---|---|
| datasets `optionledger.flex_section_row` r1, `optionledger.flex_section_fetch` r1; generated `contracts.FlexSectionRow`, `contracts.FlexSectionFetch` (names as the generator gives them) | contracts, `contracts/datasets/optionledger.flex_section_row.r1.json` and `optionledger.flex_section_fetch.r1.json` | row: `alias`, `section`, `row_key`, `revision`, `fetch_id`, `row_index`, `attributes` (JSON object as text), `attributes_sha256`, `recorded_at`, key `(alias, section, row_key, revision)`; fetch: `fetch_id`, `login`, `query_id`, `first_date`, `last_date`, `started_at`, `finished_at`, `status` (ok, failed), `error_kind`, `error_code`, `raw_sha256`, `parser_version`, `statements`, `statements_unmapped`, `sections_found`, `rows_parsed`, `rows_new`, `rows_revised`, `rows_unchanged`, `rows_refused`; no field holds an account number |
| `sources.FlexStatement.Sections map[string][]map[string]string` | sources, `internal/sources/api.go` | additive: one field. The key is the section element name, the value its rows as text maps, every attribute as sent; every section of the statement except `Trades` (which keeps its own field and level rule) is in it, the account attributes are still in the rows and the job drops them. `FlexSource`, `FlexRequest`, `FlexReport` and `Trades` are unchanged |
| package `flexsection`: `Row{Alias, Section, RowKey string; Revision int; FetchID string; RowIndex int; Attributes, AttributesSHA256 string; RecordedAt time.Time}`, `Fetch{ID, Login, QueryID string; First, Last contracts.Day; StartedAt, FinishedAt time.Time; Status, ErrorKind, ErrorCode, RawSHA256, ParserVersion string; Statements, StatementsUnmapped, SectionsFound, RowsParsed, RowsRefused int}`, `Insert{New, Revised, Unchanged int}`, `Writer{RecordFetch(ctx, Fetch, []Row) (Insert, error)}`, `Reader{Rows(ctx, alias, section string) ([]Row, error); LatestOkFetch(ctx, login, queryID string) (Fetch, bool, error)}`, `Store{Writer; Reader; Close() error}`, `OpenFunc func(ctx context.Context, dsn string) (Store, error)` | store, `internal/store/flexsection/api.go` (declarations only; the implementation is `internal/store/flexsectionpg.Open`) | `RecordFetch` writes the record and its rows in one transaction and applies the revision rule per row on `(Alias, Section, RowKey)`; the `Revision` of a row given is ignored; `Attributes` that hold `accountId` or `acctAlias` are an error and nothing is written; no rows still writes the record. `Rows` returns the highest revision of every key of the alias and section, sorted by `RowKey`; `LatestOkFetch` is the newest `ok` record of the login and query. It reads the views `rd.flex_section_row_r1` and `rd.flex_section_fetch_r1`, never a base table. Dates are `contracts.Day`; the package imports `context`, `time` and `contracts` only |
| `jobs.FlexSectionsConfig{Logins []FlexRun; Accounts map[string]string; HistoryFrom string; OverlapDays int}`, `jobs.NewFlexSectionsJobFunc func(flexsection.Writer, flexsection.Reader, FlexSectionsConfig, FlexAttributesSHA256Func, clock.Clock) (contracts.Job, error)` | jobs, `internal/jobs/api.go` | additions only; `FlexRun` and `FlexAttributesSHA256Func` are reused as they are. A login with a nil `Source` is accepted and handled at `Run` (Decision 8). The implementation is `internal/jobs/flexsections.New` |

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses the fake clock of the test kit, never sleeps, opens no port, and touches no live broker, no IBKR and no database (a test that needs PostgreSQL carries the build tag `pg` and runs only in GitHub CI). Fixtures use the labels `a` and `b`, invented symbols, invented transaction ids and invented figures. Acceptance is read offline from the files and from git; `BASE=$(git merge-base HEAD origin/main)`. Test runs are listed once, under `## Checked by CI`. No test name starts with `Open`.

### 1 flexsection-datasets (interface)

Module: contracts. Size: S.
Files: `contracts/datasets/optionledger.flex_section_row.r1.json`, `contracts/datasets/optionledger.flex_section_fetch.r1.json`, `contracts/generate.sh`, `internal/contracts/gen/records.go`, `docs/contracts/datasets/optionledger.flex_section_row.md`, `docs/contracts/datasets/optionledger.flex_section_fetch.md`, `internal/contracts/schema_test.go`
Design: docs/design/account-flex-fx.md#interface
Order: first.
Goal: the two datasets of the Interface table, a source and its generated output committed together (the pattern of `optionledger.flex_trade_row`), with `generate.sh` naming both as root types.
Acceptance:
1. `grep -c '"x-primary-key": \["alias", "section", "row_key", "revision"\]' contracts/datasets/optionledger.flex_section_row.r1.json` prints 1; `grep -c '"x-primary-key": \["fetch_id"\]' contracts/datasets/optionledger.flex_section_fetch.r1.json` prints 1.
2. `grep -c '"account' contracts/datasets/optionledger.flex_section_row.r1.json contracts/datasets/optionledger.flex_section_fetch.r1.json | grep -vc ':0'` prints 0 (no property whose name starts with `account`); `grep -c '"section"' contracts/datasets/optionledger.flex_section_row.r1.json` is at least 2 and the section property has no `enum` (a new section is not a schema change): `grep -c 'enum' contracts/datasets/optionledger.flex_section_row.r1.json` prints 0.
3. `grep -c 'sections_found\|rows_refused' contracts/datasets/optionledger.flex_section_fetch.r1.json` is at least 3.
4. `grep -c 'FlexSectionRow\|FlexSectionFetch' internal/contracts/gen/records.go` is at least 2; `grep -c 'flex_section_row\|flex_section_fetch' contracts/generate.sh` is at least 4; `grep -c 'flex_section' internal/contracts/schema_test.go` is at least 2.
5. `git diff --name-only $BASE | grep -vxF -e contracts/datasets/optionledger.flex_section_row.r1.json -e contracts/datasets/optionledger.flex_section_fetch.r1.json -e contracts/generate.sh -e internal/contracts/gen/records.go -e docs/contracts/datasets/optionledger.flex_section_row.md -e docs/contracts/datasets/optionledger.flex_section_fetch.md -e internal/contracts/schema_test.go | grep -c .` prints 0.
Checked by CI: ci-go; the generated-code check of `contracts/generate.sh`; the design verdict.

### 2 flexsection-sources-api (interface)

Module: sources. Size: S.
Files: `internal/sources/api.go`, `internal/sources/flex_section_api_test.go`, `docs/contracts/sources.md`
Design: docs/design/account-flex-fx.md#interface
Order: first.
Goal: the additive field `Sections` of `FlexStatement`, declarations only, with a contract test from both sides (a fake `FlexSource` that returns a statement with sections, and a caller that reads them) in a new file; `docs/contracts/sources.md` records that every section except `Trades` is returned as text maps and that the account attributes are still in the rows.
Acceptance:
1. `grep -c 'Sections *map\[string\]\[\]map\[string\]string' internal/sources/api.go` prints 1; `grep -c 'Trades *\[\]map\[string\]string' internal/sources/api.go` prints 1 (the field is kept).
2. `grep -c 'Fetch(ctx context.Context, req FlexRequest) (FlexReport, error)' internal/sources/api.go` prints 1 (the interface is not widened); `grep -c 'float32\|float64\|os.Getenv' internal/sources/api.go` prints 0.
3. `grep -c 'Sections' internal/sources/flex_section_api_test.go` is at least 2 and `grep -c 'sources.FlexSource' internal/sources/flex_section_api_test.go` is at least 1.
4. `grep -c 'Sections' docs/contracts/sources.md` is at least 1.
5. `git diff --name-only $BASE | grep -vxF -e internal/sources/api.go -e internal/sources/flex_section_api_test.go -e docs/contracts/sources.md | grep -c .` prints 0.
Checked by CI: ci-go; the design verdict.

### 3 flexsection-store-api (interface)

Module: store, plus repo (licensed pair: `scripts/arch/main.go`, `scripts/arch/main_test.go`, `.go-arch-lint.yml`). Size: S.
Files: `internal/store/flexsection/api.go`, `internal/store/flexsection/api_test.go`, `docs/contracts/store.md`, `scripts/arch/main.go`, `scripts/arch/main_test.go`, `.go-arch-lint.yml`
Design: docs/design/account-flex-fx.md#interface
Order: after 1; before 4 and 7 (the three edit `.go-arch-lint.yml` in that order); cut only after pull request 768 (`bar-store-api` of `docs/design/market-bars.md`) has merged, because `scripts/arch/main.go` is edited by it first.
Goal: the declarations of the Interface table in the new declarations-only package, with a contract test from both sides (a fake `Store` that implements `Writer` and `Reader`, and a caller that writes a record with rows and reads them back), the package added to the list `declarationsOnly` of `scripts/arch/main.go` and to its test, the component regenerated in `.go-arch-lint.yml`, and `docs/contracts/store.md` recording the revision rule of Decision 5 and the views of Decision 9.
Acceptance:
1. `grep -c 'type Writer interface\|type Reader interface\|type Store interface\|type OpenFunc ' internal/store/flexsection/api.go` prints 4; `grep -c 'RecordFetch\|LatestOkFetch\|Rows(' internal/store/flexsection/api.go` is at least 3.
2. The package holds declarations only: `ls internal/store/flexsection` lists exactly `api.go` and `api_test.go`; `grep -c '^func ' internal/store/flexsection/api.go` prints 0.
3. The import block names only `context`, `time` and the contracts package: `grep -c 'database/sql\|pgx\|internal/store"\|internal/jobs\|internal/broker' internal/store/flexsection/api.go` prints 0; `grep -c 'float32\|float64' internal/store/flexsection/api.go` prints 0.
4. `grep -c 'internal/store/flexsection' scripts/arch/main.go` is at least 1 and `grep -c 'internal/store/flexsection' .go-arch-lint.yml` is at least 1; `grep -c 'var _ flexsection.Writer\|var _ flexsection.Reader' internal/store/flexsection/api_test.go` prints 2.
5. `grep -c 'flex_section_row_r1\|accountId' docs/contracts/store.md` is at least 2.
6. `git diff --name-only $BASE | grep -vxF -e internal/store/flexsection/api.go -e internal/store/flexsection/api_test.go -e docs/contracts/store.md -e scripts/arch/main.go -e scripts/arch/main_test.go -e .go-arch-lint.yml | grep -c .` prints 0.
Checked by CI: ci-go; `go run ./scripts/arch --check` printing `arch: ok`; the design verdict.

### 4 flexsection-jobs-api (interface)

Module: jobs, plus repo (licensed pair: `.go-arch-lint.yml`). Size: S.
Files: `internal/jobs/api.go`, `internal/jobs/flex_section_api_test.go`, `internal/jobs/flexsections/doc.go`, `docs/contracts/jobs.md`, `.go-arch-lint.yml`
Design: docs/design/account-flex-fx.md#interface
Order: after 2 and 3; before 7 (`.go-arch-lint.yml` chain 3, 4, 7).
Goal: `FlexSectionsConfig` and `NewFlexSectionsJobFunc` of the Interface table, additions only, with a contract test from both sides (a fake constructor and a caller that builds the job with fakes of `flexsection.Writer` and `flexsection.Reader`) in a new file; the package `flexsections` as a documented stub; `docs/contracts/jobs.md` records the job id, the run key, the schedule anchor and the delivery rule of Decisions 11 and 13.
Acceptance:
1. `grep -c 'type FlexSectionsConfig struct\|type NewFlexSectionsJobFunc ' internal/jobs/api.go` prints 2; `grep -c 'flexsection.Writer' internal/jobs/api.go` is at least 1; `grep -c 'FlexAttributesSHA256Func' internal/jobs/api.go` is at least 2 (declared once, used once more).
2. `grep -c 'type FlexRun struct' internal/jobs/api.go` prints 1 (`FlexRun` is reused, not copied); `grep -c 'float32\|float64\|os.Getenv' internal/jobs/api.go` prints 0.
3. `grep -c 'var _ jobs.NewFlexSectionsJobFunc' internal/jobs/flex_section_api_test.go` prints 1; `grep -c 'package flexsections' internal/jobs/flexsections/doc.go` prints 1.
4. `grep -c 'flex_sections' docs/contracts/jobs.md` is at least 2 and `grep -c 'jobs/flexsections' .go-arch-lint.yml` is at least 1.
5. `git diff --name-only $BASE | grep -vxF -e internal/jobs/api.go -e internal/jobs/flex_section_api_test.go -e internal/jobs/flexsections/doc.go -e docs/contracts/jobs.md -e .go-arch-lint.yml | grep -c .` prints 0.
Checked by CI: ci-go; `go run ./scripts/arch --check` printing `arch: ok`; the design verdict.

### 5 flexsection-source-parse

Module: sources. Size: M.
Files: `internal/sources/flex/parse.go`, `internal/sources/flex/sections.go`, `internal/sources/flex/sections_test.go`, `internal/sources/flex/testdata/report_sections.xml`
Design: docs/design/account-flex-fx.md#decisions (Decisions 1, 4, 7)
Order: after 2; cut only after `docs/design/option-ledger.md` issue 6 has landed (it creates `parse.go`, which this entry edits by one call).
Goal: the parser of option-ledger issue 6 fills `FlexStatement.Sections`: every child section element of a statement other than `Trades` becomes a list of text maps, one per row element, every attribute as sent, under the section element's name; the rules of that parser stay (no document type or entity declaration, no UTF-16 or UTF-32 body, body size limit); the code is in the new file `sections.go` and `parse.go` calls it once.
Acceptance:
1. `grep -c 'Sections' internal/sources/flex/parse.go internal/sources/flex/sections.go | grep -vc ':0'` prints 2; `grep -c 'DOCTYPE\|ENTITY' internal/sources/flex/parse.go` is at least 1 (the refusal of option-ledger issue 6 is kept).
2. `internal/sources/flex/sections.go` has no import of `os`, `net/http`, `internal/jobs`, `internal/store` or `internal/contracts/gen`: `grep -c '"os"\|net/http\|internal/jobs\|internal/store\|contracts/gen' internal/sources/flex/sections.go` prints 0; `grep -c 'float32\|float64\|strconv.ParseFloat' internal/sources/flex/sections.go` prints 0.
3. The fixture is invented and has no account-shaped id: `grep -c 'accountId="[A-Z][0-9]\{5,\}"' internal/sources/flex/testdata/report_sections.xml` prints 0; `grep -c 'CashTransaction\|OpenPosition\|ChangeInNAV\|EquitySummaryByReportDateInBase\|ConversionRate' internal/sources/flex/testdata/report_sections.xml` is at least 5.
4. Each of these functions is in `internal/sources/flex/sections_test.go` once: `TestSectionsKeptAsText`, `TestSectionRowsKeepEveryAttribute`, `TestTradesFieldUnchanged`, `TestNoSectionsGivesEmptyMap`, `TestEntityInSectionStillRefused`.
5. `git diff --name-only $BASE | grep -vxF -e internal/sources/flex/parse.go -e internal/sources/flex/sections.go -e internal/sources/flex/sections_test.go -e internal/sources/flex/testdata/report_sections.xml | grep -c .` prints 0; each new file is under 600 lines.
Checked by CI: ci-go (`go test ./internal/sources/... -race -count=1`).

### 6 flexsection-store-migration

Module: store. Size: S.
Files: `migrations/0021_flex_section.sql`
Design: docs/design/account-flex-fx.md#decisions (Decision 9)
Order: after 1 and 3; cut only after the migration of `docs/design/option-ledger.md` issue 8 (which creates schema `optionledger` and the role `ot_w_optionledger`) has landed, and its number must be greater than that one: 0021 is the next free number today and is re-cut if another migration lands first.
Goal: the two tables, their views and their grants, and nothing else.
Acceptance:
1. `grep -c 'CREATE TABLE optionledger.flex_section_row\|CREATE TABLE optionledger.flex_section_fetch' migrations/0021_flex_section.sql` prints 2; `grep -c 'PRIMARY KEY (alias, section, row_key, revision)' migrations/0021_flex_section.sql` prints 1.
2. `grep -ci 'GRANT .*\(UPDATE\|DELETE\|TRUNCATE\)' migrations/0021_flex_section.sql` prints 0; `grep -ci 'CREATE ROLE\|CREATE SCHEMA' migrations/0021_flex_section.sql` prints 0 (no new role and no new schema); `grep -c 'GRANT INSERT, SELECT ON optionledger.flex_section_row\|GRANT INSERT, SELECT ON optionledger.flex_section_fetch' migrations/0021_flex_section.sql` prints 2.
3. `grep -c 'CREATE VIEW rd.flex_section_row_r1\|CREATE VIEW rd.flex_section_fetch_r1' migrations/0021_flex_section.sql` prints 2; `grep -c 'GRANT SELECT ON rd\.flex_section_.*TO ot_ro_pages' migrations/0021_flex_section.sql` is at least 1; `grep -v '^--' migrations/0021_flex_section.sql | grep ot_ro_pages | grep -c 'optionledger\.'` prints 0 (no grant to pages on a base table).
4. `grep -ci 'account_id\|accountid\|account_number' migrations/0021_flex_section.sql` prints 0.
5. `git diff --name-only $BASE | grep -vxF -e migrations/0021_flex_section.sql | grep -c .` prints 0.
Checked by CI: ci-go; `go test -tags pg ./internal/store/...` (the `pg` job, GitHub CI only). The owner runs `ot migrate` (the owner's command, not an agent's).

### 7 flexsection-store-impl

Module: store, plus repo (licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `internal/store/flexsectionpg/store.go`, `internal/store/flexsectionpg/queries.go`, `internal/store/flexsectionpg/store_test.go`, `internal/store/flexsectionpg/pg_test.go`, `.go-arch-lint.yml`
Design: docs/design/account-flex-fx.md#decisions (Decisions 5, 9, 10)
Order: after 3, 4 and 6 (`.go-arch-lint.yml` chain 3, 4, 7).
Goal: `flexsectionpg.Open` returns a `flexsection.Store` over the DSN of `store.DSNEnvOptionLedger`: the writer applies the revision rule inside one transaction (the highest revision of the key is read and the next written there; `ON CONFLICT DO NOTHING` is a second guard), refuses a row whose attributes hold `accountId` or `acctAlias`, and the reader reads the two views only.
Acceptance:
1. `grep -c 'flexsection.Store' internal/store/flexsectionpg/store.go` is at least 1; `grep -c 'func Open' internal/store/flexsectionpg/store.go` prints 1.
2. `grep -c 'rd.flex_section_row_r1\|rd.flex_section_fetch_r1' internal/store/flexsectionpg/queries.go` is at least 2 and no query reads a base table: `grep -c 'FROM optionledger\.' internal/store/flexsectionpg/queries.go` prints 0; `grep -ci 'UPDATE \|DELETE \|TRUNCATE' internal/store/flexsectionpg/queries.go` prints 0.
3. `grep -c 'ON CONFLICT DO NOTHING' internal/store/flexsectionpg/queries.go` is at least 1; `grep -c 'accountId\|acctAlias' internal/store/flexsectionpg/store.go` is at least 2.
4. These functions are each present once in `internal/store/flexsectionpg/store_test.go`: `TestNewKeyIsRevisionOne`, `TestEqualHashNotWritten`, `TestChangedHashNextRevision`, `TestFetchAndRowsOneTransaction`, `TestRowWithAccountKeyRefused`, `TestReaderHighestRevision`, `TestNoAccountColumn`; `grep -c '^//go:build pg' internal/store/flexsectionpg/pg_test.go` prints 1.
5. `grep -rn 'internal/jobs\|internal/compute\|internal/contracts/gen' internal/store/flexsectionpg | grep -c .` prints 0; `grep -c 'store/flexsectionpg' .go-arch-lint.yml` is at least 1; each new file is under 600 lines.
6. `git diff --name-only $BASE | grep -vxE 'internal/store/flexsectionpg/[a-z_]+\.go|\.go-arch-lint\.yml' | grep -c .` prints 0.
Checked by CI: ci-go; `go test -tags pg ./internal/store/...`; `go run ./scripts/arch --check`; the design verdict and the advisor's verdict (Decision 12).

### 8 flexsections-registry

Module: jobs. Size: S.
Files: `internal/jobs/flexsections/registry.go`, `internal/jobs/flexsections/registry_test.go`
Design: docs/design/account-flex-fx.md#decisions (Decision 4)
Order: after 4.
Goal: the registry of Decision 4 as a Go table (section name and key attributes) and the function that builds a `row_key`: the key attributes in registry order joined by `|`, and a refusal (a false result) when any is missing or empty. A new section is one more row of the table.
Acceptance:
1. `grep -c 'CashTransaction\|OpenPosition\|ChangeInNAV\|EquitySummaryByReportDateInBase\|ConversionRate' internal/jobs/flexsections/registry.go` is at least 5.
2. `grep -c 'transactionID\|reportDate\|conid\|fromDate\|toDate\|fromCurrency\|toCurrency' internal/jobs/flexsections/registry.go` is at least 7; `grep -c 'os.Getenv\|net/http\|database/sql' internal/jobs/flexsections/registry.go` prints 0.
3. Each of these functions is in `internal/jobs/flexsections/registry_test.go` once: `TestRowKeyJoinsKeyAttributes`, `TestRowKeyMissingAttributeRefused`, `TestRowKeyEmptyAttributeRefused`, `TestRegistryNamesUnique`, `TestUnknownSectionNotInRegistry`.
4. `grep -c 'func ' internal/jobs/flexsections/registry.go` is at most 3 (a table and one key function, no logic beyond it); each new file is under 600 lines.
5. `git diff --name-only $BASE | grep -vxF -e internal/jobs/flexsections/registry.go -e internal/jobs/flexsections/registry_test.go | grep -c .` prints 0.
Checked by CI: ci-go (`go test ./internal/jobs/... -race -count=1`).

### 9 flexsections-job

Module: jobs. Size: M.
Files: `internal/jobs/flexsections/job.go`, `internal/jobs/flexsections/window.go`, `internal/jobs/flexsections/job.toml`, `internal/jobs/flexsections/job_test.go`, `internal/jobs/flexsections/window_test.go`, `internal/jobs/flexsections/testdata/report_a.json`
Design: docs/design/account-flex-fx.md#decisions (Decisions 3 to 8, 11, 13)
Order: after 4 and 8; its tests use a fake `sources.FlexSource`, fakes of `flexsection.Writer` and `Reader`, and a fake hash function, so it needs no implementation entry.
Goal: `flexsections.New` as `jobs.NewFlexSectionsJobFunc`: per login, query and window one fetch, mapping of accounts to aliases (an unmapped statement counted and dropped), the rows built from the registry with `accountId` and `acctAlias` removed before the injected hash, a row without a key refused and counted, the record and rows written with `RecordFetch`, the window rule (history start, overlap of `overlap_days`), the main-only rule of Decision 8, and a result that carries a job id, a status and an alias only.
Acceptance:
1. `grep -c 'flex_sections' internal/jobs/flexsections/job.toml` is at least 1; `grep -c 'history_from\|overlap_days\|session_close' internal/jobs/flexsections/job.toml` is at least 3; `grep -c 'offset *= *"1h15m"' internal/jobs/flexsections/job.toml` prints 1.
2. The job imports interfaces only and no other jobs package: `grep -rn 'internal/jobs/\|internal/store/flexsectionpg\|internal/sources/flex"' internal/jobs/flexsections | grep -c .` prints 0; `grep -rn 'os.Getenv\|time.Now()\|time.Sleep' internal/jobs/flexsections | grep -c .` prints 0 (time comes from the injected clock); `grep -rn 'notify\|telegram' internal/jobs/flexsections | grep -c .` prints 0.
3. Each of these functions is in `internal/jobs/flexsections/job_test.go` or `window_test.go` once: `TestRequiredLoginFailureFailsResult`, `TestNonMainLoginDegradedNotFailed`, `TestAccountIdDroppedBeforeHash`, `TestUnmappedStatementCountedNotStored`, `TestSectionAbsentIsNotCollected`, `TestNoSectionsAtAllIsOkDegraded`, `TestRowWithoutKeyRefused`, `TestWindowOverlapThirtyDays`, `TestWindowStartsAtHistoryFrom`, `TestResultCarriesNoFigures`, `TestRecordFailureWritesNoRows`.
4. The fixture is invented and has no account-shaped id: `grep -c '[A-Z][0-9]\{6,\}' internal/jobs/flexsections/testdata/report_a.json` prints 0.
5. `git diff --name-only $BASE | grep -vxE 'internal/jobs/flexsections/(job|window)(_test)?\.go|internal/jobs/flexsections/job\.toml|internal/jobs/flexsections/testdata/report_a\.json' | grep -c .` prints 0; each new file is under 600 lines.
Checked by CI: ci-go (`go test ./internal/jobs/... -race -count=1`); `go run ./scripts/arch --check`.

### 10 flexsections-wire

Module: ot. Size: M.
Files: `cmd/ot/flexsections.go`, `cmd/ot/flexsections_test.go`, `cmd/ot/runonce.go`, `cmd/ot/runonce_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `docs/contracts/ot.md`
Design: docs/design/account-flex-fx.md#decisions (Decisions 3, 8, 11, 12, 13)
Order: after 5, 7 and 9; same `wire.go`, `runonce.go` and `docs/contracts/ot.md` as `optionledger-wire` of `docs/design/option-ledger.md` and other open designs; rebase on whichever lands first.
Goal: wiring only. The registry entry `flex_sections`, after `option_ledger`: the store opened with `store.DSNEnvOptionLedger` through `flexsectionpg.Open`, the logins, accounts and tokens built exactly as the `option_ledger` wiring builds them (one `flex.New` per login, the token read from the variable `flex.toml` names, a login whose variable is empty passed with a nil `Source`), `optionledger.AttributesSHA256` passed as the hash function, a missing `flex.toml` leaving the job out and the other jobs as they were. No new variable, file or key.
Acceptance:
1. `grep -c 'flex_sections' cmd/ot/runonce.go` is at least 1; `grep -c 'flexsectionpg.Open\|AttributesSHA256' cmd/ot/flexsections.go` is at least 2.
2. `grep -c 'os.Getenv' cmd/ot/flexsections.go` prints 0 (the wiring's existing environment reader is used); `grep -rn 'IBKR_FLEX' cmd docs/contracts | grep -c .` prints 0; `grep -rn 'flexsection\|flexsections' internal/notify | grep -c .` prints 0 (Decision 11).
3. `grep -c 'flex_sections' docs/contracts/ot.md` is at least 1 and `grep -c 'flex.toml' docs/contracts/ot.md` is at least 1; the contract text names no variable other than the existing ones: `git diff $BASE -- docs/contracts/ot.md | grep '^+' | grep 'OT_PG_DSN_' | grep -vc 'OT_PG_DSN_OPTION_LEDGER'` prints 0.
4. Each of these functions is in `cmd/ot/flexsections_test.go` once: `TestFlexSectionsInRegistry`, `TestFlexSectionsAfterOptionLedger`, `TestLoginWithoutTokenLeftOutSections`, `TestRequiredLoginWithoutTokenFailsSections`, `TestMissingFlexFileKeepsOtherJobsSections`, `TestSectionsValuesNeverPrinted`, `TestSectionsResultShape`.
5. Each new file is under 600 lines; `git diff --name-only $BASE | grep -vxE 'cmd/ot/(flexsections|runonce|wire)(_test)?\.go|docs/contracts/ot\.md' | grep -c .` prints 0.
Checked by CI: ci-go (`go test ./cmd/ot -race -count=1`), golangci-lint, `go run ./scripts/arch --check`. The owner edits his Flex query in the Client Portal to add the five sections (Decision 3), runs `ot migrate`, then `ot run-once flex_sections`, and compares the stored rows with the statement on the broker's screen.

## Checked by CI

- `go test ./internal/sources/... ./internal/store/... ./internal/jobs/... ./cmd/ot -race -count=1` (ci-go: parallel, `t.TempDir()`, fake clock, no sleep, no live broker, no network).
- `go test -tags pg ./internal/store/...` (the `pg` job, GitHub CI only).
- `go run ./scripts/arch --check` printing `arch: ok` (entries 3, 4, 7, 9 and 10); the generated-code check of `contracts/generate.sh` (entry 1). Both run Go tools that the acceptance checker refuses.
- The design verdict on every entry, and the advisor's verdict on entries 7 and 10 (Decision 12).

## Left out

- The daily FX job: not built here; it is `fx_refresh` of `docs/design/market-bars.md` (Decision 9 there, pull request 768, not on main). If that design is dropped or changed, this design's FX half is the Flex `ConversionRate` rows only, and the FX job returns to the alignment table as missing. The 15 s FX stream, the ECB cross-check, the shadow period and `fx_override` of ADR 0004 are not built (the stream dropped by the owner 2026-11-09, the rest named in `market-bars.md` Left out).
- Trades, expiry, assignment, exercise, the history import and the replay: `docs/design/option-ledger.md`; the API executions, positions and balances: `docs/design/account-data.md`. Nothing here reads or changes them.
- Figures over the stored rows, all stage 2 and each its own later design: dividend and interest income, a cash balance, net value and its change, the check of the replayed ledger against `OpenPosition`, the check of IBKR's conversion rate against `obs.fx_rate`, any FX conversion of a total (option-ledger Decision 11 and `option-positions.md` keep "no conversion" until a design wires FX). `ot` subcommands and pages that show the rows; a page block or local notice for them; Telegram text about them: never.
- Flex sections not in the registry (for example `FIFOPerformanceSummaryInBase`, `StmtFunds`, `OpenDividendAccrual`): a later design adds them as registry rows; the dataset needs no change for that (Decision 4).
- The raw XML of a report: only `raw_sha256` of the fetch record is kept, as for trades; a refetch of the window gives the report again.
- A second query per login (`section_queries`): the answer (b) of Decision 3; it would change `sources.FlexLogin` and the loader of option-ledger issue 6, and is not designed.
- A second daily FX source (ECB, FRED `DEXJPUS`): the answer (b) of Decision 2; a new outside service, needs the owner first.
- Rejected: an extension of the `option_ledger` job (shared fetch records and windows, Decision 3); storing sections in `optionledger.flex_fetch` or `flex_trade_row` (it would feed the trade job's window rule and the ledger replay); a new role or DSN line for the two tables (one schema, one role, Decision 9); a typed table per section (every section's attributes are unverified until the first real fetch, and the trade rows are stored the same way); the section registry in a file the owner edits (it would be a new file and a new place for a typo, and the registry is five rows); an `UPDATE` when Flex corrects a row (a revision, highest wins).
- Lock-ins, named: the key `(alias, section, row_key, revision)` and the shape of each `row_key` are permanent once rows are stored (a changed key attribute for a section orphans its history); the alias is the identity of every row; the section names are Flex's own and are not renamed.
- Open points: (1) the key attributes of Decision 4 and the section names are read from the first real fetch and change the registry and its fixture if they differ; (2) whether one Flex query can carry all five sections and the trade section together, and the largest window it answers, are not verified; (3) which attributes of a cash event hold free text with personal data is not known until the first fetch (Decision 5, the (b) answer).
