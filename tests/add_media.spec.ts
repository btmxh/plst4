import { expect, Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { clearToasts, test, TestAccount, TestMedia } from "./common";

test.describe("media add test", () => {
  test.beforeEach(async ({ page, browserName }) => {
    await new TestAccount('default', browserName, 'default-password').login(page);
    await page.goto("/watch")
    const playlistName = `random-playlist-${browserName}-${randomUUID()}`;
    page.on("dialog", async dialog => {
      expect(dialog.type()).toBe("prompt");
      expect(dialog.message()).toBe("Enter the new playlist name");
      await dialog.accept(playlistName);
    });
    await page.getByRole('button', { name: "New playlist" }).click();
    await expect(page).toHaveTitle(`plst4 - ${playlistName}`);
  });

  const addMedia = async (page: Page, url: string, position: "Add to start" | "Add to end" | "Queue next" = "Queue next") => {
    await page.getByPlaceholder("URL").fill(url);
    await page.getByRole("combobox").selectOption({ label: position });
    await page.getByRole('button', { name: "Add" }).click();
    await page.waitForSelector(".info > .toast-wrapper > h1:has-text('Adding new media')");
    await page.waitForSelector(".info > .toast-wrapper > h1:has-text('Media added successfully')");
    await clearToasts(page);
  };

  test("add single media", async ({ page }) => {
    await addMedia(page, TestMedia.v10s360p);
    await page.waitForSelector(".playlist-entry > label:has-text('1. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > .playlist-entry-length:has-text('00:00:10')");
  });

  test("add multiple media", async ({ page }) => {
    await addMedia(page, TestMedia.v10s360p);
    await addMedia(page, TestMedia.v5s360p);
    await addMedia(page, TestMedia.v1m360p);
    await page.waitForSelector(".playlist-entry > label:has-text('1. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('2. 5 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('3. 1 minute 360p test video')");
    await page.waitForSelector(".playlist-entry > .playlist-entry-length:has-text('00:00:10')");
  });

  test("goto media", async ({ page }) => {
    await addMedia(page, TestMedia.v10s360p);
    await page.waitForSelector(".playlist-entry > label:has-text('1. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > .playlist-entry-length:has-text('00:00:10')");

    await page.hover(".playlist-entry > label:has-text('1. 10 second 360p test video')");
    await page.getByRole("link", { name: "goto" }).click();
    await page.waitForSelector("video").then(elm => elm.click());

    await page.waitForSelector(".playlist-entry > label:has-text('> 1. 10 second 360p test video')");
  });

  test("queue next after goto media", async ({ page }) => {
    await addMedia(page, TestMedia.v10s360p);
    await page.waitForSelector(".playlist-entry > label:has-text('1. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > .playlist-entry-length:has-text('00:00:10')");

    await page.hover(".playlist-entry > label:has-text('1. 10 second 360p test video')");
    await page.getByRole("link", { name: "goto" }).click();
    await page.waitForSelector("video").then(elm => elm.click());

    await page.waitForSelector(".playlist-entry > label:has-text('> 1. 10 second 360p test video')");

    await addMedia(page, TestMedia.v5s360p, "Queue next");
    await addMedia(page, TestMedia.v1m360p, "Queue next");
    await page.waitForSelector(".playlist-entry > label:has-text('2. 1 minute 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('3. 5 second 360p test video')");
  });

  test("a whole lot of queue nexts", async ({ page }) => {
    await addMedia(page, TestMedia.v10s360p);
    await page.hover(".playlist-entry > label:has-text('1. 10 second 360p test video')");
    await page.getByRole("link", { name: "goto" }).click();
    await page.waitForSelector("video").then(elm => elm.click());

    await addMedia(page, TestMedia.v1m360p, "Queue next");
    await addMedia(page, TestMedia.v5s360p, "Queue next");
    await addMedia(page, TestMedia.v10s360p, "Queue next");
    await addMedia(page, TestMedia.v10s360p, "Queue next");
    await addMedia(page, TestMedia.v5s360p, "Queue next");
    await addMedia(page, TestMedia.v1m360p, "Queue next");
    await addMedia(page, TestMedia.v1m360p, "Queue next");
    await addMedia(page, TestMedia.v5s360p, "Queue next");
    await addMedia(page, TestMedia.v10s360p, "Queue next");
    await page.waitForSelector(".playlist-entry > label:has-text('2. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('3. 5 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('4. 1 minute 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('5. 1 minute 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('6. 5 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('7. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('8. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('9. 5 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('10. 1 minute 360p test video')");

    await addMedia(page, TestMedia.v10s360p, "Queue next");
    await addMedia(page, TestMedia.v5s360p, "Queue next");
    await addMedia(page, TestMedia.v1m360p, "Queue next");
    await addMedia(page, TestMedia.v1m360p, "Queue next");
    await addMedia(page, TestMedia.v5s360p, "Queue next");
    await addMedia(page, TestMedia.v10s360p, "Queue next");
    await addMedia(page, TestMedia.v10s360p, "Queue next");
    await addMedia(page, TestMedia.v5s360p, "Queue next");
    await addMedia(page, TestMedia.v1m360p, "Queue next");
    await page.waitForSelector(".playlist-entry > label:has-text('2. 1 minute 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('3. 5 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('4. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('5. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('6. 5 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('7. 1 minute 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('8. 1 minute 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('9. 5 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('10. 10 second 360p test video')");

    await page.getByRole("button", { name: ">", exact: true }).click();

    await page.waitForSelector(".playlist-entry > label:has-text('11. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('12. 5 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('13. 1 minute 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('14. 1 minute 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('15. 5 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('16. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('17. 10 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('18. 5 second 360p test video')");
    await page.waitForSelector(".playlist-entry > label:has-text('19. 1 minute 360p test video')");
  })
});
