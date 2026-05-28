package api

import (
	"encoding/json"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

func TestGraphEndpointsEndpoint(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0"))
	huma.Get(api, "/v0/graph/endpoints", handleGraphEndpointsImpl)

	resp := api.Get("/v0/graph/endpoints?limit=50")
	if resp.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", resp.Code, resp.Body.String())
	}
	var out []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("malformed JSON: %v", err)
	}
	// Shape: each entry has urn + rig.
	for _, e := range out {
		for _, k := range []string{"urn", "rig"} {
			if _, ok := e[k]; !ok {
				t.Errorf("endpoint missing key %q in %+v", k, e)
			}
		}
	}
}

func TestGraphEndpointsQueryFilter(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0"))
	huma.Get(api, "/v0/graph/endpoints", handleGraphEndpointsImpl)

	// Should accept rig + q filters without erroring.
	resp := api.Get("/v0/graph/endpoints?rig=gridbase-core&q=auth&limit=20")
	if resp.Code != 200 {
		t.Errorf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
}
