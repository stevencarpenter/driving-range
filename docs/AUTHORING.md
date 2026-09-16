# Authoring a Driving Range exercise

Edit `internal/catalog/data/catalog.json`, run `go test ./internal/catalog`, then run the sandbox content audit before proposing a change. The bundled catalog is the runnable example of the manifest format. The application loads curated embedded content; `catalog.Decode` validates explicitly supplied JSON for authoring and does not execute it or install it.

## Content contract

Every exercise has a stable lowercase dotted ID and positive integer revision. Changing a published objective, fixture, reference solution, or validator contract requires a new revision. Keep the previous revision available so existing attempts retain their meaning. Revisions are resolved exactly for daily assignments and saved attempts; revision zero in `Find` explicitly selects the newest version for browsing.

Supply a title, objective, complete brief, difficulty from 1 to 5, estimated minutes from 1 to 60, tools, concepts, at least two graduated hints, an explanation, and a Bash reference-solution script. Inline key references must let a player complete the exercise without browser access. State regex dialect, sorting, duplicate handling, newline requirements, filename assumptions, and data-format limits where they affect the answer. The standard image provides GNU tools on Linux, including when the host is macOS.

All supplied instructional prose is an original draft created for this repository. It has not been claimed as human reviewed. The `author` field records that origin explicitly. Instructional prose uses CC BY-SA 4.0; fixtures and solution code use MIT. No exercise content was copied from `vim-golf` or a third-party challenge bank. Contributions must state their own origin and rights. Runnable references establish correctness for supplied fixtures; they do not establish editorial quality, optimality, or skill transfer.

## Fixtures and validation

`fixtures[].files` maps relative paths to UTF-8 content. The first fixture is the player's workspace. Additional fixtures test a reusable submitted script. Paths must be canonical relative file paths without traversal, backslashes, absolute roots, line breaks, or file/directory collisions. Symlinks are not part of the manifest format. Packs are limited to 8 MiB, 1,000 exercises, 200 files per fixture map, and 1 MiB per file.

1. **`tree`** compares workspace files against `expected_files`. The default rejects extra files. Use a real editor exercise when final file content is the learning objective. The unchanged tree must fail. Reference scripts must leave exactly the expected files.
2. **`stdout`** executes `submission_argv` with the player's `submission_file` copied into each fixture. Supply at least two fixtures with different input and nonempty expected output. Each fixture contains the same no-op starter file. The runner must retain the submitted file when rebuilding a fixture. The current curriculum uses `bash solution.sh` or `python3 solution.py` and exact output bytes, including the final newline. Fixture data must exercise the actual rule rather than merely changing labels.
3. **`commands`** runs curated validator argv inside the isolated workspace. Each check declares a name, expected stdout, and expected exit status. Repository checks inspect messages, files, index state, and ancestry. Never assert a timestamp-dependent commit hash. Use `bash --noprofile --norc -c` when a semantic assertion needs shell operators.

`fixtures[].setup` creates synthetic repositories inside the container. `reference_solution` is a Bash script that transforms a fresh, already initialized first fixture into the correct solution. For stdout challenges the reference writes the reusable solution file; the validator then runs it against every fixture. Never run setup, references, or submitted code on the host. A fixture directory is not an isolation boundary.

The bundled exercises all use validator version `1`, profile `standard`, and output policy `exact`. Sorting is explicit in solutions when order is required. No baseline no-op script is allowed to pass. The default tree contract is exact, while repository exercises use semantic checks rather than comparing internal `.git` or `.jj` files.

## Daily schedule

The schedule contains literal assignments for **2026-09-14 through 2026-09-23 UTC**, one per day for each of `vim`, `search`, and `shell`. These 30 assignments form a ten-day preview. Other tracks are practice content. A date/track pair is unique and resolves a fixed exercise revision and seed. The seed identifies the comparison cohort; these fixtures do not use procedural randomness.

`Today` converts its argument to UTC before looking up the literal date. It never rotates an exercise or invents a shared daily after the preview ends. Expired or absent assignments must be presented as practice. Starting an attempt freezes its assignment. Publishing additional days requires adding explicit schedule entries and reviewing their referenced exercise revisions.

## Review checks

1. Run structural catalog tests, including unsafe path, missing metadata, duplicate identity, revision, and schedule rejection.
2. In the isolated runner, verify that each reference solution passes all fixtures and the untouched starter fails.
3. Test a representative wrong answer. For reusable scripts, include a hardcoded first-fixture answer and a script that ignores a meaningful edge case. For tree exercises, change an unrelated file or add an extra file. For repository exercises, leave the target state incomplete.
4. Read the brief without its solution. Confirm the expected state follows from the stated objective and that every hint adds a specific operation without promising a minimum score.
5. Record which operating system, standard image, and tool versions were audited. A check on one host does not establish terminal handoff or runtime support on another.
