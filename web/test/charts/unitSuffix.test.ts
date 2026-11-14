import { describe, expect, it } from 'vitest'
import { unitSuffix } from '@/charts/unitSuffix'

describe('unitSuffix', () => {
  it.each([
    ['percent', '%'],
    ['percentage_points', ' pp'],
    ['basis_points', ' bp'],
    ['index_points', ''],
    ['price', ''],
    ['count', ''],
    ['ratio', ''],
    ['usd_signed', ''],
    ['iv_decimal', ''],
    ['delta', ''],
    ['gamma', ''],
    ['theta', ''],
    ['vega', ''],
    ['no_such_unit', ''],
    ['', ''],
  ])('%s gives %j', (id, suffix) => {
    expect(unitSuffix(id)).toBe(suffix)
  })

  it('never returns an id for an inherited object key', () => {
    expect(unitSuffix('constructor')).toBe('')
    expect(unitSuffix('toString')).toBe('')
  })
})
