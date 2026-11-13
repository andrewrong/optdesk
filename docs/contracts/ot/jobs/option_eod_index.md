# ot job `option_eod_index`

Order: 75

Family file: `cmd/ot/wire_optioneodindex.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Lines it reads

- The same as `option_eod`: `OT_PG_DSN_OPTION_EOD` (the writer store), `OT_PG_DSN_MASTER` (the instrument store), the broker socket `$OT_HOME/run/ibkr.sock`.
- The price bar reader and the index id reader of the compute stores, both optional: together they make the reader of index closes (the `trades` closes of a named index, oldest first; an unknown or ambiguous name is an error).

## Left out

- A failed broker dial at start (`startOff` is `brokerDownOff`): as `option_eod`, the probe brings the job back on the first passing tick.
- When either reader is missing the job runs without closes: it is built with a nil reader and one Warn line `option_eod_index runs without closes`.

## Notes

The family takes the constructor `optioneod.NewOptionEodIndexFunc` as an argument and is given `optioneod.NewIndex`; the job is wrapped with the reason writer as `option_eod` is.

## Version

v1: wired at order 75.
