# Terminal interface

The approved product plan supplies the interface hierarchy. The HTML review is a planning artifact, not a web product to reproduce.

Use the terminal's background and a restrained optional green accent for the selected action and successful checks. Use literal ASCII labels and explicit textual statuses. No essential information depends on color. Themes adapt accent/secondary text to light and dark backgrounds; plain mode emits no escape sequences.

At 80×24, prioritize title/navigation, the current task, a bounded content viewport, status, and keyboard help. Lists scroll without losing selection. Long briefs and results scroll within the viewport. Child tools own the entire terminal during an exercise. Restore TUI after they exit.

Today, Practice, Progress, and Settings are top-level destinations. Exercise details contain start/resume, check, hints, explanation, retry, and abandon. Destructive data removal needs explicit confirmation. Show errors with the operation and recovery action. Never claim a result was saved if persistence failed.
