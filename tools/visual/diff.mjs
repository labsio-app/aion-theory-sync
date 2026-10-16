// Re-captures every fixture into a fresh "current" directory and compares
// each one, pixel by pixel, against tools/visual/baselines/ — the actual
// regression check: fails (non-zero exit, listing which screens changed)
// when a code change moved more than a few stray pixels, and leaves the
// diff PNGs in current/ for a human to look at. Re-baseline a screen you
// changed on purpose with `cp current/<name>.png baselines/<name>.png`.
import { PNG } from "pngjs";
import pixelmatch from "pixelmatch";
import { readFileSync, readdirSync, mkdirSync, writeFileSync } from "node:fs";
import { execSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const baselineDir = join(here, "baselines");
const currentDir = join(here, "current");
const threshold = Number(process.env.VISUAL_THRESHOLD || 0.1); // pixelmatch per-pixel sensitivity
const maxDiffPixels = Number(process.env.VISUAL_MAX_DIFF_PIXELS || 40); // allow anti-aliasing noise

function readPng(path) {
  return PNG.sync.read(readFileSync(path));
}

async function main() {
  mkdirSync(currentDir, { recursive: true });
  execSync("node capture.mjs", { cwd: here, env: { ...process.env, OUT_DIR: currentDir }, stdio: "inherit" });

  const baselineNames = readdirSync(baselineDir).filter((n) => n.endsWith(".png"));
  let failures = [];

  for (const name of baselineNames) {
    const basePath = join(baselineDir, name);
    const curPath = join(currentDir, name);
    let cur;
    try {
      cur = readPng(curPath);
    } catch {
      failures.push(`${name}: missing from current capture`);
      continue;
    }
    const base = readPng(basePath);
    if (base.width !== cur.width || base.height !== cur.height) {
      failures.push(`${name}: size changed (${base.width}x${base.height} -> ${cur.width}x${cur.height})`);
      continue;
    }
    const diff = new PNG({ width: base.width, height: base.height });
    const diffPixels = pixelmatch(base.data, cur.data, diff.data, base.width, base.height, { threshold });
    if (diffPixels > maxDiffPixels) {
      const diffPath = join(currentDir, name.replace(/\.png$/, ".diff.png"));
      writeFileSync(diffPath, PNG.sync.write(diff));
      failures.push(`${name}: ${diffPixels} pixels differ (see ${diffPath})`);
    }
  }

  if (failures.length > 0) {
    console.error("Régressions visuelles détectées:\n" + failures.map((f) => "  - " + f).join("\n"));
    process.exit(1);
  }
  console.log(`OK — ${baselineNames.length} écrans conformes aux références.`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
