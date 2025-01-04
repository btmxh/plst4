import { expect } from "@playwright/test";
import { test, TestAccount } from "./common";

test.describe("login scenarios", () => {
  test("title (indirect via homepage)", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "auth" }).click();
    await expect(page).toHaveTitle('plst4 - Log in');
  });

  test("title (direct via /auth/login)", async ({ page }) => {
    await page.goto("/auth/login");
    await expect(page).toHaveTitle('plst4 - Log in');
  });

  test("expected flow", async ({ page, browserName }) => {
    const acc = new TestAccount('default', browserName, 'default-password');
    await page.goto('/auth/login');
    await page.getByLabel("Username").fill(acc.username);
    await page.getByLabel("Password").fill('default-password');
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Home');
  });

  test("blank username", async ({ page }) => {
    await page.goto('/auth/login');
    await page.getByLabel("Username").fill('');
    await page.getByLabel("Password").fill('password');
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Username must not be empty.')");
  });

  test("blank password", async ({ page }) => {
    await page.goto('/auth/login');
    await page.getByLabel("Username").fill('username');
    await page.getByLabel("Password").fill('');
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Password must not be empty.')");
  });

  test("wrong credentials", async ({ page }) => {
    await page.goto('/auth/login');
    await page.getByLabel("Username").fill('username');
    await page.getByLabel("Password").fill('password');
    await page.getByRole('button', { name: "Continue" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Either username or password is incorrect.')");
  });
})
