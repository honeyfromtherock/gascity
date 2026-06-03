package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/agents"
)

const (
	codegraphHookStart = "# --- BEGIN CODEGRAPH v1 ---"
	codegraphHookEnd   = "# --- END CODEGRAPH v1 ---"
)

// installGitHooks appends the codegraph reindex fragment to the rig's
// post-commit and post-merge git hooks. Resolves core.hooksPath if set
// (husky/managed frameworks), else uses <rigRoot>/.git/hooks. Idempotent:
// replaces an existing CODEGRAPH section in place; preserves other sections
// (e.g. beads'). Creates the hook file with a shebang + chmod +x if absent.
func installGitHooks(rigRoot, rigName string) error {
	hookDir, err := resolveHookDir(rigRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return err
	}
	fragment := strings.ReplaceAll(string(agents.GitHookSh()), "__RIG_NAME__", rigName)
	for _, hook := range []string{"post-commit", "post-merge"} {
		if err := appendHookSection(filepath.Join(hookDir, hook), fragment); err != nil {
			return fmt.Errorf("install %s: %w", hook, err)
		}
	}
	return nil
}

// resolveHookDir returns the rig's effective git-hook directory, honoring
// core.hooksPath (relative paths resolve against the repo root).
func resolveHookDir(rigRoot string) (string, error) {
	out, err := exec.Command("git", "-C", rigRoot, "config", "--get", "core.hooksPath").Output()
	hp := strings.TrimSpace(string(out))
	if err == nil && hp != "" {
		if filepath.IsAbs(hp) {
			return hp, nil
		}
		return filepath.Join(rigRoot, hp), nil
	}
	return filepath.Join(rigRoot, ".git", "hooks"), nil
}

// appendHookSection writes the codegraph fragment into a hook file. If the
// file has an existing CODEGRAPH section, it is replaced in place; otherwise
// the fragment is appended. A shebang is added when the file is created.
func appendHookSection(path, fragment string) error {
	body := ""
	if b, err := os.ReadFile(path); err == nil {
		body = string(b)
	} else {
		body = "#!/usr/bin/env sh\n"
	}

	start := strings.Index(body, codegraphHookStart)
	end := strings.Index(body, codegraphHookEnd)
	if start >= 0 && end > start {
		newBody := body[:start] + strings.TrimRight(fragment, "\n") + body[end+len(codegraphHookEnd):]
		return writeExecutable(path, newBody)
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += fragment
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return writeExecutable(path, body)
}

// writeExecutable atomically writes content and ensures the file is +x.
func writeExecutable(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o755)
}
