# Risk and alerts: market alerts over stored figures, said once, held in quiet hours (stage 1); account risk (stage 2, described)

Author: the optdesk designer (revision 2, after the adversarial pass)
Owner approved: (pending)
Revision: 2

Source: owner decisions of 2026-11-08 (optdesk-1042), binding: (1) stage 1 is market-only alerts, rules over stored figures crossing thresholds; stage 2, in this design but not cut now, is positions and account risk over the landed account service; (2) delivery by Telegram through the existing notify `Sender` and by the `risks` block of `report_page` v2; (3) quiet hours 23:00 to 07:00 in the owner's zone, held alerts are sent in the morning; (4) thresholds are defaults in a TOML file in the repo that the owner edits later, none in Go. Earlier owner words that still hold: modules are designed interface first; a mature library wherever one exists; own glue code sits behind an interface so a library can replace it; the front end computes nothing.
Read on main at 2323d1e: `AGENTS.md`, `docs/design/modules.md`, `docs/design/notify.md` (and `internal/notify/api.go`, `internal/notify/core/quiet.go`), `docs/design/scheduler.md` (Decisions 12 and 13, `sched-jobs-schedule`), `docs/design/compute-figures.md` (Decisions 2, 5, 9), `docs/design/compute-core.md`, `docs/design/report-page.md`, `docs/design/pages-builders.md` (line 58), `docs/design/tracking-household-reports.md`, `docs/design/pyside-account-b.md`, `docs/adr/0010-reports-and-delivery.md`, `docs/conventions/register.toml` (the nine market figure ids), `internal/compute/rules/rules.json`, `contracts/schemas/page.report_page.v2.json` (`RisksBlock`, `RiskItem`, `Gap`), `internal/store/api.go` (`ReadingReader`, `Gap`, `GapReader`, DSN names), `internal/store/reading`, `internal/broker/grpcbroker/account.go`, `internal/profile/api.go`, `migrations/0013_readings.sql`.

## What main holds today

- Figures: the readings store is landed (`store.ReadingReader.LatestReadings(ctx, name, subjectKind, subject, from, to)`, `internal/store/reading`, role `ot_w_readings`, DSN `OT_PG_DSN_READINGS`). The producing job `compute_figures` is not on main; its design fixes the subjects: market figures have `subject_kind` `market` and the FRED id of the first input as subject, own-IV figures have `subject_kind` `underlying` and the symbol (`compute-figures.md` Decision 9). A missing figure is state `missing_input` with a null value (Decision 2). A reading row is dated the as-of (slot) day and carries the parameter row `data_day`, the day of its newest input (Decision 3); a FRED figure stays `ok` when its newest point is up to 8 calendar days old and "FRED lagged 4 sessions" is up to 6 calendar days (Decision 7, `register.toml:1112,1142`), so the row day is not the day of the data. The register holds `market.ivr_own`, `market.ivp_own`, `market.vix_pctile_3y`, `market.vxn_pctile_3y`, `market.rvx_pctile_3y`, `market.vix_term_ratio` (v1) and `market.sp500_vs_200d` v2; `internal/compute/rules/rules.json` holds the unit and `max_age_days` of each (4 for `ivr_own` and `ivp_own`, 8 for the others; `decimal` for ranks and percentiles, `ratio` for the term ratio and the 200-day figure). `market.ivr_own` and `market.ivp_own` need `min_valid` 252, so 252 valid sessions of the owner's own IV30 history (`TestFirstRunOnEmptyStores` of the compute design gives `short_history` before that).
- Notify (landed): `notify.Sender`, `notify.Notifier.NotifyGaps`, `notify.Config{QuietFrom, QuietTo, Zone}`, the quiet window as a pure function in `internal/notify/core/quiet.go` (package-private), the Telegram and dry-run senders. Its `## Left out` says risk and alerts add `Sender` calls, not a new sender. Quiet hours are a notify setting with the default 23:00 to 07:00 (notify Decision 7); the zone is `profile.Zone`, read from `OT_ZONE`. ADR 0010 says every time shown is "the owner's local time (ADR 0010)", so the owner's zone is Etc/GMT-8 (UTC+8). `cmd/ot/scheduler.go` and the notify wiring are not on main (`notify-wiring` and `sched-cmd-run` are open).
- Page: `report_page` v2 has the always-present block `risks` with `key`, `title`, `say`, `sub`, `data_times`, `state` (`ok|partial|gap`), `gaps`, `items` (at most 50 `RiskItem{kind, icon_key, level n|w|d, title, detail}` or a `Gap`), all eight required; `gap_text` defaults to a Chinese sentence. No Go builder or reader exists for `report_page` (`report-page.md` Decision 1: the view is fixtures only until a producer exists); the pages builders use fixed Chinese constants of the package (`pages-builders.md:58`).
- Account: `grpcbroker.AccountClient.Account(ctx)` exists on the Go side; `internal/broker/api.go` has no account method, there is no positions or account dataset, store or job (`pyside-account-b.md`).
- ADR 0010 (`docs/adr/0010-reports-and-delivery.md`): line 31 "Telegram never carries account-derived information (owner decision): no holdings, balances, percentages, sizes, or tickers chosen from holdings"; lines 53 to 54 "Alerts contain only a registered job id, status and the owner's local time (ADR 0010)". A market alert does not fit line 54 as written, so the ADR needs an amendment (Decision 10). The file belongs to module docs (`modules.md`: every ADR but 0034 belongs to docs).

## Decisions

1. What stage 1 is. The rules of a TOML file are a pure function over the stored readings: for each rule, the winner reading row of the newest day of the last 5 calendar days (Decision 7) gives `fired`, `clear` or `no_data`. The function is run in two places, the notify pass (which says each fired result once, outside the quiet window) and `risks_block.Build` (which makes the `risks` block). Nothing is stored but what was said: one insert-only table `alerts.sent(rule_id, day)`. There is no evaluate job, no slot, no alert table or view, no `Alert` record and no jobs interface. No rule reads a position, an account value or a broker; no order is placed anywhere in this design.
2. Dedupe. `alerts.sent` has the key `(rule_id, day)`, where `day` is the day of the reading row that fired. The pass says a fired result only when no row of that key exists, and inserts the row (`ON CONFLICT DO NOTHING`, time from the database's `now()`) after the sender returned nil. A rule whose condition still holds the next day has a new reading row with a new day and says itself again: once per rule per reading day is what the owner asked for (question 1). A later rev of the same reading row does not say it again. Because nothing is evaluated ahead of time, a reading that arrives late is simply seen by the next pass, and a pass that cannot read is retried at the next tick.
3. The shared function lives in module compute as the package `internal/compute/alertrules`, behind declarations in `internal/compute/api.go`, because notify and pages may import only an interface file and compute is where a pure function of readings belongs. Types, all plain values: `AlertRule{ID, Title, Figure string; FigureVersion int; SubjectKind, Subject, Unit, Op, Threshold, Level string; MaxDataAgeDays int}`, `AlertRules{Rules []AlertRule; PublicSubjects []string; Hash string}`, `AlertResult{RuleID, Title, Level, Figure, Value, Detail, Reason string; State string; Day, DataDay contracts.Day; Public bool}` (`State` is `fired`, `clear` or `no_data`; `Reason` is empty or `no_reading`, `missing_input`, `unit_mismatch`, `stale_data` or `no_data_day`); `ReadingFunc func(ctx, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error)` (a shape that the method value `store.ReadingReader.LatestReadings` fits and that a reader of the winner rule fits too, so compute does not import store; the function passed in must apply the winner rule of Decision 7); `EvaluateAlertsFunc func(ctx, AlertRules, ReadingFunc, today contracts.Day) ([]AlertResult, error)`; `LoadAlertRulesFunc func(io.Reader) (AlertRules, error)`; `DefaultAlertRulesFunc func() (AlertRules, error)`. The implementation (`alertrules.Evaluate`, `alertrules.Load`, `alertrules.DefaultRules`) is passed in by `cmd/ot`, so notify and pages hold the function as a value. `modules.md` rows `notify` and `pages` gain `compute` in their depends cell (see Modules).
4. Store: one table, one role. `internal/store/alert` implements `store.SentStore` with the role `ot_w_alerts`, DSN line `OT_PG_DSN_ALERTS` (`store.DSNEnvAlerts`), INSERT and SELECT on `alerts.sent` and nothing else (no UPDATE, DELETE or TRUNCATE, as `ot_w_readings`). Only the notify adapter in `cmd/ot` opens it. The pages read nothing from it. The migration number is the next free one at the cut (0015 on main today).
5. Rules are data. `internal/compute/alertrules/rules.toml` is embedded and holds `version = 1`, `public_subjects` (a list of strings, default `["SPY", "VIXCLS", "VXNCLS", "RVXCLS", "SP500"]`) and a list of `[[rule]]` tables with the keys `id`, `title` (at most 60 characters), `figure` (a register id), `figure_version`, `subject_kind`, `subject`, `unit` (a `reading_unit` word), `op` (`gt`, `ge`, `lt`, `le`), `threshold` (decimal text), `level` (`n`, `w`, `d`), `max_data_age_days` (Decision 7). The Go code holds no threshold, figure id, subject or text of a rule. Loading rejects an unknown key, a duplicate `id`, an `op` or `level` outside the set, a threshold that is not a decimal, a `title` over 60 characters, and a `max_data_age_days` below 0. A test checks the default rules against `docs/conventions/register.toml` (id and version exist) and `rules.json` (unit equal, `max_data_age_days` not above `max_age_days`); the loader cannot read them at run time. The owner may place `$OT_HOME/alerts.toml` in the same format: it replaces the whole file, a bad file is an error that names the key and the rule id and stops the start, and it is never replaced by the defaults silently. A default changed in the repo later does not reach an owner who has the override (the owner copies the new rule; `ot alerts check` lists the ids of both files so a difference is visible); this is named in Left out. Decision 6 gives the defaults.
6. Default rules, proposals that the owner confirms at the review of issue 4 (rule id: figure, subject, comparison, level, data age). `spy_ivr_high`: `market.ivr_own`, underlying SPY, `ge 0.80`, w, 4. `spy_ivr_low`: `le 0.10`, n, 4. `spy_ivp_high`: `market.ivp_own`, SPY, `ge 0.90`, w, 4. `spy_ivp_low`: `le 0.10`, n, 4. `vix_pctile_high`: `market.vix_pctile_3y`, market VIXCLS, `ge 0.90`, w, 6. `vix_pctile_extreme`: same, `ge 0.97`, d, 6. `vxn_pctile_high`: `market.vxn_pctile_3y`, market VXNCLS, `ge 0.90`, w, 6. `rvx_pctile_high`: `market.rvx_pctile_3y`, market RVXCLS, `ge 0.90`, w, 6. `vix_backwardation`: `market.vix_term_ratio`, market VIXCLS, `gt 1.0`, w, 6. `sp500_below_200d`: `market.sp500_vs_200d` version 2, market SP500, `lt 0`, w, 6. `sp500_far_below_200d`: same, `lt -0.05`, d, 6. Units are the `rules.json` units. The data age of 6 is the usual FRED lag (a holiday weekend gives 8 and the rule says `no_data` with `stale_data` rather than an old alert); 4 is the age `rules.json` gives the IV figures. The IV rank and percentile of SPY, the owner's headline ask, need 252 valid sessions of the owner's own IV30 history: until then the four SPY rules are `no_data` (`short_history` makes the reading `missing_input`), about a year after capture starts unless a backfill exists (there is none in this design). Meanwhile the seven rules on the VIX family and the 200-day figure carry stage 1, and the page shows `partial` with four gaps every day. Raw VIX or VXN levels are not rules: they are series, not readings.
7. Evaluation, pure. For each rule, `read` is called with the rule's figure id, subject kind and subject for the days `today - 5` to `today`, and the newest row it returns is used. The reader passed in applies the winner rule of the report page, the semantic of the `rd` view of migration 0013: per day, a `missing_input` row shows only when the day has no `ok` row, and the highest rev of state `ok` wins. So the notify pass and the report page say the same thing for the same day (the owner's page and the Telegram alert never differ because one read the newest rev and the other the winner). Evaluation does not choose a winner itself: it takes the rows as the reader gives them. The reader that exposes the winner rule with the parameter `data_day` (`WinnerReadings`, over a new view `rd.reading_parameter_r1` granted to `ot_ro_pages`) is delivered by the separate design "report page producer", not by this one; this design states that its rules take a `ReadingFunc`-shaped reader, and `alerts-wire` is not cut by this design: it is cut only after the winner reader (`WinnerReadings` over `rd.reading_parameter_r1`) of `docs/design/report-page-producer.md` has landed, together with `notify-wiring` and `sched-cmd-run`; until then nothing wires the alerts pass, so the page and the alert never differ. The result is `no_data` when there is no row (`no_reading`), when the row is `missing_input` (`missing_input`), when its unit differs from the rule's (`unit_mismatch`; counted and logged by rule id, since it means the rule file is wrong), when the parameter row `data_day` is absent or not a day (`no_data_day`), or when the row day minus `data_day` is more than the rule's `max_data_age_days` calendar days (`stale_data`). Only the row day is not enough: the row of the slot day exists even when its newest FRED point is days old, so the check reads `data_day` (the stored form is a day text in `value_tag`; the compute design names the parameter and not its form, see Left out, "What is not verified"). Otherwise the result is `fired` when the comparison holds (exact comparison of decimal text with `math/big.Rat`, no float) and `clear` when it does not. `Day` is the row day and `DataDay` the data day. `Detail` is built by the code from fixed words and the stored value: `<figure id> <subject> <value> <op word> <threshold>`, never from free text of the file. `Public` is true when the rule's subject is in `public_subjects`. A reader error is an error of the call, not a `no_data`. With 11 rules the pass makes 11 reads every 15 minutes, which is trivial; the history of what was evaluated is not kept, the readings keep their own revs.
8. Delivery through notify, no new sender. `internal/notify/api.go` gains `AlertNotifier` (`NotifyAlerts(ctx) (int, error)`), `AlertDeps{Rules compute.AlertRules; Evaluate compute.EvaluateAlertsFunc; Read compute.ReadingFunc; Sent store.SentStore}` and `NewAlertNotifierFunc(AlertDeps, Sender, clockwork.Clock, Config) (AlertNotifier, error)`, implemented in `internal/notify/core` (new files `alerts.go` and `alerts_text.go`, same package as `quiet.go`, so the same quiet window); `Notifier` and `NewNotifierFunc` are not changed. The pass follows notify Decision 6: inside the quiet window return 0 and read nothing; otherwise take `today` as the date of the clock in `Config.Zone`, evaluate, keep the results that are `fired` and `Public`, drop those with a row in `alerts.sent`, and send one text of at most 20 alerts; only after the sender returned nil `MarkSent` for each alert printed, never for one behind "and M more"; a failed send marks nothing; a failed mark is retried at the next pass (at-least-once, the one case where an alert can repeat). The backoff after a failed send is the same 15, 30, 60 minutes on the injected clock, in the alert notifier's own state. The scheduler calls `NotifyAlerts` after `NotifyGaps` (issue 10); an error is logged by class and never stops the ticker.
9. The text: first line `optdesk: N market alert(s)`, then one line per alert sorted by level (`d` before `w` before `n`), then day, then rule id: `<D|W|N> <Title>: <Detail> (data of <DataDay>) <Day>`, so the age of the figure is on the line; at most 20 lines then `and M more`; under 3500 bytes; plain text, one message per pass. It carries the rule title, the public figure and its value, no account id, DSN, path, token, chat id or error text. A result whose subject is not in `public_subjects` is never sent (it still appears on the page). It is a market series or a public index symbol and never anything derived from the account: the guard is the list, and the list is the owner's file, so the owner decides what is public.
10. ADR 0010 and the Telegram content. Choice (a), written as the proposal: `docs/adr/0010-reports-and-delivery.md` gets an amendment (a section `## Amendment (<merge date>)` appended after the last amendment; lines 31 and 53 to 54 stay as they are) saying that for market alerts only the Telegram text may hold the rule title, the public market figure and its value and the day of its data, and never anything derived from holdings or the account; the account-derived ban of line 31 and the job-id rule of lines 53 to 54 for job alerts are unchanged. The amendment is its own small issue (9), in module docs, and the notify text issue (7) lands after it. If the owner chooses (b), the text is only the rule id, the level and the the owner's local time (ADR 0010), `<D|W|N> <rule id> <YYYY-MM-DD HH:MM>`, with the details on the report page; the ADR then needs no amendment, issue 9 is dropped, and Decision 9 and issue 7 change to that text (the page block is the same). The owner is being asked (question 2).
11. Quiet hours and the zone. The quiet window is the notify setting that exists: `Config.QuietFrom` and `QuietTo`, defaults `23:00` and `07:00`, in `Config.Zone`. This design adds no second setting. Nothing is lost in the window: a fired result that was not said has no row in `alerts.sent`, so the first pass at or after 07:00 says it. The owner's zone is Etc/GMT-8 by ADR 0010; `OT_ZONE=Etc/GMT-8` in the owner's `.env` is the owner's line to set (question 3). There is no per-level exception: a `d` alert is held too, as the owner decided.
12. Stage 2 (described, not cut): positions and account risk. It needs, in this order: contracts datasets for positions and account values (`account.position` r1, `account.value` r1, built on `grpcbroker.Account`: alias never an account id, decimal text); `broker` gains an account method (today only the concrete `grpcbroker.AccountClient` has one); a store with its own role and DSN (`ot_w_account`); an `account_capture` job; compute figures for net delta, concentration and margin use (`risk.net_delta`, `risk.concentration`, `risk.margin_use`, register entries first, IV and Greeks only from IBKR model values as `docs/engineering.md` says); rules in the same TOML format with a `scope = "account"` key (added then, default `market`); and readers and builders for `household_risk` and `positions`. Account results are never `Public`: stage 2 adds the `scope` key and the notify pass sends only market scope, so an account rule can reach the page and not Telegram. Whether Telegram says anything about account risk is the owner's (question 4; default page-only). Stage 1 builds none of this.
13. The risks block. `internal/pages/risks_block` is a builder shaped like `macro_page.Build`: `risks_block.Build(ctx, in Input) ([]byte, error)` with `Input{Rules compute.AlertRules; Evaluate compute.EvaluateAlertsFunc; Read compute.ReadingFunc (the winner reader of Decision 7); Today contracts.Day; GeneratedAt time.Time}` returns the JSON of `RisksBlock` of `contracts/schemas/page.report_page.v2.json`: a block, not a page. It is not visible anywhere until a `report_page` producer exists; that producer is the next step and is listed in Left out with what it needs. Fields: `key` `risks`; `title`, `sub`, `say`, the gap texts and `data_times` are fixed Chinese constants of the package, the language of the pages (`pages-builders.md:58`, wording advisor-set, not an owner decision): `title` `风险`, `sub` `市场提醒来自已存储的读数，每条规则每天一次。`, `say` `<day> 有 N 条市场提醒。` or `<day> 没有市场提醒。`, a gap text `<rule title> 暂缺`. `items` is one `RiskItem` per `fired` result (`kind` `market_alert`, `icon_key` `volatility` or `trend` from a table keyed by figure id in the builder's own file, `level` the result's, `title` the rule's title, `detail` the code's `Detail` with the data day); `state` is `ok` when no result is `no_data`, `partial` when some are (with a `Gap` per such rule, `reason` `missing_input` for `missing_input` and `no_data_day`, `stale_input` for `stale_data`, `no_data` for `no_reading` and `unit_mismatch`, and the other items), `gap` when all are (`items` is a `Gap` of reason `no_data`); `gaps` is always present, empty for `ok`. `data_times` names the newest reading day and the data day of each fired item in the owner's local time (ADR 0010). The front end renders the strings and computes nothing. The test validates every output against `#/$defs/RisksBlock` with `santhosh-tekuri/jsonschema/v6` (already in `go.mod`), so a missing required field fails it.
14. Libraries (AGENTS.md "Libraries"). TOML: `github.com/pelletier/go-toml/v2` (already direct). Exact comparison: the standard library `math/big`. Time: `github.com/jonboulle/clockwork` (already direct). Schema check in tests: `santhosh-tekuri/jsonschema/v6` (already direct). Telegram: the existing sender. No new direct dependency, no `go.mod` edit. Own implementation, because no mainstream library does a threshold comparison on a stored row: the evaluation is a few dozen lines behind the `compute` interface (the owner's rule of 2026-11-08, own glue behind an interface), so a library can replace it.
15. Tests: `t.Parallel()`, `t.TempDir()` for any file, a fake clock (`clockwork.NewFakeClock`), no sleep, fakes of `compute.ReadingFunc`, `store.SentStore` and `notify.Sender`, no live Telegram, no live broker, no fixed port or file. A test that needs PostgreSQL carries the build tag `pg` and runs only in GitHub CI.

## Second opinion

A new database role (`ot_w_alerts`) and a text that goes to a phone: a second-opinion case, as `store-reading-tables` is. The brief goes to the advisor after the design review: (a) the role has one table and no UPDATE, and the mark is written after the send; (b) the text is built from fixed words, a public figure and the `public_subjects` list, and an account figure is never public (stage 1 has no account rule); (c) the override file cannot silently fall back to the defaults; (d) at-least-once delivery, mark after send. The waiver lines go into issues 5 (alerts-store-migration), 6 (alerts-store-impl) and 7 (alerts-notify-core), the three entries marked second-opinion case, when the advisor has answered; not into entry 10 (alerts-wire), which is wiring only.

## Modules

New directories: `internal/compute/alertrules` (rules and evaluation), `internal/store/alert` (the sent table), `internal/pages/risks_block` (block builder). This pull request edits `docs/design/modules.md` by one list, two depends cells and one rewritten sentence of the bullet 'Modules not touched in week 1' (it names notify and pages as added by this design and says compute's packages are on main), and nothing else: the sentence that lists the implementation packages `cmd/ot` may import gains `internal/compute/alertrules`, `internal/store/alert` and `internal/pages/risks_block`; the cells of `notify` and `pages` gain `compute` (they use the declarations of `internal/compute/api.go`, as `jobs` does; `compute` itself depends on `contracts` only, so no cycle). It carries the regeneration of `arch.toml` and, because the two depends cells change the edges of existing components, of `.go-arch-lint.yml` by `go run ./scripts/arch` (`go run ./scripts/arch --check` prints `arch: ok`, which is what the CI arch check compares); the new components are not in it yet because their directories are absent. No path row changes: `compute`, `store`, `notify`, `pages`, `docs` and `ot` already cover every path below. The `.go-arch-lint.yml` (module repo) is regenerated again, only to add the new components, by the entries that add a directory (1, 2 and 8: judged two-module entries, as in `compute-figures.md`), serialized by their Order lines because they share the file; `internal/store/setup/setup.go` and `migrations/**` belong to store, `cmd/**` and `docs/contracts/ot.md` to ot, `docs/adr/0010-reports-and-delivery.md` to docs. No `go.mod` or `go.sum` change.

| # | issue | module | size | order |
|---|---|---|---|---|
| 1 | alerts-compute-interface (interface) | compute, plus repo (judged two-module entry: `.go-arch-lint.yml`) | S | first |
| 2 | alerts-store-interface (interface) | store, plus repo (judged two-module entry: `.go-arch-lint.yml`) | S | after 1 |
| 3 | alerts-notify-interface (interface) | notify | S | after 1, 2 |
| 4 | alerts-rules | compute | M | after 1 |
| 5 | alerts-store-migration | store; second-opinion case | M | after 2 |
| 6 | alerts-store-impl | store; second-opinion case | M | after 2, 5 |
| 7 | alerts-notify-core | notify; second-opinion case | M | after 1, 2, 3, 9 |
| 8 | alerts-risks-block | pages, plus repo (judged two-module entry: `.go-arch-lint.yml`) | M | after 1, 2 |
| 9 | alerts-adr-0010 | docs | S | first |
| 10 | alerts-wire (not cut by this design) | ot | M | not cut: only after the winner reader of `docs/design/report-page-producer.md` has landed, together with `notify-wiring` and `sched-cmd-run` |

The numbers are the entries of `## Issues` in this file; a name without a number is an issue of another design. The file `.go-arch-lint.yml` is in 1, 2 and 8 (2 follows 1, 8 follows 2). Entries 3, 4, 5, 8 and 9 can run in parallel once their Order lines are met. `internal/compute/api.go` is in 1 only, `internal/store/api.go` in 2 only, `internal/notify/api.go` in 3 only.

## Interface

- compute (`internal/compute/api.go`, owner compute): `AlertRule`, `AlertRules`, `AlertResult`, `ReadingFunc`, `EvaluateAlertsFunc`, `LoadAlertRulesFunc`, `DefaultAlertRulesFunc` (Decision 3). Additions only.
- store (`internal/store/api.go`, owner store): `SentMark{RuleID string; Day contracts.Day}`; `SentStore`: `SentSince(ctx, from contracts.Day) ([]SentMark, error)` (rows with `day >= from`, ascending by day then rule id), `MarkSent(ctx, ruleID string, day contracts.Day) error` (insert `ON CONFLICT DO NOTHING`, time from the database's `now()`) and `Close() error`; `DSNEnvAlerts = "OT_PG_DSN_ALERTS"`; `OpenSentFunc func(ctx, dsn) (SentStore, error)` (implementation `internal/store/alert.Open`).
- notify (`internal/notify/api.go`, owner notify): `AlertDeps`, `AlertNotifier`, `NewAlertNotifierFunc` (Decision 8); `Notifier`, `Sender` and `Config` are unchanged.
- pages: no interface change. `risks_block.Build` is a plain function like `macro_page.Build`; `internal/pages/api.go` is not edited.

## Issues

### alerts-compute-interface (interface)

- Module: compute and repo (judged two-module entry: `.go-arch-lint.yml` regenerated by `go run ./scripts/arch`). Size: S. Depends on: none.
- Order: first.
- Design: docs/design/risk-alerts.md#Interface
- Files: internal/compute/api.go, internal/compute/api_test.go, internal/compute/alertrules/doc.go, docs/contracts/compute.md, .go-arch-lint.yml

- Goal: the compute declarations of the Interface section, declarations only, the stub `internal/compute/alertrules/doc.go` (a package clause and a comment), `.go-arch-lint.yml` regenerated for the component `compute/alertrules`, a contract test from both sides (a fake `EvaluateAlertsFunc` and a caller) and `docs/contracts/compute.md` recording the states, the reasons, the data-day rule and the 5-day window.

Acceptance:

1. `grep -c "type AlertRule struct\|type AlertRules struct\|type AlertResult struct\|type ReadingFunc \|type EvaluateAlertsFunc \|type LoadAlertRulesFunc \|type DefaultAlertRulesFunc " internal/compute/api.go` prints 7; `git diff $(git merge-base HEAD origin/main) -- internal/compute/api.go | grep -c '^-[^-]'` prints 0 (additions only).
2. `grep -c "var _ compute.EvaluateAlertsFunc\|var _ compute.LoadAlertRulesFunc" internal/compute/api_test.go` prints 2; `go test ./internal/compute -run 'TestAlert' -race -count=1` passes.
3. `grep -c "stale_data\|no_data_day\|data_day" docs/contracts/compute.md` is at least 3.
4. `grep -c "compute/alertrules" .go-arch-lint.yml` is at least 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the five paths of the Files line.

Checked by CI: ci-go (`go test ./...`, go-arch-lint, golangci-lint); the `arch.toml` check and `go run ./scripts/arch --check` printing `arch: ok`; the design verdict (the result matches the Interface section).

### alerts-store-interface (interface)

- Module: store and repo (judged two-module entry: `.go-arch-lint.yml` regenerated by `go run ./scripts/arch`). Size: S. Depends on: alerts-compute-interface (the shared `.go-arch-lint.yml` only).
- Order: after 1 (alerts-compute-interface).
- Design: docs/design/risk-alerts.md#Interface
- Files: internal/store/api.go, internal/store/api_test.go, internal/store/alert/doc.go, docs/contracts/store.md, .go-arch-lint.yml

- Goal: `SentMark`, `SentStore`, `DSNEnvAlerts` and `OpenSentFunc`, declarations only, the stub `internal/store/alert/doc.go`, `.go-arch-lint.yml` regenerated for the component `store/alert`, a contract test from both sides (a fake `SentStore` and a caller) and `docs/contracts/store.md` recording the table `alerts.sent`, its key `(rule_id, day)` and the mark-after-send rule.

Acceptance:

1. `grep -c "type SentMark struct\|type SentStore interface\|type OpenSentFunc " internal/store/api.go` prints 3; `grep -c "DSNEnvAlerts = \"OT_PG_DSN_ALERTS\"" internal/store/api.go` prints 1; `git diff $(git merge-base HEAD origin/main) -- internal/store/api.go | grep -c '^-[^-]'` prints 0.
2. `grep -c "var _ store.OpenSentFunc" internal/store/api_test.go` prints 1; `go test ./internal/store -run 'TestSent' -race -count=1` passes.
3. `grep -c "alerts.sent\|MarkSent" docs/contracts/store.md` is at least 2.
4. `grep -c "store/alert" .go-arch-lint.yml` is at least 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the five paths of the Files line.

Checked by CI: ci-go; the `arch.toml` check and `go run ./scripts/arch --check` printing `arch: ok`; the design verdict.

### alerts-notify-interface (interface)

- Module: notify. Size: S. Depends on: alerts-compute-interface, alerts-store-interface (`AlertDeps` names `compute.EvaluateAlertsFunc` and `store.SentStore`).
- Order: after 1, 2 (alerts-compute-interface, alerts-store-interface).
- Design: docs/design/risk-alerts.md#Interface (an interface change of `internal/notify/api.go`: additions only)
- Files: internal/notify/api.go, internal/notify/api_test.go, docs/contracts/notify.md

- Goal: `AlertDeps`, `AlertNotifier` and `NewAlertNotifierFunc`, declarations only, with a contract test from both sides (a fake `Sender`, a fake `SentStore` and a caller) and `docs/contracts/notify.md` recording the alert text (Decision 9), the public-subject rule, the mark-after-send rule and that the quiet window is the one of `Config`.

Acceptance:

1. `grep -c "type AlertDeps struct\|type AlertNotifier interface\|type NewAlertNotifierFunc " internal/notify/api.go` prints 3; `git diff $(git merge-base HEAD origin/main) -- internal/notify/api.go | grep -c '^-[^-]'` prints 0.
2. `grep -c "var _ notify.NewAlertNotifierFunc" internal/notify/api_test.go` prints 1; `go test ./internal/notify -run 'TestAlert' -race -count=1` passes.
3. `grep -c "MarkSent\|public_subjects\|market alert" docs/contracts/notify.md` is at least 3; `grep -c "[0-9]\{6,\}:[A-Za-z0-9_-]\{20,\}" internal/notify/api.go internal/notify/api_test.go docs/contracts/notify.md` prints 0 for each file.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

Checked by CI: ci-go; `go test ./internal/notify -race -count=1`; the design verdict.

### alerts-rules

- Module: compute. Size: M. Depends on: alerts-compute-interface.
- Order: after 1 (alerts-compute-interface).
- Design: docs/design/risk-alerts.md#Decisions (Decisions 5, 6, 7, 14; library: `github.com/pelletier/go-toml/v2`, already a direct dependency; own implementation of the comparison on `math/big`, because no mainstream library fits)
- Files: internal/compute/alertrules/rules.toml, internal/compute/alertrules/rules.go, internal/compute/alertrules/eval.go, internal/compute/alertrules/rules_test.go, internal/compute/alertrules/eval_test.go

- Goal: `alertrules.Load` is a `compute.LoadAlertRulesFunc`, `alertrules.DefaultRules` a `compute.DefaultAlertRulesFunc` and `alertrules.Evaluate` a `compute.EvaluateAlertsFunc`: strict TOML decoding (`DisallowUnknownFields`) with the checks of Decision 5, the embedded `rules.toml` holding the eleven default rules of Decision 6 and `public_subjects`, the rule-file hash (sha256 of the bytes), and the evaluation of Decision 7 (the five reasons, the data-day check, the fixed-word `Detail`, the title limit, `Public`). Errors name the key and the rule id and never a value.

Acceptance:

1. `grep -c "^\[\[rule\]\]" internal/compute/alertrules/rules.toml` prints 11; `grep -c "^id = " internal/compute/alertrules/rules.toml` prints 11; `grep -c "^public_subjects" internal/compute/alertrules/rules.toml` prints 1.
2. `grep -rhE "^func (TestDefaultRulesLoad|TestDefaultRulesMatchRegister|TestDefaultRulesMatchRulesJson|TestLoadRejectsUnknownKey|TestLoadRejectsDuplicateID|TestLoadRejectsBadOp|TestLoadRejectsBadLevel|TestLoadRejectsBadThreshold|TestLoadRejectsLongTitle|TestEvalFires|TestEvalClear|TestEvalBoundary|TestEvalMissingInputNoData|TestEvalUnitMismatchNoData|TestEvalOldRowNoData|TestEvalStaleDataDayNoData|TestEvalMissingDataDayNoData|TestEvalLateFredStillFreshEnough|TestEvalDetailFixedWords|TestEvalExactDecimal|TestEvalPublicSubject|TestEvalReaderErrorIsError)\(" internal/compute/alertrules | wc -l` prints 22; `go test ./internal/compute/alertrules -race -count=1` passes.
3. `grep -c "t.Parallel()" internal/compute/alertrules/rules_test.go internal/compute/alertrules/eval_test.go` is at least the `grep -c "^func Test"` of each file; `grep -c "time.Sleep\|time.Now()\|float64\|ParseFloat\|\"os\"\|net/http" internal/compute/alertrules/rules.go internal/compute/alertrules/eval.go` prints 0 for each file.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the five paths of the Files line; `git diff $(git merge-base HEAD origin/main) -- go.mod go.sum internal/compute/api.go | grep -c .` prints 0.

Pull request description (a requirement, not an acceptance item): it states that the thresholds and data ages of Decision 6 are proposals the owner confirms.

Checked by CI: ci-go (`go test ./internal/compute/... -race -count=1`); the size rule.

### alerts-store-migration

- Module: store. Size: M. Depends on: alerts-store-interface.
- Order: after 2 (alerts-store-interface).
- Design: docs/design/risk-alerts.md#Decisions (Decision 4; second-opinion case)
- Files: one new file `migrations/<next free number>_alerts.sql` (the number is taken at the cut; 0015 on main today, and another design may take it first), internal/store/setup/setup.go, internal/store/setup/setup_test.go, internal/store/setup/alerts_pg_test.go

- Goal: schema `alerts` with the one table `alerts.sent(rule_id text, day date, sent_at timestamptz, PK(rule_id, day))`, the role `ot_w_alerts` (INSERT and SELECT on that table, nothing else, no UPDATE, DELETE or TRUNCATE) made by `setup` with the DSN line `OT_PG_DSN_ALERTS`.

Acceptance:

1. `M=$(git diff --name-only --diff-filter=A $(git merge-base HEAD origin/main) -- migrations/)` is exactly one path; `grep -c "CREATE TABLE alerts.sent" $M` prints 1; `grep -v '^--' $M | grep -ci "UPDATE\|DELETE\|TRUNCATE"` prints 0.
2. `grep -c "ot_w_alerts" internal/store/setup/setup.go` is at least 2; `grep -c "OT_PG_DSN_ALERTS\|DSNEnvAlerts" internal/store/setup/setup.go` is at least 1; `go test ./internal/store/setup -run 'TestRoles|TestAlertRole' -race -count=1` passes.
3. `grep -rc "//go:build pg" internal/store/setup/alerts_pg_test.go` prints 1.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists the three `internal/store/setup` paths of the Files line and the one added path under `migrations/`, and no other.

Checked by CI: ci-go; the `pg` job in GitHub CI (`TestAlertMigration` in `alerts_pg_test.go`: the migration applies on a clean database and the role has the grants above and no more); the design verdict.

### alerts-store-impl

- Module: store. Size: M. Depends on: alerts-store-interface, alerts-store-migration.
- Order: after 2, 5 (alerts-store-interface, alerts-store-migration).
- Design: docs/design/risk-alerts.md#Interface (own code is the pgx access only, as `internal/store/reading`; second-opinion case, waiver lines added after the advisor answers)
- Files: internal/store/alert/store.go, internal/store/alert/store_test.go, internal/store/alert/store_pg_test.go

- Goal: `alert.Open` is a `store.OpenSentFunc` over pgx as the other store packages: `SentSince` reads `alerts.sent`, `MarkSent` inserts `ON CONFLICT DO NOTHING` with the database's `now()`. Unit tests (no database) cover the row mapping and the argument checks.

Acceptance:

1. `grep -c "var _ store.OpenSentFunc" internal/store/alert/store_test.go` prints 1; `grep -c "ON CONFLICT DO NOTHING" internal/store/alert/store.go` is at least 1; `grep -c "//go:build pg" internal/store/alert/store_pg_test.go` prints 1.
2. `grep -rhE "^func (TestSentSinceOrdersByDay|TestMarkSentIdempotent|TestMarkSentRejectsEmptyRule)\(" internal/store/alert | wc -l` prints 3; `go test ./internal/store/alert -race -count=1` passes.
3. `grep -c "t.Parallel()" internal/store/alert/store_test.go` is at least `grep -c "^func Test" internal/store/alert/store_test.go`; `grep -c "UPDATE \|DELETE \|time.Sleep" internal/store/alert/store.go` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

Checked by CI: ci-go; the `pg` job (`TestMarkSentIdempotent` against a database is the SQL semantics: a second mark of the same key changes nothing); the size rule.

### alerts-notify-core

- Module: notify. Size: M. Depends on: alerts-compute-interface, alerts-store-interface, alerts-notify-interface, alerts-adr-0010.
- Order: after 1, 2, 3, 9 (alerts-compute-interface, alerts-store-interface, alerts-notify-interface, alerts-adr-0010).
- Design: docs/design/risk-alerts.md#Decisions (Decisions 8, 9, 10, 11, 15; second-opinion case, waiver lines added after the advisor answers)
- Files: internal/notify/core/alerts.go, internal/notify/core/alerts_text.go, internal/notify/core/alerts_test.go, internal/notify/notifytest/notifytest.go

- Goal: `core.NewAlerts` is a `notify.NewAlertNotifierFunc`: the pass of Decision 8 on the quiet window of `quiet.go` (the same package, no copy), the text of Decision 9 as a pure function of `[]compute.AlertResult`, the public-subject guard, and the in-memory backoff on the injected clock. `notifytest` gains a fake `SentStore` (rows in memory, records marks, can fail on demand). `core.go`, `text.go` and `quiet.go` are not edited. If the owner chooses (b) of Decision 10 the text is the one given there.

Acceptance:

1. `grep -rhE "^func (TestAlertMarkOnlyAfterSend|TestAlertFailedSendMarksNothing|TestAlertOneMessagePerBatch|TestAlertSentNotRepeated|TestAlertQuietHoursHold|TestAlertHeldSentAtSeven|TestAlertQuietWindowCrossesMidnight|TestAlertTextHoldsNoSentinel|TestAlertTextCapsLines|TestAlertTextOrder|TestAlertTextShowsDataDay|TestAlertBackoffDoubles|TestAlertMarkFailureRetried|TestAlertCapMarksOnlyPrinted|TestAlertNonPublicSubjectNeverSent|TestAlertNoDataNeverSent)\(" internal/notify/core | wc -l` prints 16; `go test ./internal/notify/... -race -count=1` passes.
2. `go test ./internal/notify/core -run 'TestAlertHeldSentAtSeven|TestAlertQuietHoursHold' -race -count=1 -v | grep -c '^--- PASS'` prints 2; `grep -c "Etc/GMT-8" internal/notify/core/alerts_test.go` is at least 1.
3. `grep -c "t.Parallel()" internal/notify/core/alerts_test.go` is at least `grep -c "^func Test" internal/notify/core/alerts_test.go`; `grep -c "time.Sleep\|time.Now()\|net/http\|os.Getenv\|telegram" internal/notify/core/alerts.go internal/notify/core/alerts_text.go` prints 0 for each file; `grep -c "clockwork.NewFakeClock" internal/notify/core/alerts_test.go` is at least 1.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the four paths of the Files line; `git diff $(git merge-base HEAD origin/main) -- internal/notify/core/core.go internal/notify/core/text.go internal/notify/core/quiet.go internal/notify/api.go | grep -c .` prints 0.

Checked by CI: ci-go; the size rule.

### alerts-risks-block

- Module: pages and repo (judged two-module entry: `.go-arch-lint.yml` regenerated by `go run ./scripts/arch`). Size: M. Depends on: alerts-compute-interface, alerts-store-interface (for the shared `.go-arch-lint.yml` only).
- Order: after 1, 2 (alerts-compute-interface, alerts-store-interface).
- Design: docs/design/risk-alerts.md#Decisions (Decision 13; the shape and constants of `docs/design/pages-builders.md`)
- Files: internal/pages/risks_block/builder.go, internal/pages/risks_block/strings.go, internal/pages/risks_block/builder_test.go, internal/pages/risks_block/testdata/ (the expected JSON of each case, new), .go-arch-lint.yml

- Goal: `risks_block.Build(ctx, Input) ([]byte, error)` returns the JSON of `RisksBlock`, the fields, states and items of Decision 13, with the fixed Chinese constants in `strings.go`, and every time passed in (the builder reads no clock). The block is not visible until a `report_page` producer exists (Left out). `.go-arch-lint.yml` is regenerated for the new directory.

Acceptance:

1. `grep -rhE "^func (TestBlockOkWithItems|TestBlockOkNoAlert|TestBlockPartialNoData|TestBlockGapAllNoData|TestBlockStaleDataIsStaleInput|TestBlockLevelsCarried|TestBlockRequiredFieldsPresent|TestBlockMatchesSchema|TestBlockReaderErrorIsError)\(" internal/pages/risks_block | wc -l` prints 9; `go test ./internal/pages/risks_block -race -count=1` passes (`TestBlockMatchesSchema` validates every case against `#/$defs/RisksBlock` of `contracts/schemas/page.report_page.v2.json`).
2. `grep -c "t.Parallel()" internal/pages/risks_block/builder_test.go` is at least `grep -c "^func Test" internal/pages/risks_block/builder_test.go`; `grep -c "time.Now()\|time.Sleep\|float64" internal/pages/risks_block/builder.go internal/pages/risks_block/strings.go` prints 0 for each file.
3. `grep -rc "internal/store/alert\|internal/notify\|internal/jobs\|internal/compute/alertrules" internal/pages/risks_block | grep -v ':0$'` prints nothing; `grep -c "pages/risks_block" .go-arch-lint.yml` is at least 1.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists only paths of the Files line or files under `internal/pages/risks_block/testdata/`; `git diff $(git merge-base HEAD origin/main) -- contracts web internal/pages/api.go | grep -c .` prints 0. The pull request carries `Contracts: unchanged`.

Checked by CI: ci-go (go-arch-lint, golangci-lint, `go run ./scripts/arch --check` printing `arch: ok`); the size rule.

### alerts-adr-0010

- Module: docs. Size: S. Depends on: none.
- Order: first.
- Design: docs/design/risk-alerts.md#Decisions (Decision 10, choice (a); dropped if the owner chooses (b))
- Files: docs/adr/0010-reports-and-delivery.md

- Goal: a dated section `## Amendment (<date>)` appended after the last amendment of `docs/adr/0010-reports-and-delivery.md`, owner decision recorded as such, saying: for market alerts only, the Telegram text may hold the rule title, the public market figure and its value and the day of its data; it never holds anything derived from holdings or the account; line 31 (account-derived ban) and lines 53 to 54 (job alerts: job id, status, the owner's local time (ADR 0010)) stand for what they cover. No existing line is changed.

Acceptance:

1. `git diff $(git merge-base HEAD origin/main) -- docs/adr/0010-reports-and-delivery.md | grep -c '^-[^-]'` prints 0; `git diff $(git merge-base HEAD origin/main) -- docs/adr/0010-reports-and-delivery.md | grep -c '^+## Amendment'` prints 1 (one section is added).
2. `git diff $(git merge-base HEAD origin/main) -- docs/adr/0010-reports-and-delivery.md | grep '^+' | grep -c "market alert"` is at least 1; `grep -c "Telegram never carries account-derived information" docs/adr/0010-reports-and-delivery.md` prints 1.
3. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the one path of the Files line.

Checked by CI: the design verdict (the amendment says what Decision 10 says); the owner's word, which the pull request description cites.

### alerts-wire

Not cut by this design: `alerts-wire` is not cut by this design: it is cut only after the winner reader (`WinnerReadings` over `rd.reading_parameter_r1`) of `docs/design/report-page-producer.md` has landed, together with `notify-wiring` and `sched-cmd-run`; until then nothing wires the alerts pass. The text below is the intent for that later cut, not an issue to open from this design.

- Module: ot. Size: M. Depends on: not cut by this design; see the note under the heading.
- Order: not cut by this design; see the note under the heading. When cut, it edits the files that `notify-wiring` and `sched-cmd-run` create or change (`cmd/ot/scheduler.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `docs/contracts/ot.md`).
- Design: docs/design/risk-alerts.md#Decisions (Decisions 4, 5, 7, 8)
- Files: cmd/ot/scheduler.go, cmd/ot/scheduler_test.go, cmd/ot/alerts.go, cmd/ot/alerts_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, docs/contracts/ot.md

- Goal: wiring only. The scheduler process reads `OT_PG_DSN_ALERTS` through `deps.readEnv`, opens one `store.SentStore` with `alert.Open`, loads the rules (`alertrules.Load` over `$OT_HOME/alerts.toml` when the file exists, `alertrules.DefaultRules` otherwise; a bad file stops the start with an error naming the key and rule id), builds `core.NewAlerts` with the `Sender` and `Config` that `notify-wiring` builds, `alertrules.Evaluate` and a `compute.ReadingFunc` that is the winner reader `WinnerReadings` over `rd.reading_parameter_r1` (never `LatestReadings`), so the alert reads the rows the page reads, and calls `NotifyAlerts` after `NotifyGaps` in the same wrapper; its error is logged by class and never stops the ticker. `cmd/ot/alerts.go` is also `ot alerts check [--file path]`: loads a rule file and prints the rule count and each rule id, opens no database. No resident process, keeper entry or health change. Flags and errors hold no token, chat id or DSN. The `ot page` wiring of the risks block is not here (Left out).

Acceptance:

1. `grep -c 'register("alerts"' cmd/ot/alerts.go` prints 1; `grep -c "OT_PG_DSN_PAGES\|DSNEnvPages\|AdminDSNEnv" cmd/ot/alerts.go` prints 0; `grep -c "internal/notify/notifytest" cmd/ot/alerts.go cmd/ot/wire.go cmd/ot/scheduler.go` prints 0 for each file.
2. `grep -rhE "^func (TestSchedulerCallsNotifyAlertsAfterGaps|TestSchedulerSurvivesNotifyAlertsError|TestAlertsOverrideFileReplacesDefaults|TestAlertsBadOverrideStopsStart|TestAlertsCheckOpensNoDatabase|TestAlertsErrorsHoldNoSecret|TestNewDepsFilled)\(" cmd/ot | wc -l` prints 7; `go test ./cmd/ot -race -count=1` passes.
3. `grep -c "t.Parallel()" cmd/ot/alerts_test.go` is at least `grep -c "^func Test" cmd/ot/alerts_test.go`; `grep -c "time.Sleep" cmd/ot/alerts.go cmd/ot/alerts_test.go` prints 0 for each file.
4. `grep -c "ot alerts\|alerts.toml\|OT_PG_DSN_ALERTS" docs/contracts/ot.md` is at least 3; `git diff --name-only $(git merge-base HEAD origin/main)` lists only paths of the Files line.

Pull request description (a requirement, not an acceptance item): that it wires the winner reader, and the owner's smoke steps after the merge, which no verify decides: set `OT_PG_DSN_ALERTS` and `OT_ZONE=Etc/GMT-8` in `.env`, `ot alerts check`, then wait for 07:00 after a session day with readings.

Checked by CI: ci-go (`go run ./scripts/arch --check` exits 0); the design verdict.

## Left out

- The `report_page` producer, the next step after this design: until it exists the `risks` block reaches no browser, so after stage 1 lands the owner sees alerts on Telegram only. It needs: a Go reader and builder for each always-present block of `report_page` v2 (`report-page.md`: the other four always-present blocks and the six optional ones have no data), an `ot page` entry that calls them and `risks_block.Build` for the `risks` key, with the readings read through the winner reader (`WinnerReadings`, over the new view `rd.reading_parameter_r1` granted to `ot_ro_pages`; the role `ot_ro_pages` only SELECTs `rd` views and never a base table), the same reader that `alerts-wire` then passes to the notify pass (`alerts-wire` is not cut by this design: it is cut only after the winner reader (`WinnerReadings` over `rd.reading_parameter_r1`) of `docs/design/report-page-producer.md` has landed, together with `notify-wiring` and `sched-cmd-run`; until then nothing wires the alerts pass), so the page and the alert say the same thing for the same day, the `report_page` registration in the serve path, and its own design. `risks_block.Build` is built now because the block is the goal of owner decision 2 and has a schema test of its own; it is not wired.
- Stage 2, listed for later issues and not cut now (Decision 12): `account-datasets`, `broker-account-interface`, `store-account`, `jobs-account-capture`, `compute-risk-figures`, `alerts-account-rules` (the `scope` key, a page-only path), `pages-household-risk-builder` and `pages-positions-builder`, `alerts-account-telegram` (only if the owner answers question 4 yes). Each is its own design cut when stage 2 starts.
- Lock-ins, named: the dedupe key is `(rule_id, day)` once per rule per reading day, so intraday alerts, a re-alert after a clear and a recovery text each need a migration; a rule id is a permanent key of `alerts.sent`, so renaming a rule in the owner's file orphans its rows and says it again that day; the owner's `alerts.toml` replaces the whole list, so a later change of a default does not reach it and one edited threshold means copying every rule; the `sent` table is the one new table and role, so a second kind of delivery state (acknowledge, snooze) adds a table.
- Alerting only on a change of state (fire when a rule goes from clear to fired, say a recovery): rejected for stage 1 because the owner asked for one alert per rule per day and a recovery needs a second delivery state, as notify Decision 14 says for gaps. A persistent condition says itself every day (question 1).
- A backfill of the owner's own IV30 history: the four SPY rules stay `no_data` for about a year (Decision 6).
- Intraday alerts, price-level alerts on a quote, raw VIX levels, per-instrument option alerts, a `d` alert that ignores the quiet window, a second chat or channel, alert acknowledgement or snooze, inbound Telegram commands.
- Order placement of any kind: nothing here calls a broker.
- What is not verified. `compute_figures`, `notify-wiring` and `sched-cmd-run` are not on main; the subject strings of the default rules (`VIXCLS`, `VXNCLS`, `RVXCLS`, `SP500`, `SPY`) follow `compute-figures.md` Decision 9 and are not compiled against landed code; the stored form of the parameter `data_day` (a day text in `value_tag` is assumed; `compute-figures.md` Decision 3 names the parameter, not its form, and a form that differs is a one-line change in `alertrules`, caught by `TestEvalMissingDataDayNoData` against the first real rows); `TestDefaultRulesMatchRegister` checks ids, versions and units, not subjects. The owner's zone is read from ADR 0010 ("the owner's local time (ADR 0010)"), not from a value in the repository. `scripts/arch` accepts the three new packages (`--check` passes on this branch); `.go-arch-lint.yml` is regenerated in this pull request for the changed notify and pages edges, and by issues 1, 2 and 8 only to add the new components.

## Open questions for the owner

1. A condition that stays true (VIX percentile above 0.90 for a week) says itself every day, once per rule per reading day (Decision 2). Keep that, or say a rule only when it turns from clear to fired? Default: once per rule per day, as asked.
2. Telegram text, ADR 0010 (Decision 10): (a) the amendment that lets a market alert carry the rule title, the public figure and its value and the day of the data, or (b) the text is only rule id, level and the owner's local time (ADR 0010) with the details on the page. Default and written here: (a).
3. The owner's zone is Etc/GMT-8 by ADR 0010; set `OT_ZONE=Etc/GMT-8` in `.env`? Default: the zone ADR 0010 names (UTC+8). A `d` alert does not break the quiet window, as decided on 2026-11-08.
4. Stage 2: may Telegram say that an account risk needs a look, with no figure, or is the account risk page-only (ADR 0010)? Default: page-only.
5. The default thresholds and data ages of Decision 6 are proposals; the FRED rules use a data age of 6 calendar days, so an alert can be on a figure up to 6 days old and its line says the data day; edit them at the review of issue 4.

## Adversarial

Read: the full review of revision 1 at `the dispatcher's scratch file adversarial-risk-alerts.md (outside the repository)` (kept by the dispatcher, not committed). Adopted means the text above changed.

1. Gap against the goal (the block reaches no browser; `key`, `title`, `sub` missing; English strings; SPY IVR/IVP dark; whole-file override): adopted in part. The block builder stays and Decision 13 and Left out say plainly that it is not visible until a `report_page` producer exists, and list that producer as the next step with what it needs. The fields are named and tested against the schema; the strings are the Chinese page constants (`pages-builders.md:58`). Decision 6 states that SPY IVR/IVP need about 252 sessions of own IV30 history and stay `no_data` until then, the VIX family and SP500 rules carrying stage 1. The whole-file override is kept (a partial merge is a second format) and named in Left out as a lock-in.
2. Biggest unnamed risk (stale FRED data fires today's alert; Telegram content against ADR 0010): adopted. Decision 7 reads `data_day` and applies a per-figure age consistent with `rules.json` (4 for IV figures, 6 for FRED rules, never above `max_age_days`); the text carries the data day. The ADR conflict is Decision 10: choice (a) as an amendment of `docs/adr/0010-reports-and-delivery.md` (lines 31 and 53 to 54 stay), its own issue 9 in module docs, with the sentence for choice (b); a `public_subjects` list guards the subject, not only the scope. The owner's answer is open (question 2).
3. Simpler way (drop the alert table, role, job and slot): adopted. Rules are a pure function over `ReadingFunc`, run in the notify pass and in `risks_block.Build`; one insert-only table `alerts.sent`; no evaluate job, slot, view, `Alert` record or jobs interface. Ten issues stay ten, with different content, and two interface entries instead of three.
4. Lock-in: not changed in code (once per rule per day is the owner's word), named in Left out: the rule-id rename that says again, intraday and re-alert needing a migration, the whole-file override. The role and DSN are kept because notify must write the table.
5. Claim most likely false (D7 "an older reading is not used, so a stale figure cannot fire"): adopted. The claim is removed; the `data_day` check replaces it (Decision 7), and the claim about the slot inside the quiet window is gone with the slot.
6. Likeliest slip (entry 10 waits on unlanded entries; arch file regenerated three times; "judged" missing): adopted in part. The jobs entries and `compute-jobs-wire` are gone from the wire entry's wait, which is taken out of this cut: it is cut only after the winner reader (`WinnerReadings` over `rd.reading_parameter_r1`) of `docs/design/report-page-producer.md` has landed, together with `notify-wiring` and `sched-cmd-run`; until then nothing wires the alerts pass; the arch file stays serialized through 1, 2, 8, each marked a judged two-module entry; the waiver lines go to 5, 6 and 7, not to the wire entry; the table, the Design lines and the pull request description of issue 10 say the same.
- List "acceptance not checkable by a command": adopted. Every acceptance item is a command; the SQL runs of the `pg` files and the content of tests the command does not run are under Checked by CI; `TestAlertHeldSentAtSeven` is run by name with `-v` and counted.
- List "two-module entries without the word judged": adopted for 1, 2 and 8, in the table and in the Module lines, as `compute-figures.md` does.
- List "Order lines not in the form": adopted. Every line is `Order: after <numbers> (<names>)` or `Order: first.`, with other designs' entries named after the numbers.
