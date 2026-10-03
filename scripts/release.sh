#!/bin/sh
set -eu

version=${1:-dev}
case "$version" in
    ''|[!A-Za-z0-9]*|*[!A-Za-z0-9._-]*) printf 'Version must start with a letter or digit and contain only letters, digits, dots, underscores, or hyphens.\n' >&2; exit 2 ;;
esac
project_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"
output=${RELEASE_DIR:-"$project_dir/dist/$version"}
if [ -e "$output" ]; then
    printf 'Release output already exists: %s\n' "$output" >&2
    exit 2
fi
mkdir -p "$output"
output=$(CDPATH='' cd -- "$output" && pwd)
stage=$(mktemp -d "${TMPDIR:-/tmp}/golf-release.XXXXXX")
trap 'rm -rf "$stage"' EXIT HUP INT TERM

for target in darwin/arm64 linux/amd64 linux/arm64; do
    os=${target%/*}
    arch=${target#*/}
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" "${GO:-go}" build \
        -trimpath -ldflags "-s -w -X main.version=$version" -o "$stage/golf" ./cmd/golf
    cp README.md CONTRIBUTING.md SECURITY.md LICENSE CONTENT_LICENSE.md THIRD_PARTY_NOTICES.md "$stage/"
    mkdir -p "$stage/docs"
    cp docs/AUTHORING.md docs/CURRICULUM.md docs/SOURCES.md docs/LAUNCH_AUDIT.md "$stage/docs/"
    COPYFILE_DISABLE=1 tar --no-xattrs -czf "$output/golf_${version}_${os}_${arch}.tar.gz" -C "$stage" golf README.md CONTRIBUTING.md SECURITY.md LICENSE CONTENT_LICENSE.md THIRD_PARTY_NOTICES.md docs
done

cd "$output"
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.tar.gz > SHA256SUMS
    sha256sum -c SHA256SUMS
else
    shasum -a 256 ./*.tar.gz > SHA256SUMS
    shasum -a 256 -c SHA256SUMS
fi
printf 'Release archives and checksums: %s\n' "$output"
