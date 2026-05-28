package queries

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// ListEndpoints returns endpoints across the registered rigs. If rigFilter
// is non-empty, only that rig is scanned. If q is non-empty, only URNs
// containing q (substring) are returned. limit caps the total rows.
func ListEndpoints(rigs []rigdir.Rig, rigFilter, q string, limit int) []EndpointInfo {
	out := []EndpointInfo{}
	if limit <= 0 {
		return out
	}
	for _, r := range rigs {
		if rigFilter != "" && r.Name != rigFilter {
			continue
		}
		if len(out) >= limit {
			break
		}
		path := filepath.Join(r.Root, ".codegraph", "graph.kuzu")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		db, err := store.Open(path, store.ModeReadOnly)
		if err != nil {
			continue
		}
		conn := db.Connect()
		remaining := limit - len(out)
		cypher := fmt.Sprintf("MATCH (e:Endpoint) RETURN e.urn, e.transport, e.route, e.verb LIMIT %d;", remaining)
		rows, err := conn.Query(cypher)
		if err != nil {
			_ = conn.Close()
			_ = db.Close()
			continue
		}
		for rows.HasNext() {
			if len(out) >= limit {
				break
			}
			t, err := rows.Next()
			if err != nil {
				break
			}
			urn, _ := t.GetValue(0)
			transport, _ := t.GetValue(1)
			route, _ := t.GetValue(2)
			verb, _ := t.GetValue(3)
			t.Close()
			urnStr := fmt.Sprint(urn)
			if q != "" && !strings.Contains(urnStr, q) {
				continue
			}
			out = append(out, EndpointInfo{
				URN:       urnStr,
				Rig:       r.Name,
				Transport: fmt.Sprint(transport),
				Route:     fmt.Sprint(route),
				Verb:      fmt.Sprint(verb),
			})
		}
		rows.Close()
		_ = conn.Close()
		_ = db.Close()
	}
	return out
}

// EndpointConsumers returns the callers of a given endpoint URN across all
// registered rigs. Opens the first rig read-write as host, ATTACHes the
// remainder read-only, and issues a per-rig MATCH (LadybugDB cannot use
// `alias.Label` in MATCH patterns — each rig must be USEd first).
func EndpointConsumers(rigs []rigdir.Rig, urn string) []ConsumerInfo {
	out := []ConsumerInfo{}
	if len(rigs) == 0 {
		return out
	}
	hostPath := filepath.Join(rigs[0].Root, ".codegraph", "graph.kuzu")
	db, err := store.Open(hostPath, store.ModeReadWrite)
	if err != nil {
		return out
	}
	defer func() { _ = db.Close() }()
	conn := db.Connect()
	defer func() { _ = conn.Close() }()

	query := "MATCH (caller)-[c:CALLS_EP]->(e:Endpoint) WHERE e.urn = $urn " +
		"RETURN caller.path, c.site_line LIMIT 200;"

	// Host first while its scope is active (ATTACH switches active scope).
	host := rigs[0]
	collectConsumers(conn, host, query, urn, &out)

	for _, r := range rigs[1:] {
		path := filepath.Join(r.Root, ".codegraph", "graph.kuzu")
		alias := aliasName(r.Name)
		attach := fmt.Sprintf("ATTACH '%s' AS %s (DBTYPE LBUG, READ_ONLY);", path, alias)
		if err := conn.Exec(attach); err != nil {
			continue
		}
		if err := conn.Exec("USE " + alias + ";"); err != nil {
			continue
		}
		collectConsumers(conn, r, query, urn, &out)
	}
	return out
}

func collectConsumers(conn *store.Conn, r rigdir.Rig, query, urn string, out *[]ConsumerInfo) {
	rows, err := conn.Query(query, map[string]any{"urn": urn})
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.HasNext() {
		t, err := rows.Next()
		if err != nil {
			break
		}
		caller, _ := t.GetValue(0)
		lineVal, _ := t.GetValue(1)
		t.Close()
		line := 0
		switch v := lineVal.(type) {
		case int:
			line = v
		case int32:
			line = int(v)
		case int64:
			line = int(v)
		}
		*out = append(*out, ConsumerInfo{
			Rig:    r.Name,
			Tier:   r.Tier,
			Caller: fmt.Sprint(caller),
			Line:   line,
		})
	}
}
