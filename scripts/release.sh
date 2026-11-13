#!/usr/bin/env bash
set -euo pipefail

version=${1:?usage: release.sh vVERSION OUTPUT_DIR}
output=${2:?usage: release.sh vVERSION OUTPUT_DIR}
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'Expected a stable version tag, got %s\n' "$version" >&2
  exit 1
fi

scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
go build -trimpath -o "$scratch/version-check" ./cmd/rem
actual=$("$scratch/version-check" --version)
if [[ "$version" != "v$actual" ]]; then
  printf 'Tag %s does not match CLI version %s\n' "$version" "$actual" >&2
  exit 1
fi
mkdir -p "$output"
output=$(cd "$output" && pwd)
assets=()

for platform in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64 windows/amd64; do
  os=${platform%/*}
  arch=${platform#*/}
  name="rem-from-funnelscript-${version}-${os}-${arch}"
  staging="$scratch/$name"
  mkdir "$staging"
  binary=rem
  if [[ "$os" == windows ]]; then binary=rem.exe; fi
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "$staging/$binary" ./cmd/rem
  cp LICENSE README.md "$staging/"
  if [[ "$os" == windows ]]; then
    archive="$name.zip"
    (cd "$scratch" && zip -qr "$scratch/$archive" "$name")
    unzip -tq "$scratch/$archive"
  else
    archive="$name.tar.gz"
    tar -czf "$scratch/$archive" -C "$scratch" "$name"
    tar -tzf "$scratch/$archive" > /dev/null
  fi
  mv -f "$scratch/$archive" "$output/$archive"
  assets+=("$archive")
done

cd "$output"
if command -v sha256sum > /dev/null; then
  sha256sum "${assets[@]}" > SHA256SUMS
  sha256sum -c SHA256SUMS
else
  shasum -a 256 "${assets[@]}" > SHA256SUMS
  shasum -a 256 -c SHA256SUMS
fi
