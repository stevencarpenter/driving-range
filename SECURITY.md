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

Resource limits in this table apply to Docker execution, except the one-hour native session deadline and the snapshot limits. Native tools otherwise have normal host resource access. Docker enforcement depends on the engine and host kernel. Named volumes have no aggregate disk quota. A process can create many files before snapshot validation rejects the workspace, so limit Docker's backing storage when running untrusted experiments. Interactive child output passes through a virtual terminal emulator that parses bytes into cells, so escape sequences from the child cannot reach your terminal, and clipboard writes (OSC 52) are dropped. Output returning to the outer TUI is sanitized. Full-terminal handoff (`golf --classic` or the `golf play` command) instead sends interactive child output directly to your terminal, including terminal control sequences and possible clipboard operations.

`golf setup` performs networked image construction and runs package installation. It pins the Debian base digest, jj archive hashes, and selected security-fix package versions. Other Debian package versions are resolved at build time. Runtime rebuilds can produce different image IDs. `GOLF_IMAGE` may select arbitrary trusted local code; it is not an image signature verifier. No external pack installation is provided. Native practice is the default; Docker remains required for setup and validation.

`golf update` explicitly downloads and builds the application source selected by Go's `@latest` query. This prefers a stable release, then a prerelease when no stable release exists, then the untagged default branch when neither exists. It uses the installed Go toolchain and its module download configuration. The build runs on the host with normal permissions and requires network access. It stages the executable in the installation directory and atomically replaces the running installation after a successful build. It does not open application history or modify workspaces. Only run updates from an upstream you trust.

## Dependency scan

[The launch audit](docs/LAUNCH_AUDIT.md) owns the recorded image identities, scan results, package updates, and evidence scope. Package-presence findings do not establish reachability or exploitability. No scan establishes that the application or runtime is vulnerability-free.

Reproduce a current image scan with an installed Trivy binary:

```sh
trivy image --scanners vuln --format json --output trivy-runtime.json --timeout 5m driving-range-runtime:1
```

To reproduce a recorded development-image scan, select its image from the launch audit. A tag can identify a different image after rebuilding, and advisory data changes over time. Compare the report's image ID and scan timestamp before comparing counts. This preview does not claim a vulnerability-free image.

## Runtime recovery

### When to use this

Use these checks when Docker is unavailable, an exercise reports an infrastructure error, or a previous session ended unexpectedly.

1. Run `golf doctor`. Expected success: `Isolated Linux runtime is ready.` plus an environment ID. A missing daemon requires starting Docker; a missing runtime image requires `golf setup`. This checks daemon access and image presence, not container startup, isolation, or native tool availability. If `doctor` passes but container operations stall, diagnose the Docker engine; a ready image alone does not prove runtime health.
2. Run `golf history`. Expected output includes the attempt ID and its status. An interrupted session may show `+ unknown` duration. History lookup does not need a running exercise container.
3. Run `golf config`. Expected output includes the selected image and state path. The TUI remains available with a recovery warning when Docker cleanup fails, so exercises, progress, and settings can still be inspected.
4. Start the same Docker engine used for the attempt and run `golf play ATTEMPT`, replacing `ATTEMPT` with the history ID. Expected behavior: stop leftover containers belonging to unfinished local attempts, mark unclosed sessions interrupted, and resume the retained workspace. Execution retries failed recovery before launching or checking work.

Retain the state directory’s `workspaces` tree, Docker volumes, and recorded image IDs when migrating a machine. History alone cannot reconstruct edits. Native workspaces preserve saved edits after a crash. Use a new attempt to start again. Do not relabel another container or volume to bypass the ownership checks.

## Data handling

The state directory defaults to private permissions; the SQLite database and exports use mode `0600`. SQLite WAL sidecars belong to the same state directory. Close Golf before making a filesystem backup of that directory, and include all its files. Include the `workspaces` directory to preserve native edits. Back up Docker volumes separately to retain the original container workspaces. Migration backups include committed WAL data; the current schema is version 1.

JSON exports include attempt identity, environment IDs, sessions, and check details. CSV exports include attempt summaries and escape spreadsheet formula prefixes. Neither export includes complete workspaces or a raw command history collected by golf. Native shells and plugins retain their normal history and logging behavior; golf does not disable it. Review exported check details before sharing them if you typed sensitive data into an exercise.

`golf abandon ATTEMPT --yes` preserves records and work. `golf forget ATTEMPT --yes` permanently deletes a finished attempt, its owned native workspace, and its owned volume. There is no automatic data expiry or global Docker prune operation.

## Reporting

Include the source revision or `golf version`, host architecture, Docker version, exercise ID/revision, and a minimal synthetic reproduction. Do not include credentials, private workspaces, or personal history. For suspected host access, preserve logs and stop using the affected build. For a public release, use [GitHub private vulnerability reporting](https://github.com/stevencarpenter/driving-range/security/advisories/new) when the repository's Security tab exposes the Report a vulnerability button. The [recorded launch audit](docs/LAUNCH_AUDIT.md#repository-configuration-evidence) found the repository private and could not verify this channel. GitHub documents the feature for public repositories. The maintainer must enable it and verify receipt before public launch. If that channel is unavailable, contact the repository owner privately through an established channel; do not open a public issue with exploit details. No response-time or supported-release guarantee is established for this preview.
