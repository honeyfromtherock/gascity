// Package scrape — Encore-generated TypeScript client walker.
//
// The real Encore v1.56.x TS client uses:
//
//	export namespace <svc> {
//	    export class ServiceClient {
//	        constructor(baseClient: BaseClient) { ... }
//	        public async <Method>(params): Promise<...> { ... }
//	    }
//	}
//
// We extract one :Endpoint per (namespace, public async method) pair, and we
// walk caller TS files for `<binding>.<service>.<method>(` to emit :CALLS_EP.
package scrape

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

var (
	namespaceRe = regexp.MustCompile(`^\s*export\s+namespace\s+(\w+)\s*\{`)
	classRe     = regexp.MustCompile(`^\s*export\s+class\s+ServiceClient\b`)
	// Methods are declared `public async Name(` or `public Name(`.
	// Excludes `constructor(`, `private`, and the `this.X = this.X.bind(this)`
	// lines (those don't start with `public`).
	methodRe = regexp.MustCompile(`^\s*public\s+(?:async\s+)?(\w+)\s*\(`)
)

// ScanEncoreClient parses a generated Encore client TS file and emits a
// :Endpoint NodeFact per (namespace, method) pair declared in a
// ServiceClient class.
func ScanEncoreClient(path string) ([]facts.NodeFact, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var out []facts.NodeFact
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)

	var ns string
	nsDepth := 0
	inServiceClient := false
	svcDepth := 0
	for sc.Scan() {
		line := sc.Text()

		// Open a new namespace.
		if ns == "" {
			if m := namespaceRe.FindStringSubmatch(line); m != nil {
				ns = m[1]
				nsDepth = 1
				inServiceClient = false
				svcDepth = 0
				continue
			}
			continue
		}

		// Inside a namespace — track brace depth so we know when it closes.
		opens := strings.Count(line, "{")
		closes := strings.Count(line, "}")

		if !inServiceClient {
			if classRe.MatchString(line) {
				inServiceClient = true
				// Class body opens on this line; count its `{` here so
				// svcDepth reflects the class scope from the start.
				svcDepth = opens - closes
			}
		} else {
			if m := methodRe.FindStringSubmatch(line); m != nil {
				method := m[1]
				urn := fmt.Sprintf("endpoint:%s.%s", ns, method)
				out = append(out, facts.NodeFact{
					Kind: facts.KindEndpoint,
					URN:  urn,
					Props: map[string]any{
						"service": ns,
						"method":  method,
						"source":  "encore-client",
					},
				})
			}
			// Track class brace depth; when it returns to 0 after entering,
			// the class body has closed.
			svcDepth += opens - closes
			if svcDepth <= 0 && closes > 0 {
				inServiceClient = false
			}
		}

		nsDepth += opens - closes
		if nsDepth <= 0 {
			ns = ""
			inServiceClient = false
		}
	}
	return out, sc.Err()
}

// ScanCallSites walks a TS file for `<binding>.<service>.<method>(` patterns
// and emits :CALLS_EP edges. SrcURN is "<rig>:file:<abs>"; DstURN is the
// canonical "endpoint:<service>.<method>".
func ScanCallSites(path, rig string, bindings []string) ([]facts.EdgeFact, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	if len(bindings) == 0 {
		return nil, nil
	}
	pattern := fmt.Sprintf(`(?:\b)(?:%s)\.(\w+)\.(\w+)\s*\(`, strings.Join(bindings, "|"))
	re := regexp.MustCompile(pattern)

	abs := path
	if a, err := filepath.Abs(path); err == nil {
		abs = a
	}
	fileURN := fmt.Sprintf("%s:file:%s", rig, abs)

	var out []facts.EdgeFact
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		for _, m := range re.FindAllStringSubmatch(sc.Text(), -1) {
			out = append(out, facts.EdgeFact{
				Kind:    facts.EdgeCallsEP,
				SrcKind: facts.KindFile,
				SrcURN:  fileURN,
				DstKind: facts.KindEndpoint,
				DstURN:  fmt.Sprintf("endpoint:%s.%s", m[1], m[2]),
				Props: map[string]any{
					"site_line": line,
					"site_file": abs,
				},
			})
		}
	}
	return out, sc.Err()
}
