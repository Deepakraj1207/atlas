#!/usr/bin/env bash
# Builds every release file into OUTDIR. Uploads and publishes nothing.
#   packaging/build-release.sh VERSION OUTDIR
# VERSION like 0.1.0 (a release) or 0.0.0-dev.1a2b3c4 (a dry run).
# Needs Go, zip, dpkg-deb, and rpmbuild for the .rpm files (skipped with a
# note when it's missing).
set -euo pipefail
version="$1" out="$2"
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
export GOTOOLCHAIN="${GOTOOLCHAIN:-local}" CGO_ENABLED=0

commit="$(git rev-parse HEAD)"
# The commit's own date, so the same commit always builds the same files.
date="$(git show -s --format=%cs HEAD)"
ldflags="-s -w -X main.version=$version -X main.commit=$commit -X main.date=$date"

rm -rf "$out"
mkdir -p "$out"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

for target in windows/amd64 windows/arm64 linux/amd64 linux/arm64; do
  os="${target%/*}" arch="${target#*/}"
  name="atlas_${version}_${os}_${arch}"
  dir="$work/$name"
  mkdir -p "$dir"
  exe="atlas"; [ "$os" = windows ] && exe="atlas.exe"
  echo "building $name"
  GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "$ldflags" -o "$dir/$exe" ./cmd/atlas
  cp LICENSE README.md "$dir/"
  if [ "$os" = windows ]; then
    # atlas.exe at the top of the zip: the winget manifest points there.
    (cd "$dir" && zip -q -X "$root/$out/$name.zip" "$exe" LICENSE README.md)
  else
    tar -C "$work" --owner=0 --group=0 --sort=name --mtime="$date" -czf "$out/$name.tar.gz" "$name"
    packaging/linux/build-deb.sh "$version" "$arch" "$dir/$exe" "$out" >/dev/null
    if command -v rpmbuild >/dev/null; then
      packaging/linux/build-rpm.sh "$version" "$arch" "$dir/$exe" "$out" >/dev/null 2>&1
    else
      echo "  note: rpmbuild isn't installed, so no .rpm for $arch"
    fi
  fi
done

go run ./tools/release winget -version "$version" -date "$date" \
  -x64 "$out/atlas_${version}_windows_amd64.zip" \
  -arm64 "$out/atlas_${version}_windows_arm64.zip" \
  -out "$out/winget"

(cd "$out" && sha256sum -- *.zip *.tar.gz *.deb $(ls *.rpm 2>/dev/null) > SHA256SUMS)
echo
echo "Release files in $out:"
ls -1 "$out"
