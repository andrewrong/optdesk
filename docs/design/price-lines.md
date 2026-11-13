# price lines, board 14: the owner's buy price per stock, saved, listed and cleared on the existing view, with no new storage

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-14
Revision: 3
Revision note: revision 3 follows the advisor's first review (TopNav test results, the identity and fixed texts of the builder, the confirm decision, the diff items that named other modules). Revision 2 follows the adversarial read (section `## Adversarial`): a price line is the existing `settings.underlying.buy_price`, so there is no table, migration, role, store package or interface entry. Revision 1 stored a new table that nothing read.

Source: the plan's board 14 "Price lines"; `docs/design/alignment-v25.md` (row "Price lines": view exists, no save handler, storage or route); owner goal "valuation and buy price set the strike" (selling options); owner 2026-11-12 (the owner works on the web: a save is a page action).

Goal: the owner opens `/plan/price-lines`, sees the buy price of each name he keeps, saves a new or changed one, clears one, and sees the held stocks that have none. The number is the one the put gate already reads, so a save takes effect on the option candidates without a second step.

## Today

All paths at the head of origin/main when this revision was cut (the worker reads `git merge-base origin/main HEAD` as BASE).

- The view exists: `contracts/schemas/page.price_lines_page.v1.json`, `web/src/pages/pricelines/PriceLinesPage.tsx` (save and delete buttons per row, an add form, a list of held names without a line, plain form posts to the constant `/plan/price-lines` with the hidden `expires` and `mac`), `web/src/generated/price_lines_page.v1.ts`, `internal/contracts/price_lines_page_v1_test.go`. A row holds `symbol`, `market` (`US` or `JP`), `price` (text or null) and `note` (text or null); `unset` holds `symbol` and `market`.
- No builder writes this page and no route serves it: `/plan/price-lines` answers 404 (`docs/design/missing-pages.md` item 7 left the route to a later `ot` design: this one).
- `settings.underlying` already stores `buy_price` (decimal text, "" unset), `sale_price` and `note` per name (`migrations/0033_underlying.sql`, `underlying.Row`). `underlyingpg` validates them (`validateFields`: price rules, `sale_price` not below `buy_price`, note at most 200 characters) and `Apply` with an `underlying.Edit` whose field is a pointer writes only fields that differ; a pointer to "" clears `buy_price`. The store is insert-only; a clear is a new row with an empty price.
- The put gate reads it: `internal/compute/optioncand/put.go` blocks a put whose strike minus bid is above the buy price (`above_buy_price`; a name with none is marked `no_buy_price`), fed by `cmd/ot/watch.go`. `companies_page` shows it as the hand price (`cmd/ot/companies_site.go`). `/underlyings` edits it today.
- The owner post pattern to copy: `cmd/ot/underlyings_post.go` (`postOwnerUnderlyings`: method check, `guardOwnerPost`, parse, `writeUnderlyings` which opens the writer on the line from `a.ownerEditSecrets()`, a 303 with `?saved=N`), `cmd/ot/site.go` (`siteHandler` dispatches a post by path when `rd.edit != nil`), `cmd/ot/underlyings_site.go` (`registerPage`, `heldPositions`, `rd.underlying`), `internal/pages/underlyings` (`Build(in Input)`, `Page()`).

## Decisions

1. A price line is the `buy_price` of one name on the underlying list, shown with that name's symbol, market and `note`. Nothing is stored anew. The set-at stays in the table (`SetAt` of the row, the history of `/underlyings`) and is not shown; there is no kind or label (the view has none).
2. The page rows are the current underlying rows (`underlying.Reader.Current`) whose `buy_price` is not empty, in any status, sorted by market then symbol; `price` is the stored text and `note` the stored note (null when empty). `note` is the same field `/underlyings` edits: it holds a name's one note, and a row added from a held position starts with the text `underlying.NoteHeld`. Saving a line with a note changes that field; saving with the note field left as it is writes nothing for it (Doubt 2).
3. `unset` lists the current rows with `buy_price` empty whose symbol is held as a stock (`heldPositions`, `addHeld`, as the underlyings page does) and whose status is not retired. A held stock that is not on the underlying list is not listed (`/underlyings` adds it from the held list, as it does today).
4. Save: the post carries `action` `save`, `symbol`, `market`, `price`, `note`. The typed symbol is trimmed and upper-cased before the lookup. The key is `market:SYMBOL` as `underlying.Row.Key`; a key that is not on the list is refused with `Field` `symbol`, "not on the underlying list; add it on /underlyings". An empty price is refused with `Field` `price`, "required" (clearing is Delete). The change is one `underlying.Edit` with `BuyPrice` and, when the posted note differs from the stored one, `Note`; the store's own refusals (price format, note length, `sale_price` below the new buy price) come back as an `underlying.InvalidError` and are shown. A save equal to the stored row writes nothing. The note is shared with `/underlyings` and the last write wins: a note edited there between the page being served and this post is overwritten when the posted note differs from the stored one.
5. Delete: `action` `delete` writes one `underlying.Edit` with `BuyPrice` a pointer to "" and nothing else; the note stays. A delete of a name with no price writes nothing. Effect on the put gate: after a delete the name is `no_buy_price` for the next candidate run; after a save the new price applies to the next run. No interface changes: the `Edit` pointer already expresses a clear.
6. Route and guard: GET of `/plan/price-lines` serves the page from `rd.underlying` (nil reader: one `no_reader` gap, no rows); POST of the same path is handled only when `rd.edit != nil` (the `--owner-edit` mode of the other forms), with `guardOwnerPost`. The schema fixes `post_path` to the page's own path (`const`), so unlike the other owner posts this one shares the path and the `site.go` branch tests the method; moving the post to its own path is a schema r2 (Left out). Without owner-edit a POST is 405 as for any path. The write goes through `writeUnderlyings`, the same store, role `ot_w_settings` and line.
7. No confirm step here although `/underlyings` has one (`confirm=1` after a preview page), because `page.price_lines_page.v1` has no preview field and a confirm step is a contracts change of its own (Left out). A delete opens the put gate for that name at once (it is marked `no_buy_price`) and the row then shows under `unset`. The owner's "plan-edit mode" of `docs/design/missing-pages.md` item 5 is `rd.edit != nil` (`--owner-edit`).
8. A refusal re-renders the page with status 422 and the text in `error`; a write redirects with 303 to `/plan/price-lines?saved=1`, a no-op to `?saved=0`; the page prints `saved` from that query as `underlyings` does. 

## Modules

None changed: `internal/pages/price_lines_page` falls under `pages`, `cmd/ot/**` under `ot`, `web/**` under `web`; no dependency is added (`ot` already depends on `pages` and `store`). `docs/design/modules.md` is not edited by this design.

## Interface

None. The page builder's input is `underlying.Row` and the held set, both existing; the store interface `internal/store/underlying/api.go` is not edited. The JSON schema is unchanged.

## Issues

Order: page first, then the route, then the nav item. `Contracts: unchanged` (no file under `contracts/schemas`, `contracts/datasets` or `contracts/proto` changes; `page.price_lines_page.v1.json` is read, not edited). No migration, so no deploy hold. Test runs are in `## Checked by CI`.

### 1 price-lines-page

Module: pages. Size: M.
Order: first.
Files: `internal/pages/price_lines_page/builder.go`, `internal/pages/price_lines_page/page.go`, `internal/pages/price_lines_page/builder_test.go`, `internal/pages/price_lines_page/page_test.go`, `docs/contracts/pages.md`
Design: docs/design/price-lines.md#Decisions (Decisions 1 to 3, 6, 7)
Acceptance:
1. `builder.go` has `Build(in Input) ([]byte, error)` with `Input` (new) holding `RunID`, `BuildID` and `Scope` (as `settings_page.Config` does; the route passes `ot-serve`, `buildIDOf(version)` and `price_lines` as the settings route passes its own), `Rows []underlying.Row`, `Held map[string]bool` (held stock symbols), `Reader bool` (false: the reader is missing), `Form *Form` (nil is read-only: the inert constant token), `Saved string`, `Error string` and `Now time.Time` (the page reads no clock). `Build` (new) returns JSON with `schema` `price_lines_page`, `schema_version` 1, `generated_at` from `Now`, `data_times` an empty list, `run_status` `ok` (`partial` when the reader is missing), `stale` null, `notes` empty, `gaps` empty or the one gap, and the fixed texts as constants of the package, each 1 to 200 characters: `title` "Price lines", `sentence` "Your buy price for each name you keep. The put gate reads it.", `intro` "A saved price applies to the next option candidate run; clearing it removes the check for that name.", `add.hint` "Type the code and market of a name on the underlying list.", `footnote` "The price is the buy_price of the name on the underlyings page; the note is shared with it.". The JSON validates against `contracts/schemas/page.price_lines_page.v1.json`.
2. `rows` holds one row per input row with a non-empty `BuyPrice`, sorted by market then symbol, with `price` the stored text and `note` the stored note or null when empty; `unset` holds each row with an empty `BuyPrice`, a held symbol and a status other than retired, in the order of the rows; `add.markets` is `US` and `JP`; with `Reader` false the page has one `no_reader` gap, no rows and no `unset`.
3. `saved` and `error` carry `Saved` and `Error` and are null when empty; `form.post_path` is the constant of the schema, the route plan/price-lines with a leading slash.
4. `page.go` has the new function `Page() pages.Page` with its `Meta` naming `price_lines_page` and `page.price_lines_page.v1`, and its Builder builds the read-only page of no reader.
5. `builder_test.go` has new tests: `TestBuildRowsSorted`, `TestBuildUnsetSkipsPriced`, `TestBuildUnsetSkipsRetired`, `TestBuildNoReaderGap`, `TestBuildValidatesAgainstSchema`; `page_test.go` has the new test `TestPageMeta`; each calls `t.Parallel()`.
6. `docs/contracts/pages.md` lists the new `Build` and `Page` of the package `price_lines_page` (`git grep -n "price_lines_page" docs/contracts/pages.md` finds them).
7. `git diff --name-only BASE` lists only the new files of the directory `internal/pages/price_lines_page` (`builder.go`, `page.go`, `builder_test.go`, `page_test.go`) and `docs/contracts/pages.md`.

### 2 price-lines-route

Module: ot. Size: M.
Order: after price-lines-page (entry 1).
Files: `cmd/ot/price_lines_site.go`, `cmd/ot/price_lines_post.go`, `cmd/ot/price_lines_site_test.go`, `cmd/ot/price_lines_post_test.go`, `cmd/ot/site.go`, `docs/contracts/ot.md`
Design: docs/design/price-lines.md#Decisions (Decisions 3 to 7)
`site.go` (668 lines) gains one dispatch branch and nothing else. `site_test.go` is 796 lines and is not edited: the new tests are in the two new files, which build the handler through `siteHandler`, whose signature does not change. No `siteOpeners` entry is added (the page reads `rd.underlying`, which the site already opens), so `cmd/ot/ops_today_site_test.go`, which reads `siteOpeners[siteOpsTodayKey]`, stays valid; `cmd/ot/pagereg_test.go` runs on a clone of `pageFamilies`, and the new route has an empty `name` and `order` 0, so no name or order check can clash. `cmd/ot/wire.go` is not edited.
Acceptance:
1. `price_lines_site.go` (new) registers the new route `plan/price-lines` with `registerPage` (empty name, order 0) and builds the page from `rd.underlying.Current`, the held set of `heldPositions` and `addHeld` (the underlyings page's way), the owner token only when `rd.edit != nil`, and the query `saved`; a nil `rd.underlying` gives the `no_reader` gap.
2. `price_lines_post.go` has the new function `postOwnerPriceLines`: method check, `guardOwnerPost`, then the save or delete of Decisions 4 and 5 through `writeUnderlyings` with one `underlying.Edit`; a key not on the list and an empty price on save answer 422 with the text in `error`; an `underlying.InvalidError` answers 422 with `Why`; a write answers 303 to the page route with the query `saved=1`, a no-op `saved=0`; any other error answers 500 with the fixed text and one log line; nothing is written before the guard passes.
3. `site.go` dispatches the path `plan/price-lines` with method POST to the new `postOwnerPriceLines` only when `rd.edit != nil`; with `rd.edit == nil` a POST answers 405 and a GET serves the page; with `rd.edit != nil` a GET still serves the page.
4. `price_lines_post_test.go` has new tests (fake underlying store, fake clock, `t.Parallel()`): `TestPostPriceLineSymbolNormalised` (a posted ` aapl ` is looked up as `US:AAPL`), `TestPostPriceLineSave` (303 and one `Edit` with `BuyPrice` set and `Note` nil), `TestPostPriceLineSaveNote` (a changed note is in the `Edit`), `TestPostPriceLineDelete` (one `Edit` with `BuyPrice` a pointer to "" and nothing else), `TestPostPriceLineEqualNoop` (`?saved=0`), `TestPostPriceLineUnknownKey` (422, no `Apply`), `TestPostPriceLineEmptyPrice` (422, no `Apply`), `TestPostPriceLineRefusedByStore` (a fake `InvalidError` on `sale_price` is shown), `TestPostPriceLineNoCookie` and `TestPostPriceLineWrongMedia` (403, no `Apply`); `price_lines_site_test.go` has the new tests `TestPriceLinesPageRows` (priced rows and the held unpriced name), `TestPriceLinesReadOnly` (no owner edit: GET serves, POST is 405) and `TestPriceLinesNoReader`.
5. `docs/contracts/ot.md` states the route, the post fields and that the write is the `buy_price` of `settings.underlying` (`git grep -n "plan/price-lines" docs/contracts/ot.md` finds it).
6. `git diff --name-only BASE` lists only the four new files `price_lines_site.go`, `price_lines_post.go`, `price_lines_site_test.go` and `price_lines_post_test.go` of `cmd/ot`, and `cmd/ot/site.go` and `docs/contracts/ot.md`.

### 3 price-lines-nav

Module: web. Size: S.
Order: after price-lines-route (entry 2); `TopNav.tsx` lists live routes only.
Files: `web/src/shell/TopNav.tsx`, `web/test/shell/TopNav.test.tsx`
Design: docs/design/price-lines.md#Decisions (Decision 6)
按 frontend-stack：用 react 做 TopNav 里的 Price lines 导航项（路由 plan/price-lines，一个 `ITEMS` 条目，无计算）。
按 frontend-stack：用 vitest 做 TopNav 测试（新增一条断言）。
Acceptance:
1. `TopNav.tsx` has a new `ITEMS` entry with label `Price lines` and path the route plan/price-lines with a leading slash, placed after `Settings` and before `Job console`.
2. `web/test/shell/TopNav.test.tsx` is updated for it: the test of the fixed links expects thirteen items, the last one `Option positions`, and the twelve fixed links in the order Home, Macro, Household, Positions, Risk, Options, Option data, Option candidates, Advice, Settings, Price lines, Job console; the sheet list test expects the same twelve links in that order and then `Structures`, `Result`, `Notices`, `Tracking`; the titles of both tests say thirteen items where they said twelve.
3. `TopNav.test.tsx` has the new test `TopNav lists Price lines` (the link of Price lines has the route plan/price-lines with a leading slash as its href and is marked current when the page path is that route).
4. `git diff --name-only BASE` lists only `web/src/shell/TopNav.tsx` and `web/test/shell/TopNav.test.tsx`.

## Checked by CI

`go test ./internal/pages/... ./cmd/ot/... -race -count=1`, `go run ./scripts/arch --check`, `go build ./...`, `golangci-lint`, the contracts generation check (no change expected), and for the web `pnpm --dir web exec tsc`, `eslint`, `vitest` and the build. Results only a run shows: the page JSON against the schema, the 405 and 303 answers, the store receiving one `Edit`.

## Left out

- Alerts when a price crosses a line: nothing watches a line; no notice, no job.
- Linking a line to option candidates or strikes beyond what the put gate already does with `buy_price` (`above_buy_price`): no other rule reads a line.
- Computing the price from valuation: the owner types it; `docs/design/valuation.md` output does not feed it.
- `sale_price`, a kind or label, several lines per name, the set-at and a history list on this page: each needs a schema change (`page.price_lines_page` r2) or is on `/underlyings`.
- Adding a name from this page: a symbol not on the underlying list is refused; `/underlyings` adds names.
- A post path of its own (as the other owner posts have): the schema fixes the path, so it needs r2.
- A confirm step (a preview, then `confirm=1`, as `/underlyings` has): its own step with a schema change (`page.price_lines_page` r2 with a preview field), decided with the owner.
- A command-line wrapper: none; the page is the action.

## Doubts

1. Two pages now write `buy_price` (`/underlyings` and this one); both write the same row field, so they cannot disagree, but a save here does change the put gate. `/underlyings` has a confirm step and this page has none (Decision 7); the owner has not been asked whether a delete should be confirmed.
2. `note` is shared with `/underlyings` and may hold `underlying.NoteHeld` for a name added from a held position; this page shows it as the line's note. A different note per line needs a column and is left out.
3. A `sale_price` already above zero and a new buy price above it is refused by the store with the text of `sale_price`; the owner fixes it on `/underlyings`.

## Adversarial

The dispatcher's scratchpad (price-lines-adversarial.md) read revision 1 and found six points; the dispatcher decided the cut (a price line is `buy_price`). Answers:

1. Gap against the goal (a number nothing reads): adopted. The line is the `buy_price` the put gate reads, so a save changes the candidates at once (Decision 5, Today). `sale_price` is left out and named.
2. Biggest risk (two sources of truth; ambiguous `rd.stock` key): adopted. One field, no second number; the key is the underlying row key `market:SYMBOL`, not `rd.stock`, so there is no lookup that can match an index or an etf twice.
3. Simpler way (reuse `settings.underlying`): adopted in full: no table, migration, role, store package, interface entry or `modules.md` change; the `Edit` pointer expresses the clear, so the interface is unchanged.
4. Lock-in: no new schema, table or role is introduced; the schema is read as it is. The one fixed thing is the shared post path, named in Decision 6 and Left out.
5. Claim most likely false (grants and `setup.go`, a stale base, GET swallowed by the branch): adopted. No new grant and no `setup.go` change remain; the base is read as `git merge-base origin/main HEAD`, with no commit named; the `site.go` branch tests the method and acceptance 3 of entry 2 states GET still serves the page.
6. Likeliest slip (placeholder migration path, `site_test.go` at 796 lines, tests that count openers and families, test runs in acceptance): adopted. No migration path remains; `site_test.go` is not in Files and the tests are in new files; the tests that read `siteOpeners` and `pageFamilies` are named in entry 2 and need no change; the test runs are under `## Checked by CI`, and the Order lines name entries, not pull requests.
