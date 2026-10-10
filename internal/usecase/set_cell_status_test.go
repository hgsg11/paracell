package usecase

import (
	"context"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestSetCellStatusはReady時に通知する(t *testing.T) {
	ports := &setStatusTestPorts{
		cells: newUsecaseTestSet(t, "cell-1", "123", "feat"),
	}

	uc := SetCellStatusUseCase{Cells: ports, NotificationFactory: ports}
	cell, err := uc.Execute(context.Background(), SetCellStatusInput{Cell: "123", Status: domain.Ready})
	if err != nil {
		t.Fatalf("SetCellStatusでエラーが返った: %v", err)
	}
	if cell.Status != domain.Ready {
		t.Fatalf("cell = %#v, want status %q", cell, domain.Ready)
	}
	want := []string{"save:1", "factory:notification:none", "notify:myapp-123:Ready: 123"}
	if !reflect.DeepEqual(ports.calls, want) {
		t.Fatalf("calls = %#v, want %#v", ports.calls, want)
	}
}

func TestSetCellStatusはPending時に通知しない(t *testing.T) {
	ports := &setStatusTestPorts{
		cells: newUsecaseTestSet(t, "cell-1", "123", "feat"),
	}

	uc := SetCellStatusUseCase{Cells: ports, NotificationFactory: ports}
	_, err := uc.Execute(context.Background(), SetCellStatusInput{Cell: "123", Status: domain.Pending})
	if err != nil {
		t.Fatalf("SetCellStatusでエラーが返った: %v", err)
	}
	want := []string{"save:1"}
	if !reflect.DeepEqual(ports.calls, want) {
		t.Fatalf("calls = %#v, want %#v", ports.calls, want)
	}
}

type setStatusTestPorts struct {
	cells CellSet
	calls []string
}

func (p *setStatusTestPorts) LoadCells(ctx context.Context) (CellSet, error) {
	_ = ctx
	return NewCellSet(p.cells.Commanders, p.cells.Groups, p.cells.Targets, p.cells.Dependencies), nil
}

func (p *setStatusTestPorts) UpdateCells(ctx context.Context, update func(CellSet) (CellSet, error)) error {
	_ = ctx
	cells, err := update(NewCellSet(p.cells.Commanders, p.cells.Groups, p.cells.Targets, p.cells.Dependencies))
	if err != nil {
		return err
	}
	p.cells = cells
	p.calls = append(p.calls, "save:1")
	return nil
}

func (p *setStatusTestPorts) Notification(driver domain.NotificationDriverType) (Notifier, error) {
	p.calls = append(p.calls, "factory:notification:"+string(driver))
	return p, nil
}

func (p *setStatusTestPorts) NotifyReady(ctx context.Context, sessionName string, message string) error {
	_ = ctx
	p.calls = append(p.calls, "notify:"+sessionName+":"+message)
	return nil
}
