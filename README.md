# Driving Range

Sharpen your terminal skills with short, practical challenges in your real editor and shell.

**Driving Range** is a local-first TUI named `golf`. Practice Vim, search, shell scripting, text processing, Git, jj, and Python. Correctness comes first; hints, explanations, retries, and history export require no account. Your tools and dotfiles stay yours.

**Current status: pre-release, 600 exercise drafts across 12 tracks.** The minimum is 365 practice challenges across the tools, not 365 per track. The authored bank is 600, with 100 each for Vim/Neovim, regex/search, Bash/core CLI, and awk. These are the priorities for repeated multi-exercise practice. Fundamentals and deliberate repetition are part of that bank. The year-long curriculum and daily schedule are not published yet. See [the curriculum requirements](docs/CURRICULUM.md) and [launch audit evidence](docs/LAUNCH_AUDIT.md).

[Install](#quick-start) · [Practice](#practice) · [Contribute](CONTRIBUTING.md) · [Author exercises](docs/AUTHORING.md) · [Security](SECURITY.md) · [Licenses](#contributing)

This is an independent application; it neither imports nor modifies `vim-golf` or its state.

## Quick start

From this checkout, with Go 1.27 or newer, `just`, your practice tools (including `nvim`, Bash, and zsh), and a running Docker-compatible Linux engine:

```sh
just build
./golf setup
./golf doctor
./golf
```

`setup` explicitly downloads the runtime's build inputs. Install and start your Docker engine first; on macOS it needs a Linux VM. `doctor` checks the daemon and the locally built image. The exercise catalog and Docker build context are embedded in the binary, so an installed binary also supports `setup`. Practice works offline after the image is built.

Practice launches your installed Neovim or an interactive shell with your normal environment. Exercises that teach the shell itself use that shell; otherwise your `SHELL` is preferred. Your dotfiles, Neovim plugins and keybindings, shell aliases, and tool configuration load normally. `HOME`, `XDG_*`, `NVIM_APPNAME`, and `ZDOTDIR` are inherited. Native practice has your normal host permissions and network access.

Docker prepares fixtures and checks results with Debian Linux tools. The base image digest and jj archive hashes are pinned; Debian packages resolve at build time. Each attempt records the checker image ID. Native tool versions and configuration are not pinned; host utilities can differ from the Linux checker, particularly on macOS.

## Practice

One exercise is a useful daily session. Stop after it, or continue in Practice for as long as you want; there are no daily locks or streak penalties. Completing catalog content early means a later recommendation may be review rather than an unseen challenge.

Choose a starting track on first launch. `Today`, `Practice`, `Progress`, and `Settings` are the main destinations. Use `Tab` or `1` through `4` to navigate, `j/k` or arrows to select, `Enter` to open, and `?` for help. Search the practice catalog with `/`, including tools, concepts, `difficulty:1`, `solved`, `missed`, or `untried`.

1. Open an exercise and read its success conditions.
2. Press `Enter` to launch its real editor or shell. It runs in a pane with the objective on a band above it, and keeps its normal keybindings.
3. Save and exit Neovim with `:wq`, or exit the exercise shell with `exit`. Golf checks the result and saves it.
4. Resume after a failed check, request a hint with `h`, or create a separate attempt with `r`.

`F12` is the only key golf takes while an exercise runs. It opens a palette: `c` check, `h` hint, `v` reveal, `b` collapse or expand the brief band, `q` quit, `Esc` back to the exercise. Every other keystroke reaches the child, including all Ctrl and Alt chords and the tmux prefix.

`golf-check` and `golf-hint` do the same from the exercise prompt, taking no keys at all. A check run this way reports on the status line and leaves the attempt open, so you can keep working; the attempt is finished by the check that runs when you exit.

`golf --classic` gives the exercise the whole terminal instead, which is the behaviour before the pane existed. Golf also falls back to it, and says so, if it cannot allocate a pseudo-terminal.

In shell exercises, `golf-brief` prints the active brief. Output exercises require a reusable submission in `solution.sh`, or `solution.py` for Python. Edit that file inside the exercise workspace, then exit. The checker replays the submission against multiple fixtures; commands typed only at the prompt are not an output submission. File-editing exercises check the resulting tree. Git and jj exercises check repository state.

`v` reveals the explanation and reference solution. Revealing an active attempt records assistance. External assistance can also be recorded with `x`; assistance remains recorded for that attempt. Finished attempts are immutable. Retrying creates a new attempt and preserves the prior result and workspace.

New practice and `golf list` offer the latest revision of each exercise. Saved attempts and published daily assignments retain their exact revision. See the [revision contract](docs/AUTHORING.md#content-contract) for authoring details.

### Included exercises

The embedded catalog contains **600 exercises across 12 tracks**:

| Tracks | Exercises | Daily assignments |
| --- | --- | --- |
| Vim | 100 | Ten-day preview assignments; additional practice |
| Search (`rg`, regex, `grep`) | 100 | Ten-day preview assignments; additional practice |
| Bash/core CLI | 100 | Ten-day preview assignments; additional practice |
| `awk` | 100 | Practice |
| `sed` | 25 | Practice |
| `fd` | 20 | Practice |
| `find` | 25 | Practice |
| Python | 35 | Practice |
| zsh | 20 | Practice |
| fzf | 10 | Practice |
| Git | 35 | Practice |
| jj | 30 | Practice |

Vim search/editing, regex, Bash/core CLI, and awk are the priorities. The added drills repeat forward/backward and word search, next-match editing, scoped substitution, captures, global deletion, regex boundaries and extraction, head/tail/cut/paste/sort/uniq/tr/comm/join/wc/tee, and awk fields, records, filters, grouped reports, ranges, and numbering. Repetition is intentional; Practice remains open for sessions of two to ten exercises or more. [Source research and rights](docs/SOURCES.md) are recorded separately from executable and editorial review.

The fixed ten-day preview runs **14 September through 23 September 2026, UTC**. An attempt keeps its assignment when a session crosses midnight. Outside those dates, the TUI labels its selection as practice, and `golf today` reports that no daily is published. The entire catalog remains playable. There is no rolling daily content service or automatic content download.

Exercises are original curriculum drafts. Executable audits check failing starter fixtures and passing reference solutions; they do not establish human editorial review or learning effectiveness. Players review the material as they practice and report unclear or unhelpful exercises. Human review of the whole bank is not a publication gate. zsh and fzf samples validate command behavior. The application does not measure Readline/ZLE key sequences, physical keystrokes, or personal skill retention. tmux is in the runtime, but has no dedicated exercise track.

### Plain commands

Run `golf help` for the complete command reference. Global flags precede the command.

```sh
golf --plain
golf --classic
golf list rg
golf show vim.change-value
golf today search 2026-09-14
golf play vim.change-value
```

`play` requires an interactive terminal. `list`, `show`, `today`, and the plain overview work without Docker or stored history. For the following commands, replace `ATTEMPT` with an ID from `golf history`:

```sh
golf history
golf play ATTEMPT
golf retry ATTEMPT
golf check ATTEMPT
golf hint ATTEMPT
```

Share text contains the exercise ID, assignment date, outcome, and assistance status, without the solution. Export JSON for attempts, sessions, and check events, or CSV for one summary row per attempt:

```sh
golf share ATTEMPT
golf export --format json --output history.json
golf export --format csv --output history.csv
```

Export refuses to overwrite an existing file. Omit `--output` to write to standard output. TUI exports appear in the state directory's `exports` folder.

## Local state and recovery

State defaults to `~/.local/state/driving-range` on both macOS and Linux. `XDG_STATE_HOME` changes the parent directory. `GOLF_STATE_DIR` overrides that location, and `golf --state-dir DIR` overrides the environment. `config.json` stores preferences; `driving-range.db` stores attempts, sessions, and checks. No telemetry or raw keystroke logs are collected.

Elapsed exercise time accumulates across foreground tool sessions, including thinking time. Setup, menu navigation, and validation are excluded. An unobserved session after a crash has unknown duration; the application does not invent missing time. Personal bests group attempts by exercise revision, seed, checker image, validator, profile, and assistance. They do not fingerprint your native tool versions or dotfiles, so configuration changes can affect comparisons.

Native workspaces live in `workspaces/ATTEMPT_WORKSPACE/files` under the state directory, separate from SQLite history. Saved edits stay there after exit or interruption. Initial fixtures also have attempt-owned Docker volumes. Returning from a native tool checks the local files, including additions and deletions, in fresh Docker containers. Startup stops leftover containers belonging to unfinished attempts before marking unclosed sessions interrupted. If Docker is unavailable, the TUI shows a recovery warning and still permits browsing exercises, progress, and settings. Execution retries cleanup before launching or checking work. Start Docker, then use `golf play ATTEMPT` to resume. Read-only history and export remain available while another process owns the writer lock.

An exercise session lasts at most one hour. Native editors and shells run on your host; they are not sandboxed. Fixture setup, reference audits, and validation containers have no network, host bind mounts, or Docker socket. They use a non-root user, a read-only root filesystem, dropped capabilities, and no privilege escalation. Docker resource limits apply to those containers. Local snapshots reject links and special files and enforce the same size limits as Docker snapshots. See [SECURITY.md](SECURITY.md) for boundaries and recovery checks.

Abandoning closes an attempt without erasing it. Deletion is separate and permanent:

```sh
golf abandon ATTEMPT --yes
golf forget ATTEMPT --yes
```

`forget` accepts only solved or abandoned attempts and removes that attempt's owned local workspace, Docker volume, and history. Do not use Docker volume pruning to reset a challenge. JSON/CSV exports preserve performance records, not workspace files, and the application has no export import command.

## Configuration

```sh
golf config track search
golf config theme plain
golf config
```

Themes are `auto`, `dark`, `light`, and `plain`. `NO_COLOR` disables accent colors. The TUI uses ASCII controls and supports an 80×24 terminal. `--plain` emits a linear overview; use the plain commands for redirected output or assistive technology.

`GOLF_IMAGE` selects a trusted local runtime image for development. It changes new attempts and runtime setup, not the image ID already recorded on an attempt. Docker is required for fixture setup and checking. Native practice uses the tools already installed on your host; missing tools produce an installation error.

## Installation and upgrades

From a source checkout, run the same command for a fresh installation or an upgrade:

```sh
just install
golf
```

`just install` builds the current checkout, creates `~/.local/bin` if needed, and atomically replaces `golf` there. It replaces any installed version without an uninstall step and preserves settings, attempts, and workspaces. Build or copy failures leave the installed binary intact. Set `GOLF_INSTALL_DIR` to select another installation directory. `just VERSION=v0.1.0 install` sets the binary's version label; it does not download or select that source version.

The installer reports when the destination is missing from PATH or another `golf` takes precedence. For the default destination, add `export PATH="$HOME/.local/bin:$PATH"` to your shell configuration if needed. Runtime preparation remains `golf setup` as described in Quick start.

Release packaging targets macOS arm64 and Linux amd64/arm64. `just VERSION=v0.1.0 release` builds archives and `SHA256SUMS` locally; it does not publish. No published release or signed artifact is assumed by these instructions. For a downloaded release, verify the selected archive against its checksum before extracting, then run `mkdir -p "$HOME/.local/bin"` and `install -m 755 golf "$HOME/.local/bin/golf"`. Use `shasum -a 256` on macOS or `sha256sum` on Linux. See [CONTRIBUTING.md](CONTRIBUTING.md#release-packaging) for optional provenance verification.

## Contributing

```sh
just check
just setup
just integration
just audit-solutions
```

See [CONTRIBUTING.md](CONTRIBUTING.md#checks) for each check's scope, prerequisites, shard selection, and release requirements.

Local history, replay, hints, and export are core features. Accounts, billing, hosted execution, and paid infrastructure are not implemented. See [the current launch requirements](docs/CURRICULUM.md) and [source policy](docs/SOURCES.md) before contributing exercises.

Application code, fixtures, and solution code use the [MIT license](LICENSE). Exercise prose uses [CC BY-SA 4.0](CONTENT_LICENSE.md). Bundled dependencies retain their [third-party notices](THIRD_PARTY_NOTICES.md).
