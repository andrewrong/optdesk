// Colour parsing and WCAG 2.x contrast for the theme tests. Written once (issue #1109 item 7).
export interface Rgba {
  r: number
  g: number
  b: number
  a: number
}

const clamp = (x: number) => Math.min(1, Math.max(0, x))
const encode = (linear: number) =>
  Math.round(
    255 * clamp(linear <= 0.0031308 ? 12.92 * linear : 1.055 * linear ** (1 / 2.4) - 0.055),
  )

// oklch -> oklab -> linear sRGB (Björn Ottosson's matrices), out-of-gamut channels clamped.
function fromOklch(l: number, c: number, hDeg: number, a: number): Rgba {
  const h = (hDeg * Math.PI) / 180
  const A = c * Math.cos(h)
  const B = c * Math.sin(h)
  const lp = (l + 0.3963377774 * A + 0.2158037573 * B) ** 3
  const mp = (l - 0.1055613458 * A - 0.0638541728 * B) ** 3
  const sp = (l - 0.0894841775 * A - 1.291485548 * B) ** 3
  return {
    r: encode(4.0767416621 * lp - 3.3077115913 * mp + 0.2309699292 * sp),
    g: encode(-1.2684380046 * lp + 2.6097574011 * mp - 0.3413193965 * sp),
    b: encode(-0.0041960863 * lp - 0.7034186147 * mp + 1.707614701 * sp),
    a,
  }
}

export function parseColor(text: string): Rgba {
  const value = text.trim()
  const hex = /^#([0-9a-f]{6})$/i.exec(value)
  if (hex) {
    const n = parseInt(hex[1] as string, 16)
    return { r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255, a: 1 }
  }
  const ok = /^oklch\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*(?:\/\s*([\d.]+)(%?))?\s*\)$/.exec(value)
  if (ok) {
    const alpha = ok[4] === undefined ? 1 : ok[5] ? Number(ok[4]) / 100 : Number(ok[4])
    return fromOklch(Number(ok[1]), Number(ok[2]), Number(ok[3]), alpha)
  }
  const hs = /^hsl\(\s*([\d.]+)\s+([\d.]+)%\s+([\d.]+)%\s*\)$/.exec(value)
  if (hs) {
    const [h, sat, lig] = [Number(hs[1]), Number(hs[2]) / 100, Number(hs[3]) / 100]
    const a = sat * Math.min(lig, 1 - lig)
    const f = (n: number) => {
      const k = (n + h / 30) % 12
      return Math.round(255 * (lig - a * Math.max(-1, Math.min(k - 3, 9 - k, 1))))
    }
    return { r: f(0), g: f(8), b: f(4), a: 1 }
  }
  throw new Error(`unsupported colour: ${text}`)
}

const channel = (s: number) => {
  const v = s / 255
  return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
}

function luminance({ r, g, b }: Rgba): number {
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)
}

function over(top: Rgba, bottom: Rgba): Rgba {
  const mix = (t: number, u: number) => Math.round(t * top.a + u * (1 - top.a))
  return { r: mix(top.r, bottom.r), g: mix(top.g, bottom.g), b: mix(top.b, bottom.b), a: 1 }
}

/** WCAG 2.x contrast ratio of `foreground` on `background` (a translucent foreground is composited first). */
export function contrast(foreground: string, background: string): number {
  const back = parseColor(background)
  const front = over(parseColor(foreground), back)
  const [hi, lo] = [luminance(front), luminance(back)].sort((x, y) => y - x) as [number, number]
  return (hi + 0.05) / (lo + 0.05)
}
