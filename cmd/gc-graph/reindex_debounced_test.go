package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestOpenReindexLog_CreatesAndAppends(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".codegraph"), 0o755); err != nil {
		t.Fatal(err)
	}
	w, err := openReindexLog(dir, 123)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	b, err := os.ReadFile(filepath.Join(dir, ".codegraph", ".reindex.log"))
	if err != nil {
		t.Fatalf("log not created: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, "123") {
		t.Errorf("log missing header timestamp 123: %q", s)
	}
	if !strings.Contains(s, "hello") {
		t.Errorf("log missing written content: %q", s)
	}

	// A second open appends rather than truncates.
	w2, err := openReindexLog(dir, 456)
	if err != nil {
		t.Fatal(err)
	}
	_ = w2.Close()
	b2, _ := os.ReadFile(filepath.Join(dir, ".codegraph", ".reindex.log"))
	if !strings.Contains(string(b2), "123") || !strings.Contains(string(b2), "456") {
		t.Errorf("second open should append, keeping both headers: %q", b2)
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
