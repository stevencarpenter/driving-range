// Package tui implements the shell-native practice interface.
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stevencarpenter/driving-range/internal/app"
	"github.com/stevencarpenter/driving-range/internal/model"
)

type screen int

const (
	today screen = iota
	practice
	progress
	settings
	exercise
	detail
)

// Model keeps navigation separate from the persisted attempt lifecycle.
type Model struct {
	service          *app.Service
	renderer         *lipgloss.Renderer
	spinner          spinner.Model
	lifecycle        *lifecycle
	screen           screen
	returnTo         screen
	detailFrom       screen
	width, height    int
	selected, offset int
	query            string
	searching        bool
	chooseTrack      bool
	trackIndex       int
	help             bool
	confirm          string
	busy             bool
	status, notice   string
	errorText        string
	challenge        *model.Challenge
	assignment       *model.Assignment
	attemptID        string
	records          []model.Progress
	checks           []model.CheckEvent
	sessions         []model.Session
	best             *model.Progress
	now              func() time.Time
}

type loadedMsg struct {
	records []model.Progress
	err     error
}
type operationMsg struct {
	text, id string
	err      error
}
type preparedMsg struct {
	play *app.Play
	id   string
	err  error
}
type exitedMsg struct {
	play *app.Play
	err  error
}
type detailMsg struct {
	checks   []model.CheckEvent
	sessions []model.Session
	best     *model.Progress
	err      error
}

func New(s *app.Service) Model {
	return Model{service: s, spinner: spinner.New(spinner.WithSpinner(spinner.MiniDot)), renderer: lipgloss.NewRenderer(os.Stdout), lifecycle: newLifecycle(), width: 80, height: 24, chooseTrack: s.Config.Track == "", now: time.Now}
}

func Run(s *app.Service) (err error) {
	m := New(s)
	defer func() { err = errors.Join(err, m.lifecycle.shutdown()) }()
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m Model) Init() tea.Cmd { return m.refresh() }

func (m Model) refresh() tea.Cmd {
	s := m.service
	return m.lifecycle.command(func() tea.Msg { records, err := s.Store.Progress(); return loadedMsg{records, err} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
		m.offset = 0
		return m, nil
	case loadedMsg:
		if msg.err != nil {
			m.errorText = "Read history: " + msg.err.Error()
		} else {
			m.records = msg.records
		}
		m.clampSelection()
		return m, nil
	case preparedMsg:
		if msg.id != "" {
			m.attemptID = msg.id
		}
		if msg.err != nil {
			m.busy = false
			m.status = ""
			m.offset = 0
			m.notice = ""
			m.errorText = "Start exercise: " + msg.err.Error() + "\nOpen Settings and run dependency checks."
			if msg.id != "" {
				m.errorText += " Your attempt is retained."
			}
			return m, m.refresh()
		}
		m.status = "Exercise running. Exit the child tool to return."
		return m, tea.ExecProcess(msg.play.Command(), func(err error) tea.Msg { return exitedMsg{msg.play, err} })
	case exitedMsg:
		m.status = "Saving exercise session and checking result..."
		s := m.service
		return m, m.lifecycle.command(func() tea.Msg {
			result, err := s.Finish(m.lifecycle.ctx, msg.play, msg.err)
			m.lifecycle.finished(err)
			return operationMsg{text: formatCheck(result), err: err}
		})
	case operationMsg:
		m.lifecycle.acknowledge()
		m.busy = false
		m.status = ""
		if msg.id != "" {
			m.attemptID = msg.id
		}
		if msg.err != nil {
			m.notice = ""
			m.offset = 0
			m.errorText = msg.err.Error()
		} else {
			m.errorText = ""
			m.notice = msg.text
			m.offset = 0
		}
		return m, m.refresh()
	case detailMsg:
		m.busy = false
		m.status = ""
		m.checks, m.sessions, m.best = msg.checks, msg.sessions, msg.best
		if msg.err != nil {
			m.errorText = "Read attempt details: " + msg.err.Error()
		}
		return m, nil
	case spinner.TickMsg:
		if !m.busy {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		next, cmd := m.key(msg)
		updated := next.(Model)
		if !m.busy && updated.busy {
			cmd = tea.Batch(cmd, updated.spinner.Tick)
		}
		return updated, cmd
	}
	return m, nil
}

func (m Model) key(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := key.String()
	if m.busy {
		return m, nil
	}
	if m.confirm != "" {
		if k == "esc" || k == "n" {
			m.confirm = ""
			return m, nil
		}
		if k != "y" {
			return m, nil
		}
		action := m.confirm
		m.confirm = ""
		return m.perform(action)
	}
	if m.searching {
		switch k {
		case "enter", "esc":
			m.searching = false
		case "backspace", "ctrl+h":
			r := []rune(m.query)
			if len(r) > 0 {
				m.query = string(r[:len(r)-1])
			}
		case "ctrl+u":
			m.query = ""
		default:
			if key.Type == tea.KeyRunes {
				m.query += string(key.Runes)
			}
		}
		m.selected, m.offset = 0, 0
		return m, nil
	}
	if m.help {
		switch k {
		case "?", "esc", "q":
			m.help = false
			m.offset = 0
		case "j", "down":
			m.scroll(1)
		case "k", "up":
			m.scroll(-1)
		case "pgdown", "ctrl+f":
			m.scroll(m.contentHeight())
		case "pgup", "ctrl+b":
			m.scroll(-m.contentHeight())
		case "home":
			m.offset = 0
		}
		return m, nil
	}
	if k == "?" {
		m.help = true
		m.offset = 0
		return m, nil
	}
	if m.chooseTrack {
		tracks := m.service.Catalog.Tracks()
		switch k {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			m.trackIndex = min(len(tracks)-1, m.trackIndex+1)
		case "k", "up":
			m.trackIndex = max(0, m.trackIndex-1)
		case "esc":
			if m.service.Config.Track != "" {
				m.chooseTrack = false
			}
		case "enter":
			if len(tracks) == 0 {
				return m, nil
			}
			config := m.service.Config
			config.Track = tracks[m.trackIndex]
			if err := m.service.SetConfig(config); err != nil {
				m.errorText = "Save track: " + err.Error()
				return m, nil
			}
			m.chooseTrack = false
			m.screen = today
			m.selected = 0
			m.offset = 0
			cmd := m.doctor()
			return m, cmd
		}
		return m, nil
	}
	if k == "q" || k == "ctrl+c" {
		return m, tea.Quit
	}
	switch k {
	case "tab":
		m.navigate(screen((int(m.topLevel()) + 1) % 4))
		return m, nil
	case "shift+tab":
		m.navigate(screen((int(m.topLevel()) + 3) % 4))
		return m, nil
	case "1":
		m.navigate(today)
		return m, nil
	case "2":
		m.navigate(practice)
		return m, nil
	case "3":
		m.navigate(progress)
		return m, nil
	case "4":
		m.navigate(settings)
		return m, nil
	case "esc":
		if m.screen == detail {
			m.navigate(m.detailFrom)
		} else if m.screen == exercise {
			m.navigate(m.returnTo)
		} else {
			m.notice = ""
			m.errorText = ""
			m.query = ""
		}
		return m, nil
	case "pgdown", "ctrl+f":
		if m.listNavigating() {
			m.selected += m.contentHeight()
			m.clampSelection()
		} else {
			m.scroll(m.contentHeight())
		}
		return m, nil
	case "pgup", "ctrl+b":
		if m.listNavigating() {
			m.selected -= m.contentHeight()
			m.clampSelection()
		} else {
			m.scroll(-m.contentHeight())
		}
		return m, nil
	case "home":
		m.offset = 0
		m.selected = 0
		return m, nil
	case "j", "down", "k", "up":
		delta := 1
		if k == "k" || k == "up" {
			delta = -1
		}
		if m.listNavigating() {
			m.selected += delta
			m.clampSelection()
		} else {
			m.scroll(delta)
		}
		return m, nil
	}
	switch m.screen {
	case today:
		if k == "t" {
			m.selectTrack()
		}
		if k == "enter" || k == "s" {
			c, a := m.todayChallenge()
			if c != nil {
				m.openChallenge(*c, a, today)
			}
		}
	case practice:
		if k == "/" {
			m.searching = true
		}
		if k == "enter" {
			items := m.filtered()
			if len(items) > 0 {
				m.openChallenge(items[m.selected], nil, practice)
			}
		}
	case progress:
		if k == "enter" && len(m.records) > 0 {
			m.screen = detail
			m.detailFrom = progress
			m.offset = 0
			m.attemptID = m.records[m.selected].Attempt.ID
			cmd := m.loadDetail(m.records[m.selected].Attempt)
			return m, cmd
		}
		if k == "e" {
			return m.perform("export-json")
		}
		if k == "c" {
			return m.perform("export-csv")
		}
	case settings:
		if k == "t" {
			m.selectTrack()
		}
		if k == "d" {
			cmd := m.doctor()
			return m, cmd
		}
		if k == "l" {
			config := m.service.Config
			switch config.Theme {
			case "light":
				config.Theme = "dark"
			case "dark":
				config.Theme = "plain"
			default:
				config.Theme = "light"
			}
			if err := m.service.SetConfig(config); err != nil {
				m.errorText = "Save theme: " + err.Error()
			} else {
				m.notice = "Theme saved: " + config.Theme
			}
		}
	case exercise:
		switch k {
		case "enter", "s":
			return m.launch(false)
		case "c":
			return m.perform("check")
		case "h":
			return m.perform("hint")
		case "v":
			m.confirm = "reveal"
		case "r":
			return m.launch(true)
		case "a":
			if m.attemptID != "" {
				m.confirm = "abandon"
			}
		case "x":
			return m.perform("assistance")
		case "p":
			if a := m.currentAttempt(); a != nil {
				m.screen = detail
				m.detailFrom = exercise
				m.offset = 0
				cmd := m.loadDetail(*a)
				return m, cmd
			}
		}
	case detail:
		if k == "s" {
			return m.perform("share")
		}
		if k == "e" {
			return m.perform("export-json")
		}
		if k == "c" {
			return m.perform("export-csv")
		}
		if k == "enter" {
			if a := m.currentAttempt(); a != nil {
				if c, err := m.service.Catalog.Find(a.ExerciseID, a.Revision); err == nil {
					m.challenge = &c
					m.assignment = nil
					if a.AssignmentDate != "" {
						m.assignment = &model.Assignment{Date: a.AssignmentDate, Track: a.Track, ExerciseID: a.ExerciseID, Revision: a.Revision, Seed: a.Seed}
					}
					m.screen = exercise
					m.returnTo = progress
					m.offset = 0
					m.notice = ""
				} else {
					m.errorText = err.Error()
				}
			}
		}
	}
	return m, nil
}

func (m *Model) navigate(s screen) {
	m.screen = s
	m.selected = 0
	m.offset = 0
	m.notice = ""
	m.errorText = ""
	m.confirm = ""
}

func (m *Model) selectTrack() {
	m.chooseTrack = true
	m.trackIndex = 0
	for i, track := range m.service.Catalog.Tracks() {
		if track == m.service.Config.Track {
			m.trackIndex = i
			break
		}
	}
}
func (m Model) topLevel() screen {
	if m.screen == detail {
		if m.detailFrom == exercise {
			return m.returnTo
		}
		return m.detailFrom
	}
	if m.screen == exercise {
		return m.returnTo
	}
	return m.screen
}

func (m *Model) openChallenge(c model.Challenge, a *model.Assignment, from screen) {
	m.challenge = &c
	m.assignment = a
	m.screen = exercise
	m.returnTo = from
	m.offset = 0
	m.notice = ""
	m.errorText = ""
	m.attemptID = ""
	for _, r := range m.records {
		if r.Attempt.ExerciseID == c.ID && r.Attempt.Revision == c.Revision && (a == nil || r.Attempt.MatchesAssignment(*a)) {
			m.attemptID = r.Attempt.ID
			break
		}
	}
}

func (m Model) todayChallenge() (*model.Challenge, *model.Assignment) {
	if a, ok := m.service.Catalog.Today(m.now(), m.service.Config.Track); ok {
		if c, err := m.service.Catalog.Find(a.ExerciseID, a.Revision); err == nil {
			return &c, &a
		}
	}
	for _, c := range m.service.Catalog.All() {
		if c.Track == m.service.Config.Track {
			return &c, nil
		}
	}
	return nil, nil
}

func (m Model) currentAttempt() *model.Attempt {
	for _, r := range m.records {
		if r.Attempt.ID == m.attemptID {
			a := r.Attempt
			return &a
		}
	}
	return nil
}

func (m Model) filtered() []model.Challenge {
	var result []model.Challenge
	for _, c := range m.service.Catalog.All() {
		status := "untried"
		for _, r := range m.records {
			if r.Attempt.ExerciseID == c.ID && r.Attempt.Revision == c.Revision {
				status = r.Attempt.Status
				if r.FailedChecks > 0 && status != "solved" {
					status += " missed"
				}
				break
			}
		}
		haystack := strings.ToLower(fmt.Sprintf("%s %s %s %s %s difficulty:%d %s", c.ID, c.Title, c.Track, strings.Join(c.Tools, " "), strings.Join(c.Concepts, " "), c.Difficulty, status))
		match := true
		for _, word := range strings.Fields(strings.ToLower(m.query)) {
			if !strings.Contains(haystack, word) {
				match = false
				break
			}
		}
		if match {
			result = append(result, c)
		}
	}
	return result
}

func (m *Model) clampSelection() {
	n := len(m.records)
	if m.screen == practice {
		n = len(m.filtered())
	}
	m.selected = max(0, min(m.selected, n-1))
}

func (m Model) listNavigating() bool {
	return (m.screen == practice || m.screen == progress) && m.notice == "" && m.errorText == ""
}

func (m Model) launch(retry bool) (tea.Model, tea.Cmd) {
	if m.challenge == nil {
		return m, nil
	}
	a := m.currentAttempt()
	if !retry && a != nil && (a.Status == "solved" || a.Status == "abandoned") {
		m.notice = "This attempt is complete. Press r for a new attempt; the recorded result is preserved."
		return m, nil
	}
	m.busy = true
	m.errorText = ""
	m.status = "Preparing isolated workspace..."
	s, id, c, assignment := m.service, m.attemptID, *m.challenge, m.assignment
	return m, m.lifecycle.command(func() tea.Msg {
		if id == "" || retry {
			retryOf := ""
			if retry {
				retryOf = id
			}
			a, err := s.NewAttempt(m.lifecycle.ctx, c.ID, assignment, retryOf)
			if err != nil {
				return preparedMsg{err: err}
			}
			id = a.ID
		}
		play, err := s.Prepare(m.lifecycle.ctx, id)
		if play != nil {
			m.lifecycle.prepared(func() error { _, err := s.Finish(m.lifecycle.ctx, play, context.Canceled); return err })
		}
		return preparedMsg{play: play, id: id, err: err}
	})
}

func (m Model) perform(action string) (tea.Model, tea.Cmd) {
	s, id := m.service, m.attemptID
	if action == "check" || action == "abandon" {
		if id == "" {
			m.notice = "Start the exercise before checking or abandoning it."
			return m, nil
		}
	}
	if action == "share" && id == "" {
		return m, nil
	}
	var challengeID string
	if m.challenge != nil {
		challengeID = m.challenge.ID
	}
	assignment := m.assignment
	assisted := false
	if a := m.currentAttempt(); a != nil {
		assisted = a.ExternalAssistance
	}
	if action == "assistance" && assisted {
		m.notice = "External assistance remains recorded for this attempt."
		m.errorText = ""
		m.offset = 0
		return m, nil
	}
	m.busy = true
	m.status = "Saving..."
	m.errorText = ""
	return m, m.lifecycle.command(func() tea.Msg {
		var err error
		var text string
		if (action == "hint" || action == "reveal" || action == "assistance") && id == "" {
			a, e := s.NewAttempt(m.lifecycle.ctx, challengeID, assignment, "")
			if e != nil {
				return operationMsg{err: e}
			}
			id = a.ID
		}
		switch action {
		case "check":
			var result model.CheckResult
			result, err = s.Check(m.lifecycle.ctx, id)
			text = formatCheck(result)
		case "hint":
			text, err = s.Hint(id)
		case "reveal":
			text, err = s.Reveal(id)
		case "abandon":
			err = s.Abandon(id)
			text = "Attempt abandoned. Its history is preserved. Press r to start a new attempt."
		case "assistance":
			err = s.SetExternalAssistance(id, true)
			text = "External assistance recorded for this attempt."
		case "share":
			text, err = s.Share(id)
		case "export-json", "export-csv":
			format := strings.TrimPrefix(action, "export-")
			path := filepath.Join(s.StateDir, "exports", "progress-"+time.Now().UTC().Format("20060102T150405.000000000")+"."+format)
			err = os.MkdirAll(filepath.Dir(path), 0700)
			if err == nil {
				err = s.Export(format, path)
			}
			text = "Export saved: " + path
		}
		if err != nil {
			err = fmt.Errorf("%s: %w. Result was not confirmed saved; retry after resolving this error", action, err)
		}
		return operationMsg{text: text, id: id, err: err}
	})
}

func (m *Model) doctor() tea.Cmd {
	m.busy = true
	m.status = "Checking Docker and the standard tool image..."
	s := m.service
	return m.lifecycle.command(func() tea.Msg {
		report, err := s.Doctor(m.lifecycle.ctx)
		text := report.Message
		if report.Available {
			text = "Runtime ready. " + text + "\nImage: " + report.ImageID
		} else {
			text = "Runtime unavailable. " + text + "\nRun golf setup from your shell."
		}
		return operationMsg{text: text, err: err}
	})
}

func (m *Model) loadDetail(a model.Attempt) tea.Cmd {
	m.busy = true
	m.status = "Reading saved result..."
	m.checks = nil
	m.sessions = nil
	m.best = nil
	s := m.service
	return m.lifecycle.command(func() tea.Msg {
		checks, err := s.Store.Checks(a.ID)
		if err != nil {
			return detailMsg{err: err}
		}
		sessions, err := s.Store.Sessions(a.ID)
		if err != nil {
			return detailMsg{err: err}
		}
		best, err := s.Store.Best(a)
		return detailMsg{checks: checks, sessions: sessions, best: best, err: err}
	})
}

func formatCheck(r model.CheckResult) string {
	return strings.Join(append([]string{strings.ToUpper(r.Outcome) + ": " + r.Summary}, r.Details...), "\n")
}

// clean prevents challenge text or validator output from controlling the terminal.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

func (m Model) contentHeight() int {
	if m.framed() {
		return max(1, m.height-8)
	}
	return max(1, m.height-6)
}

func duration(p model.Progress) string {
	if p.UnknownDuration {
		return "unknown"
	}
	return (time.Duration(p.DurationMS) * time.Millisecond).Round(time.Second).String()
}
