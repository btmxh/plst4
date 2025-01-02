import { promisify } from "util";
import { env } from "process";
import proc from "child_process";
import dotenv from "dotenv";
import path from "path";

dotenv.config({ path: path.resolve(__dirname, '../.env') });
export const exec = promisify(proc.exec);
export const DATABASE_URL = env.DATABASE_URL;
