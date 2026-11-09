# Position marks: one stored mark per open position of each account capture, at the mid, read by the builders

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 1

Source: owner 2026-11-08/09: options trading first and done completely; every position is marked at the mid of bid and ask; everything is per account, accounts are never pooled; only the main IBKR account must work; the web renders data and computes nothing; own glue code sits behind an interface. `docs/design/alignment-v25.md` (rows marks, positions, quote) and the open item of `docs/design/account-data.md` Decision 1 ("position prices ... are not collected").

Read on origin/main at 16588f8: `AGENTS.md`, `docs/design/modules.md`, `account-data.md`, `option-positions.md`, `option-ledger.md`, `internal/store/api.go` (`Position`, `AccountReader`, `QuoteSnapshotReader`, `OptionEodReader`), `internal/broker/api.go` (`Broker.Snapshot`, `Quote`, `Contract`), `internal/jobs/optioneod/held.go`, `contracts/datasets/account.position.r1.json`, `migrations/0019_account.sql`, `internal/store/setup/setup.go`. Read from open pull requests: the "Left out" section of PR 775 (builders-wave) and `docs/design/modules.md` of PR 768 (market-bars).

On main: `account.position` and `store.AccountReader.Positions` (account-data stage 1, reader and migration), `broker.Broker.Snapshot` with `broker.Contract.ConID`, `quote_snapshot` and `option_eod` with their writers and readers, `optioneod.NewWithHeld` (open short calls only).
On main since #768 (market-bars): the declarations-only package import rule of `docs/design/modules.md` (jobs and pages import `internal/store/mark`, `store/markpg` imports it as a package of its own row); this design copies nothing of it and only adds `internal/store/mark` and `internal/store/markpg` to its "today" lists. Not on main yet: the code of that rule in `scripts/arch` (entry `bar-store-api` of market-bars); entry 2 below carries it too if it lands first. The account capture job (`internal/jobs/accountcapture`, issues of account-data): entry 5 reads its slot from its manifest. builders-wave (PR 775) and option-positions consume this design and are not needed by it.
Needs the owner: nothing. No `contracts/proto/**` file and no `pyside/src/optdesk/ibkr/transport.py` edit is needed by the chosen source (Decision 1); the alternative of Decision 1 b) needs both and is not designed.

## Goal

For the main account (and any other alias whose gateway is up), every position of a capture, stock or option, US or JP, has one stored mark: a price, the basis it comes from (mid, last, close or none), the bid and ask it was taken from, and the currency, written right after the capture and read back by `(alias, session day)`. The household, report and option builders then compute value, unrealized P&L and concentration from the position quantity and this mark, and a position with no mark is a named gap (`missing_quote`), never a guess.

## Decisions

1. Source (window default, owner may change): the mark job asks the existing `broker.Broker.Snapshot` for each held contract, by the `ConID` the position already carries, with kind `KindSnapshot` (bid, ask, last, close) for stocks, indexes and options alike (`KindOptionEod` waits for model values, which a mark does not need). Compared: (a) this; (b) IBKR portfolio `marketPrice` delivered with `account.position` by pyside; (c) `quote_snapshot` plus an instrument resolver; (d) `option_eod` mids. (a) needs no proto and no owner-only path and aims at every held contract (stocks, indexes, long and short options, puts and calls, JP); which contracts the subscriptions of the market-data gateway really serve is not proven and is the probe of Decision 10. (b) is one request fewer but needs a new field in `contracts/proto/ibkr/v1/account.proto` and in `transport.py` (owner-merge-only), and IBKR's portfolio price is the last or model price, not the mid the owner asked for. (c) is keyed by instrument id and covers only the instruments a manifest lists, so each held contract would need a master row and a manifest line before it is marked; the resolver is not needed here because a mark is keyed by `(capture_id, con_id)`. (d) covers only the contracts the option jobs select plus the open short calls (`held.go`), so long options and short puts are missing, and it has no stocks. `quote_snapshot` and `option_eod` stay as they are and keep their other readers.
2. The mark is the mid: `(bid + ask) / 2` when both are above zero and ask is not below bid, basis `mid`. When there is no two-sided quote the job falls back to the last price (basis `last`), then to the close (basis `close`); with none of them the price is empty and the basis is `none` (window default, owner may change: fall back, flagged, or write no price). The builder shows the basis next to the number.
3. One row per position per capture, written also when there is no price: the row says `none` and why (`status`, the broker status in words), so "not marked" differs from "the job did not run". Correction path, no update: the key is `(capture_id, con_id, attempt)`, `attempt` starting at 1; a retry of the run, or a later run for the same capture, snapshots only the positions whose newest row has basis `none` and inserts attempt + 1 for them; the reader returns the highest attempt per position, so a later mark supersedes by insert order. A row with a price is never superseded, and the next capture marks again.
4. Per account and insert-only: a new table `account.position_mark` beside `account.position`, key `(capture_id, con_id, attempt)` (Decision 3), every row carries the alias; nothing is updated and nothing is pooled. `account.position` is not changed (account-data stands).
5. Timing (window default, owner may change): the job runs after each account capture, ten minutes after its slot, so the mark has the capture's session day and no position is marked at another time than it was counted. A JP position marked after the US close is marked at the last or close basis; the job does not run a second JP slot. Without a capture for the day the run ends `failed` with status `no_capture` and the scheduler's retry applies.
6. Which gateway serves the quotes, a rule and not a choice per alias: `Broker.Snapshot` is always called on the market-data gateway (the entry whose `collect` has `market_data`; in the example deploy file the other gateway collects the account), whatever account holds the position. The alias of a position only says whose positions are marked and is the `alias` of the stored row. Only the main account must work: the other alias may be logged out, its positions are then not marked and that never fails the run of the main one; the run is `failed` only when the main alias has no marks, or the market-data gateway is down (the alias of the main account is the deploy file's, not named here). Nothing goes to notify (account-data Decision 7 stands): a result carries a job id, a status and an alias only.
7. The mark is the price of one share or one contract, not multiplied: value is `quantity x mark x multiplier` and is computed by the builder in `compute`; the web computes nothing. No Greeks and no implied vol are stored (Left out).
8. The reader is `mark.Reader.Marks(alias, session day)`: the rows of the newest capture of that day that has marks, each with its `CaptureID`. A builder joins them to `store.AccountReader.Positions(alias, day)` by `(CaptureID, ConID)`; a position whose capture differs or that has no row is the gap `missing_quote`, and a mark older than 3 session days is `stale_input`, as in builders-wave.
9. Where the code lives and the library check. The row types and interfaces are a declarations-only package `internal/store/mark` (own `api.go`), its PostgreSQL implementation is `internal/store/markpg` (imported by `cmd/ot` only), and the job is `internal/jobs/positionmarks` behind `jobs.NewPositionMarksFunc`. The mid rule is own implementation, because it is one division and `math/big` is the standard library; it sits in the job package behind that constructor, so a library or a different source can replace the job without a reader change. No new direct dependency.
10. The plan step, explicit. Per position: `sec_type` `stock` becomes `STK`, `index` becomes `IND`, `option` becomes `OPT`; `Expiry` is stored `YYYY-MM-DD` (account.position) and the broker wants `YYYYMMDD`, so the dashes are removed; `Right` `put` or `call` becomes `P` or `C`; `Multiplier`, `Strike`, `Currency` and `ConID` are copied. Exchange: a US contract (currency USD) uses `SMART`. A JP contract (currency JPY) is first looked up with `Broker.ResolveContract` (ConID, sec type, currency) and the exchange of the `Resolved` contract (its primary exchange) is used; `store/instrument` is not used, because `ResolveInstrument` creates a row and reads are by id only, so the job would write the master to find an exchange. A contract that is not listed or does not route gets basis `none` with status `no_data` or the broker's status; it is never dropped. Batches: the job splits the contracts into batches below the gateway line limit (70, `DEFAULT_LINES` in pyside) and stops starting batches when a wall-clock budget set below `SnapshotTimeout` (60 s) is spent; positions not reached get no row, the run ends `failed` with status `partial`, and the retry marks them (Decision 3). Each row keeps the per-contract status, so `no_subscription` is visible. The first run against the real gateway is a window step (a probe, not a test): its answer, with synthetic ids, may later replace the synthetic fixture of entry 5; what it shows about US, JP and OPRA subscriptions is reported to the owner as a separate window step after entry 5, not a condition for closing it.
11. Why a separate job and not a call inside the account capture: the account capture then never depends on market data, a logged-out or unsubscribed market-data gateway cannot fail or delay the capture of positions, and the capture stays what account-data built. The cost is one more job, role and DSN, accepted.
12. JP and the owner's rule "mark at mid": the mid is stored wherever the quote is two-sided; a market that is closed at the capture gives last or close with the basis shown, which is not a mid, and that is accepted and named, not hidden.

## Modules

No new row. Changed cells of `docs/design/modules.md`, edited in this pull request: the entry cell of the store row gains `internal/store/mark` (declarations only: `api.go` and its test; the paragraph on declarations-only packages is main's (from #768) and gains `internal/store/mark` (and `internal/store/markpg` as its implementation) in its two "today" lists and in the `store/bar` sentence of the depends bullet; `internal/store/bar` is already in the cell on main); the list of implementation packages `cmd/ot` may import gains `internal/store/markpg` and `internal/jobs/positionmarks`. `arch.toml` is regenerated with `go run ./scripts/arch` in this pull request. The new paths belong to rows that exist, first matching glob: `internal/store/mark`, `internal/store/markpg`, `migrations/**` and `docs/contracts/store.md` to store; `internal/jobs/positionmarks` and `docs/contracts/jobs.md` to jobs; `contracts/datasets/account.position_mark.r1.json`, `docs/contracts/datasets/account.position_mark.md` and `internal/contracts/gen/records.go` to contracts; `cmd/ot/**` to ot. `.go-arch-lint.yml` is tree-derived and belongs to repo; entries 2, 3 and 5 each add one component to it (the licensed pair) and are ordered 2, then 3, then 5.

## Interface

contracts: dataset `account.position_mark` r1 (insert-only, `contracts/datasets/account.position_mark.r1.json`), columns: `capture_id` (string), `alias` (string), `marked_at` (date-time, UTC), `session_day` (date, of the capture), `con_id` (integer), `attempt` (integer, from 1), `currency` (string), `price` (decimal text, empty when basis is none), `basis` (enum mid, last, close, none), `bid`, `ask`, `last`, `close` (decimal text, empty when absent), `market_data_type` (integer, 0 when unknown), `status` (enum ok, partial, no_data, no_subscription, timeout, error, disconnected). Primary key `(capture_id, con_id, attempt)`. Like `account.position` it has no `internal/contracts/api.go` type: the store holds plain row types.

store, `internal/store/mark/api.go` (declarations only, package `mark`):

- `type PositionMark struct { CaptureID, Alias string; MarkedAt time.Time; SessionDay string; ConID int64; Attempt int; Currency, Price, Basis, Bid, Ask, Last, Close string; MarketDataType int; Status string }` (decimals as text, `SessionDay` as `YYYY-MM-DD`); constants `BasisMid`, `BasisLast`, `BasisClose`, `BasisNone`.
- `type Writer interface { WriteMarks(ctx context.Context, rows []PositionMark) error }`: one transaction, a row whose key exists is left as it is (a correction is a row with a higher `Attempt`).
- `type Reader interface { Marks(ctx context.Context, alias, day string) ([]PositionMark, error) }`: the newest capture of the day with marks, the highest `Attempt` of each `ConID`, sorted by `ConID`, an empty slice when none.
- `type Store interface { Writer; Reader; Close() error }` and `type OpenFunc func(ctx context.Context, dsn string) (Store, error)` (implemented by `internal/store/markpg.Open`).
- `internal/store/api.go` gains `DSNEnvPositionMark = "OT_PG_DSN_POSMARK"` (role `ot_w_posmark`, INSERT and SELECT on `account.position_mark`; pages read the view `rd.account_position_mark_r1` with `ot_ro_pages`).

jobs, `internal/jobs/api.go`: `type NewPositionMarksFunc func(market broker.Broker, aliases []string, acct store.AccountReader, w mark.Writer, clk clock.Clock) (contracts.Job, error)`: `market` is the one market-data gateway (Decision 6), `aliases` the accounts whose positions are marked, the main one first. The job reads the newest capture of the day per alias from `acct`, maps each position by Decision 10, calls `Snapshot` in batches with `KindSnapshot`, applies Decision 2 and writes the rows.

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

Module: store, plus repo (the licensed pair: `scripts/arch`, `arch.toml` and `.go-arch-lint.yml`). Size: S.
Files: `internal/store/mark/api.go`, `internal/store/mark/api_test.go`, `internal/store/api.go`, `internal/store/api_test.go`, `docs/contracts/store.md`, `scripts/arch/main.go`, `scripts/arch/main_test.go`, `arch.toml`, `.go-arch-lint.yml`
Design: docs/design/position-marks.md#Interface
Order: after 1; no longer waits for market-bars: it adds `internal/store/mark` to the `declarationsOnly` list of `scripts/arch/main.go`, and creates that list and its two derivation rules (a root component of a dependent row, and a sub-package of the same row, may import a listed package) when entry `bar-store-api` of market-bars has not landed first; whichever lands second merges the list; before 3 and 5 (each of 3 and 5 edits `.go-arch-lint.yml`, strict order 2, 3, 5).
Goal: the declarations of `## Interface` in the new package, `DSNEnvPositionMark` in the store interface, the contract test from both sides (a fake writer and reader in the test), the component of `store/mark` in `.go-arch-lint.yml`.
Acceptance:
1. `internal/store/mark/api.go` holds no `func ` with a body: `grep -c "^func " internal/store/mark/api.go` is 0, and it declares `Writer`, `Reader`, `Store`, `OpenFunc` and `PositionMark`.
2. `grep -c "DSNEnvPositionMark" internal/store/api.go` is 1 and the change in `internal/store/api.go` only adds lines: `git diff --numstat BASE -- internal/store/api.go` shows 0 deleted lines.
3. `grep -c "store/mark" .go-arch-lint.yml` is at least 1, and `internal/store/mark/api.go` imports only the standard library (`grep -c "optdesk/internal" internal/store/mark/api.go` is 0).
4. `docs/contracts/store.md` names `internal/store/mark`, `Marks`, `WriteMarks` and `OT_PG_DSN_POSMARK`.
5. `grep -c "declarationsOnly" scripts/arch/main.go` is at least 1 and names `internal/store/mark`; `scripts/arch/main_test.go` has a test where the root component `jobs` and the sub-package `store/markpg` each import `store/mark` without a finding (`grep -c "store/mark" scripts/arch/main_test.go` is at least 2).

### 3 position-marks-store-pg

Module: store, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `migrations/NNNN_position_mark.sql`, `internal/store/markpg/store.go`, `internal/store/markpg/store_test.go`, `internal/store/markpg/store_pg_test.go`, `internal/store/setup/setup.go`, `internal/store/setup/setup_test.go`, `docs/contracts/store.md`, `.go-arch-lint.yml`
Design: docs/design/position-marks.md#Decisions
Order: after 2 (shares `docs/contracts/store.md` and `.go-arch-lint.yml`); before 5. `NNNN` is the next free migration number at cut time and is renumbered before merge if another migration takes it (the file name is fixed when the issue is cut).
Goal: the table `account.position_mark`, the view `rd.account_position_mark_r1`, the role `ot_w_posmark` made by setup, and `markpg.Open` implementing `mark.Store`.
Acceptance:
1. `migrations/NNNN_position_mark.sql` creates `account.position_mark` with `PRIMARY KEY (capture_id, con_id, attempt)` and a CHECK on `basis` of the four values, and creates the view `rd.account_position_mark_r1`.
2. The migration grants `INSERT, SELECT` on the table to `ot_w_posmark` and `SELECT` on the view to `ot_ro_pages`, and `grep -i "^GRANT" migrations/NNNN_position_mark.sql | grep -c "UPDATE\|DELETE\|TRUNCATE"` is 0.
3. `internal/store/markpg/store.go` uses `ON CONFLICT DO NOTHING` and `grep -c "optdesk/internal/store/mark\"" internal/store/markpg/store.go` is at least 1: `markpg` names `mark.PositionMark` and returns `mark.Store`, which the declarations-only rule of modules.md allows for a sub-package of the same row (`markpg` is imported by `cmd/ot` only).
4. `grep -c "roleMark\|ot_w_posmark" internal/store/setup/setup.go` is at least 2 and `DSNEnvPositionMark` appears in the role table of `internal/store/setup/setup.go`.
5. `internal/store/markpg/store_pg_test.go` carries the build tag `pg` on its first line.

### 4 position-marks-job-api (interface)

Module: jobs. Size: S.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `docs/contracts/jobs.md`
Design: docs/design/position-marks.md#Interface
Order: after 2 (imports `internal/store/mark`); before 5.
Goal: `NewPositionMarksFunc` declared, the contract test with a fake constructor.
Acceptance:
1. `grep -c "NewPositionMarksFunc" internal/jobs/api.go` is at least 1 and `grep -c "MarkTarget" internal/jobs/api.go` is 0 (the market-data broker is a plain argument).
2. `internal/jobs/api.go` imports `internal/store/mark` and not `internal/store/markpg` (`grep -c "store/markpg" internal/jobs/api.go` is 0).
3. `git diff --numstat BASE -- internal/jobs/api.go` shows 0 deleted lines.
4. `docs/contracts/jobs.md` names `position_marks` and `NewPositionMarksFunc`.

### 5 position-marks-job

Module: jobs, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: L.
Files: `internal/jobs/positionmarks/doc.go`, `internal/jobs/positionmarks/job.toml`, `internal/jobs/positionmarks/manifest.go`, `internal/jobs/positionmarks/plan.go`, `internal/jobs/positionmarks/mark.go`, `internal/jobs/positionmarks/run.go`, `internal/jobs/positionmarks/plan_test.go`, `internal/jobs/positionmarks/mark_test.go`, `internal/jobs/positionmarks/run_test.go`, `internal/jobs/positionmarks/testdata/probe_snapshot.json`, `.go-arch-lint.yml`
Design: docs/design/position-marks.md#Decisions
Order: after 2, 3 and 4 (2 and 3 share `.go-arch-lint.yml`: strict order 2, 3, 5). Waits, not on main yet: the account capture job (its manifest gives the slot this one is an offset of; until it lands the offset is written against the name `account_capture`). The import rule is on main (#768) and its `scripts/arch` code comes with entry 2 or with `bar-store-api` of market-bars, whichever lands first. The probe of Decision 10 is a separate window step, outside this entry's acceptance and closing condition; the fixture here is synthetic.
Goal: the job of Decisions 1 to 6: plan (one `broker.Contract` per position, kind by security type), `MarkOf` (Decision 2), run (per alias, retry fills missing rows only, main-alias failure rule), manifest with the slot ten minutes after the account job.
Acceptance:
1. `internal/jobs/positionmarks/mark.go` holds the rule: it returns the basis `mid` only when bid and ask are both above zero and ask is not below bid, `mark_test.go` has a test named for the crossed quote (`grep -c "Crossed" internal/jobs/positionmarks/mark_test.go` is at least 1), and `grep -c "BasisLast\|BasisClose\|BasisNone" internal/jobs/positionmarks/mark.go` is at least 3.
2. `plan.go` builds the contract from `ConID` per Decision 10 (`STK`, `IND`, `OPT`, dashes removed from the expiry, `P` and `C`, `SMART` for USD, `ResolveContract` for JPY) and uses `broker.KindSnapshot` only (`grep -c "KindOptionEod" internal/jobs/positionmarks/plan.go` is 0; `grep -c "ResolveContract\|SMART\|IND" internal/jobs/positionmarks/plan.go` is at least 3).
3. `grep -c "no_capture\|partial" internal/jobs/positionmarks/run.go` is at least 2, `run.go` batches and keeps a wall-clock budget (`grep -c "batch\|budget" internal/jobs/positionmarks/run.go` is at least 2), a retry inserts `attempt` + 1 only for rows of basis `none`, and the run writes a row with basis `none` for a quote whose status is not ok (read `run.go`).
4. The package imports `internal/store/mark` and neither `internal/store/markpg` nor `internal/notify` (`grep -rc "markpg\|internal/notify" internal/jobs/positionmarks` totals 0), and `grep -c "positionmarks" .go-arch-lint.yml` is at least 1.
5. `job.toml` has `id = "position_marks"` and a zone and calendar like the account job's, its slot is stated as an offset after the account job's slot and not as a fixed time, and `testdata/probe_snapshot.json` exists, is synthetic (a hand-made answer in the shape of the gateway's, with no account id) and says so in a `"synthetic": true` field; no file of the package is over 600 lines.

### 6 position-marks-wire

Module: ot. Size: S.
Files: `cmd/ot/positionmarks.go`, `cmd/ot/positionmarks_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `docs/contracts/ot.md`
Design: docs/design/position-marks.md#Modules
Order: after 3 and 5.
Goal: `ot run-once position_marks` and the scheduler build the job with `markpg.Open` on `store.DSNEnvPositionMark`, the one market-data gateway socket and the aliases of the account job, and the same `markpg` store is opened read-only for pages by the site wiring that later designs add.
Acceptance:
1. `grep -c "positionmarks\|markpg" cmd/ot/positionmarks.go` is at least 2 and `cmd/ot/wire.go` registers `position_marks`.
2. `cmd/ot/positionmarks.go` imports the packages `internal/store/markpg` and `internal/jobs/positionmarks`, both in the list of `docs/design/modules.md` for `ot`.
3. `grep -c "OT_PG_DSN_POSMARK\|DSNEnvPositionMark" cmd/ot/positionmarks.go` is at least 1.
4. `docs/contracts/ot.md` names `position_marks`.

## Checked by CI

`go test ./internal/store/mark/... ./internal/store/markpg/... ./internal/store/setup/... ./internal/jobs/... ./internal/contracts/... ./cmd/ot/...`, `go run ./scripts/arch --check` prints ok and `go run ./scripts/arch` leaves `arch.toml` unchanged, the `pg` tests of entries 3 on GitHub CI, `golangci-lint`, and the `test` check.

## Left out

- Greeks and implied vol of held contracts outside the `option_eod` selection: the mark row stores prices only (Decision 7); the Greeks of a held leg still come from `obs.option_eod` where a row exists, as in `option-positions.md`. A design that stores the model values of every held contract is its own later step.
- A day change for movers: the previous-day mark exists only from the first run of this job; history before it is not back-filled (no source) and the builder writes `stale_input` or `missing_quote` for it.
- Intraday marks: one mark per capture, as often as the account job runs; no stream and no 15 s refresh.
- A second JP slot and a JP-specific close mark: JP positions are marked at the capture's time with the last or close basis (Decision 5, window default).
- The IBKR portfolio `marketPrice` (Decision 1 b): needs the owner-only `account.proto` and `transport.py`; named so the owner can choose it.
- Marks for names the owner does not hold: the `market_watch.companies` rows, `global_assets` proxies and sector rows, and the held-company values of PR 775 that are about watch lists, not positions, need `quote_snapshot` with a symbol-to-instrument resolver and the bars of market-bars; they remain a separate design.
- Non-main accounts that fail, and Flex-only accounts (spouse, daughter): no gateway, no mark (as in account-data Decision 8).
- The builder-side join (`(CaptureID, ConID)`, Decision 8) is written by the designs that consume it (builders-wave PR 775, option-positions, option-tracking), not here.

## Adversarial

Answers to the read-only attack, kept in the window's scratch, items 1 to 6.

1. Gateway: adopted. Decision 6 says `Snapshot` is always served by the market-data gateway whatever account holds the position; the alias only names whose positions are marked; the main account alone must work; `NewPositionMarksFunc` takes the one `market` broker and no per-alias broker. JP at last or close is accepted and named (Decision 12).
2. Subscriptions, line limit, budget: adopted. Decision 10 batches below 70 lines, keeps a budget under the 60 s snapshot timeout, stores per-contract status, and makes the first real run a window probe outside the acceptance (entry 5 holds a synthetic fixture).
3. Simpler alternatives: not adopted. A price column on `account.position` or `marketPrice` needs owner-only proto and `transport.py` and is not the mid; a call inside the capture would make the capture depend on market data (Decision 11). The cost, one job, role and DSN, is accepted.
4. Lock-in, no repair: adopted in part. `attempt` in the key gives a correction by insert order, no update (Decision 3); the interface stays small (one constructor, one reader), and builders join by `CaptureID`.
5. False claims: adopted. Decision 10 states conversions (stock, index, option; dates without dashes; P and C), SMART for US, `ResolveContract` for JP (`store/instrument` cannot look up by con id without writing), basis none with a reason for a contract that does not route, and `KindSnapshot` for options.
6. Slips: adopted. The migration number is left to cut time (`NNNN`), the slot is an offset after the account capture manifest and a stated wait, the order 2, 3, 5 stays strict, and the shared cell of `modules.md` is named as a merge point with market-bars; the long `cmd/ot` list line cannot be split without changing the table, so it stays one line.
