# Visual regression — cmd/gui's HTML/CSS screens

`internal/webui` renders every screen as server-side HTML/CSS (the
replacement for the old walk-widget rendering). This directory screenshots
every fixture (`internal/webui.Fixtures()`) with headless Chromium and
diffs the result against committed baselines, so a CSS/template change
that visibly breaks a screen fails instead of shipping silently.

## One-time setup

```sh
npm install
npx playwright install chromium
```

On a normal dev machine with sudo, also run
`npx playwright install-deps chromium` once (needs root) so headless
Chromium's system libraries (NSS, NSPR, …) are present. On a sandbox
without sudo, download just those three packages and point
`LD_LIBRARY_PATH` at the extracted `.so` files instead — see
`./setup-sandbox-deps.sh`, which does exactly that into
`/tmp/*/aion-deps` and prints the `LD_LIBRARY_PATH` to export.

## Running it

```sh
# 1. serve every fixture
go run ./cmd/gui-preview &

# 2. capture baselines (first time, or after a deliberate visual change)
node capture.mjs

# 3. check for regressions (captures into current/, diffs against baselines/)
node diff.mjs
```

`diff.mjs` exits non-zero and lists every screen that changed by more than
a few stray anti-aliasing pixels, with a `<name>.diff.png` highlighting
what moved. To accept a deliberate visual change as the new baseline:

```sh
cp current/<name>.png baselines/<name>.png
```

Each fixture is captured at the app's default window size (960×600) and
three representative screens are also captured at the documented minimum
window size (800×520, `<name>--small.png`) specifically to catch layout
that holds together at the default size but breaks/overlaps once the
window is resized down.

## Manual review

`go run ./cmd/gui-preview` also serves a plain index at `http://127.0.0.1:4173/`
linking every fixture — open it in any browser to eyeball a screen
directly, no screenshot pipeline needed.
