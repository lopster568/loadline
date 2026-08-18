# Right of Reply

> Version 0, in force since 2026-08-18. Changes to this document are changelog events.

| Field | Value |
| --- | --- |
| Scope | Every server whose numbers are about to be published for the first time |
| Window | 14 calendar days before first publication |
| Per PRD.md | Section 4, "Right of reply" |

## 1. Purpose

A server's maintainer sees their own numbers before the public does, gets a chance to flag a factual error before publication, and gets their reply published alongside the row if they choose to respond. This process does not give maintainers control over the methodology or the result; it gives them a documented chance to be heard before their data goes live.

## 2. Timing

The window is 14 calendar days, counted from the day the maintainer is notified to the day the server's numbers are first published. This applies only to first publication of a server's numbers. Routine monthly refreshes of an already-published server do not restart the window.

## 3. What the maintainer receives

At notification, the maintainer is sent:

1. The exact rows that will be published for their server (all metrics, all client modes, all stamps).
2. The raw run artifacts backing those rows (the `tools/list` response, the hashes, the run record).
3. The methodology version the run was measured under. Each release is git-tagged, and the notice links the tag.

## 4. What happens during the window

The maintainer may reply at any point during the 14 days. A reply is capped at a bounded length (500 words). If a reply exceeds the bound, the operator asks for a shortened version; the operator does not edit the maintainer's words to fit.

A published reply appears verbatim alongside the row it responds to. It is not paraphrased, summarized, or edited for tone. The operator may add a short factual note beneath it if the reply contains a factual error, but the reply itself is not altered.

If the maintainer does not respond within the window, the row is published with a neutral note: "No reply received." This is not a claim that the maintainer had no objection, only a record that none was received in time.

## 5. Routing disputes

A maintainer's objection is routed by its kind:

- **Dispute over methodology** (how something is measured, what a metric means, whether a rule is fair): routed to a public issue against the project repo. It is discussed in the open and, if it leads to a methodology change, that change follows the versioning rule in `docs/methodology-v0.md` section 9.
- **Dispute over facts** (a wrong version pinned, a stale server, a miscounted tool, a wrong hash): routed to re-measurement. The harness is re-run against the disputed claim, and the result, whichever way it comes out, is recorded. If the original number was wrong, an entry goes into `corrections-log.md`.

A single objection may contain both kinds. Each part is routed on its own terms.

## 6. Publication does not wait beyond the window

Publication proceeds at the end of the 14-day window regardless of whether a reply was received, unless the operator has independently found a fact-level error that changes the row. In that case the fix happens first, and the corrected row starts a fresh notice, not a fresh 14-day window. A maintainer cannot indefinitely delay publication by not responding, and cannot delay it further by responding late.

## 7. Notification email template (skeleton)

> DRAFT: final wording is the operator's own.

```
Subject: [SERVER] numbers publishing on loadline in 14 days

Hello,

loadline is a standing measurement of MCP server context cost.
Funding sources and conflict-of-interest exclusions are published
in the project's governance directory. [SERVER] is scheduled to
publish for the first time on [PUBLISH_DATE].

Your rows, as they will appear: [ROWS_LINK]
Raw run artifacts: [ARTIFACTS_LINK]
Methodology version measured under: [METHODOLOGY_VERSION_LINK]

You have until [REPLY_DEADLINE] to reply. A reply of up to 500
words will be published verbatim next to your row. If we do not
hear back by then, the row publishes with a neutral note that no
reply was received.

If you believe something is factually wrong (wrong version, wrong
tool count, wrong hash), say so and we will re-run the measurement
before publishing. If your concern is about how we measure rather
than what we measured, please open an issue at [REPO_ISSUES_LINK].

[OPERATOR_NAME]
```
