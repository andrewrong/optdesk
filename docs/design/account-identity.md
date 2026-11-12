# Account identity: the IBKR account id is the key everywhere

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 1

Source: owner 2026-11-12 (ask optdesk-3211): accounts are named differently in different places (gateway side `main`/gateways `account`, Flex ledger side `a`/`b`, pages "alias"); pages cannot join them and the owner cannot tell what a/b are. "Align the names; configure it in ONE place; stop naming accounts everywhere; use the U-prefixed IBKR account number." Dev mode: data may be deleted and re-collected; no time rules.

## What main holds today (names of an account)

- Gateways file `account` field = "alias" (pyside `GatewaySpec.account`; proto `AccountResponse.account` and `ExecutionsResponse.account` say "never the account id"; `broker.Account.Alias`). Read in Go by `cmd/ot/account.go:91 accountAliases` (main = the `required` entry) and by `cmd/ot/site_positions.go:43 siteAliasList` for the pages.
- `OT_ACCOUNT_SOCKETS` `label=socket` (`cmd/ot/account.go:~60 parseAccountSockets`): a second name, the socket label, must equal the alias.
- Flex: `flex.toml [[account]] alias` (`a`/`b`, `internal/sources/flex/config.go:21,214`) plus `account_env` (the NAME of a variable holding the id); `cmd/ot/optionledger.go:187` builds `Accounts map[number]alias`; `internal/jobs/optionledger/types.go:35`, `sections.go:86` map `st.AccountID` to the alias.
- Store: column `alias` in `account.capture/position/value/execution` (migrations/0019), `option_candidate` (0018, 0027), `flex_trade` (0022, "alias is the only identity"), `flex_section*` (0024), `position_mark` (0026), and their views. `store.OptionLedgerReader.StoredAliases` (`internal/store/api.go:691`).
- Readers: `internal/pages/positions_page/builder.go:89,101`, `internal/pages/household_page/builder.go:41,50` (`Config.Aliases`, `alias ""` = main), `internal/pages/option_tracking_page/builder.go:117`, `internal/pages/home_options/member.go:194` (StoredAliases), `cmd/ot/tracking_site.go:48 aliasedLedger`, `cmd/ot/optionledger.go:252` and `ot ledger --alias`, jobs `accountcapture`, `positionmarks`, `optioncandidates`, `optionledger`.
- The id is dropped on purpose today (pyside-account-b.md Decisions: scrub regex, ib_async log line), so the two sides never meet.

## Goal

One key per account: the IBKR account id (`U0000001` shape). The gateway reports it (managed accounts) and the Flex statement reports it (`accountId`), so no mapping exists anywhere. The ids live only in the runtime store and in one file under `$OT_HOME`; never in code, commits, committed config or fixtures (tests use `U0000001`, `U0000002`).

## Decisions

1. The key of an account in every store row, reader, page scope, job key and CLI flag is `account_id` (the U id). The word "alias" in code and schema then means only the owner alias of `accounts.toml`, shown on pages; store columns are `account_id`.
2. The one config place is `$OT_HOME/accounts.toml`, one line per account, `alias = "U-id"`, the alias chosen by the owner (`mine`, `wife`, `daughter`), plus one line `main = "mine"` naming the main alias (alias regexp `^[a-z][a-z0-9_-]{0,31}$`, as flex's; `main` is reserved; the single letters `a` and `b` are refused so the old names cannot come back). Example: `main = "mine"`, `mine = "U0000001"`, `wife = "U0000002"`. Other configs never name an account by their own name: they refer to it by this alias or by the U id (today none needs to: Decisions 4 to 6). Read by Go only, owned by a new package `internal/accounts` (own implementation of a 30-line parse over the already-used TOML library; no new dependency). It is the allow-list (which accounts the pages, the ledger plan and the flex job handle) and the one map U id to owner alias. The file is a runtime file like `flex.toml`; no example with real ids is committed (an example with `U0000001` is).
3. The page shows the owner alias with the id: `wife (U0000002)`. The store, code and page scope carry the id; the URL may carry the alias, which resolves to the id at the page edge. an id not in `accounts.toml` is the existing unknown-account gap page. `alias ""` stays "the main account" and means the `main` alias.
4. Gateway side: the pyside server answers with the account id the gateway reports (the single managed account; two managed accounts stays refused as today). The gateways file `account` field and `broker.Account.Alias` go; `broker.Account` gets `ID`. `token_env` and any env key stay for credentials only and name nothing. Safety nets stay: the id is only in rpc responses and the store, never in a log line (the logger level and the scrub regex stay; the test that asserted an id-shaped string never reaches a response is inverted to assert it reaches the response and no log record).
5. `OT_ACCOUNT_SOCKETS` becomes a plain list of sockets (`path,path`); the label is not a name of an account. A socket whose gateway reports an id not in `accounts.toml` is skipped with a WARN naming no id; main absent is the existing Retryable failure of account_capture.
6. Flex: `flex.toml [[account]]` (`alias`, `account_env`) is removed; `flex.toml` keeps `[[login]]` and token env names only. A statement is kept when its `accountId` is in `accounts.toml`, otherwise refused as today's unknown account. The ledger plan has one key per `accounts.toml` account. `FlexAccount` and `OptionLedgerConfig.Accounts map[number]alias` go (the allow-list is a `[]string` of ids).
7. Existing rows under aliases: delete and re-collect, no mapping. The migration renames column `alias` to `account_id` in every table and view of the list above and deletes all rows of the account-keyed tables (`account.*`, `position_mark`, `option_candidate`, `flex_trade*`, `flex_section*` and their fetch rows) because the old key cannot be turned into an id without a table, which Decision 1 forbids. Flex rows come back by `ot backfill option_ledger` for the Flex window; gateway captures and marks are point-in-time and restart from the next run (dev mode accepts the loss of past captures). The migration and the `ot` step that follows it are run by the window (setup pg, migrate, setup pg) as usual.
8. `StoredAliases` becomes `StoredAccounts`; every `Config.Aliases` becomes `Accounts []accounts.Account` (id, alias, main). Pages never merge accounts (unchanged).
9. Push and notify stay account-free (ADR 0010); neither the alias nor the id is used there.

## Modules

| module | change |
| --- | --- |
| contracts | proto comments and `account` meaning in `account.proto`; no field added |
| pyside | return the real id; drop `account` from gateways file; tests |
| broker | `Account.Alias` to `Account.ID`; Go client |
| sources | flex config without `[[account]]`; `FlexConfig` without accounts |
| store | migration rename and delete; `account_id` columns; `StoredAccounts` |
| jobs | accountcapture, positionmarks, optioncandidates, optionledger use ids |
| pages | positions_page, household_page, option_tracking_page, home_options take `Accounts` |
| repo | new `internal/accounts` (config reader, interface file `api.go`) |
| ot | wiring, `ot ledger --account`, drop `siteAliasList`/`accountAliases` |
| docs | contracts docs, glossary term |

## Interface

- Owning module repo, `internal/accounts/api.go`: `type Account struct { ID, Alias string; Main bool }`; `func Load(path string) (List, error)`; `type List []Account`; methods `Main() Account`, `Find(id string) (Account, bool)`, `ByAlias(alias string) (Account, bool)`, `Display(id string) string` (alias plus id as in Decision 3). Load refuses: no `main` line, a `main` naming an unknown alias, a bad or reserved alias (`main`, `a`, `b`), a repeated id, an id not shaped `^U[0-9]{5,9}$`.
- Owning module broker: `Account.ID string` replaces `Alias`.
- Owning module store: every `Reader`/`Writer` method that takes or returns `alias string` takes `accountID string`; `StoredAccounts(ctx) ([]string, error)`.
- Owning module sources: `sources.FlexConfig` carries tokens only; the statement's `AccountID` is already on `Statement`.
- Owning module pyside: `AccountResponse.account` and `ExecutionsResponse.account` carry the id; the file `pyside/src/optdesk/ibkr/api.py` updates its docstrings.

## Issues

### 1 account-identity-interface (interface)
Design line: this file. Files: `internal/accounts/api.go`, `internal/accounts/api_test.go`, `internal/broker/api.go`, `internal/broker/api_test.go`, `contracts/proto/ibkr/v1/account.proto` and its generated output, `pyside/src/optdesk/ibkr/api.py`, `pyside/tests/contracts/test_ibkr_api.py`, `internal/store/api.go`, `internal/store/api_test.go`, `docs/contracts/broker.md`, `docs/contracts/store.md`.
Acceptance:
1. `internal/accounts/api.go` declares exactly the types and methods of Interface and no logic; its test checks the contract from both sides with fake ids.
2. `broker.Account` has `ID` and no `Alias`; the comment on it and in `account.proto` says the field is the account id the gateway reports.
3. `internal/store/api.go` has no identifier or comment word "alias" for an account; `StoredAccounts` replaces `StoredAliases`.
4. The generated proto output matches the proto source (same commit).

### 2 accounts-config
Order: after 1. Files: `internal/accounts/**` (not `api.go`), `docs/contracts/accounts.md`.
Acceptance:
1. `Load` implements Decision 2 and refuses each case listed in Interface, each with an error that names the line number and never the id.
2. A test per refusal and one with the reserved aliases `a`, `b`, `main`; all ids in the test files are `U0000001`-style.
3. No file under the repo other than tests holds a string shaped like an account id (the existing scan passes).

### 3 pyside-real-id
Order: after 1. Files: `pyside/src/optdesk/ibkr/**`, `pyside/tests/**`, `pyside/examples/gateways.example.toml`.
Acceptance:
1. The account servicers put the single managed account id of the gateway in `account` of both responses; the gateways file no longer has or accepts an `account` key (unknown-key refusal test).
2. A test builds a fake id from two pieces at run time, as before, and asserts it is in the serialized response and in no log record at any level (`test_account_log_has_no_values` keeps its whitelist).
3. `ruff`, `mypy --strict` and `pytest` are listed under "## Checked by CI".

### 4 store-account-id
Order: after 1. Files: `migrations/0028_account_id.sql` (next free number), `internal/store/**`.
Acceptance:
1. The migration renames `alias` to `account_id` in each table and recreates each view of the list in "What main holds today", and deletes all rows of those tables (Decision 7); a pg test of the rename is under the `pg` tag.
2. `grep -rniw alias internal/store migrations` after the migration (new file excluded) finds no account column; primary keys keep the same column order with `account_id` in place of `alias`.
3. Every writer and reader method of the interface takes `accountID`; `StoredAccounts` returns ascending distinct ids.

### 5 sources-flex-no-accounts
Order: after 1. Files: `internal/sources/**`, `docs/contracts/sources.md`.
Acceptance:
1. `flex.Load` accepts a file with `[[login]]` only and refuses a `[[account]]` table with an unknown-key error; `FlexAccount` is gone from `internal/sources/api.go`.
2. A statement's `AccountID` is passed through unchanged; the source decides nothing about accounts.

### 6 jobs-account-id
Order: after 2, 4, 5 and the broker issue is part of 1 (Go client adapter in this issue). Files: `internal/jobs/accountcapture/**`, `internal/jobs/positionmarks/**`, `internal/jobs/optioncandidates/**`, `internal/jobs/optionledger/**`, `internal/broker/grpcbroker/**`, `docs/contracts/jobs/**`.
Acceptance:
1. The grpc client sets `Account.ID` from the response `account`; no `Alias` remains in `internal/broker/**`.
2. `accountcapture.Config` takes `Accounts accounts.List` and readers as a list; a row is written under `Account.ID` of the reader's answer; a reader whose id is not in the list is skipped and counted as not usable but not failed; main absent is Retryable (as today).
3. `optionledger.OptionLedgerConfig` has `Accounts []string`; `sections.go` keeps a statement when `st.AccountID` is in it and refuses it otherwise; Plan has one key per id, sorted.
4. Job keys, hashes, typed-error texts and result attributes carry `account_id`/the id only where they carried the alias before; no text carries an owner alias.
5. positionmarks and optioncandidates read and write `account_id` and take `Accounts` from config; no function has a parameter named `alias`.

### 7 pages-account-id
Order: after 2, 4. Files: `internal/pages/**`, `docs/contracts/pages.md`, `contracts/schemas/**` and generated types for the page fields that carry the scope (only if a field name `alias` appears; a rename carries source and output together).
Acceptance:
1. `positions_page.Build`, `household_page.Build`, `option_tracking_page.Build` and `home_options` take `Config.Accounts accounts.List`; scope strings in the pages are ids; `alias ""` resolves to `Accounts.Main().ID`.
2. Every place that printed `alias` prints `Accounts.Display(id)`; the page of an id not in the list is the unknown-account gap page as today.
3. `home_options` and the tracking builder read `StoredAccounts` filtered by the list; no account is merged with another.

### 8 ot-wiring
Order: after 3, 6, 7. Files: `cmd/**`, `docs/contracts/ot.md`, `docs/contracts/ot/**`, `tools/ot-flex-export/**`.
Acceptance:
1. `accountAliases`, `accountMain`, `siteAliasList` and `parseAccountSockets` are gone; `ot` loads `$OT_HOME/accounts.toml` once through `accounts.Load` for the jobs, the site and the CLI; a missing or invalid file stops `serve` and the jobs with a one-line error naming the file, not an id (the demo profile uses a built-in list of `U0000001`, `U0000002`).
2. `OT_ACCOUNT_SOCKETS` is a comma list of paths; `OT_IBKR_GATEWAYS` is no longer read by `ot`.
3. `ot ledger` has `--account <id>` in place of `--alias`; the ledger table prints `Display(id)`.
4. `cmd/ot/optionledger.go` no longer reads `account_env` values from `.env`.

### 9 docs-glossary
Order: after 8. Files: `docs/design/pyside-account-b.md` is owner-only and left alone; `docs/contracts/**` is edited in the issues above. This issue edits `docs/glossary.md` (if present) only: "account" is the IBKR account id, "alias" is the owner's name for an account, defined only in `accounts.toml`; `a`/`b`/`mine_account` are retired.
Acceptance:
1. The glossary defines account, alias and main, and has no gateway alias or Flex alias entry.

## Left out

- Mapping old alias rows to ids: deleted instead (Decision 7). Past gateway captures and marks are lost; Flex rows return by backfill.
- Account ids in push and morning reports: stay out (ADR 0010).
- Editing `accounts.toml` from a page: later; the file is the one place for now.
- `docs/design/pyside-account-b.md` and older designs that say "alias": not rewritten (history); this design supersedes their naming.
- Gateways with two managed accounts (FA/master): refused as today.
