# Notify: a missed-job alert over Telegram, said once (stage 1)

Author: the optdesk designer (first draft)
Owner approved: (pending)
Revision: 1

Source: owner asks of 2026-11-08 (the missed-job alert is pulled forward ahead of the risk and alerts design; "missed work is caught up"; modules are designed interface first so they are built in parallel, against the interfaces as designed and not against landed code; library: `github.com/go-telegram/bot`; own glue code sits behind an interface so a library can replace it), the advisor's check list of 2026-11-08, `docs/adr/0010-reports-and-delivery.md` (Telegram never carries account-derived information).
Read on main at 929878e: `AGENTS.md`, `docs/design/modules.md` (row notify), `docs/design/scheduler.md` (Decision 9, Interface, `sched-store-interface`, `sched-cmd-run`), `docs/design/home-board-1.md` (format), `internal/store/api.go` (`DSNEnvRunner`), `internal/profile/api.go` (`Zone`), `cmd/ot/env.go`, `arch.toml`.
Read on the branch of open PR #358 (`design/scheduler-order`): the same file; there `Gap{JobID, DatasetID string; Day contracts.Day; Reason string; FirstSeenAt, ReportedAt, ClearedAt time.Time}` is in the Interface section and Decision 9 says "`reported_at` stays null in stage 1 ... the later `notify` module will call `MarkGapReported`". This design is written against that text, not against landed code (see `## Left out`, "What is not verified").

## Decisions

1. What the module does. The scheduler writes a row in `ops.job_gap` when work is missed (`expired`, `crashed_repeatedly`, `preflight_failed`) and catches the work up (one `Reconcile` at start, a retry at each 15-minute tick); the row is cleared when the key is present. `notify` reads the open rows that have no `reported_at`, sends one text, and marks each row reported. It never decides that something is missed and never runs a job: the gap rows are the whole input.
2. Ownership of `MarkGapReported`: store. It is already declared on `store.SchedulerLedger` (written by role `ot_runner`) in the Interface section of `docs/design/scheduler.md`, and `OpenGaps` on `store.GapReader`. This design does not edit the store interface. `internal/notify/api.go` declares its own consumer-side port, `GapSource`, with the two methods it needs; `cmd/ot` builds it from a `store.GapReader` and a `store.SchedulerLedger` with a small adapter (issue 5). Result: notify can be built and tested now against a fake `GapSource`, and if the store signature of `MarkGapReported` ends up different from the one assumed here, only the adapter changes, not notify. No cross-module edit is needed, so the (interface) entry is notify's own.
3. Interface in one file, no logic: `internal/notify/api.go` holds `Sender`, `GapSource`, `Notifier`, `Config` and the constructor types, as the one interface file of the package (AGENTS.md). The implementations are sub-packages, so each can be built in parallel: `internal/notify/core` (the Notifier), `internal/notify/dryrun` (stdout), `internal/notify/telegram` (the library). The fake `Sender` and fake `GapSource` for tests are `internal/notify/notifytest` (a test kit, never imported by `cmd/ot`).
4. Who runs it: a separate resident subcommand, `ot notify`, not a scheduler job and not `ot serve`. A scheduler job (`runner.Scheduled`) is a unit that produces a key for a dataset window; notify produces no key and would need a window and a gap of its own, so it does not fit, and a gap about the notifier would be said by the notifier. `ot serve` reads through `ot_ro_pages` and must stay without a write. So `ot notify` is its own process, as `ot scheduler` is (`docs/design/scheduler.md`, Decision 2), run by the keeper like it.
5. Role and DSN: `ot notify` opens one DSN, the line `OT_PG_DSN_RUNNER` (`store.DSNEnvRunner`, role `ot_runner`), for both the read and the write. `ot_runner` already has `SELECT` on the views `rd.job_gap_r1` and `rd.expectation_r1` and the write on `ops.job_gap` (`migrations/0012_scheduler.sql` of the scheduler design). The notify code can call only the two methods of `GapSource`, so the only write it can make is `MarkGapReported`. No new role and no migration. A narrower role (`UPDATE reported_at` only) is left out; the owner may ask for it (question 1).
6. One pass, `Notifier.NotifyGaps(ctx)`: read open gaps; drop a gap with `ClearedAt` set or `ReportedAt` set; if none, send nothing and return 0; inside quiet hours (7) return 0 and leave the rows unmarked; otherwise build one text for the whole batch (8), send it once, and only after the sender returned nil call `MarkGapReported(ctx, gap, now)` for each gap of the batch. A failed send marks nothing. A failed mark after a confirmed send returns the error and is retried at the next pass, so the delivery is at-least-once: the text of a gap may repeat only when the mark itself fails. That is stated in `docs/contracts/notify.md`; it is the one case where "said once" can break, and the alternative (mark first) can lose a message, which is worse.
7. Quiet hours, a setting with a code default: `Config.QuietFrom` and `Config.QuietTo` as `"HH:MM"` in the zone `Config.Zone` (the `profile.Zone` that `cmd/ot` passes; no market hours are written by hand). The default is 23:00 to 07:00 (question 2), the window may cross midnight, and `ot notify --quiet 23:00-07:00` overrides it (`--quiet off` disables it). Held gaps are said at the first pass after the window; nothing is lost because the rows stay unreported.
8. Message text, fixed by the code and not by the gap: first line `optdesk: N job(s) missed work`, then one line per gap, `job <JobID> day <YYYY-MM-DD> <reason>`, `reason` one of `expired`, `crashed_repeatedly`, `preflight_failed` and `unknown` for anything else (the stored string is never copied). At most 20 lines, then `and M more`; the whole text stays under 3500 bytes (Telegram's limit is 4096). Plain text: no parse mode, so no escaping. One message per batch. The text carries no dataset id, account id, DSN, path, token, chat id or error text; ADR 0010 holds (no account-derived information). Gaps are sorted by `FirstSeenAt`, then `JobID`, so the text is deterministic. The test of a text sets a sentinel in `Reason` and `DatasetID` and asserts it is absent.
9. Retry is bounded so an outage does not loop. A pass makes one send attempt. `Notifier.Run(ctx, Config)` calls `NotifyGaps` at the start and then every `Config.Every` (default 15 minutes, the scheduler's tick) on the injected `clockwork.Clock`; after a failed pass the next wait doubles (15, 30, 60 minutes) and stays at 60 until a pass succeeds, and then returns to `Every`. There is no inner retry loop and no sleep. `Run` returns when the context ends; it returns an error only for a bad `Config`.
10. Secrets: the bot token and the chat id are two lines of `$OT_HOME/.env`, `OT_TELEGRAM_TOKEN` and `OT_TELEGRAM_CHAT_ID` (constants `notify.TokenEnv` and `notify.ChatIDEnv`), read only by the `ot notify` process through `deps.readEnv` as the DSN lines are; the owner supplies the values and no agent writes them. They are in no code, no configuration, no fixture, no log line and no error text (AGENTS.md); tests use synthetic values (a token `test-token`, a chat id `1000001`, which are not real) and never call Telegram. Without `--dry-run`, a missing line is an error that names the variable and nothing else. The Telegram sender returns its own error classes (`notify.ErrSend`, with an HTTP status class when there is one) and never wraps the library's error, because the library's errors can carry the request URL, which holds the token; `TestTelegramErrorHoldsNoToken` uses a sentinel token against an `httptest` server.
11. Library, per AGENTS.md "Libraries": `github.com/go-telegram/bot` v1.27.0 (module proxy, tagged 2026-09-11; repository pushed 2026-11-02), license MIT, `go 1.18`, no `require` line in its `go.mod` (no transitive dependency), 1.9k stars, one `SendMessage` call is all we use (`bot.New(token, WithServerURL, WithHTTPClient, WithSkipGetMe)`, so a test points it at an `httptest` server with no fixed port). It is chosen by the owner's word of 2026-11-08, and it touches secrets and the network, so the owner's word is also the sign-off the rule asks for. The Design line of issue 4 reads "library chosen by the owner on 2026-11-08; own code is the sender wrapper only, behind `notify.Sender`". The own glue (`telegram.Sender`, the Notifier, the text) sits behind `Sender` and `GapSource`, so the library can be replaced by editing one package (owner, 2026-11-08).
12. Dry run: `dryrun.Sender` writes the text to an `io.Writer` (stdout in `ot notify --dry-run`, a buffer in tests) and needs neither token nor chat id. With `--dry-run` `ot notify` reads the gaps and prints the text but calls no `MarkGapReported`, so a dry run never uses up a message (the `GapSource` is wrapped by a read-only view in `cmd/ot`); `--once` makes one pass and exits. Both flags are for local use and for the owner's first smoke.
13. Tests (all of them): `t.Parallel()`, a fake clock (`clockwork.NewFakeClock`), no sleep, a fake `Sender` and a fake `GapSource` from `notifytest`, no live Telegram, no fixed port or file; the Telegram package tests run against `httptest`.

## Second opinion

This is a second-opinion case: a new outside service and a new account (Telegram), and a new credential (the token, the chat id) that the process holds. The brief goes to the advisor (optdesk-advisor) after the design review; the answer is recorded here in a later revision, and the merge waits for it and for the owner's word. The issues that wire Telegram, 4 and 5, carry the waiver lines when the advisor has answered, not now.

Brief for the advisor: (a) the sender never wraps a library error, so the token cannot reach a log; (b) the text is generic by construction (job id, day, a fixed reason word); (c) at-least-once delivery with the mark after the send; (d) one DSN, `ot_runner`, for read and write; (e) the token and chat id in `.env` only.

## Modules

One change of `docs/design/modules.md`, in text and in one cell: the depends cell of row `ot` gains `notify`, and the sentence that lists the implementation packages `cmd/ot` may import gains `internal/notify/core`, `internal/notify/telegram` and `internal/notify/dryrun` (the pull request of this design carries both and the regeneration of `arch.toml` by `go run ./scripts/arch`; `go run ./scripts/arch --check` exits 0). Row `notify` is unchanged (paths `internal/notify/** docs/contracts/notify.md`, entry `internal/notify/api.go`, tests `internal/notify/api_test.go`, depends `contracts store`); its cells already cover every path below. The `go.mod` and `go.sum` of issue 4 belong to repo, so issue 4 is a two-module entry (notify and repo), as the rule in `docs/design/modules.md` says. `cmd/**` and `docs/contracts/ot.md` belong to ot. The layer rule holds: notify (scheduling) depends on store (data) only through `store.Gap`.

## Interface

Owner: notify. Frozen by `notify-interface`; read by every issue after it. In `internal/notify/api.go`:

- `Sender`: `Send(ctx context.Context, text string) error`. Delivers one text message; nil only when the service confirmed it. `ErrSend` is the error class; the text of an error never holds a token, a chat id or a URL.
- `GapSource`: `OpenGaps(ctx) ([]store.Gap, error)` and `MarkGapReported(ctx, g store.Gap, at time.Time) error` (the first is `store.GapReader.OpenGaps`, the second `store.SchedulerLedger.MarkGapReported`, joined by the adapter of issue 5).
- `Notifier`: `NotifyGaps(ctx) (int, error)` (one pass, the number of gaps reported) and `Run(ctx) error` (the loop of Decision 9). `Config{Every time.Duration; QuietFrom, QuietTo string; Zone *time.Location}`. Constants `TokenEnv`, `ChatIDEnv`, `DefaultEvery`, `DefaultQuietFrom`, `DefaultQuietTo`. Constructor types `NewNotifierFunc(GapSource, Sender, clockwork.Clock, Config) (Notifier, error)`, `NewTelegramSenderFunc(token, chatID string, opts ...string) (Sender, error)` and `NewDryRunSenderFunc(io.Writer) Sender`.

## Issues

### notify-interface (interface)

- Module: notify. Size: S. Depends on: none.
- Order: first.
- Design: docs/design/notify.md#Interface
- Files: internal/notify/api.go, internal/notify/api_test.go, docs/contracts/notify.md

- Goal: the declarations of the Interface section, declarations only, with a contract test from both sides (a fake `Sender`, a fake `GapSource` and a caller that uses them) and `docs/contracts/notify.md` recording who reads and who writes (Decision 2, 5), the at-least-once rule (Decision 6), the message rules (Decision 8) and the secret lines (Decision 10).

Acceptance:

1. `grep -c "type Sender interface\|type GapSource interface\|type Notifier interface\|type Config struct" internal/notify/api.go` prints 4; `grep -c "^func [A-Za-z]*(" internal/notify/api.go` prints 0.
2. `grep -c "var _ notify.NewNotifierFunc\|var _ notify.NewTelegramSenderFunc\|var _ notify.NewDryRunSenderFunc" internal/notify/api_test.go` prints 3; `grep -c "t.Parallel()" internal/notify/api_test.go` is at least `grep -c "^func Test" internal/notify/api_test.go`.
3. `grep -c "OT_TELEGRAM_TOKEN\|OT_TELEGRAM_CHAT_ID\|at-least-once\|MarkGapReported" docs/contracts/notify.md` is at least 4; `grep -c "[0-9]\{6,\}:[A-Za-z0-9_-]\{20,\}" internal/notify/api.go internal/notify/api_test.go docs/contracts/notify.md` prints 0 for each file.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line; `git diff $(git merge-base HEAD origin/main) -- internal/store | grep -c .` prints 0.

Checked by CI: ci-go (`go test ./...`, go-arch-lint, golangci-lint); `go test ./internal/notify -race -count=1`; the `arch.toml` check; the design verdict (the result matches the Interface section).

### notify-core

- Module: notify. Size: M. Depends on: notify-interface.
- Order: after 1 (notify-interface).
- Design: docs/design/notify.md#Decisions
- Files: internal/notify/core/core.go, internal/notify/core/text.go, internal/notify/core/quiet.go, internal/notify/core/core_test.go, internal/notify/notifytest/notifytest.go

- Goal: Decisions 6 to 9 and 13. `core.New` is a `notify.NewNotifierFunc`: the pass, the text (a pure function of `[]store.Gap`), the quiet window (a pure function of the clock's now, the zone and the two strings; a window may cross midnight) and the bounded loop on the injected clock. `notifytest` holds the fake `Sender` (records the texts, can fail on demand) and the fake `GapSource` (rows in memory, records marks, can fail on demand).

Acceptance:

1. `grep -c "^func Test" internal/notify/core/core_test.go` is at least 12, and the names include `TestMarkOnlyAfterSend`, `TestFailedSendMarksNothing`, `TestOneMessagePerBatch`, `TestReportedGapNotRepeated`, `TestClearedGapSkipped`, `TestQuietHoursHoldGaps`, `TestQuietWindowCrossesMidnight`, `TestTextHoldsNoSentinel`, `TestTextCapsLines`, `TestBackoffDoubles`, `TestMarkFailureRetried` and `TestBadConfigRefused`: `grep -c "func TestMarkOnlyAfterSend\|func TestFailedSendMarksNothing\|func TestOneMessagePerBatch\|func TestReportedGapNotRepeated\|func TestClearedGapSkipped\|func TestQuietHoursHoldGaps\|func TestQuietWindowCrossesMidnight\|func TestTextHoldsNoSentinel\|func TestTextCapsLines\|func TestBackoffDoubles\|func TestMarkFailureRetried\|func TestBadConfigRefused" internal/notify/core/core_test.go` prints 12.
2. `grep -c "t.Parallel()" internal/notify/core/core_test.go` is at least `grep -c "^func Test" internal/notify/core/core_test.go`; `grep -c "time.Sleep\|time.Now()" internal/notify/core/*.go` prints 0 for each file.
3. `grep -c "telegram\|net/http\|os.Getenv" internal/notify/core/core.go internal/notify/core/text.go internal/notify/core/quiet.go` prints 0 for each file; `git ls-files internal/notify/core | grep -c "_test.go"` prints 1.
4. `grep -c "clockwork.NewFakeClock" internal/notify/core/core_test.go` is at least 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the five paths of the Files line.

Checked by CI: ci-go; `go test ./internal/notify/... -race -count=1`; the size rule (each new file under 600 lines).

### notify-dryrun

- Module: notify. Size: S. Depends on: notify-interface.
- Order: after 1 (notify-interface).
- Design: docs/design/notify.md#Decisions
- Files: internal/notify/dryrun/dryrun.go, internal/notify/dryrun/dryrun_test.go

- Goal: Decision 12, the sender half: `dryrun.New(io.Writer) notify.Sender` writes the text and one newline to the writer, never fails except on a writer error (then `notify.ErrSend`), needs no configuration.

Acceptance:

1. `grep -c "var _ notify.NewDryRunSenderFunc" internal/notify/dryrun/dryrun_test.go` prints 1; `grep -c "^func Test" internal/notify/dryrun/dryrun_test.go` is at least 2 (`TestDryRunWritesText`, `TestDryRunWriterErrorIsErrSend`).
2. `grep -c "t.Parallel()" internal/notify/dryrun/dryrun_test.go` is at least `grep -c "^func Test" internal/notify/dryrun/dryrun_test.go`; `grep -c "os.Stdout\|os.Getenv\|net/http" internal/notify/dryrun/dryrun.go` prints 0.
3. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the two paths of the Files line.

Checked by CI: ci-go; `go test ./internal/notify/dryrun -race -count=1`.

### notify-telegram

- Module: notify and repo (go.mod, go.sum). Size: M. Depends on: notify-interface.
- Order: after 1 (notify-interface).
- Design: docs/design/notify.md#Decisions (library chosen by the owner on 2026-11-08; own code is the sender wrapper only, behind `notify.Sender`; new direct dependency `github.com/go-telegram/bot` v1.27.0, MIT)
- Files: internal/notify/telegram/telegram.go, internal/notify/telegram/telegram_test.go, go.mod, go.sum

- Goal: Decisions 10 and 11: `telegram.New(token, chatID string, opts ...string)` is a `notify.NewTelegramSenderFunc` over `bot.New(token, bot.WithSkipGetMe(), ...)` and one `SendMessage` with no parse mode; an option names a server URL for tests. A failure becomes `notify.ErrSend` with a status class (`4xx`, `5xx`, `network`, `timeout`) and nothing else; each call has a 10-second deadline taken from the context. Waiver lines for the second opinion are added to this entry after the advisor answers.

Acceptance:

1. `grep -c "github.com/go-telegram/bot v1.27.0" go.mod` prints 1; `git diff $(git merge-base HEAD origin/main) -- go.mod | grep -c '^+[^+]'` prints 1 (the one direct dependency, no other line added); `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the four paths of the Files line.
2. `grep -c "^func Test" internal/notify/telegram/telegram_test.go` is at least 6, with `TestTelegramSendsText`, `TestTelegramErrorHoldsNoToken`, `TestTelegramStatusClasses`, `TestTelegramNoParseMode`, `TestTelegramDeadline` and `TestTelegramEmptyConfigRefused` present (`grep -c` of the six names prints 6); every test uses an `httptest` server, `grep -c "api.telegram.org" internal/notify/telegram/telegram_test.go` prints 0.
3. `grep -c "t.Parallel()" internal/notify/telegram/telegram_test.go` is at least `grep -c "^func Test" internal/notify/telegram/telegram_test.go`; `grep -c "%w\|err.Error()\|Errorf" internal/notify/telegram/telegram.go` prints 0 (no library error is wrapped or copied).
4. `grep -c "[0-9]\{6,\}:[A-Za-z0-9_-]\{20,\}" internal/notify/telegram/telegram.go internal/notify/telegram/telegram_test.go` prints 0 for each file; `grep -rc "github.com/go-telegram/bot" --include=*.go internal cmd | grep -v ":0" | grep -vc "internal/notify/telegram/"` prints 0 (only this package imports the library).
5. The pull request description names the new dependency with its license (MIT) and the owner's word of 2026-11-08, and carries `Contracts: unchanged`.

Checked by CI: ci-go (`go mod verify`, `go test ./internal/notify/telegram -race -count=1`, go-arch-lint, golangci-lint); the dependency-license check if CI has one; the design verdict.

### notify-cmd

- Module: ot. Size: M. Depends on: notify-interface, notify-core, notify-dryrun, notify-telegram, and sched-cmd-run of `docs/design/scheduler.md` (it edits the same `wire.go` and `docs/contracts/ot.md`; that entry is in another design, so the crew orders it by that issue's landing and this design's `Order` names only its own numbers).
- Order: after 1, 2, 3 and 4 (notify-interface, notify-core, notify-dryrun, notify-telegram).
- Design: docs/design/notify.md#Decisions
- Files: cmd/ot/notify.go, cmd/ot/notify_test.go, cmd/ot/gapadapter.go, cmd/ot/wire.go, cmd/ot/wire_test.go, docs/contracts/ot.md

- Goal: `ot notify` (Decisions 4, 5, 10, 12): reads the DSN line `OT_PG_DSN_RUNNER` and the two Telegram lines through `deps.readEnv`, builds through `withWiring` (no copy of it), joins `store.GapReader` and `store.SchedulerLedger` into a `notify.GapSource` in `gapadapter.go`, wraps it read-only for `--dry-run`, and runs `Notifier.Run` until SIGTERM (`--once` runs one pass; `--quiet`). It does not start the site and does not take the scheduler's lock. Flags and errors hold no token, chat id or DSN. The `deps` of `wire.go` gains the notify constructors; `wire_test.go` follows the field count.

Acceptance:

1. `grep -c 'register("notify"' cmd/ot/notify.go` prints 1; `grep -c "OT_PG_DSN_RUNNER\|store.DSNEnvRunner" cmd/ot/notify.go` is at least 1 and `grep -c "OT_PG_DSN_PAGES\|DSNEnvPages\|AdminDSNEnv" cmd/ot/notify.go` prints 0.
2. `grep -c "func Test" cmd/ot/notify_test.go` is at least 8, and `grep -c "func TestNotifyDryRunMarksNothing\|func TestNotifyMissingTokenNamesVariableOnly\|func TestNotifyErrorsHoldNoSecret\|func TestNotifyOnceExits\|func TestNotifyDrainsOnTerm\|func TestNotifyDoesNotStartTheSite\|func TestGapAdapterJoinsReaderAndLedger\|func TestNotifyNoSchedulerLock" cmd/ot/notify_test.go` prints 8.
3. `grep -c "t.Parallel()" cmd/ot/notify_test.go` is at least `grep -c "^func Test" cmd/ot/notify_test.go`; `grep -c "time.Sleep" cmd/ot/notify.go cmd/ot/notify_test.go cmd/ot/gapadapter.go` prints 0 for each.
4. `grep -c "internal/notify/core\|internal/notify/telegram\|internal/notify/dryrun" cmd/ot/notify.go cmd/ot/wire.go` is at least 3 in total, `grep -rc "internal/notify/notifytest" cmd/ot/notify.go cmd/ot/wire.go cmd/ot/gapadapter.go` prints 0 for each, and `go run ./scripts/arch --check` exits 0 with `git diff $(git merge-base HEAD origin/main) -- arch.toml .go-arch-lint.yml` empty.
5. `grep -c "ot notify" docs/contracts/ot.md` is at least 1; `grep -c "[0-9]\{6,\}:[A-Za-z0-9_-]\{20,\}" cmd/ot/notify.go cmd/ot/notify_test.go` prints 0 for each; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the six paths of the Files line.
6. The pull request description carries the waiver lines of the second opinion (added to this entry after the advisor's answer) and the owner's smoke step: `ot notify --dry-run --once`, then one real `ot notify --once`, run by the owner.

Checked by CI: ci-go; `go test ./cmd/ot -run 'TestNotify|TestGapAdapter|TestNewDepsFilled' -race -count=1`; the `arch.toml` check against `docs/design/modules.md`; the design verdict.

## Left out

- Message kinds other than the missed-job text: the morning report, the night rule, pushes, a "recovered" message when a gap clears, and anything built from `rd.expectation_r1` (the scheduler design leaves them to the later notify design; risk and alerts come after this one and add `Sender` calls, not a new sender).
- A narrower database role for `MarkGapReported` (Decision 5); a second chat or a second channel; inbound Telegram commands (no polling, no webhook: the process only sends).
- Putting `Notifier.Run` inside the scheduler process; a scheduler job for it (Decision 4).
- What is not verified. PR #358 is open and not merged: the field names of `store.Gap` and the existence of `OpenGaps` and `MarkGapReported` are taken from its text, and the exact signature of `MarkGapReported` and the return type of `OpenGaps` (open gaps only, or open and cleared) are not stated there; Decision 2 absorbs a different signature in the adapter, and the core drops a cleared gap itself. If #358 changes the names, the adapter and the `GapSource` method list are the only places that move. The library API names (`WithServerURL`, `WithHTTPClient`, `WithSkipGetMe`, `SendMessage`) were read in the v1.27.0 source from the module proxy, not compiled here.

## Open questions for the owner

1. The role for `MarkGapReported`: `ot_runner` (Decision 5, no new role) or a new narrow role? Default: runner.
2. Quiet hours 23:00 to 07:00 in the profile zone as the code default? Default: yes.
