#!/bin/sh
# Install ATLAS on Linux:
#
#   curl -fsSL https://raw.githubusercontent.com/Deepakraj1207/atlas/main/install.sh | sh
#
# It downloads the latest release for this computer from GitHub, checks it
# against the release's SHA256SUMS, and puts the `atlas` command in
# ~/.local/bin. No sudo, nothing outside that folder is changed. Run it again
# to update.
#
# Options (environment variables):
#   ATLAS_VERSION=0.2.0      install that version instead of the latest
#   ATLAS_INSTALL_DIR=DIR    install into DIR instead of ~/.local/bin
#   ATLAS_UNINSTALL=1        remove the atlas command this script installed
#
# Everything is inside main(), so nothing runs if the download is cut short.

set -eu

REPO="Deepakraj1207/atlas"

say() { printf '%s\n' "$*"; }
fail() {
	printf 'atlas install: %s\n' "$*" >&2
	exit 1
}
have() { command -v "$1" >/dev/null 2>&1; }

# fetch URL FILE downloads with curl or wget.
fetch() {
	if have curl; then
		curl -fsSL --retry 3 -o "$2" "$1"
	elif have wget; then
		wget -q -O "$2" "$1"
	else
		fail "needs curl or wget to download ATLAS"
	fi
}

latest_version() {
	api="https://api.github.com/repos/$REPO/releases/latest"
	if have curl; then
		json=$(curl -fsSL --retry 3 "$api") || return 1
	else
		json=$(wget -q -O - "$api") || return 1
	fi
	printf '%s\n' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v\{0,1\}\([^"]*\)".*/\1/p' | head -n 1
}

sha256_of() {
	if have sha256sum; then
		sha256sum "$1" | cut -d ' ' -f 1
	elif have shasum; then
		shasum -a 256 "$1" | cut -d ' ' -f 1
	else
		fail "needs sha256sum or shasum to check the download"
	fi
}

main() {
	dir="${ATLAS_INSTALL_DIR:-$HOME/.local/bin}"

	if [ "${ATLAS_UNINSTALL:-}" = "1" ]; then
		if [ -f "$dir/atlas" ]; then
			rm -f "$dir/atlas"
			say "Removed $dir/atlas."
		else
			say "Nothing to remove: $dir/atlas isn't there."
		fi
		say "Your settings, saved data and logs are kept (~/.config/atlas, ~/.local/share/atlas, ~/.local/state/atlas); delete those folders too if you want them gone."
		return 0
	fi

	case "$(uname -s)" in
	Linux) ;;
	MINGW* | MSYS* | CYGWIN*)
		fail "this is the Linux installer. On Windows, run this in PowerShell instead:
  irm https://raw.githubusercontent.com/$REPO/main/install.ps1 | iex" ;;
	*) fail "ATLAS runs on Linux and Windows; this computer runs $(uname -s)." ;;
	esac

	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "there is no ATLAS build for this processor ($(uname -m)); builds exist for x86-64 and ARM64." ;;
	esac

	version="${ATLAS_VERSION:-}"
	version="${version#v}"
	if [ -z "$version" ]; then
		version=$(latest_version) || true
		[ -n "$version" ] || fail "couldn't find the latest release on GitHub. Check your internet connection, or set ATLAS_VERSION."
	fi

	name="atlas_${version}_linux_${arch}"
	base="${ATLAS_DOWNLOAD_BASE:-https://github.com/$REPO/releases/download/v$version}"
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT INT TERM

	say "Downloading ATLAS $version for Linux ($arch)…"
	fetch "$base/$name.tar.gz" "$tmp/$name.tar.gz" || fail "couldn't download $base/$name.tar.gz"
	fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS" || fail "couldn't download the checksums ($base/SHA256SUMS)"

	want=$(awk -v f="$name.tar.gz" '$2 == f || $2 == "*" f { print $1 }' "$tmp/SHA256SUMS")
	[ -n "$want" ] || fail "SHA256SUMS has no entry for $name.tar.gz"
	got=$(sha256_of "$tmp/$name.tar.gz")
	[ "$got" = "$want" ] || fail "the download doesn't match its checksum, so it wasn't installed (expected $want, got $got)"
	say "Checksum OK."

	tar -xzf "$tmp/$name.tar.gz" -C "$tmp" || fail "couldn't unpack the download"
	[ -f "$tmp/$name/atlas" ] || fail "the download doesn't contain the atlas program"

	mkdir -p "$dir" || fail "couldn't create $dir"
	# Copy next to the target, then rename: a running atlas keeps working.
	cp "$tmp/$name/atlas" "$dir/.atlas.new" && chmod 755 "$dir/.atlas.new" && mv -f "$dir/.atlas.new" "$dir/atlas" ||
		fail "couldn't write $dir/atlas"

	say "Installed: $("$dir/atlas" -version 2>/dev/null || echo "$dir/atlas")"
	say "Location:  $dir/atlas"

	case ":$PATH:" in
	*":$dir:"*)
		found=$(command -v atlas || true)
		if [ -n "$found" ] && [ "$found" != "$dir/atlas" ]; then
			say ""
			say "Note: another atlas comes first on your PATH ($found)."
			say "If that's the atlas-lifecycle package, remove it, or run $dir/atlas directly."
		fi
		say ""
		say "Start ATLAS by typing:  atlas"
		;;
	*)
		say ""
		say "$dir isn't on your PATH yet. Add it by running this once, then open a new terminal:"
		say "  echo 'export PATH=\"$dir:\$PATH\"' >> ~/.profile"
		say "Until then, start ATLAS with:  $dir/atlas"
		;;
	esac
	say "To update later, run the same install command again."
}

main "$@"
