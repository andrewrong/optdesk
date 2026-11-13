# ot job `account_capture`

Order: 100

Family file: `cmd/ot/account.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}` (see `docs/contracts/jobs.md` for the job).

## Lines it reads

- `OT_ACCOUNT_SOCKETS`: a comma list of `label=socket path`, one `grpcbroker.DialAccount` per entry.
- `OT_IBKR_GATEWAYS`: the gateways file; the main label is the `account` of its one `required = true` entry.
- `OT_PG_DSN_ACCOUNT`: the account store (role `ot_w_account`).

## Left out

- Absent only for a deps without `newAccountCapture`.
- It is not left out but refused: an unset or malformed `OT_ACCOUNT_SOCKETS`, a gateways file with no or several required entries, a main label not in `OT_ACCOUNT_SOCKETS` or an unset `OT_PG_DSN_ACCOUNT` is an error naming the key only, before any socket is dialled or store opened.
- The readers of the statement aliases of the book (`accounts.Sourced`) are left out: a statement alias gets no reader, no row and no "degraded" line.
- A gateway that cannot be dialled becomes a failed read of its alias (degraded when it is not main).

## Notes

`position_marks` (wired by `cmd/ot/wire.go`, not a registered family) follows it with the same aliases.

## Version

Introduced before the registry; moved to its own document with the job registry (no behaviour change).
