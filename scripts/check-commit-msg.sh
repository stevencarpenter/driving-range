#!/bin/sh
set -eu

if [ "$#" -ne 1 ] || [ ! -r "$1" ]; then
    printf 'Usage: sh scripts/check-commit-msg.sh COMMIT_MESSAGE_FILE\n' >&2
    exit 2
fi

if LC_ALL=C awk '
    NR == 1 { valid = $0 ~ /^[[:alpha:]][[:alnum:]-]*(\([^()[:space:]]+\))?!?: [^[:space:]].*$/ }
    END { exit !valid }
' < "$1"; then
    exit 0
fi

printf '%s\n' 'Commit title must use Conventional Commits: type(scope)!: description' \
    'Scope and ! are optional. Examples: fix: restore terminal, feat(cli): add track picker' >&2
exit 1
