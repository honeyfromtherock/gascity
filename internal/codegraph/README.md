# codegraph — Phase 1 setup & operations guide

This document covers first-time setup, the smoke-test invocation, and known
Phase 1 limitations for the codegraph pipeline (`gc-codegraph` + `gc-graph`).

For full design rationale and canonical query results see
[2026-05-18-codegraph-phase-1-results.md](../../../grid-city/docs/superpowers/specs/2026-05-18-codegraph-phase-1-results.md)
(grid-city repo).

---

## Required binaries

| Binary | Install |
|---|---|
| `scip-go` | `go install github.com/sourcegraph/scip-go/cmd/scip-go@latest` |
| `bun` | `curl -fsSL https://bun.sh/install \| bash` |
| `uvx` | ships with `uv` — `pip install uv` or `brew install uv` |
| `atlas` | `brew install ariga/tap/atlas` |
| `ripgrep` | `brew install ripgrep` |

All five must be on `$PATH` before running `gc-codegraph`.

---

## LadybugDB module proxy quirk

The `go-ladybug` module is hosted on `github.com/ladybugdb/` which is not
in the public Go checksum database. Set:

```bash
export GONOSUMDB='github.com/ladybugdb/*'
# or equivalently:
export GONOSUMCHECK='github.com/ladybugdb/*'
```

Add this to your shell profile so `go get` and `go build` don't fail with
"verifying github.com/ladybugdb/go-ladybug: checksum mismatch".

---

## Native library (`liblbug.dylib`)

The Go binding ships a pre-built dylib. After `go mod download` it lands at:

```
~/go/pkg/mod/github.com/ladybugdb/go-ladybug@v0.12.2/lib/dynamic/darwin/liblbug.dylib
```

`cgo` picks it up automatically via the `#cgo LDFLAGS` directive in the
binding package. If you need to rebuild from source, clone
`github.com/ladybugdb/ladybug` and follow the instructions in its
`CMakeLists.txt`; then replace the dylib in the path above.

---

## Smoke test invocation

Index a rig and verify the manifest lands:

```bash
cd ~/Source/forks/gascity

# Build both binaries
go build -o bin/gc-codegraph ./cmd/gc-codegraph
go build -o bin/gc-graph     ./cmd/gc-graph

# Index gridbase-core (~9 s cold)
./bin/gc-codegraph \
  --rig  gridbase-core \
  --root ~/Source/gridbase/core \
  --out  ~/Source/gridbase/core/.codegraph \
  --profile core \
  --sha  "$(git -C ~/Source/gridbase/core rev-parse HEAD)"

# Verify manifest
cat ~/Source/gridbase/core/.codegraph/manifest.json

# Spot-check a query
./bin/gc-graph find --root ~/Source/gridbase/core --kind Method Login
```

Expected last lines from `gc-codegraph`: `[load] done` then `✓ ready`.
Expected `manifest.json`: a single JSON line with `rig`, `sha`, `profile` keys.

---

## CLI rig resolution

`gc-graph` subcommands resolve the rig root in this order:

1. **`--root <path>` flag** (highest priority) — opens `<path>/.codegraph/graph.kuzu`
   directly, skipping all rig resolution. Use this for repos that don't have a
   `gc rig path` subcommand registered.
2. **`gc rig path <rig>`** — if the `gc` CLI has the subcommand, its output is
   used as the root.
3. **`~/Source/grid-city/assets/<rig>`** — assets-convention fallback. Symlinks
   are resolved. For `gridbase-core`, create the link once:
   ```bash
   ln -sfn ~/Source/gridbase/core ~/Source/grid-city/assets/gridbase-core
   ```

All five subcommands (`find`, `callers`, `blast`, `cypher`, `grep`) accept
`--root`. When `--root` is set, `--rig` is optional (used only for labelling
in JSON output).

---

## Known Phase 1 limitations

These are tracked followups, not design gaps. See the results doc linked above
for full context and priority ordering.

- **SIGSEGV at process exit (FU6, workaround applied)** — lbug's C finalizer
  can fire after `db.Close()` and SIGSEGV. Workaround: `manifest.json` is
  written _before_ `db.Close()` so the manifest always lands even if the close
  path crashes. Root fix pending upstream or a `runtime.SetFinalizer(res, nil)`
  audit of all `Query` call sites.
- **FTS extension unavailable (FU7)** — `LOAD EXTENSION FTS;` crashes on
  darwin/arm64 in v0.12.2. `gc-graph find` and `grep` use Cypher `CONTAINS`
  scan as a workaround. Switch to `QUERY_FTS_INDEX` when upstream fixes the
  extension-loading path.
- **Parquet BYTE_ARRAY BLOB bug (FU8)** — Ladybug's Parquet reader misreads
  STRING columns as BLOB. The loader materializes CSV intermediates to work
  around this; switch back to direct `COPY FROM Parquet` when fixed.
- **CALLS edge loss (FU4)** — only ~299 of 51k CALLS edges land because their
  destination URN is an un-indexed external-package symbol. Fix: emit
  placeholder nodes for SCIP ExternalSymbols so FK lookups resolve.
- **HANDLES edges = 0 (FU2)** — Encore scraper emits approximate URNs;
  reconciliation pass needed to match against real SCIP URNs.
- **GoSQL/GORM patterns (FU10)** — GoSQL analyzer matches only
  `encore.dev/storage/sqldb` literal-string queries; GORM `Where`/`Order`
  patterns tracked separately.
- **Atlas HCL path detection (FU1)** — pipeline globs `**/atlas.hcl`
  recursively (fixed in `740066ea`); DbTable/DbColumn nodes should now land
  for repos with nested HCL files.
- **scip-typescript (FU3)** — per-tsconfig walk added in `56be1657`; TS
  symbols now index for repos with tsconfigs outside the root.
- **DEFINED_IN edge dedup (FU5)** — duplicate edges per symbol fixed in
  `30. FU5`.
