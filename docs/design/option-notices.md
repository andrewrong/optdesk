# Option notices: a Telegram morning report without account data, and local option notices on a page

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 6
Revision note: revision 6 applies the advisor's ruling of 2026-11-09 that a second store interface file does not stand (AGENTS.md: one interface file per package): the gap history is a new module `gaphistory` with its own `api.go` and contract test (Decision 15, replaces the exception of revision 5; question A is gone); this pull request now also edits `docs/design/modules.md` (a new row before the store row, three depends cells, the layers sentence and table) and `arch.toml` follows from it; issues 3 and 4 are re-cut. Revision 5 answers the verdict at 19a5f12: issue 11 is again ordered after `option-candidates-page-view` (Order line, table row 11, the Modules paragraph and Decision 12 now say the same); the second store interface file is kept and justified in Decision 15 and put to the owner as question A (the window asks); the stale text of revision 2 is corrected; acceptance 3 of issue 11 counts the link by its path. Revision 4 renames the dropdown: the one created by `option-positions-nav` is labelled `期权持仓` (the existing `期权` item at `/options` stays), the link `提示` goes into the `期权持仓` menu and the bar stays at ten items (coordinator 2026-11-09). Revision 3 applies the owner's answers of 2026-11-09 ("a a a a a a"): stage 1 keeps pushing every gap once and the morning report adds the self-healed section (Decision 4); the thresholds stand (Decision 5); the morning message is sent on a quiet day (Decision 2); the link to the notices page goes into the one top-bar dropdown `期权持仓` that `option-positions-nav` (design PR 747, entry 12) creates, not a tenth top item (Decision 10, issue 11); the answered questions moved from Left out into Decisions and the Left out question list is gone. Revision 2 notes follow. Revision 2 answers the design verdict at 1163c88 (four points) and the window's defaults. (1) `GapHistoryReader` moves out of `internal/store/api.go` (799 lines) into the new interface file `internal/store/gap_history_api.go`, so `api.go` is not edited and stays at 799; issue 3 checks the line count; question F is gone. (2) The previous session day of the morning report is named: the newest `ExpectedDay` found in the rows of `Expect.Expectations` for the last 7 calendar days (Decision 2, issue 6). (3) Issue 11 keeps `after 12` and now states the reason (the bar item must not point at a missing route). (4) The Modules section names the other design whose issues 11 and 12 wait for. Window defaults: the issuer is the position symbol (Decision 13, was question C) and notices are recomputed at each build with no first-seen memory (Decision 14, was question D).

Source: plan v2.5 section 11 week 11 ("notify morning report") and section 1.2 and 1.6 (events go to a 08:00 KL morning report; the report is one message; Telegram never carries account-derived information, ADR 0010); owner decisions of 2026-11-08/09, binding: the margin cushion and the assignment cash are shown only on a local page or as a local notice, never on Telegram; no alert is repeated; only the main IBKR account must work and the second may be logged out without failing anything; candidates are gates plus annualized-yield order with no composite score; the front end renders data and computes nothing. The gap table of 2026-11-09 lists "Notices: 08:00 morning report, night rule, option notices" as NOT DESIGNED.
Read on origin/main at 35f3c88: `AGENTS.md`, `docs/design/modules.md`, `docs/design/notify.md` (Left out), `risk-alerts.md`, `option-candidates.md`, `option-candidates-page.md`, `option-ledger.md`, `account-data.md` (Decisions 7 to 10), `event-sources.md`, `docs/adr/0010-reports-and-delivery.md`, `internal/notify/api.go`, `internal/compute/api.go`, `internal/store/api.go` (`Gap`, `GapReader`, `SentStore`, `Candidate`, `Position`, `AccountReader`, `EarningsDateReader`), `migrations/0012_scheduler.sql` (view `rd.job_gap_r1` holds open and cleared rows), `cmd/ot/notify.go`, `cmd/ot/scheduler.go`.
Not on main yet (this design is written against their designs, not their code): the account capture job (`account-data.md`), the candidate job and page (`option-candidates.md`, `option-candidates-page.md`); see Decision 12.
Adversarial pass: `scratchpad/design/option-notices-adversarial.md` of the design session (local to the machine); the response is in `## Adversarial`.

## Goal

Two things, kept apart by construction. First, a Telegram morning report: one message a day from 08:00 KL that says what the machine did overnight (self-healed gaps, datasets that are late) and holds nothing derived from the account. Second, option notices for a seller: expiry near, an early-assignment or ex-dividend guard on a held short call, earnings inside a held contract, an assignment seen, a low margin cushion, and the cash an assignment would need by date. These go to a local page `/options/notices` and nowhere else.

## Decisions

1. The boundary. `notify` sends the morning report and reads only job and dataset ledgers (gaps, expectations, the sent marks). It imports no account, candidate, ledger or position type. The notices are built by a pure rule in `compute` and shown by a page in `pages`; no code path from the notice rule or the notices page reaches a `Sender`. Result: the owner's rule "cushion and assignment cash never go to Telegram" is true because the types are absent, and a test greps for it (issues 6 and 10).
2. The morning report. `MorningNotifier.NotifyMorning(ctx)` runs inside the scheduler process after the gap pass, as `NotifyGaps` does (`notify.md` Decision 4: no resident process). It sends at most one message per KL calendar day: not before `MorningFrom` (default `08:00`), not after `MorningUntil` (default `12:00`, a late scheduler start still sends in the morning, never in the evening), and the day is marked in `alerts.sent` under the rule id `morning_report` with the KL date as `day` (`store.SentStore`, no new table, role or migration). The message is sent even when nothing happened ("all quiet", owner 2026-11-09), because a missing 08:00 message then means the machine is down. The text is built from fixed words: a header with the KL date, "self-healed" (gaps whose `ClearedAt` is since the previous 08:00 and whose `ReportedAt` is unset, that is gaps never pushed: job or dataset id, reason, cleared time in KL), counts only for the rest (gaps cleared after they were pushed, gaps still open, datasets of the previous session day, the newest `ExpectedDay` among the expectation rows of the last 7 calendar days, with a non-empty expectation `Gap`), and one fixed line that details are on the local pages. At most 20 lines; more are cut with "and N more".
3. Self-healed gaps need history. `store.GapReader.OpenGaps` returns open gaps only, but the view `rd.job_gap_r1` keeps cleared rows (`migrations/0012_scheduler.sql`). A new interface `gaphistory.Reader` (`GapsSince(ctx, since time.Time) ([]gaphistory.Gap, error)`, the rows whose `FirstSeenAt` or `ClearedAt` is at or after `since`) is declared by the new module `gaphistory` (Decision 15) and implemented by the existing reader in `internal/store/ops/gaps.go` through the same view and the same role; `store.GapReader` is not edited, so no fake of it elsewhere stops compiling.
4. No repeated alert. The morning report never prints the text of a gap that `NotifyGaps` has already sent (a pushed gap, open or cleared, and a late dataset, appear as counts only, with no id or reason) and never prints a market alert (`NotifyAlerts` said it once, `alerts.sent`). Stage 1 keeps pushing a missed-job gap once, as approved in `notify.md` (owner 2026-11-09: keep it, the morning report adds the self-healed section); the morning report adds the "self-healed" section the plan asks for. A gap that cleared before it was ever reported is printed once, in the morning report, and its `reported_at` stays unset (the report does not call `MarkGapReported`, `notify.md` Decision 1 keeps one owner of that column in the gap pass).
5. The notices. A pure rule `internal/compute/optionnotice` behind `compute.OptionNotices` (`Evaluate(NoticeInput, NoticeParams) NoticeResult`; no I/O, no clock, exact arithmetic with `math/big.Rat`, as `optioncand` does). Codes: `expiry_today` and `expiry_near` (a short option position, expiry within `ExpiryDays` calendar days, default 5), `call_guard` (a stored `held_call_guard` candidate row of a held short call whose status is `blocked`, carrying its reason codes: the rule reads the stored verdict and does not recompute the guard), `earnings_inside` (a held short option whose issuer has a next earnings date on or before its expiry), `assignment_seen` (an option position of the previous captured session day that is gone on its expiry day while a stock position of that symbol rose: strike x multiplier x quantity, the figure of `account-data.md` Decision 9), `cushion_low` (the stored `Cushion` tag at or below `CushionWarn` 0.15 is `warn`, at or below `CushionAct` 0.05 is `act`) and `capture_stale` (the capture is older than one session day, `info`). Severities are `info`, `warn`, `act`. Thresholds are confirmed by the owner (2026-11-09): expiry near 5 calendar days, cushion warn 0.15, act 0.05.
6. Assignment cash by date. The same rule returns `CashByDate`: for each expiry day, the sum of strike x multiplier x quantity of the short puts expiring that day, per currency, with no netting against cash on hand and no conversion. Only the page shows it. It is the cash an assignment would need, not a forecast.
7. Who reads what. The page builder (pages) reads the account capture of each alias for the latest day with positions (stepping back at most 7 calendar days) and for the day before it, the `held_call_guard` rows of the candidate reader, and the earnings rows of the earnings reader, converts them to the plain values of `NoticeInput` (compute depends on contracts only and cannot import store types) and calls the rule. The `held_call_guard` rows are read for the capture day of the alias; with none for that day there is no `call_guard` notice and the block carries one `info` note "no guard rows". `NoticeGuard.Reasons` is filled by the builder from the JSON array text `Candidate.Gates` (the codes of the blocked gates); a text that does not parse keeps the row with no reasons. The issuer of a position is its symbol (US names; an alias map is Left out, Decision 13); a symbol with no earnings row simply has no `earnings_inside` notice.
8. Two accounts. `Config.Main` names the one account that must work and `Config.Aliases` lists the others. A listed alias with no capture in 7 days gets an alias block with status `no_capture` and one `info` text; it never becomes a page gap, a failed build or a notice above `info`. A missing capture for `Main` is a page gap `no_capture` (the page still builds and says so). Nothing in the notices path writes a run, a gap or an alert.
9. Notices are state, not alerts. A notice is recomputed at each page build from stored rows and printed with the data day it came from; nothing is sent, so there is nothing to repeat and no dedupe table. The page lists open notices by severity, then expiry, then contract; the order is the rule's and the front end never sorts. A "first seen" memory (so that an old notice can be folded) is Left out (Decision 14).
10. The page. `option_notices_page` v1 at `/options/notices`, built like the candidates page (`option-candidates-page.md` Decisions 1 to 3): a function of the stored rows, `generated_at` from the caller, a gap object where data is missing. One block per alias (status, data day, notices, cash by date, cushion text), a stale label when the data day is older than the build day, and a link `提示` (`/options/notices`) in the one top-bar dropdown `期权持仓` (shadcn dropdown-menu) that `option-positions-nav` (design PR 747, entry 12) creates; this design adds no top item of its own (owner 2026-11-09). Every text and number string is written by the builder; `web` renders and computes nothing.
11. Libraries. None new. The rule uses `math/big` of the standard library: own implementation, because go.mod holds no decimal library and the rule needs only exact sums and products (same reason as `option-candidates.md` Decision 1). Telegram stays `github.com/go-telegram/bot` behind `notify.Sender`, unchanged.
12. Dependencies on designs not landed. Issues 8 to 12 read tables and types that main has as declarations (`store.Position`, `AccountReader`, `Candidate`, `CandidateReader`, `EarningsDateReader` are in `internal/store/api.go` at 35f3c88) but whose writers (the account job, the candidate job) are other designs; until they run, the page shows the gaps `no_capture` and `no_rows`, which Decision 8 and the candidates page already define. Issue 12 shares files with `option-candidates-page-route` and issue 11 shares `TopNav.tsx` with `option-candidates-page-view` and with `option-positions-nav`; both wait for them (Order lines). Issues 5 to 7 (the morning report) depend on nothing unlanded.
13. The issuer of a held position is its symbol (window default, was question C): a US name has one ticker and one company; a symbol with no earnings row has no `earnings_inside` notice. A symbol-to-company map is Left out.
14. A notice is recomputed at each build and every open notice is listed, with no first-seen memory (window default, was question D): no table, role or migration; the page prints the data day.
15. The gap history is its own module (advisor 2026-11-09; this replaces the second-store-file exception of revision 5, which the advisor ruled out because AGENTS.md allows one interface file per package, and `internal/store/api.go` is 799 lines). A new row `gaphistory` of `docs/design/modules.md`, placed before the store row, holds the package `internal/gaphistory` with its own `api.go` (declarations only: `Gap`, `Reader`, `OpenFunc`) and its own contract test; it depends on `contracts` only and returns its own `Gap` row type, so there is no cycle with `store`. `store` (the implementation in `internal/store/ops`, the only place allowed to import pgx by the depguard rule), `notify` and `ot` list `gaphistory` in their depends cells. Path: the advisor named `internal/store/gaphistory`; `scripts/arch` derives the root component of a row from a glob `internal/<name>/**` (a path of two levels gives the row no component and notify could not import it), and the depguard rule lets only `internal/store/**` import pgx, so the interface package is `internal/gaphistory` and the implementation stays in `internal/store/ops`.

## Modules

One row is added to `docs/design/modules.md`, before the store row: `| gaphistory | internal/gaphistory/** docs/contracts/gaphistory.md | internal/gaphistory/api.go | internal/gaphistory/api_test.go | contracts |`; the depends cells of `store`, `notify` and `ot` gain `gaphistory`; the layers sentence and table list it in the data layer before `store`. This pull request also edits line 47 of that file (two names appended to the sentence that lists the implementation packages `cmd/ot` may import: `internal/compute/optionnotice`, `internal/pages/option_notices_page`) and `arch.toml` follows from it (generated at check time, not committed) (`go run ./scripts/arch --check` prints `arch: ok`; precedent: PR 730, `af36e44`). `.go-arch-lint.yml` is tree-derived, generated at check time and in no Files line; it belongs to repo and no entry names it. The module `compute` gains one package.

| # | issue | module | order |
|---|---|---|---|
| 1 | option-notices-schema (interface) | contracts | first |
| 2 | option-notices-compute-interface (interface) | compute | first |
| 3 | option-notices-gaphistory-interface (interface) | gaphistory | first |
| 4 | option-notices-gaphistory-reader | store | after 3 |
| 5 | option-notices-morning-interface (interface) | notify | after 3 |
| 6 | option-notices-morning-core | notify | after 5 |
| 7 | option-notices-morning-wiring | ot | after 4, 5, 6 |
| 8 | option-notices-rule | compute | after 2, 3 |
| 9 | option-notices-page-interface (interface) | pages | after 1, 2, 3, 8 |
| 10 | option-notices-page-builder | pages | after 1, 2, 9 |
| 11 | option-notices-view | web | after 1, 12, `option-positions-nav` and `option-candidates-page-view` |
| 12 | option-notices-route | ot | after 7, 9, 10 and after `option-candidates-page-route` |

Entry 11 cannot start until `option-candidates-page-view` (and `option-positions-nav`, which also edits `TopNav.tsx`) has merged, and entry 12 not until `option-candidates-page-route` has(issues 4 and 5 of `docs/design/option-candidates-page.md`); they share `TopNav.tsx`, `site.go`, `wire.go`, `demo.go`, `pagelist.go` and `ot.md` with them. Issues 1 to 10 do not wait.

Issues 1, 2 and 3 are independent and start together. The morning report (3 to 7) and the notices (1, 2, 8 to 12) share no file except `cmd/ot` (7 and 12 name each other).

## Interface

Marked: the schema owned by contracts, frozen by issue 1; the compute declarations frozen by issue 2; `gaphistory.Reader` frozen by issue 3; the notify declarations frozen by issue 5; the pages function frozen by issue 9; `deps.optionNoticesPage` and `siteNoticesRoute` owned by ot, frozen by issue 12 (private to `cmd/ot`).

- Gaphistory (owner gaphistory), in `internal/gaphistory/api.go` (a new row of `modules.md`, Decision 15): `type Gap struct { JobID, DatasetID string; Day contracts.Day; Reason string; FirstSeenAt, ReportedAt, ClearedAt time.Time }` (the columns of `store.Gap`, a zero time is NULL); `type Reader interface { GapsSince(ctx context.Context, since time.Time) ([]Gap, error); Close() error }` (rows of `rd.job_gap_r1` with `first_seen_at >= since` or `cleared_at >= since`, ascending by `FirstSeenAt`, never nil); `type OpenFunc func(ctx context.Context, dsn string) (Reader, error)`.
- Notify (owner notify), in `internal/notify/api.go`: `Config` gains `MorningFrom, MorningUntil string` (`HH:MM` in `Config.Zone`; empty means the constants `DefaultMorningFrom = "08:00"` and `DefaultMorningUntil = "12:00"`); `const MorningRuleID = "morning_report"`; `type MorningDeps struct { History gaphistory.Reader; Expect store.GapReader; Sent store.SentStore }`; `type MorningNotifier interface { NotifyMorning(ctx context.Context) (bool, error) }` (true when a message was sent); `type NewMorningNotifierFunc func(MorningDeps, Sender, clockwork.Clock, Config) (MorningNotifier, error)`. No other type of the package changes.
- Compute (owner compute), in `internal/compute/api.go`, plain values and decimal text only: `NoticePosition{Symbol, SecType, Expiry, Strike, Right, Multiplier, Quantity, Currency string}`; `NoticeGuard{Symbol, Expiry, Strike, Status string; Reasons []string}`; `NoticeEarnings{Symbol string; Next *contracts.Day}`; `NoticeValue{Tag, Value, Currency string}`; `NoticeInput{Day contracts.Day; Alias string; CaptureDay contracts.Day; Positions []NoticePosition; PrevDay contracts.Day; PrevPositions []NoticePosition; Guards []NoticeGuard; Earnings []NoticeEarnings; Values []NoticeValue}`; `NoticeParams{ExpiryDays int; CushionWarn, CushionAct string}`; `Notice{Code, Severity, Symbol, Contract string; Expiry contracts.Day; Facts []NoticeFact}` with `NoticeFact{Label, Value string}`; `CashDay{Day contracts.Day; Currency, Amount string}`; `NoticeResult{RuleVersion int; Notices []Notice; CashByDate []CashDay; Cushion string}`; `type OptionNotices interface { Evaluate(NoticeInput, NoticeParams) NoticeResult }`; `type NewOptionNoticesFunc func() OptionNotices`; `type DefaultNoticeParamsFunc func() NoticeParams`.
- Schema `contracts/schemas/page.option_notices_page.v1.json`, const `schema` `option_notices_page`, `schema_version` 1, `additionalProperties` false, under 600 lines, `$ref`s to `common.v2.json` as the candidates page does. Envelope as `market_page`. Page fields: `day`, `accounts` (list of `AliasBlock {alias, main (boolean), status (ok|no_capture|stale), data_day (date or null), notices (list of NoticeRow), cash_by_date (list of {day, currency, amount_text}), cushion_text (string or null), note (string or null)}`), `NoticeRow {id (^notice-[A-Za-z0-9_-]{1,64}$), code (^[a-z_]{1,32}$), severity (info|warn|act), title, contract, expiry (date or null), facts (list of {label, text}), reasons (list of strings)}`. A pattern, not an enum, for `code`, so a new code needs no schema change.
- Pages (owner pages): package `internal/pages/option_notices_page`: `type Config struct { RunID, BuildID, Scope string; Zone *time.Location; Main string; Aliases []string; Account store.AccountReader; Candidates store.CandidateReader; Earnings store.EarningsDateReader; Rule compute.OptionNotices; Params compute.NoticeParams }`; `func NewPage(c Config) pages.Page` with `Meta{Name: "option_notices_page", Schema: "page.option_notices_page.v1"}`; `func Build(ctx context.Context, c Config, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error)` (nil `Zone` or nil `Rule` is an error; a nil reader gives the gap `no_reader`).
- ot: `deps.optionNoticesPage func(...) pages.Page` and `siteNoticesRoute = "options/notices"`.

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses a fake clock, never sleeps, opens no port, and touches no live broker or database; a test that needs PostgreSQL carries the build tag `pg`. Acceptance items are decided by reading, grep or git; the commands run by CI are on the `Checked by CI` line of each entry.

### 1 option-notices-schema (interface)

Module: contracts. Size: M.
Files: `contracts/schemas/page.option_notices_page.v1.json`, `web/src/generated/option_notices_page.v1.ts`, `internal/contracts/option_notices_page_v1_test.go`, `docs/contracts/page-schemas.md`
Design: docs/design/option-notices.md#Interface
Order: first.
Goal: the schema of `## Interface` and its generated type, with a contract test in the style of `internal/contracts/market_page_v1_test.go` (a minimal valid page passes; a wrong `schema_version`, an unknown field, a severity outside `info|warn|act` and a `code` with an uppercase letter fail).
Acceptance:
1. `grep -c '"const": "option_notices_page"' contracts/schemas/page.option_notices_page.v1.json` prints 1; `grep -c '"additionalProperties": false' contracts/schemas/page.option_notices_page.v1.json` is at least 5; `wc -l < contracts/schemas/page.option_notices_page.v1.json` prints less than 600.
2. `grep -c "export interface OptionNoticesPage" web/src/generated/option_notices_page.v1.ts` prints 1 and line 1 is the generated banner; `grep -c "option_notices_page" docs/contracts/page-schemas.md` is at least 1.
3. `grep -c "^func Test" internal/contracts/option_notices_page_v1_test.go` is at least 4 and `grep -c "t.Parallel()"` of the file is at least that count.
4. `grep -c "enum" contracts/schemas/page.option_notices_page.v1.json` shows an enum for `severity` and `status` only (`grep -n '"code"'` shows a `pattern`, no `enum`); `git diff $(git merge-base HEAD origin/main) -- contracts/schemas/carryover.sha256 | grep -c .` prints 0.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/contracts -race -count=1`; `pnpm --dir web run generate:types` leaves `web/src/generated` unchanged; the body carries `Contracts: page.option_notices_page.v1 (new)`.
Frontend stack lines: none (no UI code in this entry).

### 2 option-notices-compute-interface (interface)

Module: compute. Size: S.
Files: `internal/compute/api.go`, `internal/compute/api_test.go`, `docs/contracts/compute.md`
Design: docs/design/option-notices.md#Interface
Order: first. `internal/compute/api.go` is 596 lines and `api_test.go` 587 at 35f3c88; the declarations add under 90 lines, so both stay under 800; if either would pass 800 the change is reported in the pull request description and not split.
Goal: the compute declarations of `## Interface`, declarations only, with a contract test from both sides (a fake `OptionNotices` that returns a fixed `NoticeResult`, and a caller that fills every field of `NoticeInput` and `NoticeParams` and reads the result); `docs/contracts/compute.md` lists the new types.
Acceptance:
1. `grep -c "type OptionNotices interface\|type NoticeInput struct\|type NoticeParams struct\|type NoticeResult struct\|type NewOptionNoticesFunc\|type DefaultNoticeParamsFunc" internal/compute/api.go` prints 6; `git diff $(git merge-base HEAD origin/main) -- internal/compute/api.go | grep '^-' | grep -vc '^---'` prints 0 (additions only).
2. `grep -c "float32\|float64" internal/compute/api.go` prints the same number as at the merge base (`git show $(git merge-base HEAD origin/main):internal/compute/api.go | grep -c "float32\|float64"`).
3. `grep -c "^func TestOptionNoticesContract" internal/compute/api_test.go` is at least 1; `grep -c "time.Sleep\|time.Now()" internal/compute/api_test.go` prints 0.
4. `grep -c "OptionNotices" docs/contracts/compute.md` is at least 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the Files line.
Checked by CI: `go test ./internal/compute -race -count=1`; ci-go.

### 3 option-notices-gaphistory-interface (interface)

Module: gaphistory. Size: S.
Files: `internal/gaphistory/api.go`, `internal/gaphistory/api_test.go`, `docs/contracts/gaphistory.md`
Design: docs/design/option-notices.md#Interface
Order: first; entries 8 and 9 follow it.
Goal: `Gap`, `Reader` and `OpenFunc` of `## Interface`, declarations only, in the new package (the row of `docs/design/modules.md` exists from this design's pull request); `docs/contracts/gaphistory.md` records them; `.go-arch-lint.yml` is generated at check time and is in no Files line. The contract test has two sides: a fake that implements `Reader`, and a caller that uses its rows.
Acceptance:
1. `grep -c "type Gap struct\|type Reader interface\|type OpenFunc" internal/gaphistory/api.go` prints 3; `wc -l < internal/gaphistory/api.go` prints less than 60; the file imports only `context`, `time` and `contracts` of this module (`grep -c "internal/store" internal/gaphistory/api.go` prints 0).
2. `grep -c "^func TestReaderContract" internal/gaphistory/api_test.go` is at least 1 and `grep -c "t.Parallel()"` of it is at least that count; `grep -c "time.Sleep\|time.Now()" internal/gaphistory/api_test.go` prints 0.
3. `git diff $(git merge-base HEAD origin/main) -- go.mod go.sum docs/design/modules.md | grep -c .` prints 0; `git diff $(git merge-base HEAD origin/main) -- internal/store/api.go | grep -c .` prints 0; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the Files line.
Checked by CI: `go test ./internal/gaphistory -race -count=1`; `go run ./scripts/arch --check` (prints `arch: ok`); ci-go.

### 4 option-notices-gaphistory-reader

Module: store. Size: S.
Files: `internal/store/ops/gaps.go`, `internal/store/ops/gaps_test.go`, `internal/store/ops/gap_history_pg_test.go`
Design: docs/design/option-notices.md#Decisions (Decision 3)
Order: after 3. It edits `gaps.go`, which holds the `OpenGaps` of `GapReader`; no other open issue names it.
Goal: the reader of `gaps.go` also implements `gaphistory.Reader` over `rd.job_gap_r1` (SELECT on the view only, the role it already uses), converting each row to `gaphistory.Gap`, and `OpenGapHistory` implements `gaphistory.OpenFunc`; `GapsSince` returns the rows with `first_seen_at >= since` or `cleared_at >= since`, ascending by `FirstSeenAt` then key, an empty slice and not nil for none.
Acceptance:
1. `grep -c "func (r \*gapReader) GapsSince\|func OpenGapHistory" internal/store/ops/gaps.go` prints 2; `grep -c "rd.job_gap_r1" internal/store/ops/gaps.go` is at least 2; `grep -c "ops\.job_gap" internal/store/ops/gaps.go` prints 0.
2. `grep -c "var _ gaphistory.OpenFunc" internal/store/ops/gaps.go` prints 1.
3. `grep -rhE "^func (TestGapsSinceEmptyIsNotNil|TestGapsSinceClearedAfterSince|TestGapsSinceOpenFirstSeenAfterSince|TestGapsSinceOrder)\(" internal/store/ops | wc -l` prints 4; they run on a fake rows source in `gaps_test.go`, and `gap_history_pg_test.go` has the first line `//go:build pg`.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/store/ops -race -count=1`; the `pg` test runs only in GitHub CI (`go test -tags pg ./internal/store/ops`).

### 5 option-notices-morning-interface (interface)

Module: notify. Size: S.
Files: `internal/notify/api.go`, `internal/notify/api_test.go`, `docs/contracts/notify.md`
Design: docs/design/option-notices.md#Interface
Order: after 3 (it names `gaphistory.Reader`).
Goal: the notify declarations of `## Interface`: `MorningFrom` and `MorningUntil` on `Config`, the two constants, `MorningRuleID`, `MorningDeps`, `MorningNotifier`, `NewMorningNotifierFunc`; declarations only. The contract test has two sides: a fake `MorningNotifier`, and a caller that builds `MorningDeps` from fakes and reads the boolean.
Acceptance:
1. `grep -c "type MorningNotifier interface\|type MorningDeps struct\|type NewMorningNotifierFunc\|MorningRuleID\|DefaultMorningFrom\|DefaultMorningUntil" internal/notify/api.go` is at least 6; `git diff $(git merge-base HEAD origin/main) -- internal/notify/api.go | grep '^-' | grep -vc '^---'` prints at most 2 (the `Config` struct lines only).
2. `grep -c "Position\|Candidate\|OptionLedger\|AccountReader\|AccountValue" internal/notify/api.go` prints 0 (Decision 1).
3. `grep -c "^func TestMorningContract" internal/notify/api_test.go` is at least 1; `grep -c "time.Sleep\|time.Now()" internal/notify/api_test.go` prints 0.
4. `grep -c "NotifyMorning" docs/contracts/notify.md` is at least 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the Files line.
Checked by CI: `go test ./internal/notify -race -count=1`; ci-go.

### 6 option-notices-morning-core

Module: notify. Size: M.
Files: `internal/notify/core/morning.go`, `internal/notify/core/morning_text.go`, `internal/notify/core/morning_test.go`, `internal/notify/core/morning_text_test.go`, `internal/notify/core/morning_purity_test.go`
Design: docs/design/option-notices.md#Decisions (Decisions 1 to 4)
Order: after 5. No other entry edits these files; `core.go` and `alerts.go` are not touched.
Goal: `NewMorningNotifier` implements `NewMorningNotifierFunc`. `NotifyMorning`: with the clock before `MorningFrom` or at or after `MorningUntil` in `Config.Zone`, return false and read nothing; with `SentSince` holding `(morning_report, KL date)`, return false; otherwise read `GapsSince(previous 08:00 KL)`, the expectations through `Expect.Expectations` for the last 7 calendar days, taking as the previous session day the newest `ExpectedDay` in the rows returned (no calendar, no clock beyond the injected one), and the open gaps count, build the text of Decision 2, `Send` it once, then `MarkSent(MorningRuleID, KL date)`; a failed send returns the error and marks nothing (the next tick retries inside the window); a failed mark after a send returns the error (at-least-once, as `notify.md` Decision 6). A gap printed in "self-healed" is never marked reported.
Acceptance:
1. `grep -rhE "^func (TestBeforeFromSendsNothing|TestAfterUntilSendsNothing|TestSendsOncePerKLDay|TestSecondTickSameDayIsNoop|TestNextDaySendsAgain|TestQuietDayStillSends|TestSelfHealedListed|TestUnclearedGapsCounted|TestLateExpectationListed|TestMoreThan20LinesCut|TestSendFailureMarksNothing|TestMarkFailureReturnsError|TestNeverMarksGapReported|TestZoneIsKL)\(" internal/notify/core | wc -l` prints 14.
2. `morning_purity_test.go` has `TestNoAccountTypesNoTelegramLeak`: it parses the non-test `.go` files of `internal/notify` (all sub-packages) with `go/parser` and fails on any identifier among `Position`, `Candidate`, `Cushion`, `ExcessLiquidity`, `AccountReader`, `AccountValue`, `OptionLedger`, `NoticeResult`, `CashByDate`, or an import of `internal/pages`, `internal/jobs` or a path containing `optionnotice`; `grep -c "^func TestNoAccountTypesNoTelegramLeak" internal/notify/core/morning_purity_test.go` prints 1.
3. `grep -c "MarkGapReported" internal/notify/core/morning.go` prints 0; `grep -c "time.Now\|time.Sleep" internal/notify/core/morning.go internal/notify/core/morning_text.go` prints 0 for both files; each new `.go` file is under 600 lines.
4. `grep -c "t.Parallel()"` of each new test file is at least its count of `^func Test`; the tests use `clockwork.NewFakeClock` and fakes of `gaphistory.Reader`, `store.GapReader`, `SentStore` and `Sender`, no network.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/notify/... -race -count=1`; ci-go.

### 7 option-notices-morning-wiring

Module: ot. Size: S.
Files: `cmd/ot/notify.go`, `cmd/ot/notify_test.go`, `cmd/ot/scheduler.go`, `cmd/ot/morning_test.go`, `docs/contracts/ot.md`
Design: docs/design/option-notices.md#Decisions (Decisions 2 and 3)
Order: after 4, 5, 6; before 12, which names `cmd/ot` files but not these. `cmd/ot/notify.go` already holds `notifyingGaps`, whose method `notifyAlerts` the morning call follows.
Goal: `notifyingGaps` also calls `NotifyMorning` after the gap and alert passes, with a morning notifier built from `core.NewMorningNotifier`, the sender it already has, `gaphistory.OpenFunc` (`ops.OpenGapHistory`) opened with the line the gap reader of the same file already uses, and the sent store it already opens; an error is logged by `errorClass` and never stops the tick. `ot notify --once` prints the morning text through the dry-run sender and sends nothing when `--dry-run` is set; the quiet-hours flag semantics stay. `docs/contracts/ot.md` records the call.
Acceptance:
1. `grep -c "NotifyMorning" cmd/ot/notify.go` is at least 1; `grep -c "OT_PG_DSN" cmd/ot/notify.go` is not greater than at the merge base (`git show $(git merge-base HEAD origin/main):cmd/ot/notify.go | grep -c "OT_PG_DSN"`): no new DSN line.
2. `grep -c "^func Test" cmd/ot/morning_test.go` is at least 4 (new file, because `cmd/ot/scheduler_test.go` is 767 lines); `grep -rhE "^func (TestSchedulerCallsMorningAfterGaps|TestMorningErrorDoesNotStopTick|TestNotifyOnceDryRunPrintsMorning|TestMorningUsesExistingDSNLine)\(" cmd/ot | wc -l` prints 4.
3. `grep -c "Telegram\|telegram" cmd/ot/scheduler.go` is not greater than at the merge base; `grep -c "NotifyMorning" docs/contracts/ot.md` is at least 1.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./cmd/ot -race -count=1`; ci-go.

### 8 option-notices-rule

Module: compute. Size: L.
Files: `internal/compute/optionnotice/doc.go`, `internal/compute/optionnotice/notice.go`, `internal/compute/optionnotice/expiry.go`, `internal/compute/optionnotice/guard.go`, `internal/compute/optionnotice/assigned.go`, `internal/compute/optionnotice/cushion.go`, `internal/compute/optionnotice/cash.go`, `internal/compute/optionnotice/params.go`, `internal/compute/optionnotice/notice_test.go`, `internal/compute/optionnotice/expiry_test.go`, `internal/compute/optionnotice/guard_test.go`, `internal/compute/optionnotice/assigned_test.go`, `internal/compute/optionnotice/cushion_test.go`, `internal/compute/optionnotice/cash_test.go`, `internal/compute/optionnotice/purity_test.go`
Design: docs/design/option-notices.md#Decisions (Decisions 5, 6 and 11)
Order: after 2, 3.
Goal: `internal/compute/optionnotice` implements `compute.OptionNotices` and `compute.DefaultNoticeParamsFunc` for the codes, severities, thresholds and `CashByDate` of Decisions 5 and 6, with exact arithmetic on decimal text and no float; `.go-arch-lint.yml` is generated at check time and is in no Files line.
Acceptance:
1. `grep -rhE "^func (TestExpiryTodayIsAct|TestExpiryNearWithinDays|TestExpiryBeyondDaysNone|TestLongPositionNoNotice|TestCallGuardFromBlockedRow|TestCallGuardOkRowNone|TestEarningsInsideBeforeExpiry|TestEarningsAfterExpiryNone|TestEarningsMissingNone|TestAssignmentSeen|TestAssignmentNeedsStockRise|TestCushionWarnAndAct|TestCushionAbsentNoNotice|TestCaptureStaleInfo|TestCashByDateSumsShortPuts|TestCashByDatePerCurrency|TestCashIgnoresCallsAndLongs|TestOrderSeverityExpiryContract|TestSameInputSameResult|TestBadDecimalIsSkippedNotPanic)\(" internal/compute/optionnotice | wc -l` prints 20.
2. `TestAssignmentSeen`: a short put of 2 contracts, strike 50, multiplier 100, in `PrevPositions`, absent on its expiry day while 200 shares of the symbol appear, gives the notice `assignment_seen` with the fact `cash` equal to `10000.00`; `TestCashByDateSumsShortPuts`: two short puts expiring the same day (strike 50 x 1 and strike 40 x 2, multiplier 100) give one `CashDay` of `13000`.
3. `purity_test.go` has `TestNoFloatNoClockNoIO`: it parses the non-test files and fails on `float32`, `float64`, an import of `os`, `net/http`, `database/sql`, `time` (the package takes days as `contracts.Day` text), `internal/store` or `internal/notify`, and a call of `time.Now`.
4. `git diff $(git merge-base HEAD origin/main) -- go.mod go.sum | grep -c .` prints 0; every new `.go` file is under 600 lines.
5. `grep -c "t.Parallel()"` of each new test file is at least its count of `^func Test`; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/compute/optionnotice -race -count=1`; `go run ./scripts/arch --check`; ci-go.

### 9 option-notices-page-interface (interface)

Module: pages. Size: S.
Files: `internal/pages/option_notices_page/page.go`, `internal/pages/option_notices_page/page_test.go`
Design: docs/design/option-notices.md#Interface
Order: after 1, 2, 3, 8 (1, 2 and 3 are first).
Goal: `page.go` holds `Config`, `NewPage` and a `Build` whose body answers the error `option_notices_page: not built`. The contract test has two sides: `NewPage(Config{...})` has the stated `Meta`; a caller fills every field of `Config` with fakes of `store.AccountReader`, `store.CandidateReader`, `store.EarningsDateReader` and `compute.OptionNotices` and calls `Build`.
Acceptance:
1. `grep -c 'type Config struct\|^func NewPage\|^func Build' internal/pages/option_notices_page/page.go` prints 3; `wc -l < internal/pages/option_notices_page/page.go` prints less than 70; the file imports only `context`, `errors`, `time`, `contracts`, `store`, `compute` and `pages` of this module (`grep -c 'internal/notify\|internal/jobs' internal/pages/option_notices_page/page.go` prints 0).
2. `grep -c "^func TestNewPageMeta\|^func TestConfigContract" internal/pages/option_notices_page/page_test.go` prints 2; `grep -c "time.Sleep\|time.Now()" internal/pages/option_notices_page/page_test.go` prints 0.
3. `git diff $(git merge-base HEAD origin/main) -- go.mod go.sum | grep -c .` prints 0; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the Files line.
Checked by CI: `go test ./internal/pages/option_notices_page -race -count=1`; `go run ./scripts/arch --check`; ci-go.

### 10 option-notices-page-builder

Module: pages. Size: L.
Files: `internal/pages/option_notices_page/page.go`, `internal/pages/option_notices_page/page_test.go`, `internal/pages/option_notices_page/builder.go`, `internal/pages/option_notices_page/capture.go`, `internal/pages/option_notices_page/inputs.go`, `internal/pages/option_notices_page/blocks.go`, `internal/pages/option_notices_page/text.go`, `internal/pages/option_notices_page/builder_test.go`, `internal/pages/option_notices_page/capture_test.go`, `internal/pages/option_notices_page/inputs_test.go`, `internal/pages/option_notices_page/blocks_test.go`, `internal/pages/option_notices_page/text_test.go`, `internal/pages/option_notices_page/purity_test.go`, `internal/pages/option_notices_page/testdata/`, `docs/contracts/pages.md`
Design: docs/design/option-notices.md#Decisions (Decisions 7 to 10)
Order: after 1, 2 and 9; it edits `page.go` and `page_test.go` of 9 (the stub body and its test).
Goal: `Build` finds, for each alias, the latest capture day with positions (at most 7 calendar days back) and the one before it, reads the `held_call_guard` rows of that day and the earnings rows, converts to `compute.NoticeInput`, calls `Config.Rule.Evaluate` with `Config.Params`, and writes the alias blocks of Decision 10 in the rule's order; a non-main alias with no capture is a block with status `no_capture` and one `info` note; a main alias with none is the page gap `no_capture`; text and number strings (amounts as decimal text with the currency, the cushion as written by the rule) are written here from the result; the output validates against the schema of issue 1.
Acceptance:
1. `grep -rhE "^func (TestAliasBlockPerAlias|TestMainMissingIsPageGap|TestSecondMissingIsInfoBlock|TestSecondMissingNeverFailsBuild|TestLookbackSevenDays|TestPreviousDayFound|TestGuardRowsOnlyBlockedKind|TestInputIsPlainValues|TestRuleOrderKept|TestCashByDateText|TestCushionText|TestStaleLabelWhenOlderDay|TestNilReaderIsGap|TestNilRuleIsError|TestOutputValidatesAgainstSchema|TestNoFloatNoSortNoNotify|TestNoClockNoIO)\(" internal/pages/option_notices_page | wc -l` prints 17.
2. `TestSecondMissingNeverFailsBuild`: `Account` returns rows for `Main` and an error `no rows` for the other alias; `Build` returns no error, the other alias has status `no_capture` and the output has no page gap. `TestMainMissingIsPageGap`: no rows for `Main` in 7 days gives the gap `no_capture` and a valid page.
3. `purity_test.go` `TestNoFloatNoSortNoNotify` parses the non-test files and fails on `float32`, `float64`, an import of `math/big`, `sort`, `slices` with a `Sort` call, `os`, `net/http`, `database/sql`, `internal/notify`, `internal/jobs`; `TestNoClockNoIO` fails on a call of `time.Now` or `time.Sleep`.
4. `grep -c "t.Parallel()"` of each new test file is at least its count of `^func Test`; `TestOutputValidatesAgainstSchema` validates three testdata inputs (a notice of each severity, only a second alias missing, no rows) against `contracts/schemas/page.option_notices_page.v1.json`; every new `.go` file is under 600 lines.
5. `grep -c "option_notices_page" docs/contracts/pages.md` is at least 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/pages/option_notices_page -race -count=1`; ci-go.

### 11 option-notices-view

Module: web. Size: M.
Files: `web/src/shell/TopNav.tsx`, `web/test/shell/TopNav.test.tsx`, `web/src/types/option_notices_page.ts`, `web/src/pages/notices/OptionNoticesPage.tsx`, `web/src/pages/notices/AliasBlock.tsx`, `web/src/pages/notices/NoticeList.tsx`, `web/test/pages/optionnotices/OptionNoticesPage.test.tsx`, `web/test/pages/optionnotices/fixtures/ok.ts`, `web/test/pages/optionnotices/fixtures/second-missing.ts`, `web/test/pages/optionnotices/fixtures/gap.ts`, `web/test/pages/optionnotices/fixtures/bad-version.json`, `web/test/pages/optionnotices/fixtures/MANIFEST.synthetic-v1.json`
Design: docs/design/option-notices.md#Decisions (Decision 10)
Order: after 1 and after 12 (this design's route, so the link never points at a missing route), after `option-positions-nav` (entry 12 of `docs/design/option-positions.md`, design PR 747), which creates the dropdown `期权持仓` in `TopNav.tsx` and edits `TopNav.test.tsx`, and after `option-candidates-page-view`, which also edits both files; this entry adds one link to the menu.
Goal: the view of `OptionNoticesPage`: a header with the data day and the stale alert; one card per alias (main first as the payload lists them) with its notices in the payload order and a severity badge, a "cash by date" table and the cushion text; a block with status `no_capture` shows its note and no table; a page gap shows the cause once; `TopNav.tsx` gets the link `提示` (`/options/notices`) inside the dropdown `期权持仓` made by `option-positions-nav`, and no new top item (the bar stays at ten items). Every text is a payload field; nothing is compared, summed, sorted or formatted here.
Acceptance:
1. `web/src/types/option_notices_page.ts` imports `OptionNoticesPage` from `@/generated/option_notices_page.v1` and `grep -c "SUPPORTED_VERSIONS" web/src/types/option_notices_page.ts` is at least 1; `grep -c "registration = { schema: 'option_notices_page'" web/src/pages/notices/OptionNoticesPage.tsx` prints 1.
2. `grep -rc "Math\.\|\.sort(\|\.reduce(\|parseFloat\|Number(\|toFixed" web/src/pages/notices/OptionNoticesPage.tsx web/src/pages/notices/AliasBlock.tsx web/src/pages/notices/NoticeList.tsx` prints 0 for each file; `git diff $(git merge-base HEAD origin/main) -- web/no-compute-allow.txt web/package.json web/deps-allowlist.txt frontend-whitelist.json pnpm-lock.yaml | grep -c .` prints 0.
3. `grep -c "/options/notices" web/src/shell/TopNav.tsx` prints 1 and `grep -c "提示" web/src/shell/TopNav.tsx` is at least 1; the diff of `web/src/shell/TopNav.tsx` adds exactly one line holding `/options/notices` (`git diff $(git merge-base HEAD origin/main) -- web/src/shell/TopNav.tsx | grep -c '^+.*/options/notices'` prints 1); `grep -c "Telegram\|telegram" web/src/pages/notices/*.tsx` prints 0 for each file.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `pnpm --dir web exec tsc --noEmit`, `pnpm --dir web exec eslint .`, `pnpm --dir web exec vitest run`, `pnpm --dir web run build`.
Frontend stack lines: no new dependency; React and the shared table and card components of `web/src/shell` as the other option pages use them.

### 12 option-notices-route

Module: ot. Size: M.
Files: `cmd/ot/notices_site.go`, `cmd/ot/notices_site_test.go`, `cmd/ot/site.go`, `cmd/ot/site_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/demo.go`, `cmd/ot/pagelist.go`, `cmd/ot/pagelist_test.go`, `docs/contracts/ot.md`
Design: docs/design/option-notices.md#Decisions (Decisions 7, 8 and 10)
Order: after 7, 9, 10, and after `option-candidates-page-route`, which edits `site.go`, `site_test.go`, `wire.go`, `wire_test.go`, `demo.go`, `pagelist.go`, `pagelist_test.go` and `ot.md`; also after 7 (`docs/contracts/ot.md`).
Goal: `siteHandler` serves `options/notices` (`siteNoticesRoute`) with the day of `--day` or today in the zone and `ot page --name option_notices_page` builds the same page; `openSiteReaders` opens the account, candidate and earnings stores once with the one pages line `OT_PG_DSN_PAGES` and adds them to `siteReaders`; `Config.Main` and `Config.Aliases` come from the profile exactly as the account job reads them (no account id, only the aliases); `compute.DefaultNoticeParamsFunc` gives `Params`; the demo profile serves a fixed demo result (one notice of each severity, a second alias with `no_capture`) and opens no store; a failed build answers 500 with the fixed text; the bar is the web issue's and `cmd/ot` holds no bar (`grep -c "提示" cmd/ot/*.go` prints 0).
Acceptance:
1. `grep -rhE "^func (TestNoticesRouteServesPage|TestNoticesRouteOnlyGet|TestNoticesDemoNeedsNoStore|TestNoticesUsesPagesDSN|TestNoticesBuildErrorIs500|TestPageCommandBuildsNotices)\(" cmd/ot | wc -l` prints 6.
2. `grep -c "OT_PG_DSN_ACCOUNT\|OT_PG_DSN_CANDIDATE\|OT_PG_DSN_EVENTS" cmd/ot/notices_site.go` prints 0; `grep -c "internal/notify\|telegram\|Sender" cmd/ot/notices_site.go` prints 0; `grep -c "sort\.\|big\.\|float64" cmd/ot/notices_site.go` prints 0; `grep -c "options/notices" docs/contracts/ot.md` is at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./cmd/ot -race -count=1`; ci-go.

## Left out

- Pushes about holdings of any kind (expiry, assignment, cushion, cash, guard): never, by Decision 1; a Telegram text that says "a notice exists" is also out, because the fact alone is derived from positions.
- Market alerts in the morning report (said once by `NotifyAlerts`, Decision 4); a "recovered" push (`notify.md` Decision 14); inbound Telegram commands; a second chat.
- A first-seen memory for notices (a table, a role and a migration), so a notice open for ten days could fold; the page prints the data day and the order, nothing more (Decision 14).
- Account figures beyond what a notice needs: the margin cushion block, the observed-assignment history and the cost basis and wheel pages are the later design `account-figures` (`option-ledger.md` Left out); the rule here computes only `cushion_low`, `assignment_seen` and `CashByDate`, each in one function, so that design can call or replace them.
- Notices for Japanese options, for long positions and for stock positions alone; an issuer map from symbol to company (Decision 13); the night rule beyond the quiet hours that already exist.
- Calendar or session logic in the rule: days are calendar days, DTE is an ET calendar-day difference as the plan's units say, and holidays do not move an expiry.
- Alternatives the adversarial pass offered and this design did not take: no history reader (notify remembering the keys it pushed: not chosen because a restart would lose them and the table is the one truth); folding the healed lines into `NotifyGaps` (not chosen because the owner asked for an 08:00 message).
- What is not verified: that `alerts.sent` accepts the rule id `morning_report` (issue 6 tests it against the store fake and the `pg` test of the store, not a live database), and the exact name of the DSN line the gap reader of `cmd/ot/notify.go` uses (issue 7 reuses whichever it is).

Owner answers of 2026-11-09 ("a a a a a a") are applied in Decisions 2, 4, 5 and 10; no question is open.

## Adversarial

The pass is saved in full at `scratchpad/design/option-notices-adversarial.md` of the design session.

1. Gap: adopted in part. Decision 7 now fixes the guard day (the capture day, an `info` note when none) and the source of `Reasons` (parsed from `Gates`). The empty page until the jobs land stays and is named (Decision 12); the heuristic of `assignment_seen` stays, as `account-data.md` Decision 9 says it is only as reliable as the captures. Seeing a notice needs opening the page: not adopted, the owner's rule forbids a push.
2. Biggest risk (repeat): adopted. Decisions 2 and 4 now print only never-pushed healed gaps with ids and everything else as counts.
3. Simpler way: not adopted. The history reader stays (Left out names why); the issue count stays 12 because each is one module and the interface entries are required by AGENTS.md. The suggestion to drop `Facts` is not adopted: the page shows several labelled numbers per notice and the front end may not format.
4. Lock-in: not adopted as changes; named. `alerts.sent` carries one extra rule id with no schema change, `Config` gains two string fields, and the schema `code` is a pattern. The flat string input is the price of compute depending on contracts only.
5. Claim most likely false: adopted. The "no repeat" claim is fixed (2). The size findings are adopted: issue 3 declares in the new module `gaphistory` (revisions 2 and 6), issue 7 moves its tests to the new `cmd/ot/morning_test.go`. Issue 1 item 4 stays a countable grep, read as two commands. The mark after send stays at-least-once as in `notify.md` Decision 6.
6. Slip: partly adopted. The collisions with the candidates entries are in the Order lines of 11 and 12; the size slip is removed by the module `gaphistory` of issue 3 (revision 6).
