package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkillFile(t *testing.T) {
	dir := t.TempDir()
	if err := writeSkillFile(filepath.Join(dir, ".claude", "skills")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".claude", "skills", "gc-graph.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("skill file not written: %v", err)
	}
	if !strings.Contains(string(body), "name: gc-graph") {
		t.Errorf("skill file missing frontmatter: %s", string(body))
	}
}

func TestInstallSettingsMergeIdempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	if err := mergeHooksIntoSettings(settingsPath); err != nil {
		t.Fatalf("first install: %v", err)
	}
	doc1 := readJSON(t, settingsPath)

	if err := mergeHooksIntoSettings(settingsPath); err != nil {
		t.Fatalf("second install: %v", err)
	}
	doc2 := readJSON(t, settingsPath)

	if !jsonEqual(doc1, doc2) {
		t.Errorf("install is not idempotent:\nfirst:  %v\nsecond: %v", doc1, doc2)
	}
}

func TestInstallSettingsMergePreservesUserHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	existing := `{"hooks": {"SessionStart": [{"hooks":[{"type":"command","command":"echo hi","timeout":1000}]}]}}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mergeHooksIntoSettings(settingsPath); err != nil {
		t.Fatal(err)
	}
	doc := readJSON(t, settingsPath)
	hooks := doc["hooks"].(map[string]any)
	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("user's SessionStart hook lost during merge")
	}
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("our PreToolUse hook not added")
	}
}

func TestInstallDoctrineAppendIdempotent(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "CLAUDE.md")
	original := "# Project Instructions\n\nSome user content.\n"
	if err := os.WriteFile(mdPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendDoctrine(mdPath); err != nil {
		t.Fatal(err)
	}
	after1, _ := os.ReadFile(mdPath)
	if err := appendDoctrine(mdPath); err != nil {
		t.Fatal(err)
	}
	after2, _ := os.ReadFile(mdPath)

	if string(after1) != string(after2) {
		t.Errorf("appendDoctrine is not idempotent")
	}
	if !strings.Contains(string(after1), "<!-- gc-graph:doctrine start -->") {
		t.Errorf("doctrine sentinel not added")
	}
	if !strings.Contains(string(after1), "Some user content.") {
		t.Errorf("user content lost")
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func jsonEqual(a, b map[string]any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}
