package api

import (
	"context"

	"github.com/gastownhall/gascity/internal/codegraph/queries"
	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
)

// GraphRigsInput has no parameters.
type GraphRigsInput struct{}

// GraphRigsOutput wraps the rig list.
type GraphRigsOutput struct {
	Body []queries.RigInfo `json:"-"`
}

// handleGraphRigsImpl returns the registered rigs and their manifest data.
// Returns an empty list (200) if rigs.toml is missing — never errors on
// missing config; the dashboard interprets the empty list as "no rigs
// registered yet."
func handleGraphRigsImpl(_ context.Context, _ *GraphRigsInput) (*GraphRigsOutput, error) {
	rigsPath, err := rigdir.DefaultPath()
	if err != nil {
		return &GraphRigsOutput{Body: []queries.RigInfo{}}, nil
	}
	rigs, err := rigdir.Load(rigsPath)
	if err != nil {
		return &GraphRigsOutput{Body: []queries.RigInfo{}}, nil
	}
	return &GraphRigsOutput{Body: queries.ListRigs(rigs)}, nil
}
