package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestRunDebounced_ReindexesWhenNewest(t *testing.T) {
	dir := t.TempDir()
	cg := filepath.Join(dir, ".codegraph")
	if err := os.MkdirAll(cg, 0o755); err != nil {
		t.Fatal(err)
	}
	reindexed := false
	did, err := runDebounced(dir, 100, time.Millisecond, func(time.Duration) {}, func() error {
		reindexed = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !did || !reindexed {
		t.Errorf("expected reindex (did=%v reindexed=%v)", did, reindexed)
	}
	b, _ := os.ReadFile(filepath.Join(cg, ".last-trigger"))
	if string(b) != "100" {
		t.Errorf(".last-trigger = %q, want 100", string(b))
	}
}

func TestRunDebounced_SkipsWhenSuperseded(t *testing.T) {
	dir := t.TempDir()
	cg := filepath.Join(dir, ".codegraph")
	if err := os.MkdirAll(cg, 0o755); err != nil {
		t.Fatal(err)
	}
	reindexed := false
	sleep := func(time.Duration) {
		_ = os.WriteFile(filepath.Join(cg, ".last-trigger"), []byte("200"), 0o644)
	}
	did, err := runDebounced(dir, 100, time.Millisecond, sleep, func() error {
		reindexed = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if did || reindexed {
		t.Errorf("expected skip (did=%v reindexed=%v)", did, reindexed)
	}
}

func TestRunDebounced_LatestOfFlurryWins(t *testing.T) {
	dir := t.TempDir()
	cg := filepath.Join(dir, ".codegraph")
	_ = os.MkdirAll(cg, 0o755)
	for _, ts := range []int64{100, 200, 300} {
		_ = os.WriteFile(filepath.Join(cg, ".last-trigger"), []byte(strconv.FormatInt(ts, 10)), 0o644)
	}
	reindexCount := 0
	did, err := runDebounced(dir, 300, time.Millisecond, func(time.Duration) {}, func() error {
		reindexCount++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !did || reindexCount != 1 {
		t.Errorf("latest (300) should reindex once (did=%v count=%d)", did, reindexCount)
	}
}
