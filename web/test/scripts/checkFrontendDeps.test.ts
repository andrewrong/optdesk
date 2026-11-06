import { spawnSync } from 'node:child_process'
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { afterAll, describe, expect, it } from 'vitest'

const repo = resolve(__dirname, '..', '..', '..')
const script = join(repo, 'scripts', 'check-frontend-deps.mjs')
const temps: string[] = []

// A temporary copy of the files the checker reads; `edit` changes it before the run.
function run(edit: (dir: string) => void = () => {}) {
  const dir = mkdtempSync(join(tmpdir(), 'deps-'))
  temps.push(dir)
  mkdirSync(join(dir, 'web'))
  for (const f of ['frontend-whitelist.json', 'package.json', 'web/package.json', 'web/deps-allowlist.txt']) {
    cpSync(join(repo, f), join(dir, f))
  }
  edit(dir)
  return spawnSync('node', [script, dir], { encoding: 'utf8' })
}

function editWebPackage(dir: string, change: (pkg: Record<string, Record<string, string>>) => void) {
  const file = join(dir, 'web/package.json')
  const pkg = JSON.parse(readFileSync(file, 'utf8'))
  change(pkg)
  writeFileSync(file, JSON.stringify(pkg))
}

afterAll(() => {
  for (const d of temps) rmSync(d, { recursive: true, force: true })
})

describe('check-frontend-deps', () => {
  it('passes on a clean copy', () => {
    expect(run().status).toBe(0)
  })
  it('fails on a dependency that is not in the whitelist', () => {
    const r = run((d) => editWebPackage(d, (p) => (p.dependencies.lodash = '4.17.21')))
    expect(r.status).toBe(1)
    expect(r.stderr).toContain('lodash')
  })
  it('fails on a caret version', () => {
    const r = run((d) => editWebPackage(d, (p) => (p.dependencies.react = '^19.3.1')))
    expect(r.status).toBe(1)
  })
  it('fails on an @radix-ui name', () => {
    const r = run((d) => editWebPackage(d, (p) => (p.dependencies['@radix-ui/react-tabs'] = '1.0.0')))
    expect(r.status).toBe(1)
    expect(r.stderr).toContain('@radix-ui/react-tabs')
  })
  it('fails on a name missing from the allowlist', () => {
    const r = run((d) => {
      const f = join(d, 'web/deps-allowlist.txt')
      writeFileSync(f, readFileSync(f, 'utf8').replace(/^react-dom\n/m, ''))
    })
    expect(r.status).toBe(1)
    expect(r.stderr).toContain('react-dom')
  })
  it('fails when web/package.json is missing', () => {
    const r = run((d) => rmSync(join(d, 'web/package.json')))
    expect(r.status).toBe(1)
    expect(r.stderr).toContain('web/package.json')
  })
})
