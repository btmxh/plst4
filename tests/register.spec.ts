import { expect } from "@playwright/test";
import { test, TestAccount } from "./common";

test.describe("register scenarios", () => {
  test("title (indirect via auth page)", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "auth" }).click();
    await expect(page).toHaveTitle('plst4 - Log in');
    await page.getByRole('button', { name: "new account" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
  });

  test("title (direct via /auth/register)", async ({ page }) => {
    await page.goto("/auth/register");
    await expect(page).toHaveTitle('plst4 - Register');
  });

  test("expected flow", async ({ page, browserName }) => {
    const acc = new TestAccount('register-expected', browserName, 'password');
    await acc.register(page);
    await page.goto('/auth/login');
    await page.getByLabel("Username").fill(acc.username);
    await page.getByLabel("Password").fill('password');
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Home');
  });

  test("blank username", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("test@email.com");
    await page.getByLabel("Username").fill("");
    await page.getByLabel("Password", { exact: true }).fill("password");
    await page.getByLabel("Confirm password").fill("password");
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Username must contain 3-50 characters, including lowercase letters, uppercase letters, numbers, hyphens (-), and underscores (_).')");
  });

  test("invalid username", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("test@email.com");
    await page.getByLabel("Username").fill("inv@lid");
    await page.getByLabel("Password", { exact: true }).fill("password");
    await page.getByLabel("Confirm password").fill("password");
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Username must contain 3-50 characters, including lowercase letters, uppercase letters, numbers, hyphens (-), and underscores (_).')");
  });

  test("short username", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("test@email.com");
    await page.getByLabel("Username").fill("12");
    await page.getByLabel("Password", { exact: true }).fill("password");
    await page.getByLabel("Confirm password").fill("password");
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Username must contain 3-50 characters, including lowercase letters, uppercase letters, numbers, hyphens (-), and underscores (_).')");
  });

  test("long username", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("test@email.com");
    await page.getByLabel("Username").fill("a".repeat(100));
    await page.getByLabel("Password", { exact: true }).fill("password");
    await page.getByLabel("Confirm password").fill("password");
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Username must contain 3-50 characters, including lowercase letters, uppercase letters, numbers, hyphens (-), and underscores (_).')");
  });

  test("blank email", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("");
    await page.getByLabel("Username").fill("username");
    await page.getByLabel("Password", { exact: true }).fill("password");
    await page.getByLabel("Confirm password").fill("password");
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Invalid email.')");
  });

  test("invalid email", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("invalid email");
    await page.getByLabel("Username").fill("username");
    await page.getByLabel("Password", { exact: true }).fill("password");
    await page.getByLabel("Confirm password").fill("password");
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Invalid email.')");
  });

  test("long email", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("a".repeat(100) + "@email.com");
    await page.getByLabel("Username").fill("username");
    await page.getByLabel("Password", { exact: true }).fill("password");
    await page.getByLabel("Confirm password").fill("password");
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Email must not be longer than 100 characters.')");
  });

  test("blank password", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("test@email.com");
    await page.getByLabel("Username").fill("username");
    await page.getByLabel("Password", { exact: true }).fill("");
    await page.getByLabel("Confirm password").fill("");
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Password must contain 8-64 characters')");
  });

  test("short password", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("test@email.com");
    await page.getByLabel("Username").fill("username");
    await page.getByLabel("Password", { exact: true }).fill("short");
    await page.getByLabel("Confirm password").fill("short");
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Password must contain 8-64 characters')");
  });

  test("long password", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("test@email.com");
    await page.getByLabel("Username").fill("username");
    await page.getByLabel("Password", { exact: true }).fill("a".repeat(100));
    await page.getByLabel("Confirm password").fill("a".repeat(100));
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Password must contain 8-64 characters')");
  });

  test("invalid password", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("test@email.com");
    await page.getByLabel("Username").fill("username");
    await page.getByLabel("Password", { exact: true }).fill("{}{}{}{}");
    await page.getByLabel("Confirm password").fill("{}{}{}{}");
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Password must contain 8-64 characters')");
  });

  test("mismatch password", async ({ page }) => {
    await page.goto('/auth/register');
    await page.getByLabel("Email").fill("test@email.com");
    await page.getByLabel("Username").fill("username");
    await page.getByLabel("Password", { exact: true }).fill("password");
    await page.getByLabel("Confirm password").fill("password1");
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Register');
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Passwords do not match')");
  });
});
