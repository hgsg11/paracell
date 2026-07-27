# Paracell Configuration Reference

Read this reference when creating or editing `paracell.yaml`, selecting branch behavior, or diagnosing a template.

## Commands

| Command | Purpose | Important constraint |
| --- | --- | --- |
| `paracell` | Enter the project root tmux session | Run from the project or with `PARACELL_ROOT` set |
| `paracell init` | Create `paracell.yaml` | Fails when the file already exists |
| `paracell fork <issue> --template <name> [--command <text>]` | Create a cell | The template must exist |
| `paracell view` | Open the cell/template TUI | Interactive |
| `paracell ls` | List cells and status | Safe inspection command |
| `paracell pending` | Set the current cell to pending | Requires `PARACELL_CELL` |
| `paracell ready` | Set the current cell to ready and optionally notify | Requires `PARACELL_CELL` |
| `paracell clean <cell> [--force]` | Remove managed cell resources | Requires a done cell in the current source; potentially destructive |
| `paracell exit` | Detach the current tmux client | Intended for managed sessions |
| `paracell version` | Print build version information | Safe inspection command |

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
  feat:
    repository:
      branchPrefix: feat/
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
          command: 'codex "{{.Command}}"'
```

`providers.container` is optional. Omit it when no Docker-backed service is required. Supported providers are currently `git` for source, `tmux` for sessions and notifications, and `docker` for containers.

## Repository Settings

- `branchPrefix`: Prefix the issue or task argument to form the branch name.
- `base`: Use an explicit branch or `current`.
- `branchMode`:
  - Omitted or `create`: require a new branch.
  - `reuse`: reuse an existing branch or create it when absent.
  - `require`: require an existing branch.

Use `reuse` for repeated work on a known branch and `require` for review or recovery flows where accidentally creating a branch would be wrong.

The current source accepts `clean --force` syntactically but does not use it to bypass the done guard. Do not treat it as a recovery or data-preservation mechanism without checking the installed version.

## Template Variables

Session window commands support:

- `{{.issue}}`: The argument supplied to `fork`.
- `{{.name}}`: The resulting cell name.
- `{{.Command}}`: The value supplied through `fork --command`; empty when omitted or when forked through the TUI.

Quote YAML and shell commands carefully. A template expression is rendered before the shell starts.

## Files and Containers

- `files` contains project-root-relative paths copied into the cell source.
- `containers.network` accepts `isolated` or `shared`.
- Each `containers.services.<role>` may specify `sourceContainer` and `volumeMode`.
- `volumeMode` accepts `copy` or `readonly` for ordinary services.
- Database configuration is supported only for the `db` role and requires `volumeMode: copy`.
- Database `system` currently supports `mysql`.
- Database `copyMode` accepts `schema` or `data`; `data` is reserved and not implemented in the current behavior.
- Database `initFiles` must stay within the project root and use relative paths.

Example database service:

```yaml
containers:
  network: isolated
  services:
    db:
      sourceContainer: my-project-db-1
      volumeMode: copy
      database:
        system: mysql
        copyMode: schema
        initFiles:
          - docker/mysql/init.sql
```

## Runtime State

- `PARACELL_ROOT` points commands back to the managed project root.
- `PARACELL_CELL` identifies the current cell inside its tmux session.
- `.paracell/state.json` is Paracell-managed state.
- `.paracell/cells/<cell>/source` is the cell worktree.
- Root session names use `<project>-root`; cell sessions use `<project>-<cell>`.

## Source-of-Truth Checks

When this reference may be stale, inspect these files in the Paracell repository:

- `internal/app/cli.go` for commands and environment behavior.
- `internal/domain/template.go` for template fields and allowed values.
- `internal/adapter/config/yaml_config.go` for YAML mapping and validation.
- `internal/usecase/init_project.go` for generated defaults.
- `README.md` for user-visible workflow.
