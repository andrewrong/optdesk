// @vitest-environment node
// ui-english: every user-visible text of web/src (and the fallback of index.html) is English. The only
// CJK that may stay are the four published market_weather.v1 values, frozen by their schema.
// The values are written as \u escapes so that this file holds no CJK itself.
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { describe, expect, it } from 'vitest'

const allowedCJK = [
  '\u5de6\u53f3\u6ed1\u52a8\u53ef\u770b\u5168\u90e8\u5217', // swipe sideways to see every column (ScrollHint)
  '\u6536\u76d8', // close
  '\u76d8\u4e2d\u8bd5\u7b97', // intraday estimate
  '\u76d8\u4e2d', // intraday
  '\u76d8\u4e2d\u00b7\u5ef6\u8fdf', // intraday, delayed
  '\u9664\u606f\u65e5\u5217\u6682\u65e0\u6570\u636e\uff1a\u5f53\u524d\u6ca1\u6709\u53ef\u7528\u7684\u9664\u606f\u65e5\u6765\u6e90\uff0c\u5df2\u9690\u85cf\u3002', // ex-dividend column hidden: no data source
  '\u672c\u5468\u6ca1\u6709\u9700\u8981\u51b3\u5b9a\u7684\u7ed3\u6784\u3002', // no structure to decide this week
  '\u6ca1\u6709\u5176\u4ed6\u7ed3\u6784\u3002', // no other structures
  '\u65e0\u8d22\u62a5\u65e5\u671f\uff08\u6307\u6570\u6216\u672a\u767b\u8bb0\u53d1\u884c\u4eba\uff09', // no earnings date (index or issuer not registered)
  '\u5230\u91c7\u96c6\u8bb0\u5f55\u9875\u5904\u7406', // go to the collection log page to handle it (repair link of the base data page)
  '\u6ca1\u6709\u672a\u5e73\u4ed3\u7684\u5356\u51fa\u770b\u8dcc\uff0c\u4e0d\u9700\u8981\u73b0\u91d1\u3002', // no short put open: no cash due
  '\u6ca1\u6709\u671f\u6743\u94fe\u6570\u636e', // no chain data
  '\u6570\u636e\u683c\u5f0f\u4e0e\u7ea6\u5b9a\u4e0d\u7b26\uff0c\u8bf7\u4e0d\u8981\u636e\u6b64\u64cd\u4f5c\u3002', // option data payload does not match its schema
  '\u6ca1\u6709\u5b58\u6536\u76d8\u4ef7\uff0c\u6240\u4ee5\u6ca1\u6709\u4ef7\u683c\u533a\u95f4\u3002', // no close price stored: no zone
  '\u8fd9\u4e2a\u5e02\u573a\u6ca1\u6709\u4ef7\u683c\u6765\u6e90\uff0c\u6240\u4ee5\u6ca1\u6709\u4ef7\u683c\u533a\u95f4\u3002', // market without a price source: no zone
  '\u6bcf\u884c\u4e00\u6863\uff1a\u6536\u76ca\u7387\u4e0a\u9650\u3001US\u3001JP\u3001HKCN', // R table hint: one tier per row
  '\u6bcf\u884c\u4e00\u6863\uff1aP \u8d77\u70b9\u3001M\u3001W\u3001H\u3001S', // margin table hint: one tier per row
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
