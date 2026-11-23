#!/bin/sh
# Installs the latest agy-switch release.
#   curl -fsSL https://raw.githubusercontent.com/renatormc/agy-switcher/master/install.sh | sh
# Environment: INSTALL_DIR (default ~/.local/bin), VERSION (e.g. v0.3.0; default latest).
set -eu

REPO="renatormc/agy-switcher"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-latest}"

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "agy-switch: unsupported OS $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "agy-switch: unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

asset="agy-switch_${os}_${arch}"
if [ "$VERSION" = latest ]; then
  url="https://github.com/${REPO}/releases/latest/download/${asset}"
else
  url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
fi

mkdir -p "$INSTALL_DIR"
tmp="$(mktemp "${INSTALL_DIR}/.agy-switch.XXXXXX")"
trap 'rm -f "$tmp"' EXIT

echo "Downloading ${url}"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$url" -o "$tmp"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$tmp" "$url"
else
  echo "agy-switch: curl or wget is required" >&2
  exit 1
fi
chmod 755 "$tmp"
mv -f "$tmp" "${INSTALL_DIR}/agy-switch"
trap - EXIT

echo "Installed $("${INSTALL_DIR}/agy-switch" --version) to ${INSTALL_DIR}/agy-switch"
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *) echo "Note: ${INSTALL_DIR} is not on your PATH. Add it, e.g.:"
     echo "  echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.profile" ;;
esac
