import fs from 'fs/promises';
import path from 'path';

export const newEmail = (browser: string, identifier: string) => {
  return `${identifier}.${browser}@plst.dev`;
};

export interface MailContent {
  subject: string
  body: string
}

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

export const getLatestMail = async (email: string): Promise<MailContent | undefined> => {
  await sleep(1000);
  const dirName = path.join(".mail", Buffer.from(email).toString("base64"));
  const dir = await fs.readdir(dirName);
  const files = dir.map(async (filename) => {
    const filePath = path.join(dirName, filename);
    const stats = await fs.stat(filePath);
    return { path: filePath, mtime: stats.mtime };
  });
  const fileInfos = await Promise.all(files);
  fileInfos.sort((a, b) => b.mtime.getTime() - a.mtime.getTime());

  if (fileInfos.length == 0) {
    return undefined;
  }

  const content = await fs.readFile(fileInfos[0].path);
  return JSON.parse(content.toString('utf-8')) satisfies MailContent;
}
