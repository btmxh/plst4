import { expect } from "@playwright/test";
import { test, TestAccount } from "./common";

test.describe("search playlist scenarios", () => {
  test.beforeEach(async ({ page, browserName }) => {
    await new TestAccount('default', browserName, 'default-password').login(page);
  });

  test("find playlist after creation", async ({ page, browserName }) => {
    await page.goto("/watch");
    page.on("dialog", async dialog => {
      expect(dialog.type()).toBe("prompt");
      expect(dialog.message()).toBe("Enter the new playlist name");
      await dialog.accept(`find playlist after creation ${browserName}`);
    });
    await page.getByRole('button', { name: "New playlist" }).click();
    await expect(page).toHaveTitle(`plst4 - find playlist after creation ${browserName}`);

    await page.goto("/watch");
    await page.getByPlaceholder("Search").fill(`find playlist after creation ${browserName}`);
    await page.getByRole('button', { name: "Search" }).click();
    await page.waitForSelector(`a:has-text('find playlist after creation ${browserName}')`);

    await page.getByRole('button', { name: "Watch" }).click();
    await expect(page).toHaveTitle(`plst4 - find playlist after creation ${browserName}`);
  });

  test("rename playlist after creation", async ({ page, browserName }) => {
    await page.goto("/watch");
    page.once("dialog", async dialog => {
      expect(dialog.type()).toBe("prompt");
      expect(dialog.message()).toBe("Enter the new playlist name");
      await dialog.accept(`rename playlist after creation ${browserName}`);
    });
    await page.getByRole('button', { name: "New playlist" }).click();
    await expect(page).toHaveTitle(`plst4 - rename playlist after creation ${browserName}`);

    await page.goto("/watch");
    await page.getByPlaceholder("Search").fill(`rename playlist after creation ${browserName}`);
    await page.getByRole('button', { name: "Search" }).click();
    await page.waitForSelector(`a:has-text('rename playlist after creation ${browserName}')`);

    page.once("dialog", async dialog => {
      expect(dialog.type()).toBe("prompt");
      expect(dialog.message()).toBe("Enter the new playlist name");
      await dialog.accept(`renamed playlist after creation ${browserName}`);
    });

    await page.getByRole("button", { name: "Rename" }).click();
    await page.getByPlaceholder("Search").fill(`renamed playlist after creation ${browserName}`);
    await page.getByRole('button', { name: "Search" }).click();
    await page.waitForSelector(`a:has-text('renamed playlist after creation ${browserName}')`);
  });

  test("delete playlist after creation", async ({ page, browserName }) => {
    await page.goto("/watch");
    page.once("dialog", async dialog => {
      expect(dialog.type()).toBe("prompt");
      expect(dialog.message()).toBe("Enter the new playlist name");
      await dialog.accept(`delete playlist after creation ${browserName}`);
    });
    await page.getByRole('button', { name: "New playlist" }).click();
    await expect(page).toHaveTitle(`plst4 - delete playlist after creation ${browserName}`);

    await page.goto("/watch");
    await page.getByPlaceholder("Search").fill(`delete playlist after creation ${browserName}`);
    await page.getByRole('button', { name: "Search" }).click();
    await page.waitForSelector(`a:has-text('delete playlist after creation ${browserName}')`);

    page.once("dialog", async dialog => {
      expect(dialog.type()).toBe("confirm");
      expect(dialog.message()).toBe("Are you sure you want to delete this playlist?");
      await dialog.accept("");
    });

    await page.getByRole("button", { name: "Delete" }).click();
    await page.getByPlaceholder("Search").fill(`delete playlist after creation ${browserName}`);
    await page.getByRole('button', { name: "Search" }).click();
    await page.waitForSelector("h1:has-text('No results found.')");
  });
})
