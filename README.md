# loadline

A standing measurement of what MCP servers cost an agent's context window.

[Calculator](https://loadline-dev.netlify.app) ·
[Launch writeup](https://dev.to/lopster568/i-measured-what-14-mcp-servers-cost-a-context-window-claude-counts-them-64-higher-than-tiktoken-10pj) ·
[Methodology](docs/methodology-v0.md) ·
[Corrections log](governance/corrections-log.md)

![The loadline stack calculator with GitHub MCP and Linear MCP selected. The server grid lists all 14 corpus rows with tool counts and hygiene grades, including the two unreachable rows marked "no data"; the gauge on the right reads 115,869 tokens, 57.9% of a 200k window, under naive full load counted with the Claude tokenizer.](docs/assets/readme-hero.png)

## What it measures

Every published server row carries three tokenizer columns over one canonical
serialization of its tool surface: OpenAI `o200k_base` counted locally, Claude
counted through `count_tokens` (pinned to `claude-opus-5`), and Gemini counted
through `countTokens` (pinned to `gemini-3.1-pro-preview`). Counting the same
string three ways is what makes the columns comparable.

Cost is reported per client mode, never collapsed into one number, because the
same stack costs a different amount depending on how the client loads it:

| Mode | Basis |
| --- | --- |
| Naive full load | MEASURED. The canonical serialization count. |
| Tool search / progressive disclosure | MODELED total over measured inputs: a search stub plus 3 to 5 retrieved tools. |
| Code mode | MODELED. |

Two further columns exist so the scoreboard cannot be won by gutting
descriptions: **retrievability** (does a tool surface under search over its own
name and description) and a **schema hygiene grade** over description adequacy,
parameter descriptions, enum documentation, naming clarity, and disambiguation.
Servers that fail to enumerate are published as rows with no counts rather than
dropped, because server rot is part of the subject.

Five rows from the 2026-08-18 run:

| Server | Tools | Naive, o200k | Naive, Claude | Hygiene |
| --- | --- | --- | --- | --- |
| GitHub | 47 | 59,084 | 86,843 | B |
| Chrome DevTools | 52 | 7,984 | 13,317 | B |
| Notion | 24 | 5,180 | 8,470 | D |
| Cloudflare | 3 | 1,596 | 2,641 | B |
| Context7 | 2 | 1,052 | 1,691 | A |

Two more rows, `fetch` and `postgres`, were published as `unreachable` on
2026-08-18 with no counts in any column. Both failed at import against the MCP
Python SDK version their unbounded dependency ranges resolved to, and the
`fetch` failure did not reproduce an hour later (see corrections log entry 2).
Full data:
[`data/latest.json`](data/latest.json).

## What the first run found

Claude counts the same canonical tool-surface string higher than `o200k_base`
on every one of the 12 measured servers, by 47.0% at the low end (GitHub) and
70.5% at the high end (Kubernetes), median 64.1%. A budget sized on a local
tiktoken count is a different number from what the same surface costs on
Claude, and the gap is per server, not a constant. Tool count does not stand in
for cost either: Chrome DevTools exposes 52 tools for 7,984 `o200k` tokens
while GitHub exposes 47 for 59,084. Two of the 14 corpus rows carry no counts
at all, and are published that way.

## Reproduce a row

```
git clone https://github.com/lopster568/loadline.git
cd loadline
go build -o loadline ./cmd/loadline
./loadline scan --servers servers.yaml --only filesystem --out /tmp/loadline-run
```

Point `--out` at a scratch directory as shown. `scan` rewrites
`<out>/latest.json` from the servers it just swept, so running it against
`data/` with `--only` replaces the published 14-row file with a one-row file.

The run writes, under `<out>/runs/<date>/`:

| File | Contents |
| --- | --- |
| `filesystem.json` | The row: counts, modes, retrievability, hygiene, provenance |
| `filesystem-wire.jsonl` | The raw `tools/list` response pages, verbatim, one page per line |
| `filesystem-queries.json` | The generated query set behind the retrievability score |

`provenance.wire_sha256` is a checkable claim, not an assertion. It is the
SHA-256 over the concatenated wire pages with the line separators removed, so
the published artifact reproduces it:

```
python3 -c "import hashlib,sys; pages=open(sys.argv[1],'rb').read().split(b'\n')[:-1]; print(hashlib.sha256(b''.join(pages)).hexdigest())" \
  data/runs/2026-08-18/filesystem-wire.jsonl
```

That prints `32070959d6d7e8fbc7809599723c24565f4fd72d26641acbcd7236761381f69d`,
which is the `wire_sha256` on the published `filesystem` row.

**Credentials.** The `o200k` column runs offline and needs no key. The Claude
column needs `ANTHROPIC_API_KEY`; the Gemini column needs `GEMINI_API_KEY` (or
`GOOGLE_API_KEY`). Both endpoints are free at sweep volumes and no inference is
performed. A missing key does not produce an estimate: the cell publishes as
`available: false`. Servers that need their own credential to enumerate name
their environment variable in `servers.yaml`; `filesystem` needs none, which is
why it is the example.

`docs/pc-sweep-runbook.md` covers the full sweep.

## Repo map

| Path | Contents |
| --- | --- |
| `cmd/` | CLI: `scan` (Tier 1 sweep), `analyze` (Tier 2 trial log), `version` |
| `internal/` | Harness packages: MCP wire, canonicalization, tokenizers, metrics, report writer |
| `interposer/` | MCP call-logging proxy that supplies the Tier 2 frame logs |
| `tier2/` | Tier 2 fixtures and verified task inputs |
| `site/` | Calculator and leaderboard, built with Vite and React |
| `docs/` | Methodology, server selection rule, sweep runbook, Tier 2 task suites |
| `governance/` | Corrections log, right of reply, recusal policy, self-submission |
| `data/` | `latest.json` plus per-run rows, wire artifacts, and query sets |

## Governance

| Mechanism | Where |
| --- | --- |
| Selection rule: the gates that decide inclusion, published so a reader can predict the outcome before asking | [`docs/server-selection.md`](docs/server-selection.md), Part A |
| Corrections log: append-only, entries are never edited or removed, a number is never changed without one. Two entries so far | [`governance/corrections-log.md`](governance/corrections-log.md) |
| Right of reply: 14 calendar days before a server's numbers are first published, with the rows and the raw artifacts sent to the maintainer, and a reply of up to 500 words published alongside the row | [`governance/right-of-reply.md`](governance/right-of-reply.md) |
| Recusal: ownership, employment, and sponsorship exclusions, plus funding disclosure before the first ranking | [`governance/recusal-policy.md`](governance/recusal-policy.md) |
| Self-submission: what a maintainer sends and what happens to it | [`governance/self-submission.md`](governance/self-submission.md), via the [server submission template](.github/ISSUE_TEMPLATE/server-submission.md) |
| Raw artifacts: the wire pages behind every published hash | [`data/runs/`](data/runs) |

Every published cell is stamped with model, tokenizer, `measured_at`, protocol
revision, server version, `tools_list_sha256`, methodology version, and harness
version. A cell missing a stamp field is withheld.

## Cadence and what is next

The sweep re-runs monthly, driven manually today per
`docs/pc-sweep-runbook.md`. Tier 2, scripted real tasks measured through the
interposer, is in preparation: the proxy and the `analyze` subcommand are built
and tested, the task suites are specified in `docs/tier2-task-suites.md`, and
fixtures are in `tier2/`.

Open items, stated rather than deferred:

- The harness now records the resolved dependency versions per acquisition, in
  `acquisition.resolved_deps` (methodology 1.1, added in 0.3.0), which closes
  the instrument gap behind the `fetch` row per corrections log entry 2. The
  published 2026-08-18 rows predate it and carry no resolve; the next monthly
  run is the first to carry the field.
- Tool search and code mode totals are modeled from published client behaviour,
  not measured against those clients. They are labelled MODELED everywhere they
  appear.
- The Claude column is a count of the canonical cross-provider string. The
  Anthropic-native `tools`-parameter count is recorded separately per row and
  is not blended into it.
- `slack` is in the corpus but unpublished, pending an OAuth credential that
  can enumerate its full surface without browser-session cookie extraction.
  `figma` is excluded under the partial-surface gate.
- `docs/methodology-v0.md` carries its unresolved choices inline as **OPEN**
  markers. The recusal, right-of-reply, and self-submission documents are at
  v0 and marked as such.

## License

Apache-2.0. See [`LICENSE`](LICENSE).

The server and model marks on the calculator come from
[Simple Icons](https://simpleicons.org) (CC0 1.0). Trademarks belong to their
respective owners; the marks identify the products being measured and imply no
endorsement.
