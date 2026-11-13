#!/usr/bin/env bash
# Builds an .rpm with rpmbuild (Ubuntu's "rpm" package). Nothing is downloaded.
#   packaging/linux/build-rpm.sh VERSION ARCH BINARY OUTDIR
# ARCH is amd64 or arm64. VERSION like 0.1.0 or 0.0.0-dev.1a2b3c4.
set -euo pipefail
version="$1" arch="$2" binary="$3" out="$4"
here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../.." && pwd)"
case "$arch" in
  amd64) rpmarch=x86_64 ;;
  arm64) rpmarch=aarch64 ;;
  *) echo "unknown arch $arch" >&2; exit 1 ;;
esac
# RPM versions can't contain "-": 0.0.0-dev.x becomes version 0.0.0~dev.x.
rpmver="${version//-/\~}"

top="$(mktemp -d)"
trap 'rm -rf "$top"' EXIT
rpmbuild -bb --target "$rpmarch" \
  --define "_topdir $top" \
  --define "atlas_version $rpmver" \
  --define "atlas_release 1" \
  --define "atlas_arch $rpmarch" \
  --define "atlas_binary $(realpath "$binary")" \
  --define "atlas_src $root" \
  "$here/atlas-lifecycle.spec"
mkdir -p "$out"
cp "$top"/RPMS/"$rpmarch"/*.rpm "$out/"
