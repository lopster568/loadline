# Self-Submission

> STATUS: draft v0. Public-bound: requires owner review before publication.

## 1. Purpose

This describes how a server author or maintainer requests that their server be considered for inclusion. It covers what to submit and how the request is handled. It does not restate the selection gates themselves; those live in `docs/server-selection.md` and are referenced here, not duplicated.

## 2. What to provide

A submission includes:

1. **Repository or package spec.** A link to the source repository, or, for a server with no public repo, the published package (npm, PyPI, container image) or vendor documentation URL.
2. **Auth requirements.** What credential the server needs to enumerate its full tool surface: none, an API key, OAuth, or another scheme.
3. **Free-tier credential path.** How a free-tier, no-cost credential can be obtained to run the enumeration. If no free-tier path exists, say so; this is checked directly and affects the outcome.

Submit by opening an issue against the project repository. The repository path is published at first public release; use the submission template at `.github/ISSUE_TEMPLATE/server-submission.md` in that repository.

## 3. What gets checked

Every submission is run through the five mandatory gates defined in `docs/server-selection.md` section A.2. See that document for the gate definitions and thresholds; they are not repeated here, and this document does not override them. The result, pass or fail, is published, with the specific gate cited if the submission fails.

## 4. Queue behavior

Submissions are not processed on demand. They are queued and processed at the next monthly Tier 1 run; the monthly cadence is policy, not an automated schedule. A submission received partway through a month waits for that month's run; it is not run ahead of the queue.

A submission that passes all five gates is added to the ranked set at that run if it wins its category's ranking under `docs/server-selection.md` section A.4. If it passes the gates but does not win its category, it is recorded as qualified and reconsidered every time a slot in its category opens, without needing to be resubmitted.

## 5. What this does not guarantee

Submitting a server does not guarantee inclusion. Passing the gates does not guarantee a ranked slot; category size and diversity rules apply, per `docs/server-selection.md` section A.3.

Inclusion does not imply endorsement. A server appearing in the rankings is a statement about what the harness measured, not a statement that the operator recommends the server, vouches for its quality, or endorses its vendor.
