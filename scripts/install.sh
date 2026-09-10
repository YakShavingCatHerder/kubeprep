#!/usr/bin/env bash
# Install the KubePrep release binary for this machine.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/YakShavingCatHerder/kubeprep/main/scripts/install.sh | bash
#   KUBEPREP_VERSION=v0.1.1 bash scripts/install.sh
#   PREFIX=$HOME/.local/bin bash scripts/install.sh --no-sudo
#
# Requires bash (3.2+), curl, tar, and sha256sum (Linux) or shasum (macOS).
# Login shell may be zsh; still invoke this file with bash, never zsh.
# Archives are kubeprep_<version>_<os>_<arch>.tar.gz with lowercase os
# (darwin|linux) and Go arch (amd64|arm64), matching GoReleaser.

set -euo pipefail

REPO="${KUBEPREP_REPO:-YakShavingCatHerder/kubeprep}"
PREFIX="${PREFIX:-}"
USE_SUDO="${KUBEPREP_USE_SUDO:-auto}"
VERSION="${KUBEPREP_VERSION:-}"

usage() {
  cat <<'EOF'
Install the KubePrep release binary for this OS and architecture.

Options:
  --help, -h     Show this help
  --no-sudo      Never use sudo; install to PREFIX or ~/.local/bin
  --self-test    Check uname → archive mapping; do not download

Environment:
  KUBEPREP_VERSION  Release tag or version (default: latest)
  PREFIX            Install directory (default: /usr/local/bin, or ~/.local/bin)
  KUBEPREP_REPO     GitHub owner/name (default: YakShavingCatHerder/kubeprep)
EOF
}

normalize_os() {
  case "$1" in
    Darwin | darwin) printf '%s\n' darwin ;;
    Linux | linux) printf '%s\n' linux ;;
    *)
      printf 'KubePrep has no prebuilt binary for OS %s. Use macOS or Linux.\n' "$1" >&2
      return 1
      ;;
  esac
}

normalize_arch() {
  case "$1" in
    x86_64 | amd64) printf '%s\n' amd64 ;;
    aarch64 | arm64) printf '%s\n' arm64 ;;
    *)
      printf 'KubePrep has no prebuilt binary for architecture %s.\n' "$1" >&2
      return 1
      ;;
  esac
}

archive_name() {
  # version os arch → kubeprep_0.1.1_darwin_arm64.tar.gz (no leading v)
  local version="$1" os="$2" arch="$3"
  version="${version#v}"
  printf 'kubeprep_%s_%s_%s.tar.gz\n' "$version" "$os" "$arch"
}

self_test() {
  local failed=0
  check() {
    local got want
    got="$1"
    want="$2"
    if [[ "$got" != "$want" ]]; then
      printf 'self-test failed: got %s, want %s\n' "$got" "$want" >&2
      failed=1
    fi
  }
  check "$(normalize_os Darwin)" darwin
  check "$(normalize_os Linux)" linux
  check "$(normalize_arch x86_64)" amd64
  check "$(normalize_arch amd64)" amd64
  check "$(normalize_arch aarch64)" arm64
  check "$(normalize_arch arm64)" arm64
  check "$(archive_name v0.1.1 darwin arm64)" kubeprep_0.1.1_darwin_arm64.tar.gz
  check "$(archive_name 0.1.1 linux amd64)" kubeprep_0.1.1_linux_amd64.tar.gz
  if [[ "$failed" -ne 0 ]]; then
    return 1
  fi
  printf 'install.sh self-test ok\n'
}

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Need %s on PATH to install KubePrep.\n' "$1" >&2
    return 1
  fi
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    printf 'Need sha256sum or shasum to verify the download.\n' >&2
    return 1
  fi
}

fetch() {
  # curl --fail so HTTP errors abort under set -e / pipefail
  curl --proto '=https' --tlsv1.2 --fail --show-error --location --silent "$@"
}

latest_tag() {
  local url
  url="$(fetch -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest")"
  url="${url%$'\r'}"
  printf '%s\n' "${url##*/}"
}

resolve_prefix() {
  if [[ -n "$PREFIX" ]]; then
    printf '%s\n' "$PREFIX"
    return 0
  fi
  if [[ "$USE_SUDO" == "never" ]]; then
    printf '%s\n' "${HOME}/.local/bin"
    return 0
  fi
  if [[ -d /usr/local/bin ]] && [[ -w /usr/local/bin ]]; then
    printf '%s\n' /usr/local/bin
    return 0
  fi
  if [[ "$USE_SUDO" == "auto" ]] && command -v sudo >/dev/null 2>&1 && [[ -d /usr/local/bin ]]; then
    printf '%s\n' /usr/local/bin
    return 0
  fi
  printf '%s\n' "${HOME}/.local/bin"
}

install_path() {
  local src="$1" dest_dir="$2" dest
  dest="${dest_dir}/kubeprep"
  mkdir -p "$dest_dir"
  if [[ -w "$dest_dir" ]]; then
    install -m 755 "$src" "$dest"
    printf '%s\n' "$dest"
    return 0
  fi
  if [[ "$USE_SUDO" != "never" ]] && command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$dest_dir"
    sudo install -m 755 "$src" "$dest"
    printf '%s\n' "$dest"
    return 0
  fi
  printf 'Cannot write %s. Re-run with PREFIX=$HOME/.local/bin or --no-sudo.\n' "$dest_dir" >&2
  return 1
}

path_contains() {
  local dir="$1"
  case ":${PATH}:" in
    *":${dir}:"*) return 0 ;;
    *) return 1 ;;
  esac
}

main() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --help | -h)
        usage
        return 0
        ;;
      --no-sudo)
        USE_SUDO="never"
        ;;
      --self-test)
        self_test
        return 0
        ;;
      *)
        printf 'Unknown option %s\n' "$1" >&2
        usage >&2
        return 1
        ;;
    esac
    shift
  done

  need_cmd curl
  need_cmd tar
  need_cmd uname
  need_cmd mktemp
  need_cmd install
  need_cmd awk

  local os arch tag version archive dest_dir expected actual dest
  os="$(normalize_os "$(uname -s)")"
  arch="$(normalize_arch "$(uname -m)")"

  if [[ -z "$VERSION" ]]; then
    tag="$(latest_tag)"
  else
    tag="$VERSION"
  fi
  if [[ "$tag" != v* ]]; then
    tag="v${tag}"
  fi
  if [[ -z "$tag" || "$tag" == "latest" || "$tag" == "v" ]]; then
    printf 'Could not resolve the latest KubePrep release from GitHub.\n' >&2
    return 1
  fi
  version="${tag#v}"
  archive="$(archive_name "$version" "$os" "$arch")"
  dest_dir="$(resolve_prefix)"

  # Temp path must be global: bash EXIT traps cannot see `local` vars (set -u).
  _KUBEPREP_TMP="$(mktemp -d "${TMPDIR:-/tmp}/kubeprep-install.XXXXXX")"
  cleanup() {
    if [[ -n "${_KUBEPREP_TMP:-}" ]]; then
      rm -rf "$_KUBEPREP_TMP"
    fi
  }
  trap cleanup EXIT

  printf 'Downloading %s (%s)\n' "$archive" "$tag"
  fetch -o "${_KUBEPREP_TMP}/${archive}" "https://github.com/${REPO}/releases/download/${tag}/${archive}"
  fetch -o "${_KUBEPREP_TMP}/checksums.txt" "https://github.com/${REPO}/releases/download/${tag}/checksums.txt"

  expected="$(awk -v name="$archive" '$2 == name || $2 == "*"name { print $1; found=1 } END { if (!found) exit 1 }' "${_KUBEPREP_TMP}/checksums.txt")"
  actual="$(sha256_file "${_KUBEPREP_TMP}/${archive}")"
  if [[ "$expected" != "$actual" ]]; then
    printf 'Checksum mismatch for %s\n  expected %s\n  got      %s\n' "$archive" "$expected" "$actual" >&2
    return 1
  fi
  printf 'Checksum ok\n'

  # Extract into the temp dir so LICENSE/README do not clobber the cwd.
  tar -xzf "${_KUBEPREP_TMP}/${archive}" -C "$_KUBEPREP_TMP"
  if [[ ! -f "${_KUBEPREP_TMP}/kubeprep" ]]; then
    printf 'Archive %s did not contain a kubeprep binary.\n' "$archive" >&2
    return 1
  fi

  dest="$(install_path "${_KUBEPREP_TMP}/kubeprep" "$dest_dir")"
  printf 'Installed %s\n' "$dest"

  if ! path_contains "$dest_dir"; then
    printf 'add %s to PATH:\n' "$dest_dir"
    printf '  export PATH="%s:$PATH"\n' "$dest_dir"
  fi
  printf 'Next: kubeprep --version && kubeprep doctor && kubeprep start --track=beginner\n'
}

main "$@"
