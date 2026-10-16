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

Une application installable pour Windows. Elle lit les journaux de combat
que le [DPS Meter A2Tools](https://a2tools.app) écrit sur votre machine et
envoie chaque nouveau combat vers votre compte Aion Theory, où les mesures
sont attachées à votre personnage déclaré.

### Fonctionnement

1. Téléchargez `aion-theory-sync-setup.exe` depuis la
   [dernière version](https://github.com/labsio-app/aion-theory-sync/releases/latest)
   et installez-le — un dossier d'installation, un raccourci dans le menu
   Démarrer et, si vous le cochez, sur le bureau.
2. Au premier lancement, un parcours guidé vous relie à votre compte : un
   code s'affiche et le navigateur s'ouvre sur Aion Theory, vous
   connectez-vous et saisissez le code. L'outil ne voit jamais votre mot
   de passe, seulement une habilitation révocable que vous pouvez retirer
   à tout moment depuis votre page de compte ou depuis les paramètres de
   l'application (« Délier ce PC »).
3. L'outil vérifie ensuite que le DPS Meter A2Tools est détecté (sinon, il
   vous laisse indiquer son dossier d'historique une fois) et vous propose
   de lancer l'application au démarrage de Windows et de synchroniser
   automatiquement les nouveaux combats — les deux sont activés par
   défaut, modifiables à tout moment dans les paramètres.
4. Depuis l'écran principal, « Synchroniser maintenant » envoie les
   combats pas encore synchronisés. La synchronisation est idempotente :
   la relancer ne duplique jamais rien, un envoi en échec est retenté sans
   risque, et les journaux d'un personnage déclaré plus tard sont repris
   automatiquement.

Avec la synchronisation automatique activée (par défaut), l'application
surveille en arrière-plan le dossier d'historique du DPS meter et envoie
les nouveaux combats d'elle-même, quelques secondes après la fin d'un
combat ; fermer la fenêtre la réduit alors dans la zone de notification
plutôt que de la quitter (clic sur l'icône, ou menu « Quitter », pour
fermer réellement). Avec la synchronisation automatique désactivée,
l'application ne fait rien tant que vous ne cliquez pas sur
« Synchroniser maintenant ». À chaque lancement, elle vérifie en silence
si une version plus récente est publiée et, le cas échéant, propose de
l'installer en un clic (pas de réouverture du navigateur, pas de nouveau
téléchargement manuel).

> **Windows peut encore afficher un avertissement SmartScreen** au premier
> lancement (« Éditeur non reconnu ») : l'installeur n'est pas encore signé.
> Voir [docs/code-signing.md](docs/code-signing.md) pour le statut et la
> démarche (signature gratuite via SignPath, pour les projets open source).

> **L'application a besoin du WebView2 Runtime**, fourni par défaut avec
> Windows 11 et installé automatiquement sur la plupart des Windows 10 via
> Windows Update. S'il manque (poste figé ou hors-ligne), l'application
> l'indique clairement au lancement avec un lien de téléchargement, plutôt
> que de planter silencieusement.

### Options avancées (ligne de commande)

L'installeur pose aussi `aion-theory-sync.exe`, un exécutable console sans
fenêtre pour un usage scripté ou un serveur de staging :

```
aion-theory-sync.exe [flags]

  -server string        serveur Aion Theory (défaut "https://aion-theory.labsio.app")
  -history-dir string    dossier d'historique du DPS meter A2Tools (défaut : %APPDATA%\com.a2tools.dps-meter\history)
  -config-dir string     où l'outil garde sa propre habilitation (défaut : %APPDATA%\aion-theory-sync)
  -relink                oublier l'habilitation enregistrée et relier cet appareil
```

Ce même exécutable, en version autonome (l'exécutable brut, sans zip ni
installeur), reste publié à chaque version.

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
  en rien. L'icône A2Tools affichée dans l'application
  ([`assets/a2tools.png`](assets/a2tools.png)) est reprise telle quelle du
  dépôt officiel [`taengu/A2Tools-DPS-Meter`](https://github.com/taengu/A2Tools-DPS-Meter)
  (`src-tauri/icons/Square142x142Logo.png`).
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
  [`main.go`](main.go) (console) et [`cmd/gui/`](cmd/gui) (application), le
  flux d'autorisation dans [`internal/auth/`](internal/auth), ce qui est
  envoyé dans [`internal/sync/`](internal/sync).
- Chaque version est construite par
  [GitHub Actions](.github/workflows/release.yml) à partir de cette source
  exacte, pas depuis une machine à qui il faudrait faire confiance —
  comparez les sommes de contrôle publiées, ou construisez l'outil
  vous-même : `go build` depuis un clone propre. L'exécutable console
  (`main.go`) n'a toujours aucune dépendance externe ; l'application
  (`cmd/gui`) en a quelques-unes, toutes open source et épinglées dans
  [`go.mod`](go.mod) — [`tailscale/walk`](https://github.com/tailscale/walk)
  pour la fenêtre native Windows et ce qu'elle entraîne avec elle
  (`golang.org/x/sys`, notamment), rien d'opaque ni de compilé à l'avance.

</details>

---

## English

<details>
<summary>Expand / collapse</summary>

An installable Windows application. It reads the combat logs the
[A2Tools DPS Meter](https://a2tools.app) writes on your machine and uploads
each new combat to your Aion Theory account, where the measures are attached
to your declared character.

### How it works

1. Download `aion-theory-sync-setup.exe` from the
   [latest release](https://github.com/labsio-app/aion-theory-sync/releases/latest)
   and install it — an install folder, a Start Menu shortcut, and a desktop
   icon if you check that box.
2. On first launch, a guided flow links you to your account: a code
   appears and your browser opens on Aion Theory, you sign in and enter
   the code. The tool never sees your password, only a revocable
   credential you can take back any time from your account page or from
   the app's own settings ("Délier ce PC").
3. It then checks that the A2Tools DPS Meter is detected (if not, you can
   point it at the right history folder once) and offers to launch at
   Windows startup and auto-sync new combats — both on by default,
   changeable any time in settings.
4. From the main screen, "Synchroniser maintenant" uploads the combats not
   yet synced. The sync is idempotent: re-running it never duplicates
   anything, a failed upload is safely retried, and logs for a character
   you declare later are picked up automatically.

With auto-sync on (the default), the app watches the DPS meter's history
folder in the background and uploads new combats on its own, a few
seconds after a fight ends; closing the window then minimizes it to the
notification area instead of quitting (click the icon, or its "Quitter"
menu entry, to actually close it). With auto-sync off, the app does
nothing until you click "Synchroniser maintenant". On every launch it
quietly checks whether a newer version was published and, if so, offers
to install it in one click — no browser, no manual re-download.

> **Windows may still show a SmartScreen warning** ("Unrecognized
> publisher") on first launch: the installer is not signed yet. See
> [docs/code-signing.md](docs/code-signing.md) for the plan (free signing
> through SignPath for open-source projects).

> **The app needs the WebView2 Runtime**, bundled by default with Windows
> 11 and installed automatically on most Windows 10 machines via Windows
> Update. If it's missing (a locked-down or offline machine), the app says
> so clearly at launch with a download link, instead of silently crashing.

### Advanced options (command line)

The installer also places `aion-theory-sync.exe`, a windowless console
build for scripted use or a staging server:

```
aion-theory-sync.exe [flags]

  -server string        Aion Theory server (default "https://aion-theory.labsio.app")
  -history-dir string    A2Tools DPS meter history folder (default: %APPDATA%\com.a2tools.dps-meter\history)
  -config-dir string     where this tool keeps its own credential (default: %APPDATA%\aion-theory-sync)
  -relink                forget the stored credential and link this device again
```

This same executable also keeps shipping on its own, as a raw binary with
no zip and no installer, on every release.

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
  to AION 2's terms of service, which this tool does not alter. The A2Tools
  icon shown in the app ([`assets/a2tools.png`](assets/a2tools.png)) is
  taken as-is from the official
  [`taengu/A2Tools-DPS-Meter`](https://github.com/taengu/A2Tools-DPS-Meter)
  repository (`src-tauri/icons/Square142x142Logo.png`).
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

- Read the code before you run it: linking starts in [`main.go`](main.go)
  (console) and [`cmd/gui/`](cmd/gui) (the app), the device flow in
  [`internal/auth/`](internal/auth), what gets uploaded in
  [`internal/sync/`](internal/sync).
- Every release is built by [GitHub Actions](.github/workflows/release.yml)
  from this exact source, not from a machine you have to trust — compare the
  published checksums, or build it yourself: `go build` from a clean clone.
  The console build (`main.go`) still has no external dependency; the app
  (`cmd/gui`) has a few, all open source and pinned in
  [`go.mod`](go.mod) — [`tailscale/walk`](https://github.com/tailscale/walk)
  for the native Windows window and what it brings with it
  (`golang.org/x/sys`, notably), nothing closed or pre-compiled.

</details>

---

## License

[MIT](LICENSE)
