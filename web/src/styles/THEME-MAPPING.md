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
| `--background`        | `--background`                | `--foreground`                | `oklch(1 0 0)`     | `oklch(0.145 0 0)` | 19.80          | 18.97         | text  | page background: v2 board, white / neutral 0.145; the old grey page behind white cards is gone (page and card are one surface)   |
| `--surface`           | `--card`                      | `--background`                | `oklch(1 0 0)`     | `oklch(0.205 0 0)` | 1.00           | 1.10          | decor | cards and table cells; the card edge is the --border line, so no ratio is required                                               |
| `--text`              | `--foreground`                | `--background`                | `oklch(0.145 0 0)` | `oklch(0.985 0 0)` | 19.80          | 18.97         | text  |                                                                                                                                  |
| `--muted-text`        | `--muted-foreground`          | `--background`                | `hsl(0 0% 42%)`    | `hsl(0 0% 66%)`    | 5.33           | 8.33          | text  | light value 0.52 instead of the board 0.556, which is 4.35:1 on --muted                                                          |
| `--border`            | `--border`                    | `--background`                | `hsl(0 0% 89.8%)`  | `hsl(0 0% 17%)`    | 1.26           | 1.40          | decor | divider line; not a control boundary, so no ratio is required (WCAG 1.4.11 applies to boundaries a user needs to find a control) |
| `--accent`            | `--chart-1`                   | `--background`                | `#155e9a`          | `#8cc7fa`          | 6.78           | 11.00         | text  | old link and chart-line blue; the shadcn --accent is a hover surface and is not the same thing                                   |
| `--ok`                | `--ok-foreground`             | `--ok`                        | `#21653c`          | `#8bdbab`          | 5.88           | 8.45          | text  | badge text on its tint; --ok is the tint                                                                                         |
| `--hold`              | `--hold-foreground`           | `--hold`                      | `#a52636`          | `#ffabb3`          | 5.87           | 7.86          | text  | badge text on its tint; --hold is the tint                                                                                       |
| `--unknown`           | `--unknown-foreground`        | `--unknown`                   | `#815400`          | `#f0ca75`          | 5.48           | 8.86          | text  | badge text on its tint; --unknown is the tint                                                                                    |
| `--muted`             | `--muted-foreground`          | `--muted`                     | `hsl(0 0% 42%)`    | `hsl(0 0% 66%)`    | 4.89           | 6.36          | text  | old --muted (grey of st-muted) and --muted-text were the same role; both map to --muted-foreground                               |
| `--banner-normal-bg`  | `--banner-normal`             | `--banner-normal-foreground`  | `#e3f2e8`          | `#173824`          | 9.23           | 10.06         | text  | background is --banner-normal                                                                                                    |
| `--banner-normal-fg`  | `--banner-normal-foreground`  | `--banner-normal`             | `#14472a`          | `#bfeed2`          | 9.23           | 10.06         | text  |                                                                                                                                  |
| `--banner-partial-bg` | `--banner-partial`            | `--banner-partial-foreground` | `#fbefd0`          | `#3d2f0a`          | 8.82           | 9.72          | text  | background is --banner-partial                                                                                                   |
| `--banner-partial-fg` | `--banner-partial-foreground` | `--banner-partial`            | `#5c3b00`          | `#f6dc9c`          | 8.82           | 9.72          | text  |                                                                                                                                  |
| `--banner-stopped-bg` | `--banner-stopped`            | `--banner-stopped-foreground` | `#a52636`          | `#6e1a26`          | 7.17           | 9.32          | text  | background is --banner-stopped                                                                                                   |
| `--banner-stopped-fg` | `--banner-stopped-foreground` | `--banner-stopped`            | `#ffffff`          | `#ffe1e4`          | 7.17           | 9.32          | text  |                                                                                                                                  |

## Where the values differ from the v2 boards

- Issue #1769: `--success`, `--muted`, `--muted-foreground` and `--border` take the `:root` values of docs/design/v2/design-set-shadcn.html as `hsl(...)`; `--warn` and `--down` are new, and `--warning` and `--destructive` point at them. The badge tints are 10% in both themes (the design's `down` is 4.29:1 on a 20% dark tint).
- `--ring` light is `oklch(0.55 0 0)`, the board has `0.708` (2.58:1 on white, under the 3:1 a focus indicator needs).
- `--chart-1` is the old report blue (`#155e9a` / `#8cc7fa`); the board's grey `0.87` is 1.3:1 on white and cannot draw a line.
  `--chart-2..5` are greys, darker in light and (reversed order) lighter in dark, so every series is at least 3:1 on the page.
- `--destructive-foreground` is added (Nova has none): the text on a solid destructive fill.
- The signal tokens `--ok`, `--hold`, `--unknown` are tints (12% of the old colour over the card), the old colour is the `-foreground`.
