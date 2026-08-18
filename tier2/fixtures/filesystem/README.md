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

`$SCRATCH` is the fixture root the rendered prompt's relative paths resolve against, and the directory the state checks below run in. It is discarded after the trial is evaluated.

## Task map

`$TRANSCRIPT` below is the file holding the client's final assistant-facing message, captured per section 3.2. `$TRIAL_ID` is the 8-character hex token the runner generated for the trial.

| Task | Fixture prerequisites | Check | Mechanical success check |
| --- | --- | --- | --- |
| **FS-01** | `config/app.json` present with `"version":"4.2.1"` | Transcript | `grep -Fq '4.2.1' "$TRANSCRIPT"` |
| **FS-02** | `docs/intro.md` and `docs/changelog.md` contain `TODO`; `docs/roadmap.md` does not; no other file does | Transcript | `grep -Fq 'docs/intro.md' "$TRANSCRIPT" && grep -Fq 'docs/changelog.md' "$TRANSCRIPT" && ! grep -Fq 'docs/roadmap.md' "$TRANSCRIPT"` |
| **FS-03** | Scratch copy has no `output/` directory | State | `[ "$(cat "$SCRATCH/output/$TRIAL_ID.txt")" = "tier2 probe $TRIAL_ID" ]` |
| **FS-04** | `logs/access.log` at 137 lines is the unique maximum | State | `[ "$(cat "$SCRATCH/summary.txt")" = "$(printf 'logs/access.log\n137')" ]` |
| **FS-05** | `notes.txt` holds its single line `seed value: baseline` | State | `[ "$(cat "$SCRATCH/notes.txt")" = "$(printf 'seed value: baseline\ntrial %s recorded' "$TRIAL_ID")" ]` |

Notes on the checks:

- FS-03's criterion tolerates a single trailing newline. `$(cat ...)` strips trailing newlines, which is exactly that tolerance and no more: two trailing newlines still compare equal under `$( )`, so a run that wants the stricter reading should compare bytes with `cmp` against a file built by `printf 'tier2 probe %s' "$TRIAL_ID"`.
- FS-04 and FS-05 both assert the full file contents rather than grepping for a substring, because both tasks specify an exact line count. A grep would pass on a file that also carried extra lines.
- Every check is a shell exit status. None of them reads the client's account of its own work except where section 2.1 explicitly calls for a transcript check.
