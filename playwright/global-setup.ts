import { exec, DATABASE_URL } from "./common.ts";

const globalSetup = async () => {
  await exec(`pg_dump -d ${DATABASE_URL} -f plst4.dump -c`);
};

export default globalSetup;
