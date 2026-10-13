#!/usr/bin/env sh
set -e

GOTM_REPO='meop/gotm'
GOTM_BIN="$HOME/.gotm/bin"
RELEASES="https://github.com/$GOTM_REPO/releases"

ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')

case "$ARCH" in
  arm64|aarch64) ARCH_GO='arm64' ;;
  amd64|x86_64)  ARCH_GO='amd64' ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  darwin|linux) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# every request is a plain page or release download: no GitHub API, so no rate
# limit and no token
fetch() {
  curl --fail-with-body --location --no-progress-meter "$@"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

# the newest release is where the release page's "latest" address redirects to
echo "Fetching latest gotm release: github.com/$GOTM_REPO"
TAG=$(fetch --output /dev/null --write-out '%{url_effective}' --url "$RELEASES/latest")
TAG=${TAG##*/}
case "$TAG" in
  v*) ;;
  *) echo "  could not read the latest release from $RELEASES/latest" >&2; exit 1 ;;
esac
echo "  version: $TAG"

ASSET="gotm-${TAG#v}-${OS}-${ARCH_GO}.tar.gz"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "  downloading $RELEASES/download/$TAG/$ASSET"
fetch --output "$TMP/$ASSET" --url "$RELEASES/download/$TAG/$ASSET"
fetch --output "$TMP/SHA256SUMS" --url "$RELEASES/download/$TAG/SHA256SUMS"

# the archive is checked against the release's own checksum manifest before
# anything in it is used
WANT=$(grep " $ASSET\$" "$TMP/SHA256SUMS" | cut -d' ' -f1)
GOT=$(sha256_of "$TMP/$ASSET")
if [ -z "$WANT" ] || [ "$WANT" != "$GOT" ]; then
  echo "  $ASSET does not match its checksum" >&2
  echo "    want: ${WANT:-not listed in SHA256SUMS}" >&2
  echo "    got:  $GOT" >&2
  exit 1
fi

tar -xzf "$TMP/$ASSET" -C "$TMP"
if [ ! -f "$TMP/gotm" ]; then
  echo "  gotm not found in $ASSET" >&2
  exit 1
fi
mkdir -p "$GOTM_BIN"
mv "$TMP/gotm" "$GOTM_BIN/gotm"
chmod +x "$GOTM_BIN/gotm"
echo "  installed $GOTM_BIN/gotm"

echo ''
echo 'Refer to the project README for how to activate gotm in your shell.'
