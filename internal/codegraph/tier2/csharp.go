// internal/codegraph/tier2/csharp.go
package tier2

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CSharpIndexer extracts HTTP endpoint calls from C# source.
// Patterns: HttpClient.{Get,Post,Put,Delete,Patch}Async("url" or $"url").
type CSharpIndexer struct{}

func (CSharpIndexer) Language() string { return "csharp" }

func (CSharpIndexer) DetectFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			n := info.Name()
			if n == "bin" || n == "obj" || (strings.HasPrefix(n, ".") && n != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".cs") {
			out = append(out, p)
		}
		return nil
	})
	return out, err
}

var csharpHTTPRe = regexp.MustCompile(`\.(Get|Post|Put|Delete|Patch)Async\s*\(\s*(\$?)"([^"]+)"`)

func (CSharpIndexer) ExtractEndpointCalls(path string) ([]EndpointCall, error) {
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
		for _, m := range csharpHTTPRe.FindAllStringSubmatch(sc.Text(), -1) {
			dynamic := m[2] == "$"
			if !dynamic && strings.Contains(m[3], "{") && !templateOnly(m[3]) {
				dynamic = true
			}
			out = append(out, EndpointCall{
				FilePath: path,
				Line:     line,
				URL:      m[3],
				Method:   strings.ToUpper(m[1]),
				Dynamic:  dynamic,
			})
		}
	}
	return out, sc.Err()
}

// templateOnly returns true iff every {…} segment is a path template
// (only alphanumeric / underscore inside). Returns false on C# string-interp
// content like {dotted.expr} or {method()}.
func templateOnly(s string) bool {
	depth := 0
	current := strings.Builder{}
	for _, r := range s {
		switch r {
		case '{':
			depth = 1
			current.Reset()
		case '}':
			if depth == 1 {
				seg := current.String()
				for _, c := range seg {
					if !(c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
						return false
					}
				}
			}
			depth = 0
		default:
			if depth == 1 {
				current.WriteRune(r)
			}
		}
	}
	return true
}
