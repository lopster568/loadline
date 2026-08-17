# loadline

> STATUS: pre-release draft. Not published. All text pending owner review.

loadline is a standing, versioned measurement of what MCP servers cost an
agent's context window. It runs a monthly automated sweep across a curated
set of servers, counts tool-schema tokens with multiple tokenizers, and
reports the result through a stack calculator: pick servers, a client, and a
model, and get total context footprint, window share, per-server
attribution, and a cold-write/cache-read dollar pair. Costs are always
reported per client mode (naive full-load, tool search / progressive
disclosure, code mode), never collapsed into one number, because the same
stack costs a different amount depending on how the client loads it.

See `PRD.md` for the full product and execution plan, and
`docs/methodology-v0.md` for how the numbers are produced and how to
reproduce a published cell.

## Repo layout

| Path | Contents |
| --- | --- |
| `cmd/loadline/` | CLI entrypoint(s) |
| `internal/` | Go packages not meant for external import |
| `interposer/` | MCP call-logging proxy used for Tier 2 dynamic runs |
| `site/` | Calculator and leaderboard site |
| `data/` | Run artifacts and measurement results (JSON), not ignored by git |
| `governance/` | Selection rule, recusal policy, corrections log, right-of-reply process |
| `docs/` | PRD, methodology, server selection, and related reference docs |

## Status

This repository is a scaffold. No harness, calculator, or site code exists
yet. `docs/server-selection.md` and `servers.yaml` carry the ratified
initial server corpus; `docs/methodology-v0.md` carries the draft
measurement methodology, with open questions marked inline.
