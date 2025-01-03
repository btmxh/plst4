import { expect, Page } from '@playwright/test';
import { load as loadHtml } from "cheerio";
import { newEmail, getLatestMail } from './mail';
import { test } from './common';

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

test.describe("tests with recovering default user password", () => {
  test.describe.configure({ mode: 'serial' });

  test.beforeEach(async ({ page, browserName }) => {
    await page.goto("/auth/recover");
    await expect(page).toHaveTitle('plst4 - Account recovery');
    await page.getByLabel("Email").fill(`${browserName}@plst.dev`);
    await page.getByRole('button', { name: "Continue" }).click();
  });

  test("expected flow", async ({ page, browserName }) => {
    const mailContent = await getLatestMail(`${browserName}@plst.dev`);
    expect(mailContent).toBeDefined();
    await page.goto(loadHtml(mailContent!.body)('a').attr('href')!);

    await page.getByLabel("Password", { exact: true }).fill('default-password');
    await page.getByLabel("Confirm password").fill('default-password');
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Log in');
  });

  test("invalid password", async ({ page, browserName }) => {
    const mailContent = await getLatestMail(`${browserName}@plst.dev`);
    expect(mailContent).toBeDefined();
    await page.goto(loadHtml(mailContent!.body)('a').attr('href')!);

    await page.getByLabel("Password", { exact: true }).fill('short');
    await page.getByLabel("Confirm password").fill('short');
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('contain 8-64 characters')");
  });

  test("mismatch password", async ({ page, browserName }) => {
    const mailContent = await getLatestMail(`${browserName}@plst.dev`);
    expect(mailContent).toBeDefined();
    await page.goto(loadHtml(mailContent!.body)('a').attr('href')!);

    await page.getByLabel("Password", { exact: true }).fill('password1');
    await page.getByLabel("Confirm password").fill('password2');
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Passwords do not match')");
  });

  test("wrong link", async ({ page, browserName }) => {
    await page.goto(`/auth/resetpassword?code=wrongcode&email=${browserName}@plst.dev`);
    await page.waitForSelector("h1:has-text('Reset password error')");
  });

  test("wrong email", async ({ page, browserName }) => {
    await page.goto(`/auth/resetpassword?code=wrongcode&email=${browserName}@plst.dev.wrong.email`);
    await page.waitForSelector("h1:has-text('Reset password error')");
  })
});

