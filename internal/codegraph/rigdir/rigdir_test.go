package rigdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMinimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rigs.toml")
	if err := os.WriteFile(path, []byte(`
[[rig]]
name = "core"
root = "~/Source/core"
tier = "scip"
profile = "core"

[[rig]]
name = "ios"
root = "/abs/path/ios"
tier = "endpoint"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rigs, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rigs) != 2 {
		t.Fatalf("got %d rigs, want 2", len(rigs))
	}
	if rigs[0].Name != "core" || rigs[0].Tier != "scip" || rigs[0].Profile != "core" {
		t.Errorf("rig[0] = %+v", rigs[0])
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "Source/core")
	if rigs[0].Root != want {
		t.Errorf("rig[0].Root = %q, want %q (tilde expansion)", rigs[0].Root, want)
	}
	if rigs[1].Root != "/abs/path/ios" {
		t.Errorf("rig[1].Root absolute path mangled: %q", rigs[1].Root)
	}
}

func TestLoadRejectsDuplicateName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rigs.toml")
	if err := os.WriteFile(path, []byte(`
[[rig]]
name = "dup"
root = "/a"
tier = "scip"

[[rig]]
name = "dup"
root = "/b"
tier = "scip"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestLoadRejectsInvalidTier(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rigs.toml")
	if err := os.WriteFile(path, []byte(`
[[rig]]
name = "x"
root = "/a"
tier = "magic"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid tier")
	}
}

func TestLoadCanonicalFrom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rigs.toml")
	if err := os.WriteFile(path, []byte(`
[[rig]]
name = "core"
root = "/abs/core"
tier = "scip"

[[rig]]
name = "ios"
root = "/abs/ios"
tier = "endpoint"
canonical_from = "core"

[[rig]]
name = "autocad"
root = "/abs/autocad"
tier = "endpoint"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rigs, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	wantCanonical := map[string]string{
		"core":    "",
		"ios":     "core",
		"autocad": "",
	}
	for _, r := range rigs {
		if r.CanonicalFrom != wantCanonical[r.Name] {
			t.Errorf("rig %q CanonicalFrom = %q, want %q", r.Name, r.CanonicalFrom, wantCanonical[r.Name])
		}
	}
}
