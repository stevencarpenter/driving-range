# Contributing

Build and test with Go 1.27 or newer and `just`. CI and release builds pin Go 1.27.1. Docker is required for fixture setup and execution checks. Interactive practice uses installed host tools and their normal configuration.

## Implementation defaults

1. Trace the affected flow and callers before editing. Keep changes focused on the requested behavior.
2. Reuse existing code, then prefer the Go standard library and native platform features. Use `maps.Equal` and `slices.Contains` instead of manual equality or membership loops. New dependencies need a concrete requirement that existing facilities cannot meet.
3. Implement supported behavior only. Keep catalog validation and runner behavior consistent; remove unreachable modes and speculative configuration. Add abstractions when current callers require them.
4. Preserve validation, Docker isolation for setup and checks, native workspace ownership checks, durable writes, crash recovery, terminal restoration, and accessibility. Reducing line count does not justify weakening these contracts.
5. Before merging to `main`, run `just check` and retain a focused regression check for changed nontrivial behavior. Run Docker integration and affected solution audits for runner or validator changes, `just smoke` for terminal handoff changes, and race tests for concurrency changes. Report commands and observed results; distinguish skipped checks from passes.

## Git hooks

Install [Lefthook](https://lefthook.dev/install/) and enable the repository hooks after cloning:

```sh
just hooks
```

The `commit-msg` hook rejects titles that do not follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/). Use `type: description`, optionally adding a scope and `!`, such as `fix(tui): restore terminal` or `feat(cli)!: change command syntax`. Types may include custom names; scopes contain no whitespace, and descriptions must be nonempty. Commit bodies and footers are preserved. Git makes the message available at `commit-msg`, after `pre-commit` runs. The installed hook fails if Lefthook is unavailable.

Use the same convention for pull request titles, since squash merges supply the release commit title. Git hooks are local to each checkout and do not validate GitHub squash messages. Run `just test-commit-msg` to check the validator without installing Lefthook.

## Checks

```sh
just check
```

This runs unit tests, `go vet ./...`, `golf audit` for catalog metadata, installation regression checks, commit-title regression checks, and runtime-shard dispatch regression checks without Docker. The terminal smoke check requires native Neovim, Bash, and zsh as well as Docker. Runtime tests skip unless `GOLF_INTEGRATION=1`. A passing ordinary test run does not verify Docker isolation or reference solutions.

Run the execution checks against a trusted local Docker engine:

```sh
just setup
just integration
just audit-solutions
```

Verification is partitioned into six shards: `base`, `vim`, `search`, `shell`, `awk`, and `ancillary`.

- `just integration` runs all six shards with temporary, owned workspaces; `just integration search` runs one. Every growing curriculum test belongs to exactly one shard; base runs every remaining test. Each Go invocation has a 20-minute deadline. Individual Docker, submission, and validator deadlines remain independently bounded.
- `just audit-solutions` audits all twelve tracks and every retained revision. A shard argument selects its tracks; base has no reference tracks. Each audit requires an unchanged starter to fail and its reference solution to pass, with a two-minute context per revision.
- To audit one track, run `just build` followed by `./golf audit --solutions --track jj`. Track IDs and shard names differ: `jj` belongs to `ancillary` and is not a valid argument to `just audit-solutions`.
- `GOLF_IMAGE` selects the application image; `GOLF_TEST_IMAGE` selects the integration test image. The verification script maps a nonempty `GOLF_IMAGE` to `GOLF_TEST_IMAGE` for consistency.

CI runs ordinary checks on Linux and macOS. Pull requests and pushes to `main` are configured to build and audit the native Linux amd64 and arm64 runtimes separately. Workflow definitions describe intended checks, not evidence that a particular revision passed; retain run results with its source commit.

A maintainer can dispatch the same checks with `docker_checks` enabled. Docker jobs run integration and reference audits for every shard on both native architectures; base also runs terminal smoke. Release packaging depends on successful completion of every Linux amd64 verification shard. A metadata-only pass is not sufficient for exercise changes. The separate Go vulnerability workflow runs on pull requests, `main`, and a weekly schedule. Crosscompilation proves binary construction, not terminal behavior or execution on another architecture.

`just smoke` runs the terminal smoke script against the built binary and requires Python 3 and the prepared Docker runtime.

## Exercise contract

Edit `internal/catalog/data/catalog.json`. Each challenge has an immutable ID/revision pair, track, objective, brief, tools, concepts, difficulty, duration estimate, at least two hints, an explanation, an executable reference solution, and license/author metadata.

1. Supply a starter fixture and explicit success conditions. Keep data synthetic and deterministic.
2. Choose `tree` for exact file contents, `stdout` for replayable submissions, or `commands` for repository-state checks. Tree checks use only the first fixture and reject extra files. Stdout and command checks require exact output, including trailing newlines and ordering.
3. For `stdout`, declare the submission filename and interpreter arguments (currently Bash, zsh, or Python). Include at least two fixtures that exercise different inputs. The checker replays only the declared submission file against each fresh fixture, not auxiliary files created during practice.
4. Run `just check` and the affected track's `audit --solutions`. An unchanged starter must fail; the reference must pass. Add a wrong-answer regression when it protects a meaningful validation boundary.
5. Attribute source material and record validation evidence. Follow [the source and adaptation policy](docs/SOURCES.md), including revision-specific rights for Stack Overflow material. Editorial feedback is collected as players practice and report problems; a human review of every exercise is not a publication gate. Passing code checks does not establish teaching quality. The agreed bank contains [at least 365 practice challenges across all tools](docs/CURRICULUM.md), with Vim/Neovim, regex/search, Bash/core CLI, and awk as the expanded grinding priorities. Fundamentals and deliberate repetition are included. Give each authored exercise a stable ID and a useful practice contract; retries and schedule repetitions do not add exercises.

Fixture paths must be relative regular files with no traversal, symlinks, hard links, devices, or file/directory collisions. Setup and reference scripts execute trusted code inside the sandbox. Never add network dependencies, secrets, personal shell history, or host paths to an exercise.

Daily entries fix a UTC date, track, exercise revision, and seed. Do not silently replace an existing assignment or change a released revision's meaning. A corrected exercise gets a new revision. The embedded preview ends on 23 September 2026; extending it requires explicit new schedule entries.

Submit focused changes with the relevant validation command and observed result. Keep performance metrics comparable across revisions and environments.

## Release versioning

Release Please maintains a release PR on pushes to `main`. It generates `CHANGELOG.md` and updates `.release-please-manifest.json`; no application version file is needed. The first release is `v0.1.0`. Use Conventional Commit titles for squash merges: `fix:` increments patch, `feat:` increments minor, and `!` or a `BREAKING CHANGE:` footer marks an incompatible change. While the version is below 1.0.0, incompatible changes increment minor. Other commit types do not independently trigger a release. Tags follow [Go's semantic version convention](https://go.dev/doc/modules/version-numbers) and must not be moved or reused.

1. Merge normal changes with Conventional Commit titles.
2. Review the generated release PR's version and changelog. Approve its workflow runs if GitHub requires it, and wait for the required checks before merging.
3. Merge the release PR. Automation creates its tag and GitHub Release, then dispatches the existing archive workflow at that exact tag.

The workflow uses the built-in `GITHUB_TOKEN`; no personal access token is required. In repository Settings, Actions, General, enable **Allow GitHub Actions to create and approve pull requests**. [GitHub documents](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow) that token-created release events do not trigger another workflow, while explicit workflow dispatch does. Packaging is dispatched with the release tag as both the workflow ref and its input, so build provenance identifies the source being packaged. Token-created pull requests can require a maintainer to approve their workflow runs. Release Please's [action documentation](https://github.com/googleapis/release-please-action) describes its versioning and release PR behavior.

If packaging fails, rerun its failed jobs. For a separate retry, dispatch Release archives with the same existing tag as the workflow ref and `tag_name` input. Packaging uploads to an existing release and refuses to replace existing assets. `golf version` reads the injected tag in release archives or Go's module build information in `go install` builds. Local checkout builds remain `dev` unless `VERSION` is supplied. Installation and `golf update` use `@latest`; before any version is tagged, Go selects the default branch's untagged build.

## Release packaging

```sh
just check
just VERSION=v0.1.0 release
```

The release script creates a new `dist/v0.1.0` directory containing `golf_v0.1.0_darwin_arm64.tar.gz`, `golf_v0.1.0_linux_amd64.tar.gz`, `golf_v0.1.0_linux_arm64.tar.gz`, and `SHA256SUMS`. Each archive includes the binary, documentation, and license notices. It refuses to replace an existing output directory. Set `GO` to an alternate Go executable or `RELEASE_DIR` to a new output directory if needed.

Check the archives from that output directory:

```sh
cd dist/v0.1.0
shasum -a 256 -c SHA256SUMS
tar -tzf golf_v0.1.0_darwin_arm64.tar.gz
```

Linux also supports `sha256sum -c SHA256SUMS`. Preserve the source commit, Go version, native runtime audit logs, and actual terminal test results when reviewing a release. Do not describe crossbuilt targets as runtime-tested without execution evidence.

The archive workflow is dispatched by release versioning, runs when a GitHub Release is published manually, or accepts an existing tag through `workflow_dispatch`. Every job checks out the selected release tag. Linux amd64 verification jobs run every integration/reference shard and base terminal smoke. Packaging waits for all of them, runs ordinary checks, then crossbuilds the archives and attaches them to that existing release. Native Linux arm64 and macOS verification remain separate evidence; the release packaging job does not execute those archives. The archive workflow does not create releases or push tags. Set the repository Actions variable `GOLF_ATTEST_RELEASES=true` to generate signed GitHub provenance attestations before upload. Attestation availability depends on the repository's GitHub plan and visibility; see [the official action documentation](https://github.com/actions/attest). No signing key needs to be stored in this repository.

For a release that actually contains an attestation, verify its downloaded archive with GitHub CLI:

```sh
gh attestation verify golf_v0.1.0_darwin_arm64.tar.gz --repo stevencarpenter/driving-range
```

A checksum detects altered bytes. An attestation additionally binds an artifact to its build workflow. Neither verifies exercise pedagogy or rules out runtime vulnerabilities.

## Licensing

Contributions to application code, original instructional prose, fixtures, setup scripts, validators, solution code, and documentation use [MIT](LICENSE). See [CONTENT_LICENSE.md](CONTENT_LICENSE.md) for the exercise content scope. Include required attribution and modification notices for permitted third-party material. Do not copy external content without confirmed rights and provenance compatible with this license.

When changing dependencies, update [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) from the module cache's license files for modules returned by `go list -deps` on each release target. Retain additional upstream license notices. Release archives include this notice file and the Go runtime license.
