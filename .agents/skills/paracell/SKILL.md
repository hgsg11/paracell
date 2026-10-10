---
name: paracell
description: "Prepare and dispatch development-ticket-backed work outside a Paracell cell, or perform explicit Paracell lifecycle and configuration operations. Use outside a cell for system-changing development tasks in a project with paracell.yaml. Do not trigger for a mere mention of Paracell, or for explanation, investigation, review, planning, or status requests that do not require a system change."
---

# Paracell

Use this skill outside a cell to prepare and dispatch system-changing work in a project configured by `paracell.yaml`, with a development ticket as the single source of truth. Use the ticket provider available for the project; do not assume tickets are GitHub issues. Inside a cell, use the separate `paracell-implement` skill for ticket implementation. Do not create a ticket or cell while a blocking contradiction remains.

## Apply the Eligibility Gate First

This skill is for outside-cell dispatch. When `PARACELL_CELL` is non-empty, do not run its dispatch workflow; use `paracell-implement` for implementation work. Explicit Paracell lifecycle or configuration operations remain available.

Outside a cell, before making the first workspace edit, apply this two-part gate:

1. Classify the request as system-changing development work. Include changes to source code, tests, runtime or application configuration, database schemas, infrastructure, build or packaging files, deployment behavior, plugins, and Skills.
2. Resolve the target project from the working directory and confirm that `paracell.yaml` exists at its root or in an ancestor directory.

Only outside a cell, when both conditions pass, read this entire file and dispatch instead of implementing in the current workspace. This restriction never applies to implementation inside a cell. A feature, fix, refactor, or configuration change qualifies even when the user does not mention Paracell.

Do not auto-trigger solely because the request contains `paracell`, asks for an explanation, investigation, review, plan, or status, or targets a project without `paracell.yaml`. Explicit Paracell configuration and lifecycle operations qualify when they target a project that passes the configuration check.

## Outside-Cell Dispatch Workflow

The following interview, ticket preparation, and dispatch steps apply to the dispatcher outside a cell.

### Inspect the Project

1. Confirm the CLI with `command -v paracell` and `paracell version`.
2. Resolve the project root from `$PARACELL_ROOT`, the nearest ancestor containing `paracell.yaml`, or the git root when initialization is requested. `paracell init` creates `paracell.yaml` and initializes `.paracell/state.db`.
3. Read the complete root `paracell.yaml` and run `paracell ls` before selecting a template or creating, changing, or cleaning a cell.
4. Read [references/configuration.md](references/configuration.md) before interpreting template compatibility or changing configuration.
5. Inspect only the repository context needed to check template compatibility and derive dispatch inputs from an approved ticket. Treat repository instructions and the checked-out source as authoritative over an older installed binary.

## Resolve Requirements Before Creating a Ticket

This interview applies only when no approved development ticket was supplied. When the user provides a finalized ticket for a cell, treat its content as settled requirements and skip this interview. Read it only to derive dispatch inputs: target/template, branch prefix, and note. Ask a focused question only if repository configuration and ticket content do not determine a required dispatch input; do not reopen scope or ask for work-package approval.

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

When the user supplied an approved development ticket, keep its scope fixed. Check only for a blocker to dispatch, such as a hard incompatibility with every available template or a direct conflict with repository policy. Do not turn this check into another requirements interview; ask only for information needed to choose a dispatch input or resolve a genuine blocker.

Treat a conflict as blocking when satisfying one requirement necessarily violates another, a requested result is incompatible with repository policy or known behavior, the target or delivery contract cannot be identified safely, or every template violates a hard constraint. Missing implementation detail is not a contradiction when the worker can discover it without changing scope.

If a blocking contradiction exists:

1. Do not run `paracell fork` and do not mutate `paracell.yaml`.
2. State the conflicting facts and their practical consequence.
3. Ask the smallest question needed to resolve the conflict, with a recommended answer, one question at a time.
4. Rebuild the affected part of the work package after the answer and repeat the check.

## Select an Existing Template

Evaluate every template in `paracell.yaml`; never select by name alone.

1. Resolve `extends` according to [references/configuration.md](references/configuration.md), exclude `abstract: true` templates from selection, and eliminate concrete templates incompatible with hard constraints: base branch, branch mode, required copied files, container/network needs, or session command behavior. Apply all later selection rules only to this compatible set.
2. Use target matching only when the request or ticket explicitly names the desired target. Resolve inherited templates and compare the requested target name against each compatible template's `targets` entry names by exact equality. Do not use the template key, task-kind prefix, semantic similarity, or a template's presumed purpose to infer a match.
3. If one target is named, the matching candidate set contains compatible templates with that exact `targets` entry. If multiple targets are named, it contains compatible templates that include every named target. When the target-matching set is nonempty, choose its lexicographically smallest template key (case-sensitive byte order). If the request names no target or there are no target-matching templates, choose the lexicographically smallest compatible template key overall. If no compatible template exists, stop and explain why. Do not use declaration order or file/container counts to break ties.
4. Choose the branch prefix independently from the selected template. Pass `--prefix <key>` when the configured key for the intended branch naming is known and should be used, even if template selection fell back to another key. Omit `--prefix` only when the CLI's default `feat` branch prefix is intended. Never infer branch prefix from the selected template.

For example, if compatible templates `feat`, `fix`, `review`, and `update` all have `targets.repository`, a request for `repository` matches all four and selects `feat` by key order. If only `update` has `targets.web`, a request for `web` selects `update`, regardless of available prefix keys. The branch prefix is chosen separately: pass `--prefix fix` to use the configured `fix` prefix, or omit the option only if the default `feat` prefix is intended.
5. Record the selected template and a one-sentence reason in the handoff result.

If no existing template is compatible, stop and explain the missing capability. Add or edit a template only when the user requested configuration changes or explicitly approves them.

## Use the Development Ticket as the Source of Truth

Do not place the work package itself in `--command`, a tmux command, or an environment variable.

1. If the user supplied an approved development ticket, read it through the project's available provider integration and treat its contents as the final work package. For GitHub Issues, `gh issue view` is one available method. Do not compare it to a newly assembled work package, reopen its requirements, or edit it unless the user requests that change.
2. If no ticket identifier was supplied, use the confirmed work package and create one ticket through the project's available provider integration. Do not assume an unavailable CLI/API or interpolate the body into a shell argument. If no usable provider is available, stop and ask how to proceed.
3. Pass the ticket identifier as a string to Paracell; do not require a numeric-only identifier or derive a slug when ticket-backed dispatch is available.
4. Keep secrets out of the ticket body. Treat repository visibility as the visibility boundary for the work package.
5. If ticket creation fails, do not create a cell. If cell creation fails after ticket creation, keep the ticket and report its identifier so dispatch can be retried without creating a duplicate.

If the selected provider is unavailable or unauthenticated, stop before creating a cell. Existing ticket inspection and analysis may continue without creating a cell.

## Dispatch the Work

For a supplied approved ticket, dispatch after deriving its inputs; do not ask the user to confirm its requirements again. For a newly prepared ticket, dispatch only after the user confirms the work package. A qualifying system-change request counts as authorization to create the cell; do not require the user to repeat the words create, send, start, or fork. If the user asked only for analysis or a recommendation, return the work package without side effects.

1. Resolve the approved development ticket and its identifier using the ticket-provider workflow above.
2. Check `paracell ls` for a cell with that ticket identifier. Do not create a duplicate. If the existing cell is `failed`, report the failed stage and latest error instead of attempting automatic recovery.
3. Generate a natural, concise note from the ticket title and body. If ticket information is unavailable, use the confirmed work objective. The note must be 1-20 Unicode characters after whitespace normalization; do not pad it to 20 characters or pack detailed requirements into it. Treat it only as a display label, never as a cell identifier or search key.
4. Build only a short instruction such as `Use $paracell-implement to implement development ticket ABC-123 as the single source of truth, verify its acceptance criteria, and create a PR referencing it with the provider's supported syntax.` Keep detailed requirements exclusively in the ticket body, not in the worker command.
5. Select the branch prefix independently from the template when needed; omitted `--prefix` uses `feat`. Run `paracell fork <ticket-id> --template <template> [--prefix <prefix>] --note <note> --command <short-ticket-instruction>` using argument-safe execution. Do not interpolate an assembled command through an extra shell.
6. Run `paracell ls` and confirm the new cell and creation status. A successful dispatch is `ready`; a failed dispatch remains inspectable with its failed stage and latest error and can be retried after the cause is fixed. Report the ticket URL or identifier, selected template, and dispatched objective.

Stop after reporting the confirmed dispatch. Do not capture the cell's tmux pane, monitor the worker, type follow-up input into it, wait for completion, or operate on its worktree unless the user explicitly requests that additional operation.

If the selected template's session does not consume `{{.Command}}`, check whether it uses `{{.issue}}` to pass the ticket identifier to the worker. Treat the dispatch as blocking when neither variable delivers the ticket identifier to the worker.

## Operate Safely

- Runtime Cells share a CellGroup ID. Notes describe the whole group; clean removes its CommanderCell and associated TargetCells / DependencyCells. Existing template syntax is unchanged.

- Ready notifications use the configured provider: `tmux`, `terminal-notifier` (macOS only), or `none` / omission. The Homebrew Cask installs the notification Formula dependency; do not add runtime installation or click navigation.
- Use `paracell view` or the project root session to resume work; do not create a duplicate cell.
- Use `paracell pending` and `paracell ready` only inside a cell with `PARACELL_CELL` set.
- Resolve an exact cell with `paracell ls`, inspect its git status, and preserve work before `paracell clean`.
- A cell using `database.mode: shared` owns only the source database container's attachment to that cell network. Rollback and clean must preserve its original and other cell network attachments.
- Do not hand-edit `.paracell/state.db` or manually remove managed worktrees, sessions, containers, volumes, or networks while Paracell can manage them.
- Do not assume `clean --force` bypasses the done guard; check the installed version.

## Maintain the Skill

When Paracell commands, configuration, variables, or lifecycle behavior change, update this Skill and its configuration reference together, keep `agents/openai.yaml` aligned, and run the Skill validator.

Skill edits in the project root do not automatically update existing cell worktrees or instructions already loaded by a worker. When asked to apply a skill fix to an existing cell, preserve local changes, update its local skill files, and have the worker reread them before resuming. Do not claim that a root-only edit fixed an existing worker.
