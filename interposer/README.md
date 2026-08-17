# loadline-interposer

A call-logging proxy for stdio MCP servers.

The interposer sits between an MCP client and a server. The client is configured
to launch the interposer instead of the server. The interposer spawns the real
server, relays stdin and stdout byte-for-byte in both directions, passes server
stderr straight through, and appends every JSON-RPC frame to a JSONL log.

It exists to capture one thing the existing benchmark instrumentation in this
estate does not: the full `params` object of every request, including
`tools/call` arguments. Everything else in the log is there to make those
arguments interpretable.

The proxy is stdlib only, with no dependencies, so it is auditable in one
sitting. Tier 2 of loadline depends on it, and the measurement rules in
`../docs/methodology-v0.md` apply to anything derived from its logs.

## Install

```
go build -o loadline-interposer ./cmd/loadline-interposer
```

or

```
go install github.com/lopster568/loadline/interposer/cmd/loadline-interposer@latest
```

## Usage

```
loadline-interposer --log <path> [--full-results] -- <server command...>
```

| Flag | Meaning |
| --- | --- |
| `--log <path>` | JSONL frame log. Required. Opened append-only, created with mode 0600. |
| `--full-results` | Log complete response payloads in addition to the summaries. Off by default because tool results are the bulk of the bytes. |
| `--version` | Print the interposer version and exit. |

Everything after `--` is the server command and is executed unchanged.

Direct invocation, useful for checking that a server works before wiring it into
a client:

```
loadline-interposer --log /tmp/github.jsonl -- npx -y @modelcontextprotocol/server-github
```

### Claude Code

Wrap the server command in your MCP config (`.mcp.json` in a project, or
`~/.claude.json` for user scope). Before:

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": { "GITHUB_PERSONAL_ACCESS_TOKEN": "..." }
    }
  }
}
```

After:

```json
{
  "mcpServers": {
    "github": {
      "command": "/usr/local/bin/loadline-interposer",
      "args": [
        "--log", "/home/you/loadline-runs/github.jsonl",
        "--", "npx", "-y", "@modelcontextprotocol/server-github"
      ],
      "env": { "GITHUB_PERSONAL_ACCESS_TOKEN": "..." }
    }
  }
}
```

The `env` block still applies. The interposer inherits its environment and
passes it to the server, which is how stdio servers receive credentials.

Use an absolute path for both the binary and the log. The client may launch the
server from a working directory you did not pick.

The equivalent CLI form is:

```
claude mcp add github -- loadline-interposer --log /home/you/loadline-runs/github.jsonl -- npx -y @modelcontextprotocol/server-github
```

The JSON form is easier to get right, because the CLI form relies on the second
`--` surviving argument parsing.

### Gemini CLI

Same shape, in `~/.gemini/settings.json` or a project `.gemini/settings.json`:

```json
{
  "mcpServers": {
    "github": {
      "command": "/usr/local/bin/loadline-interposer",
      "args": [
        "--log", "/home/you/loadline-runs/github.jsonl",
        "--", "npx", "-y", "@modelcontextprotocol/server-github"
      ],
      "env": { "GITHUB_PERSONAL_ACCESS_TOKEN": "..." }
    }
  }
}
```

Give each server its own log file. One log per server per run keeps attribution
unambiguous and avoids interleaving frames from concurrently launched servers.

## Log format

One JSON object per line. The first line of every session is a header; the rest
are frames, in the order they crossed the pipe.

### Header line

| Field | Type | Meaning |
| --- | --- | --- |
| `interposer_version` | string | Semver of the binary that produced the log. |
| `started_at` | string | RFC3339Nano UTC, when the server was spawned. |
| `server_cmd` | array | The server argv as executed. |
| `pid` | number | Process id of the spawned server. |

### Frame lines

| Field | Type | Present when | Meaning |
| --- | --- | --- | --- |
| `ts` | string | always | RFC3339Nano UTC, recorded after the frame was relayed. |
| `dir` | string | always | `c2s` client to server, `s2c` server to client. |
| `size_bytes` | number | always | Byte length of the frame on the wire, trailing newline included. |
| `unparseable` | bool | frame is not a JSON-RPC message | The frame was relayed unchanged; only its size was recorded. |
| `batch` | bool | frame is a JSON array | Per-message fields move into `items`. |
| `batch_len` | number | batch | Number of messages in the array. |
| `items` | array | batch | One object per message, carrying the per-message fields below. |
| `method` | string | requests and notifications | JSON-RPC method. |
| `id` | any | request or response | JSON-RPC id, preserved as sent (number or string). |
| `is_response` | bool | responses | Set when the message carries `result` or `error`. |
| `params_full` | object | message has params | The complete params object, unmodified. This is the point of the tool. |
| `result_summary` | object | responses with a result | See below. |
| `result_full` | any | `--full-results` | The complete result member. |
| `error` | object | error responses | `code` and `message`. |
| `error_full` | any | `--full-results` | The complete error member, including `data`. |

`result_summary` fields:

| Field | Present when | Meaning |
| --- | --- | --- |
| `bytes` | always | Byte length of the serialized `result` member. |
| `content_blocks` | result has a `content` array | Number of content blocks. In practice this means a `tools/call` result. |
| `text_len` | result has a `content` array | Total UTF-8 byte length of the `text` members across all blocks. |
| `is_error` | result has `isError` | The tool-level error flag, which is distinct from a JSON-RPC error. |

Notes on interpretation:

- A frame is one newline-terminated chunk. CRLF does not start a new frame; the
  carriage return is counted in `size_bytes` like any other byte.
- A trailing chunk with no newline is logged as its own frame.
- Requests and responses are correlated offline by `id`, which is why the
  interposer keeps no state of its own between frames.
- Records are written unbuffered, one write syscall per frame, after the frame
  has already been relayed. A crash of the interposer cannot lose a frame it
  already forwarded.
- The log is append-only. Running two sessions against the same path produces
  two header lines in one file.

## Version discipline

`Version` is a hardcoded semver constant, stamped into the header line of every
log file. Results are comparable only within one interposer version. This
follows section 9 of `../docs/methodology-v0.md`, where harness change is one of
the three delta types a release must classify, and it exists because harness
variance can be large enough to swamp the effect being measured.

Any change to framing, to which fields are logged, or to how `result_summary` is
computed is a version bump. Do not compare runs across versions, and do not
merge logs from different versions into one dataset.

## Security

**The log contains complete tool-call arguments.** Those arguments routinely
carry API tokens, file paths, customer records, repository contents, and
anything else a model passes to a tool. With `--full-results` the log also
contains complete tool output.

There is no redaction in v0.1. Treat a frame log as a secret of the same grade
as the credentials the server was given.

- Log files are created with mode 0600, but the containing directory is yours to
  choose. Pick one that is not synced, not backed up to a shared location, and
  not inside a git working tree.
- Do not attach a raw frame log to an issue, a PR, or a published artifact.
  loadline publishes derived counts, not raw logs.
- Delete logs when the run they support is finished.

## Non-goals for v0.1

Deliberately out of scope. None of these are planned for this version, and
adding any of them is a version bump with a methodology note.

| Not in v0.1 | Note |
| --- | --- |
| HTTP transports | Streamable HTTP and the deprecated HTTP+SSE are not proxied. stdio only. |
| Request or response mutation | The relay is a byte pipe. It never rewrites, reorders, injects, or drops a frame. |
| Sampling | Every frame is logged. There is no rate limit and no size cap. |
| Redaction | Nothing is stripped or masked. See the security note. |
| Token counting | No tokenizer is bundled, which is what keeps the binary dependency-free. A separate `analyze` subcommand can compute per-call counts from the JSONL later. |
| Signal fidelity | Interrupts are forwarded to the server, but a server killed by a signal reports exit code 1 rather than the shell's 128 plus signal convention. |

## Tests

```
go test ./...
go vet ./...
```

The tests drive the relay through real pipes with the test binary standing in as
the MCP server. They cover byte-exact passthrough including CRLF and
unterminated frames, frame parsing for requests, notifications, responses,
errors, batches and garbage, multi-megabyte frames arriving in partial reads,
append-only logging, log file permissions, stderr passthrough, and exit code
propagation.
