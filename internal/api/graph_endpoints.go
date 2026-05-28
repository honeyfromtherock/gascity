package api

import (
	"context"

	"github.com/gastownhall/gascity/internal/codegraph/queries"
	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
)

// GraphEndpointsInput holds the optional filters for the catalog query.
type GraphEndpointsInput struct {
	Rig   string `query:"rig" doc:"filter to a single rig name"`
	Q     string `query:"q" doc:"substring URN filter"`
	Limit int    `query:"limit" default:"200" doc:"max rows total (across all rigs)"`
}

type GraphEndpointsOutput struct {
	Body []queries.EndpointInfo `json:"-"`
}

func handleGraphEndpointsImpl(_ context.Context, in *GraphEndpointsInput) (*GraphEndpointsOutput, error) {
	rigsPath, err := rigdir.DefaultPath()
	if err != nil {
		return &GraphEndpointsOutput{Body: []queries.EndpointInfo{}}, nil
	}
	rigs, err := rigdir.Load(rigsPath)
	if err != nil {
		return &GraphEndpointsOutput{Body: []queries.EndpointInfo{}}, nil
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 200
	}
	return &GraphEndpointsOutput{Body: queries.ListEndpoints(rigs, in.Rig, in.Q, limit)}, nil
}
