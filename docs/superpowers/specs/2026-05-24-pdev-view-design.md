# pdev view Design

## Goal

Add a new interactive command, `pdev view`, that shows the current cell list in
an interactive terminal UI. The first version only needs vertical navigation:

- `j` moves the selection down
- `k` moves the selection up
- `q` exits the view

The command name is `view`, and the primary domain term shown to the user is
`cell`.

## Scope

This feature adds a new view command and a minimal TUI for browsing cells.
It does not change the behavior of `pdev ls`, `pdev create`, or `pdev remove`.

The first implementation only needs:

- loading cells from `.pdev/state.json`
- rendering the list of cells
- moving the selection with `j` and `k`
- exiting with `q`

It does not need to open a selected cell yet. The `l` action is out of scope
for this step.

## Architecture

`internal/app` parses a new `view` command and routes it to a new usecase.

The usecase loads cell state through the existing state port and returns the
cells to the adapter layer. The adapter layer owns the terminal UI and runs the
Bubble Tea program.

Suggested structure:

- `internal/usecase`
  - add a `ViewCellsUseCase` that loads and returns cells
- `internal/app`
  - add `view` command parsing and wiring
- `internal/adapter/view`
  - add the Bubble Tea model, update loop, and renderer

The model can keep only the state needed for the first version:

- the full list of cells
- the current selected index

## Behavior

`pdev view` should:

- render the cell list in the terminal
- highlight the currently selected row
- move selection down on `j`
- move selection up on `k`
- quit on `q`

Selection should clamp at the ends of the list. Pressing `j` on the last row or
`k` on the first row should keep the selection in place.

If there are no cells, the view should still render cleanly and allow `q` to
exit.

## Data Flow

1. `pdev view` starts.
2. `internal/app` loads the config/state wiring it already owns.
3. `ViewCellsUseCase` loads cells from the state adapter.
4. `internal/adapter/view` receives the cells and starts the Bubble Tea program.
5. The program renders the list and processes `j`, `k`, and `q`.

## Error Handling

The command should fail if:

- the state file cannot be loaded
- the terminal UI cannot start

The first version does not need special handling for an empty list beyond
rendering an empty state.

## Testing

Add tests for:

- `view` command parsing
- `ViewCellsUseCase` returning the cell list from state
- the Bubble Tea model moving selection down and up correctly
- the model clamping at the list boundaries
- `q` exiting the program
- `pdev view` wiring in `internal/app`

Keep the existing `pdev ls` behavior unchanged.
