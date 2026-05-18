package discover_test

import (
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/discover"
)

func TestDiscover_DetectsLangsAndExcludes(t *testing.T) {
	rig, err := discover.Discover("testdata/sample-rig")
	if err != nil {
		t.Fatal(err)
	}

	wantLangs := map[string]bool{"go": true, "typescript": true, "python": true, "sql": true}
	got := map[string]bool{}
	for _, l := range rig.Langs {
		got[l] = true
	}
	for l := range wantLangs {
		if !got[l] {
			t.Errorf("missing lang %s; got %v", l, rig.Langs)
		}
	}

	// .worktrees must be excluded ALWAYS (hard-coded)
	if !rig.IsExcluded(".worktrees/wt1") {
		t.Error(".worktrees/wt1 should be excluded by default")
	}
	// docs/ from .codegraphignore
	if !rig.IsExcluded("docs/foo.md") {
		t.Error("docs/foo.md should be excluded by .codegraphignore")
	}
	// node_modules must be excluded ALWAYS
	if !rig.IsExcluded("node_modules/anything") {
		t.Error("node_modules should be excluded")
	}
	// normal file should not be excluded
	if rig.IsExcluded("scripts/x.py") {
		t.Error("scripts/x.py should not be excluded")
	}
}
