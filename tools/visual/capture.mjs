// Captures one PNG screenshot per fixture served by `go run ./cmd/gui-preview`
// (internal/webui.Fixtures): the visual-regression baselines this tool's
// sibling diff.mjs compares future renders against. Not shipped — dev
// tooling only.
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const baseURL = process.env.PREVIEW_URL || "http://127.0.0.1:4173";
const outDir = process.env.OUT_DIR || join(here, "baselines");

async function fixtureNames() {
  const res = await fetch(baseURL + "/");
  const html = await res.text();
  const matches = [...html.matchAll(/\/preview\/([^"]+)"/g)];
  return matches.map((m) => m[1]);
}

async function main() {
  mkdirSync(outDir, { recursive: true });
  const names = await fixtureNames();
  if (names.length === 0) {
    console.error("No fixtures found at " + baseURL + " — is gui-preview running?");
    process.exit(1);
  }

  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 960, height: 600 } });

  for (const name of names) {
    await page.goto(`${baseURL}/preview/${name}`, { waitUntil: "networkidle" });
    const path = join(outDir, `${name}.png`);
    await page.screenshot({ path });
    console.log("captured", name);
  }

  // A couple of screens get a second pass at the window's documented
  // MinSize (800x520, app.go) to prove the layout holds at the smallest
  // supported size instead of deforming — the exact complaint that
  // started this rewrite.
  await page.setViewportSize({ width: 800, height: 520 });
  for (const name of ["02-prerequisites-complete", "04-syncing", "08-settings"]) {
    if (!names.includes(name)) continue;
    await page.goto(`${baseURL}/preview/${name}`, { waitUntil: "networkidle" });
    const path = join(outDir, `${name}--small.png`);
    await page.screenshot({ path });
    console.log("captured", name, "(small viewport)");
  }

  await browser.close();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
