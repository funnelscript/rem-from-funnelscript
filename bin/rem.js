#!/usr/bin/env node

/**
 * REM CLI Launcher (Node.js & Bun compatible)
 * Finds or builds the native Go REM binary and transparently streams stdio.
 */

import { spawn, execSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, "..");

// Locate precompiled binary
const candidates = [
  path.join(rootDir, "rem"),
  path.join(rootDir, "rem-from-funnelscript", "rem"),
  path.join(process.cwd(), "rem"),
];

let binaryPath = candidates.find((p) => fs.existsSync(p));

if (!binaryPath) {
  // Attempt to build if go is installed
  let goSourceDir = path.join(rootDir, "rem-from-funnelscript");
  if (!fs.existsSync(goSourceDir) && fs.existsSync(path.join(rootDir, "cmd", "rem"))) {
    goSourceDir = rootDir;
  }
  if (fs.existsSync(goSourceDir)) {
    try {
      execSync("go build -trimpath -ldflags=\"-s -w\" -o rem ./cmd/rem", {
        cwd: goSourceDir,
        stdio: "ignore",
      });
      const built = path.join(goSourceDir, "rem");
      if (fs.existsSync(built)) {
        binaryPath = built;
      }
    } catch {
      // Build failed or go not installed
    }
  }
}

if (!binaryPath) {
  console.error("Error: REM binary not found. Please run `make build` or `./install.sh` to compile REM.");
  process.exit(1);
}

// Forward REM_ROOT for driver resolution
const env = { ...process.env, REM_ROOT: rootDir };

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: "inherit",
  env,
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
  } else {
    process.exit(code ?? 0);
  }
});
