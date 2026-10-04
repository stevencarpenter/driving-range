# Driving Range

## Platform

Terminal application (TUI), launched as `golf` on macOS and Linux.

## Purpose

Short practical exercises for developers who want better command-line fluency. Use actual tools, validate results, and retain performance locally. Correctness is the result criterion; input optimization is a personal practice choice, not a measured score.

## Confirmed scope

The local product includes 600 exercises across 12 tracks, native practice, isolated checks, and durable history. Only the ten-day September 2026 preview has published daily assignments. A year-long mixed-tool schedule remains a [release requirement](docs/CURRICULUM.md), not shipped behavior. [PRODUCT_PLAN.md](PRODUCT_PLAN.md) records the historical proposal.

Preserve `vim-golf` as an independent project. Do not claim market validation, payment demand, completed human editorial review, or measured learning effectiveness.

## Stack

Go, Bubble Tea, SQLite, and an embedded versioned catalog. Installed host tools run interactive practice with normal permissions and dotfiles; Docker isolates fixture setup, reference audits, and validation. No browser is required for practice.

## Interaction

Today shows a published assignment for the selected track when one exists, otherwise its first current exercise labeled as practice. Practice catalog, exercise brief, progress, settings. Keyboard navigation and native child-tool keybindings. Stable resume and new attempts on retry. Clear errors, no streak punishment, no required telemetry or account.

## Accessibility

Support 80×24 terminals, light/dark themes, NO_COLOR, ASCII text, plain command output, and terminal restoration after child exit.
