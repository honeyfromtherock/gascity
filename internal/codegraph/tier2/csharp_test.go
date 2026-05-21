package tier2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCSharpExtractor(t *testing.T) {
	src := "" +
		"using System.Net.Http;\n" +
		"class Sync {\n" +
		"    private readonly HttpClient _http = new HttpClient();\n" +
		"    public async Task DoIt() {\n" +
		"        var r1 = await _http.GetAsync(\"/api/work-orders\");\n" +
		"        var r2 = await _http.PostAsync(\"/api/work-orders/{id}/sync\", content);\n" +
		"        var r3 = await _http.DeleteAsync($\"/api/work-orders/{id}\");\n" +
		"    }\n" +
		"}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "Sync.cs")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	idx := CSharpIndexer{}
	calls, err := idx.ExtractEndpointCalls(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 {
		t.Fatalf("got %d calls, want 3 (%+v)", len(calls), calls)
	}
	wants := []struct {
		method  string
		url     string
		dynamic bool
	}{
		{"GET", "/api/work-orders", false},
		{"POST", "/api/work-orders/{id}/sync", false},
		{"DELETE", "/api/work-orders/{id}", true}, // $-interpolation
	}
	for i, c := range calls {
		if c.Method != wants[i].method || c.URL != wants[i].url || c.Dynamic != wants[i].dynamic {
			t.Errorf("call[%d] = %+v, want %+v", i, c, wants[i])
		}
	}
}
