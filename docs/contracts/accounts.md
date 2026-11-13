# accounts contract

## Responsibility

The owner's account book. `internal/accounts` declares `accounts.Book` (`Alias`, `Main`, `Aliases`, `IDs`) the optional interface `accounts.Sourced` (one method, `Statement() []string`: the aliases whose data comes only from the broker statement, in file order, never the main alias; a caller asks `book.(Sourced)`, and a book without it has none) and the sentinel `ErrUnknownAccount`; `internal/accounts/toml` holds `Load(path)`, which reads `$OT_HOME/accounts.toml` and returns the book. The file lives only under `$OT_HOME` and is never committed. Design: `docs/design/account-identity.md`.

## File format

One line per account, `<alias> = "<U-number>"`, one line `main = "<alias>"` and the optional line `statement = [...]`:

```toml
mine = "<U-number>"
wife = "<U-number>"
main = "mine"
statement = ["wife"]
```

- An alias matches `^[a-z][a-z0-9_-]{0,31}$`. `main` and `statement` are reserved as keys: neither is ever an alias.
- An id is a `U` and 5 to 9 digits. Each id and each alias appears once.
- `main` names an alias of the file.
- `Aliases()` lists the aliases in the order the lines are written; `Alias(id)` returns `false` as its second result for an id the file does not hold.
- `IDs()` lists the account ids in the order of `Aliases()`, so `Alias(IDs()[i])` is `Aliases()[i]`; `cmd/ot` pairs them to build its id-to-alias map.
- `statement` is an array of strings, each an alias of the file, written once and not the main alias; it declares the accounts with no gateway, and `Statement()` returns them in the order written, as a copy. A file without the line loads as before and `Statement()` is empty.
- Comments and blank lines are allowed; tables, dotted keys and non-string values are not, except the `statement` array of strings.

## Errors

`Load` refuses, and the error names the line number and never an id: a missing file, a missing `main`, a `main` naming no alias, a `statement` that is not an array, has an item that is not a string, names no alias of the file, repeats an alias or names the main alias, a second `statement` line, a bad alias, the key `main` or `statement` as an alias, a repeated alias or id, a bad id shape, text that is not TOML. The caller (`cmd/ot`) refuses to start on any of them.

## Migration order (Decision 9)

(a) the window stops serve and the scheduler; (b) `setup pg`, `migrate`, `setup pg`; (c) the window calls the rename function once per old alias; (d) only then the owner, in one step before the new binaries start, writes `accounts.toml` and changes the `OT_ACCOUNT_SOCKETS` line of the service `.env` to the plain socket list; then the new binaries start. Until the file exists they refuse to start.
