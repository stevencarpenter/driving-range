package runner

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/stevencarpenter/driving-range/internal/model"
)

func (r *Runner) Check(ctx context.Context, c model.Challenge, a model.Attempt) (model.CheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := identity(a); err != nil {
		return model.CheckResult{}, err
	}
	if err := r.CleanupContainers(ctx, []model.Attempt{a}); err != nil {
		return model.CheckResult{}, err
	}
	fresh, err := r.ensureVolume(ctx, a)
	if err != nil {
		return model.CheckResult{}, err
	}
	if fresh {
		r.Cleanup(ctx, a)
		return model.CheckResult{}, errors.New("attempt workspace is missing; start an exercise before checking")
	}
	files, err := r.snapshot(ctx, a)
	if err != nil {
		return model.CheckResult{}, err
	}
	if len(c.Fixtures) == 0 {
		return model.CheckResult{}, errors.New("exercise has no fixtures")
	}
	result := model.CheckResult{Outcome: "pass", Summary: "All checks passed."}
	switch c.Validator.Kind {
	case "tree":
		expected := c.Fixtures[0].ExpectedFiles
		for name, want := range expected {
			actual, ok := files[name]
			if !ok {
				result.Details = append(result.Details, "Missing file: "+SafeText(name))
			} else if actual != want {
				result.Details = append(result.Details, fmt.Sprintf("%s: expected %s; got %s", SafeText(name), preview(want), preview(actual)))
			}
		}
		if !c.Validator.AllowExtraFiles {
			for name := range files {
				if _, ok := expected[name]; !ok {
					result.Details = append(result.Details, "Unexpected file: "+SafeText(name))
				}
			}
		}
	case "stdout":
		name := c.SubmissionFile
		if name == "" {
			name = "solution.sh"
		}
		if !safePath(name) {
			return model.CheckResult{}, errors.New("invalid submission path")
		}
		script, ok := files[name]
		if !ok {
			result.Details = append(result.Details, "Write your submission to "+SafeText(name))
			break
		}
		argv := c.SubmissionArgv
		if len(argv) == 0 {
			argv = []string{"/bin/bash", "--noprofile", "--norc", name}
		}
		for _, fixture := range c.Fixtures {
			out, code, err := r.fixtureCommand(ctx, a, fixture, map[string]string{name: script}, argv)
			if err != nil {
				return model.CheckResult{}, err
			}
			equal, err := outputsEqual(out, fixture.ExpectedStdout, c.Validator.OutputPolicy)
			if err != nil {
				return model.CheckResult{}, err
			}
			if code != 0 || !equal {
				result.ExitCode = code
				result.Details = append(result.Details, fmt.Sprintf("%s: exit %d; expected %s; got %s", SafeText(fixture.Name), code, preview(fixture.ExpectedStdout), preview(out)))
			}
		}
	case "commands":
		if len(c.Validator.Checks) == 0 {
			return model.CheckResult{}, errors.New("repository validator has no checks")
		}
		// Each semantic check gets an independent snapshot. The candidate's repository
		// configuration and hooks never run on the host or affect another check.
		for _, check := range c.Validator.Checks {
			out, code, err := r.fixtureCommand(ctx, a, model.Fixture{Files: files}, nil, check.Argv)
			if err != nil {
				return model.CheckResult{}, err
			}
			equal, err := outputsEqual(out, check.ExpectedStdout, c.Validator.OutputPolicy)
			if err != nil {
				return model.CheckResult{}, err
			}
			if code != check.ExitCode || !equal {
				result.ExitCode = code
				result.Details = append(result.Details, fmt.Sprintf("%s: expected exit %d and %s; got exit %d and %s", SafeText(check.Name), check.ExitCode, preview(check.ExpectedStdout), code, preview(out)))
			}
		}
	default:
		return model.CheckResult{}, fmt.Errorf("unsupported validator %q", c.Validator.Kind)
	}
	if len(result.Details) > 0 {
		sort.Strings(result.Details)
		if len(result.Details) > 20 {
			result.Details = append(result.Details[:20], "Additional differences omitted.")
		}
		result.Outcome = "fail"
		result.Summary = "The result does not match the exercise yet."
	}
	return result, nil
}

func (r *Runner) fixtureCommand(ctx context.Context, parent model.Attempt, fixture model.Fixture, submission map[string]string, argv []string) (stdout string, code int, resultErr error) {
	if len(argv) == 0 {
		return "", -1, errors.New("empty validation command")
	}
	id := randomID()
	a := model.Attempt{ID: id, Workspace: "golf-check-" + id, EnvironmentID: parent.EnvironmentID}
	if _, err := r.ensureVolume(ctx, a); err != nil {
		return "", -1, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := r.Cleanup(cleanup, a); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("validation cleanup failed: %w", err))
		}
	}()
	container, err := r.create(ctx, a, false)
	if err != nil {
		return "", -1, err
	}
	if err = r.populate(ctx, container, fixture.Files); err != nil {
		return "", -1, err
	}
	if err = r.setup(ctx, container, fixture.Setup); err != nil {
		return "", -1, err
	}
	if len(submission) > 0 {
		if err = r.populate(ctx, container, submission); err != nil {
			return "", -1, err
		}
	}
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	status := "/tmp/golf-status-" + randomID()
	args := completionArgs(container, status, false, argv)
	out, _, err := docker(bounded, nil, args...)
	if bounded.Err() != nil {
		return "", -1, fmt.Errorf("validation command exceeded deadline: %w", bounded.Err())
	}
	if err != nil {
		return "", -1, err
	}
	code, err = containerStatus(container, status)
	return string(out), code, err
}

func outputsEqual(actual, expected, policy string) (bool, error) {
	switch policy {
	case "", "exact", "exact-bytes":
		return actual == expected, nil
	case "unordered-lines":
		a := strings.Split(strings.TrimSuffix(actual, "\n"), "\n")
		b := strings.Split(strings.TrimSuffix(expected, "\n"), "\n")
		sort.Strings(a)
		sort.Strings(b)
		return strings.Join(a, "\n") == strings.Join(b, "\n"), nil
	case "line-set":
		set := func(s string) string {
			m := map[string]bool{}
			for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
				m[line] = true
			}
			v := make([]string, 0, len(m))
			for line := range m {
				v = append(v, line)
			}
			sort.Strings(v)
			return strings.Join(v, "\n")
		}
		return set(actual) == set(expected), nil
	default:
		return false, fmt.Errorf("unsupported output policy %q", policy)
	}
}

func preview(s string) string {
	if len(s) > 256 {
		s = s[:256] + "..."
	}
	return fmt.Sprintf("%q", s)
}

// VerifyReference exercises the same preparation and checking path as players.
func (r *Runner) VerifyReference(ctx context.Context, c model.Challenge) (model.CheckResult, error) {
	report, err := r.Doctor(ctx)
	if err != nil {
		return model.CheckResult{}, err
	}
	if !report.Available {
		return model.CheckResult{}, errors.New(report.Message)
	}
	id := randomID()
	a := model.Attempt{ID: id, Workspace: "golf-reference-" + id, EnvironmentID: report.ImageID}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		r.Cleanup(cleanup, a)
	}()
	if _, err = r.Execute(ctx, c, a, c.ReferenceSolution); err != nil {
		return model.CheckResult{}, err
	}
	return r.Check(ctx, c, a)
}

// Audit requires a failing baseline and a passing executable reference solution.
func (r *Runner) Audit(ctx context.Context, c model.Challenge) (model.CheckResult, error) {
	report, err := r.Doctor(ctx)
	if err != nil {
		return model.CheckResult{}, err
	}
	if !report.Available {
		return model.CheckResult{}, errors.New(report.Message)
	}
	id := randomID()
	a := model.Attempt{ID: id, Workspace: "golf-audit-" + id, EnvironmentID: report.ImageID}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		r.Cleanup(cleanup, a)
	}()
	s, err := r.Prepare(ctx, c, a)
	if err != nil {
		return model.CheckResult{}, err
	}
	if result := s.Finish(nil); result.Error != "" {
		return model.CheckResult{}, errors.New(result.Error)
	}
	baseline, err := r.Check(ctx, c, a)
	if err != nil {
		return baseline, err
	}
	if baseline.Outcome != "fail" {
		return baseline, errors.New("unchanged fixture incorrectly passes")
	}
	if _, err = r.Execute(ctx, c, a, c.ReferenceSolution); err != nil {
		return model.CheckResult{}, err
	}
	result, err := r.Check(ctx, c, a)
	if err == nil && result.Outcome != "pass" {
		err = errors.New("reference solution failed")
	}
	return result, err
}
