package app

import (
	"testing"
	"time"

	"github.com/stevencarpenter/driving-range/internal/model"
)

func TestSavedGitRevisionsRemainResolvable(t *testing.T) {
	for _, id := range []string{"git.unstage-preserve", "git.restore-one"} {
		t.Run(id, func(t *testing.T) {
			dir := t.TempDir()
			s, err := Open(dir, false)
			if err != nil {
				t.Fatal(err)
			}
			a := model.Attempt{ID: "saved", ExerciseID: id, Revision: 1, Track: "git", Profile: "standard", ValidatorVersion: "1", EnvironmentID: "test-image", Seed: "default", Workspace: "golf-saved", Status: "active", CreatedAt: time.Now().UTC()}
			err = s.Store.CreateAttempt(a)
			s.Close()
			if err != nil {
				t.Fatal(err)
			}
			s, err = Open(dir, false)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			saved, err := s.Store.Attempt(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			ch, err := s.resolveAttempt(saved)
			if err != nil || ch.ID != id || ch.Revision != 1 {
				t.Fatalf("saved revision: %+v, %v", ch, err)
			}
			latest, err := s.Catalog.Find(id, 0)
			if err != nil || latest.Revision != 2 {
				t.Fatalf("new practice revision: %+v, %v", latest, err)
			}
		})
	}
}
