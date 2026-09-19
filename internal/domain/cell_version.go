package domain

import "fmt"

type CellVersion uint64

func NewCellVersion(value uint64) (CellVersion, error) {
	if value == 0 {
		return 0, fmt.Errorf("cell version must be greater than zero")
	}
	return CellVersion(value), nil
}

func (v CellVersion) Add() (CellVersion, error) {
	return NewCellVersion(uint64(v) + 1)
}
