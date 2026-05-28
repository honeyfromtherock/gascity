package api

import (
	"encoding/json"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

func TestGraphRigsEndpoint(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0"))
	huma.Get(api, "/v0/graph/rigs", handleGraphRigsImpl)

	resp := api.Get("/v0/graph/rigs")
	if resp.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", resp.Code, resp.Body.String())
	}
	var out []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("malformed JSON: %v\n%s", err, resp.Body.String())
	}
	// Test environment may have a real ~/.codegraph/rigs.toml on this dev box;
	// just verify shape if the list is non-empty.
	for _, r := range out {
		for _, k := range []string{"name", "tier"} {
			if _, ok := r[k]; !ok {
				t.Errorf("rig info missing key %q in %+v", k, r)
			}
		}
	}
}
