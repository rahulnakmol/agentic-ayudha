#!/bin/sh
# Install aayudha (alias aayu) from a signed GitHub release (docs/BUILD-PLAN.md WP0.4).
#
#   curl -fsSL https://raw.githubusercontent.com/rahulnakmol/agentic-ayudha/main/install.sh | sh
#
# What it checks before anything is installed:
#   1. checksums.txt carries a Sigstore bundle whose certificate was issued to this repository's
#      release.yml workflow running on the exact v* tag being installed (keyless cosign signature);
#   2. the downloaded archive's SHA-256 matches its line in that signed checksums.txt.
# A failure in either step installs nothing and exits non-zero.
#
# Environment:
#   AAYU_VERSION      release to install, e.g. v0.1.0 (default: the latest release)
#   AAYU_INSTALL_DIR  where to put the binary (default: $HOME/.local/bin)
#   AAYU_BASE_URL     testing only: a directory or URL holding the release assets
#   AAYU_CERT_IDENTITY_REGEXP
#                     testing only: accept another signing workflow identity (CI snapshot tests)
#
# cosign is used from PATH; if it is missing, a pinned cosign release is downloaded and checked against the
# SHA-256 below before it runs.
set -eu

REPO="rahulnakmol/agentic-ayudha"
ISSUER="https://token.actions.githubusercontent.com"
COSIGN_VERSION="v3.1.3"

say() { printf 'aayudha install: %s\n' "$*" >&2; }
die() { say "error: $*"; exit 1; }

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | cut -d' ' -f1
  else die "need sha256sum or shasum"
  fi
}

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  MINGW* | MSYS* | CYGWIN*) os=windows ;;
  *) die "unsupported OS: $(uname -s)" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

base="${AAYU_BASE_URL:-}"
fetch() { # fetch <asset> <dest>
  if [ -n "$base" ] && [ -d "$base" ]; then
    cp "$base/$1" "$2" 2>/dev/null || die "missing asset $1 in $base"
  else
    curl -fsSL --proto '=https' --tlsv1.2 -o "$2" "$base/$1" || die "download failed: $base/$1"
  fi
}

version="${AAYU_VERSION:-}"
if [ -z "$version" ]; then
  [ -z "$base" ] || die "set AAYU_VERSION when AAYU_BASE_URL is set"
  latest="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")" ||
    die "could not resolve the latest release"
  version="${latest##*/}"
fi
version="v${version#v}"
case "$version" in
  v[0-9]*) ;;
  *) die "not a release version: $version" ;;
esac
[ -n "$base" ] || base="https://github.com/$REPO/releases/download/$version"

# Only release.yml on exactly this tag may have signed the checksums.
tag_re="$(printf '%s' "$version" | sed 's/[.+]/\\&/g')"
identity_re="^https://github\\.com/rahulnakmol/agentic-ayudha/\\.github/workflows/release\\.yml@refs/tags/${tag_re}\$"
if [ -n "${AAYU_CERT_IDENTITY_REGEXP:-}" ]; then
  identity_re="$AAYU_CERT_IDENTITY_REGEXP"
  say "warning: AAYU_CERT_IDENTITY_REGEXP is set; accepting signatures from $identity_re (testing only)"
fi

ext=tar.gz
[ "$os" = windows ] && ext=zip
archive="aayudha_${version#v}_${os}_${arch}.${ext}"

say "downloading $archive ($version)"
fetch checksums.txt "$tmp/checksums.txt"
fetch checksums.txt.sigstore.json "$tmp/checksums.txt.sigstore.json"
fetch "$archive" "$tmp/$archive"

cosign="$(command -v cosign || true)"
if [ -z "$cosign" ]; then
  case "$os-$arch" in
    linux-amd64) want=4629c757b7618056f8ddd7e2625ae9fdd94c0372a65049520bc7d9df9efc7f71 ;;
    linux-arm64) want=c5d324e091826b0d7a78eb16fef316450b4eb9aaec045611c08ba06f5e73220a ;;
    darwin-amd64) want=2347488e5d5b25336644024dfeca5601b190e91197a71a917bda44744aff106c ;;
    darwin-arm64) want=5cf948c2f4dfe59687bdd0b8523709067383e03982cc543475c8a7dc70e92a76 ;;
    windows-amd64) want=9fe59be0eca1271873ce019061335eb1ac419b7059202e797828467ddabe33be ;;
    *) die "cosign is required; install it from https://docs.sigstore.dev/cosign/system_config/installation/" ;;
  esac
  name="cosign-$os-$arch"
  [ "$os" = windows ] && name="$name.exe"
  say "cosign not found; downloading pinned cosign $COSIGN_VERSION"
  curl -fsSL --proto '=https' --tlsv1.2 -o "$tmp/cosign" \
    "https://github.com/sigstore/cosign/releases/download/$COSIGN_VERSION/$name" || die "cosign download failed"
  [ "$(sha256 "$tmp/cosign")" = "$want" ] || die "pinned cosign checksum mismatch; refusing to run it"
  chmod +x "$tmp/cosign"
  cosign="$tmp/cosign"
fi

say "verifying the signature on checksums.txt"
"$cosign" verify-blob \
  --bundle "$tmp/checksums.txt.sigstore.json" \
  --certificate-identity-regexp "$identity_re" \
  --certificate-oidc-issuer "$ISSUER" \
  "$tmp/checksums.txt" >/dev/null 2>"$tmp/cosign.log" || {
  cat "$tmp/cosign.log" >&2
  die "signature verification failed: checksums.txt was not signed by $REPO release.yml for $version"
}

want="$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")"
[ -n "$want" ] || die "$archive is not listed in the signed checksums.txt"
[ "$(sha256 "$tmp/$archive")" = "$want" ] || die "checksum mismatch for $archive: the archive was tampered with or corrupted"

say "signature and checksum verified"
mkdir -p "$tmp/x"
if [ "$ext" = zip ]; then unzip -q "$tmp/$archive" -d "$tmp/x"; else tar -xzf "$tmp/$archive" -C "$tmp/x"; fi

dir="${AAYU_INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$dir"
if [ "$os" = windows ]; then
  cp "$tmp/x/aayudha.exe" "$dir/aayudha.exe"
  cp "$tmp/x/aayudha.exe" "$dir/aayu.exe"
else
  cp "$tmp/x/aayudha" "$dir/aayudha.tmp"
  chmod 755 "$dir/aayudha.tmp"
  mv -f "$dir/aayudha.tmp" "$dir/aayudha"
  ln -sf aayudha "$dir/aayu"
fi
say "installed $("$dir/aayudha" version) to $dir (alias: aayu)"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) say "add $dir to your PATH" ;;
esac
