package domain

import "fmt"

// CellVersion is the persisted version used for optimistic locking. The domain
// only reads it; the state adapter advances it.
type CellVersion uint64

func NewCellVersion(value uint64) (CellVersion, error) {
	if value == 0 {
		return 0, fmt.Errorf("cell version must be greater than zero")
	}
	return CellVersion(value), nil
}
