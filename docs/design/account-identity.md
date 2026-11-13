# Account identity: one owner alias per account, from one accounts.toml, mapped at the source edges

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 5

Source: owner 2026-11-12 (asks optdesk-3211, optdesk-3212): the owner names each account once, meaningfully (`mine`, `wife`, `daughter`), in ONE config place, keyed to the IBKR U-account id; no `a`/`b`; no per-file names; pages show the owner's alias; positions join the Flex ledger. Dev mode: data may be deleted and re-collected. Revision 2 replaces the id-as-key rewrite (9 issues, 6 modules) by the simpler cut A of the adversarial review: the store key stays `alias`, its value becomes the owner alias.

## Today

- Gateway alias: gateways file `account` (pyside) answers in `AccountResponse.account`/`ExecutionsResponse.account`; `OT_ACCOUNT_SOCKETS` `label=socket` must equal it; `cmd/ot/account.go` `accountAliases`, `cmd/ot/site_positions.go` `siteAliasList`.
- Flex alias: `flex.toml [[account]] alias` + `account_env` (`internal/sources/flex/config.go`), turned into `map[number]alias` in `cmd/ot/optionledger.go`; `internal/jobs/optionledger` already takes that map as plain data.
- Pyside already holds the id (`accounts.py _one_account`, `managedAccounts()`); a Flex statement always carries `accountId` (`parse.go`). The two sides differ only in the label they put on it.

## Decisions

1. The one config place is `$OT_HOME/accounts.toml`: lines `<alias> = "<U-number>"` (alias `^[a-z][a-z0-9_-]{0,31}$`, id a `U` and 5 to 9 digits) and one line `main = "<alias>"` (`main` is reserved as a key). It lives only under `$OT_HOME`, never committed. The committed example shows `<U-number>`, not a digit string; tests write the file at run time with invented ids built from two pieces (`"U" + "000" + "0001"`), because `scripts/hygiene/main.go` fails any committed `U` plus 6 to 8 digits (the practice of `pyside-account-b.md`).
2. One reader, one owner: a new module `accounts` (package `internal/accounts`, `api.go` declarations only; implementation in `internal/accounts/toml`) reads the file and answers `Alias(id)`, `Main()`, `Aliases()`, `IDs()`. The row below and the `ot` depends cell are in `docs/design/modules.md` in this PR (owner-only path, so no worker issue edits it); `ot` is the only module that depends on `accounts`; `arch.toml` and `.go-arch-lint.yml` are generated from the table and not committed, so they need no entry. `cmd/ot` constructs it and hands plain data (alias lists, `map[number]alias`, readers keyed by alias) to sources, jobs and pages, which import nothing new.
3. The store keeps its `alias` columns and reader parameters; the value stored is the owner alias. `alias ""` stays the main account and means the `main` line. No store interface change.
4. Pyside reports the raw account id the gateway reports, in the `account` field of both responses (no contracts field change; comment only, not a blocker). It maps nothing and reads no `accounts.toml`. Two managed accounts stays refused by pyside, so one response is one account.
5. Mapping is per answer, which is per row: each gateway response and each Flex statement (`parse.go` already refuses a statement mixing accounts) carries exactly one account id, and every row under it takes that id's alias; no mapping per socket or per file. A statement with several accounts in a report is several statements and maps each. `OT_ACCOUNT_SOCKETS` becomes a plain list of sockets (no label); a resolver in `cmd/ot` gives the job one reader per `accounts.toml` alias, found by the id each socket's answer reports, cached per socket.
6. An id not in `accounts.toml` is refused, never stored under a default, `main` or the raw id: the reading is a permanent typed error `unknown account`, which the existing job result and gap page show as the named state `unknown_account` (the text carries no id and the socket index only). Flex does the same per statement as today.
7. Flex: `cmd/ot` builds `map[number]alias` from the accounts module, pairing each id of `IDs()` with `Alias(id)`; `flex.toml [[account]]` and `account_env` become unread, removed from `internal/sources/flex/config.go` in a later entry.
8. Ids are personal data. Go holds an id only in the accounts module, on the wire, in the resolver's compare and in the Flex account map `cmd/ot` builds; logs, job keys, errors, results, run rows and page JSON use the alias. Proof: a `cmd/ot` test runs the account and ledger wiring with a run-time built invented id and asserts no log record, job key, error text or page JSON contains it; pyside keeps its log scrub and its test.
9. Old rows are renamed, not deleted: the Flex ledger cannot come back, because the store keeps only `raw_sha256` of a Flex fetch (`0022_optionledger.sql`), not the XML, and a new query has a limited look-back. Migration `0037` creates `ops.rename_account_alias(old text, new text)`, which in ONE transaction updates the column `alias` from `old` to `new` in `account.capture`, `account.position`, `account.value`, `account.execution`, `account.position_mark`, `candidates.option_candidate`, `optionledger.flex_trade_row`, `optionledger.flex_section_row`, `optionledger.flex_section_status` (the nine tables with an `alias` column, each named in the file; `optionledger.flex_fetch` (`0022`) and `ops.capture_expected_key` (`0003`) have no `alias` column and are not touched). Rule for a collision: the function first checks, in every table, whether a row under `new` has the same primary key as a row under `old` (the key with `alias` replaced); if one does, it raises one exception naming the table and changes nothing (the transaction rolls back), so there is no half-way state. Collisions cannot happen in the planned order below, so the rule is a guard, not a path. The function runs as the owner role (the writer roles have no UPDATE); `0037` has a `-- +goose Down` that drops the function (it cannot undo a rename). Hashed keys in `ops.capture_expected_key` stay (Left out).
   Order, which holds collection: (a) the window stops serve and the scheduler; (b) `setup pg`, `migrate`, `setup pg`; (c) the window calls the function once per old alias (`a`, `b`, `main`, `A`, `B` as they exist); (d) only then the owner, in ONE step before the new binaries start, writes `accounts.toml` and changes the `OT_ACCOUNT_SOCKETS` line of the service `.env` from the `label=socket` form to the plain socket list (the window never edits `.env`); the window hands this step as one command list; then the new binaries start. Until the file exists the new binaries refuse to start (Issue 2, a missing file is refused; every job and page reads the alias list from the book), and the old binaries are stopped in (a), so no row is written under a new alias before (c). That is the simplest order that makes a collision impossible; the rename is not part of the migration because the migration cannot know the owner's alias names.
10. The two hard-coded owner lists `heldAliases = {"A","B"}` (`internal/jobs/optioneod/held.go`, `internal/jobs/optionopen/inputs.go`) and the third copy `cmd/ot/wire_underlying.go` (`heldAliases`, used by `heldKeys`) become a list given by the caller; unset means no held names (the behaviour of a nil reader), so nothing breaks between merges; `cmd/ot` passes `book.Aliases()`.
11. Other users of the word alias keep it, unchanged, as the stored name stays an alias: `internal/compute` interfaces (`AliasFunds`, `trackapi`, `blockapi`), `optionledger/replay.go`, the generated contracts, the cmd site files. Legacy imports (`internal/jobs/cutover/*`, the alias map of `optionledger/importfile.go`, `cmd/ot/accountimport.go`, `ledgerimport.go`) keep `--map from=to`; `from` is the option-trader name, `to` must be an alias of the book (checked in `cmd/ot` only).
12. Pages show the alias only, never the id; page JSON carries aliases only.
13. `tools/ot-history-export` refusing an id-shaped alias stays right; `tools/ot-flex-export` selects the column `alias`, which stays. Web and datasets need no change.

## Modules

| module | change |
| --- | --- |
| accounts | NEW row (in `docs/design/modules.md` of this PR, with the layer-table row and `accounts` in the depends cell of `ot`): `accounts \| internal/accounts/** docs/contracts/accounts.md \| internal/accounts/api.go \| internal/accounts/api_test.go \|  \|` (no depends); layer table row: outside the layers, reads `$OT_HOME/accounts.toml`, provides the id to alias map |
| jobs | `optioneod`, `optionopen` take the held alias list |
| ot | depends cell gains `accounts`; `cmd/ot/accounts.go` wiring; account, optionledger, site_positions, importers |
| pyside | servicers answer the managed id; gateways `account` no longer read |
| contracts | comment of `account` in `account.proto` and its generated output |
| broker | comment of `Account.Alias`, error texts of grpcbroker |
| sources | flex config without `[[account]]` |
| store | one migration with the rename function |

## Interface

- (interface) `internal/accounts/api.go`, owner accounts: `type Book interface { Alias(id string) (alias string, ok bool); Main() string; Aliases() []string; IDs() []string }` (`IDs` returns every account id of the book in the same order as `Aliases`, so `IDs()[i]` is the id of `Aliases()[i]`; only `cmd/ot` calls it, to build `map[number]alias` per Decision 7) and `var ErrUnknownAccount error` (a sentinel value, no method); `Load` is in `internal/accounts/toml`. Consumed by `cmd/ot` only.
- File `$OT_HOME/accounts.toml` (Decision 1).
- Wire: `AccountResponse.account`, `ExecutionsResponse.account` carry the IBKR account id (contracts comment, implemented by pyside, consumed by grpcbroker and the resolver).
- (interface) `internal/broker/api.go`: comment of `Account.Alias` and `Executions.Alias` only: the value the gateway reports, replaced by the owner alias before a job sees it.
- Store, jobs, pages, web, datasets: unchanged.

## Issues

Entries 1 to 5 are the smallest cut (file, reader, rename, held lists, Flex edge, page join); 6 to 10 follow.

### 1 accounts-interface (interface)
Design line: this file, Decisions 2, 6. Files: `internal/accounts/api.go`, `internal/accounts/api_test.go`.
Acceptance:
1. `api.go` declares exactly `Book` and the sentinel `ErrUnknownAccount` of Interface, no logic; its test checks the contract from both sides with a fake and run-time built invented ids.

### 2 accounts-file
Order: after 1. Files: `internal/accounts/toml/**`, `docs/contracts/accounts.md`.
Acceptance:
1. `Load(path)` refuses, naming the line number and never the id: a missing file, a missing `main`, a `main` naming no alias, a bad alias, the key `main` as an alias, a repeated id, a bad id shape; one test per refusal, files written to `t.TempDir()`.
2. `Alias` of an id not in the file returns `ok == false`; the doc states the format with `<U-number>` as the example and the window order of Decision 9; no file of the issue holds a digit string shaped like an id.

### 3 store-rename-function
Order: none. Files: `migrations/0037_account_alias_rename.sql`, `internal/store/**` (tests that list migrations, found by grep for `0036`).
Acceptance:
1. The migration creates only `ops.rename_account_alias(text, text)`, which in one transaction updates `alias` from old to new in `account.capture`, `account.position`, `account.value`, `account.execution` (`0019`), `account.position_mark` (`0026`), `candidates.option_candidate` (`0018`, `0027`), `optionledger.flex_trade_row` (`0022`), `optionledger.flex_section_row`, `optionledger.flex_section_status` (`0024`), these nine tables only (`optionledger.flex_fetch` and `ops.capture_expected_key` have no `alias` column); each named in the file; no table, column or view changes.
2. In the function body the existence check of a new-alias row with the key of an old-alias row, for every table, comes before the first UPDATE, and its `RAISE EXCEPTION` names the table.
3. The file has a `-- +goose Down` that drops the function.

### 4 jobs-held-aliases
Order: after 3. Files: `internal/jobs/optioneod/held.go`, `internal/jobs/optioneod/**` (tests that use `heldAliases`), `internal/jobs/optionopen/inputs.go`, `internal/jobs/optionopen/**` (tests that use `heldAliases`, found by grep).
Acceptance:
1. The two package variables `heldAliases` are gone. Every existing constructor signature in both packages is unchanged (`NewWithHeldSymbols` and the `deps` types of `cmd/ot` still compile). The list is a new optional input whose zero value means no held names: `optionopen.Stored.HeldAliases []string`, and in `optioneod` a new function `WithHeldAliases(NewOptionEodHeldSymbolsFunc, []string) NewOptionEodHeldSymbolsFunc`.
2. A test per package gives one alias and shows its positions are read, and gives none and shows no read.

### 5 ot-flex-edge-and-pages
Order: after 2, 3, 4, 11. Files: `cmd/ot/accounts.go` (new), `cmd/ot/accounts_test.go` (new), `cmd/ot/optionledger.go`, `cmd/ot/optionledger_test.go`, `cmd/ot/flex_import_post.go` (its one call of `flexAccounts`), `cmd/ot/flex_import_kit_test.go`, `cmd/ot/flex_import_post_test.go`, `cmd/ot/flex_import_site_test.go` (the tests that build `FlexAccount` rows), `cmd/ot/site_positions.go`, `cmd/ot/site_positions_test.go`, `cmd/ot/accountimport.go`, `cmd/ot/accountimport_test.go`, `cmd/ot/ledgerimport.go`, `cmd/ot/ledgerimport_test.go`, `cmd/ot/wire_optionopen.go` (`openStored` sets `Stored.HeldAliases`), `cmd/ot/wire.go` (only `wireHeldCalls` wraps the held constructor with `optioneod.WithHeldAliases(held, book.Aliases())`: at most 4 added lines, the file stays under 800), `cmd/ot/testdata/**` (holding a flex `[[account]]`, found by grep), `docs/contracts/ot/accounts.md` (any other edit of `wire.go`, 795 lines, is reported, not made).
Acceptance:
1. `optionledger.go` builds `Accounts map[number]alias` from the book (`internal/jobs/optionledger/map.go` is untouched and keeps mapping `st.AccountID`); it no longer reads `ff.Accounts` or `AccountEnv` (`flexAccounts` takes the book, and the web import in `flex_import_post.go` takes its alias list from `accounts.toml` the same way, as the same `map[number]alias` from the book, so one rule maps both); a test shows a statement whose id is not in the book stores no row and is the refused state.
2. The account pages take the alias list from `book.Aliases()`, main first; the demo profile keeps `["main"]`.
3. `optioneod` (through `wireHeldCalls`) and `optionopen` (through `openStored`) get `book.Aliases()`.
4. The importers refuse a `--map` target that is not an alias of the book.
5. The no-id test of Decision 8 passes for the ledger wiring and the page JSON.

### 6 ot-gateway-resolver
Order: after 5. Files: `cmd/ot/account.go`, `cmd/ot/account_test.go`, `cmd/ot/accounts.go`, `cmd/ot/accounts_test.go`, `cmd/ot/serve_test.go`, `cmd/ot/site_test.go`, `cmd/ot/candidates_test.go`, `cmd/ot/notices_site_test.go`, `cmd/ot/positionmarks_test.go`, `cmd/ot/wire_underlying_test.go`, `cmd/ot/candidates.go` (`candidateAliases`, which calls `accountAliases` on main since #1896), `cmd/ot/notices_site.go`, `cmd/ot/positionmarks.go`, `cmd/ot/serve.go`, `cmd/ot/site.go` (where serve starts and `site.go` builds the page aliases), `cmd/ot/wire_underlying.go` (the callers of `accountAliases`/`accountMain` and the third `heldAliases`), `cmd/ot/testdata/**` (holding `OT_ACCOUNT_SOCKETS`, found by grep).
Acceptance:
1. `OT_ACCOUNT_SOCKETS` parses as a plain list; a `label=` form is refused; `accountAliases`, `accountMain` and `siteAliasList` no longer read the gateways file, and neither do their callers in `candidates.go`, `notices_site.go` and `positionmarks.go`; `wire_underlying.go` takes the held list from `book.Aliases()` in place of its `heldAliases` variable.
2. The resolver gives `accountcapture.Config.Readers` one reader per alias whose answers carry the alias; tests with fake readers cover a match, an id not in the book (named state `unknown_account`, nothing stored, no id in the text) and a socket down.
3. `serve` (`serve.go`, `site.go`) and the builders of this entry that take the alias list (`candidates.go`, `notices_site.go`, `positionmarks.go`, `wire_underlying.go`) refuse to start when `accounts.toml` cannot be loaded, except under the demo profile (the missing-file refusal of issue 2), so nothing collects under a new alias before the file is written (Decision 9).
4. The no-id test of Decision 8 passes for the account wiring.

### 7 pyside-reports-account-id
Order: after 6. Files: `pyside/src/optdesk/ibkr/transport.py`, `pyside/src/optdesk/ibkr/gateways.py`, `pyside/examples/gateways.example.toml`, `pyside/tests/ibkr/test_accounts.py`, `pyside/tests/ibkr/test_gateways.py`, `pyside/tests/ibkr/test_logfilter.py`, `pyside/src/optdesk/ibkr/accounts.py`, `pyside/src/optdesk/ibkr/model.py`, `pyside/tests/ibkr/test_executions.py`, `pyside/tests/ibkr/test_server.py`, `pyside/tests/contracts/test_contracts_api.py` (the readers put the id on the rows, so `accounts.py` and `model.py` change; `pyside/tests/contracts/test_contracts_api.py` is a path of the contracts module and is in this entry because it constructs `AccountServicer` with an alias).
Acceptance:
1. Both servicers answer `str(managedAccounts()[0])` in `account` and no longer read `self._alias`; two managed accounts is still `MultipleAccounts`.
2. `gateways.py` neither requires nor rejects `account`; no code path uses it.
3. A test with a fake `ib` shows the id in both responses and in no log record; the existing scrub tests are unchanged.

### 8 contracts-account-field-comment
Files: `contracts/proto/ibkr/v1/account.proto`, `internal/contracts/genpb/account.pb.go`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/account_pb2.pyi` (the generated output in the same commit, and `account_pb2.py` beside it when the generator rewrites it).
Acceptance:
1. The comments of `account` in both messages say it is the IBKR account id the gateway reports; "never the account id" is gone. No field, number or type changes; the PR body says `Contracts: comment only`, not `unchanged`.

### 9 broker-account-comment (interface)
Design line: this file, Interface. Order: after 8. Files: `internal/broker/api.go`, `internal/broker/api_test.go`, `internal/broker/grpcbroker/account.go`, `internal/broker/grpcbroker/convert.go`, `internal/broker/grpcbroker/convert_test.go`, `internal/broker/grpcbroker/account_test.go`.
Acceptance:
1. The comments of `Account.Alias`, `Executions.Alias` and `AccountClient.Account` say what Interface says; no signature changes; the `permanent(...)` texts say "answer has no account" and the tests match.

### 10 sources-flex-no-account-table
Order: after 5, 6. Files: `internal/sources/flex/config.go`, `internal/sources/flex/config_test.go`, `internal/sources/flex/testdata/**` (holding `[[account]]`, found by grep), `internal/sources/api.go` (the `FlexAccount` type: an interface change, Design line this file), `docs/contracts/sources.md`.
Acceptance:
1. `flex.toml` parses with `[[login]]` and token env names only; `FlexConfig` and `api.go` have no `FlexAccount`; a file with `[[account]]` is refused as an unknown table; `parse.go` is untouched.

## Checked by CI

- `scripts/hygiene` (checks.yml): no committed file holds an id-shaped string; the tests of issues 2, 5, 6 build their ids at run time.
- `go test`, `go-arch-lint` generated from the `accounts` row and the `ot` depends cell, `buf` and generated-output check (issue 8), `go test -tags pg` for the migration of issue 3 (rename in the nine tables of Decision 9, and the collision refusal) (GitHub CI only).
- `ruff check`, `mypy --strict`, `pytest -n auto` for issue 7.

## Left out

- Keying the store by id; if an IBKR number changes, the owner edits the id in `accounts.toml`, history keeps its alias.
- Old run-ledger and expected-key rows of `account_capture` and `option_ledger` (hashed alias keys) are not renamed; if they show as gaps, a follow-up deletes them.
- Gateway `name` slugs of pyside, flex `[[login]]` names and socket paths stay: they name logins and sockets, not accounts. A gateway with several managed accounts stays refused by pyside.
- Showing the id on pages; an `ot` command for the rename (the window calls the function).

## Adversarial

Full review: `account-identity-adversarial.md` in the designer's scratchpad (a local file, not committed).

1. Gap against the goal (datasets, web, surviving names, importers, run rows): adopted by keeping the column, so datasets and web need no change (Decision 13); importers in Issue 5.4; run rows in Left out; `name` slugs in Left out.
2. Id in logs, jobs, page JSON: adopted. Decision 8 and the test in Issues 5 and 6.
3. Simpler way: adopted, cut A, with the coordinator's change that the reader is one owned module (Decision 2) and the rename replaces the delete. Cut B (id as key) not adopted: it costs eight tables and every page for no owner-visible gain.
4. Lock-in: the id is not a key, so the case disappears (Left out).
5. False claims: migration number is 0037; `internal/accounts` gets its row and the `ot` depends cell (Modules); hygiene scan and held lists are Decisions 1, 10; no issue spans modules except entry 7, which also holds `pyside/tests/contracts/test_contracts_api.py` (contracts module) because that test constructs `AccountServicer` with an alias and must change with it; `wire.go` (795) is not edited.
6. Slip cause: the 6-module interface issue is gone; chain is 1, then 2 and 3 in parallel, 4 after 3 (they share the tests of the store package that list migrations), then 5, 6, 7; the migration and deploy order is Decision 9.

### 11 accounts-ids (interface)
Order: after 1, 2. Design line: this file, Decision 7 and Interface. Files: `internal/accounts/api.go`, `internal/accounts/api_test.go`, `internal/accounts/toml/book.go`, `internal/accounts/toml/book_test.go`, `docs/contracts/accounts.md` (one entry because `Book` gains a method and its only implementers, the toml book and the fake of `api_test.go`, must compile in the same change; no `Book` implementer exists outside `internal/accounts`, found by grep).
Acceptance:
1. `api.go` declares `IDs() []string` on `Book` with a comment that it returns every account id in the order of `Aliases()`; nothing else in `api.go` changes. The fake of `api_test.go` implements it and the test checks `len(IDs()) == len(Aliases())` and that `Alias(IDs()[i])` equals `Aliases()[i]`, with run-time built invented ids.
2. The toml book returns its ids in the order the owner wrote the lines, as a copy: a test loads a file of two accounts written to `t.TempDir()`, checks the order and that changing the returned slice does not change a second call.
3. `docs/contracts/accounts.md` names `IDs` among the methods of `accounts.Book` and says the ids come in the order of `Aliases()`.
