#!/usr/bin/env bash
#
# Resets the Tier 2 GitHub fixture repo to the baseline of
# docs/tier2-task-suites.md section 2.3, using the operator's authenticated
# gh CLI.
#
# Baseline, as tagged by tier2-baseline:
#   NOTES.md          contains the line "TARGET: 42"
#   docs/alpha.md, docs/beta.md, and nothing else under docs/
#   exactly one open issue, labeled tier2-fixture, titled
#     "Fixture issue: do not close"
#   main only; no tier2/* branches; no open pull requests from them
#
# What it fixes:
#   - closes and deletes every tier2-probe-* issue left by GH-03
#   - closes pull requests from tier2/* branches and deletes those branches,
#     which is what GH-04 leaves behind
#   - reopens the fixture issue if a trial closed it
#   - deletes comments left on the fixture issue, which the baseline has none of
#
# What it refuses to fix, exiting non-zero instead: content drift on the
# tracked baseline files. A run against a mutated NOTES.md or docs/ tree would
# be measuring a different fixture than the one the task criteria were written
# against, and repairing that by force-pushing over whatever the drift was
# would destroy the evidence of how it happened.
#
# Usage:
#   ./reset.sh              reset, then verify; non-zero on unfixable drift
#   ./reset.sh --check      verify only, change nothing
#
# Environment:
#   LOADLINE_TIER2_GH_REPO   owner/name override (default the fixture repo)
#   LOADLINE_TIER2_GH_TAG    baseline tag override (default tier2-baseline)

set -euo pipefail

REPO="${LOADLINE_TIER2_GH_REPO:-lopster568/loadline-tier2-fixture}"
TAG="${LOADLINE_TIER2_GH_TAG:-tier2-baseline}"
FIXTURE_ISSUE_TITLE="Fixture issue: do not close"
FIXTURE_LABEL="tier2-fixture"

CHECK_ONLY=0
case "${1:-}" in
--check) CHECK_ONLY=1 ;;
"") ;;
*)
  echo "usage: $0 [--check]" >&2
  exit 2
  ;;
esac

die() {
  echo "reset.sh: $*" >&2
  exit 1
}
log() { echo "reset.sh: $*" >&2; }

command -v gh >/dev/null 2>&1 || die "gh CLI is required"
gh auth status >/dev/null 2>&1 || die "gh is not authenticated"
gh api "repos/${REPO}" --jq '.full_name' >/dev/null 2>&1 ||
  die "cannot reach ${REPO} with the current gh credential"

drift=0

# ------------------------------------------------------------------ issues --

# Every probe issue GH-03 created, open or closed. They are deleted rather
# than closed: a closed issue still matches a title search, and GH-03's check
# asserts "exactly one open issue with this title", which a leftover would not
# break today but would make ambiguous the moment a trial id repeated.
mapfile -t probe_issues < <(
  gh api "repos/${REPO}/issues?state=all&per_page=100" --paginate \
    --jq '.[] | select(.pull_request == null) | select(.title | startswith("tier2-probe-")) | .number' 2>/dev/null || true
)

if [[ ${#probe_issues[@]} -gt 0 ]]; then
  if ((CHECK_ONLY)); then
    log "drift: ${#probe_issues[@]} tier2-probe-* issue(s) present"
    drift=1
  else
    for n in "${probe_issues[@]}"; do
      # deleteIssue is a GraphQL-only mutation; the REST API cannot delete an
      # issue. Closing first keeps the repo consistent if the delete is
      # refused for a permission reason.
      gh api -X PATCH "repos/${REPO}/issues/${n}" -f state=closed >/dev/null 2>&1 || true
      id="$(gh api "repos/${REPO}/issues/${n}" --jq '.node_id' 2>/dev/null || true)"
      if [[ -n "$id" ]] &&
        gh api graphql -f query='mutation($id: ID!) { deleteIssue(input: {issueId: $id}) { clientMutationId } }' \
          -F id="$id" >/dev/null 2>&1; then
        log "deleted issue #${n}"
      else
        log "could not delete issue #${n}; it is closed but still present"
        drift=1
      fi
    done
  fi
fi

# The one issue the baseline requires. GH-02 reads it, nothing should close it.
fixture_state="$(gh api "repos/${REPO}/issues?state=all&per_page=100" --paginate \
  --jq "[.[] | select(.pull_request == null) | select(.title == \"${FIXTURE_ISSUE_TITLE}\")] | .[0] | \"\(.number) \(.state)\"" 2>/dev/null || true)"
if [[ -z "$fixture_state" || "$fixture_state" == "null null" ]]; then
  log "drift: the fixture issue titled \"${FIXTURE_ISSUE_TITLE}\" does not exist"
  drift=1
else
  fixture_number="${fixture_state%% *}"
  fixture_status="${fixture_state##* }"
  if [[ "$fixture_status" != "open" ]]; then
    if ((CHECK_ONLY)); then
      log "drift: the fixture issue is ${fixture_status}"
      drift=1
    else
      gh api -X PATCH "repos/${REPO}/issues/${fixture_number}" -f state=open >/dev/null &&
        log "reopened the fixture issue #${fixture_number}"
    fi
  fi
  # Comments on the fixture issue. The baseline has none, and a trial that
  # leaves one is not a harmless artifact: GH-02 asks a client to report the
  # issue title, and a prior trial's answer sitting in the comment thread is a
  # crib for the next one. This is not hypothetical. In the 2026-08-18
  # shakedown Gemini CLI answered GH-02 by calling add_issue_comment with the
  # title in the body rather than by saying it in its final message, which
  # failed the transcript check on its merits and left the answer in the repo.
  # The verification below did not notice, because nothing was comparing
  # comment counts.
  mapfile -t fixture_comments < <(
    gh api "repos/${REPO}/issues/${fixture_number}/comments?per_page=100" --paginate \
      --jq '.[].id' 2>/dev/null || true
  )
  if [[ ${#fixture_comments[@]} -gt 0 ]]; then
    if ((CHECK_ONLY)); then
      log "drift: ${#fixture_comments[@]} comment(s) on the fixture issue; the baseline has none"
      drift=1
    else
      for c in "${fixture_comments[@]}"; do
        [[ -n "$c" ]] || continue
        if gh api -X DELETE "repos/${REPO}/issues/comments/${c}" >/dev/null 2>&1; then
          log "deleted comment ${c} on the fixture issue"
        else
          log "could not delete comment ${c} on the fixture issue"
          drift=1
        fi
      done
    fi
  fi

  labels="$(gh api "repos/${REPO}/issues/${fixture_number}" --jq '[.labels[].name] | join(",")' 2>/dev/null || true)"
  if [[ ",${labels}," != *",${FIXTURE_LABEL},"* ]]; then
    if ((CHECK_ONLY)); then
      log "drift: the fixture issue is missing the ${FIXTURE_LABEL} label"
      drift=1
    else
      gh api -X POST "repos/${REPO}/issues/${fixture_number}/labels" \
        -f "labels[]=${FIXTURE_LABEL}" >/dev/null &&
        log "restored the ${FIXTURE_LABEL} label"
    fi
  fi
fi

# ------------------------------------------------------- branches and PRs ---

mapfile -t probe_branches < <(
  gh api "repos/${REPO}/branches?per_page=100" --paginate \
    --jq '.[] | select(.name | startswith("tier2/")) | .name' 2>/dev/null || true
)

if [[ ${#probe_branches[@]} -gt 0 ]]; then
  if ((CHECK_ONLY)); then
    log "drift: ${#probe_branches[@]} tier2/* branch(es) present"
    drift=1
  else
    for b in "${probe_branches[@]}"; do
      # Close any pull request from the branch first. Deleting the ref closes
      # the PR implicitly, but closing it explicitly leaves a cleaner state if
      # the ref delete fails.
      mapfile -t prs < <(
        gh api "repos/${REPO}/pulls?state=open&per_page=100&head=${REPO%%/*}:${b}" \
          --jq '.[].number' 2>/dev/null || true
      )
      for p in "${prs[@]}"; do
        [[ -n "$p" ]] || continue
        gh api -X PATCH "repos/${REPO}/pulls/${p}" -f state=closed >/dev/null 2>&1 &&
          log "closed pull request #${p}"
      done
      if gh api -X DELETE "repos/${REPO}/git/refs/heads/${b}" >/dev/null 2>&1; then
        log "deleted branch ${b}"
      else
        log "could not delete branch ${b}"
        drift=1
      fi
    done
  fi
fi

open_prs="$(gh api "repos/${REPO}/pulls?state=open&per_page=100" --jq 'length' 2>/dev/null || echo 0)"
if [[ "$open_prs" != "0" ]]; then
  log "drift: ${open_prs} open pull request(s) remain; the baseline has none"
  drift=1
fi

# ------------------------------------------------------- baseline contents --
#
# The tag is the definition of record. Comparing main against it catches any
# edit to a tracked file, which is the drift class this script deliberately
# does not repair.

if ! gh api "repos/${REPO}/git/ref/tags/${TAG}" >/dev/null 2>&1; then
  die "baseline tag ${TAG} does not exist in ${REPO}; the fixture cannot be verified"
fi

changed="$(gh api "repos/${REPO}/compare/${TAG}...main" --jq '[.files[]?.filename] | join(", ")' 2>/dev/null || true)"
if [[ -n "$changed" ]]; then
  log "drift: main differs from ${TAG} in: ${changed}"
  log "this is not auto-repaired; restore the files or re-tag deliberately"
  drift=1
fi

# The two content invariants the task criteria actually read, checked against
# the live default branch rather than against the tag, because that is what a
# trial will see.
target_line="$(gh api "repos/${REPO}/contents/NOTES.md" --jq '.content' 2>/dev/null |
  tr -d '\n' | base64 -d 2>/dev/null | grep -E '^TARGET:' || true)"
if [[ "$(echo "$target_line" | tr -d '\r')" != "TARGET: 42" ]]; then
  log "drift: NOTES.md TARGET line is \"${target_line}\", expected \"TARGET: 42\""
  drift=1
fi

docs_files="$(gh api "repos/${REPO}/contents/docs" --jq '[.[].name] | sort | join(",")' 2>/dev/null || true)"
if [[ "$docs_files" != "alpha.md,beta.md" ]]; then
  log "drift: docs/ holds \"${docs_files}\", expected \"alpha.md,beta.md\""
  drift=1
fi

if ((drift)); then
  echo "reset.sh: ${REPO} is NOT at baseline" >&2
  exit 1
fi

echo "reset.sh: ${REPO} is at the ${TAG} baseline"
