import { test as base, expect, Page } from '@playwright/test';
import { env } from 'process';
import { getLatestMail } from './mail';

export const test = base.extend({
  page: async ({ page }, use) => {
    if (!env.PLAYWRIGHT_ENABLE_ANIMATIONS) await page.emulateMedia({ reducedMotion: 'reduce' });
    await use(page);
  },
});

export class TestAccount {
  identifier: string;
  browser: string;
  email: string;
  username: string;
  password: string;

  constructor(identifier: string, browser: string, password: string) {
    this.identifier = identifier;
    this.browser = browser;
    this.email = `${identifier}@${browser}.plst.dev`;
    this.username = `${identifier}-${browser}`;
    this.password = password;
  }

  async tryRegister(page: Page, baseUrl = '') {
    await page.goto(`${baseUrl}/auth/register`);
    await page.getByLabel("Email").fill(this.email);
    await page.getByLabel("Username").fill(this.username);
    await page.getByLabel("Password", { exact: true }).fill(this.password);
    await page.getByLabel("Confirm password").fill(this.password);
    await page.getByRole('button', { name: "Continue" }).click();
  }

  async register(page: Page, baseUrl = '') {
    this.tryRegister(page, baseUrl);

    await page.waitForSelector("h1:has-text('Confirm your email')");
    const mailContent = await getLatestMail(this.email);
    expect(mailContent).toBeDefined();
    expect(mailContent!.subject).toBe('Confirm your plst4 email');
    expect(mailContent!.body).toContain('your confirmation code:');

    // take the code from mailContent!.body after the "your confirmation code:" substring
    const code = mailContent!.body.substring(mailContent!.body.indexOf('your confirmation code:') + 'your confirmation code:'.length + 1).trim();

    await expect(page).toHaveTitle('plst4 - Confirm your email');
    await page.getByLabel('Enter the confirmation code mailed to your email').fill(code);
    await page.getByRole('button', { name: "Continue" }).click();

    await expect(page).toHaveTitle('plst4 - Log in');
  }

  async login(page: Page) {
    await page.goto('/auth/login');
    await page.getByLabel("Username").fill(this.username);
    await page.getByLabel("Password").fill(this.password);
    await page.getByRole('button', { name: "Continue" }).click();
    await expect(page).toHaveTitle('plst4 - Home');
  }
}

export const clearToasts = async (page: Page) => {
  await page.evaluate(() => {
    document.querySelectorAll(".toast-wrapper").forEach(elm => (elm as HTMLElement).click());
  });
};

export const TestMedia = {
  "v5s360p": "http://localhost:6972/testmedias/5s360p.mp4",
  "v10s360p": "http://localhost:6972/testmedias/10s360p.mp4",
  "v1m360p": "http://localhost:6972/testmedias/1m360p.mp4",
  "videos": "http://localhost:6972/testmedias/videos.mp4.json",
};
