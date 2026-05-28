package api

import (
	"encoding/json"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

func TestGraphBlastByURN(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0"))
	huma.Get(api, "/v0/graph/blast", handleGraphBlastImpl)

	resp := api.Get("/v0/graph/blast?urn=endpoint:auth.Login&depth=3")
	if resp.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", resp.Code, resp.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("malformed JSON: %v", err)
	}
	for _, k := range []string{"subject", "symbols", "files", "tests", "endpoints", "db_columns"} {
		if _, ok := out[k]; !ok {
			t.Errorf("blast result missing key %q in %+v", k, out)
		}
	}
}

func TestGraphBlastByFile(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0"))
	huma.Get(api, "/v0/graph/blast", handleGraphBlastImpl)

	resp := api.Get("/v0/graph/blast?file=/tmp/nonexistent.go")
	if resp.Code != 200 {
		t.Errorf("status = %d, want 200 (missing file → empty result, not error)", resp.Code)
	}
}

func TestGraphBlastMissingBoth(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0"))
	huma.Get(api, "/v0/graph/blast", handleGraphBlastImpl)

	resp := api.Get("/v0/graph/blast")
	if resp.Code < 400 || resp.Code >= 500 {
		t.Errorf("status = %d, expected 4xx for missing required input", resp.Code)
	}
}
