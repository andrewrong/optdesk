# ot job `option_list_zone`

Order: 186

Family file: `cmd/ot/wire_optionlist.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`) through the same family as `option_list_preopen`. The common rules of every job are in `docs/contracts/ot.md`. Design: `docs/design/option-open-gap-jp.md#Same-morning-visibility` (entry 8).

The US zone slot: dataset `master.option_list`, subject `zone`, 08:15 ET (open minus 75 minutes) to `stop_time` 08:40 ET. It reads a pre-open spot of each name and lists again the expiries whose target zone left the stored range; it writes groups and reason rows (`outside_range`, `no_spot`, `late_start`) and no list run.

## Window

The job states its own window, slot and `stop_time` through its manifest in `internal/jobs/optionopen`; `ot` passes the zero `runner.WindowRule{}` and adds no time rule.

## Lines it reads

- `OT_PG_DSN_OPTION_OPEN`: the store of `optionopenpg`, opened by `openOptionOpen` and closed through the closer set; it is also the `OptionListStore`.
- `OT_PG_DSN_MASTER`, `OT_PG_DSN_BAR`: the instrument and bar stores; `$OT_HOME/watch.toml`: the watch list.
- The broker socket: a client of its own dialled with one resolve in flight. It is the `ContractLister`, the `Snapshotter` (`Spot`) and the `HistoryClient` (`History`) of the job.
- The reason writer of the ledger (`Reasons`, `jc.reasons`): a ledger that cannot write reasons is a start error naming the job, never a nil port.

## Left out

- A missing `OT_PG_DSN_OPTION_OPEN`, instrument line or bar line: the job is left out with one startup line, and under `ot scheduler` a `not_wired` row (dataset `ops.not_wired`, scope `option_list_zone`) names the key, never a value.
- A store without the list port: the job is left out with one startup line and a `not_wired` row naming `OT_PG_DSN_OPTION_OPEN`; it is not held.
- The store cannot be opened, or the broker has no `ContractLister`: the job is held with its reason (`list_absent`, `broker is not reachable`).

## Version

v1: wired by `ot run-once` and `ot scheduler` (issue #1972).
