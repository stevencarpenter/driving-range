# Authoring a Driving Range exercise

Edit `internal/catalog/data/catalog.json`. The bundled catalog is the runnable example of the manifest format. The application loads curated embedded content; `catalog.Decode` validates explicitly supplied JSON for authoring and does not execute it or install it. Rebuild the binary to use catalog edits.

Before proposing a change, run ordinary checks and the affected track's sandbox audit:

```sh
just check
just setup
./golf audit --solutions --track vim
```

Replace `vim` with the edited track ID. `just setup` builds the current binary and runtime image and requires network access. If the runtime is already prepared, use `just build` instead. Run the affected integration shard for exercise changes; see [CONTRIBUTING.md](../CONTRIBUTING.md#checks).

## Content contract

Every exercise has a stable lowercase ID such as `vim.change-value` and a positive integer revision. Track IDs are `vim`, `search`, `shell`, `awk`, `sed`, `fd`, `find`, `python`, `zsh`, `fzf`, `git`, and `jj`; tool names such as `rg` are not track IDs. Changing a published objective, fixture, reference solution, or validator contract requires a new revision. Keep the previous revision available so existing attempts retain their meaning. Revisions are resolved exactly for daily assignments and saved attempts; revision zero in `Find` explicitly selects the newest version for browsing. `Current` selects the highest revision per ID in first-seen ID order for practice listings and counts. `All` retains every definition for reference audits, whose results identify both ID and revision.

Supply a title, objective, complete brief, difficulty from 1 to 5, estimated minutes from 1 to 60, tools, concepts, at least two graduated hints, an explanation, and a Bash reference-solution script. Inline key references must let a player complete the exercise without browser access. State regex dialect, sorting, duplicate handling, newline requirements, filename assumptions, and data-format limits where they affect the answer. Checks use the standard Linux image, including GNU utilities and mawk as `awk`. Native practice uses host tools, which may differ, particularly on macOS. Declare the required interpreter in `submission_argv`; an interactive shell preference does not change it.

All supplied instructional prose is an original draft created for this repository. It has not been claimed as human reviewed. The `author` field records that origin explicitly. Original instructional prose, fixtures, and solution code use MIT; set `license` to `MIT` for original contributions. See [the content notice](../CONTENT_LICENSE.md). No exercise content was copied from a third-party challenge bank. Contributions must state their own origin and rights. Record official tool references and problem research in [SOURCES.md](SOURCES.md). Stack Overflow scores identify candidates, not correctness; copying an answer requires its authors, exact revision, compatible license, and modification notice. Do not relicense third-party answer code as MIT. See [CURRICULUM.md](CURRICULUM.md) for the agreed 365-challenge launch requirement. Runnable references establish correctness for supplied fixtures; they do not establish editorial quality, optimality, or skill transfer.

## Fixtures and validation

`fixtures[].files` maps relative paths to JSON string content. Use `fixtures[].setup` inside Docker for binary input that cannot be represented directly, such as ZIP or gzip files. The first fixture is the player's workspace. Additional fixtures test a reusable submitted script. Native export preserves regular-file bytes, not executable modes, timestamps, symlinks, or empty directories. Do not rely on those properties without an explicit supported contract and regression evidence.

Paths must be canonical relative file paths without traversal, backslashes, absolute roots, line breaks, or file/directory collisions. Symlinks are not part of the manifest format. Packs are limited to 8 MiB, 1,000 revision definitions (including retained revisions), 200 files per fixture map, and 1 MiB per file.

1. **`tree`** compares workspace files against the first fixture's `expected_files`; additional fixtures are not replayed. Extra files always fail the tree check. Use a real editor exercise when final file content is the learning objective. The unchanged tree must fail. Reference scripts must leave exactly the expected files.
2. **`stdout`** executes `submission_argv` with the player's `submission_file` copied into each fixture. Supply at least two fixtures with different input. An individual fixture may legitimately expect empty output, but at least one must expect nonempty output so the combined replay matrix rejects the no-op starter. Each fixture contains the same no-op starter file. The runner must retain the submitted file when rebuilding a fixture. The current curriculum uses `bash solution.sh`, `zsh solution.sh`, or `python3 solution.py` and exact output bytes, including the final newline. Only the declared submission file is copied from the player's workspace, not auxiliary scripts or generated data. Fixture data must exercise the actual rule rather than merely changing labels.
3. **`commands`** runs curated validator argv inside the isolated workspace. Each check declares a name, expected stdout, and expected exit status. Repository checks inspect messages, files, index state, and ancestry. Never assert a timestamp-dependent commit hash. Use `bash --noprofile --norc -c` when a semantic assertion needs shell operators.

`fixtures[].setup` creates synthetic repositories inside the container. `reference_solution` is a Bash script that transforms a fresh, already initialized first fixture into the correct solution. For stdout challenges the reference writes the reusable solution file; the validator then runs it against every fixture. Automated setup, reference execution, and submission replay must run inside Docker, not on the host. Players can run their own scripts during native practice, which has normal host permissions. A fixture directory is not an isolation boundary.

The bundled exercises all use validator version `1` and profile `standard`; stdout and tree checks compare exact bytes, and the tree check always rejects extra files. Sorting is explicit in solutions when order is required. No baseline no-op script is allowed to pass. Repository exercises use semantic checks rather than comparing internal `.git` or `.jj` files.

## Daily schedule

The schedule contains literal assignments for **2026-09-14 through 2026-09-23 UTC**, one per day for each of `vim`, `search`, and `shell`. These 30 assignments form a ten-day preview. Other tracks are practice content. A date/track pair is unique and resolves a fixed exercise revision and seed. The seed identifies the comparison cohort; these fixtures do not use procedural randomness.

`Today` converts its argument to UTC before looking up the literal date. It never rotates an exercise or invents a shared daily after the preview ends. Expired or absent assignments must be presented as practice. Starting an attempt freezes its assignment. Publishing additional days requires adding explicit schedule entries and reviewing their referenced exercise revisions.

## Review checks

1. Run structural catalog tests, including unsafe path, missing metadata, duplicate identity, revision, and schedule rejection.
2. In the isolated runner, verify that each reference solution passes all fixtures and the untouched starter fails.
3. Test a representative wrong answer. For reusable scripts, include a hardcoded first-fixture answer and a script that ignores a meaningful edge case. For tree exercises, change an unrelated file or add an extra file. For repository exercises, leave the target state incomplete.
4. Read the brief without its solution. Confirm the expected state follows from the stated objective and that every hint adds a specific operation without promising a minimum score.
5. Record which operating system, standard image, and tool versions were audited. A check on one host does not establish terminal handoff or runtime support on another. Pull-request CI executes the reference audit and integration checks on Linux amd64 and arm64. Retain the exercise ID/revision and source commit with the results. Collect editorial feedback as players practice and identify reported problems by exercise ID/revision. Human review of the entire bank is not a publication gate; describe any actual review separately from execution evidence.
