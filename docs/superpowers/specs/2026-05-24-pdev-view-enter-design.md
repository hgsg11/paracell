# pdev view enter Design

## Goal

Extend `pdev view` so that pressing `Enter` on a selected cell exits the view
and enters that cell by attaching to its tmux session.

The first version only needs the `Enter` path. Returning to the view after
detaching is out of scope.

## Scope

This feature builds on `pdev view` and adds one new action:

- `Enter` on a selected cell

The existing `j`, `k`, and `q` behavior stays the same.

The `Enter` action should:

- exit the view
- return the selected cell to `internal/app`
- run the tmux attach operation for that cell

`pdev ls`, `pdev create`, and `pdev remove` do not change.

## Architecture

Keep selection and rendering in `internal/adapter/view`. The Bubble Tea model
should stop on `Enter` and return the selected `domain.Cell` to the caller.

`internal/app` coordinates the next step after the view exits:

1. load cells with `ViewCellsUseCase`
2. run the interactive view
3. if the user pressed `Enter`, call a new usecase that enters the selected cell

The new enter usecase owns the tmux attach action. It uses the session port,
and the adapter layer implements that port with the existing tmux runner.

Suggested new pieces:

- `internal/usecase/EnterCellUseCase`
- `SessionPort.EnterSession(ctx, cell domain.Cell) error`
- `internal/adapter/session.TmuxAdapter.EnterSession(...)`
- a small result type from the view adapter so `internal/app` can tell whether
  the user pressed `Enter` or `q`

## Behavior

`pdev view` should now support:

- `j` to move down
- `k` to move up
- `q` to exit without entering a cell
- `Enter` to exit and enter the selected cell

The selected cell should be preserved when the view exits so `internal/app`
can use it for the enter step.

If the selected cell is the first or last row, selection should still clamp at
the list boundaries.

## Data Flow

1. `pdev view` loads cells through `ViewCellsUseCase`.
2. The Bubble Tea model renders the list and tracks the selected index.
3. On `Enter`, the model exits and returns the selected cell.
4. `internal/app` receives the result and calls `EnterCellUseCase`.
5. `EnterCellUseCase` calls the session port, which attaches to tmux.

## Error Handling

The command should fail if:

- the state file cannot be loaded
- the view cannot run
- tmux attach fails

Pressing `q` should exit without calling the enter usecase.

## Testing

Add tests for:

- the view model returning the selected cell on `Enter`
- the view adapter returning the selected cell result to the caller
- `SessionPort` and `TmuxAdapter` supporting enter/attach behavior
- `EnterCellUseCase` invoking the session port with the selected cell
- `internal/app` calling the enter usecase after the view returns an entered cell

Keep the existing `j`, `k`, and `q` tests in place.
