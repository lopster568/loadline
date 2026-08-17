# Harness validation log

Tier 1 harness, `harness_version` 0.1.0, `methodology_version` 0.1.0.

This file records how the harness was validated against live servers, so the
run can be reproduced by a third party with their own credentials. It is a
harness artifact, not a published release.

## Reproducing the validation run

```
go build ./...
go vet ./...
go test ./...
go run ./cmd/loadline scan --servers servers.yaml --only filesystem,fetch --out data \
  --step-timeout 180s --server-timeout 360s
```

Artifacts land at `data/runs/<YYYY-MM-DD>/<server>.json` and `data/latest.json`.
`--dry-run` resolves the corpus and prints the acquisition plan without
contacting any server.

## Environment used, 2026-08-17

| Item | Value |
| --- | --- |
| Go | 1.24.6 linux/amd64 (WSL2) |
| node / npx | v22.14.0 |
| uv / uvx | present on PATH |
| Network | available |
| `ANTHROPIC_API_KEY` | not set |
| `GEMINI_API_KEY` / `GOOGLE_API_KEY` | not set |

Because no Anthropic or Gemini credential was present, both cells published
`available: false` with the reason recorded and no count. The harness never
substitutes an estimate for a missing credential.

## Result: filesystem (MEASURED)

`@modelcontextprotocol/server-filesystem`, launched with `npx -y`, one allowed
directory (a per-run scratch dir).

| Field | Value |
| --- | --- |
| status | `ok` |
| protocol revision | `2025-06-18`, branch `legacy_initialize` |
| serverInfo.version | 0.2.0 |
| tool count | 14 |
| naive full-load, o200k_base | **2697 tokens** |
| per-tool mean, o200k_base | 192 tokens |
| tool_search modeled total | 500 + k x 192, so 1076 at k=3 and 1460 at k=5 |
| code mode, modeled | 1000 tokens |
| `wire_sha256` | `32070959d6d7e8fbc7809599723c24565f4fd72d26641acbcd7236761381f69d` |
| `canonical_sha256` | `a9f0f9e451249c9328d40fb9c6b4bd40359b4ed7830e9f182e2116660763b959` |

Per-tool counts, highest first: `read_media_file` 281, `read_text_file` 247,
`edit_file` 236, `search_files` 209, `read_multiple_files` 201,
`directory_tree` 193, `list_directory_with_sizes` 192, `move_file` 183,
`read_file` 170, `create_directory` 168, `write_file` 165, `list_directory`
157, `get_file_info` 153, `list_allowed_directories` 140.

The server answered `server/discover` with method-not-found and the harness
fell back to `initialize`, which is the methodology 1.3 legacy branch working
against a real server rather than a fixture.

## Result: fetch (`unreachable`, and the reason is upstream)

`uvx mcp-server-fetch` does not start as published, on any machine, as of
2026-08-17:

```
ImportError: cannot import name 'McpError' from 'mcp.shared.exceptions'.
Did you mean: 'MCPError'?
```

`mcp-server-fetch` 2026.7.10 declares `mcp>=1.1.3` with no upper bound, and
`mcp` 2.0.0 renamed `McpError` to `MCPError`. A clean `uvx` resolve therefore
pulls an SDK the server cannot import. This is server rot of exactly the kind
methodology 7 keeps in the dataset, so the published row is a real
`unreachable` failure with the traceback captured in `error`.

Constraining the transitive dependency makes the same published version start,
which confirms the failure is a dependency bound rather than a dead server:

```
uvx --with "mcp<2" mcp-server-fetch==2026.7.10
```

Measured under that constraint, for the record and **not** published in
`data/`:

| Field | Value |
| --- | --- |
| status | `ok` |
| protocol revision | `2025-06-18`, branch `legacy_initialize` |
| serverInfo.version | 1.29.0 |
| tool count | 1 (`fetch`) |
| naive full-load, o200k_base | 238 tokens |
| `wire_sha256` | `b40c1ebe2a918ab2abf285d49e65676d85eff9f6ec34805573d2a4fd9c56272e` |
| `canonical_sha256` | `5d0a1b56f6c08786d7d764b6f4376fba201015e5a5995b6b39c14350bd4aeeb3` |

The harness supports a constrained pin through a `with` list on a `pypi`
package entry, and records the constraint inside `acquisition.source` and
`acquisition.args` so a constrained acquisition is never invisible on a row.
Whether the corpus adopts that pin for `fetch` is an onboarding decision, not a
harness decision, so `servers.yaml` was left untouched.

## Reference launch defaults

`servers.yaml` leaves several package blocks as "TODO verify at onboarding".
The harness skips those as `unreachable` with the reason recorded, except for
the two auth-free reference servers it validates itself against, whose launch
vectors live in `internal/sweep/acquire.go`:

- `filesystem`: the corpus supplies the npm name, and the harness appends the
  required allowed-directory argument, recorded as `{{SCRATCH}}` so the run
  record stays reproducible.
- `fetch`: the corpus package block is a stub, so the harness falls back to
  `uvx mcp-server-fetch`.

Both move into `servers.yaml` once onboarding pins them.

## Credential-bearing servers

No server requiring a credential was contacted during validation. To sweep one,
export the env var the corpus declares in `auth.token_env` and add its id to
`--only`. A server whose credential is absent publishes `auth` with the missing
variable named, and never a partial surface.
