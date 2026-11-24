import { expect, test } from '@playwright/test'
import { existsSync, readFileSync } from 'node:fs'
import path from 'node:path'

const webDir = path.resolve(import.meta.dirname, '..')
const distDir = path.resolve(webDir, '../internal/pages/embed/dist')
const origin = 'http://optdesk.test'
const payload = JSON.parse(
  readFileSync(path.join(webDir, '../contracts/fixtures/page.valuation_page.v1.company.json'), 'utf8'),
) as { band: { min: string; max: string; marks: unknown[] } }

const mime: Record<string, string> = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.json': 'application/json',
  '.woff2': 'font/woff2',
  '.png': 'image/png',
}

function pageWithPayload(): string {
  const html = readFileSync(path.join(distDir, 'index.html'), 'utf8')
  const json = JSON.stringify(payload).replaceAll('<', '\\u003c')
  const pattern = /(<script\b[^>]*\bid="vm"[^>]*>)[\s\S]*?(<\/script>)/
  if (!pattern.test(html)) throw new Error('built index.html has no <script id="vm">')
  return html.replace(pattern, (_all, open: string, close: string) => `${open}${json}${close}`)
}

test('the price band draws its fair zone, its mark lines and both axis labels at 1440', async ({ page }) => {
  const html = pageWithPayload()
  await page.route(`${origin}/**`, async (route) => {
    const { pathname } = new URL(route.request().url())
    if (pathname === '/' || pathname === '/index.html') {
      await route.fulfill({ status: 200, contentType: mime['.html'], body: html })
      return
    }
    const file = path.join(distDir, path.normalize(pathname))
    if (!file.startsWith(distDir + path.sep) || !existsSync(file)) {
      await route.fulfill({ status: 404, body: 'not found' })
      return
    }
    await route.fulfill({
      status: 200,
      contentType: mime[path.extname(file)] ?? 'application/octet-stream',
      body: readFileSync(file),
    })
  })
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto(`${origin}/`)
  const band = page.locator('[data-chart="price-band"]')
  await expect(band.locator('.recharts-reference-area')).toHaveCount(1)
  await expect(band.locator('.recharts-reference-line')).toHaveCount(payload.band.marks.length)
  const ticks = band.locator('.recharts-cartesian-axis-tick-value')
  await expect(ticks).toHaveText([payload.band.min, payload.band.max])
})
