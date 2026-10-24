package usecase

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

// statusTestPorts records saves and notifications around the version-aware fake.
type statusTestPorts struct {
	*fakePorts
}

func newStatusTestPorts(t *testing.T) statusTestPorts {
	t.Helper()
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"})
	return statusTestPorts{fakePorts: ports}
}

func (p statusTestPorts) UpdateCells(ctx context.Context, update func(CellSet) (CellSet, error)) error {
	if err := p.fakePorts.UpdateCells(ctx, update); err != nil {
		return err
	}
	p.calls = append(p.calls, "save")
	return nil
}

func (p statusTestPorts) Notification(driver domain.NotificationDriverType) (Notifier, error) {
	p.calls = append(p.calls, "factory:notification:"+string(driver))
	return p, nil
}

func (p statusTestPorts) NotifyReady(_ context.Context, resource domain.WorkspaceResource, message string) error {
	p.calls = append(p.calls, "notify:"+resource.Name+":"+message)
	return nil
}

func TestMarkCellReadyはReadyにして通知する(t *testing.T) {
	ports := newStatusTestPorts(t)
	ports.cells.Commanders[0].MarkPending()
	cell, err := (MarkCellReadyUseCase{Cells: ports, NotificationFactory: ports}).Execute(context.Background(), MarkCellReadyInput{Cell: "123"})
	if err != nil {
		t.Fatalf("MarkCellReadyでエラーが返った: %v", err)
	}
	if cell.Status != domain.Ready || ports.cells.Commanders[0].Status != domain.Ready || ports.cells.Commanders[0].Version != 2 {
		t.Fatalf("cell = %#v, stored = %#v", cell, ports.cells.Commanders[0])
	}
	want := []string{"save", "factory:notification:none", "notify:myapp-123:Ready: 123"}
	if !reflect.DeepEqual(ports.calls, want) {
		t.Fatalf("calls = %#v, want %#v", ports.calls, want)
	}
}

func TestMarkCellReadyはReadyのCellでも成功し通知する(t *testing.T) {
	ports := newStatusTestPorts(t)
	if _, err := (MarkCellReadyUseCase{Cells: ports, NotificationFactory: ports}).Execute(context.Background(), MarkCellReadyInput{Cell: "group-cell-1"}); err != nil {
		t.Fatal(err)
	}
	if ports.cells.Commanders[0].Status != domain.Ready || ports.cells.Commanders[0].Version != 1 {
		t.Fatalf("stored = %#v", ports.cells.Commanders[0])
	}
	want := []string{"save", "factory:notification:none", "notify:myapp-123:Ready: 123"}
	if !reflect.DeepEqual(ports.calls, want) {
		t.Fatalf("calls = %#v, want %#v", ports.calls, want)
	}
}

func TestMarkCellPendingはPendingにして通知しない(t *testing.T) {
	ports := newStatusTestPorts(t)
	cell, err := (MarkCellPendingUseCase{Cells: ports}).Execute(context.Background(), MarkCellPendingInput{Cell: "123"})
	if err != nil {
		t.Fatalf("MarkCellPendingでエラーが返った: %v", err)
	}
	if cell.Status != domain.Pending || ports.cells.Commanders[0].Status != domain.Pending {
		t.Fatalf("cell = %#v", cell)
	}
	if _, err := (MarkCellPendingUseCase{Cells: ports}).Execute(context.Background(), MarkCellPendingInput{Cell: "123"}); err != nil {
		t.Fatalf("pending again returned %v", err)
	}
	if want := []string{"save", "save"}; !reflect.DeepEqual(ports.calls, want) {
		t.Fatalf("calls = %#v, want %#v", ports.calls, want)
	}
}

func TestMarkCellPendingとReadyは見つからないCellを保存も通知もしない(t *testing.T) {
	ports := newStatusTestPorts(t)
	_, pendingErr := (MarkCellPendingUseCase{Cells: ports}).Execute(context.Background(), MarkCellPendingInput{Cell: "missing"})
	_, readyErr := (MarkCellReadyUseCase{Cells: ports, NotificationFactory: ports}).Execute(context.Background(), MarkCellReadyInput{Cell: "cell-1"})
	for _, err := range []error{pendingErr, readyErr} {
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("error = %v", err)
		}
	}
	if len(ports.calls) != 0 || ports.cells.Commanders[0].Version != 1 {
		t.Fatalf("calls = %#v, stored = %#v", ports.calls, ports.cells.Commanders[0])
	}
}
