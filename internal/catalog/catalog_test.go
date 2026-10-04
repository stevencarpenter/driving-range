package catalog

import (
	"bytes"
	"encoding/json"
	"maps"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRuntimeShardsPartitionEveryExercise(t *testing.T) {
	cmd := exec.Command("sh", "../../scripts/test-verify-runtime.sh")
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dispatch: %v\n%s", err, data)
	}
	var selectors []*regexp.Regexp
	for _, call := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		args := strings.Split(call, " ")
		for i, arg := range args {
			if arg == "-run" && i+1 < len(args) {
				_, selector, ok := strings.Cut(args[i+1], "/")
				if !ok {
					t.Fatalf("missing exercise selector: %q", args[i+1])
				}
				selectors = append(selectors, regexp.MustCompile(selector))
			}
		}
	}
	if len(selectors) != 5 {
		t.Fatalf("got %d curriculum shard selectors, want 5", len(selectors))
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range c.All() {
		matches := 0
		for _, selector := range selectors {
			if selector.MatchString(ch.ID) {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("%s belongs to %d runtime shards, want exactly one", ch.ID, matches)
		}
	}
}

func TestBundledCatalogAndFrozenSchedule(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Current()) != 600 || len(c.All()) != 602 {
		t.Fatalf("got %d exercises, %d revisions", len(c.Current()), len(c.All()))
	}
	counts := map[string]int{}
	for _, ch := range c.Current() {
		counts[ch.Track]++
		if ch.Track == "zsh" && ch.Editor != "zsh" {
			t.Errorf("%s launches %s instead of zsh", ch.ID, ch.Editor)
		}
		if !strings.Contains(ch.Brief, "Key reference:") {
			t.Errorf("%s lacks inline reference", ch.ID)
		}
		for _, f := range ch.Fixtures {
			if ch.Validator.Kind == "stdout" {
				starter := f.Files[ch.SubmissionFile]
				for _, line := range strings.Split(starter, "\n") {
					if line != "" && !strings.HasPrefix(line, "#") {
						t.Errorf("%s starter executes content", ch.ID)
					}
				}
			}
			if ch.Validator.Kind == "commands" && f.Setup == "" {
				t.Errorf("%s has no repository setup", ch.ID)
			}
		}
	}
	wantCounts := map[string]int{"vim": 100, "search": 100, "shell": 100, "sed": 25, "awk": 100, "fd": 20, "find": 25, "git": 35, "jj": 30, "python": 35, "zsh": 20, "fzf": 10}
	if !maps.Equal(counts, wantCounts) {
		t.Errorf("track counts %v, want %v", counts, wantCounts)
	}
	if len(c.Assignments) != 30 {
		t.Fatalf("got %d assignments", len(c.Assignments))
	}
	for _, a := range c.Assignments {
		if _, err := c.Resolve(a); err != nil {
			t.Fatal(err)
		}
	}
	// UTC date wins over the local date at either edge of the preview.
	now := time.Date(2026, 9, 13, 18, 0, 0, 0, time.FixedZone("MDT", -6*3600))
	a, ok := c.Today(now, "vim")
	if !ok || a.Date != "2026-09-14" {
		t.Fatalf("UTC assignment: %+v %v", a, ok)
	}
	if _, ok = c.Today(time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC), "vim"); ok {
		t.Fatal("expired schedule claimed a daily")
	}
	if _, ok = c.Today(now, "git"); ok {
		t.Fatal("practice-only track claimed a daily")
	}
	if _, err = c.Find("missing", 1); err == nil {
		t.Fatal("missing ID accepted")
	}
	if _, err = c.Find("vim.change-value", 2); err == nil {
		t.Fatal("missing revision accepted")
	}
}

func TestDecodeAcceptsAnEmptyStdoutVariant(t *testing.T) {
	for _, emptyIndex := range []int{0, 1} {
		c, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		for i := range c.Challenges {
			if c.Challenges[i].Validator.Kind == "stdout" {
				c.Challenges[i].Fixtures[emptyIndex].ExpectedStdout = ""
				break
			}
		}
		data, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Decode(bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDecodeRejectsInvalidContent(t *testing.T) {
	tests := map[string]func(*Catalog){
		"traversal":            func(c *Catalog) { c.Challenges[0].Fixtures[0].Files["../escape"] = "x" },
		"absolute":             func(c *Catalog) { c.Challenges[0].Fixtures[0].Files["/escape"] = "x" },
		"backslash":            func(c *Catalog) { c.Challenges[0].Fixtures[0].Files[`..\escape`] = "x" },
		"collision":            func(c *Catalog) { c.Challenges[0].Fixtures[0].Files["config.txt/child"] = "x" },
		"zero revision":        func(c *Catalog) { c.Challenges[0].Revision = 0 },
		"bad id":               func(c *Catalog) { c.Challenges[0].ID = "../vim" },
		"no hints":             func(c *Catalog) { c.Challenges[0].Hints = nil },
		"no license":           func(c *Catalog) { c.Challenges[0].License = "" },
		"duplicate exercise":   func(c *Catalog) { c.Challenges = append(c.Challenges, c.Challenges[0]) },
		"baseline tree passes": func(c *Catalog) { c.Challenges[0].Fixtures[0].ExpectedFiles = c.Challenges[0].Fixtures[0].Files },
		"all-empty stdout": func(c *Catalog) {
			for i := range c.Challenges {
				if c.Challenges[i].Validator.Kind == "stdout" {
					for j := range c.Challenges[i].Fixtures {
						c.Challenges[i].Fixtures[j].ExpectedStdout = ""
					}
					return
				}
			}
		},
		"duplicate day":               func(c *Catalog) { c.Assignments = append(c.Assignments, c.Assignments[0]) },
		"invalid date":                func(c *Catalog) { c.Assignments[0].Date = "2026-09-31" },
		"missing assignment revision": func(c *Catalog) { c.Assignments[0].Revision = 99 },
		"wrong assignment track":      func(c *Catalog) { c.Assignments[0].Track = "shell" },
		"unsupported editor":          func(c *Catalog) { c.Challenges[0].Editor = "nvmi" },
		"control path":                func(c *Catalog) { c.Challenges[0].Fixtures[0].Files["bad\x1bname"] = "x" },
		"empty hint":                  func(c *Catalog) { c.Challenges[0].Hints[0] = " " },
		"NUL argv": func(c *Catalog) {
			for i := range c.Challenges {
				if c.Challenges[i].Validator.Kind == "stdout" {
					c.Challenges[i].SubmissionArgv = []string{"bash", "bad\x00arg"}
					break
				}
			}
		},
		"wrong profile":   func(c *Catalog) { c.Challenges[0].Profile = "host" },
		"wrong validator": func(c *Catalog) { c.Challenges[0].Validator.Kind = "plugin" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			c, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			mutate(c)
			data, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = Decode(bytes.NewReader(data)); err == nil {
				t.Fatal("accepted invalid catalog")
			}
		})
	}
	for _, data := range []string{`{"unknown":1}`, `{"challenges":[],"schedule":[]} {}`, `null`, strings.Repeat(" ", 8<<20+1)} {
		if _, err := Decode(strings.NewReader(data)); err == nil {
			t.Fatal("accepted invalid JSON document")
		}
	}
}
