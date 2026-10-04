#!/bin/sh
# Keep every growing curriculum group within its own 20-minute invocation.
set -eu
set -f
shard=${1:-all}
mode=${2:-integration}
case "$mode" in
    integration|references) ;;
    *) printf 'Unknown verification mode: %s\n' "$mode" >&2; exit 2 ;;
esac
case "$shard" in
    all)
        for group in base vim search shell awk ancillary; do
            sh "$0" "$group" "$mode"
        done
        exit 0
        ;;
    base) tracks='' ;;
    vim|search|shell|awk) tracks=$shard ;;
    ancillary) tracks='sed|fd|find|python|zsh|fzf|git|jj' ;;
    *) printf 'Unknown runtime shard: %s\n' "$shard" >&2; exit 2 ;;
esac
printf '\n=== %s: %s ===\n' "$mode" "$shard"
export GOLF_INTEGRATION=1
export GOLF_TEST_IMAGE="${GOLF_IMAGE:-${GOLF_TEST_IMAGE:-}}"
curriculum='^TestIntegration(CurriculumRejectsWrongAnswers|NewCurriculumRejectsHardcodedOutput|NativeGitCurriculumStates|NativeJJCurriculumStates)$'
if [ "$mode" = references ]; then
    for track in $(printf '%s\n' "$tracks" | tr '|' ' '); do
        ./golf audit --solutions --track "$track"
    done
elif [ "$shard" = base ]; then
    "${GO:-go}" test -v -count=1 -timeout=20m -skip "$curriculum" ./...
else
    "${GO:-go}" test -v -count=1 -timeout=20m -run "$curriculum/^($tracks)[.]" ./internal/runner
fi
