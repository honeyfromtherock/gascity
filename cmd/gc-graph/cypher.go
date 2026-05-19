package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gastownhall/gascity/cmd/gc-graph/internal"
)

// cmdCypher implements `gc graph cypher --rig <name> "<read-only cypher>"`.
// Best-effort regex gate rejects write-shaped queries; the RO connection
// is the real safety net.
func cmdCypher(args []string) int {
	fs := flag.NewFlagSet("cypher", flag.ExitOnError)
	rig := fs.String("rig", "", "")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *rig == "" || fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, `usage: gc graph cypher --rig <name> "<read-only cypher>"`)
		return 2
	}
	q := fs.Arg(0)
	if isWriteShaped(q) {
		fmt.Fprintln(os.Stderr, "refusing write-shaped Cypher; gc graph cypher is read-only")
		return 1
	}
	db, err := internal.OpenRig(*rig)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = db.Close() }()
	c := db.Connect()
	defer func() { _ = c.Close() }()
	rows, err := c.Query(q)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer rows.Close()
	numCols := rows.GetNumberOfColumns()
	count := 0
	for rows.HasNext() {
		t, err := rows.Next()
		if err != nil {
			break
		}
		parts := make([]string, 0, numCols)
		for i := uint64(0); i < numCols; i++ {
			v, _ := t.GetValue(i)
			parts = append(parts, fmt.Sprint(v))
		}
		fmt.Println(strings.Join(parts, "\t"))
		t.Close()
		count++
	}
	if count == 0 {
		return 2
	}
	return 0
}

// isWriteShaped is a best-effort guard against write-shaped Cypher queries.
// The read-only DB connection is the real enforcement mechanism.
func isWriteShaped(q string) bool {
	u := strings.ToUpper(strings.ReplaceAll(q, "\n", " "))
	for _, kw := range []string{
		"CREATE ", "MERGE ", "DELETE ", "DETACH ", "SET ", "DROP ",
		"COPY ", "ALTER ", "REMOVE ", "CALL CREATE_", "CALL DROP_",
	} {
		if strings.Contains(u, kw) {
			return true
		}
	}
	return false
}
