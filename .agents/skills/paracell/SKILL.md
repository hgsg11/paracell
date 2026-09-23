---
name: paracell
description: "Prepare and dispatch issue-backed development work outside a Paracell cell; inside a cell, implement issue-backed work through a pull request. Use outside a cell for system-changing development tasks in a project with paracell.yaml, and inside a cell for implementation of an identified issue. Also use for explicit Paracell lifecycle or configuration operations. Do not trigger for a mere mention of Paracell, or for explanation, investigation, review, planning, or status requests that do not require a system change."
---

# Paracell

Outside a cell, turn system-changing development work in a project configured by `paracell.yaml` into a coherent GitHub issue and hand its issue number to the most suitable existing Paracell template. Treat the issue body as the single source of truth. Do not create an issue or cell while a blocking contradiction remains.

## Apply the Eligibility Gate First

Check whether the current session is inside a cell before applying the dispatch gate. When `PARACELL_CELL` is non-empty, perform requested development work directly in the current cell. Do not apply the dispatch workflow or its prohibition on direct implementation, and do not automatically create another issue or cell. Direct implementation does not require the task's issue number to match the cell identifier. Explicitly requested Paracell lifecycle or configuration operations remain available.

For an identified issue's implementation inside a cell, treat a review-ready PR as the default delivery artifact, even if the user says only “implement issue #123.” After implementation and verification, commit and push the intended changes, create or update a PR against the appropriate base branch, link the issue, and report the PR URL. Do not report the issue implementation complete after tests alone. Honor an explicit request for local changes only, no PR, or another delivery artifact; if GitHub access or push is blocked, report the blocker and preserve the local work. This default does not apply to investigation, review, planning, or an unrelated change merely made inside a cell.

Outside a cell, before making the first workspace edit, apply this two-part gate:

1. Classify the request as system-changing development work. Include changes to source code, tests, runtime or application configuration, database schemas, infrastructure, build or packaging files, deployment behavior, plugins, and Skills.
2. Resolve the target project from the working directory and confirm that `paracell.yaml` exists at its root or in an ancestor directory.

Only outside a cell, when both conditions pass, read this entire file and dispatch instead of implementing in the current workspace. This restriction never applies to implementation inside a cell. A feature, fix, refactor, or configuration change qualifies even when the user does not mention Paracell.

Do not auto-trigger solely because the request contains `paracell`, asks for an explanation, investigation, review, plan, or status, or targets a project without `paracell.yaml`. Explicit Paracell configuration and lifecycle operations qualify when they target a project that passes the configuration check.

## Outside-Cell Dispatch Workflow

The following interview, issue preparation, and dispatch steps apply to the dispatcher outside a cell, not to the worker implementing inside a cell.

### Inspect the Project

1. Confirm the CLI with `command -v paracell` and `paracell version`.
2. Resolve the project root from `$PARACELL_ROOT`, the nearest ancestor containing `paracell.yaml`, or the git root when initialization is requested. `paracell init` creates `paracell.yaml` and initializes `.paracell/state.db`.
3. Read the complete root `paracell.yaml` and run `paracell ls` before selecting a template or creating, changing, or cleaning a cell.
4. Read [references/configuration.md](references/configuration.md) before interpreting template compatibility or changing configuration.
5. Inspect only the repository context needed to settle requirements and selection criteria. Treat repository instructions and the checked-out source as authoritative over an older installed binary.

## Grill the Requirements

Treat requirements as a decision tree. Resolve parent decisions before the choices that depend on them, and keep exploring until every branch that can materially change the result has been settled.

1. Extract what is explicit, what is assumed, and what remains undecided from the request.
2. Explore the codebase and available tools before asking anything they can answer. Facts are discoverable; product and tradeoff decisions belong to the user.
3. Pick the most load-bearing unresolved decision.
4. Ask exactly one focused question and wait for the answer. Never send a batch of questions.
5. Include a recommended answer and a short reason with every question so the user can react to a concrete proposal.
6. Incorporate the answer, revisit dependent branches, and repeat.
7. Challenge vague language, hidden assumptions, edge cases, failure behavior, boundaries, and deferred “figure it out later” decisions only when they can materially affect the outcome.

Do not implement, mutate configuration, select a final template, or dispatch while the interview is active. For a small mechanical request with no material decision branches, state the inferred work package and ask the user to confirm it instead of inventing interview questions.

Build a concise, self-contained work package as decisions settle:

- Objective: the user-visible or operational outcome.
- Scope: included behavior, excluded behavior, and likely affected area.
- Requirements: concrete functional and non-functional constraints.
- Acceptance criteria: observable conditions that prove completion.
- Verification: relevant tests, checks, or manual evidence.
- Delivery: expected artifact such as code, report, commit, or pull request.
- Context: identifiers, paths, links, and decisions the worker must retain.
- Assumptions: only reversible defaults that do not materially change scope.

Do not force the user to provide implementation details that can be discovered safely in the cell. Preserve explicit wording when it is a hard constraint.

The interview is complete only when no material decision branch remains, the work package contains no blocking contradiction, and the user explicitly confirms that it matches the intended outcome. If the user changes an earlier decision, reopen its dependent branches before asking for confirmation again.

## Check for Contradictions

Compare the work package against itself, the user's latest instructions, repository facts and policies, existing cells, and template capabilities.

Treat a conflict as blocking when satisfying one requirement necessarily violates another, a requested result is incompatible with repository policy or known behavior, the target or delivery contract cannot be identified safely, or every template violates a hard constraint. Missing implementation detail is not a contradiction when the worker can discover it without changing scope.

If a blocking contradiction exists:

1. Do not run `paracell fork` and do not mutate `paracell.yaml`.
2. State the conflicting facts and their practical consequence.
3. Ask the smallest question needed to resolve the conflict, with a recommended answer, one question at a time.
4. Rebuild the affected part of the work package after the answer and repeat the check.

## Select an Existing Template

Evaluate every template in `paracell.yaml`; never select by name alone.

1. Resolve `extends` according to [references/configuration.md](references/configuration.md), exclude `abstract: true` templates from selection, and eliminate concrete templates incompatible with hard constraints: base branch, branch mode, required copied files, container/network needs, or session command behavior.
2. Prefer the template whose purpose and branch prefix most specifically match the task: for example, a bug repair generally favors `fix`, while new behavior generally favors `feat`.
3. Prefer fewer unnecessary files, containers, services, and session windows.
4. Break a remaining tie by the more specific semantic match, then by declaration order in `paracell.yaml`.
5. Record the selected template and a one-sentence reason in the handoff result.

If no existing template is compatible, stop and explain the missing capability. Add or edit a template only when the user requested configuration changes or explicitly approves them.

## Use the Issue as the Source of Truth

Do not place the work package itself in `--command`, a tmux command, or an environment variable.

1. If the user supplied an issue number, read it with `gh issue view` and confirm that its body matches the approved work package. Update a stale body only after the user approves the changed requirements.
2. If no issue number was supplied, write the approved work package to a temporary Markdown file and create one GitHub issue with `gh issue create --body-file`. Use a concise title and never interpolate the body through a shell argument.
3. Use the returned numeric issue number as the Paracell identifier. Do not derive a slug when issue-backed dispatch is available.
4. Keep secrets out of the issue body. Treat repository visibility as the visibility boundary for the work package.
5. If issue creation fails, do not create a cell. If cell creation fails after issue creation, keep the issue and report its number so dispatch can be retried without creating a duplicate.

Treat a missing `gh` executable, missing GitHub authentication, or a repository without a usable GitHub remote as blocking for new issue-backed dispatch. Existing issue inspection and analysis may continue without creating a cell.

## Dispatch the Work

Dispatch only when the eligibility gate passed and the user has confirmed the shared understanding reached by the requirements interview. A qualifying system-change request counts as authorization to create the cell; do not require the user to repeat the words create, send, start, or fork. If the user asked only for analysis or a recommendation, return the work package without side effects.

1. Resolve the approved GitHub issue and its numeric issue number using the issue-backed workflow above.
2. Check `paracell ls` for a cell with that issue number. Do not create a duplicate. If the existing cell is `failed`, report the failed stage and latest error instead of attempting automatic recovery.
3. Generate a natural, concise note from the ticket title and body. If ticket information is unavailable, use the confirmed work objective. The note must be 1-20 Unicode characters after whitespace normalization; do not pad it to 20 characters or pack detailed requirements into it. Treat it only as a display label, never as a cell identifier or search key.
4. Build only a short instruction such as `You are already inside a Paracell cell; implement directly here without redispatch. Read GitHub issue #123 as the single source of truth, verify its acceptance criteria, and create a PR with Closes #123.` Keep detailed requirements exclusively in the issue body and worker command.
5. Run `paracell fork <issue-number> --template <template> --note <note> --command <short-issue-instruction>` using argument-safe execution. Do not interpolate an assembled command through an extra shell.
6. Run `paracell ls` and confirm the new cell and creation status. A successful dispatch is `ready`; a failed dispatch remains inspectable with its failed stage and latest error and can be retried after the cause is fixed. Report the issue URL or number, selected template, and dispatched objective.

Stop after reporting the confirmed dispatch. Do not capture the cell's tmux pane, monitor the worker, type follow-up input into it, wait for completion, or operate on its worktree unless the user explicitly requests that additional operation.

If the selected template's session does not consume `{{.Command}}`, check whether it uses `{{.issue}}` to tell the worker to read the issue. Treat the dispatch as blocking when neither variable delivers the issue number to the worker.

## Operate Safely

- Use `paracell view` or the project root session to resume work; do not create a duplicate cell.
- Use `paracell pending` and `paracell ready` only inside a cell with `PARACELL_CELL` set.
- Resolve an exact cell with `paracell ls`, inspect its git status, and preserve work before `paracell clean`.
- A cell using `database.mode: shared` owns only the source database container's attachment to that cell network. Rollback and clean must preserve its original and other cell network attachments.
- Do not hand-edit `.paracell/state.db` or manually remove managed worktrees, sessions, containers, volumes, or networks while Paracell can manage them.
- Do not assume `clean --force` bypasses the done guard; check the installed version.

## Maintain the Skill

When Paracell commands, configuration, variables, or lifecycle behavior change, update this Skill and its configuration reference together, keep `agents/openai.yaml` aligned, and run the Skill validator.

Skill edits in the project root do not automatically update existing cell worktrees or instructions already loaded by a worker. When asked to apply a skill fix to an existing cell, preserve local changes, update its local skill files, and have the worker reread them before resuming. Do not claim that a root-only edit fixed an existing worker.
