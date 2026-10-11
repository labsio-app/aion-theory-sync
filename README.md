# aion-theory-sync

[![CI](https://github.com/labsio-app/aion-theory-sync/actions/workflows/ci.yml/badge.svg)](https://github.com/labsio-app/aion-theory-sync/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/labsio-app/aion-theory-sync)](https://github.com/labsio-app/aion-theory-sync/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Sync your AION 2 DPS meter combat logs to your [Aion Theory](https://aion-theory.labsio.app) account.

🇫🇷 [Français](#français) · 🇬🇧 [English](#english)

---

## Français

<details open>
<summary>Déplier / replier</summary>

Un seul exécutable, sans dépendance, pour Windows. Il lit les journaux de
combat que le [DPS Meter A2Tools](https://a2tools.app) écrit sur votre
machine et envoie chaque nouveau combat vers votre compte Aion Theory, où les
mesures sont attachées à votre personnage déclaré.

### Fonctionnement

1. Téléchargez la dernière version, décompressez-la et lancez
   `aion-theory-sync.exe`.
2. Au premier lancement, un code et un lien s'affichent : ouvrez le lien,
   connectez-vous à Aion Theory, puis saisissez le code. L'outil est alors
   lié à votre compte — il ne voit jamais votre mot de passe, seulement une
   habilitation révocable que vous pouvez retirer à tout moment depuis votre
   page de compte.
3. Chaque lancement parcourt `%APPDATA%\com.a2tools.dps-meter\history\` et
   envoie les combats pas encore synchronisés. La synchronisation est
   idempotente : un nouveau lancement ne duplique jamais rien, un envoi en
   échec est retenté sans risque, et les journaux d'un personnage déclaré
   plus tard sont repris au lancement suivant.

Relancez l'outil quand vous le souhaitez pour synchroniser vos derniers
combats — cette version ne tourne pas en arrière-plan.

### Options

```
aion-theory-sync.exe [flags]

  -server string        serveur Aion Theory (défaut "https://aion-theory.labsio.app")
  -history-dir string    dossier d'historique du DPS meter A2Tools (défaut : %APPDATA%\com.a2tools.dps-meter\history)
  -config-dir string     où l'outil garde sa propre habilitation (défaut : %APPDATA%\aion-theory-sync)
  -relink                oublier l'habilitation enregistrée et relier cet appareil
```

### Ce qui est envoyé

Exactement ce que le DPS meter a déjà écrit sur le disque pour un combat :
boss, durée, dégâts, détail par compétence, et la ligne de chaque
participant telle que le meter l'a masquée ou démasquée. Aion Theory
rapproche le participant démasqué de votre personnage déclaré et enregistre
le combat sur celui-ci ; les lignes des autres participants sont conservées
telles que reçues — jamais enrichies, jamais démasquées, jamais montrées à
quiconque au-delà de ce que votre groupe voyait déjà dans le meter lui-même.

Le seul appel réseau que cet outil effectue va vers l'adresse `-server`
ci-dessus (votre compte Aion Theory) — rien d'autre, aucune télémétrie,
aucun tiers.

### Votre responsabilité

- Relier cet outil autorise des envois *depuis cette machine* vers *votre
  propre* compte Aion Theory, jusqu'à ce que vous révoquiez l'accès depuis
  votre page de compte ou que vous utilisiez `-relink`.
- Les données de combat proviennent d'un outil tiers (A2Tools) auquel ce
  projet n'est pas affilié ; son usage reste soumis à ses propres conditions
  ainsi qu'aux conditions d'utilisation d'AION 2, que cet outil ne modifie
  en rien.
- L'habilitation stockée localement par l'outil ne sert qu'à l'envoi des
  journaux de combat — ce n'est pas le mot de passe de votre compte, et la
  perdre ne coûte qu'une révocation et une nouvelle liaison.
- Ce logiciel est fourni "tel quel", sans garantie d'aucune sorte — voir
  [LICENSE](LICENSE).

### Open source, par conception

Un outil qui demande à relier votre compte et à lire des fichiers sur votre
machine ne devrait pas demander une confiance aveugle. Ce dépôt est
intégralement la source réelle de chaque version publiée. Rien n'est caché.

- Lisez le code avant de l'exécuter : la liaison commence dans
  [`main.go`](main.go), le flux d'autorisation dans
  [`internal/auth/`](internal/auth), ce qui est envoyé dans
  [`internal/sync/`](internal/sync).
- Chaque version est construite par
  [GitHub Actions](.github/workflows/release.yml) à partir de cette source
  exacte, pas depuis une machine à qui il faudrait faire confiance —
  comparez les sommes de contrôle publiées, ou construisez l'outil
  vous-même : `go build` depuis un clone propre, sans dépendance externe à
  récupérer.

</details>

---

## English

<details>
<summary>Expand / collapse</summary>

A single, dependency-free executable for Windows. It reads the combat logs
the [A2Tools DPS Meter](https://a2tools.app) writes on your machine and
uploads each new combat to your Aion Theory account, where the measures are
attached to your declared character.

### How it works

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

### What is uploaded

Exactly what the DPS meter already wrote to disk for one fight: boss,
duration, damage, skill breakdown, and every actor's row as the meter masked
or unmasked it. Aion Theory matches the unmasked actor against your declared
character and records the combat against it; other actors' rows are kept
exactly as received — never enriched, never un-masked, never shown to anyone
beyond what your group already saw in the meter itself.

The only network call this tool ever makes is to the `-server` address above
(your Aion Theory account) — nothing else, no telemetry, no third party.

### Your responsibility

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

### Open source, by design

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

</details>

---

## License

[MIT](LICENSE)
