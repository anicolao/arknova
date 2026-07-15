import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30_000,
  expect: { timeout: 2_000, toHaveScreenshot: { maxDiffPixels: 0 } },
  snapshotPathTemplate: '{testDir}/{testFileDir}/screenshots/{arg}{ext}',
  use: { timezoneId: 'America/Toronto', locale: 'en-CA', reducedMotion: 'reduce', trace: 'retain-on-failure' },
  reporter: [['list'], ['html', { open: 'never' }]]
});
