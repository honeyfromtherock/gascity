// Package internal/api — graph route registration.
//
// /v0/graph/* endpoints expose the codegraph for the dashboard and any
// future client. Routes are global (no /v0/city/{cityName}/ prefix) because
// rigs are per-machine via ~/.codegraph/rigs.toml, not per-city.
package api

import (
	"github.com/danielgtaylor/huma/v2"
)

// RegisterGraphRoutes wires the /v0/graph/* endpoints onto the supervisor mux.
// Called once from huma_handlers_supervisor.go.
func RegisterGraphRoutes(sm *SupervisorMux) {
	huma.Get(sm.humaAPI, "/v0/graph/rigs", handleGraphRigsImpl)
	huma.Get(sm.humaAPI, "/v0/graph/endpoints", handleGraphEndpointsImpl)
	huma.Get(sm.humaAPI, "/v0/graph/endpoints/{urn}/consumers", handleGraphEndpointConsumers)
	huma.Get(sm.humaAPI, "/v0/graph/blast", handleGraphBlast)
	huma.Get(sm.humaAPI, "/v0/graph/callers", handleGraphCallers)
}
