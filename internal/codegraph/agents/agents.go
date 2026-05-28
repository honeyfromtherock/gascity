// Package agents holds the canonical sources for Claude Code skill,
// hook config, and CLAUDE.md doctrine — embedded into the gc binary
// so `gc graph install` can write them without depending on the source tree.
package agents

import (
	_ "embed"
)

//go:embed skill.md
var skillMD []byte

//go:embed hooks.json
var hooksJSON []byte

//go:embed doctrine.md
var doctrineMD []byte

// SkillMD returns the canonical Claude Code skill file content.
func SkillMD() []byte { return append([]byte(nil), skillMD...) }

// HooksJSON returns the canonical .claude/settings.json hook block content.
func HooksJSON() []byte { return append([]byte(nil), hooksJSON...) }

// DoctrineMD returns the canonical CLAUDE.md doctrine fragment.
func DoctrineMD() []byte { return append([]byte(nil), doctrineMD...) }
