# Filesystem fixture

Backs the five FS tasks in `docs/tier2-task-suites.md` section 2.1.

| Path | Purpose |
| --- | --- |
| `setup.sh` | Rebuilds `seed/` deterministically. `./setup.sh --verify` diffs a fresh build against the committed tree and exits non-zero on drift. |
| `seed/` | The committed seed tree. Read-only during a run. |

## Seed tree

```
seed/
  config/app.json      one line, {"name":"loadline-fixture","version":"4.2.1","env":"test"}
  logs/access.log      137 lines, every field derived from the line index
  docs/intro.md        5 lines, one of them "TODO: fill in the roadmap section"
  docs/roadmap.md      4 lines, no TODO marker anywhere
  docs/changelog.md    7 lines, one of them "TODO: backfill 2025 entries"
  notes.txt            one line, "seed value: baseline"
```

Two invariants hold the tasks up, and `setup.sh --verify` is what protects them:

- **`logs/access.log` is the unique line-count maximum** at 137 lines. The next largest file has 7. FS-04 has one correct answer only while that gap holds.
- **Exactly two files contain `TODO`**: `docs/intro.md` and `docs/changelog.md`. `docs/roadmap.md` is the negative control, and no other file in the tree, `access.log` included, carries the marker.

## Per-trial setup

The filesystem server is never pointed at `seed/`. Per section 2.1 the runner copies the tree to a fresh scratch directory before every trial and sets the server's allowed root to the copy:

```sh
SCRATCH="$(mktemp -d)/seed"
cp -a tier2/fixtures/filesystem/seed "$SCRATCH"
```

`$SCRATCH` is the fixture root the rendered prompt's relative paths resolve against, and the directory the state checks below run in.

**The client runs with `$SCRATCH` as its working directory, and nothing else is written there.** Both clients advertise the MCP roots capability and send their working directory as the only root, and `@modelcontextprotocol/server-filesystem` replaces its command-line allowed directories with the client's roots when it receives them. A client launched anywhere else is denied every path in the scratch copy, whatever the server was started with. Harness files (MCP config, rendered prompt, transcript, client output) therefore live in a sibling work directory, never in the tree, and `run-suite.sh` records any path the client leaves behind in the fixture as `fixture_extra_paths` on the trial's manifest row.

## Task map

`$TRANSCRIPT` below is the file holding the client's final assistant-facing message, captured per section 3.2. `$TRIAL_ID` is the 8-character hex token the runner generated for the trial.

| Task | Fixture prerequisites | Check | Mechanical success check |
| --- | --- | --- | --- |
| **FS-01** | `config/app.json` present with `"version":"4.2.1"` | Transcript | `grep -Fq '4.2.1' "$TRANSCRIPT"` |
| **FS-02** | `docs/intro.md` and `docs/changelog.md` contain `TODO`; `docs/roadmap.md` does not; no other file does | Transcript | `grep -Fq 'docs/intro.md' "$TRANSCRIPT" && grep -Fq 'docs/changelog.md' "$TRANSCRIPT"` |
| **FS-03** | Scratch copy has no `output/` directory | State | `[ "$(cat "$SCRATCH/output/$TRIAL_ID.txt")" = "tier2 probe $TRIAL_ID" ]` |
| **FS-04** | `logs/access.log` at 137 lines is the unique maximum | State | Two lines, normalized then compared to `logs/access.log` and `137`; see `check_task` FS-04 in `run-suite.sh` |
| **FS-05** | `notes.txt` holds its single line `seed value: baseline` | State | `[ "$(cat "$SCRATCH/notes.txt")" = "$(printf 'seed value: baseline\ntrial %s recorded' "$TRIAL_ID")" ]` |

Notes on the checks:

- FS-03's criterion tolerates a single trailing newline. `$(cat ...)` strips trailing newlines, which is exactly that tolerance and no more: two trailing newlines still compare equal under `$( )`, so a run that wants the stricter reading should compare bytes with `cmp` against a file built by `printf 'tier2 probe %s' "$TRIAL_ID"`.
- FS-05 asserts the full file contents rather than grepping for a substring, because the task specifies an exact line count. A grep would pass on a file that also carried extra lines.
- FS-04 compares two lines after normalization rather than the raw bytes. Trailing blank lines are dropped first, and exactly two lines must remain. The path line is then normalized: whitespace trimmed, one layer of surrounding backticks or quotes stripped, one trailing comma or period stripped, one leading `./` stripped, and a path ending in `/logs/access.log` reduced to that trailing segment. The count line is normalized separately: whitespace trimmed, one layer of surrounding backticks or quotes stripped, and comma thousands separators removed, leaving bare digits. Neither normalizer can turn a wrong answer into a right one: `logs/other.log` stays `logs/other.log`, and a count of `100` stays `100`. The old byte comparison failed a trial that wrote `./logs/access.log` with the correct count, which is a path-format artifact and not a capability result, while the genuine miscount the same client produced in a different trial still fails.
- FS-02 no longer asserts the absence of `docs/roadmap.md`. That clause tested phrasing, not capability: a client that listed the two matching files correctly and then named the non-matching files in an explanatory sentence was recorded as a failure, and the same client phrasing the same correct finding without that sentence passed.
- Every check is a shell exit status. None of them reads the client's account of its own work except where section 2.1 explicitly calls for a transcript check.
