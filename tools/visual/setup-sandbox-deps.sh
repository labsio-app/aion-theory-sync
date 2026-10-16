#!/usr/bin/env sh
# Downloads the handful of shared libraries headless Chromium needs
# (libnspr4, libnss3 and friends) without root, for a sandbox where
# `playwright install-deps` can't run apt as root. Extracts the .debs
# into a local prefix and prints the LD_LIBRARY_PATH to export.
#
# On a normal dev machine with sudo, prefer:
#   npx playwright install-deps chromium
# and skip this script entirely.
set -eu

PREFIX="${1:-/tmp/aion-visual-deps}"
mkdir -p "$PREFIX/debs" "$PREFIX/extracted"

cd "$PREFIX/debs"
apt-get download libnspr4 libnss3

for f in *.deb; do
  dpkg-deb -x "$f" "$PREFIX/extracted"
done

LIBDIR="$PREFIX/extracted/usr/lib/x86_64-linux-gnu"
echo "Libraries extracted to: $LIBDIR"
echo
echo "Export this before running capture.mjs/diff.mjs:"
echo "  export LD_LIBRARY_PATH=\"$LIBDIR:\${LD_LIBRARY_PATH:-}\""
