import { test, expect } from '@playwright/test'

// Verifies the marketplace shows "coming soon" apps as listed-but-not-installable
// (badge + disabled card). Immich ships as coming_soon.

const EMAIL = process.env.E2E_EMAIL || 'e2e@test.local'
const PASSWORD = process.env.E2E_PASSWORD || 'e2e-test-pass-123'

test('coming-soon apps are shown but not installable', async ({ page }) => {
  await page.goto('/')

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

  const openMarketplace = page.getByRole('button', { name: /Install (app|your first app)/i }).first()
  await expect(openMarketplace).toBeVisible({ timeout: 20_000 })
  await openMarketplace.click()

  await page.getByPlaceholder('Search apps…').fill('Immich')

  // The Immich card shows a "Coming soon" badge and is not selectable.
  await expect(page.getByText('Coming soon').first()).toBeVisible()
  const immich = page.getByRole('button', { name: /Immich/i }).first()
  await expect(immich).toBeDisabled()
})
