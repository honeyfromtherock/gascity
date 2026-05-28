package api

import (
	"encoding/json"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

func TestGraphCallersEndpoint(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0"))
	huma.Get(api, "/v0/graph/callers", handleGraphCallersImpl)

	resp := api.Get("/v0/graph/callers?urn=endpoint:auth.Login&depth=3")
	if resp.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", resp.Code, resp.Body.String())
	}
	var out []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("malformed JSON: %v", err)
	}
}

func TestGraphCallersMissingURN(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0"))
	huma.Get(api, "/v0/graph/callers", handleGraphCallersImpl)

	resp := api.Get("/v0/graph/callers")
	// Huma will reject missing required query param with 4xx via validation.
	if resp.Code < 400 || resp.Code >= 500 {
		t.Errorf("status = %d, expected 4xx for missing required urn", resp.Code)
	}
}
