---
name: paracell-implement
description: "Implement an identified development ticket inside a Paracell cell, verify the change, and deliver it through a pull request. Use for ticket-backed implementation work in a cell; do not use for outside-cell ticket preparation or dispatch."
---

# Paracell Ticket Implementation

Use this skill to implement an identified development ticket from inside a Paracell cell. The ticket is the single source of truth. Ticket providers vary by project; use the available provider integration rather than assuming GitHub Issues. Do not reopen finalized requirements unless a concrete blocker prevents implementation.

## Workflow

1. Confirm the current cell context and identify the supplied ticket. Read the full ticket through its provider integration. Paracell's current CLI may expose the identifier using the legacy `issue` argument or `{{.issue}}` variable; treat it as an arbitrary string identifier, not necessarily a number.
2. Read the repository instructions and inspect the relevant code before editing. Implement only the ticket's scope; ask a focused question only when an unresolved requirement materially blocks safe implementation.
3. Run the relevant tests and checks, review the diff for unrelated changes, and report any verification that could not be completed.
4. Unless the user requests a different delivery, commit and push the intended changes, then create or update a review-ready PR against the appropriate base branch. Reference or close the ticket using the provider's supported syntax. For GitHub Issues, use the appropriate GitHub closing keyword such as `Closes #123`; do not use GitHub syntax for other providers.
5. Report the PR URL and verification results. Do not claim completion after tests alone when the requested/default delivery includes a PR. If repository access or push is blocked, preserve local work and report the blocker.

Honor explicit requests for local changes only, no PR, or another delivery artifact. Do not automatically create another ticket or cell from inside the current cell.
