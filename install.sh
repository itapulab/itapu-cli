#!/bin/sh
# Itapu CLI installer.
#
#   curl -fsSL https://raw.githubusercontent.com/itapulab/itapu-cli/main/install.sh | sh
#
# Downloads the latest release binary for this platform and installs it to
# ~/.local/bin (override with ITAPU_INSTALL_DIR). No sudo required.
set -eu

REPO="itapulab/itapu-cli"
INSTALL_DIR="${ITAPU_INSTALL_DIR:-$HOME/.local/bin}"

err() { printf 'install: %s\n' "$1" >&2; exit 1; }

case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux)  os="linux" ;;
  *) err "unsupported OS: $(uname -s). Download a binary from https://github.com/$REPO/releases" ;;
esac

case "$(uname -m)" in
  x86_64|amd64)  arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) err "unsupported architecture: $(uname -m)" ;;
esac

command -v curl >/dev/null 2>&1 || err "curl is required"
command -v tar  >/dev/null 2>&1 || err "tar is required"

printf 'Fetching the latest release...\n'
# Resolve the tag from the /releases/latest redirect — unlike the GitHub
# API, this is not rate-limited for anonymous clients.
tag=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
  "https://github.com/$REPO/releases/latest") || tag=""
tag=${tag##*/}
case "$tag" in
  v[0-9]*) ;;
  *) err "could not determine the latest release" ;;
esac
version=${tag#v}

url="https://github.com/$REPO/releases/download/$tag/itapu_${version}_${os}_${arch}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

printf 'Downloading itapu %s (%s/%s)...\n' "$tag" "$os" "$arch"
curl -fsSL "$url" -o "$tmp/itapu.tar.gz"
tar -xzf "$tmp/itapu.tar.gz" -C "$tmp" itapu

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/itapu" "$INSTALL_DIR/itapu"

printf '\n✔ Installed itapu %s to %s/itapu\n' "$tag" "$INSTALL_DIR"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    printf '\n%s is not on your PATH. Add this to your shell profile:\n' "$INSTALL_DIR"
    printf '\n    export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    ;;
esac

printf '\nGet started:\n\n    itapu login\n    itapu init\n    itapu run -- <command>\n'
