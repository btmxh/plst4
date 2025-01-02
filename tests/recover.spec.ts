import { test, expect, Page } from '@playwright/test';
import { load as loadHtml } from "cheerio";
import { newEmail, getLatestMail } from './mail';

const register = async (page: Page, browserName: string, identifier: string, password: string) => {
  const email = newEmail(browserName, identifier);
  const username = `${identifier}-${browserName}`;

  await page.goto('/auth/register');
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByLabel("Confirm password").fill(password);
  await page.getByRole('button', { name: "Continue" }).click();

  const mailContent = await getLatestMail(email);
  const code = mailContent!.body.substring(mailContent!.body.indexOf('your confirmation code:') + 'your confirmation code:'.length + 1).trim();

  await page.getByLabel('Enter the confirmation code mailed to your email').fill(code);
  await page.getByRole('button', { name: "Continue" }).click();
}

test('basic-recover-password', async ({ page, browserName }) => {
  await register(page, browserName, 'basic-recover-password', 'password');
  await page.goto('/auth/login');
  await page.getByRole('link', { name: "Forgot password" }).click();

  const username = `basic-recover-password-${browserName}`;
  const email = newEmail(browserName, 'basic-recover-password');

  await page.getByLabel("Email").fill(email);
  await page.getByRole('button', { name: "Continue" }).click();

  const mailContent = await getLatestMail(email);
  expect(mailContent).toBeDefined();
  expect(mailContent!.subject).toBe('Recover your plst4 account');
  const $ = loadHtml(mailContent!.body);
  const link = $('a').attr('href');
  expect(link).toBeDefined();

  await page.goto(link!);

  await expect(page).toHaveTitle('plst4 - Account recovery');
  await page.getByLabel("Password", { exact: true }).fill('new-password');
  await page.getByLabel("Confirm password").fill('new-password');
  await page.getByRole('button', { name: "Continue" }).click();

  await expect(page).toHaveTitle('plst4 - Log in');
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Password").fill('new-password');
  await page.getByRole('button', { name: "Continue" }).click();
  await expect(page).toHaveTitle('plst4 - Home');
});
