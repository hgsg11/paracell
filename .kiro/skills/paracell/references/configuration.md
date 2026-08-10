# Paracell Configuration Reference

Use this reference when selecting a template, interpreting `paracell.yaml`, preparing a dispatch command, or performing a root lifecycle operation. Prefer the checked-out repository source and root configuration when they differ from an installed binary or this document.

## Lifecycle Commands

| Command | Purpose | Constraint |
| --- | --- | --- |
| `paracell` | Enter the project root tmux session | Run in the configured project or with `PARACELL_ROOT` set |
| `paracell init` | Create `paracell.yaml` | Fails when the file already exists |
| `paracell fork --name <name> --template <template> --command <instruction>` | Create and start a named cell | Requires a CLI version with `fork --name`; never fall back to a positional identifier |
| `paracell retry <cell>` | Resume a failed creation | Acquires a per-cell lease; concurrent retry fails immediately and completed stages are retained |
| `paracell view` | Open the cell and template TUI | Interactive; launch only on explicit request |
| `paracell ls` | List cells and their state | Use before and after dispatch and before destructive actions |
| `paracell clean <cell> [--force]` | Remove managed resources for one cell | Confirm the exact cell, worktree state, and installed done guard first |
| `paracell pending` | Mark the current cell pending | Cell-local; requires `PARACELL_CELL` or source-supported cell detection |
| `paracell ready` | Mark the current cell ready and optionally notify | Cell-local; requires `PARACELL_CELL` or source-supported cell detection |
| `paracell exit` | Detach the current managed tmux client | Use only in a managed session |
| `paracell version` | Print build version information | Safe preflight inspection |

The no-argument `paracell` root-session command and `paracell exit` are the exact forms in `internal/app/cli.go`. Recheck that source before documenting or using a different form.

## Named Fork Contract

Dispatch uses this argument structure:

```sh
paracell fork \
  --name <lowercase-kebab-case-name> \
  --template <template> \
  --command <self-contained-task-instruction>
```

- Confirm that the installed command supports `--name`; otherwise stop and report the required capability.
- Keep executable and arguments separate. Do not assemble the command and evaluate it through another shell.
- Names are short, readable, provider-independent kebab-case. The CLI owns final resource-safe normalization.
- Pass the worker instruction only in `--command`. It must be self-contained without making an external backlog item mandatory.
- Check `paracell ls` first. Reuse an exact-name match rather than creating a duplicate or silently adding a suffix.

## Configuration Shape

```yaml
project:
  name: my-project
providers:
  source: git
  container: docker
  session: tmux
  notifications: tmux
templates:
  base:
    abstract: true
    repository:
      base: main
      branchMode: create
    files:
      - .env
    containers:
      network: isolated
      services: {}
    session:
      windows:
        - name: agent
          command: 'worker "{{.Command}}"'
  feat:
    extends: base
    repository:
      branchPrefix: feat/
```

`providers.container` is optional when the project needs no Docker-backed services. Current providers are `git` for source, `docker` for containers, and `tmux` for sessions and notifications.

## Template Selection Fields

Evaluate the fully resolved fields of every concrete template:

- Template key and `repository.branchPrefix` indicate intended use such as feature, fix, or review, but field compatibility takes precedence.
- `extends` names one parent. Inheritance chains are allowed; missing parents and cycles are invalid.
- `abstract: true` marks a reusable parent that cannot be selected for a fork.
- `repository.base` is an explicit branch or `current`.
- `repository.branchMode` controls branch handling:
  - omitted or `create`: require creation of a new branch;
  - `reuse`: reuse an existing branch or create it when absent;
  - `require`: require an existing branch.
- `files` lists project-root-relative inputs copied into the cell source.
- `containers.network` is `isolated` or `shared`.
- `containers.services` declares copied runtime services.
- `session.windows` defines tmux windows and startup commands. A dispatch-capable template must consume `{{.Command}}` in the appropriate worker command.

An omitted child scalar or struct field inherits its parent value. A supplied child scalar replaces the parent, including an explicit empty value. Supplied slices and maps such as `files`, `session.windows`, and `containers.services` replace the whole inherited collection; they are not appended or deep-merged. Explicit `[]` and `{}` clear inherited collections.

Eliminate templates that violate task hard constraints. Among compatible templates, prefer the closest intent and branch-prefix match, fewer unnecessary copied files and runtime resources, then the more semantically specific template and declaration order. Do not mutate configuration when no template fits.

## Template Variables and Command Handoff

Session commands may use:

- `{{.name}}`: the resulting cell name.
- `{{.project}}`: the configured project name.
- `{{.Command}}`: the exact instruction supplied by `fork --command`; empty when omitted.
- `{{.issue}}`: a legacy/general template value when supported by the checked-out source; the named root workflow does not depend on it.

Variables are rendered before the session shell starts. Keep YAML quoting, Go-template delimiters, and shell quoting distinct. The task instruction belongs only in `--command`, not an environment variable or later tmux input.

## Containers and Databases

- Each `containers.services.<role>` can set `sourceContainer`, `volumeMode`, `environment`, and optional `database` configuration.
- `volumeMode` supports `copy` and `readonly` for ordinary services.
- Database copying is supported only for the `db` role with `volumeMode: copy`.
- `database.system` currently supports `mysql`.
- `database.copyMode: schema` copies schema. `data` is reserved but not implemented in the current source.
- `database.initFiles` entries are project-root-relative and must stay within the project root.
- Prefer templates without containers, services, or isolated networks when the task does not need them.

## Runtime Environment and Managed Resources

- `PARACELL_ROOT` points commands to the managed project root.
- `PARACELL_CELL` identifies a cell within its managed session. Current source can also infer a cell from `.paracell/cells/<cell>/source` after session restoration, but root automation must not rely on that to run status commands.
- `.paracell/state.db` is the managed SQLite state store. Retry ownership uses an attempt ID, a 10-second heartbeat, and a lease reclaimable when the last heartbeat is more than two minutes old. Do not edit it directly.
- `.paracell/cells/<cell>/source` is the managed cell worktree.
- Root tmux sessions use `<project>-root`; cell sessions use `<project>-<cell>`.
- Creation proceeds through source, files, containers, and session stages. Failed creation retains completed resources for `retry`.

Do not manually remove managed worktrees, tmux sessions, containers, volumes, or networks. Use the lifecycle command after resolving the exact cell and safeguarding work.

## Repository Sources to Check

When behavior may have changed, inspect these checked-out files:

- `internal/app/cli.go` for commands, project-root behavior, and environment handling.
- `internal/domain/template.go` for template fields, variables, branch modes, networks, and database shape.
- `internal/adapter/config/yaml_config.go` for YAML mapping, inheritance, rendering, and validation.
- `internal/usecase/create_cell.go` and `internal/usecase/retry_cell.go` for creation and retry behavior.
- `internal/usecase/init_project.go` for generated defaults.
- `README.md` for the current user-visible workflow.
