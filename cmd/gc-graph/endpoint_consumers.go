package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// cmdEndpointConsumers implements `gc graph endpoint-consumers --urn <urn>`.
// Cross-rig query: opens the first rig in ~/.codegraph/rigs.toml as host
// (read-write so ATTACH succeeds), ATTACHes the remaining rigs READ_ONLY,
// then issues a per-rig MATCH for callers of the given endpoint URN.
//
// LadybugDB does not support the `(n:alias.Label)` form, so each attached
// rig is queried in its own statement after `USE <alias>;`.
func cmdEndpointConsumers(args []string) int {
	fs := flag.NewFlagSet("endpoint-consumers", flag.ExitOnError)
	urn := fs.String("urn", "", "Canonical endpoint URN, e.g. endpoint:auth.Login")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *urn == "" {
		fmt.Fprintln(os.Stderr, "usage: gc graph endpoint-consumers --urn <urn>")
		return 2
	}

	rigsPath, err := rigdir.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rigdir: %v\n", err)
		return 1
	}
	rigs, err := rigdir.Load(rigsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rigdir load: %v\n", err)
		return 1
	}
	if len(rigs) == 0 {
		fmt.Fprintln(os.Stderr, "no rigs registered in ~/.codegraph/rigs.toml")
		return 1
	}

	hostPath := filepath.Join(rigs[0].Root, ".codegraph", "graph.kuzu")
	db, err := store.Open(hostPath, store.ModeReadWrite)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open host (%s): %v\n", hostPath, err)
		return 1
	}
	defer func() { _ = db.Close() }()

	// Attach remaining rigs one-at-a-time so a single bad alias doesn't
	// disable cross-rig querying entirely. LadybugDB's parser rejects
	// hyphenated raw identifiers in `AS <alias>`, so rigs with hyphenated
	// names will fail to attach until the quoting story is unified
	// (tracked under Phase 2 Task 16). Host rig is always queryable.
	attached := map[string]bool{rigs[0].Name: true}
	for _, r := range rigs[1:] {
		if err := AttachAll(db, []rigdir.Rig{r}); err != nil {
			fmt.Fprintf(os.Stderr, "attach %s: %v (skipping)\n", r.Name, err)
			continue
		}
		attached[r.Name] = true
	}

	conn := db.Connect()
	defer func() { _ = conn.Close() }()

	fmt.Fprintf(os.Stdout, "%-25s %-9s %-60s %s\n", "RIG", "TIER", "CALLER", "LINE")
	total := 0
	anyAttached := len(attached) > 1
	for i, r := range rigs {
		if !attached[r.Name] {
			continue
		}
		// USE switches the active database scope. If nothing else was
		// attached, the host rig is already the default — skipping USE
		// also avoids LadybugDB's hyphenated-identifier parser quirk.
		if anyAttached || i > 0 {
			if err := conn.Exec("USE " + Alias(r.Name) + ";"); err != nil {
				fmt.Fprintf(os.Stderr, "[%s] USE failed: %v\n", r.Name, err)
				continue
			}
		}
		q := "MATCH (caller)-[c:CALLS_EP]->(e:Endpoint) WHERE e.urn = $urn " +
			"RETURN caller.path, c.site_line LIMIT 200;"
		rows, err := conn.Query(q, map[string]any{"urn": *urn})
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%s] query: %v\n", r.Name, err)
			continue
		}
		for rows.HasNext() {
			t, err := rows.Next()
			if err != nil {
				break
			}
			caller, _ := t.GetValue(0)
			line, _ := t.GetValue(1)
			fmt.Fprintf(os.Stdout, "%-25s %-9s %-60s %v\n", r.Name, r.Tier, fmt.Sprint(caller), line)
			t.Close()
			total++
		}
		rows.Close()
	}
	if total == 0 {
		return 2
	}
	return 0
}
