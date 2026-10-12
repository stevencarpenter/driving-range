package tui

import (
	"cmp"
	"slices"
	"time"

	"github.com/stevencarpenter/driving-range/internal/model"
)

type dailyTrack struct {
	track         string
	challenge     *model.Challenge
	assignment    *model.Assignment
	attempt       *model.Progress
	solved, total int
}

func (r dailyTrack) status() string {
	if r.challenge == nil {
		return "Complete"
	}
	if r.attempt == nil {
		return "Ready"
	}
	switch r.attempt.Attempt.Status {
	case "solved":
		return "Done today"
	case "abandoned":
		return "Retry"
	case "infrastructure_error":
		return "Check error"
	}
	if r.attempt.LastCheckOutcome == "fail" {
		return "Check failed"
	}
	return "Resume"
}

type dailyGroup struct {
	assignment model.Assignment
	challenge  model.Challenge
	attempt    model.Progress
	created    time.Time
	completed  *time.Time
	success    *model.Progress
}

// dailyTracks derives the roster from saved assignment identities, so a calendar
// change cannot replace unfinished work or a successful result.
func (m Model) dailyTracks() []dailyTrack {
	type revisionKey struct {
		id       string
		revision int
	}
	solved := make(map[revisionKey]bool)
	groups := make(map[model.Assignment]*dailyGroup)
	for i := range m.records {
		r := &m.records[i]
		a := r.Attempt
		if a.Status == "solved" {
			solved[revisionKey{a.ExerciseID, a.Revision}] = true
		}
		if a.AssignmentDate == "" || a.Profile != "standard" {
			continue
		}
		c, err := m.service.Catalog.Find(a.ExerciseID, a.Revision)
		if err != nil || c.Track != a.Track || c.Profile != a.Profile {
			continue
		}
		identity := model.Assignment{Date: a.AssignmentDate, Track: a.Track, ExerciseID: a.ExerciseID, Revision: a.Revision, Seed: a.Seed}
		g := groups[identity]
		if g == nil {
			g = &dailyGroup{assignment: identity, challenge: c, attempt: *r, created: a.CreatedAt}
			groups[identity] = g
		}
		if a.CreatedAt.Before(g.created) {
			g.created = a.CreatedAt
		}
		if a.CreatedAt.After(g.attempt.Attempt.CreatedAt) || a.CreatedAt.Equal(g.attempt.Attempt.CreatedAt) && a.ID > g.attempt.Attempt.ID {
			g.attempt = *r
		}
		if a.Status == "solved" {
			completed := time.Time{}
			if a.FinishedAt != nil {
				completed = a.FinishedAt.UTC()
			}
			if g.completed == nil || completed.Before(*g.completed) {
				g.completed = &completed
				g.success = r
			}
		}
	}
	ordered := make([]*dailyGroup, 0, len(groups))
	for _, g := range groups {
		ordered = append(ordered, g)
	}
	slices.SortFunc(ordered, func(a, b *dailyGroup) int {
		if v := a.created.Compare(b.created); v != 0 {
			return v
		}
		if v := cmp.Compare(a.assignment.Date, b.assignment.Date); v != 0 {
			return v
		}
		if v := cmp.Compare(a.assignment.ExerciseID, b.assignment.ExerciseID); v != 0 {
			return v
		}
		if v := cmp.Compare(a.assignment.Revision, b.assignment.Revision); v != 0 {
			return v
		}
		return cmp.Compare(a.assignment.Seed, b.assignment.Seed)
	})
	date := m.now().UTC().Format("2006-01-02")
	current := m.service.Catalog.Current()
	rows := make([]dailyTrack, 0, len(m.service.Catalog.Tracks()))
	for _, track := range m.service.Catalog.Tracks() {
		row := dailyTrack{track: track}
		for _, c := range current {
			if c.Track != track {
				continue
			}
			row.total++
			if solved[revisionKey{c.ID, c.Revision}] {
				row.solved++
			}
		}
		var selected *dailyGroup
		for _, g := range ordered {
			if g.assignment.Track == track && (g.completed == nil || g.completed.Format("2006-01-02") == date) {
				selected = g
				break
			}
		}

		if selected != nil {
			row.challenge, row.assignment, row.attempt = &selected.challenge, &selected.assignment, &selected.attempt
			if selected.completed != nil {
				row.attempt = selected.success
			}
		} else {
			for _, c := range current {
				if c.Track != track || solved[revisionKey{c.ID, c.Revision}] {
					continue
				}
				row.challenge = &c
				a := model.Assignment{Date: date, Track: track, ExerciseID: c.ID, Revision: c.Revision, Seed: "default"}
				if published, ok := m.service.Catalog.Today(m.now(), track); ok && published.ExerciseID == c.ID && published.Revision == c.Revision {
					a = published
				}
				row.assignment = &a
				break
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func (m Model) selectedDaily() dailyTrack {
	rows := m.dailyTracks()
	if len(rows) == 0 {
		return dailyTrack{}
	}
	return rows[max(0, min(m.selected, len(rows)-1))]
}

// exerciseProgress retains a successful overview even after a failed retry.
func (m Model) exerciseProgress(c model.Challenge) *model.Progress {
	var latest *model.Progress
	for _, r := range m.records {
		if r.Attempt.ExerciseID != c.ID || r.Attempt.Revision != c.Revision {
			continue
		}
		if r.Attempt.Status == "solved" {
			return &r
		}
		if latest == nil {
			latest = &r
		}
	}
	return latest
}

func nextFilter(values []string, current string) string {
	i := slices.Index(values, current)
	return values[(i+1)%len(values)]
}
