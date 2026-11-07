# Store logger: a failed option terms insert is never silent

Author: designer of optdesk
Source: `## Left out` item 6 of `docs/design/option-page-live.md` ("a failed terms insert is silent ... a log line needs a logger in the
store, its own design") and the advisor's request that a failed terms insert is never silent. Read on main at fefd76d.

## What main holds today

- `internal/store/instrument/resolve.go` holds `ResolveInstrument` (129 lines): one transaction through `s.db.InTx`; it has no savepoint and
  writes no option terms yet. `option-terms-written` of `docs/design/option-page-live.md` (not merged) adds the terms insert inside a savepoint
  and, on failure, runs `ROLLBACK TO SAVEPOINT`, commits the instrument and identifier inserts, and returns the id with a nil error. After
  that work lands the failure is invisible to the caller, the job and the page (the page shows the gap values).
- No file of `internal/store` logs: `grep -rn 'slog' internal/store` finds only `internal/store/setup/setup.go`, which takes a
  `*slog.Logger` in its unexported `run` and passes `slog.Default()` from its exported entry. `cmd/ot` logs with `a.log.Info(msg, "component",
  "ot", "status", "ok")` lines through `log/slog` (`cmd/ot/main.go` builds `slog.New(profile.NewLogHandler(...))`). `log/slog` is the one
  logging library of the repository.
- The profile handler (`internal/profile/logging.go`) drops every attribute whose key is not in `logFields` (`time level msg component job
  dataset run_id attempt status rows duration_ms err profile version`) and replaces a whitelisted account-shaped string by `[redacted]`.
- `Open` of `internal/store/instrument` has the fixed type `store.OpenInstrumentFunc` (`func(ctx, dsn) (InstrumentStore, error)`); `cmd/ot/wire.go`
  passes `instrument.Open`.

## Decisions

1. The seam is an unexported field `log *slog.Logger` on `instrumentStore`, as `setup` does with `run`. `Open` sets it to `slog.Default()`;
   a test builds the store with its own logger over a buffer. No exported name, no `api.go` change, no change of `OpenInstrumentFunc`, no
   new dependency. A nil field means `slog.Default()`, so a store built without it still works.
2. One line per failed terms insert, level `WARN`, message `option terms insert rolled back`, attributes `component=store`,
   `status=rolled_back`, `instrument_id=<n>` and `err=<class>`. The line is the count: `grep -c 'option terms insert rolled back'` over
   the log of a run is the number of rolled-back inserts. No counter type and no metric are added.
3. The class is never the error text, because a database message can carry values (a duplicate-key detail repeats the key). It is the
   SQLSTATE code (five characters, for example `42501` for a missing grant) when the error has a `SQLState() string` method, which pgx's error
   has, found with `errors.As` against a local interface so `instrument` still imports no new package; otherwise it is `other`. The line carries
   no strike, expiry, right, osi value, name or DSN.
4. The line is written after `InTx` returns nil, not at the failure: the closure records the class in a local variable. If the outer
   transaction fails later (a lock or commit error), the instrument was not created and no line is written for it; the caller already gets that
   error. A resolve that finds an existing key, a type other than `option`, an option with no valid `osi`, or a terms insert that succeeds, logs nothing.
5. The savepoint behaviour does not change: the return value, the nil error and the committed instrument stay as `option-terms-written` makes them.
6. The attribute key `instrument_id` is not in the profile whitelist. When `ot` later passes its handler as the default logger the key is
   dropped and only `component`, `status`, `err` and the message remain: see `## Left out` 1.

## Modules

No row of `docs/design/modules.md` is added or changed, so `arch.toml` and `.go-arch-lint.yml` are unchanged. `internal/store/**` and
`docs/contracts/store.md` are store. No new dependency (`log/slog` is the standard library).

| issue | module |
|---|---|
| store-terms-log | store |

## Interface

No interface change. `internal/store/api.go` is untouched; the seam is unexported, inside the package `internal/store/instrument`.

## Issues

### store-terms-log

Module: store.

Files: internal/store/instrument/open.go, internal/store/instrument/resolve.go, internal/store/instrument/termslog.go, internal/store/instrument/termslog_test.go, docs/contracts/store.md

Depends on: option-terms-written

Order: after option-terms-written (it edits `internal/store/instrument/resolve.go` and `docs/contracts/store.md` after that work, and the savepoint it logs is that work's).

Design: docs/design/store-logger.md#decisions

Goal: a rolled-back option terms insert in `ResolveInstrument` writes one log line with the instrument id and the error class, never values, through a logger the store holds.

Acceptance:

1. `internal/store/instrument/open.go` gives `instrumentStore` a field `log *slog.Logger` and `Open` sets it to `slog.Default()`; `grep -c 'var _ store.OpenInstrumentFunc = Open' internal/store/instrument/open.go` prints 1 and `git diff --stat origin/main -- internal/store/api.go` prints nothing.
2. `internal/store/instrument/termslog.go` holds a method that returns the field or `slog.Default()` when it is nil, a function `sqlClass(err error) string` that returns the `SQLState()` of the first error in the chain that has one, else `other`, and the method that writes the line of decision 2; `go vet ./internal/store/instrument` passes and `grep -n 'pgconn\|pgx' internal/store/instrument/termslog.go` prints nothing.
3. `internal/store/instrument/resolve.go` records the class in a local variable where the terms insert fails and the savepoint is rolled back, and calls the log method only after `InTx` returned nil; `TestResolveTermsFailureLogged` in `internal/store/instrument/termslog_test.go`, with a fake database whose terms insert returns an error whose text contains a planted marker and whose `SQLState()` is `42501`, finds exactly one line holding `option terms insert rolled back`, `instrument_id=` with the returned id, `err=42501` and `component=store`, finds no planted marker, and still sees the id returned with a nil error.
4. `TestResolveTermsFailureNoState` (the same fake with an error that has no `SQLState` method) finds `err=other`; `TestSQLClass` is a table of an error with a state, an error without one, and an error with a state wrapped twice.
5. `TestResolveTermsFailureOuterError` (the terms insert fails and then the commit or a later statement fails, so `ResolveInstrument` returns an error) finds no line; `TestResolveNoTermsNoLog` (an existing key, a type other than `option`, an option with no `osi`, an unparsable `osi`, and a successful terms insert) finds no line.
6. Every test of `termslog_test.go` calls `t.Parallel()`, builds the store with a handler over its own `bytes.Buffer` and never sleeps; `go test ./internal/store/instrument -count=1 -race` passes offline.
7. `docs/contracts/store.md` states the line of decision 2 (message, the four attributes, that `err` is a SQLSTATE or `other`, never an error text) in the paragraph on `ResolveInstrument`; it adds no version line, because no interface or schema changes.
8. `internal/store/instrument/termslog.go` and `internal/store/instrument/termslog_test.go` stay under 600 lines, and `wc -l internal/store/instrument/resolve.go` prints less than 800.
9. No file outside the five listed is in the diff, and none of them is a file generated from `contracts/`.
10. The result matches the section Decisions (points 1 to 5) of docs/design/store-logger.md, judged from the code and the design file.

## Left out

1. `ot` hands its own logger to the store: `Open` has a fixed type and `cmd/ot` logs through `a.log`, not the default. The line is written through `slog.Default()` as the `setup` package does, which reaches stderr in full. Making it pass the profile handler needs `slog.SetDefault(a.log)` in `cmd/ot/main.go` (ot module) and the key `instrument_id` added to `logFields` in `internal/profile/profile.go` (repo module, with its test); both are small and separate, and neither changes what a reader of the line sees today.
2. An exported option (`OpenWithLogger`) or a changed `OpenInstrumentFunc`: an interface change for one caller, and every opener would follow; the unexported field gives the tests their seam.
3. A counter, a metric or a status row for rolled-back inserts: the advisor asked for visibility only; the line is countable with grep. A counter in the runner status is the freshness design's business (`option-page-live.md` Left out 4).
4. Logging the other failures of `ResolveInstrument` (they return an error to the job, which logs it) and the success path: not silent today, so not changed.
5. A retry or a backfill of the missing terms row: `option-page-live.md` Left out 1.

## Adversarial

pending
