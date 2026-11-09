# Position marks: one stored mark per open position of each account capture, at the mid, read by the builders

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 1

Source: owner 2026-11-08/09: options trading first and done completely; every position is marked at the mid of bid and ask; everything is per account, accounts are never pooled; only the main IBKR account must work; the web renders data and computes nothing; own glue code sits behind an interface. `docs/design/alignment-v25.md` (rows marks, positions, quote) and the open item of `docs/design/account-data.md` Decision 1 ("position prices ... are not collected").

Read on origin/main at 16588f8: `AGENTS.md`, `docs/design/modules.md`, `account-data.md`, `option-positions.md`, `option-ledger.md`, `internal/store/api.go` (`Position`, `AccountReader`, `QuoteSnapshotReader`, `OptionEodReader`), `internal/broker/api.go` (`Broker.Snapshot`, `Quote`, `Contract`), `internal/jobs/optioneod/held.go`, `contracts/datasets/account.position.r1.json`, `migrations/0019_account.sql`, `internal/store/setup/setup.go`. Read from open pull requests: the "Left out" section of PR 775 (builders-wave) and `docs/design/modules.md` of PR 768 (market-bars).

On main: `account.position` and `store.AccountReader.Positions` (account-data stage 1, reader and migration), `broker.Broker.Snapshot` with `broker.Contract.ConID`, `quote_snapshot` and `option_eod` with their writers and readers, `optioneod.NewWithHeld` (open short calls only).
Not on main yet: market-bars (PR 768): the general import rule of `docs/design/modules.md` that lets the root component and the sub-packages of any row depending on the row of a declarations-only package import it (jobs and pages import `internal/store/mark` by it); this design does not restate that rule and waits for it. The account capture job (`internal/jobs/accountcapture`, issues of account-data): entry 5 reads its slot from its manifest. builders-wave (PR 775) and option-positions consume this design and are not needed by it.
Needs the owner: nothing. No `contracts/proto/**` file and no `pyside/src/optdesk/ibkr/transport.py` edit is needed by the chosen source (Decision 1); the alternative of Decision 1 b) needs both and is not designed.

## Goal

For the main account (and any other alias whose gateway is up), every position of a capture, stock or option, US or JP, has one stored mark: a price, the basis it comes from (mid, last, close or none), the bid and ask it was taken from, and the currency, written right after the capture and read back by `(alias, session day)`. The household, report and option builders then compute value, unrealized P&L and concentration from the position quantity and this mark, and a position with no mark is a named gap (`missing_quote`), never a guess.

## Decisions

1. Source (window default, owner may change): the mark job asks the existing `broker.Broker.Snapshot` for each held contract, by the `ConID` the position already carries. Compared: (a) this; (b) IBKR portfolio `marketPrice` delivered with `account.position` by pyside; (c) `quote_snapshot` plus an instrument resolver; (d) `option_eod` mids. (a) needs no proto and no owner-only path, asks the gateway of the same account the position came from, and reaches every held contract (stocks, long and short options, puts and calls, JP). (b) is one request fewer but needs a new field in `contracts/proto/ibkr/v1/account.proto` and in `transport.py` (owner-merge-only), and IBKR's portfolio price is the last or model price, not the mid the owner asked for. (c) is keyed by instrument id and covers only the instruments a manifest lists, so each held contract would need a master row and a manifest line before it is marked; the resolver is not needed here because a mark is keyed by `(capture_id, con_id)`. (d) covers only the contracts the option jobs select plus the open short calls (`held.go`), so long options and short puts are missing, and it has no stocks. `quote_snapshot` and `option_eod` stay as they are and keep their other readers.
2. The mark is the mid: `(bid + ask) / 2` when both are above zero and ask is not below bid, basis `mid`. When there is no two-sided quote the job falls back to the last price (basis `last`), then to the close (basis `close`); with none of them the price is empty and the basis is `none` (window default, owner may change: fall back, flagged, or write no price). The builder shows the basis next to the number.
3. One row per position per capture, written also when there is no price: the row says `none` and why (`status`, the broker status in words), so "not marked" differs from "the job did not run". A row is final (insert-only, `ON CONFLICT DO NOTHING`); a retry of the same run snapshots only the positions that have no row yet, and the next capture marks again.
4. Per account and insert-only: a new table `account.position_mark` beside `account.position`, key `(capture_id, con_id)`, every row carries the alias; nothing is updated and nothing is pooled. `account.position` is not changed (account-data stands).
5. Timing (window default, owner may change): the job runs after each account capture, ten minutes after its slot, so the mark has the capture's session day and no position is marked at another time than it was counted. A JP position marked after the US close is marked at the last or close basis; the job does not run a second JP slot. Without a capture for the day the run ends `failed` with status `no_capture` and the scheduler's retry applies.
6. Main account first: the job takes the aliases of the account job; a gateway that is down fails that alias only, the others are marked, and the run is `failed` only when the main alias has no marks (same rule as account-data Decision 8; the alias of the main account is the deploy file's, not named here). Nothing goes to notify (account-data Decision 7 stands): a result carries a job id, a status and an alias only.
7. The mark is the price of one share or one contract, not multiplied: value is `quantity x mark x multiplier` and is computed by the builder in `compute`; the web computes nothing. No Greeks and no implied vol are stored (Left out).
8. The reader is `mark.Reader.Marks(alias, session day)`: the rows of the newest capture of that day that has marks, each with its `CaptureID`. A builder joins them to `store.AccountReader.Positions(alias, day)` by `(CaptureID, ConID)`; a position whose capture differs or that has no row is the gap `missing_quote`, and a mark older than 3 session days is `stale_input`, as in builders-wave.
9. Where the code lives and the library check. The row types and interfaces are a declarations-only package `internal/store/mark` (own `api.go`), its PostgreSQL implementation is `internal/store/markpg` (imported by `cmd/ot` only), and the job is `internal/jobs/positionmarks` behind `jobs.NewPositionMarksFunc`. The mid rule is own implementation, because it is one division and `math/big` is the standard library; it sits in the job package behind that constructor, so a library or a different source can replace the job without a reader change. No new direct dependency.

## Modules

No new row. Changed cells of `docs/design/modules.md`, edited in this pull request: the entry cell of the store row gains `internal/store/mark` (declarations only: `api.go` and its test; the general import rule is the one of market-bars, PR 768, which lists `internal/store/bar` in the same cell; whichever pull request lands second merges the two names); the list of implementation packages `cmd/ot` may import gains `internal/store/markpg` and `internal/jobs/positionmarks`. `arch.toml` is regenerated with `go run ./scripts/arch` in this pull request. The new paths belong to rows that exist, first matching glob: `internal/store/mark`, `internal/store/markpg`, `migrations/**` and `docs/contracts/store.md` to store; `internal/jobs/positionmarks` and `docs/contracts/jobs.md` to jobs; `contracts/datasets/account.position_mark.r1.json`, `docs/contracts/datasets/account.position_mark.md` and `internal/contracts/gen/records.go` to contracts; `cmd/ot/**` to ot. `.go-arch-lint.yml` is tree-derived and belongs to repo; entries 2, 3 and 5 each add one component to it (the licensed pair) and are ordered 2, then 3, then 5.

## Interface

contracts: dataset `account.position_mark` r1 (insert-only, `contracts/datasets/account.position_mark.r1.json`), columns: `capture_id` (string), `alias` (string), `marked_at` (date-time, UTC), `session_day` (date, of the capture), `con_id` (integer), `currency` (string), `price` (decimal text, empty when basis is none), `basis` (enum mid, last, close, none), `bid`, `ask`, `last`, `close` (decimal text, empty when absent), `market_data_type` (integer, 0 when unknown), `status` (enum ok, partial, no_data, no_subscription, timeout, error, disconnected). Primary key `(capture_id, con_id)`. Like `account.position` it has no `internal/contracts/api.go` type: the store holds plain row types.

store, `internal/store/mark/api.go` (declarations only, package `mark`):

- `type PositionMark struct { CaptureID, Alias string; MarkedAt time.Time; SessionDay string; ConID int64; Currency, Price, Basis, Bid, Ask, Last, Close string; MarketDataType int; Status string }` (decimals as text, `SessionDay` as `YYYY-MM-DD`); constants `BasisMid`, `BasisLast`, `BasisClose`, `BasisNone`.
- `type Writer interface { WriteMarks(ctx context.Context, rows []PositionMark) error }`: one transaction, a row whose key exists is left as it is.
- `type Reader interface { Marks(ctx context.Context, alias, day string) ([]PositionMark, error) }`: the newest capture of the day with marks, sorted by `ConID`, an empty slice when none.
- `type Store interface { Writer; Reader; Close() error }` and `type OpenFunc func(ctx context.Context, dsn string) (Store, error)` (implemented by `internal/store/markpg.Open`).
- `internal/store/api.go` gains `DSNEnvPositionMark = "OT_PG_DSN_POSMARK"` (role `ot_w_posmark`, INSERT and SELECT on `account.position_mark`; pages read the view `rd.account_position_mark_r1` with `ot_ro_pages`).

jobs, `internal/jobs/api.go`: `type MarkTarget struct { Alias string; Broker broker.Broker }` and `type NewPositionMarksFunc func(targets []MarkTarget, acct store.AccountReader, w mark.Writer, clk clock.Clock) (contracts.Job, error)`. The job reads the newest capture of the day per alias from `acct`, maps each `store.Position` to a `broker.Contract` (kind `KindOptionEod` for options, `KindSnapshot` otherwise), calls `Snapshot` once per alias, applies Decision 2 and writes the rows.

## Issues

Every test calls `t.Parallel()`, writes files only under a temporary directory, uses a fake clock, never sleeps, opens no port, and touches no live broker or database (a test that needs PostgreSQL carries the tag `pg`). Acceptance is checked offline by reading, grep and git; test runs are under "Checked by CI".

### 1 position-marks-dataset (interface)

Module: contracts. Size: S.
Files: `contracts/datasets/account.position_mark.r1.json`, `contracts/generate.sh`, `internal/contracts/gen/records.go`, `docs/contracts/datasets/account.position_mark.md`, `contracts/fixtures/account.position_mark.r1.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/schema_test.go`
Design: docs/design/position-marks.md#Interface
Order: first.
Goal: the dataset schema of `## Interface`, its generated record and document, one fixture of basis `mid` and one of basis `none`, manifest hashes.
Acceptance:
1. `grep -c "account.position_mark" contracts/generate.sh` is at least 2 and `grep -c "account.position_mark" internal/contracts/gen/records.go` is at least 1.
2. `contracts/datasets/account.position_mark.r1.json` names the columns `capture_id`, `con_id`, `basis`, `price` and `status`, and `grep -c "ibkr_conid\|account_id" contracts/datasets/account.position_mark.r1.json` is 0.
3. `grep -c "\"basis\": \"none\"" contracts/fixtures/account.position_mark.r1.json` is at least 1 and `grep -c "\"basis\": \"mid\"" contracts/fixtures/account.position_mark.r1.json` is at least 1.
4. `grep -c "account.position_mark" contracts/fixtures/MANIFEST.synthetic-v1.json` is at least 1, and `git diff --name-only BASE -- contracts/proto` is empty (`BASE` is `git merge-base HEAD origin/main`).

### 2 position-marks-store-api (interface)

Module: store, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: S.
Files: `internal/store/mark/api.go`, `internal/store/mark/api_test.go`, `internal/store/api.go`, `internal/store/api_test.go`, `docs/contracts/store.md`, `.go-arch-lint.yml`
Design: docs/design/position-marks.md#Interface
Order: after 1; before 3 and 5 (each of 3 and 5 edits `.go-arch-lint.yml`, strict order 2, 3, 5).
Goal: the declarations of `## Interface` in the new package, `DSNEnvPositionMark` in the store interface, the contract test from both sides (a fake writer and reader in the test), the component of `store/mark` in `.go-arch-lint.yml`.
Acceptance:
1. `internal/store/mark/api.go` holds no `func ` with a body: `grep -c "^func " internal/store/mark/api.go` is 0, and it declares `Writer`, `Reader`, `Store`, `OpenFunc` and `PositionMark`.
2. `grep -c "DSNEnvPositionMark" internal/store/api.go` is 1 and the change in `internal/store/api.go` only adds lines: `git diff --numstat BASE -- internal/store/api.go` shows 0 deleted lines.
3. `grep -c "store/mark" .go-arch-lint.yml` is at least 1, and `internal/store/mark/api.go` imports only the standard library (`grep -c "optdesk/internal" internal/store/mark/api.go` is 0).
4. `docs/contracts/store.md` names `internal/store/mark`, `Marks`, `WriteMarks` and `OT_PG_DSN_POSMARK`.

### 3 position-marks-store-pg

Module: store, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `migrations/0021_position_mark.sql`, `internal/store/markpg/store.go`, `internal/store/markpg/store_test.go`, `internal/store/markpg/store_pg_test.go`, `internal/store/setup/setup.go`, `internal/store/setup/setup_test.go`, `docs/contracts/store.md`, `.go-arch-lint.yml`
Design: docs/design/position-marks.md#Decisions
Order: after 2 (shares `docs/contracts/store.md` and `.go-arch-lint.yml`); before 5. If another migration takes the number 0021 first, this entry renumbers to the next free one before it merges.
Goal: the table `account.position_mark`, the view `rd.account_position_mark_r1`, the role `ot_w_posmark` made by setup, and `markpg.Open` implementing `mark.Store`.
Acceptance:
1. `migrations/0021_position_mark.sql` creates `account.position_mark` with `PRIMARY KEY (capture_id, con_id)` and a CHECK on `basis` of the four values, and creates the view `rd.account_position_mark_r1`.
2. The migration grants `INSERT, SELECT` on the table to `ot_w_posmark` and `SELECT` on the view to `ot_ro_pages`, and `grep -c "UPDATE\|DELETE\|TRUNCATE" migrations/0021_position_mark.sql` counts no GRANT of them (read the grants).
3. `internal/store/markpg/store.go` uses `ON CONFLICT DO NOTHING` and `grep -c "optdesk/internal/store/mark\"" internal/store/markpg/store.go` is 0 (its test asserts the interfaces, as `markpg` is imported by `cmd/ot` only).
4. `grep -c "roleMark\|ot_w_posmark" internal/store/setup/setup.go` is at least 2 and `DSNEnvPositionMark` appears in the role table of `internal/store/setup/setup.go`.
5. `internal/store/markpg/store_pg_test.go` carries the build tag `pg` on its first line.

### 4 position-marks-job-api (interface)

Module: jobs. Size: S.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `docs/contracts/jobs.md`
Design: docs/design/position-marks.md#Interface
Order: after 2 (imports `internal/store/mark`); before 5.
Goal: `MarkTarget` and `NewPositionMarksFunc` declared, the contract test with a fake constructor.
Acceptance:
1. `grep -c "NewPositionMarksFunc\|MarkTarget" internal/jobs/api.go` is at least 2.
2. `internal/jobs/api.go` imports `internal/store/mark` and not `internal/store/markpg` (`grep -c "store/markpg" internal/jobs/api.go` is 0).
3. `git diff --numstat BASE -- internal/jobs/api.go` shows 0 deleted lines.
4. `docs/contracts/jobs.md` names `position_marks` and `NewPositionMarksFunc`.

### 5 position-marks-job

Module: jobs, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: L.
Files: `internal/jobs/positionmarks/doc.go`, `internal/jobs/positionmarks/job.toml`, `internal/jobs/positionmarks/manifest.go`, `internal/jobs/positionmarks/plan.go`, `internal/jobs/positionmarks/mark.go`, `internal/jobs/positionmarks/run.go`, `internal/jobs/positionmarks/plan_test.go`, `internal/jobs/positionmarks/mark_test.go`, `internal/jobs/positionmarks/run_test.go`, `.go-arch-lint.yml`
Design: docs/design/position-marks.md#Decisions
Order: after 2, 3 and 4 (3 and 2 share `.go-arch-lint.yml`: strict order 2, 3, 5); not on main yet: the account capture job, whose slot `job.toml` follows; market-bars (the general import rule).
Goal: the job of Decisions 1 to 6: plan (one `broker.Contract` per position, kind by security type), `MarkOf` (Decision 2), run (per alias, retry fills missing rows only, main-alias failure rule), manifest with the slot ten minutes after the account job.
Acceptance:
1. `internal/jobs/positionmarks/mark.go` holds the rule: it returns the basis `mid` only when bid and ask are both above zero and `grep -c "BasisLast\|BasisClose\|BasisNone" internal/jobs/positionmarks/mark.go` is at least 3.
2. `plan.go` builds the contract from `ConID` and uses `broker.KindOptionEod` for `option` positions and `broker.KindSnapshot` otherwise (`grep -c "KindOptionEod\|KindSnapshot" internal/jobs/positionmarks/plan.go` is at least 2).
3. `grep -c "no_capture" internal/jobs/positionmarks/run.go` is at least 1, and the run writes a row with basis `none` for a quote whose status is not ok (read `run.go`).
4. The package imports `internal/store/mark` and neither `internal/store/markpg` nor `internal/notify` (`grep -rc "markpg\|internal/notify" internal/jobs/positionmarks` totals 0), and `grep -c "positionmarks" .go-arch-lint.yml` is at least 1.
5. `job.toml` has `id = "position_marks"` and a zone and calendar like the account job's; no file of the package is over 600 lines.

### 6 position-marks-wire

Module: ot. Size: S.
Files: `cmd/ot/positionmarks.go`, `cmd/ot/positionmarks_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `docs/contracts/ot.md`
Design: docs/design/position-marks.md#Modules
Order: after 3 and 5.
Goal: `ot run-once position_marks` and the scheduler build the job with `markpg.Open` on `store.DSNEnvPositionMark`, one `MarkTarget` per gateway socket of the account job, and the same `markpg` store is opened read-only for pages by the site wiring that later designs add.
Acceptance:
1. `grep -c "positionmarks\|markpg" cmd/ot/positionmarks.go` is at least 2 and `cmd/ot/wire.go` registers `position_marks`.
2. `cmd/ot/positionmarks.go` imports the packages `internal/store/markpg` and `internal/jobs/positionmarks`, both in the list of `docs/design/modules.md` for `ot`.
3. `grep -c "OT_PG_DSN_POSMARK\|DSNEnvPositionMark" cmd/ot/positionmarks.go` is at least 1.
4. `docs/contracts/ot.md` names `position_marks`.

## Checked by CI

`go test ./internal/store/mark/... ./internal/store/markpg/... ./internal/store/setup/... ./internal/jobs/... ./internal/contracts/... ./cmd/ot/...`, `go run ./scripts/arch` leaves `arch.toml` unchanged, the `pg` tests of entries 3 on GitHub CI, `golangci-lint`, and the `test` check.

## Left out

- Greeks and implied vol of held contracts outside the `option_eod` selection: the mark row stores prices only (Decision 7); the Greeks of a held leg still come from `obs.option_eod` where a row exists, as in `option-positions.md`. A design that stores the model values of every held contract is its own later step.
- A day change for movers: the previous-day mark exists only from the first run of this job; history before it is not back-filled (no source) and the builder writes `stale_input` or `missing_quote` for it.
- Intraday marks: one mark per capture, as often as the account job runs; no stream and no 15 s refresh.
- A second JP slot and a JP-specific close mark: JP positions are marked at the capture's time with the last or close basis (Decision 5, window default).
- The IBKR portfolio `marketPrice` (Decision 1 b): needs the owner-only `account.proto` and `transport.py`; named so the owner can choose it.
- Marks for names the owner does not hold: the `market_watch.companies` rows, `global_assets` proxies and sector rows, and the held-company values of PR 775 that are about watch lists, not positions, need `quote_snapshot` with a symbol-to-instrument resolver and the bars of market-bars; they remain a separate design.
- Non-main accounts that fail, and Flex-only accounts (spouse, daughter): no gateway, no mark (as in account-data Decision 8).
- The builder-side join (`(CaptureID, ConID)`, Decision 8) is written by the designs that consume it (builders-wave PR 775, option-positions, option-tracking), not here.
