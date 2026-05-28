package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gastownhall/gascity/internal/codegraph/queries"
	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
)

type GraphBlastInput struct {
	File       string `query:"file" doc:"file path; mutually exclusive with urn"`
	URN        string `query:"urn" doc:"symbol URN; mutually exclusive with file"`
	Depth      int    `query:"depth" default:"3" doc:"transitive depth cap"`
	MaxResults int    `query:"max_results" default:"200" doc:"per-section row cap"`
}

type GraphBlastOutput struct {
	Body queries.BlastResult `json:"-"`
}

func handleGraphBlastImpl(_ context.Context, in *GraphBlastInput) (*GraphBlastOutput, error) {
	if in.File == "" && in.URN == "" {
		return nil, huma.Error400BadRequest("blast: either ?file= or ?urn= required")
	}
	rigsPath, err := rigdir.DefaultPath()
	if err != nil {
		return &GraphBlastOutput{Body: queries.BlastResult{}}, nil
	}
	rigs, err := rigdir.Load(rigsPath)
	if err != nil {
		return &GraphBlastOutput{Body: queries.BlastResult{}}, nil
	}
	return &GraphBlastOutput{Body: queries.Blast(rigs, in.File, in.URN, in.Depth, in.MaxResults)}, nil
}
