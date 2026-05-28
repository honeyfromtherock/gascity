package api

import (
	"context"
	"net/url"

	"github.com/gastownhall/gascity/internal/codegraph/queries"
	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
)

type GraphEndpointConsumersInput struct {
	URN string `path:"urn" doc:"canonical endpoint URN, e.g. endpoint:auth.Login"`
}

type GraphEndpointConsumersOutput struct {
	Body []queries.ConsumerInfo `json:"-"`
}

func handleGraphEndpointConsumersImpl(_ context.Context, in *GraphEndpointConsumersInput) (*GraphEndpointConsumersOutput, error) {
	urn := in.URN
	if decoded, err := url.PathUnescape(urn); err == nil {
		urn = decoded
	}
	rigsPath, err := rigdir.DefaultPath()
	if err != nil {
		return &GraphEndpointConsumersOutput{Body: []queries.ConsumerInfo{}}, nil
	}
	rigs, err := rigdir.Load(rigsPath)
	if err != nil {
		return &GraphEndpointConsumersOutput{Body: []queries.ConsumerInfo{}}, nil
	}
	return &GraphEndpointConsumersOutput{Body: queries.EndpointConsumers(rigs, urn)}, nil
}
