package tier2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKotlinExtractor(t *testing.T) {
	src := "" +
		"package com.gridbase.api\n" +
		"import retrofit2.http.*\n" +
		"import okhttp3.OkHttpClient\n" +
		"import okhttp3.Request\n" +
		"\n" +
		"interface API {\n" +
		"    @GET(\"/api/work-orders\")\n" +
		"    suspend fun getOrders(): List<Order>\n" +
		"\n" +
		"    @POST(\"/api/auth/login\")\n" +
		"    suspend fun login(@Body body: LoginRequest): LoginResponse\n" +
		"\n" +
		"    @DELETE(\"/api/work-orders/{id}\")\n" +
		"    suspend fun delete(@Path(\"id\") id: String)\n" +
		"}\n" +
		"\n" +
		"class Direct {\n" +
		"    fun ping() {\n" +
		"        val client = OkHttpClient()\n" +
		"        val req = Request.Builder().url(\"/api/health\").build()\n" +
		"        client.newCall(req).execute()\n" +
		"    }\n" +
		"}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "API.kt")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	idx := KotlinIndexer{}
	calls, err := idx.ExtractEndpointCalls(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 {
		t.Fatalf("got %d calls, want 4 (%+v)", len(calls), calls)
	}
	wantMethods := map[string]string{
		"/api/work-orders":      "GET",
		"/api/auth/login":       "POST",
		"/api/work-orders/{id}": "DELETE",
		"/api/health":           "GET",
	}
	for _, c := range calls {
		if wantMethods[c.URL] != c.Method {
			t.Errorf("URL %q method=%q, want %q", c.URL, c.Method, wantMethods[c.URL])
		}
	}
}
