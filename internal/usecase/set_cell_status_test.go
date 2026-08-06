package usecase

import (
	"context"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestSetCellStatusはReady時に通知する(t *testing.T) {
	ports := &setStatusTestPorts{
		cells: []domain.Cell{{ID: "cell-1", Issue: "123", Name: "feature-123"}},
	}

	uc := SetCellStatusUseCase{State: ports, Notifier: ports}
	cell, err := uc.Execute(context.Background(), SetCellStatusInput{Cell: "123", Status: domain.Ready})
	if err != nil {
		t.Fatalf("SetCellStatusでエラーが返った: %v", err)
	}
	if got := cell.Status(); got != domain.Ready {
		t.Fatalf("Status = %q, want %q", got, domain.Ready)
	}
	want := []string{"save:1", "notify:feature-123:Ready: feature-123"}
	if !reflect.DeepEqual(ports.calls, want) {
		t.Fatalf("calls = %#v, want %#v", ports.calls, want)
	}
}

func TestSetCellStatusはPending時に通知しない(t *testing.T) {
	ports := &setStatusTestPorts{
		cells: []domain.Cell{{ID: "cell-1", Issue: "123", Name: "feature-123"}},
	}

	uc := SetCellStatusUseCase{State: ports, Notifier: ports}
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
	cells []domain.Cell
	calls []string
}

func (p *setStatusTestPorts) LoadCells(ctx context.Context) ([]domain.Cell, error) {
	_ = ctx
	return append([]domain.Cell(nil), p.cells...), nil
}

func (p *setStatusTestPorts) UpdateCells(ctx context.Context, update func([]domain.Cell) ([]domain.Cell, error)) error {
	_ = ctx
	cells, err := update(append([]domain.Cell(nil), p.cells...))
	if err != nil {
		return err
	}
	p.cells = append([]domain.Cell(nil), cells...)
	p.calls = append(p.calls, "save:1")
	return nil
}

func (p *setStatusTestPorts) NotifyReady(ctx context.Context, cell domain.Cell, message string) error {
	_ = ctx
	p.calls = append(p.calls, "notify:"+cell.Name+":"+message)
	return nil
}
