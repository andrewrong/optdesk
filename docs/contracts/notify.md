# notify contract

## Responsibility

Says missed work once, as one plain-text message. The scheduler process calls `Notifier.NotifyGaps(ctx)` after each gap pass; notify reads the open gaps that have no `reported_at`, sends one text and marks the gaps it printed. notify is the one owner of `ops.job_gap.reported_at`; the pages never write it. It never decides that something is missed and never runs a job. Design: `docs/design/notify.md`.

## Inputs

- `notify.GapSource`: `OpenGaps(ctx)` (from `store.SchedulerLedger.Status`) and `MarkGapReported(ctx, g, at)` (from `store.SchedulerLedger.MarkGapReported`), joined by an adapter in `cmd/ot`. Gaps are `store.Gap`.
- `notify.Sender`: `Send(ctx, text)`; nil only when the service confirmed the delivery.
- `clockwork.Clock` and `notify.Config{QuietFrom, QuietTo, Zone}` (defaults `DefaultQuietFrom` 23:00 and `DefaultQuietTo` 07:00; the window may cross midnight).
- Secrets: `OT_TELEGRAM_TOKEN` and `OT_TELEGRAM_CHAT_ID` (`notify.TokenEnv`, `notify.ChatIDEnv`), two lines of `$OT_HOME/.env` read by the wiring only. They are in no code, fixture, log line or error text.

## Outputs

- `NotifyGaps` returns the number of gaps reported; each gap printed is marked with `MarkGapReported`, the only write notify can make. One pass makes one send attempt; there is no `Run`, loop or sleep.
- Constructors: `notify.NewNotifierFunc` (`internal/notify/core`), `notify.NewTelegramSenderFunc` (`internal/notify/telegram`), `notify.NewDryRunSenderFunc` (`internal/notify/dryrun`); fakes in `internal/notify/notifytest`.

## Invariants

- Delivery is at-least-once: the mark comes after a confirmed send, and only for the gaps printed in the text. A failed send marks nothing; a failed `MarkGapReported` after a confirmed send returns the error and is retried at the next pass, so a text may repeat only in that case.
- Gaps with `ClearedAt` or `ReportedAt` set are not said. Inside quiet hours nothing is sent and the rows stay unmarked.
- The text is built from the job or dataset id, the day and a fixed reason word; it carries no account id, path, token, chat id or error text.
- `ErrSend` is the error class of a failed delivery; its text never holds a token, a chat id or a URL.
- `internal/notify` holds only `api.go` and `api_test.go`; it depends on `store`, `clockwork` and the standard library.

## Market alerts

- `AlertNotifier.NotifyAlerts(ctx)` is a second pass beside `NotifyGaps`, built by `notify.NewAlertNotifierFunc(AlertDeps, Sender, clock, Config)` (`internal/notify/core.NewAlerts`). `AlertDeps{Rules, Evaluate, Read, Sent}` names `compute.EvaluateAlertsFunc`, `compute.ReadingFunc` and `store.SentStore`. `Notifier`, `Sender` and `Config` are unchanged.
- The quiet window is the one of `Config` (`QuietFrom`, `QuietTo`, `Zone`); there is no second setting. Inside it the pass returns 0 and reads nothing; a fired alert without a mark is said by the first pass after the window.
- Alert text (risk-alerts Decision 9): first line `optdesk: N market alert(s)`, then one line per alert sorted by level (`d`, `w`, `n`), day, rule id: `<D|W|N> <Title>: <Detail> (data of <DataDay>) <Day>`; at most 20 lines, then `and M more`; under 3500 bytes; plain text, one message per pass. No account id, DSN, path, token, chat id or error text.
- Public-subject rule: only results that are `fired` and `Public` (the rule's subject is in `public_subjects` of the rules) are sent; a non-public result is never sent.
- Mark-after-send rule: `SentStore.MarkSent(ctx, ruleID, day)` is called only after `Sender.Send` returned nil, and only for the alerts printed, never for one behind "and M more". A failed send marks nothing; a failed mark is retried at the next pass, the one case where an alert can repeat. Results with a mark in `alerts.sent` are not sent again.

## Version history

- v1: interface declared (stage 1).
- v2: alert notifier interface declared (additions only).
