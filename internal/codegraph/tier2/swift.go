package tier2

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SwiftIndexer extracts HTTP endpoint calls from Swift source.
// Patterns:
//   - URL(string: "...")  (method inferred from a nearby `httpMethod = "..."`)
//   - URLRequest(url: ...)
//   - Alamofire AF.request("url", method: .get)
type SwiftIndexer struct{}

// Language returns the language identifier for the Swift indexer.
func (SwiftIndexer) Language() string { return "swift" }

// DetectFiles walks root and returns all .swift source files, skipping
// build/dependency directories like Pods, DerivedData, and .build.
func (SwiftIndexer) DetectFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return err
		}
		if info.IsDir() {
			n := info.Name()
			if n == "Pods" || n == "DerivedData" || n == ".build" || strings.HasPrefix(n, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".swift") {
			out = append(out, p)
		}
		return nil
	})
	return out, err
}

var (
	swiftURLLiteralRe = regexp.MustCompile(`URL\s*\(\s*string\s*:\s*"([^"]+)"`)
	swiftAFRe         = regexp.MustCompile(`AF\.request\s*\(\s*"([^"]+)"\s*,\s*method\s*:\s*\.(\w+)`)
	swiftHTTPMethodRe = regexp.MustCompile(`httpMethod\s*=\s*"(\w+)"`)
	swiftInterpRe     = regexp.MustCompile(`\\\(`)
)

// ExtractEndpointCalls scans a Swift file and returns discovered HTTP call sites.
func (SwiftIndexer) ExtractEndpointCalls(path string) ([]EndpointCall, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []EndpointCall
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	type pending struct {
		line int
		url  string
	}
	var lastURL pending
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if m := swiftAFRe.FindStringSubmatch(text); m != nil {
			out = append(out, EndpointCall{
				FilePath: path,
				Line:     line,
				URL:      m[1],
				Method:   strings.ToUpper(m[2]),
				Dynamic:  swiftInterpRe.MatchString(text),
			})
			continue
		}
		if m := swiftURLLiteralRe.FindStringSubmatch(text); m != nil {
			lastURL = pending{line: line, url: m[1]}
			continue
		}
		if m := swiftHTTPMethodRe.FindStringSubmatch(text); m != nil && lastURL.url != "" {
			out = append(out, EndpointCall{
				FilePath: path,
				Line:     lastURL.line,
				URL:      lastURL.url,
				Method:   strings.ToUpper(m[1]),
				Dynamic:  swiftInterpRe.MatchString(lastURL.url),
			})
			lastURL = pending{}
		}
	}
	if lastURL.url != "" {
		out = append(out, EndpointCall{
			FilePath: path,
			Line:     lastURL.line,
			URL:      lastURL.url,
			Method:   "GET",
			Dynamic:  swiftInterpRe.MatchString(lastURL.url),
		})
	}
	return out, sc.Err()
}
