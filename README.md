# paracell

`paracell` is a Go CLI for creating isolated per-issue development cells from a project repo.

Each cell is made from:

- a git worktree under `.paracell/cells/<cell>/source`
- container processes based on the configured template
- a detached tmux session

## Commands

```text
paracell init
paracell fork <issue> --template <template>
paracell ls
paracell view
paracell clean <cell>
paracell clean <cell> --force
paracell version
paracell --version
```

## Files

- `paracell.yaml`: project configuration and templates
- `.paracell/state.json`: created cell state
- `.paracell/`: runtime data for the project

## Example Config

```yaml
project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    files:
      - .env
    containers:
      services:
        web:
          sourceContainer: myapp-web
          volumeMode: copy
        db:
          sourceContainer: myapp-db
          database:
            system: mysql
            copyMode: schema
            initFiles:
              - docker/mysql/init/001-users.sql
    session:
      windows:
        - name: editor
          command: nvim {{.issue}}
```

## Template Variables

Template commands can use:

- `{{.issue}}`
- `{{.name}}`

## Notes

- `paracell init` generates a default `paracell.yaml`.
- `paracell ls` reads the stored state and does not require `paracell.yaml`.
- `paracell fork` copies configured files into the cell source before starting containers.
- `volumeMode: readonly` keeps the current shared read-only volume behavior for non-database services.
- `volumeMode: copy` clones named Docker volumes for non-database services.
- `database.copyMode: data` is reserved and is not implemented yet.
- `paracell version` and `paracell --version` show release metadata injected at build time.

## Release
- Homebrew distribution is published to the `hgsg11/homebrew-paracell` tap as a cask.
- Install with `brew install --cask hgsg11/homebrew-paracell/paracell`.
