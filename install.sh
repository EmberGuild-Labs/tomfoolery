#!/bin/sh
# tomfoolery installer.
#
#   curl -fsSL https://raw.githubusercontent.com/EmberGuild-Labs/tomfoolery/main/install.sh | sh
#
# Downloads the prebuilt universal macOS binary from the latest GitHub
# release, checks its SHA-256, installs it to ~/.local/bin (no sudo), and
# runs `tomfoolery install`.
#
# Options (environment variables):
#   TOMFOOLERY_PREFIX=/usr/local   install to $PREFIX/bin instead of ~/.local/bin
#   TOMFOOLERY_VERSION=v0.1.0      install a specific release instead of the latest
#   TOMFOOLERY_NO_INTRO=1          skip the EmberGuild Labs intro
#   TOMFOOLERY_NO_SETUP=1          install the binary only; don't run `tomfoolery install`
set -eu

REPO="EmberGuild-Labs/tomfoolery"
ASSET="tomfoolery-darwin-universal.tar.gz"
PREFIX="${TOMFOOLERY_PREFIX:-$HOME/.local}"
VERSION="${TOMFOOLERY_VERSION:-latest}"
BIN_DIR="$PREFIX/bin"

say() { printf '%s\n' "$*"; }
die() { printf 'tomfoolery installer: %s\n' "$*" >&2; exit 1; }

[ "$(uname -s)" = "Darwin" ] || die "tomfoolery is macOS only (this is $(uname -s))."
command -v curl >/dev/null || die "curl is required."
command -v shasum >/dev/null || die "shasum is required."

if [ "$VERSION" = "latest" ]; then
  base="https://github.com/$REPO/releases/latest/download"
else
  base="https://github.com/$REPO/releases/download/$VERSION"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

say "Downloading tomfoolery ($VERSION)..."
curl -fsSL "$base/$ASSET" -o "$tmp/$ASSET" || die "download failed: $base/$ASSET"
curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS" || die "download failed: $base/SHA256SUMS"

say "Verifying checksum..."
(cd "$tmp" && grep " $ASSET\$" SHA256SUMS | shasum -a 256 -c - >/dev/null) \
  || die "checksum mismatch; not installing."

tar -xzf "$tmp/$ASSET" -C "$tmp"
mkdir -p "$BIN_DIR" || die "cannot create $BIN_DIR (for /usr/local, run with sudo or pick another TOMFOOLERY_PREFIX)"
install -m 0755 "$tmp/tomfoolery" "$BIN_DIR/tomfoolery"
say "Installed $BIN_DIR/tomfoolery ($("$BIN_DIR/tomfoolery" version))"

if [ -n "${TOMFOOLERY_NO_SETUP:-}" ]; then
  say "Skipping setup. Run '$BIN_DIR/tomfoolery install' when you're ready."
  exit 0
fi

say ""
# When piped into sh, stdin is this script, so hand setup the real terminal.
# That lets the intro play and respond to a keypress.
if [ -r /dev/tty ] && [ -w /dev/tty ] && (exec </dev/tty) 2>/dev/null; then
  "$BIN_DIR/tomfoolery" install </dev/tty >/dev/tty 2>&1
else
  "$BIN_DIR/tomfoolery" install --no-intro
fi

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    say ""
    say "Note: $BIN_DIR is not on your PATH. The ls and kill wrappers work anyway,"
    say "but to run ls-gossip, uptime-brag and friends by name, add this to ~/.zshrc:"
    say "  export PATH=\"$BIN_DIR:\$PATH\""
    ;;
esac
