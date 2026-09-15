# Terminal interface

The approved product plan supplies the interface hierarchy. The HTML review is a planning artifact, not a web product to reproduce.

Preserve the terminal's background. Charm Lip Gloss supplies shared visual roles across Today, Practice, Progress, Settings, exercise and attempt details, track selection, help, and confirmations. Keep exact theme colors in [styles.go](internal/tui/styles.go).

- Bold titles identify the current task. Blue headings and ASCII rules separate content sections.
- Green primary actions identify the next operation. Tinted rows and active tabs show selection, reinforced by `>` and brackets.
- Neutral secondary text distinguishes metadata from instructions. Literal table statuses receive green success, amber active/interrupted, or red failure accents when the row is not selected.

Use literal ASCII labels and explicit textual statuses. No essential information depends on color. Support `light`, `dark`, `auto`, and `plain` themes; `auto` follows the detected terminal background. Plain mode and `NO_COLOR` emit no styling escape sequences, including bold or background styling.

At 80x24, keep the title/navigation header and status/keyboard footer fixed around a bounded content viewport. Cap the layout at 120 terminal columns including margins. Reserve two columns per side at terminal widths of 60 or more, one at widths of 30 through 59, and none below 30. Collapse list columns and shorten navigation and keyboard help as width decreases. Lists scroll without losing selection. Long briefs, results, and help scroll within the viewport. Layout and content ordering live in [view.go](internal/tui/view.go).

Put the exercise title, primary action, goal, and brief before attempt bookkeeping. Keep revision IDs, elapsed time, assistance records, and runtime details secondary to the instruction they support.

All controls remain keyboard accessible without fonts or a mouse. Child tools own the entire terminal and every key during an exercise. Restore the TUI after they exit.

Today, Practice, Progress, and Settings are top-level destinations. Exercise details contain start/resume, check, hints, explanation, retry, and abandon. Destructive data removal needs explicit confirmation. Show errors with the operation and recovery action. Never claim a result was saved if persistence failed.
