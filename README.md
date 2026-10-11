# aion-theory-sync

Sync your AION 2 DPS meter combat logs to [Aion Theory](https://github.com/labsio-app/aion-theory).

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
or unmasked it. The platform matches the unmasked actor against your
declared character and records the combat against it; other actors' rows are
kept exactly as received, never enriched or un-masked. See
[ADR 0064](https://github.com/labsio-app/aion-theory/blob/main/docs/adr/0064-a-members-combat-logs-sync-through-a-device-authorized-tool.md)
for the full data-handling decision.

## Status

See the [US-COMBAT-001 story](https://github.com/labsio-app/aion-theory) in
the platform repository.

## License

[MIT](LICENSE)
