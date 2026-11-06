// WCAG 2.x contrast helper (issue #1109 item 7). Expected values come from the W3C definition of relative
// luminance, L = 0.2126 R + 0.7152 G + 0.0722 B with c = ((s + 0.055) / 1.055) ^ 2.4 for s > 0.03928, and
// ratio = (L1 + 0.05) / (L2 + 0.05), hand-computed here and equal to the WebAIM contrast checker:
//   black on white:   (1 + 0.05) / (0 + 0.05)                = 21
//   #767676 on white: s = 118/255 = 0.46275, c = 0.18117, (1.05) / (0.18117 + 0.05) = 4.54
//   #777777 on white: s = 119/255 = 0.46667, c = 0.18447, (1.05) / (0.18447 + 0.05) = 4.48
import { describe, expect, it } from 'vitest'
import { contrast, parseColor } from './color'

describe('contrast', () => {
  it('black on white is 21:1', () => {
    expect(contrast('#000000', '#ffffff').toFixed(2)).toBe('21.00')
  })
  it('#767676 on white is 4.54:1', () => {
    expect(contrast('#767676', '#ffffff').toFixed(2)).toBe('4.54')
  })
  it('#777777 on white is 4.48:1', () => {
    expect(contrast('#777777', '#ffffff').toFixed(2)).toBe('4.48')
  })
  it('reads oklch like the browser does (white and black)', () => {
    expect(parseColor('oklch(1 0 0)')).toMatchObject({ r: 255, g: 255, b: 255, a: 1 })
    expect(parseColor('oklch(0 0 0)')).toMatchObject({ r: 0, g: 0, b: 0, a: 1 })
  })
  it('reads an oklch grey as sRGB (CSS Color 4 example: oklch(0.5 0 0) is #636363)', () => {
    // Source: oklab of a neutral grey, L^3 linear light = 0.125 -> sRGB 0.3899 -> 99.
    expect(parseColor('oklch(0.5 0 0)')).toMatchObject({ r: 99, g: 99, b: 99 })
  })
  it('composites a translucent colour over its background before measuring', () => {
    // white at 50% over black is 50% linear light mixed in sRGB: (255 + 0) / 2 = 127.5 -> 128
    expect(contrast('oklch(1 0 0 / 50%)', '#000000')).toBeCloseTo(contrast('#808080', '#000000'), 1)
  })
})
