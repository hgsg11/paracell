# paracell

`paracell` is a Go CLI for creating isolated per-issue development cells from a project repo.

Each cell is made from:

- a git worktree under `.paracell/cells/<cell>/source`
- container processes based on the configured template
- a detached tmux session

## Commands

```text
paracell init
paracell create <issue> --template <template>
paracell ls
paracell view
paracell remove <cell>
paracell remove <cell> --force
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
- `paracell create` copies configured files into the cell source before starting containers.

## Release

- GitHub Releases are generated with GoReleaser from the `v*` tag workflow in `.github/workflows/release.yml`.
- Local release dry-runs can use `goreleaser release --clean --snapshot --skip=publish`.
