# strategy replay page: one button on the web that runs `ot strategy-replay` and shows its report

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-19. Checked head: `origin/main` `8db7efbf`.

Owner rule: 2026-11-12 (he works on the web; a run is a page action and the CLI is a thin wrapper). Plan item P2, plan step 12.

## Goal

The owner opens the page `/strategy-replay` (menu Options > `Strategy replay`), sees a short form in Chinese (which lots, which account alias, as-of day, the early-close rule, the minimum number of lots) with defaults filled in, presses `开始回放`, and gets the same page back with the report under the form: the first and last stored quote day, the counts, and one row per line (hold, the early rule, his actual) with the lot count beside every figure and the words `不足 20 笔 [T]：无结论` on a thin line. Nothing is written and nothing is stored. `ot strategy-replay` keeps printing the same text from the same function.

## Today

- `ot strategy-replay` is `cmd/ot/strategy_replay.go` (`parseStrategyReplay`, `replayVariants`, `strategyReplayCmd`, `replayEnv`, `replayLots`, `replayPositions`), `strategy_replay_lots.go`, `strategy_replay_market.go`, `strategy_replay_print.go` (`printReplay`, `replayHeader`). The engine is `replay.New().Replay(...)` of `internal/strategy/replay`, behind `replayapi.Replayer` (`internal/strategy/replayapi/api.go`, `Report`, `Line`). It is not a `contracts.Job`: the job console (`cmd/ot/jobconsole_jobs.go`) queues a job id, a day and force, and has no place for its parameters or for a `Report`; this design does not use it.
- `replayLots` calls `context.Background()`; a request deadline does not reach its reads today.
- An owner post exists as a pattern: `cmd/ot/valuation_import_post.go` (`ownerCookieOK`, `refuseOwner`, `pages.NewHandlerFunc`, the page answered whole), `guardOwnerPost` (`cmd/ot/ownertoken.go`: urlencoded body under 64 KiB, cookie, `ParseForm`), the route table in `cmd/ot/site.go` (718 lines at the head). The web may make no network request (ADR 0034): the form is a native `<form method="post">`.
- `cmd/ot/english_test.go` forbids CJK text in every non-test `.go` of `cmd/ot`; the precedent is `cmd/ot/valuation_import_site.go` (an English enum, the Chinese in the builder package). `cmd/ot/valuation_site_test.go:315` pins the last three registered page names by order: `valuation_import_page` (175), `company_quarters_page` (180), `company_data_page` (190).
- `docs/design/modules.md`: `pages` depends on `contracts store compute`, not on `strategy`; `ot` depends on `strategy`.

## Decisions

1. One page, one post, stateless. Route `strategy-replay` (page name `strategy_replay_page`, order 136; Decision 11), post route `strategy-replay-post`. The page without a post result is the view `form`; the post answers the whole page with the view `report` (the form with the values the owner sent, then the report), as `valuation_import_post.go` does. No script request, no JSON answer.
2. The post is a form post, not an upload: `guardOwnerPost` is the guard (cookie, size, `ParseForm`). With `--owner-edit` off the route is not served and the page carries the notice key `form_off` (text `回放未开启：用 --owner-edit 启动 ot serve`, a constant of the builder), as the valuation import page does. The post writes nothing, but the ledger is his private data, so it takes the owner guard anyway.
3. One function for both ways in. `replayRun(ctx, a, o replayOpts) (replayResult, error)` is `strategyReplayCmd` minus the printing: the checks of `parseStrategyReplay` that are not about flags (`lots` value, `min-lots` not negative), `replayVariants`, `replayAsOf`, `replayEnv`, `replayLots`, `Replay`. `replayResult` is the `replayapi.Report`, the `replayHeader` and the variant names with their hashes. The CLI calls it and then `printReplay`; the post calls it and then maps the result to the page input. The demo profile and a missing line give an error in both.
4. Time cap. I could not infer the ledger size from the code: the repository holds no count, and the stored option quotes begin about 2026-11-07, so the market read is bounded by weeks of chains. What is certain: `Replay` is pure and in memory and takes no context, and the cost is the four store reads, which the CLI does without any limit. So the post takes a deadline of 60 seconds [T] (a constant `replayPostTimeout`) on the request context, and `replayLots` takes the context as a parameter instead of `context.Background()`, so the reads stop at the deadline; the deadline cannot stop `Replay` itself, which is quick. A run over the deadline answers the form with the notice key `timed_out` (Chinese text in the builder). No cap on the number of lots. `ot serve` has `ReadHeaderTimeout` only, no write timeout (`cmd/ot/serve.go`), so a 60 second answer is not cut by the server; a proxy in front of it was not checked. The cap moves to a console job only when a measured run passes it.
5. One run at a time. A second post while one runs gets the form with the notice key `busy` (a try-lock in the post file, no queue). The lock is released when the work really ends (the goroutine that runs `replayRun` returns), not when the handler has answered `timed_out`; so a timed-out run that is still finishing keeps the lock and a new press is `busy` until it ends.
6. No stored report. The report lives in the answer of the post; reloading the page shows the form. Keeping past runs is a separate design that needs a table (a store module issue) (Left out). A run is lost when `ot serve` restarts (every merge deploys); it left nothing behind, so the owner presses again, and the form hint says so (`中途被打断的回放不留任何东西，再按一次即可`).
7. `replayapi` is read by the `ot` wiring alone. `pages` may not import `strategy` (modules.md), and adding that edge is a change of the module map for one page. So the builder takes its own plain `Input` (strings and counts, same names as the report fields) and `cmd/ot/strategy_replay_site.go` copies a `replayapi.Report` into it field by field with no arithmetic and no rounding; the money and the shares are the decimal texts of `Line` already. No `strategy` facade is added.
8. Errors are keys, not texts. `cmd/ot` has no CJK in a non-test `.go` (`cmd/ot/english_test.go`), so the pages `Input` carries a notice key (English enum: `form_off`, `timed_out`, `busy`, `read_failed`, `bad_lots`, `bad_as_of`, `bad_early_keep`, `bad_early_dte`, `bad_min_lots`) and the builder maps it to a constant of `zh_texts.go` (precedent `cmd/ot/valuation_import_site.go`). The English error of `replayRun` is never shown; the post classifies it to a key and logs the failing store line name, never a DSN. A bad field gives 422 and the key of that field; the rest answer 200.
9. The visible texts are in the builder (`zh_texts.go`), the front end shows them. Jargon is glossed beside it, in the field's `hint`: `持有到期（hold）：不提前平仓`, `早平（early）：权利金赚到一定比例或离到期不足若干天就买回`, `留存比例（kept share）：赚到的权利金占收到的权利金的比例`, `[T]：试用值，不是定论`. `notes` carries every note the CLI prints (`printReplay`): the line "early assignment is not modelled; a day without a stored quote is a gap, not a guess" (two sentences), "the stored quote is the morning snapshot 90 minutes after the open; settlement is the close" and "a rolled lot's actual does not include the lot it was rolled into" (pinned by `TestStrategyReplayPrintsRolledNote`): four sentences, each its own Chinese constant. `minimum_text` is composed in `builder.go` from `zh_texts.go` constants and the input's minimum (`不足 20 笔 [T]：无结论`).
10. The schema. `contracts/schemas/page.strategy_replay_page.v1.json`, named like `page.valuation_import_page.v1.json`, top-level `schema` const `strategy_replay_page`, `schema_version` 1, `additionalProperties` false.
11. Page order 136. Any order above 175 puts the new page inside the last three names and breaks the tail check of `cmd/ot/valuation_site_test.go:315` (above 190 it becomes the last, between 175 and 190 it splits the tail), so the order is below 175. 136 is free (page orders at the head: 10 to 135, 140 to 170, 175, 180, 190), after `base_data_page` (135) and before the four valuation pages (140 to 170): the tail of three, the order of the four valuation names and that test stay as they are and need no edit. The test pins the last three names only.
12. Serve reads the four DSN lines that `replayEnv` needs (`OT_PG_DSN_OPTION_LEDGER`, `_PAGES`, `_RUNNER`, `_BAR`), through `a.envFile()` like every post (`ops_today_site.go` already reads the runner line, `flex_import_post.go` the ledger line). A missing line or an unwired opener gives the notice key `read_failed` (one fixed Chinese text: `读取数据失败，请检查服务环境里的四行数据库设置`; the log line names the missing line, the page never does) and not a crash. Whether the deployed serve env already holds the bar and runner lines was not checked; the first run shows it.
13. Native `<select name>` for the lots choice, not shadcn `Select`: Radix `Select` posts only through a hidden select after hydration, and no page uses `web/src/components/ui/select.tsx`. The field is a native element styled with Tailwind classes; `Input` and `Label` stay shadcn.
14. Empty reports. With quotes from about 2026-11-07 a replay can have zero lots or no priced line (`internal/strategy/replay/report.go:169` makes lines only from lots), and `minimum_text` hangs on a line. The report then carries `empty_text` (built in `builder.go`: `数据不足，无结论`, then the lot count, the stored quote day count and the first and last quote day used, or `无` when none), and `lines` is empty. The route-off page (view `form`, `form` null, notice `form_off`) has its own fixture.

## Modules

No row of `docs/design/modules.md` changes; every path of this design falls in an existing row, by the first matching glob:

| paths | row |
|---|---|
| `contracts/schemas/**`, `contracts/fixtures/**`, `web/src/generated/**`, `internal/contracts/**`, `docs/contracts/page-schemas.md` | contracts (`web/src/generated/**` is a contracts glob and comes before `web/**`) |
| `internal/pages/strategy_replay_page/**`, `docs/contracts/pages.md` | pages (`internal/pages/**`) |
| `cmd/ot/**`, `docs/contracts/ot/**` | ot (`cmd/**`, `docs/contracts/ot/**`) |
| `web/src/pages/strategy_replay/**`, `web/src/types/**`, `web/src/shell/TopNav.tsx`, `web/test/**`, `web/fixtures/**` | web (`web/**`) |
| `docs/design/strategy-replay-page.md` | docs |

No new edge: `pages` imports `contracts` only (Decision 7), `ot` already imports `pages`, `strategy`, `store`, `compute`, `contracts`; `web` imports the generated type. `internal/strategy/replayapi` is unchanged.

## Interface

1. Page document `page.strategy_replay_page.v1` (the contracts entry, `(interface)`): `schema`, `schema_version`, `generated_at`, `title`, `view` (`form` or `report`), `post_path` (const `strategy-replay-post`), `intro` (one Chinese sentence), `form` (null when the route is off, else `fields`: an array of `{name, label, hint, kind, value, options}` for `lots` (kind `choice`, options `{value, label}` for `short_puts` and `short_lots`), `alias` (text, empty means every stored alias), `as_of` (text `YYYY-MM-DD`, empty means today), `early_keep` (default `0.50`), `early_dte` (default `21`), `min_lots` (default `20`, text), and `submit_label`), `notices` (an array of `{key, text}`: `key` is the English enum of Decision 8, `text` the Chinese sentence), `report` (null or: `as_of`, `lots_label`, `first_quote_day` and `last_quote_day` (texts or null), `variants` (`{name, params_hash}`), `counts` (`lots`, `lots_with_quote`, `still_open`, `lot_days`, `quote_days`, integers), `excluded` (`{reason, text, lots}`), `lines` (below), `notes` (Chinese sentences), `empty_text` (text or null, Decision 14)). A line: `set`, `variant`, `currency`, `lots`, `cycles`, `closed_early`, `held`, `assigned`, `rolled`, `in_the_money` (integers), `kept_share`, `mean_gross`, `worst_gross`, `hold_kept_share`, `actual_kept_share` (decimal texts or null for `n/a`), `under_minimum` (bool) and `minimum_text` (`不足 20 笔 [T]：无结论` or null). The field names of `form.fields` are the names the post reads.
2. `ot`-internal, no other module uses it: `replayRun(ctx context.Context, a *app, o replayOpts) (replayResult, error)` (Decision 3); `type replayResult struct{ Report replayapi.Report; Header replayHeader }`. `replayLots` gains a `ctx context.Context` first parameter.
3. The pages package: `strategy_replay_page.Build(ctx context.Context, in Input) ([]byte, error)` with `Input` of plain fields (view, form values, report fields, notice keys); it imports `contracts` and the standard library.

## Issues

Each entry is one module; none spans two. `(interface)` entries land first; the numbers name entries of this design. All entries: `Plan: P2`, `Design: docs/design/strategy-replay-page.md#<n>`. No migration is added.

### 1 strategy-replay-page-contract (interface)

Module: contracts. Size: M. Order: first.
Files: `contracts/schemas/page.strategy_replay_page.v1.json`, `web/src/generated/strategy_replay_page.v1.ts`, `contracts/fixtures/page.strategy_replay_page.v1.form.json`, `contracts/fixtures/page.strategy_replay_page.v1.report.json`, `contracts/fixtures/page.strategy_replay_page.v1.empty.json`, `contracts/fixtures/page.strategy_replay_page.v1.off.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/fixtures_test.go`, `internal/contracts/strategy_replay_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.strategy_replay_page v1 (new)
Acceptance:
- `contracts/schemas/page.strategy_replay_page.v1.json` has the properties of Interface 1 including `notices` and `empty_text`, `additionalProperties` false at every object, `post_path` const `strategy-replay-post`, and the money and share fields typed as string or null (no number type).
- `web/src/generated/strategy_replay_page.v1.ts` is the generator's output for that schema (it names every property of the schema and no other).
- Four fixtures with invented counts and amounts: `form` (view form, five fields with defaults), `report` (view report with one under-minimum line and a null figure), `empty` (view report, zero lots, `lines` empty, `empty_text` set) and `off` (view form, `form` null, notice key `form_off`); all four are listed in `contracts/fixtures/MANIFEST.synthetic-v1.json`.
- `internal/contracts/strategy_replay_page_v1_test.go` is a new test file that validates the four fixtures against the schema; it is registered in `internal/contracts/fixtures_test.go`.
- `docs/contracts/page-schemas.md` has a paragraph and a table row for the schema in the form of `page.job_console.v1`.

### 2 strategy-replay-page-builder

Module: pages. Size: M. Order: after 1.
Files: `internal/pages/strategy_replay_page/page.go`, `internal/pages/strategy_replay_page/builder.go`, `internal/pages/strategy_replay_page/zh_texts.go`, `internal/pages/strategy_replay_page/purity_test.go`, `internal/pages/strategy_replay_page/builder_test.go`, `internal/pages/strategy_replay_page/testdata/form.json`, `internal/pages/strategy_replay_page/testdata/report.json`, `internal/pages/strategy_replay_page/testdata/empty.json`, `internal/pages/strategy_replay_page/testdata/off.json`, `docs/contracts/pages.md`
Contracts: unchanged
Acceptance:
- `Build` turns an `Input` into the document of Interface 1; every number and text is copied from the input or is a constant of `zh_texts.go`; `builder.go` and `page.go` hold no CJK character; no field is added, divided or rounded.
- `builder.go` imports only `contracts` and the standard library (no `internal/strategy`, `internal/store`, `internal/compute`); `purity_test.go` is a new test in the form of `internal/pages/valuation_import_page/purity_test.go` that fails on any other import.
- The notice keys of Decision 8 each map to one constant of `zh_texts.go`, and an unknown key is a build error.
- `zh_texts.go` holds the hints of Decision 9, the four notes of the CLI header, the form hint about a cut run, and the pieces of `minimum_text` and `empty_text`; an empty figure becomes null; a line with `UnderMinimum` carries `minimum_text` with the words `不足 N 笔 [T]：无结论`; a report with no line carries `empty_text` naming the lot count, the quote day count and the first and last day.
- A new `builder_test.go` builds the four views from `testdata/form.json`, `report.json`, `empty.json` and `off.json` and compares each to the document (the zero-lot and the form-off page included); it uses `t.Parallel()` and no clock.

### 3 strategy-replay-wire

Module: ot. Size: L. Order: after 1 and 2.
Files: `cmd/ot/strategy_replay.go`, `cmd/ot/strategy_replay_run.go`, `cmd/ot/strategy_replay_site.go`, `cmd/ot/strategy_replay_post.go`, `cmd/ot/strategy_replay_test.go`, `cmd/ot/strategy_replay_lots_test.go`, `cmd/ot/strategy_replay_post_test.go`, `cmd/ot/site.go`, `cmd/ot/pagereg_test.go`, `cmd/ot/pagelist_test.go`, `docs/contracts/ot/strategy-replay.md`, `docs/contracts/ot/pages/strategy_replay_page.md`
Contracts: unchanged
Acceptance:
- `replayLots` is defined in `cmd/ot/strategy_replay.go` (line 258 at the head), not in `strategy_replay_lots.go`; it takes `ctx context.Context` as its first parameter and the string `context.Background()` no longer appears in `strategy_replay.go`; every call of `replayLots` passes a context, and `replayPositions` and `resolveMarket` receive that context.
- `cmd/ot/strategy_replay_run.go` holds `replayRun` and `replayResult` (Interface 2); `strategyReplayCmd` only parses flags, calls `replayRun`, then `printReplay`; `replay.New()` appears in `strategy_replay_run.go` and nowhere else in `cmd/ot`; `printReplay`, the flag text and the `parseStrategyReplay` messages are not edited, and the calls in `strategy_replay_test.go` and `strategy_replay_lots_test.go` change only for the new `replayLots` signature.
- `cmd/ot/strategy_replay_site.go` registers the page (`registerPage`, route `strategy-replay`, name `strategy_replay_page`, order 136), copies a `replayResult` into `strategy_replay_page.Input` field by field with no arithmetic, and has a named builder for `ot page --name strategy_replay_page`; with `rd.edit == nil` the input carries the notice key `form_off`. `strategy_replay_site.go`, `strategy_replay_post.go` and `strategy_replay_run.go` hold no CJK character.
- `cmd/ot/strategy_replay_post.go` serves `strategy-replay-post`: `guardOwnerPost` first, the fields read by the names of Interface 1, `replayRun` in a goroutine under `replayPostTimeout` (60 s [T]), the try-lock of Decision 5 released at the end of that goroutine, the notice keys of Decisions 4, 5, 8 and 12 for each failure, and the page answered whole with `pages.NewHandlerFunc`; it calls no store method that writes. `cmd/ot/site.go` gains the one route case for the post and nothing else; `cmd/ot/valuation_site_test.go` is not edited (Decision 11); `pagereg_test.go` and `pagelist_test.go` gain the name and route only.
- New tests in `strategy_replay_post_test.go` (`t.Parallel()`, fake stores, fake clock, no sleep): a new post whose body is built from the field names and defaults of the document `Build` returns for the form view answers the report view; a new post without the cookie is 403 and `replayRun` is not called; a new test per bad field (`as_of`, `lots`, `early_keep`, `early_dte`, `min_lots`) gives its key and 422; a new test with reads that return the deadline error gives `timed_out` while the lock stays held until the fake run returns; a new second post while one is held gives `busy`; a new test with a missing DSN line gives `read_failed`; a new test that the CLI and the post give the same `Report` for the same fake stores.
- `docs/contracts/ot/pages/strategy_replay_page.md` names the route, the post, the guard, the cap, the four DSN lines and the restart note; `docs/contracts/ot/strategy-replay.md` says the report is also shown on the page and that `replayRun` is the one function.

### 4 strategy-replay-web

Module: web. Size: M. Order: after 1.
Files: `web/src/pages/strategy_replay/StrategyReplayPage.tsx`, `web/src/pages/strategy_replay/ReplayReport.tsx`, `web/src/types/strategy_replay_page.ts`, `web/src/shell/TopNav.tsx`, `web/test/shell/TopNav.test.tsx`, `web/test/pages/strategy_replay/strategy_replay.test.tsx`, `web/test/pages/strategy_replay/fixtures/form.ts`, `web/test/pages/strategy_replay/fixtures/report.ts`, `web/test/pages/strategy_replay/fixtures/empty.ts`, `web/test/pages/strategy_replay/fixtures/off.ts`, `web/test/pages/strategy_replay/fixtures/MANIFEST.synthetic-v1.json`, `web/fixtures/strategy-replay-page/report.json`, `web/fixtures/MANIFEST.synthetic-v1.json`
Contracts: unchanged
Acceptance:
- `StrategyReplayPage.tsx` renders `form.fields` as a native `<form method="post" action={post_path}>`; each input has the field's `name`; labels, hints, notices, notes and figures are payload fields; the file contains no `fetch`, no `XMLHttpRequest`, no arithmetic operator on a payload field and no import of `components/ui/select`; the lots choice is a native `<select name>`.
- `ReplayReport.tsx` shows the counts, the `excluded` rows, the `lines` table with the lot count beside each figure, `minimum_text` where not null, `notes`, and `empty_text` when `lines` is empty; a null figure shows `n/a`; a view with `form` null shows only the notices.
- The file exports `registration = { schema: 'strategy_replay_page', ... }`; `web/src/shell/pages.tsx` is not edited.
- `TopNav.tsx` has `{ label: 'Strategy replay', path: '/strategy-replay' }` in the Options group after the `Held options` section, and `TopNav.test.tsx` has it in its expected list.
- The fixtures `form.ts`, `report.ts`, `empty.ts`, `off.ts` are listed in `web/test/pages/strategy_replay/fixtures/MANIFEST.synthetic-v1.json` in the shape of `web/test/pages/valuation_import/fixtures/MANIFEST.synthetic-v1.json` (`marker` `synthetic-v1`, `files`), and `web/fixtures/strategy-replay-page/report.json` is listed in `web/fixtures/MANIFEST.synthetic-v1.json`.
- New tests in `strategy_replay.test.tsx` render the four fixtures and check the field names and the shown texts equal the fixture (the zero-lot and the form-off page included).
UI:
- 按 frontend-stack：用 shadcn `Input` 与 `Label` 做 表单文本字段，用原生 `<select>` 加 Tailwind 类 做 取单范围选择，用 shadcn `Button` 做 `开始回放` 提交按钮。
- 按 frontend-stack：用 shadcn `Card` 做 表单与报告的分块，用 shadcn `Table` 做 逐行报告表，用 shadcn `Badge` 做 `不足 N 笔 [T]：无结论` 标记。
- 按 frontend-stack：用 shadcn `Alert` 做 提示、错误与超时；用 Tailwind `text-muted-foreground` 做 字段旁的中文说明；用 lucide-react `TriangleAlert` 做 提示图标。本页没有图，不用 Recharts。

## Checked by CI

- `ci-go` for entries 1, 2 and 3: build, vet, the tests of the packages (the existing tests of `strategy_replay_test.go`, `strategy_replay_lots_test.go`, `valuation_site_test.go` and `main_test.go` stay green), the architecture check that `internal/pages/strategy_replay_page` imports `contracts` only, `cmd/ot/english_test.go`, the contract test of entry 1; `ci-web` for entry 4 (`tsc`, `eslint`, `vitest`, the build, the hygiene check of the fixtures dir) and the `ui-shots` screenshot of the page against the owner's sketch.
- The tests of entries 2 and 3 use `t.Parallel()`, `t.TempDir()` where a file is needed, fake stores and no sleep.

## Left out

- No queued or background run, no stored report, no history of past runs: the answer of the post is the report. A history needs a table for the `Report` (a store module issue) and is a separate design if the owner asks.
- No cap on the number of lots; the only limit is the 60 second [T] deadline. If the first real run is cut, the cap is a later one-constant change.
- No chart of cumulative hold against actual: later, not step 12.
- No making replay a console job (`/job-console`): its row has no place for the parameters or the `Report`.
- No change to `replayapi`, the engine, or the rule (`shortput`); no new flag on the CLI; no `strategy` facade (Decision 7).
- No change to `modules.md`.
- Entry 4 may land before entry 3: the menu link answers 404 until entry 3 merges. Harmless; no order line is added for it.
- A reload of the answer page re-posts and re-runs; no redirect-after-post, because nothing is stored (Decision 6).

## Adversarial

Raw text: `state/optdesk-drafts/designs/replay-button/adversarial-raw.md`.

- B1 order 188 breaks `valuation_site_test.go:315`: adopted; order 136, Decision 11, the test is not edited.
- B2 fixtures manifest in the web test dir: adopted; entry 4 Files and acceptance.
- B3 the rolled-lot note was dropped: adopted; Decision 9 carries every CLI note.
- Q1 gap, the four DSN lines in serve: adopted as Decision 12 (`read_failed`, no crash); the deployed env is not checked here.
- Q2 risk, restart and the 60 s cap: adopted; Decisions 4 to 6, the hint about a cut run, console job only when a measured run passes the cap.
- Q3 simpler (drop the three trial fields, or a console job): not adopted; the owner tunes `early_keep` and `early_dte`, and a console job needs a table (Left out).
- Q4 lock-in (the Report shape copied into `Input`): accepted as the cost of keeping `pages` off `strategy` (Decision 7).
- Q5 false claim: the `replayLots` file was wrong (it is in `strategy_replay.go`); fixed in entry 3; the quote start date stays hedged.
- Q6 slip (entry 3 stops on the unlisted test, entry 4 on hygiene): adopted; both resolved by B1 and B2, and the CJK rule of `english_test.go` by Decision 8.
