import { defineConfig, devices } from '@playwright/test'

// Browser E2E runs against a live stack (dev via localtest.me, or a deployed
// instance). Set E2E_BASE_URL / E2E_EMAIL / E2E_PASSWORD to point it. Slow by
// design — image pulls and container startup — so it runs per-release, not per-PR.
export default defineConfig({
  testDir: './tests',
  timeout: 180_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://home.localtest.me',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    // When E2E_RESOLVE is set (e.g. 127.0.0.1), map *.localtest.me to it in the
    // browser so tests work even when public DNS for localtest.me is unavailable.
    launchOptions: process.env.E2E_RESOLVE
      ? {
          args: [
            `--host-resolver-rules=MAP *.localtest.me ${process.env.E2E_RESOLVE}, MAP localtest.me ${process.env.E2E_RESOLVE}`,
          ],
        }
      : {},
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
