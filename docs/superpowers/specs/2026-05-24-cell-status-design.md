# Cell Status Design

## Goal

Add a cell completion flag to the domain and use it to control when a cell can
be deleted. New cells should start as not done, and `pdev view` should allow a
manual transition to done.

## Scope

This change adds a new domain concept and wires it through the existing create,
view, and remove flows.

The supported states are:

- not done
- done

## Architecture

Store the completion flag on `domain.Cell` and keep the transition rules in the
domain package. The usecases should call the domain rules instead of duplicating
the checks.

Suggested domain helpers:

- `Cell.done bool` as an unexported field
- `Cell.MarkDone() error`
- `Cell.IsDone() bool`
- `Cell.CanDelete() bool`

`CreateCellUseCase` should save new cells with the done flag unset.
`RemoveCellUseCase` should reject deletion unless `Cell.CanDelete()` is true.

`pdev view` should support pressing `l` on the selected cell to mark it as
`done`. That action should update the stored state and keep the view open.

## Behavior

### Creation

When `pdev create` finishes successfully:

1. the new cell is created with the done flag unset
2. the cell is saved to state

### View

In `pdev view`:

- `j` moves down
- `k` moves up
- `Enter` enters the selected cell
- `d` deletes the selected cell using the existing delete flow
- `l` marks the selected cell as `done`
- `q` exits the view

The `l` action should update the selected cell in state and refresh the list in
the view. It should not exit the view.

### Deletion

Cells may only be deleted when their status is `done`.

If a delete is attempted on a cell that is not done, the operation should
fail with a clear domain error and the view should keep showing the error.

## Data Flow

1. `CreateCellUseCase` builds a new cell.
2. The cell starts as not done.
4. `pdev view` loads cells from state.
5. `l` marks the selected cell as done and persists the updated state.
6. `d` deletes a cell only when it is done.

## Error Handling

The domain should reject invalid status transitions.

Expected errors:

- trying to delete a cell that is not done
- trying to mark an already done cell done again

## Testing

Add tests for:

- `domain.Cell` default done flag on creation
- `domain.Cell.MarkDone()` and `domain.Cell.IsDone()`
- `domain.Cell.CanDelete()`
- `CreateCellUseCase` saving a cell as not done
- `RemoveCellUseCase` rejecting not-done cells
- `pdev view` handling `l` and updating the selected cell status
- `pdev view` preserving the view after `l`
- `go test ./...` still passing
