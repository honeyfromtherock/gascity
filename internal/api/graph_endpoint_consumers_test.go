package api

import (
	"encoding/json"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

func TestGraphEndpointConsumersEndpoint(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0"))
	huma.Get(api, "/v0/graph/endpoints/{urn}/consumers", handleGraphEndpointConsumersImpl)

	resp := api.Get("/v0/graph/endpoints/endpoint:auth.Login/consumers")
	if resp.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", resp.Code, resp.Body.String())
	}
	var out []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("malformed JSON: %v", err)
	}
}

// URL-encoded URN (colons sometimes get encoded by clients).
func TestGraphEndpointConsumersEndpointEncodedURN(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0"))
	huma.Get(api, "/v0/graph/endpoints/{urn}/consumers", handleGraphEndpointConsumersImpl)

	resp := api.Get("/v0/graph/endpoints/endpoint%3Aauth.Login/consumers")
	if resp.Code != 200 {
		t.Errorf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
}
