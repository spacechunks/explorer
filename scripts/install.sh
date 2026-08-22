#!/bin/sh

set -eu

repository_url="https://github.com/spacechunks/explorer"
binary_name="explorer"
temp_dir=""
install_temp=""

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	escape="$(printf '\033')"
	bold="${escape}[1m"
	dim="${escape}[2m"
	green="${escape}[32m"
	purple="${escape}[35m"
	red="${escape}[31m"
	reset="${escape}[0m"
else
	bold=""
	dim=""
	green=""
	purple=""
	red=""
	reset=""
fi

cleanup() {
	if [ -n "$install_temp" ] && [ -f "$install_temp" ]; then
		rm -f "$install_temp"
	fi
	if [ -n "$temp_dir" ] && [ -d "$temp_dir" ]; then
		rm -rf "$temp_dir"
	fi
}

fail() {
	printf '\n  %sError:%s %s\n\n' "$red" "$reset" "$*" >&2
	exit 1
}

step() {
	printf '  %s✓%s %s\n' "$green" "$reset" "$1"
}

download() {
	curl --fail --silent --show-error --location --retry 3 \
		--connect-timeout 10 "$1" --output "$2"
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{ print $1 }'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{ print $1 }'
	elif command -v openssl >/dev/null 2>&1; then
		openssl dgst -sha256 "$1" | awk '{ print $NF }'
	else
		return 1
	fi
}

trap cleanup EXIT
trap 'exit 1' HUP INT TERM

command -v curl >/dev/null 2>&1 || fail "curl is required to install the Explorer CLI."
command -v tar >/dev/null 2>&1 || fail "tar is required to install the Explorer CLI."

kernel="$(uname -s)"
machine="$(uname -m)"

case "$kernel" in
	Darwin)
		operating_system="darwin"
		operating_system_label="macOS"
		binary_extension=""
		;;
	Linux)
		operating_system="linux"
		operating_system_label="Linux"
		binary_extension=""
		;;
	MINGW* | MSYS* | CYGWIN*)
		operating_system="windows"
		operating_system_label="Windows"
		binary_extension=".exe"
		;;
	*)
		fail "Unsupported operating system: $kernel"
		;;
esac

case "$machine" in
	x86_64 | amd64 | AMD64)
		architecture="amd64"
		;;
	aarch64 | arm64 | ARM64)
		architecture="arm64"
		;;
	*)
		fail "Unsupported architecture: $machine"
		;;
esac

version="${EXPLORER_VERSION:-}"
if [ -z "$version" ]; then
	latest_url="$(curl --fail --silent --show-error --location --retry 3 \
		--connect-timeout 10 --output /dev/null --write-out '%{url_effective}' \
		"$repository_url/releases/latest")" || fail "Could not determine the latest Explorer CLI version."
	version="${latest_url##*/}"
	version="$(printf '%s' "$version" | sed 's/%2[Bb]/+/g')"
fi

case "$version" in
	v*) ;;
	*) version="v$version" ;;
esac

case "$version" in
	*[!A-Za-z0-9._+-]*) fail "Invalid Explorer CLI version: $version" ;;
esac

if [ -n "${EXPLORER_INSTALL_DIR:-}" ]; then
	install_dir="${EXPLORER_INSTALL_DIR%/}"
elif [ -n "${XDG_BIN_HOME:-}" ]; then
	install_dir="${XDG_BIN_HOME%/}"
elif [ -n "${HOME:-}" ]; then
	install_dir="$HOME/.local/bin"
else
	fail "HOME is not set. Set EXPLORER_INSTALL_DIR to choose an installation directory."
fi

[ -n "$install_dir" ] || fail "The installation directory cannot be empty."

archive_name="explorer_${version}_${operating_system}_${architecture}.tar.gz"
checksum_name="explorer_${version}_sha256sums"
release_binary="explorer_${version}_${operating_system}_${architecture}${binary_extension}"
release_url="$repository_url/releases/download/$version"

temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/explorer-install.XXXXXX")" || \
	fail "Could not create a temporary directory."
archive_path="$temp_dir/$archive_name"
checksum_path="$temp_dir/$checksum_name"

printf '\n%s✦%s %sChunk Explorer%s\n\n' "$purple" "$reset" "$bold" "$reset"
printf '  %sInstalling %s for %s %s%s\n\n' \
	"$dim" "$version" "$operating_system_label" "$architecture" "$reset"

download "$release_url/$archive_name" "$archive_path" || \
	fail "Could not download $archive_name. Check that the release supports $operating_system/$architecture."
download "$release_url/$checksum_name" "$checksum_path" || \
	fail "Could not download the checksum manifest for $version."
step "Downloaded $archive_name"

expected_checksum="$(awk -v name="$archive_name" '$2 == name { print $1; exit }' "$checksum_path")"
[ -n "$expected_checksum" ] || fail "The checksum manifest does not contain $archive_name."
actual_checksum="$(sha256 "$archive_path")" || \
	fail "A SHA-256 tool is required. Install sha256sum, shasum, or openssl."

[ "$actual_checksum" = "$expected_checksum" ] || \
	fail "Checksum verification failed for $archive_name."
step "Verified the SHA-256 checksum"

tar -xzf "$archive_path" -C "$temp_dir" || fail "Could not extract $archive_name."
source_binary="$temp_dir/$release_binary"
[ -f "$source_binary" ] || fail "The release archive does not contain $release_binary."

mkdir -p "$install_dir" || fail "Could not create $install_dir."
installed_binary="$install_dir/$binary_name$binary_extension"
install_temp="$install_dir/.$binary_name.tmp.$$"
cp "$source_binary" "$install_temp" || fail "Could not write to $install_dir."
chmod 0755 "$install_temp" || fail "Could not make the Explorer CLI executable."
mv -f "$install_temp" "$installed_binary" || fail "Could not install the Explorer CLI to $installed_binary."
install_temp=""
step "Installed $binary_name to $installed_binary"

printf '\n%sExplorer CLI is ready.%s\n' "$bold" "$reset"
case ":${PATH:-}:" in
	*":$install_dir:"*)
		printf 'Run %sexplorer --help%s to get started.\n\n' "$bold" "$reset"
		;;
	*)
		printf 'Add the installation directory to your PATH:\n\n'
		printf '  %sexport PATH="%s:$PATH"%s\n\n' "$dim" "$install_dir" "$reset"
		;;
esac
