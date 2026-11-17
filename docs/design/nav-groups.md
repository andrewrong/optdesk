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
- `web/test/shell/pages.test.tsx:53-55` only checks that `nav[aria-label="Main navigation"]` exists on a registry page; it does not change.

## Decisions

1. **The grouping (owner, 2026-11-17).** Six top-level items, routes unchanged:
   - Home (a link, `/home`)
   - Market: Macro `/macro`, Market state `/market-state`
   - Portfolio: Household `/household`, Positions `/positions`, Risk `/risk`
   - Options: Overview `/options`, Data `/options/data` (prefix match kept), Candidates `/options/candidates`, Held options (a label, not a link) with Structures `/options/structures`, Result `/options/result`, Notices `/options/notices`, Tracking `/options/tracking`, then Advice `/advice` (keeps its `Lightbulb` icon)
   - Companies: Watch list `/underlyings` (the underlying list, served today but not in the nav; where the owner turns a name's options, fundamentals or status on and off, owner 2026-11-17), Price lines `/plan/price-lines`; Company data `/company-data` is not a link of this entry: `docs/design/data-collection.md` entry 9 adds it after its route is served (Decision 6)
   - System: Settings `/settings`, Job console `/job-console`, Data health `/data-health`
   The separate Option positions dropdown is removed; its four links keep their targets and move into the Options menu, under the label "Held options".
   Inside a menu a label does not repeat the menu name (owner, 2026-11-17: "不要重复"): in Options the links read Overview (`/options`), Data, Candidates and the label Held options; the page titles are unchanged.
2. **Component: the shadcn `DropdownMenu` already in `web/src/components/ui/`.** Each group item is a `DropdownMenuTrigger` button (label and `ChevronDown`) with a `DropdownMenuContent` of `DropdownMenuItem asChild` links; Home is a plain link button. No component is added, no `shadcn add`, no new package, no change to `components/ui/`. The shadcn `NavigationMenu` was considered: it needs `shadcn add navigation-menu` (a new file and a `SOURCES.md` line) and its viewport and hover-open behaviour have no use here, because every group is a short list of links. The label "Held options" is a plain `div role="presentation"` inside the content (the file has no `DropdownMenuLabel` export and is not edited).
3. **Active state: the group holding the current route is highlighted.** The trigger of that group takes the `secondary` variant and `aria-current="true"`; Home, when current, is the link with `aria-current="page"`; inside an open menu the current link has `aria-current="page"` and the same `secondary` look. The match rule is today's (`currentPath`, exact path, `prefix` for Option data), applied to the group's links; the Option positions links now match too (a page under `/options/structures` marks the Options group). A path in no group marks nothing (as `/structure` today).
4. **The phone sheet shows the groups as headings.** Same `Sheet`, button and 44 px rows; a group is a heading row (`li`, the muted text style of today's "Option positions" row) followed by its links; Home comes first as a plain link with no heading, so the sheet has five heading rows (Market, Portfolio, Options, Companies, System); "Held options" is a sub-heading with its four links indented. Order: the order of Decision 1. The current link keeps `aria-current="page"`.
5. **Future pages join a group.** The groups are data in one new file, `web/src/shell/navGroups.ts` (the list of groups, each with a label and its links; the desktop bar and the sheet both read it, so they cannot disagree). A page adds one link to one group; it never adds a top-level item. A test pins at most six top-level items (`nav > div` has six children). A change that needs a seventh group is a design change.
6. **Placement of the two web entries still to land.** `docs/design/data-collection.md` entry 9 adds "Company data" and `docs/design/valuation-structure.md` entry 8 adds "Valuation" (`/valuation`): both items land inside the Companies group, as a line of `navGroups.ts`. The owner's grouping names Company data but not Valuation; Valuation goes to Companies (it is a per-company page, next to Price lines), and the owner may move it by a one-line edit of `navGroups.ts`.
7. **Desktop bar.** The container keeps `hidden md:flex` and the 44 px phone button stays; `flex-wrap` stays as a safety at 768 px, six items fit one row from 768 px up. The bar is a `nav` with `aria-label="Main navigation"` as today.

## Modules

No row of `docs/design/modules.md` changes. The entry edits only `web/**` (the `web` row owns `web/**` and `web/test/**`).

## Interface

None. `TopNav` keeps its name and its props (none); no generated type, JSON schema or `api.go`/`api.py` file is touched.

## Issues

### 1 nav-groups-web

Module: web. Size: S.
Order: first; land it before the entries of `docs/design/data-collection.md` (9) and `docs/design/valuation-structure.md` (8) are cut, so that their nav item is one line of `navGroups.ts` (see Left out for the text those drafts must change).
Files: `web/src/shell/TopNav.tsx`, `web/src/shell/navGroups.ts`, `web/test/shell/TopNav.test.tsx`
Contracts: unchanged
Design: docs/design/nav-groups.md#Decisions (Decisions 1 to 7)
Acceptance:
1. 按 frontend-stack：用 shadcn `DropdownMenu` 做 the five group menus of the desktop bar (a trigger button with `ChevronDown`, `DropdownMenuItem asChild` links), the Home link a plain `Button asChild` link; 按 frontend-stack：用 shadcn `Sheet` 做 the phone sheet, groups as heading rows. `web/src/components/ui/`, `package.json` and `pnpm-lock.yaml` are unchanged.
2. `navGroups.ts` holds the groups of Decision 1 in that order with these links and paths and nothing else (Company data and Valuation are added by the entries of Decision 6); `TopNav.tsx` reads it for both the bar and the sheet, holds no list of links of its own, and `POSITION_LINKS` and the second dropdown are gone.
3. `TopNav.test.tsx`: `TestTopNavSixTopLevelItems` (new: the bar `nav > div` has six children whose text is Home, Market, Portfolio, Options, Companies, System, and a count above six fails the test); `TestTopNavGroupMenus` (new: opening each menu by keyboard lists its links, text and `href`, in the order of Decision 1, the Options menu holding the label "Held options" before Structures, Result, Notices, Tracking, and Advice last); `TestTopNavMarksGroup` (new, `it.each` over `/home`, `/macro`, `/risk/`, `/options`, `/options/data/9001`, `/options/structures`, `/advice`, `/plan/price-lines`, `/job-console`, `/data-health`: exactly one top-level item carries `aria-current`, the one of the group named in Decision 3, and `/structure` marks none); `TestTopNavSheetGroups` (new: the sheet shows Home first as a plain link, then the five groups (Market, Portfolio, Options, Companies, System) as heading rows, each with its links in the order of Decision 1, and "Held options" as a sub-heading; it marks the current link with `aria-current="page"`).
4. The tests of today that pin the flat list (the fifteen-item list and `toHaveLength(15)`, the Option positions menu test, the Job console and flex-wrap test, the eighteen-link sheet list (`the sheet opens and lists the fifteen items, the position links under Option positions`), `the sheet marks the current page with aria-current`, `marks only the link of %s`, `lists the job console link` and `TopNav lists Price lines`) are replaced by the tests of item 3, and `TestTopNavSheetGroups` states that the marked link at `/options/data/9001` reads "Data"; the two tests that need no change (`marks none on a page that is not in the bar`, the closed sheet with the 44 px button) still pass.
5. `TopNav.tsx` and `navGroups.ts` are each under 600 lines.

## Checked by CI

`ci-web`: `tsc`, `eslint`, `vitest` and the build (entry 1). After deploy the window renders the site at 1440 px and at 390 px and compares the bar and the sheet with the mockup (owner rule: a UI change passes only with a screenshot against the mockup).

## Checked by owner

The grouping and the order of the groups at the first deploy; where Valuation sits (Decision 6).

## Left out

- **Other layouts that were weighed.** The shadcn `NavigationMenu` (Decision 2: a new component file for hover behaviour nothing here needs) and a flat bar with an overflow menu (it keeps 14 top-level items and hides pages in a "More" menu with no meaning; the owner chose named groups, 2026-11-17).
- **Hover-open menus and a mega menu.** Menus open on click or key, as the Option positions menu does today.
- **Moving a route.** No path changes; a link in a group points where the flat item pointed.
- **Conflicts with drafts and designs that count the top-level items or edit `TopNav.tsx` directly.** The window fixes these when entry 1 here is cut; each is a line to change:
  - `docs/design/data-collection.md:238` (entry 9 Order): "`TopNav.test.tsx:47` counts the items as the count on main at the cut (15 today) plus 1" conflicts, because the top-level count stays 6 and the new link is inside Companies; it becomes "the Companies group of `navGroups.ts` gains Company data, and the count of top-level items stays six".
  - `docs/design/data-collection.md:239` (entry 9 Files): `web/src/shell/TopNav.tsx` becomes `web/src/shell/navGroups.ts` (TopNav.tsx is not edited by the entry), `web/test/shell/TopNav.test.tsx` stays (the Companies menu list).
  - `docs/design/data-collection.md:245` (entry 9 acceptance 3): "The nav entry "Company data" links to `/company-data`: `TestTopNavHasCompanyData`" becomes "the Companies menu of `navGroups.ts` holds Company data linking to `/company-data`", the six-item count unchanged.
  - `docs/design/data-collection.md:277` (Doubts 12) and `docs/design/data-collection.md:125` (Build order: "`web/src/shell/TopNav.tsx`" as the shared file) name `TopNav.tsx`; the shared file is `navGroups.ts` after this design.
  - `docs/design/valuation-structure.md:289-290` (entry 8 Order and Files) and `:299` (item 6): "`TopNav.tsx` gains one entry "Valuation" ... following the pinned list in `TopNav.test.tsx`" becomes "the Companies group of `navGroups.ts` gains Valuation (`valuation`), the test of the Companies menu lists it", and `web/src/shell/TopNav.tsx` in the Files becomes `web/src/shell/navGroups.ts`.
  - Drafts in `state/optdesk-drafts/issues/` with the same text: `dc-9-company-data-web.md:9` and `:17` (counts "15 ... plus 1", "16 with Company data") and `:23` (item 3), `val-8-web.md:28` (item 6), and `ms-4.md:21` ("updated to 14 items": stale, "Market state" is already on main and the item is registered by the market-state design; do not register it).
  - `web/test/shell/TopNav.test.tsx:90-98` (the flex-wrap pin of `web-nav-fits-viewport`) is replaced by entry 1 item 3 and 4.
- **A seventh group.** Needs a design change (Decision 5).

## Doubts

1. Whether the owner wants Valuation under Companies (Decision 6); it is one line to move.
2. Desktop widths between 768 and 1000 px: six items fit one row (about 560 px); the phone sheet is used below 768 px as today.
