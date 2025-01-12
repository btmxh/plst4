import { expect, Page } from '@playwright/test';
import { load as loadHtml } from "cheerio";
import { getLatestMail } from './mail';
import { test, TestAccount } from './common';

const getRecoverLink = async (acc: TestAccount) => {
  const mailContent = await getLatestMail(acc.email);
  expect(mailContent).toBeDefined();
  expect(mailContent!.subject).toBe('Recover your plst4 account');
  const $ = loadHtml(mailContent!.body);
  const link = $('a').attr('href');
  expect(link).toBeDefined();
  return link!;
};

const submitForgotPassEmail = async (page: Page, acc: TestAccount) => {
  await page.goto('/auth/login');
  await page.getByRole('link', { name: "Forgot password" }).click();
  await page.getByLabel("Email").fill(acc.email);
  await page.getByRole('button', { name: "Continue" }).click();
  await page.waitForSelector("h1:has-text('Check your email')");
};

test('basic-recover-password', async ({ page, browserName }) => {
  const acc = new TestAccount('basic-recover-password', browserName, 'password');
  await acc.register(page);
  await submitForgotPassEmail(page, acc);
  const link = await getRecoverLink(acc);
  await page.goto(link);

  await expect(page).toHaveTitle('plst4 - Account recovery');
  await page.getByLabel("Password", { exact: true }).fill('new-password');
  await page.getByLabel("Confirm password").fill('new-password');
  await page.getByRole('button', { name: "Continue" }).click();

  await expect(page).toHaveTitle('plst4 - Log in');
  await page.getByLabel("Username").fill(acc.username);
  await page.getByLabel("Password").fill('password');
  await page.getByRole('button', { name: "Continue" }).click();
  await page.waitForSelector(".error > .toast-wrapper > p:has-text('Either username or password is incorrect.')");
  await page.getByLabel("Username").fill(acc.username);
  await page.getByLabel("Password").fill('new-password');
  await page.getByRole('button', { name: "Continue" }).click();
  await expect(page).toHaveTitle('plst4 - Home');
});

test.describe("tests with recovering default user password", () => {
  test("expected flow", async ({ page, browserName }) => {
    const acc = new TestAccount('recover-default-password-expected', browserName, 'default-password');
    await acc.register(page);
    await submitForgotPassEmail(page, acc);
    await page.goto(await getRecoverLink(acc));
    await page.getByLabel("Password", { exact: true }).fill('default-password');
    await page.getByLabel("Confirm password").fill('default-password');
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Log in');
  });

  test("blank password", async ({ page, browserName }) => {
    const acc = new TestAccount('recover-default-password-blank', browserName, 'default-password');
    await acc.register(page);
    await submitForgotPassEmail(page, acc);
    await page.goto(await getRecoverLink(acc));
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('contain 8-64 characters')");
  });

  test("short password", async ({ page, browserName }) => {
    const acc = new TestAccount('recover-default-password-short', browserName, 'default-password');
    await acc.register(page);
    await submitForgotPassEmail(page, acc);
    await page.goto(await getRecoverLink(acc));
    await page.getByLabel("Password", { exact: true }).fill('short');
    await page.getByLabel("Confirm password").fill('short');
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('contain 8-64 characters')");
  });

  test("mismatch password", async ({ page, browserName }) => {
    const acc = new TestAccount('recover-default-password-mismatch', browserName, 'default-password');
    await acc.register(page);
    await submitForgotPassEmail(page, acc);
    await page.goto(await getRecoverLink(acc));
    await page.getByLabel("Password", { exact: true }).fill('password1');
    await page.getByLabel("Confirm password").fill('password2');
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Passwords do not match')");
  });

  test("invalid password", async ({ page, browserName }) => {
    const acc = new TestAccount('recover-default-password-invalid', browserName, 'default-password');
    await acc.register(page);
    await submitForgotPassEmail(page, acc);
    await page.goto(await getRecoverLink(acc));
    await page.getByLabel("Password", { exact: true }).fill('{}{}{}{}');
    await page.getByLabel("Confirm password").fill('{}{}{}{}');
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('contain 8-64 characters')");
  })

  test("long password", async ({ page, browserName }) => {
    const acc = new TestAccount('recover-default-password-long', browserName, 'default-password');
    await acc.register(page);
    await submitForgotPassEmail(page, acc);
    await page.goto(await getRecoverLink(acc));
    await page.getByLabel("Password", { exact: true }).fill('a'.repeat(100));
    await page.getByLabel("Confirm password").fill('a'.repeat(100));
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('contain 8-64 characters')");
  });

  test("wrong code", async ({ page, browserName }) => {
    const acc = new TestAccount('recover-default-password-link', browserName, 'default-password');
    await acc.register(page);
    await submitForgotPassEmail(page, acc);
    await page.goto(`/auth/resetpassword?code=wrongcode&email=${acc.email}`);
    await page.waitForSelector("h1:has-text('Reset password error')");
  });

  test("wrong email", async ({ page, browserName }) => {
    await page.goto(`/auth/resetpassword?code=wrongcode&email=${browserName}@plst.dev.wrong.email`);
    await page.waitForSelector("h1:has-text('Reset password error')");
  });

  test("blank email recover", async ({ page }) => {
    await page.goto("/auth/recover");
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Invalid email.')");
  });

  test("invalid email recover", async ({ page }) => {
    await page.goto("/auth/recover");
    await page.getByLabel("Email").fill(`invalid email desu`);
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Invalid email.')");
  });

  test("unavailable email recover", async ({ page }) => {
    await page.goto("/auth/recover");
    await page.getByLabel("Email").fill(`no.one.has@this.email`);
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('No such account with that email address.')");
  });
});

