// Issue #1088: web/index.html carries the exact vm element that internal/pages/embed/inject.go
// (vmElement) requires once, so `ot serve` can inject the payload.
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const VM_ELEMENT = '<script id="vm" type="application/json"></script>'

describe('web/index.html', () => {
  it('holds the vm element of inject.go exactly once', () => {
    const html = readFileSync(resolve(__dirname, '../../index.html'), 'utf8')
    expect(html.split(VM_ELEMENT).length - 1).toBe(1)
    expect(html.split('id="vm"').length - 1).toBe(1)
  })
})
