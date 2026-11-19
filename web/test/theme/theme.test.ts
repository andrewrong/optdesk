// @vitest-environment node
// Issue #1109 items 1, 2 and 7: the theme variables, the mapping table and the contrast of every pair.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { contrast, parseColor } from './color'

const read = (path: string) =>
  readFileSync(new URL(`../../src/styles/${path}`, import.meta.url), 'utf8')
const css = read('theme.css')
const mapping = read('THEME-MAPPING.md')

// Variables of issue #1109 item 1 (the shadcn set, the chart colours, the signal tokens).
const SHADCN = [
  'background', 'foreground', 'card', 'card-foreground', 'popover', 'popover-foreground', 'primary',
  'primary-foreground', 'secondary', 'secondary-foreground', 'muted', 'muted-foreground', 'accent',
  'accent-foreground', 'destructive', 'destructive-foreground', 'border', 'input', 'ring', 'chart-1',
  'chart-2', 'chart-3', 'chart-4', 'chart-5',
] // prettier-ignore
const SIGNALS = [
  'ok', 'ok-foreground', 'hold', 'hold-foreground', 'unknown', 'unknown-foreground', 'banner-normal',
  'banner-normal-foreground', 'banner-partial', 'banner-partial-foreground', 'banner-stopped',
  'banner-stopped-foreground', 'success', 'warning',
] // prettier-ignore
// Written from THEME in src/option_trader/reports/render.py (the variable names of its two blocks).
const OLD_THEME = [
  '--background', '--surface', '--text', '--muted-text', '--border', '--accent', '--ok', '--hold',
  '--unknown', '--muted', '--banner-normal-bg', '--banner-normal-fg', '--banner-partial-bg',
  '--banner-partial-fg', '--banner-stopped-bg', '--banner-stopped-fg',
] // prettier-ignore

function declarations(block: string): Map<string, string> {
  const found = new Map<string, string>()
  for (const m of block.matchAll(/^\s*--([a-z0-9-]+):\s*([^;]+);/gm))
    found.set(m[1] as string, (m[2] as string).trim())
  return found
}
const darkStart = css.indexOf('@media (prefers-color-scheme: dark) {\n  :root {')
const rootStart = css.indexOf(':root {')
const light = declarations(css.slice(rootStart, darkStart))
const dark = declarations(css.slice(darkStart))
const value = (theme: 'light' | 'dark', name: string): string => {
  const raw = (theme === 'dark' ? (dark.get(name) ?? light.get(name)) : light.get(name)) ?? ''
  const ref = /^var\(--([a-z0-9-]+)\)$/.exec(raw) // warning and destructive point at warn and down
  return ref ? value(theme, ref[1] as string) : raw
}

describe('theme.css', () => {
  it.each([...SHADCN, ...SIGNALS])('defines --%s for light and dark', (name) => {
    expect(light.get(name), `light ${name}`).toBeTruthy()
    expect(dark.get(name), `dark ${name}`).toBeTruthy()
  })
  it('defines the radius', () => {
    expect(light.get('radius')).toBe('0.5rem')
  })
  // Source: docs/design/site-look.md Decision 6 (the candidates mockup), light / dark.
  it.each([
    ['background', '#f6f7f9', '#10151c'],
    ['card', '#ffffff', '#171e28'],
    ['foreground', '#1b2430', '#e4e9f0'],
    ['muted-foreground', '#5f6c7e', '#8f9db0'],
    ['border', '#e3e7ed', '#283140'],
    ['primary', '#1f5fbf', '#7aa8f0'],
    ['success', '#14794a', '#5fd39a'],
    ['ok', '#e3f4ea', '#173326'],
    ['warn', '#8a5a00', '#e8b85a'],
    ['unknown', '#fdf0d5', '#3a2e12'],
    ['down', '#b3261e', '#f08a82'],
    ['hold', '#fbe4e2', '#3d1b19'],
    ['muted', '#eceff3', '#252e3b'],
  ])('sets --%s to the design values', (name, lightValue, darkValue) => {
    expect(light.get(name)).toBe(lightValue)
    expect(dark.get(name)).toBe(darkValue)
  })
  it('keeps no oklch or hsl for a surface, text or line token', () => {
    for (const theme of [light, dark])
      for (const [name, v] of theme)
        if (!/^(chart-|sidebar)/.test(name)) expect(v, name).not.toMatch(/oklch\(|hsl\(/)
  })
  it('maps warning and destructive to the design tokens and registers warn and down as colours', () => {
    expect(light.get('warning')).toBe('var(--warn)')
    expect(light.get('destructive')).toBe('var(--down)')
    expect(css).toContain('--color-warn: var(--warn);')
    expect(css).toContain('--color-down: var(--down);')
  })
  it('follows prefers-color-scheme only: no toggle, no data-theme', () => {
    expect(css).toContain('@custom-variant dark (@media (prefers-color-scheme: dark));')
    expect(css).not.toMatch(/data-theme|\.dark\s*\{/)
  })
  it('sets the 44 px target and the 14 px body', () => {
    expect(css).toContain('--target-min: 2.75rem;') // 2.75 * 16 = 44
    expect(css).not.toMatch(/--text-[a-z0-9]+:\s*1rem;/)
    expect(css).toMatch(/body\s*\{[^}]*font-size:\s*0\.875rem;[^}]*line-height:\s*1\.45/)
  })
  it('keeps the 44 px minimum height of button, toggle, tabs and accordion', () => {
    for (const slot of ['button', 'toggle', 'tabs-trigger', 'accordion-trigger'])
      expect(css).toContain(`[data-slot='${slot}']`)
    expect(css).toContain('min-height: var(--target-min);')
  })
  it('sets tabular numbers for tables and .num, and .num is mono 13 px', () => {
    expect(css).toMatch(/table,\s*td,\s*th,\s*\.num\s*\{\s*font-variant-numeric:\s*tabular-nums;/)
    expect(css).toMatch(/\.num\s*\{\s*font-family:\s*var\(--font-mono\);\s*font-size:\s*0\.8125rem;/)
  })
})

describe('new theme tokens', () => {
  it('has the six tier tokens in light and dark, no URL import, and a 14 px --text-base', () => {
    for (const name of ['fit', 'check', 'grey'])
      for (const suffix of ['', '-soft']) {
        expect(light.get(`tier-${name}${suffix}`), `light ${name}${suffix}`).toBeTruthy()
        expect(dark.get(`tier-${name}${suffix}`), `dark ${name}${suffix}`).toBeTruthy()
      }
    for (const m of css.matchAll(/@import[^;]*;/g)) expect(m[0]).not.toContain('http')
    expect(css).not.toContain('@font-face')
    expect(css).toMatch(/--text-base:\s*0\.875rem;/)
  })
})

interface Row {
  old: string
  fg: string
  bg: string
  lightValue: string
  darkValue: string
  lightRatio: string
  darkRatio: string
  kind: string
}
const rows: Row[] = mapping
  .split('\n')
  .filter((line) => /^\|\s*`--/.test(line)) // the old-variable table; the new-token table starts with a kind word
  .map((line) => {
    const c = line
      .split('|')
      .slice(1, -1)
      .map((s) => s.trim().replace(/`/g, ''))
    return {
      old: c[0]!,
      fg: c[1]!,
      bg: c[2]!,
      lightValue: c[3]!,
      darkValue: c[4]!,
      lightRatio: c[5]!,
      darkRatio: c[6]!,
      kind: c[7]!,
    }
  })

describe('THEME-MAPPING.md', () => {
  it('has one row for every old THEME variable', () => {
    expect(rows.map((r) => r.old).sort()).toEqual([...OLD_THEME].sort())
  })
  it.each(rows)('row $old: values, ratios and the WCAG floor are those of theme.css', (row) => {
    for (const theme of ['light', 'dark'] as const) {
      const fg = value(theme, row.fg.slice(2))
      const bg = value(theme, row.bg.slice(2))
      expect(theme === 'light' ? row.lightValue : row.darkValue).toBe(fg)
      const ratio = contrast(fg, bg)
      expect(Number(theme === 'light' ? row.lightRatio : row.darkRatio)).toBeCloseTo(ratio, 1)
      if (row.kind === 'text') expect(ratio, `${row.old} ${theme}`).toBeGreaterThanOrEqual(4.5)
      if (row.kind === 'ui') expect(ratio, `${row.old} ${theme}`).toBeGreaterThanOrEqual(3)
    }
  })
})

// Pairs that are not in the old THEME but are used by the sheet: text 4.5, graphics and focus 3.
const extraText: [string, string][] = [
  ['foreground', 'background'], ['card-foreground', 'card'], ['popover-foreground', 'popover'],
  ['primary-foreground', 'primary'], ['secondary-foreground', 'secondary'], ['accent-foreground', 'accent'],
  ['muted-foreground', 'background'], ['muted-foreground', 'card'], ['muted-foreground', 'muted'],
  ['destructive', 'background'], ['destructive', 'card'], ['success', 'background'], ['success', 'card'],
  ['warning', 'background'], ['warning', 'card'], ['ok-foreground', 'card'], ['hold-foreground', 'card'],
  ['unknown-foreground', 'card'], ['destructive-foreground', 'destructive'],
] // prettier-ignore
const extraUi: [string, string][] = [
  ['ring', 'background'], ['chart-1', 'background'], ['chart-2', 'background'], ['chart-3', 'background'],
  ['chart-4', 'background'], ['chart-5', 'background'],
] // prettier-ignore

describe('contrast of the other pairs', () => {
  for (const theme of ['light', 'dark'] as const) {
    it.each(extraText)(`${theme}: --%s on --%s is at least 4.5:1`, (fg, bg) => {
      expect(contrast(value(theme, fg), value(theme, bg))).toBeGreaterThanOrEqual(4.5)
    })
    it.each(extraUi)(`${theme}: --%s on --%s is at least 3:1`, (fg, bg) => {
      expect(contrast(value(theme, fg), value(theme, bg))).toBeGreaterThanOrEqual(3)
    })
    it(`${theme}: success, warning and destructive stay legible on their badge tint`, () => {
      // badge.tsx: success and warning use 10% in both themes, destructive 10% (issue #1769: the dark 20% tint was 4.29:1 on the design's down).
      const tints: Array<[string, number]> = [['success', 0.1], ['warning', 0.1], ['destructive', 0.1]] // prettier-ignore
      for (const [name, alpha] of tints) {
        const front = parseColor(value(theme, name))
        const back = parseColor(value(theme, 'card'))
        const mix = (f: number, b: number) => Math.round(f * alpha + b * (1 - alpha))
        const tint = `#${[mix(front.r, back.r), mix(front.g, back.g), mix(front.b, back.b)].map((n) => n.toString(16).padStart(2, '0')).join('')}`
        expect(contrast(value(theme, name), tint), name).toBeGreaterThanOrEqual(4.5)
      }
    })
  }
})

// The new-token table of THEME-MAPPING.md: tier colours, the two font stacks and the text scale.
const newRows = mapping
  .split('\n')
  .filter((line) => /^\|\s*(tier|font|scale)\s*\|/.test(line))
  .map((line) => line.split('|').slice(1, -1).map((s) => s.trim().replace(/^`|`$/g, '')))

describe('THEME-MAPPING.md new tokens', () => {
  it('lists the six tier tokens, both font stacks and the six text sizes', () => {
    expect(newRows.map((r) => r[0])).toEqual([...Array(6).fill('tier'), 'font', 'font', ...Array(6).fill('scale')])
  })
  it.each(newRows)('%s %s: values are those of theme.css', (kind, name, lightValue, darkValue) => {
    const key = (name as string).slice(2)
    if (kind === 'tier') {
      expect(value('light', key)).toBe(lightValue?.replace(/`/g, ''))
      expect(value('dark', key)).toBe(darkValue?.replace(/`/g, ''))
    } else {
      const decl = new RegExp(`^\\s*--${key}:\\s*([^;]+);`, 'm').exec(css)
      expect(decl?.[1]?.trim()).toBe(lightValue?.replace(/`/g, ''))
    }
  })
})
