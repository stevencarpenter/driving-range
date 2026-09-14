# Contributing

Build and test with Go 1.26.8 or newer. CI and release builds pin Go 1.26.8. Docker is required only for interactive practice and execution checks.

## Checks

```sh
make check
```

This runs unit tests, `go vet ./...`, and `golf audit` for catalog metadata. Runtime tests skip unless `GOLF_INTEGRATION=1`. A passing ordinary test run does not verify Docker isolation or reference solutions.

Run the execution checks against a trusted local Docker engine:

```sh
make setup
make integration
make audit-solutions
```

`integration` verifies runtime behavior with temporary, owned workspaces. `audit-solutions` rejects starters that already pass and reference solutions that fail. To inspect one track, run `./golf audit --solutions --track jj`. `GOLF_IMAGE` selects the application image; `GOLF_TEST_IMAGE` selects the integration test image. `make integration` maps a nonempty `GOLF_IMAGE` to `GOLF_TEST_IMAGE` for consistency.

CI runs ordinary checks on Linux and macOS. Docker checks run only when a maintainer dispatches the CI workflow with `docker_checks` enabled. They build and audit the native Linux amd64 and arm64 runtimes separately. Crosscompilation proves binary construction, not terminal behavior or execution on another architecture.

`make smoke` runs the terminal smoke script against the built binary and requires Python 3 and the prepared Docker runtime.

## Exercise contract

Edit `internal/catalog/data/catalog.json`. Each challenge has an immutable ID/revision pair, track, objective, brief, tools, concepts, difficulty, duration estimate, at least two hints, an explanation, an executable reference solution, and license/author metadata.

1. Supply a starter fixture and explicit success conditions. Keep data synthetic and deterministic.
2. Choose `tree` for exact file contents, `stdout` for replayable submissions, or `commands` for repository-state checks. Current packs require exact output, including trailing newlines and ordering.
3. For `stdout`, declare the submission filename and interpreter arguments. Include at least two fixtures that exercise different inputs. The checker replays only the declared submission file against each fresh fixture.
4. Run `make check` and the affected track's `audit --solutions`. An unchanged starter must fail; the reference must pass. Add a wrong-answer regression when it protects a meaningful validation boundary.
5. Attribute source material and record review evidence. Passing code checks does not mean a human reviewed the brief or teaching objective.

Fixture paths must be relative regular files with no traversal, symlinks, hard links, devices, or file/directory collisions. Setup and reference scripts execute trusted code inside the sandbox. Never add network dependencies, secrets, personal shell history, or host paths to an exercise.

Daily entries fix a UTC date, track, exercise revision, and seed. Do not silently replace an existing assignment or change a released revision's meaning. A corrected exercise gets a new revision. The embedded preview ends on 23 September 2026; extending it requires explicit new schedule entries.

Submit focused changes with the relevant validation command and observed result. Keep performance metrics comparable across revisions and environments. Do not change the sibling `vim-golf` repository as part of this project.

## Release packaging

```sh
make check
make release VERSION=v0.1.0
```

The release script creates a new `dist/v0.1.0` directory containing `golf_v0.1.0_darwin_arm64.tar.gz`, `golf_v0.1.0_linux_amd64.tar.gz`, `golf_v0.1.0_linux_arm64.tar.gz`, and `SHA256SUMS`. Each archive includes the binary, documentation, and license notices. It refuses to replace an existing output directory. Set `GO` to an alternate Go executable or `RELEASE_DIR` to a new output directory if needed.

Check the archives from that output directory:

```sh
cd dist/v0.1.0
shasum -a 256 -c SHA256SUMS
tar -tzf golf_v0.1.0_darwin_arm64.tar.gz
```

Linux also supports `sha256sum -c SHA256SUMS`. Preserve the source commit, Go version, native runtime audit logs, and actual terminal test results when reviewing a release. Do not describe crossbuilt targets as runtime-tested without execution evidence.

The release workflow runs when a GitHub Release is published. It runs ordinary checks, crossbuilds the archives, and attaches them to that existing release. It does not create releases or push tags. Set the repository Actions variable `GOLF_ATTEST_RELEASES=true` to generate signed GitHub provenance attestations before upload. Attestation availability depends on the repository's GitHub plan and visibility; see [the official action documentation](https://github.com/actions/attest). No signing key needs to be stored in this repository.

For a release that actually contains an attestation, verify its downloaded archive with GitHub CLI:

```sh
gh attestation verify golf_v0.1.0_darwin_arm64.tar.gz --repo stevencarpenter/driving-range
```

A checksum detects altered bytes. An attestation additionally binds an artifact to its build workflow. Neither verifies exercise pedagogy or rules out runtime vulnerabilities.

## Licensing

Contributions to application code, fixtures, setup scripts, validators, and solution code use [MIT](LICENSE). Instructional prose uses [CC BY-SA 4.0](CONTENT_LICENSE.md). Include attribution and modification notices for permitted adaptations. Do not copy content from `vim-golf` or an external challenge site without confirmed rights and provenance.

When changing dependencies, update [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) from the module cache's license files for modules returned by `go list -deps` on each release target. Retain additional upstream license notices. Release archives include this notice file and the Go runtime license.
