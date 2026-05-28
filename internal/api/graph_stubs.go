package api

import (
	"context"
)

// Stub types + handlers so RegisterGraphRoutes compiles before per-endpoint
// tasks land. Each Task 3-7 replaces one of these with a real handler.
type GraphStubInput struct{}
type GraphStubOutput struct {
	Body []any `json:"-"`
}
type GraphPathInput struct {
	URN string `path:"urn"`
}

func handleGraphRigs(_ context.Context, _ *GraphStubInput) (*GraphStubOutput, error) {
	return &GraphStubOutput{}, nil
}
func handleGraphEndpoints(_ context.Context, _ *GraphStubInput) (*GraphStubOutput, error) {
	return &GraphStubOutput{}, nil
}
func handleGraphEndpointConsumers(_ context.Context, _ *GraphPathInput) (*GraphStubOutput, error) {
	return &GraphStubOutput{}, nil
}
func handleGraphBlast(_ context.Context, _ *GraphStubInput) (*GraphStubOutput, error) {
	return &GraphStubOutput{}, nil
}
func handleGraphCallers(_ context.Context, _ *GraphStubInput) (*GraphStubOutput, error) {
	return &GraphStubOutput{}, nil
}
