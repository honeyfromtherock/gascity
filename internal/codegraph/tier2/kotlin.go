package tier2

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// KotlinIndexer extracts HTTP endpoint calls from Kotlin source.
// Patterns:
//   - Retrofit @GET/@POST/@PUT/@DELETE/@PATCH("url")
//   - OkHttp Request.Builder().url("url").build() — defaults to GET
type KotlinIndexer struct{}

// Language returns the language identifier for the Kotlin indexer.
func (KotlinIndexer) Language() string { return "kotlin" }

// DetectFiles walks root and returns all .kt files, skipping build/IDE dirs.
func (KotlinIndexer) DetectFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return err
		}
		if info.IsDir() {
			n := info.Name()
			if n == "build" || n == ".gradle" || n == ".idea" || strings.HasPrefix(n, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".kt") {
			out = append(out, p)
		}
		return nil
	})
	return out, err
}

var (
	kotlinRetrofitRe = regexp.MustCompile(`@(GET|POST|PUT|DELETE|PATCH)\s*\(\s*"([^"]+)"`)
	kotlinOkHttpRe   = regexp.MustCompile(`\.url\s*\(\s*"([^"]+)"`)
)

// ExtractEndpointCalls scans the file for Retrofit annotations and OkHttp
// Request.Builder().url(...) calls, returning one EndpointCall per match.
func (KotlinIndexer) ExtractEndpointCalls(path string) ([]EndpointCall, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []EndpointCall
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		for _, m := range kotlinRetrofitRe.FindAllStringSubmatch(text, -1) {
			out = append(out, EndpointCall{
				FilePath: path,
				Line:     line,
				URL:      m[2],
				Method:   strings.ToUpper(m[1]),
			})
		}
		for _, m := range kotlinOkHttpRe.FindAllStringSubmatch(text, -1) {
			out = append(out, EndpointCall{
				FilePath: path,
				Line:     line,
				URL:      m[1],
				Method:   "GET",
			})
		}
	}
	return out, sc.Err()
}
