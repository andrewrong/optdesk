# ot job `option_list_preopen`

Order: 185

Family file: `cmd/ot/wire_optionlist.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`. Design: `docs/design/option-contract-lists.md#Decisions`.

The job has one key: dataset `master.option_list`, subject `preopen`. Order 185 places it before the gap pass (190) in the registration list only; the keys of a turn run by DatasetID.

## Window

The job states its own window, slot and `stop_time` through its manifest in `internal/jobs/optionopen`: window calendar `XNYS`, run bound 47 minutes. `ot` passes the zero `runner.WindowRule{}` and adds no time rule, no `late_start` refusal and no guard of its own.

## Lines it reads

- `OT_PG_DSN_OPTION_OPEN` (process environment or env file): the store of `optionopenpg`, opened by the family through `openOptionOpen` and closed through the closer set. It is also the `OptionListStore` (`master.option_list`, `master.option_list_run`).
- `OT_PG_DSN_MASTER`: the instrument store; `OT_PG_DSN_BAR`: the bar store (the prior close).
- `$OT_HOME/watch.toml`: the watch list.
- The broker socket `$OT_HOME/run/ibkr.sock` (a profile value can name `ibkr-main.sock`): the shared broker client, used as `broker.ContractLister`.

`History`, `Spot` and `ReasonWriter` are not set here (entry 8 of `docs/design/option-open-gap-jp.md` wires them): a name with no stored close has no spot and the run is Retryable, and no reason rows are written.

pyside must be restarted before the job lists anything: until then the service does not serve the call and the job ends `list_unserved`.

## Left out

- `OT_PG_DSN_OPTION_OPEN` missing (or the instrument or bar store missing): the job is left out with one startup line that names the job and the line; the other jobs run.
- The store cannot be opened: the job is held with that reason; the scheduler lists it as disabled and `run-once` leaves it out.
- A failed broker dial at start (`broker is not reachable`, the `startOff` of the family `ifOpenDSN(brokerDownOff)`, set only when `OT_PG_DSN_OPTION_OPEN` is set): `ot scheduler` lists it as disabled and probes it again at each turn.
- `list_absent`: the broker has no `ContractLister` or the store no `OptionListStore`. The job is held with that reason, one startup line names the job and `list_absent` (once per start), the scheduler lists it as disabled and `run-once` leaves it out; a stand-in answers Permanent, so the key ends Permanent for the day.

## Version

v1: wired by `ot run-once`, `ot scheduler` (issue #1954).
