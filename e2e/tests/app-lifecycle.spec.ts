import { test, expect } from '@playwright/test'

// Full click-through in a real browser: sign in, install an app from the
// marketplace, watch its container come up (status "Running"), then remove it
// and confirm the card disappears. Complements the API-level Go lifecycle test
// by exercising the actual dashboard UI wiring.

const EMAIL = process.env.E2E_EMAIL || 'e2e@test.local'
const PASSWORD = process.env.E2E_PASSWORD || 'e2e-test-pass-123'
const APP_NAME = 'Excalidraw' // small, static image; reliably serves and shows "Running"

test('install and uninstall an app through the dashboard', async ({ page }) => {
  await page.goto('/')

  // Authenticate: complete first-run setup on a fresh instance, else log in.
  const setupHeading = page.getByRole('heading', { name: /Welcome to Private Cloud Gateway/i })
  if (await setupHeading.isVisible().catch(() => false)) {
    await page.getByPlaceholder('John').fill('E2E')
    await page.getByPlaceholder('you@example.com').fill(EMAIL)
    await page.getByPlaceholder('At least 8 characters').fill(PASSWORD)
    await page.getByPlaceholder('••••••••').fill(PASSWORD)
    await page.getByRole('button', { name: /Create account/i }).click()
  } else {
    await page.locator('#email').fill(EMAIL)
    await page.locator('#password').fill(PASSWORD)
    await page.getByRole('button', { name: 'Sign in' }).click()
  }

  // Dashboard: open the marketplace (button reads "Install app" or
  // "Install your first app" depending on whether any app exists yet).
  const openMarketplace = page.getByRole('button', { name: /Install (app|your first app)/i }).first()
  await expect(openMarketplace).toBeVisible({ timeout: 20_000 })
  await openMarketplace.click()

  // Marketplace dialog: search, select the app card, confirm install.
  await page.getByPlaceholder('Search apps…').fill(APP_NAME)
  await page.getByRole('button', { name: new RegExp(APP_NAME, 'i') }).first().click()
  await page.getByRole('button', { name: 'Install', exact: true }).click()

  // The app card appears and the container reaches "Running".
  await expect(page.getByRole('heading', { name: APP_NAME })).toBeVisible({ timeout: 150_000 })
  await expect(page.getByText('Running').first()).toBeVisible({ timeout: 150_000 })

  // Remove it, and confirm the card disappears.
  await page.getByRole('button', { name: 'Remove' }).first().click()
  await expect(page.getByRole('heading', { name: APP_NAME })).toBeHidden({ timeout: 60_000 })
})
