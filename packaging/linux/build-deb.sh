#!/usr/bin/env bash
# Builds a .deb with Debian's own dpkg-deb. Nothing is downloaded.
#   packaging/linux/build-deb.sh VERSION ARCH BINARY OUTDIR
# ARCH is amd64 or arm64. VERSION like 0.1.0 or 0.0.0-dev.1a2b3c4.
set -euo pipefail
version="$1" arch="$2" binary="$3" out="$4"
here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../.." && pwd)"
debver="${version//-/\~}" # 0.0.0-dev.x sorts before 0.0.0 in Debian as 0.0.0~dev.x

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
chmod 0755 "$stage"
install -D -m 0755 "$binary" "$stage/usr/bin/atlas"
install -D -m 0644 "$root/README.md" "$stage/usr/share/doc/atlas-lifecycle/README.md"
install -D -m 0644 "$root/LICENSE" "$stage/usr/share/doc/atlas-lifecycle/copyright"
mkdir -p "$stage/DEBIAN"
sed -e "s/@VERSION@/$debver/" -e "s/@ARCH@/$arch/" \
    -e "s/@SIZE@/$(du -sk --exclude=DEBIAN "$stage" | cut -f1)/" \
    "$here/deb-control" > "$stage/DEBIAN/control"

mkdir -p "$out"
dpkg-deb --root-owner-group -Zxz --build "$stage" "$out/atlas-lifecycle_${debver}_${arch}.deb"
