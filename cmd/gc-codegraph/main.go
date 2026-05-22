// Command gc-codegraph runs the full indexer pipeline for one rig.
//
// Usage: gc-codegraph --rig <name> --root <path> --out <path> [--profile core|base] [--sha SHA]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gastownhall/gascity/internal/codegraph/discover"
	"github.com/gastownhall/gascity/internal/codegraph/facts"
	"github.com/gastownhall/gascity/internal/codegraph/load"
	"github.com/gastownhall/gascity/internal/codegraph/manifest"
	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/schema"
	"github.com/gastownhall/gascity/internal/codegraph/scip"
	"github.com/gastownhall/gascity/internal/codegraph/scrape"
	"github.com/gastownhall/gascity/internal/codegraph/store"
	"github.com/gastownhall/gascity/internal/codegraph/tier2"
	"github.com/gastownhall/gascity/internal/codegraph/transform"
)

const indexerVersion = "0.2.0"

// resolveRig consults ~/.codegraph/rigs.toml. If found, it overrides --root
// and --profile from the rig entry. If not, it synthesizes a Rig from the
// flags with tier=scip (legacy behavior).
func resolveRig(name, root, profile string) rigdir.Rig {
	path, err := rigdir.DefaultPath()
	if err == nil {
		if _, statErr := os.Stat(path); statErr == nil {
			rigs, loadErr := rigdir.Load(path)
			if loadErr == nil {
				if r, lookupErr := rigdir.Lookup(rigs, name); lookupErr == nil {
					if root == "" {
						root = r.Root
					}
					if profile == "" {
						profile = r.Profile
					}
					return rigdir.Rig{Name: name, Root: root, Tier: r.Tier, Profile: profile}
				}
			}
		}
	}
	// Default: legacy SCIP path
	return rigdir.Rig{Name: name, Root: root, Tier: "scip", Profile: profile}
}

func main() {
	var (
		rigName   = flag.String("rig", "", "rig name (for logging)")
		root      = flag.String("root", "", "rig repo root")
		out       = flag.String("out", "", "output dir for .codegraph (graph.kuzu)")
		profile   = flag.String("profile", "base", "schema profile: base | core")
		sha       = flag.String("sha", "HEAD", "commit SHA stamp")
		sqlSchema = flag.String("sql-schema", "public", "default SQL schema name for GoSQL/GORM scrapers")
	)
	flag.Parse()
	if *rigName == "" || *out == "" {
		log.Fatal("--rig and --out required")
	}

	rigEntry := resolveRig(*rigName, *root, *profile)
	*root = rigEntry.Root
	*profile = rigEntry.Profile
	if *root == "" {
		log.Fatal("--root required (no rigs.toml entry found)")
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatalf("create out dir: %v", err)
	}

	pqDir := filepath.Join(*out, "shards")
	if err := os.RemoveAll(pqDir); err != nil {
		log.Fatalf("clean shards: %v", err)
	}
	w := transform.NewWriters(pqDir)

	if rigEntry.Tier == "scip" {
		runSCIP(*rigName, *root, *sha, *sqlSchema, *out, w)
	} else if rigEntry.Tier == "endpoint" {
		log.Printf("[tier2] rig=%s root=%s", rigEntry.Name, rigEntry.Root)
		indexers := []tier2.Tier2Indexer{
			tier2.CSharpIndexer{},
			// tier2.SwiftIndexer{}, // Task 17
			// tier2.KotlinIndexer{}, // Task 19
		}
		nodes, edges, err := tier2.Run(rigEntry.Name, rigEntry.Root, indexers)
		if err != nil {
			log.Fatalf("tier2: %v", err)
		}
		for _, n := range nodes {
			w.AddNode(n)
		}
		for _, e := range edges {
			w.AddEdge(e)
		}
		log.Printf("[tier2] %d File nodes, %d CALLS_EP edges", len(nodes), len(edges))
	} else {
		log.Fatalf("unknown tier: %q", rigEntry.Tier)
	}

	// Emit uniform Manifest node before flush.
	m := manifest.Manifest{
		Rig:            rigEntry.Name,
		SHA:            *sha,
		Profile:        rigEntry.Profile,
		IndexedAt:      time.Now().UTC(),
		IndexerVersion: indexerVersion,
		Tier:           rigEntry.Tier,
	}
	w.AddNode(m.Emit())

	must(w.Flush())
	log.Printf("[transform] parquet shards in %s", pqDir)

	db, err := store.Open(filepath.Join(*out, "graph.kuzu"), store.ModeReadWrite)
	must(err)
	prof := schema.ProfileBase
	if *profile == "core" {
		prof = schema.ProfileCore
	}
	must(load.Full(db, pqDir, prof))

	if err := db.Close(); err != nil {
		log.Printf("warn: close db: %v", err)
	}
	log.Printf("[load] done")
	fmt.Println("✓ ready")
}

func runSCIP(rigName, root, sha, sqlSchema, out string, w *transform.Writers) {
	rig, err := discover.Discover(root)
	must(err)
	log.Printf("[discover] langs=%v root=%s", rig.Langs, rig.Root)

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Sidecars for URN reconciliation: collected under mu, processed after wg.Wait().
	var allFuncs []facts.NodeFact
	var encoreHandles []facts.EdgeFact
	var gosqlEdges []facts.EdgeFact
	var gormEdges []facts.EdgeFact

	addNodes := func(ns []facts.NodeFact) {
		mu.Lock()
		defer mu.Unlock()
		for _, n := range ns {
			w.AddNode(n)
		}
	}
	addEdges := func(es []facts.EdgeFact) {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range es {
			w.AddEdge(e)
		}
	}

	// Run SCIP indexers in parallel per language.
	// TypeScript is handled separately below (multi-tsconfig walk).
	for _, lang := range rig.Langs {
		if lang == "typescript" {
			continue // handled separately below
		}
		runner := pickRunner(lang)
		if runner == nil {
			continue
		}
		wg.Add(1)
		go func(l string, r scip.Runner) {
			defer wg.Done()
			indexFile := filepath.Join(out, l+".scip")
			if err := r.Run(root, indexFile); err != nil {
				log.Printf("[scip:%s] FAILED: %v", l, err)
				return
			}
			raw, err := os.ReadFile(indexFile)
			if err != nil {
				log.Printf("[scip:%s] read index: %v", l, err)
				return
			}
			nodes, edges, err := scip.Parse(raw, rigName, sha)
			if err != nil {
				log.Printf("[scip:%s] parse: %v", l, err)
				return
			}
			mu.Lock()
			for _, n := range nodes {
				w.AddNode(n)
				if n.Kind == facts.KindFunction || n.Kind == facts.KindMethod {
					allFuncs = append(allFuncs, n)
				}
			}
			for _, e := range edges {
				w.AddEdge(e)
			}
			mu.Unlock()
			log.Printf("[scip:%s] %d nodes %d edges", l, len(nodes), len(edges))
		}(lang, runner)
	}

	// TypeScript: discover all tsconfig.json files and run one indexer per
	// directory in parallel, then merge results into the shared graph.
	if hasLang(rig.Langs, "typescript") {
		tsconfigDirs := findTSConfigs(root, rig)
		log.Printf("[scip:typescript] found %d tsconfig(s)", len(tsconfigDirs))
		for i, dir := range tsconfigDirs {
			i, dir := i, dir
			wg.Add(1)
			go func() {
				defer wg.Done()
				indexFile := filepath.Join(out, fmt.Sprintf("typescript-%d.scip", i))
				if err := (scip.TypeScriptRunner{}).Run(dir, indexFile); err != nil {
					log.Printf("[scip:typescript:%s] FAILED: %v", filepath.Base(dir), err)
					return
				}
				raw, err := os.ReadFile(indexFile)
				if err != nil {
					log.Printf("[scip:typescript:%s] read: %v", filepath.Base(dir), err)
					return
				}
				nodes, edges, err := scip.Parse(raw, rigName, sha)
				if err != nil {
					log.Printf("[scip:typescript:%s] parse: %v", filepath.Base(dir), err)
					return
				}
				// scip-typescript emits paths relative to its --cwd (the
				// tsconfig directory). Re-anchor them to the repo root so
				// File.path values are consistent with Go/Python output.
				relDir, _ := filepath.Rel(root, dir)
				if relDir != "" && relDir != "." {
					rewritePaths(nodes, edges, relDir)
				}
				mu.Lock()
				for _, n := range nodes {
					w.AddNode(n)
					if n.Kind == facts.KindFunction || n.Kind == facts.KindMethod {
						allFuncs = append(allFuncs, n)
					}
				}
				for _, e := range edges {
					w.AddEdge(e)
				}
				mu.Unlock()
				log.Printf("[scip:typescript:%s] %d nodes %d edges", filepath.Base(dir), len(nodes), len(edges))
			}()
		}
	}

	// Encore scraper (always tries — produces 0 facts if no annotations).
	// HANDLES edges are held back for reconciliation; all other edges go through.
	wg.Add(1)
	go func() {
		defer wg.Done()
		nodes, edges, err := scrape.Encore(root)
		if err != nil {
			log.Printf("[encore] FAILED: %v", err)
			return
		}
		mu.Lock()
		for _, n := range nodes {
			w.AddNode(n)
		}
		for _, e := range edges {
			if e.Kind == facts.EdgeHandles {
				encoreHandles = append(encoreHandles, e)
			} else {
				w.AddEdge(e)
			}
		}
		mu.Unlock()
		log.Printf("[encore] %d nodes %d edges", len(nodes), len(edges))
	}()

	// SQL/Atlas scrapers only when sql lang detected
	if hasLang(rig.Langs, "sql") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				rel, _ := filepath.Rel(root, path)
				if rig.IsExcluded(rel) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if d.IsDir() {
					return nil
				}
				base := filepath.Base(path)
				if base != "atlas.hcl" && base != "schema.hcl" {
					return nil
				}
				nodes, edges, err := scrape.Atlas(path)
				if err != nil {
					log.Printf("[atlas] %s: FAILED: %v", rel, err)
					return nil
				}
				addNodes(nodes)
				addEdges(edges)
				log.Printf("[atlas] %s: %d nodes %d edges", rel, len(nodes), len(edges))
				return nil
			})
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, edges, err := scrape.GoSQL(root, sqlSchema)
			if err != nil {
				log.Printf("[gosql] FAILED: %v", err)
				return
			}
			mu.Lock()
			gosqlEdges = append(gosqlEdges, edges...)
			mu.Unlock()
			log.Printf("[gosql] %d edges (held for reconciliation)", len(edges))
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, edges, err := scrape.GORM(root, sqlSchema)
			if err != nil {
				log.Printf("[gorm] FAILED: %v", err)
				return
			}
			mu.Lock()
			gormEdges = append(gormEdges, edges...)
			mu.Unlock()
			log.Printf("[gorm] %d edges (held for reconciliation)", len(edges))
		}()
	}

	// SQL migrations scraper: walks for Atlas migration directories and
	// extracts cumulative schema (tables, columns, indexes, FKs).
	// Handles repos whose Atlas HCL uses Pro features that scrape.Atlas can't parse.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			if rig.IsExcluded(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			base := filepath.Base(path)
			if base != "db_migrations" && base != "migrations" {
				return nil
			}
			sqlFiles, _ := filepath.Glob(filepath.Join(path, "*.sql"))
			if len(sqlFiles) == 0 {
				return nil
			}
			nodes, edges, err := scrape.SQLMigrations(path)
			if err != nil {
				log.Printf("[sqlmigrations] %s: FAILED: %v", rel, err)
				return nil
			}
			addNodes(nodes)
			addEdges(edges)
			log.Printf("[sqlmigrations] %s: %d files, %d nodes %d edges", rel, len(sqlFiles), len(nodes), len(edges))
			return nil
		})
	}()

	wg.Wait()

	// Encore generated client + frontend call sites. The Encore client (TypeScript)
	// declares the canonical service.method endpoint surface; the frontend invokes
	// it via a `client` binding. Emit :Endpoint nodes from the client and :CALLS_EP
	// edges from any .ts/.tsx file that imports/uses the client.
	clientPaths := []string{
		"frontend/src/services/client.ts",
		"src/lib/client.ts",
		"src/generated/encore.gen.ts",
		"src/generated/client.ts",
	}
	var encoreClientPath string
	for _, rel := range clientPaths {
		p := filepath.Join(root, rel)
		if _, err := os.Stat(p); err == nil {
			encoreClientPath = p
			break
		}
	}
	if encoreClientPath != "" {
		eps, err := scrape.ScanEncoreClient(encoreClientPath)
		if err != nil {
			log.Printf("[encore-client] scan %s: %v", encoreClientPath, err)
		} else {
			for _, n := range eps {
				w.AddNode(n)
			}
			log.Printf("[encore-client] %d endpoints from %s", len(eps), encoreClientPath)
		}
		frontendDir := filepath.Join(root, "frontend", "src")
		if _, err := os.Stat(frontendDir); err == nil {
			var callSiteCount int
			_ = filepath.Walk(frontendDir, func(p string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if !(strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".tsx")) {
					return nil
				}
				if strings.HasSuffix(p, ".d.ts") {
					return nil
				}
				edges, err := scrape.ScanCallSites(p, rigName, []string{"client"})
				if err != nil {
					return nil
				}
				// Rewrite SrcURN from "<rig>:file:<abs>" to the File node's
				// primary key (path relative to rig root).
				rel, relErr := filepath.Rel(root, p)
				if relErr != nil {
					return nil
				}
				for _, e := range edges {
					e.SrcURN = rel
					w.AddEdge(e)
					callSiteCount++
				}
				return nil
			})
			log.Printf("[encore-client] %d call-site edges", callSiteCount)
		}
	}

	// Reconcile all approximate-URN edges against real SCIP function/method URNs.
	// Must run after all goroutines complete so allFuncs is fully populated.
	countMatched := func(edges []facts.EdgeFact) (matched int) {
		for _, e := range edges {
			if !isApproxEncoreURN(e.SrcURN) {
				matched++
			}
		}
		return matched
	}

	reconHandles := scrape.ReconcileEdgeSrcURNs(encoreHandles, allFuncs)
	reconGoSQL := scrape.ReconcileEdgeSrcURNs(gosqlEdges, allFuncs)
	reconGORM := scrape.ReconcileEdgeSrcURNs(gormEdges, allFuncs)

	for _, e := range reconHandles {
		w.AddEdge(e)
	}
	for _, e := range reconGoSQL {
		w.AddEdge(e)
	}
	for _, e := range reconGORM {
		w.AddEdge(e)
	}
	log.Printf("[reconcile] handles=%d(%d matched) gosql=%d(%d matched) gorm=%d(%d matched)",
		len(reconHandles), countMatched(reconHandles),
		len(reconGoSQL), countMatched(reconGoSQL),
		len(reconGORM), countMatched(reconGORM))
}

func pickRunner(lang string) scip.Runner {
	switch lang {
	case "go":
		return scip.GoRunner{}
	case "typescript":
		return scip.TypeScriptRunner{}
	case "python":
		return scip.PythonRunner{}
	}
	return nil
}

func hasLang(ls []string, want string) bool {
	for _, l := range ls {
		if l == want {
			return true
		}
	}
	return false
}

// isApproxEncoreURN reports whether a URN is still an approximate
// Encore-emitted placeholder (i.e., reconciliation did not match it).
func isApproxEncoreURN(urn string) bool {
	return strings.HasPrefix(urn, "scip-go . . ")
}

// rewritePaths prepends dirPrefix to all file-related paths in nodes and edges
// so that paths produced by scip-typescript (relative to its --cwd tsconfig
// directory) are re-anchored to the repo root, matching Go/Python output.
func rewritePaths(nodes []facts.NodeFact, edges []facts.EdgeFact, dirPrefix string) {
	prefix := dirPrefix + string(filepath.Separator)
	prependPath := func(props map[string]any, key string) {
		if v, ok := props[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				props[key] = prefix + s
			}
		}
	}
	for i := range nodes {
		prependPath(nodes[i].Props, "path") // File nodes
		prependPath(nodes[i].Props, "file") // Function/Method/Class/etc nodes
		// URN for File nodes is the RelativePath itself
		if nodes[i].Kind == facts.KindFile {
			nodes[i].URN = prefix + nodes[i].URN
		}
	}
	for i := range edges {
		// DEFINED_IN edges: DstURN is the file RelativePath
		if edges[i].Kind == facts.EdgeDefinedIn && edges[i].DstKind == facts.KindFile {
			edges[i].DstURN = prefix + edges[i].DstURN
		}
		// site_file prop on CALLS edges
		if v, ok := edges[i].Props["site_file"]; ok {
			if s, ok := v.(string); ok && s != "" {
				edges[i].Props["site_file"] = prefix + s
			}
		}
	}
}

// findTSConfigs returns directories under root containing a tsconfig.json,
// respecting the rig's exclude list. Nested tsconfigs are NOT included if
// a parent already has one (scip-typescript walks recursively via project
// references).
func findTSConfigs(root string, rig *discover.Rig) []string {
	var dirs []string
	known := map[string]bool{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if rig.IsExcluded(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "tsconfig.json" {
			return nil
		}
		dir := filepath.Dir(path)
		// Skip if a parent directory is already in the list — scip-typescript
		// will handle nested projects via references.
		for k := range known {
			if strings.HasPrefix(dir, k+string(filepath.Separator)) {
				return nil
			}
		}
		known[dir] = true
		dirs = append(dirs, dir)
		return nil
	})
	sort.Strings(dirs)
	return dirs
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
