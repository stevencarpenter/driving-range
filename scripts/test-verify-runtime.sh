#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' 0
mkdir "$work/with spaces"
cat > "$work/with spaces/go" <<'FAKE_GO'
#!/bin/sh
printf '%s|%s|%s\n' "$GOLF_INTEGRATION" "$GOLF_TEST_IMAGE" "$*" >> "$CALLS"
exit "${FAKE_EXIT:-0}"
FAKE_GO
cat > "$work/golf" <<'FAKE_GOLF'
#!/bin/sh
printf '%s\n' "$*" >> "$CALLS"
exit "${FAKE_EXIT:-0}"
FAKE_GOLF
chmod +x "$work/with spaces/go" "$work/golf"
cd "$work"
export GO="$work/with spaces/go" CALLS="$work/calls"
export GOLF_IMAGE=preferred GOLF_TEST_IMAGE=fallback
sh "$root/scripts/verify-runtime.sh" all integration > /dev/null
curriculum='^TestIntegration(CurriculumRejectsWrongAnswers|NewCurriculumRejectsHardcodedOutput|NativeGitCurriculumStates|NativeJJCurriculumStates)$'
{
    printf '1|preferred|test -v -count=1 -timeout=20m -skip %s ./...\n' "$curriculum"
    for selector in vim search shell awk 'sed|fd|find|python|zsh|fzf|git|jj'; do
        printf '1|preferred|test -v -count=1 -timeout=20m -run %s/^(%s)[.] ./internal/runner\n' "$curriculum" "$selector"
    done
} > "$work/expected"
cmp "$CALLS" "$work/expected"
: > "$CALLS"
sh "$root/scripts/verify-runtime.sh" all references > /dev/null
cat > "$work/expected" <<'EXPECTED'
audit --solutions --track vim
audit --solutions --track search
audit --solutions --track shell
audit --solutions --track awk
audit --solutions --track sed
audit --solutions --track fd
audit --solutions --track find
audit --solutions --track python
audit --solutions --track zsh
audit --solutions --track fzf
audit --solutions --track git
audit --solutions --track jj
EXPECTED
cmp "$CALLS" "$work/expected"
: > "$CALLS"
unset GOLF_IMAGE
sh "$root/scripts/verify-runtime.sh" base integration > /dev/null
printf '1|fallback|test -v -count=1 -timeout=20m -skip %s ./...\n' "$curriculum" > "$work/expected"
cmp "$CALLS" "$work/expected"
: > "$CALLS"
if sh "$root/scripts/verify-runtime.sh" unknown integration > /dev/null 2>&1; then exit 1; fi
if sh "$root/scripts/verify-runtime.sh" base unknown > /dev/null 2>&1; then exit 1; fi
test ! -s "$CALLS"
export FAKE_EXIT=9
if sh "$root/scripts/verify-runtime.sh" all integration > /dev/null; then exit 1; else test "$?" -eq 9; fi
test "$(wc -l < "$CALLS")" -eq 1
: > "$CALLS"
if sh "$root/scripts/verify-runtime.sh" all references > /dev/null; then exit 1; else test "$?" -eq 9; fi
test "$(wc -l < "$CALLS")" -eq 1
printf 'PASS: all six integration shards, all twelve reference tracks, image precedence, quoted GO path, rejected options, and fail-fast propagation\n'
