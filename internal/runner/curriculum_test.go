package runner

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/stevencarpenter/driving-range/internal/catalog"
)

func TestIntegrationCurriculumRejectsWrongAnswers(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ id, script string }{
		{"vim.practice-forward-search-release", "sed 's/pending/ready/g' notes.txt > changed; mv changed notes.txt\n"},
		{"vim.practice-backward-search-release", "sed 's/pending/ready/g' notes.txt > changed; mv changed notes.txt\n"},
		{"vim.practice-next-match-release", "sed '0,/pending/s/pending/ready/' notes.txt > changed; mv changed notes.txt\n"},
		{"vim.practice-star-word-release", "sed 's/pending_key/ready/' notes.txt > changed; mv changed notes.txt\n"},
		{"vim.practice-word-substitute-release", "sed 's/pending/ready/g' notes.txt > changed; mv changed notes.txt\n"},
		{"vim.practice-range-substitute-release", "sed 's/pending/ready/g' notes.txt > changed; mv changed notes.txt\n"},
		{"vim.practice-first-per-line-release", "sed 's/pending/ready/g' notes.txt > changed; mv changed notes.txt\n"},
		{"vim.practice-global-delete-release", "sed '/REMOVE/d' notes.txt > changed; mv changed notes.txt\n"},
		{"vim.practice-capture-reorder-release", "sed -E 's/^([a-z]+): ([0-9]+)$/\\2-\\1/' notes.txt > changed; mv changed notes.txt\n"},
		{"search.practice-record-shape-release", "rg --no-filename --color=never 'JOB [A-Z]{2}-[0-9]{3}' input.txt\n"},
		{"search.practice-extract-ids-release", "rg --no-filename --color=never -o 'id=[0-9]+' input.txt | cut -d= -f2 | awk '{print $1 + 0}'\n"},
		{"search.practice-quoted-spans-release", "rg --no-filename --color=never -o '\".*\"' input.txt\n"},
		{"search.practice-severity-alternation-release", "rg --no-filename --color=never '^WARN|ERROR: ' input.txt\n"},
		{"search.practice-bounded-fields-release", "rg --no-filename --color=never -x '[a-z]+=[0-9]+' input.txt\n"},
		{"search.practice-repeated-word-release", "rg --no-filename --color=never -P -o '\\b[a-z]+[ \\t]+[a-z]+\\b' input.txt\n"},
		{"search.practice-literal-dot-release", "rg --no-filename --color=never -x '[a-z]+.log' input.txt\n"},
		{"search.practice-whole-token-release", "rg --no-filename --color=never -o 'pending|ready' input.txt\n"},
		{"search.practice-context-digits-release", "rg --no-filename --color=never -P -o '(?<=\\bkey=)[0-9]+' input.txt\n"},
		{"shell.practice-head-lines-release", "tail -n 2 input.txt\n"},
		{"shell.practice-tail-lines-release", "head -n 2 input.txt\n"},
		{"shell.practice-cut-tabs-release", "awk '{print $2}' input.tsv\n"},
		{"shell.practice-paste-columns-release", "cat left.txt right.txt\n"},
		{"shell.practice-numeric-sort-release", "sort input.txt\n"},
		{"shell.practice-adjacent-uniq-release", "sort -u input.txt\n"},
		{"shell.practice-tr-ascii-release", "tr 'a-z' 'A-Z' < input.txt | tr -d '0-9'\n"},
		{"shell.practice-comm-sets-release", "LC_ALL=C comm -12 left.txt right.txt\n"},
		{"shell.practice-join-keys-release", "paste left.txt right.txt\n"},
		{"shell.practice-wc-words-release", "wc -l < input.txt | awk '{print $1}'\n"},
		{"shell.practice-tee-copy-release", "printf '%s\\n' release 'pending value' ready > copy.txt\n"},
		{"awk.practice-second-field-release", "awk '{print $1}' input.txt\n"},
		{"awk.practice-colon-field-release", "awk '{print $2}' input.txt\n"},
		{"awk.practice-record-number-release", "awk 'NF {print NR \":\" $0}' input.txt\n"},
		{"awk.practice-field-count-release", "awk '{print NR}' input.txt\n"},
		{"awk.practice-numeric-threshold-release", "awk '$2 > 10 {print $0}' input.txt\n"},
		{"awk.practice-integer-sum-release", "awk '{total += $2; print total}' input.txt\n"},
		{"awk.practice-group-counts-release", "awk 'END {print NR}' input.txt\n"},
		{"awk.practice-group-sums-release", "awk '{total += $2} END {print total}' input.txt\n"},
		{"awk.practice-stable-lines-release", "awk '!seen[$1]++' input.txt\n"},
		{"awk.practice-min-max-release", "awk 'BEGIN {minimum = maximum = 0} {if ($1 < minimum) minimum = $1; if ($1 > maximum) maximum = $1} END {print minimum, maximum}' input.txt\n"},
		{"awk.practice-cumulative-release", "awk '{total += $2} END {print total}' input.txt\n"},
		{"awk.practice-output-separator-release", "awk '{print $2, $1}' input.txt\n"},
		{"awk.practice-inclusive-range-release", "awk '/^BEGIN$/ {inside=1; next} /^END$/ {inside=0; next} inside' input.txt\n"},
		{"awk.practice-file-number-release", "awk '{print FILENAME \":\" NR \":\" $0}' first.txt second.txt\n"},
		{"search.count-occurrences", "rg --no-filename --count --include-zero -F 'event=click' events.txt; code=$?; if (( code > 1 )); then exit \"$code\"; fi\n"},
		{"search.unicode-letter-runs", "rg --no-filename -o '[A-Za-z]+' words.txt\n"},
		{"search.ascii-digit-runs", "rg --no-filename -o '\\d+' numbers.txt\n"},
		{"search.crlf-records", "tr -d '\\r' < states.txt | rg -x READY\n"},
		{"search.match-byte-offsets", "python3 - <<'PY'\nimport re\ntext=open('records.txt',encoding='utf-8').read()\nfor match in re.finditer('ID=[0-9]+',text): print(f'{match.start()}:{match.group()}')\nPY\n"},
		{"search.lookaround-digits", "rg --no-filename -o 'id=[0-9]+' messages.txt | cut -d= -f2\n"},
		{"search.negative-lookahead-word", "rg '^deploy ' commands.txt | rg -v legacy\n"},
		{"search.hex-color-records", "rg --no-filename '#[0-9A-Fa-f]{6}' colors.txt\n"},
		{"search.bounded-angle-spans", "rg --no-filename -o '<.*>' fragments.txt\n"},
		{"search.multiline-blocks", "awk '/^BEGIN$/,/^END$/' blocks.txt\n"},
		{"search.hidden-respects-ignore", "rg --hidden --no-ignore -l -F needle data | LC_ALL=C sort\n"},
		{"search.hidden-respects-ignore", "rg -l -F needle data | LC_ALL=C sort\n"},
		{"search.same-record-conjunction", "rg -F -e red -e blue events.txt\n"},
		{"search.whole-file-conjunction", "rg -l 'red.*blue|blue.*red' logs | LC_ALL=C sort\n"},
		{"search.repeated-word-backreference", "rg -o '\\b[a-z]+[ \\t]+[a-z]+\\b' words.txt\n"},
		{"search.named-capture-report", "rg -o --replace='${value}|${key}' '(?P<key>[A-Z]+)=(?P<value>[0-9]+)' pairs.txt\n"},
		{"search.escaped-quoted-fields", "rg -o '\"[^\"\\r\\n]*\"' fields.txt\n"},
		{"search.option-like-pattern", "IFS= read -r pattern < pattern.txt\nrg -F \"$pattern\" labels.txt\n"},
		{"search.binary-as-text", "rg -o 'token=[0-9]+' payload.bin\n"},
		{"search.utf16-bom-log", "rg -a --encoding=none '^ERROR ' utf16.log\n"},
		{"search.gzip-log", "rg -a '^ERROR ' events.gz\n"},
		{"search.punctuation-word-match", "rg '\\b-2\\b' tokens.txt\n"},
		{"search.per-file-zero-counts", "rg --with-filename --count -F needle counts | LC_ALL=C sort\n"},
		{"search.smart-case-query", "IFS= read -r query < query.txt\nrg -F -- \"$query\" messages.txt\n"},
		{"search.smart-case-query", "IFS= read -r query < query.txt\nrg -i -F -- \"$query\" messages.txt\n"},
		{"search.merged-context", "rg -n '^ERROR ' events.txt\n"},
		{"search.per-file-match-limit", "rg --with-filename '^ERROR ' logs/a.log logs/b.log | head -n 2\n"},
		{"search.first-matching-run", "rg '^ERROR ' events.txt\n"},
		{"search.first-match-byte-column", "python3 - <<'PY'\nfrom pathlib import Path\nfor n,row in enumerate(Path('positions.txt').read_text().splitlines(),1):\n    if 'OK' in row: print(f\"{n}:{row.index('OK')+1}:{row}\")\nPY\n"},
		{"search.nul-delimited-records", "rg -a '^ERROR ' records.bin\n"},
		{"search.nul-delimited-records", "rg --null-data -x 'ERROR [^\\x00]*' records.bin\n"},
		{"search.grapheme-clusters", "rg -P -o '.' text.txt\n"},
		{"search.nested-parenthesized-spans", "rg -P -o '\\(.*?\\)' groups.txt\n"},
		{"search.tokens-outside-quotes", "rg -P -o '\\bTODO\\b' notes.txt\n"},
		{"search.depth-limited-content", "rg -l -F READY data | LC_ALL=C sort\n"},
		{"search.ascii-word-boundaries", "rg '\\bOK\\b' boundaries.txt\n"},
		{"git.selective-stage", "git add -- api.txt; printf 'corrupted notes\\n' > notes.txt\n"},
		{"git.selective-stage", "git commit --amend -qm 'Changed history'; git add -- api.txt\n"},
		{"git.selective-stage", "git commit --amend --author='Changed Author <changed@example.invalid>' -qm 'Initial files'; git add -- api.txt\n"},
		{"git.amend-message", "printf 'extra\\n' > extra.txt; git add -- extra.txt; git commit --amend -qm 'Document launch checklist'\n"},
		{"git.stage-deletion", "git add .\n"},
		{"git.restore-index-version", "git restore --source=HEAD -- api.txt\n"},
		{"git.restore-historical-path", "git restore --source=HEAD~1 --staged --worktree -- api.txt\n"},
		{"git.untrack-and-ignore", "printf 'cache.bin\\n' > .gitignore\ngit add .gitignore\n"},
		{"git.stage-tracked-rename", "mv api.txt service.txt\n"},
		{"git.commit-staged-snapshot", "git commit -am 'Publish API'\n"},
		{"git.amend-staged-content", "git commit -m Update\n"},
		{"git.amend-staged-content", "git commit -a --amend --no-edit\n"},
		{"git.undo-commit-keep-index", "git reset --mixed HEAD~1\n"},
		{"git.undo-commit-keep-work", "git reset --soft HEAD~1\n"},
		{"git.revert-local-tip", "git reset --hard HEAD~1\n"},
		{"git.create-branch-without-switch", "git switch -c scratch HEAD~1\n"},
		{"git.switch-existing-branch", "git reset --hard review\n"},
		{"git.rename-current-branch", "git branch release\n"},
		{"git.delete-merged-branch", "git branch -d scratch keep\n"},
		{"git.move-unchecked-branch", "git switch topic\ngit reset --hard main\n"},
		{"git.detach-at-parent", "git reset --hard HEAD~1\n"},
		{"git.attach-detached-work", "git switch main\n"},
		{"git.tag-old-revision", "git tag v0\n"},
		{"git.annotated-tag", "git tag v1\n"},
		{"git.delete-tag-only", "git tag -d v0 keep\n"},
		{"git.restore-both-destinations", "git restore -- api.txt\n"},
		{"git.intent-to-add", "git add -- draft.tmp\n"},
		{"git.intent-to-add", "printf '' > draft.tmp\ngit add -- draft.tmp\nprintf 'private\\n' > draft.tmp\n"},
		{"git.intent-to-add", "git add -N .\n"},
		{"git.cherry-pick-local", "git switch topic\n"},
		{"git.abort-cherry-pick", "git cherry-pick --quit\n"},
		{"git.resolve-two-parent-merge", "git checkout --ours -- api.txt\ngit add api.txt\ngit commit -m 'Merge topic'\n"},
		{"git.resolve-two-parent-merge", "git merge --abort\nprintf 'combined api\\n' > api.txt\ngit add api.txt\ngit commit -m 'Merge topic'\n"},
		{"git.abort-merge", "git merge --quit\n"},
		{"git.fast-forward-only", "git merge --no-ff -m 'Merge topic' topic\n"},
		{"git.explicit-merge-commit", "git merge --ff-only topic\n"},
		{"git.squash-merge-no-commit", "git merge --no-commit topic\n"},
		{"git.stash-apply-preserve", "git stash apply\n"},
		{"git.stash-apply-preserve", "git stash pop --index\n"},
		{"git.stash-path-only", "git stash push -m API\n"},
		{"fzf.or-with-required-term", "fzf +i --literal --filter=\"'red | 'blue\" < labels.txt | LC_ALL=C sort\n"},
		{"fzf.suffix-padding", "fzf +i --literal --filter=\"'.go\" < labels.txt | LC_ALL=C sort\n"},
		{"fzf.whole-command", "fzf +i --literal --filter=\"'make 'test\" < commands.txt | LC_ALL=C sort\n"},
		{"fzf.fuzzy-input-order", "fzf +i --literal --filter=abc < labels.txt\n"},
		{"fzf.nul-framed-records", "fzf +i --literal --no-sort --read0 --filter=\"'keep\" < candidates.bin\n"},
		{"fzf.filter-status", "IFS= read -r scope < scope.txt; if fzf +i --literal --exact --delimiter='[|]' --nth=\"$scope\" --filter=keep < rows.txt >/dev/null 2>/dev/null; then printf 'matched\\n'; else printf 'empty\\n'; fi\n"},
		{"zsh.inclusive-array-range", "items=(); while IFS= read -r item; do items+=(\"$item\"); done < items.txt; IFS='|' read -r first last < range.txt; first=$((first-1)); for item in \"${(@)items[$first,$last]}\"; do print -r -- \"[$item]\"; done\n"},
		{"zsh.reverse-array-order", "items=(); while IFS= read -r item; do items+=(\"$item\"); done < items.txt; print -rl -- \"${(@O)items}\"\n"},
		{"zsh.dynamic-local-scope", "leaf() { print -r -- \"leaf=[$label]\"; }; inner() { IFS= read -r label < inner.txt; print -r -- \"inner=[$label]\"; leaf; }; IFS= read -r label < outer.txt; print -r -- \"before=[$label]\"; inner; print -r -- \"after=[$label]\"\n"},
		{"zsh.function-local-options", "IFS= read -r mode < mode.txt; if [[ $mode == on ]]; then setopt extendedglob; else unsetopt extendedglob; fi; show() { if [[ -o extendedglob ]]; then print -r -- \"$1=on\"; else print -r -- \"$1=off\"; fi; }; worker() { if [[ $mode == on ]]; then unsetopt extendedglob; else setopt extendedglob; fi; show inside; }; show before; worker; show after\n"},
		{"zsh.pipeline-status-vector", "zsh producer.zsh | zsh consumer.zsh; print -r -- \"producer=${pipestatus[1]}\"; print -r -- \"consumer=${pipestatus[2]}\"\n"},
		{"zsh.autoload-function", "source helpers/dr-greet\n"},
		{"zsh.largest-file-glob", "export LC_ALL=C; print -rl -- inbox/*(.on[1,2])\n"},
		{"zsh.recursive-hidden-glob", "export LC_ALL=C; setopt extendedglob; files=(tree/**/(#i)*.txt(.N)); if (( ${#files} )); then print -rl -- \"${files[@]}\" | sort; fi\n"},
		{"zsh.natural-array-sort", "export LC_ALL=C; labels=(); while IFS= read -r label; do labels+=(\"$label\"); done < labels.txt; print -rl -- ${(o)labels}\n"},
		{"zsh.unique-array-order", "LC_ALL=C sort -u labels.txt\n"},
		{"zsh.associative-presence", "typeset -A settings; while IFS='|' read -r key value; do settings[$key]=$value; done < settings.txt; while IFS= read -r key; do if [[ -n ${settings[$key]} ]]; then print -r -- \"$key=${settings[$key]}\"; else print -r -- \"$key=MISSING\"; fi; done < requests.txt\n"},
		{"zsh.explicit-word-split", "IFS= read -r line < words.txt; words=($line); print -r -- \"count=${#words}\"; for word in \"${words[@]}\"; do print -r -- \"[$word]\"; done\n"},
		{"zsh.delimited-empty-fields", "IFS= read -r line < record.txt; fields=(${(s:|:)line}); print -r -- \"count=${#fields}\"; for field in \"${fields[@]}\"; do print -r -- \"[$field]\"; done\n"},
		{"zsh.join-array", "while IFS= read -r item; do printf '%s|' \"$item\"; done < items.txt; print\n"},
		{"zsh.dynamic-pattern", "IFS= read -r pattern < pattern.txt; while IFS= read -r label; do if [[ $label == \"$pattern\" ]]; then print -r -- \"$label\"; fi; done < labels.txt\n"},
		{"zsh.pattern-captures", "export LC_ALL=C; setopt extendedglob; while IFS= read -r code; do if [[ $code == (#b)([a-z]##)(<->) ]]; then print -r -- \"${match[1]}|$((10#${match[2]}))\"; else print -r -- invalid; fi; done < codes.txt\n"},
		{"shell.read-unterminated", "while IFS= read -r line; do printf '<%s>\\n' \"$line\"; done < names.txt\n"},
		{"awk.stable-deduplicate", "LC_ALL=C sort -u events.txt\n"},
		{"awk.latest-by-key", "awk '!seen[$1]++ {print $1 \"=\" $2}' settings.txt | LC_ALL=C sort\n"},
		{"find.empty-files", "find inbox -empty -print | LC_ALL=C sort\n"},
		{"find.byte-size", "find payloads -type f -size +3c -print | LC_ALL=C sort\n"},
		{"fzf.prefix-query", "fzf +i --filter='^git' < commands.txt | LC_ALL=C sort\n"},
		{"git.unstage-preserve", "git restore --staged -- api.txt; git restore -- api.txt\n"},
		{"git.restore-one", "git restore -- .\n"},
		{"git.unstage-preserve", "git restore --staged -- api.txt; printf new > api.txt\n"},
		{"git.unstage-preserve", "git restore --staged -- api.txt; printf notes > notes.txt\n"},
		{"git.restore-one", "git restore -- notes.txt; printf 'new api' > api.txt\n"},
		{"git.restore-one", "git restore -- notes.txt; printf 'original notes' > notes.txt\n"},
		{"jj.delete-bookmark", "jj bookmark delete scratch; jj describe -m Wrong\n"},
		{"jj.edit-parent", "jj new base\n"},
		{"jj.new-parallel-change", "jj new -m Experiment\n"},
		{"jj.restore-parent-path", "jj restore --from @-\n"},
		{"jj.restore-historical-path", "jj restore --from @- api.txt\n"},
		{"jj.squash-one-path", "jj squash --use-destination-message\n"},
		{"jj.split-paths", "jj split -m Api notes.txt\n"},
		{"jj.bookmark-advance", "jj bookmark move --from @- --to @\n"},
		{"jj.bookmark-backwards", "if jj bookmark move release --to @-; then exit 1; else test \"$?\" -eq 1; fi\n"},
		{"jj.rename-bookmark", "jj bookmark create release\n"},
		{"jj.undo-description-operation", "jj abandon @\n"},
		{"vim.inner-word-setting", "printf '%s\\n' 'mode = ready' > drill.txt\n"},
		{"vim.inner-big-word-url", "printf '%s\\n' 'visit https://new.example/a today' > drill.txt\n"},
		{"vim.delete-inner-word-gap", "printf '%s\\n' 'keep keep' > drill.txt\n"},
		{"vim.delete-around-word-end", "printf '%s\\n' 'keep ' > drill.txt\n"},
		{"vim.change-quotes-message", "printf '%s\\n' 'message=Ready; enabled=true' > drill.txt\n"},
		{"vim.empty-quotes-assignment", "printf '%s\\n' 'message=; enabled=true' > drill.txt\n"},
		{"vim.change-parens-nested", "printf '%s\\n' 'outer(value);' > drill.txt\n"},
		{"vim.change-braces-nested", "printf '%s\\n' 'outer={ready:true};' > drill.txt\n"},
		{"vim.change-tag-paragraph", "printf '%s\\n' '<p>Ready</p>' > drill.txt\n"},
		{"vim.change-brackets-nested", "printf '%s\\n' 'grid=[ready];' > drill.txt\n"},
		{"vim.change-till-comma-field", "printf '%s\\n' 'readykeep' > drill.txt\n"},
		{"vim.delete-through-semicolon-prefix", "printf '%s\\n' '; keep' > drill.txt\n"},
		{"vim.change-line-tail-setting", "printf '%s\\n' 'status=ready # temporary' '# keep' > drill.txt\n"},
		{"vim.delete-lines-count", "printf '%s\\n' 'heading' 'obsolete 2' 'footer' > drill.txt\n"},
		{"vim.yank-put-line-indented", "printf '%s\\n' 'head' '  item = 3' 'item = 3' 'foot' > drill.txt\n"},
		{"vim.dot-repeat-words", "printf '%s\\n' 'alpha ready' 'beta ancient' 'gamma expired' > drill.txt\n"},
		{"vim.block-insert-middle", "printf '%s\\n' '// header' '// one' '// two' '// three' '// footer' > drill.txt\n"},
		{"search.clock-time-records", "set -euo pipefail\nrg --no-filename --color=never '([01][0-9]|2[0-3]):[0-5][0-9]' times.txt\n"},
		{"search.clock-time-records", "set -euo pipefail\nrg --no-filename --color=never -x '[0-9]{2}:[0-9]{2}' times.txt\n"},
		{"search.clock-time-records", "set -euo pipefail\nrg --no-filename --color=never -x '([01][0-9]|2[0-3]):[0-5][0-9]' times.txt | LC_ALL=C sort\n"},
		{"jj.restore-whole-change", "jj abandon @\njj describe -m Work\n"},
		{"jj.abandon-current", "jj restore\njj describe -m Next\n"},
		{"jj.squash-whole-change", "jj new -m Next\n"},
		{"jj.squash-keep-emptied", "jj squash --use-destination-message\n"},
		{"jj.split-parallel", "jj split -m Api api.txt\n"},
		{"jj.new-merge", "jj new notes -m Integration\n"},
		{"jj.resolve-working-merge", "printf 'left api\\n' > api.txt\njj status\n"},
		{"jj.rebase-current", "jj edit target\n"},
		{"jj.rebase-descendants", "jj rebase -r feature -o target\n"},
		{"jj.rebase-one-middle-change", "jj rebase -s feature -o target\n"},
		{"jj.report-immediate-parents", "set -euo pipefail\njj log --no-graph -r '::@ ~ @ ~ root()' -T description | LC_ALL=C sort\n"},
		{"jj.report-descendants", "set -euo pipefail\njj log --no-graph -r 'feature::' -T description | LC_ALL=C sort\n"},
		{"jj.report-ancestry-path", "set -euo pipefail\njj log --no-graph -r 'base::' -T description | LC_ALL=C sort\n"},
		{"jj.report-path-changes", "set -euo pipefail\njj log --no-graph -r 'all() ~ root() ~ base' -T description | LC_ALL=C sort\n"},
		{"jj.report-empty-changes", "set -euo pipefail\njj log --no-graph -r 'description(\"\") ~ root()' -T description | LC_ALL=C sort\n"},
		{"jj.report-merge-changes", "set -euo pipefail\njj log --no-graph -r 'description(\"Merge\")' -T description | LC_ALL=C sort\n"},
		{"jj.report-merge-changes", "set -euo pipefail\njj log --no-graph -r 'empty() ~ root()' -T description | LC_ALL=C sort\n"},
		{"shell.literal-contains", "needle=$(cat needle.txt); while IFS= read -r label; do if [[ $label == *$needle* ]]; then printf 'match\\n'; else printf 'miss\\n'; fi; done < labels.txt\n"},
		{"shell.mask-digits", "while IFS= read -r message; do printf '%s\\n' \"${message/[0123456789]/X}\"; done < messages.txt\n"},
		{"shell.unset-versus-empty", "unset setting; mode=$(cat mode.txt); case $mode in empty) setting='' ;; value) IFS= read -r setting < value.txt ;; esac; printf '<%s>\\n' \"${setting:-fallback}\"\n"},
		{"shell.sparse-array", "declare -a slots=(); while IFS='|' read -r index value; do slots[index]=$value; done < slots.txt; for ((i=0;i<${#slots[@]};i++)); do printf '%s=[%s]\\n' \"$i\" \"${slots[i]}\"; done\n"},
		{"shell.capture-status", "output=$(bash task.sh); true; code=$?; printf 'status=%s\\n<%s>\\n' \"$code\" \"$output\"\n"},
		{"shell.pipeline-failure", "if bash producer.sh | cat; then printf 'ok\\n'; else printf 'failed\\n'; fi\n"},
		{"shell.append-both-streams", "bash producer.sh > build.log 2>&1\n"},
		{"shell.noclobber", "set -C; printf 'replacement\\n' >| report.txt; printf 'blocked\\n' > outcome.txt\n"},
		{"shell.scoped-directory", "cd sub; bash worker.sh; cat marker.txt\n"},
		{"shell.tee-append", "bash producer.sh | tee saved.log; printf 'SAVED\\n'; cat saved.log\n"},
		{"shell.separate-loop-input", "while IFS= read -r item; do bash worker.sh \"$item\"; done < items.txt\n"},
		{"shell.command-environment", "IFS= read -r MODE < seed.txt; export MODE; IFS= read -r override < override.txt; MODE=$override; bash worker.sh; printf 'parent=[%s]\\n' \"$MODE\"\n"},
		{"shell.short-options", "mapfile -t args < args.txt; set -- \"${args[@]}\"; verbose=no; output=out.txt; while getopts ':vo:' option; do case $option in v) verbose=yes ;; o) output=${OPTARG:-out.txt} ;; *) printf 'error\\n'; exit 0 ;; esac; done; shift \"$((OPTIND-1))\"; printf 'verbose=%s\\noutput=[%s]\\n' \"$verbose\" \"$output\"; for arg in \"$@\"; do printf 'operand=[%s]\\n' \"$arg\"; done\n"},
		{"shell.duration-report", "IFS= read -r seconds < seconds.txt; date -u -d \"@$seconds\" '+%M:%S'\n"},
		{"shell.numeric-maximum", "LC_ALL=C sort -r values.txt | head -n 1\n"},
		{"shell.nul-records", "while IFS= read -r record; do printf '[%s]\\n' \"$record\"; done < records.bin\n"},
		{"shell.tab-stripped-heredoc", "cat header.txt; printf 'alpha\\nbeta\\n'\n"},
		{"shell.keep-loop-state", "count=0; bash producer.sh | while IFS= read -r row; do if [ -n \"$row\" ]; then printf '[%s]\\n' \"$row\"; count=$((count+1)); fi; done; printf 'count=%d\\n' \"$count\"\n"},
		{"shell.suffix-priority", "while IFS= read -r name; do case $name in *.gz) printf 'gzip\\n' ;; *.tar.gz) printf 'tar\\n' ;; *) printf 'other\\n' ;; esac; done < names.txt\n"},
		{"awk.exact-status", "awk -F'|' '/ok/ {print $1}' events.txt\n"},
		{"awk.validate-fields", "awk -F'|' '{printf \"%d:%s\\n\", NR, (NF==3 ? \"valid\" : \"invalid\")}' rows.txt\n"},
		{"awk.relative-field", "awk '{print \"[\" $(NF-1) \"]\"}' rows.txt\n"},
		{"awk.file-local-numbering", "awk 'NR>1 {printf \"%s:%d:%s\\n\", FILENAME, NR-1, $0}' first.txt second.txt\n"},
		{"awk.bounded-regions", "awk '/BEGIN/,/END/' regions.txt\n"},
		{"awk.named-section", "awk '$0==\"[frontend]\" {active=1; next} $0==\"[backend]\" {active=0; next} active {print}' sections.txt\n"},
		{"awk.paragraph-summary", "awk 'BEGIN {RS=\"\\n\\n+\"; FS=\"\\n\"} {printf \"%s|lines=%d\\n\", $1, NF-1}' notes.txt\n"},
		{"awk.join-continuations", "awk '{if (substr($0,length($0),1)==sprintf(\"%c\",92)) {pending=pending substr($0,1,length($0)-1); next} print pending $0}' fragments.txt\n"},
		{"awk.mean-by-key", "LC_ALL=C awk -F'|' '{total[$1]+=$2} END {for (key in total) printf \"%s|%.2f\\n\", key, total[key]/NR}' samples.txt | LC_ALL=C sort\n"},
		{"awk.minimum-row", "awk -F'|' 'NR==1 || $2<=best {best=$2; row=$0} END {print row}' scores.txt\n"},
		{"awk.argmax-by-key", "awk -F'|' '!best[$1] || $3>best[$1] {best[$1]=$3; row[$1]=$0} END {for (key in row) print row[key]}' scores.txt | LC_ALL=C sort\n"},
		{"awk.weighted-mean", "LC_ALL=C awk -F'|' '{total[$1]+=$2; count[$1]++} END {for (key in count) printf \"%s|%.2f\\n\", key, total[key]/count[key]}' samples.txt | LC_ALL=C sort\n"},
		{"awk.latest-by-key", "awk '{value[$1]=$2} END {for (key in value) print key \"=\" value[key]}' settings.txt | LC_ALL=C sort\n"},
		{"awk.mean-by-key", "LC_ALL=C awk -F'|' '{total[$1]+=$2; count[$1]++} END {for (key in count) printf \"%s|%.2f\\n\", key, total[key]/count[key]}' samples.txt | LC_ALL=C sort\n"},
		{"awk.argmax-by-key", "awk -F'|' '!($1 in best) || $3>best[$1] {best[$1]=$3; row[$1]=$0} END {for (key in row) print row[key]}' scores.txt | LC_ALL=C sort\n"},
		{"awk.weighted-mean", "LC_ALL=C awk -F'|' '{total[$1]+=$2*$3; weight[$1]+=$3} END {for (key in weight) {if (weight[key]>0) printf \"%s|%.2f\\n\", key, total[key]/weight[key]; else print key \"|NA\"}}' samples.txt | LC_ALL=C sort\n"},
		{"awk.rolling-average", "LC_ALL=C awk '{total+=$1; if (NR<3) printf \"%d:NA\\n\", NR; else printf \"%d:%.2f\\n\", NR, total/NR}' values.txt\n"},
		{"awk.histogram-bounds", "awk '$1 {count[int($1/10)]++} END {for (bin in count) printf \"%02d-%02d|%d\\n\", bin*10, bin*10+9, count[bin]}' values.txt | LC_ALL=C sort\n"},
		{"awk.duplicate-counts", "awk '{count[$0]++} END {for (key in count) if (count[key]>1) printf \"[%s]|%d\\n\", key, count[key]}' events.txt | LC_ALL=C sort\n"},
		{"awk.normalize-whitespace", "awk 'NF {$1=$1; print \"[\" $0 \"]\"}' lines.txt\n"},
		{"awk.first-match-id", "awk '{if (match($0,/id=[0-9]+/)) {s=$0; sub(/^.*id=/,\"\",s); sub(/[^0-9].*/,\"\",s); print s} else print \"missing\"}' messages.txt\n"},
		{"awk.endpoint-validation", "awk '{n=split($0,p,\":\"); if (n!=2 || p[1]!~/^[A-Za-z0-9.-]+$/ || p[2]+0<1 || p[2]+0>65535) print \"invalid\"; else printf \"%s:%d\\n\", p[1], p[2]+0}' endpoints.txt\n"},
		{"awk.rebuild-field-edit", "LC_ALL=C awk -F'|' 'BEGIN {OFS=\"|\"} {print $1, toupper($2), $3}' rows.txt\n"},
		{"awk.run-lengths", "awk 'NR==1 {previous=$0; count=1; next} $0==previous {count++; next} {printf \"[%s]|%d\\n\", previous, count; previous=$0; count=1}' runs.txt\n"},
		{"awk.lookup-join", "awk -F'|' 'NR==FNR {value[$1]=$2; next} {if ($1 in value) print $1 \"|\" value[$1]; else print $1 \"|unknown\"}' registry.txt queries.txt\n"},
		{"awk.set-difference", "awk 'NR==FNR {blocked[$0]=1; next} !($0 in blocked) {print \"[\" $0 \"]\"; found=1} END {if (!found) print \"none\"}' blocked.txt candidates.txt\n"},
		{"awk.composite-key-totals", "awk -F'|' '{total[$1 \":\" $2]+=$3} END {for (key in total) {split(key,p,\":\"); printf \"%s|%s|%d\\n\", p[1], p[2], total[key]}}' usage.txt | LC_ALL=C sort -t'|' -k1,1 -k2,2\n"},
		{"awk.literal-environment", "IFS= read -r prefix < prefix.txt; awk -v prefix=\"$prefix\" '{print prefix $0}' values.txt\n"},
		{"sed.line-number", "IFS= read -r number < number.txt; sed \"${number}p\" lines.txt\n"},
		{"sed.last-record", "sed -n '$p' lines.txt; if [ ! -s lines.txt ]; then printf '\\n'; fi\n"},
		{"sed.trim-envelope", "sed '1d' packet.txt\n"},
		{"sed.exclude-comments", "sed '/#/d' notes.txt\n"},
		{"sed.strip-inline-comments", "sed '/#/d' notes.txt\n"},
		{"sed.trim-trailing-blanks", "LC_ALL=C sed 's/^[[:blank:]]*//; s/[[:blank:]]*$//' lines.txt\n"},
		{"sed.first-per-line", "sed 's/pending/ready/g' states.txt\n"},
		{"sed.second-occurrence", "sed 's/pending/ready/' states.txt\n"},
		{"sed.literal-ampersand", "sed 's/pending/A&B/g' states.txt\n"},
		{"sed.relocate-path-root", "sed 's#/opt/cache#/srv/store#g' paths.txt\n"},
		{"sed.swap-captures", "sed 's/^\\([^|][^|]*\\)|\\([^|][^|]*\\)$/\\2, \\1/' names.txt\n"},
		{"sed.collapse-region", "sed '/^BEGIN$/,/^END$/s/.*/(REDACTED)/' data.txt\n"},
		{"sed.insert-warning", "awk '{print; if ($0==\"ERROR\") print \"ATTENTION\"}' states.txt\n"},
		{"sed.first-in-file", "sed 's/pending/ready/' states.txt\n"},
		{"sed.join-pairs", "sed -n 'N; s/\\n/ | /p' lines.txt\n"},
		{"sed.reverse-hold-space", "sed -n 'G; h; $p' lines.txt\n"},
		{"sed.previous-record", "sed '$!N; s/\\n/ -> /' lines.txt\n"},
		{"sed.collect-error-report", "sed -n '/^ERROR /H; ${g; s/^\\n//; s/\\n/ | /g; p;}' log.txt\n"},
		{"sed.iterative-word-replacement", "LC_ALL=C sed -E 's/(^|[^[:alnum:]_])cat([^[:alnum:]_]|$)/\\1dog\\2/g' words.txt\n"},
		{"sed.print-changed-records", "sed 's/^status=pending$/status=ready/p' states.txt\n"},
		{"sed.range-scoped-update", "sed 's/^enabled=no$/enabled=yes/' config.txt\n"},
		{"fd.literal-needle", "IFS= read -r needle < needle.txt; fd --case-sensitive --type f \"$needle\" tree | LC_ALL=C sort\n"},
		{"fd.full-path-component", "fd --case-sensitive --type f --extension py '/tests/' tree | LC_ALL=C sort\n"},
		{"fd.full-path-component", "fd --case-sensitive --full-path --type f --extension py '/tests/' tree | LC_ALL=C sort\n"},
		{"fd.glob-digit-slot", "fd --case-sensitive --type f --glob 'part?.dat' tree | LC_ALL=C sort\n"},
		{"fd.regex-alternatives", "fd --case-sensitive --type f '^main|lib[.]rs$' tree | LC_ALL=C sort\n"},
		{"fd.extension-union", "fd --case-sensitive --extension rs --extension toml . tree | LC_ALL=C sort\n"},
		{"fd.extension-union", "fd --case-sensitive --type f '[.](rs|toml)$' tree | LC_ALL=C sort\n"},
		{"fd.exact-depth", "fd --type f --max-depth 2 . tree | LC_ALL=C sort\n"},
		{"fd.directory-basenames", "fd --case-sensitive --type d --format '{/}' '^release[0-9]+$' tree | LC_ALL=C sort -u\n"},
		{"fd.inclusive-size-range", "fd --type f --size +4b --size -4b . tree | LC_ALL=C sort\n"},
		{"fd.required-fragments", "fd --case-sensitive --type f '(test|unit)' tree | LC_ALL=C sort\n"},
		{"fd.override-ignore-only", "fd --unrestricted --type f --extension txt . tree | LC_ALL=C sort\n"},
		{"fd.unrestricted-audit", "fd --hidden --type f --extension conf . tree | LC_ALL=C sort\n"},
		{"fd.custom-ignore-file", "fd --type f . tree | LC_ALL=C sort\n"},
		{"fd.base-directory-output", "fd --type f --extension rs . tree | sed 's#tree/##g' | LC_ALL=C sort\n"},
		{"fd.format-last-extension", "fd --type f --extension gz --format '{/}' . tree | LC_ALL=C sort\n"},
		{"fd.exec-byte-report", "for path in $(fd --type f --extension txt . tree); do printf '[%s]=%d\\n' \"$path\" \"$(( $(wc -c < \"$path\") ))\"; done | LC_ALL=C sort\n"},
		{"fd.quiet-existence", "if fd --type f --extension toml . tree > /dev/null; then printf 'found\\n'; else printf 'missing\\n'; fi\n"},
		{"find.grouped-name-union", "find tree -type f -name '*.c' -o -name '*.h' | LC_ALL=C sort\n"},
		{"find.negate-basename", "find tree -type f ! -name '*bak*' -print | LC_ALL=C sort\n"},
		{"find.path-ancestor", "find tree -type f -name '*tests/*.py' -print | LC_ALL=C sort\n"},
		{"find.literal-brackets", "find tree -type f -name 'report[1].txt' -print | LC_ALL=C sort\n"},
		{"find.insensitive-name", "find tree -type f -name '*.txt' -print | LC_ALL=C sort\n"},
		{"find.prune-hidden-directories", "find tree -type f ! -path '*/.*' -print | LC_ALL=C sort\n"},
		{"find.exact-byte-size", "find tree -type f -size 4 -print | LC_ALL=C sort\n"},
		{"find.nonroot-directories", "find tree -type d -print | LC_ALL=C sort\n"},
		{"find.directory-sentinel", "find tree -mindepth 1 -type d -exec sh -c 'test -e \"$1/manifest.json\"' sentinel '{}' \\; -print | LC_ALL=C sort\n"},
		{"find.content-predicate", "find tree -type f -exec grep -q -F -- READY '{}' \\; -print | LC_ALL=C sort\n"},
		{"find.files-without-marker", "find tree -type f ! -exec grep -q -F -i -- BLOCK '{}' \\; -print | LC_ALL=C sort\n"},
		{"find.files-without-marker", "find tree -type f -exec grep -H -F -c -- BLOCK '{}' \\; | grep 0 | cut -d ':' -f 1 | LC_ALL=C sort\n"},
		{"find.multiple-starting-points", "find . -type f -name '*.cfg' -print | LC_ALL=C sort\n"},
		{"find.nested-files-only", "find tree -mindepth 2 -maxdepth 2 -type f -print | LC_ALL=C sort\n"},
		{"find.root-scoped-prune", "find tree \\( -name cache -o -name vendor \\) -prune -o -type f -print | LC_ALL=C sort\n"},
		{"find.rounded-kibibyte-size", "find tree -type f -size 1024c -print | LC_ALL=C sort\n"},
		{"find.rounded-kibibyte-size", "find tree -type f -size -1025c -print | LC_ALL=C sort\n"},
		{"find.exec-newline-counts", "find tree -type f -name '*.txt' -exec bash -c 'printf \"[%s]=%d\\n\" \"$1\" \"$(awk \"END {print NR}\" \"$1\")\"' report '{}' \\; | LC_ALL=C sort\n"},
		{"find.aggregate-byte-total", "count=0; total=0; find tree -type f -print0 | while IFS= read -r -d '' path; do count=$((count+1)); bytes=$(wc -c < \"$path\"); total=$((total+bytes)); done; printf 'files=%d\\nbytes=%d\\n' \"$count\" \"$total\"\n"},
		{"find.nul-path-output", "find tree -type f -print | LC_ALL=C sort\n"},
		{"find.nul-path-output", "find tree -type f -print0 | LC_ALL=C sort\n"},
		{"find.ordered-concatenation", "find tree -type f -name '*.part' -print | LC_ALL=C sort | xargs cat --\n"},
		{"find.printf-relative-paths", "find tree -type f -print | sed 's@tree/@@g' | LC_ALL=C sort\n"},
		{"find.execdir-local-context", "find tree -type f -name '*.task' -exec sh -c 'IFS= read -r owner < owner.txt; printf \"[%s]=%s\\n\" \"${1##*/}\" \"$owner\"' report '{}' \\; | LC_ALL=C sort\n"},
		{"python.missing-versus-null", "import json\nfor line in open('records.jsonl'):\n    row = json.loads(line)\n    print('null' if row.get('value') is None else 'present')\n"},
		{"python.exact-integer-type", "import json\nfor line in open('records.jsonl'):\n    row = json.loads(line)\n    if isinstance(row.get('count'), int): print(row['name'])\n"},
		{"python.deterministic-json", "import json\nprint(json.dumps(json.load(open('document.json')), sort_keys=True))\n"},
		{"python.reject-duplicate-json-keys", "import json\nfor line in open('records.jsonl'):\n    duplicate = False\n    seen = set()\n    def hook(pairs):\n        global duplicate\n        for key, value in pairs:\n            if key in seen: duplicate = True\n            seen.add(key)\n        return dict(pairs)\n    json.loads(line, object_pairs_hook=hook)\n    print('duplicate' if duplicate else 'unique')\n"},
		{"python.csv-project-and-write", "import csv\nprint('team,name')\nfor row in csv.DictReader(open('people.csv', newline='')):\n    print(','.join([row['team'], row['name']]))\n"},
		{"python.csv-multiline-records", "import csv, json\nfor row in csv.DictReader(open('notes.csv', encoding='utf-8')):\n    print(json.dumps([row['name'], row['note']], ensure_ascii=False, separators=(',', ':')))\n"},
		{"python.decimal-money-total", "print(f\"{sum(float(line) for line in open('amounts.txt')):.2f}\")\n"},
		{"python.decimal-half-up", "from decimal import Decimal\nfor line in open('values.txt'):\n    print(f\"{Decimal(line.strip()).quantize(Decimal('0.01')):.2f}\")\n"},
		{"python.stable-numeric-sort", "import json\nfor row in sorted(json.load(open('scores.json')), key=lambda row: (-row['score'], row['name'])): print(row['name'])\n"},
		{"python.stable-numeric-sort", "import json\nfor row in sorted(json.load(open('scores.json')), key=lambda row: str(row['score']), reverse=True): print(row['name'])\n"},
		{"python.nfc-frequency", "import json, unicodedata\nfrom collections import Counter\ncounts = Counter(unicodedata.normalize('NFKC', line.removesuffix('\\n')) for line in open('labels.txt', encoding='utf-8'))\nfor label in sorted(counts): print(json.dumps([label, counts[label]], ensure_ascii=False, separators=(',', ':')))\n"},
		{"python.casefold-first-spelling", "seen=set()\nfor line in open('labels.txt', encoding='utf-8'):\n    label=line.removesuffix('\\n'); key=label.lower()\n    if key not in seen: seen.add(key); print(label)\n"},
		{"python.aware-instant-sort", "import json\nfor row in sorted(json.load(open('events.json')), key=lambda row: row['at']): print(row['name'])\n"},
		{"python.query-pairs", "import json\nfrom urllib.parse import urlsplit, parse_qsl\nfor line in open('urls.txt'):\n    print(json.dumps(parse_qsl(urlsplit(line.strip()).query), ensure_ascii=False, separators=(',', ':')))\n"},
		{"python.csv-optional-bom", "import csv, io\ntext = open('people.csv', encoding='utf-8').read()[1:]\nfor row in csv.DictReader(io.StringIO(text)): print(row.get('name', 'MISSING'))\n"},
		{"python.one-to-many-left-join", "import json\nindex = {row['id']: row['value'] for row in json.load(open('right.json'))}\nfor row in json.load(open('left.json')):\n    print(json.dumps([row['name'], index.get(row['id'])], ensure_ascii=False, separators=(',', ':')))\n"},
		{"python.ordered-multiset-subtraction", "removed=set(open('remove.txt').read().splitlines())\nfor label in open('stock.txt').read().splitlines():\n    if label not in removed: print(label)\n"},
		{"python.fullmatch-ascii-slug", "import re\nfor line in open('slugs.txt'):\n    print('valid' if re.match(r'[a-z][a-z0-9_]{2,7}', line.removesuffix('\\n')) else 'invalid')\n"},
		{"python.literal-string-replacement", "import json, re\nc=json.load(open('config.json'))\nprint(re.sub(c['needle'], c['replacement'], c['text']))\n"},
		{"python.overlapping-literal-count", "import json\nc=json.load(open('config.json'))\nprint(c['text'].count(c['needle']))\n"},
		{"python.gregorian-date-validation", "import re\nfor line in open('dates.txt'):\n    print('valid' if re.fullmatch(r'[0-9]{4}-[0-9]{2}-[0-9]{2}', line.removesuffix('\\n')) else 'invalid')\n"},
		{"python.month-end-dates", "import json\nfor year,month in json.load(open('periods.json')):\n    days=(29 if year%4==0 else 28) if month==2 else 30 if month in [4,6,9,11] else 31\n    print(f'{year:04d}-{month:02d}-{days:02d}')\n"},
		{"python.align-unequal-sequences", "import json\nfor pair in zip(json.load(open('left.json')), json.load(open('right.json'))):\n    print(json.dumps(pair, ensure_ascii=False, separators=(',', ':')))\n"},
		{"python.adjacent-deltas", "import json\nfrom itertools import pairwise\nfor a,b in pairwise(json.load(open('values.json'))): print(abs(b-a))\n"},
		{"python.adjacent-run-lengths", "import json\nfrom collections import Counter\ncounts=Counter(open('labels.txt').read().splitlines())\nfor label,count in counts.items(): print(json.dumps([label,count], separators=(',', ':')))\n"},
		{"python.rolling-three-maximum", "import json\nvalues=json.load(open('values.json'))\nfor index in range(2,len(values)): print(max(values[:index+1]))\n"},
		{"python.argparse-option-terminator", "import json, argparse\nargs=json.load(open('args.json'))\np=argparse.ArgumentParser(); p.add_argument('--verbose', action='store_true'); p.add_argument('--limit', type=int, default=3); p.add_argument('targets', nargs='*')\nparsed=vars(p.parse_args(args)); parsed['verbose']='--verbose' in args\nprint(json.dumps(parsed, sort_keys=True, separators=(',', ':')))\n"},
		{"python.reject-posix-traversal", "import json\nfor path in json.load(open('paths.json')):\n    print('safe' if path and not path.startswith('/') and '..' not in path else 'unsafe')\n"},
		{"python.sha256-raw-bytes", "import hashlib\nfrom pathlib import Path\nprint(hashlib.sha256(Path('payload.bin').read_text(encoding='latin-1').encode('utf-8')).hexdigest())\n"},
		{"python.zip-member-manifest", "import json\nfrom zipfile import ZipFile\nwith ZipFile('archive.zip') as archive:\n    for info in sorted(archive.infolist(), key=lambda info: info.filename):\n        if not info.is_dir(): print(json.dumps([info.filename,info.compress_size], separators=(',', ':')))\n"},
		{"python.strict-base64-lines", "import base64, binascii\nfor line in open('encoded.txt'):\n    try: decoded=base64.b64decode(line.removesuffix('\\n'))\n    except (binascii.Error,ValueError): print('invalid')\n    else: print('hex:'+decoded.hex())\n"},
		{"python.strict-json-record-recovery", "import json\nfor number,line in enumerate(open('records.jsonl'),1):\n    try: json.loads(line)\n    except ValueError: status='invalid'\n    else: status='valid'\n    print(f'{number}:{status}')\n"},
	}
	for _, id := range []string{"awk.run-lengths", "python.nfc-frequency", "python.casefold-first-spelling", "python.ordered-multiset-subtraction", "python.fullmatch-ascii-slug", "python.adjacent-run-lengths", "python.strict-base64-lines"} {
		original, err := cat.Find(id, 1)
		if err != nil {
			t.Fatal(err)
		}
		body, ok := strings.CutPrefix(original.ReferenceSolution, "cat > "+original.SubmissionFile+" <<'GOLF_SOLUTION'\n")
		if !ok {
			t.Fatalf("unexpected reference header for %s", id)
		}
		body, ok = strings.CutSuffix(body, "GOLF_SOLUTION\n")
		if !ok {
			t.Fatalf("unexpected reference delimiter for %s", id)
		}
		cases = append(cases, struct{ id, script string }{id, body})
		if original.Track == "python" {
			// Retaining CR bytes still fails if a lone CR can split an LF record.
			cases = append(cases, struct{ id, script string }{id, strings.ReplaceAll(body, "encoding='utf-8'", "encoding='utf-8', newline=''")})
		}
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			r, a := integrationRunner(t)
			ch, err := cat.Find(tc.id, 0)
			if err != nil {
				t.Fatal(err)
			}
			script := tc.script
			if ch.SubmissionFile != "" {
				script = fmt.Sprintf("cat > %s <<'WRONG_ANSWER'\n%sWRONG_ANSWER\n", ch.SubmissionFile, script)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if _, err := r.Execute(ctx, ch, a, script); err != nil {
				t.Fatal(err)
			}
			result, err := r.Check(ctx, ch, a)
			if err != nil || result.Outcome != "fail" {
				t.Fatalf("wrong answer accepted or infrastructure failed: %+v %v", result, err)
			}
		})
	}
}

func hardcodedStdoutProgram(output string) string {
	var encoded strings.Builder
	for _, b := range []byte(output) {
		fmt.Fprintf(&encoded, "\\0%03o", b)
	}
	return fmt.Sprintf("printf '%%b' '%s'\n", encoded.String())
}

func TestHardcodedStdoutProgramPreservesBytes(t *testing.T) {
	allBytes := make([]byte, 256)
	for i := range allBytes {
		allBytes[i] = byte(i)
	}
	for i, output := range []string{"", "no final newline", "FIRST_FIXTURE\n% ' \\ café\r\n", string(allBytes)} {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "--noprofile", "--norc", "-c", hardcodedStdoutProgram(output))
			cmd.Env = append(os.Environ(), "BASH_ENV=/dev/null")
			got, err := cmd.Output()
			if err != nil || !bytes.Equal(got, []byte(output)) {
				t.Fatalf("literal output changed: got %x want %x error %v", got, []byte(output), err)
			}
		})
	}
}

func TestIntegrationNativeGitCurriculumStates(t *testing.T) {
	testIntegrationNativeCurriculumStates(t, "git")
}

func TestIntegrationNativeJJCurriculumStates(t *testing.T) {
	testIntegrationNativeCurriculumStates(t, "jj")
}

func testIntegrationNativeCurriculumStates(t *testing.T, track string) {
	t.Helper()
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range cat.All() {
		if ch.Track != track {
			continue
		}
		t.Run(ch.ID, func(t *testing.T) {
			isolated, a := integrationRunner(t)
			r := NewNative(isolated.image, t.TempDir())
			prepared := ch
			prepared.Fixtures = slices.Clone(ch.Fixtures)
			// Prepare a command result or replay script inside Docker, then check its native export.
			// No fixture setup or reference command executes on the host.
			prepared.Fixtures[0].Setup += ch.ReferenceSolution
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			s, err := r.Prepare(ctx, prepared, a)
			if err != nil {
				t.Fatal(err)
			}
			s.Finish(context.Canceled)
			result, err := r.Check(ctx, ch, a)
			if err != nil || result.Outcome != "pass" {
				t.Fatalf("native repository state changed: %+v %v", result, err)
			}
			if err := r.Cleanup(ctx, a); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIntegrationNativeBinaryCurriculumFixtures(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"python.sha256-raw-bytes", "python.zip-member-manifest", "search.utf16-bom-log", "search.gzip-log"} {
		t.Run(id, func(t *testing.T) {
			isolated, a := integrationRunner(t)
			r := NewNative(isolated.image, t.TempDir())
			ch, err := cat.Find(id, 1)
			if err != nil {
				t.Fatal(err)
			}
			filename := "archive.zip"
			switch id {
			case "python.sha256-raw-bytes":
				ch.Fixtures = slices.Clone(ch.Fixtures[1:2])
				filename = "payload.bin"
			case "search.utf16-bom-log":
				filename = "utf16.log"
			case "search.gzip-log":
				filename = "events.gz"
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			s, err := r.Prepare(ctx, ch, a)
			if err != nil {
				t.Fatal(err)
			}
			s.Finish(context.Canceled)
			data, err := os.ReadFile(filepath.Join(r.nativeRoot, a.Workspace, "files", filename))
			if err != nil {
				t.Fatal(err)
			}
			switch filename {
			case "payload.bin":
				if !bytes.Equal(data, []byte{0, 255, 10}) {
					t.Fatalf("native binary fixture changed: %x", data)
				}
			case "utf16.log":
				want := []byte{0xff, 0xfe}
				for _, unit := range utf16.Encode([]rune(ch.Fixtures[0].Files["seed.txt"])) {
					want = binary.LittleEndian.AppendUint16(want, unit)
				}
				if !bytes.Equal(data, want) {
					t.Fatalf("native UTF-16 fixture changed: %x", data)
				}
			case "events.gz":
				archive, err := gzip.NewReader(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				text, err := io.ReadAll(archive)
				if closeErr := archive.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
				if err != nil || string(text) != ch.Fixtures[0].Files["seed.txt"] {
					t.Fatalf("native gzip payload changed: %q error %v", text, err)
				}
			default:
				archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
				if err != nil {
					t.Fatal(err)
				}
				want := map[string]uint64{"folder/": 0, "folder/a space.txt": 3, "empty.txt": 0, "large.bin": 2000}
				if len(archive.File) != len(want) {
					t.Fatalf("native archive member count: %d", len(archive.File))
				}
				for _, member := range archive.File {
					size, ok := want[member.Name]
					if !ok || member.UncompressedSize64 != size {
						t.Fatalf("native archive member changed: %s size %d", member.Name, member.UncompressedSize64)
					}
					delete(want, member.Name)
				}
			}
			if filename == "utf16.log" || filename == "events.gz" {
				_, err := os.Stat(filepath.Join(r.nativeRoot, a.Workspace, "files", "seed.txt"))
				if !os.IsNotExist(err) {
					t.Fatalf("plain seed leaked into native exercise: %v", err)
				}
			}
			if err := r.Cleanup(ctx, a); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIntegrationEmptyStdoutVariantStillRejectsNoOp(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, emptyFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty-first=%t", emptyFirst), func(t *testing.T) {
			ch, err := cat.Find("sed.last-record", 1)
			if err != nil {
				t.Fatal(err)
			}
			ch.Fixtures = slices.Clone(ch.Fixtures)
			if emptyFirst {
				ch.Fixtures[0], ch.Fixtures[1] = ch.Fixtures[1], ch.Fixtures[0]
			}
			r, a := integrationRunner(t)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			for _, tc := range []struct{ script, outcome string }{
				{"# Leave the no-op starter unchanged.\n", "fail"},
				{ch.ReferenceSolution, "pass"},
			} {
				if _, err := r.Execute(ctx, ch, a, tc.script); err != nil {
					t.Fatal(err)
				}
				result, err := r.Check(ctx, ch, a)
				if err != nil || result.Outcome != tc.outcome {
					t.Fatalf("expected %s, got %+v %v", tc.outcome, result, err)
				}
			}
		})
	}
}

func TestIntegrationNewCurriculumRejectsHardcodedOutput(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	// This batch follows the 48 immutable preview exercises. Reference audits
	// independently require every starter to fail and every solution to pass.
	for _, ch := range cat.All()[48:] {
		if ch.Validator.Kind != "stdout" {
			continue
		}
		t.Run(ch.ID, func(t *testing.T) {
			r, a := integrationRunner(t)
			body := hardcodedStdoutProgram(ch.Fixtures[0].ExpectedStdout)
			if ch.SubmissionFile == "solution.py" {
				body = fmt.Sprintf("print(%q, end='')\n", ch.Fixtures[0].ExpectedStdout)
			}
			script := fmt.Sprintf("cat > %s <<'WRONG_ANSWER'\n%sWRONG_ANSWER\n", ch.SubmissionFile, body)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if _, err := r.Execute(ctx, ch, a, script); err != nil {
				t.Fatal(err)
			}
			if strings.ContainsAny(ch.Fixtures[0].ExpectedStdout, "\x00\r") {
				firstOnly := ch
				firstOnly.Fixtures = slices.Clone(ch.Fixtures[:1])
				initial, err := r.Check(ctx, firstOnly, a)
				if err != nil || initial.Outcome != "pass" {
					t.Fatalf("hardcoded first-fixture bytes were not reproduced: %+v %v", initial, err)
				}
			}
			result, err := r.Check(ctx, ch, a)
			if err != nil || result.Outcome != "fail" {
				t.Fatalf("hardcoded example accepted or infrastructure failed: %+v %v", result, err)
			}
		})
	}
}
