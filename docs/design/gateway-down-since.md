---
type: design
title: "gateway down since: the data health page says since when a gateway has no connection, from a state table written by the gateway health pass"
status: draft
date: "2026-11-27"
need: [none]
none_reason: "owner ask 2026-11-27: after a 6-day silent gateway outage the page must say since when a gateway has no connection; no need record exists"
---

# gateway down since: the data health page says since when a gateway has no connection, from a state table written by the gateway health pass

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-27
Plan: P1

Source: owner ask 2026-11-27 (a gateway was down for 6 days and nothing on the page said when it began); `docs/design/gateway-health.md` (landed; Decisions 1, 3, 7, 12, 13 and entries 6, 8, 9). Every fact below was read on origin/main `33b3e4c2`.

Owner-visible result: on `/data-health`, a gateway block whose gateway has no connection (the verdict "session_retrying" or "unreachable") has one more line, "无连接自 <time>", with the time the optdesk scheduler first saw that gateway without a connection in the present outage. A gateway that is connected shows no such line. A farm that is down while the API session is up is not "no connection" and shows no such line.

## Frame

- Today nothing records when an outage began. `broker.Health` carries a `Since` only per farm (`internal/broker/api.go`, `Farm.Since`), and it comes from pyside. When the gateway is fully down, pyside either answers `down` with no time, or does not answer at all (`gatewayReading.answered` false in `cmd/ot/gateway_health.go`), and Go gets no timestamp. A 6-day silent outage happened.
- `docs/design/gateway-health.md` Decision 7 kept the since-time off the page ("the since-time stays in pyside and in the notice, not on the page"). That decision was written for a farm in its grace time. This design amends it, see Decision 6.
- Go already reads every gateway on each scheduler tick: `gatewayPass.pass` (`cmd/ot/gateway_health.go`) is called by `notifyGateways` (`cmd/ot/notify.go`) with one fact per gateway. It keeps no previous verdict (`TestGatewayNotifyPass`), and it is the one place that sees every state of every gateway, including "no answer".
- The role `ot_runner` already writes the schema `ops` (`migrations/0063_job_run.sql`: INSERT, UPDATE on `ops.job_run`) and the scheduler already holds its DSN (`cfg.runnerDSN`, `cmd/ot/scheduler.go`). The role `ot_ro_pages` reads views of schema `rd`. Neither role is new.

## Owner rules and how this design meets them

1. "The front end renders data and computes nothing" (`AGENTS.md`, Tests). The builder makes the text "无连接自 <time>" and the web prints it as given. Self-check: the web entry has no date arithmetic, no formatting and no condition on the verdict.
2. "Pages are in Chinese, a term explained next to it" (owner 2026-11-14, 2026-11-17). The phrase is a constant of `zh_texts.go` and holds no account id and no port.
3. "Dev mode: no time guards" (`docs/design/data-health.md` Decision 13). The pass writes on every tick at any hour; no grace is added to the page line.
4. "Tests run in parallel, never sleep, time from a fake clock, a PostgreSQL test carries the tag `pg`" (`AGENTS.md`, Tests). The time of an observation is passed in by the caller from the injected `clockwork.Clock`; the store reads no wall clock for it. Test runs are in "Checked by CI".
5. "Own code behind an interface" (owner 2026-11-08). The state is read and written behind `gatewaystate.Reader` and `gatewaystate.Writer`.
6. "No rule in `cmd/ot`" (`docs/design/gateway-health.md` entry 9 item 4). The rule "which readings are no connection" is one pure function of the pages package, used by the page and the pass; `cmd/ot` calls it and holds no rule.
7. "One issue, one module" (`AGENTS.md`, Modules). Each entry carries one `Module:` and lists only paths of that module.
8. "No secrets, account ids or personal data" (`AGENTS.md`). A row holds an alias (`main`, `wife`), a flag and two times.

## Decisions

1. **One table, `ops.gateway_state`, one row per gateway alias.** Columns: `alias text PRIMARY KEY` (lower-case letters and digits, as the alias rule of `validGatewayAlias`), `down boolean NOT NULL`, `since timestamptz NOT NULL` (the time the present value of `down` began), `observed_at timestamptz NOT NULL` (the last tick that saw it). Owner `ot_owner`; `ot_runner` has SELECT, INSERT, UPDATE; no DELETE or TRUNCATE for any role but the owner. The page reads the view `rd.gateway_state_r1` (the same columns) as `ot_ro_pages`, which gets the view and not the table.
2. **The write is one observation per gateway per tick.** `Writer.Observe(ctx, alias, down, at)`: no row inserts `(down, since = at, observed_at = at)`; a row with the same `down` sets only `observed_at = at`; a row with another `down` sets `down`, `since = at`, `observed_at = at`. It is one statement (`INSERT ... ON CONFLICT (alias) DO UPDATE`), so two concurrent passes cannot split a row. An `at` earlier than the stored `observed_at` changes nothing (a late tick never moves `since` back).
3. **What "no connection" is.** `data_health_page.ConnectionLost(answered bool, state string) bool` in `internal/pages/data_health_page/gateways.go`: true when `answered` is false, or `state` is `soft_lost` or `down`; false for `up` and `farm_broken` (the API session is up, a farm is not); an unknown state is true, as `gatewayVerdict` already treats an unknown state as `unreachable`. `gatewayVerdict` is rewritten to call it for the "lost" cases, so the page's verdict and the stored flag cannot disagree.
4. **Who writes.** The gateway pass of `cmd/ot/gateway_health.go`, on every call, after the reading and before the notifier: for each reading it calls `Observe(alias, ConnectionLost(answered, state), clk.Now())`. A failed write logs one line (`gateway state not written`, class only) and the notifier still runs; the pass never fails or blocks on the store. The writer is opened by the scheduler with `store.DSNEnvRunner` (`cfg.runnerDSN`); when it cannot be opened the pass runs without it (nil writer, one log line) and the page shows no since-line. No new role, no new DSN line, no `setup.go` change.
5. **Who reads.** `cmd/ot/data_health_site.go` opens the reader with the pages line (`store.DSNEnvPages`), reads the rows once per page build and fills `GatewayInput.Since` (new, a `time.Time`, zero when there is no row or the reader failed). A failed read gives zero times and no gap: the line is an addition to the page.
6. **The page shows the line only when it is true now.** The builder gives `since` and `since_text` to a gateway only when `ConnectionLost(...)` is true for the reading just taken and `GatewayInput.Since` is not zero; otherwise `since` is `""` and `since_text` is `""`. The stored `down` is not consulted by the builder: if the row still says "up" because no tick has run since the outage began, the page shows no since-line rather than a wrong one; if the row says "down" and the reading is connected, the line is off. `since_text` is `无连接自 ` plus `Since` in the zone and layout of `BuiltAt` (`in.Now.Format(timeLayout)`, `builder.go`), a constant prefix in `zh_texts.go`. **This amends `docs/design/gateway-health.md` Decision 7**, where the since-time stayed off the page: for a gateway with no connection the since-time is now on the page. For a farm that is down (the grace time of that Decision) the since-time still stays off the page, and the grace and the notice are unchanged.
7. **What "since" means, and what it does not.** `since` is the first tick of the scheduler that saw the gateway without a connection in this outage, not the instant the gateway failed. It is later than the real start by at most one tick and by the time the scheduler itself was down. For an outage that began before the first deploy of this design the row does not exist, so the first tick after deploy writes `since` = that tick. The note under the line on the page is not changed; the text says "自 <time>" and the glossary line of `zh_texts.go` explains "无连接" (the API session to the gateway is not up) next to it.
8. **Schema of the page.** `Gateway` of `contracts/schemas/page.data_health_page.v1.json` gains two optional properties, `since` (a string, `date-time` or the empty string) and `since_text` (a string), both not in `required`, so every older document and fixture stays valid and `schema_version` stays 1. This is an additive field under the rule of `docs/design/gateway-health.md` Decision 14; this design is the amendment. The builder writes both on every gateway (empty strings when there is no line), so a current document always carries them; an older document has neither, and the web treats an absent field as no line.
9. **Web.** `GatewayBlock.tsx` prints `since_text` as a line of the gateway when it is a non-empty string. Nothing else of the block changes.
10. **Migration.** One migration, `migrations/0074_gateway_state.sql`, with Up and Down: the last on origin/main is `0073_data_check.sql`. A design that lands first takes the next number and this one is renumbered in its pull request. Deploy holds on migrations (the window rebuilds `ot`, then `setup pg`, `migrate`, `setup pg`, owner 2026-11-09); the code that uses the table merges after the migration (Order lines). No new role, so `setup pg` has nothing new to make.

## Modules

`docs/design/modules.md`: the row `store` gains `internal/store/gatewaystate` in its entry cell (declarations only, a package of its own because `internal/store/api.go` is 800 lines and is not touched); this design's commit makes that edit. No other row changes and no dependency edge is added to a depends cell. First row that matches decides: contracts (`contracts/**`, `internal/contracts/**`, `web/src/generated/**`, `docs/contracts/page-schemas.md`: entry 1), store (`internal/store/**`, `migrations/**`, `docs/contracts/store/**`: entries 2, 3, 4), pages (`internal/pages/**`, `docs/contracts/pages.md`: entry 5), web (`web/src/pages/**`, `web/test/**`: entry 6), ot (`cmd/**`, `docs/contracts/ot/**`: entry 7). New import edges, all inside the map: `internal/store/gatewaystatepg` implements `gatewaystate`; `cmd/ot` imports `internal/store/gatewaystate` and `internal/store/gatewaystatepg`; `internal/pages/data_health_page` imports nothing new. `internal/store/setup` is not touched.

## Interface

Entry 1 (contracts). The schema `Gateway` gains `since` and `since_text` of Decision 8; `web/src/generated/data_health_page.v1.ts` is generated from it.

Entry 2 (store), `internal/store/gatewaystate/api.go`, declarations only:

1. `type State struct { Alias string; Down bool; Since time.Time; ObservedAt time.Time }`.
2. `type Reader interface { States(ctx context.Context) ([]State, error); Close() error }`: every row, ordered by alias; no rows is an empty non-nil slice.
3. `type Writer interface { Observe(ctx context.Context, alias string, down bool, at time.Time) error; Close() error }` with the rule of Decision 2 in its doc.
4. `type OpenReaderFunc func(ctx context.Context, dsn string) (Reader, error)` and `type OpenWriterFunc func(ctx context.Context, dsn string) (Writer, error)`.

Entry 5 (pages, not an entry file of a module): `GatewayInput` gains `Since time.Time`; `GatewayVM` gains `Since string` (`json:"since"`) and `SinceText string` (`json:"since_text"`); `func ConnectionLost(answered bool, state string) bool`.

## Issues

Build order: entries 1 and 2 first (the two interfaces, parallel); 3 after 2; 4 after 2 and 3; 5 after 1; 6 after 1; 7 after 4, 5 and 6. Entry 3 is the only migration and holds the auto-deploy. Every entry carries a `Contracts:` line; entry 1 alone changes a file under `contracts/schemas`. Test runs are in `## Checked by CI`. Each entry is one module and lists only paths of that module. All entries: `Plan: P1`.

### 1 gwsince-contract (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/schemas/page.data_health_page.v1.json`, `web/src/generated/data_health_page.v1.ts`, `contracts/fixtures/page.data_health_page.v1.gateways.json`, `internal/contracts/data_health_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.data_health_page v1 (optional `since` and `since_text` on Gateway added; `schema_version` stays 1)
Design: docs/design/gateway-down-since.md#Interface
Acceptance:
1. `$defs/Gateway` of the schema has the properties `since` and `since_text` of Decision 8, neither in `required`, `additionalProperties` still false, and no existing property or enum value changes; `web/src/generated/data_health_page.v1.ts` is produced from the schema, names both as optional, and nothing else in it changes.
2. The fixture `page.data_health_page.v1.gateways.json` gives its `unreachable` gateway a `since` and a `since_text` that starts with `无连接自 `, and its `farm_down` gateway the two empty strings; the fixture holds invented values only.
3. `data_health_page_v1_test.go` has `TestGatewaySinceFixtureValidates` (new) asserting the fixture passes, `TestGatewaySinceRejectsBadTime` (new) asserting a `since` that is neither a date-time nor the empty string fails, and `TestGatewayWithoutSinceStillValidates` (new) asserting a gateway with neither field passes.
4. `docs/contracts/page-schemas.md` names `since` and `since_text` as an additive amendment of `docs/design/gateway-health.md` Decision 14 and of its Decision 7.

### 2 gwsince-store-api (interface)

Module: store. Size: S.
Order: first.
Files: `docs/design/modules.md`
New files: `internal/store/gatewaystate/api.go` (new), `internal/store/gatewaystate/api_test.go` (new), `docs/contracts/store/gatewaystate.md` (new)
Contracts: unchanged
Design: docs/design/gateway-down-since.md#Interface
Acceptance:
1. `api.go` declares what Interface entry 2 lists (`State`, `Reader`, `Writer`, `OpenReaderFunc`, `OpenWriterFunc`) with docs and no function body, and is the only non-test `.go` file of its package; `grep -c 'internal/store/gatewaystate' docs/design/modules.md` prints 1; `git diff --name-only BASE -- internal/store/api.go docs/design/ratchet.json` prints nothing.
2. `api_test.go` has `TestGatewayStateContractFake` (new) asserting a fake writer's `Observe` is read back by the fake reader's `States`, and `TestStatesNonNilSlice` (new) asserting the fake's empty answer is a non-nil slice.
3. `docs/contracts/store/gatewaystate.md` (new) lists the package, the rule of `Observe` (Decision 2) and the sentence "a row holds an alias, a flag and two times, never free text, an id or a secret".

### 3 gwsince-store-schema

Module: store. Size: S.
Order: after 2.
New files: `migrations/0074_gateway_state.sql` (new), `internal/store/setup/gateway_state_pg_test.go` (new)
Contracts: unchanged
Design: docs/design/gateway-down-since.md#Decisions (1, 10)
Acceptance:
1. `migrations/0074_gateway_state.sql` (new) has Up and Down; Up creates `ops.gateway_state` with the four columns of Decision 1, `alias` the primary key and a CHECK that `alias` matches `^[a-z0-9]+$`, and the view `rd.gateway_state_r1` with the same four columns named, both owned by `ot_owner`; it grants SELECT, INSERT, UPDATE on the table to `ot_runner` and SELECT on the view to `ot_ro_pages`, and no DELETE or TRUNCATE to any role but the owner; Down drops the view and the table.
2. `git diff --name-only BASE -- migrations` lists only the one new migration file; `git diff --name-only BASE -- internal/store/setup/setup.go` prints nothing.
3. `gateway_state_pg_test.go` (new, tag `pg`) has `TestGatewayStateGrants` (new) asserting `ot_runner` can INSERT and UPDATE the table and cannot DELETE, `ot_ro_pages` can SELECT the view and cannot SELECT the table, and `TestMigrate0074Down` (new) asserting the table and view are gone after Down.

### 4 gwsince-store-pg

Module: store. Size: M.
Order: after 2 and 3.
New files: `internal/store/gatewaystatepg/store.go` (new), `internal/store/gatewaystatepg/store_test.go` (new), `internal/store/gatewaystatepg/store_pg_test.go` (new)
Contracts: unchanged
Design: docs/design/gateway-down-since.md#Decisions (2)
Acceptance:
1. `store.go` (new) implements `gatewaystate.Reader` and `gatewaystate.Writer` with compile-time assertions, reads the view `rd.gateway_state_r1` and writes `ops.gateway_state` with one `INSERT ... ON CONFLICT (alias) DO UPDATE` statement for `Observe`, and reads no wall clock: every stored time is the `at` of the caller.
2. `store_test.go` (new, no database) has `TestObserveStatementKeepsSinceOnSameFlag` (new) asserting the statement text sets `since` only in the branch where `down` changes and does not move `observed_at` back, and `TestOpenWithEmptyDSNFails` (new) asserting the opens return an error without dialing.
3. `store_pg_test.go` (new, tag `pg`) has `TestObserveSequence` (new) asserting insert, same flag, flipped flag and an earlier `at` give the `since` values of Decision 2, and `TestStatesOrderedByAlias` (new).

### 5 gwsince-page

Module: pages. Size: M.
Order: after 1.
Files: `internal/pages/data_health_page/gateways.go`, `internal/pages/data_health_page/zh_texts.go`, `docs/contracts/pages.md`
New files: `internal/pages/data_health_page/gateways_since_test.go` (new)
Contracts: unchanged
Design: docs/design/gateway-down-since.md#Decisions (3, 6, 8)
Acceptance:
1. `gateways.go` declares `ConnectionLost` with the truth table of Decision 3 and `gatewayVerdict` calls it for the `session_retrying` and `unreachable` cases; the file stays under 600 lines.
2. `GatewayInput` has the field `Since time.Time` and `GatewayVM` the fields `Since` and `SinceText` with the json names of the Interface; the builder fills them as Decision 6 gives, and `gatewayRows` copies no other field of `GatewayInput` into them.
3. `gateways_since_test.go` (new) has `TestConnectionLostTable` (new), `TestSinceShownOnlyWhenLostNow` (new: a connected reading with a non-zero `Since` gives empty `since` and `since_text`), `TestSinceTextPrefix` (new: a lost reading with a non-zero `Since` gives `since_text` starting with `无连接自 ` and the time in the layout of `BuiltAt`), `TestNoSinceWithoutRow` (new: a lost reading with a zero `Since` gives empty strings), and `TestSinceTextHasNoIdOrPort` (new).
4. `zh_texts.go` holds the prefix as a constant and a glossary sentence that explains "无连接" next to it; `docs/contracts/pages.md` names `since` and `since_text` and Decision 6 of this design as the amendment of Decision 7 of `docs/design/gateway-health.md`.

### 6 gwsince-web

Module: web. Size: S.
Order: after 1.
Files: `web/src/pages/data_health/GatewayBlock.tsx`, `web/test/pages/data_health/GatewayBlock.test.tsx`
Contracts: unchanged
Design: docs/design/gateway-down-since.md#Decisions (9)
Acceptance:
1. 按 frontend-stack：用 shadcn card (`web/src/components/ui/card.tsx`, already used by `GatewayBlock.tsx`) 做 the gateway block, with the since-line as one `<p>` of the gateway; no new component and no new dependency.
2. `GatewayBlock.tsx` prints `since_text` as given when it is a non-empty string and prints nothing for it when it is empty or absent; it holds no date call, no format call and no condition on `verdict` for this line.
3. `GatewayBlock.test.tsx` has `TestSinceTextRendered` (new) asserting the `unreachable` gateway of the fixture `page.data_health_page.v1.gateways.json` shows its `since_text`, `TestNoSinceLineWhenEmpty` (new) asserting the `farm_down` gateway shows none, and `TestAbsentSinceRendersAsBefore` (new) asserting a gateway without the field shows none.

### 7 gwsince-wire

Module: ot. Size: M.
Order: after 4, 5 and 6.
Files: `cmd/ot/gateway_health.go`, `cmd/ot/gateway_health_test.go`, `cmd/ot/scheduler.go`, `cmd/ot/data_health_site.go`, `cmd/ot/data_health_site_test.go`, `docs/contracts/ot/pages/data_health_page.md`
Contracts: unchanged
Design: docs/design/gateway-down-since.md#Decisions (4, 5)
Acceptance:
1. `newGatewayPass` takes a `gatewaystate.Writer` (nil allowed) and `gatewayPass.pass` calls `Observe(alias, data_health_page.ConnectionLost(answered, state), clk.Now())` once per reading before `NotifyGateways`; `TestGatewayPassObserves` (new) shows with a fake writer and a fake clock one call per gateway per pass with the flag from `ConnectionLost`, and `TestGatewayPassWriteFailureStillNotifies` (new) shows a writer that errors changes nothing in the facts the notifier gets.
2. `cmd/ot/scheduler.go` opens the writer with `cfg.runnerDSN` through `gatewaystatepg` and closes it with the other stores; when the open fails it passes nil and logs one line.
3. `data_health_site.go` opens `gatewaystate.Reader` with `store.DSNEnvPages` through a package variable (the shape of `openSiteHours`) and fills `GatewayInput.Since` from the row of the same alias; `TestDataHealthGatewaySince` (new) shows a fake reader row for an unreachable gateway gives the fixture's `since_text` prefix, and `TestDataHealthGatewaySinceReaderFails` (new) shows a failing reader gives a document that validates, with empty `since`.
4. `TestGatewayPassNilWriter` (new) shows a nil writer runs the pass as before.
5. No line of `cmd/ot` holds the rule of Decision 3: `grep -n 'soft_lost' cmd/ot/gateway_health.go` prints nothing new, and `cmd/ot/gateway_health.go` and `cmd/ot/scheduler.go` each stay under 800 lines.
6. `docs/contracts/ot/pages/data_health_page.md` names the table `ops.gateway_state`, the writer line `OT_PG_DSN_RUNNER` and the reader line `OT_PG_DSN_PAGES`.

## Checked by CI

`go test ./...` (entries 1, 2, 4, 5, 7; the `pg` tests of entries 3 and 4 run only in GitHub CI), `go run ./scripts/arch` and go-arch-lint (the new import edges of Modules), the schema test of entry 1, the generation check for `web/src/generated`, `ci-web` (`tsc`, `eslint`, `vitest`, the build; entries 1 and 6), `go run ./scripts/precheck`, and the migration check with PostgreSQL 17 and TimescaleDB. After deploy the window runs `ot setup pg`, `migrate`, `setup pg`, waits for one scheduler tick, renders `/data-health` and checks the gateway block with a screenshot (owner rule of 2026-11-14); that check is not an acceptance item.

## Left out

- **The real start of an outage.** `since` is the first tick that saw the loss (Decision 7). A true start time would need pyside to write its own record, which does not exist when pyside is down; not done.
- **A history of outages.** The table holds the present value only, one row per gateway; no insert-only log. The notice of `docs/design/gateway-health.md` Decision 13 is unchanged.
- **A since-time for a farm that is down.** Decision 7 of `docs/design/gateway-health.md` stays for farms (grace time, the time in the notice, not on the page).
- **A new message.** The owner is already told by the notice for a broken farm; a message for "no connection" is its own decision and is not in this design (Doubts, 2).
- **Another page of the table.** No page other than `/data-health` reads `ops.gateway_state`.

## Doubts

1. A scheduler that is itself down does not tick, so the table is stale then; the page shows `since` only when the reading just taken is a loss, so a stale "down" row never shows a line for a connected gateway, and a stale "up" row shows no line for a lost one until the next tick. Whether that last case needs a fallback (for example `observed_at` older than N ticks) was not decided; it is not built.
2. The most useful alarm for the 6-day outage is a message, not only a page line. The pass already has the state needed; whether the owner wants a message "no connection since" is for him to say.
3. The migration number 0074 is taken from main; the open branches were not all read.

## Adversarial

Written by the author, no sub-agent. Each point is answered below.

1. **Gap vs goal.** Adopted in part. The goal is "since when is it down". The design gives "since the first tick that saw it" (Decision 7) and says so in the text of Decision 7 and in Left out; a true start is not available when pyside is down. A scheduler that does not run leaves the table stale (Doubts, 1).
2. **Biggest unnamed risk.** The page and the pass can disagree: the pass writes a flag with one rule and the page shows a verdict with another. Adopted: Decision 3 makes one function, `ConnectionLost`, the rule of both, and Decision 6 makes the page show the line only when its own reading is a loss. The second risk, that the pass runs in `ot scheduler` and the page in the site process with different DSNs and roles, is covered by the grants of entry 3 and the two lines of entry 7 item 6.
3. **Simpler way.** Not adopted. Keeping the first-seen time in memory of the pass is lost at each restart and invisible to the page process; reusing `alerts.sent` (keyed by rule and day) cannot hold a time that moves. The table has four columns and no new role.
4. **Lock-in.** Accepted and named: a table and a view `_r1` (a column change is `_r2`), two optional fields of the page, one exported function of the pages package. The fields are additive under Decision 14 of `docs/design/gateway-health.md`.
5. **Claim most likely false.** "Go gets no timestamp when the gateway is fully down" is read in `gatewayReading` (an unanswered reading holds only the alias), but whether pyside answers `down` with a time for some states was not run. If it does, the pass could still use only its own clock; the design does not rely on pyside's time. "`ot_runner` can write `ops` and the scheduler holds its DSN" is read from `migrations/0063_job_run.sql` and `cmd/ot/scheduler.go`, not run; entry 3 item 3 is the test.
6. **Likeliest slip.** The `since` shown for an outage older than the deploy is the first tick after deploy, which the owner may read as the real start (Decision 7 states it; the text says "自 <time>" and the glossary explains). Second, the migration number clashes with another design; Decision 10 says the later one renumbers. Third, `scheduler.go` (475 lines) and `gateway_health.go` (206) gain a few lines and stay under 800 (entry 7 item 5).

## 摘要

数据健康页上，没有连接的网关多显示一行"无连接自 <时间>"。做法：调度器每个 tick 在网关健康检查里把每个网关"是否无连接"记入新表 `ops.gateway_state`（迁移 0074，沿用已有角色 `ot_runner` 写、`ot_ro_pages` 读视图），状态翻转时才更新 `since`；页面构建器只在当前读数确实无连接且有记录时给出 `since` 与 `since_text`，前端原样显示。"是否无连接"由页面包里同一个函数判定，页面与写表不会不一致。本设计修订 `docs/design/gateway-health.md` 的决定 7：无连接的网关的起始时间现在上页面，坏掉的数据农场仍不上页面。`since` 是调度器第一次看到的时间，不是网关真正出故障的时刻。
