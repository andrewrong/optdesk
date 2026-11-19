# Theme mapping: the old report variables (`THEME` in `reports/render.py`) on the shadcn variables of `theme.css`

Source of truth: sections 2 (主题令牌) and 3 (组件映射表) of `docs/design/v2/design-system-shadcn.html`. Where the boards
are silent (the signal and banner colours) the values are those of `THEME` in `src/option_trader/reports/render.py`.
Light is `:root`, dark is `@media (prefers-color-scheme: dark)`; there is no toggle and no `data-theme`.

One row per old variable. "Text colour" and "on" are the new variables of the pair measured; the values are those of
the text colour. Contrast is the WCAG 2.x ratio (relative luminance, W3C), measured by `test/theme/color.ts`, which the
test `test/theme/theme.test.ts` runs against this table and against `theme.css`. Kind: `text` needs 4.5:1 (3:1 for large
text), `decor` is a divider or card edge that no control relies on and carries no floor.

| Old variable          | Text colour                   | On                            | Light              | Dark               | Contrast light | Contrast dark | Kind  | Note                                                                                                                             |
| --------------------- | ----------------------------- | ----------------------------- | ------------------ | ------------------ | -------------- | ------------- | ----- | -------------------------------------------------------------------------------------------------------------------------------- |
| `--background`        | `--background`                | `--foreground`                | `#f6f7f9` | `#10151c` | 14.60 | 15.02 | text  | page background: the candidates mockup, pale blue-grey (the ratio is against --foreground)   |
| `--surface`           | `--card`                      | `--background`                | `#ffffff` | `#171e28` | 1.07 | 1.09 | decor | cards and table cells; the card edge is the --border line, so no ratio is required                                               |
| `--text`              | `--foreground`                | `--background`                | `#1b2430` | `#e4e9f0` | 14.60 | 15.02 | text  |                                                                                                                                  |
| `--muted-text`        | `--muted-foreground`          | `--background`                | `#5f6c7e` | `#8f9db0` | 4.98 | 6.65 | text  | mockup #667385 darkened to #5f6c7e: 4.98 on the page and 4.63 on --muted (docs/design/site-look.md Decision 6)                                                          |
| `--border`            | `--border`                    | `--background`                | `#e3e7ed` | `#283140` | 1.16 | 1.40 | decor | divider line; not a control boundary, so no ratio is required (WCAG 1.4.11 applies to boundaries a user needs to find a control) |
| `--accent`            | `--chart-1`                   | `--background`                | `#155e9a` | `#8cc7fa` | 6.32 | 10.18 | text  | old link and chart-line blue; the shadcn --accent is a hover surface and is not the same thing                                   |
| `--ok`                | `--ok-foreground`             | `--ok`                        | `#14794a` | `#5fd39a` | 4.76 | 7.33 | text  | badge text on its tint; --ok is the tint                                                                                         |
| `--hold`              | `--hold-foreground`           | `--hold`                      | `#b3261e` | `#f08a82` | 5.38 | 6.33 | text  | badge text on its tint; --hold is the tint                                                                                       |
| `--unknown`           | `--unknown-foreground`        | `--unknown`                   | `#8a5a00` | `#e8b85a` | 5.25 | 7.24 | text  | badge text on its tint; --unknown is the tint                                                                                    |
| `--muted`             | `--muted-foreground`          | `--muted`                     | `#5f6c7e` | `#8f9db0` | 4.63 | 4.97 | text  | old --muted (grey of st-muted) and --muted-text were the same role; both map to --muted-foreground                               |
| `--banner-normal-bg`  | `--banner-normal`             | `--banner-normal-foreground`  | `#e3f2e8`          | `#173824`          | 9.23           | 10.06         | text  | background is --banner-normal                                                                                                    |
| `--banner-normal-fg`  | `--banner-normal-foreground`  | `--banner-normal`             | `#14472a`          | `#bfeed2`          | 9.23           | 10.06         | text  |                                                                                                                                  |
| `--banner-partial-bg` | `--banner-partial`            | `--banner-partial-foreground` | `#fbefd0`          | `#3d2f0a`          | 8.82           | 9.72          | text  | background is --banner-partial                                                                                                   |
| `--banner-partial-fg` | `--banner-partial-foreground` | `--banner-partial`            | `#5c3b00`          | `#f6dc9c`          | 8.82           | 9.72          | text  |                                                                                                                                  |
| `--banner-stopped-bg` | `--banner-stopped`            | `--banner-stopped-foreground` | `#a52636`          | `#6e1a26`          | 7.17           | 9.32          | text  | background is --banner-stopped                                                                                                   |
| `--banner-stopped-fg` | `--banner-stopped-foreground` | `--banner-stopped`            | `#ffffff`          | `#ffe1e4`          | 7.17           | 9.32          | text  |                                                                                                                                  |

## New tokens, font stacks and text scale (docs/design/site-look.md Decisions 1, 4, 6)

Rows that `test/theme/theme.test.ts` parses against `theme.css`: the first cell is the kind, the value cells are the light
and dark values (the tier tokens resolve through `--success`, `--warn`, `--ok` and `--unknown`).

| Kind  | Name                 | Light                                                                                                                | Dark                      |
| ----- | -------------------- | -------------------------------------------------------------------------------------------------------------------- | ------------------------- |
| tier  | `--tier-fit`         | `#14794a`                                                                                                            | `#5fd39a`                 |
| tier  | `--tier-fit-soft`    | `#e3f4ea`                                                                                                            | `#173326`                 |
| tier  | `--tier-check`       | `#8a5a00`                                                                                                            | `#e8b85a`                 |
| tier  | `--tier-check-soft`  | `#fdf0d5`                                                                                                            | `#3a2e12`                 |
| tier  | `--tier-grey`        | `#566275`                                                                                                            | `#a3b0c2`                 |
| tier  | `--tier-grey-soft`   | `#eceff3`                                                                                                            | `#252e3b`                 |
| font  | `--font-sans`        | `"Inter Variable", Inter, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC", system-ui, sans-serif` | same                      |
| font  | `--font-mono`        | `"JetBrains Mono Variable", "JetBrains Mono", ui-monospace, Menlo, monospace`                                        | same                      |
| scale | `--text-xs`          | `0.6875rem`                                                                                                          | same                      |
| scale | `--text-sm`          | `0.8125rem`                                                                                                          | same                      |
| scale | `--text-base`        | `0.875rem`                                                                                                           | same                      |
| scale | `--text-lg`          | `0.9375rem`                                                                                                          | same                      |
| scale | `--text-xl`          | `1.0625rem`                                                                                                          | same                      |
| scale | `--text-2xl`         | `1.375rem`                                                                                                           | same                      |

## Where the values differ from the v2 boards

- Issue #1769 (superseded by #3214 for the values): `--warn` and `--down` are tokens, and `--warning` and `--destructive` point at them. The badge tints are 10% in both themes.
- Site look (issue #3214): every surface, text and line token is the hex of the candidates mockup (Decision 6); `--ring` is the accent (`#1f5fbf` / `#7aa8f0`); `--primary` is the accent. `--chart-*` and `--sidebar-*` keep their values.
- `--chart-1` is the old report blue (`#155e9a` / `#8cc7fa`); the board's grey `0.87` is 1.3:1 on white and cannot draw a line.
  `--chart-2..5` are greys, darker in light and (reversed order) lighter in dark, so every series is at least 3:1 on the page.
- `--destructive-foreground` is added (Nova has none): the text on a solid destructive fill.
- The signal tokens `--ok`, `--hold`, `--unknown` are tints (12% of the old colour over the card), the old colour is the `-foreground`.
