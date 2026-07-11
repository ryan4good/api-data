import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
export const platformDir = path.join(rootDir, "ai-data-platform");

export async function readJson(filePath) {
  return JSON.parse(await readFile(filePath, "utf8"));
}

export async function readJsonDir(dirPath) {
  const files = (await readdir(dirPath)).filter((file) => file.endsWith(".json"));
  const items = [];
  for (const file of files) {
    items.push(await readJson(path.join(dirPath, file)));
  }
  return items;
}

export async function writeJson(filePath, value) {
  await mkdir(path.dirname(filePath), { recursive: true });
  await writeFile(filePath, `${JSON.stringify(value, null, 2)}\n`, "utf8");
}
