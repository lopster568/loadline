# loadline

> STATUS: pre-release draft. Not published. All text pending owner review.

loadline is a standing, versioned measurement of what MCP servers cost an
agent's context window. It measures on a monthly cadence, run manually today
per `docs/pc-sweep-runbook.md`, across a curated set of servers, counts
tool-schema tokens with three tokenizer adapters, and reports the result
through a stack calculator: pick servers, a client mode, and a model, and
get total context footprint, window share, per-server attribution, and a
cold-write/cache-read dollar pair. Published rows carry counts from all three
tokenizers (OpenAI o200k locally, Claude and Gemini via their token-counting
APIs); a cell whose count could not be obtained publishes as
`available: false`, never as an estimate. Costs
are always reported per client mode (naive full-load, tool search /
progressive disclosure, code mode), never collapsed into one number,
because the same stack costs a different amount depending on how the
client loads it; the calculator shows all three client modes side by side.

See `PRD.md` for the full product and execution plan, and
`docs/methodology-v0.md` for how the numbers are produced and how to
reproduce a published cell.

To reproduce a single-server measurement:

```
go run ./cmd/loadline scan --servers servers.yaml --only filesystem --out data/
```

`docs/pc-sweep-runbook.md` covers the full sweep.

## Repo layout

| Path | Contents |
| --- | --- |
| `cmd/loadline/` | CLI entrypoint(s) |
| `internal/` | Go packages not meant for external import |
| `interposer/` | MCP call-logging proxy used for Tier 2 (live task runs) |
| `site/` | Calculator and leaderboard site |
| `data/` | Published runs, raw wire artifacts, derived query sets |
| `governance/` | Recusal policy, corrections log, right-of-reply, self-submission (selection rule lives in `docs/server-selection.md`) |
| `PRD.md` | Full product and execution plan |
| `docs/` | Methodology, server selection, and related reference docs |

## Status

The Tier 1 harness, interposer, and calculator site are built and tested.
The first full sweep ran 2026-08-18 across the 15-server corpus: 12 servers
measured with all three tokenizer columns, 2 published as unreachable
(genuine upstream breakage, kept as data), and 1 (slack) pending an operator
credential. The monthly sweep is run manually today, per
`docs/pc-sweep-runbook.md`; no data has been published publicly yet.
`docs/server-selection.md` and `servers.yaml` carry the ratified initial
server corpus; `docs/methodology-v0.md` carries the measurement
methodology, with open questions marked inline.
