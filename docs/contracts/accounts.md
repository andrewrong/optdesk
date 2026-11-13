# accounts contract

## Responsibility

The owner's account book. `internal/accounts` declares `accounts.Book` (`Alias`, `Main`, `Aliases`, `IDs`) and the sentinel `ErrUnknownAccount`; `internal/accounts/toml` holds `Load(path)`, which reads `$OT_HOME/accounts.toml` and returns the book. The file lives only under `$OT_HOME` and is never committed. Design: `docs/design/account-identity.md`.

## File format

One line per account, `<alias> = "<U-number>"`, and one line `main = "<alias>"`:

```toml
mine = "<U-number>"
wife = "<U-number>"
main = "mine"
```

- An alias matches `^[a-z][a-z0-9_-]{0,31}$`. `main` is reserved as a key: it is never an alias.
- An id is a `U` and 5 to 9 digits. Each id and each alias appears once.
- `main` names an alias of the file.
- `Aliases()` lists the aliases in the order the lines are written; `Alias(id)` returns `false` as its second result for an id the file does not hold.
- `IDs()` lists the account ids in the order of `Aliases()`, so `Alias(IDs()[i])` is `Aliases()[i]`; `cmd/ot` pairs them to build its id-to-alias map.
- Comments and blank lines are allowed; tables, dotted keys and non-string values are not.

## Errors

`Load` refuses, and the error names the line number and never an id: a missing file, a missing `main`, a `main` naming no alias, a bad alias, the key `main` as an alias, a repeated alias or id, a bad id shape, text that is not TOML. The caller (`cmd/ot`) refuses to start on any of them.

## Migration order (Decision 9)

(a) the window stops serve and the scheduler; (b) `setup pg`, `migrate`, `setup pg`; (c) the window calls the rename function once per old alias; (d) only then the owner, in one step before the new binaries start, writes `accounts.toml` and changes the `OT_ACCOUNT_SOCKETS` line of the service `.env` to the plain socket list; then the new binaries start. Until the file exists they refuse to start.
