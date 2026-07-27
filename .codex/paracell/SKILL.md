---
name: paracell
description: Operate and maintain Paracell's isolated git worktree, tmux, and container workspaces. Use when Codex needs to initialize or edit paracell.yaml, choose or create a template, fork or re-enter a cell for an issue or task, inspect cell state, mark work pending or ready, clean a cell, troubleshoot Paracell, or keep Paracell's project-local automation current with CLI or configuration changes.
---

# Paracell

Use Paracell to give each issue or task an isolated worktree, branch, tmux session, and optional containers. Adapt templates to the repository and task instead of assuming a fixed workflow.

## Inspect Before Acting

1. Confirm that `paracell` is installed with `command -v paracell` and inspect its version with `paracell version`.
2. Resolve the project root in this order:
   - Use `$PARACELL_ROOT` when set.
   - Otherwise, walk upward from the working directory for `paracell.yaml`.
   - Otherwise, use the git root when initialization is part of the request.
3. Read the root `paracell.yaml` and run `paracell ls` before creating, changing, or cleaning cells.
4. Read [references/configuration.md](references/configuration.md) before editing `paracell.yaml`, choosing branch behavior, or configuring containers.
5. Treat the installed CLI as authoritative for available commands. When working in the Paracell source repository, prefer the checked-out source and tests over an older installed binary.

## Choose the Operation

- Initialize a project: run `paracell init` at the git root only when no `paracell.yaml` exists, then set `project.name` and tailor the generated templates.
- Create isolated work: reuse a suitable template or add the smallest task-specific template, then run `paracell fork <issue-or-name> --template <template>`.
- Pass an initial instruction: add `--command <instruction>` only when a session command consumes `{{.Command}}`.
- Resume work: use `paracell view` or enter the root session with `paracell`; do not create a duplicate cell.
- Inspect state: use `paracell ls`. Use `paracell view` when interactive TUI access is useful.
- Update agent state: run `paracell pending` when starting or resuming work and `paracell ready` when user attention is needed. These require a cell session with `PARACELL_CELL` set.
- Finish or remove work: mark `done` in the TUI when appropriate. Run `paracell clean <cell>` only when removal is requested or clearly required by the workflow.

## Create and Update Templates

Edit `paracell.yaml` directly when existing templates do not fit. Preserve unrelated project-specific settings and existing templates.

Choose settings from the actual task:

- Use a stable branch prefix such as `feat/`, `fix/`, `review/`, or a repository convention.
- Use `base: current` when isolation must start from the current branch; otherwise use an explicit base branch.
- Use `branchMode: create` for new work, `reuse` for resumable work, and `require` when an existing branch must already exist.
- Add `files` only for untracked or ignored local inputs needed inside the cell.
- Configure containers only when the task needs service isolation or copied data.
- Keep tmux windows minimal. Use `{{.issue}}`, `{{.name}}`, and `{{.Command}}` only where their runtime expansion is intended.

After editing configuration, parse the YAML with an available YAML tool and run the repository's relevant config tests when working in Paracell source. Do not create a disposable cell merely to validate configuration unless the user accepts the resulting worktree, session, and container side effects.

## Safety Rules

- Resolve an exact cell name with `paracell ls` before cleaning.
- Do not rely on `--force` to preserve or recover work. Check the installed version's behavior; in the current source it is accepted by the parser but does not bypass the done requirement.
- Inspect git status and preserve work before cleanup. A cell can contain commits or uncommitted changes not present elsewhere.
- Do not hand-edit `.paracell/state.json`; use Paracell commands.
- Do not manually remove Paracell worktrees, tmux sessions, containers, volumes, or networks while `paracell clean` can manage them.
- Do not run status commands outside a cell by fabricating `PARACELL_CELL`.

## Maintain This Skill

When changing Paracell's commands, configuration schema, environment variables, or lifecycle behavior in this repository:

1. Update this skill and [references/configuration.md](references/configuration.md) in the same change.
2. Keep instructions compatible with the behavior covered by source tests.
3. Keep `agents/openai.yaml` aligned with the skill's purpose.
4. Run the skill validator after material edits.

Keep detailed configuration facts in the reference file and keep this workflow concise.
