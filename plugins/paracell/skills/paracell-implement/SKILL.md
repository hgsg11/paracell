---
name: paracell-implement
description: "Implement an identified development ticket inside a Paracell cell, verify the change, and deliver it through a pull request. Use for ticket-backed implementation work in a cell; do not use for outside-cell ticket preparation or dispatch."
---

# Paracell Ticket Implementation

Use this skill to implement an identified development ticket from inside a Paracell cell. The ticket is the single source of truth. Ticket providers vary by project; use the available provider integration rather than assuming GitHub Issues. Do not reopen finalized requirements unless a concrete blocker prevents implementation.

## Inspect the Cell Environment First

Before reading the ticket, find out where you are and what the cell contains. Use the `paracell` skill only for this inspection: read its configuration reference (`references/configuration.md`) to interpret what you find, and do not run its dispatch workflow or any lifecycle operation (`fork`, `clean`, `annotate`) from inside the cell.

1. Read `PARACELL_CELL` (the current cell name) and `PARACELL_ROOT` (the project root). If `PARACELL_CELL` is empty, you are not in a cell; stop and report that instead of implementing.
2. Run `paracell ls` from `PARACELL_ROOT` and find the row for the current cell: its template, creation status, and any failed stage.
3. Read `PARACELL_ROOT/paracell.yaml` and the cell's template. From its `targets` and `dependencies`, determine:
   - each target repository, its worktree at `.paracell/cells/<cell>/<source.path>` (the cell directory mirrors the workspace, so `source.path: .` makes the cell directory itself the worktree), and the branch it is on (`git -C <worktree> branch --show-current`);
   - the containers copied for each target and the shared dependency containers (`docker ps --filter network=paracell-<project>-<cell>` when containers are enabled).
4. Report the environment in a few lines (cell, template, targets with worktree and branch, containers) before starting work. Make code changes only inside the target worktrees.

## Workflow

1. Confirm the current cell context and identify the supplied ticket. Read the full ticket through its provider integration. Paracell's current CLI may expose the identifier using the legacy `issue` argument or `{{.issue}}` variable; treat it as an arbitrary string identifier, not necessarily a number.
2. Read the repository instructions and inspect the relevant code before editing. Implement only the ticket's scope; ask a focused question only when an unresolved requirement materially blocks safe implementation.
3. Run the relevant tests and checks, review the diff for unrelated changes, and report any verification that could not be completed.
4. Unless the user requests a different delivery, hand the work to review as described in "Deliver Through Review" below.
5. Report the PR and verification results. Do not claim completion after tests alone when the requested/default delivery includes a PR. If repository access or push is blocked, preserve local work and report the blocker.

## Deliver Through Review

Check `providers.review` in `PARACELL_ROOT/paracell.yaml`.

When it is `github`, Paracell pushes, opens the PRs, waits for the review, and merges; do not push or create PRs with `gh` yourself:

1. Commit every change. `paracell review` fails while any worktree has uncommitted changes.
2. Write the PR body to a file outside the worktrees (for example in a temporary directory). Paracell appends the ticket closing reference (`Closes #<issue>` on GitHub), so you do not need to add it.
3. Run `paracell review --title "<title>" --body-file <file>` **in the background** (for example with Bash `run_in_background`), then end your turn. It opens one PR per target that has commits, moves the cell to in review, and waits without using tokens. Claude Code wakes you with its output when it exits.
4. When you are woken, follow the output:
   - New reviews or comments: read them with `gh` (for example `gh pr view <number> --comments`), run `paracell implement`, fix, commit, and run `paracell review` (no arguments after the first time) in the background again. Describe what you addressed in the commit messages.
   - Merged, closed, or a merge failure: report the result. After a merge failure, fix the cause, commit, and run `paracell review` again.
   - Partly merged and partly closed: stop; a person decides with `paracell merged` or `paracell abandon`.
5. If the background command is cut off by a time limit, run `paracell review` (no arguments) in the background again; it resumes waiting without delivering an event twice.

Never write comments, reviews, or replies on the PRs yourself: Paracell treats every new comment as review feedback, so your own comment would wake you again in a loop. Put responses in commit messages instead.

When `providers.review` is `none` or omitted, commit and push the intended changes, then create or update a review-ready PR against the appropriate base branch with the provider's tools. Reference or close the ticket using the provider's supported syntax. For GitHub Issues, use the appropriate GitHub closing keyword such as `Closes #123`; do not use GitHub syntax for other providers. Right after creating the PR, run `paracell review` to move the cell to in review. When review comments send you back, run `paracell implement` before editing and `paracell review` again after pushing the fixes.

Do not run `paracell merge`, `paracell merged`, or `paracell abandon` from inside the cell; a person decides those from outside.

Honor explicit requests for local changes only, no PR, or another delivery artifact. Do not automatically create another ticket or cell from inside the current cell.
