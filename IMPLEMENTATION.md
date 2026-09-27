# Implementation status

> **Historical snapshot (branch state 14 September 2026; marked historical 27 September 2026).** This file records pre-launch status and is not current. The GitHub repository now exists, CI runs green on `main`, and the product is implemented. See [README.md](README.md), [CONTRIBUTING.md](CONTRIBUTING.md), and [DESIGN.md](DESIGN.md) for current documentation.

Branch: `feat/driving-range`. Source project `vim-golf` is protected and remains independent.

The local alpha is implemented and verified on 14 September 2026. No GitHub repository, release, or hosted service has been created or changed.

1. Implemented: Go application, Bubble Tea TUI, plain CLI, Docker execution, and SQLite attempt/session/check history. The minimum toolchain is Go 1.26.8.
2. Implemented: 48 exercises across 12 tracks, with 30 daily assignments from 14 through 23 September 2026 UTC and practice outside that schedule. Hints, reveal, separate retries, resume, progress, comparable personal bests, and JSON/CSV export are available.
3. Implemented: ownership-checked Docker workspaces, bounded validation, crash recovery, terminal handoff, dependency diagnosis, and explicit per-attempt deletion. Runtime security findings and enforcement limits are recorded in [SECURITY.md](SECURITY.md).
4. Verified after review corrections: `go test -race ./...`, `go vet ./...`, catalog metadata, and real Docker integration tests with the race detector. All 48 unchanged starter submissions fail and all 48 reference solutions pass. The PTY smoke covers shell failure/resume/retry, durable history, SIGTERM persistence, terminal restoration, TUI resize, and real Neovim handoff. Regression checks cover preparation crashes, preserved workspace edits, frozen retry identity, daily selection, unavailable Docker, and asynchronous shutdown.
5. Implemented: manual installation, contributor and security references, application/content/dependency license notices, opt-in Docker CI, and release archive/optional attestation workflows. No release or attestation has been published.

The implementation covers the local product. Interviews, retention studies, human editorial approval, and paid pilots require actual participants and are not represented as completed software features.

Release archives for `v0.1.0-alpha.1` are built locally for macOS arm64 and Linux amd64/arm64 with matching checksums and dependency notices. Native macOS arm64 execution and Linux arm64 catalog execution were verified. Linux amd64 is crosscompiled; native interactive Linux host verification remains an acceptance check for a public release. CI definitions passed workflow validation but have not run on GitHub.

Source preservation was checked against the pre-implementation snapshot: all 135 non-Git files in `vim-golf` have identical SHA-256 hashes, with no additions or deletions. Its clean working tree and HEAD `c7f161029896e88ddd662ab1681ce930859255ff` are unchanged.
