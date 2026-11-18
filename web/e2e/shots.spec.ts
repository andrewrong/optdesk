import { expect, test } from '@playwright/test'
import { existsSync, mkdirSync, readdirSync, readFileSync } from 'node:fs'
import path from 'node:path'

const webDir = path.resolve(import.meta.dirname, '..')
const distDir = path.resolve(webDir, '../internal/pages/embed/dist')
const fixturesDir = path.join(webDir, 'fixtures')
const outDir = path.join(import.meta.dirname, 'out')
const origin = 'http://optdesk.test'
const widths = [390, 1280]

const mime: Record<string, string> = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.json': 'application/json',
  '.woff2': 'font/woff2',
  '.png': 'image/png',
}

const fixtureDirs = readdirSync(fixturesDir, { withFileTypes: true })
  .filter((entry) => entry.isDirectory() && existsSync(path.join(fixturesDir, entry.name, 'ok.json')))
  .map((entry) => entry.name)
  .sort()

const readJson = (file: string): unknown => JSON.parse(readFileSync(file, 'utf8'))

// Pages whose web/fixtures/<dir>/ok.json is of an older schema_version than the page reads: the payload
// of the version the page reads (SUPPORTED_VERSIONS in web/src/types/<schema>.ts) comes from here.
const current: Record<string, () => Promise<unknown>> = {
  'macro-summary': async () => (await import('../test/pages/home/macro-v2/fixtures/ok')).ok,
  'held-companies-summary': async () => (await import('../test/pages/home/held-v2/fixtures/ok')).ok,
  'household-summary': async () => (await import('../test/pages/home/household-v2/fixtures/ok')).ok,
  tracking: async () => (await import('../test/pages/options/tracking-v2/fixtures/ok')).ok,
  'market-summary': async () => readJson(path.join(fixturesDir, 'v3/market-summary/ok.json')),
  'options-summary': async () =>
    readJson(path.join(webDir, '../contracts/fixtures/page.options_summary.v4.ok.json')),
}

// The built index.html with the empty `<script id="vm">` replaced by the fixture as its payload.
async function pageWithPayload(fixture: string): Promise<string> {
  const html = readFileSync(path.join(distDir, 'index.html'), 'utf8')
  const load = current[fixture] ?? (async () => readJson(path.join(fixturesDir, fixture, 'ok.json')))
  const payload = JSON.stringify(await load()).replaceAll('<', '\\u003c')
  const pattern = /(<script\b[^>]*\bid="vm"[^>]*>)[\s\S]*?(<\/script>)/
  if (!pattern.test(html)) throw new Error('built index.html has no <script id="vm">')
  return html.replace(pattern, (_all, open: string, close: string) => `${open}${payload}${close}`)
}

test.beforeAll(() => {
  mkdirSync(outDir, { recursive: true })
})

test('there is a fixture page to render', () => {
  expect(fixtureDirs.length).toBeGreaterThan(0)
})

for (const fixture of fixtureDirs) {
  for (const width of widths) {
    test(`${fixture} at ${width}`, async ({ page }) => {
      const html = await pageWithPayload(fixture)
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

      await page.setViewportSize({ width, height: 900 })
      await page.goto(`${origin}/`)
      await expect(page.locator('#fail-closed')).toBeHidden()
      await expect.poll(async () => (await page.locator('#root').innerText()).length).toBeGreaterThan(20)
      await page.screenshot({ path: path.join(outDir, `${fixture}.${width}.png`), fullPage: true })
    })
  }
}
