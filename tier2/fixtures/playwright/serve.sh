#!/usr/bin/env bash
#
# Serves the Tier 2 playwright fixture pages for the duration of a run, per
# docs/tier2-task-suites.md section 2.2.
#
# python3's http.server is the tooling choice: it is already present on both
# machines in the estate, it adds no dependency to the repo, and it serves
# static files read-only with no configuration, which is the entire
# requirement. Serving directory is site/ only, so nothing else in the repo is
# reachable from the browser under test.
#
# Usage:
#   ./serve.sh                     serve on 127.0.0.1:8930 until interrupted
#   LOADLINE_TIER2_HTTP_PORT=9001 ./serve.sh
#   ./serve.sh --check             verify the pages are already being served
#
# Bound to the loopback interface explicitly. The pages are fixtures, not a
# public site, and nothing on them should ever be reachable off the host.

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="${here}/site"
port="${LOADLINE_TIER2_HTTP_PORT:-8930}"

if [[ "${1:-}" == "--check" ]]; then
  for page in landing.html docs.html catalog.html form.html; do
    if ! curl -fsS "http://127.0.0.1:${port}/${page}" >/dev/null; then
      echo "not served: http://127.0.0.1:${port}/${page}" >&2
      exit 1
    fi
  done
  echo "fixture pages served on 127.0.0.1:${port}"
  exit 0
fi

if [[ -n "${1:-}" ]]; then
  echo "usage: $0 [--check]" >&2
  exit 2
fi

command -v python3 >/dev/null || {
  echo "python3 is required to serve the playwright fixtures" >&2
  exit 1
}

echo "serving ${root} on http://127.0.0.1:${port}/ (ctrl-c to stop)"
exec python3 -m http.server "$port" --bind 127.0.0.1 --directory "$root"
