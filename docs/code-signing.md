# Code signing (SignPath, free for open source)

Windows SmartScreen flags `aion-theory-sync-setup.exe` as untrusted because
it is unsigned. [SignPath.io](https://signpath.io) signs builds of
open-source projects for free; `.github/workflows/release.yml` already has
the signing step wired in, inactive until the secrets below exist — nothing
else in the release breaks in the meantime.

## One-time setup (not something CI can do on its own)

1. **Apply.** Create a SignPath account and apply to their
   [open-source program](https://signpath.io/pricing) for this repository
   (`labsio-app/aion-theory-sync`, MIT-licensed, public). Approval is manual
   on their side.
2. **Create a project** in the SignPath dashboard for this repository, with
   a **signing policy** (how the certificate may be used — release builds
   only, is the usual default) and an **artifact configuration** describing
   what is signed (a single Authenticode-signed `.exe`).
3. **Connect SignPath to this GitHub repository** as a trusted build
   system, following SignPath's
   [GitHub integration guide](https://about.signpath.io/documentation/trusted-build-systems/github) —
   this is what lets the `signpath/github-action-submit-signing-request`
   step in CI authenticate without the certificate's private key ever
   touching GitHub.
4. **Add two repository secrets** (Settings → Secrets and variables →
   Actions):
   - `SIGNPATH_API_TOKEN`
   - `SIGNPATH_ORGANIZATION_ID`
5. **Fill in the three slugs** the signing step in
   `.github/workflows/release.yml` currently carries as placeholders
   (`project-slug`, `signing-policy-slug`, `artifact-configuration-slug`),
   reading their actual values off the SignPath dashboard once the project
   from step 2 exists.

Once all five are in place, the next `v*` tag push signs the installer
automatically — no further workflow change needed.

## What to expect afterward

A SignPath-issued certificate is not Extended Validation: SmartScreen's
trust builds with download volume over time rather than disappearing
immediately on the first signed release. It is still a materially better
starting point than an unsigned binary (a visible, verifiable publisher
identity on the UAC/SmartScreen prompt, immediately) and costs nothing.
