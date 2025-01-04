import { baseUrl } from '../playwright/common';

export interface MailContent {
  subject: string
  body: string
}

export const getLatestMail = async (email: string): Promise<MailContent | undefined> => {
  const query = new URLSearchParams({ email })
  const response = await fetch(`${baseUrl}/mail?${query.toString()}`);
  if (!response.ok) {
    return undefined;
  }
  return {
    subject: response.headers.get("Subject")!,
    body: await response.text()
  };
}
