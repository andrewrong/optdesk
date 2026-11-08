# Alignment with plan v2.5 and design set v2.2: an index design

Author: advisor of optdesk (first draft)
Source: owner decision 2026-11-08 (optdesk-1128): optdesk designs align with plan v2.5 section 11 and design set v2.2;
nothing is dropped silently; what is left out is named and asked. Built from the plan, design set v2.2, the old figure
register and origin/main (PRs 500, 533 and 539 read as pending; rows citing them change when they merge).

This file is an index and is not binding: it writes no code and changes no interface. This document is the source. The owner
page, the artifact "计划对齐清单", is a snapshot of it in Chinese; when a row changes here the page is regenerated.

## Modules

No change to `docs/design/modules.md`, `arch.toml` or `.go-arch-lint.yml`. Each follow-on design writes its own module rows.

## Interface

None. No file under `contracts/` and no interface file changes here.

## Issues

None. The work is the follow-on designs below, each its own design pull request judged by design review.

## Decisions

1. One table, one row per plan item, register group or board; the counts are made from the rows. A status is one of covered
   wired, covered code only, design exists not built, deferred, missing. "Covered" never means built unless it says wired.
2. Follow-on names are working names. `builders-wave` stands for about ten designs, one per wave, named when cut.
3. Order of follow-on designs, with the dependency that fixes it:
   1. `probes`: plan 11.1 items 3-6 (Scanner, tick 456 dividends, 20-year history, Timescale). Items 0-2 are moot: pyside is
      merged and runs. Its answers decide dividends, scanner, tick tables and the depth `bars` can rely on.
   2. `index-iv`: the Historical rpc (a proto change, owner-merge only), the index IV job, index freeze (client id 31),
      the volatility refresh job and the 15:10 premarket capture. These are window_bound: a day without the job loses data
      that cannot be written later, so it does not wait for `bars`.
   3. `bars`: daily trades and weekly adjusted_last with adjust_epoch re-pull, the bars job, Stooq coarse layer, hv30_own,
      permanent intraday quote storage. Gates HV30, price lines, core, diagnosis and the home market card.
   4. `account-data`: account, trade, position, Flex and FX datasets (15 s stream included), store tables and jobs (the
      D12 stage 2 list in risk-alerts.md). Scope is owner question 4. Gates household, positions, ledger, tracking, P&L.
   5. `settings-store` and `runner-stage2`, before `compute-stage2` and `notify-stage2`: settings versioning feeds the rule
      parameters of scanner and scoring and the settings-as-of of pages; the morning report needs job_event, repair-run and
      window state from the runner (lease, control socket, expired and window_bound status).
   6. `compute-stage2`: the Go port and rehash of the 44 pending-recompute register entries, plus candidates, structures,
      scoring and valuation ratios. `notify-stage2`: morning report, night rule, pushes.
   7. `builders-wave`: home members, report data blocks, option pages, structures, tracking, household, companies, market,
      price lines, core, diagnosis; each wave names the datasets and figures it needs.
   8. `cut-over`: `ot import` and `--verify` (depends only on the store tables of `account-data` and `bars`, so it may be cut
      right after them), kind='import' merge, the replay tool, check-every-figure, allowlist to zero, week 13 closure.
   `pyside-guards` (size.toml, import-linter, plan 3.8) is small and independent.

## Counts

| status | rows |
|---|---|
| covered, wired | 21 |
| covered, code only (not wired to data) | 18 |
| design exists, not built | 23 |
| deferred by plan or owner | 5 |
| missing | 48 |

Register: `docs/conventions/register.toml` holds 53 entries: 41 old and 12 new (8 market own and percentile figures,
sp500_vs_200d v2, three structure entries). 44 entries are pending-recompute: all 41 old plus the 3 text-only structure
entries. 9 entries have a Go port (the 8 market ids and sp500_vs_200d v2); the job that computes them is not wired in
`cmd/ot`. The register table below groups the 41 old entries into 12 rows.

## Plan items, register and boards

There is no board 11; board 23 is the component sheet. Plan references are to plan v2.5; "silently dropped" in a row means
the plan asks for it and no design allowed the drop.

| item | plan ref | status | evidence (altered or why) | cut by |
|---|---|---|---|---|
| repo skeleton, arch.toml, ci-go, checks, demo profile | plan 11 W1 | covered, wired | merged: arch.toml, internal/profile, cmd/ot demo | - |
| master.session_day contract + one vN page schema | plan 11 W1 | covered, wired | contracts/datasets/master.session_day.r1.json, page schemas | - |
| FRED job with Plan + window reconcile | plan 11 W1 | covered, wired | internal/jobs/fredseries, runner/reconcile, week1*.md (needs PG and key) | - |
| page builder emitting vN JSON | plan 11 W1 | covered, wired | internal/pages/macro_page, macro_rates | - |
| web move-in + ci-web | plan 11 W1 | covered, code only (not wired to data) | web/, web-fixtures-vn1.md, web-views-wave*.md (fixtures only) | - |
| HTML shell embedding JSON; settings POST route | plan 11 W1 | covered, code only (not wired to data) | pages/embed, cmd/ot serve.go; settings POST route not found. Altered: settings POST route missing; persistence is a design of its own | settings-store |
| pyside move-in + gRPC Session/Contracts/MarketData | plan 11 W2 | covered, code only (not wired to data) | pyside/, pyside-*.md, contracts/proto/ibkr/v1 (真实网关此处未测 / real gateway untested here) | - |
| minimal store option_eod, quote_snapshot | plan 11 W2 | covered, wired | store/snapshot, option-jobs.md, store-week2.md | - |
| option_eod freeze job | plan 11 W2 | covered, wired | internal/jobs/optioneod, wired in wire.go | - |
| index IV job (tick 106 / IvBars), index freeze (client id 31), volatility refresh job (45 symbols x 3 history requests) | plan 11 W2 | missing | no job or design; no Historical rpc in proto; contracts/proto is owner-merge only. Why: plan asks for it; no design names it | index-iv |
| quote_snapshot job | plan 11 W2 | covered, wired | internal/jobs/quotesnapshot | - |
| gocron decision | plan 11 W2 | covered, wired | scheduler.md. Altered: own ticker instead of gocron (scheduler.md) | - |
| 15:10 premarket capture of about 45 symbols (window_bound dataset; plan 3.5, 11 note 7) | plan 11 W2 | missing | none | index-iv |
| expired / window_bound status semantics in the runner | plan 11 W2 | missing | window rule exists (window-rule.md); no window_bound or expired status | runner-stage2, index-iv |
| two jobs on real PG | plan 11 W3-4 | covered, wired | store-week2.md, setup/migrate, pg-tagged tests | - |
| remaining ordinary store tables per schema doc | plan 11 W3-4 | missing | no tables for book, positions, owner settings, exclusions, fx, bars, instrument master beyond store/instrument. Why: no design | account-data, bars, settings-store |
| clock + calendar year-table script | plan 11 W3-4 | covered, wired | clock/calendar, compute-calendar.md, pyside-calendar.md. Altered: daily calendar_refresh job, no IBKR TradingHours cross-check (compute-calendar Decision 2; owner 2026-11-08) | - |
| runner core (manifest, window, Plan, reconcile, lease, lanes, supervision, job_event, control socket) | plan 11 W3-4 | design exists, not built | runner/sched, reconcile, scheduler.md, runner-window.md, window-rule.md; lease, repair-run and control socket not seen. Altered: stage 1 own ticker; lease and control socket not seen | runner-stage2 |
| broker client + alias | plan 11 W3-4 | covered, code only (not wired to data) | broker.md, internal/broker/grpcbroker, pyside-account-b.md (account client only) | - |
| unit-conversion golden tests | plan 11 W3-4 | covered, wired | compute-core.md#compute-fixtures-hand, contracttest/units.json. Altered: hand-worked fixtures instead of old-repo synthetic ones (compute-core Left out; flagged to owner) | - |
| pages.md schema doc | plan 11 W3-4 | covered, wired | docs/contracts/pages.md, page-schemas.md | - |
| daily bars job (daily trades + weekly adjusted_last) | plan 11 W5-6 | missing | no bars dataset, no Historical rpc, no job; hv30_own unregistered. Why: designs say "a bars design later"; plan does not allow dropping | bars |
| Stooq coarse layer | plan 11 W5-6 | missing | named only in modules.md | bars |
| chain refresh job | plan 11 W5-6 | covered, code only (not wired to data) | option-jobs selects chains and terms; no separate chain-refresh job | - |
| FX job | plan 11 W5-6 | missing | none | account-data |
| account job | plan 11 W5-6 | missing | pyside-account-b.md stops at proto, pyside and Go client; no dataset, store or job | account-data |
| Flex job | plan 11 W5-6 | missing | none | account-data |
| dividends (Reference.Dividends, tick 456) | plan 11 W5-6 | deferred by plan or owner | no probe, no design. Why: plan: scheduled only if the day-1 probe passes | probes |
| scanner coarse screen (Scanner.Run) | plan 11 W5-6 | deferred by plan or owner | pyside-ibkr-transport.md leaves scanners out. Why: plan: scheduled only if the day-1 probe passes | probes |
| contract tests against real pyside | plan 11 W5-6 | covered, code only (not wired to data) | pyside tests/contracts; Go-side contract tests use fakes | - |
| weekly adjusted_last re-pull with adjust_epoch | plan 11 W5-6 | missing | one line in plan 1.8 only; no design | bars |
| continuous FX stream sampled every 15 s | plan 11 W5-6 | missing | none (FX job row covers daily only) | account-data |
| ot import + import --verify (two rehearsals) | plan 11 W7 | missing | no design mentions ot import; depends only on the store tables of account-data and bars | cut-over |
| replay tool | plan 11 W7 | missing | none | cut-over |
| kind='import' merge after import (plan 11 note 5) | plan 11 W7 | missing | none | cut-over |
| HV30 / HV(n) | plan 11 W8-9 | covered, code only (not wired to data) | compute-core Figures.HistoricalVol, tested; market.hv30_own not registered, no closes dataset. Altered: waits for a bars design (compute-core Left out) | bars |
| IV30 derivation | plan 11 W8-9 | covered, code only (not wired to data) | compute/figures/iv30.go, compute-figures.md (market.iv30_own); job not wired in cmd/ot. Altered: Cboe-style own IV30 per glossary | - |
| IV percentile / rank | plan 11 W8-9 | covered, code only (not wired to data) | figures/ivp.go, market.ivp_own, ivr_own. Altered: GLOSSARY ivp_v1 (at-or-below, 252) wins over plan line 84 strictly-below (flagged 2026-11-08) | - |
| candidates (ITM probability N(-d2), excess yield, earnings date, iv_context) | plan 11 W8-9 | missing | no compute design; register ids are Python text. Why: silently dropped | compute-stage2 |
| structures (max loss/gain, unrealized pnl, earned share, close cost, roll credit, commission, level distance) | plan 11 W8-9 | missing | no compute design. Why: silently dropped | compute-stage2 |
| scoring | plan 11 W8-9 | missing | no design names it. Why: silently dropped | compute-stage2 |
| valuation ratios | plan 11 W8-9 | missing | compute-core has generic Ratio only. Why: silently dropped | compute-stage2 |
| register migration (ids, version, hash, Go impl) | plan 11 W8-9 | design exists, not built | register.toml holds all 39 old ids + 13 new; 44 of 53 tables source_hash pending-recompute; compute-figures ports 8 market.* ids + sp500_vs_200d v2. Altered: single register.toml kept; go/printer hash only for new ids | compute-stage2 |
| macro ease score -2..+2 | plan 11 W8-9 | missing | macro_page builder emits gap "松紧度暂无数据"; no producer. Why: neither old register nor plan has it | owner question 3 |
| go/printer hash equivalence check | plan 11 W8-9 | covered, wired | compute-figures Decision 12 | - |
| web option page family | plan 11 W8-9 | covered, code only (not wired to data) | option-pages-v22.md; option_data_page live at /options/data | - |
| builder: home | plan 11 W10 | covered, code only (not wired to data) | home_page/compose.go merged; members absent so /home shows samples (PR 533, PR 539). Altered: member builders absent, home shows samples | builders-wave |
| builder: household | plan 11 W10 | missing | no design; needs stage-2 data | builders-wave, account-data |
| builder: companies | plan 11 W10 | missing | none | builders-wave |
| builders: report x4 | plan 11 W10 | design exists, not built | PR 500 report-page-producer builds the risks block only. Altered: four data blocks each a later design | builders-wave |
| web macro/market pages | plan 11 W10 | design exists, not built | macro-page-wiring (macro live); missing-pages.md#market-page-view (schema, view, no producer, no route) | builders-wave |
| option candidates page builder (option_page, stock_option_page) | plan 11 W11 | missing | option-pages-v22 Left out: builders each its own design | builders-wave, compute-stage2 |
| full scan builder | plan 11 W11 | missing | same | builders-wave |
| structures page builder (structures_list, structure_detail) | plan 11 W11 | missing | tracking-household-reports Left out | builders-wave |
| option P&L ledger / option_result builders | plan 11 W11 | missing | same | builders-wave, account-data |
| tracking builder | plan 11 W11 | missing | view and schema v2 only | builders-wave |
| notify morning report | plan 11 W11 | missing | notify.md Left out: morning report, night rule, pushes; stage 1 = missed-job alert only. Why: stage 1 without morning report (owner 2026-11-08, notify); alerts stage 1 market-only (owner 2026-11-08, optdesk-1042) | notify-stage2 |
| settings form (rule_param) | plan 11 W11 | design exists, not built | page.settings_page.v1 exists; no POST handler or persistence. Altered: missing-pages Left out: needs an ot and store design of its own | settings-store |
| stop-the-world import, replay last N sessions, cut-over | plan 11 W12 | missing | none | cut-over |
| web check-every-figure | plan 11 W12 | missing | none | cut-over |
| allowlist to zero, enforce mode, runbook | plan 11 W13 | deferred by plan or owner | not designed; runbooks exist per area | cut-over |
| 11.1 items 0-2: crew verify sandbox, grpcio wheel, grpc.aio loop | plan 11 11.1 | covered, wired | moot: pyside is merged and runs (pyside/ with uv.lock, pyside-*.md) | - |
| 11.1 items 3-6: Scanner, tick 456 dividends, 20-year history, Timescale | plan 11 11.1 | missing | no probe design | probes |
| ibkr read-only guard, pacing, session codes, two gateways | plan 1.1 | covered, wired | pyside-ibkr-transport.md, pyside-gateways-health.md | - |
| account collection with partial status | plan 1.1 | design exists, not built | pyside-account-b.md (no store) | account-data |
| continuous reconcile; windows | plan 1.2 | covered, wired | scheduler.md, runner-window.md, window-rule.md (FRED, option_eod) | - |
| push once for gateway auth/connection, <3h window left | plan 1.2 | covered, code only (not wired to data) | notify.md missed-job alert, core/quiet.go; wiring is notify-wiring issue | - |
| 08:00 KL morning report of self-healed events | plan 1.2 | missing | see W11. Why: see W11 | notify-stage2 |
| store: observation envelope, instrument master, append-only | plan 1.3 | covered, wired | store-master.md, store-week2.md | - |
| owner settings versioned in DB + settings-as-of; alert rules file (internal/compute/alertrules, rules.toml) lives in a file | plan 1.3 | missing | no design for the DB settings; alertrules merged as pure evaluation (PR 532). Why: alert rules in a file conflicts with plan 1.3 (settings in DB; only gateway and credentials in files). Altered, pending owner | settings-store |
| every scan evaluation stored incl. rejected (ADR 0018) | plan 1.3 | missing | no scan job | compute-stage2 |
| compute: glossary arithmetic only; IV/Greeks from IBKR model only | plan 1.4 | covered, wired | compute-core | - |
| register: one per figure, hash, gap_text | plan 1.4 | design exists, not built | see register rows | compute-stage2 |
| pages: vM JSON, no browser computation | plan 1.5 | covered, wired | AGENTS.md, web | - |
| gaps said once; closed market not stale; JPY with approx USD | plan 1.5 | covered, code only (not wired to data) | pages/gaps, option-pages-v22 (approx_usd_text) | - |
| notify: Telegram never account-derived | plan 1.6 | covered, wired | ADR 0010; notify.md; #537 amendment. Altered: ADR 0010 amendment: market alert may carry rule title and public figure (owner 2026-11-08) | - |
| report files immutable, atomic | plan 1.6 | missing | PR 500 serves per request, no stored report. Why: plan: stored immutable files; PR 500: computed per request. Altered, pending owner (question 5) | builders-wave |
| stock history two bases, weekly adjusted_last re-pull | plan 1.8 | missing | see W5-6 | bars |
| intraday quote storage kept permanently, monthly partitions | plan 1.3 | missing | quote_snapshot job exists; no partitioning or retention design | bars |
| pyside size.toml and import-linter locks | plan 3.8 | missing | not found on main (no size.toml, no import-linter config); pyside ci covers ruff, mypy, pytest only | pyside-guards |
| market.spx_close | plan 5.5 | missing | no bars/index close dataset; home card is sample | bars |
| market.sp500_vs_200d | plan 5.5 | covered, code only (not wired to data) | v2 built (compute-figures Decision 6); job unwired in cmd/ot; pages empty. Altered: v2 supersedes v1; v1 Python untouched | - |
| market.index_iv | plan 5.5 | missing | no index IV job | bars |
| home.options_summary.* (2) | plan 5.5 | missing | needs trades dataset | account-data, compute-stage2 |
| home.household.* (5) | plan 5.5 | missing | needs account dataset | account-data, compute-stage2 |
| home.options.* (4) | plan 5.5 | missing | needs positions and fills | account-data, compute-stage2 |
| household.* (13) | plan 5.5 | missing | household builder and account data absent | account-data, compute-stage2 |
| companies.volatility.hv30, iv30 | plan 5.5 | missing | iv30 partly via market.iv30_own; hv30 needs bars | bars, compute-stage2 |
| structure.* (7) | plan 5.5 | missing | no structure compute | compute-stage2 |
| option.candidates.* (4) | plan 5.5 | missing | no candidates compute | compute-stage2 |
| option.tracking.pnl_split | plan 5.5 | missing | no tracking data | account-data, compute-stage2 |
| new ids: market.iv30_own, ivp_own, ivr_own, vix_term_ratio, vix/vxn/rvx/sp500 pctile_3y | plan 5.5 | covered, code only (not wired to data) | compute-figures.md, Go, readings store; job unwired in cmd/ot | - |
| new ids: structure.close_cost, roll_credit, recent_commission | plan 5.5 | missing | entries added, nothing builds them | compute-stage2 |
| Home | board 1 | covered, code only (not wired to data) | home-board-1.md, home-page.md, PR 539; members samples | builders-wave |
| US close / US open / JP close reports | board 2-4 | design exists, not built | report-page.md; PR 500 risks block only; four data blocks + six open blocks missing. Altered: close-report valuation sections dropped by owner (Left out) | builders-wave |
| Option page | board 5 | design exists, not built | option-pages-v22.md (view) | builders-wave, compute-stage2 |
| Positions | board 6 | design exists, not built | tracking-household-reports Left out; stays as is (owner 2026-11-08). Altered: stays as is (owner 2026-11-08) | account-data |
| Market | board 7 | design exists, not built | missing-pages.md (schema, view; no route, no producer) | builders-wave |
| Macro | board 8 | covered, code only (not wired to data) | macro-page-wiring.md (FRED indicators live); ease score and events gap. Altered: ease score and events gapped | owner question 3 |
| Indicator detail | board 9 | design exists, not built | tracking-household-reports (v2 view) | builders-wave |
| Review | board 10 | deferred by plan or owner | tracking-household-reports Left out. Why: owner: not now, decided at strategy | - |
| Household | board 12 | design exists, not built | view with fixture; no builder or data | builders-wave, account-data |
| Companies | board 13 | design exists, not built | carried schema and view; valuation (估值一眼, 市场在赌的增长, 五问) has no design. Altered: no design for valuation method | builders-wave, bars |
| Price lines | board 14 | design exists, not built | view; no save handler, storage or route | bars, owner question 2 |
| Core | board 15 | design exists, not built | view; no producer | bars, owner question 2 |
| Diagnosis 10 years | board 16 | design exists, not built | view; no producer; needs annual-report data | bars, owner question 2 |
| Full scan | board 17 | design exists, not built | option-pages-v22.md | builders-wave |
| One-stock option | board 18 | covered, code only (not wired to data) | option_data_page live at /options/data; stock page no builder | builders-wave |
| Option result | board 19 | design exists, not built | tracking-household-reports Left out | builders-wave, account-data |
| Ledger | board 20 | design exists, not built | tracking-household-reports#ledger-chart-view | builders-wave, account-data |
| Tracking | board 21 | design exists, not built | tracking-household-reports#tracking-view | builders-wave, account-data |
| Backtest | board 22 | deferred by plan or owner | missing-pages Left out; ADR 0001. Why: plan defers (line 83); pricing is owner question 1 | owner question 1 |
| Component sheet (not a page) | board 23 | covered, wired | n/a | - |
| Structure detail | board 24 | design exists, not built | view; break-even line needs numeric chart axis (Left out) | builders-wave, compute-stage2 |
| Household risk | board 25 | design exists, not built | view and risk-fixtures; no builder or data | builders-wave, account-data |

## Open questions for the owner

1. Options pricing for backtest: where and which library (blocks board 22 and ADR 0001; plan allows IBKR model values only).
2. In which weeks boards 14-16 (price lines, core, diagnosis) get producers; they need `bars` and annual-report data.
3. Who produces the macro ease score -2..+2; neither the old register nor the plan has it, and no data source is named.
4. Scope of stage-2 account data (positions, values, fills, Flex, FX) before cut-over.
5. Report pages: stored immutable files (plan 1.6) or computed per request (PR 500). Listed as altered, pending the owner.
Also to confirm: the alert rules in a file against plan 1.3, and whether `market_summary` v3 on main has
an owner word (none traced).

## Left out

- Own pricing of Greeks and IV (plan lines 37, 83); pricing for backtest is question 1. Backtest (board 22): deferred.
- Dividends and Scanner only if the probe passes; revisited by `probes`. Timescale tick tables: second phase (plan 353).
- Backups and restore drill: deferred by the owner (plan 343, 361); revisited at `cut-over`.
- Board 10 Review: owner 2026-11-08, not now; revisited at strategy.
- Covered-call extras and close-report valuation sections: dropped by the owner (option-pages-v22, tracking-household-reports).
- Old serve is not restarted; the collection gap is accepted (plan line 25).

## Adversarial

Answers to the review in `align-adversarial.md`:

- Count did not reconcile: adopted. Counted from register.toml: 53 entries, 41 old, 12 new, 44 pending; the "tables" wording
  is removed.
- Plan items never named (premarket capture, index freeze, volatility refresh, FX 15 s, intraday storage, expired and
  window_bound, kind='import', adjust_epoch, size.toml and import-linter): adopted, one row each. size.toml and import-linter
  are not on main, so the row says missing, not done.
- Probes line wrong: adopted; items 0-2 moot with evidence, 3-6 remain.
- Alert rules file against plan 1.3: adopted as an altered row under `settings-store`, pending the owner.
- "Covered" overstates: adopted; the status words are split and counted in `## Counts`.
- Snapshot of unmerged PRs and hand-kept Chinese page: partly adopted. The doc states it is the source and the page a
  snapshot; no mechanical check exists, and the page is generated from the same rows by a script outside the repo.
- Simpler way, one table: adopted.
- Lock-in: adopted as non-binding wording and Decision 2; names are working names.
- Dependency claim false: adopted; `settings-store` and `runner-stage2` come before `compute-stage2` and `notify-stage2`.
- Bars overloaded, index IV waits: adopted; `index-iv` split out and placed second.
- Order: import depends on `account-data` and `bars` only: adopted in Decision 3.8. Replay and closure stay last.
- Shape: `## Decisions` and `## Open questions for the owner` added; the Chinese page is named as the artifact.
- Owner questions 1-4 unanswered block designs: not adoptable here; they are asked, and a fifth is added.
