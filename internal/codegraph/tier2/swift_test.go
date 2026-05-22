package tier2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwiftExtractor(t *testing.T) {
	src := "" +
		"import Foundation\n" +
		"import Alamofire\n" +
		"class API {\n" +
		"    func login() {\n" +
		"        let url = URL(string: \"/api/auth/login\")!\n" +
		"        var req = URLRequest(url: url)\n" +
		"        req.httpMethod = \"POST\"\n" +
		"        URLSession.shared.dataTask(with: req) { _, _, _ in }.resume()\n" +
		"\n" +
		"        AF.request(\"/api/work-orders\", method: .get).response { _ in }\n" +
		"        AF.request(\"\\(base)/api/users/\\(id)\", method: .delete).response { _ in }\n" +
		"    }\n" +
		"}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "API.swift")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	idx := SwiftIndexer{}
	calls, err := idx.ExtractEndpointCalls(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) < 3 {
		t.Fatalf("got %d calls, want >=3 (%+v)", len(calls), calls)
	}
	var sawDynamic bool
	for _, c := range calls {
		if c.Dynamic {
			sawDynamic = true
		}
	}
	if !sawDynamic {
		t.Error("expected at least one Dynamic=true call (Alamofire \\(…) interp)")
	}
}
