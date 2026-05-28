package api

import (
	"context"

	"github.com/gastownhall/gascity/internal/codegraph/queries"
	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
)

type GraphCallersInput struct {
	URN        string `query:"urn" required:"true" doc:"symbol URN to find callers of"`
	Depth      int    `query:"depth" default:"3"`
	MaxResults int    `query:"max_results" default:"200"`
}

type GraphCallersOutput struct {
	Body []queries.CallerInfo `json:"-"`
}

func handleGraphCallersImpl(_ context.Context, in *GraphCallersInput) (*GraphCallersOutput, error) {
	rigsPath, err := rigdir.DefaultPath()
	if err != nil {
		return &GraphCallersOutput{Body: []queries.CallerInfo{}}, nil
	}
	rigs, err := rigdir.Load(rigsPath)
	if err != nil {
		return &GraphCallersOutput{Body: []queries.CallerInfo{}}, nil
	}
	return &GraphCallersOutput{Body: queries.Callers(rigs, in.URN, in.Depth, in.MaxResults)}, nil
}
