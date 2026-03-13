#!/bin/sh
set -e

REPO="tab/fuku"

detect_os() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$os" in
    darwin) echo "macos" ;;
    linux)  echo "linux" ;;
    *)
      echo "Unsupported OS: $os" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64)  echo "x86_64" ;;
    arm64|aarch64) echo "arm64" ;;
    armv*)         echo "arm" ;;
    *)
      echo "Unsupported architecture: $arch" >&2
      exit 1
      ;;
  esac
}

fetch_latest_tag() {
  if command -v curl > /dev/null 2>&1; then
    curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"//;s/".*//'
  elif command -v wget > /dev/null 2>&1; then
    wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"//;s/".*//'
  else
    echo "curl or wget is required" >&2
    exit 1
  fi
}

download() {
  url=$1
  output=$2
  if command -v curl > /dev/null 2>&1; then
    curl -sL "$url" -o "$output"
  else
    wget -qO "$output" "$url"
  fi
}

main() {
  os=$(detect_os)
  arch=$(detect_arch)
  tag=$(fetch_latest_tag)

  if [ -z "$tag" ]; then
    echo "Failed to fetch latest release" >&2
    exit 1
  fi

  archive="fuku_${tag}_${os}_${arch}.tar.gz"
  url="https://github.com/${REPO}/releases/download/${tag}/${archive}"

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  echo "Downloading fuku ${tag} for ${os}/${arch}..."
  download "$url" "${tmpdir}/${archive}"

  tar xzf "${tmpdir}/${archive}" -C "$tmpdir"

  if [ -d "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
    install_dir="$HOME/.local/bin"
    mv "${tmpdir}/fuku" "${install_dir}/fuku"
    chmod +x "${install_dir}/fuku"
  else
    install_dir="/usr/local/bin"
    echo "Installing to ${install_dir} (requires sudo)..."
    sudo mv "${tmpdir}/fuku" "${install_dir}/fuku"
    sudo chmod +x "${install_dir}/fuku"
  fi

  echo "Installed fuku ${tag} to ${install_dir}/fuku"

  case ":$PATH:" in
    *":${install_dir}:"*) ;;
    *)
      echo ""
      echo "Add ${install_dir} to your PATH:"
      echo "  export PATH=\"${install_dir}:\$PATH\""
      ;;
  esac
}

main
