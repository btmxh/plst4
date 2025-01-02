import { exec, DATABASE_URL } from "./common.ts";
import { rm } from "fs/promises";

const globalTeardown = async () => {
  await exec(`psql -d ${DATABASE_URL} -f plst4.dump`);
  await rm("plst4.dump");
};

export default globalTeardown;
