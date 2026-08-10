---
name: paracell
description: "Dispatch development work to a Paracell cell when both conditions are true: (1) the request requires a system change to source code, tests, application or infrastructure configuration, schemas, build or deployment files, plugins, or skills; and (2) the target project root or an ancestor directory contains paracell.yaml. Also use for explicitly requested root lifecycle operations in a configured project. Do not auto-trigger for explanation, investigation, review, planning, or status-only requests without a system change, or in projects without paracell.yaml."
compatibility: "Requires shell access, paracell with fork --name support, and a target project containing paracell.yaml."
---

# Paracell

Dispatch a self-contained development task from a configured project root to the most suitable existing Paracell template. Do not modify the requested system in the root workspace.

## Run the Root Workflow

Follow these steps in order:

1. Confirm the CLI with `command -v paracell` and `paracell version`. Inspect `paracell fork --help` or the checked-out CLI source to verify `fork --name` support. If it is unavailable, report that a compatible version is required and stop; never fall back to a positional identifier.
2. Starting at the working directory, walk ancestors upward. Use the nearest directory containing `paracell.yaml` as the project root. Stop if none exists.
3. Read the entire root `paracell.yaml`, then run `paracell ls` from that root. Prefer checked-out repository source and configuration over an older installed binary or this reference.
4. Evaluate every declared template field by field. Resolve inheritance, exclude abstract templates, and remove choices incompatible with hard constraints such as repository base, branch mode, copied files, container or network needs, and session commands. Prefer the task intent's closest purpose and branch prefix, then fewer unnecessary files, services, containers, and windows. Break ties by the more specific semantic match and then declaration order. If none fits, explain the missing capability and stop without changing `paracell.yaml`.
5. Generate a short, meaningful lowercase kebab-case name from the task objective. Do not require a provider or external identifier. Prefer a user-supplied name; an explicitly supplied external identifier may inform the name but is optional. Leave resource-safe normalization to the CLI. If `paracell ls` already contains the name, do not fork: report that the existing cell should be reused, and do not invent a suffix unless the user explicitly requests a new name.
6. Create a concise, self-contained task instruction that lets the cell worker start without another conversation. Include the objective, hard constraints, expected verification, and required delivery when applicable. Do not require any particular backlog service as the source of truth. Exclude secrets, credentials, tokens, private keys, unnecessary environment details, and the full conversation.
7. Dispatch with argument-safe execution, keeping the executable and each argument separate rather than passing an assembled command through another shell:

   ```sh
   paracell fork \
     --name <task-name> \
     --template <template> \
     --command <self-contained-task-instruction>
   ```

   Pass the instruction only through `--command`; do not hand it off through an environment variable or later tmux key input.
8. Run `paracell ls` again and confirm the new cell and selected template. Report the name, template, and dispatched objective, then stop.

Do not open the interactive view, capture panes, monitor the worker, type follow-up input, or operate on its worktree unless the user explicitly requests it.

## Operate the Root Lifecycle Safely

- Use `paracell ls` before and after a fork and to resolve an exact cell.
- Use `paracell retry <cell>` only for a failed creation. A concurrent retry of the same cell fails immediately; do not loop or wait on `retry already in progress`. After an abnormal exit, the lease is reclaimable when its last heartbeat is more than two minutes old.
- `paracell view` is interactive. Start it only when explicitly requested.
- Before `paracell clean <cell>`, resolve the exact target with `paracell ls`, inspect its worktree status, preserve outstanding work, and confirm the installed source's done guard. If the target is ambiguous, stop and ask for confirmation.
- Use only the exact root-session and `exit` syntax supported by the checked-out source or installed CLI.
- Treat `paracell pending` and `paracell ready` as cell-local commands requiring cell context; never run them automatically from the root workflow.
- Never hand-edit `.paracell` state or manually delete managed worktrees, tmux sessions, containers, volumes, or networks.

Read [references/configuration.md](references/configuration.md) only when command details, template fields, variables, containers, runtime resources, or lifecycle behavior are needed.
