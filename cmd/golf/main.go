package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"

	"github.com/stevencarpenter/driving-range/internal/app"
	"github.com/stevencarpenter/driving-range/internal/catalog"
	"github.com/stevencarpenter/driving-range/internal/model"
	"github.com/stevencarpenter/driving-range/internal/runner"
	"github.com/stevencarpenter/driving-range/internal/tui"
)

var version = "dev"

func buildVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "golf:", safe(err.Error()))
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("golf", flag.ContinueOnError)
	flags.SetOutput(out)
	state := flags.String("state-dir", "", "separate state directory (or GOLF_STATE_DIR)")
	plain := flags.Bool("plain", false, "print a linear overview instead of opening the TUI")
	classic := flags.Bool("classic", false, "give the exercise the whole terminal instead of embedding it in a pane")
	ver := flags.Bool("version", false, "print version")
	flags.Usage = func() { usage(out) }
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *ver {
		fmt.Fprintln(out, "golf", buildVersion())
		return nil
	}
	args = flags.Args()
	command := ""
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}
	if command == "update" {
		if len(args) != 0 {
			return errors.New("usage: golf update")
		}
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate installed golf: %w", err)
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return updateExecutable(ctx, executable, out)
	}
	cat, err := catalog.Load()
	if err != nil {
		return err
	}
	cfg := app.Config{Theme: "auto", Image: runner.DefaultImage}
	configDir := ""
	switch command {
	case "today", "doctor", "setup", "audit", "config":
		cfg, configDir, err = app.ReadConfig(*state)
		if err != nil {
			return err
		}
	}
	switch command {
	case "help":
		usage(out)
		return nil
	case "version":
		fmt.Fprintln(out, "golf", buildVersion())
		return nil
	case "list":
		filter := strings.ToLower(strings.Join(args, " "))
		for _, ch := range cat.Current() {
			hay := strings.ToLower(ch.ID + " " + ch.Track + " " + ch.Title + " " + strings.Join(ch.Tools, " ") + " " + strings.Join(ch.Concepts, " "))
			if strings.Contains(hay, filter) {
				fmt.Fprintf(out, "%-32s %-10s %dm  %s\n", ch.ID, ch.Track, ch.Minutes, safe(ch.Title))
			}
		}
		return nil
	case "show":
		if len(args) != 1 {
			return errors.New("usage: golf show EXERCISE")
		}
		ch, e := cat.Find(args[0], 0)
		if e != nil {
			return e
		}
		printBrief(out, ch)
		return nil
	case "today":
		track := cfg.Track
		if track == "" {
			track = "vim"
		}
		if len(args) > 0 {
			track = args[0]
		}
		now := time.Now()
		if len(args) > 1 {
			now, err = time.Parse("2006-01-02", args[1])
			if err != nil {
				return err
			}
		}
		if len(args) > 2 {
			return errors.New("usage: golf today [TRACK] [YYYY-MM-DD]")
		}
		if !slices.Contains(cat.Tracks(), track) {
			return fmt.Errorf("unknown track %q; available: %s", track, strings.Join(cat.Tracks(), ", "))
		}
		assignment, ok := cat.Today(now, track)
		if !ok {
			fmt.Fprintf(out, "No published daily for %s on %s (UTC). Practice is available with golf list %s.\n", track, now.UTC().Format("2006-01-02"), track)
			return nil
		}
		ch, e := cat.Resolve(assignment)
		if e != nil {
			return e
		}
		fmt.Fprintf(out, "Daily %s UTC | %s | revision %d\n", assignment.Date, assignment.Track, assignment.Revision)
		printBrief(out, ch)
		return nil
	case "doctor":
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		report, e := runner.New(cfg.Image).Doctor(ctx)
		fmt.Fprintln(out, safe(report.Message))
		if report.ImageID != "" {
			fmt.Fprintln(out, "Environment:", report.ImageID)
		}
		if e != nil {
			return e
		}
		if !report.Available {
			return errors.New("runtime is not ready; run golf setup")
		}
		return nil
	case "setup":
		if len(args) > 0 {
			return errors.New("usage: golf setup")
		}
		fmt.Fprintln(out, "Building the isolated Linux runtime. This explicitly downloads pinned build inputs; fixture setup and validation will run without network access.")
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return runner.New(cfg.Image).BuildImage(ctx, out)
	case "audit":
		sub := flag.NewFlagSet("audit", flag.ContinueOnError)
		sub.SetOutput(out)
		solutions := sub.Bool("solutions", false, "execute reference solutions in Docker")
		filter := sub.String("track", "", "audit only this track")
		if err = sub.Parse(args); err != nil {
			return err
		}
		if len(sub.Args()) > 0 {
			return errors.New("usage: golf audit [--solutions] [--track TRACK]")
		}
		if err = cat.Validate(); err != nil {
			return err
		}
		fmt.Fprintf(out, "Catalog metadata valid: %d exercises, %d revisions.\n", len(cat.Current()), len(cat.All()))
		if !*solutions {
			return nil
		}
		r := runner.New(cfg.Image)
		failures := 0
		count := 0
		for _, ch := range cat.All() {
			if *filter != "" && ch.Track != *filter {
				continue
			}
			count++
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			result, e := r.Audit(ctx, ch)
			cancel()
			fmt.Fprintf(out, "%s %s revision %d: %s\n", safe(result.Outcome), ch.ID, ch.Revision, safe(result.Summary))
			if e != nil {
				fmt.Fprintln(out, safe(e.Error()))
			}
			if e != nil || result.Outcome != "pass" {
				failures++
				for _, detail := range result.Details {
					fmt.Fprintln(out, safe(detail))
				}
			}
		}
		if count == 0 {
			return errors.New("no exercises match this track")
		}
		if failures > 0 {
			return fmt.Errorf("%d of %d reference checks failed", failures, count)
		}
		fmt.Fprintf(out, "All %d reference solutions passed.\n", count)
		return nil
	case "config":
		if len(args) == 0 {
			fmt.Fprintf(out, "track=%s\ntheme=%s\nimage=%s\nstate=%s\n", cfg.Track, cfg.Theme, cfg.Image, configDir)
			return nil
		}
		if len(args) != 2 {
			return errors.New("usage: golf config [track|theme VALUE]")
		}
		switch args[0] {
		case "track":
			cfg.Track = args[1]
		case "theme":
			cfg.Theme = args[1]
		default:
			return errors.New("config supports track or theme")
		}
		if err = app.WriteConfig(configDir, cfg); err != nil {
			return err
		}
		fmt.Fprintln(out, "Setting saved.")
		return nil
	case "":
		if *plain || !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
			return overview(out, cat)
		}
		s, e := app.Open(*state, false)
		if e != nil {
			return e
		}
		defer s.Close()
		if *classic {
			s.Config.Classic = true
		}
		return tui.Run(s)
	}
	switch command {
	case "play", "retry", "check", "hint", "reveal", "abandon", "history", "share", "export", "forget":
	default:
		return fmt.Errorf("unknown command %q; run golf help", command)
	}
	readOnly := command == "history" || command == "export" || command == "share"
	s, err := app.Open(*state, readOnly)
	if err != nil {
		if readOnly && errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(out, "No attempt history yet. Start with golf.")
			return nil
		}
		return err
	}
	defer s.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	switch command {
	case "play", "retry":
		if len(args) != 1 {
			return fmt.Errorf("usage: golf %s EXERCISE_OR_ATTEMPT", command)
		}
		if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
			return errors.New("play needs an interactive terminal; use show, list, history, or check for plain output")
		}
		terminalState, err := term.GetState(os.Stdin.Fd())
		if err != nil {
			return fmt.Errorf("save terminal mode: %w", err)
		}
		var a model.Attempt
		var assignment *model.Assignment
		id := args[0]
		if id == "today" {
			track := s.Config.Track
			if track == "" {
				track = "vim"
			}
			v, ok := s.Catalog.Today(time.Now(), track)
			if !ok {
				return errors.New("daily schedule has expired; select an exercise with golf list")
			}
			assignment = &v
			id = v.ExerciseID
		}
		if assignment != nil {
			attempts, e := s.Store.Attempts()
			if e != nil {
				return e
			}
			a = latestAssignment(attempts, *assignment)
		} else if prior, e := s.Store.Attempt(id); e == nil {
			a = prior
			id = prior.ExerciseID
		} else {
			last, e := s.Latest(id)
			if e != nil {
				return e
			}
			if last != nil {
				a = *last
			}
		}
		if command == "retry" || a.ID == "" || a.Status == "solved" || a.Status == "abandoned" {
			retryOf := ""
			if command == "retry" {
				retryOf = a.ID
			}
			a, err = s.NewAttempt(ctx, id, assignment, retryOf)
			if err != nil {
				return err
			}
		}
		ch, e := s.Catalog.Find(a.ExerciseID, a.Revision)
		if e != nil {
			return e
		}
		printBrief(out, ch)
		fmt.Fprintln(out, "Attempt:", a.ID)
		play, e := s.Prepare(ctx, a.ID)
		if e != nil {
			return e
		}
		cmd := play.Command()
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		childErr := cmd.Run()
		// A canceled child process can exit before restoring its terminal.
		restoreErr := term.Restore(os.Stdin.Fd(), terminalState)
		result, e := s.Finish(ctx, play, childErr)
		printResult(out, result)
		if e != nil || restoreErr != nil {
			return errors.Join(e, restoreErr)
		}
		if result.Outcome != "pass" {
			return errors.New("workspace preserved; resume with golf play " + a.ID)
		}
		return nil
	case "check":
		if len(args) != 1 {
			return errors.New("usage: golf check ATTEMPT")
		}
		result, e := s.Check(ctx, args[0])
		printResult(out, result)
		if e != nil {
			return e
		}
		if result.Outcome != "pass" {
			return errors.New("exercise is not yet correct; workspace preserved")
		}
		return nil
	case "hint":
		if len(args) != 1 {
			return errors.New("usage: golf hint ATTEMPT")
		}
		v, e := s.Hint(args[0])
		if e == nil {
			fmt.Fprintln(out, safe(v))
		}
		return e
	case "reveal":
		if len(args) != 2 || args[1] != "--yes" {
			return errors.New("revealing marks an active attempt as assisted; use golf reveal ATTEMPT --yes")
		}
		v, e := s.Reveal(args[0])
		if e == nil {
			fmt.Fprintln(out, safe(v))
		}
		return e
	case "abandon":
		if len(args) != 2 || args[1] != "--yes" {
			return errors.New("abandon closes the attempt and preserves its history; use golf abandon ATTEMPT --yes")
		}
		if e := s.Abandon(args[0]); e != nil {
			return e
		}
		fmt.Fprintln(out, "Attempt abandoned; history and workspace preserved.")
		return nil
	case "history":
		progress, e := s.Store.Progress()
		if e != nil {
			return e
		}
		if len(progress) == 0 {
			fmt.Fprintln(out, "No attempt history yet.")
		}
		for _, p := range progress {
			duration := fmt.Sprintf("%.1fs", float64(p.DurationMS)/1000)
			if p.UnknownDuration {
				duration += " + unknown"
			}
			fmt.Fprintf(out, "%s  %-28s %-20s %s  hints:%d\n", p.Attempt.ID, p.Attempt.ExerciseID, p.Attempt.Status, duration, p.Attempt.HintLevel)
		}
		return nil
	case "share":
		if len(args) != 1 {
			return errors.New("usage: golf share ATTEMPT")
		}
		v, e := s.Share(args[0])
		if e == nil {
			fmt.Fprintln(out, safe(v))
		}
		return e
	case "export":
		sub := flag.NewFlagSet("export", flag.ContinueOnError)
		sub.SetOutput(out)
		format := sub.String("format", "json", "json or csv")
		dest := sub.String("output", "", "new output file, or stdout if omitted")
		if e := sub.Parse(args); e != nil {
			return e
		}
		if len(sub.Args()) > 0 {
			return errors.New("usage: golf export [--format json|csv] [--output FILE]")
		}
		if *dest == "" {
			return s.ExportTo(*format, out)
		}
		if e := s.Export(*format, *dest); e != nil {
			return e
		}
		fmt.Fprintln(out, "Export saved:", *dest)
		return nil
	case "forget":
		if len(args) != 2 || args[1] != "--yes" {
			return errors.New("forget permanently removes one attempt and its volume; use golf forget ATTEMPT --yes")
		}
		a, e := s.Store.Attempt(args[0])
		if e != nil {
			return e
		}
		if a.Status != "solved" && a.Status != "abandoned" {
			return errors.New("abandon the attempt before forgetting it")
		}
		if e = s.Runner.Cleanup(ctx, a); e != nil {
			return e
		}
		if e = s.Store.DeleteAttempt(a.ID); e != nil {
			return e
		}
		fmt.Fprintln(out, "Attempt and its owned workspace removed.")
		return nil
	default:
		return fmt.Errorf("unknown command %q; run golf help", command)
	}
}

func overview(out io.Writer, cat *catalog.Catalog) error {
	fmt.Fprintln(out, "Driving Range | daily terminal practice | local history")
	fmt.Fprintf(out, "%d exercises across %s\n", len(cat.Current()), strings.Join(cat.Tracks(), ", "))
	fmt.Fprintln(out, "Run golf in an interactive terminal for the TUI.\nPlain commands: golf today, golf list, golf show EXERCISE, golf history.\nRun golf doctor to check the runtime; golf setup builds it explicitly.")
	return nil
}

func isTerminal(f *os.File) bool {
	return term.IsTerminal(f.Fd()) && os.Getenv("TERM") != "dumb"
}

func latestAssignment(attempts []model.Attempt, daily model.Assignment) model.Attempt {
	var latest model.Attempt
	for _, a := range attempts {
		if a.MatchesAssignment(daily) && (latest.ID == "" || a.CreatedAt.After(latest.CreatedAt)) {
			latest = a
		}
	}
	return latest
}
func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r >= 32 && r != 127 && !(r >= 128 && r <= 159) {
			return r
		}
		return -1
	}, s)
}
func printBrief(out io.Writer, ch model.Challenge) {
	fmt.Fprintf(out, "%s\n%s | about %d minutes | %s\n\n%s\n\n%s\n", safe(ch.Title), ch.Track, ch.Minutes, strings.Join(ch.Tools, ", "), safe(ch.Objective), safe(ch.Brief))
}
func printResult(out io.Writer, r model.CheckResult) {
	fmt.Fprintf(out, "%s: %s\n", safe(r.Outcome), safe(r.Summary))
	for _, line := range r.Details {
		fmt.Fprintln(out, safe(line))
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `golf [--state-dir DIR] [--plain] [--classic] [COMMAND]

  golf                              Open the terminal UI
  golf --classic                    Open the terminal UI, exercises take the
                                    whole terminal instead of a pane
  today [TRACK] [YYYY-MM-DD]         Read a published daily
  list [FILTER]                     Browse exercises
  show EXERCISE                     Read the brief
  play EXERCISE_OR_ATTEMPT           Start or resume in the real tool
  retry EXERCISE_OR_ATTEMPT          Create a separate attempt
  check ATTEMPT                     Validate and retain the result
  hint ATTEMPT                      Reveal the next hint
  reveal ATTEMPT --yes               Reveal a solution and record assistance
  abandon ATTEMPT --yes              Finish an attempt without deleting history
  history                           Print local performance records
  share ATTEMPT                     Print a spoiler-free result
  export [--format json|csv]         Export history to stdout
         [--output FILE]            Write a new export file
  config [track|theme VALUE]         Inspect or change settings
  doctor                            Check Docker and cached image
  setup                             Explicitly build the isolated runtime
  update                            Build and install the latest release
  audit [--solutions] [--track NAME] Check metadata or real reference solutions
  forget ATTEMPT --yes               Permanently delete one finished attempt
  version                           Print the build version

Practice uses your native tools and dotfiles; Docker prepares and checks files. No account or
telemetry is required. Update requires Go and network access. An expired daily
schedule leaves the entire practice catalog available.`)
}
