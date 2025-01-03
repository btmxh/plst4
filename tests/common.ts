import { test as base } from '@playwright/test';
import { env } from 'process';

export const test = base.extend({
  page: async ({ page }, use) => {
    if (!env.PLAYWRIGHT_ENABLE_ANIMATIONS) await page.emulateMedia({ reducedMotion: 'reduce' });
    await use(page);
  },
});
