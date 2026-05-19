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
	"sync"

	"github.com/gastownhall/gascity/internal/codegraph/discover"
	"github.com/gastownhall/gascity/internal/codegraph/facts"
	"github.com/gastownhall/gascity/internal/codegraph/load"
	"github.com/gastownhall/gascity/internal/codegraph/schema"
	"github.com/gastownhall/gascity/internal/codegraph/scip"
	"github.com/gastownhall/gascity/internal/codegraph/scrape"
	"github.com/gastownhall/gascity/internal/codegraph/store"
	"github.com/gastownhall/gascity/internal/codegraph/transform"
)

func main() {
	var (
		rigName = flag.String("rig", "", "rig name (for logging)")
		root    = flag.String("root", "", "rig repo root")
		out     = flag.String("out", "", "output dir for .codegraph (graph.kuzu + manifest.json)")
		profile = flag.String("profile", "base", "schema profile: base | core")
		sha     = flag.String("sha", "HEAD", "commit SHA stamp")
	)
	flag.Parse()
	if *root == "" || *out == "" || *rigName == "" {
		log.Fatal("--rig, --root, --out required")
	}

	rig, err := discover.Discover(*root)
	must(err)
	log.Printf("[discover] langs=%v root=%s", rig.Langs, rig.Root)

	pqDir := filepath.Join(*out, "shards")
	if err := os.RemoveAll(pqDir); err != nil {
		log.Fatalf("clean shards: %v", err)
	}
	w := transform.NewWriters(pqDir)

	var wg sync.WaitGroup
	var mu sync.Mutex
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

	// Run SCIP indexers in parallel per language
	for _, lang := range rig.Langs {
		runner := pickRunner(lang)
		if runner == nil {
			continue
		}
		wg.Add(1)
		go func(l string, r scip.Runner) {
			defer wg.Done()
			indexFile := filepath.Join(*out, l+".scip")
			if err := r.Run(*root, indexFile); err != nil {
				log.Printf("[scip:%s] FAILED: %v", l, err)
				return
			}
			raw, err := os.ReadFile(indexFile)
			if err != nil {
				log.Printf("[scip:%s] read index: %v", l, err)
				return
			}
			nodes, edges, err := scip.Parse(raw, *rigName, *sha)
			if err != nil {
				log.Printf("[scip:%s] parse: %v", l, err)
				return
			}
			addNodes(nodes)
			addEdges(edges)
			log.Printf("[scip:%s] %d nodes %d edges", l, len(nodes), len(edges))
		}(lang, runner)
	}

	// Encore scraper (always tries — produces 0 facts if no annotations)
	wg.Add(1)
	go func() {
		defer wg.Done()
		nodes, edges, err := scrape.Encore(*root)
		if err != nil {
			log.Printf("[encore] FAILED: %v", err)
			return
		}
		addNodes(nodes)
		addEdges(edges)
		log.Printf("[encore] %d nodes %d edges", len(nodes), len(edges))
	}()

	// SQL/Atlas scrapers only when sql lang detected
	if hasLang(rig.Langs, "sql") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hcl := filepath.Join(*root, "atlas.hcl")
			if _, err := os.Stat(hcl); err == nil {
				nodes, edges, err := scrape.Atlas(hcl)
				if err != nil {
					log.Printf("[atlas] FAILED: %v", err)
					return
				}
				addNodes(nodes)
				addEdges(edges)
				log.Printf("[atlas] %d nodes %d edges", len(nodes), len(edges))
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, edges, err := scrape.GoSQL(filepath.Join(*root, "backend"), "public")
			if err != nil {
				log.Printf("[gosql] FAILED: %v", err)
				return
			}
			addEdges(edges)
			log.Printf("[gosql] %d edges", len(edges))
		}()
	}

	wg.Wait()
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
		log.Fatalf("close db: %v", err)
	}
	log.Printf("[load] done")

	manifest := filepath.Join(*out, "manifest.json")
	if err := os.WriteFile(manifest, []byte(fmt.Sprintf(`{"rig":%q,"sha":%q,"profile":%q}`+"\n",
		*rigName, *sha, *profile)), 0o644); err != nil {
		log.Fatalf("write manifest: %v", err)
	}

	fmt.Println("✓ ready")
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

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
