package domain

import "fmt"

type CellStatus string

const (
	Pending CellStatus = "pending"
	Ready   CellStatus = "ready"
)

func NewCellStatus(value string) (CellStatus, error) {
	status := CellStatus(value)
	switch status {
	case Pending, Ready:
		return status, nil
	default:
		return status, fmt.Errorf("unsupported status %q", status)
	}
}
