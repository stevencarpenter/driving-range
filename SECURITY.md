# Security and recovery

Driving Range runs interactive practice with the player’s native tools and configuration. Curated setup scripts, reference solutions, and validation run in Docker. Treat the application, the selected runtime image, and exercise definitions as trusted software. Docker isolation reduces accidental host access; it is not a guarantee against a container escape or a hostile shared Docker daemon.

## Execution boundary

Setup, reference, and validation containers run as UID/GID 1000 with no network, no host bind mounts, no Docker socket, no forwarded credential agents, and no host shell/editor configuration. The root filesystem is read-only, Linux capabilities are dropped, and `no-new-privileges` is enabled. Git hooks and global credential helpers are disabled in the standard image.

Native practice runs in an owned directory beneath the application state directory. It inherits the user’s environment, dotfiles, permissions, and network access. Native tools and their plugins can access the host; changing the working directory is not a sandbox. Shell background jobs have normal host shell behavior.

Initial fixtures occupy an attempt-owned Docker volume. Setup containers stop before files are exported to the native directory. Local snapshots reject symlinks, hard links, special files, and oversized data before validation. Validation uses bounded snapshots and fresh containers. The runner rejects archive traversal, duplicate paths, symlinks, hard links, devices, and excessive file sizes. Container output is bounded and terminal control characters are removed or escaped before display in the outer application.

| Resource | Configured limit |
| --- | --- |
| CPU / memory / processes | 1 CPU, 512 MiB including swap allowance, 128 processes |
| Open files / individual written file | 256 descriptors, 16 MiB |
| Temporary directory / interactive session | 64 MiB, 1 hour |
| Captured command output / validation | 2 MiB per output stream, 60 seconds for a check |
| Workspace snapshot | 4,096 entries, 2 MiB per file, 16 MiB total file data |

Resource limits in this table apply to Docker execution, except the one-hour native session deadline and the snapshot limits. Native tools otherwise have normal host resource access. Docker enforcement depends on the engine and host kernel. Named volumes have no aggregate disk quota. A process can create many files before snapshot validation rejects the workspace, so limit Docker's backing storage when running untrusted experiments. Interactive child output passes through a virtual terminal emulator that parses bytes into cells, so escape sequences from the child cannot reach your terminal, and clipboard writes (OSC 52) are dropped. Output returning to the outer TUI is sanitized. Classic mode (`--classic`) restores the previous behaviour, where interactive child output goes directly to your terminal.

`golf setup` performs networked image construction and runs package installation. It pins the Debian base digest and jj archive hashes, but Debian package versions are resolved at build time. Runtime rebuilds can produce different image IDs. `GOLF_IMAGE` may select arbitrary trusted local code; it is not an image signature verifier. No external pack installation is provided. Native practice is the default; Docker remains required for setup and validation.

## Dependency scan

The 30 September 2026 UTC Trivy 0.72.0 scan of rebuilt Linux arm64 runtime image `sha256:d28b2375d71ed9dd8692609ec57c326853622f2e4969e142df34dbb3c55aa72f` reports **148 distinct advisory IDs** across 386 package/advisory matches: 20 high, 49 medium, 72 low, and 7 unknown. No critical finding or Debian fixed version for the remaining findings is recorded in that scan. These are package-presence findings; reachability and exploitability have not been established. The image was built under the development tag `driving-range-runtime:oss-prep`; it is not a published release.

The rebuild updates PCRE2 from `10.46-1~deb13u2` to `10.46-1~deb13u3`, removing the two fixable advisory IDs reported for that package by the preceding scan. Debian's [PCRE2 tracker](https://security-tracker.debian.org/tracker/source-package/pcre2) identifies the security update for the JIT out-of-bounds write advisory. Go 1.26.8 source vulnerability scans found no vulnerabilities for macOS arm64 and Linux amd64/arm64 on the same date. The application requires Go 1.26.8 or newer, with CI and release builds pinned to 1.26.8. These results are not a claim that the application or runtime is vulnerability-free.

Reproduce a current image scan with an installed Trivy binary:

```sh
trivy image --scanners vuln --format json --output trivy-runtime.json --timeout 5m driving-range-runtime:1
```

To scan the audited development image, replace `driving-range-runtime:1` with `driving-range-runtime:oss-prep`. The tag can identify a different image after rebuilding, and advisory data changes over time. Compare the report's image ID and scan timestamp before comparing counts. This preview does not claim a vulnerability-free image.

## Runtime recovery

### When to use this

Use these checks when Docker is unavailable, an exercise reports an infrastructure error, or a previous session ended unexpectedly.

1. Run `golf doctor`. Expected success: `Isolated Linux runtime is ready.` plus an environment ID. A missing daemon requires starting Docker; a missing runtime image requires `golf setup`.
2. Run `golf history`. Expected output includes the attempt ID and its status. An interrupted session may show `+ unknown` duration. History lookup does not need a running exercise container.
3. Run `golf config`. Expected output includes the selected image and state path. The TUI remains available with a recovery warning when Docker cleanup fails, so exercises, progress, and settings can still be inspected.
4. Start the same Docker engine used for the attempt and run `golf play ATTEMPT`, replacing `ATTEMPT` with the history ID. Expected behavior: stop leftover containers belonging to unfinished local attempts, mark unclosed sessions interrupted, and resume the retained workspace. Execution retries failed recovery before launching or checking work.

Retain the state directory’s `workspaces` tree, Docker volumes, and recorded image IDs when migrating a machine. History alone cannot reconstruct edits. Native workspaces preserve saved edits after a crash. Use a new attempt to start again. Do not relabel another container or volume to bypass the ownership checks.

## Data handling

The state directory defaults to private permissions; the SQLite database and exports use mode `0600`. SQLite WAL sidecars belong to the same state directory. Close Golf before making a filesystem backup of that directory, and include all its files. Include the `workspaces` directory to preserve native edits. Back up Docker volumes separately to retain the original container workspaces. Migration backups include committed WAL data; the current schema is version 1.

JSON exports include attempt identity, environment IDs, sessions, and check details. CSV exports include attempt summaries and escape spreadsheet formula prefixes. Neither export includes complete workspaces or a raw command history. Review exported check details before sharing them if you typed sensitive data into an exercise.

`golf abandon ATTEMPT --yes` preserves records and work. `golf forget ATTEMPT --yes` permanently deletes a finished attempt, its owned native workspace, and its owned volume. There is no automatic data expiry or global Docker prune operation.

## Reporting

Include the source revision or `golf version`, host architecture, Docker version, exercise ID/revision, and a minimal synthetic reproduction. Do not include credentials, private workspaces, or personal history. For suspected host access, preserve logs and stop using the affected build. For a public release, use [GitHub private vulnerability reporting](https://github.com/stevencarpenter/driving-range/security/advisories/new) when the repository's Security tab exposes the Report a vulnerability button. This repository is currently private; GitHub documents that reporting feature for public repositories. The maintainer must enable it and verify receipt before public launch. If that channel is unavailable, contact the repository owner privately through an established channel; do not open a public issue with exploit details. No response-time or supported-release guarantee is established for this preview.
