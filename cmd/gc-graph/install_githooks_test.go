package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeGitRepo initializes a bare-ish repo with a .git/hooks dir.
func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return dir
}

func TestInstallGitHooks_CreatesHooks(t *testing.T) {
	repo := makeGitRepo(t)
	if err := installGitHooks(repo, "gridbase-core"); err != nil {
		t.Fatal(err)
	}
	for _, hook := range []string{"post-commit", "post-merge"} {
		path := filepath.Join(repo, ".git", "hooks", hook)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s not created: %v", hook, err)
		}
		s := string(b)
		if !strings.Contains(s, "# --- BEGIN CODEGRAPH v1 ---") {
			t.Errorf("%s missing codegraph section", hook)
		}
		if !strings.Contains(s, `reindex-debounced "gridbase-core"`) {
			t.Errorf("%s did not substitute rig name", hook)
		}
		info, _ := os.Stat(path)
		if info.Mode()&0o111 == 0 {
			t.Errorf("%s not executable", hook)
		}
	}
}

func TestInstallGitHooks_Idempotent(t *testing.T) {
	repo := makeGitRepo(t)
	if err := installGitHooks(repo, "gridbase-core"); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(repo, ".git", "hooks", "post-commit"))
	if err := installGitHooks(repo, "gridbase-core"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(repo, ".git", "hooks", "post-commit"))
	if string(first) != string(second) {
		t.Errorf("install not idempotent:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if n := strings.Count(string(second), "# --- BEGIN CODEGRAPH v1 ---"); n != 1 {
		t.Errorf("expected 1 codegraph section, got %d", n)
	}
}

func TestInstallGitHooks_PreservesExistingContent(t *testing.T) {
	repo := makeGitRepo(t)
	hookPath := filepath.Join(repo, ".git", "hooks", "post-commit")
	existing := "#!/usr/bin/env sh\n# --- BEGIN BEADS INTEGRATION v1.0.0 ---\necho beads\n# --- END BEADS INTEGRATION v1.0.0 ---\n"
	if err := os.WriteFile(hookPath, []byte(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installGitHooks(repo, "gridbase-core"); err != nil {
		t.Fatal(err)
	}
	s, _ := os.ReadFile(hookPath)
	if !strings.Contains(string(s), "BEADS INTEGRATION") {
		t.Errorf("beads section lost")
	}
	if !strings.Contains(string(s), "BEGIN CODEGRAPH v1") {
		t.Errorf("codegraph section not added")
	}
}

func TestInstallGitHooks_RespectsHooksPath(t *testing.T) {
	repo := makeGitRepo(t)
	customHooks := filepath.Join(repo, ".husky")
	if err := os.MkdirAll(customHooks, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", repo, "config", "core.hooksPath", ".husky")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}
	if err := installGitHooks(repo, "gridbase-core"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(customHooks, "post-commit")); err != nil {
		t.Errorf("post-commit not installed to core.hooksPath dir: %v", err)
	}
}
