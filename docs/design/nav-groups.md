# nav groups: the top bar holds six items, five of them menus of the pages they group

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-17

Source: owner decision 2026-11-17 (ask optdesk-4560, plan item P6): the grouping below, all routes unchanged. Mockup: `state/optdesk-drafts/designs/nav/nav-mockup.html` (desktop bar at 1440 px with the Options menu open, and the phone sheet).

## Goal

The top bar of `web/src/shell/TopNav.tsx` has 16 items today and wraps onto two rows on a desktop. Group the pages into six top-level items so the bar is one row, and make the grouping the place every later page joins, so the bar does not grow again. No route, page or link target changes; only how the links are arranged (the owner added the Watch list link, 2026-11-17).

## Today

- `web/src/shell/TopNav.tsx` (114 lines on origin/main): `ITEMS` (lines 20-35) holds 14 links in a flat list, `POSITION_LINKS` (lines 12-17) holds the four links of the Option positions menu (Structures, Result, Notices, Tracking), so the bar has 14 links plus the Option positions menu, 15 items; the owner counts 16 with the phone button. The comment at line 19 ("Ten of the eleven items") is stale.
- The desktop bar is `<div className="hidden md:flex flex-wrap gap-1 gap-y-1">` (line 84); it wraps because 15 items do not fit one row (the fix `web-nav-fits-viewport` added `flex-wrap`, pinned by `web/test/shell/TopNav.test.tsx:90-98`).
- The Option positions menu is the shadcn `DropdownMenu` of `web/src/components/ui/dropdown-menu.tsx` (exports `DropdownMenu`, `DropdownMenuContent`, `DropdownMenuItem`, `DropdownMenuTrigger` only; built on `radix-ui` 1.8.1, `web/package.json:19`), used at `TopNav.tsx:96-110`. `web/src/components/ui/` has no `navigation-menu.tsx`.
- The phone nav is a `Sheet` (`TopNav.tsx:49-83`): a 44 px `Menu` button, `size-11 md:hidden` (line 51), opening a left sheet titled "Menu" with every item as a 44 px link and the position links under a plain "Option positions" text row (line 73).
- The active page: `currentPath` (line 38) and `isActive` (line 45) compare the pathname with an item's path; `prefix: true` (only on "Option data", `/options/data/<id>`) also matches below the path. The match sets `aria-current="page"` and the `secondary` button variant. The position links are never marked.
- `web/test/shell/TopNav.test.tsx` pins today's shape: the link list in order (lines 28-47) and `toHaveLength(15)` (line 48), the position menu (lines 52-67), the marked link of eight paths (lines 69-82), the flex-wrap pin (lines 90-98), the sheet list of 18 links (lines 129-157) and the sheet mark (lines 158-163).
- After `docs/design/valuation-structure.md` entry 8 merges, `TopNav.tsx` holds a flat Valuation item (route `valuation`) and `TopNav.test.tsx` a pin of it; entry 1 here is ordered after it and moves both.
- `web/test/shell/pages.test.tsx:53-55` only checks that `nav[aria-label="Main navigation"]` exists on a registry page; it does not change.

## Decisions

1. **The grouping (owner, 2026-11-17).** Six top-level items, routes unchanged:
   - Home (a link, `/home`)
   - Market: Macro `/macro`, Market state `/market-state`
   - Portfolio: Household `/household`, Positions `/positions`, Risk `/risk`
   - Options: Overview `/options`, Data `/options/data` (prefix match kept), Candidates `/options/candidates`, Held options (a label, not a link) with Structures `/options/structures`, Result `/options/result`, Notices `/options/notices`, Tracking `/options/tracking`, then Advice `/advice` (keeps its `Lightbulb` icon)
   - Companies: Watch list `/underlyings` (the underlying list, served today but not in the nav; where the owner turns a name's options, fundamentals or status on and off, owner 2026-11-17), Price lines `/plan/price-lines`, Valuation `/valuation` (a link that `docs/design/valuation-structure.md` entry 8 adds to the flat bar and that entry 1 here moves into this menu, removing the flat item, Decision 6); Company data `/company-data` is not a link of this entry: `docs/design/data-collection.md` entry 9 adds it after its route is served (Decision 6)
   - System: Settings `/settings`, Job console `/job-console`, Data health `/data-health`
   The separate Option positions dropdown is removed; its four links keep their targets and move into the Options menu, under the label "Held options".
   Inside a menu a label does not repeat the menu name (owner, 2026-11-17: "不要重复"): in Options the links read Overview (`/options`), Data, Candidates and the label Held options; the page titles are unchanged.
2. **Component: the shadcn `DropdownMenu` already in `web/src/components/ui/`.** Each group item is a `DropdownMenuTrigger` button (label and `ChevronDown`) with a `DropdownMenuContent` of `DropdownMenuItem asChild` links; Home is a plain link button. No component file is added, no `shadcn add`, no new package. The shadcn `NavigationMenu` was considered: it needs `shadcn add navigation-menu` (a new file and a `SOURCES.md` line) and its viewport and hover-open behaviour have no use here, because every group is a short list of links. The label "Held options" is a `DropdownMenuLabel` inside the content (a Radix `Label` of the menu, announced as the label of the items under it, not a presentation `div`). `components/ui/dropdown-menu.tsx` does not export it today, so entry 1 adds the part (the registry's `radix-nova` Label over `DropdownMenuPrimitive.Label`, about eight lines) and updates the sentence of `components/ui/SOURCES.md` that lists the parts left out; no other file of `components/ui/` changes, no package is added.
3. **Active state: the group holding the current route is highlighted.** The trigger of that group takes the `secondary` variant and `aria-current="true"`; Home, when current, is the link with `aria-current="page"`; inside an open menu the current link has `aria-current="page"` and the same `secondary` look. The match rule is today's (`currentPath`, exact path, `prefix` for Option data), applied to the group's links; the Option positions links now match too (a page under `/options/structures` marks the Options group). A path in no group marks nothing (as `/structure` today). The two marks differ on purpose: a trigger is a button that opens a menu, not the page, so `aria-current="true"` says only that the page is inside its group; a link to the page takes `"page"`. A test that counts marks selects `[aria-current]` on `button, a` of the bar, not on `a` only.
4. **The phone sheet shows the groups as headings.** Same `Sheet`, button and 44 px rows; a group is a heading row (`li`, the muted text style of today's "Option positions" row) followed by its links; Home comes first as a plain link with no heading, so the sheet has five heading rows (Market, Portfolio, Options, Companies, System); "Held options" is a sub-heading with its four links indented. Order: the order of Decision 1. The current link keeps `aria-current="page"`.
5. **Future pages join a group.** The groups are data in one module-level const `NAV_GROUPS` at the top of `web/src/shell/TopNav.tsx` (the list of groups, each with a label and its links; the desktop bar and the sheet both read it, so they cannot disagree). A page adds one link to one group; it never adds a top-level item. A test pins at most six top-level items (`nav > div` has six children). A change that needs a seventh group is a design change. A separate `navGroups.ts` was dropped (chosen, 2026-11-17, after the adversarial read): `TopNav.tsx` stays near 200 lines, one const read twice already keeps the bar and the sheet equal, and with Valuation going first the entries 8 and 9 of the other two designs keep `web/src/shell/TopNav.tsx` in their Files, so only their test sentences change (Left out).
6. **Placement of the two web entries; Valuation goes first.** Entry 1 here is ordered after `docs/design/valuation-structure.md` entry 8, which adds Valuation (`/valuation`) to the flat bar; entry 1 moves it into the Companies group as a link and removes the flat item, so the bar never holds seven items. Its routes `valuation-params`, `valuation`, `valuation-company` and `valuation-form` are served at `/valuation-params`, `/valuation`, `/valuation-company` and `/valuation-form` (siblings, none below `/valuation/`), so the Valuation link matches exactly, with no `prefix: true`; the other three are reached by links inside the pages and, as any path in no group, mark nothing. `docs/design/data-collection.md` entry 9 (Company data) lands after entry 1 and adds one line to `NAV_GROUPS`. The owner's grouping names Company data but not Valuation; Valuation goes to Companies (it is a per-company page, next to Price lines), and the owner may move it by a one-line edit of `NAV_GROUPS`.
7. **Desktop bar.** The container is `hidden md:flex gap-1` with no `flex-wrap`, and the 44 px phone button stays: with six items the bar is one row by construction and a regression shows as an overflow, not as a silent second row (a test pins that the class is absent). Whether six items fit from 768 px up is not claimed here; the owner checks it (Checked by owner). The bar is a `nav` with `aria-label="Main navigation"` as today.

## Modules

No row of `docs/design/modules.md` changes. The entry edits only `web/**` (the `web` row owns `web/**` and `web/test/**`).

## Interface

None. `TopNav` keeps its name and its props (none); no generated type, JSON schema or `api.go`/`api.py` file is touched.

## Issues

### 1 nav-groups-web

Module: web. Size: S.
Order: after entry 8 of docs/design/valuation-structure.md
Files: `web/src/shell/TopNav.tsx`, `web/test/shell/TopNav.test.tsx`, `web/src/components/ui/dropdown-menu.tsx`, `web/src/components/ui/SOURCES.md`
Contracts: unchanged
Design: docs/design/nav-groups.md#Decisions (Decisions 1 to 7)
Acceptance:
1. 按 frontend-stack：用 shadcn `DropdownMenu` 做 the five group menus of the desktop bar (a trigger button with `ChevronDown`, `DropdownMenuItem asChild` links), the Home link a plain `Button asChild` link; 按 frontend-stack：用 shadcn `Sheet` 做 the phone sheet, groups as heading rows. "Held options" is a `DropdownMenuLabel`; `dropdown-menu.tsx` gains that one part and `SOURCES.md` its note, no other file of `web/src/components/ui/`, `package.json` and `pnpm-lock.yaml` change.
2. `TopNav.tsx` holds `NAV_GROUPS` with the groups of Decision 1 in that order with these links and paths and nothing else (Company data is added by entry 9 of `docs/design/data-collection.md`); the flat Valuation item of `docs/design/valuation-structure.md` entry 8 is gone and Valuation `/valuation` is a link of Companies; the bar and the sheet both read `NAV_GROUPS`, `TopNav.tsx` holds no other list of links, and `POSITION_LINKS` and the second dropdown are gone.
3. `TopNav.test.tsx`: `TestTopNavSixTopLevelItems` (new: the bar `nav > div` has six children whose text is Home, Market, Portfolio, Options, Companies, System, a count above six fails the test, and the bar's class has no `flex-wrap`); `TestTopNavGroupMenus` (new: `it.each` over the five groups, each case renders afresh and opens one menu by keyboard, never a second menu in the same render, and lists its links, text and `href`, in the order of Decision 1, the Options menu holding the label "Held options" before Structures, Result, Notices, Tracking, and Advice last, the Companies menu holding Watch list, Price lines and Valuation; one more case opens Options at `/options/data/9001` and the link "Data" carries `aria-current="page"`); `TestTopNavMarksGroup` (new, `it.each` over `/home`, `/macro`, `/risk/`, `/options`, `/options/data/9001`, `/options/structures`, `/advice`, `/plan/price-lines`, `/valuation`, `/job-console`, `/data-health`: exactly one `[aria-current]` among the `button, a` of the bar, the trigger (`"true"`) of the group named in Decision 3, or Home (`"page"`) at `/home`, and `/structure` marks none; `/valuation` marks Companies with an exact match, no `prefix`); `TestTopNavSheetGroups` (new: the sheet shows Home first as a plain link, then the five groups (Market, Portfolio, Options, Companies, System) as heading rows, each with its links in the order of Decision 1, and "Held options" as a sub-heading; it marks the current link with `aria-current="page"`).
4. The tests of today that pin the flat list (the fifteen-item list and `toHaveLength(15)`, the Option positions menu test, `the desktop bar wraps onto further rows and holds the Job console link`, the Valuation pin that entry 8 of `docs/design/valuation-structure.md` adds to `TopNav.test.tsx` (its list entry and its link test), the eighteen-link sheet list (`the sheet opens and lists the fifteen items, the position links under Option positions`), `the sheet marks the current page with aria-current`, `marks only the link of %s`, `lists the job console link` and `TopNav lists Price lines`) are replaced by the tests of item 3, and `TestTopNavSheetGroups` states that the marked link at `/options/data/9001` reads "Data"; the two tests that need no change (`marks none on a page that is not in the bar`, the closed sheet with the 44 px button) still pass.
5. `TopNav.tsx` is under 600 lines.

## Checked by CI

`ci-web`: `tsc`, `eslint`, `vitest` and the build (entry 1). After deploy the window renders the site at 1440 px and at 390 px and compares the bar and the sheet with the mockup (owner rule: a UI change passes only with a screenshot against the mockup).

## Checked by owner

The grouping and the order of the groups at the first deploy; where Valuation sits (Decision 6); that the six items are one row at 768 px, 1024 px and 1440 px (not asserted by any test: jsdom cannot measure width); that the label "Held options" and the menus read well with a screen reader.

## Left out

- **Other layouts that were weighed.** The shadcn `NavigationMenu` (Decision 2: a new component file for hover behaviour nothing here needs) and a flat bar with an overflow menu (it keeps 14 top-level items and hides pages in a "More" menu with no meaning; the owner chose named groups, 2026-11-17).
- **Hover-open menus and a mega menu.** Menus open on click or key, as the Option positions menu does today.
- **Moving a route.** No path changes; a link in a group points where the flat item pointed.
- **Conflicts with the data-collection entry 9 and drafts that count the top-level items or edit the flat list.** None is edited by this pull request nor by entry 1 (both edit `web/**` only; `docs/design/` is owner-only). They are edited when entry 9 is cut, through a design PR of the window; each is a line to change:
  - `docs/design/data-collection.md:238` (entry 9 Order): "`TopNav.test.tsx:47` counts the items as the count on main at the cut (15 today) plus 1" conflicts, because the top-level count stays 6 and the new link is inside Companies; it becomes "the Companies group of `NAV_GROUPS` in `TopNav.tsx` gains Company data, and the count of top-level items stays six", and it waits for entry 1 here.
  - `docs/design/data-collection.md:245` (entry 9 acceptance 3): "The nav entry "Company data" links to `/company-data`: `TestTopNavHasCompanyData`" becomes "the Companies menu holds Company data linking to `/company-data`", the six-item count unchanged. Its Files line (`TopNav.tsx`, `TopNav.test.tsx`) and the shared-file lines (`:277` Doubts 12, `:125` Build order) stay true, because `NAV_GROUPS` lives in `TopNav.tsx`.
  - `docs/design/valuation-structure.md` entry 8 (`:289-299`) needs no edit: it adds the flat item first and entry 1 here moves it into Companies and replaces its test.
  - Drafts in `state/optdesk-drafts/issues/` with the same text: `dc-9-company-data-web.md:9` and `:17` (counts "15 ... plus 1", "16 with Company data") and `:23` (item 3), and `ms-4.md:21` ("updated to 14 items": stale, "Market state" is already on main and the item is registered by the market-state design; do not register it).
  - `web/test/shell/TopNav.test.tsx:90-98` (the flex-wrap pin of `web-nav-fits-viewport`) is replaced by entry 1 item 3 and 4.
- **A seventh group.** Needs a design change (Decision 5).

## Doubts

1. Whether the owner wants Valuation under Companies (Decision 6); it is one line to move.
2. Desktop widths between 768 and 1000 px: whether six items fit one row is measured by the owner at the first deploy (Checked by owner); the phone sheet is used below 768 px as today. If they do not fit, the fix is a shorter label or a smaller gap, not `flex-wrap`.

## Adversarial

The adversarial read is `<runner home>/state/optdesk-drafts/designs/nav/adversarial-nav-groups-2026-11-17.md`. Each point, adopted or not:

1. Gap, one row not pinned: adopted. `flex-wrap` is removed (Decision 7) and a test pins that the class is absent; the fit from 768 px is no longer claimed and moves to Checked by owner and Doubts 2. A bound on a group's size is not adopted: no rule is invented before a group has more than ten rows; a group that grows past that is a design change like a seventh group.
2. Companies held two links and Valuation was a guess: adopted by order. Valuation goes first (Decision 6): entry 1 is ordered after entry 8 of `docs/design/valuation-structure.md`, so Companies holds three links at the first deploy. It stays an owner check.
3. Cross-design order unenforceable: partly. `Order: after entry 8 of docs/design/valuation-structure.md` is now a named dependency with the only syntax the repo has; with `navGroups.ts` dropped, entry 9 of data-collection keeps `TopNav.tsx` in its Files and the valuation design needs no edit, so only the test sentences of entry 9 change, when it is cut, through a design PR (Left out). A missed edit shows as a failing `TestTopNavHasCompanyData` before merge, not as a wrong page.
4. Simpler way, keep the list in `TopNav.tsx`: adopted (Decision 5). `navGroups.ts` is dropped: one file fewer in the Files, and with Valuation first the edits to the other designs fall from eight lines to three. A `group` field on the flat list was not taken, because the "Held options" label needs its own structure.
5. "Held options" as a `div role="presentation"`: adopted the other way. It is a `DropdownMenuLabel`; entry 1 adds that part to `dropdown-menu.tsx` and its note to `SOURCES.md` (Decision 2, Files).
6. Lock-in of the names and of the `/options/*` prefix: not changed. The names are the owner's (Decision 1); a path in no group marks nothing (Decision 3), and the Valuation routes that are not under `/valuation/` are named in Decision 6.
7. Claim "fits one row from 768 px" false or unverified: adopted. Removed as a fact; Checked by owner.
8. The wrong test title: adopted. Item 4 names `the desktop bar wraps onto further rows and holds the Job console link`.
9. Menu tests: adopted. `TestTopNavGroupMenus` is `it.each` over the groups, one menu open per render; the `aria-current` difference (trigger `"true"`, link `"page"`) is in Decision 3 and the marks test selects `[aria-current]` on `button, a`; the check of the inside link at `/options/data/9001` is a case of the menu test and of the sheet test.
