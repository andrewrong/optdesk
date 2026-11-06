# Sources of the copied shadcn components

Copied on 2026-10-23 with shadcn CLI 4.22.1, style `radix-nova` (Nova, `--base radix`), from
`https://ui.shadcn.com/r/styles/radix-nova/<name>.json`. The registry publishes no revision number, so the
revision recorded is the first 16 hex digits of the SHA-256 of the registry item JSON as retrieved that day; an
upgrade is a deliberate issue that retrieves the item again and diffs the copied file. Licence of every row:
MIT (shadcn-ui/ui, Copyright (c) 2023 shadcn). Every component adds no package beyond `radix-ui` (already
installed) and the helpers `class-variance-authority`, `clsx`, `tailwind-merge` and `lucide-react`.

Local edit to all rows: the registry imports `cn` from an npm package named `cn`; the copied files import it
from `@/lib/utils` instead (ADR 0034: `clsx` and `tailwind-merge` are the designated helpers).

| Component | File | Registry revision (SHA-256, 16 hex) | Licence |
|---|---|---|---|
| button | `button.tsx` | `c07a8feb7b7ed07a` | MIT |
| badge | `badge.tsx` | `4c7eae206938e553` | MIT |
| card | `card.tsx` | `acf04c936ab6b613` | MIT |
| table | `table.tsx` | `99e6991f093e71db` | MIT |
| alert | `alert.tsx` | `aaa6f7ee84101a00` | MIT |
| accordion | `accordion.tsx` | `203a172c0e1d1745` | MIT |
| toggle | `toggle.tsx` | `1f6119c5ff39981a` | MIT |
| tabs | `tabs.tsx` | `b91ac50256d7de55` | MIT |
| separator | `separator.tsx` | `1df209eff67aeefa` | MIT |
| tooltip | `tooltip.tsx` | `c639f7ee0511b6d2` | MIT |
| skeleton | `skeleton.tsx` | `f956c1b1a8425265` | MIT |
| scroll-area | `scroll-area.tsx` | `19a663a8a054c16b` | MIT |
| progress | `progress.tsx` | not retrieved (see below) | MIT |

Not copied (owner-only, they add packages or pull a second library): chart, calendar, sonner, drawer, command, carousel, form, input-otp.

Added by #1109: `progress` is the registry's `radix-nova` Progress written by hand over `radix-ui`'s `Progress` (the shadcn CLI could not run in the worker's sandbox, so no registry revision was recorded); a later issue retrieves the item and diffs it. Local edits by #1109: `badge.tsx` gains the signal variants (ok, hold, unknown, banner colours) that `theme.css` defines; `accordion.tsx` and `skeleton.tsx` gain `motion-reduce:animate-none`; `tooltip.tsx` has `motion-safe:` on its `animate-in` and `animate-out` classes (a `motion-reduce:` utility loses to the state selectors).

Added by #15: `chart.tsx` is hand-written in the shape of the `radix-nova` Chart over `recharts` 3.10.1 (the shadcn registry was unreachable, so no registry revision was recorded); a later issue retrieves the item and diffs it. It replaces echarts (ADR 0034, owner's choice 2026-11-03).
