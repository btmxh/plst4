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

  async register(page: Page, baseUrl = '') {
    await page.goto(`${baseUrl}/auth/register`);
    await page.getByLabel("Email").fill(this.email);
    await page.getByLabel("Username").fill(this.username);
    await page.getByLabel("Password", { exact: true }).fill(this.password);
    await page.getByLabel("Confirm password").fill(this.password);
    await page.getByRole('button', { name: "Continue" }).click();

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
};
