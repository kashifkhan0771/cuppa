// Builds ESM and CommonJS output with the tsc from node_modules.
import { execFileSync } from "node:child_process";
import { mkdirSync, rmSync, writeFileSync } from "node:fs";

const tsc = ["node_modules/typescript/bin/tsc"];
rmSync("dist", { recursive: true, force: true });
execFileSync("node", [...tsc, "-p", "tsconfig.esm.json"], { stdio: "inherit" });
execFileSync("node", [...tsc, "-p", "tsconfig.cjs.json"], { stdio: "inherit" });

// tell Node which folder is which module type
for (const [dir, type] of [["esm", "module"], ["cjs", "commonjs"]]) {
  mkdirSync(`dist/${dir}`, { recursive: true });
  writeFileSync(`dist/${dir}/package.json`, JSON.stringify({ type }) + "\n");
}
