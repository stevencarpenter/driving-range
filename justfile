GO := env("GO", "go")
VERSION := env("VERSION", "dev")

build:
    "{{ GO }}" build -trimpath -ldflags '-X main.version={{ VERSION }}' -o golf ./cmd/golf

# Build and install the current checkout, replacing any previously installed version.
install: build
    #!/bin/sh
    set -eu
    install_dir="${GOLF_INSTALL_DIR:-$HOME/.local/bin}"
    mkdir -p "$install_dir"
    install_dir=$(cd "$install_dir" && pwd -P)
    if [ -d "$install_dir/golf" ]; then
        printf 'Cannot replace directory: %s/golf\n' "$install_dir" >&2
        exit 1
    fi
    staged=$(mktemp "$install_dir/.golf.XXXXXX")
    trap 'rm -f "$staged"' 0
    install -m 755 ./golf "$staged"
    mv -f "$staged" "$install_dir/golf"
    printf 'Installed %s\n' "$install_dir/golf"
    "$install_dir/golf" --version
    resolved=$(command -v golf || :)
    if ! [ "$resolved" -ef "$install_dir/golf" ]; then
        printf 'PATH resolves golf to: %s\nPut %s first on PATH to use this installation.\n' "${resolved:-not found}" "$install_dir"
    fi

test:
    "{{ GO }}" test -count=1 ./...

test-install:
    sh scripts/test-install.sh

vet:
    "{{ GO }}" vet ./...

audit:
    "{{ GO }}" run ./cmd/golf audit

check: test vet audit test-install

setup: build
    ./golf setup

integration shard="all":
    GO="{{ GO }}" sh scripts/verify-runtime.sh "{{ shard }}" integration

audit-solutions shard="all": build
    sh scripts/verify-runtime.sh "{{ shard }}" references

smoke: build
    python3 scripts/smoke.py

release:
    GO="{{ GO }}" ./scripts/release.sh "{{ VERSION }}"
