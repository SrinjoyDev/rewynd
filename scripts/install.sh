#!/bin/sh
# Install the rewynd CLI from the latest GitHub release.
#   curl -fsSL https://raw.githubusercontent.com/SrinjoyDev/rewynd/main/scripts/install.sh | sh
set -e

REPO="SrinjoyDev/rewynd"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "rewynd: unsupported architecture: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux | darwin) ;;
  *) echo "rewynd: unsupported OS: $os (Windows: download from the releases page)" >&2; exit 1 ;;
esac

tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep -m1 '"tag_name"' | cut -d'"' -f4)
if [ -z "$tag" ]; then echo "rewynd: could not find the latest release" >&2; exit 1; fi
ver=${tag#v}
url="https://github.com/$REPO/releases/download/${tag}/rewynd_${ver}_${os}_${arch}.tar.gz"

echo "rewynd: downloading ${tag} (${os}/${arch})…"
tmp=$(mktemp -d)
curl -fsSL "$url" | tar -xz -C "$tmp"

dest="${REWYND_INSTALL_DIR:-/usr/local/bin}"
if install -m 0755 "$tmp/rewynd" "$dest/rewynd" 2>/dev/null; then :; else
  dest="$HOME/.local/bin"
  mkdir -p "$dest"
  install -m 0755 "$tmp/rewynd" "$dest/rewynd"
fi
rm -rf "$tmp"

echo "rewynd: installed to $dest/rewynd"
case ":$PATH:" in *":$dest:"*) ;; *) echo "rewynd: add $dest to your PATH" ;; esac
