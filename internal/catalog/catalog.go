// Package catalog loads the curated, versioned offline curriculum.
package catalog

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/stevencarpenter/driving-range/internal/model"
)

//go:embed data/*.json
var content embed.FS

type Catalog struct {
	Challenges  []model.Challenge  `json:"challenges"`
	Assignments []model.Assignment `json:"schedule"`
}

func Load() (*Catalog, error) {
	data, err := content.ReadFile("data/catalog.json")
	if err != nil {
		return nil, err
	}
	return Decode(bytes.NewReader(data))
}

// Decode validates an explicitly supplied pack without executing its content.
// Setup and reference scripts are trusted code and must run only in the sandbox.
func Decode(r io.Reader) (*Catalog, error) {
	data, err := io.ReadAll(io.LimitReader(r, 8<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 8<<20 {
		return nil, fmt.Errorf("catalog exceeds 8 MiB")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c Catalog
	if err = dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("catalog JSON: %w", err)
	}
	var trailing any
	if err = dec.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("catalog must contain exactly one JSON object")
	}
	if err = c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Catalog) All() []model.Challenge       { return slices.Clone(c.Challenges) }
func (c *Catalog) Schedule() []model.Assignment { return slices.Clone(c.Assignments) }
func (c *Catalog) Tracks() []string {
	result := []string{}
	for _, ch := range c.Challenges {
		if !slices.Contains(result, ch.Track) {
			result = append(result, ch.Track)
		}
	}
	slices.Sort(result)
	return result
}
func (c *Catalog) Find(id string, revision int) (model.Challenge, error) {
	var latest model.Challenge
	for _, ch := range c.Challenges {
		if ch.ID == id && (ch.Revision == revision || revision == 0 && ch.Revision > latest.Revision) {
			latest = ch
			if revision != 0 {
				return ch, nil
			}
		}
	}
	if latest.ID != "" {
		return latest, nil
	}
	return model.Challenge{}, fmt.Errorf("exercise %q revision %d not found", id, revision)
}
func (c *Catalog) Today(now time.Time, track string) (model.Assignment, bool) {
	date := now.UTC().Format(time.DateOnly)
	for _, a := range c.Assignments {
		if a.Date == date && a.Track == track {
			return a, true
		}
	}
	return model.Assignment{}, false
}
func (c *Catalog) Resolve(a model.Assignment) (model.Challenge, error) {
	ch, err := c.Find(a.ExerciseID, a.Revision)
	if err != nil {
		return ch, err
	}
	if ch.Track != a.Track || a.Revision < 1 {
		return model.Challenge{}, fmt.Errorf("assignment does not match exercise")
	}
	return ch, nil
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)

func validPath(p string) bool {
	return p != "" && len(p) <= 240 && !strings.Contains(p, "\\") && strings.IndexFunc(p, unicode.IsControl) < 0 && !path.IsAbs(p) && path.Clean(p) == p && p != "." && p != ".." && !strings.HasPrefix(p, "../")
}
func validArgv(argv []string) bool {
	if len(argv) == 0 || len(argv) > 64 || argv[0] == "" {
		return false
	}
	for _, arg := range argv {
		if strings.ContainsRune(arg, 0) || len(arg) > 64<<10 {
			return false
		}
	}
	return true
}

func (c *Catalog) Validate() error {
	if len(c.Challenges) == 0 || len(c.Challenges) > 1000 {
		return fmt.Errorf("catalog needs 1 to 1000 exercises")
	}
	seen := map[string]bool{}
	for _, ch := range c.Challenges {
		fail := func(s string) error { return fmt.Errorf("exercise %q: %s", ch.ID, s) }
		if !identifier.MatchString(ch.ID) || !identifier.MatchString(ch.Track) || ch.Revision < 1 {
			return fail("invalid identifier, track, or revision")
		}
		key := fmt.Sprintf("%s/%d", ch.ID, ch.Revision)
		if seen[key] {
			return fail("duplicate identifier/revision")
		}
		seen[key] = true
		for _, s := range []string{ch.Title, ch.Objective, ch.Brief, ch.Explanation, ch.ReferenceSolution, ch.License, ch.Author, ch.Profile, ch.Editor, ch.Validator.Version} {
			if strings.TrimSpace(s) == "" || len(s) > 64<<10 || strings.ContainsRune(s, 0) {
				return fail("missing required metadata")
			}
		}
		if !slices.Contains([]string{"nvim", "vim", "bash", "zsh"}, ch.Editor) {
			return fail("unsupported editor")
		}
		for _, list := range [][]string{ch.Tools, ch.Concepts, ch.Hints} {
			for _, s := range list {
				if strings.TrimSpace(s) == "" || len(s) > 16<<10 || strings.ContainsRune(s, 0) {
					return fail("empty or oversized tool, concept, or hint")
				}
			}
		}
		if ch.Profile != "standard" {
			return fail("unsupported profile")
		}
		if ch.Difficulty < 1 || ch.Difficulty > 5 || ch.Minutes < 1 || ch.Minutes > 60 || len(ch.Tools) == 0 || len(ch.Concepts) == 0 || len(ch.Hints) < 2 || len(ch.Fixtures) == 0 {
			return fail("invalid difficulty, duration, tools, concepts, hints, or fixtures")
		}
		if !validPath(ch.Entrypoint) {
			return fail("invalid entrypoint path")
		}
		if ch.Validator.OutputPolicy != "exact" {
			return fail("unsupported output policy")
		}
		switch ch.Validator.Kind {
		case "tree":
		case "stdout":
			if !validPath(ch.SubmissionFile) || !validArgv(ch.SubmissionArgv) || len(ch.Fixtures) < 2 {
				return fail("stdout requires a submission and at least two fixtures")
			}
		case "commands":
			if len(ch.Validator.Checks) == 0 {
				return fail("commands validator needs checks")
			}
			for _, check := range ch.Validator.Checks {
				if check.Name == "" || !validArgv(check.Argv) || check.ExitCode < 0 || check.ExitCode > 255 {
					return fail("invalid command check")
				}
			}
		default:
			return fail("unsupported validator kind")
		}
		fixtureNames := map[string]bool{}
		for _, f := range ch.Fixtures {
			if !identifier.MatchString(f.Name) || fixtureNames[f.Name] || len(f.Files) == 0 {
				return fail("missing or duplicate fixture name/files")
			}
			fixtureNames[f.Name] = true
			if len(f.Setup) > 64<<10 || strings.ContainsRune(f.Setup, 0) || len(f.ExpectedStdout) > 1<<20 {
				return fail("oversized fixture setup or expected output")
			}
			for _, files := range []map[string]string{f.Files, f.ExpectedFiles} {
				if len(files) > 200 {
					return fail("fixture has too many files")
				}
				for name, value := range files {
					if !validPath(name) || len(value) > 1<<20 {
						return fail("unsafe fixture path or oversized content")
					}
					for other := range files {
						if strings.HasPrefix(other, name+"/") {
							return fail("fixture file/directory collision")
						}
					}
				}
			}
			if _, ok := f.Files[ch.Entrypoint]; !ok && f.Setup == "" {
				return fail("entrypoint missing from fixture")
			}
			if ch.Validator.Kind == "tree" {
				if len(f.ExpectedFiles) == 0 {
					return fail("tree fixture missing expected files")
				}
				if maps.Equal(f.Files, f.ExpectedFiles) {
					return fail("unchanged tree already solves fixture")
				}
			}
			if ch.Validator.Kind == "stdout" {
				if f.ExpectedStdout == "" {
					return fail("empty stdout would accept the no-op starter")
				}
				if _, ok := f.Files[ch.SubmissionFile]; !ok {
					return fail("submission file missing")
				}
			}
		}
	}
	days := map[string]bool{}
	for _, a := range c.Assignments {
		date, err := time.Parse(time.DateOnly, a.Date)
		if err != nil || date.Format(time.DateOnly) != a.Date || a.Revision < 1 || a.Seed == "" || len(a.Seed) > 128 {
			return fmt.Errorf("invalid daily assignment")
		}
		key := a.Date + "/" + a.Track
		if days[key] {
			return fmt.Errorf("duplicate daily assignment %s", key)
		}
		days[key] = true
		if _, err := c.Resolve(a); err != nil {
			return err
		}
	}
	return nil
}
