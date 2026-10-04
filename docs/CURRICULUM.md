# Curriculum and year-long release requirements

The launch requirement is **at least 365 useful practice challenges across all tools**, available in the installed application without daily downloads. This is a product-wide minimum, not a per-track quota or a guarantee of a year of unseen exercises at every practice pace. Fundamentals and deliberate repetition count; different learning objectives are not required for every exercise. Publication requires executable-audited batches, not a new application release every day.

## Current catalog

The embedded catalog contains 600 unique exercises and 602 revision definitions. Vim/Neovim, search/regex, Bash/core CLI, and awk each have 100 exercises; the other eight tracks contain 200 total. Retained revisions preserve saved attempts and do not increase the exercise count. The 30 fixed preview assignments remain unchanged; a year-long schedule is not implemented or published. [LAUNCH_AUDIT.md](LAUNCH_AUDIT.md) records execution checkpoints and outstanding verification separately from authored counts.

Players provide editorial feedback as they practice. Human review of every exercise is not a launch or publication gate, and no completed bank-wide review or measured learning effectiveness is claimed.

## Content allocation

These are the authored practice allocations; executable checkpoints are recorded separately. Count authored exercises with stable IDs, not revisions, retries, or additional schedule entries for the same exercise. Multiple exercises may intentionally practice the same operation.

| Track | Current drafts | Allocation | Coverage |
| --- | ---: | ---: | --- |
| Vim/Neovim | 100 | 100 | Forward/backward/word search, next matches, scoped word and range substitutions, first-per-line replacement, captures and global deletion; motions, text objects, registers, macros, visual edits and multiple files |
| Search/regex | 100 | 100 | Literal/regex matching, Unicode/ASCII classes, occurrences and per-file zero counts, CRLF, byte offsets, lookarounds/backreferences, capture reports, escaped fields, record/file conjunctions, hidden/ignore policy, binary/BOM/gzip input, query case policy, merged context, per-file limits, first runs, byte columns, NUL records, grapheme clusters, nested spans, quoted-span exclusion, traversal ceilings |
| Bash/core CLI | 100 | 100 | head/tail/cut/paste/sort/uniq/tr/comm/join/wc/tee; quoting, expansions, arrays, streams, redirection, status, traps, process control and safe filename handling |
| awk | 100 | 100 | Fields/separators, record and file-local numbering, field counts, filters, sums, grouped reports, stable record identity, signed extrema, running totals and ranges; joins and stateful streams |
| sed | 25 | 25 | Addresses, exact records, replacement extent/escaping, scoped ranges, captures, branches, pattern/hold space, multiline transforms |
| find | 25 | 25 | Types, depth, location-scoped pruning, size units, Boolean expressions, NUL framing, ordered content, safe execution |
| fd | 20 | 20 | Literal/regex/glob patterns, path components, extension unions, exact depth, inclusive size bounds, independent hidden/ignore controls, formatting, safe execution, existence status |
| Git | 35 | 35 | Path-scoped index updates, dual/source restoration, intent-to-add, untrack/ignore, staged rename, partial commit/amend, soft/mixed reset, revert, branch/ref operations, detached HEAD, lightweight/annotated tags, cherry-pick/merge recovery, fast-forward/explicit/squash integration, stash index/path preservation |
| jj | 30 | 30 | Change identities, descriptions, edit/new location, path/historical/whole restore, abandon, selective/whole/retained-source squash, dependent/parallel split, bookmark operations, operation undo, working merges/conflicts, current/source/single-middle rebase, graph and patch reports |
| Python | 35 | 35 | JSON/CSV, text, collections, sorting, paths, validation and small standard-library reports |
| zsh | 20 | 20 | Natural/unique/reversed arrays, inclusive ranges, associative presence, split/join flags, dynamic patterns, captures, function locals/options, autoload, pipeline statuses, ranked/recursive globs |
| fzf | 10 | 10 | Exact/inverse/anchored queries, OR/AND grouping, padding semantics, literal accents, fuzzy input order, field scope, NUL framing, exit classification |
| **Total** | **600** | **600** | |

Each track includes fundamental operations, repeated practice, combinations, and recovery tasks. Repeated objectives are intentional when they provide another useful practice opportunity. Vim exercises should revisit core motions and text objects across word lengths, punctuation, nesting, cursor locations, and editing contexts to build fluency. State the practiced operation and provide a clear result contract; do not invent a new concept merely to justify another exercise.

## Daily and longer sessions

One recommended challenge should be sufficient for a day's practice. Practice remains unrestricted for sessions of two to ten challenges or more. Each priority track has 100 exercises so a player can stay with the same tool across repeated sessions. Future recommendations must not disappear when a player completes their exercises early; such a recommendation becomes review, not unseen content. There are no daily locks, required streaks, or penalties for stopping after one.

A 365-exercise bank provides one unseen exercise per day only at one completion per day. Multiple-exercise sessions consume that bank faster. Do not advertise a year of unseen content for every usage pattern.

The existing schedule is still the immutable ten-day preview ending 23 September 2026. It has not been extended by repeating drafts to fill 365 dates. The current implementation selects Today by track. Outside the preview dates, it offers that track's first current exercise as practice, not a rotating or progress-aware recommendation. The agreed year-long release needs one mixed-tool recommendation per UTC day across the product, not 365 assignments per track. Preserve existing date/track/revision/seed assignments and saved attempts when adding that behavior. The release date and the corresponding 365 consecutive UTC dates must be fixed before publishing the year schedule. Outside published dates, label selections as practice.

## Source and publication requirements

1. Use official manuals, tutorials, and other reliable instructional resources to inform fundamental practice. Stack Overflow questions and answers may identify recurring practical problems but are optional, not a quota or a requirement for each challenge. When used, record actual question and answer scores, retrieval date, alternative answers, and caveats. Popularity is not correctness evidence.
2. Map each practiced operation to official tool documentation and the supported checker version. Write original prose, synthetic fixtures, hints, and reference implementations where possible. Follow [the source policy](SOURCES.md) for adaptations. Do not relabel CC BY-SA code as MIT.
3. Require an unchanged starter to fail and a reference to pass. Reusable submissions need different replay inputs, a hardcoded-example rejection, and meaningful edge cases. Add wrong-answer regressions for boundaries, data preservation, and common misconceptions.
4. Collect editorial feedback as players complete exercises. Record the exercise ID/revision and the reported problem with its brief, hints, objective, solution explanation, or difficulty. Correct affected exercises under the revision policy. A named reviewer and review of the entire bank are not required before publication. Automated execution does not establish teaching quality.
5. Publish only audited revisions in the year schedule. Retain previous revisions and their attribution. A numeric catalog count alone does not satisfy the launch requirement.

## Public release requirements

Keep the repository private until content, documentation, licensing, security, and release checks have evidence. Required checks include `just check`, Docker integration, all solution audits, terminal smoke tests, dependency scans, and a redacted secret scan of Git history. Report runtime and architecture separately; a crossbuild is not an execution test.

The GitHub repository should expose verified installation instructions, supported platforms, an actual terminal demonstration, contribution and authoring guidance, issue forms, a PR checklist, automated dependency updates, required merge checks, and a usable private security-reporting channel. Resolve actionable security findings or document a reviewed risk decision. Do not claim that scans establish absence of vulnerabilities or that a draft catalog is editorially reviewed.
