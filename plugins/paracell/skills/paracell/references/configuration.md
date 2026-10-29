# Paracell Configuration Reference

Read this reference when selecting a template, creating or editing `paracell.yaml`, choosing branch behavior, or diagnosing a dispatch failure.

## Commands

| Command | Purpose | Important constraint |
| --- | --- | --- |
| `paracell` | Enter the project root tmux session | Run from the project or with `PARACELL_ROOT` set |
| `paracell init` | Create `paracell.yaml`, initialize `.paracell/state.db`, and add the Claude Code status hooks to `.claude/settings.json` | Keeps an existing configuration unchanged; the hooks are added even when `paracell.yaml` exists |
| `paracell fork <ticket-id> --template <name> [--prefix <name>] [--command <text>] [--note <note>]` | Create and start a cell | Options may appear in any order; note is display-only and 1-20 Unicode characters after normalization |
| `paracell annotate <cell> --note <note>` | Set or replace a cell note | Resolve `<cell>` by ID, issue, or name; there is no clear operation |
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
  workspace: tmux
  notifications: tmux
prefixes:
  feat: feat/
  fix: fix/
templates:
  feat:
    commanderCell:
      name: workspace
      workspace:
        windows:
          - name: agent
            command: 'claude "{{.Command}}"'
    targets:
      repository:
        source:
          path: .
          base: main
        containers:
          app:
            environment:
              APP_ENV: "{{.Project}}-{{.Name}}"
            files:
              /app/config/local.yml: config/local.yml
    dependencies:
      - db
```

Unknown keys are rejected while loading. A template accepts only `commanderCell`, `targets`, and `dependencies`; `commanderCell` accepts `name` and `workspace` (required); a target accepts `source` and `containers`.

`providers.source` is required and supports only `git`. `providers.workspace` supports only `tmux`. `providers.container` is `none` (default when omitted) or `docker`. `providers.notifications` is `tmux`, `terminal-notifier` (macOS desktop only), or `none` / omission to disable. The Homebrew Cask depends on the `terminal-notifier` Formula. Ready notifications do not provide a click action to enter a cell or workspace; execution failures are returned as notification errors.

A fresh `paracell init` creates one starter template named `self` for the `repository` target (it duplicates this repository as one worktree) and an `agent` window running `claude "{{.Command}}"`, so `fork --command <text>` starts claude with that instruction. Its default branch-prefix keys are `feat`, `update`, and `fix`; they apply to every project that does not set `prefixes`, and a prefix written in `paracell.yaml` takes precedence. Rename or add templates to represent the targets used by the project; prefix keys are not template names unless the project independently chooses matching names.

### Templates Without Targets

A template may omit `targets` (and `dependencies`) and define only `commanderCell`, for work such as requirements definition that only reads code and writes tickets or documents:

```yaml
templates:
  requirements:
    commanderCell:
      name: workspace
      workspace:
        windows:
          - name: claude
            command: claude "{{.Command}}"
```

- Create it with `paracell fork <ticket-id> --template requirements`; `clean`, `pending` / `ready`, and the TUI work as for any other cell.
- No worktree, branch, container, or cell directory is created. Only the tmux session is created, and its windows start in the project root (the directory containing `paracell.yaml`).
- The same applies to any template without a source-backed target (for example, one whose targets only have `containers`).
- Cells with at least one source-backed target start their windows in `.paracell/cells/<cell>/` as before.
- Caution: the project root is shared with other cells and with local work, so file edits made in this kind of cell can overwrite each other.

## Claude Code Status Hooks

`paracell init` adds two hooks to `.claude/settings.json` in the same directory as `paracell.yaml`, whether or not `paracell.yaml` already exists. Claude inside a cell then sets the cell to `pending` when it starts on a prompt and to `ready` when it finishes its response.

| Claude Code event | Command |
| --- | --- |
| `UserPromptSubmit` | `[ -z "$PARACELL_CELL" ] \|\| paracell pending` |
| `Stop` | `[ -z "$PARACELL_CELL" ] \|\| paracell ready` |

- Existing content (`permissions`, `env`, other events and hooks) and key order are kept; each hook is appended to its event's array, and a rewrite normalizes the indent to 2 spaces.
- A hook whose command is already present for that event is not added again. Rerunning `paracell init` does not rewrite the file.
- Invalid JSON, or a `hooks` or event value of an unexpected type, fails with an error and leaves the file unchanged.
- Outside a cell (`PARACELL_CELL` empty) the hooks do nothing and exit 0.
- Only the project's `.claude/settings.json` is written; `~/.claude` and `.claude/settings.local.json` are not touched, and hooks are never removed or updated.

## Template Selection Fields

- `prefixes`: optional map from a key to a branch-name prefix. Keys and values control branch naming only; they do not select templates. Pass `--prefix <key>` when the configured prefix key for the intended branch naming is known and should be used, regardless of which template was selected. Omit `--prefix` only when the CLI's default `feat` branch prefix is intended. Never derive the prefix from a template key.
- Template key: identifies a configured template. To select by target, match explicitly requested target names against the names under each compatible template's `targets` map by exact equality; do not compare target names to template keys. With one requested target, a candidate must contain that target entry. With several, a candidate must contain all of them. Choose the lexicographically smallest matching template key (case-sensitive byte order); if none match or no target is named, choose the lexicographically smallest compatible template key overall. Do not infer a match from a task kind, prefix, semantic closeness, or YAML declaration order.
- Templates do not inherit. `extends` and `abstract` were removed; a `paracell.yaml` containing either fails to load with an error naming the template and field. Migrate by writing the parent's definition directly into each template and deleting the parent template.
- `targets.<name>.source.path`: the repository to duplicate, relative to the project root (the directory containing `paracell.yaml`); defaults to `.`. Absolute paths and paths leaving the project root are rejected.
- `targets.<name>.source.base`: the branch the new worktree starts from. Omit it or use `current` to start from the repository's current `HEAD`.
- Every target source gets a new branch named `<prefix><ticket-id>` (for example `feat/123`) through `git worktree add -b`. An existing branch with that name makes creation fail; there is no branch reuse mode.
- `targets.<name>.containers.<container>`: copies the running source container named `<container>` into the cell. `environment` adds environment variables (values are rendered with the template variables). `files` maps an absolute path inside the container to a path relative to that target's worktree and bind-mounts it.
- A target needs a `source`, `containers`, or both. Its containers are always copies (mode `target`).
- `dependencies`: names of running containers that the cell shares instead of copying. Each one is attached to the cell network with its existing aliases.
- `commanderCell.workspace.windows`: tmux windows and their startup commands. At least one command must use `{{.Command}}` for prompt dispatch through `fork --command`.

The issue's example configuration has prefix keys `feat` and `fix`, and template keys `feat`, `fix`, `review`, and `update`. These are independent namespaces even though two keys happen to be identical. If all four compatible templates contain `targets.repository`, a request for target `repository` matches all four and selects template `feat` by key order. If only `templates.update` contains `targets.web`, a request for `web` selects `update`; the template key itself is not the target match. For branch naming, pass `--prefix fix` to use `fix/` regardless of the selected template, or omit `--prefix` only when the CLI's default `feat/` is intended.

Prefer a template without containers when the task has no container dependency.

## Ticket-Backed Dispatch

The Paracell dispatcher stores the complete work package in a development ticket before dispatch. The current CLI accepts the ticket identifier through its legacy issue-named argument but does not create or fetch the ticket.

- Create the ticket through the project's available provider integration; do not pass a long body as a shell argument. For GitHub Issues, `gh issue create --body-file <path>` is one available method.
- Pass the provider's ticket identifier as a string to the positional `fork` argument; it need not be numeric.
- Keep `--command` short: tell the worker to read the ticket and treat it as the single source of truth. Inside the cell, the worker implements it with the `paracell-implement` skill.
- Pass `--note` on every Skill dispatch. Derive it from the ticket title and body when available, or from the confirmed work objective otherwise.
- Keep the note natural and concise: 1-20 Unicode characters after whitespace normalization, without padding or detailed requirements. It is display-only, not a dispatch identifier or search key.
- Use `paracell fork <ticket-id> --template <template> [--prefix <prefix>] --note <note> --command <short-ticket-instruction>` with each argument passed separately. Omit `--prefix` to use `feat`.
- If ticket creation succeeds but `fork` fails, retain the ticket and report the failed cell. A normal `fork` with the same ticket identifier or name remains a duplicate.
- A compatible workspace window must deliver either `{{.Issue}}` or the short `{{.Command}}` instruction to the worker.

## Template Variables

Workspace window commands and container `environment` values are Go templates rendered before the shell starts. A misspelled variable is an error. The variable names are case-sensitive:

- `{{.Issue}}`: the string ticket identifier supplied to `fork`; the variable name is retained for CLI/template compatibility.
- `{{.Name}}`: the resulting cell name (the ticket identifier made safe for resource names).
- `{{.Project}}`: `project.name` from `paracell.yaml`.
- `{{.Command}}`: the value supplied through `fork --command`; the dispatcher uses a short instruction to read the ticket identified by `{{.Issue}}`. It is empty when omitted or when forked through the TUI.

Keep YAML, Go-template, and shell quoting distinct.

## Container Details

- Every cell with containers gets its own Docker network named `paracell-<project>-<cell>`. Copied containers join only that network, preserving usable aliases from their source container plus their container name, and the shared gateway joins the network for HTTP routes.
- Published TCP container ports follow the source container's bindings and use Docker-assigned random host ports. Unpublished source ports remain unpublished.
- Mounts of the source container are carried over as follows:
  - Named volumes are always copied into a new cell-specific volume (`docker volume create` and `cp -a`); a read-only mount stays read-only. There is no shared or read-only volume mode.
  - A bind mount whose host path is inside the project root is replaced with the same relative path inside the target's worktree. For a container created by docker compose, only binds declared in the compose service are kept, and a bind outside the project root keeps the original host path (shared with the source). For other containers, a bind outside the project root is dropped.
  - Other mount types, such as tmpfs, are dropped.
- Dependency containers are not copied: the running container is attached to the cell network, and rollback and `clean` disconnect it only from that network.
- There is no database-specific configuration. A database container is copied like any other container, including its named volumes.

## Runtime State

- CellGroup is the aggregate root: it holds the shared issue, project, template, note, resource drivers, and creation state, and is stored on its own. CommanderCell refers to it by CellGroupID and retains the workspace commands and execution state (status, done); TargetCells / DependencyCells record membership with CellGroupID. Existing template syntax is unchanged.
- A CellGroup ID, issue, or cell name addresses the cell for note updates, status changes, or cleanup. SQLite records in the two earlier formats (CellGroup nested in the CommanderCell, and the Commander-owned format before it) are read into the new model and rewritten transactionally on their next change, preserving existing resources and execution state.

- `PARACELL_ROOT` points commands to the managed project root.
- `PARACELL_CELL` identifies the current cell inside its tmux session. When non-empty, perform development work directly in that cell instead of automatically dispatching again; the task issue need not match the cell identifier.
- `.paracell/state.db` is Paracell-managed SQLite state. `paracell init` creates the relational schema. Legacy JSON-backed state is left unchanged.
- `.paracell/cells/<cell>/` is the cell's workspace directory; tmux windows start there. Each target's worktree is `.paracell/cells/<cell>/<target>/source` (with `source.path` appended when it is not `.`).
- Root session names use `<project>-root`; cell sessions use `<project>-<cell>`.
- CLI lists and tmux labels show the note when set, otherwise the cell name. The TUI shows `<cell name> | <note>`. Always use the CellGroup ID, issue, or name—not the note—to address a cell.

Creation stages run in `source`, `containers`, `workspace` order. A failed cell keeps its failure stage and latest error for inspection.

## Source-of-Truth Checks

When this reference may be stale, inspect:

- `internal/app/cli.go` for commands and environment behavior.
- `internal/domain/template.go` and `internal/domain/template_vars.go` for template fields and variables.
- `internal/adapter/container/docker.go` for container copies and mounts.
- `internal/adapter/config/yaml_config.go` for YAML mapping and validation.
- `internal/usecase/init_project.go` for generated defaults.
- `internal/adapter/agent/claude_settings.go` for the Claude Code hooks `paracell init` adds.
- `README.md` for the user-visible workflow.
