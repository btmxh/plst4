import { expect } from "@playwright/test";
import { test, TestAccount } from "./common";

test.describe("new playlist scenarios", () => {
  test.beforeEach(async ({ page, browserName }) => {
    await new TestAccount("default", browserName, "default-password").login(page);
  });

  test("blank playlist name", async ({ page }) => {
    await page.goto("/watch");
    page.on("dialog", async dialog => {
      expect(dialog.type()).toBe("prompt");
      expect(dialog.message()).toBe("Enter the new playlist name");
      await dialog.accept("");
    });
    await page.getByRole('button', { name: "New playlist" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Invalid title.')");
  });
  test("short playlist name", async ({ page }) => {
    await page.goto("/watch");
    page.on("dialog", async dialog => {
      expect(dialog.type()).toBe("prompt");
      expect(dialog.message()).toBe("Enter the new playlist name");
      await dialog.accept("727");
    });
    await page.getByRole('button', { name: "New playlist" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Invalid title.')");
  });
  test("long playlist name", async ({ page }) => {
    await page.goto("/watch");
    page.on("dialog", async dialog => {
      expect(dialog.type()).toBe("prompt");
      expect(dialog.message()).toBe("Enter the new playlist name");
      await dialog.accept("a".repeat(200));
    });
    await page.getByRole('button', { name: "New playlist" }).click();
    await page.waitForSelector(".error > .toast-wrapper > p:has-text('Invalid title.')");
  });
  test("expected flow", async ({ page, browserName }) => {
    await page.goto("/watch");
    page.on("dialog", async dialog => {
      expect(dialog.type()).toBe("prompt");
      expect(dialog.message()).toBe("Enter the new playlist name");
      await dialog.accept(`New playlist test ${browserName}`);
    });
    await page.getByRole('button', { name: "New playlist" }).click();
    await expect(page).toHaveTitle(`plst4 - New playlist test ${browserName}`);
  });
});
