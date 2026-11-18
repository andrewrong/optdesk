import { defineConfig } from '@playwright/test'

// The ui-shots job renders every fixture page to a PNG over the built web/ (see e2e/shots.spec.ts).
export default defineConfig({
  testDir: 'e2e',
  testMatch: '*.spec.ts',
  fullyParallel: true,
  reporter: 'list',
  outputDir: 'test-results',
})
