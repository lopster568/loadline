# Recusal Policy

> Version 0, in force since 2026-08-18. Changes to this document are changelog events.

| Field | Value |
| --- | --- |
| Scope | Funding and employment conflicts affecting the operator |
| Applies to | All Tier 1 and Tier 2 rankings, the leaderboard, and the calculator's default output |
| Publication requirement | Published before the first ranking, per PRD.md section 4 |

## 1. Purpose

This policy governs what happens when the operator has a financial or employment relationship with a server, its vendor, or a party sponsoring the project. It exists so that a conflict is disclosed and resolved by a fixed rule, not by discretion at publication time.

## 2. Funding disclosure

The operator will publish all funding sources for this project before the first ranking, as a standing page kept current thereafter. This includes sponsorships, grants, paid placements, and any other form of financial support. Any change to the disclosure is a changelog event.

## 3. Ownership, employment, and sponsorship exclusion

A server is excluded from rankings if any of the following is true:

1. The operator owns the server or majority-controls the entity that publishes it.
2. The operator is employed by the vendor that publishes the server.
3. The vendor that publishes the server sponsors this project.

An excluded server may still appear on the site, unranked, if it otherwise clears the selection gates in `docs/server-selection.md`. It is labeled clearly as excluded for conflict of interest, with the specific relationship named. It receives no score, no rank position, and no leaderboard placement.

### Standing example: HydraDNS

HydraDNS is the operator's own project. It is excluded from rankings under rule 3, item 1 above. If HydraDNS ever appears on the site, it is listed unranked with an explicit conflict-of-interest label. No research is conducted toward including it in a ranked position, and none should be cited toward that end.

## 4. Employment taken after a server is already ranked

If the operator takes employment at a vendor whose server is already ranked, the following applies from the date employment begins:

1. That vendor's rows freeze at their last-published values. No further Tier 1 or Tier 2 runs are recorded against them.
2. A dated notice is attached to every frozen row, stating the date employment began and the date the freeze took effect.
3. The frozen rows stay visible, marked frozen, until a named independent re-runner takes over measurement of that vendor's servers.
4. The independent re-runner is named publicly. They run the harness with their own credentials and publish under their own name, separate from the operator's ledger.
5. Once the independent re-runner's first result is published, the frozen rows are superseded and the freeze notice is updated to point to the new source.

## 5. Changes to this policy

Any change to this policy is a changelog event: dated, with the prior wording and the new wording both shown, and with the reason for the change stated. No change to this policy is applied retroactively to rows already published under the prior version.

## 6. Relationship to other governance documents

This policy sits alongside `docs/server-selection.md` section A.8, which sets the selection-rule side of the same conflict-of-interest question. Where the two documents overlap, this policy controls the exclude-or-disclose decision; `docs/server-selection.md` controls gate eligibility.
