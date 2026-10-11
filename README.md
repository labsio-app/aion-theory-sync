# aion-theory-sync

Sync your AION 2 DPS meter combat logs to [Aion Theory](https://github.com/labsio-app/aion-theory).

A single, dependency-free executable for Windows. It reads the combat logs the
[A2Tools DPS Meter](https://a2tools.app) writes on your machine and uploads
each new combat to your Aion Theory account, where the measures are attached
to your declared character.

## How it works

1. Download the latest release and run it.
2. The first run shows a short code: open the link, enter the code, and the
   tool is linked to your Aion Theory account (OAuth device flow — the tool
   never sees your password).
3. Every run scans `%APPDATA%\com.a2tools.dps-meter\history\` and uploads the
   combats not yet synced. The sync is idempotent: re-running it never
   duplicates anything, and logs for a character you declare later are picked
   up by the next run.

## Status

Under development — see the
[US-COMBAT-001 story](https://github.com/labsio-app/aion-theory) in the
platform repository.

## License

[MIT](LICENSE)
