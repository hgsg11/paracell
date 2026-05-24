# paracell

`paracell` is a Go CLI for creating isolated per-issue development cells from a project repo.

Each cell is made from:

- a git worktree under `.pdev/cells/<cell>/source`
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

- `.pdev.yml`: project configuration and templates
- `.pdev/state.json`: created cell state
- `.pdev/`: runtime data for the project

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

- `paracell init` generates a default `.pdev.yml`.
- `paracell ls` reads the stored state and does not require `.pdev.yml`.
- `paracell create` copies configured files into the cell source before starting containers.
