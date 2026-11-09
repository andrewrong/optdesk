# Account Flex sections and daily FX: cash events, open positions, net asset value and IBKR conversion rates from the same Flex fetch as the option ledger, stored; the daily FX job stays in market-bars

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 2 (answers the read-only attack, kept in the window's scratch: no second job, the sections are written by `option_ledger` from its own fetch; the parser reads Flex's container shape; the OpenPosition key carries the lot; no hard-coded migration number; the waits are stated by name)

Status: draft, pull request 792. This design is a follow-up of `docs/design/option-ledger.md`. It builds on main at 64a9bdb (pull request 768, market-bars, has merged: its design text and the declarations-only rule in `docs/design/modules.md` are on main): `sources.FlexSource`, `FlexStatement`, `FlexLogin`, `LoadFlexFileFunc` (`internal/sources/api.go`), `jobs.OptionLedgerStores`, `OptionLedgerConfig`, `NewOptionLedgerJobFunc`, `FlexRun`, `FlexAttributesSHA256Func` (`internal/jobs/api.go`, 164 lines), `store.OptionLedgerWriter` and `OptionLedgerReader` (`internal/store/api.go`, 799 lines), the datasets `optionledger.flex_trade_row` and `optionledger.flex_fetch`, `store.DSNEnvOptionLedger`, migrations up to `0020_events.sql`.
Waits, plainly. Entries 5, 6, 7, 8 and 9 edit or extend packages that option-ledger issues create and that are not on main: issue 6 (`internal/sources/flex`, entry 5 waits for it), issue 8 (schema `optionledger`, the role `ot_w_optionledger` and its migration; entry 6 waits for it), issue 9 (`internal/jobs/optionledger/job.go`; entry 8 waits for it) and issue 12 (`cmd/ot/optionledger.go`; entry 9 waits for it). Entries 3, 4 and 7 use a declarations-only package. The rule is on main as text (`docs/design/modules.md`, from pull request 768), but its code is not: `scripts/arch` has no list `declarationsOnly` and gives an entry-listed package to sub-packages only, so a root component such as `jobs` cannot yet import `store/flexsection`. Entries 3 and 4 therefore wait for the issue `bar-store-api` of `docs/design/market-bars.md` (it changes `scripts/arch/main.go`), and entry 7 follows them. Entries 1 and 2 wait for nothing. An entry that waits names in its Files line only paths of its own; where it edits a file an earlier issue creates, the file is in its Files line and is checked at cut time.
Needs the owner: no owner-only path is edited (`contracts/proto/**`, `transport.py`, `.github/**`, `docs/design/ratchet.json`, `services.toml` stay as they are). Three points are the owner's, in Decision 11. No ratchet ceiling is needed: the store interface goes in the new declarations-only package `internal/store/flexsection`, not in `internal/store/api.go`.
Source: owner 2026-11-09: all account data before cut-over; the ledger truth is IBKR Flex; the 15 s FX stream is dropped (daily FX only); only the main IBKR account must work; the margin cushion and the assignment cash never go to Telegram; own glue code sits behind an interface so a library can replace it; the front end renders data and computes nothing. Plan v2.5 section 11 rows W5-6 "FX job" and "Flex job" as written in `docs/design/alignment-v25.md`. `docs/design/account-data.md` Left out deferred Flex; `docs/design/option-ledger.md` Left out says the other Flex sections come "as registry entries" in a later design: this is it.
Read on main at 64a9bdb: `AGENTS.md`, `docs/design/modules.md`, `alignment-v25.md`, `account-data.md`, `option-ledger.md`, `docs/adr/0004-markets-calendars-fx.md`, `internal/sources/api.go`, `internal/jobs/api.go`, `internal/store/api.go`, `contracts/datasets/optionledger.*.r1.json`, `contracts/generate.sh`, `docs/contracts/ot.md`, `migrations` (file names). Also read, merged by pull request 768: `docs/design/market-bars.md` and its change to `docs/design/modules.md`. option-trader was not read; the Flex element and attribute names below are from `option-ledger.md` and the Flex statement layout as the attack described it, and are checked by the first real fetch (open point 1).

## Goal

The Flex statement that `option_ledger` already fetches each day carries more than trades: cash events (deposits, withdrawals, dividends, interest, fees, withholding tax), the open positions IBKR reports, the net asset value and its change, and IBKR's daily conversion rates. This design has `option_ledger` keep those sections too, as sent, append-only, keyed by alias and by the fetch that brought them, and says, for each statement, which of the expected sections the query lacked. A Go `Reader` makes the rows readable for stage 2. The daily FX job is not rebuilt: it is `fx_refresh` of `docs/design/market-bars.md`. This design builds no figure, page, notice or `ot` command.

## What main holds today

- The Flex client, the `flex.toml` loader, the `option_ledger` job and the tables of schema `optionledger` are declared (interfaces, datasets for trades and fetches) and not built. `sources.FlexStatement` carries `Trades` and counts the rows of other levels in `OtherLevels`; nothing else of a report survives the parser that option-ledger issue 6 will write.
- No cash event, open position, net asset value or conversion rate of the broker's statement is stored. `account-data.md` stores the API capture (positions and balances at the capture instant), not the end-of-day statement, and no cash event.
- The daily FX figure has no producer on main; `market-bars.md` (not on main) designs it.
- `internal/store/api.go` has 799 lines and cannot take a declaration.

## Decisions

1. Scope: the sections of a statement other than `Trades`: cash transactions, open positions, change in net asset value, equity summary by report date, and conversion rates. Rows are stored with all attributes as sent, as text (one JSON object in a text column), the way trades are (option-ledger Decision 5). Nothing is computed, converted, summed or reconciled; those are stage 2 (Left out).
2. No second job. `option_ledger` makes the one fetch per login, query and window (option-ledger Decision 4); the sections are parsed from that same report and written by that job, in the same run, into a section-row table. The fetch reference of those rows is minted by the section step from what it already holds, not read from the ledger's store: `<login>:<query_id>:<first>:<last>:<fetched_at>`, the window of the request and `FlexReport.FetchedAt` in UTC (RFC 3339), so no landed interface has to hand out an id (`RecordFlexFetch` returns counts only and `internal/store/api.go` is at 799 lines). One request pair, one pacer cost, one window rule, one job id, one place where a login fails. This design adds no job, no fetch record and no `job.toml`. The section step is switched on by a store given to the job (`OptionLedgerStores.Sections`, nil means off), so the trade path is unchanged when it is nil.
3. Flex's container shape is what the parser reads, and the fixture has it. A statement holds, as direct children, container elements that hold row elements (for example `CashTransactions` holding `CashTransaction`, `OpenPositions` holding `OpenPosition`, `ConversionRates` holding `ConversionRate`, `EquitySummaryInBase` holding `EquitySummaryByReportDateInBase`), and single elements whose attributes are one row (`ChangeInNAV`). The parser rule is generic and has no list of names: for each direct child of a statement other than `Trades`, if it has child elements, each child element is a row under the child's element name; if it has none, its own attributes are one row under its own name. `FlexStatement.Sections` is keyed by the row element name. Names that this decision gives are unverified (open point 1); a fixture that disagrees with a real fetch changes the fixture and the registry, not the rule.
4. A section the statement lacks is recorded as absent, not as ok. For each mapped statement the job writes one status row per expected section (the registry of Decision 5): `present` with the row count, or `absent`. An absent section also gives a `degraded` log record naming the login and the section, never a value. The run result is unchanged by an absent section: `ok` for the trade path, and the status rows are the notice, readable by `Reader.Statuses` so a later page or local notice shows "the Flex query lacks CashTransaction". A statement with none of the expected sections writes five `absent` rows, so the owner's missing query edit is visible and is not a silent ok. Window default — owner may change: absent sections only write the status rows and the log record, (a) yes, (b) also fail the run for the required (main) login.
5. Identity and registry. A row is keyed by `(alias, section, row_key, revision)`, with the fetch reference of Decision 2 of the first fetch that wrote that revision as the column `fetch_id`, not as part of the key, so a refetch of the same window finds the same keys and the revision rule works. The registry is a Go table in `internal/jobs/optionledger`: per section its row element name and its key attributes, required and optional, in order; `row_key` joins them with `|` (an optional attribute that is absent is the empty text). Keys, each reviewed for the case that two real rows would collide: cash transactions by `transactionID` (IBKR's own id, one per event; required); change in NAV by `fromDate` and `toDate` (one row per statement period); equity summary by `reportDate` (one row per day in base currency); conversion rates by `reportDate`, `fromCurrency` and `toCurrency`. Open positions are many rows per instrument and day, one per lot when the query asks for lots, so the key carries the lot: `reportDate`, `levelOfDetail`, `conid`, then the optional `openDateTime` and `originatingTransactionID`. Chosen because those two attributes identify a lot in Flex's own terms and neither depends on the order of the report; the alternative, an ordinal within the statement, changes when Flex reorders rows and would write a revision for every row; a `SUMMARY` row has both empty, a `LOT` row with both empty is refused as ambiguous. In every section a row whose required attribute is missing or empty is refused and counted, and two rows of one statement with the same key are both refused and counted (never merged or overwritten): a key is never guessed. A later section is one registry row and no schema change (the dataset does not enumerate sections).
6. Revision rule, the same as trades: a new key is revision 1; a row whose `attributes_sha256` equals the latest stored revision is not written; a changed row is the next revision. The hash is `jobs.FlexAttributesSHA256Func`, already used by the job. `accountId` and `acctAlias` are removed before a row is hashed or stored and a row that still holds either is an error that writes nothing. Window default — owner may change: all other attributes are kept as sent, free text of a cash event included, (a) keep, (b) also drop `description` and `notes`. The database is the owner's own and not this repository.
7. Accounts, alias and main: the job's own mapping of statement account to alias (`OptionLedgerConfig.Accounts`) is used; an unmapped statement writes no section row and no status row and is counted by the job as for trades. No account number, name or person appears in code, a fixture, a log or a record; fixtures use the labels `a` and `b` and invented figures. Only main must work: the required login's failure fails the result as option-ledger Decision 3 says; a non-main login's failure is a `degraded` record and the result stays `ok`. The section step adds no other failure to the result than a failed write of the section rows for the required login.
8. Store: schema `optionledger`, two insert-only tables, `flex_section_row` and `flex_section_status`, no new role and no new DSN line. `ot_w_optionledger` gets INSERT and SELECT on them and no UPDATE, DELETE or TRUNCATE; views `rd.flex_section_row_r1` and `rd.flex_section_status_r1`; `ot_ro_pages` gets SELECT on the views only; `ot_w_optionledger` gets SELECT on the views for its Reader (the pattern of `account-data.md` Decision 6). The migration is named `NNNN_flex_section.sql`: the next free number at cut time, greater than every migration on origin/main at that time and than the number of the option-ledger migration it depends on; no number is written in this design. If another migration takes the number first, the file is renamed to the next free one on rebase and nothing else changes. Acceptance reads it by the glob `migrations/*_flex_section.sql`.
9. Interface and own glue: the declarations are the declarations-only package `internal/store/flexsection` (`api.go` and its test) and its implementation is `internal/store/flexsectionpg`, the only code with SQL, so another store or a library can replace it. The registry and the section step are own code in `internal/jobs/optionledger`: no library fits (a table and a loop over rows), and they sit behind `jobs.OptionLedgerStores.Sections`.
10. Delivery rule, binding: nothing here reaches notify. A job result or `job_event` carries a job id, a status and an alias only; no cash amount, balance, net value or position. `internal/notify` imports none of the new packages (the grep of entry 9). When a figure is built later it goes to a page or a local notice, never to `Sender`; the margin cushion and the assignment cash stay as `account-data.md` Decision 7 says.
11. Credentials and second opinion. Keys only, no value: nothing new is read; the job already uses `$OT_HOME/flex.toml` (its `token_env` and `account_env` entries name the variables for the token and the account number) and the line `OT_PG_DSN_OPTION_LEDGER`. No new variable, file or key. The owner's points, each a second-opinion case for the advisor (it answers later; no Advisor line is written here): (a) the owner edits his Flex query in the Client Portal to add the sections, which only he can do, and until he does the status rows say `absent`; (b) the existing Flex token reads more of his account data (cash, net value, positions): same service and token as option-ledger, new data classes; (c) new tables in his PostgreSQL under the existing role, insert and select only (a write outside the repository). No new outside service, account or credential is added under the defaults.
12. FX: the daily FX job is `fx_refresh` of `docs/design/market-bars.md` Decision 9 (IBKR daily MIDPOINT of USD.JPY, table `obs.fx_rate`, no fallback rate); daily only, no stream. This design adds no FX source. It stores the Flex conversion rates as raw rows so that a later check can put IBKR's statement rate beside the stored midpoint. Window default — owner may change: (a) store the conversion rates, (b) leave that section out of the registry.
13. Money and quantities are text as sent; no float and no decimal parse. `row_index` is the position in the report and carries no meaning beyond it.

## Modules

New directories: `internal/store/flexsection` (declarations only), `internal/store/flexsectionpg`; new files in `internal/sources/flex` and `internal/jobs/optionledger` (packages that option-ledger issues 6 and 9 create). No new row. This pull request edits `docs/design/modules.md` in two places and regenerates `arch.toml` with `go run ./scripts/arch` so that `--check` passes: (1) the entry cell of the store row gains `internal/store/flexsection` after `internal/store/bar`; (2) the sentence that lists the implementation packages `cmd/ot` may import gains `internal/store/flexsection` and `internal/store/flexsectionpg`. The rule for declarations-only packages is owned by `docs/design/market-bars.md` and its paragraph is on main; this design does not restate it. No depends cell changes. No jobs package is added.

`scripts/arch` follows the declarations-only rule only after the issue `bar-store-api` of `docs/design/market-bars.md` has landed (the list `declarationsOnly` does not exist in `scripts/arch/main.go` yet); entry 3 adds this package to the list `declarationsOnly` in `scripts/arch/main.go` after it. The entries that edit `.go-arch-lint.yml` (3, 4 and 7) are one serial chain in that order, after the entries of other designs that edit it; each rebases on whichever lands first (the file is regenerated by `go run ./scripts/arch`). Entry 9 shares `cmd/ot/optionledger.go`, `cmd/ot/optionledger_test.go` and `docs/contracts/ot.md` with `optionledger-wire` of `docs/design/option-ledger.md`, and `internal/jobs/api.go` of entry 4 is shared with other open designs; the second to land rebases. Entry 1 shares `contracts/generate.sh`, `internal/contracts/gen/records.go` and `internal/contracts/schema_test.go` with dataset entries of other designs; the generator rewrites the output on rebase.

| # | issue | module | order |
|---|---|---|---|
| 1 | flexsection-datasets (interface) | contracts | first |
| 2 | flexsection-sources-api (interface) | sources | first |
| 3 | flexsection-store-api (interface) | store, plus repo (licensed pair: `scripts/arch`, `.go-arch-lint.yml`) | after 1; before 4, 7 |
| 4 | flexsection-jobs-api (interface) | jobs, plus repo (licensed pair: `.go-arch-lint.yml`) | after 3; before 7 |
| 5 | flexsection-source-parse | sources | after 2 |
| 6 | flexsection-store-migration | store | after 1, 3 |
| 7 | flexsection-store-impl | store, plus repo (licensed pair: `.go-arch-lint.yml`) | after 3, 4, 6 |
| 8 | flexsection-ledger-step | jobs | after 4, 5 |
| 9 | flexsection-wire | ot | after 7, 8 |

## Interface

| name | owner and file | declaration |
|---|---|---|
| datasets `optionledger.flex_section_row` r1 and `optionledger.flex_section_status` r1; generated `FlexSectionRow`, `FlexSectionStatus` (names as the generator gives them) | contracts, `contracts/datasets/optionledger.flex_section_row.r1.json` and `optionledger.flex_section_status.r1.json` | row: `alias`, `section`, `row_key`, `revision`, `fetch_id`, `row_index`, `attributes` (JSON object as text), `attributes_sha256`, `recorded_at`, key `(alias, section, row_key, revision)`; status: `fetch_id`, `alias`, `section`, `state` (present, absent), `row_count`, `rows_refused`, `recorded_at`, key `(fetch_id, alias, section)`; no field holds an account number; `section` is a pattern, not an enum |
| `sources.FlexStatement.Sections map[string][]map[string]string` | sources, `internal/sources/api.go` | additive: one field, keyed by row element name, rows as text maps with every attribute as sent, filled by the container rule of Decision 3; `Trades` and `OtherLevels` are unchanged; the account attributes are still in the rows and the job drops them |
| package `flexsection`: `Row{Alias, Section, RowKey string; Revision int; FetchID string; RowIndex int; Attributes, AttributesSHA256 string; RecordedAt time.Time}`, `Status{FetchID, Alias, Section, State string; RowCount, RowsRefused int; RecordedAt time.Time}`, `Insert{New, Revised, Unchanged int}`, `Writer{RecordSections(ctx, fetchID string, rows []Row, statuses []Status) (Insert, error)}`, `Reader{Rows(ctx, alias, section string) ([]Row, error); Statuses(ctx, alias string) ([]Status, error)}`, `Store{Writer; Reader; Close() error}`, `OpenFunc func(ctx context.Context, dsn string) (Store, error)` | store, `internal/store/flexsection/api.go` (declarations only; the implementation is `internal/store/flexsectionpg.Open`) | `RecordSections` writes rows and statuses in one transaction under the revision rule of Decision 6 on `(Alias, Section, RowKey)`; the `Revision` given is ignored; attributes that hold `accountId` or `acctAlias` are an error and nothing is written; no rows and no statuses makes no database call. `Rows` returns the highest revision of every key, sorted by `RowKey`; `Statuses` returns the newest status per section of the alias. It reads the views, never a base table. The package imports `context` and `time` only |
| `jobs.OptionLedgerStores.Sections flexsection.Writer` | jobs, `internal/jobs/api.go` | additive: one field; nil switches the section step off; the other fields and `NewOptionLedgerJobFunc` are unchanged |

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses the fake clock of the test kit, never sleeps, opens no port, and touches no live broker, no IBKR and no database (a test that needs PostgreSQL carries the build tag `pg` and runs only in GitHub CI). Fixtures use the labels `a` and `b`, invented symbols, ids and figures, in Flex's container shape. Acceptance is read offline from files and git; `BASE=$(git merge-base HEAD origin/main)`. Test runs are listed under `## Checked by CI`. No test name starts with `Open`.

### 1 flexsection-datasets (interface)

Module: contracts. Size: S.
Files: `contracts/datasets/optionledger.flex_section_row.r1.json`, `contracts/datasets/optionledger.flex_section_status.r1.json`, `contracts/generate.sh`, `internal/contracts/gen/records.go`, `docs/contracts/datasets/optionledger.flex_section_row.md`, `docs/contracts/datasets/optionledger.flex_section_status.md`, `internal/contracts/schema_test.go`
Design: docs/design/account-flex-fx.md#interface
Order: first.
Goal: the two datasets, a source and its generated output committed together, `generate.sh` naming both as root types.
Acceptance:
1. `grep -c '"x-primary-key": \["alias", "section", "row_key", "revision"\]' contracts/datasets/optionledger.flex_section_row.r1.json` prints 1; `grep -c '"x-primary-key": \["fetch_id", "alias", "section"\]' contracts/datasets/optionledger.flex_section_status.r1.json` prints 1.
2. No property is named after an account: `grep -c '"account' contracts/datasets/optionledger.flex_section_row.r1.json contracts/datasets/optionledger.flex_section_status.r1.json | grep -vc ':0'` prints 0; the section property is a pattern and not an enum: `grep -c 'enum' contracts/datasets/optionledger.flex_section_row.r1.json` prints 0.
3. `grep -c 'present' contracts/datasets/optionledger.flex_section_status.r1.json` is at least 1 and `grep -c 'absent' contracts/datasets/optionledger.flex_section_status.r1.json` is at least 1.
4. `grep -c 'FlexSectionRow\|FlexSectionStatus' internal/contracts/gen/records.go` is at least 2; `grep -c 'flex_section_row\|flex_section_status' contracts/generate.sh` is at least 4; `grep -c 'flex_section' internal/contracts/schema_test.go` is at least 2.
5. `git diff --name-only $BASE | grep -vxF -e contracts/datasets/optionledger.flex_section_row.r1.json -e contracts/datasets/optionledger.flex_section_status.r1.json -e contracts/generate.sh -e internal/contracts/gen/records.go -e docs/contracts/datasets/optionledger.flex_section_row.md -e docs/contracts/datasets/optionledger.flex_section_status.md -e internal/contracts/schema_test.go | grep -c .` prints 0.
Checked by CI: ci-go; the generated-code check of `contracts/generate.sh`; the design verdict.

### 2 flexsection-sources-api (interface)

Module: sources. Size: S.
Files: `internal/sources/api.go`, `internal/sources/flex_section_api_test.go`, `docs/contracts/sources.md`
Design: docs/design/account-flex-fx.md#interface
Order: first.
Goal: the additive field `Sections`, declarations only, with a contract test from both sides (a fake `FlexSource` that returns a statement with sections and a caller that reads them) in a new file; `docs/contracts/sources.md` records the container rule of Decision 3.
Acceptance:
1. `grep -c 'Sections *map\[string\]\[\]map\[string\]string' internal/sources/api.go` prints 1; `grep -c 'Trades *\[\]map\[string\]string' internal/sources/api.go` prints 1; `grep -c 'OtherLevels' internal/sources/api.go` is at least 1.
2. `grep -c 'Fetch(ctx context.Context, req FlexRequest) (FlexReport, error)' internal/sources/api.go` prints 1; `grep -c 'float32\|float64\|os.Getenv' internal/sources/api.go` prints 0.
3. `grep -c 'Sections' internal/sources/flex_section_api_test.go` is at least 2 and `grep -c 'sources.FlexSource' internal/sources/flex_section_api_test.go` is at least 1.
4. `grep -c 'container' docs/contracts/sources.md` is at least 1 and `grep -c 'ChangeInNAV' docs/contracts/sources.md` is at least 1.
5. `git diff --name-only $BASE | grep -vxF -e internal/sources/api.go -e internal/sources/flex_section_api_test.go -e docs/contracts/sources.md | grep -c .` prints 0.
Checked by CI: ci-go; the design verdict.

### 3 flexsection-store-api (interface)

Module: store, plus repo (licensed pair: `scripts/arch/main.go`, `scripts/arch/main_test.go`, `arch.toml`, `.go-arch-lint.yml`). Size: S.
Files: `internal/store/flexsection/api.go`, `internal/store/flexsection/api_test.go`, `docs/contracts/store.md`, `scripts/arch/main.go`, `scripts/arch/main_test.go`
Design: docs/design/account-flex-fx.md#interface
Order: after 1; before 4 and 7 (the three edit `.go-arch-lint.yml` in that order); cut only after the issue `bar-store-api` of `docs/design/market-bars.md` has landed, which edits `scripts/arch/main.go` first.
Goal: the package of the Interface table, declarations only, with a contract test from both sides (a fake `Store` and a caller that records sections and reads rows and statuses back), the package added to `declarationsOnly` of `scripts/arch/main.go` and its test, the component in `.go-arch-lint.yml`, and `docs/contracts/store.md` recording the revision rule and the views.
Acceptance:
1. `grep -c 'type Writer interface\|type Reader interface\|type Store interface\|type OpenFunc ' internal/store/flexsection/api.go` prints 4; `grep -c 'RecordSections\|Statuses(\|Rows(' internal/store/flexsection/api.go` is at least 3.
2. `ls internal/store/flexsection` lists exactly `api.go` and `api_test.go`; `grep -c '^func ' internal/store/flexsection/api.go` prints 0.
3. `grep -c 'database/sql\|pgx\|internal/store"\|internal/jobs\|internal/broker\|contracts/gen' internal/store/flexsection/api.go` prints 0; `grep -c 'float32\|float64' internal/store/flexsection/api.go` prints 0.
4. `grep -c 'internal/store/flexsection' scripts/arch/main.go` is at least 1; `grep -c 'internal/store/flexsection' .go-arch-lint.yml` is at least 1; `grep -c 'var _ flexsection.Writer\|var _ flexsection.Reader' internal/store/flexsection/api_test.go` prints 2.
5. `arch.toml` gives `jobs` the package: `grep -c 'store/flexsection' arch.toml` is at least 1. `grep -c 'flex_section_row_r1' docs/contracts/store.md` is at least 1 and `grep -c 'accountId' docs/contracts/store.md` is at least 1.
6. `git diff --name-only $BASE | grep -vxF -e internal/store/flexsection/api.go -e internal/store/flexsection/api_test.go -e docs/contracts/store.md -e scripts/arch/main.go -e scripts/arch/main_test.go -e arch.toml -e .go-arch-lint.yml | grep -c .` prints 0.
Checked by CI: ci-go; `go run ./scripts/arch --check` printing `arch: ok`; the design verdict.

### 4 flexsection-jobs-api (interface)

Module: jobs, plus repo (licensed pair: `.go-arch-lint.yml`). Size: S.
Files: `internal/jobs/api.go`, `internal/jobs/flex_section_api_test.go`, `docs/contracts/jobs.md`
Design: docs/design/account-flex-fx.md#interface
Order: after 3; before 7 (`.go-arch-lint.yml` chain 3, 4, 7); cut only after `bar-store-api` of `docs/design/market-bars.md` has landed, because `jobs`, a root component, may import `store/flexsection` only under the root-component rule that issue adds to `scripts/arch`.
Goal: the field `Sections flexsection.Writer` in `OptionLedgerStores`, additions only, with a contract test from both sides (a caller that builds the job with the field set and with it nil, a fake writer) in a new file; `docs/contracts/jobs.md` records that nil switches the step off and the delivery rule of Decision 10.
Acceptance:
1. `grep -c 'Sections *flexsection.Writer' internal/jobs/api.go` prints 1; `grep -c 'type OptionLedgerStores struct' internal/jobs/api.go` prints 1; `grep -c 'type NewOptionLedgerJobFunc ' internal/jobs/api.go` prints 1.
2. `grep -c 'float32\|float64\|os.Getenv' internal/jobs/api.go` prints 0.
3. `grep -c 'Sections' internal/jobs/flex_section_api_test.go` is at least 2; `grep -c 'flexsection.Writer' internal/jobs/flex_section_api_test.go` is at least 1.
4. `grep -c 'Sections' docs/contracts/jobs.md` is at least 1.
5. `git diff --name-only $BASE | grep -vxF -e internal/jobs/api.go -e internal/jobs/flex_section_api_test.go -e docs/contracts/jobs.md -e .go-arch-lint.yml | grep -c .` prints 0.
Checked by CI: ci-go; `go run ./scripts/arch --check` printing `arch: ok`; the design verdict.

### 5 flexsection-source-parse

Module: sources. Size: M.
Files: `internal/sources/flex/parse.go`, `internal/sources/flex/sections.go`, `internal/sources/flex/sections_test.go`, `internal/sources/flex/testdata/report_sections.xml`
Design: docs/design/account-flex-fx.md#decisions (Decision 3)
Order: after 2; cut only after `docs/design/option-ledger.md` issue 6 has landed (it creates `parse.go`, which this entry edits by one call).
Goal: the parser fills `FlexStatement.Sections` by the container rule of Decision 3, in the new file `sections.go` called once from `parse.go`; the rules of that parser stay (no document type or entity declaration, no UTF-16 or UTF-32 body, the body size limit). The fixture is in Flex's container shape.
Acceptance:
1. `grep -c 'Sections' internal/sources/flex/sections.go` is at least 1; `grep -c 'Sections\|parseSections' internal/sources/flex/parse.go` is at least 1.
2. `grep -c '"os"\|net/http\|internal/jobs\|internal/store\|contracts/gen\|float32\|float64' internal/sources/flex/sections.go` prints 0; the parser names no section: `grep -c 'CashTransaction\|OpenPosition\|ChangeInNAV\|ConversionRate\|EquitySummary' internal/sources/flex/sections.go` prints 0.
3. The fixture has the container shape and an invented account: `grep -c '<CashTransactions>\|<OpenPositions>\|<ConversionRates>\|<ChangeInNAV ' internal/sources/flex/testdata/report_sections.xml` is at least 4; `grep -c 'accountId="[A-Z][0-9]\{5,\}"' internal/sources/flex/testdata/report_sections.xml` prints 0.
4. Each of these functions is in `internal/sources/flex/sections_test.go` once: `TestContainerRowsKeyedByRowName`, `TestSingleElementIsOneRow`, `TestEveryAttributeKeptAsText`, `TestTradesFieldUnchanged`, `TestNoSectionsGivesEmptyMap`, `TestEntityInSectionStillRefused`.
5. `git diff --name-only $BASE | grep -vxF -e internal/sources/flex/parse.go -e internal/sources/flex/sections.go -e internal/sources/flex/sections_test.go -e internal/sources/flex/testdata/report_sections.xml | grep -c .` prints 0; each new file is under 600 lines.
Checked by CI: ci-go (`go test ./internal/sources/... -race -count=1`).

### 6 flexsection-store-migration

Module: store. Size: S.
Files: `migrations/NNNN_flex_section.sql`
Design: docs/design/account-flex-fx.md#decisions (Decision 8)
Order: after 1 and 3; cut only after the migration of `docs/design/option-ledger.md` issue 8 (which creates schema `optionledger` and the role `ot_w_optionledger`) has landed. `NNNN` is the next free number at cut time, greater than every file in `migrations` on origin/main and than the option-ledger one; on a rebase where another migration took it, the file is renamed to the next free number and nothing else changes. The acceptance below reads the file by the glob.
Goal: the two tables, their views and their grants, and nothing else.
Acceptance:
1. `grep -c 'CREATE TABLE optionledger.flex_section_row\|CREATE TABLE optionledger.flex_section_status' migrations/*_flex_section.sql` prints 2; `grep -c 'PRIMARY KEY (alias, section, row_key, revision)' migrations/*_flex_section.sql` prints 1; `grep -c 'PRIMARY KEY (fetch_id, alias, section)' migrations/*_flex_section.sql` prints 1.
2. `grep -ci 'GRANT .*\(UPDATE\|DELETE\|TRUNCATE\)' migrations/*_flex_section.sql` prints 0; `grep -ci 'CREATE ROLE\|CREATE SCHEMA' migrations/*_flex_section.sql` prints 0; `grep -c 'GRANT INSERT, SELECT ON optionledger.flex_section' migrations/*_flex_section.sql` prints 2.
3. `grep -c 'CREATE VIEW rd.flex_section_row_r1\|CREATE VIEW rd.flex_section_status_r1' migrations/*_flex_section.sql` prints 2; `grep -v '^--' migrations/*_flex_section.sql | grep ot_ro_pages | grep -c 'optionledger\.'` prints 0.
4. `grep -ci 'account_id\|accountid\|account_number' migrations/*_flex_section.sql` prints 0; `ls migrations | grep -c '_flex_section.sql'` prints 1.
5. `git diff --name-only $BASE | grep -vxE 'migrations/[0-9]{4}_flex_section\.sql' | grep -c .` prints 0.
Checked by CI: ci-go; `go test -tags pg ./internal/store/...` (GitHub CI only). The owner runs `ot migrate`.

### 7 flexsection-store-impl

Module: store, plus repo (licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `internal/store/flexsectionpg/store.go`, `internal/store/flexsectionpg/queries.go`, `internal/store/flexsectionpg/store_test.go`, `internal/store/flexsectionpg/pg_test.go`
Design: docs/design/account-flex-fx.md#decisions (Decisions 6, 8, 9)
Order: after 3, 4 and 6 (`.go-arch-lint.yml` chain 3, 4, 7).
Goal: `flexsectionpg.Open` returns a `flexsection.Store` over the DSN of `store.DSNEnvOptionLedger`: one transaction per `RecordSections` with the revision rule (the highest revision of the key is read and the next written there, `ON CONFLICT DO NOTHING` as a second guard), a refusal of attributes that hold `accountId` or `acctAlias`, and a reader over the two views only.
Acceptance:
1. `grep -c 'flexsection.Store' internal/store/flexsectionpg/store.go` is at least 1 and `grep -c 'func Open' internal/store/flexsectionpg/store.go` prints 1.
2. `grep -c 'rd.flex_section_row_r1\|rd.flex_section_status_r1' internal/store/flexsectionpg/queries.go` is at least 2; `grep -c 'FROM optionledger\.' internal/store/flexsectionpg/queries.go` prints 0; `grep -ci 'UPDATE \|DELETE \|TRUNCATE' internal/store/flexsectionpg/queries.go` prints 0.
3. `grep -c 'ON CONFLICT DO NOTHING' internal/store/flexsectionpg/queries.go` is at least 1; `grep -c 'accountId\|acctAlias' internal/store/flexsectionpg/store.go` is at least 2.
4. Each of these functions is in `internal/store/flexsectionpg/store_test.go` once: `TestNewKeyIsRevisionOne`, `TestEqualHashNotWritten`, `TestChangedHashNextRevision`, `TestRowsAndStatusesOneTransaction`, `TestRowWithAccountKeyRefused`, `TestReaderHighestRevision`, `TestStatusNewestPerSection`, `TestNoAccountColumn`; `grep -c '^//go:build pg' internal/store/flexsectionpg/pg_test.go` prints 1.
5. `grep -rn 'internal/jobs\|internal/compute\|contracts/gen' internal/store/flexsectionpg | grep -c .` prints 0; `grep -c 'store/flexsectionpg' .go-arch-lint.yml` is at least 1; each new file is under 600 lines.
6. `git diff --name-only $BASE | grep -vxE 'internal/store/flexsectionpg/[a-z_]+\.go|\.go-arch-lint\.yml' | grep -c .` prints 0.
Checked by CI: ci-go; `go test -tags pg ./internal/store/...`; `go run ./scripts/arch --check`; the design verdict and the advisor's verdict (Decision 11).

### 8 flexsection-ledger-step

Module: jobs. Size: M.
Files: `internal/jobs/optionledger/job.go`, `internal/jobs/optionledger/registry.go`, `internal/jobs/optionledger/sections.go`, `internal/jobs/optionledger/registry_test.go`, `internal/jobs/optionledger/sections_test.go`, `internal/jobs/optionledger/testdata/report_sections.json`
Design: docs/design/account-flex-fx.md#decisions (Decisions 2, 4 to 7, 10)
Order: after 4 and 5; cut only after `docs/design/option-ledger.md` issue 9 has landed (it creates `job.go`, which this entry edits by one call after the fetch record is written). Its tests use a fake `FlexSource` and fakes of the writers, so it needs no implementation entry.
Goal: the registry of Decision 5 (a Go table: row element name, required and optional key attributes) and the `row_key` function; the section step in `sections.go`, called from `job.go` after the fetch record is written (the step does not need its id) when `Stores.Sections` is not nil: map statements to aliases as the job already does, build the rows with `accountId` and `acctAlias` removed before the injected hash, refuse and count a row without its required key attributes and both rows of a duplicate key, write one status row per expected section (`present` with the count, or `absent`), log a `degraded` record for an absent section, mint the fetch reference of Decision 2 from the request and `FlexReport.FetchedAt`, and call `RecordSections` with it. The trade path is unchanged with a nil `Sections`.
Acceptance:
1. `grep -c 'CashTransaction\|OpenPosition\|ChangeInNAV\|EquitySummaryByReportDateInBase\|ConversionRate' internal/jobs/optionledger/registry.go` is at least 5; `grep -c 'openDateTime\|originatingTransactionID\|levelOfDetail' internal/jobs/optionledger/registry.go` is at least 3; `grep -c 'transactionID\|fromDate\|toDate\|fromCurrency\|toCurrency\|reportDate' internal/jobs/optionledger/registry.go` is at least 6.
2. `grep -c 'Sections' internal/jobs/optionledger/job.go` is at least 1; `grep -c 'RecordSections' internal/jobs/optionledger/sections.go` is at least 1; `grep -c 'absent' internal/jobs/optionledger/sections.go` is at least 1; `grep -c 'FetchedAt' internal/jobs/optionledger/sections.go` is at least 1.
3. The step imports interfaces only and no other jobs package: `grep -n 'internal/jobs/\|flexsectionpg\|internal/sources/flex"' internal/jobs/optionledger/sections.go internal/jobs/optionledger/registry.go | grep -c .` prints 0; `grep -rn 'os.Getenv\|time.Now()\|time.Sleep\|notify\|telegram' internal/jobs/optionledger/sections.go internal/jobs/optionledger/registry.go | grep -c .` prints 0.
4. Each of these functions is in `internal/jobs/optionledger/registry_test.go` or `sections_test.go` once: `TestRowKeyJoinsKeyAttributes`, `TestRowKeyMissingRequiredRefused`, `TestLotRowKeyCarriesOpenDateTime`, `TestLotRowWithoutLotAttributesRefused`, `TestDuplicateKeyBothRefused`, `TestAccountIdDroppedBeforeHash`, `TestAbsentSectionWritesStatusAndDegraded`, `TestNoSectionsAtAllFiveAbsent`, `TestNilSectionsStoreLeavesTradePath`, `TestUnmappedStatementWritesNoSection`, `TestSectionWriteFailureFailsRequiredLogin`, `TestResultCarriesNoFigures`.
5. The fixture is invented and has no account-shaped id: `grep -c '[A-Z][0-9]\{6,\}' internal/jobs/optionledger/testdata/report_sections.json` prints 0.
6. `git diff --name-only $BASE | grep -vxE 'internal/jobs/optionledger/(job|registry|sections)(_test)?\.go|internal/jobs/optionledger/testdata/report_sections\.json' | grep -c .` prints 0; each new file is under 600 lines.
Checked by CI: ci-go (`go test ./internal/jobs/... -race -count=1`).

### 9 flexsection-wire

Module: ot. Size: S.
Files: `cmd/ot/optionledger.go`, `cmd/ot/optionledger_test.go`, `docs/contracts/ot.md`
Design: docs/design/account-flex-fx.md#decisions (Decisions 2, 10, 11)
Order: after 7 and 8; cut only after `docs/design/option-ledger.md` issue 12 has landed (it creates `cmd/ot/optionledger.go`); it shares those files with that issue and with other open designs, and the second to land rebases.
Goal: wiring only. The `option_ledger` wiring opens `flexsectionpg.Open` with `store.DSNEnvOptionLedger` (the line it already reads) and sets `OptionLedgerStores.Sections`; if that store cannot be opened the section step is off, a `degraded` record names the line and never its value, and the trade path runs as before. No new variable, file or key.
Acceptance:
1. `grep -c 'flexsectionpg.Open' cmd/ot/optionledger.go` is at least 1; `grep -c 'Sections' cmd/ot/optionledger.go` is at least 1.
2. `grep -c 'os.Getenv' cmd/ot/optionledger.go` prints 0; `grep -rn 'IBKR_FLEX' cmd docs/contracts | grep -c .` prints 0; `grep -rn 'flexsection' internal/notify | grep -c .` prints 0 (Decision 10).
3. `grep -c 'flex_section\|Sections' docs/contracts/ot.md` is at least 1; the contract text adds no variable: `git diff $BASE -- docs/contracts/ot.md | grep '^+' | grep 'OT_PG_DSN_' | grep -vc 'OT_PG_DSN_OPTION_LEDGER'` prints 0.
4. Each of these functions is in `cmd/ot/optionledger_test.go` once: `TestSectionsStoreWiredToLedger`, `TestSectionsStoreFailureKeepsTradePath`, `TestSectionsValuesNeverPrinted`.
5. `git diff --name-only $BASE | grep -vxF -e cmd/ot/optionledger.go -e cmd/ot/optionledger_test.go -e docs/contracts/ot.md | grep -c .` prints 0.
Checked by CI: ci-go (`go test ./cmd/ot -race -count=1`), golangci-lint, `go run ./scripts/arch --check`. The owner edits his Flex query in the Client Portal (Decision 11), runs `ot migrate`, then `ot run-once option_ledger`, and compares the stored rows and the status rows with the statement on the broker's screen.

## Checked by CI

- `go test ./internal/sources/... ./internal/store/... ./internal/jobs/... ./cmd/ot -race -count=1` (ci-go: parallel, `t.TempDir()`, fake clock, no sleep, no live broker, no network).
- `go test -tags pg ./internal/store/...` (the `pg` job, GitHub CI only).
- `go run ./scripts/arch --check` printing `arch: ok` (entries 3, 4, 7 and 9); the generated-code check of `contracts/generate.sh` (entry 1). Both run Go tools that the acceptance checker refuses.
- The design verdict on every entry, and the advisor's verdict on entries 7 and 9 (Decision 11).

## Left out

- The daily FX job: `fx_refresh` of `docs/design/market-bars.md` (Decision 9 there, pull request 768, merged; its code is not on main); daily only. If that design is dropped, the FX job returns to the alignment table as missing and the Flex conversion rates stored here are the only FX data. The 15 s FX stream is dropped by the owner 2026-11-09; the ECB cross-check, the shadow period and `fx_override` of ADR 0004 are not built.
- Trades, expiry, assignment, exercise, the history import and the replay: `docs/design/option-ledger.md`; the API executions, positions and balances: `docs/design/account-data.md`. Nothing here reads or changes them, except that the ledger job gains the section step.
- Figures over the stored rows, all stage 2, each its own later design: dividend and interest income, a cash balance, net value and its change, the check of the replayed ledger against the open positions, the check of IBKR's conversion rate against `obs.fx_rate`, any FX conversion of a total. `ot` subcommands and pages that show the rows or the absent-section notice; Telegram text about them: never.
- Flex sections not in the registry (for example fund and accrual sections): one registry row each later; no schema change.
- The raw XML of a report: only `raw_sha256` of the trade fetch record is kept.
- Failing the run for an absent section: the (b) answer of Decision 4; not designed.
- A second daily FX source (ECB, FRED `DEXJPUS`): a new outside service for ECB, needs the owner first; not designed.
- Rejected: a second job `flex_sections` with its own fetch record (a second request pair and pacer cost, a second window rule and a second failure path; the read-only attack's point 3); a section key built from the position in the report (a reorder would write a revision per row); a typed table per section (attribute names unverified until the first fetch); a registry file edited by the owner; an `UPDATE` when Flex corrects a row (a revision, highest wins); a migration number written in this design.
- Lock-ins, named: the key `(alias, section, row_key, revision)` and each `row_key` shape are permanent once rows are stored (a changed key attribute orphans a section's history); the alias is the identity of every row; the section element names are Flex's own.
- Open points: (1) the element and attribute names of Decisions 3 and 5, the container names above all, are read from the first real fetch and change the registry and the fixtures if they differ; (2) whether one Flex query can carry the trades and these sections together, and the largest window it answers, are not verified; (3) which cash-event attributes hold free text with personal data is not known until the first fetch (Decision 6, answer b).

## Adversarial

The read-only attack, kept in the window's scratch, has six points.

1. Gap: five sections stored raw with no consumer; FX only delegated to an unmerged pull request; nothing stored until the owner edits his query, yet runs stay ok. Partly adopted. The absence is now visible: a status row per expected section, `present` or `absent`, and a `degraded` record, so a missing query edit is not a silent ok (Decision 4); whether it should also fail the required login's run is a window default. The lack of a consumer is by design and named in Left out (stage 2 figures). FX stays delegated to market-bars and Left out says what follows if that pull request is dropped.
2. Parser contract against Flex's real shape (containers, `ChangeInNAV` as one element). Adopted: Decision 3 states the container rule, the parser has no name list, and the fixture of entry 5 has the container shape, checked by acceptance 3 of entry 5. The names themselves stay unverified (open point 1).
3. No second job. Adopted: Decision 2; the separate job, its fetch record, registry entry and wire entry are gone and the step is a follow-up edit of the option-ledger packages after their issues land. A separate status table replaces the second fetch record, keyed by the fetch reference of Decision 2.
4. Key lock-in on guessed attributes, OpenPosition collisions. Adopted: Decision 5 puts the lot (`openDateTime`, `originatingTransactionID`) in the OpenPosition key and says why, reviews the other keys, and refuses ambiguous or duplicate keys instead of merging them. The lock-in stays named in Left out.
5. The hard-coded migration number and the stale base. Adopted: Decision 8 and entry 6 name the migration `NNNN_flex_section.sql`, the next free number at cut time with a rename rule, acceptance reads it by glob; the Status line names the main it was read at, 64a9bdb.
6. Waits and serial files. Adopted: the header states the waits plainly by issue number and pull request; each waiting entry says what it waits for; acceptance names only the entry's own Files; the three entries that edit `.go-arch-lint.yml` stay a serial chain, and the files shared with other designs are named under Modules.
