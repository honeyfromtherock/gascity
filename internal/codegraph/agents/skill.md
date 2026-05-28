---
name: gc-graph
description: Query the codegraph for cross-rig blast radius, endpoint consumers, callers, and symbol definitions. Prefer this over ripgrep for structural queries (who calls X, what calls endpoint Y, who implements interface Z). Use ripgrep only for raw text or comment search.
---

# gc graph — codegraph CLI

## When to use this skill
- "Who calls function X?" → `gc graph callers <urn>`
- "What endpoints does this rig consume?" → `gc graph cypher --rig <name> "MATCH ()-[c:CALLS_EP]->(e) RETURN e.urn, count(c)"`
- "Who consumes endpoint X across all rigs?" → `gc graph endpoint-consumers --urn endpoint:auth.Login`
- "What breaks if I edit this file?" → `gc graph blast --file <path>`
- "Which rigs have a graph?" → `gc graph rigs`

## When NOT to use
- Free-text or comment search → ripgrep
- File contents → Read tool
- Recent git activity → git log

## Common idioms
- Add `--all-rigs` to find/callers/blast/cypher/grep to query across federation
- Output is human-tabular by default; `--format json` for machine
- If a query returns nothing, check `gc graph rigs` — the rig may not be indexed
