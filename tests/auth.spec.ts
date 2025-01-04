import { expect } from '@playwright/test';
import { test, TestAccount } from './common';

test('title-login', async ({ page }) => {
  await page.goto('/auth/login');
  await expect(page).toHaveTitle('plst4 - Log in');
})

test('title-register', async ({ page }) => {
  await page.goto('/auth/login');
  await page.getByRole('button', { name: "new account" }).click();
  await expect(page).toHaveTitle('plst4 - Register');
})

test('basic-auth-flow', async ({ page, browserName }) => {
  const acc = new TestAccount('basic-auth-flow', browserName, 'password');
  await acc.register(page);
  await page.goto("/auth/login");
  await page.waitForSelector("h1:has-text('Log in')");
  await expect(page).toHaveTitle('plst4 - Log in');
  await page.getByLabel("Username").fill(acc.username);
  await page.getByLabel("Password", { exact: true }).fill(acc.password);
  await page.getByRole('button', { name: "Continue" }).click();
  await expect(page).toHaveTitle('plst4 - Home');
});
