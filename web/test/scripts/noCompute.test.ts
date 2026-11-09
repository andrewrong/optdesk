import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, relative, resolve } from 'node:path'
import { afterAll, describe, expect, it } from 'vitest'

const web = resolve(__dirname, '..', '..')
const patterns = ['toFixed', 'toLocale', 'Intl.', 'Math.']
const temps: string[] = []

function sourceFiles(dir: string, skip: string): string[] {
  const out: string[] = []
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name)
    if (p === skip) continue
    if (e.isDirectory()) out.push(...sourceFiles(p, skip))
    else out.push(p)
  }
  return out
}

// Throws when the allow file holds a line or a source file holds a computing call.
// The message names files and patterns only, never a line of content.
function enforce(root: string): void {
  const problems: string[] = []
  const allow = readFileSync(join(root, 'no-compute-allow.txt'), 'utf8')
  if (allow.split('\n').some((l) => l.trim() !== '')) {
    problems.push('no-compute-allow.txt holds a line')
  }
  const src = join(root, 'src')
  for (const f of sourceFiles(src, join(src, 'generated'))) {
    const text = readFileSync(f, 'utf8')
    for (const p of patterns) {
      if (text.includes(p)) problems.push(`${relative(root, f)}: ${p}`)
    }
  }
  if (problems.length > 0) throw new Error(problems.join('\n'))
}

function tempRoot(allow = ''): string {
  const dir = mkdtempSync(join(tmpdir(), 'nocompute-'))
  temps.push(dir)
  mkdirSync(join(dir, 'src', 'generated'), { recursive: true })
  writeFileSync(join(dir, 'no-compute-allow.txt'), allow)
  return dir
}

afterAll(() => {
  for (const d of temps) rmSync(d, { recursive: true, force: true })
})

describe('web no-compute rule', () => {
  it('holds for web/src and web/no-compute-allow.txt', () => {
    expect(() => enforce(web)).not.toThrow()
  })
  it('fails when no-compute-allow.txt holds a line', () => {
    expect(() => enforce(tempRoot('some/file.ts\n'))).toThrow('no-compute-allow')
  })
  it.each(patterns)('fails on %s outside generated', (p) => {
    const dir = tempRoot()
    writeFileSync(join(dir, 'src', 'a.ts'), `const x = ${p}\n`)
    expect(() => enforce(dir)).toThrow(p)
  })
  it('ignores src/generated', () => {
    const dir = tempRoot()
    writeFileSync(join(dir, 'src', 'generated', 'g.ts'), 'Math.max(1, 2)\n')
    expect(() => enforce(dir)).not.toThrow()
  })
  it('reports file and pattern only', () => {
    const dir = tempRoot()
    const marker = 'MARKER_LINE_CONTENT_7731'
    writeFileSync(join(dir, 'src', 'bad.ts'), `const ${marker} = (1).toFixed(2)\n`)
    let message = ''
    try {
      enforce(dir)
    } catch (e) {
      message = (e as Error).message
    }
    expect(message).toContain('bad.ts')
    expect(message).toContain('toFixed')
    expect(message).not.toContain(marker)
  })
})
