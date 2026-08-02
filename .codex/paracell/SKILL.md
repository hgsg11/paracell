---
name: paracell
description: Prepare and dispatch explicitly requested parallel or isolated work to a Paracell cell by clarifying requirements, detecting contradictions, and selecting the best existing template from paracell.yaml. Use when the user explicitly asks to use Paracell, run work in parallel or concurrently, or move work into another, separate, or isolated cell, including requests phrased as "並行", "並列", or "別cell"; also use for explicit Paracell template, fork, resume, inspection, configuration, or lifecycle operations. Do not trigger solely because the user asks to add, fix, change, update, or refactor something.
---

# Paracell

Turn an explicitly parallel or isolated request into a coherent work package and hand it to the most suitable existing Paracell template. Do not create a cell while a blocking contradiction remains.

## Keep the Trigger Explicit

- Treat an explicit request to use Paracell, work in parallel or concurrently, or use another, separate, or isolated cell as authorization to prepare a Paracell dispatch.
- Do not invoke this Skill merely because the user requests a feature, fix, change, update, refactor, investigation, or review. Handle such requests in the current workspace unless the user also requests parallel or isolated execution.
- Continue to use this Skill when the user explicitly asks to inspect, resume, configure, or clean Paracell resources.

## Inspect the Project

1. Confirm the CLI with `command -v paracell` and `paracell version`.
2. Resolve the project root from `$PARACELL_ROOT`, the nearest ancestor containing `paracell.yaml`, or the git root when initialization is requested.
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

1. Eliminate templates incompatible with hard constraints: base branch, branch mode, required copied files, container/network needs, or session command behavior.
2. Prefer the template whose purpose and branch prefix most specifically match the task: for example, a bug repair generally favors `fix`, while new behavior generally favors `feat`.
3. Prefer fewer unnecessary files, containers, services, and session windows.
4. Break a remaining tie by the more specific semantic match, then by declaration order in `paracell.yaml`.
5. Record the selected template and a one-sentence reason in the handoff result.

If no existing template is compatible, stop and explain the missing capability. Add or edit a template only when the user requested configuration changes or explicitly approves them.

## Dispatch the Work

Dispatch only when the user explicitly requested Paracell, parallel or concurrent execution, or another, separate, or isolated cell and has confirmed the shared understanding reached by the requirements interview. These requests count as authorization to create the cell; do not require the user to repeat the words create, send, start, or fork. If the user asked only for analysis or a recommendation, return the work package and template choice without side effects.

1. Reuse the user's issue number or task identifier. If absent, derive a short stable kebab-case slug only when doing so cannot collide with or misrepresent existing work.
2. Check `paracell ls` for an existing cell for the same work. Resume it instead of creating a duplicate.
3. Render the work package as the `--command` value. Make it self-contained, direct the worker to inspect repository instructions, and include acceptance and verification criteria. Do not include secrets or irrelevant conversation history.
4. Run `paracell fork <identifier> --template <template> --command <work-package>` using argument-safe execution. Do not interpolate an assembled command through an extra shell.
5. Run `paracell ls` and confirm the new cell and status. Report the identifier, selected template, and the dispatched objective.

Stop after reporting the confirmed dispatch. Do not capture the cell's tmux pane, monitor the worker, type follow-up input into it, wait for completion, or operate on its worktree unless the user explicitly requests that additional operation.

If the selected template's session does not consume `{{.Command}}`, warn that the work package cannot be delivered through `--command` and treat that as blocking unless another configured window delivers the instruction.

## Operate Safely

- Use `paracell view` or the project root session to resume work; do not create a duplicate cell.
- Use `paracell pending` and `paracell ready` only inside a cell with `PARACELL_CELL` set.
- Resolve an exact cell with `paracell ls`, inspect its git status, and preserve work before `paracell clean`.
- Do not hand-edit `.paracell/state.json` or manually remove managed worktrees, sessions, containers, volumes, or networks while Paracell can manage them.
- Do not assume `clean --force` bypasses the done guard; check the installed version.

## Maintain the Skill

When Paracell commands, configuration, variables, or lifecycle behavior change, update this Skill and its configuration reference together, keep `agents/openai.yaml` aligned, and run the Skill validator.
