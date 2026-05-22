package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// Alias returns a LadybugDB-safe alias for a rig name. Hyphens are replaced
// with underscores because LadybugDB's parser rejects hyphens in identifiers
// used by ATTACH ... AS and USE.
func Alias(rigName string) string {
	return strings.ReplaceAll(rigName, "-", "_")
}

// AttachAll runs ATTACH for each rig's graph.kuzu under the given DB. Each
// rig becomes available under the alias Alias(rig.Name). Attachments are
// read-only — callers performing federated queries should not mutate the
// attached rig databases.
//
// After attachment, callers select an attached database with `USE <alias>;`
// before issuing a MATCH against its node labels. LadybugDB does not
// support an `alias.NodeLabel` form in MATCH patterns.
// ForEachRig opens the first registered rig as host (read-write so ATTACH
// works), ATTACHes the remainder read-only, then calls fn for each rig with
// a conn already scoped to that rig (via USE <Alias(name)>;).
//
// fn receives the rig, the alias (sanitized rig name), and the *Conn. After
// fn returns, the helper restores host scope before moving to the next rig.
//
// Errors from per-rig ATTACH or USE are logged to stderr and the rig is
// skipped; errors from fn are returned.
func ForEachRig(rigs []rigdir.Rig, fn func(r rigdir.Rig, alias string, conn *store.Conn) error) error {
	if len(rigs) == 0 {
		return fmt.Errorf("no rigs registered")
	}
	hostPath := filepath.Join(rigs[0].Root, ".codegraph", "graph.kuzu")
	db, err := store.Open(hostPath, store.ModeReadWrite)
	if err != nil {
		return fmt.Errorf("open host (%s): %w", hostPath, err)
	}
	defer func() { _ = db.Close() }()
	conn := db.Connect()
	defer func() { _ = conn.Close() }()

	// Run host first while its scope is still active. LadybugDB switches
	// the active scope to whichever database was most recently ATTACHed
	// (with no way to USE back to the host by name), so we visit the host
	// before any attachments are made.
	host := rigs[0]
	if err := fn(host, Alias(host.Name), conn); err != nil {
		return fmt.Errorf("[%s]: %w", host.Name, err)
	}

	for _, r := range rigs[1:] {
		path := filepath.Join(r.Root, ".codegraph", "graph.kuzu")
		q := fmt.Sprintf("ATTACH '%s' AS %s (DBTYPE LBUG, READ_ONLY);", path, Alias(r.Name))
		if err := conn.Exec(q); err != nil {
			fmt.Fprintf(os.Stderr, "attach %s: %v (skipping)\n", r.Name, err)
			continue
		}
		// ATTACH implicitly switches active scope to the new DB, so an
		// explicit USE here is belt-and-suspenders for callers that rely
		// on the alias being current.
		if err := conn.Exec("USE " + Alias(r.Name) + ";"); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] USE %s: %v (skipping)\n", r.Name, Alias(r.Name), err)
			continue
		}
		if err := fn(r, Alias(r.Name), conn); err != nil {
			return fmt.Errorf("[%s]: %w", r.Name, err)
		}
	}
	return nil
}

// LoadRigs is a small convenience that loads the registered rigs from the
// default rigs.toml path.
func LoadRigs() ([]rigdir.Rig, error) {
	rigsPath, err := rigdir.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("rigdir: %w", err)
	}
	rigs, err := rigdir.Load(rigsPath)
	if err != nil {
		return nil, fmt.Errorf("rigdir load: %w", err)
	}
	return rigs, nil
}

func AttachAll(db *store.DB, rigs []rigdir.Rig) error {
	conn := db.Connect()
	defer func() { _ = conn.Close() }()
	for _, r := range rigs {
		path := filepath.Join(r.Root, ".codegraph", "graph.kuzu")
		q := fmt.Sprintf("ATTACH '%s' AS %s (DBTYPE LBUG, READ_ONLY);", path, Alias(r.Name))
		if err := conn.Exec(q); err != nil {
			return fmt.Errorf("attach %s (%s): %w", r.Name, path, err)
		}
	}
	return nil
}
