GO := env("GO", "go")
VERSION := env("VERSION", "dev")

build:
    "{{ GO }}" build -trimpath -ldflags '-X main.version={{ VERSION }}' -o golf ./cmd/golf

test:
    "{{ GO }}" test ./...

vet:
    "{{ GO }}" vet ./...

audit:
    "{{ GO }}" run ./cmd/golf audit

check: test vet audit

setup: build
    ./golf setup

integration:
    GOLF_INTEGRATION=1 GOLF_TEST_IMAGE="${GOLF_IMAGE:-${GOLF_TEST_IMAGE:-}}" "{{ GO }}" test -count=1 -timeout=10m ./...

audit-solutions: build
    ./golf audit --solutions

smoke: build
    python3 scripts/smoke.py

release:
    GO="{{ GO }}" ./scripts/release.sh "{{ VERSION }}"
