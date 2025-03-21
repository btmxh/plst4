import { expect, Page } from "@playwright/test";
import { test, TestAccount } from "./common";

test.describe("rename playlist scenarios", () => {
  test.beforeEach(async ({ page, browserName }) => {
    await new TestAccount('default', browserName, 'default-password').login(page);
  });

  const createPlaylist = async (title: string, page: Page, browserName: string) => {
    await page.goto("/watch");
    page.once("dialog", async dialog => {
      expect(dialog.type()).toBe("prompt");
      expect(dialog.message()).toBe("Enter the new playlist name");
      await dialog.accept(`${title} ${browserName}`);
    });
    await page.getByRole('button', { name: "New playlist" }).click();
    await expect(page).toHaveTitle(`plst4 - ${title} ${browserName}`);
  };

  // issue #13
  test("rename controller swap behavior", async ({ page, browserName }) => {
    await createPlaylist("rename controller swap #13", page, browserName);
    await page.locator("label:has-text('controller')").click({force: true});
    await page.waitForSelector("h2:has-text('Current playlist: rename controller swap #13')");

    page.once("dialog", async dialog => {
      expect(dialog.type()).toBe("prompt");
      expect(dialog.message()).toBe("Enter the new playlist name");
      await dialog.accept(`rename controller swap #12 ${browserName}`);
    });
    await page.getByRole('button', { name: "Rename" }).click();
    await page.waitForSelector("h2:has-text('Current playlist: rename controller swap #12')");
  })
});
