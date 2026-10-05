#!/bin/sh
set -eu
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' 0
message_file="$scratch/commit message"

check() {
    printf '%s' "$2" > "$message_file"
    cp "$message_file" "$scratch/original"
    status=0
    sh scripts/check-commit-msg.sh "$message_file" > "$scratch/output" 2>&1 || status=$?
    if [ "$status" -ne "$1" ]; then
        printf 'Expected exit %s, got %s for: %s\n' "$1" "$status" "$2" >&2
        cat "$scratch/output" >&2
        exit 1
    fi
    cmp "$message_file" "$scratch/original"
}

check 0 'fix: restore terminal'
check 0 'feat(cli): add track picker'
check 0 'feat!: change command syntax'
check 0 'refactor(cli)!: remove legacy flags'
check 0 'no-mistakes(review): document installation'
check 0 'FIX: tune hook timeout'
check 0 'docs: explain commit hooks

Explain the local setup.

BREAKING CHANGE: remove the old setup command'
check 1 'restore terminal'
check 1 'fix:restore terminal'
check 1 'fix: '
check 1 'fix:    '
check 1 'fix(): restore terminal'
check 1 'fix( ): restore terminal'
check 1 'fix!(cli): restore terminal'
check 1 'fix(cli)!!: restore terminal'
check 1 'fix:	restore terminal'
check 1 'restore terminal

fix: valid body cannot repair the title'
check 1 ''

status=0
sh scripts/check-commit-msg.sh "$scratch/missing" > "$scratch/output" 2>&1 || status=$?
[ "$status" -eq 2 ]
status=0
sh scripts/check-commit-msg.sh > "$scratch/output" 2>&1 || status=$?
[ "$status" -eq 2 ]
printf 'PASS: Conventional Commit titles, breaking markers, message preservation, invalid input\n'
