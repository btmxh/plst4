import { expect } from '@playwright/test';
import { newEmail, getLatestMail } from './mail';
import { test } from './common';

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
  const email = newEmail(browserName, 'basic-auth-flow');
  const username = `basic-auth-flow-${browserName}`;

  await page.goto('/');
  await page.getByRole("link", { name: "auth" }).click();
  await page.getByRole('button', { name: "Create a new account" }).click();
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Password", { exact: true }).fill('password');
  await page.getByLabel("Confirm password").fill('password');
  await page.getByRole('button', { name: "Continue" }).click();

  const mailContent = await getLatestMail(email);
  expect(mailContent).toBeDefined();
  expect(mailContent!.subject).toBe('Confirm your plst4 email');
  expect(mailContent!.body).toContain('your confirmation code:');

  // take the code from mailContent!.body after the "your confirmation code:" substring
  const code = mailContent!.body.substring(mailContent!.body.indexOf('your confirmation code:') + 'your confirmation code:'.length + 1).trim();
  expect(code).toHaveLength(16);

  await page.waitForSelector("h1:has-text('Confirm your email')");
  await expect(page).toHaveTitle('plst4 - Confirm your email');
  await page.getByLabel('Enter the confirmation code mailed to your email').fill(code);
  await page.getByRole('button', { name: "Continue" }).click();

  await page.waitForSelector("h1:has-text('Log in')");
  await expect(page).toHaveTitle('plst4 - Log in');
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Password", { exact: true }).fill('password');
  await page.getByRole('button', { name: "Continue" }).click();
  await expect(page).toHaveTitle('plst4 - Home');
});
