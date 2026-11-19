# site look: the whole site takes the look of the approved candidates mockup (Inter and JetBrains Mono, 11 to 14 px text, blue-grey palette, tier colours), then the candidates page takes its layout

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-19

Source: owner decision 2026-11-19 08:00Z (ask 5206, word "开始吧"): start plan P7, the whole site to the look of the approved candidates mockup, then each page mockup-first. Owner decision 2026-11-19 08:50Z (ask 5217, word "a"): self-host Inter and JetBrains Mono through `@fontsource-variable/inter` 5.3.0 and `@fontsource-variable/jetbrains-mono` 5.3.0 (SIL OFL 1.1, bundled by vite, no CDN) and follow the mockup text sizes (11 to 14 px); this amends ADR 0034 (no web font download, 16 px text). Owner rules carried: hints are in Chinese (2026-11-17), `web/src` stays English-only and the Chinese comes from the Go payload (`zh_texts.go`), the front end computes nothing; a UI pass counts only after a screenshot of the live page beside the mockup.

Read on origin/main at 80c9a01e: `AGENTS.md`, `docs/design/modules.md`, `docs/adr/0034-frontend-stack.md`, `docs/design/candidates-page-v2.md`, `docs/design/buy-price-source.md`, `web/src/styles/theme.css`, `web/package.json`, `frontend-whitelist.json`, `web/deps-allowlist.txt`, `scripts/check-frontend-deps.mjs`, `web/src/pages/candidates/*`, `.github/workflows/ui-shots.yml`. The mockup is the "After" half of the approved candidates table mockup (`candidates-table-mockup.html`, light by default, dark by media query); its "Before" half is today's 15-column table and is not copied.

## What you see

Every page of the site changes at once, without changing what it says. The page background is a pale blue-grey, the blocks are white panels with a thin line and rounded corners, text is small and even (14 px body, 11 px column heads in capitals), words are in Inter, every number is in JetBrains Mono so columns of numbers line up, and green, amber, red and grey are used the same way on every page (fits, check, bad, not judged). In dark mode the same colours are the dark set of the mockup. Chinese text uses the system Chinese font (no Chinese font is downloaded). The pages that are tables of cards today (market, macro, positions, risk, data health, base data, job console, settings, option data, company data, price lines, valuation, strategy replay) look like the mockup with no layout change.

`/options/candidates` changes in layout as well, as in the approved mockup: the company strip is three compact cards in a row (company, spot, buy price, next earnings, "n fit, n check, n collapsed" on two lines) instead of one full-width card per company; the legend is four short coloured boxes; the table is one table with coloured tier rows, a coloured stripe on the left of each row, small tinted pills for the badges, and the run line is one sentence in mono instead of three pills. Hint words keep the dotted underline; the Chinese explanation shows on hover and on keyboard focus, and the long explanations stay in the legend boxes.

The other pages with a layout problem (tracking, result, underlyings, home, household, notices, structures, valuation import) get the new fonts, sizes and colours from this design and keep their layout; each gets its own mockup shown to the owner first, then its own design amendment (Left out).

## Goal

One shared theme, so that a page restyle is a layout change and not a colour and font change. The theme is the tokens of the approved mockup in `web/src/styles/theme.css`; the shared shadcn components in `web/src/components/ui` read them through the existing `@theme inline` mapping, so no page file changes for the theme.

## Today

- `theme.css` (214 lines) maps shadcn tokens through tailwind 4 `@theme inline`; `--font-sans` is the system stack; `--text-xs` and `--text-sm` are forced to `1rem` with line height `1.5rem`; the body is `1rem/1.5`; `--target-min` is `2.75rem`; the colours are the shadcn neutral grey scale in oklch plus `success`, `warn`, `down`, `ok`, `hold`, `unknown` and `banner-*` tokens; the dark set is under `prefers-color-scheme`. Buttons, toggles, tabs and accordion triggers have a 44 px minimum height.
- ADR 0034 line 166: "No CDN, no web font download, no analytics"; its note on `shadcn init` says `@fontsource-variable/geist` was removed because of that rule. `frontend-whitelist.json` (revision `2026-11-18.1`) and `web/deps-allowlist.txt` list no font package, and `scripts/check-frontend-deps.mjs` fails a dependency that is in neither.
- The live candidates page: one card per company, three lines with five or six facts on the first, a reasons line above the legend, separate Fits, Check and Not judged headings, three pills in the page head (day, timestamp, rule name).
- `ui-shots.yml` renders every fixture of `contracts/fixtures/` at widths 390 and 1280 on each pull request that touches `web/` and uploads the PNGs; it compares nothing.

## Decisions

1. **Fonts are self-hosted, nothing is downloaded from outside.** `@fontsource-variable/inter` 5.3.0 and `@fontsource-variable/jetbrains-mono` 5.3.0 are direct dependencies of `web/package.json` (owner 2026-11-19 08:50Z). Only the latin subset files are imported (`.../latin-wght-normal.css` style subpath imports in `theme.css`), so the build copies two hashed `woff2` files into the embedded dist; the Google Fonts link of the later candidates mockup is not used. Licence: SIL OFL 1.1, no transitive dependencies, widely used, stable API; the reviewer checks the four criteria of AGENTS.md. A library that downloads or touches secrets, the network or the sandbox it is not: it is a static font file.
2. **CJK is not self-hosted.** Neither font has Chinese. `--font-sans` is `"Inter Variable", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC", system-ui, sans-serif` and `--font-mono` is `"JetBrains Mono Variable", ui-monospace, Menlo, monospace` (the fontsource variable families carry the `Variable` suffix).
3. **ADR 0034 is amended, in the same entry that uses it.** The sentence "no web font download" becomes "no web font from outside; two self-hosted variable fonts (Inter, JetBrains Mono) are allowed", and the 16 px rule becomes the mockup scale of Decision 4; the 44 px target rule stays for touch (Decision 5). The amendment is dated 2026-11-19 and cites ask 5217. The crew whitelist `rules/frontend-stack.md` is changed by agent-crew; its new revision is `revision <pending>` (this is the only place the string appears) and entry 1 copies the real revision string into `frontend-whitelist.json` together with the two package rows (kind `fonts`) and adds the two names to `web/deps-allowlist.txt`.
4. **Text scale is the mockup's.** Body 14 px / 1.45; h1 22 px; h2 17 px; lead and notes 13 px; column head 11 px, capitals, letter-spacing .05em, weight 600, muted; badge 11 px, weight 600, pill; sub-text 11 px muted; tooltip 12 px. `--text-xs` is 11 px and `--text-sm` 13 px, no longer `1rem`. A number is mono 13 px; every table cell and every number gets `font-variant-numeric: tabular-nums` through one base rule, so no page adds a class for it.
5. **Touch targets stay.** A button, toggle, tab and accordion trigger keeps the 44 px minimum height (the owner reads the pages on a phone); a table row is not a target. The mockup's hover-only tooltip is not copied: the hint is shown on hover, on keyboard focus and on tap, so the explanation is reachable by touch (the existing hint component of `components/gloss-header.tsx` already takes focus; entry 1 does not change its behaviour, only its colours and size).
6. **Palette is the mockup's, light and dark.** Light: page `#f6f7f9`, panel `#fff`, text `#1b2430`, muted `#667385`, line `#e3e7ed`, accent `#1f5fbf` on soft `#eef3fb`; good `#14794a` on `#e3f4ea`, warn `#8a5a00` on `#fdf0d5`, bad `#b3261e` on `#fbe4e2`, neutral `#566275` on `#eceff3`. Dark: page `#10151c`, panel `#171e28`, text `#e4e9f0`, muted `#8f9db0`, line `#283140`, accent `#7aa8f0`; good `#5fd39a` on `#173326`, warn `#e8b85a` on `#3a2e12`, bad `#f08a82` on `#3d1b19`, neutral `#a3b0c2` on `#252e3b`. The values replace the oklch shadcn greys in `:root` and in the dark block; the existing token names (`--background`, `--card`, `--border`, `--success`, `--warn`, `--down`, ...) are kept, so no component file is edited, and the tier names `--tier-fit`, `--tier-check`, `--tier-grey` (each with a `-soft` background) are added and map to good, warn and neutral. Colour is never the only signal: the tier is also a word, as today.
7. **Shape and spacing** are tokens too: panel radius 8 px with a 1 px line border, page padding 24 px 16 px, content column max-width 1180 px (the `--content-column` of 45 rem stays for text pages: a page that sets no width keeps it), table cell padding 7 px 10 px, head 8 px 10 px, card padding 10 px 12 px. The mockup's page background carries a tint behind white panels; `body` takes `--background` and `Card` takes `--card`.
8. **Chinese stays in the Go payload.** Entry 1 and 2 add no visible English or Chinese string in `web/src`. The candidates labels the mockup shows ("Tier", "Contract", "Margin of safety", "Below spot", "Premium-yield", "Badges") are payload fields today or they are not; entry 2 uses the field names that exist and does not rename a label. A label that must change is a payload change of module pages and needs its own issue and a schema or text edit by the pages module after the screenshot step shows the need (Left out).
9. **A theme change must not hide data.** The mockup's company strip drops the valuation lines (they sit in the row detail). Entry 2 shows in the strip only the facts the three short lines carry and moves no field out of the page; a field the payload gives for the strip and the row detail does not show is listed in the pull request description for the owner, not deleted.
10. **A page passes only after a screenshot.** Each pull request of this design carries the ui-shots artifact; the window, after merge, compares a screenshot of the live `/options/candidates` and of one theme-only page with the mockup, and states every difference (owner rule). No pixel comparison runs in CI.

## Modules

No row of `docs/design/modules.md` changes. Both entries are module web (`web/**` and the package, lock, whitelist, ADR and dependency-check files of that row); `web/src/generated/**` belongs to contracts and is not touched. The Go payload files of `internal/pages/option_candidates_page` belong to module pages and are not edited by this design.

- web, entry 1: `web/package.json`, `pnpm-lock.yaml`, `frontend-whitelist.json`, `web/deps-allowlist.txt`, `docs/adr/0034-frontend-stack.md`, `web/src/styles/theme.css`, `web/src/styles/THEME-MAPPING.md`, `web/test/theme/**`.
- web, entry 2: `web/src/pages/candidates/**`, `web/test/pages/**`.

## Interface

There is no Go or generated interface. The interface of entry 1 is the token set that every page and component reads:

- CSS custom properties in `theme.css`: the existing shadcn names (values replaced), `--font-sans`, `--font-mono`, `--text-xs`, `--text-sm`, `--radius` (8 px), `--tier-fit`, `--tier-fit-soft`, `--tier-check`, `--tier-check-soft`, `--tier-grey`, `--tier-grey-soft`.
- A base rule: `font-variant-numeric: tabular-nums` on `table`, `td`, `th` and on the class `.num` (mono, 13 px).
- No new React component. A shared component is added only if entry 2 needs one that a second page will use (then it is a separate interface entry before the page, named in the pull request).

## Issues

Order is a chain: entry 2 reads the tokens of entry 1.

### 1 site-look-theme (interface)

Module: web. Size: M.
Order: first.
Files: `web/package.json`, `pnpm-lock.yaml`, `frontend-whitelist.json`, `web/deps-allowlist.txt`, `docs/adr/0034-frontend-stack.md`, `web/src/styles/theme.css`, `web/src/styles/THEME-MAPPING.md`, `web/test/theme/theme-tokens.test.ts`
Contracts: unchanged
Design: docs/design/site-look.md#Decisions
Plan: P7
Acceptance:
1. `web/package.json` lists `@fontsource-variable/inter` and `@fontsource-variable/jetbrains-mono`, each at exactly `5.3.0` with no range, and `pnpm-lock.yaml` has both at `5.3.0`; the pull request description names both with licence OFL-1.1 and "no transitive dependencies".
2. `frontend-whitelist.json` has the two package rows with those versions and kind `fonts`, and its `revision` equals the revision string of `rules/frontend-stack.md` that agent-crew published (`revision <pending>` in this design; the entry copies the real one and the pull request description quotes it); `web/deps-allowlist.txt` lists the two names.
3. `docs/adr/0034-frontend-stack.md` no longer says "no web font download" or "16 px text" without the amendment: it has a dated amendment (2026-11-19, ask 5217) that allows the two self-hosted fonts and states the mockup text scale; the 44 px target rule is still in the file.
4. `theme.css` imports only the latin subset of each font (no import of a URL starting `http`, no `fonts.googleapis.com`), sets `--font-sans` and `--font-mono` as in Decision 2, sets `--text-xs` to 11 px and `--text-sm` to 13 px, the body to 14 px / 1.45, and keeps `--target-min` at `2.75rem` and the 44 px minimum height of button, toggle, tabs and accordion.
5. `theme.css` `:root` and its dark block carry the palette of Decision 6 hex for hex (every value of the Decision, light and dark), keep every token name it has today, and add the six tier tokens; no `oklch(` remains for a surface, text or line token.
6. A base rule sets `font-variant-numeric: tabular-nums` for `table`, `td`, `th` and `.num`; `.num` is mono 13 px.
7. No file under `web/src/components`, `web/src/pages` or `web/src/shell` is in `git diff --name-only BASE`.
8. `web/test/theme/theme-tokens.test.ts` (new) reads `theme.css` as text and asserts: no `http` in an `@import`, the six tier tokens exist in light and dark, `--text-sm` is not `1rem`, and `--font-sans` names Inter before `system-ui` and a Chinese font after it (test names start with `new` in the pull request description).
9. `THEME-MAPPING.md` lists the new tokens and the font stack.
10. The result matches docs/design/site-look.md#Decisions.

### 2 site-look-candidates (web)

Module: web. Size: M.
Order: after 1.
Files: `web/src/pages/candidates/CompanyStrip.tsx`, `web/src/pages/candidates/ShortList.tsx`, `web/src/pages/candidates/ShortListRows.tsx`, `web/src/pages/candidates/TierView.tsx`, `web/src/pages/candidates/TierRows.tsx`, `web/src/pages/candidates/TierLegend.tsx`, `web/src/pages/candidates/OptionCandidatesPage.tsx`, `web/test/pages/candidates-look.test.tsx`
Contracts: unchanged
Design: docs/design/site-look.md#What-you-see
Plan: P7
Acceptance:
1. `CompanyStrip.tsx` lays the companies out in a grid of at least three columns at 1280 px (`grid-cols` with `lg:grid-cols-3` or an auto-fit `minmax`) and one column at 390 px, each card with padding 10 px 12 px and the panel radius token; the company key is bold and the numbers use the `.num` class; the facts of the card come from the payload fields it reads today, and no field is read that the file does not read now.
2. `TierView.tsx` and `TierRows.tsx` render one table: a tier header row coloured with `--tier-*-soft` and the tier text colour for each of Fits, Check and Not judged, and each data row has a 3 px left border in the tier colour; `Not judged` groups stay collapsed header rows that open on click and on Enter and Space; the row detail still opens on click and on keyboard.
3. `TierLegend.tsx` renders the legend boxes in an auto-fit grid (`minmax(280px, 1fr)` or the Tailwind equivalent) with the title coloured per tier; every legend text and hint text is the payload string, unchanged, and the file has no string literal that is visible text.
4. The page head in `OptionCandidatesPage.tsx` shows the run day, the stored timestamp and the rule name as one mono line from the same payload fields (no three pills); no field is dropped.
5. A hint word keeps `tabindex="0"`, the dotted underline and shows its text on hover and on focus; the tooltip is 12 px with max width 240 px and is also set by `title` or `aria-describedby` so it is read by a screen reader.
6. Badges use the tier tokens and the 11 px weight 600 pill; no `border-black` or hard-coded hex colour is in the changed files (colours come from tokens).
7. `web/src/pages/candidates/**` has no new English or Chinese visible string literal (`web/test/english.test.ts` still holds), and no file is over 600 lines.
8. `web/test/pages/candidates-look.test.tsx` (new) renders the page from a fixture of `contracts/fixtures/` that has the three tiers and asserts (new): one `table`, three tier header rows, each data row has the tier stripe class, a company strip with three cards, and a `Not judged` header row that toggles `aria-expanded`.
9. The pull request description lists every payload field the strip or the row detail receives and does not show, and every label of the mockup that has no payload field (not renamed, not deleted).
10. The result matches docs/design/site-look.md#What-you-see.

## Checked by CI

- `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest` (including `web/test/english.test.ts` and the two new test files), the web build, and the frontend dependency check `scripts/check-frontend-deps.mjs` against `frontend-whitelist.json` and `web/deps-allowlist.txt` (this is where acceptance 1 and 2 of entry 1 are run).
- The `test` check reads `Contracts:` of the pull request body: `Contracts: unchanged` for both entries.
- `ui-shots.yml` renders each fixture at 390 and 1280 px and uploads the PNGs of the pull request; the reviewer opens them. The build output lists two `woff2` files (the size of the embedded dist is read from the build log).
- `crew issue check` of each entry against the head of the design pull request.
- After the merge, by the window and not an issue: a screenshot of the live `/options/candidates` and of one theme-only page beside the "After" half of the mockup, with each difference named; a check that no request to a font host appears when the page loads.

## Adversarial

to be filled

## Left out

- The layout of tracking, result, underlyings, home, household, notices, structures and valuation import: not cut now. Each gets a mockup shown to the owner first and its own design amendment. They get the fonts, sizes and colours of entry 1 with no entry.
- Moving English literals in `web/src` of the pages that have them (valuation, valuation params, settings, underlyings, base data, data health, options, job console, option data, structure, flex import, home and others) to Chinese Go payload text: owner rule 2026-11-17, a later design per page with that page's layout.
- Chinese hints for `/options/tracking` (the page has none today) and relabelling candidates columns to the mockup's words: a payload change in module pages after the screenshot step names the need.
- A Chinese web font: not done (megabytes; system Chinese fonts are used).
- Image comparison in CI (visual regression): not done; the owner's screenshot rule is a window step.
- Raising the 44 px touch rule for rows or table cells: not done; rows are not targets and buttons keep 44 px.
- The dark-first look of the later candidates page mockup and its Google Fonts link: not used.
