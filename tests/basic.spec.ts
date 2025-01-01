import { test, expect } from '@playwright/test';
import { host } from "./env";

test('has title', async ({ page }) => {
  await page.goto(host);
  await expect(page).toHaveTitle(/plst4/);
});

