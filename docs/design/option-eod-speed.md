# option_eod speed: the in-session chain snapshot reuses the contracts resolved before the open and resolves the rest in parallel

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-19

Source: owner ask optdesk-5067 (2026-11-19): "why does the in-session pull take over an hour when the contracts were prepared before the open?" Owner-visible result: `option_eod` (the in-session chain snapshot at session open plus 90 minutes, `docs/design/option-chain-daily.md`) for a normal day ends within 20 minutes of its start, about the time `option_open` takes, instead of about 76 minutes (measured on the scheduler log of 2026-11-18). Its outputs (open interest, model IV and greeks, the strike band, the rows and their capture ids) do not change.

Read on origin/main at 5a569d25: `AGENTS.md`, `docs/design/modules.md`, `docs/design/option-chain-daily.md`, `docs/design/option-open-pull.md` (Decisions 1, 3, 5), `docs/design/restart-resume.md` (Decision on `option_open`; it states it does not touch `option_eod`), `internal/jobs/optioneod/{run,select,plan,types,withreasons}.go` and `run_test.go`, `internal/jobs/optionopen/pull.go`, `internal/store/openquote/api.go`, `internal/broker/api.go`, `internal/broker/grpcbroker/dial.go`, `cmd/ot/wire_optioneod.go`, `cmd/ot/wire_optionopen.go`.

## Today

- Cause 1, no cache: `runKey` (`internal/jobs/optioneod/run.go:178-221`) selects the pairs of a stock (`pairs`, run.go:195) and then `listed` (run.go:372-392) calls `ResolveContract` once for each pair, one after the other, to learn its con_id and whether it is listed. Nothing reads `master.option_contract`, the table the evening job `option_open_resolve` fills before the open (`option-open-pull.md` Decision 3 and 5), so the same contracts are resolved again every day in session. `option_open` reads them (`internal/jobs/optionopen/pull.go:360-425`) and goes straight to the quote.
- Cause 2, one resolve in flight: the broker client lets 5 resolves run at once (`ResolveInFlight`, `internal/broker/api.go:137`, `internal/broker/grpcbroker/dial.go:25`), the job sends one (`for _, p := range pairs`, run.go:374). The chunks of quotes are one call in flight in both jobs (`chunk` of run.go:395; `Core.ask`, `internal/jobs/optionopen/core.go:410`), so the quote part is the same and the resolve part is the difference.
- The stocks run one key at a time (`Run` takes the keys of the plan in a loop, run.go:132), each key with its own budget of `budget_minutes` (job.toml:19, checked at run.go:149).
- What the evening resolve stores is not the whole eod selection: its band and expiry window (20 to 45 days, strikes to `store_k` 1.75) differ from the eod window (`min_dte` 14, `max_dte` 56, the buy-price band). So a stored contract is used when there is one and the rest is resolved at run time.

## Decisions

1. Resolve the pairs of a key with up to 5 in flight, in pair order for the result, stopping at the first error in pair order, checking the budget before each ask (cause 2). The width is a constant of the job equal to the broker gate (a larger number would only queue in the gate); the pacing stays where it is, in the broker client. 5 is a ceiling, not a rate: below the client the gateway pacer of `docs/adr/0003-market-data-architecture.md` (entries 2026-10-16 and 2026-10-17) starts contract details at 2 in flight, raises or halves the limit by the answer times and shares it per gateway across every connection of the process, so a slow gateway lowers the real width and the load is bounded there, not by this job. The job uses the shared client (`jc.brk`) with its default gate; the three resolve jobs dial a client of their own with 1 in flight (`cmd/ot/wire_optionopen.go`) and are not changed. The gain is up to five times on the resolves that are left, in practice less.
2. Read first: a pair that `master.option_contract` holds for the stock is not sent to `ResolveContract`; its con_id comes from the instrument of the stored reference (`ibkr_conid` identifier), exactly as `option_open` does. A stored pair becomes the pair with that con_id and is not checked for listing; a contract delisted since the evening is quoted and gives a no-quote row where today it is dropped (accepted, rare). A pair not stored is resolved by Decision 1. The match is by trading class, expiry, strike value, right and multiplier. A store error is not fatal: the stock falls back to Decision 1 for all its pairs and one Warn line is logged.
3. The job gets the reader through `optioneod.WithContracts`, a function of its own file like `WithReasons`, and the reader types are declared in that file, not in `types.go`: the interface file of the package is not edited. A job without the call behaves as today.
4. `cmd/ot` wires the reader when the option open store is configured (`openDSN` set, as for `option_open`), through the opener `deps.openOptionOpen` (`cmd/ot/wire.go`) and the instrument reader the factory has, inside the callback of `optionEod` that builds the job (the job is rebuilt for each run by `listedJob`, so the call cannot be made on the job `optionEod` returns); `option_eod` does not record itself as not wired. `preflight` and the dry run call `optionEod` without a reader and are unchanged. With no line or a store that does not open, `option_eod` runs as today and logs one line.
5. The rows do not change: the snapshot asks the same contracts (same con_ids, same order) in the same chunks, so rows, capture id and status are the same. This is pinned by a test that runs a key with and without stored contracts.
6. One Info line per key, `optioneod: contracts`, with the counts stored, resolved and not listed and the seconds of the resolve, so the share of the evening resolve and the time split are read from the scheduler log of the next trading day. Owner-visible check: `option_eod` of that day ends within 20 minutes; a share of resolved pairs that keeps it above is the signal for the widening in Left out.
7. Own implementation, because: a worker pool of five over a slice is a few lines of the standard library (`sync`), and `golang.org/x/sync` is only an indirect dependency, so a direct one would touch `go.mod` of the repo row for no gain.

## Modules

No row of `docs/design/modules.md` changes. Entries 1 and 2 are in the jobs row (`internal/jobs/**`), entry 3 in the ot row (`cmd/**`). The job imports `internal/store/openquote` for the type `OptionRef`; jobs may depend on store. No interface file is edited: `internal/jobs/api.go`, `internal/jobs/optioneod/types.go` and `internal/store/openquote/api.go` stay as they are.

## Interface

None changes. New in the package `optioneod`, file `withcontracts.go`: `ContractReader` (`ListOptions(ctx, underlyingID int64) ([]openquote.OptionRef, error)`, the read side of `openquote.OptionContractStore`) and `WithContracts(jb contracts.Job, c ContractReader, in store.InstrumentReader) (contracts.Job, error)`: a nil reader returns the job unchanged, a job not built by this package is an error.

## Issues

### 1 eod-resolve-parallel

Module jobs. `internal/jobs/optioneod/run.go`, new `internal/jobs/optioneod/resolve.go`, `internal/jobs/optioneod/run_test.go` (its fake broker counts without a lock and records the resolve order), new `internal/jobs/optioneod/resolve_test.go`. Decision 1 and 5. Order: first. Size: M.

### 2 eod-stored-contracts

Module jobs. new `internal/jobs/optioneod/withcontracts.go`, new `internal/jobs/optioneod/stored.go`, `internal/jobs/optioneod/run.go`, new `internal/jobs/optioneod/stored_test.go`. Decisions 2, 3, 5, 6. Order: after 1. Size: M.

### 3 eod-wire-stored

Module ot. `cmd/ot/wire_optioneod.go`, new `cmd/ot/wire_optioneod_stored_test.go`. Decision 4. Order: after 2. Size: S.

## Checked by CI

`go build ./...`, `go vet ./...`, `go test -race` of `./internal/jobs/optioneod/...` and `./cmd/ot/...`, `go run ./scripts/arch --check`.

## Adversarial

Raw findings: `adversarial-raw.md` in the draft directory.

- No number for the share of pairs the evening resolve does not store, and a 20-minute claim proved by invented fake-clock numbers: adopted. Decision 6 adds the counts line; item 10 of issue 2 (circular) is replaced by a test of that line; the 20 minutes stay a result read on the host.
- "A fifth of the time" and the load worry (the resolve jobs use 1 in flight): adopted. Decision 1 states the ceiling, the adaptive per-gateway pacer of ADR 0003 and the shared client; a client of its own for eod is not adopted, the pacer already bounds the gateway and a second client would split its per-connection pacing.
- Stored pair quoted without a listing check, and pair plus con_id instead of the broker's contract: adopted as named in Decision 2 and the check in Decision 6; a listing check against `master.option_not_listed` is not adopted (second store, in Left out).
- Wiring on the job `optionEod` returns, and `optionopenpg.Open` without the test seam: adopted, Decision 4 and issue 3 rewritten.
- Existing tests: `resolved` order of the fake under concurrency and the early resolve of the underlying: adopted in issues 1 and 2 (sorted list; early resolve only with a reader).
- Simpler ways (take the con_id from the chain, write run-time contracts back, widen the evening band now): not adopted; the chain has no con_id, the resolve job is the only writer, the widening is another design and waits on the measure.
- Lock-in of the width constant: not adopted, one comment names the gate.
- Budget per key: checked, true (one key per run), no change.

## Left out

- Quotes of several chunks or several stocks at once: the open pull asks one call at a time too, and the gateway paces quotes by its market-data lines; the 20 minutes come from the resolve, not from the quotes.
- `option_eod_jp` and the index jobs: they share `runKey` and so get Decision 1 at once; reading the stored contracts for the JP names is a later entry of the same shape (the Japan resolve jobs of `option-open-pull.md` exist).
- Writing the contracts that `option_eod` resolved at run time into `master.option_contract`: the resolve job is its only writer (`option-open-pull.md` Decision 6); the gap stays a run-time cost until the resolve band covers the eod window.
- Widening the evening resolve band to the eod window (14 to 56 days, buy-price band): a change of `option-open-pull.md`, to be taken if the measured gap after this design is still large.
- `budget_minutes`, the slot and the run bound: unchanged.
- Marks of `master.option_not_listed` to skip pairs known not listed: a later step; not needed to reach 20 minutes and it reads a second store.
