# pdev ls Design

## Goal

Add a `pdev ls` command that lists virtual environments created by `pdev`.

The MVP should be small and reliable: it reads the existing `.pdev/state.json`
cell state and prints only the information the user needs now.

## Command

```sh
pdev ls
```

No flags are supported in this version.

## Output

`pdev ls` prints a two-column table to standard output:

```text
NAME    TEMPLATE
123     default
```

The columns are:

- `NAME`: `domain.Cell.Name`
- `TEMPLATE`: `domain.Cell.Template`

If there are no cells, the command succeeds and prints only the header:

```text
NAME    TEMPLATE
```

## Data Source

The command uses `.pdev/state.json` through the existing `CellStatePort`.

If `.pdev/state.json` does not exist, the existing JSON state adapter already
returns an empty cell list. `pdev ls` treats that as a valid empty list.

The command does not inspect Docker containers, git worktrees, or tmux sessions.
Those checks are outside the scope of this MVP and would make a read-only list
command depend on external services.

## Architecture

Add `ListCellsUseCase` in `internal/usecase`.

Responsibilities:

- Load cells through `CellStatePort.LoadCells`.
- Return the loaded `[]domain.Cell`.
- Leave formatting to the CLI layer.

Update `internal/app`:

- Add `CommandList` with command name `ls`.
- Parse `pdev ls`.
- Wire `ListCellsUseCase` to the existing JSON state adapter.
- Print the `NAME` and `TEMPLATE` table to standard output.

## Error Handling

- Invalid usage, such as extra arguments, returns `usage: pdev ls`.
- State loading errors are returned normally and printed by `cmd/pdev/main.go`.
- Missing state file is not an error.

## Testing

Add focused tests:

- CLI parsing accepts `pdev ls`.
- CLI parsing rejects `pdev ls` with extra arguments.
- `ListCellsUseCase` returns cells from the state port.
- `pdev ls` prints `NAME` and `TEMPLATE` for stored cells.
- `pdev ls` prints only the header for an empty state.

Implementation should follow the existing test-first pattern used by the repo.
