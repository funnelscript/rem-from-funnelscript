#!/usr/bin/env bun
/**
 * REM Automated Browser Extraction Driver
 * Executes DOM inventory, design tokens, two-tier PII masking, and numbered UI annotations
 * on target SaaS applications using Playwright.
 */

import { chromium } from "playwright";
import * as fs from "node:fs";
import * as path from "node:path";

interface ExtractionOptions {
  url: string;
  dir: string;
  headless: boolean;
  mask: boolean;
  annotate: boolean;
  timeout: number;
  width: number;
  height: number;
  delay: number;
}

function parseArgs(args: string[]): ExtractionOptions {
  let url = "";
  let dir = "";
  let headless = true;
  let mask = true;
  let annotate = true;
  let timeout = 30000;
  let width = 1440;
  let height = 900;
  let delay = 1500;

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === "--help" || arg === "-h") {
      console.log(`
REM Automated Browser Extraction Driver

Usage:
  bun run tools/driver/extract.ts <url> [options]

Options:
  --dir <path>         Output directory (defaults to re-<domain>)
  --headless <bool>    Run browser headlessly (default: true)
  --mask <bool>        Apply two-tier DOM PII redaction (default: true)
  --annotate <bool>    Overlay numbered visual UI annotations (default: true)
  --timeout <ms>       Navigation timeout in ms (default: 30000)
  --viewport <w>x<h>   Viewport size (default: 1440x900)
  --delay <ms>         Post-load stabilization delay (default: 1500)
      `);
      process.exit(0);
    } else if (arg === "--dir" && i + 1 < args.length) {
      dir = args[++i];
    } else if (arg.startsWith("--dir=")) {
      dir = arg.split("=")[1];
    } else if (arg === "--headless=false" || arg === "--no-headless") {
      headless = false;
    } else if (arg === "--mask=false" || arg === "--no-mask") {
      mask = false;
    } else if (arg === "--annotate=false" || arg === "--no-annotate") {
      annotate = false;
    } else if (arg === "--timeout" && i + 1 < args.length) {
      timeout = parseInt(args[++i], 10);
    } else if (arg === "--viewport" && i + 1 < args.length) {
      const [w, h] = args[++i].split("x").map(Number);
      if (w && h) {
        width = w;
        height = h;
      }
    } else if (arg === "--delay" && i + 1 < args.length) {
      delay = parseInt(args[++i], 10);
    } else if (!arg.startsWith("-") && !url) {
      url = arg;
    }
  }

  if (!url) {
    console.error(JSON.stringify({ error: "Missing required URL argument" }));
    process.exit(1);
  }

  if (!url.startsWith("http://") && !url.startsWith("https://")) {
    url = "https://" + url;
  }

  if (!dir) {
    try {
      const parsed = new URL(url);
      const host = parsed.hostname.replace(/^www\./, "").replace(/[^a-zA-Z0-9-]/g, "-");
      dir = `re-${host}`;
    } catch {
      dir = "re-target";
    }
  }

  return { url, dir, headless, mask, annotate, timeout, width, height, delay };
}

function resolveScript(scriptName: string): string {
  const possiblePaths = [
    path.resolve(import.meta.dir, `../page/${scriptName}`),
    path.resolve(process.cwd(), `tools/page/${scriptName}`),
    path.resolve(process.cwd(), `../tools/page/${scriptName}`),
  ];
  for (const p of possiblePaths) {
    if (fs.existsSync(p)) {
      return fs.readFileSync(p, "utf-8");
    }
  }
  throw new Error(`Extraction script not found: ${scriptName}`);
}

async function main() {
  const opts = parseArgs(process.argv.slice(2));

  // Ensure output directory tree exists
  const absDir = path.resolve(process.cwd(), opts.dir);
  const subdirs = [
    absDir,
    path.join(absDir, "inventory"),
    path.join(absDir, "assets"),
    path.join(absDir, "features"),
    path.join(absDir, "rules"),
    path.join(absDir, "flows"),
    path.join(absDir, "data-model"),
    path.join(absDir, "barriers"),
    path.join(absDir, "sandbox"),
  ];
  for (const d of subdirs) {
    fs.mkdirSync(d, { recursive: true });
  }

  // Load scripts
  const inventoryScript = resolveScript("inventory.js");
  const tokensScript = resolveScript("tokens.js");
  const maskScript = resolveScript("mask.js");
  const annotateScript = resolveScript("annotate.js");

  // Launch browser with stealth flags
  const browser = await chromium.launch({
    headless: opts.headless,
    args: [
      "--no-sandbox",
      "--disable-setuid-sandbox",
      "--disable-blink-features=AutomationControlled",
      "--disable-infobars",
    ],
  });

  const context = await browser.newContext({
    viewport: { width: opts.width, height: opts.height },
    userAgent:
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
    deviceScaleFactor: 2,
  });

  const page = await context.newPage();

  try {
    // Navigate with timeout
    await page.goto(opts.url, {
      waitUntil: "domcontentloaded",
      timeout: opts.timeout,
    });

    // Stabilization delay
    await page.waitForTimeout(opts.delay);

    // 1. Run Inventory
    const inventoryResult = await page.evaluate((src) => {
      try {
        const fn = new Function(src);
        fn();
        const w = window as any;
        return w.__rem?.last || w.__rem?.inventory?.() || { url: window.location.href, title: document.title };
      } catch (err: any) {
        return { error: err.message, url: window.location.href, title: document.title };
      }
    }, inventoryScript);

    const inventoryPath = path.join(absDir, "inventory", "inventory.json");
    fs.writeFileSync(inventoryPath, JSON.stringify(inventoryResult, null, 2), "utf-8");

    // 2. Run Design Tokens
    const tokensResult = await page.evaluate((src) => {
      try {
        const fn = new Function(src);
        fn();
        const w = window as any;
        return w.__rem?.tokens?.() || w.__rem?.last || {};
      } catch (err: any) {
        return { error: err.message };
      }
    }, tokensScript);

    const tokensPath = path.join(absDir, "assets", "tokens.json");
    fs.writeFileSync(tokensPath, JSON.stringify(tokensResult, null, 2), "utf-8");

    // 3. Mask PII & Capture Clean Screenshot
    let cleanScreenshotPath = "";
    if (opts.mask) {
      await page.evaluate((src) => {
        try {
          const fn = new Function(src);
          fn();
          const w = window as any;
          if (w.__rem?.mask) {
            w.__rem.mask();
          }
        } catch {
          // ignore masking errors
        }
      }, maskScript);

      await page.waitForTimeout(300);
      cleanScreenshotPath = path.join(absDir, "assets", "screenshot-clean.png");
      await page.screenshot({ path: cleanScreenshotPath, fullPage: false });
    }

    // 4. Run Annotate & Capture Annotated Screenshot
    let annotatedScreenshotPath = "";
    let annotationsResult: any = null;
    if (opts.annotate) {
      annotationsResult = await page.evaluate((src) => {
        try {
          const fn = new Function(src);
          fn();
          const w = window as any;
          return w.__rem?.annotate?.() || w.__rem?.last || [];
        } catch (err: any) {
          return { error: err.message };
        }
      }, annotateScript);

      await page.waitForTimeout(300);
      annotatedScreenshotPath = path.join(absDir, "assets", "screenshot-annotated.png");
      await page.screenshot({ path: annotatedScreenshotPath, fullPage: false });

      const annotationsPath = path.join(absDir, "assets", "annotations.json");
      fs.writeFileSync(annotationsPath, JSON.stringify(annotationsResult, null, 2), "utf-8");
    }

    // Output structured summary
    const summary = {
      status: "success",
      url: opts.url,
      dir: opts.dir,
      inventory: {
        title: inventoryResult?.title || "",
        frameworks: inventoryResult?.frameworks || [],
        sdks: inventoryResult?.sdks || [],
        customTagsCount: inventoryResult?.customTags?.length || 0,
        ariaRolesCount: inventoryResult?.ariaRoles?.length || 0,
      },
      tokens: {
        colorsCount: Array.isArray(tokensResult?.colors) ? tokensResult.colors.length : 0,
        typographyCount: Array.isArray(tokensResult?.typography) ? tokensResult.typography.length : 0,
        radiiCount: Array.isArray(tokensResult?.radii) ? tokensResult.radii.length : 0,
      },
      artifacts: {
        inventoryJson: path.relative(process.cwd(), inventoryPath),
        tokensJson: path.relative(process.cwd(), tokensPath),
        cleanScreenshot: cleanScreenshotPath ? path.relative(process.cwd(), cleanScreenshotPath) : null,
        annotatedScreenshot: annotatedScreenshotPath ? path.relative(process.cwd(), annotatedScreenshotPath) : null,
        annotationsJson: opts.annotate ? path.relative(process.cwd(), path.join(absDir, "assets", "annotations.json")) : null,
      },
    };

    console.log(JSON.stringify(summary, null, 2));
  } catch (err: any) {
    console.error(
      JSON.stringify({
        status: "error",
        url: opts.url,
        dir: opts.dir,
        message: err.message,
      }, null, 2)
    );
    process.exit(1);
  } finally {
    await browser.close();
  }
}

main().catch((err) => {
  console.error("Fatal extraction error:", err);
  process.exit(1);
});
