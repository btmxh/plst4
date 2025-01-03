import { expect } from '@playwright/test';
import { test } from './common';

test('has title', async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveTitle(/plst4/);
});

test('has favicon', async ({ page }) => {
  await page.goto("/");
  const iconLink = await page.$("link[rel='icon']");
  expect(iconLink).not.toBeNull();

  const iconHref = await iconLink?.getAttribute('href');
  expect(iconHref).not.toBeNull();

  const iconUrl = new URL(iconHref!, page.url()).toString();

  const [faviconResponse] = await Promise.all([
    page.waitForResponse(iconUrl),
    page.goto("/"),
  ]);

  expect(faviconResponse.status()).toBe(200);
})

test('has header', async ({ page }) => {
  await page.goto("/");
  const header = await page.$("h1");
  expect(header).not.toBeNull();
});
