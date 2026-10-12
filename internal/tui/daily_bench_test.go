package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/stevencarpenter/driving-range/internal/app"
	"github.com/stevencarpenter/driving-range/internal/catalog"
	"github.com/stevencarpenter/driving-range/internal/model"
)

func BenchmarkTodayHistory(b *testing.B) {
	cat, err := catalog.Load()
	if err != nil {
		b.Fatal(err)
	}
	now := time.Date(2026, 10, 8, 18, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -2)
	current := cat.Current()
	for _, count := range []int{0, 1000, 10000} {
		records := make([]model.Progress, count)
		for i := range records {
			c := current[0]
			status, date, finished := "active", "", (*time.Time)(nil)
			created, outcome := now, "fail"
			if i%2 == 0 {
				status = "abandoned"
				finished = &now
			}
			if i >= count-len(current) {
				c = current[i-(count-len(current))]
				status, date, finished = "solved", old.Format(time.DateOnly), &old
				created, outcome = old, "pass"
			}
			records[i] = model.Progress{Attempt: model.Attempt{ID: fmt.Sprint(i), ExerciseID: c.ID, Revision: c.Revision, Track: c.Track, Profile: c.Profile, AssignmentDate: date, Seed: "default", Status: status, CreatedAt: created, FinishedAt: finished}, LastCheckOutcome: outcome}
		}
		for _, width := range []int{80, 112} {
			b.Run(fmt.Sprintf("attempts=%d/width=%d", count, width), func(b *testing.B) {
				m := Model{service: &app.Service{Catalog: cat, Config: app.Config{Theme: "plain"}}, records: records, historyLoaded: true, width: width, height: 24, now: func() time.Time { return now }}
				b.ReportAllocs()
				for b.Loop() {
					m.View()
				}
			})
		}
	}
}
