import { expect } from "@playwright/test";
import { test, TestAccount } from "./common";

test.describe("navbar scenarios", () => {
  test("test visible", async ({ page }) => {
    await page.goto("/");
    await page.waitForSelector("button:has-text('Press SPACE twice to toggle this navbar')");
  });

  test("test toggle", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Press SPACE twice to toggle this navbar" }).click();
    await expect(page.locator("button:has-text('Press SPACE twice to toggle this navbar')")).not.toBeInViewport();
  });

  test("test toggle keyboard", async ({ page }) => {
    await page.goto("/");
    await page.keyboard.type("  ");
    await expect(page.locator("button:has-text('Press SPACE twice to toggle this navbar')")).not.toBeInViewport();
    await page.keyboard.type("  ");
    await page.waitForSelector("button:has-text('Press SPACE twice to toggle this navbar')");
  });

  test("test typing", async ({ page }) => {
    await page.goto("/watch/");
    await page.getByPlaceholder("Search").focus();
    await page.keyboard.type("Test 123  ");
    await expect(page.getByPlaceholder("Search")).toHaveValue("Test 123  ");
    await page.waitForSelector("button:has-text('Press SPACE twice to toggle this navbar')");
  });
});
