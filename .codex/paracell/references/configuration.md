# Paracell Configuration Reference

Read this reference when selecting a template, creating or editing `paracell.yaml`, choosing branch behavior, or diagnosing a dispatch failure.

## Commands

| Command | Purpose | Important constraint |
| --- | --- | --- |
| `paracell` | Enter the project root tmux session | Run from the project or with `PARACELL_ROOT` set |
| `paracell init` | Create `paracell.yaml` | Fails when the file already exists |
| `paracell fork <issue> --template <name> [--command <text>]` | Create and start a cell | For Skill dispatch, `<issue>` is the numeric GitHub issue number |
| `paracell retry <cell>` | Resume a failed cell by ID, Issue, or Name | Re-renders the latest template and skips completed creation stages |
| `paracell view` | Open the cell/template TUI | Interactive |
| `paracell ls` | List cells and status | Use before dispatch to avoid duplicates |
| `paracell pending` | Set the current cell to pending | Requires `PARACELL_CELL` |
| `paracell ready` | Set the current cell to ready and optionally notify | Requires `PARACELL_CELL` |
| `paracell clean <cell> [--force]` | Remove managed cell resources | Potentially destructive; current source requires a done cell |
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
          command: 'codex "{{.Command}}"'
  feat:
    extends: base
    repository:
      branchPrefix: feat/
```

`providers.container` is optional. Omit it when no Docker-backed service is needed. Supported providers are currently `git` for source, `tmux` for sessions and notifications, and `docker` for containers.

## Template Selection Fields

- Template key: conveys intended task type but does not override field-level compatibility.
- `extends`: names one parent template. A parent may itself extend one parent; multiple inheritance is not supported.
- `abstract: true`: marks a reusable template that is excluded from both `fork --template` and the TUI template list.
- `repository.branchPrefix`: prefixes the issue or task identifier to form the branch name.
- `repository.base`: accepts an explicit branch or `current`.
- `repository.branchMode`:
  - Omitted or `create`: require a new branch.
  - `reuse`: reuse an existing branch or create it when absent.
  - `require`: require an existing branch.
- `files`: project-root-relative local inputs copied into the cell source.
- `containers.network`: accepts `isolated` or `shared`.
- `containers.services`: declares container copies required by the cell.
- `session.windows`: declares tmux windows and startup commands. At least one command must use `{{.Command}}` for prompt dispatch through `fork --command`.

Use `reuse` for resumable work and `require` for review or recovery flows where creating a new branch would be wrong. Prefer a template without containers when the task has no container dependency.

## Template Inheritance

Inheritance is resolved before template variables are rendered and before concrete-template validation:

1. An omitted scalar or struct field inherits the resolved parent value.
2. A scalar explicitly supplied by the child replaces the parent value, including an empty string.
3. Structs such as `repository`, `containers`, and `session` are resolved field by field.
4. A supplied slice or map replaces the entire parent collection. Values are never appended or deep-merged.
5. `[]` and `{}` explicitly replace an inherited collection with an empty collection.
6. Object/pointer fields replace the complete object unless the field belongs to a structure explicitly resolved field by field above.

For example, this child keeps `repository.base` but replaces both inherited collections rather than combining them:

```yaml
templates:
  base:
    abstract: true
    repository:
      base: origin/main
      branchMode: create
    files: [.env, config/base.yaml]
    containers:
      services:
        web:
          sourceContainer: app-web
  feat:
    extends: base
    repository:
      branchPrefix: feat/
    files: [config/feat.yaml]
    containers:
      services: {}
```

Here `feat.files` contains only `config/feat.yaml`, and `feat.containers.services` is empty. An abstract template may be partial because only the fully resolved concrete template is validated. Variables inherited in environment values and session commands are rendered with the selected child's runtime values.

Loading fails deterministically when a parent does not exist or inheritance is cyclic. Errors identify the child and missing parent (`template "feat" extends unknown template "base"`) or include the complete cycle (`template inheritance cycle: "a" -> "b" -> "a"`).

## Issue-Backed Dispatch

The Paracell Skill stores the complete work package in a GitHub issue before dispatch. The CLI itself accepts the issue identifier but does not create or fetch the GitHub issue.

- Create a new issue body with `gh issue create --body-file <path>`; do not pass a long body as a shell argument.
- Pass the numeric issue number as the positional `fork` argument.
- Keep `--command` short: tell the worker to read the issue and treat it as the single source of truth.
- If issue creation succeeds but `fork` fails, retain the issue and run `paracell retry <cell>` after fixing the cause. A normal `fork` with the same Issue or Name remains a duplicate.
- A compatible session window must deliver either `{{.issue}}` or the short `{{.Command}}` instruction to the worker.

## Template Variables

Session window commands support:

- `{{.issue}}`: the argument supplied to `fork`; the Skill uses a numeric GitHub issue number.
- `{{.name}}`: the resulting cell name.
- `{{.Command}}`: the value supplied through `fork --command`; the Skill uses only a short instruction to read `{{.issue}}`. It is empty when omitted or when forked through the TUI.

The template is rendered before the shell starts. Keep YAML, Go-template, and shell quoting distinct.

## Container Details

- Each `containers.services.<role>` may specify `sourceContainer` and `volumeMode`.
- `volumeMode` accepts `copy` or `readonly` for ordinary services.
- Database configuration is supported only for the `db` role and requires `volumeMode: copy`.
- Database `system` currently supports `mysql`.
- Database `copyMode` accepts `schema` or `data`; `data` is reserved and not implemented in the current behavior.
- Database `initFiles` must be project-root-relative and remain within the project root.

## Runtime State

- `PARACELL_ROOT` points commands to the managed project root.
- `PARACELL_CELL` identifies the current cell inside its tmux session.
- `.paracell/state.db` is Paracell-managed SQLite state. It stores creation status (`creating`, `failed`, or `ready`), stage checkpoints, the failed stage, and the latest error in addition to the existing work status and done flag.
- `.paracell/cells/<cell>/source` is the cell worktree.
- Root session names use `<project>-root`; cell sessions use `<project>-<cell>`.

Creation stages run in `source`, `files`, `containers`, `session` order. A failed cell keeps completed resources, including its Git branch and worktree. Retry uses the latest template for the failed and unstarted stages without replacing saved identifiers or completed resources. If a retried file already exists with different content in the worktree, Paracell refuses to overwrite it.

## Source-of-Truth Checks

When this reference may be stale, inspect:

- `internal/app/cli.go` for commands and environment behavior.
- `internal/domain/template.go` for template fields and allowed values.
- `internal/adapter/config/yaml_config.go` for YAML mapping and validation.
- `internal/usecase/init_project.go` for generated defaults.
- `README.md` for the user-visible workflow.
