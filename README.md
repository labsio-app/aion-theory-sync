# aion-theory-sync

[![CI](https://github.com/labsio-app/aion-theory-sync/actions/workflows/ci.yml/badge.svg)](https://github.com/labsio-app/aion-theory-sync/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/labsio-app/aion-theory-sync)](https://github.com/labsio-app/aion-theory-sync/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Sync your AION 2 DPS meter combat logs to your [Aion Theory](https://aion-theory.labsio.app) account.

A single, dependency-free executable for Windows. It reads the combat logs the
[A2Tools DPS Meter](https://a2tools.app) writes on your machine and uploads
each new combat to your Aion Theory account, where the measures are attached
to your declared character.

## How it works

1. Download the latest release, unzip it, and run `aion-theory-sync.exe`.
2. The first run shows a short code and a link: open the link, sign in to
   Aion Theory, and enter the code. The tool is now linked to your account —
   it never sees your password, only a revocable credential you can take back
   any time from your account page.
3. Every run scans `%APPDATA%\com.a2tools.dps-meter\history\` and uploads the
   combats not yet synced. The sync is idempotent: re-running it never
   duplicates anything, a failed upload is safely retried, and logs for a
   character you declare later are picked up by the next run.

Run it again whenever you want your latest combats synced — there is no
background watcher in this version.

### Options

```
aion-theory-sync.exe [flags]

  -server string        Aion Theory server (default "https://aion-theory.labsio.app")
  -history-dir string    A2Tools DPS meter history folder (default: %APPDATA%\com.a2tools.dps-meter\history)
  -config-dir string     where this tool keeps its own credential (default: %APPDATA%\aion-theory-sync)
  -relink                forget the stored credential and link this device again
```

## What is uploaded

Exactly what the DPS meter already wrote to disk for one fight: boss,
duration, damage, skill breakdown, and every actor's row as the meter masked
or unmasked it. Aion Theory matches the unmasked actor against your declared
character and records the combat against it; other actors' rows are kept
exactly as received — never enriched, never un-masked, never shown to anyone
beyond what your group already saw in the meter itself.

The only network call this tool ever makes is to the `-server` address above
(your Aion Theory account) — nothing else, no telemetry, no third party.

## Your responsibility

- Linking this tool authorizes uploads *from this machine* to *your own*
  Aion Theory account, until you revoke it from your account page or run
  `-relink`.
- The combat data comes from a third-party tool (A2Tools) that this project
  is not affiliated with; using it remains subject to A2Tools' own terms and
  to AION 2's terms of service, which this tool does not alter.
- The credential this tool stores locally is scoped to uploading combat logs
  only — it is not your account password, and losing it only costs a revoke
  and a re-link.
- This software is provided "as is", without warranty of any kind — see
  [LICENSE](LICENSE).

## Open source, by design

A tool that asks you to link your account and read files from your machine
should not ask for blind trust. This entire repository — the device-link
flow, what gets read from disk, what gets sent over the wire — is the real
source of every release. Nothing is withheld.

- Read the code before you run it: linking starts in [`main.go`](main.go),
  the device flow in [`internal/auth/`](internal/auth), what gets uploaded in
  [`internal/sync/`](internal/sync).
- Every release is built by [GitHub Actions](.github/workflows/release.yml)
  from this exact source, not from a machine you have to trust — compare the
  published checksums, or build it yourself: `go build` from a clean clone,
  no external dependencies to pull in.

## License

[MIT](LICENSE)
