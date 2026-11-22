# Paracell Configuration Reference

Read this reference when selecting a template, creating or editing `paracell.yaml`, choosing branch behavior, or diagnosing a dispatch failure.

## Commands

| Command | Purpose | Important constraint |
| --- | --- | --- |
| `paracell` | Enter the project root tmux session | Run from the project or with `PARACELL_ROOT` set |
| `paracell init` | Create `paracell.yaml`, initialize `.paracell/state.db`, and add the Claude Code status hooks and the `Bash(paracell review:*)` / `Bash(paracell implement:*)` permission rules to `.claude/settings.json` | Keeps an existing configuration unchanged; the hooks and rules are added even when `paracell.yaml` exists, and only when missing |
| `paracell fork <ticket-id> --template <name> [--prefix <name>] [--command <text>] [--note <note>]` | Create and start a cell | Options may appear in any order; note is display-only and 1-20 Unicode characters after normalization |
| `paracell queue <ticket-id> --template <name> [--prefix <name>] [--command <text>] [--note <note>] [--modify <path>]... [--create <path>]...` | Declare the files the cell will change, queue it, and start it at once unless it overlaps an unfinished queued cell | At least one `--modify` (existing file, deletions included) or `--create` (new file); workspace-relative file paths only, no absolute path, `..`, trailing `/`, or glob; prints `started <id>` or `waiting <id> for <ids>` |
| `paracell planned [prefix]` | List the paths declared by unfinished queue entries (`KIND`, `PATH`, `ISSUE`, `NOTE`) | `prefix` filters by path prefix; read-only |
| `paracell dequeue <ticket-id>` | Remove a waiting queue entry and start the entries it released | Fails for a running entry; use `merged` / `abandon` instead |
| `paracell annotate <cell> --note <note>` | Set or replace a cell note | Resolve `<cell>` by ID, issue, or name; there is no clear operation |
| `paracell view` | Open the cell/template TUI | Interactive |
| `paracell ls` | List cells, status, and the PR numbers of cells in review, then warnings for cells whose PRs are partly merged and partly closed, then the waiting queue entries and whom they wait for | Use before dispatch to avoid duplicates; with `providers.review` it asks the provider about cells in review |
| `paracell pending` | Set the current cell to pending | Requires `PARACELL_CELL` |
| `paracell ready` | Set the current cell to ready and optionally notify | Requires `PARACELL_CELL` |
| `paracell review [--title <title> --body-file <file>]` | With `providers.review`: push, open one PR per target with commits (first call only, with `--title` / `--body-file`), move the current cell to in review, and wait for reviews, comments, merge, or close. Without it: move the current cell from in progress to in review | Requires `PARACELL_CELL`; fails on uncommitted changes; `--title` / `--body-file` are required the first time and rejected later; run it in the background |
| `paracell implement` | Move the current cell from in review back to in progress | Requires `PARACELL_CELL`; run when going back to address review |
| `paracell merge <cell> [--squash\|--rebase]` | Merge the cell's PRs that are not merged yet on the provider, then mark it merged, remove its review, and start the released queue entries | Works outside a cell; requires a cell with an open review; a failed merge changes nothing; default is a merge commit |
| `paracell merged <cell>` | Mark a cell merged without merging anything; remove its review; for a queued cell, remove its queue entry and start the released entries | Works outside a cell; cannot be undone |
| `paracell abandon <cell>` | Mark a cell abandoned; remove its review; for a queued cell, remove its queue entry and start the released entries | Works outside a cell; cannot be undone |
| `paracell clean <cell> [--force]` | Remove managed cell resources | Potentially destructive; requires a merged or abandoned cell |
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
  review: github
prefixes:
  feat: feat/
  fix: fix/
workspace:
  copy:
    - .claude/
    - .env.local
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

Unknown keys are rejected while loading. A template accepts only `commanderCell`, `targets`, and `dependencies`; `commanderCell` accepts `name` and `workspace` (required); a target accepts `source` and `containers`; the top-level `workspace` accepts only `copy`.

`providers.source` is required and supports only `git`. `providers.workspace` supports only `tmux`. `providers.container` is `none` (default when omitted) or `docker`. `providers.notifications` is `tmux`, `terminal-notifier` (macOS desktop only), or `none` / omission to disable. The Homebrew Cask depends on the `terminal-notifier` Formula. Ready notifications do not provide a click action to enter a cell or workspace; execution failures are returned as notification errors. `providers.review` is `github` or `none` / omission. With `github`, Paracell pushes, opens and follows the PRs with the `gh` CLI (the user signs in to `gh` beforehand), and merges them; without it, `paracell review` only moves the cell to in review and a person marks merged.

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
- No worktree, branch, container, or cell directory is created, and `workspace.copy` is not applied. Only the tmux session is created, and its windows start in the project root (the directory containing `paracell.yaml`).
- The same applies to any template without a source-backed target (for example, one whose targets only have `containers`).
- Cells with at least one source-backed target start their windows in the cell directory `.paracell/cells/<cell>/`.
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
- `targets.<name>.source.path`: the repository to duplicate, relative to the project root (the directory containing `paracell.yaml`, the workspace); defaults to `.`. Absolute paths and paths leaving the project root are rejected. It must name the top level of a git repository; a subdirectory of a repository fails the source stage. Its worktree is created at the same relative path inside the cell directory (see Cell Directory Layout).
- `workspace.copy`: optional list of workspace-relative files or directories copied into the cell directory for every template. Nothing is copied by default. Absolute paths and paths leaving the workspace are rejected while loading. Copying happens in the source stage after every worktree exists; it copies real content (not symlinks) and overwrites anything at the same path, including files tracked by git. A listed path missing from the workspace fails the source stage.
- `targets.<name>.source.base`: the start point for a newly created cell branch, passed to `git worktree add` as written. `main` starts from the local `main`; `origin/main` starts from the freshly fetched `origin/main`. Omit it or use `current` to start from the repository's current `HEAD`. It is ignored when an existing branch is reused.
- Before creating a target worktree, Paracell runs `git fetch origin` when the repository has an `origin` remote; without `origin` it skips the fetch. A failed fetch fails the source stage.
- Every target source uses the branch `<prefix><ticket-id>` (for example `feat/123`). An existing local branch is reused as-is (no pull or rebase); otherwise an existing `origin/<branch>` gets a local tracking branch; otherwise a new branch is created from `base`. Reuse always applies; there is no branch mode setting.
- If the branch is already checked out elsewhere (the main working directory or another cell), `git worktree add` refuses it, the source stage fails, and the error appears in `paracell ls`. `paracell clean` removes the worktrees and the cell directory but keeps the branch.
- `targets.<name>.containers.<container>`: copies the running source container named `<container>` into the cell. `environment` adds environment variables (values are rendered with the template variables). `files` maps an absolute path inside the container to a path relative to that target's worktree and bind-mounts it.
- A target needs a `source`, `containers`, or both. Its containers are always copies (mode `target`).
- `dependencies`: names of running containers that the cell shares instead of copying. Each one is attached to the cell network with its existing aliases.
- Target and container names only have to be unique as YAML keys. Several targets may declare the same container, each with its own `environment` and `files`. Target names never appear in paths; they only qualify container names and gateway service roles. When names become equal after Paracell makes them safe for Docker (case, spaces, and symbols are ignored), Paracell keeps them apart by adding a short hash only to the colliding container names; names that do not collide are unchanged. Network aliases never get a prefix or hash.
- Targets with the same `source.path` share one worktree, because a cell has a single branch. No symlinks are created.
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
  - A bind mount whose host path is inside the project root is replaced with the same relative path inside the cell directory (`.paracell/cells/<cell>/<path relative to the project root>`), which mirrors the workspace. Without a cell directory (no source-backed target) the original host path is kept. For a container created by docker compose, only binds declared in the compose service are kept, and a bind outside the project root keeps the original host path (shared with the source). For other containers, a bind outside the project root is dropped.
  - Other mount types, such as tmpfs, are dropped.
  - A path listed in `files` replaces the source container's mount at the same path. Paths are compared after cleaning (`/data/` equals `/data`); two `files` entries for the same path must mount the same source.
- Dependency containers are not copied: the running container is attached to the cell network, and rollback and `clean` disconnect it only from that network.
- There is no database-specific configuration. A database container is copied like any other container, including its named volumes.

## Runtime State

- CellGroup is the aggregate root: it holds the shared issue, project, template, note, resource drivers, and creation state, and is stored on its own. CommanderCell refers to it by CellGroupID and retains the workspace commands, the agent status (`pending` / `ready`), and the implementation progress; TargetCells / DependencyCells record membership with CellGroupID. Existing template syntax is unchanged.
- Implementation progress is separate from the agent status: `in_progress` (the initial value; also while addressing review), `in_review` (PR open), `merged`, and `abandoned`. Only in_progress ⇄ in_review and in_progress / in_review → merged / abandoned are allowed; moving to the same progress fails, and merged and abandoned are terminal. `paracell ls` shows it in the `PROGRESS` column, and the TUI marks a cell merged with `m` `m` or abandoned with `a` `a`.
- A Review exists per CellGroup only while its PRs are in review: the first `paracell review` creates it with one PR reference (target cell and provider number) per target with commits, and it is removed in the same transaction that marks the cell merged or abandoned. PR states are not stored; each poll asks the provider. The Review keeps a handled time: a waiting `paracell review` reports only the reviews and comments created after it and advances it to the latest one it reported, so nothing is delivered twice and anything that arrived meanwhile is returned at once by the next call. When every PR is merged it marks the cell merged; when every PR is closed it marks it abandoned; when every open PR is approved and none has changes requested it merges them (who approved is not checked); a mix of merged and closed PRs is left for a person. Targets that share a source path share one worktree and branch, so they get one PR. With a Review, the TUI's `m` `m` runs `paracell merge`, and the TUI shows the PR numbers and marks a cell needing a merged-or-abandoned decision with `!`.
- The queue is one per project and stored in `.paracell/state.db` with the cells. `queue` validates the declaration only at enqueue: a modify path must exist on disk or be declared create by an unfinished entry; a create path must exist in neither; every violation is reported in one error, and an unknown template or prefix, or an issue that already has a cell or entry, is rejected before queueing. Entries overlap when they share any declared path; a later entry waits for every earlier overlapping entry, waiting or running. A cell started from the queue records the entry as its CellGroup's queue entry; marking it merged or abandoned removes the entry in the same transaction and then forks the waiting entries that no longer overlap anything, in enqueue order. A failed start leaves the cell `failed` and its entry running; abandon it and queue again. Cells created by `fork` are never part of the queue judgment.
- Records saved with the former `done` flag are not converted. Clean existing cells before upgrading.
- A CellGroup ID, issue, or cell name addresses the cell for note updates, status changes, or cleanup. SQLite records in the two earlier formats (CellGroup nested in the CommanderCell, and the Commander-owned format before it) are read into the new model and rewritten transactionally on their next change, preserving existing resources and execution state.

- `PARACELL_ROOT` points commands to the managed project root.
- `PARACELL_CELL` identifies the current cell inside its tmux session. When non-empty, perform development work directly in that cell instead of automatically dispatching again; the task issue need not match the cell identifier.
- `.paracell/state.db` is Paracell-managed SQLite state. `paracell init` creates the relational schema. Legacy JSON-backed state is left unchanged.
- `.paracell/cells/<cell>/` is the cell directory, a mirror of the workspace; tmux windows start there. Each target's worktree is `.paracell/cells/<cell>/<source.path>`, so with `source.path: .` the cell directory itself is the worktree.
- Without `PARACELL_CELL` / `PARACELL_ROOT`, `pending` and `ready` find the cell and project root from the nearest ancestor `.paracell/cells/<cell>` of the working directory.
- Root session names use `<project>-root`; cell sessions use `<project>-<cell>`.
- CLI lists and tmux labels show the note when set, otherwise the cell name. The TUI shows `<cell name> | <note>`. Always use the CellGroup ID, issue, or name—not the note—to address a cell.

Creation stages run in `source`, `containers`, `workspace` order. A failed cell keeps its failure stage and latest error for inspection.

## Cell Directory Layout

```text
ws/                          .paracell/cells/<cell>/
  paracell.yaml
  .claude/            copy  →   .claude/
  frontend/  (repo)   worktree→ frontend/
  backend/   (repo)   worktree→ backend/
```

- Worktrees are created from the shallowest `source.path` to the deepest, so nested repositories (`.` and `services/api`) work; `clean` removes them deepest first.
- `clean` removes the whole cell directory, including copied files, only after every worktree was removed. If a worktree cannot be removed, the cell directory is kept to protect its work.
- Cells created by versions that used `.paracell/cells/<cell>/<target>/source` are not migrated. Clean them before upgrading.

## Source-of-Truth Checks

When this reference may be stale, inspect:

- `internal/app/cli.go` for commands and environment behavior.
- `internal/domain/template.go` and `internal/domain/template_vars.go` for template fields and variables.
- `internal/adapter/container/docker.go` for container copies and mounts.
- `internal/adapter/config/yaml_config.go` for YAML mapping and validation.
- `internal/usecase/init_project.go` for generated defaults.
- `internal/adapter/agent/claude_settings.go` for the Claude Code hooks `paracell init` adds.
- `README.md` for the user-visible workflow.
