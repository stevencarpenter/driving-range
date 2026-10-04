# Terminal interface

The implemented interface has four top-level destinations: Today, Practice, Progress, and Settings. Exercise and attempt details are subordinate screens. [PRODUCT_PLAN.md](PRODUCT_PLAN.md) and the HTML review are historical planning artifacts, not current interface specifications.

Charm Lip Gloss fills the terminal with a themed canvas and framed content surfaces. Today, Practice, Progress, Settings, exercise and attempt details, track selection, help, confirmations, results, and errors share these visual roles. Keep exact theme colors in [styles.go](internal/tui/styles.go).

- Today and exercise details place a prominent GOAL panel immediately after the title, before controls and metadata. Bold text, a teal surface, and a teal border emphasize the objective across the content pane.
- Purple identifies the brand, active navigation, selected rows, and section headings. Neutral text identifies inactive navigation and secondary metadata.
- Teal primary actions identify the next operation. Selected rows retain `>` and active tabs retain brackets so selection remains explicit without color.
- Rounded borders and tonal surfaces separate panels in colored themes. Literal table statuses receive teal success, amber active/interrupted, or red failure accents when the row is not selected.

Use explicit textual statuses. No essential information depends on color. Support `light`, `dark`, `auto`, and `plain` themes; `auto` follows the detected terminal background. Plain mode and `NO_COLOR` use ASCII borders, rules, and progress bars and emit no styling escape sequences, including bold or background styling. The GOAL label and frame remain visible without color.

Use the terminal's available width and height without a maximum column cap. Keep the brand/navigation header and status/keyboard footer fixed around a bounded Bubbles viewport, including at 80x24. Frame the content at widths of at least 60 columns and heights of at least 16 rows. Reserve two columns per side at terminal widths of 60 or more, one at widths of 30 through 59, and none below 30. Collapse list columns and shorten navigation and Bubbles keyboard help as width decreases. Lists scroll without losing selection. Long briefs, results, and help scroll within the viewport.

At widths of at least 110 columns and heights of at least 24 rows, add a sidebar one-third of the terminal width, capped at 42 columns, with a two-column gap. Practice, Progress, and attempt details preview the selected exercise's title and goal. Today and exercise details show session context; Settings shows display and local-data context. Hide the sidebar during help, track selection, confirmations, notices, and errors so the active content receives the full width. Layout lives in [layout.go](internal/tui/layout.go); content ordering lives in [view.go](internal/tui/view.go).

Sidebar track progress uses a Bubbles progress bar and a textual solved count. Count each catalog exercise revision once, regardless of repeated solved attempts. Progress summaries count attempts separately. Show the Bubbles spinner only while work is pending; plain mode uses a static `Working:` status.

Put the goal before the primary action and metadata. In exercise details, follow the controls with the brief, then exercise details and attempt bookkeeping. Omit the brief's opening paragraph only when it exactly repeats the displayed objective. Keep revision IDs, elapsed time, assistance records, and runtime details secondary to the instruction they support.

All controls remain keyboard accessible without special fonts or a mouse. Tab and number keys select top-level destinations; j/k or arrows move through lists or scroll text. Enter opens the selected exercise in Practice and the selected attempt detail in Progress. Strip external terminal control sequences before applying application styling.

An exercise runs inside a pane in the TUI. A brief band sits above it and a status line below, so the objective stays visible while the solution is worked. The band collapses to one line. Its height is measured after rendering, because a long objective wraps; the band, the pane, and the status line always sum to the terminal height. The band omits the brief's opening line when it repeats the objective.

With the pane focused, the child receives key presses except `F12`, which opens a palette offering check, hint, a reveal request, brief-band toggling, and quit. The palette consumes its next key. Key presses are forwarded as terminal sequences, not guaranteed byte-identical input; the host terminal or tmux can also intercept them. Golf does not reserve common editor or shell shortcuts. The `golf-check` and `golf-hint` commands reach the same actions from the exercise prompt without intercepting any key.

Action results appear on the status line, which currently displays only their first line. Multi-line hints therefore require the exercise detail screen after child exit. The pane's reveal request sets a confirmation that cannot currently be completed while the pane is active; reveal on the exercise detail screen instead. A check taken mid-session records history but never finishes the attempt.

`golf --classic` opens the TUI with full-terminal handoff, restoring the TUI after child exit. The CLI `golf play` path always uses full-terminal handoff, independently of that flag. The helper check/hint commands require the embedded pane and do not operate in full-terminal mode. The TUI attempts classic handoff when pane startup fails; its notice records the fallback reason.

Today, Practice, Progress, and Settings are top-level destinations. Exercise details contain start/resume, check, hints, explanation, retry, and abandon. Destructive data removal needs explicit confirmation. Show errors with the operation and recovery action. Never claim a result was saved if persistence failed.
