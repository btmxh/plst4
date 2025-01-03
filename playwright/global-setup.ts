import { Browser, chromium } from "@playwright/test";
import { exec, DATABASE_URL, baseUrl } from "./common.ts";
import { getLatestMail } from "../tests/mail.ts";

const registerAccount = async (browser: Browser, username: string, email: string, password: string): Promise<void> => {
  const page = await browser.newPage();
  await page.goto(`${baseUrl}/auth/register`)
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByLabel("Confirm password").fill(password);
  await page.getByRole('button', { name: "Continue" }).click();

  const mailContent = await getLatestMail(email);
  const code = mailContent!.body.substring(mailContent!.body.indexOf('your confirmation code:') + 'your confirmation code:'.length + 1).trim();

  await page.waitForSelector("h1:has-text('Confirm your email')");
  await page.getByLabel('Enter the confirmation code mailed to your email').fill(code);
  await page.getByRole('button', { name: "Continue" }).click();
  await page.close();
};

const globalSetup = async () => {
  await exec(`pg_dump -d ${DATABASE_URL} -f plst4.dump -c`);

  const chrome = await chromium.launch();
  await Promise.all([
    registerAccount(chrome, "default-user", "default@plst.dev", "default-password"),
    registerAccount(chrome, "other-user", "other@plst.dev", "other-password"),
    registerAccount(chrome, "third-user", "third@plst.dev", "third-password"),
    ...["chromium", "firefox", "webkit"].map(async (browserName) => {
      return registerAccount(chrome, `${browserName}-user`, `${browserName}@plst.dev`, `browser-password`);
    })
  ]);
  await chrome.close();
};

export default globalSetup;
