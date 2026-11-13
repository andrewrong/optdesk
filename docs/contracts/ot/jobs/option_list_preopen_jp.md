# ot job `option_list_preopen_jp`

Order: 187

Family file: `cmd/ot/wire_optionlist.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`) through the same family as `option_list_preopen`. The common rules of every job are in `docs/contracts/ot.md`. Design: `docs/design/option-open-gap-jp.md#Same-morning-visibility` (entry 8).

The JP list: dataset `master.option_list.jp`, subject `preopen`, from the open 09:00 JST to `stop_time` 09:20 JST, window 14 to 90 DTE. JP has no pre-open re-check and no JP zone job: a JP gap at the open is not seen the same day before the open, and the runs page shows the JP list key the same morning (a narrowing of the design, asks 3015 and 3021).

## Window

The job states its own window, slot and `stop_time` through its manifest in `internal/jobs/optionopen`; `ot` passes the zero `runner.WindowRule{}` and adds no time rule.

## Lines it reads

- `OT_PG_DSN_OPTION_OPEN`: the store of `optionopenpg`, opened by `openOptionOpen` and closed through the closer set; it is also the `OptionListStore`.
- `OT_PG_DSN_MASTER`, `OT_PG_DSN_BAR`: the instrument and bar stores; `$OT_HOME/watch.toml`: the watch list.
- The broker socket: a client of its own dialled with one resolve in flight. It is the `ContractLister`, the `Snapshotter` (`Spot`) and the `HistoryClient` (`History`) of the job.
- The reason writer of the ledger (`Reasons`, `jc.reasons`): a ledger that cannot write reasons is a start error naming the job, never a nil port.

## Left out

- A missing `OT_PG_DSN_OPTION_OPEN`, instrument line or bar line: the job is left out with one startup line, and under `ot scheduler` a `not_wired` row (dataset `ops.not_wired`, scope `option_list_preopen_jp`) names the key, never a value.
- A store without the list port: the job is left out with one startup line and a `not_wired` row naming `OT_PG_DSN_OPTION_OPEN`; it is not held.
- The store cannot be opened, or the broker has no `ContractLister`: the job is held with its reason (`list_absent`, `broker is not reachable`).

## Version

v1: wired by `ot run-once` and `ot scheduler` (issue #1972).
