package main

import "testing"

func TestIsStale(t *testing.T) {
	cases := []struct {
		name             string
		gitSha           string
		manifestSha      string
		graphHasManifest bool
		want             bool
	}{
		{"equal shas → fresh", "abc123", "abc123", true, false},
		{"different shas → stale", "abc123", "def456", true, true},
		{"no manifest → stale", "abc123", "", false, true},
		{"no manifest even if shas would match → stale", "abc123", "abc123", false, true},
		{"non-git repo vs indexed → stale", "none", "abc123", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isStale(c.gitSha, c.manifestSha, c.graphHasManifest); got != c.want {
				t.Errorf("isStale(%q,%q,%v) = %v, want %v", c.gitSha, c.manifestSha, c.graphHasManifest, got, c.want)
			}
		})
	}
}
