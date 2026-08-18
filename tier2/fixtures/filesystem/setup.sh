#!/usr/bin/env bash
#
# Builds the Tier 2 filesystem seed tree described in
# docs/tier2-task-suites.md section 2.1.
#
# The tree is committed to the repo, so this script is normally not run: it is
# the definition of record for how the tree was produced, and the way to
# regenerate it after an accidental edit. Every byte it writes is fixed, so a
# rebuild on any machine produces the identical tree.
#
# Usage:
#   ./setup.sh              rebuild seed/ in place
#   ./setup.sh --verify     rebuild into a temp dir and diff against seed/,
#                           exiting non-zero if the committed tree has drifted
#
# The runner never points a server at seed/ directly. Per section 2.1 it
# copies the tree to a fresh scratch directory before every trial and sets the
# filesystem server's allowed root to the copy, so a write task cannot leak
# into the next trial's baseline.

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
seed="${here}/seed"

# ACCESS_LOG_LINES is the answer to FS-04. It must stay strictly greater than
# the line count of every other file in the tree, or the task stops having one
# correct answer.
readonly ACCESS_LOG_LINES=137

build() {
  local root="$1"

  mkdir -p "${root}/config" "${root}/logs" "${root}/docs"

  # FS-01 reads the version field out of this file.
  cat >"${root}/config/app.json" <<'JSON'
{"name":"loadline-fixture","version":"4.2.1","env":"test"}
JSON

  # FS-04's correct answer. Deterministic: every field is derived from the
  # line index, so there is no clock, no hostname, and no randomness in it.
  # No line contains the string TODO, which would break FS-02.
  : >"${root}/logs/access.log"
  local i n min sec status bytes
  for ((i = 1; i <= ACCESS_LOG_LINES; i++)); do
    printf -v n '%04d' "$i"
    printf -v min '%02d' "$((((i - 1) / 60) % 60))"
    printf -v sec '%02d' "$(((i - 1) % 60))"
    if ((i % 7 == 0)); then status=404; else status=200; fi
    bytes=$((512 + i * 13))
    printf '2026-08-01T00:%s:%sZ 127.0.0.1 GET /assets/item-%s %s %s\n' \
      "$min" "$sec" "$n" "$status" "$bytes" >>"${root}/logs/access.log"
  done

  # FS-02 expects exactly two files to contain TODO: these two.
  cat >"${root}/docs/intro.md" <<'MD'
# Introduction

This tree is a fixed benchmark fixture. Nothing here is generated at run time.

TODO: fill in the roadmap section
MD

  # roadmap.md is the negative control for FS-02. It must never contain the
  # marker, in any case or spelling that a search tool would match.
  cat >"${root}/docs/roadmap.md" <<'MD'
# Roadmap

The roadmap is intentionally empty. This file exists so that FS-02 has a file
that is not a match, which is what makes a passing result mean something.
MD

  cat >"${root}/docs/changelog.md" <<'MD'
# Changelog

## 4.2.1

Fixture baseline.

TODO: backfill 2025 entries
MD

  # FS-05 appends one line to this file and must leave this line untouched.
  printf 'seed value: baseline\n' >"${root}/notes.txt"

  # Fixed modes so a rebuild under a different umask still matches.
  find "$root" -type d -exec chmod 0755 {} +
  find "$root" -type f -exec chmod 0644 {} +
}

case "${1:-}" in
--verify)
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  build "${tmp}/seed"
  if diff -r "${tmp}/seed" "$seed" >/dev/null 2>&1; then
    echo "seed tree matches setup.sh"
  else
    echo "seed tree has drifted from setup.sh:" >&2
    diff -r "${tmp}/seed" "$seed" >&2 || true
    exit 1
  fi
  ;;
"")
  rm -rf "$seed"
  build "$seed"
  echo "built ${seed}"
  ;;
*)
  echo "usage: $0 [--verify]" >&2
  exit 2
  ;;
esac
