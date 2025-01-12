import { chromium } from "@playwright/test";
import { exec, DATABASE_URL, baseUrl } from "./common.ts";
import { TestAccount } from "../tests/common.ts";

const globalSetup = async () => {
  await exec(`pg_dump -d ${DATABASE_URL} -f plst4.dump -c`);
  const browsers = ["chromium", "firefox", "webkit"];
  const accounts = [
    { identifier: "default", password: "default-password" },
    { identifier: "other", password: "other-password" },
    { identifier: "third", password: "third-password" },
  ];

  const chrome = await chromium.launch();
  const promises = [] as Promise<void>[];
  for (const { identifier, password } of accounts) {
    for (const browser of browsers) {
      const acc = new TestAccount(identifier, browser, password);
      promises.push(chrome.newPage().then(page => acc.register(page, baseUrl)));
    }
  }
  await Promise.all(promises);
  await chrome.close();
};

export default globalSetup;
