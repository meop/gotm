#!/usr/bin/env sh
set -e

GOTM_REPO='meop/gotm'
GOTM_BIN="$HOME/.gotm/bin"

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

RELEASE_JSON=''

fetch_release() {
  url="https://api.github.com/repos/$1/releases/latest"
  echo "  GET $url"
  RELEASE_JSON=$(curl -fsSL "$url") || {
    echo "  failed to fetch release from $1" >&2
    exit 1
  }
}

release_tag() {
  tag=$(printf '%s' "$RELEASE_JSON" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  if [ -z "$tag" ]; then
    echo "  could not parse tag_name from response:" >&2
    printf '%.500s\n' "$RELEASE_JSON" >&2
    exit 1
  fi
  printf '%s' "$tag"
}

release_asset_url() {
  url=$(printf '%s' "$RELEASE_JSON" | grep '"browser_download_url"' | grep "/$1\"" | head -1 | sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/')
  if [ -z "$url" ]; then
    echo "  could not find asset matching '$1'" >&2
    echo "  available assets:" >&2
    printf '%s' "$RELEASE_JSON" | grep '"browser_download_url"' | sed 's/.*"browser_download_url": *"\([^"]*\)".*/    \1/' >&2
    exit 1
  fi
  printf '%s' "$url"
}

install_from_release() {
  pattern="$1" binary="$2" dest="$3"
  tmp=$(mktemp -d)
  url=$(release_asset_url "$pattern")
  pkg="$tmp/pkg"
  echo "  downloading $url"
  echo "  temp dir: $tmp"
  curl -fsSL "$url" -o "$pkg" || {
    echo "  download failed: $url" >&2
    echo "  partial file: $pkg ($(wc -c < "$pkg" 2>/dev/null | tr -d ' ') bytes)" >&2
    rm -rf "$tmp"
    exit 1
  }
  echo "  downloaded $(wc -c < "$pkg" | tr -d ' ') bytes to $pkg"
  case "$url" in
    *.tar.gz|*.tgz)
      if ! tar xzf "$pkg" -C "$tmp" 2>&1; then
        echo "  tar extraction failed for $pkg" >&2
        echo "  file type: $(file "$pkg")" >&2
        echo "  file size: $(wc -c < "$pkg" | tr -d ' ') bytes" >&2
        echo "  first bytes (hex): $(od -A x -t x1z -N 16 "$pkg" | head -1)" >&2
        rm -rf "$tmp"
        exit 1
      fi
      ;;
    *.zip)
      if ! unzip -q "$pkg" -d "$tmp" 2>&1; then
        echo "  unzip failed for $pkg" >&2
        echo "  file type: $(file "$pkg")" >&2
        echo "  file size: $(wc -c < "$pkg" | tr -d ' ') bytes" >&2
        rm -rf "$tmp"
        exit 1
      fi
      ;;
  esac
  found=$(find "$tmp" -name "$binary" -type f | head -1)
  if [ -z "$found" ]; then
    echo "  binary '$binary' not found in archive" >&2
    echo "  archive contents:" >&2
    find "$tmp" -type f | sed 's|^|    |' >&2
    rm -rf "$tmp"
    exit 1
  fi
  mkdir -p "$dest"
  mv "$found" "$dest/$binary"
  chmod +x "$dest/$binary"
  echo "  installed $dest/$binary"
  rm -rf "$tmp"
}

# Install gotm
echo "Fetching latest gotm release: github.com/$GOTM_REPO"
fetch_release "$GOTM_REPO"
GOTM_TAG=$(release_tag)
echo "  version: $GOTM_TAG"
install_from_release "gotm-.*-${OS}-${ARCH_GO}.tar.gz" 'gotm' "$GOTM_BIN"

echo ''
echo 'Refer to the project README for how to activate gotm in your shell.'
