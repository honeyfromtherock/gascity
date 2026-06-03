package agents

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSkillMDHasFrontmatter(t *testing.T) {
	s := string(SkillMD())
	if !strings.HasPrefix(s, "---\nname: gc-graph\n") {
		t.Errorf("skill.md missing expected frontmatter")
	}
	if !strings.Contains(s, "description:") {
		t.Errorf("skill.md missing description field")
	}
}

func TestHooksJSONParses(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(HooksJSON(), &doc); err != nil {
		t.Fatalf("hooks.json malformed: %v", err)
	}
	hooks, ok := doc["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks.json missing top-level hooks key")
	}
	for _, name := range []string{"PreToolUse", "UserPromptSubmit"} {
		if _, ok := hooks[name]; !ok {
			t.Errorf("hooks.json missing %s", name)
		}
	}
}

func TestDoctrineMDHasSentinel(t *testing.T) {
	s := string(DoctrineMD())
	if !strings.Contains(s, "<!-- gc-graph:doctrine start -->") {
		t.Errorf("doctrine.md missing start sentinel")
	}
	if !strings.Contains(s, "<!-- gc-graph:doctrine end -->") {
		t.Errorf("doctrine.md missing end sentinel")
	}
}

func TestGitHookShHasSentinelsAndToken(t *testing.T) {
	s := string(GitHookSh())
	if !strings.Contains(s, "# --- BEGIN CODEGRAPH v1 ---") {
		t.Errorf("githook.sh missing start sentinel")
	}
	if !strings.Contains(s, "# --- END CODEGRAPH v1 ---") {
		t.Errorf("githook.sh missing end sentinel")
	}
	if !strings.Contains(s, "__RIG_NAME__") {
		t.Errorf("githook.sh missing __RIG_NAME__ substitution token")
	}
	if !strings.Contains(s, "reindex-debounced") {
		t.Errorf("githook.sh should call reindex-debounced")
	}
}
