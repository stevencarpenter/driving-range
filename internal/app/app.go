// Package app owns the exercise lifecycle shared by the TUI and plain commands.
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stevencarpenter/driving-range/internal/catalog"
	"github.com/stevencarpenter/driving-range/internal/model"
	"github.com/stevencarpenter/driving-range/internal/runner"
	"github.com/stevencarpenter/driving-range/internal/store"
)

type Config struct {
	Track string `json:"track"`
	Theme string `json:"theme"`
	Image string `json:"image"`
}

type Service struct {
	Catalog          *catalog.Catalog
	Store            *store.Store
	Runner           *runner.Runner
	Config           Config
	StateDir         string
	unlock           func()
	recoveryMu       sync.Mutex
	recoveryPending  bool
	recoveryAttempts []model.Attempt
	recoveryWarning  atomic.Pointer[string]
}

type Play struct {
	Session   *runner.Session
	SessionID int64
	Attempt   model.Attempt
}

func (p *Play) Command() *exec.Cmd { return p.Session.Command() }

func DefaultStateDir() (string, error) {
	if s := os.Getenv("GOLF_STATE_DIR"); s != "" {
		return filepath.Abs(s)
	}
	if s := os.Getenv("XDG_STATE_HOME"); s != "" {
		return filepath.Abs(filepath.Join(s, "driving-range"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "driving-range"), nil
}

func ReadConfig(stateDir string) (Config, string, error) {
	var err error
	if stateDir == "" {
		stateDir, err = DefaultStateDir()
		if err != nil {
			return Config{}, "", err
		}
	}
	stateDir, err = filepath.Abs(stateDir)
	if err != nil {
		return Config{}, "", err
	}
	cat, err := catalog.Load()
	if err != nil {
		return Config{}, "", fmt.Errorf("load exercises: %w", err)
	}
	cfg := Config{Theme: "auto", Image: runner.DefaultImage}
	raw, err := os.ReadFile(filepath.Join(stateDir, "config.json"))
	if err == nil {
		if err = json.Unmarshal(raw, &cfg); err != nil {
			return Config{}, "", fmt.Errorf("read settings: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, "", err
	}
	if cfg.Image == "" {
		cfg.Image = runner.DefaultImage
	}
	if override := os.Getenv("GOLF_IMAGE"); override != "" {
		cfg.Image = override
	}
	if err = validateConfig(cat, cfg); err != nil {
		return Config{}, "", err
	}
	return cfg, stateDir, nil
}

func Open(stateDir string, readOnly bool) (*Service, error) {
	cfg, stateDir, err := ReadConfig(stateDir)
	if err != nil {
		return nil, err
	}
	cat, err := catalog.Load()
	if err != nil {
		return nil, err
	}
	s := &Service{Catalog: cat, Config: cfg, StateDir: stateDir, Runner: runner.NewNative(cfg.Image, filepath.Join(stateDir, "workspaces"))}
	if readOnly {
		s.Store, err = store.OpenReadOnly(stateDir)
	} else {
		if err = os.MkdirAll(stateDir, 0700); err != nil {
			return nil, err
		}
		s.unlock, err = store.Lock(stateDir)
		if err != nil {
			return nil, err
		}
		s.Store, err = store.Open(stateDir)
	}
	if err != nil {
		if s.unlock != nil {
			s.unlock()
		}
		return nil, err
	}
	if !readOnly {
		attempts, loadErr := s.Store.Attempts()
		if loadErr != nil {
			s.Close()
			return nil, loadErr
		}
		unfinished := []model.Attempt{}
		for _, a := range attempts {
			if a.Status != "solved" && a.Status != "abandoned" {
				unfinished = append(unfinished, a)
			}
		}
		s.recoveryPending = true
		s.recoveryAttempts = unfinished
		// Runtime availability must not prevent reading local exercises and history.
		// The same recovery transaction is retried before any execution operation.
		_ = s.retryRecovery(context.Background())
	}
	return s, nil
}

// RecoveryWarning is an immutable snapshot safe for rendering during recovery.
func (s *Service) RecoveryWarning() string {
	if warning := s.recoveryWarning.Load(); warning != nil {
		return *warning
	}
	return ""
}

func (s *Service) retryRecovery(ctx context.Context) error {
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	if !s.recoveryPending {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var err error
	if len(s.recoveryAttempts) > 0 {
		if cleanupErr := s.Runner.CleanupContainers(ctx, s.recoveryAttempts); cleanupErr != nil {
			err = fmt.Errorf("start Docker and retry to stop interrupted exercise containers: %w", cleanupErr)
		}
	}
	if err == nil {
		if storeErr := s.Store.RecoverInterrupted(); storeErr != nil {
			err = fmt.Errorf("recover interrupted exercise records: %w", storeErr)
		}
	}
	if err != nil {
		warning := err.Error()
		s.recoveryWarning.Store(&warning)
		return err
	}
	s.recoveryPending = false
	s.recoveryAttempts = nil
	s.recoveryWarning.Store(nil)
	return nil
}

func (s *Service) Close() error {
	err := s.Store.Close()
	if s.unlock != nil {
		s.unlock()
		s.unlock = nil
	}
	return err
}

func validateConfig(cat *catalog.Catalog, cfg Config) error {
	switch cfg.Theme {
	case "auto", "dark", "light", "plain":
	default:
		return fmt.Errorf("unknown theme %q; choose auto, dark, light, or plain", cfg.Theme)
	}
	if cfg.Track != "" && !slices.Contains(cat.Tracks(), cfg.Track) {
		return fmt.Errorf("unknown track %q", cfg.Track)
	}
	if strings.TrimSpace(cfg.Image) == "" || strings.ContainsAny(cfg.Image, "\r\n\x00") {
		return errors.New("runtime image must be a nonempty image name or digest")
	}
	return nil
}

func WriteConfig(stateDir string, cfg Config) error {
	cat, err := catalog.Load()
	if err != nil {
		return err
	}
	if err = validateConfig(cat, cfg); err != nil {
		return err
	}
	if err = os.MkdirAll(stateDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(stateDir, ".config-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(stateDir, "config.json"))
}

func (s *Service) SetConfig(cfg Config) error {
	if cfg.Image == "" {
		cfg.Image = s.Config.Image
	}
	if err := WriteConfig(s.StateDir, cfg); err != nil {
		return err
	}
	s.Config = cfg
	s.Runner = runner.NewNative(cfg.Image, filepath.Join(s.StateDir, "workspaces"))
	return nil
}

func (s *Service) Doctor(ctx context.Context) (runner.Report, error) { return s.Runner.Doctor(ctx) }

func (s *Service) NewAttempt(ctx context.Context, id string, assignment *model.Assignment, retryOf string) (model.Attempt, error) {
	if err := s.retryRecovery(ctx); err != nil {
		return model.Attempt{}, err
	}
	var ch model.Challenge
	var err error
	var previous *model.Attempt
	imageRunner := s.Runner
	if retryOf != "" {
		prior, e := s.Store.Attempt(retryOf)
		if e != nil {
			return model.Attempt{}, e
		}
		if prior.ExerciseID != id {
			return model.Attempt{}, errors.New("retry must identify the same exercise")
		}
		previous = &prior
		ch, err = s.resolveAttempt(prior)
		imageRunner = runner.New(prior.EnvironmentID)
	} else if assignment != nil {
		ch, err = s.Catalog.Resolve(*assignment)
		if ch.ID != id && err == nil {
			err = errors.New("assignment does not identify the selected exercise")
		}
	} else {
		ch, err = s.Catalog.Find(id, 0)
	}
	if err != nil {
		return model.Attempt{}, err
	}
	report, err := imageRunner.Doctor(ctx)
	if err != nil {
		return model.Attempt{}, fmt.Errorf("prepare runtime: %w", err)
	}
	if !report.Available || report.ImageID == "" {
		return model.Attempt{}, fmt.Errorf("runtime not ready: %s; run golf setup", report.Message)
	}
	var buf [16]byte
	if _, err = rand.Read(buf[:]); err != nil {
		return model.Attempt{}, err
	}
	attemptID := hex.EncodeToString(buf[:])
	a := model.Attempt{ID: attemptID, ExerciseID: ch.ID, Revision: ch.Revision, Track: ch.Track, Seed: "default", Profile: ch.Profile, EnvironmentID: report.ImageID, ValidatorVersion: ch.Validator.Version, Status: "active", CreatedAt: time.Now().UTC(), RetryOf: retryOf, Workspace: "golf-" + attemptID}
	if assignment != nil {
		a.AssignmentDate = assignment.Date
		a.Seed = assignment.Seed
	}
	if previous != nil {
		a.AssignmentDate = previous.AssignmentDate
		a.Seed = previous.Seed
		a.Profile = previous.Profile
	}
	if err = s.Store.CreateAttempt(a); err != nil {
		return a, fmt.Errorf("save new attempt: %w", err)
	}
	return a, nil
}

func (s *Service) resolveAttempt(a model.Attempt) (model.Challenge, error) {
	ch, err := s.Catalog.Find(a.ExerciseID, a.Revision)
	if err != nil {
		return ch, err
	}
	if ch.Track != a.Track || ch.Profile != a.Profile || ch.Validator.Version != a.ValidatorVersion {
		return model.Challenge{}, errors.New("exercise metadata changed without a revision; preserve this attempt and start a new exercise revision")
	}
	return ch, nil
}

func (s *Service) Latest(id string) (*model.Attempt, error) {
	attempts, err := s.Store.Attempts()
	if err != nil {
		return nil, err
	}
	var latest *model.Attempt
	for _, a := range attempts {
		if a.ExerciseID == id && (latest == nil || a.CreatedAt.After(latest.CreatedAt)) {
			v := a
			latest = &v
		}
	}
	return latest, nil
}

func (s *Service) Prepare(ctx context.Context, id string) (*Play, error) {
	if err := s.retryRecovery(ctx); err != nil {
		return nil, err
	}
	a, err := s.Store.Attempt(id)
	if err != nil {
		return nil, err
	}
	if a.Status == "solved" || a.Status == "abandoned" {
		return nil, errors.New("this attempt is finished; retry creates a new attempt")
	}
	ch, err := s.resolveAttempt(a)
	if err != nil {
		return nil, err
	}
	priorSessions, err := s.Store.Sessions(id)
	if err != nil {
		return nil, err
	}
	// No tool can run before StartSession commits. A volume without a session
	// can only contain incomplete preparation from a crash, never player work.
	if len(priorSessions) == 0 {
		if err = s.Runner.Cleanup(ctx, a); err != nil {
			return nil, fmt.Errorf("clear unfinished preparation: %w", err)
		}
	}
	session, err := s.Runner.Prepare(ctx, ch, a)
	if err != nil {
		result := model.CheckResult{Outcome: "infrastructure_error", Summary: "Could not launch exercise", Details: []string{err.Error()}}
		if saveErr := s.Store.RecordCheck(id, result); saveErr != nil {
			return nil, errors.Join(err, fmt.Errorf("save launch error: %w", saveErr))
		}
		return nil, err
	}
	sessionID, err := s.Store.StartSession(id)
	if err != nil {
		session.Finish(err)
		return nil, fmt.Errorf("save session before launch: %w", err)
	}
	return &Play{Session: session, SessionID: sessionID, Attempt: a}, nil
}

func (s *Service) Finish(ctx context.Context, p *Play, childErr error) (model.CheckResult, error) {
	res := p.Session.Finish(childErr)
	if err := s.Store.EndSession(p.SessionID, res); err != nil {
		return model.CheckResult{}, fmt.Errorf("save session: %w", err)
	}
	if res.Error != "" || res.Interrupted {
		summary := "Exercise interrupted; workspace preserved"
		if res.Error != "" {
			summary = "Exercise process failed; workspace preserved"
		}
		result := model.CheckResult{Outcome: "infrastructure_error", Summary: summary, ExitCode: res.ExitCode}
		if res.Error != "" {
			result.Details = []string{res.Error}
		}
		if res.Interrupted {
			return result, nil
		}
		return result, s.Store.RecordCheck(p.Attempt.ID, result)
	}
	return s.Check(ctx, p.Attempt.ID)
}

func (s *Service) Check(ctx context.Context, id string) (model.CheckResult, error) {
	if err := s.retryRecovery(ctx); err != nil {
		return model.CheckResult{}, err
	}
	a, err := s.Store.Attempt(id)
	if err != nil {
		return model.CheckResult{}, err
	}
	if a.Status == "solved" || a.Status == "abandoned" {
		return model.CheckResult{}, errors.New("finished attempts are immutable; select retry to make another attempt")
	}
	ch, err := s.resolveAttempt(a)
	if err != nil {
		return model.CheckResult{}, err
	}
	result, checkErr := s.Runner.Check(ctx, ch, a)
	if checkErr != nil {
		result = model.CheckResult{Outcome: "infrastructure_error", Summary: "Could not validate exercise", Details: []string{checkErr.Error()}}
	}
	if err = s.Store.RecordCheck(id, result); err != nil {
		return result, errors.Join(checkErr, fmt.Errorf("save check result: %w", err))
	}
	return result, checkErr
}

func (s *Service) Hint(id string) (string, error) {
	a, err := s.Store.Attempt(id)
	if err != nil {
		return "", err
	}
	ch, err := s.Catalog.Find(a.ExerciseID, a.Revision)
	if err != nil {
		return "", err
	}
	if len(ch.Hints) == 0 {
		return "No hints for this exercise.", nil
	}
	level := a.HintLevel + 1
	if level > len(ch.Hints) {
		level = len(ch.Hints)
	}
	if err = s.Store.SetAssistance(id, level, a.SolutionRevealed, a.ExternalAssistance); err != nil {
		return "", err
	}
	return strings.Join(ch.Hints[:level], "\n\n"), nil
}

func (s *Service) Reveal(id string) (string, error) {
	a, err := s.Store.Attempt(id)
	if err != nil {
		return "", err
	}
	ch, err := s.Catalog.Find(a.ExerciseID, a.Revision)
	if err != nil {
		return "", err
	}
	if a.Status != "solved" && a.Status != "abandoned" {
		if err = s.Store.SetAssistance(id, a.HintLevel, true, a.ExternalAssistance); err != nil {
			return "", err
		}
	}
	return ch.Explanation + "\n\nReference solution:\n" + ch.ReferenceSolution, nil
}

func (s *Service) SetExternalAssistance(id string, used bool) error {
	a, err := s.Store.Attempt(id)
	if err != nil {
		return err
	}
	return s.Store.SetAssistance(id, a.HintLevel, a.SolutionRevealed, used)
}

func (s *Service) Abandon(id string) error {
	// An offline abandonment must not hide a still-running container from the
	// next startup's unfinished-attempt recovery scan.
	if err := s.retryRecovery(context.Background()); err != nil {
		return err
	}
	return s.Store.FinishAttempt(id, "abandoned")
}

func (s *Service) Export(format, path string) error {
	if format != "json" && format != "csv" {
		return errors.New("export format must be json or csv")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create export (existing files are preserved): %w", err)
	}
	success := false
	defer func() {
		f.Close()
		if !success {
			os.Remove(path)
		}
	}()
	err = s.ExportTo(format, f)
	if err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	success = true
	return nil
}

func (s *Service) Share(id string) (string, error) {
	a, err := s.Store.Attempt(id)
	if err != nil {
		return "", err
	}
	ch, err := s.Catalog.Find(a.ExerciseID, a.Revision)
	if err != nil {
		return "", err
	}
	hint := "unaided"
	if a.HintLevel > 0 || a.SolutionRevealed || a.ExternalAssistance {
		hint = "assisted"
	}
	return fmt.Sprintf("Driving Range | %s | %s | %s | %s", ch.ID, a.AssignmentDate, a.Status, hint), nil
}

func (s *Service) ExportTo(format string, w io.Writer) error {
	switch format {
	case "json":
		return s.Store.ExportJSON(w)
	case "csv":
		return s.Store.ExportCSV(w)
	default:
		return errors.New("export format must be json or csv")
	}
}
