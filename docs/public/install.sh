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
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"//;s/".*//'
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
    curl -fsSL "$url" -o "$output"
  else
    wget -qO "$output" "$url"
  fi
}

verify_checksum() {
  archive_path=$1
  archive_name=$2
  checksums_path=$3

  expected=$(grep "  ${archive_name}$" "$checksums_path" | cut -d' ' -f1)
  if [ -z "$expected" ]; then
    echo "No checksum found for ${archive_name}" >&2
    exit 1
  fi

  if command -v sha256sum > /dev/null 2>&1; then
    actual=$(sha256sum "$archive_path" | cut -d' ' -f1)
  elif command -v shasum > /dev/null 2>&1; then
    actual=$(shasum -a 256 "$archive_path" | cut -d' ' -f1)
  else
    echo "Warning: sha256sum/shasum not found, skipping checksum verification" >&2
    return
  fi

  if [ "$actual" != "$expected" ]; then
    echo "Checksum verification failed" >&2
    echo "  expected: ${expected}" >&2
    echo "  actual:   ${actual}" >&2
    exit 1
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

  version="${tag#v}"
  archive="fuku_${tag}_${os}_${arch}.tar.gz"
  checksums="fuku_${version}_checksums.txt"
  base_url="https://github.com/${REPO}/releases/download/${tag}"

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  echo "Downloading fuku ${tag} for ${os}/${arch}..."
  download "${base_url}/${archive}" "${tmpdir}/${archive}"
  download "${base_url}/${checksums}" "${tmpdir}/${checksums}"

  verify_checksum "${tmpdir}/${archive}" "${archive}" "${tmpdir}/${checksums}"

  tar xzf "${tmpdir}/${archive}" -C "$tmpdir"

  if [ -d "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
    install_dir="$HOME/.local/bin"
    mv "${tmpdir}/fuku" "${install_dir}/fuku"
    chmod +x "${install_dir}/fuku"
  else
    install_dir="/usr/local/bin"
    echo "Installing to ${install_dir} (requires sudo)..."
    sudo mkdir -p "${install_dir}"
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
