// @vitest-environment node
// ui-english: every user-visible text of web/src (and the fallback of index.html) is English. The only
// CJK that may stay are the four published market_weather.v1 values, frozen by their schema.
// The values are written as \u escapes so that this file holds no CJK itself.
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { describe, expect, it } from 'vitest'

const allowedCJK = [
  '\u6536\u76d8', // close
  '\u76d8\u4e2d\u8bd5\u7b97', // intraday estimate
  '\u76d8\u4e2d', // intraday
  '\u76d8\u4e2d\u00b7\u5ef6\u8fdf', // intraday, delayed
]

// ideographs, CJK punctuation and full-width forms
const cjk = /[\u4e00-\u9fff\u3000-\u303f\uff00-\uffef]/

const root = join(import.meta.dirname, '..')

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const path = join(dir, e.name)
    if (e.isDirectory()) {
      return path === join(root, 'src', 'generated') || path === join(root, 'src', 'types') ? [] : sources(path)
    }
    return /\.(ts|tsx|css)$/.test(e.name) ? [path] : []
  })
}

// Removes each allowed value only where it is a whole quoted value.
function withoutAllowed(line: string): string {
  return allowedCJK.reduce((s, v) => s.replace(new RegExp(`(["'\`])${v}\\1`, 'g'), ''), line)
}

describe('English only', () => {
  it('web/src and web/index.html hold no CJK outside allowedCJK', () => {
    const files = [...sources(join(root, 'src')), join(root, 'index.html')]
    expect(files.length).toBeGreaterThan(50)
    const hits = files.flatMap((f) =>
      readFileSync(f, 'utf8')
        .split('\n')
        .flatMap((line, i) => (cjk.test(withoutAllowed(line)) ? [`${relative(root, f)}:${i + 1}: ${line.trim()}`] : [])),
    )
    expect(hits).toEqual([])
  })

  it('a quoted allowed value passes, any other CJK fails', () => {
    expect(cjk.test(withoutAllowed("label: '\u6536\u76d8'"))).toBe(false)
    expect(cjk.test(withoutAllowed('x === "\u76d8\u4e2d\u00b7\u5ef6\u8fdf"'))).toBe(false)
    expect(cjk.test(withoutAllowed("label: '\u6536\u76d8\u4ef7'"))).toBe(true)
    expect(cjk.test(withoutAllowed('// \u6536\u76d8'))).toBe(true)
    expect(cjk.test(withoutAllowed('a\uff0cb'))).toBe(true)
  })
})
