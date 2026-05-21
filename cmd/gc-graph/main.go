// Command gc-graph queries per-rig code knowledge graphs.
//
// Phase 1: opens LadybugDB READ_ONLY directly. Phase 4 will flip to
// HTTP client talking to gas-city :9443.
//
// Usage:
//
//	gc graph find <query> --rig <name>
//	gc graph callers <urn> --rig <name> [--depth N]
//	gc graph blast <urn> --rig <name> [--depth N]
//	gc graph cypher --rig <name> "<read-only cypher>"
//	gc graph grep <symbol> --rig <name>
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "find":
		os.Exit(cmdFind(os.Args[2:]))
	case "callers":
		os.Exit(cmdCallers(os.Args[2:]))
	case "blast":
		os.Exit(cmdBlast(os.Args[2:]))
	case "cypher":
		os.Exit(cmdCypher(os.Args[2:]))
	case "grep":
		os.Exit(cmdGrep(os.Args[2:]))
	case "endpoint-consumers":
		os.Exit(cmdEndpointConsumers(os.Args[2:]))
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "gc graph {find|callers|blast|cypher|grep|endpoint-consumers} ...")
}
