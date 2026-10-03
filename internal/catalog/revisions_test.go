package catalog

import (
	"slices"
	"testing"

	"github.com/stevencarpenter/driving-range/internal/model"
)

func TestCurrentSelectsLatestWithoutChangingHistory(t *testing.T) {
	c := &Catalog{Challenges: []model.Challenge{
		{ID: "git.first", Revision: 1},
		{ID: "git.second", Revision: 2},
		{ID: "git.first", Revision: 2},
		{ID: "git.second", Revision: 1},
	}}
	current := c.Current()
	if len(current) != 2 || current[0].ID != "git.first" || current[1].ID != "git.second" {
		t.Fatalf("current order: %+v", current)
	}
	for _, ch := range current {
		if ch.Revision != 2 {
			t.Fatalf("old revision listed: %+v", ch)
		}
	}
	current[0].Revision = 99
	if len(c.All()) != 4 {
		t.Fatal("historical definitions removed")
	}
	for _, revision := range []int{1, 2} {
		ch, err := c.Find("git.first", revision)
		if err != nil || ch.Revision != revision {
			t.Fatalf("exact lookup: %+v %v", ch, err)
		}
	}
}

func TestCurrentGitRevisions(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"git.unstage-preserve", "git.restore-one"} {
		current := c.Current()
		i := slices.IndexFunc(current, func(ch model.Challenge) bool { return ch.ID == id })
		if i < 0 || current[i].Revision != 2 {
			t.Fatalf("current practice missing revision 2: %s", id)
		}
		for _, revision := range []int{1, 2} {
			if _, err := c.Find(id, revision); err != nil {
				t.Fatal(err)
			}
		}
	}
}
