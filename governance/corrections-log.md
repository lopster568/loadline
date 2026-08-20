# Corrections Log

## 1. Purpose

This is the public, append-only record of every correction made to a published number, claim, or row on this site. It exists so a reader can see what was wrong, why, and what changed, without needing to ask.

## 2. Standing rule

Corrections are never silently applied. A published number, row, or claim is not edited in place without a corresponding entry below. If a number changes, an entry explains why before or at the same time the number changes on the site.

This rule applies regardless of how small the error is or who found it.

## 3. Append-only

Entries are added to the bottom of this log. Existing entries are never edited or removed. If an entry itself turns out to be wrong, a new entry corrects it; the original entry stays visible.

## 4. Entry format

Every entry uses this format, in this order:

```
### [YYYY-MM-DD] Short title

- Affected rows or claims: [which server, metric, or page]
- What was wrong: [the incorrect number, claim, or statement]
- Root cause: [why it was wrong: harness bug, stale data, transcription error, methodology gap, etc.]
- What changed: [the corrected number or statement, and where it now appears]
- Reported by: [name or handle, or "internal" if found by the operator]
```

A correction that changes a number cross-references the run record or artifact that supports the new value.

## 5. What counts as a correction

Any of the following requires an entry:

- A published token count, dollar figure, grade, or score that was wrong and is now fixed.
- A misattributed row (data published against the wrong server or version).
- A methodology description that did not match what the harness actually did.
- A factual claim about a server (adoption evidence, maintenance status, category) that was inaccurate.

A routine monthly refresh where a server's own numbers moved because the server itself changed is not a correction. That is a normal changelog delta, classified per `docs/methodology-v0.md` section 9, and is not logged here.

## 6. Log

### [2026-08-17] Methodology-vs-implementation mismatch, name-charset and canonical-order

- Affected rows or claims: `docs/methodology-v0.md`'s description of the name-charset rule and the canonical-order property for the canonical serialization. No published row was affected; no external release had shipped.
- What was wrong: the methodology stated a name-charset rule and a canonical-order property that the harness did not implement.
- Root cause: the methodology document was written ahead of the implementation.
- What changed: methodology versions 0.1.1 and 0.2.0 corrected the stated rule and property; the harness was fixed to match. No published external release was affected, because the mismatch was found and corrected pre-publication.
- Reported by: internal.

### [2026-08-18] `fetch` published as `unreachable` in the 2026-08-18 run, but starts on a fresh resolve the same afternoon

- Affected rows or claims: `fetch` row, run 2026-08-18, `status: unreachable`, all counts unavailable. The `postgres` row is also partly affected: its published failure is real, but the two rows are not the same failure.
- What was wrong: the run acquired `fetch` at 15:38 UTC and the server died at import. Re-checked at approximately 16:45 UTC the same day, on the same machine, with the resolver cache forced fresh (`uvx --refresh`), a clean resolve of `mcp-server-fetch` 2026.7.10 returns `mcp` 1.29.0 rather than 2.0.0 and `import mcp_server_fetch` succeeds. The published `unreachable` status therefore does not describe what a reader installing the server today would see. Separately, `fetch` and `postgres` fail differently: `fetch` breaks on `from mcp.shared.exceptions import McpError` (`mcp_server_fetch/server.py:6`), a symbol `mcp` 2.0.0 renamed to `MCPError`; `postgres` breaks on `from mcp.server.fastmcp import FastMCP` (`postgres_mcp/server.py:15`) with `ModuleNotFoundError: No module named 'mcp.server.fastmcp'`.
- Root cause: two causes. (1) Both servers declare unbounded dependencies on the MCP Python SDK (`mcp>=1.1.3` for fetch, `mcp[cli]>=1.5.0` for postgres), so the failure depends on which SDK version the resolver returns at acquisition time; `mcp` 2.0.0 is the latest release and is not yanked, so nothing upstream was withdrawn between the two checks. (2) Instrument gap: `acquisition` records whether the server package itself was pinned, but does not record the resolved versions of its dependencies, so the run artifact cannot explain why two resolves an hour apart differed.
- What changed: no published number was edited. The `fetch` row stands as measured at 15:38 UTC, with this entry attached, and the launch post states the row went stale within the hour. The instrument gap is the change: the harness will record resolved dependency versions per acquisition before the next monthly run, so a future `unreachable` row carries the resolve that produced it. `postgres` remains `unreachable` and still fails on a fresh resolve.
- Reported by: internal.

### [2026-08-20] Methodology 3.3 stated a code-mode label rule that the first Tier 2 run would have triggered wrongly

- Affected rows or claims: `docs/methodology-v0.md` section 3.3, the sentence stating when a code-mode figure loses its "modeled, not measured" label. No published row was affected; every code-mode figure on the site carries the MODELED label before and after.
- What was wrong: the rule said the label is removed per server "once a Tier 2 run for that server exists". The first full Tier 2 run (2026-08-18, 90 trials) created exactly that condition for `filesystem`, `playwright` and `github`, and the run measures no code-mode footprint: neither client it drives offers a code mode, and the interposer reads MCP wire traffic, not what a client loaded into the model's context (`docs/tier2-task-suites.md` 4.3). Applied as written, the rule would have relabelled three servers MEASURED on data that says nothing about code mode.
- Root cause: the rule was written before Tier 2 existed and keyed the label on the existence of a run rather than on what the run measures. Same shape as entry 1: a methodology description ahead of the instrument.
- What changed: methodology 0.3.1 (2026-08-20) restates the condition in 3.3 as a direct measurement of that server's code-mode context footprint, and 3.2 states the equivalent condition for tool search. Tier 2 call-traffic figures now publish as their own MEASURED block on the site, never summed into a mode total and never used to move a mode label. No number was edited.
- Reported by: internal.
