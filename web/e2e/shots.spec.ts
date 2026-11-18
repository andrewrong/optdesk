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

// The built index.html with the empty `<script id="vm">` replaced by the fixture as its payload.
function pageWithPayload(fixture: string): string {
  const html = readFileSync(path.join(distDir, 'index.html'), 'utf8')
  const payload = JSON.stringify(
    JSON.parse(readFileSync(path.join(fixturesDir, fixture, 'ok.json'), 'utf8')),
  ).replaceAll('<', '\\u003c')
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
      const html = pageWithPayload(fixture)
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
