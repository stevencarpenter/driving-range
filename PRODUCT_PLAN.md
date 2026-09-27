# Driving Range

> **Historical planning document (proposal dated 14 September 2026; marked historical 27 September 2026).** This is a pre-implementation product proposal, not a current specification. Current user and contributor documentation lives in [README.md](README.md), [CONTRIBUTING.md](CONTRIBUTING.md), and [docs/AUTHORING.md](docs/AUTHORING.md).

Daily terminal practice for fluency that survives the next real task.

Product proposal, 14 September 2026. Naming assumption: **Driving Range** is the product, `driving-range` is the new repository, and `golf` is the proposed executable. The final name in the request takes precedence over the earlier platform name. Package, executable, domain, and trademark availability remain unverified.

**Recommendation:** launch an open-source TUI with a useful offline practice loop, local attempt history, and three focused tracks. Validate repeat use before introducing a subscription. Subscription revenue is plausible, but neither demand nor willingness to pay has been demonstrated for this product.

## 1. Product ethos

Driving Range gives terminal users one worthwhile exercise they can finish in three to seven minutes, a clear explanation of the result, and a record of their own progress.

1. **Practice real operations.** Edit a configuration, locate the right files, transform a stream, repair repository history, or recover a command. Use actual tools with their actual behavior.
2. **Teach judgment before compression.** Correctness comes first. Show an idiomatic solution and its tradeoffs. Shortest input is an optional game objective, never a universal measure of engineering skill.
3. **Make returning easy.** One default daily exercise, immediate resume, explicit completion, no backlog guilt, and no lost streak punishment. The player can stop after one useful repetition.
4. **Respect the terminal.** Keyboard operation, fast startup, readable text, native editor and shell behavior, and predictable process control. No browser is required to practice.
5. **Let the player own the record.** Local history and export are core features. Accounts, telemetry, hosted services, and payment are optional.

The desired shell ergonomics are concrete: fewer unnecessary mode changes, confident quoting, useful completion, effective history recall, precise selection, and recovery without rebuilding a command from scratch. Visual polish means disciplined alignment, strong focus indication, restrained color, immediate feedback, and readable diffs. Decorative animation and elaborate terminal dashboards do not substitute for these behaviors.

The product teaches Bash, zsh, Vim, and related tools as distinct environments. It does not declare one configuration, shell, or version-control system universally superior.

## 2. What the existing project contributes

The inspected checkout is `stevencarpenter/vim-golf` at commit `c7f161029896e88ddd662ab1681ce930859255ff`. Its working tree was clean at inspection. The Bash runner and README were read directly because graph lookup did not represent the extensionless runner reliably. This is a product assessment, not a full correctness audit.

| Verified current behavior | Driving Range decision |
| --- | --- |
| 31 exercises dated August 2026, progressing from individual mechanics to a multi-file capstone. (vim-golf README) | Preserve the progression and realistic fixtures. Separate exercise identity from publication date. |
| Start tree, expected tree, brief, and entrypoint; recursive diff determines success. (vim-golf runner) | Reuse this content pattern for editing exercises. Add different validators only when their tracks need them. |
| Work files and cumulative Neovim input logs survive reopening. The best raw-byte count is stored per day. (vim-golf runner) | Preserve resume. Introduce distinct attempts and sessions with durable history. |
| Reset removes that day's work, log, and best score. (vim-golf runner) | Retry creates a new attempt. Erasing history becomes a separate explicit operation. |
| Personal Neovim configuration, LazyVim/Yanky/Harpoon conventions, macOS clipboard, and tmux brief integration appear in the curriculum. (vim-golf README and capstone challenge) | Use the player’s installed Neovim and configuration for practice. Keep fixture setup and validation in Docker. |

The runner has no general per-attempt timing/history ledger. Its raw-input byte count is explicitly not a portable keystroke score. The existing vim-golf tests cover resumed input accumulation and persistent short briefs.

Hippo recalled the successful use of an explicit active-brief reference and earlier inline key references. Apply both lessons: make the active exercise explicit and keep help accessible. Its synthesis inferred built-in timing metrics from shell execution durations; the source does not support that inference. Cross-project recall found no directly applicable prior implementation of the proposed Go/Bubble Tea/SQLite platform.

**Repository boundary:** create the new implementation and adapted content in a separate repository. Do not rename, move, archive, branch, add a worktree to, or change the remote of `vim-golf`. Do not mutate its existing user state. This planning delivery creates only sibling planning artifacts; repository initialization and publication belong to implementation.

No tracked license file was found in the inspected repository. Confirm ownership and permission before copying material into a publicly licensed new project. Adapt configuration-specific briefs deliberately; preserve provenance. Do not import external game content merely because it is publicly readable.

## 3. Audience and market position

**Initial audience:** working developers, SREs, and platform engineers who already use a terminal but repeatedly look up operations or rely on a narrow set of commands. Their job to be done is: “Give me a short, realistic repetition that improves something I use at work, without asking me to start a course.”

**Secondary audience:** engineers changing tools, particularly adopting Vim, modern search utilities, or jj. A guided progression can address a specific transition without becoming a complete language curriculum.

Absolute beginners need more setup explanation and teaching than a daily puzzle provides. Hiring assessment needs integrity, accessibility, and validity work that personal practice does not establish. Neither is the initial positioning.

### Competitive evidence

| Product | Verified overlap | Implication |
| --- | --- | --- |
| [VimGolf](https://www.vimgolf.com/) | Local Vim challenges, input minimization, submissions, and a substantial published challenge archive. | Vim golf itself is established. Differentiate with daily instruction, practical tasks, and several tools. |
| [Command Challenge](https://jarv.org/posts/building-cmdchallenge/) | Shell commands solve small problems against an execution/checking system; its creator describes container execution and published challenge definitions. | Small shell puzzles and automated checking are established patterns. |
| [Exercism](https://exercism.org/) | Free practice and mentoring, community funding, language tracks, and a CLI-first local workflow. | Local practice is not unique. Broad language coverage would compete with a mature free alternative. |
| [SadServers](https://sadservers.com/pricing) | Real Linux/DevOps exercises, free and paid plans, progress tracking, and paid CLI/TUI access. Listed Pro pricing is $9/month or $72/year; Pro+ is $11/month or $88/year. | TUI access is already commercialized nearby. A daily micropractice habit must provide its own value. |
| [CodeCrafters](https://codecrafters.io/pricing) | Limited free content, paid membership, substantive programming challenges, and team plans with usage analytics. | Paid developer practice is a plausible category. Longer projects solve a different time-budget problem. |

These are adjacent products and pricing offers, not proof of their profitability or Driving Range's market size. Published user totals are not a reliable estimate of a paying addressable market.

**Positioning hypothesis:** short daily practice across editing, command discovery, shell interaction, and repository manipulation is useful enough to become a recurring professional habit. The strongest differentiation is the combination of editorial quality, low startup friction, practical transfer, and personal longitudinal evidence. A TUI alone is insufficient.

Borrow the daily cadence, bounded session, recognizable format, and spoiler-free sharing of newspaper games. Do not assume their audience size, subscription economics, or retention transfers to developer tools. Do not copy their branding or puzzle content.

## 4. The daily product loop

1. Run `golf`. Today is selected, with one exercise, its tool profile, estimated duration, and your latest result. First launch asks only for a starting track and checks prerequisites.
2. Read the brief. See the goal, sample input/output, permitted environment, success conditions, and an optional reference. The attempt timer starts when the workspace is ready and the player starts.
3. Work in the real tool. The TUI yields terminal ownership to Neovim or the exercise shell. A shell helper displays the current brief; returning to the TUI also exposes it. tmux integration is optional.
4. Check. Display success or a bounded, actionable difference. A failed check preserves the workspace and records the event. Offer a hint without resetting progress.
5. Finish or retry. Save the result, show one useful technique and comparison with a comparable prior attempt, then offer exit or another attempt. Completion is the primary action; endless play is not the default.

Each selected track has a shared daily assignment. The player chooses one primary track, so the home screen does not become a checklist of everything they did not finish. A weekly mixed-tool exercise becomes available after the relevant adapters exist.

**Calendar rules:** publish immutable schedule entries keyed by UTC date, track, exercise revision, and seed. Display the local reset time. Freeze the assignment when an attempt starts, including sessions that cross midnight. Keep prior days accessible. Offline users get the cached schedule; if it has expired, offer a clearly labeled practice exercise rather than pretending it is today's shared challenge. Corrections create a new revision and preserve old result context.

## 5. TUI surface

Five primary destinations are sufficient:

| Destination | Player actions |
| --- | --- |
| Today | Start the selected daily, resume an active attempt, switch track. |
| Practice | Browse by tool, concept, difficulty, or missed exercise; replay the archive. |
| Exercise | Read brief, start/resume, check, inspect diff, request hint, retry. |
| Progress | Review attempts, outcomes, time, assistance, comparable personal bests, and export. |
| Settings | Choose tool profile, check dependencies, control display, manage local data, optionally connect services later. |

Illustrative home screen, not an implemented interface:

```text
driving range                         Tue 15 Sep     LOCAL
Today   Practice   Progress   Settings

TODAY / SEARCH                         About 5 minutes
Find the active configurations
Select production configs; exclude generated and archived files.

Profile: standard / rg                 Ready
Your last search exercise: solved, 4m 12s, 1 hint

> Start today's exercise
  Resume: Vim / replace repeated values

Enter open    j/k or arrows move    / search    ? help    q quit
```

The shell and editor keep their own keybindings while active. Do not intercept common editing keys for application navigation. `Ctrl-C` reaches the child process; quitting the outer TUI must be a distinct lifecycle transition. Recover terminal state after errors, process exit, resize, and interruption.

Target 80×24 terminals with a single-column layout. Wider terminals may show brief and result panes. Support light/dark themes, `NO_COLOR`, ASCII alternatives, visible keyboard focus, and a linear output mode for assistive technology and redirected output. Do not require Nerd Fonts, mouse input, tmux, or animation. Hide the timer during work by default; elapsed time remains available afterward.

## 6. Feature scope and track expansion

### MVP: a complete small product

1. **Daily and practice catalog:** one selected daily, archive, resume, three tracks, and 30 reviewed exercises.
2. **Execution and checking:** real Neovim and Bash, deterministic fixtures, standardized execution profile, file-tree and stdout validators, bounded errors and diffs.
3. **Performance retention:** every attempt and check recorded locally, pass/fail/abandon/interruption states, elapsed duration, hints, retries, environment identity, and export.
4. **Learning feedback:** short references, graduated hints, an explanation after success or deliberate reveal, and personal comparison on equivalent exercises.
5. **Operational quality:** dependency diagnosis, safe workspace lifecycle, offline cached play, accessible terminal behavior, installable releases, and regression checks.

Thirty launch exercises means ten per track. This provides ten scheduled days per launch track, plus unlimited practice; it is not a month of fresh daily content for every track. Announce a ten-day preview first. Public daily service requires an ongoing editorial schedule and a buffer of at least 14 reviewed future assignments per active track.

### Editing and search tracks

| Track | Exercise examples | Validation and delivery |
| --- | --- | --- |
| Vim/Neovim | Text objects, search/change/repeat, registers, macros, multi-file edits. | Practice uses installed Neovim, personal plugins and keybindings, and exact tree checks. |
| Regex, rg, grep | Select matching log records; distinguish whole words, captures, inverse matches, and excluded paths. | MVP uses explicit regex dialect/tool versions and stdout checks, including additional fixtures for reusable command submissions. |
| fd, find | Locate files by path, type, depth, and metadata; handle whitespace and leading dashes. | Next search expansion. Validate path sets with explicit order/duplicate policy and deterministic fixture metadata. |
| sed, awk | Targeted substitutions, field extraction, grouping, and aggregation. | Next text-processing expansion. Test reusable commands on multiple fixtures; label GNU/BSD and awk dialect differences. |

### Interaction, repositories, and languages

| Track | Exercise examples | Validation and delivery |
| --- | --- | --- |
| Shell agility | Quote a difficult filename, redirect safely, edit a long command, recall a seeded history entry, use completion. | MVP Bash mechanics with final-state/output checking. History and line-editor drills initially record correctness and time, without claiming to verify the key sequence. |
| Shell ergonomics | Bash Readline, zsh ZLE, vi/emacs editing modes, fzf selection, tmux navigation, jobs and signals. | Expand after the daily loop works. State the shell/keymap explicitly; add key-level observation only where the learning objective requires it. |
| Git | Selectively stage changes, split a commit, fix a message, resolve a conflict, recover a local reference. | Repository-state adapter validates trees, index, ancestry, messages, and allowed refs in disposable repositories. |
| jj | Split/squash changes, edit descriptions, manipulate bookmarks, recover an operation. | Separate adapter and teaching model. Validate jj state, not Git command equivalence; pin a supported version. |
| Languages | Small Python transformations, then Go/Rust/JavaScript exercises driven by observed demand. | Later pinned runtimes and test-based validators. Focus on practical terminal tasks; defer broad algorithm catalogs and language pedagogy. |

### Explicitly outside MVP

No accounts, billing, cloud execution, global rankings, social feed, multiplayer, universal keylogger, plugin marketplace, AI tutor, custom terminal emulator, or full language-course system. These are expansion candidates only when evidence justifies their cost. Basic history, replay, hints, and export must not become paid upgrades.

## 7. Performance and learning model

**MVP retention means durable records of performance.** It does not prove retention of a learned skill. Measure those separately.

| Record | Required information |
| --- | --- |
| Attempt identity | Unique attempt ID, exercise ID/revision, assignment date, track, seed, profile and validator version. |
| Outcome | Active, solved, abandoned, interrupted, or infrastructure error; checks and their pass/fail results. |
| Timing | UTC timestamps and accumulated foreground exercise duration across sessions. Exclude setup and outer-menu time. |
| Assistance | Hint level, solution reveal, and self-reported external/AI assistance. |
| Comparable metrics | Tool and environment versions; optional input bytes or submission length with unit, collection method, and reliability. |

Use local SQLite with three small tables: attempts, sessions, and check events. Store a schema version and apply transactional migrations with a backup. Summaries and personal bests are queries, not separately mutable truth. A retry creates a new attempt. Resume adds a session to the existing attempt. Reopening a solved task for review cannot improve or overwrite its completed result.

Record the active attempt before launching a child. Persist each check when it happens. After an unclean exit, recover the workspace and mark the unfinished session interrupted. Unobserved crash time is unknown, not zero and not an invented duration. A crash must not erase prior completed results. One active runner per state directory is sufficient initially; refuse concurrent ownership clearly while allowing read-only progress views.

Store state under the platform's user state location, with XDG overrides, in a `driving-range` directory separate from `vim-golf`. Cache challenge packs separately. Export versioned JSON and tabular CSV. Raw command logs are opt-in, local, and outside default telemetry/export; never read the player's real shell history.

### Scoring rules

1. **Correctness is categorical.** Pass the declared validation contract, fail with actionable differences, or report an infrastructure problem. Do not count a missing runtime as a failed exercise.
2. **Time is contextual.** Foreground duration includes thinking time and can include time away from the keyboard. Label it elapsed exercise time, not active cognitive effort.
3. **Input metrics are specific.** Neovim log bytes, submitted command bytes, and physical keystrokes are different measurements. Never add them into a cross-tool score.
4. **Personal bests require matching conditions.** Compare the same revision, fixture/seed, assistance class, environment profile, and metric version. Treat personal configuration changes as a new comparison cohort.
5. **Optimization is optional.** Offer an explicit golf replay after a correct solution. Author-provided targets are reference results, not proven optima. Efficient, readable solutions deserve explanation even when longer.

Show first-pass success, recent outcomes, hints used, and same-exercise improvement. A lower replay time may mean memorizing the fixture. To investigate skill retention, later present an unseen variant after approximately 1, 7, and 21 days and record unaided correctness. This schedule is a starting hypothesis, not a validated learning algorithm. Do not report a mastery percentage or certification from a few attempts.

## 8. Challenge authoring and editorial operations

Keep each exercise reviewable as files: a small manifest, Markdown brief and hints, start fixtures, expected results or tests, and an explanation/reference solution. Use stable IDs such as `search.exclude-generated`; store scheduling separately. Start with JSON manifests to avoid a new configuration parser in the runner.

A manifest declares its objective, prerequisites, difficulty, estimated duration, supported profile, tool versions/dialect, validator kind, output normalization rules, resource limits, revision, author, and license. Do not add a scripting language or external adapter interface before there is a concrete need.

Publication requires:

1. One precise learning objective and at least one independently executed reference solution.
2. A validator that rejects unchanged input and representative wrong answers, accepts documented alternatives, and handles specified edge cases.
3. Explicit output semantics: exact bytes versus sets, ordering, duplicates, trailing newline, encoding, and permitted extra files.
4. A brief, hint sequence, and solution explanation reviewed by another person; runtime, difficulty, and platform labels verified.
5. Clean provenance, bounded resource use, and a schedule entry referring to immutable content.

Do not parse shell commands to prove that the player used the advertised technique. Aliases, shell syntax, subprocesses, and equivalent solutions make that brittle. Constrain the standard environment where needed, otherwise label the technique objective as self-reported. A true method-enforced challenge needs dedicated observation and its own acceptance tests.

A maintainer owns releases and safety. A track editor owns curriculum, dialect accuracy, hints, and its publication buffer. Contributors submit small challenge PRs. Maintain an issue template for incorrect validators and unfair assumptions. Suspend a broken daily, preserve its attempt records, and provide a replacement without penalizing the player.

Planning assumption: 60 to 120 minutes to author and review a small challenge once tooling is mature. One new challenge daily implies 7 to 14 editorial hours per week. Three independent daily tracks imply 21 to 42 hours. This is a major operating cost. Use clearly labeled reviewed variants and archive replay, and measure authoring time before promising unlimited fresh tracks. AI-generated drafts require the same human review and executable validation.

## 9. Technical approach and execution boundary

**Proposed stack:** Go, Bubble Tea, local SQLite, and versioned filesystem challenge packs. Go is a recommendation for a distributable command-line application; the existing Bash runner establishes no language reuse requirement. Bubble Tea already provides child-process handoff through `ExecProcess`, including an editor example. Validate the API against the pinned release during the implementation spike. [Bubble Tea source](https://github.com/charmbracelet/bubbletea/blob/main/exec.go)

Use one executable, one local database, and direct runner functions selected by validator type. Initially there are two validator implementations, file tree and stdout. A repository validator is added when Git ships. Select one maintained SQLite driver during the spike after packaging and license checks. No ORM, service mesh, event bus, plugin ABI, or remotely hosted scheduler is required.

The TUI selects content and owns lifecycle state. The runner prepares a workspace and launches a real process. The validator reads only declared artifacts and returns a structured outcome. SQLite records attempts and check events. The catalog is bundled or explicitly updated from a versioned release manifest. This division supports new tools without making every tool a separate application.

### Native practice and isolated checks

**Validation runtime:** run fixture setup, reference solutions, and checks inside a pinned Linux container image. Use a non-root user, disabled network, dropped capabilities, no privilege escalation, read-only base filesystem, writable disposable workspace, and CPU/memory/process/time limits. Never mount the user's home, SSH agent, Docker socket, real repositories, or credentials into an exercise. Keep application state outside the container. Docker documents the controls; their combined behavior needs verification on supported hosts. [Runtime controls](https://docs.docker.com/engine/containers/run/), [network isolation](https://docs.docker.com/engine/network/drivers/none/)

**Native practice:** launch installed Neovim, Bash, or zsh with the player’s normal environment and dotfiles. Keep saved edits in an owned directory under application state across exit and interruption. Native tools, plugins, hooks, and shell configuration have normal host access. Docker remains required for setup and validation. Checker image IDs do not fingerprint native tool versions or personal configuration.

Container isolation reduces risk but is not a guarantee against hostile workloads or kernel vulnerabilities. Accept only curated challenge packs initially. Do not execute user-contributed setup scripts on the host. Verify downloaded manifests and immutable hashes, prevent path traversal and symlink escapes during extraction, and never auto-install missing tools without a deliberate user action. [Docker security model](https://docs.docker.com/engine/security/)

For Git and jj exercises, create synthetic repositories in Docker and export bounded snapshots for native practice. Disable inherited global configuration, hooks, signing, credential helpers, and network remotes during setup and validation. Native Git and jj use the player’s normal host configuration. Validate semantic repository state rather than exact commit hashes containing timestamps. Tests run against controlled files and configuration.

Reusable shell submissions run against several fixtures to discourage hardcoded output. Free-form workspace exercises validate the resulting state. Hidden tests shipped locally are inspectable; neither mode supports a claim of adversarial integrity. Sanitize untrusted terminal output when presenting it in the outer TUI.

### Platform and distribution contract

Initially support macOS arm64 and Linux amd64, with Linux arm64 added when the same image and process checks pass. Standard exercise behavior is Linux behavior even on macOS. Native BSD-tool exercises require their own later profile. Windows users can be evaluated through WSL2 later; no native Windows promise in MVP.

Ship signed/checksummed release binaries and documented manual installation; add Homebrew after release automation works. Practice happens entirely in the terminal. Initial runtime/image installation may require network and significant download time; report the measured size before fetching. Offline claims apply after required images and packs are cached. Aim for under 300 ms to render the cached catalog and under two seconds to start a warm exercise, excluding cold image preparation. These are acceptance targets, not measured performance.

## 10. OSS and subscription model

**Recommended launch model: a complete OSS local product with optional sponsorship.** This matches the initial audience and keeps the main loop viable if a hosted business does not develop.

Propose MIT for application code and CC BY-SA 4.0 for public instructional content, with code fixtures/examples licensed explicitly as code. MIT permits commercial reuse with its notice requirements. CC BY-SA permits commercial reuse and requires attribution/share-alike for adaptations. This supports an open curriculum; it does not create exclusive paid content rights. Review inherited and third-party rights before release. [MIT terms](https://opensource.org/license/mit), [CC BY-SA terms](https://creativecommons.org/licenses/by-sa/4.0/)

| Model | What remains available locally | Paid value | Assessment |
| --- | --- | --- | --- |
| OSS plus sponsorship | Core TUI, public dailies/archive, hints, complete local history, export, authoring format. | Support continued maintenance and editorial work. | Recommended launch. Revenue may be small and irregular. |
| Optional individual membership | The same complete local product. | Managed cross-device sync/backup and a dependable guided editorial program; optional original specialty packs with clearly separate terms. | Test after repeat use. Sync alone may not justify a subscription. |
| Team subscription | Personal practice remains independent. | Private exercise distribution, curated onboarding programs, cohort scheduling, and consent-based progress summaries. | Later paid pilot. More direct budget owner, but higher sales/support/privacy cost. |

Do not paywall the TUI, local history, correctness checking, replay, export, or public community contributions. Do not relabel proprietary content as OSS. A downloaded paid pack remains usable after cancellation; payment buys ongoing service and delivery. Publish cancellation, cached entitlement, export, and data deletion behavior before charging. Practice remains local if authentication or billing services are unavailable.

**Price experiments, not forecasts:** test an individual offer at $6/month or $60/year, and a facilitated team pilot at $150/month for up to ten consenting participants. Use actual purchases to establish value. SadServers' nearby paid offer makes price sensitivity worth testing; it does not validate either proposed price.

Illustrative gross revenue: 250 subscribers at $6/month produce $1,500/month; 1,000 produce $6,000/month. These figures exclude annual discounts, fees, taxes, churn, support, content labor, and infrastructure. If 1,000 monthly active users converted at an assumed 5%, revenue would be $300/month at that price. Neither conversion nor audience size is established. Calculate contribution margin from measured costs before expanding the service.

Subscription account linking, checkout, and cancellation may require an external browser or device-code flow. That is a stated account-management exception, not a browser exercise experience. Do not collect payment credentials in the TUI. A paid service is a separate implementation decision after validation.

## 11. Validation and distribution

The first test is whether people return for useful practice, not whether they star a repository.

1. **Problem interviews:** recruit 12 terminal users across development and SRE/platform work. Ask for the last command they repeatedly looked up, an abandoned learning routine, and the configuration they actually use. Treat positive concept reactions as weak evidence.
2. **Observed first use:** give 8 participants an installation and one exercise. Record time to successful start, setup failures, misunderstood instructions, and whether they can explain the learned operation.
3. **Four-week pilot:** recruit 30 to 50 participants with explicit opt-in measurement. Use the 30-exercise bank with rotation and labeled variants. Observe repeat use before investing in fresh content for every track daily.
4. **Transfer check:** give participants an unseen related task after a delay. Compare unaided correctness with their initial attempt; distinguish learning from fixture memorization.
5. **Paid experiment:** offer a concrete guided program to active repeat users. Seek ten individual purchases or three paid team pilots before building ongoing subscription infrastructure.

Proposed pilot gates, chosen for decisions rather than asserted as industry benchmarks:

| Metric | Definition | Initial gate |
| --- | --- | --- |
| Activation | Installed participant completes one exercise within 24 hours. | At least 70%; inspect installation failures separately. |
| First-session friction | Time from launching an installed app to starting a ready exercise. | Median below 60 seconds; cold dependency setup measured separately. |
| Repeat use | Activated participant completes exercises on at least three distinct days in week two. | At least 40%. |
| Week-four retention | Activated participant completes an exercise during days 22 through 28. | At least 30%. |
| Exercise quality | Eligible checked attempts affected by confirmed validator/content defects. | Below 2%, with zero known host-data-loss defects. |

Small cohorts produce noisy percentages. Publish counts and denominators, follow up with non-returners, and repeat before calling product-market fit. Never calculate retention only from people who voluntarily export their most successful sessions.

Local operation must not require analytics. Pilot participants can explicitly share a minimal event summary: anonymous study ID, exercise/revision, lifecycle timestamps, outcome, and hint level. No command text, filenames from real projects, dotfiles, or machine identifiers. Team summaries must be visible to the participant and cannot imply surveillance or hiring certification.

Distribution starts with a short terminal recording that demonstrates one useful operation, an install command, and a ten-day preview. Seek feedback in Vim, shell, and SRE communities according to their posting rules. Publish explanatory solutions and reproducible challenge contributions. Offer optional spoiler-free text sharing from the TUI. Delay paid acquisition until activation and retention hold.

## 12. Delivery sequence and acceptance gates

Estimates assume one experienced engineer working full time, a part-time content reviewer, and no hosted service in MVP. They are planning ranges. The execution spike determines the revised estimate.

| Stage | Estimate | Deliverable and exit condition |
| --- | --- | --- |
| 1. Product and execution spike | 1 week | Interview evidence, new repository, one Neovim and one Bash exercise launched and checked from the TUI on both target hosts. Demonstrate interruption recovery, terminal restoration, isolation checks, and a saved attempt. |
| 2. Vertical slice | 2 weeks | Today → brief → real tool → check → saved result → restart/resume. Ten reviewed exercises, two validators, SQLite history, JSON export. Retry demonstrably preserves prior results. |
| 3. MVP hardening and content | 2 to 3 weeks | Three launch tracks, 30 reviewed exercises, offline schedule behavior, dependency diagnosis, accessibility/resize checks, packaging, and CSV export. Pass the acceptance checks below. |
| 4. Observed pilot | 4 calendar weeks | 30 to 50 participants, counted activation/repeat-use results, defect triage, and unseen-task transfer observations. Revise the product using observed failures. |
| 5. Expansion decision | 1 week, then scoped increments | Choose the next track from usage/interviews. Search/text-processing additions are estimated at 1 to 2 weeks; Git/jj adapters at 2 to 4 weeks each; the first language adapter at 2 to 3 weeks, including initial content. |

A private alpha is approximately 5 to 6 engineering weeks from implementation start. The first product decision with four weeks of use is approximately 10 to 11 calendar weeks. Subscription work and ongoing editorial labor are additional. Do not commit to daily fresh content across every named tool during this period.

### MVP acceptance checks

1. **Lifecycle and data:** pass, failed check, hint, abandon, retry, resume, and interrupted process all produce correct records. Restart preserves completed history. Retry leaves the prior attempt intact. Export reflects the database.
2. **Execution boundary:** host canary files remain untouched; exercise processes cannot access host credentials or network; runaway processes hit resource limits. Pack extraction rejects traversal and symlink escape cases. Run these checks in a disposable test environment.
3. **Validation:** reference solutions pass; unchanged and representative incorrect results fail; extra files, ordering, newline, Unicode, filenames with spaces, and unsupported tool profiles behave according to the manifest.
4. **Terminal and portability:** both target hosts handle resize, `Ctrl-C`, child failure, suspend/resume, plain output, and missing runtime without corrupting terminal state or recording false exercise failures.
5. **Daily operation:** cached play works offline; midnight cannot replace an active assignment; outdated schedules offer labeled practice; revised challenges retain distinct result history; future published assignments remain immutable.

Run focused integration checks around these contracts. Content validation must run for every published exercise. The old `vim-golf` tests are useful behavioral references, but the new runner needs its own checks and must not execute old reset operations against existing user state.

## 13. Limits and decisions that can change the plan

### Product and market limits

| Limit | Consequence and current decision |
| --- | --- |
| TUI narrows the audience and container setup adds friction. | Target existing terminal users; measure first use before adding more tracks. |
| New daily content has continuing editorial cost. | Begin with three tracks and a bounded preview; fund and measure the publishing cadence. |
| More practice does not automatically prove skill transfer. | Record performance honestly and test unseen variants; avoid mastery/certification claims. |
| Shortest commands can reward unreadable or unsafe habits. | Default to correctness and explanation; keep optimization an explicit mode. |
| Adjacent free and paid products already exist. | Validate habitual use and purchases; maintain a useful OSS product if subscriptions fail. |

### Technical and trust limits

| Limit | Consequence and current decision |
| --- | --- |
| Arbitrary shell execution is materially riskier than editing copied files. | Native shell practice has normal host permissions. Setup and validation remain isolated in Docker. A copied directory is not a sandbox. |
| Platforms, regex dialects, shell modes, tools, and personal configurations differ. | Pin standard profiles and split comparison cohorts. Do not promise universal score portability. |
| Local state and test fixtures can be inspected or changed. | Results are personal practice records. Defer rankings and assessment integrity. |
| Global input recording is invasive and technically ambiguous. | Record lifecycle data by default; collect narrowly scoped metrics only with explicit opt-in and honest units. |
| Repo/package naming, content rights, and platform packaging are unverified. | Resolve during the new-repository spike before public distribution; preserve the existing repository. |

**First implementation action:** create the separate repository and prove one terminal handoff, one isolated shell exercise, and one durable attempt record. That experiment tests the highest-impact feasibility assumptions before a wider platform is built.
