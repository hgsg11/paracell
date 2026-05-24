# Cell Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a done flag to the domain so new cells start not done, cells can be marked done from `pdev view`, and only done cells can be deleted.

**Architecture:** Keep the done rules in `internal/domain` so the usecases only call domain methods and do not duplicate policy. `CreateCellUseCase` should save cells with the done flag unset, `RemoveCellUseCase` should reject non-done cells, and the Bubble Tea view should call a dedicated usecase to mark the selected cell done when `l` is pressed. This change touches `internal/domain`, `internal/usecase`, `internal/app`, and `internal/adapter/view`, but each file keeps one responsibility.

**Tech Stack:** Go 1.26, existing usecase ports, existing Bubble Tea view adapter.

---

### Task 1: Add the done flag to the domain

**Files:**
- Modify: `internal/domain/model.go:1-170`
- Modify: `internal/domain/cell_test.go`

- [ ] **Step 1: Write the failing test**

Add tests that define the new domain behavior:

```go
func TestCellの初期Doneはfalseにする(t *testing.T) {
	cell, err := NewCellFactory().NewCell("cell-1", "123", template, "myapp")
	if err != nil {
		t.Fatalf("Cell作成でエラーが返った: %v", err)
	}
	if cell.IsDone() {
		t.Fatal("IsDone = true, want false")
	}
}

func TestCellはMarkDoneできる(t *testing.T) {
	cell := Cell{}
	if err := cell.MarkDone(); err != nil {
		t.Fatalf("MarkDoneでエラーが返った: %v", err)
	}
	if !cell.IsDone() {
		t.Fatal("IsDone = false, want true")
	}
}

func TestDoneでないCellは削除不可(t *testing.T) {
	cell := Cell{}
	if cell.CanDelete() {
		t.Fatal("doneでないcell should not be deletable")
	}
}
```

Add the domain types and methods:

```go
type Cell struct {
	ID       string
	Issue    string
	Name     string
	Template string
	Branch   string
	Source   Source
	Containers Containers
	Session  Session
	done     bool
}

func (c *Cell) MarkDone() error {
	if c.done {
		return fmt.Errorf("cell is already done")
	}
	c.done = true
	return nil
}

func (c Cell) IsDone() bool {
	return c.done
}

func (c Cell) CanDelete() bool {
	return c.done
}
```

Initialize new cells with `done: false` in `CellFactory.NewCell`.

- [ ] **Step 2: Run the tests to confirm they fail**

Run:

```bash
go test ./internal/domain -run 'TestCellの初期Doneはfalseにする|TestCellはMarkDoneできる|TestDoneでないCellは削除不可' -v
```

Expected: fail because `done`, `MarkDone`, `IsDone`, and `CanDelete` do not exist yet.

- [ ] **Step 3: Implement minimal domain code**

Add the unexported `done bool` field, add `MarkDone`, `IsDone`, and `CanDelete`, and set the field to false in `NewCellFactory.NewCell`.

- [ ] **Step 4: Run the tests to confirm they pass**

Run:

```bash
go test ./internal/domain -v
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/model.go internal/domain/cell_test.go
git commit -m "Add cell done flag domain rules"
```

### Task 2: Wire create, remove, and mark-done usecases

**Files:**
- Modify: `internal/usecase/create_cell.go`
- Modify: `internal/usecase/remove_cell.go`
- Create: `internal/usecase/mark_cell_done.go`
- Create: `internal/usecase/mark_cell_done_test.go`
- Modify: `internal/usecase/create_remove_test.go`
- Modify: `internal/usecase/ports.go`

- [ ] **Step 1: Write the failing tests**

Update the create test assertion:

```go
if cell.IsDone() {
	t.Fatal("IsDone = true, want false")
}
```

Add a remove test that rejects not-done cells:

```go
func TestRemoveCellはDoneでないCellを削除しない(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "123"},
	}
	uc := RemoveCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
	}

	err := uc.Execute(ctx, RemoveCellInput{Cell: "123"})

	if err == nil {
		t.Fatal("doneでないcellなのに削除できてしまった")
	}
	if err.Error() != `cell "123" cannot be deleted until it is done` {
		t.Fatalf("error = %q, want %q", err.Error(), `cell "123" cannot be deleted until it is done`)
	}
}
```

Add a mark-done usecase test:

```go
func TestMarkCellDoneはStateのCellをDoneにして返す(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "123"},
	}

	uc := MarkCellDoneUseCase{State: ports}
	cell, err := uc.Execute(ctx, MarkCellDoneInput{Cell: "123"})
	if err != nil {
		t.Fatalf("MarkCellDoneでエラーが返った: %v", err)
	}
	if !cell.IsDone() {
		t.Fatal("IsDone = false, want true")
	}
	if !ports.cells[0].IsDone() {
		t.Fatal("stateのcellがdoneになっていない")
	}
}
```

Add the usecase shape:

```go
type MarkCellDoneInput struct {
	Cell string
}

type MarkCellDoneUseCase struct {
	State CellStatePort
}

func (u MarkCellDoneUseCase) Execute(ctx context.Context, input MarkCellDoneInput) (domain.Cell, error) {
	cells, err := u.State.LoadCells(ctx)
	if err != nil {
		return domain.Cell{}, err
	}
	for i, cell := range cells {
		if cell.ID == input.Cell || cell.Issue == input.Cell || cell.Name == input.Cell {
			if err := cell.MarkDone(); err != nil {
				return domain.Cell{}, err
			}
			cells[i] = cell
			if err := u.State.SaveCells(ctx, cells); err != nil {
				return domain.Cell{}, err
			}
			return cell, nil
		}
	}
	return domain.Cell{}, fmt.Errorf("cell %q not found", input.Cell)
}
```

Add `CanDelete()` checks to `RemoveCellUseCase` before calling any ports.

- [ ] **Step 2: Run the tests to confirm they fail**

Run:

```bash
go test ./internal/usecase -run 'TestCreateCellはCellを作成して外部リソースを順番に作る|TestRemoveCellはDoneでないCellを削除しない|TestMarkCellDoneはStateのCellをDoneにして返す' -v
```

Expected: fail because the done flag and mark-done usecase are not wired yet.

- [ ] **Step 3: Implement the usecase changes**

Set the new cell as not done in `CreateCellUseCase`, reject non-done cells in `RemoveCellUseCase`, and add `MarkCellDoneUseCase` that loads, marks done, saves, and returns the updated cell.

- [ ] **Step 4: Run the tests to confirm they pass**

Run:

```bash
go test ./internal/usecase -v
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/usecase/create_cell.go internal/usecase/remove_cell.go internal/usecase/mark_cell_done.go internal/usecase/mark_cell_done_test.go internal/usecase/create_remove_test.go internal/usecase/ports.go
git commit -m "Wire done flag through usecases"
```

### Task 3: Add `l` in view to mark a cell done

**Files:**
- Modify: `internal/adapter/view/model.go`
- Modify: `internal/adapter/view/run.go`
- Modify: `internal/adapter/view/model_test.go`
- Modify: `internal/adapter/view/run_test.go`
- Modify: `internal/app/cli.go`
- Modify: `internal/app/cli_test.go`

- [ ] **Step 1: Write the failing tests**

Add a view model test for `l`:

```go
func TestModelはlで選択中CellをDoneにする(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.MarkDone = func(cell domain.Cell) (domain.Cell, error) {
		if cell.Name != "123" {
			t.Fatalf("mark done cell = %#v, want name %q", cell, "123")
		}
		if err := cell.MarkDone(); err != nil {
			t.Fatalf("MarkDoneでエラーが返った: %v", err)
		}
		return cell, nil
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if cmd == nil {
		t.Fatal("lでコマンドが返らなかった")
	}
	updated, _ := next.(Model).Update(cmd())
	if !updated.(Model).Cells[0].IsDone() {
		t.Fatal("IsDone = false, want true")
	}
}
```

Add a run test that confirms `l` flows through the adapter and keeps the view open:

```go
func TestRunはlでDone結果を返す(t *testing.T) {
	original := newProgram
	defer func() { newProgram = original }()

	var got Model
	newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
		_ = opts
		got = model.(Model)
		return programFunc(func() (tea.Model, error) {
			updated, cmd := model.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
			if cmd != nil {
				updated, _ = updated.(Model).Update(cmd())
			}
			return updated, nil
		})
	}

	cells := []domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	}
	result, err := Run(
		context.Background(),
		cells,
		func(cell domain.Cell) error { return nil },
		func(cell domain.Cell) error { return nil },
		func(cell domain.Cell) (domain.Cell, error) {
			if err := cell.MarkDone(); err != nil {
				return domain.Cell{}, err
			}
			return cell, nil
		},
	)
	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if result.Action != ActionNone {
		t.Fatalf("action = %q, want %q", result.Action, ActionNone)
	}
	if !got.Cells[0].IsDone() {
		t.Fatal("IsDone = false, want true")
	}
}
```

Wire the app with a `runMarkDone` seam similar to `runEnter` / `runDelete`:

```go
var runMarkDone = func(ctx context.Context, state usecase.CellStatePort, cell domain.Cell) (domain.Cell, error) {
	uc := usecase.MarkCellDoneUseCase{State: state}
	return uc.Execute(ctx, usecase.MarkCellDoneInput{Cell: cell.Name})
}
```

Add a `l` callback to `runView` so the view can request the done transition and replace the selected cell with the returned updated cell:

```go
_, err = runView(ctx, cells,
	func(cell domain.Cell) error { return runEnter(ctx, configAdapter, provider.Factory{Runner: runner}, cell) },
	func(cell domain.Cell) error { return runDelete(ctx, configAdapter, provider.Factory{Runner: runner}, provider.Factory{Runner: runner}, provider.Factory{Runner: runner}, stateAdapter, cell) },
	func(cell domain.Cell) (domain.Cell, error) { return runMarkDone(ctx, stateAdapter, cell) },
)
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run:

```bash
go test ./internal/adapter/view ./internal/app -v
```

Expected: fail because `l` behavior and done updates are not wired yet.

- [ ] **Step 3: Implement the minimal code**

Teach the view model to call the new done callback on `l`, replace the selected cell with the returned updated cell, and keep the view open.

- [ ] **Step 4: Run the tests to confirm they pass**

Run:

```bash
go test ./internal/adapter/view ./internal/app -v
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/view/model.go internal/adapter/view/run.go internal/adapter/view/model_test.go internal/adapter/view/run_test.go internal/app/cli.go internal/app/cli_test.go
git commit -m "Add done transition from view"
```

### Task 4: Final verification

**Files:**
- None

- [ ] **Step 1: Run the full test suite**

Run:

```bash
go test ./...
```

Expected: pass.

- [ ] **Step 2: Verify the CLI behavior**

Run:

```bash
go run ./cmd/pdev ls
```

Expected: existing list output still works.

- [ ] **Step 3: Commit the feature**

```bash
git add cmd go.mod go.sum internal docs/superpowers/specs/2026-05-24-cell-status-design.md docs/superpowers/plans/2026-05-24-cell-status.md
git commit -m "Add cell done workflow"
```
