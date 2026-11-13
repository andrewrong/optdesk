# ot accounts wiring

## Responsibility

`cmd/ot` is the only module that depends on `accounts`. `cmd/ot/accounts.go` constructs the book from
`$OT_HOME/accounts.toml` (format: `docs/contracts/accounts.md`) and hands plain data to sources, jobs and pages.
Design: `docs/design/account-identity.md#5 ot-flex-edge-and-pages`.

## What each consumer gets

- Flex ledger job and the web Flex import: `map[number]alias`, built by pairing `Book.IDs()` with `Book.Aliases()`.
  `internal/jobs/optionledger/map.go` still maps `st.AccountID`; a statement whose id is not in the book stores no
  row and its fetch record derives the state `unconfigured`.
- Account pages: the alias list with `Book.Main()` first, then the other aliases in file order. The demo profile
  keeps `["main"]` and reads no file.
- `option_eod`: `optioneod.WithHeldAliases(held, book.Aliases())` in `wireHeldCalls`; `option_open`:
  `Stored.HeldAliases` in `openStored`.
- `account-import` and `ledger-import`: a `--map from=to` whose `to` is not an alias of the book is refused before
  anything is read or written. The check lives in `cmd/ot` only.

## Start refusal

Under any profile but demo, run-once, the scheduler and `serve` refuse to start when `accounts.toml` cannot be
loaded (a missing file included), before a store is opened. The error text names a line or the file name, never a
path or an account id. Ids stay inside the book and the one id-to-alias map; logs, job keys, errors and page JSON
carry aliases only.
