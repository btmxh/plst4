import { test, expect } from '@playwright/test';
import { newEmail, getLatestMail } from './mail';

test('title-login', async ({ page }) => {
  await page.goto('/auth/login');
  await expect(page).toHaveTitle('plst4 - Log in');
})

test('title-register', async ({ page }) => {
  await page.goto('/auth/login');
  await page.locator('a:has-text("new account")').click();
  await expect(page).toHaveTitle('plst4 - Register');
})

test('basic-auth-flow', async ({ page, browserName }) => {
  const email = newEmail(browserName, 'basic-auth-flow');
  const username = `basic-auth-flow-${browserName}`;

  await page.goto('/auth/login');
  await page.locator('a:has-text("new account")').click();
  await page.locator('input[name="email"]').fill(email);
  await page.locator('input[name="username"]').fill(username);
  await page.locator('input[name="password"]').fill('password');
  await page.locator('input[name="password-confirm"]').fill('password');
  await page.locator('input:has-text("Continue")').click();

  const mailContent = await getLatestMail(email);
  expect(mailContent).toBeDefined();
  expect(mailContent!.subject).toBe('Confirm your plst4 email');
  expect(mailContent!.body).toContain('your confirmation code:');

  // take the code from mailContent!.body after the "your confirmation code:" substring
  const code = mailContent!.body.substring(mailContent!.body.indexOf('your confirmation code:') + 'your confirmation code:'.length + 1).trim();
  expect(code).toHaveLength(16);

  await expect(page).toHaveTitle('plst4 - Confirm your email');
  await page.locator('input[name="code"]').fill(code);
  await page.locator('input:has-text("Continue")').click();
  await expect(page).toHaveTitle('plst4 - Log in');

  await page.locator('input[name="username"]').fill(username);
  await page.locator('input[name="password"]').fill('password');
  await page.locator('input:has-text("Continue")').click();
  await expect(page).toHaveTitle('plst4 - Home');
});
