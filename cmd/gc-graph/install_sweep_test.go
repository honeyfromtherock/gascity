package main

import (
	"strings"
	"testing"
)

func TestSweepPlistSubstitutesTokens(t *testing.T) {
	out := sweepPlist("/usr/local/bin/gc-graph", 900, "/home/u/.codegraph/sweep.log")
	if strings.Contains(out, "__GC_GRAPH__") || strings.Contains(out, "__INTERVAL__") || strings.Contains(out, "__LOG__") {
		t.Errorf("unsubstituted token remains:\n%s", out)
	}
	for _, want := range []string{
		"/usr/local/bin/gc-graph",
		"<integer>900</integer>",
		"/home/u/.codegraph/sweep.log",
		"com.gridbase.codegraph.sweep",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plist missing %q after substitution", want)
		}
	}
}

func TestSweepLogPathUnderCodegraphHome(t *testing.T) {
	p := sweepLogPath()
	if !strings.HasSuffix(p, "/.codegraph/sweep.log") {
		t.Errorf("sweepLogPath = %q, want it to end with /.codegraph/sweep.log", p)
	}
}
